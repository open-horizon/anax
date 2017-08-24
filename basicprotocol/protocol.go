package basicprotocol

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/abstractprotocol"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/metering"
	"github.com/open-horizon/anax/policy"
	"net/http"
	"time"
)

const PROTOCOL_NAME = "Basic"
const PROTOCOL_CURRENT_VERSION = 1

// This is the object which users of the agreement protocol use to get access to the protocol functions. It MUST
// implement all the functions in the abstract ProtocolHandler interface.
type ProtocolHandler struct {
	*abstractprotocol.BaseProtocolHandler
}

func NewProtocolHandler(pm *policy.PolicyManager) *ProtocolHandler {

	bph := abstractprotocol.NewBaseProtocolHandler(PROTOCOL_NAME,
		PROTOCOL_CURRENT_VERSION,
		&http.Client{Timeout: time.Duration(config.HTTPDEFAULTTIMEOUT * time.Millisecond)},
		pm)

	return &ProtocolHandler{
		BaseProtocolHandler: bph,
	}
}

// The implementation of this protocol method has no extensions to the base abstraction.
func (p *ProtocolHandler) InitiateAgreement(agreementId string,
	producerPolicy *policy.Policy,
	originalProducerPolicy string,
	consumerPolicy *policy.Policy,
	myId string,
	messageTarget interface{},
	workload *policy.Workload,
	defaultPW string,
	defaultNoData uint64,
	sendMessage func(msgTarget interface{}, pay []byte) error) (abstractprotocol.Proposal, error) {

	if bp, err := abstractprotocol.CreateProposal(p, agreementId, producerPolicy, originalProducerPolicy, consumerPolicy, PROTOCOL_CURRENT_VERSION, myId, workload, defaultPW, defaultNoData); err != nil {
		return nil, err
	} else {

		// Send the proposal to the other party
		glog.V(5).Infof("Protocol %v sending proposal %s", p.Name(), bp)

		if err := abstractprotocol.SendProposal(p, bp, consumerPolicy, messageTarget, sendMessage); err != nil {
			return nil, err
		}
		return bp, nil
	}

}

// This is an implementation of the Decide on proposal API, it has no extensions.
func (p *ProtocolHandler) DecideOnProposal(proposal abstractprotocol.Proposal,
	myId string,
	ignore []string,
	messageTarget interface{},
	sendMessage func(mt interface{}, pay []byte) error) (abstractprotocol.ProposalReply, error) {

	reply, replyErr := abstractprotocol.DecideOnProposal(p, proposal, myId)

	// Always respond to the Proposer
	return abstractprotocol.SendResponse(p, proposal, reply, replyErr, messageTarget, sendMessage)

}

// The following methods dont implement any extensions to the base agreement protocol.
func (p *ProtocolHandler) Confirm(replyValid bool,
	agreementId string,
	messageTarget interface{},
	sendMessage func(mt interface{}, pay []byte) error) error {
	return abstractprotocol.Confirm(p, replyValid, agreementId, messageTarget, sendMessage)
}

func (p *ProtocolHandler) NotifyDataReceipt(agreementId string,
	messageTarget interface{},
	sendMessage func(mt interface{}, pay []byte) error) error {
	return abstractprotocol.NotifyDataReceipt(p, agreementId, messageTarget, sendMessage)
}

func (p *ProtocolHandler) NotifyDataReceiptAck(agreementId string,
	messageTarget interface{},
	sendMessage func(mt interface{}, pay []byte) error) error {
	return abstractprotocol.NotifyDataReceiptAck(p, agreementId, messageTarget, sendMessage)
}

func (p *ProtocolHandler) NotifyMetering(agreementId string,
	mn *metering.MeteringNotification,
	messageTarget interface{},
	sendMessage func(mt interface{}, pay []byte) error) (string, error) {

	return abstractprotocol.NotifyMeterReading(p, agreementId, mn, messageTarget, sendMessage)
}

func (p *ProtocolHandler) ValidateProposal(proposal string) (abstractprotocol.Proposal, error) {

	return abstractprotocol.ValidateProposal(proposal)
}

func (p *ProtocolHandler) ValidateReply(reply string) (abstractprotocol.ProposalReply, error) {

	return abstractprotocol.ValidateReply(reply)
}

func (p *ProtocolHandler) ValidateReplyAck(replyack string) (abstractprotocol.ReplyAck, error) {
	return abstractprotocol.ValidateReplyAck(replyack)
}

func (p *ProtocolHandler) ValidateDataReceived(dr string) (abstractprotocol.DataReceived, error) {
	return abstractprotocol.ValidateDataReceived(dr)
}

func (p *ProtocolHandler) ValidateDataReceivedAck(dra string) (abstractprotocol.DataReceivedAck, error) {
	return abstractprotocol.ValidateDataReceivedAck(dra)
}

func (p *ProtocolHandler) ValidateMeterNotification(mn string) (abstractprotocol.NotifyMetering, error) {
	return abstractprotocol.ValidateMeterNotification(mn)
}

func (p *ProtocolHandler) ValidateCancel(can string) (abstractprotocol.Cancel, error) {
	return abstractprotocol.ValidateCancel(can)
}

func (p *ProtocolHandler) DemarshalProposal(proposal string) (abstractprotocol.Proposal, error) {
	return abstractprotocol.DemarshalProposal(proposal)
}

func (p *ProtocolHandler) RecordAgreement(newProposal abstractprotocol.Proposal,
	reply abstractprotocol.ProposalReply,
	addr string,
	sig string,
	consumerPolicy *policy.Policy) error {

	// Tell the policy manager that we're in this agreement
	if cerr := abstractprotocol.RecordAgreement(p, newProposal, consumerPolicy); cerr != nil {
		glog.Errorf(fmt.Sprintf("Error finalizing agreement %v in PM %v", newProposal.AgreementId(), cerr))
	}

	return nil
}

func (p *ProtocolHandler) TerminateAgreement(policy *policy.Policy,
	counterParty string,
	agreementId string,
	reason uint,
	messageTarget interface{},
	sendMessage func(mt interface{}, pay []byte) error) error {

	if messageTarget != nil {
		cancelMsg := abstractprotocol.NewBaseCancel(PROTOCOL_NAME, PROTOCOL_CURRENT_VERSION, agreementId, reason)
		if err := abstractprotocol.SendProtocolMessage(messageTarget, cancelMsg, sendMessage); err != nil {
			glog.Errorf(fmt.Sprintf("Protocol %v error sending cancel message for agreement %v, error %v", p.Name(), agreementId, err))
		}
	}

	// Tell the policy manager that we're terminating this agreement
	if cerr := abstractprotocol.TerminateAgreement(p, policy, agreementId, reason); cerr != nil {
		glog.Errorf(fmt.Sprintf("Protocol %v error cancelling agreement %v in PM %v", p.Name(), agreementId, cerr))
	}

	return nil

}

func (p *ProtocolHandler) VerifyAgreement(agreementId string,
	counterPartyAddress string,
	expectedSignature string) (bool, error) {

	return true, nil
}

func (p *ProtocolHandler) RecordMeter(agreementId string, mn *metering.MeteringNotification) error {

	return nil

}

// constants indicating why an agreement is cancelled by the producer
// const CANCEL_NOT_FINALIZED_TIMEOUT = 100  // x64
const CANCEL_POLICY_CHANGED = 101
const CANCEL_TORRENT_FAILURE = 102
const CANCEL_CONTAINER_FAILURE = 103
const CANCEL_NOT_EXECUTED_TIMEOUT = 104
const CANCEL_USER_REQUESTED = 105
const CANCEL_AGBOT_REQUESTED = 106 // x6a
const CANCEL_NO_REPLY_ACK = 107
const CANCEL_MICROSERVICE_FAILURE = 108

// These constants represent consumer cancellation reason codes
// const AB_CANCEL_NOT_FINALIZED_TIMEOUT = 200  // xc8
const AB_CANCEL_NO_REPLY = 201
const AB_CANCEL_NEGATIVE_REPLY = 202
const AB_CANCEL_NO_DATA_RECEIVED = 203
const AB_CANCEL_POLICY_CHANGED = 204
const AB_CANCEL_DISCOVERED = 205 // xcd
const AB_USER_REQUESTED = 206
const AB_CANCEL_FORCED_UPGRADE = 207

// const AB_CANCEL_BC_WRITE_FAILED       = 208  // xd0

func DecodeReasonCode(code uint64) string {

	codeMeanings := map[uint64]string{CANCEL_POLICY_CHANGED: "producer policy changed",
		CANCEL_TORRENT_FAILURE:      "torrent failed to download",
		CANCEL_CONTAINER_FAILURE:    "workload terminated",
		CANCEL_NOT_EXECUTED_TIMEOUT: "workload start timeout",
		CANCEL_USER_REQUESTED:       "user requested",
		CANCEL_AGBOT_REQUESTED:      "agbot requested",
		CANCEL_NO_REPLY_ACK:         "agreement protocol incomplete, no reply ack received",
		CANCEL_MICROSERVICE_FAILURE: "microservice failed",
		// AB_CANCEL_NOT_FINALIZED_TIMEOUT: "agreement bot never detected agreement on the blockchain",
		AB_CANCEL_NO_REPLY:         "agreement bot never received reply to proposal",
		AB_CANCEL_NEGATIVE_REPLY:   "agreement bot received negative reply",
		AB_CANCEL_NO_DATA_RECEIVED: "agreement bot did not detect data",
		AB_CANCEL_POLICY_CHANGED:   "agreement bot policy changed",
		AB_CANCEL_DISCOVERED:       "agreement bot discovered cancellation from producer",
		AB_USER_REQUESTED:          "agreement bot user requested",
		AB_CANCEL_FORCED_UPGRADE:   "agreement bot user requested workload upgrade"}
	// AB_CANCEL_BC_WRITE_FAILED:       "agreement bot agreement write failed"}

	if reasonString, ok := codeMeanings[code]; !ok {
		return "unknown reason code, device might be downlevel"
	} else {
		return reasonString
	}
}
