package policy

import (
	"errors"
	"fmt"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/externalpolicy/plugin_registry"
	"strings"
)

// The purpose this file is to abstract the CounterPartyProperties field in the Policy struct
// so that the complex processing related to handling the properties is componentized.
//
// Properties that are required by one party of another are specified in a custom syntax
// that allows for the ability to express combinations of AND, OR and NOT against
// name/value simple properties. The syntax of a RequiredProperties expression is:
//
// "counterPartyProperties": {
//     _control_operator_ : [_expression_] || _property_
// }
//
// where:
// _control_operator_    = {"and", "or", "not"}
// _expression_          = _control_operator_: [_expression_] || property
// _property_            = "name": _property_name_, "value": _property_value, "op": _comparison_operator_
// _comparison_operator_ = {"<", "=", ">", "<=", ">=", "!="}
// The "=" and "!=" comparison operators can be applied to strings and integers.
// If the "op" key is missing, then equal is assumed.
//
// See the unit tests for examples of valid and invalid syntax
//

// These are the boolean operators that can be used to construct a RequiredProperties expression
const and = "and"
const or = "or"
const not = "not"

type RequiredProperty map[string]interface{}

// Create a simple, empty RequiredProperty Object.
func RequiredProperty_Factory() *RequiredProperty {
	rp := new(RequiredProperty)
	(*rp) = make(map[string]interface{})

	return rp
}

// Create a RequiredProperty Object based on the constraint expression in an external policy. The constraint expression
// contains references to properties and provides a comparison operator and value on that property. These can be converted
// into our internal format.
func RequiredPropertyFromConstraint(extConstraint *externalpolicy.ConstraintExpression) (*RequiredProperty, error) {

	var err error
	var nextExpression, remainder, controlOp string
	var handler plugin_registry.ConstraintLanguagePlugin

	remainder = ([]string(*extConstraint))[0]

	// Create a new Required Property structure and initialize it with a top level OR followed by a top level AND. This will allow us
	// to drop expressions into the structure as they come in through the GetNextExpression function.
	newRP := RequiredProperty_Factory()

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
		pe := PropertyExpression_Factory(pieces[0], pieces[2], pieces[1])

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
				and: andArray,
			}
			orArray = append(orArray, innerAnd)
			andArray = make([]interface{}, 0)

		}

	}

	// Done with the expression, so close up the current and array initialize the RequiredProperty, and return it.
	innerAnd := map[string]interface{}{
		and: andArray,
	}
	orArray = append(orArray, innerAnd)

	newRP.Initialize(&map[string]interface{}{
		or: orArray,
	})

	return newRP, nil
}

// Return the top level elements in the requiredProperty.
func (rp *RequiredProperty) TopLevelElements() []interface{} {
	if _, ok := (*rp)[or]; ok {
		return (*rp)[or].([]interface{})
	} else if _, ok := (*rp)[and]; ok {
		return (*rp)[and].([]interface{})
	} else {
		return nil
	}
}

// These are the comparison operators that are supported
const lessthan = "<"
const greaterthan = ">"
const equalto = "=="
const lessthaneq = "<="
const greaterthaneq = ">="
const notequalto = "!="

// This struct represents property value expressions to be satisfied
type PropertyExpression struct {
	Name  string      `json:"name"`  // The Property name
	Value interface{} `json:"value"` // The Property value
	Op    string      `json:"op"`    // The operator to apply to the property value
}

func (p PropertyExpression) String() string {
	return fmt.Sprintf("PropertyExpression: Name: %v, Value: %v, Op: %v", p.Name, p.Value, p.Op)
}

func PropertyExpression_Factory(name string, value interface{}, op string) *PropertyExpression {
	pe := new(PropertyExpression)
	pe.Name = name
	pe.Value = value
	pe.Op = op

	return pe
}

// Initialize a RequiredProperty object from a plain map
func (self *RequiredProperty) Initialize(exp *map[string]interface{}) error {
	if len(*exp) != 1 {
		return errors.New(fmt.Sprintf("Input expression must have only 1 key, has %v", len(*exp)))
	} else {
		for op, val := range *exp {
			(*self)[op] = val
		}
	}
	return nil
}

// This function is used to determine if an input set of properties and values will satisfy
// the RequiredProperty expression.
func (self *RequiredProperty) IsSatisfiedBy(props []externalpolicy.Property) error {

	// Make sure the expression is valid
	if err := self.IsValid(); err != nil {
		return err
	}

	// If there is no expression at all, then there is nothing to satisify
	if len(*self) == 0 {
		return nil
	}

	// Make a copy of the object so that we can get it's type correct
	topMap := make(map[string]interface{})
	for k := range *self {
		topMap[k] = (*self)[k]
	}
	// Evaluate the RequiredProperty object against the supplied properties
	return self.satisfied(&topMap, &props)
}

// This function does the real work of evaluating the expression to see if it is satisfied by
// the list of properties and values that have been supplied. This function is called
// recursively because control operators can be nested n levels deep.
func (self *RequiredProperty) satisfied(cop *map[string]interface{}, props *[]externalpolicy.Property) error {
	controlOp := self.getControlOperator(cop)
	if controlOp == and {

		propArray := (*cop)[controlOp].([]interface{})
		for _, p := range propArray {
			if prop := isPropertyExpression(p); prop != nil {
				if !propertyInArray(prop, props) {
					return errors.New(fmt.Sprintf("Property %v with value %v not in %v\n", prop.Name, prop.Value, props))
				}
			} else if cop := isControlOp(p); cop != nil {
				if err := self.satisfied(cop, props); err != nil {
					return err
				}
			} else {
				return errors.New(fmt.Sprintf("Control Operator contains an element that is not a Property and not a control operator %v\n", p))
			}
		}
		return nil

	} else if controlOp == or {

		propArray := (*cop)[controlOp].([]interface{})
		for _, p := range propArray {
			if prop := isPropertyExpression(p); prop != nil {
				if propertyInArray(prop, props) {
					return nil
				}
			} else if cop := isControlOp(p); cop != nil {
				if err := self.satisfied(cop, props); err != nil {
					continue
				} else {
					return nil
				}
			} else {
				return errors.New(fmt.Sprintf("Control Operator contains an element that is not a Property and not a control operator %v\n", p))
			}
		}
		return errors.New(fmt.Sprintf("One of Required Properties %v not in %v\n", propArray, props))

	} else if controlOp == not {

	}

	return nil
}

// This function is used to verify that the RequiredProperty expression is syntactically valid.
func (self *RequiredProperty) IsValid() error {

	// Handle completely empty case, nothing to verify is therefore valid
	if len(*self) == 0 {
		return nil
	}

	// Make a copy of the object so that we can get it's type correct
	topMap := make(map[string]interface{})
	for k := range *self {
		topMap[k] = (*self)[k]
	}
	// Validate the expression
	return self.verify(&topMap)

}

// This function does the real work of validating the expression to see if it is syntactically
// correct. This function is called recursively because control operators can be nested n levels deep.
func (self *RequiredProperty) verify(cop *map[string]interface{}) error {

	// A Control Operator map should only have 1 key
	if len(*cop) != 1 {
		return errors.New(fmt.Sprintf("RequiredProperty Object not valid, %v should have 1 top level key, has %v", *cop, len(*cop)))
	}

	// Make sure the top level key is supported
	keys := getKeys(*cop)
	if _, ok := controlOperators()[keys[0]]; !ok {
		return errors.New(fmt.Sprintf("RequiredProperty Object not valid, top level key has to be one of %v, is %v", controlOperators(), keys))
	}

	// Iterate through the expression
	controlOp := self.getControlOperator(cop)

	// Ensure the control operator value is an array
	if !isArray((*cop)[controlOp]) {
		return errors.New(fmt.Sprintf("RequiredProperty Object not valid, control operator value is not an array, is %v", (*cop)[controlOp]))
	}

	propArray := (*cop)[controlOp].([]interface{})
	for _, p := range propArray {
		if prop := isPropertyExpression(p); prop != nil {
			continue
		} else if cop := isControlOp(p); cop != nil {
			if err := self.verify(cop); err != nil {
				return err
			}
		} else {
			return errors.New(fmt.Sprintf("Control Operator contains an element that is not a Property and not a control operator %v\n", p))
		}
	}

	return nil
}

// This function will merge 2 RequiredProperty expressions together by ANDing them.
func (self *RequiredProperty) Merge(other *RequiredProperty) *RequiredProperty {

	merged_rp := new(RequiredProperty)
	// Only merge if we need to
	if len(*self) == 0 && len(*other) == 0 {
		return merged_rp
	} else if len(*self) == 0 {
		return other
	} else if len(*other) == 0 {
		return self
	}

	// Setup the new structure to hold the merged expressions.
	(*merged_rp) = make(map[string]interface{})
	(*merged_rp)["and"] = make([]interface{}, 0, 10)

	// Establish variables with the right type so that the expression is validateable by
	// the class.
	var self_map map[string]interface{}
	var other_map map[string]interface{}
	self_map = (*self)
	other_map = (*other)

	// Add the 2 expressions to the new parent AND structure's array.
	(*merged_rp)["and"] = append((*merged_rp)["and"].([]interface{}), self_map)
	(*merged_rp)["and"] = append((*merged_rp)["and"].([]interface{}), other_map)
	return merged_rp
}

// ========================================================================================================
// These are internal utility functions used by this module.
//

// A simple function used to extract the 1 and only key of the input map. Callers of this function
/// must check that there is only 1 key in the map before calling.
func (self *RequiredProperty) getControlOperator(m *map[string]interface{}) string {
	return getKeys(*m)[0]
}

// Return a map of control operators so that it's easy to check if a string is equivalent to one
// of the supported control operators.
func controlOperators() map[string]int {
	// return map[string]int {and:0, or:0, not:0}
	return map[string]int{and: 0, or: 0}
}

// Return a map of comparison operators so that it's easy to check if a string is equivalent to one
// of the supported comparison operators.
func comparisonOperators() map[string]int {
	// return map[string]int {and:0, or:0, not:0}
	return map[string]int{lessthan: 0, greaterthan: 0, equalto: 0, lessthaneq: 0, greaterthaneq: 0, notequalto: 0}
}

// Return a map of comparison operators that only work on strings
func stringOperators() map[string]int {
	return map[string]int{equalto: 0, notequalto: 0}
}

// This function checks the type of the input interface object to see if it's a map of string to
// interface. Control operators and Properties are both of this type when deserialized by the
// JSON library.
func isMap(x interface{}) bool {
	switch x.(type) {
	case map[string]interface{}:
		return true
	default:
		return false
	}
}

// This function checks the type of the input interface object to see if it's type is a PropertyExpression.
func isPropertyExpressionType(x interface{}) bool {
	switch x.(type) {
	case PropertyExpression:
		return true
	default:
		return false
	}
}

// This function checks the type of the input interface object to see if it's a map of string to
// interface that complies with the definition of a PropertyExpression object. If so, the input parameter
// is used to construct a PropertyExpression object and return it.
func isPropertyExpression(x interface{}) *PropertyExpression {
	if isPropertyExpressionType(x) {
		pe := x.(PropertyExpression)
		return &pe
	} else if !isMap(x) {
		return nil
	} else {
		asMap := x.(map[string]interface{})
		if _, ok := asMap["name"]; !ok {
			return nil
		} else if _, ok := asMap["value"]; !ok {
			return nil
		} else {
			p := new(PropertyExpression)
			p.Name = asMap["name"].(string)
			p.Value = asMap["value"]
			if _, ok := asMap["op"]; !ok {
				p.Op = equalto
			} else if _, ok := comparisonOperators()[asMap["op"].(string)]; ok {
				p.Op = asMap["op"].(string)
			} else {
				return nil
			}
			return p
		}
	}
}

// This function checks the type of the input interface object to see if it's a map of string to
// interface that complies with the definition of a Control Operator object. If so, the input parameter
// is used to construct a map of string to interface and return it.
func isControlOp(x interface{}) *map[string]interface{} {
	if !isMap(x) {
		return nil
	} else {
		asMap := x.(map[string]interface{})
		keys := getKeys(asMap)
		if _, ok := controlOperators()[keys[0]]; !ok {
			return nil
		}
		return &asMap
	}
}

// This function checks the type of the input interface object to see if it's an array. The value of a
// Control operator is always an array.
func isArray(x interface{}) bool {
	switch x.(type) {
	case []interface{}:
		return true
	default:
		return false
	}
}

// This function checks the type of the input interface object to see if it's an int.
func isFloat64(x interface{}) bool {
	switch x.(type) {
	case float64:
		return true
	default:
		return false
	}
}

// This function checks the type of the input interface object to see if it's a string.
func isString(x interface{}) bool {
	switch x.(type) {
	case string:
		return true
	default:
		return false
	}
}

// This function checks the type of the input interface object to see if it's a boolean.
func isBoolean(x interface{}) bool {
	switch x.(type) {
	case bool:
		return true
	default:
		return false
	}
}

// This function extracts all the keys from the input map and returns them in a string array.
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, 10)
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// This function compares a Property object with an array of Property objects to see if it's
// in the array with an appropriate value.
func propertyInArray(propexp *PropertyExpression, props *[]externalpolicy.Property) bool {
	for _, p := range *props {
		if p.Name != propexp.Name {
			// These are not the droids we're looking for
			continue
		} else {
			if isFloat64(p.Value) && isFloat64(propexp.Value) {
				if propexp.Op == lessthan {
					return p.Value.(float64) < propexp.Value.(float64)
				} else if propexp.Op == greaterthan {
					return p.Value.(float64) > propexp.Value.(float64)
				} else if propexp.Op == lessthaneq {
					return p.Value.(float64) <= propexp.Value.(float64)
				} else if propexp.Op == greaterthaneq {
					return p.Value.(float64) >= propexp.Value.(float64)
				} else if propexp.Op == notequalto {
					return p.Value.(float64) != propexp.Value.(float64)
				} else {
					return p.Value.(float64) == propexp.Value.(float64)
				}
			} else if isBoolean(p.Value) && isBoolean(propexp.Value) {
				if _, ok := stringOperators()[propexp.Op]; !ok {
					return false
				} else if propexp.Op == notequalto {
					return p.Value.(bool) != propexp.Value.(bool)
				} else if propexp.Op == equalto {
					return p.Value.(bool) == propexp.Value.(bool)
				}
			} else if isString(p.Value) && isString(propexp.Value) {
				if _, ok := stringOperators()[propexp.Op]; !ok {
					return false
				} else if propexp.Op == notequalto {
					return p.Value.(string) != propexp.Value.(string)
				} else {
					return p.Value.(string) == propexp.Value.(string)
				}
			}
		}
	}
	return false
}
