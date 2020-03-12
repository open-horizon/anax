package kube_operator

import (
	"fmt"
	"github.com/open-horizon/anax/persistence"
)

type InstallCommand struct {
	LaunchContext interface{}
}

func (i InstallCommand) ShortString() string {
	return fmt.Sprintf("%v", i)
}

func NewInstallCommand(launchContext interface{}) *InstallCommand {
	return &InstallCommand{
		LaunchContext: launchContext,
	}
}

type UnInstallCommand struct {
	AgreementProtocol  string
	CurrentAgreementId string
	Deployment         persistence.DeploymentConfig
}

func (u UnInstallCommand) ShortString() string {
	return fmt.Sprintf("%v", u)
}

func NewUnInstallCommand(agp string, agId string, dc persistence.DeploymentConfig) *UnInstallCommand {
	return &UnInstallCommand{
		AgreementProtocol:  agp,
		CurrentAgreementId: agId,
		Deployment:         dc,
	}
}

type MaintenanceCommand struct {
	AgreementProtocol string
	AgreementId       string
	Deployment        persistence.DeploymentConfig
}

func (c MaintenanceCommand) String() string {
	deployment_string := ""
	if c.Deployment != nil {
		deployment_string = c.Deployment.ToString()
	}
	return fmt.Sprintf("AgreementProtocol: %v, AgreementId: %v, Deployment: %v", c.AgreementProtocol, c.AgreementId, deployment_string)
}

func (c MaintenanceCommand) ShortString() string {
	return c.String()
}

func NewMaintenanceCommand(protocol string, agreementId string, deployment persistence.DeploymentConfig) *MaintenanceCommand {
	return &MaintenanceCommand{
		AgreementProtocol: protocol,
		AgreementId:       agreementId,
		Deployment:        deployment,
	}
}
