package governance

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/citizenscientist"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/ethblockchain"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/worker"
    "repo.hovitos.engineering/MTN/go-policy"
	"net/http"
	"runtime"
	"time"
)

// TODO: make this module more aware of long-running setup operations like torrent downloading and dockerfile loading
// the max time we'll let a contract remain unconfigured by the provider
const MAX_CONTRACT_UNCONFIGURED_TIME_M = 20

const MAX_CONTRACT_PRELAUNCH_TIME_M = 60

const MAX_MICROPAYMENT_UNPAID_RUN_DURATION_M = 60

// enforced only after the workloads are running
const MAX_AGREEMENT_ACCEPTANCE_WAIT_TIME_M = 20

// constants indicating why an agreement is cancelled
const CANCEL_NOT_FINALIZED_TIMEOUT = 200
const CANCEL_POLICY_CHANGED = 201

type GovernanceWorker struct {
	worker.Worker // embedded field
	db            *bolt.DB
	bc            *ethblockchain.BaseContracts
    deviceId     string
    deviceToken  string
}

func NewGovernanceWorker(config *config.HorizonConfig, db *bolt.DB) *GovernanceWorker {
	messages := make(chan events.Message)
	commands := make(chan worker.Command, 200)

	worker := &GovernanceWorker{

		Worker: worker.Worker{
			Manager: worker.Manager{
				Config:   config,
				Messages: messages,
			},

			Commands: commands,
		},

		db: db,
	}

	worker.start()
	return worker
}

func (w *GovernanceWorker) Messages() chan events.Message {
    return w.Worker.Manager.Messages
}

func (w *GovernanceWorker) NewEvent(incoming events.Message) {

	switch incoming.(type) {
    case *events.EdgeRegisteredExchangeMessage:
	    msg, _ := incoming.(*events.EdgeRegisteredExchangeMessage)
	    w.Commands <- NewDeviceRegisteredCommand(msg.ID(), msg.Token())


    default: //nothing
    }

	return
}

type GovernanceMessage interface {
	Event() events.Event
	AgreementId() string
	Deployment() *map[string]persistence.ServiceConfig
}

// TODO: consolidate with other governance message facts
type GovernanceMaintenanceMessage struct {
	event       events.Event
	agreementId string
	deployment  *map[string]persistence.ServiceConfig
}

func (m *GovernanceMaintenanceMessage) Event() events.Event {
	return m.event
}

func (m GovernanceMaintenanceMessage) AgreementId() string {
	return m.agreementId
}

func (m GovernanceMaintenanceMessage) Deployment() *map[string]persistence.ServiceConfig {
	return m.deployment
}

func (m GovernanceMaintenanceMessage) String() string {
	return fmt.Sprintf("event: %v, AgreementId: %v, Deployment: %v", m.event, m.agreementId, m.deployment)
}

type GovernanceCancelationMessage struct {
	GovernanceMaintenanceMessage
	events.Message
	Cause              events.EndContractCause
	PreviousAgreements []string
}

func (m *GovernanceCancelationMessage) Event() events.Event {
	return m.event
}

func (m GovernanceCancelationMessage) AgreementId() string {
	return m.agreementId
}

func (m GovernanceCancelationMessage) Deployment() *map[string]persistence.ServiceConfig {
	return m.deployment
}

func (m GovernanceCancelationMessage) String() string {
	return fmt.Sprintf("event: %v, agreementId: %v, deployment: %v, Cause: %v, PreviousAgreements (sample): %v", m.event, m.agreementId, persistence.ServiceConfigNames(m.deployment), m.Cause, cutil.FirstN(10, m.PreviousAgreements))
}

func NewGovernanceMaintenanceMessage(id events.EventId, agreementId string, deployment *map[string]persistence.ServiceConfig) *GovernanceMaintenanceMessage {
	return &GovernanceMaintenanceMessage{
		event: events.Event{
			Id: id,
		},
		agreementId: agreementId,
		deployment:  deployment,
	}
}

func NewGovernanceCancelationMessage(id events.EventId, cause events.EndContractCause, agreementId string, deployment *map[string]persistence.ServiceConfig, previousAgreements []string) *GovernanceCancelationMessage {

	govMaint := NewGovernanceMaintenanceMessage(id, agreementId, deployment)

	return &GovernanceCancelationMessage{
		GovernanceMaintenanceMessage: *govMaint,
		Cause:              cause,
		PreviousAgreements: previousAgreements,
	}
}

func (w *GovernanceWorker) governAgreements() {

    // Establish the go objects that are used to interact with the ethereum blockchain.
    // This code should probably be in the protocol library.
    acct, _ := ethblockchain.AccountId()
    dir, _ := ethblockchain.DirectoryAddress()
    if bc, err := ethblockchain.InitBaseContracts(acct, w.Worker.Manager.Config.Edge.GethURL, dir); err != nil {
        glog.Errorf(logString(fmt.Sprintf("unable to initialize platform contracts, error: %v", err)))
        return
    } else {
        w.bc = bc
    }

    // go govern
	go func() {

		protocolHandler := citizenscientist.NewProtocolHandler(w.Config.Edge.GethURL, nil)

		for {
			glog.V(4).Infof(logString(fmt.Sprintf("governing pending agreements")))

			// Create a new filter for unfinalized agreements
			notYetFinalFilter := func () persistence.EAFilter {
	            return func(a persistence.EstablishedAgreement) bool { return a.AgreementCreationTime != 0 && a.AgreementAcceptedTime != 0 && a.AgreementFinalizedTime == 0 && a.AgreementTimedout == 0 && a.CounterPartyAddress != ""}
	        }

			if establishedAgreements, err := persistence.FindEstablishedAgreements(w.db, citizenscientist.PROTOCOL_NAME, []persistence.EAFilter{notYetFinalFilter()}); err != nil {
				glog.Errorf(logString(fmt.Sprintf("Unable to retrieve not yet final agreements from database: %v. Error: %v", err, err)))
			} else {

				for _, ag := range establishedAgreements {

					// Verify that the blockchain update has occurred. If not, cancel the agreement.
					glog.V(5).Infof(logString(fmt.Sprintf("checking agreement %v for finalization.", ag.CurrentAgreementId)))
					if recorded, err := protocolHandler.VerifyAgreementRecorded(ag.CurrentAgreementId, ag.CounterPartyAddress, ag.ProposalSig, w.bc.Agreements); err != nil {
						glog.Errorf(logString(fmt.Sprintf("unable to verify agreement %v on blockchain, error: %v", ag.CurrentAgreementId, err)))
					} else if recorded {
						// Update state in the database
						if _, err := persistence.AgreementStateFinalized(w.db, ag.CurrentAgreementId, citizenscientist.PROTOCOL_NAME); err != nil {
							glog.Errorf(logString(fmt.Sprintf("error persisting agreement %v finalized: %v", ag.CurrentAgreementId, err)))
						}
						// Update state in exchange
                        if proposal, err := protocolHandler.ValidateProposal(ag.Proposal); err != nil {
                            glog.Errorf(logString(fmt.Sprintf("could not hydrate proposal, error: %v", err)))
                        } else if tcPolicy, err := policy.DemarshalPolicy(proposal.TsAndCs); err != nil {
                            glog.Errorf(logString(fmt.Sprintf("error demarshalling TsAndCs policy for agreement %v, error %v", ag.CurrentAgreementId, err)))
                        } else if err := recordProducerAgreementState(w.Config.Edge.ExchangeURL, w.deviceId, w.deviceToken, ag.CurrentAgreementId, tcPolicy.APISpecs[0].SpecRef, "Finalized Agreement"); err != nil {
							glog.Errorf(logString(fmt.Sprintf("error setting agreement %v finalized state in exchange: %v", ag.CurrentAgreementId, err)))
						}
					} else {
						glog.V(5).Infof(logString(fmt.Sprintf("detected agreement %v not yet final.", ag.CurrentAgreementId)))
						now := uint64(time.Now().Unix())
						if ag.AgreementCreationTime + w.Worker.Manager.Config.Edge.AgreementTimeoutS < now {
							// Start timing out the agreement
							glog.V(3).Infof(logString(fmt.Sprintf("detected agreement %v timed out.", ag.CurrentAgreementId)))

							// Update the database
							if _, err := persistence.AgreementStateTimedout(w.db, ag.CurrentAgreementId, citizenscientist.PROTOCOL_NAME); err != nil {
							    glog.Errorf(logString(fmt.Sprintf("error marking agreement %v timed out: %v", ag.CurrentAgreementId, err)))
							}
							// Delete from the exchange
							if err := deleteProducerAgreement(w.Config.Edge.ExchangeURL, w.deviceId, w.deviceToken, ag.CurrentAgreementId); err != nil {
							    glog.Errorf(logString(fmt.Sprintf("error deleting agreement %v in exchange: %v", ag.CurrentAgreementId, err)))
							}

							// Cancel on the blockchain
							if err := protocolHandler.TerminateAgreement(ag.CounterPartyAddress, ag.CurrentAgreementId, CANCEL_NOT_FINALIZED_TIMEOUT, w.bc.Agreements); err != nil {
								glog.Errorf(logString(fmt.Sprintf("error terminating agreement %v on the blockchain: %v", ag.CurrentAgreementId, err)))
							}

							// Delete from the database
							if err := persistence.DeleteEstablishedAgreement(w.db, ag.CurrentAgreementId, citizenscientist.PROTOCOL_NAME); err != nil {
								glog.Errorf(logString(fmt.Sprintf("error deleting terminated agreement: %v, error: %v", ag.CurrentAgreementId, err)))
							}
						}
					}
				}


				// 	// TODO: need to evaluate start time for both payment check and later execution_termination
				// 	if ag.AgreementExecutionStartTime != 0 {
				// 		glog.Infof("Evaluating agreement %v for compliance with terms.", ag.CurrentAgreementId)

				// 		// current contract, ensure workloads still running
				// 		w.Messages() <- NewGovernanceMaintenanceMessage(events.CONTAINER_MAINTAIN, ag.CurrentAgreementId, &ag.CurrentDeployment)

				// 		if ag.AgreementAcceptedTime == 0 && (int64(ag.AgreementExecutionStartTime)+(MAX_AGREEMENT_ACCEPTANCE_WAIT_TIME_M*60)) < time.Now().Unix() {
				// 			glog.Infof("Max time to wait for other party to accept agreement has passed (other party should have accepted as soon as receiving data). Releasing contract %v", ag.CurrentAgreementId)
				// 			w.Messages() <- NewGovernanceCancelationMessage(events.CONTRACT_ENDED, events.CT_TERMINATED, ag.CurrentAgreementId, &ag.CurrentDeployment, []string{})
				// 		}

				// 	} else if ag.CurrentAgreementId != "" {
				// 		// workload not started yet and in an agreement ...

				// 		if (int64(ag.AgreementCreationTime) + (MAX_CONTRACT_PRELAUNCH_TIME_M * 60)) < time.Now().Unix() {
				// 			glog.Infof("Terminating agreement %v because it hasn't been launched in max allowed time. This could be because of a workload failure.", ag.CurrentAgreementId)
				// 			w.Messages() <- NewGovernanceCancelationMessage(events.CONTRACT_ENDED, events.CT_TERMINATED, ag.CurrentAgreementId, &ag.CurrentDeployment, []string{})
				// 		} else if ag.ConfigureNonce != "" && (int64(ag.AgreementCreationTime)+(MAX_CONTRACT_UNCONFIGURED_TIME_M*60)) < time.Now().Unix() {
				// 			glog.Infof("Terminating agreement %v because it hasn't been configured in time.", ag.CurrentAgreementId)
				// 			w.Messages() <- NewGovernanceCancelationMessage(events.CONTRACT_ENDED, events.CT_TERMINATED, ag.CurrentAgreementId, &ag.CurrentDeployment, []string{})
				// 		}
				// 	}

				// 	// clean up old agreement resources (necessary b/c blockchain timing can be wonky)
				// 	w.Messages() <- NewGovernanceCancelationMessage(events.PREVIOUS_AGREEMENT_REAP, events.CT_TERMINATED, ag.CurrentAgreementId, &ag.CurrentDeployment, ag.PreviousAgreements)
				// }
			}

			time.Sleep(10 * time.Second) // long so we don't send duplicate cancelations
		}
	}()
}

func (w *GovernanceWorker) start() {
	go func() {
		w.governAgreements()

		for {
			glog.V(4).Infof("GovernanceWorker command processor blocking waiting to receive incoming commands")

			command := <-w.Commands
			glog.V(2).Infof("GovernanceWorker received command: %v", command)

			// TODO: consolidate DB update cases
			switch command.(type) {
            case *DeviceRegisteredCommand:
                cmd, _ := command.(*DeviceRegisteredCommand)
                w.deviceId = cmd.Id
    			w.deviceToken = cmd.Token

			case *StartGovernExecutionCommand:
				// TODO: update db start time and tc so it can be governed
				cmd, _ := command.(*StartGovernExecutionCommand)
				glog.V(3).Infof("Starting governance on resources in agreement: %v", cmd.AgreementId)

				if _, err := persistence.AgreementStateExecutionStarted(w.db, cmd.AgreementId, cmd.Protocol, cmd.Deployment); err != nil {
					glog.Errorf("Failed to update local contract record to start governing Agreement: %v. Error: %v", cmd.AgreementId, err)
				}
			}

			runtime.Gosched()
		}
	}()
}

// TODO: consolidate below
type StartGovernExecutionCommand struct {
	AgreementId string
	Protocol    string
	Deployment  *map[string]persistence.ServiceConfig
}

func (w *GovernanceWorker) NewStartGovernExecutionCommand(deployment *map[string]persistence.ServiceConfig, agreementId string, protocol string) *StartGovernExecutionCommand {
	return &StartGovernExecutionCommand{
		AgreementId: agreementId,
		Protocol: protocol,
		Deployment:  deployment,
	}
}

type CleanupExecutionCommand struct {
	AgreementId string
}

func (w *GovernanceWorker) NewCleanupExecutionCommand(agreementId string) *CleanupExecutionCommand {
	return &CleanupExecutionCommand{
		AgreementId: agreementId,
	}
}

type DeviceRegisteredCommand struct {
    Id    string
    Token string
}

func NewDeviceRegisteredCommand(id string, token string) *DeviceRegisteredCommand {
    return &DeviceRegisteredCommand{
        Id: id,
        Token: token,
    }
}

func recordProducerAgreementState(url string, deviceId string, token string, agreementId string, microservice string, state string) error {

    glog.V(5).Infof(logString(fmt.Sprintf("setting agreement %v state to %v", agreementId, state)))

    as := new(exchange.PutAgreementState)
    as.Microservice = microservice
    as.State = state
    var resp interface{}
    resp = new(exchange.PostDeviceResponse)
    targetURL := url + "devices/" + deviceId + "/agreements/" + agreementId + "?token=" + token
    for {
        if err, tpErr := exchange.InvokeExchange(&http.Client{}, "PUT", targetURL, &as, &resp); err != nil {
            glog.Errorf(logString(fmt.Sprintf(err.Error())))
            return err
        } else if tpErr != nil {
            glog.Warningf(err.Error())
            time.Sleep(10 * time.Second)
            continue
        } else {
            glog.V(5).Infof(logString(fmt.Sprintf("set agreement %v to state %v", agreementId, state)))
            return nil
        }
    }

}

func deleteProducerAgreement(url string, deviceId string, token string, agreementId string) error {

    glog.V(5).Infof(logString(fmt.Sprintf("deleting agreement %v in exchange", agreementId)))

    var resp interface{}
    resp = new(exchange.PostDeviceResponse)
    targetURL := url + "devices/" + deviceId + "/agreements/" + agreementId + "?token=" + token
    for {
        if err, tpErr := exchange.InvokeExchange(&http.Client{}, "DELETE", targetURL, nil, &resp); err != nil {
            glog.Errorf(logString(fmt.Sprintf(err.Error())))
            return err
        } else if tpErr != nil {
            glog.Warningf(err.Error())
            time.Sleep(10 * time.Second)
            continue
        } else {
            glog.V(5).Infof(logString(fmt.Sprintf("deleted agreement %v from exchange", agreementId)))
            return nil
        }
    }

}

var logString = func(v interface{}) string {
    return fmt.Sprintf("GovernanceWorker: %v", v)
}
