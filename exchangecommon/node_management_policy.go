package exchangecommon

import (
	"fmt"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/i18n"
	"strings"
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
	AgentAutoUpgradePolicy *ExchangeAgentUpgradePolicy         `json:"agentUpgradePolicy,omitempty"`
	LastUpdated            string                              `json:"lastUpdated,omitempty"`
	Created                string                              `json:"created,omitempty"`
}

func (e *ExchangeNodeManagementPolicy) Validate() error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// make sure required fields are not empty
	if e.Label == "" {
		return fmt.Errorf(msgPrinter.Sprintf("Label, or Enabled is empty."))
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

	// even if the constraints array has non-zero length, the items in it could be emptry strings
	for _, c := range e.Constraints {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}

	return true
}

func (e ExchangeNodeManagementPolicy) String() string {
	return fmt.Sprintf("Owner: %v, Label: %v, Description: %v, Properties: %v, Constraints: %v, Patterns: %v, Enabled: %v, AgentAutoUpgradePolicy: %v, LastUpdated: %v, Created: %v",
		e.Owner, e.Label, e.Description,
		e.Properties, e.Constraints, e.Patterns,
		e.Enabled, e.AgentAutoUpgradePolicy, e.LastUpdated, e.Created)
}

// The agent upgrade policy as stored in the exchange
type ExchangeAgentUpgradePolicy struct {
	MinVersionString      string `json:"atLeastVersion"`
	PolicyUpgradeTime     string `json:"start"`
	UpgradeWindowDuration int    `json:"duration"`
}

func (e ExchangeAgentUpgradePolicy) String() string {
	return fmt.Sprintf("MinimumVersion: %v, PolicyUpgradeTime: %v, UpgradeWindowDuration: %v", e.MinVersionString,
		e.PolicyUpgradeTime, e.UpgradeWindowDuration)
}
