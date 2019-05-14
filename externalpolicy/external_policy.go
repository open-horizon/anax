package externalpolicy

import (
	"errors"
	"fmt"
)

type ExternalPolicy struct {
	// The properties this node wishes to expose about itself. These properties can be referred to by constraint expressions in other policies,
	// (e.g. service policy, model policy, business policy).
	Properties PropertyList `json:"properties,omitempty"`

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
