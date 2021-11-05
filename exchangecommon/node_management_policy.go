package exchangecommon

import (
	"fmt"
	"github.com/open-horizon/anax/externalpolicy"
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
	AgentAutoUpgradePolicy ExchangeAgentUpgradePolicy          `json:"agentUpgradePolicy,omitempty"`
	LastUpdated            string                              `json:"lastUpdated,omitempty"`
	Created                string                              `json:"created,omitempty"`
}

func (e ExchangeNodeManagementPolicy) String() string {
	return fmt.Sprintf("Owner: %v, Label: %v, Description: %v, Properties: %v, Constraints: %v, Patterns: %v, Enabled: %v, AgentAutoUpgradePolicy: %v, LastUpdated: %v",
		e.Owner, e.Label, e.Description,
		e.Properties, e.Constraints, e.Patterns,
		e.Enabled, e.AgentAutoUpgradePolicy, e.LastUpdated)
}

// The agent upgrade policy as stored in the exchange
type ExchangeAgentUpgradePolicy struct {
	MinVersionString      string `json:"atLeastVersion"`
	PolicyUpgradeTime     string `json:"start"`
	UpgradeWindowDuration string `json:"duration"`
}

func (e ExchangeAgentUpgradePolicy) String() string {
	return fmt.Sprintf("MinimumVersion: %v, PolicyUpgradeTime: %v, UpgradeWindowDuration: %v", e.MinVersionString,
		e.PolicyUpgradeTime, e.UpgradeWindowDuration)
}
