package governance

import (
	"fmt"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/persistence"
)

type StartGovernExecutionCommand struct {
	AgreementId       string
	AgreementProtocol string
	Deployment        persistence.DeploymentConfig
}

func (g StartGovernExecutionCommand) ShortString() string {
	return fmt.Sprintf("GovernExecutionCommand: AgreementId %v, AgreementProtocol %v, Deployment %v", g.AgreementId, g.AgreementProtocol, g.Deployment.ToString())
}

func (w *GovernanceWorker) NewStartGovernExecutionCommand(deployment persistence.DeploymentConfig, protocol string, agreementId string) *StartGovernExecutionCommand {
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
	Deployment        persistence.DeploymentConfig
}

func (c CleanupExecutionCommand) ShortString() string {
	depStr := ""
	if c.Deployment != nil {
		depStr = c.Deployment.ToString()
	}
	return fmt.Sprintf("CleanupExecutionCommand: AgreementId %v, AgreementProtocol %v, Reason %v, Deployment %v", c.AgreementId, c.AgreementProtocol, c.Reason, depStr)
}

func (w *GovernanceWorker) NewCleanupExecutionCommand(protocol string, agreementId string, reason uint, deployment persistence.DeploymentConfig) *CleanupExecutionCommand {
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
type CancelAgreementCommand struct {
	AgreementId       string
	AgreementProtocol string
	Reason            uint
	ReasonDescription string
}

func (c CancelAgreementCommand) ShortString() string {

	return fmt.Sprintf("CancelAgreementCommand: AgreementId %v, AgreementProtocol %v, Reason %v, ReasonDescription: %v", c.AgreementId, c.AgreementProtocol, c.Reason, c.ReasonDescription)
}

func NewCancelAgreementCommand(agreementId string, agreementProtocol string, reason uint, desc string) *CancelAgreementCommand {
	return &CancelAgreementCommand{
		AgreementId:       agreementId,
		AgreementProtocol: agreementProtocol,
		Reason:            reason,
		ReasonDescription: desc,
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

// ==============================================================================================================
// Node heartbeat restored
type NodeHeartbeatRestoredCommand struct {
}

func (c NodeHeartbeatRestoredCommand) ShortString() string {
	return fmt.Sprintf("NodeHeartbeatRestoredCommand.")
}

func (w *GovernanceWorker) NewNodeHeartbeatRestoredCommand() *NodeHeartbeatRestoredCommand {
	return &NodeHeartbeatRestoredCommand{}
}

// ==============================================================================================================
// Node heartbeat restored
type ServiceSuspendedCommand struct {
	ServiceConfigState []events.ServiceConfigState
}

func (c ServiceSuspendedCommand) ShortString() string {
	return fmt.Sprintf("ServiceSuspendedCommand: ServiceConfigState %v.", c.ServiceConfigState)
}

func (w *GovernanceWorker) NewServiceSuspendedCommand(scs []events.ServiceConfigState) *ServiceSuspendedCommand {
	return &ServiceSuspendedCommand{ServiceConfigState: scs}
}

// ==============================================================================================================
// Update (re-generate) node side policies
type UpdatePolicyCommand struct {
	Msg *events.UpdatePolicyMessage
}

func (c UpdatePolicyCommand) ShortString() string {
	return fmt.Sprintf("UpdatePolicyCommand: msg %v.", c.Msg)
}

func (w *GovernanceWorker) NewUpdatePolicyCommand(msg *events.UpdatePolicyMessage) *UpdatePolicyCommand {
	return &UpdatePolicyCommand{Msg: msg}
}

// ==============================================================================================================
// Update (re-generate) node side policies
type NodePolicyChangedCommand struct {
	Msg *events.NodePolicyMessage
}

func (d NodePolicyChangedCommand) ShortString() string {
	return fmt.Sprintf("%v", d)
}

func NewNodePolicyChangedCommand(msg *events.NodePolicyMessage) *NodePolicyChangedCommand {
	return &NodePolicyChangedCommand{
		Msg: msg,
	}
}
