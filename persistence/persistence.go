package persistence

import (
	"encoding/json"
	"errors"
	"fmt"
	docker "github.com/fsouza/go-dockerclient"
	"os"
	"time"

	"github.com/boltdb/bolt"
	"github.com/golang/glog"
)

const P_CONTRACTS = "pending_contracts"
const E_AGREEMENTS = "established_agreements" // may or may not be in agreements
const DEVMODE = "devmode"

// uses pointers for members b/c it allows nil-checking at deserialization; !Important!: the json field names here must not change w/out changing the error messages returned from the API, they are not programmatically determined
type PendingContract struct {
	Name                 *string            `json:"name"`
	SensorUrl            *string            `json:"sensor_url"`
	Arch                 string             `json:"arch"`
	CPUs                 int                `json:"cpus"`
	RAM                  *int               `json:"ram"`
	IsLocEnabled         bool               `json:"is_loc_enabled"`
	Lat                  *string            `json:"lat"`
	Lon                  *string            `json:"lon"`
	AppAttributes        *map[string]string `json:"app_attributes"`
	PrivateAppAttributes *map[string]string `json:"private_app_attributes"`
}

type DevMode struct {
	Mode     bool `json:"mode"`
	LocalGov bool `json:"localgov"`
}

type IoTFConf struct {
	Name    string     `json:"name"`
	ApiSpec []SpecR    `json:"apiSpec"`
	Arch    string     `json:"arch"`
	Quarks  QuarksConf `json:"quarks"`
}

type SpecR struct {
	SpecRef string `json:"specRef"`
}

type QuarksConf struct {
	CloudMsgBrokerHost string `json:"cloudMsgBrokerHost"`
	// CloudMsgBrokerTopics      IoTFTopics `json:"cloudMsgBrokerTopics"`
	// CloudMsgBrokerCredentials IoTFCred   `json:"cloudMsgBrokerCredentials"`
	DataVerificationInterval int `json:"dataVerificationInterval"`
}

type IoTFTopics struct {
	Apps        string   `json:"apps"`
	PublishData []string `json:"publishData"`
	Control     string   `json:"control"`
}

type IoTFCred struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

func (c PendingContract) String() string {
	return fmt.Sprintf("Name: %v, SensorUrl: %v, Arch: %v, CPUs: %v, RAM: %v, IsLocEnabled: %v, AppAttribs: %v", *c.Name, *c.SensorUrl, c.Arch, c.CPUs, *c.RAM, c.IsLocEnabled, *c.AppAttributes)
}

// N.B. Important!! Ensure new values are handled in Update function below
// This struct is for persisting agreements related to the 'Citizen Scientist' protocol
type EstablishedAgreement struct {
	Name                        string                   `json:"name"`
	SensorUrl                   string                   `json:"sensor_url"`
	Archived                    bool                     `json:"archived"` // TODO: give risham, booz a way to indicate that a contract needs to be archived; REST api
	CurrentAgreementId          string                   `json:"current_agreement_id"`
	ConsumerId                  string                   `json:"consumer_id"`
	CounterPartyAddress         string                   `json:"counterparty_address"`
	AgreementCreationTime       uint64                   `json:"agreement_creation_time"`
	AgreementAcceptedTime       uint64                   `json:"agreement_accepted_time"`
	AgreementFinalizedTime      uint64                   `json:"agreement_finalized_time"`
	AgreementTerminated         uint64                   `json:"agreement_terminated_time"`
	AgreementExecutionStartTime uint64                   `json:"agreement_execution_start_time"`
	PrivateEnvironmentAdditions map[string]string        `json:"private_environment_additions"` // platform-provided environment facts, none of which can leave the device
	EnvironmentAdditions        map[string]string        `json:"environment_additions"`         // platform-provided environment facts, some of which are published on the blockchain for marketplace searching
	CurrentDeployment           map[string]ServiceConfig `json:"current_deployment"`
	Proposal                    string                   `json:"proposal"`
	ProposalSig                 string                   `json:"proposal_sig"`       // the proposal currently in effect
	AgreementProtocol           string                   `json:"agreement_protocol"` // the agreement protocol being used. It is also in the proposal.
}

func (c EstablishedAgreement) String() string {

	return fmt.Sprintf("Name: %v , SensorUrl: %v , Archived: %v , CurrentAgreementId: %v, ConsumerId: %v, CurrentDeployment (service names): %v, PrivateEnvironmentAdditions: %v, EnvironmentAdditions: %v, AgreementCreationTime: %v, AgreementExecutionStartTime: %v, AgreementAcceptedTime: %v, AgreementFinalizedTime: %v, Agreement Protocol: %v", c.Name, c.SensorUrl, c.Archived, c.CurrentAgreementId, c.ConsumerId, ServiceConfigNames(&c.CurrentDeployment), c.PrivateEnvironmentAdditions, c.EnvironmentAdditions, c.AgreementCreationTime, c.AgreementExecutionStartTime, c.AgreementAcceptedTime, c.AgreementFinalizedTime, c.AgreementProtocol)

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


func NewEstablishedAgreement(db *bolt.DB, agreementId string, consumerId string, proposal string, protocol string, sensorUrl string) (*EstablishedAgreement, error) {

	if agreementId == "" || consumerId == "" || proposal == "" || protocol == "" {
		return nil, errors.New("Agreement id, consumer id, proposal or protocol are empty, cannot persist")
	} else {

		filters := make([]EAFilter, 0)
		filters = append(filters, UnarchivedEAFilter())
		filters = append(filters, IdEAFilter(agreementId))

		if agreements, err := FindEstablishedAgreements(db, protocol, filters); err != nil {
			return nil, err
		} else if len(agreements) != 0 {
			return nil, fmt.Errorf("Not expecting any records with id: %v, found %v", agreementId, agreements)
		} else {
			// find the user display name of the agreement
			name := "name"
			if sensorUrl != "" {
				if pcs, err := FindPendingContractByFilters(db, []PCFilter{SensorUrlPCFilter(sensorUrl)}); err != nil {
					glog.Errorf("Error getting pending contract from sensor url %v: %v", sensorUrl, err)
				} else {
					if len(pcs) > 0 {
						name = *pcs[0].Name
					}
				}
			}

			// construct map for sensitive environment stuff from pending contract
			privateEnvironmentAdditions := make(map[string]string, 0)
			privateEnvironmentAdditions["lat"] = "lat"
			privateEnvironmentAdditions["lon"] = "lon"

			newAg := &EstablishedAgreement{
				Name:                        name,
				SensorUrl:                   sensorUrl,
				Archived:                    false,
				CurrentAgreementId:          agreementId,
				ConsumerId:                  consumerId,
				CounterPartyAddress:         "",
				PrivateEnvironmentAdditions: privateEnvironmentAdditions,
				EnvironmentAdditions:        map[string]string{},
				AgreementCreationTime:       uint64(time.Now().Unix()),
				AgreementAcceptedTime:       0,
				AgreementFinalizedTime:      0,
				AgreementTerminated:         0,
				AgreementExecutionStartTime: 0,
				CurrentDeployment:           map[string]ServiceConfig{},
				Proposal:                    proposal,
				ProposalSig:                 "",
				AgreementProtocol:           protocol,
			}

			return newAg, db.Update(func(tx *bolt.Tx) error {

				if b, err := tx.CreateBucketIfNotExists([]byte(E_AGREEMENTS + "-" + protocol)); err != nil {
					return err
				} else if bytes, err := json.Marshal(newAg); err != nil {
					return fmt.Errorf("Unable to marshal new record: %v", err)
				} else if err := b.Put([]byte(agreementId), []byte(bytes)); err != nil {
					return fmt.Errorf("Unable to persist agreement: %v", err)
				} else {
					return nil
				}
			})
		}
	}
}

func DeleteEstablishedAgreement(db *bolt.DB, agreementId string, protocol string) error {

	if agreementId == "" {
		return errors.New("Agreement id empty, cannot remove")
	} else {

		filters := make([]EAFilter, 0)
		filters = append(filters, UnarchivedEAFilter())
		filters = append(filters, IdEAFilter(agreementId))

		if agreements, err := FindEstablishedAgreements(db, protocol, filters); err != nil {
			return err
		} else if len(agreements) != 1 {
			return fmt.Errorf("Expecting 1 records with id: %v, found %v", agreementId, agreements)
		} else {

			return db.Update(func(tx *bolt.Tx) error {

				if b, err := tx.CreateBucketIfNotExists([]byte(E_AGREEMENTS + "-" + protocol)); err != nil {
					return err
				} else if err := b.Delete([]byte(agreementId)); err != nil {
					return fmt.Errorf("Unable to delete agreement: %v", err)
				} else {
					return nil
				}
			})
		}
	}
}

// set contract record state to in agreement, not yet accepted; N.B. It's expected that privateEnvironmentAdditions will already have been added by this time
func AgreementStateInAgreement(db *bolt.DB, dbAgreementId string, protocol string, environmentAdditions map[string]string) (*EstablishedAgreement, error) {

	var err error

	ret, updateErr := agreementStateUpdate(db, dbAgreementId, protocol, func(c EstablishedAgreement) *EstablishedAgreement {
		c.AgreementCreationTime = uint64(time.Now().Unix())
		c.EnvironmentAdditions = environmentAdditions

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

// set agreement state to execution started
func AgreementStateExecutionStarted(db *bolt.DB, dbAgreementId string, protocol string, deployment *map[string]ServiceConfig) (*EstablishedAgreement, error) {
	return agreementStateUpdate(db, dbAgreementId, protocol, func(c EstablishedAgreement) *EstablishedAgreement {
		c.AgreementExecutionStartTime = uint64(time.Now().Unix())
		c.CurrentDeployment = *deployment
		return &c
	})
}

// set agreement state to accepted, a positive reply is being sent
func AgreementStateAccepted(db *bolt.DB, dbAgreementId string, protocol string, proposal string, from string, signature string) (*EstablishedAgreement, error) {
	return agreementStateUpdate(db, dbAgreementId, protocol, func(c EstablishedAgreement) *EstablishedAgreement {
		c.AgreementAcceptedTime = uint64(time.Now().Unix())
		c.CounterPartyAddress = from
		c.Proposal = proposal
		c.ProposalSig = signature
		return &c
	})
}

// set agreement state to finalized
func AgreementStateFinalized(db *bolt.DB, dbAgreementId string, protocol string) (*EstablishedAgreement, error) {
	return agreementStateUpdate(db, dbAgreementId, protocol, func(c EstablishedAgreement) *EstablishedAgreement {
		c.AgreementFinalizedTime = uint64(time.Now().Unix())
		return &c
	})
}

// set agreement state to terminated
func AgreementStateTerminated(db *bolt.DB, dbAgreementId string, protocol string) (*EstablishedAgreement, error) {
	return agreementStateUpdate(db, dbAgreementId, protocol, func(c EstablishedAgreement) *EstablishedAgreement {
		c.AgreementTerminated = uint64(time.Now().Unix())
		return &c
	})
}

func agreementStateUpdate(db *bolt.DB, dbAgreementId string, protocol string, fn func(EstablishedAgreement) *EstablishedAgreement) (*EstablishedAgreement, error) {
	filters := make([]EAFilter, 0)
	filters = append(filters, UnarchivedEAFilter())
	filters = append(filters, IdEAFilter(dbAgreementId))

	if agreements, err := FindEstablishedAgreements(db, protocol, filters); err != nil {
		return nil, err
	} else if len(agreements) > 1 {
		return nil, fmt.Errorf("Expected only one record for dbAgreementId: %v, but retrieved: %v", dbAgreementId, agreements)
	} else if len(agreements) == 0 {
		return nil, fmt.Errorf("No record with id: %v", dbAgreementId)
	} else {
		// run this single contract through provided update function and persist it
		updated := fn(agreements[0])
		return updated, PersistUpdatedAgreement(db, dbAgreementId, protocol, updated)
	}
}

// does whole-member replacements of values that are legal to change during the course of a contract's life
func PersistUpdatedAgreement(db *bolt.DB, dbAgreementId string, protocol string, update *EstablishedAgreement) error {
	return db.Update(func(tx *bolt.Tx) error {
		if b, err := tx.CreateBucketIfNotExists([]byte(E_AGREEMENTS + "-" + protocol)); err != nil {
			return err
		} else {
			current := b.Get([]byte(dbAgreementId))
			var mod EstablishedAgreement

			if current == nil {
				return fmt.Errorf("No agreement with given id available to update: %v", dbAgreementId)
			} else if err := json.Unmarshal(current, &mod); err != nil {
				return fmt.Errorf("Failed to unmarshal agreement DB data: %v. Error: %v", string(current), err)
			} else {

				// prevAgreementId := mod.CurrentAgreementId

				// write updates only to the fields we expect should be updateable
				mod.Archived = update.Archived
				mod.AgreementAcceptedTime = update.AgreementAcceptedTime
				mod.AgreementFinalizedTime = update.AgreementFinalizedTime
				mod.AgreementTerminated = update.AgreementTerminated
				mod.CounterPartyAddress = update.CounterPartyAddress
				mod.EnvironmentAdditions = update.EnvironmentAdditions
				mod.AgreementExecutionStartTime = update.AgreementExecutionStartTime
				mod.CurrentDeployment = update.CurrentDeployment
				mod.Proposal = update.Proposal
				mod.ProposalSig = update.ProposalSig

				if serialized, err := json.Marshal(mod); err != nil {
					return fmt.Errorf("Failed to serialize contract record: %v. Error: %v", mod, err)
				} else if err := b.Put([]byte(dbAgreementId), serialized); err != nil {
					return fmt.Errorf("Failed to write contract record with key: %v. Error: %v", dbAgreementId, err)
				} else {
					glog.V(2).Infof("Succeeded updating agreement id record to %v", mod)
					return nil
				}
			}
		}
	})
}

// Saves the pending contract in the db
func SavePendingContract(db *bolt.DB, contract PendingContract) error {
	duplicate := false

	// check for duplicate pending and established contracts
	pErr := db.View(func(tx *bolt.Tx) error {
		bp := tx.Bucket([]byte(P_CONTRACTS))
		if bp != nil {
			duplicate = (bp.Get([]byte(*contract.Name)) != nil)
		}

		return nil

	})

	if pErr != nil {
		return fmt.Errorf("Error checking duplicates of %v from db. Error: %v", contract, pErr)
	} else if duplicate {
		return fmt.Errorf("Duplicate record found in the pending contracts for %v.", *contract.Name)
	}

	writeErr := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(P_CONTRACTS))
		if err != nil {
			return err
		}

		if serial, err := json.Marshal(&contract); err != nil {
			return fmt.Errorf("Failed to serialize pending contract: %v. Error: %v", contract, err)
		} else {
			return b.Put([]byte(*contract.Name), serial)
		}
		return nil
	})

	return writeErr
}

// filter on PendingContract
type PCFilter func(PendingContract) bool

// PendingContract filter by SensorUrl
func SensorUrlPCFilter(url string) PCFilter {
	return func(pc PendingContract) bool {
		if pc.SensorUrl == nil {
			return false
		} else {
			return *pc.SensorUrl == url
		}
	}
}

// PendingContract filter by Name
func NamePCFilter(name string) PCFilter {
	return func(pc PendingContract) bool { return *pc.Name == name }
}

// Find pending contract by filters
func FindPendingContractByFilters(db *bolt.DB, filters []PCFilter) ([]PendingContract, error) {
	pcs := make([]PendingContract, 0)

	// fetch pending contracts
	readErr := db.View(func(tx *bolt.Tx) error {

		if b := tx.Bucket([]byte(P_CONTRACTS)); b != nil {
			b.ForEach(func(_, v []byte) error {

				var p PendingContract

				if err := json.Unmarshal(v, &p); err != nil {
					glog.Errorf("Unable to deserialize db record: %v. Error: %v", v, err)
				} else {
					exclude := false
					for _, filterFn := range filters {
						if !filterFn(p) {
							exclude = true
						}
					}
					if !exclude {
						pcs = append(pcs, p)
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
		return pcs, nil
	}
}

func UnarchivedEAFilter() EAFilter {
	return func(e EstablishedAgreement) bool { return !e.Archived }
}

func IdEAFilter(id string) EAFilter {
	return func(e EstablishedAgreement) bool { return e.CurrentAgreementId == id }
}

// filter on EstablishedAgreements
type EAFilter func(EstablishedAgreement) bool

func FindEstablishedAgreements(db *bolt.DB, protocol string, filters []EAFilter) ([]EstablishedAgreement, error) {
	agreements := make([]EstablishedAgreement, 0)

	// fetch contracts
	readErr := db.View(func(tx *bolt.Tx) error {

		// ok if pending contracts bucket doesn't exist yet, depends on processing of pending agreements
		if b := tx.Bucket([]byte(E_AGREEMENTS + "-" + protocol)); b != nil {
			b.ForEach(func(k, v []byte) error {

				var e EstablishedAgreement

				if err := json.Unmarshal(v, &e); err != nil {
					glog.Errorf("Unable to deserialize db record: %v", v)
				} else {
					glog.V(5).Infof("Demarshalled agreement in DB: %v", e)
					exclude := false
					for _, filterFn := range filters {
						if !filterFn(e) {
							exclude = true
						}
					}
					if !exclude {
						agreements = append(agreements, e)
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
		return agreements, nil
	}
}

// save the devmode in to the "devmode" bucket
func SaveDevmode(db *bolt.DB, devmode DevMode) error {
	// store some data
	err := db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(DEVMODE))
		if err != nil {
			return err
		}

		if serialized, err := json.Marshal(devmode); err != nil {
			return fmt.Errorf("Failed to serialize devemode: %v. Error: %v", devmode, err)
		} else if err := bucket.Put([]byte("devmode"), serialized); err != nil {
			return fmt.Errorf("Failed to write devmode: %v. Error: %v", devmode, err)
		} else {
			glog.V(2).Infof("Succeeded saving devmode %v", devmode)
			return nil
		}
	})

	return err
}

// get the devmode setting
func GetDevmode(db *bolt.DB) (DevMode, error) {
	var devmode DevMode
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(DEVMODE))
		if bucket == nil {
			devmode.Mode = false
			devmode.LocalGov = false
			return nil
		}

		val := bucket.Get([]byte("devmode"))
		if val == nil {
			devmode.Mode = false
			devmode.LocalGov = false
			return nil
		} else if err := json.Unmarshal(val, &devmode); err != nil {
			return fmt.Errorf("Failed to unmarshal devmode data.  Error: %v", err)
		} else {
			return nil
		}
	})
	return devmode, err
}

// Save the IoTF configration to a file so that the PolicyWriter can pick it up
func SaveIoTFConf(path string, iotf_conf IoTFConf) error {
	fh, err := os.OpenFile(path+"/iotfconf.json", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer fh.Close()

	if b, err := json.MarshalIndent(iotf_conf, "", "  "); err != nil {
		return err
	} else {
		if _, err := fh.Write(b); err != nil {
			return err
		}
	}

	return nil
}
