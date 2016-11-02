package persistence

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	docker "github.com/fsouza/go-dockerclient"
	"sort"
	"strconv"
	"time"

	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/cutil"
	"strings"
)

const P_CONTRACTS = "pending_contracts"
const E_CONTRACTS = "established_contracts" // may or may not be in agreements
const MICROPAYMENTS = "micropayments"

// uses pointers for members b/c it allows nil-checking at deserialization; !Important!: the json field names here must not change w/out changing the error messages returned from the API, they are not programmatically determined
type PendingContract struct {
	Name                 *string            `json:"name"`
	Arch                 string             `json:"arch"`
	CPUs                 int                `json:"cpus"`
	RAM                  *int               `json:"ram"`
	HourlyCostBacon      *int64             `json:"hourly_cost_bacon"`
	IsLocEnabled         bool               `json:"is_loc_enabled"`
	AppAttributes        *map[string]string `json:"app_attributes"`
	PrivateAppAttributes *map[string]string `json:"private_app_attributes"`
}

func (c *PendingContract) String() string {
	return fmt.Sprintf("Name: %v, Arch: %v, CPUs: %v, RAM: %v, HourlyCostBacon: %v, IsLocEnabled: %v, AppAttributes: %v, PrivateAppAttributes: %v", *c.Name, c.Arch, c.CPUs, *c.RAM, *c.HourlyCostBacon, c.IsLocEnabled, *c.AppAttributes, *c.PrivateAppAttributes)
}

type LatestMicropayment struct {
	AgreementId  string `json:"agreement_id"`
	PaymentTime  int64  `json:"payment_time"`
	PaymentValue uint64 `json:"payment_value"`
}

// N.B. Important!! Ensure new values are handled in Update function below
type EstablishedContract struct {
	ContractAddress             string                   `json:"contract_address"` // also the key in the db for this struct
	Name                        string                   `json:"name"`
	Archived                    bool                     `json:"archived"` // TODO: give risham, booz a way to indicate that a contract needs to be archived; REST api
	CurrentAgreementId          string                   `json:"current_agreement_id"`
	PreviousAgreements          []string                 `json:"previous_agreements"`
	ConfigureNonce              string                   `json:"configure_nonce"` // one-time use configure token
	AgreementAcceptedTime       uint64                   `json:"agreement_accepted_time"`
	PrivateEnvironmentAdditions map[string]string        `json:"private_environment_additions"` // platform-provided environment facts, none of which can leave the device
	EnvironmentAdditions        map[string]string        `json:"environment_additions"`         // platform-provided environment facts, some of which are published on the blockchain for marketplace searching
	AgreementCreationTime       uint64                   `json:"agreement_creation_time"`
	AgreementExecutionStartTime uint64                   `json:"agreement_execution_start_time"`
	CurrentDeployment           map[string]ServiceConfig `json:"current_deployment"`
}

func (c EstablishedContract) String() string {
	return fmt.Sprintf("ContractAddress: %v , Name: %v , Archived: %v , CurrentAgreementId: %v , ConfigureNonce: %v, CurrentDeployment (service names): %v, PrivateEnvironmentAdditions: %v, EnvironmentAdditions: %v, AgreementCreationTime: %v, AgreementExecutionStartTime: %v, AgreementAcceptedTime: %v, PreviousAgreements (sample): %v", c.ContractAddress, c.Name, c.Archived, c.CurrentAgreementId, c.ConfigureNonce, ServiceConfigNames(&c.CurrentDeployment), c.PrivateEnvironmentAdditions, c.EnvironmentAdditions, c.AgreementCreationTime, c.AgreementExecutionStartTime, c.AgreementAcceptedTime, cutil.FirstN(10, c.PreviousAgreements))
}

// the internal representation of this lib; *this is the one persisted using the persistence lib*
type ServiceConfig struct {
	Config     docker.Config     `json:"config"`
	HostConfig docker.HostConfig `json:"host_config"`
}

func ServiceConfigNames(serviceConfigs *map[string]ServiceConfig) []string {
	names := []string{}

	if serviceConfigs != nil {
		for name, _ := range *serviceConfigs {
			names = append(names, name)
		}
	}

	return names
}

func (c ServiceConfig) String() string {
	return fmt.Sprintf("Config: %v, HostConfig: %v", c.Config, c.HostConfig)
}

func NewEstablishedContract(pending *PendingContract, contractAddress string) (*EstablishedContract, error) {

	if pending.Name == nil || *pending.Name == "" {
		return nil, errors.New("Contract name unset or empty, cannot persist")
	} else if contractAddress == "" {
		return nil, errors.New("Contract address empty, cannot persist")
	} else {
		// construct map for sensitive environment stuff from pending contract
		privateEnvironmentAdditions := make(map[string]string, 0)

		// deep copy of all private environment additions
		for key, val := range *pending.PrivateAppAttributes {
			privateEnvironmentAdditions[key] = val
		}

		return &EstablishedContract{
			Name:                        *pending.Name,
			Archived:                    false,
			ContractAddress:             contractAddress,
			CurrentAgreementId:          "",
			PreviousAgreements:          []string{},
			ConfigureNonce:              "", // needs to be set before this can be used
			AgreementAcceptedTime:       0,
			PrivateEnvironmentAdditions: privateEnvironmentAdditions,
			EnvironmentAdditions:        map[string]string{},
			AgreementCreationTime:       0,
			AgreementExecutionStartTime: 0,
			CurrentDeployment:           map[string]ServiceConfig{},
		}, nil
	}
}

func DeleteEstablishedContracts(db *bolt.DB, name string) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(E_CONTRACTS))
		if b != nil {
			return b.ForEach(func(k, v []byte) error {
				var contract EstablishedContract

				if err := json.Unmarshal(v, &contract); err != nil {
					return err
				}

				if contract.Name == name {
					if err := b.Delete([]byte(contract.ContractAddress)); err != nil {
						return err
					}
				}
				return nil
			})
		} else {
			glog.Infof("DB bucket for established contracts not yet created, no need to delete contract with name: %v", name)
			return nil
		}
	})
}

// not intended to be instantiated outside of this module
type Micropayments struct {
	PayerAddress    string            `json:"payer_address"`
	ContractAddress string            `json:"contract_address"`
	AgreementId     string            `json:"agreement_id"` // PK
	Payments        map[string]uint64 `json:"payments"`     // timestamp (s): amount; N.B. the key must be a string so this is really an int timestamp converted upon serialization and deserialization
}

func (m Micropayments) String() string {
	return fmt.Sprintf("PayerAddress: %v, ContractAddress: %v, AgreementId: %v, Payments (recordedTimestamp: amountToDate): %v", m.PayerAddress, m.ContractAddress, m.AgreementId, m.Payments)
}

// TODO: share common initialization here with above factory method
// reset contract record state to begin polling
func ContractStateNew(db *bolt.DB, dbContractAddress string) (*EstablishedContract, error) {
	return contractStateUpdate(db, dbContractAddress, func(c EstablishedContract) *EstablishedContract {
		c.CurrentAgreementId = ""
		c.AgreementAcceptedTime = 0
		c.ConfigureNonce = ""
		c.EnvironmentAdditions = map[string]string{}
		c.AgreementCreationTime = 0
		c.AgreementExecutionStartTime = 0
		c.CurrentDeployment = map[string]ServiceConfig{}
		return &c
	})
}

// set contract record state to in agreement, not yet accepted; N.B. It's expected that privateEnvironmentAdditions will already have been added by this time
func ContractStateInAgreement(db *bolt.DB, dbContractAddress string, containerProvider string, currentAgreementId string, environmentAdditions map[string]string) (*EstablishedContract, error) {

	if err := initializeMicropaymentsRecord(db, containerProvider, dbContractAddress, currentAgreementId); err != nil {
		return nil, err
	} else {
		var err error

		ret, updateErr := contractStateUpdate(db, dbContractAddress, func(c EstablishedContract) *EstablishedContract {
			c.CurrentAgreementId = currentAgreementId
			c.AgreementCreationTime = uint64(time.Now().Unix())
			c.EnvironmentAdditions = environmentAdditions

			var nonce string
			nonce, err = genNonce()
			c.ConfigureNonce = nonce

			// write confignonce into environment additions
			c.EnvironmentAdditions["MTN_CONFIGURE_NONCE"] = c.ConfigureNonce
			return &c
		})

		if err != nil {
			return nil, err
		} else if updateErr != nil {
			return nil, updateErr
		} else {
			return ret, nil
		}
	}
}

func ContractStateConfigured(db *bolt.DB, dbContractAddress string) (*EstablishedContract, error) {
	return contractStateUpdate(db, dbContractAddress, func(c EstablishedContract) *EstablishedContract {
		c.ConfigureNonce = ""
		return &c
	})
}

// set contract record state to execution start; this is before payment
func ContractStateExecutionStarted(db *bolt.DB, dbContractAddress string, deployment *map[string]ServiceConfig) (*EstablishedContract, error) {
	return contractStateUpdate(db, dbContractAddress, func(c EstablishedContract) *EstablishedContract {
		c.AgreementExecutionStartTime = uint64(time.Now().Unix())
		c.CurrentDeployment = *deployment
		return &c
	})
}

// set contract record state to already in agreement and accepted (this means the other party has reported receiving data
func ContractStateAccepted(db *bolt.DB, dbContractAddress string) (*EstablishedContract, error) {
	return contractStateUpdate(db, dbContractAddress, func(c EstablishedContract) *EstablishedContract {
		c.AgreementAcceptedTime = uint64(time.Now().Unix())
		return &c
	})
}

func contractStateUpdate(db *bolt.DB, dbContractAddress string, fn func(EstablishedContract) *EstablishedContract) (*EstablishedContract, error) {
	filters := make([]ECFilter, 0)
	filters = append(filters, UnarchivedECFilter())
	filters = append(filters, AddressECFilter(dbContractAddress))

	if contracts, err := FindEstablishedContracts(db, filters); err != nil {
		return nil, err
	} else if len(contracts) > 1 {
		return nil, fmt.Errorf("Expected only one record for dbContractAddress: %v, but retrieved: %v", dbContractAddress, contracts)
	} else if len(contracts) == 0 {
		return nil, fmt.Errorf("No record with id: %v", dbContractAddress)
	} else {
		// run this single contract through provided update function and persist it
		updated := fn(contracts[0])
		return updated, PersistUpdatedContract(db, dbContractAddress, updated)
	}
}

// does whole-member replacements of values that are legal to change during the course of a contract's life
func PersistUpdatedContract(db *bolt.DB, dbContractAddress string, update *EstablishedContract) error {
	return db.Update(func(tx *bolt.Tx) error {
		if b, err := tx.CreateBucketIfNotExists([]byte(E_CONTRACTS)); err != nil {
			return err
		} else {
			current := b.Get([]byte(dbContractAddress))
			var mod EstablishedContract

			if current == nil {
				return fmt.Errorf("No contract with give address available to update: %v", dbContractAddress)
			} else if err := json.Unmarshal(current, &mod); err != nil {
				return fmt.Errorf("Failed to unmarshal contract DB data: %v. Error: %v", string(current), err)
			} else {

				prevAgreementId := mod.CurrentAgreementId

				// write updates only to the fields we expect should be updateable
				mod.Archived = update.Archived
				mod.CurrentAgreementId = update.CurrentAgreementId
				mod.AgreementAcceptedTime = update.AgreementAcceptedTime
				mod.ConfigureNonce = update.ConfigureNonce
				mod.EnvironmentAdditions = update.EnvironmentAdditions
				mod.AgreementCreationTime = update.AgreementCreationTime
				mod.AgreementExecutionStartTime = update.AgreementExecutionStartTime
				mod.CurrentDeployment = update.CurrentDeployment

				// update PreviousAgreements array if CurrentAgreementId unset
				if prevAgreementId != "" && mod.CurrentAgreementId == "" {
					mod.PreviousAgreements = append([]string{prevAgreementId}, mod.PreviousAgreements...)
				}

				if serialized, err := json.Marshal(mod); err != nil {
					return fmt.Errorf("Failed to serialize contract record: %v. Error: %v", mod, err)
				} else if err := b.Put([]byte(dbContractAddress), serialized); err != nil {
					return fmt.Errorf("Failed to write contract record with key: %v. Error: %v", dbContractAddress, err)
				} else {
					glog.V(2).Infof("Succeeded updating contract record to %v", mod)
					return nil
				}
			}
		}
	})
}

func FindPendingContracts(db *bolt.DB) ([]PendingContract, error) {
	pendingContracts := make([]PendingContract, 0)

	// fetch pending contracts
	readErr := db.View(func(tx *bolt.Tx) error {

		// ok if pending contracts bucket doesn't exist yet; user interaction with the system creates it
		if b := tx.Bucket([]byte(P_CONTRACTS)); b != nil {
			b.ForEach(func(_, v []byte) error {
				// key unnecessary, it's just the user-friendly name we gave at creation time

				var p PendingContract

				if err := json.Unmarshal(v, &p); err != nil {
					glog.Errorf("Unable to deserialize db record: %v. Error: %v", v, err)
				} else {
					pendingContracts = append(pendingContracts, p)
				}

				return nil
			})
		}

		return nil // end the transaction
	})

	if readErr != nil {
		return nil, readErr
	} else {
		return pendingContracts, nil
	}
}

func UnarchivedECFilter() ECFilter {
	return func(e EstablishedContract) bool { return !e.Archived }
}

func AddressECFilter(address string) ECFilter {
	return func(e EstablishedContract) bool { return e.ContractAddress == address }
}

func AgreementECFilter(agreementId string) ECFilter {
	return func(e EstablishedContract) bool {
		return strings.ToLower(e.CurrentAgreementId) == strings.ToLower(agreementId)
	}
}

// filter on EstablishedContracts
type ECFilter func(EstablishedContract) bool

func FindEstablishedContracts(db *bolt.DB, filters []ECFilter) ([]EstablishedContract, error) {
	contracts := make([]EstablishedContract, 0)

	// fetch contracts
	readErr := db.View(func(tx *bolt.Tx) error {

		// ok if pending contracts bucket doesn't exist yet, depends on processing of pending contracts
		if b := tx.Bucket([]byte(E_CONTRACTS)); b != nil {
			b.ForEach(func(k, v []byte) error {

				var e EstablishedContract

				if err := json.Unmarshal(v, &e); err != nil {
					glog.Errorf("Unable to deserialize db record: %v", v)
				} else {
					exclude := false
					for _, filterFn := range filters {
						if !filterFn(e) {
							exclude = true
						}
					}
					if !exclude {
						contracts = append(contracts, e)
					}
				}
				return nil
			})
		}

		return nil // end the transaction
	})

	if readErr != nil {
		return nil, readErr
	} else {
		return contracts, nil
	}
}

func initializeMicropaymentsRecord(db *bolt.DB, containerProvider string, dbContractAddress string, currentAgreementId string) error {

	return db.Update(func(tx *bolt.Tx) error {
		if b, err := tx.CreateBucketIfNotExists([]byte(MICROPAYMENTS)); err != nil {
			return err
		} else {
			if existing := b.Get([]byte(currentAgreementId)); existing != nil {
				glog.V(2).Infof("Existing micropayment record for agreement %v, not creating new one", currentAgreementId)
				return nil
			} else {
				paymentsRecord := &Micropayments{
					PayerAddress:    containerProvider,
					ContractAddress: dbContractAddress,
					AgreementId:     currentAgreementId,
					Payments:        make(map[string]uint64, 0),
				}

				if serialized, err := json.Marshal(paymentsRecord); err != nil {
					return fmt.Errorf("Failed to serialize micropayment record: %v. Error: %v", paymentsRecord, err)
				} else if err := b.Put([]byte(currentAgreementId), serialized); err != nil {
					return fmt.Errorf("Failed to write micropayment record with key: %v. Error: %v", currentAgreementId, err)
				} else {
					glog.V(2).Infof("Succeeded creating new micropayment record for %v", currentAgreementId)
					return nil
				}
			}
		}
		return nil // end transaction
	})
}

// supports both new payments on existing payment record and new payments on new payment record
func RecordMicropayment(db *bolt.DB, agreementId string, amountToDate uint64, recorded int64) error {
	if agreementId == "" || amountToDate == 0 || recorded == 0 {
		return errors.New("Illegal (empty) argument")
	} else {
		return db.Update(func(tx *bolt.Tx) error {
			if b, err := tx.CreateBucketIfNotExists([]byte(MICROPAYMENTS)); err != nil {
				return err
			} else if existing := b.Get([]byte(agreementId)); existing == nil {
				glog.V(2).Infof("No current payment record for agreement %v, ignoring payment", agreementId)
			} else {
				glog.Infof("Existing payment record for agreement %v, updating it", agreementId)
				var payment *Micropayments

				if err := json.Unmarshal(existing, &payment); err != nil {
					return err
				} else {
					payment.Payments[strconv.FormatInt(recorded, 10)] = amountToDate

					if serialized, err := json.Marshal(payment); err != nil {
						return fmt.Errorf("Failed to serialize micropayment record: %v. Error: %v", payment, err)
					} else if err := b.Put([]byte(agreementId), serialized); err != nil {
						return fmt.Errorf("Failed to write micropayment record with key: %v. Error: %v", agreementId, err)
					} else {
						glog.V(2).Infof("Wrote micropayment at %v to agreement %v", recorded, agreementId)
					}
				}
			}

			return nil // end transaction
		})
	}
}

// sortable type for payment times
type sInt64 []int64

func (a sInt64) Len() int           { return len(a) }
func (a sInt64) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a sInt64) Less(i, j int) bool { return a[i] < a[j] }

func ActiveAgreementMicropayments(db *bolt.DB) ([]Micropayments, error) {
	filters := make([]ECFilter, 0)
	filters = append(filters, UnarchivedECFilter())

	micropayments := make([]Micropayments, 0)

	ecs, err := FindEstablishedContracts(db, filters)
	if err != nil {
		return micropayments, err
	}

	if readErr := db.View(func(tx *bolt.Tx) error {

		if b := tx.Bucket([]byte(MICROPAYMENTS)); b != nil {
			b.ForEach(func(k, v []byte) error {
				var record Micropayments

				if err := json.Unmarshal(v, &record); err != nil {
					return err
				}

				// see if this is in an active contract
				for _, ec := range ecs {
					if ec.CurrentAgreementId == record.AgreementId {
						micropayments = append(micropayments, record)
						break
					}
				}

				return nil
			})
		}
		return nil
	}); readErr != nil {
		return micropayments, readErr
	}

	return micropayments, nil
}

// reports last micropayment time and value made and account id on given agreement; 0 indicates no micropayments have been made
func LastMicropayment(db *bolt.DB, agreementId string) (int64, uint64, string, error) {
	last := int64(0)
	value := uint64(0)
	accountId := ""

	readErr := db.View(func(tx *bolt.Tx) error {

		// ok if bucket doesn't exist yet
		if b := tx.Bucket([]byte(MICROPAYMENTS)); b != nil {
			if existing := b.Get([]byte(agreementId)); existing != nil {
				var payments Micropayments

				if err := json.Unmarshal(existing, &payments); err != nil {
					glog.Errorf("Error unmarshaling JSON micropayments record for agreementId: %v. Error: %v", agreementId, err)
				} else if len(payments.Payments) > 0 {

					// get keys, sort by them, and then pluck the last one for return
					keys := make(sInt64, 0)
					for key := range payments.Payments {
						if ival, err := strconv.ParseInt(key, 10, 64); err != nil {
							glog.Infof("Error converting serialized key value to int: %v", err)
						} else {
							keys = append(keys, ival)
						}
					}

					sort.Sort(keys)
					last = keys[len(keys)-1]
					value = payments.Payments[strconv.FormatInt(last, 10)]
				}

				// set payer address even if there are no payments recorded
				accountId = payments.PayerAddress
			}
		}

		return nil // end the transaction
	})

	if readErr != nil {
		return last, value, accountId, readErr
	} else {
		return last, value, accountId, nil
	}
}

func genNonce() (string, error) {
	bytes := make([]byte, 64)

	if _, err := rand.Read(bytes); err != nil {
		return "", err
	} else {
		return base64.URLEncoding.EncodeToString(bytes), nil
	}

}
