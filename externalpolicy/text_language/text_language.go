package text_language

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/externalpolicy/plugin_registry"
	"github.com/open-horizon/anax/policy"
)

func init() {
	plugin_registry.Register("text", NewTextConstraintLanguagePlugin())
}

type TextConstraintLanguagePlugin struct {
}

func NewTextConstraintLanguagePlugin() plugin_registry.ConstraintLanguagePlugin {
	return new(TextConstraintLanguagePlugin)
}

func (p *TextConstraintLanguagePlugin) Validate(dconstraints interface{}) (bool, error) {

	var err error
	var constraints externalpolicy.ConstraintExpression
	var nextExpression, nextLogicalOperator, remainder, constraint string
	var validated bool

	// Validate that the input is a ConstraintExpression type (string[])
	if !isConstraintExpression(dconstraints) {
		return false, errors.New(fmt.Sprintf("The constraint expression: %v is type %T, but is expected to be an array of strings", dconstraints, dconstraints))
	}

	// Validate that the expression is syntactically correct and parse-able
	constraints = dconstraints.(externalpolicy.ConstraintExpression)

	for _, constraint = range constraints {
		// 1 constrain inside constrain list
		// handles space inside quote and inside string list
		constraint = preprocessConstraintExpression(constraint)
		remainder = constraint

		for {
			nextExpression, remainder, err = p.GetNextExpression(remainder)

			if err != nil {
				return false, errors.New(fmt.Sprintf("unable to convert policy constraint %v into internal format, error %v", remainder, err))
			} else if nextExpression == "" {
				break
			}

			validated, err = validateOneConstraintExpression(nextExpression)
			if !validated {
				return false, err
			}

			nextLogicalOperator, remainder, err = p.GetNextOperator(remainder)
			if err != nil {
				return false, errors.New(fmt.Sprintf("unable to convert policy constraint %v into internal format, error %v", remainder, err))
			} else if nextLogicalOperator == "" {
				break
			}

			// verify logical operators
			if !isAllowedLogicalOpType(nextLogicalOperator) {
				return false, errors.New(fmt.Sprintf("Logical operator %v is not valid, expecting AND, OR, &&, ||", nextLogicalOperator))
			}

		}

	}

	return true, nil
}

// This function parses out the next property expression and returns it along with the remainder of the expression.
// It returns, the parsed out expression, and the remainder of the full expression, or an error.
func (p *TextConstraintLanguagePlugin) GetNextExpression(expression string) (string, string, error) {

	// The input expression string should begin with an expression that can be captured and returned, or it is empty.
	// This should be true because the full expression should have been validated before calling this function.

	if len(expression) == 0 {
		return "", "", nil
	}

	// Split the expression based on whitespace in the string.
	pieces := strings.Split(expression, " ")
	if len(pieces) < 3 {
		return "", "", errors.New(fmt.Sprintf("found %v token(s), expecting 3 in an expression %v, expected form is <property> == <value>", len(pieces), expression))
	}

	// Reform the expression and return the remainder of the expression.
	exp := fmt.Sprintf("%v %v %v", pieces[0], pieces[1], pieces[2])
	return exp, strings.Join(pieces[3:], " "), nil

}

func (p *TextConstraintLanguagePlugin) GetNextOperator(expression string) (string, string, error) {

	// The input expression string should begin with an operator (i.e. AND, OR), or it is empty.
	// This should be true because the full expression should have been validated before calling this function. The
	// preceding expression has alreday been removed.

	if len(expression) == 0 {
		return "", "", nil
	}

	// Split the expression based on whitespace in the string.
	pieces := strings.Split(expression, " ")
	if len(pieces) < 4 {
		return "", "", errors.New(fmt.Sprintf("found %v token(s), expecting 4 with an operator plus an expression %v, expected form is <operator> <property> == <value>", len(pieces), expression))
	}

	// Reform the expression and return the remainder of the expression.
	return pieces[0], strings.Join(pieces[1:], " "), nil
}

func isConstraintExpression(x interface{}) bool {
	switch x.(type) {
	case externalpolicy.ConstraintExpression:
		return true
	default:
		return false
	}
}

func isString(x interface{}) bool {
	switch x.(type) {
	case string:
		return true
	default:
		return false
	}
}

func isCommaSeparatedStringList(x string) bool {
	if len(x) == 0 {
		return false
	}

	if !canParseToString(x) {
		return false
	}

	if !strings.Contains(x, ",") {
		return false
	}

	return true
}

func canParseToInteger(s string) bool {
	_, err := strconv.Atoi(s)
	if err == nil {
		return true
	}
	return false
}

func canParseToFloat(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	if err == nil {
		return true
	}
	return false
}

func canParseToBoolean(s string) bool {
	_, err := strconv.ParseBool(s)
	if err == nil {
		return true
	}
	return false
}

func canParseToStringList(s interface{}) bool {
	switch s.(type) {
	case []string:
		return true
	default:
		return false
	}
}

func canParseToString(s string) bool {
	if len(s) > 0 && s[0] == '"' && s[len(s)-1] == '"' {
		content := strings.Trim(s, "\"")
		glog.V(5).Infof(formatLogString(fmt.Sprintf("content after removing quote: %v", content)))
		if strings.ToLower(content) == "true" || strings.ToLower(content) == "false" {

			return false
		}
	}

	if strings.Contains(s, " ") {
		glog.V(5).Infof("word with space not quoted: %v", s)
		return false
	}

	return true
}

func isAllowedType(s string) bool {
	if canParseToString(s) || canParseToBoolean(s) || canParseToInteger(s) || canParseToFloat(s) || canParseToStringList(s) || policy.IsVersionString(s) || policy.IsVersionExpression(s) {
		return true
	}
	return false
}

// These are the comparison operators that are supported (get from conter_party_properties.go)
const lessthan = "<"
const greaterthan = ">"
const equalto = "="
const doubleequalto = "=="
const lessthaneq = "<="
const greaterthaneq = ">="
const notequalto = "!="
const inoperator = "in"

func isAllowedComparisonOpType(s string) bool {
	if strings.Compare(s, lessthan) == 0 || strings.Compare(s, greaterthan) == 0 || strings.Compare(s, equalto) == 0 || strings.Compare(s, doubleequalto) == 0 || strings.Compare(s, lessthaneq) == 0 || strings.Compare(s, greaterthaneq) == 0 || strings.Compare(s, notequalto) == 0 || strings.Compare(s, inoperator) == 0 {
		return true
	}
	return false
}

const andsymbol = "&&"
const orsymbol = "||"
const and = "AND"
const or = "OR"

func isAllowedLogicalOpType(s string) bool {
	if strings.Compare(s, andsymbol) == 0 || strings.Compare(s, orsymbol) == 0 || strings.Compare(s, and) == 0 || strings.Compare(s, or) == 0 {
		return true
	}
	return false
}

func preprocessConstraintExpression(str string) string {
	re := regexp.MustCompile(`(?m)"(.*?)"(?m)`)

	// remove space inside string list separate by ", "
	space := regexp.MustCompile(`,\s+`)
	s := space.ReplaceAllString(str, ",")

	for _, match := range re.FindAllString(s, -1) {
		// if find "a b", replace space inside quote with invisiable charactor \a
		newStr := strings.Replace(match, " ", "\a", -1)
		s = strings.Replace(s, match, newStr, -1)
	}

	return s
}

// 1. == is supported for all types except list of string, which would use 'in'.
// 2. for numeric types, the operators ==, <, >, <=, >= are supported
// 3. false and true are the only valid values for a boolean type
// 4. for string types, a quoted string, inside which is a list of comma separated strings provide acceptable values
// 5. string values that contain spaces must be quoted
// 6. for the version type, supported values are a single version or a range of versions in the semantic version format (the same as used for service verions). The == operator implies that the value is a single version. The 'in' operator treats the value as a version range. As with service versions, the version 1.0.0 when treated as a version range is equivalent to the explicit range [1.0.0,INFINITY).

func validateOneConstraintExpression(expression string) (bool, error) {
	if len(expression) == 0 {
		return true, nil
	}

	pieces := strings.Split(expression, " ")
	if len(pieces) < 3 {
		return false, errors.New(fmt.Sprintf("found %v token(s), expecting 3 in an expression %v, expected form is expression with 3 tokens: <property> <operator> <value> in constraint expression", len(pieces), expression))
	}

	compOp := pieces[1]
	value := pieces[2]

	// if will failed on case when string values that contain spaces but not quoted (starting from 2nd interation)
	if !isAllowedComparisonOpType(pieces[1]) {
		return false, errors.New(fmt.Sprintf("Expression: %v should contain valid comparison operator - wrong operator %v. Allowed operators: %v, %v, %v, %v, %v, %v, %v, %v", expression, pieces[1], lessthan, greaterthan, equalto, doubleequalto, lessthaneq, greaterthaneq, notequalto, inoperator))
	}

	if canParseToFloat(value) || canParseToInteger(value) {
		if strings.Compare(compOp, doubleequalto) == 0 || strings.Compare(compOp, equalto) == 0 || strings.Compare(compOp, lessthan) == 0 || strings.Compare(compOp, greaterthan) == 0 || strings.Compare(compOp, lessthaneq) == 0 || strings.Compare(compOp, greaterthaneq) == 0 {
			return true, nil
		}
		return false, errors.New(fmt.Sprintf("Comparison operator: %v is not supported for numeric value: %v", compOp, value))
	}

	if canParseToBoolean(value) {
		if strings.Compare(compOp, doubleequalto) == 0 || strings.Compare(compOp, equalto) == 0 {
			return true, nil
		}
		return false, errors.New(fmt.Sprintf("Comparison operator: %v is not supported for boolean value: %v", compOp, value))
	}

	if isCommaSeparatedStringList(value) {
		if strings.Compare(strings.ToLower(compOp), inoperator) == 0 {
			return true, nil
		}
		return false, errors.New(fmt.Sprintf("Comparison operator: %v is not supported for string list value: %v", compOp, value))
	}

	if canParseToString(value) {
		if strings.Compare(compOp, doubleequalto) == 0 || strings.Compare(compOp, equalto) == 0 {
			return true, nil
		}
		return false, errors.New(fmt.Sprintf("Comparison operator: %v is not supported for string value: %v", compOp, value))
	}

	if policy.IsVersionString(value) {
		if strings.Compare(compOp, doubleequalto) == 0 || strings.Compare(compOp, equalto) == 0 {
			return true, nil
		}
		return false, errors.New(fmt.Sprintf("Comparison operator: %v is not supported for single version: %v", compOp, value))
	}

	if policy.IsVersionExpression(value) {
		if strings.Compare(strings.ToLower(compOp), inoperator) == 0 {
			return true, nil
		}
		return false, errors.New(fmt.Sprintf("Comparison operator: %v is not supported for version expression: %v", compOp, value))
	}

	return false, errors.New(fmt.Sprintf("Expression: %v is not valid", expression))

}

func formatLogString(v interface{}) string {
	return fmt.Sprintf("Constraint text language validation: %v", v)
}
