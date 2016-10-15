package governance

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/worker"
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

type GovernanceWorker struct {
	worker.Worker // embedded field
	db            *bolt.DB
}

func NewGovernanceWorker(config *config.HorizonConfig, db *bolt.DB) *GovernanceWorker {
	messages := make(chan events.Message)
	commands := make(chan worker.Command, 200)

	worker := &GovernanceWorker{

		worker.Worker{
			Manager: worker.Manager{
				Config:   config,
				Messages: messages,
			},

			Commands: commands,
		},

		db,
	}
	worker.start()
	return worker
}

func (w *GovernanceWorker) Messages() chan events.Message {
    return w.Worker.Manager.Messages
}

func (w *GovernanceWorker) NewEvent(incoming events.Message) {
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
	go func() {
		for {
			glog.V(4).Infof("Governing Agreements")

			if establishedAgreements, err := persistence.FindEstablishedAgreements(w.db, []persistence.ECFilter{persistence.UnarchivedECFilter()}); err != nil {
				glog.Errorf("Unable to retrieve established agreements from database: %v. Error: %v", err, err)
			} else {

				for _, ag := range establishedAgreements {

					// TODO: need to evaluate start time for both payment check and later execution_termination
					if ag.AgreementExecutionStartTime != 0 {
						glog.Infof("Evaluating agreement %v for compliance with terms.", ag.CurrentAgreementId)

						// current contract, ensure workloads still running
						w.Messages() <- NewGovernanceMaintenanceMessage(events.CONTAINER_MAINTAIN, ag.CurrentAgreementId, &ag.CurrentDeployment)

						if ag.AgreementAcceptedTime == 0 && (int64(ag.AgreementExecutionStartTime)+(MAX_AGREEMENT_ACCEPTANCE_WAIT_TIME_M*60)) < time.Now().Unix() {
							glog.Infof("Max time to wait for other party to accept agreement has passed (other party should have accepted as soon as receiving data). Releasing contract %v", ag.CurrentAgreementId)
							w.Messages() <- NewGovernanceCancelationMessage(events.CONTRACT_ENDED, events.CT_TERMINATED, ag.CurrentAgreementId, &ag.CurrentDeployment, []string{})
						}

					} else if ag.CurrentAgreementId != "" {
						// workload not started yet and in an agreement ...

						if (int64(ag.AgreementCreationTime) + (MAX_CONTRACT_PRELAUNCH_TIME_M * 60)) < time.Now().Unix() {
							glog.Infof("Terminating agreement %v because it hasn't been launched in max allowed time. This could be because of a workload failure.", ag.CurrentAgreementId)
							w.Messages() <- NewGovernanceCancelationMessage(events.CONTRACT_ENDED, events.CT_TERMINATED, ag.CurrentAgreementId, &ag.CurrentDeployment, []string{})
						} else if ag.ConfigureNonce != "" && (int64(ag.AgreementCreationTime)+(MAX_CONTRACT_UNCONFIGURED_TIME_M*60)) < time.Now().Unix() {
							glog.Infof("Terminating agreement %v because it hasn't been configured in time.", ag.CurrentAgreementId)
							w.Messages() <- NewGovernanceCancelationMessage(events.CONTRACT_ENDED, events.CT_TERMINATED, ag.CurrentAgreementId, &ag.CurrentDeployment, []string{})
						}
					}

					// clean up old agreement resources (necessary b/c blockchain timing can be wonky)
					w.Messages() <- NewGovernanceCancelationMessage(events.PREVIOUS_AGREEMENT_REAP, events.CT_TERMINATED, ag.CurrentAgreementId, &ag.CurrentDeployment, ag.PreviousAgreements)
				}
			}

			time.Sleep(10 * time.Minute) // long so we don't send duplicate cancelations
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
			case *StartGovernExecutionCommand:
				// TODO: update db start time and tc so it can be governed
				cmd, _ := command.(*StartGovernExecutionCommand)
				glog.V(3).Infof("Starting governance on resources in agreement: %v", cmd.AgreementId)

				if _, err := persistence.AgreementStateExecutionStarted(w.db, cmd.AgreementId, cmd.Deployment); err != nil {
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
	Deployment  *map[string]persistence.ServiceConfig
}

func (w *GovernanceWorker) NewStartGovernExecutionCommand(deployment *map[string]persistence.ServiceConfig, agreementId string) *StartGovernExecutionCommand {
	return &StartGovernExecutionCommand{
		AgreementId: agreementId,
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
