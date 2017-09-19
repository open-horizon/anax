package persistence

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"time"
)

// ephemeral as of v2.1.0
const E_AGREEMENTS = "established_agreements" // may or may not be in agreements

const DEVMODE = "devmode"

// N.B. Important!! Ensure new values are handled in Update function below
// This struct is for persisting agreements
type EstablishedAgreement struct {
	Name                            string                   `json:"name"`
	SensorUrl                       []string                 `json:"sensor_url"`
	Archived                        bool                     `json:"archived"`
	CurrentAgreementId              string                   `json:"current_agreement_id"`
	ConsumerId                      string                   `json:"consumer_id"`
	CounterPartyAddress             string                   `json:"counterparty_address"`
	AgreementCreationTime           uint64                   `json:"agreement_creation_time"`
	AgreementAcceptedTime           uint64                   `json:"agreement_accepted_time"`
	AgreementBCUpdateAckTime        uint64                   `json:"agreement_bc_update_ack_time"` // V2 protocol - time when consumer acks our blockchain update
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
	MeteringNotificationMsg         MeteringNotification     `json:"metering_notification,omitempty"` // the most recent metering notification received
	BlockchainType                  string                   `json:"blockchain_type,omitempty"`       // the name of the type of the blockchain
	BlockchainName                  string                   `json:"blockchain_name,omitempty"`       // the name of the blockchain instance
	BlockchainOrg                   string                   `json:"blockchain_org,omitempty"`        // the org of the blockchain instance
}

func (c EstablishedAgreement) String() string {

	return fmt.Sprintf("Name: %v, "+
		"SensorUrl: %v, "+
		"Archived: %v, "+
		"CurrentAgreementId: %v, "+
		"ConsumerId: %v, "+
		"CounterPartyAddress: %v, "+
		"CurrentDeployment (service names): %v, "+
		"Proposal Signature: %v, "+
		"AgreementCreationTime: %v, "+
		"AgreementExecutionStartTime: %v, "+
		"AgreementAcceptedTime: %v, "+
		"AgreementBCUpdateAckTime: %v, "+
		"AgreementFinalizedTime: %v, "+
		"AgreementDataReceivedTime: %v, "+
		"AgreementTerminatedTime: %v, "+
		"AgreementForceTerminatedTime: %v, "+
		"TerminatedReason: %v, "+
		"TerminatedDescription: %v, "+
		"Agreement Protocol: %v, "+
		"Agreement ProtocolVersion: %v, "+
		"AgreementProtocolTerminatedTime : %v, "+
		"WorkloadTerminatedTime: %v, "+
		"MeteringNotificationMsg: %v, "+
		"BlockchainType: %v, "+
		"BlockchainName: %v, "+
		"BlockchainOrg: %v",
		c.Name, c.SensorUrl, c.Archived, c.CurrentAgreementId, c.ConsumerId, c.CounterPartyAddress, ServiceConfigNames(&c.CurrentDeployment),
		c.ProposalSig,
		c.AgreementCreationTime, c.AgreementExecutionStartTime, c.AgreementAcceptedTime, c.AgreementBCUpdateAckTime, c.AgreementFinalizedTime,
		c.AgreementDataReceivedTime, c.AgreementTerminatedTime, c.AgreementForceTerminatedTime, c.TerminatedReason, c.TerminatedDescription,
		c.AgreementProtocol, c.ProtocolVersion, c.AgreementProtocolTerminatedTime, c.WorkloadTerminatedTime,
		c.MeteringNotificationMsg, c.BlockchainType, c.BlockchainName, c.BlockchainOrg)

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

func NewEstablishedAgreement(db *bolt.DB, name string, agreementId string, consumerId string, proposal string, protocol string, protocolVersion int, sensorUrl []string, signature string, address string, bcType string, bcName string, bcOrg string) (*EstablishedAgreement, error) {

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
		AgreementBCUpdateAckTime:        0,
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
		MeteringNotificationMsg:         MeteringNotification{},
		BlockchainType:                  bcType,
		BlockchainName:                  bcName,
		BlockchainOrg:                   bcOrg,
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

// set the eth signature of the proposal
func AgreementStateProposalSigned(db *bolt.DB, dbAgreementId string, protocol string, sig string) (*EstablishedAgreement, error) {
	return agreementStateUpdate(db, dbAgreementId, protocol, func(c EstablishedAgreement) *EstablishedAgreement {
		c.ProposalSig = sig
		return &c
	})
}

// set the eth counterparty address when it is received from the consumer
func AgreementStateBCDataReceived(db *bolt.DB, dbAgreementId string, protocol string, address string) (*EstablishedAgreement, error) {
	return agreementStateUpdate(db, dbAgreementId, protocol, func(c EstablishedAgreement) *EstablishedAgreement {
		c.CounterPartyAddress = address
		return &c
	})
}

// set the time when out agreement blockchain update message was Ack'd.
func AgreementStateBCUpdateAcked(db *bolt.DB, dbAgreementId string, protocol string) (*EstablishedAgreement, error) {
	return agreementStateUpdate(db, dbAgreementId, protocol, func(c EstablishedAgreement) *EstablishedAgreement {
		c.AgreementBCUpdateAckTime = uint64(time.Now().Unix())
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

// set agreement state to workload terminated
func MeteringNotificationReceived(db *bolt.DB, dbAgreementId string, mn MeteringNotification, protocol string) (*EstablishedAgreement, error) {
	return agreementStateUpdate(db, dbAgreementId, protocol, func(c EstablishedAgreement) *EstablishedAgreement {
		c.MeteringNotificationMsg = mn
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

				// This code is running in a database transaction. Within the tx, the current record is
				// read and then updated according to the updates within the input update record. It is critical
				// to check for correct data transitions within the tx.
				if !mod.Archived { // 1 transition from false to true
					mod.Archived = update.Archived
				}
				if len(mod.CounterPartyAddress) == 0 { // 1 transition from empty to non-empty
					mod.CounterPartyAddress = update.CounterPartyAddress
				}
				if mod.AgreementAcceptedTime == 0 { // 1 transition from zero to non-zero
					mod.AgreementAcceptedTime = update.AgreementAcceptedTime
				}
				if mod.AgreementBCUpdateAckTime == 0 { // 1 transition from zero to non-zero
					mod.AgreementBCUpdateAckTime = update.AgreementBCUpdateAckTime
				}
				if mod.AgreementFinalizedTime == 0 { // 1 transition from zero to non-zero
					mod.AgreementFinalizedTime = update.AgreementFinalizedTime
				}
				if mod.AgreementTerminatedTime == 0 { // 1 transition from zero to non-zero
					mod.AgreementTerminatedTime = update.AgreementTerminatedTime
				}
				if mod.AgreementForceTerminatedTime < update.AgreementForceTerminatedTime { // always moves forward
					mod.AgreementForceTerminatedTime = update.AgreementForceTerminatedTime
				}
				if mod.AgreementExecutionStartTime == 0 { // 1 transition from zero to non-zero
					mod.AgreementExecutionStartTime = update.AgreementExecutionStartTime
				}
				if mod.AgreementDataReceivedTime < update.AgreementDataReceivedTime { // always moves forward
					mod.AgreementDataReceivedTime = update.AgreementDataReceivedTime
				}
				// valid transitions are from empty to non-empty to empty, ad infinitum
				if (len(mod.CurrentDeployment) == 0 && len(update.CurrentDeployment) != 0) || (len(mod.CurrentDeployment) != 0 && len(update.CurrentDeployment) == 0) {
					mod.CurrentDeployment = update.CurrentDeployment
				}
				if mod.TerminatedReason == 0 { // 1 transition from zero to non-zero
					mod.TerminatedReason = update.TerminatedReason
				}
				if mod.TerminatedDescription == "" { // 1 transition from empty to non-empty
					mod.TerminatedDescription = update.TerminatedDescription
				}
				if mod.AgreementProtocolTerminatedTime == 0 { // 1 transition from zero to non-zero
					mod.AgreementProtocolTerminatedTime = update.AgreementProtocolTerminatedTime
				}
				if mod.WorkloadTerminatedTime == 0 { // 1 transition from zero to non-zero
					mod.WorkloadTerminatedTime = update.WorkloadTerminatedTime
				}
				if update.MeteringNotificationMsg != (MeteringNotification{}) { // only save non-empty values
					mod.MeteringNotificationMsg = update.MeteringNotificationMsg
				}
				if mod.BlockchainType == "" { // 1 transition from empty to non-empty
					mod.BlockchainType = update.BlockchainType
				}
				if mod.BlockchainName == "" { // 1 transition from empty to non-empty
					mod.BlockchainName = update.BlockchainName
				}
				if mod.BlockchainOrg == "" { // 1 transition from empty to non-empty
					mod.BlockchainOrg = update.BlockchainOrg
				}
				if mod.ProposalSig == "" { // 1 transition from empty to non-empty
					mod.ProposalSig = update.ProposalSig
				}

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

func FindEstablishedAgreementsAllProtocols(db *bolt.DB, protocols []string, filters []EAFilter) ([]EstablishedAgreement, error) {
	agreements := make([]EstablishedAgreement, 0)
	for _, protocol := range protocols {
		if ags, err := FindEstablishedAgreements(db, protocol, filters); err != nil {
			return nil, err
		} else {
			agreements = append(agreements, ags...)
		}
	}
	return agreements, nil
}

// =================================================================================================
// This is the persisted version of a Metering Notification. The persistence module has its own
// type for this object to avoid a circular dependency in go that would be created if this module
// tried to import the MeteringNotification type from the metering module.
//

type MeteringNotification struct {
	Amount                 uint64 `json:"amount"`                       // The number of tokens granted by this notification, rounded to the nearest minute
	StartTime              uint64 `json:"start_time"`                   // The time when the agreement started, in seconds since 1970.
	CurrentTime            uint64 `json:"current_time"`                 // The time when the notification was sent, in seconds since 1970.
	MissedTime             uint64 `json:"missed_time"`                  // The amount of time in seconds that the consumer detected missing data
	ConsumerMeterSignature string `json:"consumer_meter_signature"`     // The consumer's signature of the meter (amount, current time, agreement Id)
	AgreementHash          string `json:"agreement_hash"`               // The 32 byte SHA3 FIPS 202 hash of the proposal for the agreement.
	ConsumerSignature      string `json:"consumer_agreement_signature"` // The consumer's signature of the agreement hash.
	ConsumerAddress        string `json:"consumer_address"`             // The consumer's blockchain account/address.
	ProducerSignature      string `json:"producer_agreement_signature"` // The producer's signature of the agreement
	BlockchainType         string `json:"blockchain_type"`              // The type of the blockchain that this notification is intended to work with
}

func (m MeteringNotification) String() string {
	return fmt.Sprintf("Amount: %v, "+
		"StartTime: %v, "+
		"CurrentTime: %v, "+
		"Missed Time: %v, "+
		"ConsumerMeterSignature: %v, "+
		"AgreementHash: %v, "+
		"ConsumerSignature: %v, "+
		"ConsumerAddress: %v, "+
		"ProducerSignature: %v, "+
		"BlockchainType: %v",
		m.Amount, m.StartTime, m.CurrentTime, m.MissedTime, m.ConsumerMeterSignature,
		m.AgreementHash, m.ConsumerSignature, m.ConsumerAddress, m.ProducerSignature,
		m.BlockchainType)
}
