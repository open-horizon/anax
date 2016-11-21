package agreementbot

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"time"
)

const AGREEMENTS = "agreements"

type Agreement struct {
	CurrentAgreementId            string `json:"current_agreement_id"`             // unique
	DeviceId                      string `json:"device_id"`                        // the device id we are working with, immutable after construction
	AgreementProtocol             string `json:"agreement_protocol"`               // immutable after construction
	AgreementInceptionTime        uint64 `json:"agreement_inception_time"`         // immutable after construction
	AgreementCreationTime         uint64 `json:"agreement_creation_time"`          // device responds affirmatively to proposal
	AgreementFinalizedTime        uint64 `json:"agreement_finalized_time"`         // agreement is seen in the blockchain
	AgreementTimedout             uint64 `json:"agreement_timeout"`                // agreement was not finalized before it timed out
	ProposalSig                   string `json:"proposal_signature"`               // The signature used to create the agreement
	Proposal                      string `json:"proposal"`                         // JSON serialization of the proposal
	Policy                        string `json:"policy"`                           // JSON serialization of the policy used to make the proposal
	PolicyName                    string `json:"policy_name"`                      // The name of the policy for this agreement, policy names are unique
	CounterPartyAddress           string `json:"counter_party_address"`            // The blockchain address of the counterparty in the agreement
	DataVerificationURL           string `json:"data_verification_URL"`            // The URL to use to ensure that this agreement is sending data.
	DataVerificationUser          string `json:"data_verification_user"`           // The user to use with the DataVerificationURL
	DataVerificationPW            string `json:"data_verification_pw"`             // The pw of the data verification user
	DisableDataVerificationChecks bool   `json:"disable_data_verification_checks"` // disable data verification checks, assume data is being sent.
	DataVerifiedTime              uint64 `json:"data_verification_time"`           // The last time that data verification was successful
}

func (a Agreement) String() string {
	return fmt.Sprintf("CurrentAgreementId: %v, DeviceId: %v, AgreementInceptionTime: %v, AgreementCreationTime: %v, AgreementFinalizedTime: %v, AgreementTimedout: %v, ProposalSig: %v, Policy Name: %v, CounterPartyAddress: %v, DataVerificationURL: %v, DataVerificationUser: %v, DisableDataVerification: %v, DataVerifiedTime: %v", a.CurrentAgreementId, a.DeviceId, a.AgreementInceptionTime, a.AgreementCreationTime, a.AgreementFinalizedTime, a.AgreementTimedout, a.ProposalSig, a.PolicyName, a.CounterPartyAddress, a.DataVerificationURL, a.DataVerificationUser, a.DisableDataVerificationChecks, a.DataVerifiedTime)
}

// private factory method for agreement w/out persistence safety:
func agreement(agreementid string, deviceid string, policyName string, agreementProto string) (*Agreement, error) {
	if agreementid == "" || agreementProto == "" {
		return nil, errors.New("Illegal input: agreement id or agreement protocol is empty")
	} else {
		return &Agreement{
			CurrentAgreementId:            agreementid,
			DeviceId:                      deviceid,
			AgreementProtocol:             agreementProto,
			AgreementInceptionTime:        uint64(time.Now().Unix()),
			AgreementCreationTime:         0,
			AgreementFinalizedTime:        0,
			AgreementTimedout:             0,
			ProposalSig:                   "",
			Proposal:                      "",
			Policy:                        "",
			PolicyName:                    policyName,
			CounterPartyAddress:           "",
			DataVerificationURL:           "",
			DataVerificationUser:          "",
			DataVerificationPW:            "",
			DisableDataVerificationChecks: false,
			DataVerifiedTime:              0,
		}, nil
	}
}

func AgreementAttempt(db *bolt.DB, agreementid string, deviceid string, policyName string, agreementProto string) error {
	if agreement, err := agreement(agreementid, deviceid, policyName, agreementProto); err != nil {
		return err
	} else if err := PersistNew(db, agreement.CurrentAgreementId, bucketName(agreementProto), &agreement); err != nil {
		return err
	} else {
		return nil
	}
}

func AgreementUpdate(db *bolt.DB, agreementid string, proposal string, policy string, url string, user string, pw string, checks bool, protocol string) (*Agreement, error) {
	if agreement, err := singleAgreementUpdate(db, agreementid, protocol, func(a Agreement) *Agreement {
		a.AgreementCreationTime = uint64(time.Now().Unix())
		a.Proposal = proposal
		a.Policy = policy
		a.DataVerificationURL = url
		a.DataVerificationUser = user
		a.DataVerificationPW = pw
		a.DisableDataVerificationChecks = checks
		a.DataVerifiedTime = uint64(time.Now().Unix())
		return &a
	}); err != nil {
		return nil, err
	} else {
		return agreement, nil
	}
}

func AgreementMade(db *bolt.DB, agreementId string, counterParty string, signature string, protocol string) (*Agreement, error) {
	if agreement, err := singleAgreementUpdate(db, agreementId, protocol, func(a Agreement) *Agreement {
		a.CounterPartyAddress = counterParty
		a.ProposalSig = signature
		return &a
	}); err != nil {
		return nil, err
	} else {
		return agreement, nil
	}
}

func AgreementFinalized(db *bolt.DB, agreementid string, protocol string) (*Agreement, error) {
	if agreement, err := singleAgreementUpdate(db, agreementid, protocol, func(a Agreement) *Agreement {
		a.AgreementFinalizedTime = uint64(time.Now().Unix())
		return &a
	}); err != nil {
		return nil, err
	} else {
		return agreement, nil
	}
}

func AgreementTimedout(db *bolt.DB, agreementid string, protocol string) (*Agreement, error) {
	if agreement, err := singleAgreementUpdate(db, agreementid, protocol, func(a Agreement) *Agreement {
		a.AgreementTimedout = uint64(time.Now().Unix())
		return &a
	}); err != nil {
		return nil, err
	} else {
		return agreement, nil
	}
}

func DataVerified(db *bolt.DB, agreementid string, protocol string) (*Agreement, error) {
	if agreement, err := singleAgreementUpdate(db, agreementid, protocol, func(a Agreement) *Agreement {
		a.DataVerifiedTime = uint64(time.Now().Unix())
		return &a
	}); err != nil {
		return nil, err
	} else {
		return agreement, nil
	}
}

// no error on not found, only nil
func FindSingleAgreementByAgreementId(db *bolt.DB, agreementid string, protocol string) (*Agreement, error) {
	filters := make([]AFilter, 0)
	filters = append(filters, IdAFilter(agreementid))

	if agreements, err := FindAgreements(db, filters, protocol); err != nil {
		return nil, err
	} else if len(agreements) > 1 {
		return nil, fmt.Errorf("Expected only one record for agreementid: %v, but retrieved: %v", agreementid, agreements)
	} else if len(agreements) == 0 {
		return nil, nil
	} else {
		return &agreements[0], nil
	}
}

func singleAgreementUpdate(db *bolt.DB, agreementid string, protocol string, fn func(Agreement) *Agreement) (*Agreement, error) {
	if agreement, err := FindSingleAgreementByAgreementId(db, agreementid, protocol); err != nil {
		return nil, err
	} else if agreement == nil {
		return nil, fmt.Errorf("Unable to locate agreement id: %v", agreementid)
	} else {
		updated := fn(*agreement)
		return updated, persistUpdatedAgreement(db, agreementid, protocol, updated)
	}
}

// does whole-member replacements of values that are legal to change during the course of an agreement's life
func persistUpdatedAgreement(db *bolt.DB, agreementid string, protocol string, update *Agreement) error {
	return db.Update(func(tx *bolt.Tx) error {
		if b, err := tx.CreateBucketIfNotExists([]byte(AGREEMENTS + "-" + protocol)); err != nil {
			return err
		} else {
			current := b.Get([]byte(agreementid))
			var mod Agreement

			if current == nil {
				return fmt.Errorf("No agreement with given id available to update: %v", agreementid)
			} else if err := json.Unmarshal(current, &mod); err != nil {
				return fmt.Errorf("Failed to unmarshal agreement DB data: %v", string(current))
			} else {

				// write updates only to the fields we expect should be updateable
				mod.AgreementCreationTime = update.AgreementCreationTime
				mod.AgreementFinalizedTime = update.AgreementFinalizedTime
				mod.AgreementTimedout = update.AgreementTimedout
				mod.CounterPartyAddress = update.CounterPartyAddress
				mod.AgreementProtocol = update.AgreementProtocol
				mod.Proposal = update.Proposal
				mod.Policy = update.Policy
				mod.PolicyName = update.PolicyName
				mod.ProposalSig = update.ProposalSig
				mod.DataVerificationURL = update.DataVerificationURL
				mod.DataVerificationUser = update.DataVerificationUser
				mod.DataVerificationPW = update.DataVerificationPW
				mod.DisableDataVerificationChecks = update.DisableDataVerificationChecks
				mod.DataVerifiedTime = update.DataVerifiedTime

				if serialized, err := json.Marshal(mod); err != nil {
					return fmt.Errorf("Failed to serialize agreement record: %v", mod)
				} else if err := b.Put([]byte(agreementid), serialized); err != nil {
					return fmt.Errorf("Failed to write record with key: %v", agreementid)
				} else {
					glog.V(2).Infof("Succeeded updating agreement record to %v", mod)
				}
			}
		}
		return nil
	})
}

func DeleteAgreement(db *bolt.DB, pk string, protocol string) error {
	if pk == "" {
		return fmt.Errorf("Missing required arg pk")
	} else {

		return db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(bucketName(protocol)))
			if b == nil {
				return fmt.Errorf("Unknown bucket: %v", bucketName(protocol))
			} else if existing := b.Get([]byte(pk)); existing == nil {
				glog.Errorf("Warning: record deletion requested, but record does not exist: %v", pk)
				return nil // handle already-deleted agreement as success
			} else {
				var record Agreement

				if err := json.Unmarshal(existing, &record); err != nil {
					glog.Errorf("Error deserializing agreement: %v. This is a pre-deletion warning message function so deletion will still proceed", record)
				} else if record.CurrentAgreementId != "" {
					glog.Errorf("Warning! Deleting an agreement record with an agreement id, this operation should only be done after cancelling on the blockchain.")
				}
			}

			return b.Delete([]byte(pk))
		})
	}
}

func IdAFilter(id string) AFilter {
	return func(a Agreement) bool { return a.CurrentAgreementId == id }
}

type AFilter func(Agreement) bool

func FindAgreements(db *bolt.DB, filters []AFilter, protocol string) ([]Agreement, error) {
	agreements := make([]Agreement, 0)

	readErr := db.View(func(tx *bolt.Tx) error {

		if b := tx.Bucket([]byte(bucketName(protocol))); b != nil {
			b.ForEach(func(k, v []byte) error {

				var a Agreement

				if err := json.Unmarshal(v, &a); err != nil {
					glog.Errorf("Unable to deserialize db record: %v", v)
				} else {
					glog.V(5).Infof("Demarshalled agreement in DB: %v", a)
					exclude := false
					for _, filterFn := range filters {
						if !filterFn(a) {
							exclude = true
						}
					}
					if !exclude {
						agreements = append(agreements, a)
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

func PersistNew(db *bolt.DB, pk string, bucket string, record interface{}) error {
	if pk == "" || bucket == "" {
		return fmt.Errorf("Missing required args, pk and/or bucket")
	} else {
		writeErr := db.Update(func(tx *bolt.Tx) error {

			if b, err := tx.CreateBucketIfNotExists([]byte(bucket)); err != nil {
				return err
			} else if existing := b.Get([]byte(pk)); existing != nil {
				return fmt.Errorf("Bucket %v already contains record with primary key: %v", bucket, pk)
			} else if bytes, err := json.Marshal(record); err != nil {
				return fmt.Errorf("Unable to serialize record %v. Error: %v", record, err)
			} else if err := b.Put([]byte(pk), bytes); err != nil {
				return fmt.Errorf("Unable to write to record to bucket %v. Primary key of record: %v", bucket, pk)
			} else {
				glog.V(2).Infof("Succeeded writing record identified by %v in %v", pk, bucket)
				return nil
			}
		})

		return writeErr
	}
}

func bucketName(protocol string) string {
	return AGREEMENTS + "-" + protocol
}
