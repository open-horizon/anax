package agreementbot

import (
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/abstractprotocol"
	"github.com/open-horizon/anax/agreementbot/persistence"
	"github.com/open-horizon/anax/basicprotocol"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/metering"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/worker"
	"math/rand"
	"time"
)

type BasicProtocolHandler struct {
	*BaseConsumerProtocolHandler
	agreementPH *basicprotocol.ProtocolHandler
	Work        *PrioritizedWorkQueue
}

func NewBasicProtocolHandler(name string, cfg *config.HorizonConfig, db persistence.AgbotDatabase, pm *policy.PolicyManager, messages chan events.Message, mmsObjMgr *MMSObjectPolicyManager) *BasicProtocolHandler {
	if name == basicprotocol.PROTOCOL_NAME {
		return &BasicProtocolHandler{
			BaseConsumerProtocolHandler: &BaseConsumerProtocolHandler{
				name:             name,
				pm:               pm,
				db:               db,
				config:           cfg,
				httpClient:       cfg.Collaborators.HTTPClientFactory.NewHTTPClient(nil),
				agbotId:          cfg.AgreementBot.ExchangeId,
				token:            cfg.AgreementBot.ExchangeToken,
				deferredCommands: make([]AgreementWork, 0, 10),
				messages:         messages,
				mmsObjMgr:        mmsObjMgr,
			},
			agreementPH: basicprotocol.NewProtocolHandler(cfg.Collaborators.HTTPClientFactory.NewHTTPClient(nil), pm),
			// Allow the main agbot thread to distribute protocol msgs and agreement handling to the worker pool.
			Work: NewPrioritizedWorkQueue(cfg.GetAgbotAgreementQueueSize()),
		}
	} else {
		return nil
	}
}

func (c *BasicProtocolHandler) String() string {
	return fmt.Sprintf("Name: %v, "+
		"PM: %v, "+
		"DB: %v, "+
		"Agreement PH: %v",
		c.Name(), c.pm, c.db, c.agreementPH)
}

func (c *BasicProtocolHandler) Initialize() {

	glog.V(5).Infof(BsCPHlogString(fmt.Sprintf("initializing: %v ", c)))
	// Set up random number gen. This is used to generate agreement id strings.
	random := rand.New(rand.NewSource(int64(time.Now().Nanosecond())))

	// Setup a lock to protect concurrent agreement processing
	agreementLockMgr := NewAgreementLockManager()

	// Set up agreement worker pool based on the current technical config.
	for ix := 0; ix < c.config.AgreementBot.AgreementWorkers; ix++ {
		agw := NewBasicAgreementWorker(c, c.config, c.db, c.pm, agreementLockMgr, c.mmsObjMgr)
		go agw.start(c.Work, random)
	}

	worker.GetWorkerStatusManager().SetWorkerStatus("BasicProtocolHandler", worker.STATUS_INITIALIZED)
}

func (c *BasicProtocolHandler) AgreementProtocolHandler(typeName string, name string, org string) abstractprotocol.ProtocolHandler {
	return c.agreementPH
}

func (c *BasicProtocolHandler) WorkQueue() *PrioritizedWorkQueue {
	return c.Work
}

func (c *BasicProtocolHandler) AcceptCommand(cmd worker.Command) bool {

	switch cmd.(type) {
	case *NewProtocolMessageCommand:
		return true
	case *AgreementTimeoutCommand:
		return true
	case *PolicyChangedCommand:
		return true
	case *PolicyDeletedCommand:
		return true
	case *ServicePolicyChangedCommand:
		return true
	case *ServicePolicyDeletedCommand:
		return true
	case *WorkloadUpgradeCommand:
		return true
	case *MakeAgreementCommand:
		return true
	case *MMSObjectPolicyEventCommand:
		return true
	}
	return false
}

func (c *BasicProtocolHandler) PersistAgreement(wi *InitiateAgreement, proposal abstractprotocol.Proposal, workerID string) error {

	return c.BaseConsumerProtocolHandler.PersistBaseAgreement(wi, proposal, workerID, "", "")
}

func (c *BasicProtocolHandler) PersistReply(r abstractprotocol.ProposalReply, pol *policy.Policy, workerID string) error {

	return c.BaseConsumerProtocolHandler.PersistReply(r, pol, workerID)
}

func (c *BasicProtocolHandler) HandleBlockchainEvent(cmd *BlockchainEventCommand) {
	return
}

func (c *BasicProtocolHandler) CreateMeteringNotification(mp policy.Meter, ag *persistence.Agreement) (*metering.MeteringNotification, error) {

	return metering.NewMeteringNotification(mp, ag.AgreementCreationTime, uint64(ag.DataVerificationCheckRate), ag.DataVerificationMissedCount, ag.CurrentAgreementId, ag.ProposalHash, ag.ConsumerProposalSig, "", ag.ProposalSig, "")
}

func (c *BasicProtocolHandler) TerminateAgreement(ag *persistence.Agreement, reason uint, workerId string) {
	var messageTarget interface{}
	if whisperTo, pubkeyTo, err := c.BaseConsumerProtocolHandler.GetDeviceMessageEndpoint(ag.DeviceId, workerId); err != nil {
		glog.Errorf(BCPHlogstring2(workerId, fmt.Sprintf("error obtaining message target for cancel message: %v", err)))
	} else if mt, err := exchange.CreateMessageTarget(ag.DeviceId, nil, pubkeyTo, whisperTo); err != nil {
		glog.Errorf(BCPHlogstring2(workerId, fmt.Sprintf("error creating message target: %v", err)))
	} else {
		messageTarget = mt
	}
	c.BaseConsumerProtocolHandler.TerminateAgreement(ag, reason, messageTarget, workerId, c)
}

func (c *BasicProtocolHandler) GetTerminationCode(reason string) uint {
	switch reason {
	case TERM_REASON_POLICY_CHANGED:
		return basicprotocol.AB_CANCEL_POLICY_CHANGED
	// case TERM_REASON_NOT_FINALIZED_TIMEOUT:
	//     return basicprotocol.AB_CANCEL_NOT_FINALIZED_TIMEOUT
	case TERM_REASON_NO_DATA_RECEIVED:
		return basicprotocol.AB_CANCEL_NO_DATA_RECEIVED
	case TERM_REASON_NO_REPLY:
		return basicprotocol.AB_CANCEL_NO_REPLY
	case TERM_REASON_USER_REQUESTED:
		return basicprotocol.AB_USER_REQUESTED
	case TERM_REASON_DEVICE_REQUESTED:
		return basicprotocol.CANCEL_USER_REQUESTED
	case TERM_REASON_NEGATIVE_REPLY:
		return basicprotocol.AB_CANCEL_NEGATIVE_REPLY
	case TERM_REASON_CANCEL_DISCOVERED:
		return basicprotocol.AB_CANCEL_DISCOVERED
	case TERM_REASON_CANCEL_FORCED_UPGRADE:
		return basicprotocol.AB_CANCEL_FORCED_UPGRADE
	// case TERM_REASON_CANCEL_BC_WRITE_FAILED:
	//     return basicprotocol.AB_CANCEL_BC_WRITE_FAILED
	case TERM_REASON_NODE_HEARTBEAT:
		return basicprotocol.AB_CANCEL_NODE_HEARTBEAT
	case TERM_REASON_AG_MISSING:
		return basicprotocol.AB_CANCEL_AG_MISSING
	default:
		return 999
	}
}

func (c *BasicProtocolHandler) GetTerminationReason(code uint) string {
	return basicprotocol.DecodeReasonCode(uint64(code))
}

func (c *BasicProtocolHandler) IsTerminationReasonNodeShutdown(code uint) bool {
	return uint(code) == basicprotocol.CANCEL_NODE_SHUTDOWN
}

func (c *BasicProtocolHandler) SetBlockchainWritable(ev *events.AccountFundedMessage) {
	return
}

func (c *BasicProtocolHandler) IsBlockchainWritable(typeName string, name string, org string) bool {
	return true
}

func (c *BasicProtocolHandler) CanCancelNow(ag *persistence.Agreement) bool {
	return true
}

func (c *BasicProtocolHandler) HandleDeferredCommands() {

	cmds := c.GetDeferredCommands()
	for _, cmd := range cmds {
		switch cmd.Type() {
		case INITIATE:
			c.WorkQueue().InboundLow() <- &cmd
			glog.V(5).Infof(BsCPHlogString(fmt.Sprintf("queued make agreement command: %v", cmd)))
		default:
			glog.Errorf(BsCPHlogString(fmt.Sprintf("unknown deferred command: %v", cmd)))
		}
	}

}

func (b *BasicProtocolHandler) PostReply(agreementId string, proposal abstractprotocol.Proposal, reply abstractprotocol.ProposalReply, consumerPolicy *policy.Policy, org string, workerId string) error {

	if err := b.agreementPH.RecordAgreement(proposal, reply, "", "", consumerPolicy, org); err != nil {
		return err
	} else {
		glog.V(3).Infof(BCPHlogstring2(workerId, fmt.Sprintf("recorded agreement %v", agreementId)))
	}

	return nil

}

func (b *BasicProtocolHandler) HandleExtensionMessage(cmd *NewProtocolMessageCommand) error {
	glog.V(5).Infof(BsCPHlogString(fmt.Sprintf("received inbound exchange message.")))

	// Figure out what kind of message this is
	if verify, perr := b.agreementPH.ValidateAgreementVerify(string(cmd.Message)); perr == nil {
		agreementWork := NewBAgreementVerification(verify, cmd.From, cmd.PubKey, cmd.MessageId)
		b.WorkQueue().InboundHigh() <- &agreementWork
		glog.V(5).Infof(BsCPHlogString(fmt.Sprintf("queued agreement verify message")))

	} else if verifyr, perr := b.agreementPH.ValidateAgreementVerifyReply(string(cmd.Message)); perr == nil {
		agreementWork := NewBAgreementVerificationReply(verifyr, cmd.From, cmd.PubKey, cmd.MessageId)
		b.WorkQueue().InboundHigh() <- &agreementWork
		glog.V(5).Infof(BsCPHlogString(fmt.Sprintf("queued agreement verify reply message")))

	} else {
		glog.V(5).Infof(BsCPHlogString(fmt.Sprintf("ignoring  message: %v because it is an unknown type", string(cmd.Message))))
		return errors.New(BsCPHlogString(fmt.Sprintf("unknown protocol msg %s", cmd.Message)))
	}
	return nil
}

// ==========================================================================================================
// Utility functions

var BsCPHlogString = func(v interface{}) string {
	return fmt.Sprintf("AgreementBot Basic Protocol Handler %v", v)
}
