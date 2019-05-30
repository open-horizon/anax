package externalpolicy

import (
	"errors"
	"fmt"
	"github.com/open-horizon/anax/externalpolicy/plugin_registry"
	"strings"
)

// This type implements all the ConstraintLanguage Plugin methods and delegates to plugin system.
type ConstraintExpression []string

func (c *ConstraintExpression) Validate() error {
	return plugin_registry.ConstraintLanguagePlugins.ValidatedByOne((*c).GetStrings())
}

func (c *ConstraintExpression) GetLanguageHandler() (plugin_registry.ConstraintLanguagePlugin, error) {
	return plugin_registry.ConstraintLanguagePlugins.GetLanguageHandlerByOne((*c).GetStrings())
}

// Create a simple, empty ConstraintExpression Object.
func Constraint_Factory() *ConstraintExpression {
	ce := new(ConstraintExpression)
	(*ce) = make([]string, 0)

	return ce
}

// Add a new constriant
func (c *ConstraintExpression) Add_Constraint(newconstr string) {
	(*c) = append(*c, newconstr)
}

// merge two constraint into one
func (c *ConstraintExpression) Merge(other *ConstraintExpression) *ConstraintExpression {
	// the function that checks if an element is in array
	s_contains := func(s_array []string, elem string) bool {
		for _, e := range s_array {
			if e == elem {
				return true
			}
		}
		return false
	}

	if other == nil || len(*other) == 0 {
		return c
	}

	if len(*c) == 0 {
		return other
	}

	// merge the two
	merged := Constraint_Factory()
	for _, elemA := range *c {
		merged.Add_Constraint(elemA)
	}

	for _, elemB := range *other {
		if !s_contains([]string(*c), elemB) {
			merged.Add_Constraint(elemB)
		}
	}

	return merged
}

// This function checks if the 2 constraints are the same. In order to be same, they must have the same constraints.
// The order can be different.
func (c ConstraintExpression) IsSame(other ConstraintExpression) bool {
	if len(other) == 0 && len(c) == 0 {
		return true
	}

	for _, const_this := range c {
		found := false
		for _, const_other := range other {
			if const_this == const_other {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	for _, const_other := range other {
		found := false
		for _, const_this := range c {
			if const_this == const_other {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// This function is used to determine if an input set of properties and values will satisfy
// the ConstraintExpression expression.
func (self *ConstraintExpression) IsSatisfiedBy(props []Property) error {

	// If there is no expression at all, then there is nothing to satisify
	if len(*self) == 0 {
		return nil
	}

	// convert it to RequiredProperty and then check
	if rp, err := RequiredPropertyFromConstraint(self); err != nil {
		return err
	} else if rp != nil {
		return rp.IsSatisfiedBy(props)
	} else {
		return nil
	}
}

func (self *ConstraintExpression) GetStrings() []string {
	return ([]string(*self))
}

// Create a RequiredProperty Object based on the constraint expression in an external policy. The constraint expression
// contains references to properties and provides a comparison operator and value on that property. These can be converted
// into our internal format.
func RequiredPropertyFromConstraint(extConstraint *ConstraintExpression) (*RequiredProperty, error) {

	const OP_AND = "and"
	const OP_OR = "or"
	const OP_NOT = "not"

	var err error
	var nextExpression, remainder, controlOp string
	var handler plugin_registry.ConstraintLanguagePlugin

	allPropArray := make([]interface{}, 0)
	allRP := RequiredProperty_Factory()

	if extConstraint == nil || len(*extConstraint) == 0 {
		return allRP, nil
	}

	for _, remainder = range []string(*extConstraint) {
		extConstraint.Validate()

		// Create a new Required Property structure and initialize it with a top level OR followed by a top level AND. This will allow us
		// to drop expressions into the structure as they come in through the GetNextExpression function.

		andArray := make([]interface{}, 0) // An array of PropertyExpression.
		orArray := make([]interface{}, 0)  // An array of "and" structures, each with an array of PropertyExpression.

		// Get a handle to the specific language handler we will be using.
		handler, err = extConstraint.GetLanguageHandler()
		if err != nil {
			return nil, errors.New(fmt.Sprintf("unable to obtain policy constraint language handler, error %v", err))
		}

		// Loop until there are no more property expressions to consume.
		for {

			// Get a property expression from the constraint expression. If there is no expression returned, then it's the
			// end of the expression.
			nextExpression, remainder, err = handler.GetNextExpression(remainder)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("unable to convert policy constraint %v into internal format, error %v", remainder, err))
			} else if nextExpression == "" {
				break
			}

			// Convert the expression string into JSON and add it into the RequiredProperty object that we're building.
			pieces := strings.Split(nextExpression, " ")
			fullValue := pieces[2]
			if len(pieces) > 3 {
				for _, piece := range pieces[3:] {
					fullValue = fmt.Sprintf("%s %s", fullValue, piece)
				}
			}
			pe := PropertyExpression_Factory(pieces[0], fullValue, pieces[1])
			andArray = append(andArray, *pe)

			// Get control operator. If no control operator is returned, then it's the end of the expression.
			controlOp, remainder, err = handler.GetNextOperator(remainder)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("unable to convert policy constraint %v into internal format, error %v", remainder, err))
			} else if controlOp == "" {
				break
			}

			// Based on the control operator expression that we get back, add the current list of ANDed expressions into the structure.
			if strings.Contains(controlOp, "&&") || strings.Contains(strings.ToUpper(controlOp), "AND") {
				// Just consume the operator and keep going.

			} else {
				// OR means we need a new element in the "or" array.
				innerAnd := map[string]interface{}{
					OP_AND: andArray,
				}
				orArray = append(orArray, innerAnd)
				andArray = make([]interface{}, 0)

			}

		}

		// Done with the expression, so close up the current and array initialize the RequiredProperty, and return it.
		innerAnd := map[string]interface{}{
			OP_AND: andArray,
		}
		orArray = append(orArray, innerAnd)

		newRP := map[string]interface{}{OP_OR: orArray}
		allPropArray = append(allPropArray, newRP)
	}
	allRP.Initialize(&map[string]interface{}{
		OP_AND: allPropArray,
	})

	return allRP, nil
}
