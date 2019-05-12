package externalpolicy

import (
	"errors"
	"fmt"
	"github.com/open-horizon/anax/externalpolicy/plugin_registry"
	_ "github.com/open-horizon/anax/externalpolicy/text_language"
	"github.com/open-horizon/anax/policy"
	"strings"
)

// This type implements all the ConstraintLanguage Plugin methods and delegates to plugin system.
type ConstraintExpression []string

func (c *ConstraintExpression) Validate() error {
	return plugin_registry.ConstraintLanguagePlugins.ValidatedByOne(*c)
}

func (c *ConstraintExpression) GetLanguageHandler() (plugin_registry.ConstraintLanguagePlugin, error) {
	return plugin_registry.ConstraintLanguagePlugins.GetLanguageHandlerByOne(*c)
}

// Create a RequiredProperty Object based on the constraint expression in an external policy. The constraint expression
// contains references to properties and provides a comparison operator and value on that property. These can be converted
// into our internal format.
func RequiredPropertyFromConstraint(extConstraint *ConstraintExpression) (*policy.RequiredProperty, error) {

	var err error
	var nextExpression, remainder, controlOp string
	var handler plugin_registry.ConstraintLanguagePlugin

	if extConstraint == nil || len(*extConstraint) == 0 {
		return nil, nil
	}

	remainder = ([]string(*extConstraint))[0]

	// Create a new Required Property structure and initialize it with a top level OR followed by a top level AND. This will allow us
	// to drop expressions into the structure as they come in through the GetNextExpression function.
	newRP := policy.RequiredProperty_Factory()

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
		pe := policy.PropertyExpression_Factory(pieces[0], pieces[2], pieces[1])

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
				policy.OP_AND: andArray,
			}
			orArray = append(orArray, innerAnd)
			andArray = make([]interface{}, 0)

		}
	}

	// Done with the expression, so close up the current and array initialize the RequiredProperty, and return it.
	innerAnd := map[string]interface{}{
		policy.OP_AND: andArray,
	}
	orArray = append(orArray, innerAnd)

	newRP.Initialize(&map[string]interface{}{
		policy.OP_OR: orArray,
	})

	return newRP, nil
}
