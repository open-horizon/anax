package compcheck

import (
	"fmt"
	"github.com/open-horizon/anax/exchange"
)

// an implementation of common.AbstractServiceFile
type ServiceDefinition struct {
	Org string `json:"org"`
	exchange.ServiceDefinition
}

func (s *ServiceDefinition) GetOrg() string {
	return s.Org
}

func (s *ServiceDefinition) GetURL() string {
	return s.URL
}

func (s *ServiceDefinition) GetVersion() string {
	return s.Version
}

func (s *ServiceDefinition) GetArch() string {
	return s.Arch
}

func (s *ServiceDefinition) GetRequiredServices() []exchange.ServiceDependency {
	return s.RequiredServices
}

func (s *ServiceDefinition) GetUserInputs() []exchange.UserInput {
	return s.UserInputs
}

func (s *ServiceDefinition) NeedsUserInput() bool {
	if s.UserInputs == nil || len(s.UserInputs) == 0 {
		return false
	}

	for _, ui := range s.UserInputs {
		if ui.Name != "" && ui.DefaultValue == "" {
			return true
		}
	}
	return false
}

func (s *ServiceDefinition) GetDeployment() interface{} {
	return s.Deployment
}

func (s *ServiceDefinition) GetClusterDeployment() interface{} {
	return s.ClusterDeployment
}

type ServiceSpec struct {
	ServiceOrgid        string `json:"serviceOrgid"`
	ServiceUrl          string `json:"serviceUrl"`
	ServiceArch         string `json:"serviceArch"`
	ServiceVersionRange string `json:"serviceVersionRange"` // version or version range. empty string means it applies to all versions
}

func (s ServiceSpec) String() string {
	return fmt.Sprintf("ServiceOrgid: %v, "+
		"ServiceUrl: %v, "+
		"ServiceArch: %v, "+
		"ServiceVersionRange: %v",
		s.ServiceOrgid, s.ServiceUrl, s.ServiceArch, s.ServiceVersionRange)
}

func NewServiceSpec(svcName, svcOrg, svcVersion, svcArch string) *ServiceSpec {
	return &ServiceSpec{
		ServiceOrgid:        svcOrg,
		ServiceUrl:          svcName,
		ServiceArch:         svcArch,
		ServiceVersionRange: svcVersion,
	}
}
