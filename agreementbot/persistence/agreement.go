package persistence

import (
	"errors"
	"fmt"
	"github.com/open-horizon/anax/policy"
	"time"
)

// device types. make the duplicates here so that agbot does not have dependency on
// the edge side persistence
const DEVICE_TYPE_DEVICE = "device"
const DEVICE_TYPE_CLUSTER = "cluster"

type Agreement struct {
	CurrentAgreementId             string   `json:"current_agreement_id"`              // unique
	Org                            string   `json:"org"`                               // the org in which the policy exists that was used to make this agreement
	DeviceId                       string   `json:"device_id"`                         // the device id we are working with, immutable after construction
	DeviceType                     string   `json:"device_type"`                       // the type of the device, the valid values are 'device' and 'cluster', the default is 'decive'
	AgreementProtocol              string   `json:"agreement_protocol"`                // immutable after construction - name of protocol in use
	AgreementProtocolVersion       int      `json:"agreement_protocol_version"`        // version of protocol in use - New in V2 protocol
	AgreementInceptionTime         uint64   `json:"agreement_inception_time"`          // immutable after construction
	AgreementCreationTime          uint64   `json:"agreement_creation_time"`           // device responds affirmatively to proposal
	AgreementFinalizedTime         uint64   `json:"agreement_finalized_time"`          // agreement is seen in the blockchain
	AgreementTimedout              uint64   `json:"agreement_timeout"`                 // agreement was not finalized before it timed out
	ProposalSig                    string   `json:"proposal_signature"`                // The signature used to create the agreement - from the producer
	Proposal                       string   `json:"proposal"`                          // JSON serialization of the proposal
	ProposalHash                   string   `json:"proposal_hash"`                     // Hash of the proposal
	ConsumerProposalSig            string   `json:"consumer_proposal_sig"`             // Consumer's signature of the proposal
	Policy                         string   `json:"policy"`                            // JSON serialization of the policy used to make the proposal
	PolicyName                     string   `json:"policy_name"`                       // The name of the policy for this agreement, policy names are unique
	CounterPartyAddress            string   `json:"counter_party_address"`             // The blockchain address of the counterparty in the agreement
	DataVerificationURL            string   `json:"data_verification_URL"`             // The URL to use to ensure that this agreement is sending data.
	DataVerificationUser           string   `json:"data_verification_user"`            // The user to use with the DataVerificationURL
	DataVerificationPW             string   `json:"data_verification_pw"`              // The pw of the data verification user
	DataVerificationCheckRate      int      `json:"data_verification_check_rate"`      // How often to check for data
	DataVerificationMissedCount    uint64   `json:"data_verification_missed_count"`    // Number of data verification misses
	DataVerificationNoDataInterval int      `json:"data_verification_nodata_interval"` // How long to wait before deciding there is no data
	DisableDataVerificationChecks  bool     `json:"disable_data_verification_checks"`  // disable data verification checks, assume data is being sent.
	DataVerifiedTime               uint64   `json:"data_verification_time"`            // The last time that data verification was successful
	DataNotificationSent           uint64   `json:"data_notification_sent"`            // The timestamp for when data notification was sent to the device
	MeteringTokens                 uint64   `json:"metering_tokens"`                   // Number of metering tokens from proposal
	MeteringPerTimeUnit            string   `json:"metering_per_time_unit"`            // The time units of tokens per, from the proposal
	MeteringNotificationInterval   int      `json:"metering_notify_interval"`          // The interval of time between metering notifications (seconds)
	MeteringNotificationSent       uint64   `json:"metering_notification_sent"`        // The last time a metering notification was sent
	MeteringNotificationMsgs       []string `json:"metering_notification_msgs"`        // The last metering messages that were sent, oldest at the end
	Archived                       bool     `json:"archived"`                          // The record is archived
	TerminatedReason               uint     `json:"terminated_reason"`                 // The reason the agreement was terminated
	TerminatedDescription          string   `json:"terminated_description"`            // The description of why the agreement was terminated
	BlockchainType                 string   `json:"blockchain_type"`                   // The name of the blockchain type that is being used (new V2 protocol)
	BlockchainName                 string   `json:"blockchain_name"`                   // The name of the blockchain being used (new V2 protocol)
	BlockchainOrg                  string   `json:"blockchain_org"`                    // The name of the blockchain org being used (new V2 protocol)
	BCUpdateAckTime                uint64   `json:"blockchain_update_ack_time"`        // The time when the producer ACked our update ot him (new V2 protocol)
	NHMissingHBInterval            int      `json:"missing_heartbeat_interval"`        // How long a heartbeat can be missing until it is considered missing (in seconds)
	NHCheckAgreementStatus         int      `json:"check_agreement_status"`            // How often to check that the node agreement entry still exists in the exchange (in seconds)
	Pattern                        string   `json:"pattern"`                           // The pattern used to make the agreement, used for pattern case only
	ServiceId                      []string `json:"service_id"`                        // All the service ids whose policy is used to make the agreement, used for policy case only
	ProtocolTimeoutS               uint64   `json:"protocol_timeout_sec"`              // Number of seconds to wait before declaring proposal response is lost
	AgreementTimeoutS              uint64   `json:"agreement_timeout_sec"`
	LastSecretUpdateTime           uint64   `json:"last_secret_update_time"`     // The secret update time corresponding to the most recent secret update protocol msg sent for this agreement
	LastSecretUpdateTimeAck        uint64   `json:"last_secret_update_time_ack"` // Will match the LastSecretUpdateTime when the agreement update ACK is received
	LastPolicyUpdateTime           uint64   `json:"last_policy_update_time"`
	LastPolicyUpdateTimeAck        uint64   `json:"last_policy_update_time_ack"`
}

func (a Agreement) String() string {
	return fmt.Sprintf("Archived: %v, "+
		"CurrentAgreementId: %v, "+
		"Org: %v, "+
		"AgreementProtocol: %v, "+
		"AgreementProtocolVersion: %v, "+
		"DeviceId: %v, "+
		"DeviceType: %v, "+
		"AgreementInceptionTime: %v, "+
		"AgreementCreationTime: %v, "+
		"AgreementFinalizedTime: %v, "+
		"AgreementTimedout: %v, "+
		"ProposalSig: %v, "+
		"ProposalHash: %v, "+
		"ConsumerProposalSig: %v, "+
		"Policy Name: %v, "+
		"CounterPartyAddress: %v, "+
		"DataVerificationURL: %v, "+
		"DataVerificationUser: %v, "+
		"DataVerificationCheckRate: %v, "+
		"DataVerificationMissedCount: %v, "+
		"DataVerificationNoDataInterval: %v, "+
		"DisableDataVerification: %v, "+
		"DataVerifiedTime: %v, "+
		"DataNotificationSent: %v, "+
		"MeteringTokens: %v, "+
		"MeteringPerTimeUnit: %v, "+
		"MeteringNotificationInterval: %v, "+
		"MeteringNotificationSent: %v, "+
		"MeteringNotificationMsgs: %v, "+
		"TerminatedReason: %v, "+
		"TerminatedDescription: %v, "+
		"BlockchainType: %v, "+
		"BlockchainName: %v, "+
		"BlockchainOrg: %v, "+
		"BCUpdateAckTime: %v, "+
		"NHMissingHBInterval: %v, "+
		"NHCheckAgreementStatus: %v, "+
		"Pattern: %v, "+
		"ServiceId: %v, "+
		"ProtocolTimeoutS: %v, "+
		"AgreementTimeoutS: %v, "+
		"LastSecretUpdateTime: %v, "+
		"LastSecretUpdateTimeAck: %v"+
		"LastPolicyUpdateTime: %v"+
		"LastPolicyUpdateTimeAck: %v",
		a.Archived, a.CurrentAgreementId, a.Org, a.AgreementProtocol, a.AgreementProtocolVersion, a.DeviceId, a.DeviceType,
		a.AgreementInceptionTime, a.AgreementCreationTime, a.AgreementFinalizedTime,
		a.AgreementTimedout, a.ProposalSig, a.ProposalHash, a.ConsumerProposalSig, a.PolicyName, a.CounterPartyAddress,
		a.DataVerificationURL, a.DataVerificationUser, a.DataVerificationCheckRate, a.DataVerificationMissedCount, a.DataVerificationNoDataInterval,
		a.DisableDataVerificationChecks, a.DataVerifiedTime, a.DataNotificationSent,
		a.MeteringTokens, a.MeteringPerTimeUnit, a.MeteringNotificationInterval, a.MeteringNotificationSent, a.MeteringNotificationMsgs,
		a.TerminatedReason, a.TerminatedDescription, a.BlockchainType, a.BlockchainName, a.BlockchainOrg, a.BCUpdateAckTime,
		a.NHMissingHBInterval, a.NHCheckAgreementStatus, a.Pattern, a.ServiceId, a.ProtocolTimeoutS, a.AgreementTimeoutS,
		a.LastSecretUpdateTime, a.LastSecretUpdateTimeAck, a.LastPolicyUpdateTime, a.LastPolicyUpdateTimeAck)
}

// Factory method for agreement w/out persistence safety.
func NewAgreement(agreementid string, org string, deviceid string, deviceType string, policyName string, bcType string, bcName string, bcOrg string, agreementProto string, pattern string, serviceId []string, nhPolicy policy.NodeHealth, protocolTimeout uint64, agreementTimeout uint64) (*Agreement, error) {
	if agreementid == "" || agreementProto == "" {
		return nil, errors.New("Illegal input: agreement id or agreement protocol is empty")
	} else {
		return &Agreement{
			CurrentAgreementId:             agreementid,
			Org:                            org,
			DeviceId:                       deviceid,
			DeviceType:                     deviceType,
			AgreementProtocol:              agreementProto,
			AgreementProtocolVersion:       0,
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
			MeteringNotificationMsgs:       []string{"", ""},
			Archived:                       false,
			TerminatedReason:               0,
			TerminatedDescription:          "",
			BlockchainType:                 bcType,
			BlockchainName:                 bcName,
			BlockchainOrg:                  bcOrg,
			BCUpdateAckTime:                0,
			NHMissingHBInterval:            nhPolicy.MissingHBInterval,
			NHCheckAgreementStatus:         nhPolicy.CheckAgreementStatus,
			Pattern:                        pattern,
			ServiceId:                      serviceId,
			ProtocolTimeoutS:               protocolTimeout,
			AgreementTimeoutS:              agreementTimeout,
		}, nil
	}
}

func (a *Agreement) NodeHealthInUse() bool {
	return a.NHMissingHBInterval != 0 || a.NHCheckAgreementStatus != 0
}

func (a *Agreement) GetDeviceType() string {
	if a.DeviceType == "" {
		return DEVICE_TYPE_DEVICE
	} else {
		return a.DeviceType
	}
}

// Functions that are up called from the agbot database implementation. This is done so that the business
// logic in each of these functions will be the same regardless of the underlying database implementation.

func AgreementUpdate(db AgbotDatabase, agreementid string, proposal string, policy string, dvPolicy policy.DataVerification, defaultCheckRate uint64, hash string, sig string, protocol string, agreementProtoVersion int) (*Agreement, error) {
	if agreement, err := db.SingleAgreementUpdate(agreementid, protocol, func(a Agreement) *Agreement {
		a.AgreementCreationTime = uint64(time.Now().Unix())
		a.Proposal = proposal
		a.ProposalHash = hash
		a.ConsumerProposalSig = sig
		a.Policy = policy
		a.AgreementProtocolVersion = agreementProtoVersion
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

func AgreementMade(db AgbotDatabase, agreementId string, counterParty string, signature string, protocol string, bcType string, bcName string, bcOrg string) (*Agreement, error) {
	if agreement, err := db.SingleAgreementUpdate(agreementId, protocol, func(a Agreement) *Agreement {
		a.CounterPartyAddress = counterParty
		a.ProposalSig = signature
		a.BlockchainType = bcType
		a.BlockchainName = bcName
		a.BlockchainOrg = bcOrg
		return &a
	}); err != nil {
		return nil, err
	} else {
		return agreement, nil
	}
}

func AgreementBlockchainUpdate(db AgbotDatabase, agreementId string, consumerSig string, hash string, counterParty string, signature string, protocol string) (*Agreement, error) {
	if agreement, err := db.SingleAgreementUpdate(agreementId, protocol, func(a Agreement) *Agreement {
		a.ConsumerProposalSig = consumerSig
		a.ProposalHash = hash
		a.CounterPartyAddress = counterParty
		a.ProposalSig = signature
		return &a
	}); err != nil {
		return nil, err
	} else {
		return agreement, nil
	}
}

func AgreementBlockchainUpdateAck(db AgbotDatabase, agreementId string, protocol string) (*Agreement, error) {
	if agreement, err := db.SingleAgreementUpdate(agreementId, protocol, func(a Agreement) *Agreement {
		a.BCUpdateAckTime = uint64(time.Now().Unix())
		return &a
	}); err != nil {
		return nil, err
	} else {
		return agreement, nil
	}
}

func AgreementFinalized(db AgbotDatabase, agreementid string, protocol string) (*Agreement, error) {
	if agreement, err := db.SingleAgreementUpdate(agreementid, protocol, func(a Agreement) *Agreement {
		a.AgreementFinalizedTime = uint64(time.Now().Unix())
		return &a
	}); err != nil {
		return nil, err
	} else {
		return agreement, nil
	}
}

func AgreementTimedout(db AgbotDatabase, agreementid string, protocol string) (*Agreement, error) {
	if agreement, err := db.SingleAgreementUpdate(agreementid, protocol, func(a Agreement) *Agreement {
		a.AgreementTimedout = uint64(time.Now().Unix())
		return &a
	}); err != nil {
		return nil, err
	} else {
		return agreement, nil
	}
}

func DataVerified(db AgbotDatabase, agreementid string, protocol string) (*Agreement, error) {
	if agreement, err := db.SingleAgreementUpdate(agreementid, protocol, func(a Agreement) *Agreement {
		a.DataVerifiedTime = uint64(time.Now().Unix())
		return &a
	}); err != nil {
		return nil, err
	} else {
		return agreement, nil
	}
}

func DataNotVerified(db AgbotDatabase, agreementid string, protocol string) (*Agreement, error) {
	if agreement, err := db.SingleAgreementUpdate(agreementid, protocol, func(a Agreement) *Agreement {
		a.DataVerificationMissedCount += 1
		return &a
	}); err != nil {
		return nil, err
	} else {
		return agreement, nil
	}
}

func DataNotification(db AgbotDatabase, agreementid string, protocol string) (*Agreement, error) {
	if agreement, err := db.SingleAgreementUpdate(agreementid, protocol, func(a Agreement) *Agreement {
		a.DataNotificationSent = uint64(time.Now().Unix())
		return &a
	}); err != nil {
		return nil, err
	} else {
		return agreement, nil
	}
}

func MeteringNotification(db AgbotDatabase, agreementid string, protocol string, mn string) (*Agreement, error) {
	if agreement, err := db.SingleAgreementUpdate(agreementid, protocol, func(a Agreement) *Agreement {
		a.MeteringNotificationSent = uint64(time.Now().Unix())
		if len(a.MeteringNotificationMsgs) == 0 {
			a.MeteringNotificationMsgs = []string{"", ""}
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

func ArchiveAgreement(db AgbotDatabase, agreementid string, protocol string, reason uint, desc string) (*Agreement, error) {
	if agreement, err := db.SingleAgreementUpdate(agreementid, protocol, func(a Agreement) *Agreement {
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

func SetAgreementTimeouts(db AgbotDatabase, agreementid string, protocol string, agreementTimeoutS uint64, protocolTimeoutS uint64) (*Agreement, error) {
	if agreement, err := db.SingleAgreementUpdate(agreementid, protocol, func(a Agreement) *Agreement {
		a.AgreementTimeoutS = agreementTimeoutS
		a.ProtocolTimeoutS = protocolTimeoutS
		return &a
	}); err != nil {
		return nil, err
	} else {
		return agreement, nil
	}
}

func AgreementSecretUpdateTime(db AgbotDatabase, agreementid string, protocol string, secretUpdateTime uint64) (*Agreement, error) {
	if agreement, err := db.SingleAgreementUpdate(agreementid, protocol, func(a Agreement) *Agreement {
		a.LastSecretUpdateTime = secretUpdateTime
		return &a
	}); err != nil {
		return nil, err
	} else {
		return agreement, nil
	}
}

func AgreementSecretUpdateAckTime(db AgbotDatabase, agreementid string, protocol string, secretUpdateAckTime uint64) (*Agreement, error) {
	if agreement, err := db.SingleAgreementUpdate(agreementid, protocol, func(a Agreement) *Agreement {
		a.LastSecretUpdateTimeAck = secretUpdateAckTime
		return &a
	}); err != nil {
		return nil, err
	} else {
		return agreement, nil
	}
}

func AgreementPolicyUpdateTime(db AgbotDatabase, agreementid string, protocol string, policyUpdateTime uint64) (*Agreement, error) {
	if agreement, err := db.SingleAgreementUpdate(agreementid, protocol, func(a Agreement) *Agreement {
		a.LastPolicyUpdateTime = policyUpdateTime
		return &a
	}); err != nil {
		return nil, err
	} else {
		return agreement, nil
	}
}

func AgreementPolicyUpdateAckTime(db AgbotDatabase, agreementid string, protocol string, policyUpdateAckTime uint64) (*Agreement, error) {
	if agreement, err := db.SingleAgreementUpdate(agreementid, protocol, func(a Agreement) *Agreement {
		a.LastPolicyUpdateTimeAck = policyUpdateAckTime
		return &a
	}); err != nil {
		return nil, err
	} else {
		return agreement, nil
	}
}

// This code is running in a database transaction. Within the tx, the current record is
// read and then updated according to the updates within the input update record. It is critical
// to check for correct data transitions within the tx .
func ValidateStateTransition(mod *Agreement, update *Agreement) {
	if mod.AgreementCreationTime == 0 { // 1 transition from zero to non-zero
		mod.AgreementCreationTime = update.AgreementCreationTime
	}
	if mod.AgreementFinalizedTime == 0 { // 1 transition from zero to non-zero
		mod.AgreementFinalizedTime = update.AgreementFinalizedTime
	}
	if mod.AgreementTimedout == 0 { // 1 transition from zero to non-zero
		mod.AgreementTimedout = update.AgreementTimedout
	}
	if mod.CounterPartyAddress == "" { // 1 transition from empty to non-empty
		mod.CounterPartyAddress = update.CounterPartyAddress
	}
	if mod.Proposal == "" { // 1 transition from empty to non-empty
		mod.Proposal = update.Proposal
	}
	if mod.ProposalHash == "" { // 1 transition from empty to non-empty
		mod.ProposalHash = update.ProposalHash
	}
	if mod.ConsumerProposalSig == "" { // 1 transition from empty to non-empty
		mod.ConsumerProposalSig = update.ConsumerProposalSig
	}
	if mod.Policy == "" { // 1 transition from empty to non-empty
		mod.Policy = update.Policy
	}
	if mod.ProposalSig == "" { // 1 transition from empty to non-empty
		mod.ProposalSig = update.ProposalSig
	}
	if mod.DataVerificationURL == "" { // 1 transition from empty to non-empty
		mod.DataVerificationURL = update.DataVerificationURL
	}
	if mod.DataVerificationUser == "" { // 1 transition from empty to non-empty
		mod.DataVerificationUser = update.DataVerificationUser
	}
	if mod.DataVerificationPW == "" { // 1 transition from empty to non-empty
		mod.DataVerificationPW = update.DataVerificationPW
	}
	if mod.DataVerificationCheckRate == 0 { // 1 transition from zero to non-zero
		mod.DataVerificationCheckRate = update.DataVerificationCheckRate
	}
	if mod.DataVerificationMissedCount < update.DataVerificationMissedCount { // Valid transitions must move forward
		mod.DataVerificationMissedCount = update.DataVerificationMissedCount
	}
	if mod.DataVerificationNoDataInterval == 0 { // 1 transition from zero to non-zero
		mod.DataVerificationNoDataInterval = update.DataVerificationNoDataInterval
	}
	if !mod.DisableDataVerificationChecks { // 1 transition from false to true
		mod.DisableDataVerificationChecks = update.DisableDataVerificationChecks
	}
	if mod.DataVerifiedTime < update.DataVerifiedTime { // Valid transitions must move forward
		mod.DataVerifiedTime = update.DataVerifiedTime
	}
	if mod.DataNotificationSent < update.DataNotificationSent { // Valid transitions must move forward
		mod.DataNotificationSent = update.DataNotificationSent
	}
	if mod.MeteringTokens == 0 { // 1 transition from zero to non-zero
		mod.MeteringTokens = update.MeteringTokens
	}
	if mod.MeteringPerTimeUnit == "" { // 1 transition from empty to non-empty
		mod.MeteringPerTimeUnit = update.MeteringPerTimeUnit
	}
	if mod.MeteringNotificationInterval == 0 { // 1 transition from zero to non-zero
		mod.MeteringNotificationInterval = update.MeteringNotificationInterval
	}
	if mod.MeteringNotificationSent < update.MeteringNotificationSent { // Valid transitions must move forward
		mod.MeteringNotificationSent = update.MeteringNotificationSent
	}
	if len(mod.MeteringNotificationMsgs) == 0 || mod.MeteringNotificationMsgs[0] == update.MeteringNotificationMsgs[1] { // msgs must move from new to old in the array
		mod.MeteringNotificationMsgs = update.MeteringNotificationMsgs
	}
	if !mod.Archived { // 1 transition from false to true
		mod.Archived = update.Archived
	}
	if mod.TerminatedReason == 0 { // 1 valid transition from zero to non-zero
		mod.TerminatedReason = update.TerminatedReason
	}
	if mod.TerminatedDescription == "" { // 1 transition from empty to non-empty
		mod.TerminatedDescription = update.TerminatedDescription
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
	if mod.AgreementProtocolVersion == 0 { // 1 transition from empty to non-empty
		mod.AgreementProtocolVersion = update.AgreementProtocolVersion
	}
	if mod.BCUpdateAckTime == 0 { // 1 transition from zero to non-zero
		mod.BCUpdateAckTime = update.BCUpdateAckTime
	}
	if mod.LastSecretUpdateTime < update.LastSecretUpdateTime { // Valid transitions must move forward
		mod.LastSecretUpdateTime = update.LastSecretUpdateTime
	}
	if mod.LastSecretUpdateTimeAck < update.LastSecretUpdateTimeAck { // Valid transitions must move forward
		mod.LastSecretUpdateTimeAck = update.LastSecretUpdateTimeAck
	}
	if mod.LastPolicyUpdateTime < update.LastPolicyUpdateTime { // Valid transitions must move forward
		mod.LastPolicyUpdateTime = update.LastPolicyUpdateTime
	}
	if mod.LastPolicyUpdateTimeAck < update.LastPolicyUpdateTimeAck { // Valid transitions must move forward
		mod.LastPolicyUpdateTimeAck = update.LastPolicyUpdateTimeAck
	}
}

// Filters used by the caller to control what comes back from the database.
type AFilter func(Agreement) bool

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

func PolAFilter(policyName string) AFilter {
        return func(a Agreement) bool { return a.PolicyName == policyName }
}

func PatAFilter(patternName string) AFilter {
        return func(a Agreement) bool { return a.Pattern == patternName }
}

func RunFilters(ag *Agreement, filters []AFilter) *Agreement {
	for _, filterFn := range filters {
		if !filterFn(*ag) {
			return nil
		}
	}
	return ag
}
