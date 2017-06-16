package producer

import (
    "fmt"
    "github.com/boltdb/bolt"
    "github.com/golang/glog"
    "github.com/open-horizon/anax/abstractprotocol"
    "github.com/open-horizon/anax/basicprotocol"
    "github.com/open-horizon/anax/config"
    "github.com/open-horizon/anax/exchange"
    "github.com/open-horizon/anax/persistence"
    "github.com/open-horizon/anax/policy"
    "github.com/open-horizon/anax/worker"
    "net/http"
    "time"
)

type BasicProtocolHandler struct {
    *BaseProducerProtocolHandler
    agreementPH *basicprotocol.ProtocolHandler
}

func NewBasicProtocolHandler(name string, cfg *config.HorizonConfig, db *bolt.DB, pm *policy.PolicyManager, deviceId string, token string) *BasicProtocolHandler {
    if name == basicprotocol.PROTOCOL_NAME {
        return &BasicProtocolHandler{
            BaseProducerProtocolHandler: &BaseProducerProtocolHandler{
                name:       name,
                pm:         pm,
                db:         db,
                config:     cfg,
                deviceId:   deviceId,
                token:      token,
                httpClient: &http.Client{Timeout: time.Duration(config.HTTPDEFAULTTIMEOUT * time.Millisecond)},
            },
            agreementPH: basicprotocol.NewProtocolHandler(pm),
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
        c.Name, c.deviceId, c.token, c.pm, c.db, c.agreementPH)
}

func (c *BasicProtocolHandler) AgreementProtocolHandler() abstractprotocol.ProtocolHandler {
    return c.agreementPH
}

func (c *BasicProtocolHandler) AcceptCommand(cmd worker.Command) bool {

    return false
}

func (c *BasicProtocolHandler) HandleProposalMessage(proposal abstractprotocol.Proposal, protocolMsg string, exchangeMsg *exchange.DeviceMessage) bool {

    if handled, reply, tcPolicy := c.HandleProposal(c.agreementPH, proposal, protocolMsg, exchangeMsg); handled {
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
    if whisperTo, pubkeyTo, err := c. BaseProducerProtocolHandler.GetAgbotMessageEndpoint(ag.ConsumerId); err != nil {
        glog.Errorf(BPHlogString(fmt.Sprintf("error obtaining message target for agreement %v cancel: %v", ag.CurrentAgreementId, err)))
    } else if mt, err := exchange.CreateMessageTarget(ag.ConsumerId, nil, pubkeyTo, whisperTo); err != nil {
        glog.Errorf(BPHlogString(fmt.Sprintf("error creating message target: %v", err)))
    } else {
        messageTarget = mt
    }
    c.BaseProducerProtocolHandler.TerminateAgreement(ag, reason, messageTarget, c)
}

func (c *BasicProtocolHandler) GetTerminationCode(reason string) uint {
    switch reason {
    case TERM_REASON_POLICY_CHANGED:
        return basicprotocol.CANCEL_POLICY_CHANGED
    case TERM_REASON_AGBOT_REQUESTED:
        return basicprotocol.CANCEL_AGBOT_REQUESTED
    case TERM_REASON_CONTAINER_FAILURE:
        return basicprotocol.CANCEL_CONTAINER_FAILURE
    case TERM_REASON_TORRENT_FAILURE:
        return basicprotocol.CANCEL_TORRENT_FAILURE
    case TERM_REASON_USER_REQUESTED:
        return basicprotocol.CANCEL_USER_REQUESTED
    // case TERM_REASON_NOT_FINALIZED_TIMEOUT:
    //     return citizenscientist.CANCEL_NOT_FINALIZED_TIMEOUT
    case TERM_REASON_NO_REPLY_ACK:
        return basicprotocol.CANCEL_NO_REPLY_ACK
    case TERM_REASON_NOT_EXECUTED_TIMEOUT:
        return basicprotocol.CANCEL_NOT_EXECUTED_TIMEOUT
    default:
        return 999
    }
}

func (c *BasicProtocolHandler) GetTerminationReason(code uint) string {
    return basicprotocol.DecodeReasonCode(uint64(code))
}

// ==========================================================================================================
// Utility functions

var BPHlogString = func(v interface{}) string {
    return fmt.Sprintf("Producer Basic Protocol Handler %v", v)
}
