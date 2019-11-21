package externalpolicy

import (
	"fmt"
	"github.com/open-horizon/anax/externalpolicy/plugin_registry"
	"strings"
)

// This type implements all the ConstraintLanguage Plugin methods and delegates to plugin system.
type ConstraintExpression []string

func (c *ConstraintExpression) Validate() ([]string, error) {
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

// merge two constraint into one, remove the duplicates.
func (c *ConstraintExpression) MergeWith(other *ConstraintExpression) {
	// the function that checks if an element is in array
	s_contains := func(s_array []string, elem string) bool {
		for _, e := range s_array {
			if e == elem {
				return true
			}
		}
		return false
	}

	if other == nil {
		return
	}

	for _, new_ele := range *other {
		if !s_contains((*c), new_ele) {
			(*c) = append((*c), new_ele)
		}
	}
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

	allPropArray := make([]interface{}, 0)
	allRP := RequiredProperty_Factory()

	var handler plugin_registry.ConstraintLanguagePlugin
	var err error

	if extConstraint == nil || len(*extConstraint) == 0 {
		return allRP, nil
	}

	for _, remainder := range *extConstraint {
		remainder := strings.Replace(remainder, "\a", " ", -1)

		// Get a handle to the specific language handler we will be using.
		handler, err = extConstraint.GetLanguageHandler()
		if err != nil {
			return nil, fmt.Errorf("unable to obtain policy constraint language handler, error %v", err)
		}

		// Create a new Required Property structure and initialize it with a top level OR followed by a top level AND. This will allow us
		// to drop expressions into the structure as they come in through the GetNextExpression function.

		allPropArray, _, err = parseConstraintExpression(remainder, allPropArray, handler)
		if err != nil {
			return nil, err
		}

		allRP.Initialize(&map[string]interface{}{
			OP_AND: allPropArray,
		})
	}
	return allRP, nil
}

// parse a single constraint expression. This function is called recursively to handle parenthetical expressions
func parseConstraintExpression(constraint string, allPropArray []interface{}, handler plugin_registry.ConstraintLanguagePlugin) ([]interface{}, string, error) {
	andArray := make([]interface{}, 0)
	orArray := make([]interface{}, 0)

	var err error
	var nextProp string
	var ctrlOp string
	var subExpr []interface{}

	for err == nil {
		// Start of property expression. This case will consume the entire expression.
		nextProp, constraint, err = handler.GetNextExpression(constraint)
		if err != nil {
			return nil, constraint, err
		}
		if strings.TrimSpace(nextProp) != "" {
			prop := strings.Split(nextProp, "\a")
			andArray = append(andArray, *PropertyExpression_Factory(prop[0], strings.TrimSpace(prop[2]), strings.TrimSpace(prop[1])))
		}

		ctrlOp, constraint, err = handler.GetNextOperator(constraint)
		if err != nil {
			return nil, constraint, err
		}

		if ctrlOp == "(" {
			// handle a parenthetical expression as a seperate constraint expression
			subExpr, constraint, err = parseConstraintExpression(constraint, allPropArray, handler)
			if err != nil {
				return nil, constraint, err
			}

			for _, elem := range subExpr {
				andArray = append(andArray, elem)
			}
			ctrlOp, constraint, err = handler.GetNextOperator(constraint)
			if err != nil {
				return nil, constraint, err
			}
		}
		if ctrlOp == "||" || ctrlOp == "OR" {
			innerAnd := map[string]interface{}{
				OP_AND: andArray,
			}
			orArray = append(orArray, innerAnd)
			andArray = make([]interface{}, 0)
		} else if ctrlOp == ")" || ctrlOp == "" {
			// end or expression. append andArray to orArray and append the result to allPropArray
			innerAnd := map[string]interface{}{
				OP_AND: andArray,
			}
			orArray = append(orArray, innerAnd)
			newRP := map[string]interface{}{OP_OR: orArray}
			allPropArray = append(allPropArray, newRP)
			return allPropArray, constraint, nil
		}
	}

	return nil, constraint, err
}
