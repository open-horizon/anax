// +build unit

package externalpolicy

import (
	"testing"
)

// ================================================================================================================
// Verify the function that converts external policy constraint expressions to the internal JSON format, for simple
// constraint expressions.
//
func Test_simple_conversion(t *testing.T) {

	ce := new(ConstraintExpression)

	(*ce) = append((*ce), "prop == \"value\"")
	if rp, err := ce.RequiredPropertyFromConstraint(); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	} else if tle := rp.TopLevelElements(); tle == nil {
		t.Errorf("Error: There should be a top level array element")
	} else if len(tle) != 1 {
		t.Errorf("Error: Should be 1 top level array alement")
	} else {
		prop_list := `[{"name":"prop", "value":"value"}]`

		if pa := create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Error(err)
			}
		}

		prop_list = `[{"name":"propA", "value":"value"}]`

		if pa := create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err == nil {
				t.Errorf("Error: properties %v should not satisfy %v", prop_list, rp)
			}
		}
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce), `prop == "value" && prop2 == "value2"`)
	if rp, err := ce.RequiredPropertyFromConstraint(); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	} else if tle := rp.TopLevelElements(); tle == nil {
		t.Errorf("Error: There should be a top level array element")
	} else if len(tle) != 1 {
		t.Errorf("Error: Should be 1 top level array alement")
	} else {
		prop_list := `[{"name":"prop", "value":"value"},{"name":"prop2", "value":"value2"}]`

		if pa := create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Error(err)
			}
		}

		prop_list = `[{"name":"prop", "value":"value"}]`

		if pa := create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err == nil {
				t.Errorf("Error: properties %v should not satisfy %v", prop_list, rp)
			}
		}
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce), `prop == "value" && prop2 == "value2" || prop3 == "value3"`)
	if rp, err := ce.RequiredPropertyFromConstraint(); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	} else if tle := rp.TopLevelElements(); tle == nil {
		t.Errorf("Error: There should be 2 top level array elements")
	} else if len(tle) != 2 {
		t.Errorf("Error: Should be 2 top level array alements")
	} else {
		prop_list := `[{"name":"prop3", "value":"value3"}]`

		if pa := create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Error(err)
			}
		}

		prop_list = `[{"name":"prop2", "value":"value2"}]`

		if pa := create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err == nil {
				t.Errorf("Error: properties %v should not satisfy %v", prop_list, rp)
			}
		}
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce), `prop == "value" && prop2 == "value2" || prop3 == "value3" || prop4 == "value4"`)
	if rp, err := ce.RequiredPropertyFromConstraint(); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	} else if tle := rp.TopLevelElements(); tle == nil {
		t.Errorf("Error: There should be 3 top level array elements")
	} else if len(tle) != 3 {
		t.Errorf("Error: Should be 3 top level array alements")
	} else {
		prop_list := `[{"name":"prop4", "value":"value4"}]`

		if pa := create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Error(err)
			}
		}

		prop_list = `[{"name":"prop2", "value":"value2"}]`

		if pa := create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err == nil {
				t.Errorf("Error: properties %v should not satisfy %v", prop_list, rp)
			}
		}
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce), `prop == "value" && prop2 == "value2" || prop3 == "value3" || prop4 == "value4" && prop5 == "value5"`)
	if rp, err := ce.RequiredPropertyFromConstraint(); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	} else if tle := rp.TopLevelElements(); tle == nil {
		t.Errorf("Error: There should be 3 top level array elements")
	} else if len(tle) != 3 {
		t.Errorf("Error: Should be 3 top level array alement")
	} else {
		prop_list := `[{"name":"prop3", "value":"value3"}]`

		if pa := create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Error(err)
			}
		}

		prop_list = `[{"name":"prop", "value":"value"}]`

		if pa := create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err == nil {
				t.Errorf("Error: properties %v should not satisfy %v", prop_list, rp)
			}
		}
	}

}
