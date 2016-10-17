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

// enforced only after the workloads are running
const MAX_MICROPAYMENT_UNPAID_RUN_DURATION_M = 60

// enforced only after the workloads are running
const MAX_AGREEMENT_ACCEPTANCE_WAIT_TIME_M = 20

type GovernanceWorker struct {
	worker.Worker // embedded field
	db            *bolt.DB
}

func NewGovernanceWorker(config *config.Config, db *bolt.DB) *GovernanceWorker {
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

type GovernanceMessage interface {
	Event() events.Event
	ContractId() string
	AgreementId() string
	Deployment() *map[string]persistence.ServiceConfig
}

// TODO: consolidate with other governance message facts
type GovernanceMaintenanceMessage struct {
	event       events.Event
	contractId  string
	agreementId string
	deployment  *map[string]persistence.ServiceConfig
}

func (m *GovernanceMaintenanceMessage) Event() events.Event {
	return m.event
}

func (m GovernanceMaintenanceMessage) ContractId() string {
	return m.contractId
}

func (m GovernanceMaintenanceMessage) AgreementId() string {
	return m.agreementId
}

func (m GovernanceMaintenanceMessage) Deployment() *map[string]persistence.ServiceConfig {
	return m.deployment
}

func (m GovernanceMaintenanceMessage) String() string {
	return fmt.Sprintf("event: %v, ContractId: %v, AgreementId: %v, Deployment: %v", m.event, m.contractId, m.agreementId, m.deployment)
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

func (m GovernanceCancelationMessage) ContractId() string {
	return m.contractId
}

func (m GovernanceCancelationMessage) AgreementId() string {
	return m.agreementId
}

func (m GovernanceCancelationMessage) Deployment() *map[string]persistence.ServiceConfig {
	return m.deployment
}

func (m GovernanceCancelationMessage) String() string {
	return fmt.Sprintf("event: %v, contractId: %v, agreementId: %v, deployment: %v, Cause: %v, PreviousAgreements (sample): %v", m.event, m.contractId, m.agreementId, persistence.ServiceConfigNames(m.deployment), m.Cause, cutil.FirstN(10, m.PreviousAgreements))
}

func NewGovernanceMaintenanceMessage(id events.EventId, contractId string, agreementId string, deployment *map[string]persistence.ServiceConfig) *GovernanceMaintenanceMessage {
	return &GovernanceMaintenanceMessage{
		event: events.Event{
			Id: id,
		},
		contractId:  contractId,
		agreementId: agreementId,
		deployment:  deployment,
	}
}

func NewGovernanceCancelationMessage(id events.EventId, cause events.EndContractCause, contractId string, agreementId string, deployment *map[string]persistence.ServiceConfig, previousAgreements []string) *GovernanceCancelationMessage {

	govMaint := NewGovernanceMaintenanceMessage(id, contractId, agreementId, deployment)

	return &GovernanceCancelationMessage{
		GovernanceMaintenanceMessage: *govMaint,
		Cause:              cause,
		PreviousAgreements: previousAgreements,
	}
}

func (w *GovernanceWorker) governContracts() {
	go func() {
		for {
			glog.V(4).Infof("Governing contracts")

			if establishedContracts, err := persistence.FindEstablishedContracts(w.db, []persistence.ECFilter{persistence.UnarchivedECFilter()}); err != nil {
				glog.Errorf("Unable to retrieve established contracts from database: %v. Error: %v", err, err)
			} else {

				for _, contract := range establishedContracts {

					// TODO: need to evaluate start time for both payment check and later execution_termination
					if contract.AgreementExecutionStartTime != 0 {
						glog.Infof("Evaluating agreement %v for compliance with contract %v", contract.CurrentAgreementId, contract.ContractAddress)

						// get last micropayment time for this agreement
						if last, _, payer, err := persistence.LastMicropayment(w.db, contract.CurrentAgreementId); err != nil {
							glog.Error(err)
						} else if w.Config.MicropaymentEnforced {
							var latest int64
							if last == 0 {
								latest = int64(contract.AgreementExecutionStartTime)
							} else {
								latest = last
							}

							if latest+(MAX_MICROPAYMENT_UNPAID_RUN_DURATION_M*60) < time.Now().Unix() {

								// haven't been paid yet ...

								glog.Infof("Terminating agreement %v because a recent micropayment not received from payer: %v. Releasing contract %v", contract.CurrentAgreementId, payer, contract.ContractAddress)
								w.Messages <- NewGovernanceCancelationMessage(events.CONTRACT_ENDED, events.CT_TERMINATED, contract.ContractAddress, contract.CurrentAgreementId, &contract.CurrentDeployment, []string{})
							} else {
								// current contract, ensure workloads still running
								w.Messages <- NewGovernanceMaintenanceMessage(events.CONTAINER_MAINTAIN, contract.ContractAddress, contract.CurrentAgreementId, &contract.CurrentDeployment)
							}

						} else if contract.AgreementAcceptedTime == 0 && (int64(contract.AgreementExecutionStartTime)+(MAX_AGREEMENT_ACCEPTANCE_WAIT_TIME_M*60)) < time.Now().Unix() {
							glog.Infof("Max time to wait for other party to accept agreement has passed (other party should have accepted as soon as receiving data). Releasing contract %v", contract.ContractAddress)
							w.Messages <- NewGovernanceCancelationMessage(events.CONTRACT_ENDED, events.CT_TERMINATED, contract.ContractAddress, contract.CurrentAgreementId, &contract.CurrentDeployment, []string{})
						}
						// TODO: running the workload now, so should check at long interval (28 days?) if big payment has been written to blockchains; really means writing fact by blockchain worker to the DB and then enforcing it here
					} else if contract.CurrentAgreementId != "" {
						// workload not started yet and in an agreement ...

						if (int64(contract.AgreementCreationTime) + (MAX_CONTRACT_PRELAUNCH_TIME_M * 60)) < time.Now().Unix() {
							glog.Infof("Terminating agreement %v because it hasn't been launched in max allowed time. This could be because of a workload failure. Releasing contract %v", contract.CurrentAgreementId, contract.ContractAddress)
							w.Messages <- NewGovernanceCancelationMessage(events.CONTRACT_ENDED, events.CT_TERMINATED, contract.ContractAddress, contract.CurrentAgreementId, &contract.CurrentDeployment, []string{})
						} else if contract.ConfigureNonce != "" && (int64(contract.AgreementCreationTime)+(MAX_CONTRACT_UNCONFIGURED_TIME_M*60)) < time.Now().Unix() {
							glog.Infof("Terminating agreement %v because it hasn't been configured in time. Releasing contract %v", contract.CurrentAgreementId, contract.ContractAddress)
							w.Messages <- NewGovernanceCancelationMessage(events.CONTRACT_ENDED, events.CT_TERMINATED, contract.ContractAddress, contract.CurrentAgreementId, &contract.CurrentDeployment, []string{})
						}
					}

					// clean up old agreement resources (necessary b/c blockchain timing can be wonky)
					w.Messages <- NewGovernanceCancelationMessage(events.PREVIOUS_AGREEMENT_REAP, events.CT_TERMINATED, contract.ContractAddress, "", &contract.CurrentDeployment, contract.PreviousAgreements)
				}
			}

			time.Sleep(10 * time.Minute) // long so we don't send duplicate cancelations
		}
	}()
}

func (w *GovernanceWorker) start() {
	go func() {
		w.governContracts()

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

				if _, err := persistence.ContractStateExecutionStarted(w.db, cmd.ContractId, cmd.Deployment); err != nil {
					glog.Errorf("Failed to update local contract record to start governing Agreement: %v of Contract: %v. Error: %v", cmd.AgreementId, cmd.ContractId, err)
				}
			}

			runtime.Gosched()
		}
	}()
}

// TODO: consolidate below
type StartGovernExecutionCommand struct {
	ContractId  string
	AgreementId string
	Deployment  *map[string]persistence.ServiceConfig
}

func (w *GovernanceWorker) NewStartGovernExecutionCommand(deployment *map[string]persistence.ServiceConfig, contractId string, agreementId string) *StartGovernExecutionCommand {
	return &StartGovernExecutionCommand{
		ContractId:  contractId,
		AgreementId: agreementId,
		Deployment:  deployment,
	}
}

type CleanupExecutionCommand struct {
	ContractId  string
	AgreementId string
}

func (w *GovernanceWorker) NewCleanupExecutionCommand(contractId string, agreementId string) *CleanupExecutionCommand {
	return &CleanupExecutionCommand{
		ContractId:  contractId,
		AgreementId: agreementId,
	}
}
