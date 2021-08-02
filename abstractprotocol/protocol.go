package abstractprotocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/metering"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"net/http"
)

// Protocol message types
const MsgTypeProposal = "proposal"
const MsgTypeReply = "reply"
const MsgTypeReplyAck = "replyack"
const MsgTypeDataReceived = "dataverification"
const MsgTypeDataReceivedAck = "dataverificationack"
const MsgTypeNotifyMetering = "meteringnotification"
const MsgTypeCancel = "cancel"

// All protocol message have the following header info.
type ProtocolMessage interface {
	IsValid() bool
	String() string
	ShortString() string
	Type() string
	Protocol() string
	Version() int
	AgreementId() string
}

type BaseProtocolMessage struct {
	MsgType   string `json:"type"`
	AProtocol string `json:"protocol"`
	AVersion  int    `json:"version"`
	AgreeId   string `json:"agreementId"`
}

func (pm *BaseProtocolMessage) IsValid() bool {
	return pm.MsgType != "" && pm.AProtocol != "" && pm.AgreeId != ""
}

func (pm *BaseProtocolMessage) String() string {
	return fmt.Sprintf("Type: %v, Protocol: %v, Version: %v, AgreementId: %v", pm.MsgType, pm.AProtocol, pm.AVersion, pm.AgreeId)
}

func (pm *BaseProtocolMessage) ShortString() string {
	return pm.String()
}

func (pm *BaseProtocolMessage) Type() string {
	return pm.MsgType
}

func (pm *BaseProtocolMessage) Protocol() string {
	return pm.AProtocol
}

func (pm *BaseProtocolMessage) Version() int {
	return pm.AVersion
}

func (pm *BaseProtocolMessage) AgreementId() string {
	return pm.AgreeId
}

// Extract the agreement protocol name from stringified message
func ExtractProtocol(msg string) (string, error) {

	// attempt deserialization of message
	prop := new(BaseProtocolMessage)

	if err := json.Unmarshal([]byte(msg), prop); err != nil {
		return "", errors.New(fmt.Sprintf("error deserializing protocol msg: %s, error: %v", msg, err))
	} else {
		return prop.Protocol(), nil
	}

}

// =======================================================================================================
// Protocol Handler - This is the interface that Horizon uses to interact with agreement protocol
// implementations.
//
type ProtocolHandler interface {
	// Base protocol handler methods. These are implemented by the abstract interface.
	Name() string
	Version() int
	PolicyManager() *policy.PolicyManager
	HTTPClient() *http.Client

	// Protocol methods that the handler has to implement
	InitiateAgreement(agreementId string,
		producerPolicy *policy.Policy,
		consumerPolicy *policy.Policy,
		org string,
		myId string,
		messageTarget interface{},
		workload *policy.Workload,
		defaultPW string,
		defaultNoData uint64,
		sendMessage func(msgTarget interface{}, pay []byte) error) (Proposal, error)

	DemarshalProposal(proposal string) (Proposal, error)

	DecideOnProposal(proposal Proposal,
		nodeProposal *externalpolicy.ExternalPolicy,
		myId string,
		myOrg string,
		device *persistence.ExchangeDevice,
		runningBlockchains []map[string]string,
		messageTarget interface{},
		sendMessage func(mt interface{}, pay []byte) error) (ProposalReply, error)

	Confirm(replyValid bool,
		agreementId string,
		messageTarget interface{},
		sendMessage func(mt interface{}, pay []byte) error) error

	NotifyDataReceipt(agreementId string,
		messageTarget interface{},
		sendMessage func(mt interface{}, pay []byte) error) error

	NotifyDataReceiptAck(agreementId string,
		messageTarget interface{},
		sendMessage func(mt interface{}, pay []byte) error) error

	NotifyMetering(agreementId string,
		mn *metering.MeteringNotification,
		messageTarget interface{},
		sendMessage func(mt interface{}, pay []byte) error) (string, error)

	RecordAgreement(newProposal Proposal,
		reply ProposalReply,
		addr string,
		sig string,
		consumerPolicy *policy.Policy,
		org string) error

	TerminateAgreement(policies []policy.Policy,
		counterParty string,
		agreementId string,
		org string,
		reason uint,
		messageTarget interface{},
		sendMessage func(mt interface{}, pay []byte) error) error

	VerifyAgreement(agreementId string,
		counterParty string,
		expectedSignature string,
		messageTarget interface{},
		sendMessage func(mt interface{}, pay []byte) error) (bool, error)

	UpdateAgreement(agreementId string,
		updateType string,
		metadata interface{},
		messageTarget interface{},
		sendMessage func(mt interface{}, pay []byte) error) error

	RecordMeter(agreementId string,
		mn *metering.MeteringNotification) error

	// Protocol message validators
	ValidateProposal(proposal string) (Proposal, error)
	ValidateReply(reply string) (ProposalReply, error)
	ValidateReplyAck(replyAck string) (ReplyAck, error)
	ValidateDataReceived(dr string) (DataReceived, error)
	ValidateDataReceivedAck(dra string) (DataReceivedAck, error)
	ValidateMeterNotification(mn string) (NotifyMetering, error)
	ValidateCancel(can string) (Cancel, error)
}

type BaseProtocolHandler struct {
	name       string
	version    int
	httpClient *http.Client
	pm         *policy.PolicyManager
}

func (bp *BaseProtocolHandler) Name() string {
	return bp.name
}

func (bp *BaseProtocolHandler) Version() int {
	return bp.version
}

func (bp *BaseProtocolHandler) PolicyManager() *policy.PolicyManager {
	return bp.pm
}

func (bp *BaseProtocolHandler) HTTPClient() *http.Client {
	return bp.httpClient
}

func NewBaseProtocolHandler(n string, v int, h *http.Client, p *policy.PolicyManager) *BaseProtocolHandler {
	return &BaseProtocolHandler{
		name:       n,
		version:    v,
		httpClient: h,
		pm:         p,
	}
}

// The following methods are concrete implementations that can be used to construct a concrete protocol implementation.

// Create a proposal based on input policies and other configuration information.
func CreateProposal(p ProtocolHandler,
	agreementId string,
	producerPolicy *policy.Policy,
	consumerPolicy *policy.Policy,
	version int,
	myId string,
	workload *policy.Workload,
	defaultPW string,
	defaultNoData uint64) (*BaseProposal, error) {

	if TCPolicy, err := policy.Create_Terms_And_Conditions(producerPolicy, consumerPolicy, workload, agreementId, defaultPW, defaultNoData, version); err != nil {
		return nil, errors.New(fmt.Sprintf("Protocol %v initiation received error trying to merge policy %v and %v, error: %v", p.Name(), producerPolicy, consumerPolicy, err))
	} else {
		glog.V(5).Infof(AAPlogString(p.Name(), fmt.Sprintf("Merged Policy %v", TCPolicy)))

		if tcBytes, err := json.Marshal(TCPolicy); err != nil {
			return nil, errors.New(fmt.Sprintf("Protocol %v error marshalling TsAndCs %v, error: %v", p.Name(), *TCPolicy, err))
		} else if pBytes, err := json.Marshal(producerPolicy); err != nil {
			return nil, errors.New(fmt.Sprintf("Protocol %v error marshalling producer policy %v, error: %v", p.Name(), *producerPolicy, err))
		} else {
			return NewProposal(p.Name(), version, string(tcBytes), string(pBytes), agreementId, myId), nil
		}
	}
}

// Send the proposal to the other party.
func SendProposal(p ProtocolHandler,
	newProposal Proposal,
	consumerPolicy *policy.Policy,
	org string,
	messageTarget interface{},
	sendMessage func(msgTarget interface{}, pay []byte) error) error {

	// Tell the policy manager that we're going to attempt an agreement
	if err := p.PolicyManager().AttemptingAgreement([]policy.Policy{*consumerPolicy}, newProposal.AgreementId(), org); err != nil {
		glog.Errorf(AAPlogString(p.Name(), fmt.Sprintf("error saving agreement count: %v", err)))
	}

	// Send a message to the device to initiate the agreement protocol.
	if err := SendProtocolMessage(messageTarget, newProposal, sendMessage); err != nil {
		// Tell the policy manager that we're not attempting an agreement
		if perr := p.PolicyManager().CancelAgreement([]policy.Policy{*consumerPolicy}, newProposal.AgreementId(), org); perr != nil {
			glog.Errorf(AAPlogString(p.Name(), fmt.Sprintf("error saving agreement count: %v", perr)))
		}
		return errors.New(fmt.Sprintf("Protocol %v error sending proposal %v, %v", p.Name(), newProposal, err))
	} else {
		return nil
	}

}

// Decide to accept or reject a proposal based on whether the proposal is acceptable and agreement limits have not been hit.
func DecideOnProposal(p ProtocolHandler,
	proposal Proposal,
	nodePolicy *externalpolicy.ExternalPolicy,
	myId string,
	myOrg string,
	device *persistence.ExchangeDevice) (*BaseProposalReply, error) {

	glog.V(3).Infof(AAPlogString(p.Name(), fmt.Sprintf("Processing New proposal from %v, %v", proposal.ConsumerId(), proposal.ShortString())))
	glog.V(5).Infof(AAPlogString(p.Name(), fmt.Sprintf("New proposal: %v", proposal)))

	replyErr := error(nil)
	reply := NewProposalReply(p.Name(), proposal.Version(), proposal.AgreementId(), myId)

	var termsAndConditions, producerPolicy *policy.Policy

	// Marshal the policies in the proposal into in memory policy objects
	if tcPolicy, err := policy.DemarshalPolicy(proposal.TsAndCs()); err != nil {
		replyErr = errors.New(fmt.Sprintf("Protocol %v decide on proposal received error demarshalling TsAndCs, %v", p.Name(), err))
	} else if pPolicy, err := policy.DemarshalPolicy(proposal.ProducerPolicy()); err != nil {
		replyErr = errors.New(fmt.Sprintf("Protocol %v decide on proposal received error demarshalling Producer Policy, %v", p.Name(), err))
	} else {
		termsAndConditions = tcPolicy
		producerPolicy = pPolicy
		glog.V(3).Infof(AAPlogString(p.Name(), fmt.Sprintf("TsAndCs: %v", tcPolicy.ShortString())))
		glog.V(3).Infof(AAPlogString(p.Name(), fmt.Sprintf("Producer Policy: %v", pPolicy.ShortString())))

		// now add the node's built-in properties to the producer policy
		isCluster := device.IsEdgeCluster()
		var err1 error
		producerPolicy, err1 = addNodeBuiltInProps(producerPolicy, nodePolicy, isCluster)
		if err1 != nil {
			replyErr = errors.New(fmt.Sprintf("Protocol %v decide on proposal received error adding node built-in policy to the producer policy, %v", p.Name(), err))
		}
	}

	// Get all the local policies that make up the producer policy.
	policies, err := p.PolicyManager().GetPolicyList(myOrg, producerPolicy)
	if err != nil {
		replyErr = errors.New(fmt.Sprintf("Protocol %v decide on proposal received error getting policy list: %v", p.Name(), err))
	} else if err := p.PolicyManager().AttemptingAgreement(policies, proposal.AgreementId(), myOrg); err != nil {
		replyErr = errors.New(fmt.Sprintf("Protocol %v decide on proposal received error saving agreement count: %v", p.Name(), err))
	}

	// The consumer will send 2 policies, one is the merged policy that represents the
	// terms and conditions of the agreement. The other is a copy of my policy that he/she thinks
	// he/she is matching. Let's make sure it is one of my policies or a valid merger of my policies.
	// In the case of services, the agreement service might not have any dependent services and
	// therefore, there is no producer policy (or it is empty).
	if replyErr == nil {

		if mergedPolicy, err := p.PolicyManager().MergeAllProducers(&policies, producerPolicy); err != nil {
			replyErr = errors.New(fmt.Sprintf("Protocol %v unable to merge producer policies, error: %v", p.Name(), err))

			// Now that we successfully merged our policies, make sure that the input producer policy is compatible with
			// the result of our merge
		} else if _, err := policy.Are_Compatible_Producers(mergedPolicy, producerPolicy, uint64(producerPolicy.DataVerify.Interval)); err != nil {
			replyErr = errors.New(fmt.Sprintf("Protocol %v error verifying merged policy %v and %v, error: %v", p.Name(), mergedPolicy, producerPolicy, err))

			// And make sure we havent exceeded the maxAgreements in any of our policies.
		} else if maxedOut, err := p.PolicyManager().ReachedMaxAgreements(policies, myOrg); maxedOut {
			replyErr = errors.New(fmt.Sprintf("Protocol %v max agreements reached: %v", p.Name(), p.PolicyManager().AgreementCountString()))
		} else if err != nil {
			replyErr = errors.New(fmt.Sprintf("Protocol %v decide on proposal received error getting number of agreements, rejecting proposal: %v", p.Name(), err))

			// Now check to make sure that the merged policy is acceptable. The policy is not acceptable if the terms and conditions are not
			// compatible with the producer's policy.
		} else if err := policy.Are_Compatible(producerPolicy, termsAndConditions, nil); err != nil {
			replyErr = errors.New(fmt.Sprintf("Protocol %v decide on proposal received error, T and C policy is not compatible, rejecting proposal: %v", p.Name(), err))
		} else if err := p.PolicyManager().FinalAgreement(policies, proposal.AgreementId(), myOrg); err != nil {
			replyErr = errors.New(fmt.Sprintf("Protocol %v decide on proposal received error, unable to record agreement state in PM: %v", p.Name(), err))
		} else {
			reply.AcceptProposal()
		}

	}
	return reply, replyErr

}

// Send a reply to the proposal.
func SendResponse(p ProtocolHandler,
	proposal Proposal,
	newReply ProposalReply,
	myOrg string,
	replyErr error,
	messageTarget interface{},
	sendMessage func(mt interface{}, pay []byte) error) (ProposalReply, error) {

	if err := SendProtocolMessage(messageTarget, newReply, sendMessage); err != nil {
		newReply.DoNotAcceptProposal()
		replyErr = errors.New(fmt.Sprintf("Protocol %v decide on proposal received error trying to send proposal response, error: %v", p.Name(), err))
	}

	// Log any error that occurred along the way and return it. Make sure the policy manager counts are kept in sync.
	if replyErr != nil {
		glog.Errorf(AAPlogString(p.Name(), replyErr.Error()))
		producerPolicy, _ := policy.DemarshalPolicy(proposal.ProducerPolicy())
		if producerPolicy != nil {
			if policies, err := p.PolicyManager().GetPolicyList(myOrg, producerPolicy); err != nil {
				glog.Errorf(AAPlogString(p.Name(), fmt.Sprintf("Error getting policy list: %v for agreement %v", err, proposal.AgreementId())))
			} else if cerr := p.PolicyManager().CancelAgreement(policies, proposal.AgreementId(), myOrg); cerr != nil {
				glog.Errorf(AAPlogString(p.Name(), fmt.Sprintf("Error cancelling agreement %v in PM %v", proposal.AgreementId(), cerr)))
			}
		}
		return nil, replyErr
	}

	return newReply, nil

}

// Confirm a reply from a producer.
func Confirm(p ProtocolHandler,
	replyValid bool,
	agreementId string,
	messageTarget interface{},
	sendMessage func(mt interface{}, pay []byte) error) error {

	ra := NewReplyAck(p.Name(), p.Version(), replyValid, agreementId)
	return SendProtocolMessage(messageTarget, ra, sendMessage)

}

// Notify a producer that data was detected by the consumer at the data ingest.
func NotifyDataReceipt(p ProtocolHandler,
	agreementId string,
	messageTarget interface{},
	sendMessage func(mt interface{}, pay []byte) error) error {

	dr := NewDataReceived(p.Name(), p.Version(), agreementId)
	return SendProtocolMessage(messageTarget, dr, sendMessage)

}

// Confirm that a DataReceived message as received.
func NotifyDataReceiptAck(p ProtocolHandler,
	agreementId string,
	messageTarget interface{},
	sendMessage func(mt interface{}, pay []byte) error) error {

	dra := NewDataReceivedAck(p.Name(), p.Version(), agreementId)
	return SendProtocolMessage(messageTarget, dra, sendMessage)

}

// Send a metering notification to the producer.
func NotifyMeterReading(p ProtocolHandler,
	agreementId string,
	mn *metering.MeteringNotification,
	messageTarget interface{},
	sendMessage func(mt interface{}, pay []byte) error) (string, error) {

	if pay, err := json.Marshal(mn); err != nil {
		return "", errors.New(fmt.Sprintf("Unable to serialize payload %v, error: %v", mn, err))
	} else {
		nm := NewNotifyMetering(p.Name(), p.Version(), agreementId, string(pay))
		return string(pay), SendProtocolMessage(messageTarget, nm, sendMessage)
	}

}

// Send a message containing the proposal.
func SendProtocolMessage(messageTarget interface{},
	msg interface{},
	sendMessage func(mt interface{}, pay []byte) error) error {

	pay, err := json.Marshal(msg)
	if err != nil {
		return errors.New(fmt.Sprintf("unable to serialize payload %v, error: %v", msg, err))
	} else if err := sendMessage(messageTarget, pay); err != nil {
		return errors.New(fmt.Sprintf("error sending message %v, error: %v", msg, err))
	}

	return nil
}

func RecordAgreement(p ProtocolHandler,
	newProposal Proposal,
	consumerPolicy *policy.Policy,
	org string) error {

	// Tell the policy manager that we're in this agreement
	return p.PolicyManager().FinalAgreement([]policy.Policy{*consumerPolicy}, newProposal.AgreementId(), org)

}

func TerminateAgreement(p ProtocolHandler,
	policies []policy.Policy,
	agreementId string,
	org string,
	reason uint) error {

	// Tell the policy manager that we're terminating this agreement
	return p.PolicyManager().CancelAgreement(policies, agreementId, org)

}

// Validate that the input string is a proposal message.
func ValidateProposal(proposal string) (Proposal, error) {

	// attempt deserialization of message
	prop := new(BaseProposal)

	if err := json.Unmarshal([]byte(proposal), prop); err != nil {
		return nil, errors.New(fmt.Sprintf("Error deserializing proposal: %s, error: %v", proposal, err))
	} else if !prop.IsValid() {
		return nil, errors.New(fmt.Sprintf("Message is not a Proposal."))
	} else {
		return prop, nil
	}

}

func ValidateReply(replyMsg string) (ProposalReply, error) {

	// attempt deserialization of message from msg payload
	reply := new(BaseProposalReply)

	if err := json.Unmarshal([]byte(replyMsg), reply); err != nil {
		return nil, errors.New(fmt.Sprintf("Error deserializing reply: %s, error: %v", replyMsg, err))
	} else if reply.IsValid() {
		return reply, nil
	} else {
		return nil, errors.New(fmt.Sprintf("Message is not a Proposal Reply."))
	}

}

func ValidateReplyAck(replyAckMsg string) (ReplyAck, error) {

	// attempt deserialization of message from msg payload
	replyAck := new(BaseReplyAck)

	if err := json.Unmarshal([]byte(replyAckMsg), replyAck); err != nil {
		return nil, errors.New(fmt.Sprintf("Error deserializing reply ack: %s, error: %v", replyAckMsg, err))
	} else if replyAck.IsValid() {
		return replyAck, nil
	} else {
		return nil, errors.New(fmt.Sprintf("Message is not a Proposal Reply Ack."))
	}

}

func ValidateDataReceived(dr string) (DataReceived, error) {

	// attempt deserialization of message from msg payload
	dataReceived := new(BaseDataReceived)

	if err := json.Unmarshal([]byte(dr), dataReceived); err != nil {
		return nil, errors.New(fmt.Sprintf("Error deserializing data received notification: %s, error: %v", dr, err))
	} else if dataReceived.IsValid() {
		return dataReceived, nil
	} else {
		return nil, errors.New(fmt.Sprintf("Message is not a Data Received Notification."))
	}

}

func ValidateDataReceivedAck(dra string) (DataReceivedAck, error) {

	// attempt deserialization of message from msg payload
	dataReceivedAck := new(BaseDataReceivedAck)

	if err := json.Unmarshal([]byte(dra), dataReceivedAck); err != nil {
		return nil, errors.New(fmt.Sprintf("Error deserializing data received notification ack: %s, error: %v", dra, err))
	} else if dataReceivedAck.IsValid() {
		return dataReceivedAck, nil
	} else {
		return nil, errors.New(fmt.Sprintf("Message is not a Data Received Notification Ack."))
	}

}

func ValidateMeterNotification(mn string) (NotifyMetering, error) {

	// attempt deserialization of message from msg payload
	nm := new(BaseNotifyMetering)

	if err := json.Unmarshal([]byte(mn), nm); err != nil {
		return nil, errors.New(fmt.Sprintf("Error deserializing metering notification: %s, error: %v", mn, err))
	} else if nm.IsValid() {
		return nm, nil
	} else {
		return nil, errors.New(fmt.Sprintf("Message is not a Metering Notification."))
	}

}

func ValidateCancel(can string) (Cancel, error) {

	// attempt deserialization of message from msg payload
	c := new(BaseCancel)

	if err := json.Unmarshal([]byte(can), c); err != nil {
		return nil, errors.New(fmt.Sprintf("Error deserializing cancel: %s, error: %v", can, err))
	} else if c.IsValid() {
		return c, nil
	} else {
		return nil, errors.New(fmt.Sprintf("Message is not a Cancel."))
	}

}

func DemarshalProposal(proposal string) (Proposal, error) {

	// attempt deserialization of the proposal
	prop := new(BaseProposal)

	if err := json.Unmarshal([]byte(proposal), &prop); err != nil {
		return nil, errors.New(fmt.Sprintf("Error deserializing proposal: %s, error: %v", proposal, err))
	} else {
		return prop, nil
	}

}

func MarshalProposal(prop Proposal) (string, error) {
	// attempt serialization of the proposal
	if propString, err := json.Marshal(prop); err != nil {
		return "", err
	} else {
		return string(propString), nil
	}
}

func ObscureProposalSecret(proposal string) (string, error) {
	if proposal != "" {
		if prop, err := DemarshalProposal(proposal); err != nil {
			return "", err
		} else if prop.(*BaseProposal).TsandCs, err = policy.ObscureSecretDetails(prop.(*BaseProposal).TsandCs); err != nil {
			return "", err
		} else if proposal, err = MarshalProposal(prop); err != nil {
			return "", err
		}
	}
	return proposal, nil
}

// Adds node built-in properties to the producer policy.
// It will get node's CPU count, available memory and arch and add them to
// the producer policy that was used to make the proposal on agbot.
func addNodeBuiltInProps(pol *policy.Policy, nodePol *externalpolicy.ExternalPolicy, isCluster bool) (*policy.Policy, error) {
	if pol == nil {
		return nil, nil
	}

	// get built-in node properties and replace the ones in the policy,
	// the memory will be the available memory instead of total memory -- not yet,
	// still use total memory, change first parameter to true if you want available memory
	builtinNodePol, readWriteBuiltinNodePol := externalpolicy.CreateNodeBuiltInPolicy(false, true, nodePol, isCluster)
	for _, prop := range builtinNodePol.Properties {
		if err := pol.Add_Property(&prop, true); err != nil {
			return nil, err
		}
	}
	for _, prop := range readWriteBuiltinNodePol.Properties {
		if !pol.Properties.HasProperty(prop.Name) {
			if err := pol.Add_Property(&prop, false); err != nil {
				return nil, err
			}
		}
	}

	return pol, nil
}

var AAPlogString = func(p string, v interface{}) string {
	return fmt.Sprintf("AbstractProtocol (%v): %v", p, v)
}
