// +build unit

package externalpolicy

import (
	_ "github.com/open-horizon/anax/externalpolicy/text_language"
	"testing"
)

// ================================================================================================================
// Verify the function that converts external policy constraint expressions to the internal JSON format, for simple
// constraint expressions.
//
func Test_simple_conversion(t *testing.T) {

	ce := new(ConstraintExpression)

	(*ce) = append((*ce), "prop == value")
	if rp, err := RequiredPropertyFromConstraint(ce); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	} else if tle := rp.TopLevelElements(); tle == nil {
		t.Errorf("Error: There should be a top level array element")
	} else if len(tle) != 1 {
		t.Errorf("Error: Should be 1 top level array alement")
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce), "prop == value && prop2 == value2")
	if rp, err := RequiredPropertyFromConstraint(ce); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	} else if tle := rp.TopLevelElements(); tle == nil {
		t.Errorf("Error: There should be a top level array element")
	} else if len(tle) != 1 {
		t.Errorf("Error: Should be 1 top level array alement")
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce), "prop == value && prop2 == value2 || prop3 == value3")
	if rp, err := RequiredPropertyFromConstraint(ce); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	} else if tle := rp.TopLevelElements(); tle == nil {
		t.Errorf("Error: There should be 2 top level array elements")
	} else if len(tle) != 1 {
		t.Errorf("Error: Should be 1 top level array alement")
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce), "prop == value && prop2 == value2 || prop3 == value3 || prop4 == value4")
	if rp, err := RequiredPropertyFromConstraint(ce); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	} else if tle := rp.TopLevelElements(); tle == nil {
		t.Errorf("Error: There should be 3 top level array elements")
	} else if len(tle) != 1 {
		t.Errorf("Error: Should be 1 top level array alement len(tle): %v tle: %v", len(tle), tle)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce), "prop == value && prop2 == value2 || prop3 == value3 || prop4 == value4 && prop5 == value5")
	if rp, err := RequiredPropertyFromConstraint(ce); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	} else if tle := rp.TopLevelElements(); tle == nil {
		t.Errorf("Error: There should be 3 top level array elements")
	} else if len(tle) != 1 {
		t.Errorf("Error: Should be 1 top level array alement")
	}
}

func Test_succeed_IsSatisfiedBy(t *testing.T) {
	ce := new(ConstraintExpression)
	(*ce) = append((*ce), "prop == true && prop2 == value2")
	props := new([]Property)
	(*props) = append((*props), *(Property_Factory("prop", true)), *(Property_Factory("prop2", "value2")), *(Property_Factory("prop3", "value3")), *(Property_Factory("prop4", "value4")), *(Property_Factory("prop5", "value5")))
	if err := ce.IsSatisfiedBy(*props); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce), "prop == onefishtwofish && prop2 == value2")
	props = new([]Property)
	(*props) = append((*props), *(Property_Factory("prop", "onefishtwofish")), *(Property_Factory("prop2", "value2")), *(Property_Factory("prop3", "value3")), *(Property_Factory("prop4", "value4")), *(Property_Factory("prop5", "value5")))
	if err := ce.IsSatisfiedBy(*props); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce), "prop == value && prop2 == value2", "prop3 == value3 || prop4 == value4 && prop5 <= 5", "property6 >= 6")
	props = new([]Property)
	(*props) = append((*props), *(Property_Factory("prop", "value")), *(Property_Factory("prop2", "value2")), *(Property_Factory("prop3", "value3")), *(Property_Factory("prop4", "value4")), *(Property_Factory("prop5", 5)), *(Property_Factory("property6", 7.0)))
	if err := ce.IsSatisfiedBy(*props); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce),
		"iame2edev == true && cpu == 3 || memory <= 32",
		"hello == \"world\"",
		//"hello in \"'hiworld', 'test'\"",
		"eggs == \"truckload\" AND certification in \"USDA,Organic\"",
		"version == 1.1.1 OR USDA == true",
		"version in [1.1.1,INFINITY) OR cert == USDA")
	prop_list := `[{"name":"iame2edev", "value":true},{"name":"cpu", "value":3},{"name":"memory", "value":32},{"name":"hello", "value":"world"},{"name":"eggs","value":"truckload"},{"name":"USDA","value":true},{"name":"certification","value":"USDA"},{"name":"version","value":"1.2.1","type":"version"}]`
	props = create_property_list(prop_list, t)
	if err := ce.IsSatisfiedBy(*props); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce),
		"version == 1.1.1 OR USDA in \"United,States,Department,of,Agriculture\"",
		"version in [1.1.1,INFINITY) OR cert == USDA",
		"color == \"orange\"")
	prop_list = `[{"name":"version", "value":"2.1.5", "type":"version"},{"name":"USDA", "value":"Department"},{"name":"color", "value":"orange","type":"string"}]`
	props = create_property_list(prop_list, t)
	if err := ce.IsSatisfiedBy(*props); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	}
}

func Test_fail_IsSatisfiedBy(t *testing.T) {
	ce := new(ConstraintExpression)
	(*ce) = append((*ce), "prop == true && prop2 == \"value2, value3, value4\"")
	props := new([]Property)
	(*props) = append((*props), *(Property_Factory("prop", true)), *(Property_Factory("prop2", "value3")), *(Property_Factory("prop3", "value3")), *(Property_Factory("prop4", "value4")), *(Property_Factory("prop5", "value5")))
	if err := ce.IsSatisfiedBy(*props); err == nil {
		t.Errorf("Error: constraints not satisfied but no error occured %v %v", ce, props)
	}
}

func Test_MergeWith(t *testing.T) {
	ce1 := new(ConstraintExpression)
	ce2 := new(ConstraintExpression)
	(*ce1) = append((*ce1), "prop == true")
	(*ce2) = append((*ce2), "prop == true")
	ce1.MergeWith(ce2)
	if len(*ce1) != 1 {
		t.Errorf("Error: constraints %v should have 1 element but got %v", ce1, len(*ce1))
	}

	ce1 = new(ConstraintExpression)
	ce2 = new(ConstraintExpression)
	(*ce1) = append((*ce1),
		"version == 1.1.1 OR USDA in \"United,States,Department,of,Agriculture\"",
		"version in [1.1.1,INFINITY) OR cert == USDA",
		"color == \"orange\"")
	(*ce2) = append((*ce2),
		"version == 1.1.1 OR USDA in \"United,States,Department,of,Agriculture\"",
		"version in [1.1.1,INFINITY) OR cert == USDA",
		"color == \"orange\"")
	ce1.MergeWith(ce2)
	if len(*ce1) != 3 {
		t.Errorf("Error: constraints %v should have 3 elements but got %v", ce1, len(*ce1))
	}

	ce1 = new(ConstraintExpression)
	ce2 = new(ConstraintExpression)
	(*ce1) = append((*ce1),
		"version == 1.1.1 OR USDA in \"United,States,Department,of,Agriculture\"",
		"version in [1.1.1,INFINITY) OR cert == USDA",
		"color == \"orange\"")
	(*ce2) = append((*ce2),
		"version == 1.1.1 OR USDA in \"United,States,Department,of,Agriculture\"",
		"version in [1.1.1,INFINITY) OR cert == USDA_PART1")
	ce1.MergeWith(ce2)
	if len(*ce1) != 4 {
		t.Errorf("Error: constraints %v should have 4 elements but got %v", ce1, len(*ce1))
	}
}
