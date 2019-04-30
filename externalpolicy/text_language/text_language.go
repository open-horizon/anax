package text_language

import (
	"errors"
	"fmt"
	"github.com/open-horizon/anax/externalpolicy/plugin_registry"
	"strings"
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

	// Validate that the input is a ConstraintExpression type (string)

	// Validate that the expression is syntactically correct and parse-able

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
