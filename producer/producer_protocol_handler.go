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
	"net/http"
	"time"
)

func CreateProducerPH(name string, cfg *config.HorizonConfig, db *bolt.DB, pm *policy.PolicyManager, id string, token string) ProducerProtocolHandler {
	if handler := NewCSProtocolHandler(name, cfg, db, pm, id, token); handler != nil {
		return handler
	} // Add new producer side protocol handlers here
	return nil
}

type ProducerProtocolHandler interface {
	Initialize()
	AcceptCommand(cmd worker.Command) bool
	AgreementProtocolHandler() abstractprotocol.ProtocolHandler
	HandleProposalMessage(proposal abstractprotocol.Proposal, protocolMsg string, exchangeMsg *exchange.DeviceMessage) bool
	HandleBlockchainEventMessage(cmd *BlockchainEventCommand) (string, bool, uint64, bool, error)
	GetSendMessage() func(mt interface{}, pay []byte) error
	GetTerminationCode(reason string) uint
	GetTerminationReason(code uint) string
}

type BaseProducerProtocolHandler struct {
	Name       string
	pm         *policy.PolicyManager
	db         *bolt.DB
	config     *config.HorizonConfig
	deviceId   string
	token      string
	httpClient *http.Client
}

func (w *BaseProducerProtocolHandler) GetSendMessage() func(mt interface{}, pay []byte) error {
	return w.sendMessage
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
	glog.V(3).Infof(BPPHlogString(w.Name, fmt.Sprintf("Sending exchange message to: %v, message %v", messageTarget.ReceiverExchangeId, string(pay))))

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
		targetURL := w.config.Edge.ExchangeURL + "agbots/" + messageTarget.ReceiverExchangeId + "/msgs"
		for {
			if err, tpErr := exchange.InvokeExchange(w.httpClient, "POST", targetURL, w.deviceId, w.token, pm, &resp); err != nil {
				return err
			} else if tpErr != nil {
				glog.Warningf(tpErr.Error())
				time.Sleep(10 * time.Second)
				continue
			} else {
				glog.V(5).Infof(BPPHlogString(w.Name, fmt.Sprintf("Sent message for %v to exchange.", messageTarget.ReceiverExchangeId)))
				return nil
			}
		}
	}

	return nil
}

func (w *BaseProducerProtocolHandler) HandleProposal(ph abstractprotocol.ProtocolHandler, proposal abstractprotocol.Proposal, protocolMsg string, exchangeMsg *exchange.DeviceMessage) (bool, abstractprotocol.ProposalReply, *policy.Policy) {

	handled := false

	if agAlreadyExists, err := persistence.FindEstablishedAgreements(w.db, w.Name, []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(proposal.AgreementId())}); err != nil {
		glog.Errorf(BPPHlogString(w.Name, fmt.Sprintf("unable to retrieve agreements from database, error %v", err)))
	} else if len(agAlreadyExists) != 0 {
		glog.Errorf(BPPHlogString(w.Name, fmt.Sprintf("agreement %v already exists, ignoring proposal: %v", proposal.AgreementId(), proposal.ShortString())))
		handled = true
	} else if tcPolicy, err := policy.DemarshalPolicy(proposal.TsAndCs()); err != nil {
		glog.Errorf(BPPHlogString(w.Name, fmt.Sprintf("received error demarshalling TsAndCs, %v", err)))
	} else if err := tcPolicy.Is_Self_Consistent(w.config.Edge.PublicKeyPath, w.config.UserPublicKeyPath()); err != nil {
		glog.Errorf(BPPHlogString(w.Name, fmt.Sprintf("received error checking self consistency of TsAndCs, %v", err)))
		handled = true
	} else if messageTarget, err := exchange.CreateMessageTarget(exchangeMsg.AgbotId, nil, exchangeMsg.AgbotPubKey, ""); err != nil {
		glog.Errorf(BPPHlogString(w.Name, fmt.Sprintf("error creating message target: %v", err)))
	} else {
		handled = true
		if r, err := ph.DecideOnProposal(proposal, w.deviceId, messageTarget, w.sendMessage); err != nil {
			glog.Errorf(BPPHlogString(w.Name, fmt.Sprintf("respond to proposal with error: %v", err)))
		} else {
			return handled, r, tcPolicy
		}
	}
	return handled, nil, nil

}

func (w *BaseProducerProtocolHandler) PersistProposal(proposal abstractprotocol.Proposal, reply abstractprotocol.ProposalReply, tcPolicy *policy.Policy, protocolMsg string) {
	if _, err := persistence.NewEstablishedAgreement(w.db, tcPolicy.Header.Name, proposal.AgreementId(), proposal.ConsumerId(), protocolMsg, w.Name, proposal.Version(), tcPolicy.APISpecs[0].SpecRef, "", ""); err != nil {
		glog.Errorf(BPPHlogString(w.Name, fmt.Sprintf("error persisting new agreement: %v, error: %v", proposal.AgreementId(), err)))
	}
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

// ==========================================================================================================
// Utility functions

var BPPHlogString = func(p string, v interface{}) string {
	return fmt.Sprintf("Base Producer Protocol Handler (%v): %v", p, v)
}
