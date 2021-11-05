package exchangecommon

import (
	"fmt"
	"github.com/open-horizon/anax/externalpolicy"
)

// ServicePolicy the service policy
type ServicePolicy struct {
	Label                         string `json:"label,omitempty"`
	Description                   string `json:"description,omitempty"`
	externalpolicy.ExternalPolicy        // properties and constraints,
}

func (s ServicePolicy) String() string {
	return fmt.Sprintf("ServicePolicy: Label: %v, Description: %v, Properties: %v, Constraints: %v", s.Label, s.Description, s.Properties, s.Constraints)
}

// This function validates the properties and constrains. It also updates the node's
// writable built-in properties inside this policy to make sure they have correct data types.
// The validation returns errors if the policy does not validate. It uses the constraint language
// plugins to handle the constraints field.
func (s *ServicePolicy) ValidateAndNormalize() error {
	if err := (&s.ExternalPolicy).ValidateAndNormalize(); err != nil {
		return err
	}
	return nil
}

// return a pointer to a copy of ServicePolicy
func (s ServicePolicy) GetExternalPolicy() *externalpolicy.ExternalPolicy {
	return &s.ExternalPolicy
}

// return a pointer to a copy of ServicePolicy
func (n ServicePolicy) DeepCopy() *ServicePolicy {
	copyN := ServicePolicy{}

	copyN.Label = n.Label
	copyN.Description = n.Description

	copyN.Properties = externalpolicy.CopyProperties(n.Properties)
	copyN.Constraints = externalpolicy.CopyConstraints(n.Constraints)

	return &copyN
}
