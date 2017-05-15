package agreement

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/citizenscientist"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/device"
	"github.com/open-horizon/anax/ethblockchain"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/worker"
	"net/http"
	"reflect"
	"runtime"
	"strconv"
	"time"
)

// must be safely-constructed!!
type AgreementWorker struct {
	worker.Worker            // embedded field
	db                       *bolt.DB
	httpClient               *http.Client
	userId                   string
	deviceId                 string
	deviceToken              string
	protocols                map[string]bool
	pm                       *policy.PolicyManager
	bcClientInitialized      bool
	containerSyncUpEvent     bool
	containerSyncUpSucessful bool
}

func NewAgreementWorker(config *config.HorizonConfig, db *bolt.DB, pm *policy.PolicyManager) *AgreementWorker {
	messages := make(chan events.Message)
	commands := make(chan worker.Command, 100)

	id, _ := device.Id()

	token := ""
	if dev, _ := persistence.FindExchangeDevice(db); dev != nil {
		token = dev.Token
	}

	worker := &AgreementWorker{
		Worker: worker.Worker{
			Manager: worker.Manager{
				Config:   config,
				Messages: messages,
			},

			Commands: commands,
		},

		db:                  db,
		httpClient:          &http.Client{},
		protocols:           make(map[string]bool),
		pm:                  pm,
		bcClientInitialized: false,
		deviceId:            id,
		deviceToken:         token,
	}

	glog.Info("Starting Agreement worker")
	worker.start()
	return worker
}

func (w *AgreementWorker) Messages() chan events.Message {
	return w.Worker.Manager.Messages
}

func (w *AgreementWorker) NewEvent(incoming events.Message) {

	switch incoming.(type) {
	case *events.EdgeRegisteredExchangeMessage:
		msg, _ := incoming.(*events.EdgeRegisteredExchangeMessage)
		w.Commands <- NewDeviceRegisteredCommand(msg.Token())

	case *events.PolicyCreatedMessage:
		msg, _ := incoming.(*events.PolicyCreatedMessage)

		switch msg.Event().Id {
		case events.NEW_POLICY:
			w.Commands <- NewAdvertisePolicyCommand(msg.PolicyFile())
		default:
			glog.Errorf("AgreementWorker received Unsupported event: %v", incoming.Event().Id)
		}

	case *events.BlockchainClientInitializedMessage:
		msg, _ := incoming.(*events.BlockchainClientInitializedMessage)
		switch msg.Event().Id {
		case events.BC_CLIENT_INITIALIZED:
			w.bcClientInitialized = true
		}

	case *events.ExchangeDeviceMessage:
		msg, _ := incoming.(*events.ExchangeDeviceMessage)
		switch msg.Event().Id {
		case events.RECEIVED_EXCHANGE_DEV_MSG:
			w.Commands <- NewExchangeMessageCommand(*msg)
		}

	case *events.DeviceContainersSyncedMessage:
		msg, _ := incoming.(*events.DeviceContainersSyncedMessage)
		switch msg.Event().Id {
		case events.DEVICE_CONTAINERS_SYNCED:
			w.containerSyncUpSucessful = msg.IsCompleted()
			w.containerSyncUpEvent = true
		}

	default: //nothing
	}

	return
}

func (w *AgreementWorker) start() {

	sendMessage := func(mt interface{}, pay []byte) error {
		// The mt parameter is an abstract message target object that is passed to this routine
		// by the agreement protocol. It's an interface{} type so that we can avoid the protocol knowing
		// about non protocol types.

		var messageTarget *exchange.ExchangeMessageTarget
		switch mt.(type) {
		case *exchange.ExchangeMessageTarget:
			messageTarget = mt.(*exchange.ExchangeMessageTarget)
		default:
			return errors.New(fmt.Sprintf("input message target is %T, expecting exchange.MessageTarget", mt))
		}

		// If the message target is using whisper, then send via whisper
		if len(messageTarget.ReceiverMsgEndPoint) != 0 {
			return errors.New(fmt.Sprintf("Message target should never be whisper, %v", messageTarget))

			// The message target is using the exchange message queue, so use it
		} else {

			// Grab the exchange ID of the message receiver
			glog.V(3).Infof("Sending exchange message to: %v, message %v", messageTarget.ReceiverExchangeId, string(pay))

			// Get my own keys
			myPubKey, myPrivKey, _ := exchange.GetKeys("")

			// Demarshal the receiver's public key if we need to
			if messageTarget.ReceiverPublicKeyObj == nil {
				if mtpk, err := exchange.DemarshalPublicKey(messageTarget.ReceiverPublicKeyBytes); err != nil {
					return errors.New(fmt.Sprintf("Unable to demarshal device's public key %x, error %v", messageTarget.ReceiverPublicKeyBytes, err))
				} else {
					messageTarget.ReceiverPublicKeyObj = mtpk
				}
			}

			// Create an encrypted message
			if encryptedMsg, err := exchange.ConstructExchangeMessage(pay, myPubKey, myPrivKey, messageTarget.ReceiverPublicKeyObj); err != nil {
				return errors.New(fmt.Sprintf("Unable to construct encrypted message from %v, error %v", pay, err))
				// Marshal it into a byte array
			} else if msgBody, err := json.Marshal(encryptedMsg); err != nil {
				return errors.New(fmt.Sprintf("Unable to marshal exchange message %v, error %v", encryptedMsg, err))
				// Send it to the device's message queue
			} else {
				pm := exchange.CreatePostMessage(msgBody, w.Worker.Manager.Config.Edge.ExchangeMessageTTL)
				var resp interface{}
				resp = new(exchange.PostDeviceResponse)
				targetURL := w.Worker.Manager.Config.Edge.ExchangeURL + "agbots/" + messageTarget.ReceiverExchangeId + "/msgs"
				for {
					if err, tpErr := exchange.InvokeExchange(w.httpClient, "POST", targetURL, w.deviceId, w.deviceToken, pm, &resp); err != nil {
						return err
					} else if tpErr != nil {
						glog.V(5).Infof(tpErr.Error())
						time.Sleep(10 * time.Second)
						continue
					} else {
						glog.V(5).Infof("Sent message for %v to exchange.", messageTarget.ReceiverExchangeId)
						return nil
					}
				}
			}
		}
		return nil
	}

	glog.Info(logString(fmt.Sprintf("started")))

	// If there is no edge config then there is nothing to init on the edge
	if w.Worker.Manager.Config.Edge == (config.Config{}) {
		return
	}

	// Enter the command processing loop. Initialization is complete so wait for commands to
	// perform. Commands are created as the result of events that are triggered elsewhere
	// in the system.
	go func() {

		// Wait for blockchain client to fully initialize before syncing and advertising the policies
		for {
			if w.bcClientInitialized == false {
				time.Sleep(time.Duration(5) * time.Second)
				glog.V(3).Infof("AgreementWorker waiting for blockchain client to be fully initialized.")
			} else {
				if w.Config.Edge.RegistrationDelayS != 0 {
					glog.V(3).Infof("AgreementWorker blocking for registration delay, %v seconds.", w.Config.Edge.RegistrationDelayS)
					time.Sleep(time.Duration(w.Config.Edge.RegistrationDelayS) * time.Second)
				}
				break
			}
		}

		// Block for the container syncup message, to make sure the docker state matches our local DB.
		for {
			if w.containerSyncUpEvent == false {
				time.Sleep(time.Duration(5) * time.Second)
				glog.V(3).Infof("AgreementWorker waiting for container syncup to be done.")
			} else if w.containerSyncUpSucessful {
				break
			} else {
				panic(logString(fmt.Sprintf("Terminating, unable to sync up containers")))
			}
		}

		if w.deviceToken != "" {
			// Sync up between what's in our database versus what's in the exchange, and make sure that the policy manager's
			// agreement counts are correct. This function will cancel any agreements whose state might have changed
			// while the device was down. We will also check to make sure that policies havent changed. If they have, then
			// we will cancel agreements and allow them to re-negotiate.
			if err := w.syncOnInit(); err != nil {
				glog.Errorf(logString(fmt.Sprintf("Terminating, unable to sync up, error: %v", err)))
				panic(logString(fmt.Sprintf("Terminating, unable to complete agreement sync up, error: %v", err)))
			} else {
				w.Messages() <- events.NewDeviceAgreementsSyncedMessage(events.DEVICE_AGREEMENTS_SYNCED, true)
			}

			// If the device is registered, start heartbeating. If the device isn't registered yet, then we will
			// start heartbeating when the registration event comes in.
			targetURL := w.Manager.Config.Edge.ExchangeURL + "devices/" + w.deviceId + "/heartbeat"
			go exchange.Heartbeat(w.httpClient, targetURL, w.deviceId, w.deviceToken, w.Worker.Manager.Config.Edge.ExchangeHeartbeat)

		}

		// Publish what we have for the world to see
		if err := w.advertiseAllPolicies(w.Worker.Manager.Config.Edge.PolicyPath); err != nil {
			glog.Errorf(logString(fmt.Sprintf("unable to advertise policies with exchange, error: %v", err)))
		}

		protocolHandler := citizenscientist.NewProtocolHandler(w.Config.Edge.GethURL, w.pm)
		// Handle agreement processor commands
		for {
			glog.V(2).Infof(logString(fmt.Sprintf("blocking for commands")))
			command := <-w.Commands
			glog.V(2).Infof(logString(fmt.Sprintf("received command: %v", command.ShortString())))
			glog.V(5).Infof(logString(fmt.Sprintf("received command: %v", command)))

			switch command.(type) {
			case *DeviceRegisteredCommand:
				cmd, _ := command.(*DeviceRegisteredCommand)
				w.handleDeviceRegistered(cmd)

			case *TerminateCommand:
				cmd, _ := command.(*TerminateCommand)
				glog.Errorf(logString(fmt.Sprintf("terminating, reason: %v", cmd.reason)))
				return

			case *AdvertisePolicyCommand:
				cmd, _ := command.(*AdvertisePolicyCommand)

				if newPolicy, err := policy.ReadPolicyFile(cmd.PolicyFile); err != nil {
					glog.Errorf(logString(fmt.Sprintf("unable to read policy file %v into memory, error: %v", cmd.PolicyFile, err)))
				} else if err := w.pm.AddPolicy(newPolicy); err != nil {
					glog.Errorf(logString(fmt.Sprintf("policy name is a duplicate, not added, error: %v", err)))
				} else {

					// Publish what we have for the world to see
					if err := w.advertiseAllPolicies(w.Worker.Manager.Config.Edge.PolicyPath); err != nil {
						glog.Errorf(logString(fmt.Sprintf("unable to advertise policies with exchange, error: %v", err)))
					}
				}

			case *ExchangeMessageCommand:
				cmd, _ := command.(*ExchangeMessageCommand)
				exchangeMsg := new(exchange.DeviceMessage)
				if err := json.Unmarshal(cmd.Msg.ExchangeMessage(), &exchangeMsg); err != nil {
					glog.Errorf(logString(fmt.Sprintf("unable to demarshal exchange device message %v, error %v", cmd.Msg.ExchangeMessage(), err)))
				}
				protocolMsg := cmd.Msg.ProtocolMessage()

				glog.V(3).Infof(logString(fmt.Sprintf("received message %v from the exchange", exchangeMsg.MsgId)))

				// Process the message if it's a proposal.
				deleteMessage := false
				if proposal, err := protocolHandler.ValidateProposal(string(protocolMsg)); err != nil {
					glog.Warningf(logString(fmt.Sprintf("Proposal handler ignoring non-proposal message: %s due to %v", cmd.Msg.ShortProtocolMessage(), err)))
				} else if agAlreadyExists, err := persistence.FindEstablishedAgreements(w.db, citizenscientist.PROTOCOL_NAME, []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(proposal.AgreementId)}); err != nil {
					glog.Errorf(logString(fmt.Sprintf("unable to retrieve agreements from database, error %v", err)))
				} else if len(agAlreadyExists) != 0 {
					glog.Errorf(logString(fmt.Sprintf("agreement %v already exists, ignoring proposal: %v", proposal.AgreementId, proposal.ShortString())))
					deleteMessage = true
				} else if tcPolicy, err := policy.DemarshalPolicy(proposal.TsAndCs); err != nil {
					glog.Errorf(logString(fmt.Sprintf("received error demarshalling TsAndCs, %v", err)))
				} else if err := tcPolicy.Is_Self_Consistent(w.Config.Edge.PublicKeyPath, config.USERKEYDIR); err != nil {
					glog.Errorf(logString(fmt.Sprintf("received error check self consistency of TsAndCs, %v", err)))
				} else if messageTarget, err := exchange.CreateMessageTarget(exchangeMsg.AgbotId, nil, exchangeMsg.AgbotPubKey, ""); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error creating message target: %v", err)))
				} else {
					deleteMessage = true
					if reply, err := protocolHandler.DecideOnProposal(proposal, w.deviceId, messageTarget, sendMessage); err != nil {
						glog.Errorf(logString(fmt.Sprintf("respond to proposal with error: %v", err)))
					} else if _, err := persistence.NewEstablishedAgreement(w.db, tcPolicy.Header.Name, proposal.AgreementId, proposal.ConsumerId, protocolMsg, citizenscientist.PROTOCOL_NAME, proposal.Version, tcPolicy.APISpecs[0].SpecRef, reply.Signature, proposal.Address); err != nil {
						glog.Errorf(logString(fmt.Sprintf("error persisting new agreement: %v, error: %v", proposal.AgreementId, err)))
						deleteMessage = false
					}
				}

				if deleteMessage {
					if err := w.deleteMessage(exchangeMsg); err != nil {
						glog.Errorf(logString(fmt.Sprintf("error deleting exchange message %v, error %v", exchangeMsg.MsgId, err)))
					}
				}

			default:
				glog.Errorf("Unknown command (%T): %v", command, command)
			}

			glog.V(5).Infof(logString(fmt.Sprintf("handled command")))
			runtime.Gosched()
		}

	}()

	glog.Info(logString(fmt.Sprintf("waiting for commands.")))

}

func (w *AgreementWorker) handleDeviceRegistered(cmd *DeviceRegisteredCommand) {

	w.deviceToken = cmd.Token

	// Start the go thread that heartbeats to the exchange
	targetURL := w.Manager.Config.Edge.ExchangeURL + "devices/" + w.deviceId + "/heartbeat"
	go exchange.Heartbeat(w.httpClient, targetURL, w.deviceId, w.deviceToken, w.Worker.Manager.Config.Edge.ExchangeHeartbeat)

}

// This function is only called when anax device side initializes. The agbot has it's own initialization checking.
// This function is responsible for reconciling the agreements in our local DB with the agreements recorded in the exchange
// and the blockchain, as well as looking for agreements that need to change based on changes to policy files. This function
// handles agreements that exist in the exchange for which we have no DB records, it handles DB records for which
// the state in the exchange is missing, and it handles agreements whose state has changed in the blockchain.
func (w *AgreementWorker) syncOnInit() error {

	glog.V(3).Infof(logString("beginning sync up."))

	protocolHandler := citizenscientist.NewProtocolHandler(w.Config.Edge.GethURL, w.pm)

	// Establish the go objects that are used to interact with the ethereum blockchain.
	// This code should probably be in the protocol library.
	acct, _ := ethblockchain.AccountId()
	dir, _ := ethblockchain.DirectoryAddress()
	bc, err := ethblockchain.InitBaseContracts(acct, w.Config.Edge.GethURL, dir)
	if err != nil {
		return errors.New(logString(fmt.Sprintf("unable to initialize platform contracts, error: %v", err)))
	}

	// Reconcile the set of agreements recorded in the exchange for this device with the agreements in the local DB.
	// First get all the agreements for this device from the exchange.

	exchangeDeviceAgreements, err := w.getAllAgreements()
	if err != nil {
		return errors.New(logString(fmt.Sprintf("encountered error getting device agreement list from exchange, error %v", err)))
	} else {

		// Loop through each agreement in the exchange and search for that agreement in our DB. If it should not
		// be in the exchange, then we have to delete it from the exchange because its presence in the exchange
		// prevents an agbot from making an agreement with our device. It is posible to have a DB record for
		// an agreement that is not yet recorded in the exchange (this case is handled later in this function),
		// but the reverse should not occur normally. Agreements in the exchange must have a record on our local DB.
		for exchangeAg, _ := range exchangeDeviceAgreements {
			if agreements, err := persistence.FindEstablishedAgreements(w.db, citizenscientist.PROTOCOL_NAME, []persistence.EAFilter{persistence.IdEAFilter(exchangeAg), persistence.UnarchivedEAFilter()}); err != nil {
				glog.Errorf(logString(fmt.Sprintf("error searching for agreement %v from exchange agreements", exchangeAg, err)))
			} else if len(agreements) == 0 {
				glog.V(3).Infof(logString(fmt.Sprintf("found agreement %v in the exchange that is not in our DB.", exchangeAg)))
				// Delete the agreement from the exchange.
				if err := deleteProducerAgreement(w.Config.Edge.ExchangeURL, w.deviceId, w.deviceToken, exchangeAg); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error deleting agreement %v in exchange: %v", exchangeAg, err)))
				}
			}
		}

	}

	// Now perform the reverse set of checks, looping through our database and checking each record for accuracy with the exchange
	// and the blockchain.
	if agreements, err := persistence.FindEstablishedAgreements(w.db, citizenscientist.PROTOCOL_NAME, []persistence.EAFilter{persistence.UnarchivedEAFilter()}); err == nil {

		// If there are agreemens in the database then we will assume that the device is already registered
		for _, ag := range agreements {

			// If there is an active agreement that is marked as terminated, then anax was restarted in the middle of
			// termination processing, and therefore we dont know how far it got. Initiate a cancel again to clean it up.
			if ag.AgreementTerminatedTime != 0 {
				reason := uint(ag.TerminatedReason)
				if _, err := persistence.AgreementStateForceTerminated(w.db, ag.CurrentAgreementId, citizenscientist.PROTOCOL_NAME); err != nil {
					glog.Errorf(logString(fmt.Sprintf("unable to set force termination for agreement %v, error %v", ag.CurrentAgreementId, err)))
				}
				w.Messages() <- events.NewInitAgreementCancelationMessage(events.AGREEMENT_ENDED, reason, citizenscientist.PROTOCOL_NAME, ag.CurrentAgreementId, ag.CurrentDeployment)

			// Make sure the blockchain state agrees with our local DB state for this agreement. If not, then we might need to cancel the agreement.
			// Anax could have been down for a long time (or inoperable), and the blockchain event logger worker doesnt go back in time to
			// find events that we might have missed, so we need to query the BC for this info.
			} else if ok, err := w.checkAgreementInBlockchain(&ag, protocolHandler, bc); err != nil {
				return errors.New(logString(fmt.Sprintf("unable to check for agreement %v in blockchain, error %v", ag.CurrentAgreementId, err)))
			} else if !ok {
				w.Messages() <- events.NewInitAgreementCancelationMessage(events.AGREEMENT_ENDED, citizenscientist.CANCEL_AGBOT_REQUESTED, citizenscientist.PROTOCOL_NAME, ag.CurrentAgreementId, ag.CurrentDeployment)

			// If the agreement has been started then we just need to make sure that the policy manager's agreement counts
			// are correct. Even for already timedout agreements, the governance process will cleanup old and outdated agreements,
			// so we don't need to do anything here.

			} else if proposal, err := protocolHandler.DemarshalProposal(ag.Proposal); err != nil {
				glog.Errorf(logString(fmt.Sprintf("unable to demarshal proposal for agreement %v, error %v", ag.CurrentAgreementId, err)))
			} else if pol, err := policy.DemarshalPolicy(proposal.ProducerPolicy); err != nil {
				glog.Errorf(logString(fmt.Sprintf("unable to demarshal policy for agreement %v, error %v", ag.CurrentAgreementId, err)))
			} else if existingPol := w.pm.GetPolicy(pol.Header.Name); existingPol == nil {
				glog.Errorf(logString(fmt.Sprintf("agreement %v has a policy %v that doesn't exist anymore", ag.CurrentAgreementId, pol.Header.Name)))
				w.Messages() <- events.NewInitAgreementCancelationMessage(events.AGREEMENT_ENDED, citizenscientist.CANCEL_POLICY_CHANGED, citizenscientist.PROTOCOL_NAME, ag.CurrentAgreementId, ag.CurrentDeployment)

			} else if err := w.pm.MatchesMine(pol); err != nil {
				glog.Warningf(logString(fmt.Sprintf("agreement %v has a policy %v that has changed.", ag.CurrentAgreementId, pol.Header.Name)))
				w.Messages() <- events.NewInitAgreementCancelationMessage(events.AGREEMENT_ENDED, citizenscientist.CANCEL_POLICY_CHANGED, citizenscientist.PROTOCOL_NAME, ag.CurrentAgreementId, ag.CurrentDeployment)

			} else if err := w.pm.AttemptingAgreement(existingPol, ag.CurrentAgreementId); err != nil {
				glog.Errorf(logString(fmt.Sprintf("cannot update agreement count for %v, error: %v", ag.CurrentAgreementId, err)))
			} else if err := w.pm.FinalAgreement(existingPol, ag.CurrentAgreementId); err != nil {
				glog.Errorf(logString(fmt.Sprintf("cannot update agreement count for %v, error: %v", ag.CurrentAgreementId, err)))

				// There is a small window where an agreement might not have been recorded in the exchange. Let's just make sure.
			} else if ag.AgreementAcceptedTime != 0 && ag.AgreementTerminatedTime == 0 {

				if _, there := exchangeDeviceAgreements[ag.CurrentAgreementId]; !there {
					glog.V(3).Infof(logString(fmt.Sprintf("agreement %v missing from exchange, adding it back in.", ag.CurrentAgreementId)))
					state := ""
					if ag.AgreementFinalizedTime != 0 {
						state = "Finalized Agreement"
					} else if ag.AgreementAcceptedTime != 0 {
						state = "Agree to proposal"
					} else {
						state = "unknown"
					}
					w.recordAgreementState(ag.CurrentAgreementId, pol.APISpecs[0].SpecRef, state)
				}
				glog.V(3).Infof(logString(fmt.Sprintf("added agreement %v to policy agreement counter.", ag.CurrentAgreementId)))
			}

		}
	} else {
		return errors.New(logString(fmt.Sprintf("error searching database: %v", err)))
	}

	glog.V(3).Infof(logString("sync up completed normally."))
	return nil

}

// This function verifies that an agreement is present in the blockchain. An agreement might not be present for a variety of reasons,
// some of which are legitimate. The purpose of this routine is to figure out whether or not an agreement cancellation
// has occurred. It returns false if the agreement needs to be cancelled, or if there was an error.
func (w *AgreementWorker) checkAgreementInBlockchain(ag *persistence.EstablishedAgreement, protocolHandler *citizenscientist.ProtocolHandler, bc *ethblockchain.BaseContracts) (bool, error) {

	// Agreements that havent been accepted yet by the device will not be in the blockchain and it's ok that they are not in
	// the blockchain, so return true.
	if ag.AgreementAcceptedTime == 0 {
		return true, nil
	}

	// Check to see if the agreement is in the blockchain. This call to the blockchain should be very fast.
	if recorded, err := protocolHandler.VerifyAgreementRecorded(ag.CurrentAgreementId, ag.CounterPartyAddress, ag.ProposalSig, bc.Agreements); err != nil {
		return false, errors.New(logString(fmt.Sprintf("encountered error verifying agreement %v on blockchain, error %v", ag.CurrentAgreementId, err)))
	} else if !recorded {
		// A finalized agreement should be on the blockchain
		if ag.AgreementFinalizedTime != 0 && ag.AgreementTerminatedTime == 0 {
			glog.V(3).Infof(logString(fmt.Sprintf("agreement %v is not in the blockchain, cancelling.", ag.CurrentAgreementId)))
			return false, nil
		}
	}
	return true, nil

}

func (w *AgreementWorker) getAllAgreements() (map[string]exchange.DeviceAgreement, error) {

	var exchangeDeviceAgreements map[string]exchange.DeviceAgreement
	var resp interface{}
	resp = new(exchange.AllDeviceAgreementsResponse)

	targetURL := w.Worker.Manager.Config.Edge.ExchangeURL + "devices/" + w.deviceId + "/agreements"
	for {
		if err, tpErr := exchange.InvokeExchange(w.httpClient, "GET", targetURL, w.deviceId, w.deviceToken, nil, &resp); err != nil {
			glog.Errorf(err.Error())
			return exchangeDeviceAgreements, err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			exchangeDeviceAgreements = resp.(*exchange.AllDeviceAgreementsResponse).Agreements
			glog.V(5).Infof(logString(fmt.Sprintf("found agreements %v in the exchange.", exchangeDeviceAgreements)))
			return exchangeDeviceAgreements, nil
		}
	}

}

// ===============================================================================================
// Utility functions
//

func (w *AgreementWorker) advertiseAllPolicies(location string) error {

	var pType, pValue, pCompare string
	var deviceName string

	if dev, err := persistence.FindExchangeDevice(w.db); err != nil {
		return errors.New(fmt.Sprintf("AgreementWorker received error getting device name: %v", err))
	} else if dev == nil {
		return errors.New("AgreementWorker could not get device name because no device was registered yet.")
	} else {
		deviceName = dev.Name
	}

	policies := w.pm.GetAllPolicies()

	if len(policies) > 0 {
		ms := make([]exchange.Microservice, 0, 10)
		for _, p := range policies {
			newMS := new(exchange.Microservice)
			newMS.Url = p.APISpecs[0].SpecRef
			newMS.NumAgreements = p.MaxAgreements

			p.DataVerify.Obscure()
			if pBytes, err := json.Marshal(p); err != nil {
				return errors.New(fmt.Sprintf("AgreementWorker received error marshalling policy: %v", err))
			} else {
				newMS.Policy = string(pBytes)
			}

			if props, err := policy.RetrieveAllProperties(&p); err != nil {
				return errors.New(fmt.Sprintf("AgreementWorker received error calculating properties: %v", err))
			} else {
				for _, prop := range *props {
					switch prop.Value.(type) {
					case string:
						pType = "string"
						pValue = prop.Value.(string)
						pCompare = "in"
					case int:
						pType = "int"
						pValue = strconv.Itoa(prop.Value.(int))
						pCompare = ">="
					case bool:
						pType = "boolean"
						pValue = strconv.FormatBool(prop.Value.(bool))
						pCompare = "="
					case []string:
						pType = "list"
						pValue = exchange.ConvertToString(prop.Value.([]string))
						pCompare = "in"
					default:
						return errors.New(fmt.Sprintf("AgreementWorker encountered unsupported property type: %v", reflect.TypeOf(prop.Value).String()))
					}
					// Now put the property together
					newProp := &exchange.MSProp{
						Name:     prop.Name,
						Value:    pValue,
						PropType: pType,
						Op:       pCompare,
					}
					newMS.Properties = append(newMS.Properties, *newProp)
				}
			}

			// Make sure whisper is listening for message in all agreement protocols
			for _, agp := range p.AgreementProtocols {
				w.Messages() <- events.NewWhisperSubscribeToMessage(events.SUBSCRIBE_TO, agp.Name)
			}

			ms = append(ms, *newMS)

		}

		pdr := exchange.CreateDevicePut(w.Config.Edge.GethURL, w.deviceToken, deviceName)
		pdr.RegisteredMicroservices = ms
		var resp interface{}
		resp = new(exchange.PutDeviceResponse)
		targetURL := w.Manager.Config.Edge.ExchangeURL + "devices/" + w.deviceId

		glog.V(3).Infof("AgreementWorker Registering microservices: %v at %v", pdr.ShortString(), targetURL)

		for {
			if err, tpErr := exchange.InvokeExchange(w.httpClient, "PUT", targetURL, w.deviceId, w.deviceToken, pdr, &resp); err != nil {
				return err
			} else if tpErr != nil {
				glog.V(5).Infof(tpErr.Error())
				time.Sleep(10 * time.Second)
				continue
			} else {
				glog.V(3).Infof(logString(fmt.Sprintf("advertised policies for device %v in exchange: %v", w.deviceId, resp)))
				return nil
			}
		}
	}

	return nil
}

func (w *AgreementWorker) recordAgreementState(agreementId string, microservice string, state string) error {

	glog.V(5).Infof(logString(fmt.Sprintf("setting agreement %v state to %v", agreementId, state)))

	as := new(exchange.PutAgreementState)
	as.Microservice = microservice
	as.State = state
	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := w.Manager.Config.Edge.ExchangeURL + "devices/" + w.deviceId + "/agreements/" + agreementId
	for {
		if err, tpErr := exchange.InvokeExchange(w.httpClient, "PUT", targetURL, w.deviceId, w.deviceToken, as, &resp); err != nil {
			glog.Errorf(err.Error())
			return err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(5).Infof(logString(fmt.Sprintf("set agreement %v to state %v", agreementId, state)))
			return nil
		}
	}

}

func deleteProducerAgreement(url string, deviceId string, token string, agreementId string) error {

	glog.V(5).Infof(logString(fmt.Sprintf("deleting agreement %v in exchange", agreementId)))

	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := url + "devices/" + deviceId + "/agreements/" + agreementId
	for {
		if err, tpErr := exchange.InvokeExchange(&http.Client{}, "DELETE", targetURL, deviceId, token, nil, &resp); err != nil {
			glog.Errorf(logString(fmt.Sprintf(err.Error())))
			return err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(5).Infof(logString(fmt.Sprintf("deleted agreement %v from exchange", agreementId)))
			return nil
		}
	}

}

func (w *AgreementWorker) deleteMessage(msg *exchange.DeviceMessage) error {
	var resp interface{}
	resp = new(exchange.PostDeviceResponse)
	targetURL := w.Manager.Config.Edge.ExchangeURL + "devices/" + w.deviceId + "/msgs/" + strconv.Itoa(msg.MsgId)
	for {
		if err, tpErr := exchange.InvokeExchange(w.httpClient, "DELETE", targetURL, w.deviceId, w.deviceToken, nil, &resp); err != nil {
			glog.Errorf(err.Error())
			return err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(3).Infof(logString(fmt.Sprintf("deleted message %v", msg.MsgId)))
			return nil
		}
	}
}

var logString = func(v interface{}) string {
	return fmt.Sprintf("AgreementWorker %v", v)
}
