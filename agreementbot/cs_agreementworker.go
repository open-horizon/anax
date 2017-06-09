package agreementbot

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/policy"
	"github.com/satori/go.uuid"
	"math/rand"
	"net/http"
	"runtime"
	"time"
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
			httpClient: &http.Client{Timeout: time.Duration(config.HTTPDEFAULTTIMEOUT * time.Millisecond)},
		},
		protocolHandler: c,
	}

	return p
}

// // These structs are the work items that flow to the agreement workers

const BC_RECORDED = "AGREEMENT_BC_RECORDED"
const BC_TERMINATED = "AGREEMENT_BC_TERMINATED"

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
				go a.DoBlockchainCancel(a.protocolHandler, ag, ag.TerminatedReason, a.workerID)
			} else {
				// Update state in the database
				if _, err := AgreementFinalized(a.protocolHandler.db, wi.AgreementId, a.protocolHandler.Name()); err != nil {
					glog.Errorf(logstring(a.workerID, fmt.Sprintf("error persisting agreement %v finalized: %v", wi.AgreementId, err)))
				}

				// Update state in exchange
				if pol, err := policy.DemarshalPolicy(ag.Policy); err != nil {
					glog.Errorf(logstring(a.workerID, fmt.Sprintf("error demarshalling policy from agreement %v, error: %v", wi.AgreementId, err)))
				} else if err := a.protocolHandler.RecordConsumerAgreementState(wi.AgreementId, pol.APISpecs[0].SpecRef, "Finalized Agreement", a.workerID); err != nil {
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

		} else {
			glog.Errorf(logstring(a.workerID, fmt.Sprintf("received unknown work request: %v", workItem)))
		}

		glog.V(5).Infof(logstring(a.workerID, fmt.Sprintf("handled work: %v", workItem)))
		runtime.Gosched()

	}
}

var logstring = func(workerID string, v interface{}) string {
	return fmt.Sprintf("CSAgreementWorker (%v): %v", workerID, v)
}
