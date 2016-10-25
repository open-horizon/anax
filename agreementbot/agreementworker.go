package agreementbot

import (
    "encoding/hex"
    "encoding/json"
    "fmt"
    "github.com/boltdb/bolt"
    "github.com/golang/glog"
    "github.com/satori/go.uuid"
    "math/rand"
    "net/http"
    "github.com/open-horizon/anax/citizenscientist"
    "github.com/open-horizon/anax/config"
    "github.com/open-horizon/anax/ethblockchain"
    "github.com/open-horizon/anax/exchange"
    "github.com/open-horizon/anax/policy"
    "runtime"
    "time"
)

type CSAgreementWorker struct {
    pm         *policy.PolicyManager
    db         *bolt.DB
    config     *config.HorizonConfig
    httpClient *http.Client
    agbotId     string
    token       string
    protocol    string
}

func NewCSAgreementWorker(policyManager *policy.PolicyManager, config *config.HorizonConfig, db *bolt.DB) *CSAgreementWorker {

    p := &CSAgreementWorker{
        pm: policyManager,
        db: db,
        config: config,
        httpClient : &http.Client{},
        agbotId: config.AgreementBot.ExchangeId,
        token: config.AgreementBot.ExchangeToken,
    }

    return p
}

// These structs are the event bodies that flows from the processor to the agreement workers
const INITIATE = "INITIATE_AGREEMENT"
const REPLY    = "AGREEMENT_REPLY"
const CANCEL   = "AGREEMENT_CANCEL"
type CSAgreementWork interface {
    Type() string
}

type CSInitiateAgreement struct {
    workType        string
    ProducerPolicy *policy.Policy     // the producer policy received from the exchange
    ConsumerPolicy *policy.Policy     // the consumer policy we're matched up with
    Device         *exchange.Device   // the device entry in the exchange
}

func (c CSInitiateAgreement) Type() string {
    return c.workType
}

type CSHandleReply struct {
    workType       string
    Reply          string
}

func (c CSHandleReply) Type() string {
    return c.workType
}

type CSCancelAgreement struct {
    workType       string
    AgreementId    string
    Protocol       string
    Reason         uint
}

func (c CSCancelAgreement) Type() string {
    return c.workType
}

// These constants represent agreement cancellation reason codes
const CANCEL_NOT_FINALIZED_TIMEOUT = 200
const CANCEL_NO_DATA_RECEIVED = 201
const CANCEL_POLICY_CHANGED = 202


// This function receives an event to "make a new agreement" from the Process function, and then synchronously calls a function
// to actually work through the agreement protocol.
func (a *CSAgreementWorker) start(work chan CSAgreementWork, random *rand.Rand, bc *ethblockchain.BaseContracts) {
    workerID := uuid.NewV4().String()

    logString := func(v interface{}) string {
        return fmt.Sprintf("CSAgreementWorker (%v): %v", workerID, v)
    }

    protocolHandler := citizenscientist.NewProtocolHandler(a.config.AgreementBot.GethURL, a.pm)

    for {
        glog.V(2).Infof(logString(fmt.Sprintf("blocking for work")))
        workItem := <-work // block waiting for work
        glog.V(2).Infof(logString(fmt.Sprintf("received work: %v", workItem)))

        if workItem.Type() == INITIATE {
            wi := workItem.(CSInitiateAgreement)
            // Generate an agreement ID
            agreementId := generateAgreementId(random)
            myAddress, _ := ethblockchain.AccountId()
            agreementIdString := hex.EncodeToString(agreementId)
            glog.V(5).Infof(logString(fmt.Sprintf("using AgreementId %v", agreementIdString)))

            // Create pending agreement in database
            if err := AgreementAttempt(a.db, agreementIdString, "Citizen Scientist"); err != nil {
                glog.Errorf(logString(fmt.Sprintf("error persisting agreement attempt: %v", err)))

            // Initiate the protocol
            } else if proposal, err := protocolHandler.InitiateAgreement(agreementId, wi.ProducerPolicy, wi.ConsumerPolicy, wi.Device, myAddress); err != nil {
                glog.Errorf(logString(fmt.Sprintf("error initiating agreement: %v", err)))

                // Remove pending agreement from database
                if err := DeleteAgreement(a.db, agreementIdString); err != nil {
                    glog.Errorf(logString(fmt.Sprintf("error deleting pending agreement: %v, error %v", agreementIdString, err)))
                }

                // TODO: Publish error on the message bus

            // Update the agreement in the DB with the proposal string
            } else if pBytes, err := json.Marshal(proposal); err != nil {
                glog.Errorf(logString(fmt.Sprintf("error marshalling proposal for storage %v, error: %v", *proposal, err)))
            } else if _, err := AgreementUpdate(a.db, agreementIdString, string(pBytes), "", false); err != nil {
                glog.Errorf(logString(fmt.Sprintf("error updating agreement with proposal %v in DB, error: %v", *proposal, err)))

            // Record that the agreement was initiated, in the exchange
            } else if err := a.recordConsumerAgreementState(agreementIdString, wi.ConsumerPolicy.APISpecs[0].SpecRef, "Formed Proposal"); err != nil {
                glog.Errorf(logString(fmt.Sprintf("error setting agreement state for %v", agreementIdString)))
            }

        } else if workItem.Type() == REPLY {
            wi := workItem.(CSHandleReply)

            if reply, err := protocolHandler.ValidateReply(wi.Reply); err != nil {
                glog.V(5).Infof(logString(fmt.Sprintf("discarding message: %v", wi.Reply)))
            } else if reply.ProposalAccepted() {
                // The producer is happy with the proposal.

                // Find the saved agreement in the database
                if agreement, err := FindSingleAgreementByAgreementId(a.db, reply.AgreementId()); err != nil {
                    glog.Errorf(logString(fmt.Sprintf("error querying pending agreement %v, error: %v", reply.AgreementId(), err)))

                // Now we need to write the info to the exchange and the database
                } else if proposal, err := protocolHandler.ValidateProposal(agreement.Proposal); err != nil {
                    glog.Errorf(logString(fmt.Sprintf("error validating proposal from pending agreement %v, error: %v", reply.AgreementId(), err)))
                } else if pol, err := policy.DemarshalPolicy(proposal.TsAndCs); err != nil {
                    glog.Errorf(logString(fmt.Sprintf("error demarshalling tsandcs policy from pending agreement %v, error: %v", reply.AgreementId(), err)))

                } else if _, err := AgreementMade(a.db, reply.AgreementId(), reply.Address, reply.Signature); err != nil {
                    glog.Errorf(logString(fmt.Sprintf("error updating agreement with proposal %v in DB, error: %v", *proposal, err)))

                } else if err := a.recordConsumerAgreementState(reply.AgreementId(), pol.APISpecs[0].SpecRef, "Producer agreed"); err != nil {
                    glog.Errorf(logString(fmt.Sprintf("error setting agreement state for %v", reply.AgreementId())))

                // We need to write the info to the blockchain
                } else if err := protocolHandler.RecordAgreement(proposal, reply, bc.Agreements); err != nil {
                    glog.Errorf(logString(fmt.Sprintf("error trying to record agreement in blockchain, %v", err)))

                } else {
                    glog.V(3).Infof(logString(fmt.Sprintf("recorded agreement %v", reply.AgreementId())))
                }

            } else {
                if err := a.recordConsumerAgreementState(reply.AgreementId(), "", "Producer rejected"); err != nil {
                    glog.Errorf(logString(fmt.Sprintf("error setting agreement state for %v", reply.AgreementId())))
                }
                glog.Errorf(logString(fmt.Sprintf("received rejection from producer %v", *reply)))
            }

        } else if workItem.Type() == CANCEL {
            wi := workItem.(CSCancelAgreement)

            if ag, err := FindSingleAgreementByAgreementId(a.db, wi.AgreementId); err != nil {
                glog.Errorf(logString(fmt.Sprintf("error querying timed out agreement %v, error: %v", wi.AgreementId, err)))
            } else if err := protocolHandler.TerminateAgreement(ag.CounterPartyAddress, ag.CurrentAgreementId, wi.Reason, bc.Agreements); err != nil {
                glog.Errorf(logString(fmt.Sprintf("error terminating agreement %v on the blockchain: %v", wi.AgreementId, err)))
            }
            if err := DeleteAgreement(a.db, wi.AgreementId); err != nil {
                glog.Errorf(logString(fmt.Sprintf("error deleting terminated agreement: %v, error: %v", wi.AgreementId, err)))
            }

        } else {
            glog.Errorf(logString(fmt.Sprintf("received unknown work request: %v", workItem)))
        }

        glog.V(5).Infof(logString(fmt.Sprintf("handled work: %v", workItem)))
        runtime.Gosched()

    }
}

// This function is used to generate a random 32 byte bitstring that is used as the agreement id.
func generateAgreementId(random *rand.Rand) []byte {

    b := make([]byte, 32, 32)
    for i := range b {
        b[i] = byte(random.Intn(256))
    }
    return b
}


func (a *CSAgreementWorker) recordConsumerAgreementState(agreementId string, workloadID string, state string) error {

    glog.V(5).Infof("CSAgreementWorker setting agreement %v state to %v", agreementId, state)

    as := new(exchange.PutAgbotAgreementState)
    as.Workload = workloadID
    as.State = state
    var resp interface{}
    resp = new(exchange.PostDeviceResponse)
    targetURL := a.config.AgreementBot.ExchangeURL + "agbots/" + a.agbotId + "/agreements/" + agreementId + "?token=" + a.token
    for {
        if err, tpErr := exchange.InvokeExchange(a.httpClient, "PUT", targetURL, &as, &resp); err != nil {
            glog.Errorf(err.Error())
            return err
        } else if tpErr != nil {
            glog.Warningf(err.Error())
            time.Sleep(10 * time.Second)
            continue
        } else {
            glog.V(5).Infof("CSAgreementWorker set agreement %v to state %v", agreementId, state)
            return nil
        }
    }

}

