package businesspolicy

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/policy"
	"strings"
)

const DEFAULT_MAX_AGREEMENT = 0

// BusinessPolicy the business policy
// swagger:model
type BusinessPolicy struct {
	Owner         string                              `json:"owner,omitempty"`
	Label         string                              `json:"label"`
	Description   string                              `json:"description"`
	Service       ServiceRef                          `json:"service"`
	Properties    externalpolicy.PropertyList         `json:"properties,omitempty"`
	Constraints   externalpolicy.ConstraintExpression `json:"constraints,omitempty"`
	UserInput     []policy.UserInput                  `json:"userInput,omitempty"`
	SecretBinding []exchangecommon.SecretBinding      `json:"secretBinding,omitempty"` // The secret binding from service secret names to secret manager secret names.
}

func (w BusinessPolicy) String() string {
	return fmt.Sprintf("Owner: %v, Label: %v, Description: %v, Service: %v, Properties: %v, Constraints: %v, UserInput: %v, SecretBinding: %v",
		w.Owner,
		w.Label,
		w.Description,
		w.Service,
		w.Properties,
		w.Constraints,
		w.UserInput,
		w.SecretBinding)
}

type ServiceRef struct {
	Name            string           `json:"name"`                      // refers to a service definition in the exchange
	Org             string           `json:"org,omitempty"`             // the org holding the service definition
	Arch            string           `json:"arch,omitempty"`            // the hardware architecture of the service definition
	ServiceVersions []WorkloadChoice `json:"serviceVersions,omitempty"` // a list of service version for rollback
	NodeH           NodeHealth       `json:"nodeHealth"`                // policy for determining when a node's health is violating its agreements
}

func (w ServiceRef) String() string {
	return fmt.Sprintf("Name: %v, Org: %v, Arch: %v, ServiceVersions: %v, NodeH: %v",
		w.Name,
		w.Org,
		w.Arch,
		w.ServiceVersions,
		w.NodeH)
}

type WorkloadPriority struct {
	PriorityValue     int `json:"priority_value,omitempty"`     // The priority of the workload
	Retries           int `json:"retries,omitempty"`            // The number of retries before giving up and moving to the next priority
	RetryDurationS    int `json:"retry_durations,omitempty"`    // The number of seconds in which the specified number of retries must occur in order for the next priority workload to be attempted.
	VerifiedDurationS int `json:"verified_durations,omitempty"` // The number of second in which verified data must exist before the rollback retry feature is turned off
}

func (w WorkloadPriority) String() string {
	return fmt.Sprintf("PriorityValue: %v, Retries: %v, RetryDurationS: %v, VerifiedDurationS: %v",
		w.PriorityValue,
		w.Retries,
		w.RetryDurationS,
		w.VerifiedDurationS)
}

type UpgradePolicy struct {
	Lifecycle string `json:"lifecycle,omitempty"` // immediate, never, agreement
	Time      string `json:"time,omitempty"`      // the time of the upgrade
}

func (w UpgradePolicy) String() string {
	return fmt.Sprintf("Lifecycle: %v, Time: %v",
		w.Lifecycle,
		w.Time)
}

type WorkloadChoice struct {
	Version  string           `json:"version,omitempty"`  // the version of the workload
	Priority WorkloadPriority `json:"priority,omitempty"` // the highest priority workload is tried first for an agreement, if it fails, the next priority is tried. Priority 1 is the highest, priority 2 is next, etc.
	Upgrade  UpgradePolicy    `json:"upgradePolicy,omitempty"`
}

func (w WorkloadChoice) String() string {
	return fmt.Sprintf("Version: %v, Priority: %v, Upgrade: %v",
		w.Version,
		w.Priority,
		w.Upgrade)
}

type NodeHealth struct {
	MissingHBInterval    int `json:"missing_heartbeat_interval,omitempty"` // How long a heartbeat can be missing until it is considered missing (in seconds)
	CheckAgreementStatus int `json:"check_agreement_status,omitempty"`     // How often to check that the node agreement entry still exists in the exchange (in seconds)
}

func (w NodeHealth) String() string {
	return fmt.Sprintf("MissingHBInterval: %v, CheckAgreementStatus: %v",
		w.MissingHBInterval,
		w.CheckAgreementStatus)
}

// The validate function returns errors if the policy does not validate. It uses the constraint language
// plugins to handle the constraints field.
func (b *BusinessPolicy) Validate() error {

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// make sure required fields are not empty
	if b.Service.Name == "" || b.Service.Org == "" {
		return fmt.Errorf(msgPrinter.Sprintf("Name, or Org is empty string."))
	} else if b.Service.ServiceVersions == nil || len(b.Service.ServiceVersions) == 0 {
		return fmt.Errorf(msgPrinter.Sprintf("The serviceVersions array is empty."))
	}

	// Validate the PropertyList.
	if b != nil && len(b.Properties) != 0 {
		if err := b.Properties.Validate(); err != nil {
			return fmt.Errorf(msgPrinter.Sprintf("properties contains an invalid property: %v", err))
		}
	}

	if b.Properties.HasProperty(externalpolicy.PROP_SVC_PRIVILEGED) {
		privProp, _ := b.Properties.GetProperty(externalpolicy.PROP_SVC_PRIVILEGED)
		if _, ok := privProp.Value.(bool); !ok {
			if privProp.Value == "true" {
				privProp.Value = true
				b.Properties.Add_Property(&privProp, true)
			} else if privProp.Value == "false" {
				privProp.Value = false
				b.Properties.Add_Property(&privProp, true)
			} else {
				return fmt.Errorf(msgPrinter.Sprintf("The property %s must have a boolean value (true or false).", externalpolicy.PROP_SVC_PRIVILEGED))
			}
		}
	}

	// Validate the Constraints expression by invoking the plugins.
	if b != nil && len(b.Constraints) != 0 {
		_, err := b.Constraints.Validate()
		return err
	}

	// We only get here if the input object is nil OR all of the top level fields are empty.
	return nil
}

// Check if there is no contraints or not
func (b *BusinessPolicy) HasNoConstraints() bool {
	if b.Constraints == nil || len(b.Constraints) == 0 {
		return true
	}

	// even if the constraints array has non-zero length, the items in it could be emptry strings
	for _, c := range b.Constraints {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}

	return true
}

// Convert business policy to a policy object.
func (b *BusinessPolicy) GenPolicyFromBusinessPolicy(policyName string) (*policy.Policy, error) {

	// validate first
	if err := b.Validate(); err != nil {
		return nil, fmt.Errorf("Failed to validate the business policy: %v", err)
	}

	service := b.Service
	pol := policy.Policy_Factory(fmt.Sprintf("%v", policyName))

	// Copy service metadata into the policy
	for _, wl := range service.ServiceVersions {
		if wl.Version == "" {
			return nil, fmt.Errorf("The version for service %v arch %v is empty in the business policy for %v", service.Name, service.Arch, policyName)
		}
		ConvertChoice(wl, service.Name, service.Org, service.Arch, pol)
	}

	// properties and constrains
	if err := ConvertProperties(b.Properties, pol); err != nil {
		return nil, err
	}
	if err := ConvertConstraints(b.Constraints, pol); err != nil {
		return nil, err
	}

	// node health
	ConvertNodeHealth(service.NodeH, pol)

	pol.MaxAgreements = DEFAULT_MAX_AGREEMENT

	// add default agreement protocol
	newAGP := policy.AgreementProtocol_Factory(policy.BasicProtocol)
	newAGP.Initialize()
	pol.Add_Agreement_Protocol(newAGP)

	// make a copy of the user input
	pol.UserInput = make([]policy.UserInput, len(b.UserInput))
	copy(pol.UserInput, b.UserInput)

	// make a copy of the secretBindings
	pol.SecretBinding = make([]exchangecommon.SecretBinding, 0)
	for _, sb := range b.SecretBinding {
		newSB := sb.MakeCopy()
		pol.SecretBinding = append(pol.SecretBinding, newSB)
	}

	glog.V(3).Infof("converted %v into policy %v.", service, policyName)

	return pol, nil
}

func ConvertChoice(wl WorkloadChoice, url string, org string, arch string, pol *policy.Policy) {
	newWL := policy.Workload_Factory(url, org, wl.Version, arch)
	newWL.Priority = (*policy.Workload_Priority_Factory(wl.Priority.PriorityValue, wl.Priority.Retries, wl.Priority.RetryDurationS, wl.Priority.VerifiedDurationS))
	pol.Add_Workload(newWL)
}

func ConvertNodeHealth(nodeh NodeHealth, pol *policy.Policy) {
	// Copy over the node health policy
	nh := policy.NodeHealth_Factory(nodeh.MissingHBInterval, nodeh.CheckAgreementStatus)
	pol.Add_NodeHealth(nh)
}

func ConvertProperties(properties externalpolicy.PropertyList, pol *policy.Policy) error {
	for _, p := range properties {
		if err := pol.Add_Property(&p, false); err != nil {
			return fmt.Errorf("error trying add external policy property %v to policy. %v", p, err)
		}
	}
	return nil
}

func ConvertConstraints(constraints externalpolicy.ConstraintExpression, pol *policy.Policy) error {
	newconstr := externalpolicy.Constraint_Factory()
	for _, c := range constraints {
		newconstr.Add_Constraint(c)
	}
	pol.Constraints = *newconstr
	return nil
}
