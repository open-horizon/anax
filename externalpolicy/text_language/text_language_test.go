// +build unit

package text_language

import (
	"testing"
)

func Test_Validate_Succeed1(t *testing.T) {
	// boolean, int, string
	textConstraintLanguagePlugin := NewTextConstraintLanguagePlugin()
	constraintStrings := []string{
		"iame2edev == true && cpu == 3 || memory <= 32",
		"iame2edev == \"true\"",
		"hello == \"world\"",
		"hello in \"'hi world', 'test'\"",
		"eggs == \"truck load\" AND certification in \"USDA, Organic\"",
		"version == 1.1.1 OR USDA == true",
		"version in [1.1.1,INFINITY) OR cert == USDA",
	}
	ce := constraintStrings

	t.Log("ce: ", ce)

	var validated bool
	var err error

	validated, _, err = textConstraintLanguagePlugin.Validate(interface{}(ce))
	if validated == false {
		t.Errorf("Should validate successfully but not, err: %v", err)
	} else if err != nil {
		t.Errorf("Should validate without err, but returned err: %v", err)
	}
}

func Test_Validate_Failed1(t *testing.T) {

	// string with multiple words, string list should not support ==
	textConstraintLanguagePlugin := NewTextConstraintLanguagePlugin()
	constraintStrings := []string{"eggs == \"truck load\" || certification == \"USDA, Organic\""}
	ce := constraintStrings

	var validated bool
	var err error

	validated, _, err = textConstraintLanguagePlugin.Validate(interface{}(ce))
	if validated == true {
		t.Errorf("Validation should fail but did not, err: %v", err)
	} else if err == nil {
		t.Errorf("Validation should fail and return err, but didn't")
	} else if err.Error() != "Comparison operator: == is not supported for string list value: \"USDA,Organic\"" {
		t.Errorf("Error message: %v is not the expected error message", err)
	}

	// string list should not support ==
	constraintStrings = []string{"hello == \"'hi world', 'test'\""}
	ce = constraintStrings

	validated, _, err = textConstraintLanguagePlugin.Validate(interface{}(ce))
	if validated == true {
		t.Errorf("Validation should fail but did not, err: %v", err)
	} else if err == nil {
		t.Errorf("Validation should fail and return err, but didn't")
	} else if err.Error() != "Comparison operator: == is not supported for string list value: \"'hi\aworld','test'\"" {
		t.Errorf("Error message: %v is not the expected error message", err)
	}
}

func Test_Validate_Failed2(t *testing.T) {

	// <, > only supported for numeric value
	textConstraintLanguagePlugin := NewTextConstraintLanguagePlugin()
	constraintStrings := []string{"iame2edev < true && cpu == 3 || memory <= 32", "hello > world"}
	ce := constraintStrings

	var validated bool
	var err error

	validated, _, err = textConstraintLanguagePlugin.Validate(interface{}(ce))
	if validated == true {
		t.Errorf("Validation should fail but did not, err: %v", err)
	} else if err == nil {
		t.Errorf("Validation should fail and return err, but didn't")
	} else if err.Error() != "Comparison operator: < is not supported for boolean value: true" {
		t.Errorf("Error message: %v is not the expected error message", err)
	}
}

func Test_Validate_Failed3(t *testing.T) {

	// string must be quoted if it has white space
	textConstraintLanguagePlugin := NewTextConstraintLanguagePlugin()
	constraintStrings := []string{"eggs == truck load && certification == USDA"}
	ce := constraintStrings

	var validated bool
	var err error

	validated, _, err = textConstraintLanguagePlugin.Validate(interface{}(ce))
	if validated == true {
		t.Errorf("Validation should fail but did not, err: %v", err)
	} else if err == nil {
		t.Errorf("Validation should fail and return err, but didn't")
	} else if err.Error() != "Logical operator load is not valid, expecting AND, OR, &&, ||" {
		t.Errorf("Error message: %v is not the expected error message", err)
	}
}

func Test_Validate_Failed4(t *testing.T) {

	// invalid logical operator 'abcdefg'
	textConstraintLanguagePlugin := NewTextConstraintLanguagePlugin()
	constraintStrings := []string{"version == 1.1.1 abcdefg USDA == true"}
	ce := constraintStrings

	var validated bool
	var err error

	validated, _, err = textConstraintLanguagePlugin.Validate(interface{}(ce))
	if validated == true {
		t.Errorf("Validation should fail but did not, err: %v", err)
	} else if err == nil {
		t.Errorf("Validation should fail and return err, but didn't")
	} else if err.Error() != "Logical operator abcdefg is not valid, expecting AND, OR, &&, ||" {
		t.Errorf("Error message: %v is not the expected error message", err)
	}
}

func Test_Validate_Succeed2(t *testing.T) {

	// no whitespace between prop, operator and value.
	textConstraintLanguagePlugin := NewTextConstraintLanguagePlugin()
	constraintStrings := []string{"version==1.1.1 AND USDA == true"}
	ce := constraintStrings

	var validated bool
	var err error

	validated, _, err = textConstraintLanguagePlugin.Validate(interface{}(ce))
	if validated == false {
		t.Errorf("Validation failed but should not, err: %v", err)
	} else if err != nil {
		t.Errorf("Validation succeeded but also returned an error: %v", err)
	}
}

func Test_Validate_Succeed3(t *testing.T) {

	// no whitespace between prop, operator and value.
	textConstraintLanguagePlugin := NewTextConstraintLanguagePlugin()
	constraintStrings := []string{"version == 1.1.1 OR USDA == true", "version==1.1.1 AND USDA == true"}
	ce := constraintStrings

	var validated bool
	var err error

	validated, _, err = textConstraintLanguagePlugin.Validate(interface{}(ce))
	if validated == false {
		t.Errorf("Validation failed but should not, err: %v", err)
	} else if err != nil {
		t.Errorf("Validation succeeded but also returned an error: %v", err)
	}
}

func Test_Validate_Succeed4(t *testing.T) {

	// no whitespace between prop, operator and value.
	textConstraintLanguagePlugin := NewTextConstraintLanguagePlugin()
	constraintStrings := []string{"version>1 OR version<1 OR version>=0 AND version<=0 OR USDA==true OR USDA=false", "version=1.1.1 AND xyz!=abc"}
	ce := constraintStrings

	var validated bool
	var err error

	validated, _, err = textConstraintLanguagePlugin.Validate(interface{}(ce))
	if validated == false {
		t.Errorf("Validation failed but should not, err: %v", err)
	} else if err != nil {
		t.Errorf("Validation succeeded but also returned an error: %v", err)
	}
}

func Test_Validate_Succeed5(t *testing.T) {

	// no whitespace between prop, operator and value.
	textConstraintLanguagePlugin := NewTextConstraintLanguagePlugin()
	constraintStrings := []string{"version> 1 OR version <1 OR version >=0 AND version<= 0 OR USDA ==true OR USDA= false", "version =1.1.1 AND xyz !=123"}
	ce := constraintStrings

	var validated bool
	var err error

	validated, _, err = textConstraintLanguagePlugin.Validate(interface{}(ce))
	if validated == false {
		t.Errorf("Validation failed but should not, err: %v", err)
	} else if err != nil {
		t.Errorf("Validation succeeded but also returned an error: %v", err)
	}
}

func Test_GetNextExpression_Succeed(t *testing.T) {
	textConstraintLanguagePlugin := NewTextConstraintLanguagePlugin()
	ce := "version == 1.1.1 OR USDA == true AND book == \"one fish two fish\" && author == \"Suess\""
	rem := ce
	var err error
	for {
		_, rem, err = textConstraintLanguagePlugin.GetNextExpression(rem)
		if err != nil {
			t.Errorf("Error parsing constraint expression %v with GetNextExpression: %v", ce, err)
		}
		if rem == "" {
			break
		}
		_, rem, err = textConstraintLanguagePlugin.GetNextOperator(rem)
		if err != nil {
			t.Errorf("Error parsing constraint expression %v with GetNextOperator: %v", ce, err)
		}
	}

	ce = "iame2edev == true && iame2edev == \"true\" && cpu == 3 || memory <= 32 AND hello == \"world\" && hello in \"'hi world', 'test'\" AND eggs == \"truck load\" AND certification in \"USDA, Organic\" AND version == 1.1.1 OR USDA == true AND version in [1.1.1,INFINITY) OR cert == USDA"
	rem = ce
	for {
		_, rem, err = textConstraintLanguagePlugin.GetNextExpression(rem)
		if err != nil {
			t.Errorf("Error parsing constraint expression %v with GetNextExpression: %v", ce, err)
		}
		if rem == "" {
			break
		}
		_, rem, err = textConstraintLanguagePlugin.GetNextOperator(rem)
		if err != nil {
			t.Errorf("Error parsing constraint expression %v with GetNextOperator: %v", ce, err)
		}
	}

	ce = "iame2edev==true && cpu==3 AND hello==\"world\" && hello in \"'hi world', 'test'\""
	rem = ce
	for {
		_, rem, err = textConstraintLanguagePlugin.GetNextExpression(rem)
		if err != nil {
			t.Errorf("Error parsing constraint expression %v with GetNextExpression: %v", ce, err)
		}
		if rem == "" {
			break
		}
		_, rem, err = textConstraintLanguagePlugin.GetNextOperator(rem)
		if err != nil {
			t.Errorf("Error parsing constraint expression %v with GetNextOperator: %v", ce, err)
		}
	}

}
