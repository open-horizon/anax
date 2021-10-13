package externalpolicy

import (
	"errors"
	"fmt"
	"github.com/open-horizon/anax/i18n"
)

const (
	// for node policy update state
	EP_COMPARE_NOCHANGE           = 0x0000 // no change
	EP_COMPARE_PROPERTY_CHANGED   = 0x0001 // properties changed
	EP_COMPARE_CONSTRAINT_CHANGED = 0x0002 // constraints changed
	EP_COMPARE_DELETED            = 0x0004 // deleted
	EP_ALLOWPRIVILEGED_CHANGED    = 0x0008 // built-in node property openhorizon.allowPrivileged changed
)

// BusinessPolicy the external policy
// swagger:model
type ExternalPolicy struct {
	// The properties this node wishes to expose about itself. These properties can be referred to by constraint expressions in other policies,
	// (e.g. service policy, model policy, business policy).
	Properties PropertyList `json:"properties"`

	// A textual expression indicating requirements on the other party in order to make an agreement.
	Constraints ConstraintExpression `json:"constraints"`
}

func (e ExternalPolicy) String() string {
	return fmt.Sprintf("Properties: %v, Constraints: %v", e.Properties, e.Constraints)
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
func (e ExternalPolicy) DeepCopy() *ExternalPolicy {

	copyE := ExternalPolicy{Properties: CopyProperties(e.Properties), Constraints: CopyConstraints(e.Constraints)}

	return &copyE
}

// compare two external policies. The result is the OR of the EP_COMPARE_*
// constants defined in this file. newPol should not be nil.
func (ep ExternalPolicy) CompareWith(newPol *ExternalPolicy) int {
	// just in case
	if newPol == nil {
		return EP_COMPARE_PROPERTY_CHANGED & EP_COMPARE_CONSTRAINT_CHANGED
	}

	rc := EP_COMPARE_NOCHANGE

	// check properties
	if len(ep.Properties) != len(newPol.Properties) {
		rc = rc | EP_COMPARE_PROPERTY_CHANGED
	} else if len(ep.Properties) != 0 && !ep.Properties.IsSame(newPol.Properties) {
		rc = rc | EP_COMPARE_PROPERTY_CHANGED
	}

	// check built-in property openhorizon.allowPrivileged
	if rc&EP_COMPARE_PROPERTY_CHANGED != 0 {

		// default value for PROP_NODE_PRIVILEGED is false
		privileged1 := false
		privileged2 := false

		if ep.Properties != nil {
			if prop, err := ep.Properties.GetProperty(PROP_NODE_PRIVILEGED); err == nil {
				if priv, ok := prop.Value.(bool); ok {
					privileged1 = priv
				}
			}
		}
		if newPol.Properties != nil {
			if prop, err := newPol.Properties.GetProperty(PROP_NODE_PRIVILEGED); err == nil {
				if priv, ok := prop.Value.(bool); ok {
					privileged2 = priv
				}
			}
		}

		if privileged1 != privileged2 {
			rc = rc | EP_ALLOWPRIVILEGED_CHANGED
		}
	}

	// check constraints
	if len(ep.Constraints) != len(newPol.Constraints) {
		rc = rc | EP_COMPARE_CONSTRAINT_CHANGED
	} else if len(ep.Constraints) != 0 && !ep.Constraints.IsSame(newPol.Constraints) {
		rc = rc | EP_COMPARE_CONSTRAINT_CHANGED
	}

	return rc
}

// returns a copy of the given PropertyList
func CopyProperties(prop PropertyList) PropertyList {
	if prop == nil {
		return nil
	}
	copyProp := make(PropertyList, len(prop))
	copy(copyProp, prop)
	return copyProp
}

// returns a copy of the given ConstraintExpression
func CopyConstraints(constraints ConstraintExpression) ConstraintExpression {
	if constraints == nil {
		return nil
	}
	copyConstraints := make([]string, len(constraints))
	copy(copyConstraints, constraints)
	return copyConstraints
}
