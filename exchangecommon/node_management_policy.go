package exchangecommon

import (
	"fmt"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/i18n"
	"strings"
	"time"
)

// Node management policy as represented in the exchange
type ExchangeNodeManagementPolicy struct {
	Owner                  string                              `json:"owner,omitempty"`
	Label                  string                              `json:"label"`
	Description            string                              `json:"description"`
	Constraints            externalpolicy.ConstraintExpression `json:"constraints"`
	Properties             externalpolicy.PropertyList         `json:"properties"`
	Patterns               []string                            `json:"patterns"`
	Enabled                bool                                `json:"enabled"`
	PolicyUpgradeTime      string                              `json:"start"`
	UpgradeWindowDuration  int                                 `json:"startWindow"`
	AgentAutoUpgradePolicy *ExchangeAgentUpgradePolicy         `json:"agentUpgradePolicy,omitempty"`
	LastUpdated            string                              `json:"lastUpdated,omitempty"`
	Created                string                              `json:"created,omitempty"`
}

func (e ExchangeNodeManagementPolicy) String() string {
	return fmt.Sprintf("Owner: %v, Label: %v, Description: %v, Properties: %v, Constraints: %v, Patterns: %v, Enabled: %v, PolicyUpgradeTime: %v, UpgradeWindowDuration: %v AgentAutoUpgradePolicy: %v, LastUpdated: %v, Created: %v",
		e.Owner, e.Label, e.Description,
		e.Properties, e.Constraints, e.Patterns,
		e.Enabled, e.PolicyUpgradeTime, e.UpgradeWindowDuration, e.AgentAutoUpgradePolicy, e.LastUpdated, e.Created)
}

func (e *ExchangeNodeManagementPolicy) Validate() error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Validate the timestamp
	if e.PolicyUpgradeTime != "now" && e.PolicyUpgradeTime != "" {
		if _, err := time.Parse(time.RFC3339, e.PolicyUpgradeTime); err != nil {
			return fmt.Errorf(msgPrinter.Sprintf("The start time must be in RFC3339 format or set to \"now\"."))
		}
	}

	// Validate the PropertyList.
	if e != nil && len(e.Properties) != 0 {
		if err := e.Properties.Validate(); err != nil {
			return fmt.Errorf(msgPrinter.Sprintf("properties contains an invalid property: %v", err))
		}
	}

	if e.Properties.HasProperty(externalpolicy.PROP_SVC_PRIVILEGED) {
		privProp, _ := e.Properties.GetProperty(externalpolicy.PROP_SVC_PRIVILEGED)
		if _, ok := privProp.Value.(bool); !ok {
			if privProp.Value == "true" {
				privProp.Value = true
				e.Properties.Add_Property(&privProp, true)
			} else if privProp.Value == "false" {
				privProp.Value = false
				e.Properties.Add_Property(&privProp, true)
			} else {
				return fmt.Errorf(msgPrinter.Sprintf("The property %s must have a boolean value (true or false).", externalpolicy.PROP_SVC_PRIVILEGED))
			}
		}
	}

	// Validate the Constraints expression by invoking the plugins.
	if e != nil && len(e.Constraints) != 0 {
		_, err := e.Constraints.Validate()
		return err
	}

	// We only get here if the input object is nil OR all of the top level fields are empty.
	return nil
}

func (e *ExchangeNodeManagementPolicy) HasNoConstraints() bool {
	if e.Constraints == nil || len(e.Constraints) == 0 {
		return true
	}

	// even if the constraints array has non-zero length, the items in it could be empty strings
	for _, c := range e.Constraints {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}

	return true
}

func (e *ExchangeNodeManagementPolicy) HasNoPatterns() bool {
	if e.Patterns == nil || len(e.Patterns) == 0 {
		return true
	}

	// even if the pattern array has non-zero length, the items in it could be empty strings
	for _, c := range e.Patterns {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}

	return true
}

// The agent upgrade policy as stored in the exchange
type ExchangeAgentUpgradePolicy struct {
	Manifest       string `json:"manifest"`
	AllowDowngrade bool   `json:"allowDowngrade"`
}

func (e ExchangeAgentUpgradePolicy) String() string {
	return fmt.Sprintf("Manifest: %v, AllowDowngrade: %v", e.Manifest, e.AllowDowngrade)
}

type UpgradeManifest struct {
	Software      UpgradeDescription `json:"softwareUpgrade"`
	Certificate   UpgradeDescription `json:"certificateUpgrade"`
	Configuration UpgradeDescription `json:"configurationUpgrade"`
}

type UpgradeDescription struct {
	Version  string   `json:"version"`
	FileList []string `json:"files"`
}

type AgentFileVersions struct {
	SoftwareVersions []string `json:"agentSoftwareVersions"`
	ConfigVersions   []string `json:"agentConfigVersions"`
	CertVersions     []string `json:"agentCertVersions"`
	LastUpdated      string   `json:"lastUpdated,omitempty"`
}

func (a AgentFileVersions) String() string {
	return fmt.Sprintf("SoftwareVersions: %v, ConfigVersions: %v, CertVersions: %v", a.SoftwareVersions, a.ConfigVersions, a.CertVersions)
}
