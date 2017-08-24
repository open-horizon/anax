package container

import (
	"fmt"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/persistence"
)

// ==============================================================================================================
type WorkloadConfigureCommand struct {
	ImageFiles             []string
	AgreementLaunchContext *events.AgreementLaunchContext
}

func (c WorkloadConfigureCommand) String() string {
	return fmt.Sprintf("AgreementLaunchContext: %v, ImageFiles: %v", c.AgreementLaunchContext, c.ImageFiles)
}

func (c WorkloadConfigureCommand) ShortString() string {
	return c.String()
}

func (b *ContainerWorker) NewWorkloadConfigureCommand(imageFiles []string, agreementLaunchContext *events.AgreementLaunchContext) *WorkloadConfigureCommand {
	return &WorkloadConfigureCommand{
		ImageFiles:             imageFiles,
		AgreementLaunchContext: agreementLaunchContext,
	}
}

// ==============================================================================================================
type ContainerConfigureCommand struct {
	ImageFiles             []string
	ContainerLaunchContext *events.ContainerLaunchContext
}

func (c ContainerConfigureCommand) String() string {
	return fmt.Sprintf("ContainerLaunchContext: %v, ImageFiles: %v", c.ContainerLaunchContext, c.ImageFiles)
}

func (c ContainerConfigureCommand) ShortString() string {
	return c.String()
}

func (b *ContainerWorker) NewContainerConfigureCommand(imageFiles []string, containerLaunchContext *events.ContainerLaunchContext) *ContainerConfigureCommand {
	return &ContainerConfigureCommand{
		ImageFiles:             imageFiles,
		ContainerLaunchContext: containerLaunchContext,
	}
}

// ==============================================================================================================
type ContainerMaintenanceCommand struct {
	AgreementProtocol string
	AgreementId       string
	Deployment        map[string]persistence.ServiceConfig
}

func (c ContainerMaintenanceCommand) String() string {
	return fmt.Sprintf("AgreementProtocol: %v, AgreementId: %v, Deployment: %v", c.AgreementProtocol, c.AgreementId, persistence.ServiceConfigNames(&c.Deployment))
}

func (c ContainerMaintenanceCommand) ShortString() string {
	return c.String()
}

func (b *ContainerWorker) NewContainerMaintenanceCommand(protocol string, agreementId string, deployment map[string]persistence.ServiceConfig) *ContainerMaintenanceCommand {
	return &ContainerMaintenanceCommand{
		AgreementProtocol: protocol,
		AgreementId:       agreementId,
		Deployment:        deployment,
	}
}

// ==============================================================================================================
type WorkloadShutdownCommand struct {
	AgreementProtocol  string
	CurrentAgreementId string
	Deployment         map[string]persistence.ServiceConfig
	Agreements         []string
}

func (c WorkloadShutdownCommand) String() string {
	return fmt.Sprintf("AgreementProtocol: %v, CurrentAgreementId: %v, Deployment: %v, Agreements (sample): %v", c.AgreementProtocol, c.CurrentAgreementId, persistence.ServiceConfigNames(&c.Deployment), cutil.FirstN(10, c.Agreements))
}

func (c WorkloadShutdownCommand) ShortString() string {
	return c.String()
}

func (b *ContainerWorker) NewWorkloadShutdownCommand(protocol string, currentAgreementId string, deployment map[string]persistence.ServiceConfig, agreements []string) *WorkloadShutdownCommand {
	return &WorkloadShutdownCommand{
		AgreementProtocol:  protocol,
		CurrentAgreementId: currentAgreementId,
		Deployment:         deployment,
		Agreements:         agreements,
	}
}

// ==============================================================================================================
type ContainerStopCommand struct {
	Msg events.ContainerStopMessage
}

func (c ContainerStopCommand) String() string {
	return fmt.Sprintf("Msg: %v", c.Msg)
}

func (c ContainerStopCommand) ShortString() string {
	return c.String()
}

func (b *ContainerWorker) NewContainerStopCommand(msg *events.ContainerStopMessage) *ContainerStopCommand {
	return &ContainerStopCommand{
		Msg: *msg,
	}
}

// ==============================================================================================================
type MaintainMicroserviceCommand struct {
	MsInstKey string // the name that was passed into the ContainerLaunchContext, it is the key to the MicroserviceInstance table.
}

func (c MaintainMicroserviceCommand) ShortString() string {
	return fmt.Sprintf("MaintainMicroserviceCommand: MsInstKey %v", c.MsInstKey)
}

func (b *ContainerWorker) NewMaintainMicroserviceCommand(key string) *MaintainMicroserviceCommand {
	return &MaintainMicroserviceCommand{
		MsInstKey: key,
	}
}

// ==============================================================================================================
type ShutdownMicroserviceCommand struct {
	MsInstKey string // key to the MicroserviceInstance table.
}

func (c ShutdownMicroserviceCommand) ShortString() string {
	return fmt.Sprintf("MaintainMicroserviceCommand: MsInstKey %v", c.MsInstKey)
}

func (b *ContainerWorker) NewShutdownMicroserviceCommand(key string) *ShutdownMicroserviceCommand {
	return &ShutdownMicroserviceCommand{
		MsInstKey: key,
	}
}
