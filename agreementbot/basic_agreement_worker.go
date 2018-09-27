package agreementbot

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/agreementbot/persistence"
	"github.com/open-horizon/anax/basicprotocol"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/worker"
	"github.com/satori/go.uuid"
	"math/rand"
	"runtime"
)

type BasicAgreementWorker struct {
	*BaseAgreementWorker
	protocolHandler *BasicProtocolHandler
}

func NewBasicAgreementWorker(c *BasicProtocolHandler, cfg *config.HorizonConfig, db persistence.AgbotDatabase, pm *policy.PolicyManager, alm *AgreementLockManager) *BasicAgreementWorker {

	p := &BasicAgreementWorker{
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

// These are work items that represent extensions to the protocol.
const AGREEMENT_VERIFICATION = "AGREEMENT_VERIFY"

type BAgreementVerification struct {
	workType     string
	Verify       basicprotocol.BAgreementVerify
	From         string // deprecated whisper address
	SenderId     string // exchange Id of sender
	SenderPubKey []byte
	MessageId    int
}

func (b BAgreementVerification) Type() string {
	return b.workType
}

func (b BAgreementVerification) String() string {
	pkey := "not set"
	if len(b.SenderPubKey) != 0 {
		pkey = "set"
	}
	return fmt.Sprintf("WorkType: %v, "+
		"Verify: %v, "+
		"MsgEndpoint: %v, "+
		"SenderId: %v, "+
		"SenderPubKey: %v, "+
		"MessageId: %v",
		b.workType, b.Verify, b.From, b.SenderId, pkey, b.MessageId)
}

// This function receives an event to "make a new agreement" from the Process function, and then synchronously calls a function
// to actually work through the agreement protocol.
func (a *BasicAgreementWorker) start(work chan AgreementWork, random *rand.Rand) {

	worker.GetWorkerStatusManager().SetSubworkerStatus("BasicProtocolHandler", a.workerID, worker.STATUS_STARTED)
	for {
		glog.V(5).Infof(bwlogstring(a.workerID, fmt.Sprintf("blocking for work")))
		workItem := <-work // block waiting for work
		glog.V(2).Infof(bwlogstring(a.workerID, fmt.Sprintf("received work: %v", workItem)))

		if workItem.Type() == INITIATE {
			wi := workItem.(InitiateAgreement)
			a.InitiateNewAgreement(a.protocolHandler, &wi, random, a.workerID)

		} else if workItem.Type() == REPLY {
			wi := workItem.(HandleReply)
			if ok := a.HandleAgreementReply(a.protocolHandler, &wi, a.workerID); ok {
				// Update state in the database
				if ag, err := a.db.AgreementFinalized(wi.Reply.AgreementId(), a.protocolHandler.Name()); err != nil {
					glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error persisting agreement %v finalized: %v", wi.Reply.AgreementId(), err)))

					// Update state in exchange
				} else if pol, err := policy.DemarshalPolicy(ag.Policy); err != nil {
					glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error demarshalling policy from agreement %v, error: %v", wi.Reply.AgreementId(), err)))
				} else if err := a.protocolHandler.RecordConsumerAgreementState(wi.Reply.AgreementId(), pol, ag.Org, "Finalized Agreement", a.workerID); err != nil {
					glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error setting agreement %v finalized state in exchange: %v", wi.Reply.AgreementId(), err)))
				}
			}

		} else if workItem.Type() == DATARECEIVEDACK {
			wi := workItem.(HandleDataReceivedAck)
			a.HandleDataReceivedAck(a.protocolHandler, &wi, a.workerID)

		} else if workItem.Type() == CANCEL {
			wi := workItem.(CancelAgreement)
			a.CancelAgreementWithLock(a.protocolHandler, wi.AgreementId, wi.Reason, a.workerID)

			// Get rid of the original agreement cancellation message.
			if wi.MessageId != 0 {
				if err := a.protocolHandler.DeleteMessage(wi.MessageId); err != nil {
					glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error deleting message %v from exchange", wi.MessageId)))
				}
			}

		} else if workItem.Type() == WORKLOAD_UPGRADE {
			// upgrade a workload on a device
			wi := workItem.(HandleWorkloadUpgrade)
			a.HandleWorkloadUpgrade(a.protocolHandler, &wi, a.workerID)

		} else if workItem.Type() == ASYNC_CANCEL {
			wi := workItem.(AsyncCancelAgreement)
			a.ExternalCancel(a.protocolHandler, wi.AgreementId, wi.Reason, a.workerID)

		} else if workItem.Type() == AGREEMENT_VERIFICATION {
			wi := workItem.(BAgreementVerification)

			// Archived and terminating agreements are considered to be non-existent.
			agreement, err := a.db.FindSingleAgreementByAgreementId(wi.Verify.AgreementId(), a.protocolHandler.Name(), []persistence.AFilter{persistence.UnarchivedAFilter()})
			if err != nil {
				glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error querying agreement %v, error: %v", wi.Verify.AgreementId(), err)))
			}

			exists := false
			if agreement != nil && agreement.AgreementTimedout == 0 {
				exists = true
			}

			// Reply to the sender with our decision on the agreement.
			if mt, err := exchange.CreateMessageTarget(wi.SenderId, nil, wi.SenderPubKey, wi.From); err != nil {
				glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error creating message target: %v", err)))
			} else if aph, ok := a.protocolHandler.AgreementProtocolHandler("", "", "").(*basicprotocol.ProtocolHandler); !ok {
				glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error casting to basic protocol handler (%T): %v", a.protocolHandler.AgreementProtocolHandler("", "", ""), err)))
			} else if err := aph.SendAgreementVerificationReply(wi.Verify.AgreementId(), exists, mt, a.protocolHandler.GetSendMessage()); err != nil {
				glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error trying to send agreement verification reply for %v to %v, error: %v", wi.Verify.AgreementId(), mt, err)))
			}

			// Get rid of the original agreement validation request message.
			if wi.MessageId != 0 {
				if err := a.protocolHandler.DeleteMessage(wi.MessageId); err != nil {
					glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error deleting message %v from exchange", wi.MessageId)))
				}
			}

		} else if workItem.Type() == STOP {
			// At this point, we assume that the parent agreement bot worker has already decided that it's ok to terminate.
			break

		} else {
			glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("received unknown work request: %v", workItem)))
		}

		glog.V(5).Infof(bwlogstring(a.workerID, fmt.Sprintf("handled work: %v", workItem)))
		runtime.Gosched()

	}

	glog.V(5).Infof(bwlogstring(a.workerID, fmt.Sprintf("terminating")))

}

var bwlogstring = func(workerID string, v interface{}) string {
	return fmt.Sprintf("BasicAgreementWorker (%v): %v", workerID, v)
}
