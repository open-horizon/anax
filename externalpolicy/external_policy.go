package externalpolicy

import (
	"errors"
	"fmt"
	"github.com/open-horizon/anax/i18n"
)

// BusinessPolicy the external policy
// swagger:model
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

// This function validates the properties and constrains. It also updates the node's and service's
// writable built-in properties inside this policy to make sure they have correct data types.
// The validation returns errors if the policy does not validate. It uses the constraint language
// plugins to handle the constraints field.
func (e *ExternalPolicy) ValidateAndNormalize() error {

	// get message printer because this function is called by CLI
	msgPrinter := i18n.GetMessagePrinter()

	// Validate the PropertyList.
	if e != nil && len(e.Properties) != 0 {
		if err := e.Properties.Validate(); err != nil {
			return errors.New(msgPrinter.Sprintf("properties contains an invalid property: %v", err))
		}
	}

	// accepts string "true" or "false" for PROP_NODE_PRIVILEGED, but change them to boolean
	if e.Properties.HasProperty(PROP_NODE_PRIVILEGED) {
		privProp, err := e.Properties.GetProperty(PROP_NODE_PRIVILEGED)
		if err != nil {
			return err
		}
		if _, ok := privProp.Value.(bool); !ok {
			if privStr, ok := privProp.Value.(string); ok && (privStr == "true" || privStr == "false") {
				if privStr == "true" {
					e.Properties.Add_Property(Property_Factory(PROP_NODE_PRIVILEGED, true), true)
				} else {
					e.Properties.Add_Property(Property_Factory(PROP_NODE_PRIVILEGED, false), true)
				}
			} else {
				return errors.New(msgPrinter.Sprintf("Property %s must have a boolean value (true or false).", PROP_NODE_PRIVILEGED))
			}
		}
	}

	// accepts string "true" or "false" for PROP_SVC_PRIVILEGED, but change them to boolean
	if e.Properties.HasProperty(PROP_SVC_PRIVILEGED) {
		privProp, err := e.Properties.GetProperty(PROP_SVC_PRIVILEGED)
		if err != nil {
			return err
		}
		if _, ok := privProp.Value.(bool); !ok {
			if privStr, ok := privProp.Value.(string); ok && (privStr == "true" || privStr == "false") {
				if privStr == "true" {
					e.Properties.Add_Property(Property_Factory(PROP_SVC_PRIVILEGED, true), true)
				} else {
					e.Properties.Add_Property(Property_Factory(PROP_SVC_PRIVILEGED, false), true)
				}
			} else {
				return errors.New(msgPrinter.Sprintf("Property %s must have a boolean value (true or false).", PROP_SVC_PRIVILEGED))
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

// return a pointer to a copy of ExternalPolicy
func (e *ExternalPolicy) DeepCopy() *ExternalPolicy {
	var copyProp PropertyList
	if e.Properties == nil {
		copyProp = nil
	} else {
		copyProp = make(PropertyList, len(e.Properties))
		copy(copyProp, e.Properties)
	}

	var copyCons ConstraintExpression
	if e.Constraints == nil {
		copyCons = nil
	} else {
		copyCons = make(ConstraintExpression, len(e.Constraints))
		copy(copyCons, e.Constraints)
	}

	copyE := ExternalPolicy{Properties: copyProp, Constraints: copyCons}

	return &copyE

}
