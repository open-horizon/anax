// +build unit

package persistence

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"testing"
	"time"
)

type EstablishedAgreement_Old struct {
	Name                         string   `json:"name"`
	SensorUrl                    []string `json:"sensor_url"`
	Archived                     bool     `json:"archived"`
	CurrentAgreementId           string   `json:"current_agreement_id"`
	ConsumerId                   string   `json:"consumer_id"`
	CounterPartyAddress          string   `json:"counterparty_address"`
	AgreementCreationTime        uint64   `json:"agreement_creation_time"`
	AgreementAcceptedTime        uint64   `json:"agreement_accepted_time"`
	AgreementBCUpdateAckTime     uint64   `json:"agreement_bc_update_ack_time"` // V2 protocol - time when consumer acks our blockchain update
	AgreementFinalizedTime       uint64   `json:"agreement_finalized_time"`
	AgreementTerminatedTime      uint64   `json:"agreement_terminated_time"`
	AgreementForceTerminatedTime uint64   `json:"agreement_force_terminated_time"`
	AgreementExecutionStartTime  uint64   `json:"agreement_execution_start_time"`
	AgreementDataReceivedTime    uint64   `json:"agreement_data_received_time"`
	// One of the following 2 fields are set when the worker that owns deployment for this agreement, starts deploying the services in the agreement.
	CurrentDeployment               map[string]ServiceConfig `json:"current_deployment"`  // Native Horizon deployment config goes here, mutually exclusive with the extended deployment field. This field is set before the imagefetch worker starts the workload.
	ExtendedDeployment              map[string]interface{}   `json:"extended_deployment"` // All non-native deployment configs go here.
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
	RunningWorkload                 WorkloadInfo             `json:"workload_to_run,omitempty"`       // For display purposes, a copy of the workload info that this agreement is managing. It should be the same info that is buried inside the proposal.
}

func (c EstablishedAgreement_Old) String() string {

	return fmt.Sprintf("Name: %v, "+
		"SensorUrl: %v, "+
		"Archived: %v, "+
		"CurrentAgreementId: %v, "+
		"ConsumerId: %v, "+
		"CounterPartyAddress: %v, "+
		"CurrentDeployment (service names): %v, "+
		"ExtendedDeployment: %v, "+
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
		c.ExtendedDeployment, c.ProposalSig,
		c.AgreementCreationTime, c.AgreementExecutionStartTime, c.AgreementAcceptedTime, c.AgreementBCUpdateAckTime, c.AgreementFinalizedTime,
		c.AgreementDataReceivedTime, c.AgreementTerminatedTime, c.AgreementForceTerminatedTime, c.TerminatedReason, c.TerminatedDescription,
		c.AgreementProtocol, c.ProtocolVersion, c.AgreementProtocolTerminatedTime, c.WorkloadTerminatedTime,
		c.MeteringNotificationMsg, c.BlockchainType, c.BlockchainName, c.BlockchainOrg)

}

func NewEstablishedAgreement_Old(db *bolt.DB, name string, agreementId string, consumerId string, proposal string, protocol string, protocolVersion int, sensorUrl []string, signature string, address string, bcType string, bcName string, bcOrg string, wi *WorkloadInfo) (*EstablishedAgreement_Old, error) {

	if name == "" || agreementId == "" || consumerId == "" || proposal == "" || protocol == "" || protocolVersion == 0 {
		return nil, errors.New("Agreement id, consumer id, proposal, protocol, or protocol version are empty, cannot persist")
	}

	newAg := &EstablishedAgreement_Old{
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
		ExtendedDeployment:              map[string]interface{}{},
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
		RunningWorkload:                 *wi,
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

func Test_Backward_Compitibility_EstablishedAgreement(t *testing.T) {
	dir, testDb, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	// save an agreement with old structure
	wi_old1, _ := NewWorkloadInfo("myurl_old1", "myorg_old1", "myversion_old1", "")
	_, err = NewEstablishedAgreement_Old(testDb, "Old agreement structure 1", "123451", "agbot1", "proposal", "Basic", 1, []string{"url1", "url2"}, "signature", "address", "bcType", "bcName", "bcOrg", wi_old1)
	if err != nil {
		t.Error(err)
	}

	// save an agreement with old structure with empty sensor url
	wi_old2, _ := NewWorkloadInfo("myurl_old2", "myorg_old2", "myversion_old2", "")
	_, err = NewEstablishedAgreement_Old(testDb, "Old agreement structure 2", "123452", "agbot1", "proposal", "Basic", 1, []string{}, "signature", "address", "bcType", "bcName", "bcOrg", wi_old2)
	if err != nil {
		t.Error(err)
	}

	// save an agreement with new structure
	wi_new1, _ := NewWorkloadInfo("myurl_new1", "myorg_new1", "myversion_new1", "")
	sp3 := ServiceSpec{Url: "url3", Org: "myorg3"}
	sp4 := ServiceSpec{Url: "url4", Org: "myorg4"}
	_, err = NewEstablishedAgreement(testDb, "New agreement structure 1", "543211", "agbot2", "proposal2", "Basic2", 1, []ServiceSpec{sp3, sp4}, "signature2", "address2", "bcType2", "bcName2", "bcOrg2", wi_new1)
	if err != nil {
		t.Error(err)
	}

	// save an agreement with new structure with empty DependentServices
	wi_new2, _ := NewWorkloadInfo("myurl_new2", "myorg_new2", "myversion_new2", "")
	_, err = NewEstablishedAgreement(testDb, "New agreement structure 2", "543212", "agbot2", "proposal2", "Basic2", 1, []ServiceSpec{}, "signature2", "address2", "bcType2", "bcName2", "bcOrg2", wi_new2)
	if err != nil {
		t.Error(err)
	}

	// now retrive them
	ags, err := FindEstablishedAgreementsAllProtocols(testDb, []string{"Basic", "Basic2"}, []EAFilter{})
	if err != nil {
		t.Error(err)
	} else if len(ags) != 4 {
		t.Errorf("Should get 3 agreements, but got %v", len(ags))
	} else {
		for _, ag := range ags {
			if ag.Name == "Old agreement structure 1" {
				if ag.CurrentAgreementId != "123451" || ag.DependentServices == nil || len(ag.DependentServices) != 2 ||
					ag.DependentServices[0].Url != "url1" || ag.DependentServices[0].Org != "" ||
					ag.DependentServices[1].Url != "url2" || ag.DependentServices[1].Org != "" ||
					ag.RunningWorkload.URL != wi_old1.URL || ag.RunningWorkload.Org != wi_old1.Org {
					t.Errorf("Retrieving old agreement1 from db failed. %v", ag)
				}
			} else if ag.Name == "Old agreement structure 2" {
				if ag.CurrentAgreementId != "123452" || ag.DependentServices == nil || len(ag.DependentServices) != 0 ||
					ag.RunningWorkload.URL != wi_old2.URL || ag.RunningWorkload.Org != wi_old2.Org {
					t.Errorf("Retrieving old agreement2 from db failed. %v", ag)
				}
			} else if ag.Name == "New agreement structure 1" {
				if ag.CurrentAgreementId != "543211" || ag.DependentServices == nil || len(ag.DependentServices) != 2 ||
					ag.DependentServices[0].Url != sp3.Url || ag.DependentServices[0].Org != sp3.Org ||
					ag.DependentServices[1].Url != sp4.Url || ag.DependentServices[1].Org != sp4.Org ||
					ag.RunningWorkload.URL != wi_new1.URL || ag.RunningWorkload.Org != wi_new1.Org {
					t.Errorf("Retrieving new agreement3 from db failed. %v", ag)
				}
			} else {
				if ag.CurrentAgreementId != "543212" || ag.DependentServices == nil || len(ag.DependentServices) != 0 ||
					ag.RunningWorkload.URL != wi_new2.URL || ag.RunningWorkload.Org != wi_new2.Org {
					t.Errorf("Retrieving new agreement4 from db failed. %v", ag)
				}
			}
		}
	}
}
