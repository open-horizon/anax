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

type BasicAgreementWorker struct {
    *BaseAgreementWorker
    protocolHandler *BasicProtocolHandler
}

func NewBasicAgreementWorker(c *BasicProtocolHandler, cfg *config.HorizonConfig, db *bolt.DB, pm *policy.PolicyManager, alm *AgreementLockManager) *BasicAgreementWorker {

    p := &BasicAgreementWorker{
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

// This function receives an event to "make a new agreement" from the Process function, and then synchronously calls a function
// to actually work through the agreement protocol.
func (a *BasicAgreementWorker) start(work chan AgreementWork, random *rand.Rand) {

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
                if ag, err := AgreementFinalized(a.db, wi.Reply.AgreementId(), a.protocolHandler.Name()); err != nil {
                    glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error persisting agreement %v finalized: %v", wi.Reply.AgreementId(), err)))

                // Update state in exchange
                } else if pol, err := policy.DemarshalPolicy(ag.Policy); err != nil {
                    glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error demarshalling policy from agreement %v, error: %v", wi.Reply.AgreementId(), err)))
                } else if err := a.protocolHandler.RecordConsumerAgreementState(wi.Reply.AgreementId(), pol, "Finalized Agreement", a.workerID); err != nil {
                    glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("error setting agreement %v finalized state in exchange: %v", wi.Reply.AgreementId(), err)))
                }
            }

        } else if workItem.Type() == DATARECEIVEDACK {
            wi := workItem.(HandleDataReceivedAck)
            a.HandleDataReceivedAck(a.protocolHandler, &wi, a.workerID)

        } else if workItem.Type() == CANCEL {
            wi := workItem.(CancelAgreement)
            a.CancelAgreementWithLock(a.protocolHandler, wi.AgreementId, wi.Reason, a.workerID)

        } else if workItem.Type() == WORKLOAD_UPGRADE {
            // upgrade a workload on a device
            wi := workItem.(HandleWorkloadUpgrade)
            a.HandleWorkloadUpgrade(a.protocolHandler, &wi, a.workerID)

        } else if workItem.Type() == ASYNC_CANCEL {
            wi := workItem.(AsyncCancelAgreement)
            a.ExternalCancel(a.protocolHandler, wi.AgreementId, wi.Reason, a.workerID)

        } else {
            glog.Errorf(bwlogstring(a.workerID, fmt.Sprintf("received unknown work request: %v", workItem)))
        }

        glog.V(5).Infof(bwlogstring(a.workerID, fmt.Sprintf("handled work: %v", workItem)))
        runtime.Gosched()

    }
}

var bwlogstring = func(workerID string, v interface{}) string {
    return fmt.Sprintf("BasicAgreementWorker (%v): %v", workerID, v)
}
