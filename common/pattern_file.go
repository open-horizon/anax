package common

import (
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/policy"
	"golang.org/x/text/message"
)

type AbstractPatternFile interface {
	GetOrg() string
	GetServices() []exchange.ServiceReference
	GetUserInputs() []policy.UserInput
	GetSecretBinding() []exchangecommon.SecretBinding
	IsPublic() bool
}

// An implementation of AbstractPatternFile.
// It is used when reading json file the user gives us as an input to create the pattern
type PatternFile struct {
	Name               string                         `json:"name,omitempty"`
	Org                string                         `json:"org,omitempty"` // optional
	Label              string                         `json:"label"`
	Description        string                         `json:"description,omitempty"`
	Public             bool                           `json:"public"`
	Services           []ServiceReferenceFile         `json:"services"`
	AgreementProtocols []exchange.AgreementProtocol   `json:"agreementProtocols,omitempty"`
	UserInput          []policy.UserInput             `json:"userInput,omitempty"`
	SecretBinding      []exchangecommon.SecretBinding `json:"secretBinding,omitempty"`
}

func (p *PatternFile) GetOrg() string {
	return p.Org
}

func (p *PatternFile) IsPublic() bool {
	return p.Public
}

// convert the []ServiceReferenceFile to []exchange.ServiceReference
// Not converting te depployment strings for now.
func (p *PatternFile) GetServices() []exchange.ServiceReference {
	service_refs := []exchange.ServiceReference{}
	if p.Services != nil {
		for _, svc := range p.Services {
			sref := exchange.ServiceReference{}
			sref.ServiceURL = svc.ServiceURL
			sref.ServiceOrg = svc.ServiceOrg
			sref.ServiceArch = svc.ServiceArch
			sref.AgreementLess = svc.AgreementLess
			if svc.DataVerify != nil {
				sref.DataVerify = *svc.DataVerify
			}
			if svc.NodeH != nil {
				sref.NodeH = *svc.NodeH
			}

			versions := []exchange.WorkloadChoice{}
			if svc.ServiceVersions != nil {
				for _, v := range svc.ServiceVersions {
					c := exchange.WorkloadChoice{Version: v.Version}
					if v.Priority != nil {
						c.Priority = *v.Priority
					}
					if v.Upgrade != nil {
						c.Upgrade = *v.Upgrade
					}
					versions = append(versions, c)
				}
			}
			sref.ServiceVersions = versions

			service_refs = append(service_refs, sref)
		}
	}

	return service_refs
}

func (p *PatternFile) GetUserInputs() []policy.UserInput {
	return p.UserInput
}

func (p *PatternFile) GetSecretBinding() []exchangecommon.SecretBinding {
	return p.SecretBinding
}

// make sure that all service secrets have vault bindings.
// it does not verify that the vault secrets exist.
func (p *PatternFile) ValidateSecretBinding(ec exchange.ExchangeContext,
	msgPrinter *message.Printer) ([]exchangecommon.SecretBinding, []exchangecommon.SecretBinding, error) {
	// no need to verify if both are empty
	if p.Services == nil || len(p.Services) == 0 {
		if p.SecretBinding == nil || len(p.SecretBinding) == 0 {
			return nil, nil, nil
		}
	}

	return ValidateSecretBinding(p.SecretBinding, p.GetServices(), ec, true, msgPrinter)
}

type ServiceReferenceFile struct {
	ServiceURL      string                     `json:"serviceUrl"`                 // refers to a service definition in the exchange
	ServiceOrg      string                     `json:"serviceOrgid"`               // the org holding the service definition
	ServiceArch     string                     `json:"serviceArch"`                // the hardware architecture of the service definition
	AgreementLess   bool                       `json:"agreementLess,omitempty"`    // a special case where this service will also be required by others
	ServiceVersions []ServiceChoiceFile        `json:"serviceVersions"`            // a list of service version for rollback
	DataVerify      *exchange.DataVerification `json:"dataVerification,omitempty"` // policy for verifying that the node is sending data
	NodeH           *exchange.NodeHealth       `json:"nodeHealth,omitempty"`       // this needs to be a ptr so it will be omitted if not specified, so exchange will default it
}

type ServiceChoiceFile struct {
	Version                      string                     `json:"version"`            // the version of the service
	Priority                     *exchange.WorkloadPriority `json:"priority,omitempty"` // the highest priority service is tried first for an agreement, if it fails, the next priority is tried. Priority 1 is the highest, priority 2 is next, etc.
	Upgrade                      *exchange.UpgradePolicy    `json:"upgradePolicy,omitempty"`
	DeploymentOverrides          interface{}                `json:"deployment_overrides,omitempty"`           // env var overrides for the service
	DeploymentOverridesSignature string                     `json:"deployment_overrides_signature,omitempty"` // signature of env var overrides
}
