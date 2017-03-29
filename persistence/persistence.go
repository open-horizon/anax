package persistence

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"os"
	"time"
)

// ephemeral as of v2.1.0
const E_AGREEMENTS = "established_agreements" // may or may not be in agreements

const DEVMODE = "devmode"

// N.B. Important!! Ensure new values are handled in Update function below
// This struct is for persisting agreements related to the 'Citizen Scientist' protocol
type EstablishedAgreement struct {
	Name                            string                   `json:"name"`
	SensorUrl                       string                   `json:"sensor_url"`
	Archived                        bool                     `json:"archived"`
	CurrentAgreementId              string                   `json:"current_agreement_id"`
	ConsumerId                      string                   `json:"consumer_id"`
	CounterPartyAddress             string                   `json:"counterparty_address"`
	AgreementCreationTime           uint64                   `json:"agreement_creation_time"`
	AgreementAcceptedTime           uint64                   `json:"agreement_accepted_time"`
	AgreementFinalizedTime          uint64                   `json:"agreement_finalized_time"`
	AgreementTerminatedTime         uint64                   `json:"agreement_terminated_time"`
	AgreementForceTerminatedTime    uint64                   `json:"agreement_force_terminated_time"`
	AgreementExecutionStartTime     uint64                   `json:"agreement_execution_start_time"`
	AgreementDataReceivedTime       uint64                   `json:"agreement_data_received_time"`
	CurrentDeployment               map[string]ServiceConfig `json:"current_deployment"`
	Proposal                        string                   `json:"proposal"`
	ProposalSig                     string                   `json:"proposal_sig"`           // the proposal currently in effect
	AgreementProtocol               string                   `json:"agreement_protocol"`     // the agreement protocol being used. It is also in the proposal.
	ProtocolVersion                 int                      `json:"protocol_version"`       // the agreement protocol version being used.
	TerminatedReason                uint64                   `json:"terminated_reason"`      // the reason that the agreement was terminated
	TerminatedDescription           string                   `json:"terminated_description"` // a string form of the reason that the agreement was terminated
	AgreementProtocolTerminatedTime uint64                   `json:"agreement_protocol_terminated_time"`
	WorkloadTerminatedTime          uint64                   `json:"workload_terminated_time"`
}

func (c EstablishedAgreement) String() string {

	return fmt.Sprintf("Name: %v, " +
		"SensorUrl: %v, " +
		"Archived: %v, " +
		"CurrentAgreementId: %v, " +
		"ConsumerId: %v, " +
		"CurrentDeployment (service names): %v, " +
		"AgreementCreationTime: %v, " +
		"AgreementExecutionStartTime: %v, " +
		"AgreementAcceptedTime: %v, " +
		"AgreementFinalizedTime: %v, " +
		"AgreementDataReceivedTime: %v, " +
		"AgreementTerminatedTime: %v, " +
		"AgreementForceTerminatedTime: %v, " +
		"TerminatedReason: %v, " +
		"TerminatedDescription: %v, " +
		"Agreement Protocol: %v, " +
		"AgreementProtocolTerminatedTime : %v, " +
		"WorkloadTerminatedTime: %v",
		c.Name, c.SensorUrl, c.Archived, c.CurrentAgreementId, c.ConsumerId, ServiceConfigNames(&c.CurrentDeployment),
		c.AgreementCreationTime, c.AgreementExecutionStartTime, c.AgreementAcceptedTime, c.AgreementFinalizedTime,
		c.AgreementDataReceivedTime, c.AgreementTerminatedTime, c.AgreementForceTerminatedTime, c.TerminatedReason, c.TerminatedDescription,
		c.AgreementProtocol, c.AgreementProtocolTerminatedTime, c.WorkloadTerminatedTime)

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

func NewEstablishedAgreement(db *bolt.DB, name string, agreementId string, consumerId string, proposal string, protocol string, protocolVersion int, sensorUrl string, signature string, address string) (*EstablishedAgreement, error) {

	if name == "" || agreementId == "" || consumerId == "" || proposal == "" || protocol == "" || protocolVersion == 0 {
		return nil, errors.New("Agreement id, consumer id, proposal, protocol, or protocol version are empty, cannot persist")
	}

	var filters []EAFilter
	filters = append(filters, UnarchivedEAFilter())
	filters = append(filters, IdEAFilter(agreementId))

	if agreements, err := FindEstablishedAgreements(db, protocol, filters); err != nil {
		return nil, err
	} else if len(agreements) != 0 {
		return nil, fmt.Errorf("Not expecting any records with id: %v, found %v", agreementId, agreements)
	}

	newAg := &EstablishedAgreement{
		Name:                            name,
		SensorUrl:                       sensorUrl,
		Archived:                        false,
		CurrentAgreementId:              agreementId,
		ConsumerId:                      consumerId,
		CounterPartyAddress:             address,
		AgreementCreationTime:           uint64(time.Now().Unix()),
		AgreementAcceptedTime:           0,
		AgreementFinalizedTime:          0,
		AgreementTerminatedTime:         0,
		AgreementForceTerminatedTime:    0,
		AgreementExecutionStartTime:     0,
		AgreementDataReceivedTime:       0,
		CurrentDeployment:               map[string]ServiceConfig{},
		Proposal:                        proposal,
		ProposalSig:                     signature,
		AgreementProtocol:               protocol,
		ProtocolVersion:                 protocolVersion,
		TerminatedReason:                0,
		TerminatedDescription:           "",
		AgreementProtocolTerminatedTime: 0,
		WorkloadTerminatedTime:          0,
	}

	return newAg, db.Update(func(tx *bolt.Tx) error {

		if b, err := tx.CreateBucketIfNotExists([]byte(E_AGREEMENTS + "-" + protocol)); err != nil {
			return err
		} else if bytes, err := json.Marshal(newAg); err != nil {
			return fmt.Errorf("Unable to marshal new record: %v", err)
		} else if err := b.Put([]byte(agreementId), []byte(bytes)); err != nil {
			return fmt.Errorf("Unable to persist agreement: %v", err)
		}

		// success, close tx
		return nil
	})
}

func ArchiveEstablishedAgreement(db *bolt.DB, agreementId string, protocol string) (*EstablishedAgreement, error) {
	return agreementStateUpdate(db, agreementId, protocol, func(c EstablishedAgreement) *EstablishedAgreement {
		c.Archived = true
		c.CurrentDeployment = map[string]ServiceConfig{}
		return &c
	})
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
func AgreementStateAccepted(db *bolt.DB, dbAgreementId string, protocol string) (*EstablishedAgreement, error) {
	return agreementStateUpdate(db, dbAgreementId, protocol, func(c EstablishedAgreement) *EstablishedAgreement {
		c.AgreementAcceptedTime = uint64(time.Now().Unix())
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
func AgreementStateTerminated(db *bolt.DB, dbAgreementId string, reason uint64, reasonString string, protocol string) (*EstablishedAgreement, error) {
	return agreementStateUpdate(db, dbAgreementId, protocol, func(c EstablishedAgreement) *EstablishedAgreement {
		c.AgreementTerminatedTime = uint64(time.Now().Unix())
		c.TerminatedReason = reason
		c.TerminatedDescription = reasonString
		return &c
	})
}

// reset agreement state to not-terminated so that we can retry the termination
func AgreementStateForceTerminated(db *bolt.DB, dbAgreementId string, protocol string) (*EstablishedAgreement, error) {
	return agreementStateUpdate(db, dbAgreementId, protocol, func(c EstablishedAgreement) *EstablishedAgreement {
		c.AgreementForceTerminatedTime = uint64(time.Now().Unix())
		return &c
	})
}

// set agreement state to data received
func AgreementStateDataReceived(db *bolt.DB, dbAgreementId string, protocol string) (*EstablishedAgreement, error) {
	return agreementStateUpdate(db, dbAgreementId, protocol, func(c EstablishedAgreement) *EstablishedAgreement {
		c.AgreementDataReceivedTime = uint64(time.Now().Unix())
		return &c
	})
}

// set agreement state to agreement protocol terminated
func AgreementStateAgreementProtocolTerminated(db *bolt.DB, dbAgreementId string, protocol string) (*EstablishedAgreement, error) {
	return agreementStateUpdate(db, dbAgreementId, protocol, func(c EstablishedAgreement) *EstablishedAgreement {
		c.AgreementProtocolTerminatedTime = uint64(time.Now().Unix())
		return &c
	})
}

// set agreement state to workload terminated
func AgreementStateWorkloadTerminated(db *bolt.DB, dbAgreementId string, protocol string) (*EstablishedAgreement, error) {
	return agreementStateUpdate(db, dbAgreementId, protocol, func(c EstablishedAgreement) *EstablishedAgreement {
		c.WorkloadTerminatedTime = uint64(time.Now().Unix())
		return &c
	})
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
		return updated, persistUpdatedAgreement(db, dbAgreementId, protocol, updated)
	}
}

// does whole-member replacements of values that are legal to change during the course of a contract's life
func persistUpdatedAgreement(db *bolt.DB, dbAgreementId string, protocol string, update *EstablishedAgreement) error {
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
				mod.AgreementTerminatedTime = update.AgreementTerminatedTime
				mod.AgreementForceTerminatedTime = update.AgreementForceTerminatedTime
				mod.AgreementExecutionStartTime = update.AgreementExecutionStartTime
				mod.AgreementDataReceivedTime = update.AgreementDataReceivedTime
				mod.CurrentDeployment = update.CurrentDeployment
				mod.TerminatedReason = update.TerminatedReason
				mod.TerminatedDescription = update.TerminatedDescription
				mod.AgreementProtocolTerminatedTime = update.AgreementProtocolTerminatedTime
				mod.WorkloadTerminatedTime = update.WorkloadTerminatedTime

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

		if b := tx.Bucket([]byte(E_AGREEMENTS + "-" + protocol)); b != nil {
			b.ForEach(func(k, v []byte) error {

				var e EstablishedAgreement

				if err := json.Unmarshal(v, &e); err != nil {
					glog.Errorf("Unable to deserialize db record: %v", v)
				} else {
					if !e.Archived {
						glog.V(5).Infof("Demarshalled agreement in DB: %v", e)
					}
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

// TODO: do something with this, must persist them and refer to the Service

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
