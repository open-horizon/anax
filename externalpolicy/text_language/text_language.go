package text_language

import (
	"errors"
	"fmt"
	"github.com/alecthomas/participle/lexer"
	"github.com/alecthomas/participle/lexer/ebnf"
	"github.com/open-horizon/anax/externalpolicy/plugin_registry"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/semanticversion"
	"strconv"
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

func (p *TextConstraintLanguagePlugin) Validate(dconstraints interface{}) (bool, []string, error) {

	var err error
	var constraints []string
	var constraint string

	// get message printer because this function is called by CLI
	msgPrinter := i18n.GetMessagePrinter()

	// Validate that the input is a ConstraintExpression type (string[])
	if !isConstraintExpression(dconstraints) {
		return false, []string{}, errors.New(msgPrinter.Sprintf("The constraint expression: %v is type %T, but is expected to be an array of strings", dconstraints, dconstraints))
	}

	// Validate that the expression is syntactically correct and parse-able
	constraints = dconstraints.([]string)
	validConstraints := make([]string, 0, 2)

	for _, constraint = range constraints {
		// 1 constraint inside constraint list
		parenCount := 0
		var exp string
		var ctrlOp string
		fullConstr := constraint

		for len(constraint) > 0 {
			exp, constraint, err = p.GetNextExpression(constraint)
			if err != nil {
				return false, nil, fmt.Errorf("Error finding an expression in %s. Error was: %v", fullConstr, err)
			}
			foundOp := false

			for !foundOp {
				ctrlOp, constraint, err = p.GetNextOperator(constraint)
				if err != nil && exp != "" {
					return false, nil, fmt.Errorf("Error finding a control operator in %s. Error was: %v", fullConstr, err)
				}

				if ctrlOp == ")" {
					parenCount--
				} else if ctrlOp == "(" {
					parenCount++
				} else {
					foundOp = true
				}
			}
		}
		if err == nil && parenCount != 0 {
			return false, nil, fmt.Errorf(msgPrinter.Sprintf("The constraint expression contains unmatched parentheses."))
		}
		validConstraints = append(validConstraints, constraint)

	}

	return true, validConstraints, nil
}

// This function parses out the next property expression and returns it along with the remainder of the expression.
// It returns, the parsed out expression, and the remainder of the full expression, or an error.
func (p *TextConstraintLanguagePlugin) GetNextExpression(expression string) (string, string, error) {
	// The input expression string should begin with an expression that can be captured and returned, or it is empty.
	// This should be true because the full expression should have been validated before calling this function.
	if strings.TrimSpace(expression) == "" {
		return "", "", nil
	}

	lexDef := getLexer()
	def := getLexer().Symbols()
	lex, err := lexDef.Lex(strings.NewReader(expression))
	if err != nil {
		return "", expression, err
	}
	nextToken, err := lex.Next()
	if err != nil {
		return "", expression, err
	}
	nextRune := nextToken.Type

	// Start of property expression. This case will consume the entire expression.
	if nextRune == def["Str"] || nextRune == def["InStr"] {
		name := nextToken.Value
		var opType rune
		var valType rune
		op := ""
		val := ""
		nextToken, err = lex.Next()
		if err != nil {
			return "", expression, fmt.Errorf("Unrecognized token found: %v", err)
		}

		nextRune = nextToken.Type
		if nextRune != def["OpEq"] && nextRune != def["OpComp"] && nextRune != def["OpIn"] {
			if len(name) > 3 && name[len(name)-2:] == "in" {
				op = "in"
				opType = def["in"]
				name = name[:len(name)-2]
				val = nextToken.Value
				valType = nextRune
			} else {
				return "", expression, fmt.Errorf("Non-operator token proceeding property name token. \"%s%s\"", name, nextToken.Value)
			}
		}
		if op == "" {
			op = nextToken.Value
			opType = nextRune
			nextToken, err = lex.Next()
			if err != nil {
				return "", expression, fmt.Errorf("Unrecognized token found: %v", err)
			}
			nextRune = nextToken.Type
		}

		if nextRune != def["Str"] && nextRune != def["InStr"] && nextRune != def["QuoteStr"] && nextRune != def["ListStr"] && nextRune != def["Vers"] && nextRune != def["VersRange"] && nextRune != def["Num"] {
			return "", expression, fmt.Errorf("Invalid property value. %v%v%v", name, op, nextToken.Value)
		}
		if val == "" {
			val = nextToken.Value
			valType = nextRune
		}
		if err = validOpValuePair(name, op, opType, val, valType, def); err != nil {
			return "", expression, err
		}
		return fmt.Sprintf("%v\a%v\a%v", name, strings.TrimSpace(op), strings.TrimSpace(val)), strings.Replace(expression, fmt.Sprintf(("%v%v%v"), name, op, val), "", 1), nil
	}
	if nextRune == def["OpenParen"] || nextRune == def["CloseParen"] {
		return "", expression, nil
	}
	return "", expression, fmt.Errorf("Next expression not found: %v", expression)
}

func (p *TextConstraintLanguagePlugin) GetNextOperator(expression string) (string, string, error) {
	// The input expression string should begin with an operator (i.e. AND, OR), or it is empty.
	// This should be true because the full expression should have been validated before calling this function. The
	// preceding expression has alreday been removed.
	if strings.TrimSpace(expression) == "" {
		return "", "", nil
	}

	lexDef := getLexer()
	def := getLexer().Symbols()
	lex, err := lexDef.Lex(strings.NewReader(expression))
	if err != nil {
		return "", expression, err
	}
	nextToken, err := lex.Next()
	if err != nil {
		return "", expression, err
	}
	nextRune := nextToken.Type

	if nextRune == def["CloseParen"] || nextRune == lexer.EOF {
		return ")", strings.Replace(expression, nextToken.Value, "", 1), nil
	}

	// Append the next element to the correct array depending on the proceeding control operator
	// For AND: append the next element to the andArray and continue

	if nextRune == def["AndOp"] || nextRune == def["OrOp"] || nextRune == def["OpenParen"] {
		op := nextToken.Value
		return strings.TrimSpace(op), strings.Replace(expression, op, "", 1), nil
	}
	return "", expression, fmt.Errorf("No control operator found. Expecting one of AND,&&,OR,||. Found: %v", expression)
}

// 1. == is supported for all types except list of strings, which would use 'in'.
// 2. for numeric types, the operators ==, <, >, <=, >= are supported
// 3. false and true are the only valid values for a boolean type
// 4. for string types, a quoted string, inside which is a list of comma separated strings provide acceptable values
// 5. string values that contain spaces must be quoted
// 6. for the version type, supported values are a single version or a range of versions in the semantic version format (the same as used for service verions). The == operator implies that the value is a single version. The 'in' operator treats the value as a version range. As with service versions, the version 1.0.0 when treated as a version range is equivalent to the explicit range [1.0.0,INFINITY).

// This function checks that the operator is valid for the specified value and validates version ranges with the semanticversion Factory function
// Returns a property expression struct with numerical values as float64
// Returns an empty property expression and an error if the property expression does not validate
func validOpValuePair(name string, op string, opType rune, val interface{}, valType rune, lexMap map[string]rune) error {
	var err error

	if lexMap["OpEq"] == opType {
		if lexMap["VersRange"] == valType {
			return fmt.Errorf("Version range can only use operator 'in'.")
		}
		if lexMap["ListStr"] == valType {
			return fmt.Errorf("Property type list of strings can only use operator 'in'.")
		}
	}
	if lexMap["OpComp"] == opType {
		if _, err := strconv.ParseFloat(val.(string), 64); err != nil {
			return fmt.Errorf("Cannot use numerical comparison operator %s with value %v.", op, val)
		}
	}
	if lexMap["OpIn"] == opType {
		if lexMap["ListStr"] != valType && lexMap["QuoteStr"] != valType && lexMap["VersRange"] != valType && lexMap["Vers"] != valType {
			return fmt.Errorf("The 'in' operator can only be used for types version and list of strings")
		}
		if lexMap["VersRange"] == valType {
			// Using the factory function to validate version ranges
			_, err = semanticversion.Version_Expression_Factory(val.(string))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func getLexer() lexer.Definition {
	return lexer.Must(ebnf.New(`
	  alphanumeric = digit | alpha .

	  vers = {digit} "." {digit} "." {digit} .
	  digit = "0"…"9" .
	  alpha = "a"…"z" | "A"…"Z" .

	  InStr = "in" {"in"} (alphanumeric | "_" | "-" | "/" | "!" | "?" | "+" | "~" | "'" | ".") {alphanumeric | "_" | "-" | "/" | "!" | "?" | "+" | "~" | "'" | "."} .

	  AndOp = whitespace {whitespace} ("AND" | "&&") whitespace {whitespace} .
	  OrOp = whitespace {whitespace} ("OR" | "||") whitespace {whitespace} .

		OpComp =  {whitespace} ( ["="] (">" | "<") ["="] ) {whitespace} .
		OpIn =  {whitespace} "in" {whitespace} .
	  OpEq =  {whitespace}  ( "!=" | "="["="] )  {whitespace} .

	  VersRange = {whitespace}  ( "(" | "[" )  vers {whitespace}  "," {whitespace}  (vers | "INFINITY")  ("]" | ")").
		Vers = {whitespace}  vers .
	  Num = {whitespace} ["-"] digit {digit} ["." {digit}] .
	  whitespace = "\n" | "\r" | "\t" | " " .
	  OpenParen = {whitespace} "(" {whitespace} .
	  CloseParen = {whitespace} ")" .


	  Str =  {whitespace} (alphanumeric | "_" | "-" | "/" | "!" | "?" | "+" | "~" | "'" | ".") {alphanumeric | "_" | "-" | "/" | "!" | "?" | "+" | "~" | "'" | "."} .
	  QuoteStr = {whitespace} "\x22" (alphanumeric  | "_" | "-" |  "/" | "!" | "?" | "+" | "~" | "." | "'" | " " | "\t") {alphanumeric | "_" | "-" |  "/" | "!" | "?" | "+" | "~" | "." | "'" | " " | "\t" } "\x22" .
		ListStr = {whitespace} "\x22" (alphanumeric  | "_" | "-" |  "/" | "!" | "?" | "+" | "~" | "." | "'" | "," | " " | "\t") {alphanumeric | "_" | "-" |  "/" | "!" | "?" | "+" | "~" | "." | "'" | "," | " " | "\t" } "\x22" .


	  Unused = digit .`))
}

func isConstraintExpression(x interface{}) bool {
	switch x.(type) {
	case []string:
		return true
	default:
		return false
	}
}

func formatLogString(v interface{}) string {
	return fmt.Sprintf("Constraint text language validation: %v", v)
}
