package agreementbot

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/abstractprotocol"
	"github.com/open-horizon/anax/citizenscientist"
	"github.com/open-horizon/anax/ethblockchain"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/metering"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/worker"
	"math/rand"
	"net/http"
	"time"
)

type CSProtocolHandler struct {
	*BaseConsumerProtocolHandler
	agreementPH *citizenscientist.ProtocolHandler
	Work        chan AgreementWork // outgoing commands for the workers
}

func NewCSProtocolHandler(name string, cfg *config.HorizonConfig, db *bolt.DB, pm *policy.PolicyManager) *CSProtocolHandler {
	if name == citizenscientist.PROTOCOL_NAME {
		return &CSProtocolHandler{
			BaseConsumerProtocolHandler: &BaseConsumerProtocolHandler{
				name:       name,
				pm:         pm,
				db:         db,
				config:     cfg,
				httpClient: &http.Client{Timeout: time.Duration(config.HTTPDEFAULTTIMEOUT * time.Millisecond)},
				agbotId:    cfg.AgreementBot.ExchangeId,
				token:      cfg.AgreementBot.ExchangeToken,
			},
			agreementPH: citizenscientist.NewProtocolHandler(cfg.AgreementBot.GethURL, pm),
			Work:        make(chan AgreementWork),
		}
	} else {
		return nil
	}
}

func (c *CSProtocolHandler) String() string {
	return fmt.Sprintf("Name: %v, "+
		"PM: %v, "+
		"DB: %v, "+
		"Agreement PH: %v",
		c.Name(), c.pm, c.db, c.agreementPH)
}

func (c *CSProtocolHandler) Initialize() {

	glog.V(5).Infof(CPHlogString(fmt.Sprintf("initializing: %v ", c)))
	// Set up random number gen. This is used to generate agreement id strings.
	random := rand.New(rand.NewSource(int64(time.Now().Nanosecond())))

	// Setup a lock to protect concurrent agreement processing
	agreementLockMgr := NewAgreementLockManager()

	// Set up agreement worker pool based on the current technical config.
	for ix := 0; ix < c.config.AgreementBot.AgreementWorkers; ix++ {
		agw := NewCSAgreementWorker(c, c.config, c.db, c.pm, agreementLockMgr)
		go agw.start(c.Work, random)
	}

}

func (c *CSProtocolHandler) AgreementProtocolHandler() abstractprotocol.ProtocolHandler {
	return c.agreementPH
}

func (c *CSProtocolHandler) WorkQueue() chan AgreementWork {
	return c.Work
}

func (c *CSProtocolHandler) AcceptCommand(cmd worker.Command) bool {

	switch cmd.(type) {
	case *NewProtocolMessageCommand:
		return true
	case *AgreementTimeoutCommand:
		return true
	case *BlockchainEventCommand:
		return true
	case *PolicyChangedCommand:
		return true
	case *PolicyDeletedCommand:
		return true
	case *WorkloadUpgradeCommand:
		return true
	case *MakeAgreementCommand:
		return true
	}
	return false
}

func (c *CSProtocolHandler) PersistAgreement(wi *InitiateAgreement, proposal abstractprotocol.Proposal, workerID string) error {

	if hash, sig, err := c.agreementPH.SignProposal(proposal); err != nil {
		glog.Errorf(CPHlogStringW(workerID, fmt.Sprintf("error signing proposal %v, error: %v", proposal, err)))
	} else {
		c.BaseConsumerProtocolHandler.PersistBaseAgreement(wi, proposal, workerID, hash, sig)
	}
	return nil
}

func (c *CSProtocolHandler) PersistReply(r abstractprotocol.ProposalReply, pol *policy.Policy, workerID string) error {

	if reply, ok := r.(*citizenscientist.CSProposalReply); !ok {
		return errors.New(CPHlogStringW(workerID, fmt.Sprintf("unable to cast reply %v to %v Proposal Reply, is %T", r, c.Name(), r)))
	} else if _, err := AgreementMade(c.db, reply.AgreementId(), reply.Address, reply.Signature, c.Name(), pol.HAGroup.Partners); err != nil {
		return errors.New(CPHlogStringW(workerID, fmt.Sprintf("error updating agreement %v with reply info DB, error: %v", reply.AgreementId(), err)))
	}
	return nil
}

func (c *CSProtocolHandler) HandleBlockchainEvent(cmd *BlockchainEventCommand) {

	glog.V(5).Infof(CPHlogString("received blockchain event."))
	// Unmarshal the raw event
	if csaph, ok := c.AgreementProtocolHandler().(*citizenscientist.ProtocolHandler); !ok {
		glog.Errorf(CPHlogString(fmt.Sprintf("unable to cast agreement protocol handler %T to CS specific handler to process BC event %v", c.AgreementProtocolHandler(), cmd.Msg.RawEvent())))
	} else if rawEvent, err := csaph.DemarshalEvent(cmd.Msg.RawEvent()); err != nil {
		glog.Errorf(CPHlogString(fmt.Sprintf("unable to demarshal raw event %v, error: %v", cmd.Msg.RawEvent(), err)))
	} else if !csaph.AgreementCreated(rawEvent) && !csaph.ProducerTermination(rawEvent) && !csaph.ConsumerTermination(rawEvent) {
		glog.V(5).Infof(CPHlogString(fmt.Sprintf("ignoring the blockchain event because it is not agreement creation or termination event.")))
	} else {
		agreementId := csaph.GetAgreementId(rawEvent)

		if csaph.AgreementCreated(rawEvent) {
			agreementWork := CSHandleBCRecorded{
				workType:    BC_RECORDED,
				AgreementId: agreementId,
				Protocol:    c.Name(),
			}
			c.Work <- agreementWork
			glog.V(5).Infof(CPHlogString(fmt.Sprintf("queued blockchain agreement recorded event: %v", agreementWork)))

			// If the event is a agreement terminated event
		} else if csaph.ProducerTermination(rawEvent) || csaph.ConsumerTermination(rawEvent) {
			agreementWork := CSHandleBCTerminated{
				workType:    BC_TERMINATED,
				AgreementId: agreementId,
				Protocol:    c.Name(),
			}
			c.Work <- agreementWork
			glog.V(5).Infof(CPHlogString(fmt.Sprintf("queued agreement cancellation due to blockchain termination event: %v", agreementWork)))
		}
	}

}

func (c *CSProtocolHandler) CreateMeteringNotification(mp policy.Meter, ag *Agreement) (*metering.MeteringNotification, error) {

	myAddress, _ := ethblockchain.AccountId()
	return metering.NewMeteringNotification(mp, ag.AgreementCreationTime, uint64(ag.DataVerificationCheckRate), ag.DataVerificationMissedCount, ag.CurrentAgreementId, ag.ProposalHash, ag.ConsumerProposalSig, myAddress, ag.ProposalSig, "ethereum")
}

func (c *CSProtocolHandler) TerminateAgreement(ag *Agreement, reason uint, workerId string) {
	// The CS protocol doesnt send cancel messages, it depends on the blockchain to maintain the state of
	// any given agreement. This means we can fake up a message target for the TerminateAgreement call
	// because we know that the CS implementation of the agreement protocol wont be sending a message.
	fakeMT := &exchange.ExchangeMessageTarget{
			ReceiverExchangeId:     "",
			ReceiverPublicKeyObj:   nil,
			ReceiverPublicKeyBytes: []byte(""),
			ReceiverMsgEndPoint:    "",
			}
	c.BaseConsumerProtocolHandler.TerminateAgreement(ag, reason, fakeMT, workerId, c)
}

func (c *CSProtocolHandler) GetTerminationCode(reason string) uint {
	switch reason {
	case TERM_REASON_POLICY_CHANGED:
		return citizenscientist.AB_CANCEL_POLICY_CHANGED
	case TERM_REASON_NOT_FINALIZED_TIMEOUT:
		return citizenscientist.AB_CANCEL_NOT_FINALIZED_TIMEOUT
	case TERM_REASON_NO_DATA_RECEIVED:
		return citizenscientist.AB_CANCEL_NO_DATA_RECEIVED
	case TERM_REASON_NO_REPLY:
		return citizenscientist.AB_CANCEL_NO_REPLY
	case TERM_REASON_USER_REQUESTED:
		return citizenscientist.AB_USER_REQUESTED
	case TERM_REASON_NEGATIVE_REPLY:
		return citizenscientist.AB_CANCEL_NEGATIVE_REPLY
	case TERM_REASON_CANCEL_DISCOVERED:
		return citizenscientist.AB_CANCEL_DISCOVERED
	case TERM_REASON_CANCEL_FORCED_UPGRADE:
		return citizenscientist.AB_CANCEL_FORCED_UPGRADE
	case TERM_REASON_CANCEL_BC_WRITE_FAILED:
		return citizenscientist.AB_CANCEL_BC_WRITE_FAILED
	default:
		return 999
	}
}

func (c *CSProtocolHandler) GetTerminationReason(code uint) string {
	return citizenscientist.DecodeReasonCode(uint64(code))
}

// ==========================================================================================================
// Utility functions

var CPHlogString = func(v interface{}) string {
	return fmt.Sprintf("AgreementBot CS Protocol Handler %v", v)
}

var CPHlogStringW = func(workerId string, v interface{}) string {
	return fmt.Sprintf("AgreementBot CS Protocol Handler (%v) %v", workerId, v)
}
