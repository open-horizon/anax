package producer

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/abstractprotocol"
	"github.com/open-horizon/anax/basicprotocol"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/worker"
)

type BasicProtocolHandler struct {
	*BaseProducerProtocolHandler
	agreementPH *basicprotocol.ProtocolHandler
}

func NewBasicProtocolHandler(name string, cfg *config.HorizonConfig, db *bolt.DB, pm *policy.PolicyManager, ec exchange.ExchangeContext) *BasicProtocolHandler {
	if name == basicprotocol.PROTOCOL_NAME {
		return &BasicProtocolHandler{
			BaseProducerProtocolHandler: &BaseProducerProtocolHandler{
				name:   name,
				pm:     pm,
				db:     db,
				config: cfg,
				ec:     ec,
			},
			agreementPH: basicprotocol.NewProtocolHandler(cfg.Collaborators.HTTPClientFactory.NewHTTPClient(nil), pm),
		}
	} else {
		return nil
	}
}

func (c *BasicProtocolHandler) Initialize() {
	glog.V(5).Infof(BPHlogString(fmt.Sprintf("initializing: %v ", c)))
}

func (c *BasicProtocolHandler) String() string {
	return fmt.Sprintf("Name: %v, "+
		"DeviceId: %v, "+
		"Token: %v, "+
		"PM: %v, "+
		"DB: %v, "+
		"Agreement PH: %v",
		c.name, c.ec.GetExchangeId(), "********", c.pm, c.db, c.agreementPH)
}

func (c *BasicProtocolHandler) AgreementProtocolHandler(typeName string, name string, org string) abstractprotocol.ProtocolHandler {
	return c.agreementPH
}

func (c *BasicProtocolHandler) AcceptCommand(cmd worker.Command) bool {

	return false
}

func (c *BasicProtocolHandler) HandleProposalMessage(proposal abstractprotocol.Proposal, protocolMsg string, exchangeMsg *exchange.DeviceMessage) bool {

	if handled, reply, tcPolicy := c.HandleProposal(c.agreementPH, proposal, protocolMsg, []map[string]string{}, exchangeMsg); handled {
		if reply != nil {
			c.PersistProposal(proposal, reply, tcPolicy, protocolMsg)
		}
		return handled
	}
	return false

}

func (c *BasicProtocolHandler) HandleBlockchainEventMessage(cmd *BlockchainEventCommand) (string, bool, uint64, bool, error) {
	return "", false, 0, false, nil
}

func (c *BasicProtocolHandler) TerminateAgreement(ag *persistence.EstablishedAgreement, reason uint) {
	// Delegate to the parent implementation
	var messageTarget interface{}
	if whisperTo, pubkeyTo, err := c.BaseProducerProtocolHandler.GetAgbotMessageEndpoint(ag.ConsumerId); err != nil {
		glog.Errorf(BPHlogString(fmt.Sprintf("error obtaining message target for agreement %v cancel: %v", ag.CurrentAgreementId, err)))
	} else if mt, err := exchange.CreateMessageTarget(ag.ConsumerId, nil, pubkeyTo, whisperTo); err != nil {
		glog.Errorf(BPHlogString(fmt.Sprintf("error creating message target: %v", err)))
	} else {
		messageTarget = mt
	}
	c.BaseProducerProtocolHandler.TerminateAgreement(ag, reason, messageTarget, c)
}

func (c *BasicProtocolHandler) VerifyAgreement(ag *persistence.EstablishedAgreement) (bool, error) {
	if _, pubkey, err := c.BaseProducerProtocolHandler.GetAgbotMessageEndpoint(ag.ConsumerId); err != nil {
		return false, errors.New(BPHlogString(fmt.Sprintf("error getting agbot message target: %v", err)))
	} else if mt, err := exchange.CreateMessageTarget(ag.ConsumerId, nil, pubkey, ""); err != nil {
		return false, errors.New(BPHlogString(fmt.Sprintf("error creating message target: %v", err)))
	} else if recorded, err := c.agreementPH.VerifyAgreement(ag.CurrentAgreementId, ag.CounterPartyAddress, ag.ProposalSig, mt, c.GetSendMessage()); err != nil {
		return false, errors.New(BPHlogString(fmt.Sprintf("encountered error verifying agreement %v, error %v", ag.CurrentAgreementId, err)))
	} else {
		return recorded, nil
	}
}

// Returns 2 booleans, first is whether or not the message was handled, the second is whether or not to cancel the agreement in the protocol msg.
func (c *BasicProtocolHandler) HandleExtensionMessages(msg *events.ExchangeDeviceMessage, exchangeMsg *exchange.DeviceMessage) (bool, bool, string, error) {

	// The agreement verification reply indicates whether or not the consumer thinks the agreement is still valid.
	if verify, err := c.agreementPH.ValidateAgreementVerifyReply(msg.ProtocolMessage()); err == nil {
		glog.V(5).Infof(BPHlogString(fmt.Sprintf("extension handler handled agreement verification reply for %v", verify.AgreementId())))
		return true, !verify.Exists, verify.AgreementId(), nil

	} else if verify, err := c.agreementPH.ValidateAgreementVerify(msg.ProtocolMessage()); err == nil {
		// This is a request to verify that an agreement exists.
		exists := false
		sendReply := true
		agreements, err := persistence.FindEstablishedAgreements(c.db, c.Name(), []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(verify.AgreementId())})
		if err != nil {
			glog.Errorf(BPHlogString(fmt.Sprintf("unable to retrieve agreement %v from database, error %v", verify.AgreementId(), err)))
			sendReply = false
		} else if len(agreements) > 0 {
			// The verify is for an agreement that is active, so reply that it exists.
			exists = true
		}

		// Reply to the sender with our decision on the agreement.
		if sendReply {
			if _, pubkey, err := c.BaseProducerProtocolHandler.GetAgbotMessageEndpoint(msg.AgbotId()); err != nil {
				glog.Errorf(BPHlogString(fmt.Sprintf("error getting agbot message target: %v", err)))
			} else if mt, err := exchange.CreateMessageTarget(msg.AgbotId(), nil, pubkey, ""); err != nil {
				glog.Errorf(BPHlogString(fmt.Sprintf("error creating message target: %v", err)))
			} else if err := c.agreementPH.SendAgreementVerificationReply(verify.AgreementId(), exists, mt, c.GetSendMessage()); err != nil {
				glog.Errorf(BPHlogString(fmt.Sprintf("error sending verify response for agreement %v, error %v", verify.AgreementId(), err)))
			}
		}

		return true, false, verify.AgreementId(), nil

	} else {

		// Not a known protocol extension message. The only protocol message that is not handled in this code path is the proposal
		// so make sure it's not one of those, then we know if it's an unknown msg or not.
		if _, err := c.agreementPH.ValidateProposal(msg.ProtocolMessage()); err == nil {
			glog.V(5).Infof(BPHlogString(fmt.Sprintf("extension message handler ignoring message: %s because it is a proposal.", msg.ShortProtocolMessage())))
			return false, false, "", nil
		} else {
			glog.V(3).Infof(BPHlogString(fmt.Sprintf("extension message handler ignoring message: %s because it is not a known protocol msg.", msg.ShortProtocolMessage())))
			return true, false, "", nil
		}
	}

}

func (c *BasicProtocolHandler) GetTerminationCode(reason string) uint {
	switch reason {
	case TERM_REASON_POLICY_CHANGED:
		return basicprotocol.CANCEL_POLICY_CHANGED
	case TERM_REASON_AGBOT_REQUESTED:
		return basicprotocol.CANCEL_AGBOT_REQUESTED
	case TERM_REASON_CONTAINER_FAILURE:
		return basicprotocol.CANCEL_CONTAINER_FAILURE
	case TERM_REASON_USER_REQUESTED:
		return basicprotocol.CANCEL_USER_REQUESTED
	// case TERM_REASON_NOT_FINALIZED_TIMEOUT:
	//     return citizenscientist.CANCEL_NOT_FINALIZED_TIMEOUT
	case TERM_REASON_NO_REPLY_ACK:
		return basicprotocol.CANCEL_NO_REPLY_ACK
	case TERM_REASON_NOT_EXECUTED_TIMEOUT:
		return basicprotocol.CANCEL_NOT_EXECUTED_TIMEOUT
	case TERM_REASON_MICROSERVICE_FAILURE:
		return basicprotocol.CANCEL_MICROSERVICE_FAILURE
	case TERM_REASON_WL_IMAGE_LOAD_FAILURE:
		return basicprotocol.CANCEL_WL_IMAGE_LOAD_FAILURE
	case TERM_REASON_MS_IMAGE_LOAD_FAILURE:
		return basicprotocol.CANCEL_MS_IMAGE_LOAD_FAILURE
	case TERM_REASON_MS_IMAGE_FETCH_FAILURE:
		return basicprotocol.CANCEL_MS_IMAGE_FETCH_FAILURE
	case TERM_REASON_MS_UPGRADE_REQUIRED:
		return basicprotocol.CANCEL_MS_UPGRADE_REQUIRED
	case TERM_REASON_MS_DOWNGRADE_REQUIRED:
		return basicprotocol.CANCEL_MS_DOWNGRADE_REQUIRED
	case TERM_REASON_IMAGE_DATA_ERROR:
		return basicprotocol.CANCEL_IMAGE_DATA_ERROR
	case TERM_REASON_IMAGE_FETCH_FAILURE:
		return basicprotocol.CANCEL_IMAGE_FETCH_FAILURE
	case TERM_REASON_IMAGE_FETCH_AUTH_FAILURE:
		return basicprotocol.CANCEL_IMAGE_FETCH_AUTH_FAILURE
	case TERM_REASON_IMAGE_SIG_VERIF_FAILURE:
		return basicprotocol.CANCEL_IMAGE_SIG_VERIF_FAILURE
	case TERM_REASON_NODE_SHUTDOWN:
		return basicprotocol.CANCEL_NODE_SHUTDOWN
	case TERM_REASON_SERVICE_SUSPENDED:
		return basicprotocol.CANCEL_SERVICE_SUSPENDED
	case TERM_REASON_NODE_USERINPUT_CHANGED:
		return basicprotocol.CANCEL_NODE_USERINPUT_CHANGED
	case TERM_REASON_NODE_PATTERN_CHANGED:
		return basicprotocol.CANCEL_NODE_PATTERN_CHANGED
	default:
		return 999
	}
}

func (c *BasicProtocolHandler) GetTerminationReason(code uint) string {
	return basicprotocol.DecodeReasonCode(uint64(code))
}

func (c *BasicProtocolHandler) IsBlockchainClientAvailable(typeName string, name string, org string) bool {
	return true
}

func (c *BasicProtocolHandler) SetBlockchainWritable(cmd *BCWritableCommand) {
	return
}

func (c *BasicProtocolHandler) IsBlockchainWritable(agreement *persistence.EstablishedAgreement) bool {
	return true
}

func (c *BasicProtocolHandler) IsAgreementVerifiable(ag *persistence.EstablishedAgreement) bool {
	return true
}

// ==========================================================================================================
// Utility functions

var BPHlogString = func(v interface{}) string {
	return fmt.Sprintf("Producer Basic Protocol Handler %v", v)
}
