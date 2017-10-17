package producer

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/abstractprotocol"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/worker"
	"time"
)

func CreateProducerPH(name string, cfg *config.HorizonConfig, db *bolt.DB, pm *policy.PolicyManager, id string, token string) ProducerProtocolHandler {
	if handler := NewCSProtocolHandler(name, cfg, db, pm, id, token); handler != nil {
		return handler
	} else if handler := NewBasicProtocolHandler(name, cfg, db, pm, id, token); handler != nil {
		return handler
	} // Add new producer side protocol handlers here
	return nil
}

type ProducerProtocolHandler interface {
	Initialize()
	Name() string
	AcceptCommand(cmd worker.Command) bool
	AgreementProtocolHandler(typeName string, name string, org string) abstractprotocol.ProtocolHandler
	HandleProposalMessage(proposal abstractprotocol.Proposal, protocolMsg string, exchangeMsg *exchange.DeviceMessage) bool
	HandleBlockchainEventMessage(cmd *BlockchainEventCommand) (string, bool, uint64, bool, error)
	TerminateAgreement(agreement *persistence.EstablishedAgreement, reason uint)
	GetSendMessage() func(mt interface{}, pay []byte) error
	GetTerminationCode(reason string) uint
	GetTerminationReason(code uint) string
	SetBlockchainClientAvailable(cmd *BCInitializedCommand)
	SetBlockchainClientNotAvailable(cmd *BCStoppingCommand)
	IsBlockchainClientAvailable(typeName string, name string, org string) bool
	SetBlockchainWritable(cmd *BCWritableCommand)
	IsBlockchainWritable(agreement *persistence.EstablishedAgreement) bool
	IsAgreementVerifiable(agreement *persistence.EstablishedAgreement) bool
	HandleExtensionMessages(msg *events.ExchangeDeviceMessage, exchangeMsg *exchange.DeviceMessage) (bool, bool, string, error)
	UpdateConsumer(ag *persistence.EstablishedAgreement)
	UpdateConsumers()
	GetKnownBlockchain(ag *persistence.EstablishedAgreement) (string, string, string)
	VerifyAgreement(ag *persistence.EstablishedAgreement) (bool, error)
}

type BaseProducerProtocolHandler struct {
	name     string
	pm       *policy.PolicyManager
	db       *bolt.DB
	config   *config.HorizonConfig
	deviceId string
	token    string
}

func (w *BaseProducerProtocolHandler) GetSendMessage() func(mt interface{}, pay []byte) error {
	return w.sendMessage
}

func (w *BaseProducerProtocolHandler) Name() string {
	return w.name
}

func (w *BaseProducerProtocolHandler) sendMessage(mt interface{}, pay []byte) error {
	// The mt parameter is an abstract message target object that is passed to this routine
	// by the agreement protocol. It's an interface{} type so that we can avoid the protocol knowing
	// about non protocol types.

	var messageTarget *exchange.ExchangeMessageTarget
	switch mt.(type) {
	case *exchange.ExchangeMessageTarget:
		messageTarget = mt.(*exchange.ExchangeMessageTarget)
	default:
		return errors.New(fmt.Sprintf("input message target is %T, expecting exchange.MessageTarget", mt))
	}

	// Grab the exchange ID of the message receiver
	glog.V(3).Infof(BPPHlogString(w.Name(), fmt.Sprintf("Sending exchange message to: %v, message %v", messageTarget.ReceiverExchangeId, string(pay))))

	// Get my own keys
	myPubKey, myPrivKey, _ := exchange.GetKeys("")

	// Demarshal the receiver's public key if we need to
	if messageTarget.ReceiverPublicKeyObj == nil {
		if mtpk, err := exchange.DemarshalPublicKey(messageTarget.ReceiverPublicKeyBytes); err != nil {
			return errors.New(fmt.Sprintf("Unable to demarshal device's public key %x, error %v", messageTarget.ReceiverPublicKeyBytes, err))
		} else {
			messageTarget.ReceiverPublicKeyObj = mtpk
		}
	}

	// Create an encrypted message
	if encryptedMsg, err := exchange.ConstructExchangeMessage(pay, myPubKey, myPrivKey, messageTarget.ReceiverPublicKeyObj); err != nil {
		return errors.New(fmt.Sprintf("Unable to construct encrypted message from %v, error %v", pay, err))
		// Marshal it into a byte array
	} else if msgBody, err := json.Marshal(encryptedMsg); err != nil {
		return errors.New(fmt.Sprintf("Unable to marshal exchange message %v, error %v", encryptedMsg, err))
		// Send it to the device's message queue
	} else {
		pm := exchange.CreatePostMessage(msgBody, w.config.Edge.ExchangeMessageTTL)
		var resp interface{}
		resp = new(exchange.PostDeviceResponse)
		targetURL := w.config.Edge.ExchangeURL + "orgs/" + exchange.GetOrg(messageTarget.ReceiverExchangeId) + "/agbots/" + exchange.GetId(messageTarget.ReceiverExchangeId) + "/msgs"
		for {
			if err, tpErr := exchange.InvokeExchange(w.config.Collaborators.HTTPClientFactory.NewHTTPClient(nil), "POST", targetURL, w.deviceId, w.token, pm, &resp); err != nil {
				return err
			} else if tpErr != nil {
				glog.Warningf(tpErr.Error())
				time.Sleep(10 * time.Second)
				continue
			} else {
				glog.V(5).Infof(BPPHlogString(w.Name(), fmt.Sprintf("Sent message for %v to exchange.", messageTarget.ReceiverExchangeId)))
				return nil
			}
		}
	}
}

func (w *BaseProducerProtocolHandler) GetWorkloadResolver() func(wURL string, wOrg string, wVersion string, wArch string) (*policy.APISpecList, error) {
	return w.workloadResolver
}

func (w *BaseProducerProtocolHandler) workloadResolver(wURL string, wOrg string, wVersion string, wArch string) (*policy.APISpecList, error) {

	asl, _, err := exchange.WorkloadResolver(w.config.Collaborators.HTTPClientFactory, wURL, wOrg, wVersion, wArch, w.config.Edge.ExchangeURL, w.deviceId, w.token)
	if err != nil {
		glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("unable to resolve workload, error %v", err)))
	}
	return asl, err
}

func (w *BaseProducerProtocolHandler) HandleProposal(ph abstractprotocol.ProtocolHandler, proposal abstractprotocol.Proposal, protocolMsg string, runningBCs []map[string]string, exchangeMsg *exchange.DeviceMessage) (bool, abstractprotocol.ProposalReply, *policy.Policy) {

	handled := false

	if agAlreadyExists, err := persistence.FindEstablishedAgreements(w.db, w.Name(), []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(proposal.AgreementId())}); err != nil {
		glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("unable to retrieve agreements from database, error %v", err)))
	} else if len(agAlreadyExists) != 0 {
		glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("agreement %v already exists, ignoring proposal: %v", proposal.AgreementId(), proposal.ShortString())))
		handled = true
	} else if tcPolicy, err := policy.DemarshalPolicy(proposal.TsAndCs()); err != nil {
		glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("received error demarshalling TsAndCs, %v", err)))
	} else if err := tcPolicy.Is_Self_Consistent(w.config.Edge.PublicKeyPath, w.config.UserPublicKeyPath(), w.GetWorkloadResolver()); err != nil {
		glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("received error checking self consistency of TsAndCs, %v", err)))
		handled = true
	} else if found, err := w.FindAgreementWithSameWorkload(ph, tcPolicy.Header.Name); err != nil {
		glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("error finding agreement with TsAndCs name '%v', error %v", tcPolicy.Header.Name, err)))
		handled = true
	} else if found {
		glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("agreement with TsAndCs name '%v' exists, ignoring proposal: %v", tcPolicy.Header.Name, proposal.ShortString())))
		handled = true
	} else if messageTarget, err := exchange.CreateMessageTarget(exchangeMsg.AgbotId, nil, exchangeMsg.AgbotPubKey, ""); err != nil {
		glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("error creating message target: %v", err)))
	} else {
		handled = true
		if r, err := ph.DecideOnProposal(proposal, w.deviceId, exchange.GetOrg(w.deviceId), runningBCs, messageTarget, w.sendMessage); err != nil {
			glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("respond to proposal with error: %v", err)))
		} else {
			return handled, r, tcPolicy
		}
	}
	return handled, nil, nil

}

// Check if there are current unarchived agreements that have the same workload.
func (w *BaseProducerProtocolHandler) FindAgreementWithSameWorkload(ph abstractprotocol.ProtocolHandler, tcpol_name string) (bool, error) {

	notTerminated := func() persistence.EAFilter {
		return func(a persistence.EstablishedAgreement) bool {
			return a.AgreementTerminatedTime == 0
		}
	}

	if ags, err := persistence.FindEstablishedAgreements(w.db, w.Name(), []persistence.EAFilter{notTerminated(), persistence.UnarchivedEAFilter()}); err != nil {
		return false, fmt.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("error retrieving unarchived agreements from db: %v", err)))
	} else {
		for _, ag := range ags {
			if proposal, err := ph.DemarshalProposal(ag.Proposal); err != nil {
				return false, fmt.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("error demarshalling agreement %v proposal: %v", ag, err)))
			} else if tcPolicy, err := policy.DemarshalPolicy(proposal.TsAndCs()); err != nil {
				return false, fmt.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("error demarshalling agreement %v Producer Policy: %v", ag.CurrentAgreementId, err)))
			} else if tcPolicy.Header.Name == tcpol_name {
				return true, nil
			}
		}
	}

	return false, nil
}

func (w *BaseProducerProtocolHandler) PersistProposal(proposal abstractprotocol.Proposal, reply abstractprotocol.ProposalReply, tcPolicy *policy.Policy, protocolMsg string) {
	if _, err := persistence.NewEstablishedAgreement(w.db, tcPolicy.Header.Name, proposal.AgreementId(), proposal.ConsumerId(), protocolMsg, w.Name(), proposal.Version(), (&tcPolicy.APISpecs).AsStringArray(), "", proposal.ConsumerId(), "", "", ""); err != nil {
		glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("error persisting new agreement: %v, error: %v", proposal.AgreementId(), err)))
	}
}

func (w *BaseProducerProtocolHandler) TerminateAgreement(ag *persistence.EstablishedAgreement, reason uint, mt interface{}, pph ProducerProtocolHandler) {
	if proposal, err := pph.AgreementProtocolHandler("", "", "").DemarshalProposal(ag.Proposal); err != nil {
		glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("error demarshalling agreement %v proposal: %v", ag.CurrentAgreementId, err)))
	} else if pPolicy, err := policy.DemarshalPolicy(proposal.ProducerPolicy()); err != nil {
		glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("error demarshalling agreement %v Producer Policy: %v", ag.CurrentAgreementId, err)))
	} else {
		bcType, bcName, bcOrg := pph.GetKnownBlockchain(ag)
		if aph := pph.AgreementProtocolHandler(bcType, bcName, bcOrg); aph == nil {
			glog.Warningf(BPPHlogString(w.Name(), fmt.Sprintf("cannot terminate agreement %v, agreement protocol handler doesnt exist yet.", ag.CurrentAgreementId)))
		} else if policies, err := w.pm.GetPolicyList(pPolicy); err != nil {
			glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("agreement %v error getting policy list: %v", ag.CurrentAgreementId, err)))
		} else if err := aph.TerminateAgreement(policies, ag.CounterPartyAddress, ag.CurrentAgreementId, exchange.GetOrg(w.deviceId), reason, mt, pph.GetSendMessage()); err != nil {
			glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf("error terminating agreement %v on the blockchain: %v", ag.CurrentAgreementId, err)))
		}
	}
}

func (w *BaseProducerProtocolHandler) GetAgbotMessageEndpoint(agbotId string) (string, []byte, error) {

	glog.V(5).Infof(BPPHlogString(w.Name(), fmt.Sprintf("retrieving agbot %v msg endpoint from exchange", agbotId)))

	if ag, err := w.getAgbot(agbotId, w.config.Edge.ExchangeURL, w.deviceId, w.token); err != nil {
		return "", nil, err
	} else {
		glog.V(5).Infof(BPPHlogString(w.Name(), fmt.Sprintf("retrieved agbot %v msg endpoint from exchange %v", agbotId, ag.MsgEndPoint)))
		return ag.MsgEndPoint, ag.PublicKey, nil
	}

}

func (w *BaseProducerProtocolHandler) getAgbot(agbotId string, url string, deviceId string, token string) (*exchange.Agbot, error) {

	glog.V(5).Infof(BPPHlogString(w.Name(), fmt.Sprintf("retrieving agbot %v from exchange", agbotId)))

	var resp interface{}
	resp = new(exchange.GetAgbotsResponse)
	targetURL := url + "orgs/" + exchange.GetOrg(agbotId) + "/agbots/" + exchange.GetId(agbotId)
	for {
		if err, tpErr := exchange.InvokeExchange(w.config.Collaborators.HTTPClientFactory.NewHTTPClient(nil), "GET", targetURL, deviceId, token, nil, &resp); err != nil {
			glog.Errorf(BPPHlogString(w.Name(), fmt.Sprintf(err.Error())))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(BPPHlogString(w.Name(), tpErr.Error()))
			time.Sleep(10 * time.Second)
			continue
		} else {
			ags := resp.(*exchange.GetAgbotsResponse).Agbots
			if ag, there := ags[agbotId]; !there {
				return nil, errors.New(fmt.Sprintf("agbot %v not in GET response %v as expected", agbotId, ags))
			} else {
				glog.V(5).Infof(BPPHlogString(w.Name(), fmt.Sprintf("retrieved agbot %v from exchange %v", agbotId, ag)))
				return &ag, nil
			}
		}
	}

}

func (b *BaseProducerProtocolHandler) HandleExtensionMessages(msg *events.ExchangeDeviceMessage, exchangeMsg *exchange.DeviceMessage) (bool, bool, string, error) {
	return false, false, "", nil
}

func (b *BaseProducerProtocolHandler) UpdateConsumer(ag *persistence.EstablishedAgreement) {}

func (b *BaseProducerProtocolHandler) UpdateConsumers() {}

func (c *BaseProducerProtocolHandler) SetBlockchainClientAvailable(cmd *BCInitializedCommand) {
	return
}

func (c *BaseProducerProtocolHandler) SetBlockchainClientNotAvailable(cmd *BCStoppingCommand) {
	return
}

func (c *BaseProducerProtocolHandler) GetKnownBlockchain(ag *persistence.EstablishedAgreement) (string, string, string) {
	return "", "", ""
}

// The list of termination reasons that should be supported by all agreement protocols. The caller can pass these into
// the GetTerminationCode API to get a protocol specific reason code for that termination reason.
const TERM_REASON_POLICY_CHANGED = "PolicyChanged"
const TERM_REASON_AGBOT_REQUESTED = "ConsumerCancelled"
const TERM_REASON_CONTAINER_FAILURE = "ContainerFailure"
const TERM_REASON_TORRENT_FAILURE = "TorrentFailure"
const TERM_REASON_USER_REQUESTED = "UserRequested"
const TERM_REASON_NOT_FINALIZED_TIMEOUT = "NotFinalized"
const TERM_REASON_NO_REPLY_ACK = "NoReplyAck"
const TERM_REASON_NOT_EXECUTED_TIMEOUT = "NotExecuted"
const TERM_REASON_MICROSERVICE_FAILURE = "MicroserviceFailure"

// ==============================================================================================================
type ExchangeMessageCommand struct {
	Msg events.ExchangeDeviceMessage
}

func (e ExchangeMessageCommand) ShortString() string {
	return e.Msg.ShortString()
}

func NewExchangeMessageCommand(msg events.ExchangeDeviceMessage) *ExchangeMessageCommand {
	return &ExchangeMessageCommand{
		Msg: msg,
	}
}

// ==============================================================================================================
type BlockchainEventCommand struct {
	Msg events.EthBlockchainEventMessage
}

func (e BlockchainEventCommand) ShortString() string {
	return e.Msg.ShortString()
}

func NewBlockchainEventCommand(msg events.EthBlockchainEventMessage) *BlockchainEventCommand {
	return &BlockchainEventCommand{
		Msg: msg,
	}
}

// ==============================================================================================================
type BCInitializedCommand struct {
	Msg *events.BlockchainClientInitializedMessage
}

func (c BCInitializedCommand) ShortString() string {

	return fmt.Sprintf("BCInitializedCommand: Msg %v", c.Msg)
}

func NewBCInitializedCommand(msg *events.BlockchainClientInitializedMessage) *BCInitializedCommand {
	return &BCInitializedCommand{
		Msg: msg,
	}
}

// ==============================================================================================================
type BCStoppingCommand struct {
	Msg *events.BlockchainClientStoppingMessage
}

func (c BCStoppingCommand) ShortString() string {

	return fmt.Sprintf("BCStoppingCommand: Msg %v", c.Msg)
}

func NewBCStoppingCommand(msg *events.BlockchainClientStoppingMessage) *BCStoppingCommand {
	return &BCStoppingCommand{
		Msg: msg,
	}
}

// ==============================================================================================================
type BCWritableCommand struct {
	Msg events.AccountFundedMessage
}

func (c BCWritableCommand) ShortString() string {

	return fmt.Sprintf("BCWritableCommand: Msg %v", c.Msg)
}

func NewBCWritableCommand(msg *events.AccountFundedMessage) *BCWritableCommand {
	return &BCWritableCommand{
		Msg: *msg,
	}
}

// ==========================================================================================================
// Utility functions

var BPPHlogString = func(p string, v interface{}) string {
	return fmt.Sprintf("Base Producer Protocol Handler (%v): %v", p, v)
}
