package basicprotocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/abstractprotocol"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/metering"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"net/http"
)

const PROTOCOL_NAME = "Basic"
const PROTOCOL_CURRENT_VERSION = 1

// Protocol specific extension messages go here.

// Extended message types
const MsgTypeVerifyAgreement = "basicagreementverification"
const MsgTypeVerifyAgreementReply = "basicagreementverificationreply"
const MsgTypeUpdateAgreement = "basicagreementupdate"
const MsgTypeUpdateAgreementReply = "basicagreementupdatereply"

// This message enables a producer to ask the consumer to verify that a specific agreement still exists. If the
// consumer replies with NO (false), the producer can cancel the agreement.
type BAgreementVerify struct {
	*abstractprotocol.BaseProtocolMessage
}

func (b *BAgreementVerify) String() string {
	return b.BaseProtocolMessage.String()
}

func (b *BAgreementVerify) ShortString() string {
	return b.BaseProtocolMessage.ShortString()
}

func (b *BAgreementVerify) IsValid() bool {
	return b.BaseProtocolMessage.IsValid() && b.MsgType == MsgTypeVerifyAgreement
}

func NewBAgreementVerify(bp *abstractprotocol.BaseProtocolMessage) *BAgreementVerify {
	return &BAgreementVerify{
		BaseProtocolMessage: bp,
	}
}

// This message is the reply from the consumer confirming or denying the existence of the agreement.
type BAgreementVerifyReply struct {
	*abstractprotocol.BaseProtocolMessage
	Exists bool `json:"exists"` // whether or not the agreement id exists on that consumer.
}

func (b *BAgreementVerifyReply) String() string {
	return b.BaseProtocolMessage.String() + fmt.Sprintf(", Exists: %v", b.Exists)
}

func (b *BAgreementVerifyReply) ShortString() string {
	return b.BaseProtocolMessage.ShortString() + fmt.Sprintf(", Exists: %v", b.Exists)
}

func (b *BAgreementVerifyReply) IsValid() bool {
	return b.BaseProtocolMessage.IsValid() && b.MsgType == MsgTypeVerifyAgreementReply
}

func NewBAgreementVerifyReply(bp *abstractprotocol.BaseProtocolMessage, exists bool) *BAgreementVerifyReply {
	return &BAgreementVerifyReply{
		BaseProtocolMessage: bp,
		Exists:              exists,
	}
}

// This message enables a consumer or producer to propose an update to an existing agreement. Usually, changing an aspect
// of the agreement requires the entire agreement to be re-negotiated. Either party might reject the update. Rejection does NOT
// imply that the rejecting party is cancelling the agreement. However, the sending party is free to cancel the agreement upon
// receipt of a rejection.
const MsgUpdateTypeSecret = "basicagreementupdatesecret"

type BAgreementUpdate struct {
	*abstractprotocol.BaseProtocolMessage
	Updatetype string                 `json:"updateType"`
	Metadata   map[string]interface{} `json:"metadata,omitempty`
}

func (b *BAgreementUpdate) String() string {
	return b.BaseProtocolMessage.String() + fmt.Sprintf(", Updatetype: %v, Metadata: %v", b.UpdateType, b.Metadata)
}

func (b *BAgreementUpdate) ShortString() string {
	return b.BaseProtocolMessage.ShortString() + fmt.Sprintf(", Updatetype: %v", b.UpdateType)
}

// UpdateType is not checked here because it should be checked by the caller when deciding whether or not
// to accept the update. A caller that does not know how to handle a given UpdateType type should reject
// the update, instead of treating the update as invalid.
func (b *BAgreementUpdate) IsValid() bool {
	return b.BaseProtocolMessage.IsValid() && b.MsgType == MsgTypeUpdateAgreement
}

func (b* BAgreementUpdate) IsSecretUpdate() bool {
	return b.Updatetype == MsgUpdateTypeSecret
}

func (b* BAgreementUpdate) UpdateType() string {
	return b.Updatetype
}

func NewBAgreementUpdate(bp *abstractprotocol.BaseProtocolMessage, updateType string, metadata map[string]interface{}) *BAgreementUpdate {
	return &BAgreementUpdate{
		BaseProtocolMessage: bp,
		Updatetype: updateType,
		Metadata: metadata,
	}
}

// This message is the reply from the agreement update, accepting or rejecting the update.
type BAgreementUpdateReply struct {
	*abstractprotocol.BaseProtocolMessage
	Updatetype string `json:"updateType"`
	Accepted   bool   `json:"accepted"` // whether or not the agreement update is accepted.
}

func (b *BAgreementUpdateReply) String() string {
	return b.BaseProtocolMessage.String() + fmt.Sprintf(", UpdateType: %v, Accepted: %v", b.Updatetype, b.Accepted)
}

func (b *BAgreementUpdateReply) ShortString() string {
	return b.BaseProtocolMessage.ShortString() + fmt.Sprintf(", UpdateType: %v, Accepted: %v", b.Updatetype, b.Accepted)
}

func (b *BAgreementUpdateReply) IsValid() bool {
	return b.BaseProtocolMessage.IsValid() && b.MsgType == MsgTypeUpdateAgreementReply
}

func (b* BAgreementUpdateReply) IsSecretUpdate() bool {
	return b.Updatetype == MsgUpdateTypeSecret
}

func (b* BAgreementUpdateReply) IsAccepted() bool {
	return b.Accepted
}

func (b* BAgreementUpdateReply) UpdateType() string {
	return b.Updatetype
}

func NewBAgreementUpdateReply(bp *abstractprotocol.BaseProtocolMessage, updateType string, accepted bool) *BAgreementUpdateReply {
	return &BAgreementUpdateReply{
		BaseProtocolMessage: bp,
		Updatetype:          updateType,
		Accepted:            accepted,
	}
}

// This is the object which users of the agreement protocol use to get access to the protocol functions. It MUST
// implement all the functions in the abstract ProtocolHandler interface.
type ProtocolHandler struct {
	*abstractprotocol.BaseProtocolHandler
}

func NewProtocolHandler(httpClient *http.Client, pm *policy.PolicyManager) *ProtocolHandler {

	bph := abstractprotocol.NewBaseProtocolHandler(PROTOCOL_NAME,
		PROTOCOL_CURRENT_VERSION,
		httpClient,
		pm)

	return &ProtocolHandler{
		BaseProtocolHandler: bph,
	}
}

// The implementation of this protocol method has no extensions to the base abstraction.
func (p *ProtocolHandler) InitiateAgreement(agreementId string,
	producerPolicy *policy.Policy,
	consumerPolicy *policy.Policy,
	org string,
	myId string,
	messageTarget interface{},
	workload *policy.Workload,
	defaultPW string,
	defaultNoData uint64,
	sendMessage func(msgTarget interface{}, pay []byte) error) (abstractprotocol.Proposal, error) {

	if bp, err := abstractprotocol.CreateProposal(p, agreementId, producerPolicy, consumerPolicy, PROTOCOL_CURRENT_VERSION, myId, workload, defaultPW, defaultNoData); err != nil {
		return nil, err
	} else {

		// Send the proposal to the other party
		glog.V(5).Infof("Protocol %v sending proposal %s", p.Name(), bp)

		if err := abstractprotocol.SendProposal(p, bp, consumerPolicy, org, messageTarget, sendMessage); err != nil {
			return nil, err
		}
		return bp, nil
	}

}

// This is an implementation of the Decide on proposal API, it has no extensions.
func (p *ProtocolHandler) DecideOnProposal(proposal abstractprotocol.Proposal,
	nodePol *externalpolicy.ExternalPolicy,
	myId string,
	myOrg string,
	device *persistence.ExchangeDevice,
	ignore []map[string]string,
	messageTarget interface{},
	sendMessage func(mt interface{}, pay []byte) error) (abstractprotocol.ProposalReply, error) {

	reply, replyErr := abstractprotocol.DecideOnProposal(p, proposal, nodePol, myId, myOrg, device)

	// Always respond to the Proposer
	return abstractprotocol.SendResponse(p, proposal, reply, myOrg, replyErr, messageTarget, sendMessage)

}

// Functions to send the protocol messages which are extensions to the base protocol.
func (p *ProtocolHandler) SendAgreementVerification(
	agreementId string,
	messageTarget interface{},
	sendMessage func(mt interface{}, pay []byte) error) error {

	verify := NewBAgreementVerify(&abstractprotocol.BaseProtocolMessage{
		MsgType:   MsgTypeVerifyAgreement,
		AProtocol: p.Name(),
		AVersion:  PROTOCOL_CURRENT_VERSION,
		AgreeId:   agreementId,
	},
	)

	// Send the message
	if err := abstractprotocol.SendProtocolMessage(messageTarget, verify, sendMessage); err != nil {
		return errors.New(fmt.Sprintf("Protocol %v error sending agreement verification request %v, %v", p.Name(), verify, err))
	}
	return nil

}

func (p *ProtocolHandler) SendAgreementVerificationReply(
	agreementId string,
	exists bool,
	messageTarget interface{},
	sendMessage func(mt interface{}, pay []byte) error) error {

	verify := NewBAgreementVerifyReply(&abstractprotocol.BaseProtocolMessage{
		MsgType:   MsgTypeVerifyAgreementReply,
		AProtocol: p.Name(),
		AVersion:  PROTOCOL_CURRENT_VERSION,
		AgreeId:   agreementId,
	},
		exists)

	// Send the message
	if err := abstractprotocol.SendProtocolMessage(messageTarget, verify, sendMessage); err != nil {
		return errors.New(fmt.Sprintf("Protocol %v error sending agreement verification reply %v, %v", p.Name(), verify, err))
	}
	return nil

}

func (p *ProtocolHandler) SendAgreementUpdate(
	agreementId string,
	updateType string,
	metadata map[string]interface{},
	messageTarget interface{},
	sendMessage func(mt interface{}, pay []byte) error) error {

	update := NewBAgreementUpdate(&abstractprotocol.BaseProtocolMessage{
		MsgType:   MsgTypeUpdateAgreement,
		AProtocol: p.Name(),
		AVersion:  PROTOCOL_CURRENT_VERSION,
		AgreeId:   agreementId,
	},
		updateType,
		metadata)

	// Send the message
	if err := abstractprotocol.SendProtocolMessage(messageTarget, update, sendMessage); err != nil {
		return errors.New(fmt.Sprintf("Protocol %v error sending agreement update request %v, %v", p.Name(), update, err))
	}
	return nil

}

func (p *ProtocolHandler) SendAgreementUpdateReply(
	agreementId string,
	updateType string,
	accepted bool,
	messageTarget interface{},
	sendMessage func(mt interface{}, pay []byte) error) error {

	reply := NewBAgreementUpdateReply(&abstractprotocol.BaseProtocolMessage{
		MsgType:   MsgTypeUpdateAgreementReply,
		AProtocol: p.Name(),
		AVersion:  PROTOCOL_CURRENT_VERSION,
		AgreeId:   agreementId,
	},
		updateType,
		accepted)

	// Send the message
	if err := abstractprotocol.SendProtocolMessage(messageTarget, reply, sendMessage); err != nil {
		return errors.New(fmt.Sprintf("Protocol %v error sending agreement update reply %v, %v", p.Name(), reply, err))
	}
	return nil

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

func (p *ProtocolHandler) ValidateAgreementVerify(verify string) (*BAgreementVerify, error) {

	// attempt deserialization of message
	vObj := new(BAgreementVerify)

	if err := json.Unmarshal([]byte(verify), vObj); err != nil {
		return nil, errors.New(fmt.Sprintf("Error deserializing agreement verification request: %s, error: %v", verify, err))
	} else if !vObj.IsValid() {
		return nil, errors.New(fmt.Sprintf("Message is not an agreement verification request."))
	} else {
		return vObj, nil
	}

}

func (p *ProtocolHandler) ValidateAgreementVerifyReply(verify string) (*BAgreementVerifyReply, error) {

	// attempt deserialization of message
	vObj := new(BAgreementVerifyReply)

	if err := json.Unmarshal([]byte(verify), vObj); err != nil {
		return nil, errors.New(fmt.Sprintf("Error deserializing agreement verification reply: %s, error: %v", verify, err))
	} else if !vObj.IsValid() {
		return nil, errors.New(fmt.Sprintf("Message is not an agreement verification reply."))
	} else {
		return vObj, nil
	}

}

func (p *ProtocolHandler) ValidateUpdate(update string) (*BAgreementUpdate, error) {

	// attempt deserialization of message
	vObj := new(BAgreementUpdate)

	if err := json.Unmarshal([]byte(update), vObj); err != nil {
		return nil, errors.New(fmt.Sprintf("Error deserializing agreement update: %s, error: %v", update, err))
	} else if !vObj.IsValid() {
		return nil, errors.New(fmt.Sprintf("Message is not an agreement update."))
	} else {
		return vObj, nil
	}

}

func (p *ProtocolHandler) ValidateUpdateReply(reply string) (*BAgreementUpdateReply, error) {

	// attempt deserialization of message
	vObj := new(BAgreementUpdateReply)

	if err := json.Unmarshal([]byte(reply), vObj); err != nil {
		return nil, errors.New(fmt.Sprintf("Error deserializing agreement update reply: %s, error: %v", reply, err))
	} else if !vObj.IsValid() {
		return nil, errors.New(fmt.Sprintf("Message is not an agreement update reply."))
	} else {
		return vObj, nil
	}

}

func (p *ProtocolHandler) DemarshalProposal(proposal string) (abstractprotocol.Proposal, error) {
	return abstractprotocol.DemarshalProposal(proposal)
}

func (p *ProtocolHandler) RecordAgreement(newProposal abstractprotocol.Proposal,
	reply abstractprotocol.ProposalReply,
	addr string,
	sig string,
	consumerPolicy *policy.Policy,
	org string) error {

	// Tell the policy manager that we're in this agreement
	if cerr := abstractprotocol.RecordAgreement(p, newProposal, consumerPolicy, org); cerr != nil {
		glog.Errorf(fmt.Sprintf("Error finalizing agreement %v in PM %v", newProposal.AgreementId(), cerr))
	}

	return nil
}

func (p *ProtocolHandler) TerminateAgreement(policies []policy.Policy,
	counterParty string,
	agreementId string,
	org string,
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
	if cerr := abstractprotocol.TerminateAgreement(p, policies, agreementId, org, reason); cerr != nil {
		glog.Errorf(fmt.Sprintf("Protocol %v error cancelling agreement %v in PM %v", p.Name(), agreementId, cerr))
	}

	return nil

}

// This protocol function exploits a protocol extension to send a verification request to the consumer.
func (p *ProtocolHandler) VerifyAgreement(agreementId string,
	counterPartyAddress string,
	expectedSignature string,
	messageTarget interface{},
	sendMessage func(mt interface{}, pay []byte) error) (bool, error) {

	if messageTarget != nil {
		if err := p.SendAgreementVerification(agreementId, messageTarget, sendMessage); err != nil {
			return false, err
		}
	}

	return true, nil
}

func (p *ProtocolHandler) RecordMeter(agreementId string, mn *metering.MeteringNotification) error {

	return nil

}

// constants indicating why an agreement is cancelled by the producer
// const CANCEL_NOT_FINALIZED_TIMEOUT = 100  // x64
const CANCEL_POLICY_CHANGED = 101

const CANCEL_CONTAINER_FAILURE = 103
const CANCEL_NOT_EXECUTED_TIMEOUT = 104
const CANCEL_USER_REQUESTED = 105
const CANCEL_AGBOT_REQUESTED = 106 // x6a
const CANCEL_NO_REPLY_ACK = 107
const CANCEL_MICROSERVICE_FAILURE = 108
const CANCEL_WL_IMAGE_LOAD_FAILURE = 109
const CANCEL_MS_IMAGE_LOAD_FAILURE = 110
const CANCEL_MS_UPGRADE_REQUIRED = 111
const CANCEL_IMAGE_DATA_ERROR = 112 // x70
const CANCEL_IMAGE_FETCH_FAILURE = 113
const CANCEL_IMAGE_FETCH_AUTH_FAILURE = 114
const CANCEL_IMAGE_SIG_VERIF_FAILURE = 115
const CANCEL_NODE_SHUTDOWN = 116 // x74
const CANCEL_MS_IMAGE_FETCH_FAILURE = 117
const CANCEL_MS_DOWNGRADE_REQUIRED = 118
const CANCEL_SERVICE_SUSPENDED = 119
const CANCEL_NODE_USERINPUT_CHANGED = 120
const CANCEL_NODE_PATTERN_CHANGED = 121

// These constants represent consumer cancellation reason codes
// const AB_CANCEL_NOT_FINALIZED_TIMEOUT = 200  // xc8
const AB_CANCEL_NO_REPLY = 201
const AB_CANCEL_NEGATIVE_REPLY = 202
const AB_CANCEL_NO_DATA_RECEIVED = 203
const AB_CANCEL_POLICY_CHANGED = 204
const AB_CANCEL_DISCOVERED = 205 // xcd
const AB_USER_REQUESTED = 206
const AB_CANCEL_FORCED_UPGRADE = 207
const AB_CANCEL_NODE_HEARTBEAT = 208
const AB_CANCEL_AG_MISSING = 209

// const AB_CANCEL_BC_WRITE_FAILED       = 208  // xd0

func DecodeReasonCode(code uint64) string {

	codeMeanings := map[uint64]string{
		CANCEL_POLICY_CHANGED:           "node policy changed",
		CANCEL_CONTAINER_FAILURE:        "service terminated",
		CANCEL_NOT_EXECUTED_TIMEOUT:     "service start timeout",
		CANCEL_USER_REQUESTED:           "user requested",
		CANCEL_AGBOT_REQUESTED:          "agbot requested",
		CANCEL_NO_REPLY_ACK:             "agreement protocol incomplete, no reply ack received",
		CANCEL_MICROSERVICE_FAILURE:     "dependent service failed",
		CANCEL_WL_IMAGE_LOAD_FAILURE:    "service image loading failed",
		CANCEL_MS_IMAGE_LOAD_FAILURE:    "dependent service image loading failed",
		CANCEL_MS_IMAGE_FETCH_FAILURE:   "dependent service image fetching failed",
		CANCEL_MS_UPGRADE_REQUIRED:      "required by dependent service upgrade process",
		CANCEL_MS_DOWNGRADE_REQUIRED:    "dependent service failed, need to downgrade to lower version",
		CANCEL_IMAGE_DATA_ERROR:         "image data error",
		CANCEL_IMAGE_FETCH_FAILURE:      "image fetching failed",
		CANCEL_IMAGE_FETCH_AUTH_FAILURE: "authorization failed for image fetching",
		CANCEL_IMAGE_SIG_VERIF_FAILURE:  "image signature verification failed",
		CANCEL_NODE_SHUTDOWN:            "node was unconfigured",
		CANCEL_SERVICE_SUSPENDED:        "service suspended",
		CANCEL_NODE_USERINPUT_CHANGED:   "node user input changed",
		CANCEL_NODE_PATTERN_CHANGED:     "node pattern changed",
		// AB_CANCEL_NOT_FINALIZED_TIMEOUT: "agreement bot never detected agreement on the blockchain",
		AB_CANCEL_NO_REPLY:         "agreement bot never received reply to proposal",
		AB_CANCEL_NEGATIVE_REPLY:   "agreement bot received negative reply",
		AB_CANCEL_NO_DATA_RECEIVED: "agreement bot did not detect data",
		AB_CANCEL_POLICY_CHANGED:   "agreement bot policy changed",
		AB_CANCEL_DISCOVERED:       "agreement bot discovered cancellation from producer",
		AB_USER_REQUESTED:          "agreement bot user requested",
		AB_CANCEL_FORCED_UPGRADE:   "agreement bot user requested service upgrade",
		// AB_CANCEL_BC_WRITE_FAILED:   "agreement bot agreement write failed"}
		AB_CANCEL_NODE_HEARTBEAT: "agreement bot detected node heartbeat stopped",
		AB_CANCEL_AG_MISSING:     "agreement bot detected agreement missing from node"}

	if reasonString, ok := codeMeanings[code]; !ok {
		return "unknown reason code, device might be downlevel"
	} else {
		return reasonString
	}
}
