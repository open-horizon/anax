// +build unit

package text_language

import (
	"fmt"
	"testing"

	"github.com/open-horizon/anax/externalpolicy"
)

func Test_Validate_Succeed1(t *testing.T) {
	// boolean, int, string
	textConstraintLanguagePlugin := NewTextConstraintLanguagePlugin()
	constraintStrings := []string{
		"iame2edev == true && cpu == 3 NOT memory <= 32",
		"hello == \"world\"",
		"hello in \"'hi world', 'test'\"",
		"eggs == \"truck load\" AND certification in \"USDA, Organic\"",
		"version == 1.1.1 OR USDA == true",
		"version in [1.1.1,INFINITY) ^ cert == USDA",
	}
	ce := externalpolicy.ConstraintExpression(constraintStrings)

	fmt.Println("ce: ", ce)

	var validated bool
	var err error

	validated, err = textConstraintLanguagePlugin.Validate(interface{}(ce))
	if validated == false {
		t.Errorf("Should validate successfully but not, err: %v", err)
	} else if err != nil {
		t.Errorf("Should validated and don't return err, but returned err: %v", err)
	}
}

func Test_Validate_Failed1(t *testing.T) {

	// string with multiple words, string list should not support ==
	textConstraintLanguagePlugin := NewTextConstraintLanguagePlugin()
	constraintStrings := []string{"eggs == \"truck load\" || certification == \"USDA, Organic\""}
	ce := externalpolicy.ConstraintExpression(constraintStrings)

	var validated bool
	var err error

	validated, err = textConstraintLanguagePlugin.Validate(interface{}(ce))
	if validated == true {
		t.Errorf("Validation should fail but not, err: %v", err)
	} else if err == nil {
		t.Errorf("Validated should fail and return err, but didn't")
	}

	// string list should not support ==
	constraintStrings = []string{"hello == \"'hi world', 'test'\""}
	ce = externalpolicy.ConstraintExpression(constraintStrings)

	validated, err = textConstraintLanguagePlugin.Validate(interface{}(ce))
	if validated == true {
		t.Errorf("Validation should fail but not, err: %v", err)
	} else if err == nil {
		t.Errorf("Validated should fail and return err, but didn't")
	}
}

func Test_Validate_Failed2(t *testing.T) {

	// <, > only supported for numeric value
	textConstraintLanguagePlugin := NewTextConstraintLanguagePlugin()
	constraintStrings := []string{"iame2edev < true && cpu == 3 NOT memory <= 32", "hello > world"}
	ce := externalpolicy.ConstraintExpression(constraintStrings)

	var validated bool
	var err error

	validated, err = textConstraintLanguagePlugin.Validate(interface{}(ce))
	if validated == true {
		t.Errorf("Validation should fail but not, err: %v", err)
	} else if err == nil {
		t.Errorf("Validated should fail and return err, but didn't")
	}
}

func Test_Validate_Failed3(t *testing.T) {

	// true should only supported as boolean value, not string value
	textConstraintLanguagePlugin := NewTextConstraintLanguagePlugin()
	constraintStrings := []string{"version in [1.1.1,INFINITY) OR USDA == \"true\""}
	ce := externalpolicy.ConstraintExpression(constraintStrings)

	var validated bool
	var err error

	validated, err = textConstraintLanguagePlugin.Validate(interface{}(ce))
	if validated == true {
		t.Errorf("Validation should fail but not, err: %v", err)
	} else if err == nil {
		t.Errorf("Validated should fail and return err, but didn't")
	}
}

func Test_Validate_Failed4(t *testing.T) {

	// string must be quoted if it has white space
	textConstraintLanguagePlugin := NewTextConstraintLanguagePlugin()
	constraintStrings := []string{"eggs == truck load"}
	ce := externalpolicy.ConstraintExpression(constraintStrings)

	var validated bool
	var err error

	validated, err = textConstraintLanguagePlugin.Validate(interface{}(ce))
	if validated == true {
		t.Errorf("Validation should fail but not, err: %v", err)
	} else if err == nil {
		t.Errorf("Validated should fail and return err, but didn't")
	}
}
