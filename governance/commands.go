package governance

import (
	"fmt"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/persistence"
)

type StartGovernExecutionCommand struct {
	AgreementId       string
	AgreementProtocol string
	Deployment        map[string]persistence.ServiceConfig
}

func (g StartGovernExecutionCommand) ShortString() string {
	depStr := ""
	for key, _ := range g.Deployment {
		depStr = depStr + key + ","
	}

	return fmt.Sprintf("GovernExecutionCommand: AgreementId %v, AgreementProtocol %v, Deployed Services %v", g.AgreementId, g.AgreementProtocol, depStr)
}

func (w *GovernanceWorker) NewStartGovernExecutionCommand(deployment map[string]persistence.ServiceConfig, protocol string, agreementId string) *StartGovernExecutionCommand {
	return &StartGovernExecutionCommand{
		AgreementId:       agreementId,
		AgreementProtocol: protocol,
		Deployment:        deployment,
	}
}

// ==============================================================================================================
type CleanupExecutionCommand struct {
	AgreementProtocol string
	AgreementId       string
	Reason            uint
	Deployment        map[string]persistence.ServiceConfig
}

func (c CleanupExecutionCommand) ShortString() string {
	depStr := ""
	for key, _ := range c.Deployment {
		depStr = depStr + key + ","
	}

	return fmt.Sprintf("CleanupExecutionCommand: AgreementId %v, AgreementProtocol %v, Reason %v, Deployed Services %v", c.AgreementId, c.AgreementProtocol, c.Reason, depStr)
}

func (w *GovernanceWorker) NewCleanupExecutionCommand(protocol string, agreementId string, reason uint, deployment map[string]persistence.ServiceConfig) *CleanupExecutionCommand {
	return &CleanupExecutionCommand{
		AgreementProtocol: protocol,
		AgreementId:       agreementId,
		Reason:            reason,
		Deployment:        deployment,
	}
}

// ==============================================================================================================
type CleanupStatusCommand struct {
	AgreementProtocol string
	AgreementId       string
	Status            uint
}

func (c CleanupStatusCommand) ShortString() string {

	return fmt.Sprintf("CleanupStatusCommand: AgreementId %v, AgreementProtocol %v, Status %v", c.AgreementId, c.AgreementProtocol, c.Status)
}

func (w *GovernanceWorker) NewCleanupStatusCommand(protocol string, agreementId string, status uint) *CleanupStatusCommand {
	return &CleanupStatusCommand{
		AgreementProtocol: protocol,
		AgreementId:       agreementId,
		Status:            status,
	}
}

// ==============================================================================================================
type AsyncTerminationCommand struct {
	AgreementId       string
	AgreementProtocol string
	Reason            uint
}

func (c AsyncTerminationCommand) ShortString() string {

	return fmt.Sprintf("AsyncTerminationCommand: AgreementId %v, AgreementProtocol %v, Reason %v", c.AgreementId, c.AgreementProtocol, c.Reason)
}

func NewAsyncTerminationCommand(agreementId string, agreementProtocol string, reason uint) *AsyncTerminationCommand {
	return &AsyncTerminationCommand{
		AgreementId:       agreementId,
		AgreementProtocol: agreementProtocol,
		Reason:            reason,
	}
}

// ==============================================================================================================
type UpdateMicroserviceCommand struct {
	MsInstKey            string // the name that was passed into the ContainerLaunchContext, it is the key to the MicroserviceInstance table.
	ExecutionStarted     bool
	ExecutionFailureCode uint
	ExecutionFailureDesc string
}

func (c UpdateMicroserviceCommand) ShortString() string {
	return fmt.Sprintf("UpdateServiceCommand: MsInstKey %v, ExecutionStarted %v, ExecutionFailureCode %v, ExecutionFailureDesc %v",
		c.MsInstKey, c.ExecutionStarted, c.ExecutionFailureCode, c.ExecutionFailureDesc)
}

func (w *GovernanceWorker) NewUpdateMicroserviceCommand(key string, started bool, failure_code uint, failure_desc string) *UpdateMicroserviceCommand {
	return &UpdateMicroserviceCommand{
		MsInstKey:            key,
		ExecutionStarted:     started,      // true for EXECUTION_BEGUN, false for EXECUTION_FAILED and CONTAINER_DESTROYED case
		ExecutionFailureCode: failure_code, // 0 for EXECUTION_BEGUN and CONTAINER_DESTROYED case
		ExecutionFailureDesc: failure_desc,
	}
}

// ==============================================================================================================
type ReportDeviceStatusCommand struct {
}

func (c ReportDeviceStatusCommand) ShortString() string {
	return fmt.Sprintf("ReportDeviceStatusCommand")
}

func (w *GovernanceWorker) NewReportDeviceStatusCommand() *ReportDeviceStatusCommand {
	return &ReportDeviceStatusCommand{}
}

// ==============================================================================================================
type NodeShutdownCommand struct {
	Msg *events.NodeShutdownMessage
}

func (n NodeShutdownCommand) ShortString() string {
	return fmt.Sprintf("NodeShutdownCommand Msg: %v", n.Msg)
}

func (w *GovernanceWorker) NewNodeShutdownCommand(msg *events.NodeShutdownMessage) *NodeShutdownCommand {
	return &NodeShutdownCommand{
		Msg: msg,
	}
}

// ==============================================================================================================
// Upgrade the given microservice if needed
type UpgradeMicroserviceCommand struct {
	MsDefId string
}

func (c UpgradeMicroserviceCommand) ShortString() string {
	return fmt.Sprintf("UpgradeServiceCommand: MsDefId %v", c.MsDefId)
}

func (w *GovernanceWorker) NewUpgradeMicroserviceCommand(msdef_id string) *UpgradeMicroserviceCommand {
	return &UpgradeMicroserviceCommand{
		MsDefId: msdef_id,
	}
}

// ==============================================================================================================
// Start agreement-less services
type StartAgreementLessServicesCommand struct {
}

func (c StartAgreementLessServicesCommand) ShortString() string {
	return fmt.Sprintf("StartAgreementLessServicesCommand")
}

func (w *GovernanceWorker) NewStartAgreementLessServicesCommand() *StartAgreementLessServicesCommand {
	return &StartAgreementLessServicesCommand{}
}
