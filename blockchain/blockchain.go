package blockchain

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/worker"
	"github.com/open-horizon/go-solidity/contract_api"
	gwhisper "github.com/open-horizon/go-whisper"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const WRITE_POLL_TIMEOUT_S = 480

// must be safely-constructed!!
type BlockchainWorker struct {
	worker.Worker            // embedded field
	db                       *bolt.DB
	registryContract         *contract_api.SolidityContract
	bankContract             *contract_api.SolidityContract
	directoryContract        *contract_api.SolidityContract
	whisperDirectoryContract *contract_api.SolidityContract
}

func NewBlockchainWorker(config *config.Config, db *bolt.DB) *BlockchainWorker {
	messages := make(chan events.Message)
	commands := make(chan worker.Command, 100)

	worker := &BlockchainWorker{
		Worker: worker.Worker{
			Manager: worker.Manager{
				Config:   config,
				Messages: messages,
			},

			Commands: commands,
		},

		db: db,
	}

	glog.Info("Starting blockchain worker")
	worker.start()
	return worker
}

func deployDeviceContract(deviceOwner string, pendingContract persistence.PendingContract, gethURL string) (*contract_api.SolidityContract, error) {

	// Deploy the device contract
	contract := contract_api.SolidityContractFactory("container_executor")
	contract.Set_skip_eventlistener()

	if _, err := contract.Deploy_contract(deviceOwner, gethURL); err != nil {
		return nil, fmt.Errorf("Could not deploy device contract: %v", err)
	}
	glog.Infof("container_executor deployed at %v", contract.Get_contract_address())

	// Test to make sure the device contract is invokable
	if owner, err := contract.Invoke_method("get_owner", nil); err != nil {
		return nil, fmt.Errorf("Could not invoke get_owner on device contract: %v", err)
	} else if owner.(string)[2:] != deviceOwner {
		return nil, fmt.Errorf("Wrong owner returned: %v should be %v", owner, deviceOwner)
	}

	return contract, nil
}

func (w *BlockchainWorker) connectTokenBank(contract *contract_api.SolidityContract, directoryContract *contract_api.SolidityContract) error {

	if tokenBankAddress, err := address("token_bank", directoryContract); err != nil {
		return err
	} else {

		// Connect the global token bank to the contract
		if _, err := contract.Invoke_method("set_bank", ContractParam(tokenBankAddress)); err != nil {
			return fmt.Errorf("Could not find token_bank in directory: %v", err)
		} else {

			expire := time.Now().Unix() + WRITE_POLL_TIMEOUT_S

			// poll on set_bank write to ensure it was set
			for {
				if time.Now().Unix() > expire {
					return fmt.Errorf("Expired write val check poll on get_bank")
				} else {
					if echoBank, err := contract.Invoke_method("get_bank", nil); err != nil {
						return fmt.Errorf("Could not invoke get_bank: %v", err)
					} else {
						if echoBank.(string) != "0x0000000000000000000000000000000000000000" {
							// success!
							glog.Infof("Device using bank at %v", echoBank)
							break
						}
					}
				}
			}

			if bal, err := w.bankContract.Invoke_method("account_balance", nil); err != nil {
				return fmt.Errorf("Could not get token balance: %v", err)
			} else {
				glog.Infof("Owner bacon balance is: %v", bal)
			}

			return nil
		}
	}
}

// handle the transition of a pending contract to a contract that can be polled; a db update operation
func (w *BlockchainWorker) pendingToNewContractRecord(pending persistence.PendingContract, contract *contract_api.SolidityContract) error {

	if contract.Get_contract_address() == "" {
		return fmt.Errorf("Contract lacks an address, unable to persist it")
	} else {
		// remove pending contract from DB, add contract so contract state polling can begin
		return w.db.Update(func(tx *bolt.Tx) error {
			// do both updates in a single transaction

			// TODO: move this to the persistence lib
			if b, err := tx.CreateBucketIfNotExists([]byte(persistence.E_CONTRACTS)); err != nil {
				return err
			} else if existing := b.Get([]byte(contract.Get_contract_address())); existing != nil {
				// check for existing contract with this id, bail if there is one because that's lunacy
				return fmt.Errorf("Contract already exists in %s bucket: %v", existing, persistence.E_CONTRACTS)
			} else if established, err := persistence.NewEstablishedContract(&pending, contract.Get_contract_address()); err != nil {
				return fmt.Errorf("Unable to create persistent contract instance: %v", err)
			} else if bytes, err := json.Marshal(established); err != nil {
				return fmt.Errorf("Unable to serialize contract instance: %v", err)
			} else if err := b.Put([]byte(contract.Get_contract_address()), []byte(bytes)); err != nil {
				return fmt.Errorf("Unable to persist contract: %v", err)
			} else if pBucket := tx.Bucket([]byte(persistence.P_CONTRACTS)); pBucket == nil {
				return fmt.Errorf("Persistence contract bucket (%v) unavailable", pBucket)
			} else if err := pBucket.Delete([]byte(*pending.Name)); err != nil {
				return fmt.Errorf("Unable to delete pendingContract %v from bucket: %v, Error: %v", pending, persistence.P_CONTRACTS, err)
			} else {
				glog.V(2).Infof("Succeeded at promoting pending contract: %v to established contract: %v in local storage", pending, established)
				return nil // end transaction
			}
		})
	}
}

func prepEnvironmentAdditions(dbContractRecord *persistence.EstablishedContract, agreementId string, deviceRegistry *contract_api.SolidityContract) (map[string]string, error) {
	environmentAdditions := make(map[string]string, 0)

	if attributesRaw, err := deviceRegistry.Invoke_method("get_description", ContractParam(dbContractRecord.ContractAddress)); err != nil {
		return nil, fmt.Errorf("Could not invoke get_description on deviceRegistry. Error: %v", err)
	} else {
		// extract attributes from the device contract, write them to environmentAdditions
		attribute_map := extractAll(attributesRaw.([]string))
		for key, val := range attribute_map {
			if val != "" {
				environmentAdditions[fmt.Sprintf("MTN_%s", strings.ToUpper(key))] = val
			}
		}

		// add private envs to the environmentAdditions
		if p_env := dbContractRecord.PrivateEnvironmentAdditions; p_env != nil {
			for key, val := range p_env {
				if val != "" {
					environmentAdditions[fmt.Sprintf("MTN_%s", strings.ToUpper(key))] = val
				}
			}
		}

		// add contract / agreement features too
		environmentAdditions["MTN_CONTRACT"] = dbContractRecord.ContractAddress
		environmentAdditions["MTN_AGREEMENTID"] = agreementId
	}

	return environmentAdditions, nil
}

func (w *BlockchainWorker) registerContractInDirectory(directoryContract *contract_api.SolidityContract, contract *contract_api.SolidityContract, pending persistence.PendingContract) error {

	// extract device detail from pending contract and write them to contract in the registry
	attrs := make([]string, 0)
	attrs = append(attrs, "name", *pending.Name)
	attrs = append(attrs, "arch", pending.Arch)
	attrs = append(attrs, "cpus", strconv.Itoa(pending.CPUs))
	attrs = append(attrs, "ram", strconv.Itoa(*pending.RAM))
	attrs = append(attrs, "hourly_cost_bacon", strconv.FormatInt(*pending.HourlyCostBacon, 10))
	attrs = append(attrs, "is_loc_enabled", strconv.FormatBool(pending.IsLocEnabled))

	// extract the application related data from pending contract
	for key, value := range *pending.AppAttributes {
		attrs = append(attrs, key, value)
	}

	contractRecord := ContractParam(contract.Get_contract_address())
	contractRecord = append(contractRecord, attrs)

	glog.V(3).Infof("Contract attributes to register: %v", contractRecord)

	if _, err := w.registryContract.Invoke_method("register", contractRecord); err != nil {
		return fmt.Errorf("Could not register device: %v", err)
	} else {

		empty_array := make([]interface{}, 0, 5)
		has_attributes := func(c *contract_api.SolidityContract, params []interface{}, old interface{}) (bool, error) {
			if resp, err := c.Invoke_method("get_description", params); err != nil {
				glog.Errorf("Error invoking get_description on %v, error: %v", c.Get_contract_address(), err)
				return false, err
			} else {
				// return true if result != old
				switch resp.(type) {
				case []string:
					glog.V(5).Infof("get_description returns %v at stable block %v", resp.([]string), c.Get_stable_block())
					return len(resp.([]string)) != len(old.([]interface{})), nil
				default:
					return false, nil
				}
			}
		}

		if err := verify_blockchain_change(has_attributes, w.registryContract, ContractParam(contract.Get_contract_address()), empty_array); err != nil {
			glog.Errorf("Unable to verify contract registration %v on blockchain. Error: %v", contract.Get_contract_address(), err)
			return err
		} else if attributes, err := w.registryContract.Invoke_method("get_description", ContractParam(contract.Get_contract_address())); err != nil {
			return fmt.Errorf("Could not invoke get_description on deviceRegistry: %v", err)
		} else if baconAskStr := extractAttr(attributes.([]string), "hourly_cost_bacon"); baconAskStr == "" {
			return errors.New("hourly_cost_bacon missing from blockchain device registry for this contract")
		} else {
			glog.Infof("Device registered with: %v", attributes)
		}
		return nil
	}
}

// if contract is nil, expected that contractAddress will be provided and the contract can be loaded
// N.B.: this does *not* end workloads or bring local state into consistency, we wait for the blockchain poll process to find that the write was successful and then effect consistency
func (w *BlockchainWorker) endContract(contractAddress string, method string, agreementId string) error {
	if contract, err := w.deviceContract(w.Config.GethURL, contractAddress); err != nil {
		return fmt.Errorf("Could not load contract with address: %v. Error: %v", contractAddress, err)
	} else if _, err := contract.Invoke_method(method, nil); err != nil {
		return fmt.Errorf("Error writing contract end to blockchain; Attempted contract: %v. Error: %v", contract, err)
	} else {
		// Delay here to ensure that current agreement id is out of the blockchain before continuing
		agreement_changed := func(c *contract_api.SolidityContract, params []interface{}, old interface{}) (bool, error) {
			if resp, err := c.Invoke_method("get_agreement_id", params); err != nil {
				glog.Errorf("Error invoking get_agreement_id on %v, error: %v", c.Get_contract_address(), err)
				return false, err
			} else {
				// return true if result != old
				switch resp.(type) {
				case string:
					glog.V(5).Infof("get_agreement_id returns %v at stable block %v", resp.(string), c.Get_stable_block())
					return resp.(string) != old.(string), nil
				default:
					return false, nil
				}
			}
		}

		if err := verify_blockchain_change(agreement_changed, contract, nil, agreementId); err != nil {
			return fmt.Errorf("Unable to verify cancellation of agreement %v on contract %v. Error: %v", agreementId, contractAddress, err)
		}

		glog.Infof("Ended contract %v. method: %v", contract, method)
		_, err := persistence.ContractStateNew(w.db, contractAddress)
		return err
	}
}

func verify_blockchain_change(f func(c *contract_api.SolidityContract, params []interface{}, old interface{}) (bool, error), c *contract_api.SolidityContract, params []interface{}, old interface{}) error {
	start_timer := time.Now()
	for {
		time.Sleep(5000 * time.Millisecond)
		delta := time.Now().Sub(start_timer).Seconds()
		glog.V(5).Infof("Polled for %v seconds for blockchain state change on %v.", delta, c.Get_contract_address())
		if int(delta) < WRITE_POLL_TIMEOUT_S {
			if change, err := f(c, params, old); err != nil {
				return err
			} else if change {
				glog.V(5).Infof("State change observed on %v.", c.Get_contract_address())
				return nil
			}
		} else {
			glog.Errorf("Timeout verifying blockchain state chage for %v.", c.Get_contract_address())
			return nil
		}
	}
}

func registerPending(pending persistence.PendingContract, w *BlockchainWorker) {
	glog.Infof("Starting registration of pending contract: %v", pending)

	if contract, err := deployDeviceContract(w.Config.BlockchainAccountId, pending, w.Config.GethURL); err != nil {
		glog.Errorf("Failed to deploy new device contract for pending contract: %v. Error: %v", pending, err)
	} else if err := w.connectTokenBank(contract, w.directoryContract); err != nil {
		glog.Errorf("Failed to set up contract and token bank references: %v", err)
	} else if err := w.registerContractInDirectory(w.directoryContract, contract, pending); err != nil {
		glog.Errorf("Failed to register contract in directory: %v", err)
	} else if err := w.pendingToNewContractRecord(pending, contract); err != nil {
		glog.Errorf("Error writing new contract record to local storage: %v", err)
	} else {
		// send an event
		if devmode, err := persistence.GetDevmode(w.db); err != nil {
			glog.Errorf("Error getting devmode from db:%v", err)
		} else {
			registered := NewBlockchainRegMessage(events.CONTRACT_REGISTERED, pending, devmode)
			w.Messages <- registered
		}
		// success!
		glog.Infof("Succeeded at creating new contract in blockchain %v given pending contract: %v", contract, pending)
	}
}

func (w *BlockchainWorker) updateWhisperIdInRegistry() {

	id := func() string {
		if wId, err := gwhisper.AccountId(w.Config.GethURL); err != nil {
			glog.Error(err)
			return ""
		} else {
			return wId
		}
	}

	last_log := int64(0)
	level := 1

	for {
		now := time.Now().Unix()

		diskId := id()

		if now-last_log > 600 {
			level = 3
			last_log = now
		} else {
			level = 5
		}

		if bkId, err := w.whisperDirectoryContract.Invoke_method("get_entry", ContractParam(w.Config.BlockchainAccountId)); err != nil && bkId != nil {
			glog.Errorf("Could not read whisper id %v to whisper directory. Error: %v", bkId, err)
		} else if bkId != diskId {
			glog.Infof("Whisper id on disk (%v) differs from whisper id in blockchain (%v), updating blockchain.", diskId, bkId)

			if _, err := w.whisperDirectoryContract.Invoke_method("add_entry", ContractParam(diskId)); err != nil {
				glog.Errorf("Could not write whisper id %v to whisper directory. Error: %v", diskId, err)
			} else {
				glog.V(glog.Level(level)).Infof("Succeeded writing whisper id %v to whisper directory", diskId)

				whisper_changed := func(c *contract_api.SolidityContract, params []interface{}, old interface{}) (bool, error) {
					if resp, err := c.Invoke_method("get_entry", params); err != nil {
						glog.Errorf("Error invoking get_entry on %v, error: %v", c.Get_contract_address(), err)
						return false, err
					} else {
						// return true if result != old
						switch resp.(type) {
						case string:
							glog.V(5).Infof("get_entry returns %v at stable block %v", resp.(string), c.Get_stable_block())
							return resp.(string) != old.(string), nil
						default:
							return false, nil
						}
					}
				}

				if err := verify_blockchain_change(whisper_changed, w.whisperDirectoryContract, ContractParam(w.Config.BlockchainAccountId), bkId.(string)); err != nil {
					glog.Errorf("Unable to verify whisper Id %v write to blockchain. Error: %v", bkId, err)
				}
			}
		} else {
			glog.V(glog.Level(level)).Infof("Whisper id on disk and in blockchain are consistent")
		}

		time.Sleep(30 * time.Second)
		runtime.Gosched()
	}
}

func (w *BlockchainWorker) pollPendingContracts() {
	glog.V(2).Infof("Polling pending contracts")
	handled := make(map[string]*persistence.PendingContract, 0)

	var m = &sync.Mutex{}

	for {
		if pendingContracts, err := persistence.FindPendingContracts(w.db); err != nil {
			glog.Errorf("Unable to read pending contracts from database: %v", err)
		} else {
			for _, pending := range pendingContracts {

				m.Lock()
				if handled[*pending.Name] == nil {
					handled[*pending.Name] = &pending
					go registerPending(pending, w) // schedule registration in a new goroutine
					glog.V(4).Infof("Scheduled registration of pending contract: %v", pending)
				}
				m.Unlock()
			}
		}

		time.Sleep(1 * time.Second)
	}
}

func (w *BlockchainWorker) handleInContractAgreementStateChange(dbContractRecord *persistence.EstablishedContract, topAgreementId string, contract *contract_api.SolidityContract) error {

	// caller guarantees topAgreementId is set

	if topAgreementId != dbContractRecord.CurrentAgreementId {
		// have a current agreement id in the db, but it doesn't match blockchain agreement id

		// clean up, agreement in blockchain inconsistent with our local state, hose dbContractRecord and let re-negotiate

		glog.V(2).Infof("Ending DB contract b/c topAgreementId (%v) and agreement id (%v) stored locally do not match", topAgreementId, dbContractRecord.CurrentAgreementId)
		if err := w.endDBContract(dbContractRecord, topAgreementId); err != nil {
			return fmt.Errorf("Error handling contract agreement state changes: %v", err)
		}
	} // else unnecessary, the state is consistent b/c isInContract.(bool) && db.ContractRecord.CurrentAgreementId == topAgreementId

	// new agreement that our local state doesn't know about, pursue it
	if dbContractRecord.CurrentAgreementId == "" {

		// newly-formed agreement on blockchain
		glog.Infof("Contract %v has new agreement: %v", contract.Get_contract_address(), topAgreementId)

		if containerProvider, err := contract.Invoke_method("get_container_provider", nil); err != nil {
			return fmt.Errorf("Unable to get container provider: %v", err)
		} else {
			glog.Infof("Agreement formed with provider: %v", containerProvider)

			proposalParam := make([]interface{}, 0, 10)
			proposalParam = append(proposalParam, containerProvider)
			proposalParam = append(proposalParam, w.Config.BlockchainAccountId)
			proposalParam = append(proposalParam, contract.Get_contract_address())

			if proposalAmount, err := w.bankContract.Invoke_method("get_escrow_amount", proposalParam); err != nil {
				return fmt.Errorf("Unable to use tokenBank to discover escrow bacon amount: %v", err)
			} else if attributes, err := w.registryContract.Invoke_method("get_description", ContractParam(contract.Get_contract_address())); err != nil {
				return fmt.Errorf("Unable to fetch device attributes from blockchain: %v", err)
			} else if baconAskStr := extractAttr(attributes.([]string), "hourly_cost_bacon"); baconAskStr == "" {
				return fmt.Errorf("hourly_cost_bacon missing from blockchain device registry for this contract")
			} else if baconAsk, err := strconv.ParseUint(baconAskStr, 10, 64); err != nil {
				return fmt.Errorf("Bogus value in hourly_cost_bacon field of device attributes: %v", err)
			} else if baconAsk <= proposalAmount.(uint64) {

				glog.Infof("Proposed payment meets demand of asking price, pursuing acceptance of agreement %v on contract: %v with provider: %v by voting to accept in bank", topAgreementId, contract.Get_contract_address(), containerProvider)

				voteParam := make([]interface{}, 0, 10)
				voteParam = append(voteParam, containerProvider)
				voteParam = append(voteParam, contract.Get_contract_address())
				voteParam = append(voteParam, true)

				if _, err := w.bankContract.Invoke_method("counter_party_vote", voteParam); err != nil {
					return fmt.Errorf("Failed to send counter party vote: %v", err)
				} else if agreementId, err := contract.Invoke_method("get_agreement_id", nil); err != nil {
					return fmt.Errorf("Unable to retrieved agreementID from contract: %v", err)
				} else {

					// TODO: determine if it's necessary to check here that this agreementId is the same as the topAgreementId
					currentAgreementId := agreementId.(string)
					glog.Infof("Token bank voting complete, proceeding with whisper exchange then checking back later for proposal_accepted on contract in blockchain to ensure agreement accepted by other party")

					if whisperId, err := contract.Invoke_method("get_whisper", nil); err != nil {
						return fmt.Errorf("Unable to retrieve whisperId from contract: %v", err)
					} else if environmentAdditions, err := prepEnvironmentAdditions(dbContractRecord, currentAgreementId, w.registryContract); err != nil {
						return fmt.Errorf("Failed to create environment additions map from contract: %v", err)
					} else if establishedContract, err := persistence.ContractStateInAgreement(w.db, dbContractRecord.ContractAddress, containerProvider.(string), currentAgreementId, environmentAdditions); err != nil {
						return fmt.Errorf("Failed to update db agreementId: %v", err)
					} else {
						// not really 'accepted', more the case that agreement was reached. acceptance by the other party happens later
						glog.Infof("Contract proposal accepted, sending CONTRACT_ACCEPTED event for contract with address: %v", establishedContract.ContractAddress)
						accepted := NewBlockchainMessage(events.CONTRACT_ACCEPTED,
							&Agreement{
								EstablishedContract: establishedContract,
								WhisperId:           whisperId.(string),
							})
						w.Messages <- accepted
					}
				}
			} else if err := w.endContract(contract.Get_contract_address(), "reject_container", topAgreementId); err != nil {
				return fmt.Errorf("Failed to reject container in contract: %v. Error: %v", contract.Get_contract_address(), err)
			} else {
				// rejected b/c too low
				glog.Infof("Rejected proposed agreement on contract %v by provider: %v. proposed bacon: %v does need meet requested value: %v", contract.Get_contract_address(), containerProvider, proposalAmount, baconAsk)
			}
		}

	}

	return nil
}

func (w *BlockchainWorker) endDBContract(dbContractRecord *persistence.EstablishedContract, topAgreementId string) error {
	if dbContractRecord.CurrentAgreementId != "" {

		// newly-ended agreement on blockchain
		// cancel whatever resources exist in this agreement, e.g. send event to container manager
		// change db record, set CurrentAgreementId to empty string

		if _, err := persistence.ContractStateNew(w.db, dbContractRecord.ContractAddress); err != nil {
			return fmt.Errorf("Failed to update db agreementId: %v", err)
		} else {
			glog.V(2).Infof("Brought blockchain contract state and db record state into consistency by unsetting db agreementId. Previously, CurrentAgreementId: %v, topAgreementId: %v, for Contract address: %v", dbContractRecord.CurrentAgreementId, topAgreementId, dbContractRecord.ContractAddress)

			// make a copy of the agreement before sending message so old state is preserved
			cpRecord := *dbContractRecord

			accepted := NewBlockchainMessage(events.CONTRACT_ENDED, &Agreement{
				EstablishedContract: &cpRecord,
				WhisperId:           "",
			})
			w.Messages <- accepted
		}
	}

	return nil
}

func (w *BlockchainWorker) handleContractAgreementAcceptedState(dbContractRecord *persistence.EstablishedContract, topAgreementId string, contract *contract_api.SolidityContract) (bool, error) {

	if containerProvider, err := contract.Invoke_method("get_container_provider", nil); err != nil {
		return false, fmt.Errorf("Unable to get container provider: %v", err)
	} else if agreementId, err := contract.Invoke_method("get_agreement_id", nil); err != nil {
		return false, fmt.Errorf("Unable to fetch agreementId from contract: %v", err)
	} else if agreementId.(string) != topAgreementId {
		// The contract was picked up by another provider while we were processing, need to begin the contract processing from the start
		glog.Info("Contract topAgreementId and agreementId on contract not the same, skipping acceptance check")

		return false, nil
	} else {
		// can be the case that he bailed here; important to have checked agreementId to guard this

		proposalParam := make([]interface{}, 0, 10)
		proposalParam = append(proposalParam, containerProvider)
		proposalParam = append(proposalParam, w.Config.BlockchainAccountId)
		proposalParam = append(proposalParam, contract.Get_contract_address())

		if proposalAcceptance, err := w.bankContract.Invoke_method("get_proposer_accepted", proposalParam); err != nil {
			return false, fmt.Errorf("Unable to fetch proposal facts from tokenBank. Error: %v", err)
		} else if proposalAcceptance.(bool) && dbContractRecord.AgreementAcceptedTime == 0 {
			persistence.ContractStateAccepted(w.db, dbContractRecord.ContractAddress)
			return true, nil
		} // don't need the other side of this conditional: we already knew we got accepted

		return false, nil
	}
}

func (w *BlockchainWorker) evaluateContractChanges(dbContractRecord *persistence.EstablishedContract, contract *contract_api.SolidityContract) error {
	glog.V(4).Infof("Evaluating changes on contract %v", contract.Get_contract_address())

	// TODO: evaluate how useful this is given that we don't long-poll here any longer for proposal_acceptance
	if topAgreement, err := contract.Invoke_method("get_agreement_id", nil); err != nil {
		return fmt.Errorf("Unable to fetch agreementId from contract: %v", err)
	} else if isInContract, err := contract.Invoke_method("in_contract", nil); err != nil {
		return fmt.Errorf("Error reading contract detail from: %v. Error: %v", contract, err)
	} else {
		topAgreementId := topAgreement.(string)

		if isInContract.(bool) {
			if topAgreementId == "" {
				return fmt.Errorf("in_contract is true for %v, but empty agreementId, what gives?", dbContractRecord.ContractAddress)
			}

			if newAcceptance, err := w.handleContractAgreementAcceptedState(dbContractRecord, topAgreementId, contract); err != nil {
				return fmt.Errorf("Error handling contract payment state changes: %v", err)
			} else if newAcceptance {
				// return from this poll loop b/c it's unnecessary to check for a contract agreement state change just after closing acceptance
				return nil

			} else if err := w.handleInContractAgreementStateChange(dbContractRecord, topAgreementId, contract); err != nil {
				return fmt.Errorf("Error handling contract agreement state changes: %v", err)
			}

		} else {
			glog.V(2).Infof("Ending DB contract %v b/c in_contract is false", topAgreementId)
			if err := w.endDBContract(dbContractRecord, topAgreementId); err != nil {
				return fmt.Errorf("Error handling contract agreement state changes: %v", err)
			}
		}

		return nil
	}
}

func (w *BlockchainWorker) processEstablishedContracts() {

	// do db fetch each time
	fetch := func() []persistence.EstablishedContract {
		if dbContracts, err := persistence.FindEstablishedContracts(w.db, []persistence.ECFilter{persistence.UnarchivedECFilter()}); err != nil {
			glog.Errorf("Unable to read established contracts from database: %v", err)
			return make([]persistence.EstablishedContract, 0)
		} else {
			glog.V(4).Infof("Fetched %v established contracts from DB to process", len(dbContracts))
			return dbContracts
		}
	}

	cachedContracts := make([]*contract_api.SolidityContract, 0)

	cached := func(address string) (*contract_api.SolidityContract, error) {
		for _, c := range cachedContracts {
			if address == c.Get_contract_address() {
				return c, nil
			}
		}

		if loadedContract, err := w.deviceContract(w.Config.GethURL, address); err != nil {
			return nil, fmt.Errorf("Unable to load device contract: %v", err)
		} else {
			cachedContracts = append(cachedContracts, loadedContract)
			return loadedContract, nil
		}
	}

	last_log := int64(0)
	level := 1

	for {
		for _, dbContract := range fetch() {
			now := time.Now().Unix()

			if now-last_log > 300 {
				level = 3
				last_log = now
			} else {
				level = 5
			}

			glog.V(glog.Level(level)).Infof("Operating on contract: %v", dbContract)

			if loadedContract, err := cached(dbContract.ContractAddress); err != nil {
				glog.Errorf("Unable to load device contract: %v", err)
			} else if err := w.evaluateContractChanges(&dbContract, loadedContract); err != nil {
				glog.Errorf("Unable to operate on contract: %v", err)
			}
			time.Sleep(30 * time.Second)
		}

		time.Sleep(500 * time.Millisecond)
	}
}

func (w *BlockchainWorker) start() {
	// only one worker at-a-time can invoke this function!

	loader := func() error {
		glog.V(2).Info("Starting blockchain contract loader process")

		if blockchainAccountId, err := AccountId(); err != nil {
			return fmt.Errorf("Unable to read blockchain account id. Error: %v", err)
		} else if blockchainDirectoryAddress, err := DirectoryAddress(); err != nil {
			return fmt.Errorf("Unable to read blockchain directory address: %v", err)
		} else if directoryContract, err := DirectoryContract(w.Config.GethURL, blockchainAccountId, blockchainDirectoryAddress); err != nil {
			return fmt.Errorf("Unable to read directory contract: %v", err)
		} else {
			w.Config.BlockchainAccountId = blockchainAccountId
			w.Config.BlockchainDirectoryAddress = blockchainDirectoryAddress
			glog.Infof("Wrote blockchain directory values to config object: %v", w.Config)

			w.directoryContract = directoryContract

			if c, err := ContractInDirectory(w.Config.GethURL, "whisper_directory", w.Config.BlockchainAccountId, w.directoryContract); err != nil {
				return err
			} else {
				w.whisperDirectoryContract = c
			}

			if c, err := ContractInDirectory(w.Config.GethURL, "token_bank", w.Config.BlockchainAccountId, w.directoryContract); err != nil {
				return err
			} else {
				w.bankContract = c
			}

			if c, err := ContractInDirectory(w.Config.GethURL, "device_registry", w.Config.BlockchainAccountId, w.directoryContract); err != nil {
				return err
			} else {
				w.registryContract = c
			}
		}
		return nil
	}

	for {
		if err := loader(); err != nil {
			// push these errors into lower logging level, they are the result of polling on startup
			glog.V(4).Infof("%v", err)
			time.Sleep(10 * time.Second)
		} else {
			break
		}
	}
	glog.Infof("Finished loading smart contracts")

	// eval changes in whisper id and update in blockchain registry
	go w.updateWhisperIdInRegistry()

	// process pending contracts
	go w.pollPendingContracts()

	// process established contracts
	go w.processEstablishedContracts()

	// TODO: handle case of contract killed? (permanently put out of service); tracked in EstablishedContract as "Archived"; need to provide API path to booz and risham to use it

	go func() {
		for {
			command := <-w.Commands
			glog.V(2).Infof("BlockchainWorker received command: %v", command)

			// TODO: add defaults to these cases
			switch command.(type) {
			case *BlockchainEndContractCommand:
				cmd, _ := command.(*BlockchainEndContractCommand)

				// bail if the current contract doesn't match the current agreement; could be a laggy event
				if contracts, err := persistence.FindEstablishedContracts(w.db, []persistence.ECFilter{persistence.UnarchivedECFilter(), persistence.AddressECFilter(cmd.ContractAddress)}); err != nil {
					glog.Error(err)
				} else if len(contracts) > 1 {
					glog.Errorf("Expected only one contract address: %v. Found: %v", cmd.ContractAddress, contracts)
				} else if len(contracts) == 0 {
					glog.Errorf("No matching db contract record for Contract: %v.", cmd.ContractAddress)
				} else if contracts[0].CurrentAgreementId != "" && contracts[0].CurrentAgreementId != cmd.AgreementId {
					glog.Infof("Refusing to end agreement: %v on contract: %v because the command's agreement id does not match the current agreement id.", cmd.AgreementId, cmd.ContractAddress)
				} else {

					var method string
					switch cmd.Cause {
					case events.CT_TERMINATED:
						method = "reject_container"
					// case events.CT_FULFILLED:
					// 	method = "exec_complete"
					case events.CT_ERROR:
						method = "reject_container"
					default:
						glog.Errorf("Unknown BlockchainEndContractCommand cause: %v", cmd.Cause)
					}

					if err := w.endContract(cmd.ContractAddress, method, cmd.AgreementId); err != nil {
						glog.Errorf("Failed ending contract %v with agreement ID %v using method %v. Error: %v", cmd.ContractAddress, cmd.AgreementId, method, err)
					} else {
						glog.Infof("Succeeded ending agreement %v with contract %v using method %v.", cmd.AgreementId, cmd.ContractAddress, method)
					}
				}
			default:
				glog.Errorf("Unknown command (%T): %v", command, command)
			}

			runtime.Gosched()
		}
	}()
}

type Agreement struct {
	EstablishedContract *persistence.EstablishedContract
	WhisperId           string
}

type BlockchainMessage struct {
	event     events.Event
	Agreement *Agreement
}

func (m BlockchainMessage) String() string {
	return fmt.Sprintf("event: %v, Agreement: %v", m.event, m.Agreement)
}

func (b *BlockchainMessage) Event() events.Event {
	return b.event
}

func NewBlockchainMessage(id events.EventId, agreement *Agreement) *BlockchainMessage {

	return &BlockchainMessage{
		event: events.Event{
			Id: id,
		},
		Agreement: agreement,
	}
}

type BlockchainRegMessage struct {
	event    events.Event
	Contract persistence.PendingContract
}

func (m BlockchainRegMessage) String() string {
	return fmt.Sprintf("event: %v, Contract: %v", m.event, m.Contract)
}

func (b *BlockchainRegMessage) Event() events.Event {
	return b.event
}

func NewBlockchainRegMessage(id events.EventId, contract persistence.PendingContract) *BlockchainRegMessage {

	return &BlockchainRegMessage{
		event: events.Event{
			Id: id,
		},
		Contract: contract,
	}
}

type BlockchainEndContractCommand struct {
	Cause           events.EndContractCause
	ContractAddress string
	AgreementId     string
}

func (w *BlockchainWorker) NewBlockchainEndContractCommand(cause events.EndContractCause, contractId string, agreementId string) *BlockchainEndContractCommand {
	return &BlockchainEndContractCommand{
		Cause:           cause,
		ContractAddress: contractId,
		AgreementId:     agreementId,
	}
}

type BlockchainRecordWhisperAccountIdCommand struct {
	AccountId string
}

func (w *BlockchainWorker) NewBlockchainRecordWhisperAccountIdCommand(accountId string) *BlockchainRecordWhisperAccountIdCommand {
	return &BlockchainRecordWhisperAccountIdCommand{
		AccountId: accountId,
	}
}
