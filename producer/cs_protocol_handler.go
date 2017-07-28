package producer

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/abstractprotocol"
	"github.com/open-horizon/anax/citizenscientist"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/worker"
	"net/http"
	"os"
	"time"
)

type BlockchainState struct {
	ready       bool   // the blockchain is ready
	writable    bool   // the blockchain is writable
	service     string // the name of the network alias used to contact the container
	servicePort string // the port of the network alias used to contact the container
	colonusDir  string // the anax side filesystem location for this BC instance
	agreementPH *citizenscientist.ProtocolHandler
}


type CSProtocolHandler struct {
	*BaseProducerProtocolHandler
	genericAgreementPH *citizenscientist.ProtocolHandler
	bcState     map[string]map[string]*BlockchainState
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
			genericAgreementPH: citizenscientist.NewProtocolHandler(pm),
			bcState:     make(map[string]map[string]*BlockchainState),
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
		"Generic Agreement PH: %v",
		c.name, c.deviceId, c.token, c.pm, c.db, c.genericAgreementPH)
}

func (c *CSProtocolHandler) AgreementProtocolHandler(typeName string, name string) abstractprotocol.ProtocolHandler {
	if typeName == "" && name == "" {
		return c.genericAgreementPH
	}

	nameMap := c.getBCNameMap(typeName)
	namedBC, ok := nameMap[name]
	if ok && namedBC.ready {
		return namedBC.agreementPH
	}
	return nil

}

func (c *CSProtocolHandler) AcceptCommand(cmd worker.Command) bool {

	switch cmd.(type) {
	case *BlockchainEventCommand:
		bcc := cmd.(*BlockchainEventCommand)
		if c.IsBlockchainClientAvailable(policy.Ethereum_bc, bcc.Msg.Name()) {
			return true
		} else {
			return false
		}
	}
	return false
}

func (c *CSProtocolHandler) HandleProposalMessage(proposal abstractprotocol.Proposal, protocolMsg string, exchangeMsg *exchange.DeviceMessage) bool {

	if proposal.Version() != citizenscientist.PROTOCOL_CURRENT_VERSION {
		// Discard the proposal until we get one at the current protocol version
		return true
	}

	// Grab the list of running BCs that we know about
	runningBCs := make([]string, 0, 5)
	ethBCs := c.getBCNameMap(policy.Ethereum_bc)
	for name, bc := range ethBCs {
		if bc.ready {
			runningBCs = append(runningBCs, name)
		}
	}

	// Go make a decision about the proposal
	if handled, reply, tcPolicy := c.HandleProposal(c.genericAgreementPH, proposal, protocolMsg, runningBCs, exchangeMsg); handled {
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
	} else if _, err := persistence.NewEstablishedAgreement(c.db, tcPolicy.Header.Name, proposal.AgreementId(), proposal.ConsumerId(), protocolMsg, c.Name(), proposal.Version(), tcPolicy.APISpecs[0].SpecRef, reply.Signature, proposal.Address, reply.BlockchainType, reply.BlockchainName); err != nil {
		glog.Errorf(PPHlogString(fmt.Sprintf("error persisting new agreement: %v, error: %v", proposal.AgreementId(), err)))
	}
}

func (c *CSProtocolHandler) HandleBlockchainEventMessage(cmd *BlockchainEventCommand) (string, bool, uint64, bool, error) {
	// Unmarshal the raw event
	if rawEvent, err := c.genericAgreementPH.DemarshalEvent(cmd.Msg.RawEvent()); err != nil {
		return "", false, 0, false, errors.New(PPHlogString(fmt.Sprintf("unable to demarshal raw event %v, error: %v", cmd.Msg.RawEvent(), err)))
	} else {
		agId := c.genericAgreementPH.GetAgreementId(rawEvent)
		if c.genericAgreementPH.ConsumerTermination(rawEvent) {
			if reason, err := c.genericAgreementPH.GetReasonCode(rawEvent); err != nil {
				return "", false, 0, false, errors.New(PPHlogString(fmt.Sprintf("unable to retrieve reason code from %v, error %v", rawEvent, err)))
			} else {
				return agId, true, reason, false, nil
			}
		} else if c.genericAgreementPH.AgreementCreated(rawEvent) {
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

func (c *CSProtocolHandler) SetBlockchainClientAvailable(cmd *BCInitializedCommand) {}

func (c *CSProtocolHandler) IsBlockchainClientAvailable(typeName string, name string) bool {
	nameMap := c.getBCNameMap(typeName)

	namedBC, ok := nameMap[name]
	if !ok {
		return false
	} else {
		return namedBC.ready
	}
}

func (c *CSProtocolHandler) SetBlockchainClientNotAvailable(cmd *BCStoppingCommand) {
	nameMap := c.getBCNameMap(policy.Ethereum_bc)

	delete(nameMap, cmd.Msg.BlockchainInstance())

	glog.V(3).Infof(PPHlogString(fmt.Sprintf("agreement protocol handler for %v cannot use blockchain because it is stopping.", cmd.Msg.BlockchainInstance())))

}

func (c *CSProtocolHandler) SetBlockchainWritable(cmd *BCWritableCommand) {

	nameMap := c.getBCNameMap(cmd.Msg.BlockchainType())

	_, ok := nameMap[cmd.Msg.BlockchainInstance()]
	if !ok {
		nameMap[cmd.Msg.BlockchainInstance()] = &BlockchainState{
								ready:       true,
								writable:    true,
								service:     cmd.Msg.ServiceName(),
								servicePort: cmd.Msg.ServicePort(),
								colonusDir:  cmd.Msg.ColonusDir(),
								agreementPH: citizenscientist.NewProtocolHandler(c.pm),
							}
	} else {
		nameMap[cmd.Msg.BlockchainInstance()].ready = true
		nameMap[cmd.Msg.BlockchainInstance()].writable = true
		nameMap[cmd.Msg.BlockchainInstance()].service = cmd.Msg.ServiceName()
		nameMap[cmd.Msg.BlockchainInstance()].servicePort = cmd.Msg.ServicePort()
		nameMap[cmd.Msg.BlockchainInstance()].colonusDir = cmd.Msg.ColonusDir()
		nameMap[cmd.Msg.BlockchainInstance()].agreementPH = citizenscientist.NewProtocolHandler(c.pm)
	}

	glog.V(3).Infof(PPHlogString(fmt.Sprintf("initializing agreement protocol handler for %v", cmd)))
	if err := nameMap[cmd.Msg.BlockchainInstance()].agreementPH.InitBlockchain(&cmd.Msg); err != nil {
		glog.Errorf(PPHlogString(fmt.Sprintf("failed initializing CS agreement protocol blockchain handler for %v, error: %v", cmd, err)))
	}

	glog.V(3).Infof(PPHlogString(fmt.Sprintf("agreement protocol handler can write to the blockchain now: %v", *nameMap[cmd.Msg.BlockchainInstance()])))

}

func (c *CSProtocolHandler) UpdateConsumers() {
	// A filter for limiting the returned set of agreements just to those that are waiting on protocol version 2 messages.
	notYetUpFilter := func() persistence.EAFilter {
		return func(a persistence.EstablishedAgreement) bool { return a.ProtocolVersion == 2 && a.AgreementBCUpdateAckTime == 0 && a.AgreementTerminatedTime == 0}
	}

	// Find all agreements that are in progress, waiting for the blockchain to come up.
	if agreements, err := persistence.FindEstablishedAgreements(c.db, c.Name(), []persistence.EAFilter{notYetUpFilter(),persistence.UnarchivedEAFilter()}); err != nil {
		glog.Errorf(PPHlogString(fmt.Sprintf("failed to get agreements for %v from the database, error: %v", c.Name(), err)))
	} else {

		for _, ag := range agreements {
			c.UpdateConsumer(&ag)
		}

	}
}

func (c *CSProtocolHandler) UpdateConsumer(ag *persistence.EstablishedAgreement) {

	glog.V(5).Infof(PPHlogString(fmt.Sprintf("agreement %v can complete agreement protocol", ag.CurrentAgreementId)))

	signature := ""
	if ag.ProposalSig == "" {
	    if proposal, err := c.genericAgreementPH.DemarshalProposal(ag.Proposal); err != nil {
			glog.Errorf(PPHlogString(fmt.Sprintf("unable to demarshal proposal for agreement %v from database", ag.CurrentAgreementId)))
			return
		} else {
			ph := c.AgreementProtocolHandler(ag.BlockchainType, ag.BlockchainName)
			if csph, ok := ph.(*citizenscientist.ProtocolHandler); ok {
				if _, sig, err := csph.SignProposal(proposal); err != nil {
					glog.Errorf(PPHlogString(fmt.Sprintf("Protocol %v agreement %v error signing proposal, error %v, %v", c.Name(), ag.CurrentAgreementId, err)))
				} else {
					signature = sig
					if _, err := persistence.AgreementStateProposalSigned(c.db, ag.CurrentAgreementId, c.Name(), signature); err != nil {
						glog.Errorf(PPHlogString(fmt.Sprintf("Protocol %v agreement %v error saving signature, error %v, %v", c.Name(), ag.CurrentAgreementId, err)))
					}
				}
			} else {
				glog.Errorf(PPHlogString(fmt.Sprintf("for agreement %v, error casting protocol handler to CS protocol handler, is %T", ag.CurrentAgreementId, ph)))
			}
		}
	} else {
		signature = ag.ProposalSig
	}

	if _, pubKey, err := c.GetAgbotMessageEndpoint(ag.ConsumerId); err != nil {
		glog.Errorf(PPHlogString(fmt.Sprintf("for agreement %v error getting agbot %v public key, error %v", ag.CurrentAgreementId, ag.ConsumerId, err)))
	} else if mt, err := exchange.CreateMessageTarget(ag.ConsumerId, nil, pubKey, ""); err != nil {
		glog.Errorf(PPHlogString(fmt.Sprintf("for agreement %v error creating message target %v", ag.CurrentAgreementId, err)))
	} else {
		ph := c.AgreementProtocolHandler(ag.BlockchainType, ag.BlockchainName)
		if csph, ok := ph.(*citizenscientist.ProtocolHandler); !ok {
			glog.Errorf(PPHlogString(fmt.Sprintf("for agreement %v, error casting protocol handler to CS protocol handler, is %T", ag.CurrentAgreementId, ph)))
		} else if err := csph.SendBlockchainProducerUpdate(ag.CurrentAgreementId, signature, mt, c.GetSendMessage()); err != nil {
			glog.Errorf(PPHlogString(fmt.Sprintf("error sending update for agreement %v, error: %v", ag.CurrentAgreementId, err)))
		}
	}

}

func (c *CSProtocolHandler) IsBlockchainWritable(ag *persistence.EstablishedAgreement) bool {
    if ag == nil { return true }

    bcType, bcName := c.GetKnownBlockchain(ag)

	nameMap := c.getBCNameMap(bcType)

	namedBC, ok := nameMap[bcName]
	if !ok {
		return false
	} else {
		return namedBC.writable
	}

}

func (c *CSProtocolHandler) getBCNameMap(typeName string) map[string]*BlockchainState {
	nameMap, ok := c.bcState[typeName]
	if !ok {
		c.bcState[typeName] = make(map[string]*BlockchainState)
		nameMap = c.bcState[typeName]
	}
	return nameMap
}

func (c *CSProtocolHandler) IsAgreementVerifiable(ag *persistence.EstablishedAgreement) bool {
	return (ag.ProtocolVersion == 0 || ag.ProtocolVersion == 1) || (ag.ProtocolVersion == 2 && ag.CounterPartyAddress != "")
}

func (c *CSProtocolHandler) HandleExtensionMessages(msg *events.ExchangeDeviceMessage, exchangeMsg *exchange.DeviceMessage) (bool, error) {

	deleteMessage := false

	// The BlockchainUpdate message contains the eth ID of the consumer, save it and return an Ack message.
	if update, err := c.genericAgreementPH.ValidateBlockchainConsumerUpdate(msg.ProtocolMessage()); err != nil {
		glog.V(5).Infof(PPHlogString(fmt.Sprintf("extension message handler ignoring non-blockchain update message: %s due to %v", msg.ShortProtocolMessage(), err)))
	} else {
		deleteMessage = true
		if ags, err := persistence.FindEstablishedAgreements(c.db, c.Name(), []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(update.AgreementId())}); err != nil {
			glog.Warningf(PPHlogString(fmt.Sprintf("unable to retrieve agreement %v from database, might be archived: %v", update.AgreementId(), err)))
		} else if len(ags) != 1 {
			glog.Errorf(PPHlogString(fmt.Sprintf("unable to retrieve single agreement %v from database.", update.AgreementId())))
		} else if _, err := persistence.AgreementStateBCDataReceived(c.db, ags[0].CurrentAgreementId, c.Name(), update.Address); err != nil {
			glog.Errorf(PPHlogString(fmt.Sprintf("unable to update blockchain address for agreement %v in database, error %v", update.AgreementId(), err)))
		} else if mt, err := exchange.CreateMessageTarget(exchangeMsg.AgbotId, nil, exchangeMsg.AgbotPubKey, ""); err != nil {
			glog.Errorf(PPHlogString(fmt.Sprintf("for agreement %v error creating message target %v", ags[0].CurrentAgreementId, err)))
		} else {
			ph := c.AgreementProtocolHandler(ags[0].BlockchainType, ags[0].BlockchainName)
			if csph, ok := ph.(*citizenscientist.ProtocolHandler); !ok {
				glog.Errorf(PPHlogString(fmt.Sprintf("for agreement %v, error casting protocol handler to CS protocol handler, is %T", update.AgreementId(), ph)))
			} else if err := csph.SendBlockchainConsumerUpdateAck(ags[0].CurrentAgreementId, mt, c.GetSendMessage()); err != nil {
				glog.Errorf(PPHlogString(fmt.Sprintf("error sending consumer update ack for agreement %v, error: %v", ags[0].CurrentAgreementId, err)))
			}
		}
	}

	if updateAck, err := c.genericAgreementPH.ValidateBlockchainProducerUpdateAck(msg.ProtocolMessage()); err != nil {
		glog.V(5).Infof(PPHlogString(fmt.Sprintf("extension message handler ignoring non-blockchain update ack message: %s due to %v", msg.ShortProtocolMessage(), err)))
	} else {
		deleteMessage = true
		if ags, err := persistence.FindEstablishedAgreements(c.db, c.Name(), []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(updateAck.AgreementId())}); err != nil {
			glog.Warningf(PPHlogString(fmt.Sprintf("unable to retrieve agreement %v from database, error %v", updateAck.AgreementId(), err)))
		} else if len(ags) != 1 {
			glog.Errorf(PPHlogString(fmt.Sprintf("unable to retrieve single agreement %v from database.", updateAck.AgreementId())))
		} else if _, err := persistence.AgreementStateBCUpdateAcked(c.db, ags[0].CurrentAgreementId, c.Name()); err != nil {
			glog.Errorf(PPHlogString(fmt.Sprintf("unable to update blockchain update ack time for agreement %v in database, error %v", updateAck.AgreementId(), err)))
		}
	}

	return deleteMessage, nil
}

func (c *CSProtocolHandler) GetKnownBlockchain(ag *persistence.EstablishedAgreement) (string, string) {
	bcType := ag.BlockchainType
	bcName := ag.BlockchainName
	if ag.AgreementProtocol == policy.CitizenScientist && ag.ProtocolVersion < 2 {
		if overrideName := os.Getenv("CMTN_BLOCKCHAIN"); overrideName != "" {
			return policy.Ethereum_bc, overrideName
		} else if proposal, err := c.genericAgreementPH.DemarshalProposal(ag.Proposal); err != nil {
			glog.Errorf(PPHlogString(fmt.Sprintf("error demarshalling proposal from agreement %v, error: %v", ag.CurrentAgreementId, err)))
		} else if pol, err := policy.DemarshalPolicy(proposal.TsAndCs()); err != nil {
			glog.Errorf(PPHlogString(fmt.Sprintf("error demarshalling tsandcs policy from agreement %v, error: %v", ag.CurrentAgreementId, err)))
		} else {
			agp := pol.AgreementProtocols[0]
			if agp.Blockchains == nil || len(agp.Blockchains) == 0 {
				return policy.Ethereum_bc, policy.Default_Blockchain_name
			} else {
				bcType = agp.Blockchains[0].Type
				bcName = agp.Blockchains[0].Name
			}
		}
	}
	return bcType, bcName
}

// ==========================================================================================================
// Utility functions

var PPHlogString = func(v interface{}) string {
	return fmt.Sprintf("Producer CS Protocol Handler %v", v)
}
