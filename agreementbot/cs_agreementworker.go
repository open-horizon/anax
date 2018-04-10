package agreementbot

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/citizenscientist"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/policy"
	"github.com/satori/go.uuid"
	"math/rand"
	"runtime"
)

type CSAgreementWorker struct {
	*BaseAgreementWorker
	protocolHandler *CSProtocolHandler
}

func NewCSAgreementWorker(c *CSProtocolHandler, cfg *config.HorizonConfig, db *bolt.DB, pm *policy.PolicyManager, alm *AgreementLockManager) *CSAgreementWorker {

	p := &CSAgreementWorker{
		BaseAgreementWorker: &BaseAgreementWorker{
			pm:         pm,
			db:         db,
			config:     cfg,
			alm:        alm,
			workerID:   uuid.NewV4().String(),
			httpClient: cfg.Collaborators.HTTPClientFactory.NewHTTPClient(nil),
		},
		protocolHandler: c,
	}

	return p
}

// // These structs are the work items that flow to the agreement workers

const BC_RECORDED = "AGREEMENT_BC_RECORDED"
const BC_TERMINATED = "AGREEMENT_BC_TERMINATED"
const ASYNC_WRITE = "ASYNC_WRITE"
const ASYNC_UPDATE = "ASYNC_UPDATE"
const PRODUCER_UPDATE = "PRODUCER_UPDATE"
const CONSUMER_UPDATE_ACK = "CONSUMER_UPDATE_ACK"

type CSHandleBCRecorded struct {
	workType    string
	AgreementId string
	Protocol    string
}

func (c CSHandleBCRecorded) Type() string {
	return c.workType
}

type CSHandleBCTerminated struct {
	workType    string
	AgreementId string
	Protocol    string
}

func (c CSHandleBCTerminated) Type() string {
	return c.workType
}

type AsyncWriteAgreement struct {
	workType    string
	AgreementId string
	Protocol    string
}

func (c AsyncWriteAgreement) Type() string {
	return c.workType
}

type AsyncUpdateAgreement struct {
	workType    string
	AgreementId string
	Protocol    string
}

func (c AsyncUpdateAgreement) Type() string {
	return c.workType
}

type CSProducerUpdate struct {
	workType     string
	Update       citizenscientist.CSBlockchainProducerUpdate
	From         string // deprecated whisper address
	SenderId     string // exchange Id of sender
	SenderPubKey []byte
	MessageId    int
}

func (c CSProducerUpdate) Type() string {
	return c.workType
}

func (c CSProducerUpdate) String() string {
	return fmt.Sprintf("Workitem: %v, SenderId: %v, MessageId: %v, From: %v, Update: %v, SenderPubKey: %x", c.workType, c.SenderId, c.MessageId, c.From, c.Update, c.SenderPubKey)
}

type CSConsumerUpdateAck struct {
	workType     string
	Update       citizenscientist.CSBlockchainConsumerUpdateAck
	From         string // deprecated whisper address
	SenderId     string // exchange Id of sender
	SenderPubKey []byte
	MessageId    int
}

func (c CSConsumerUpdateAck) Type() string {
	return c.workType
}

func (c CSConsumerUpdateAck) String() string {
	return fmt.Sprintf("Workitem: %v, SenderId: %v, MessageId: %v, From: %v, Update: %v, SenderPubKey: %x", c.workType, c.SenderId, c.MessageId, c.From, c.Update, c.SenderPubKey)
}

// This function receives an event to "make a new agreement" from the Process function, and then synchronously calls a function
// to actually work through the agreement protocol.
func (a *CSAgreementWorker) start(work chan AgreementWork, random *rand.Rand) {

	for {
		glog.V(5).Infof(logstring(a.workerID, fmt.Sprintf("blocking for work")))
		workItem := <-work // block waiting for work
		glog.V(2).Infof(logstring(a.workerID, fmt.Sprintf("received work: %v", workItem)))

		if workItem.Type() == INITIATE {
			wi := workItem.(InitiateAgreement)
			a.InitiateNewAgreement(a.protocolHandler, &wi, random, a.workerID)

		} else if workItem.Type() == REPLY {
			wi := workItem.(HandleReply)
			a.HandleAgreementReply(a.protocolHandler, &wi, a.workerID)

		} else if workItem.Type() == DATARECEIVEDACK {
			wi := workItem.(HandleDataReceivedAck)
			a.HandleDataReceivedAck(a.protocolHandler, &wi, a.workerID)

		} else if workItem.Type() == CANCEL {
			wi := workItem.(CancelAgreement)
			a.CancelAgreementWithLock(a.protocolHandler, wi.AgreementId, wi.Reason, a.workerID)

		} else if workItem.Type() == BC_RECORDED {
			// the agreement is recorded on the blockchain
			wi := workItem.(CSHandleBCRecorded)

			// Get the agreement id lock to prevent any other thread from processing this same agreement.
			lock := a.alm.getAgreementLock(wi.AgreementId)
			lock.Lock()

			if ag, err := FindSingleAgreementByAgreementId(a.protocolHandler.db, wi.AgreementId, a.protocolHandler.Name(), []AFilter{}); err != nil {
				glog.Errorf(logstring(a.workerID, fmt.Sprintf("error querying agreement %v from database, error: %v", wi.AgreementId, err)))
			} else if ag == nil {
				glog.V(3).Infof(logstring(a.workerID, fmt.Sprintf("nothing to do for agreement %v, no database record.", wi.AgreementId)))
			} else if ag.Archived || ag.AgreementTimedout != 0 {
				// The agreement could be cancelled BEFORE it is written to the blockchain. If we find a BC recorded event for an archived
				// or timed out agreement then we know this occurred. Cancel the agreement again so that the device will see the cancel.
				// This routine does not need to be a subworker because it will terminate on its own.
				go a.DoAsyncCancel(a.protocolHandler, ag, ag.TerminatedReason, a.workerID)
			} else {
				// Update state in the database
				if _, err := AgreementFinalized(a.protocolHandler.db, wi.AgreementId, a.protocolHandler.Name()); err != nil {
					glog.Errorf(logstring(a.workerID, fmt.Sprintf("error persisting agreement %v finalized: %v", wi.AgreementId, err)))
				}

				// Update state in exchange
				if pol, err := policy.DemarshalPolicy(ag.Policy); err != nil {
					glog.Errorf(logstring(a.workerID, fmt.Sprintf("error demarshalling policy from agreement %v, error: %v", wi.AgreementId, err)))
				} else if err := a.protocolHandler.RecordConsumerAgreementState(wi.AgreementId, pol, ag.Org, "Finalized Agreement", a.workerID); err != nil {
					glog.Errorf(logstring(a.workerID, fmt.Sprintf("error setting agreement %v finalized state in exchange: %v", wi.AgreementId, err)))
				}
			}

			// Drop the lock. The code above must always flow through this point.
			lock.Unlock()

		} else if workItem.Type() == BC_TERMINATED {
			// the agreement is terminated on the blockchain
			wi := workItem.(CSHandleBCTerminated)
			a.CancelAgreementWithLock(a.protocolHandler, wi.AgreementId, a.protocolHandler.GetTerminationCode(TERM_REASON_CANCEL_DISCOVERED), a.workerID)

		} else if workItem.Type() == WORKLOAD_UPGRADE {
			// upgrade a workload on a device
			wi := workItem.(HandleWorkloadUpgrade)
			a.HandleWorkloadUpgrade(a.protocolHandler, &wi, a.workerID)

		} else if workItem.Type() == ASYNC_CANCEL {
			wi := workItem.(AsyncCancelAgreement)
			a.ExternalCancel(a.protocolHandler, wi.AgreementId, wi.Reason, a.workerID)

		} else if workItem.Type() == ASYNC_WRITE {
			wi := workItem.(AsyncWriteAgreement)
			a.ExternalWrite(a.protocolHandler, wi.AgreementId, a.workerID)

		} else if workItem.Type() == ASYNC_UPDATE {
			wi := workItem.(AsyncUpdateAgreement)
			a.SendBCUpdate(a.protocolHandler, wi.AgreementId, a.workerID)

		} else if workItem.Type() == PRODUCER_UPDATE {
			wi := workItem.(CSProducerUpdate)
			a.HandleProducerUpdate(a.protocolHandler, &wi, a.workerID)

		} else if workItem.Type() == CONSUMER_UPDATE_ACK {
			wi := workItem.(CSConsumerUpdateAck)
			a.HandleConsumerUpdateAck(a.protocolHandler, &wi, a.workerID)

		} else {
			glog.Errorf(logstring(a.workerID, fmt.Sprintf("received unknown work request: %v", workItem)))
		}

		glog.V(5).Infof(logstring(a.workerID, fmt.Sprintf("handled work: %v", workItem)))
		runtime.Gosched()

	}
}

func (a *CSAgreementWorker) ExternalWrite(cph ConsumerProtocolHandler, agreementId string, workerID string) {

	lock := a.alm.getAgreementLock(agreementId)
	lock.Lock()
	defer lock.Unlock()

	if ag, err := FindSingleAgreementByAgreementId(a.db, agreementId, cph.Name(), []AFilter{UnarchivedAFilter()}); err != nil {
		glog.Errorf(logstring(workerID, fmt.Sprintf("error querying agreement %v, error: %v", agreementId, err)))
	} else if ag == nil {
		glog.V(3).Infof(logstring(workerID, fmt.Sprintf("agreement %v no longer active, cancelling deferred write.", agreementId)))
	} else if ag.AgreementTimedout != 0 {
		glog.V(3).Infof(logstring(workerID, fmt.Sprintf("agreement %v terminating, cancelling deferred write.", agreementId)))
	} else if cph.IsBlockchainWritable(ag.BlockchainType, ag.BlockchainName, ag.BlockchainOrg) && ag.CounterPartyAddress != "" {

		// Recording the agreement on the blockchain could take a long time.
		// This routine does not need to be a subworker because it will terminate on its own.
		go a.DoAsyncWrite(cph, ag, workerID)

	} else {
		// create deferred write command
		glog.V(5).Infof(logstring(workerID, fmt.Sprintf("agreement %v deferring blockchain write.", agreementId)))
		cph.DeferCommand(AsyncWriteAgreement{
			workType:    ASYNC_WRITE,
			AgreementId: ag.CurrentAgreementId,
			Protocol:    cph.Name(),
		})
	}
}

func (a *CSAgreementWorker) DoAsyncWrite(cph ConsumerProtocolHandler, ag *Agreement, workerID string) {
	if proposal, err := cph.AgreementProtocolHandler(ag.BlockchainType, ag.BlockchainName, ag.BlockchainOrg).DemarshalProposal(ag.Proposal); err != nil {
		glog.Errorf(logstring(workerID, fmt.Sprintf("error demarshalling proposal from pending agreement %v, error: %v", ag.CurrentAgreementId, err)))
	} else if pol, err := policy.DemarshalPolicy(ag.Policy); err != nil {
		glog.Errorf(logstring(workerID, fmt.Sprintf("error demarshalling tsandcs policy from pending agreement %v, error: %v", ag.CurrentAgreementId, err)))
	} else if err := cph.AgreementProtocolHandler(ag.BlockchainType, ag.BlockchainName, ag.BlockchainOrg).RecordAgreement(proposal, nil, ag.CounterPartyAddress, ag.ProposalSig, pol, ag.Org); err != nil {
		glog.Errorf(logstring(workerID, fmt.Sprintf("error trying to record agreement in blockchain, %v", err)))
		a.CancelAgreementWithLock(cph, ag.CurrentAgreementId, cph.GetTerminationCode(TERM_REASON_CANCEL_BC_WRITE_FAILED), workerID)
	} else {
		glog.V(3).Infof(logstring(workerID, fmt.Sprintf("recorded agreement %v", ag.CurrentAgreementId)))
	}
}

func (a *CSAgreementWorker) SendBCUpdate(ph ConsumerProtocolHandler, agreementId string, workerID string) {

	lock := a.alm.getAgreementLock(agreementId)
	lock.Lock()
	defer lock.Unlock()

	if cph, ok := ph.(*CSProtocolHandler); !ok {
		glog.Errorf(logstring(workerID, fmt.Sprintf("for agreement %v error casting protocol handler to CS specific handler, is type %T", agreementId, ph)))
	} else if ag, err := FindSingleAgreementByAgreementId(a.db, agreementId, cph.Name(), []AFilter{UnarchivedAFilter()}); err != nil {
		glog.Errorf(logstring(workerID, fmt.Sprintf("error querying agreement %v, error: %v", agreementId, err)))
	} else if ag == nil {
		glog.V(3).Infof(logstring(workerID, fmt.Sprintf("agreement %v no longer active, cancelling deferred update.", agreementId)))
	} else if ag.AgreementTimedout != 0 {
		glog.V(3).Infof(logstring(workerID, fmt.Sprintf("agreement %v terminating, cancelling deferred update.", agreementId)))
	} else if ag.BCUpdateAckTime != 0 {
		glog.V(3).Infof(logstring(workerID, fmt.Sprintf("agreement %v received update ack, cancelling deferred update.", agreementId)))
	} else if cph.IsBlockchainReady(ag.BlockchainType, ag.BlockchainName, ag.BlockchainOrg) && ag.BCUpdateAckTime == 0 {
		cph.UpdateProducer(ag)
		// create deferred update command as a mechanism to retry the update if messaging fails to deliver the message.
		cph.DeferCommand(AsyncUpdateAgreement{
			workType:    ASYNC_UPDATE,
			AgreementId: ag.CurrentAgreementId,
			Protocol:    cph.Name(),
		})
	} else {
		// create deferred update command to wait until blockchain comes up
		glog.V(5).Infof(logstring(workerID, fmt.Sprintf("agreement %v deferring blockchain update.", agreementId)))
		cph.DeferCommand(AsyncUpdateAgreement{
			workType:    ASYNC_UPDATE,
			AgreementId: ag.CurrentAgreementId,
			Protocol:    cph.Name(),
		})
	}

}

func (a *CSAgreementWorker) HandleProducerUpdate(cph *CSProtocolHandler, wi *CSProducerUpdate, workerID string) {

	deletedMessage := false

	// Get the agreement id lock to prevent any other thread from processing this same agreement.
	lock := a.alm.getAgreementLock(wi.Update.AgreementId())
	lock.Lock()
	defer lock.Unlock()

	// The protocol message has already been validated. Store the signature and the address from the producer, and then
	// return an ack.

	if ag, err := FindSingleAgreementByAgreementId(a.db, wi.Update.AgreementId(), cph.Name(), []AFilter{UnarchivedAFilter()}); err != nil {
		glog.Errorf(logstring(workerID, fmt.Sprintf("error querying agreement %v, error: %v", wi.Update.AgreementId(), err)))
	} else if ag == nil {
		glog.V(3).Infof(logstring(workerID, fmt.Sprintf("agreement %v no longer active.", wi.Update.AgreementId())))
	} else if ag.AgreementTimedout != 0 {
		glog.V(3).Infof(logstring(workerID, fmt.Sprintf("agreement %v terminating.", wi.Update.AgreementId())))
	} else if _, err := AgreementBlockchainUpdate(a.db, wi.Update.AgreementId(), "", "", wi.Update.Address, wi.Update.Signature, cph.Name()); err != nil {
		glog.Errorf(logstring(workerID, fmt.Sprintf("error hardening producer sig and address for agreement %v, error: %v", wi.Update.AgreementId(), err)))
	} else if mt, err := exchange.CreateMessageTarget(wi.SenderId, nil, wi.SenderPubKey, ""); err != nil {
		glog.Errorf(logstring(workerID, fmt.Sprintf("error creating message target for producer update ack, agreement: %v, error: %v", wi.Update.AgreementId(), err)))
	} else if err := cph.genericAgreementPH.SendBlockchainProducerUpdateAck(wi.Update.AgreementId(), mt, cph.GetSendMessage()); err != nil {
		glog.Errorf(logstring(workerID, fmt.Sprintf("error sending producer update ack, agreement: %v, error: %v", wi.Update.AgreementId(), err)))
	}

	// Get rid of the exchange message if there is one
	if wi.MessageId != 0 && !deletedMessage {
		if err := cph.DeleteMessage(wi.MessageId); err != nil {
			glog.Errorf(logstring(workerID, fmt.Sprintf("error deleting message %v from exchange for agbot %v", wi.MessageId, cph.GetExchangeId())))
		}
	}

}

func (a *CSAgreementWorker) HandleConsumerUpdateAck(cph *CSProtocolHandler, wi *CSConsumerUpdateAck, workerID string) {

	deletedMessage := false

	// Get the agreement id lock to prevent any other thread from processing this same agreement.
	lock := a.alm.getAgreementLock(wi.Update.AgreementId())
	lock.Lock()
	defer lock.Unlock()

	// The protocol message has already been validated. Record the fact that we got the Ack.

	if ag, err := FindSingleAgreementByAgreementId(a.db, wi.Update.AgreementId(), cph.Name(), []AFilter{UnarchivedAFilter()}); err != nil {
		glog.Errorf(logstring(workerID, fmt.Sprintf("error querying agreement %v, error: %v", wi.Update.AgreementId(), err)))
	} else if ag == nil {
		glog.V(3).Infof(logstring(workerID, fmt.Sprintf("agreement %v no longer active.", wi.Update.AgreementId())))
	} else if ag.AgreementTimedout != 0 {
		glog.V(3).Infof(logstring(workerID, fmt.Sprintf("agreement %v terminating.", wi.Update.AgreementId())))
	} else if _, err := AgreementBlockchainUpdateAck(a.db, wi.Update.AgreementId(), cph.Name()); err != nil {
		glog.Errorf(logstring(workerID, fmt.Sprintf("error hardening consumer update ack for agreement %v, error: %v", wi.Update.AgreementId(), err)))
	}

	// Get rid of the exchange message if there is one
	if wi.MessageId != 0 && !deletedMessage {
		if err := cph.DeleteMessage(wi.MessageId); err != nil {
			glog.Errorf(logstring(workerID, fmt.Sprintf("error deleting message %v from exchange for agbot %v", wi.MessageId, cph.GetExchangeId())))
		}
	}

}

var logstring = func(workerID string, v interface{}) string {
	return fmt.Sprintf("CSAgreementWorker (%v): %v", workerID, v)
}
