package common

import (
	"errors"
	"fmt"
	"github.com/open-horizon/anax/containermessage"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/semanticversion"
	"golang.org/x/text/message"
)

type AbstractServiceFile interface {
	GetOrg() string
	GetURL() string
	GetVersion() string
	GetArch() string
	GetServiceType() string // device, cluster or both
	GetRequiredServices() []exchangecommon.ServiceDependency
	GetUserInputs() []exchangecommon.UserInput
	NeedsUserInput() bool
	GetDeployment() interface{}
	GetClusterDeployment() interface{}
}

// ServiceFile An implementation of AbstractServiceFile
// 
// It is used when reading json file the user gives us as input to create the service
// swagger:model
type ServiceFile struct {
	Org                        string                             `json:"org"` // optional
	Label                      string                             `json:"label"`
	Description                string                             `json:"description"`
	Public                     bool                               `json:"public"`
	Documentation              string                             `json:"documentation"`
	URL                        string                             `json:"url"`
	Version                    string                             `json:"version"`
	Arch                       string                             `json:"arch"`
	Sharable                   string                             `json:"sharable"`
	MatchHardware              map[string]interface{}             `json:"matchHardware,omitempty"`
	RequiredServices           []exchangecommon.ServiceDependency `json:"requiredServices"`
	UserInputs                 []exchangecommon.UserInput         `json:"userInput"`
	Deployment                 interface{}                        `json:"deployment,omitempty"` // interface{} because pre-signed services can be stringified json
	DeploymentSignature        string                             `json:"deploymentSignature,omitempty"`
	ClusterDeployment          interface{}                        `json:"clusterDeployment,omitempty"`
	ClusterDeploymentSignature string                             `json:"clusterDeploymentSignature,omitempty"`
}

func (sf *ServiceFile) GetOrg() string {
	return sf.Org
}

func (sf *ServiceFile) GetURL() string {
	return sf.URL
}

func (sf *ServiceFile) GetVersion() string {
	return sf.Version
}

func (sf *ServiceFile) GetArch() string {
	return sf.Arch
}

func (sf *ServiceFile) GetRequiredServices() []exchangecommon.ServiceDependency {
	return sf.RequiredServices
}

func (sf *ServiceFile) GetUserInputs() []exchangecommon.UserInput {
	return sf.UserInputs
}

func (s *ServiceFile) NeedsUserInput() bool {
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

func (sf *ServiceFile) GetDeployment() interface{} {
	return sf.Deployment
}

func (sf *ServiceFile) GetClusterDeployment() interface{} {
	return sf.ClusterDeployment
}

// Get the service type
// Check for nil, "" and {} for deployment and cluster deployment.
func (s *ServiceFile) GetServiceType() string {
	sType := exchangecommon.SERVICE_TYPE_DEVICE
	if s.ClusterDeployment != nil && s.ClusterDeployment != "" {
		if s.Deployment == nil || s.Deployment == "" {
			sType = exchangecommon.SERVICE_TYPE_CLUSTER
		} else {
			sType = exchangecommon.SERVICE_TYPE_BOTH
		}
	}
	return sType
}

// Returns true if the service definition userinputs define the variable.
func (sf *ServiceFile) DefinesVariable(name string) string {
	for _, ui := range sf.UserInputs {
		if ui.Name == name && ui.Type != "" {
			return ui.Type
		}
	}
	return ""
}

// Returns true if the service definition has required services.
func (sf *ServiceFile) HasDependencies() bool {
	if len(sf.RequiredServices) == 0 {
		return false
	}
	return true
}

// Return true if the service definition is a dependency in the input list of service references.
func (sf *ServiceFile) IsDependent(deps []exchangecommon.ServiceDependency) bool {
	for _, dep := range deps {
		if sf.URL == dep.URL && sf.Org == dep.Org {
			return true
		}
	}
	return false
}

// Convert the Deployment Configuration to a full Deployment Description.
func (sf *ServiceFile) ConvertToDeploymentDescription(agreementService bool,
	msgPrinter *message.Printer) (*DeploymentConfig, *containermessage.DeploymentDescription, error) {
	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	depConfig, err := ConvertToDeploymentConfig(sf.Deployment, msgPrinter)
	if err != nil {
		return nil, nil, err
	}
	infra := !agreementService
	return depConfig, &containermessage.DeploymentDescription{
		Services: depConfig.Services,
		ServicePattern: containermessage.Pattern{
			Shared: map[string][]string{},
		},
		Infrastructure: infra,
		Overrides:      map[string]*containermessage.Service{},
	}, nil
}

// Verify that non default user inputs are set in the input map.
func (sf *ServiceFile) RequiredVariablesAreSet(setVarNames []string) error {
	for _, ui := range sf.UserInputs {
		if ui.DefaultValue == "" && ui.Name != "" {
			found := false
			for _, v := range setVarNames {
				if v == ui.Name {
					found = true
				}
			}
			if !found {
				return errors.New(i18n.GetMessagePrinter().Sprintf("user input %v has no default value and is not set", ui.Name))
			}
		}
	}
	return nil
}

func (sf *ServiceFile) SupportVersionRange() {
	for ix, sdep := range sf.RequiredServices {
		if sdep.VersionRange == "" {
			sf.RequiredServices[ix].VersionRange = sf.RequiredServices[ix].Version
		}
	}
}

// Validate a service definition.
// Varifies the existance of the dependent services.
// Verifies consistence for the dependent service types
// Make sure userinput and requiredServices are not supported for cluster services.
func ValidateService(serviceDefResolverHandler exchange.ServiceDefResolverHandler, svcFile AbstractServiceFile, msgPrinter *message.Printer) error {
	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	// cluster type, userinput and requiredServices are not allowed
	topSvcType := svcFile.GetServiceType()
	requiredServices := svcFile.GetRequiredServices()
	if topSvcType == exchangecommon.SERVICE_TYPE_CLUSTER {
		if requiredServices != nil && len(requiredServices) != 0 {
			return fmt.Errorf(msgPrinter.Sprintf("'requiredServices' is not supported for cluster type service."))
		}
	} else {
		// if it the service type is 'device' or 'both', make sure all the dependent services are 'device' or 'both' types
		if requiredServices != nil {
			for _, reqSvc := range requiredServices {

				// get the service definition for the required service and all of it dependents
				ver := reqSvc.GetVersionRange()
				vExp, err := semanticversion.Version_Expression_Factory(ver)
				if err != nil {
					return fmt.Errorf(msgPrinter.Sprintf("Failed to convert version %v for service %v to version range expression.", ver, reqSvc))
				}
				svc_map, sDef, sId, err := serviceDefResolverHandler(reqSvc.URL, reqSvc.Org, vExp.Get_expression(), reqSvc.Arch)
				if err != nil {
					return fmt.Errorf(msgPrinter.Sprintf("Error retrieving service from the Exchange for %v. %v", reqSvc, err))
				}

				// check the node type for the required service
				sType := sDef.GetServiceType()
				if sType == exchangecommon.SERVICE_TYPE_CLUSTER {
					return fmt.Errorf(msgPrinter.Sprintf("The required service %v has the wrong service type: %v.", sId, sType))
				}

				// check the node type of the dependent services of the required service
				for id, s := range svc_map {
					sType1 := s.GetServiceType()
					if sType == exchangecommon.SERVICE_TYPE_CLUSTER {
						return fmt.Errorf(msgPrinter.Sprintf("The dependent service %v for the required service %v has the wrong service type: %v.", id, sId, sType1))
					}
				}
			}
		}
	}

	return nil
}

// check if the deployment is empty. The following cases are considered empty in JSON:
// "deployment": {}
// "deployment": null
// "deployment": ""
func DeploymentIsEmpty(deployment interface{}) bool {
	switch deployment.(type) {
	case nil:
		return true
	case map[string]interface{}:
		if len(deployment.(map[string]interface{})) == 0 {
			return true
		}
	case string:
		if deployment.(string) == "" {
			return true
		}
	}

	return false
}
