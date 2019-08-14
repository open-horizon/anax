package externalpolicy

import (
	"errors"
	"fmt"
	"github.com/open-horizon/anax/i18n"
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

	// get message printer because this function is called by CLI
	msgPrinter := i18n.GetMessagePrinter()

	// Validate the PropertyList.
	if e != nil && len(e.Properties) != 0 {
		if err := e.Properties.Validate(); err != nil {
			return errors.New(msgPrinter.Sprintf("properties contains an invalid property: %v", err))
		}
	}

	// Validate the Constraints expression by invoking the plugins.
	if e != nil && len(e.Constraints) != 0 {
		return e.Constraints.Validate()
	}

	// We only get here if the input object is nil OR all of the top level fields are empty.
	return nil
}

// merge the two policies. If the newPol contains the same properties, ignore them unless replaceExsiting is true.
func (e *ExternalPolicy) MergeWith(newPol *ExternalPolicy, replaceExsiting bool) {
	if newPol == nil {
		return
	}

	if len(newPol.Properties) != 0 {
		(&e.Properties).MergeWith(&newPol.Properties, replaceExsiting)
	}

	if len(newPol.Constraints) != 0 {
		(&e.Constraints).MergeWith(&newPol.Constraints)
	}
}
