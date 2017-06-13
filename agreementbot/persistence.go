package agreementbot

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/policy"
	"time"
)

const AGREEMENTS = "agreements"

type Agreement struct {
	CurrentAgreementId             string   `json:"current_agreement_id"`             // unique
	DeviceId                       string   `json:"device_id"`                        // the device id we are working with, immutable after construction
	HAPartners                     []string `json:"ha_partners"`                      // list of HA partner device IDs
	AgreementProtocol              string   `json:"agreement_protocol"`               // immutable after construction
	AgreementInceptionTime         uint64   `json:"agreement_inception_time"`         // immutable after construction
	AgreementCreationTime          uint64   `json:"agreement_creation_time"`          // device responds affirmatively to proposal
	AgreementFinalizedTime         uint64   `json:"agreement_finalized_time"`         // agreement is seen in the blockchain
	AgreementTimedout              uint64   `json:"agreement_timeout"`                // agreement was not finalized before it timed out
	ProposalSig                    string   `json:"proposal_signature"`               // The signature used to create the agreement - from the producer
	Proposal                       string   `json:"proposal"`                         // JSON serialization of the proposal
	ProposalHash                   string   `json:"proposal_hash"`                    // Hash of the proposal
	ConsumerProposalSig            string   `json:"consumer_proposal_sig"`            // Consumer's signature of the proposal
	Policy                         string   `json:"policy"`                           // JSON serialization of the policy used to make the proposal
	PolicyName                     string   `json:"policy_name"`                      // The name of the policy for this agreement, policy names are unique
	CounterPartyAddress            string   `json:"counter_party_address"`            // The blockchain address of the counterparty in the agreement
	DataVerificationURL            string   `json:"data_verification_URL"`            // The URL to use to ensure that this agreement is sending data.
	DataVerificationUser           string   `json:"data_verification_user"`           // The user to use with the DataVerificationURL
	DataVerificationPW             string   `json:"data_verification_pw"`             // The pw of the data verification user
	DataVerificationCheckRate      int      `json:"data_verification_check_rate"`     // How often to check for data
	DataVerificationMissedCount    uint64   `json:"data_verification_missed_count"`   // Number of data verification misses
	DataVerificationNoDataInterval int      `json:"data_verification_nodata_interval"` // How long to wait before deciding there is no data
	DisableDataVerificationChecks  bool     `json:"disable_data_verification_checks"` // disable data verification checks, assume data is being sent.
	DataVerifiedTime               uint64   `json:"data_verification_time"`           // The last time that data verification was successful
	DataNotificationSent           uint64   `json:"data_notification_sent"`           // The timestamp for when data notification was sent to the device
	MeteringTokens                 uint64   `json:"metering_tokens"`                  // Number of metering tokens from proposal
	MeteringPerTimeUnit            string   `json:"metering_per_time_unit"`           // The time units of tokens per, from the proposal
	MeteringNotificationInterval   int      `json:"metering_notify_interval"`         // The interval of time between metering notifications (seconds)
	MeteringNotificationSent       uint64   `json:"metering_notification_sent"`       // The last time a metering notification was sent
	MeteringNotificationMsgs       []string `json:"metering_notification_msgs"`       // The last metering messages that were sent, oldest at the end
	Archived                       bool     `json:"archived"`                         // The record is archived
	TerminatedReason               uint     `json:"terminated_reason"`                // The reason the agreement was terminated
	TerminatedDescription          string   `json:"terminated_description"`           // The description of why the agreement was terminated
}

func (a Agreement) String() string {
	return fmt.Sprintf("Archived: %v, " +
		"CurrentAgreementId: %v, " +
		"DeviceId: %v, " +
		"HA Partners: %v, " +
		"AgreementInceptionTime: %v, " +
		"AgreementCreationTime: %v, " +
		"AgreementFinalizedTime: %v, " +
		"AgreementTimedout: %v, " +
		"ProposalSig: %v, " +
		"ProposalHash: %v, " +
		"ConsumerProposalSig: %v, " +
		"Policy Name: %v, " +
		"CounterPartyAddress: %v, " +
		"DataVerificationURL: %v, " +
		"DataVerificationUser: %v, " +
		"DataVerificationCheckRate: %v, " +
		"DataVerificationMissedCount: %v, " +
		"DataVerificationNoDataInterval: %v, " +
		"DisableDataVerification: %v, " +
		"DataVerifiedTime: %v, " +
		"DataNotificationSent: %v, " +
		"MeteringTokens: %v, " +
		"MeteringPerTimeUnit: %v, " +
		"MeteringNotificationInterval: %v, " +
		"MeteringNotificationSent: %v, " +
		"MeteringNotificationMsgs: %v, " +
		"TerminatedReason: %v, " +
		"TerminatedDescription: %v",
		a.Archived, a.CurrentAgreementId, a.DeviceId, a.HAPartners, a.AgreementInceptionTime, a.AgreementCreationTime, a.AgreementFinalizedTime,
		a.AgreementTimedout, a.ProposalSig, a.ProposalHash, a.ConsumerProposalSig, a.PolicyName, a.CounterPartyAddress,
		a.DataVerificationURL, a.DataVerificationUser, a.DataVerificationCheckRate, a.DataVerificationMissedCount, a.DataVerificationNoDataInterval,
		a.DisableDataVerificationChecks, a.DataVerifiedTime, a.DataNotificationSent,
		a.MeteringTokens, a.MeteringPerTimeUnit, a.MeteringNotificationInterval, a.MeteringNotificationSent, a.MeteringNotificationMsgs,
		a.TerminatedReason, a.TerminatedDescription)
}

// private factory method for agreement w/out persistence safety:
func agreement(agreementid string, deviceid string, policyName string, agreementProto string) (*Agreement, error) {
	if agreementid == "" || agreementProto == "" {
		return nil, errors.New("Illegal input: agreement id or agreement protocol is empty")
	} else {
		return &Agreement{
			CurrentAgreementId:             agreementid,
			DeviceId:                       deviceid,
			HAPartners:                     []string{},
			AgreementProtocol:              agreementProto,
			AgreementInceptionTime:         uint64(time.Now().Unix()),
			AgreementCreationTime:          0,
			AgreementFinalizedTime:         0,
			AgreementTimedout:              0,
			ProposalSig:                    "",
			Proposal:                       "",
			ProposalHash:                   "",
			ConsumerProposalSig:            "",
			Policy:                         "",
			PolicyName:                     policyName,
			CounterPartyAddress:            "",
			DataVerificationURL:            "",
			DataVerificationUser:           "",
			DataVerificationPW:             "",
			DataVerificationCheckRate:      0,
			DataVerificationNoDataInterval: 0,
			DisableDataVerificationChecks:  false,
			DataVerifiedTime:               0,
			DataNotificationSent:           0,
			MeteringTokens:                 0,
			MeteringPerTimeUnit:            "",
			MeteringNotificationInterval:   0,
			MeteringNotificationSent:       0,
			MeteringNotificationMsgs:       []string{"",""},
			Archived:                       false,
			TerminatedReason:               0,
			TerminatedDescription:          "",
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

func AgreementUpdate(db *bolt.DB, agreementid string, proposal string, policy string, dvPolicy policy.DataVerification, defaultCheckRate uint64, hash string, sig string, protocol string) (*Agreement, error) {
	if agreement, err := singleAgreementUpdate(db, agreementid, protocol, func(a Agreement) *Agreement {
		a.AgreementCreationTime = uint64(time.Now().Unix())
		a.Proposal = proposal
		a.ProposalHash = hash
		a.ConsumerProposalSig = sig
		a.Policy = policy
		a.DisableDataVerificationChecks = !dvPolicy.Enabled
		if dvPolicy.Enabled {
			a.DataVerificationURL = dvPolicy.URL
			a.DataVerificationUser = dvPolicy.URLUser
			a.DataVerificationPW = dvPolicy.URLPassword
			a.DataVerificationCheckRate = dvPolicy.CheckRate
			if a.DataVerificationCheckRate == 0 {
				a.DataVerificationCheckRate = int(defaultCheckRate)
			}
			a.DataVerificationNoDataInterval = dvPolicy.Interval
			a.DataVerifiedTime = uint64(time.Now().Unix())
			a.MeteringTokens = dvPolicy.Metering.Tokens
			a.MeteringPerTimeUnit = dvPolicy.Metering.PerTimeUnit
			a.MeteringNotificationInterval = dvPolicy.Metering.NotificationIntervalS
		}
		return &a
	}); err != nil {
		return nil, err
	} else {
		return agreement, nil
	}
}

func AgreementMade(db *bolt.DB, agreementId string, counterParty string, signature string, protocol string, hapartners []string) (*Agreement, error) {
	if agreement, err := singleAgreementUpdate(db, agreementId, protocol, func(a Agreement) *Agreement {
		a.CounterPartyAddress = counterParty
		a.ProposalSig = signature
		a.HAPartners = hapartners
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

func DataNotVerified(db *bolt.DB, agreementid string, protocol string) (*Agreement, error) {
	if agreement, err := singleAgreementUpdate(db, agreementid, protocol, func(a Agreement) *Agreement {
		a.DataVerificationMissedCount += 1
		return &a
	}); err != nil {
		return nil, err
	} else {
		return agreement, nil
	}
}

func DataNotification(db *bolt.DB, agreementid string, protocol string) (*Agreement, error) {
	if agreement, err := singleAgreementUpdate(db, agreementid, protocol, func(a Agreement) *Agreement {
		a.DataNotificationSent = uint64(time.Now().Unix())
		return &a
	}); err != nil {
		return nil, err
	} else {
		return agreement, nil
	}
}

func MeteringNotification(db *bolt.DB, agreementid string, protocol string, mn string) (*Agreement, error) {
	if agreement, err := singleAgreementUpdate(db, agreementid, protocol, func(a Agreement) *Agreement {
		a.MeteringNotificationSent = uint64(time.Now().Unix())
		if len(a.MeteringNotificationMsgs) == 0 {
			a.MeteringNotificationMsgs = []string{"",""}
		}
		a.MeteringNotificationMsgs[1] = a.MeteringNotificationMsgs[0]
		a.MeteringNotificationMsgs[0] = mn
		return &a
	}); err != nil {
		return nil, err
	} else {
		return agreement, nil
	}
}

func ArchiveAgreement(db *bolt.DB, agreementid string, protocol string, reason uint, desc string) (*Agreement, error) {
	if agreement, err := singleAgreementUpdate(db, agreementid, protocol, func(a Agreement) *Agreement {
		a.Archived = true
		a.TerminatedReason = reason
		a.TerminatedDescription = desc
		return &a
	}); err != nil {
		return nil, err
	} else {
		return agreement, nil
	}
}

// no error on not found, only nil
func FindSingleAgreementByAgreementId(db *bolt.DB, agreementid string, protocol string, filters []AFilter) (*Agreement, error) {
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

// no error on not found, only nil
func FindSingleAgreementByAgreementIdAllProtocols(db *bolt.DB, agreementid string, protocols []string, filters []AFilter) (*Agreement, error) {
	filters = append(filters, IdAFilter(agreementid))

	for _, protocol := range protocols {
		if agreements, err := FindAgreements(db, filters, protocol); err != nil {
			return nil, err
		} else if len(agreements) > 1 {
			return nil, fmt.Errorf("Expected only one record for agreementid: %v, but retrieved: %v", agreementid, agreements)
		} else if len(agreements) == 0 {
			continue
		} else {
			return &agreements[0], nil
		}
	}
	return nil, nil
}

func singleAgreementUpdate(db *bolt.DB, agreementid string, protocol string, fn func(Agreement) *Agreement) (*Agreement, error) {
	if agreement, err := FindSingleAgreementByAgreementId(db, agreementid, protocol, []AFilter{}); err != nil {
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

				// This code is running in a database transaction. Within the tx, the current record is
				// read and then updated according to the updates within the input update record. It is critical
				// to check for correct data transitions within the tx .
				if mod.AgreementCreationTime == 0 {		// 1 transition from zero to non-zero
					mod.AgreementCreationTime = update.AgreementCreationTime
				}
				if mod.AgreementFinalizedTime == 0 {	// 1 transition from zero to non-zero
					mod.AgreementFinalizedTime = update.AgreementFinalizedTime
				}
				if mod.AgreementTimedout == 0 {			// 1 transition from zero to non-zero
					mod.AgreementTimedout = update.AgreementTimedout
				}
				if mod.CounterPartyAddress == "" {		// 1 transition from empty to non-empty
					mod.CounterPartyAddress = update.CounterPartyAddress
				}
				if mod.Proposal == "" {					// 1 transition from empty to non-empty
					mod.Proposal = update.Proposal
				}
				if mod.ProposalHash == "" {				// 1 transition from empty to non-empty
					mod.ProposalHash = update.ProposalHash
				}
				if mod.ConsumerProposalSig == "" {		// 1 transition from empty to non-empty
					mod.ConsumerProposalSig = update.ConsumerProposalSig
				}
				if mod.Policy == "" {					// 1 transition from empty to non-empty
					mod.Policy = update.Policy
				}
				if mod.ProposalSig == "" {				// 1 transition from empty to non-empty
					mod.ProposalSig = update.ProposalSig
				}
				if mod.DataVerificationURL == "" {		// 1 transition from empty to non-empty
					mod.DataVerificationURL = update.DataVerificationURL
				}
				if mod.DataVerificationUser == "" {		// 1 transition from empty to non-empty
					mod.DataVerificationUser = update.DataVerificationUser
				}
				if mod.DataVerificationPW == "" {		// 1 transition from empty to non-empty
					mod.DataVerificationPW = update.DataVerificationPW
				}
				if mod.DataVerificationCheckRate == 0 {	// 1 transition from zero to non-zero
					mod.DataVerificationCheckRate = update.DataVerificationCheckRate
				}
				if mod.DataVerificationMissedCount < update.DataVerificationMissedCount {	// Valid transitions must move forward
					mod.DataVerificationMissedCount = update.DataVerificationMissedCount
				}
				if mod.DataVerificationNoDataInterval == 0 {	// 1 transition from zero to non-zero
					mod.DataVerificationNoDataInterval = update.DataVerificationNoDataInterval
				}
				if !mod.DisableDataVerificationChecks {		// 1 transition from false to true
					mod.DisableDataVerificationChecks = update.DisableDataVerificationChecks
				}
				if mod.DataVerifiedTime < update.DataVerifiedTime {	// Valid transitions must move forward
					mod.DataVerifiedTime = update.DataVerifiedTime
				}
				if mod.DataNotificationSent < update.DataNotificationSent {	// Valid transitions must move forward
					mod.DataNotificationSent = update.DataNotificationSent
				}
				if len(mod.HAPartners) == 0 { 		// 1 transition from empty array to non-empty
					mod.HAPartners = update.HAPartners
				}
				if mod.MeteringTokens == 0 {		// 1 transition from zero to non-zero
					mod.MeteringTokens = update.MeteringTokens
				}
				if mod.MeteringPerTimeUnit == "" {	// 1 transition from empty to non-empty
					mod.MeteringPerTimeUnit = update.MeteringPerTimeUnit
				}
				if mod.MeteringNotificationInterval == 0 {	// 1 transition from zero to non-zero
					mod.MeteringNotificationInterval = update.MeteringNotificationInterval
				}
				if mod.MeteringNotificationSent < update.MeteringNotificationSent {	// Valid transitions must move forward
					mod.MeteringNotificationSent = update.MeteringNotificationSent
				}
				if len(mod.MeteringNotificationMsgs) == 0 || mod.MeteringNotificationMsgs[0] == update.MeteringNotificationMsgs[1] {	// msgs must move from new to old in the array
					mod.MeteringNotificationMsgs = update.MeteringNotificationMsgs
				}
				if !mod.Archived {					// 1 transition from false to true
					mod.Archived = update.Archived
				}
				if mod.TerminatedReason == 0 {		// 1 valid transition from zero to non-zero
					mod.TerminatedReason = update.TerminatedReason
				}
				if mod.TerminatedDescription == "" {	// 1 transition from empty to non-empty
					mod.TerminatedDescription = update.TerminatedDescription
				}

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
				} else if record.CurrentAgreementId != "" && !record.Archived {
					glog.Warningf("Warning! Deleting an agreement record with an agreement id, this operation should only be done after cancelling on the blockchain.")
				}
			}

			return b.Delete([]byte(pk))
		})
	}
}

func UnarchivedAFilter() AFilter {
	return func(e Agreement) bool { return !e.Archived }
}

func ArchivedAFilter() AFilter {
	return func(e Agreement) bool { return e.Archived }
}

func IdAFilter(id string) AFilter {
	return func(a Agreement) bool { return a.CurrentAgreementId == id }
}

func DevPolAFilter(deviceId string, policyName string) AFilter {
	return func(a Agreement) bool { return a.DeviceId == deviceId && a.PolicyName == policyName }
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
					if !a.Archived {
						glog.V(5).Infof("Demarshalled agreement in DB: %v", a)
					}
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
