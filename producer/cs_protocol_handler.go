package producer

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/abstractprotocol"
	"github.com/open-horizon/anax/citizenscientist"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/worker"
	"net/http"
	"time"
)

type CSProtocolHandler struct {
	*BaseProducerProtocolHandler
	agreementPH *citizenscientist.ProtocolHandler
}

func NewCSProtocolHandler(name string, cfg *config.HorizonConfig, db *bolt.DB, pm *policy.PolicyManager, deviceId string, token string) *CSProtocolHandler {
	if name == citizenscientist.PROTOCOL_NAME {
		return &CSProtocolHandler{
			BaseProducerProtocolHandler: &BaseProducerProtocolHandler{
				name:       name,
				pm:         pm,
				db:         db,
				config:     cfg,
				deviceId:   deviceId,
				token:      token,
				httpClient: &http.Client{Timeout: time.Duration(config.HTTPDEFAULTTIMEOUT * time.Millisecond)},
			},
			agreementPH: citizenscientist.NewProtocolHandler(cfg.Edge.GethURL, pm),
		}
	} else {
		return nil
	}
}

func (c *CSProtocolHandler) Initialize() {
	glog.V(5).Infof(PPHlogString(fmt.Sprintf("initializing: %v ", c)))
}

func (c *CSProtocolHandler) String() string {
	return fmt.Sprintf("Name: %v, "+
		"DeviceId: %v, "+
		"Token: %v, "+
		"PM: %v, "+
		"DB: %v, "+
		"Agreement PH: %v",
		c.Name, c.deviceId, c.token, c.pm, c.db, c.agreementPH)
}

func (c *CSProtocolHandler) AgreementProtocolHandler() abstractprotocol.ProtocolHandler {
	return c.agreementPH
}

func (c *CSProtocolHandler) AcceptCommand(cmd worker.Command) bool {

	switch cmd.(type) {
	case *BlockchainEventCommand:
		return true
	}
	return false
}

func (c *CSProtocolHandler) HandleProposalMessage(proposal abstractprotocol.Proposal, protocolMsg string, exchangeMsg *exchange.DeviceMessage) bool {

	if handled, reply, tcPolicy := c.HandleProposal(c.agreementPH, proposal, protocolMsg, exchangeMsg); handled {
		if reply != nil {
			c.PersistProposal(proposal, reply, tcPolicy, protocolMsg)
		}
		return handled
	}
	return false

}

func (c *CSProtocolHandler) PersistProposal(p abstractprotocol.Proposal, r abstractprotocol.ProposalReply, tcPolicy *policy.Policy, protocolMsg string) {
	if reply, ok := r.(*citizenscientist.CSProposalReply); !ok {
		glog.Errorf(PPHlogString(fmt.Sprintf("unable to cast reply %v to %v Proposal Reply, is %T", r, c.Name, r)))
	} else if proposal, ok := p.(*citizenscientist.CSProposal); !ok {
		glog.Errorf(PPHlogString(fmt.Sprintf("unable to cast proposal %v to %v Proposal, is %T", p, c.Name, p)))
	} else if _, err := persistence.NewEstablishedAgreement(c.db, tcPolicy.Header.Name, proposal.AgreementId(), proposal.ConsumerId(), protocolMsg, c.Name(), proposal.Version(), tcPolicy.APISpecs[0].SpecRef, reply.Signature, proposal.Address); err != nil {
		glog.Errorf(PPHlogString(fmt.Sprintf("error persisting new agreement: %v, error: %v", proposal.AgreementId(), err)))
	}
}

func (c *CSProtocolHandler) HandleBlockchainEventMessage(cmd *BlockchainEventCommand) (string, bool, uint64, bool, error) {
	// Unmarshal the raw event
	if rawEvent, err := c.agreementPH.DemarshalEvent(cmd.Msg.RawEvent()); err != nil {
		return "", false, 0, false, errors.New(PPHlogString(fmt.Sprintf("unable to demarshal raw event %v, error: %v", cmd.Msg.RawEvent(), err)))
	} else {
		agId := c.agreementPH.GetAgreementId(rawEvent)
		if c.agreementPH.ConsumerTermination(rawEvent) {
			if reason, err := c.agreementPH.GetReasonCode(rawEvent); err != nil {
				return "", false, 0, false, errors.New(PPHlogString(fmt.Sprintf("unable to retrieve reason code from %v, error %v", rawEvent, err)))
			} else {
				return agId, true, reason, false, nil
			}
		} else if c.agreementPH.AgreementCreated(rawEvent) {
			return agId, false, 0, true, nil
		} else {
			glog.V(3).Infof(PPHlogString(fmt.Sprintf("ignoring event %v.", cmd.Msg.RawEvent())))
			return "", false, 0, false, nil
		}
	}
}

func (c *CSProtocolHandler) TerminateAgreement(ag *persistence.EstablishedAgreement, reason uint) {

	// The CS protocol doesnt send cancel messages, it depends on the blockchain to maintain the state of
	// any given agreement. This means we can fake up a message target for the TerminateAgreement call
	// because we know that the CS implementation of the agreement protocol wont be sending a message.
	fakeMT := &exchange.ExchangeMessageTarget{
			ReceiverExchangeId:     "",
			ReceiverPublicKeyObj:   nil,
			ReceiverPublicKeyBytes: []byte(""),
			ReceiverMsgEndPoint:    "",
			}

	c.BaseProducerProtocolHandler.TerminateAgreement(ag, reason, fakeMT, c)
}

func (c *CSProtocolHandler) GetTerminationCode(reason string) uint {
	switch reason {
	case TERM_REASON_POLICY_CHANGED:
		return citizenscientist.CANCEL_POLICY_CHANGED
	case TERM_REASON_AGBOT_REQUESTED:
		return citizenscientist.CANCEL_AGBOT_REQUESTED
	case TERM_REASON_CONTAINER_FAILURE:
		return citizenscientist.CANCEL_CONTAINER_FAILURE
	case TERM_REASON_TORRENT_FAILURE:
		return citizenscientist.CANCEL_TORRENT_FAILURE
	case TERM_REASON_USER_REQUESTED:
		return citizenscientist.CANCEL_USER_REQUESTED
	case TERM_REASON_NOT_FINALIZED_TIMEOUT:
		return citizenscientist.CANCEL_NOT_FINALIZED_TIMEOUT
	case TERM_REASON_NO_REPLY_ACK:
		return citizenscientist.CANCEL_NO_REPLY_ACK
	case TERM_REASON_NOT_EXECUTED_TIMEOUT:
		return citizenscientist.CANCEL_NOT_EXECUTED_TIMEOUT
	default:
		return 999
	}
}

func (c *CSProtocolHandler) GetTerminationReason(code uint) string {
	return citizenscientist.DecodeReasonCode(uint64(code))
}

// ==========================================================================================================
// Utility functions

var PPHlogString = func(v interface{}) string {
	return fmt.Sprintf("Producer CS Protocol Handler %v", v)
}
