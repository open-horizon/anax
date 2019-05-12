package externalpolicy

import (
	"errors"
	"fmt"
	"github.com/open-horizon/anax/policy"
)

type ExternalPolicy struct {
	// The properties this node wishes to expose about itself. These properties can be referred to by constraint expressions in other policies,
	// (e.g. service policy, model policy, business policy).
	Properties policy.PropertyList `json:"properties,omitempty"`

	// A textual expression indicating requirements on the other party in order to make an agreement.
	Constraints ConstraintExpression `json:"constraints,omitempty"`
}

func (e ExternalPolicy) String() string {
	return fmt.Sprintf("ExternalPolicy: Properties: %v, Constraints: %v", e.Properties, e.Constraints)
}

// The validate function returns errors if the policy does not validate. It uses the constraint language
// plugins to handle the constraints field.
func (e *ExternalPolicy) Validate() error {

	// Validate the PropertyList.
	if e != nil && len(e.Properties) != 0 {
		if err := e.Properties.Validate(); err != nil {
			return errors.New(fmt.Sprintf("properties contains an invalid property: %v", err))
		}
	}

	// Validate the Constraints expression by invoking the plugins.
	if e != nil && len(e.Constraints) != 0 {
		return e.Constraints.Validate()
	}

	// We only get here if the input object is nil OR all of the top level fields are empty.
	return nil
}

// Create a header name for the generated policy that should be unique within the org.
// The input can be a device id or a servcie id.
func MakeExternalPolicyHeaderName(id string) string {
	return fmt.Sprintf("Policy for %v", id)
}

// Generate a policy from the external policy.
// The input is the device id or service id.
func (e *ExternalPolicy) GenPolicyFromExternalPolicy(id string) (*policy.Policy, error) {
	// validate first
	if err := e.Validate(); err != nil {
		return nil, fmt.Errorf("Failed to validate the external policy: %v", e)
	}

	polName := MakeExternalPolicyHeaderName(id)
	pPolicy := policy.Policy_Factory(polName)

	for _, p := range e.Properties {
		if err := pPolicy.Add_Property(&p); err != nil {
			return nil, fmt.Errorf("Failed to add property %v to policy. %v", p, err)
		}
	}

	rp, err := RequiredPropertyFromConstraint(&(e.Constraints))
	if err != nil {
		return nil, fmt.Errorf("error trying to convert external policy constraints to JSON: %v", err)
	}
	if rp != nil {
		pPolicy.CounterPartyProperties = (*rp)
	}
	return pPolicy, nil
}
