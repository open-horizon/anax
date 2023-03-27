package kube_operator

import (
	"fmt"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/persistence"
)

type InstallCommand struct {
	LaunchContext interface{}
}

func (i InstallCommand) ShortString() string {
	lc := ""
	lcObj := events.GetLaunchContext(i.LaunchContext)
	if lcObj != nil {
		lc = lcObj.ShortString()
	}
	return fmt.Sprintf("LaunchContext: %v", lc)
}

func NewInstallCommand(launchContext interface{}) *InstallCommand {
	return &InstallCommand{
		LaunchContext: launchContext,
	}
}

type UnInstallCommand struct {
	AgreementProtocol  string
	CurrentAgreementId string
	ClusterNamespace   string
	Deployment         persistence.DeploymentConfig
}

func (u UnInstallCommand) ShortString() string {
	return fmt.Sprintf("%v", u)
}

func NewUnInstallCommand(agp string, agId string, clusterNamespace string, dc persistence.DeploymentConfig) *UnInstallCommand {
	return &UnInstallCommand{
		AgreementProtocol:  agp,
		CurrentAgreementId: agId,
		ClusterNamespace:   clusterNamespace,
		Deployment:         dc,
	}
}

type MaintenanceCommand struct {
	AgreementProtocol string
	AgreementId       string
	ClusterNamespace  string
	Deployment        persistence.DeploymentConfig
}

func (c MaintenanceCommand) String() string {
	deployment_string := ""
	if c.Deployment != nil {
		deployment_string = c.Deployment.ToString()
	}
	return fmt.Sprintf("AgreementProtocol: %v, AgreementId: %v, ClusterNamespace: %v, Deployment: %v", c.AgreementProtocol, c.AgreementId, c.ClusterNamespace, deployment_string)
}

func (c MaintenanceCommand) ShortString() string {
	return c.String()
}

func NewMaintenanceCommand(protocol string, agreementId string, clusterNamespace string, deployment persistence.DeploymentConfig) *MaintenanceCommand {
	return &MaintenanceCommand{
		AgreementProtocol: protocol,
		AgreementId:       agreementId,
		ClusterNamespace:  clusterNamespace,
		Deployment:        deployment,
	}
}
