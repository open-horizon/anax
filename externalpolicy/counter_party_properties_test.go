// +build unit

package externalpolicy

import (
	"encoding/json"
	_ "github.com/open-horizon/anax/externalpolicy/text_language"
	"testing"
)

// Test that valid simple expressions are detected as valid expressions.
func Test_valid_simple1(t *testing.T) {

	var rp *RequiredProperty

	simple_and := `{"and":[{"name":"prop1", "value":"val1"}]}`
	if rp = create_RP(simple_and, t); rp != nil {
		if err := rp.IsValid(); err != nil {
			t.Error(err)
		}
	}

	simple_and = `{"and":[{"name":"prop1", "value":"val1", "op":"!="}]}`
	if rp = create_RP(simple_and, t); rp != nil {
		if err := rp.IsValid(); err != nil {
			t.Error(err)
		}
	}

	simple_and = `{"and":[{"name":"prop1", "value":true, "op":"!="}]}`
	if rp = create_RP(simple_and, t); rp != nil {
		if err := rp.IsValid(); err != nil {
			t.Error(err)
		}
	}

	simple_or := `{"or":[{"name":"prop1", "value":"val1"}]}`
	if rp = create_RP(simple_or, t); rp != nil {
		if err := rp.IsValid(); err != nil {
			t.Error(err)
		}
	}

	simple_or = `{"or":[{"name":"prop1", "value":"val1", "op":"!="}]}`
	if rp = create_RP(simple_or, t); rp != nil {
		if err := rp.IsValid(); err != nil {
			t.Error(err)
		}
	}

	// simple_not := `{"not":[{"name":"prop1", "value":"val1"}]}`
	// if rp = create_RP(simple_not, t); rp != nil {
	//     if err := rp.IsValid(); err != nil {
	//         t.Error(err)
	//     }
	// }
}

// Test that invalid simple expressions as detected as invalid expressions.
func Test_invalid_simple1(t *testing.T) {

	var rp *RequiredProperty

	invalid_control_operator := `{"nand":[{"name":"prop1", "value":"val1"}]}`
	if rp = create_RP(invalid_control_operator, t); rp != nil {
		if err := rp.IsValid(); err == nil {
			t.Errorf("Error: %v is an invalid RequiredProperty value, but it was not detected as invalid.\n", invalid_control_operator)
		}
	}

	single_property := `{"name":"prop1", "value":"val1"}`
	if rp = create_RP(single_property, t); rp != nil {
		if err := rp.IsValid(); err == nil {
			t.Errorf("Error: %v is an invalid RequiredProperty value, but it was not detected as invalid.\n", single_property)
		}
	}

	invalid_control_value := `{"and":{"name":"prop1", "value":"val1"}}`
	if rp = create_RP(invalid_control_value, t); rp != nil {
		if err := rp.IsValid(); err == nil {
			t.Errorf("Error: %v is an invalid RequiredProperty value, but it was not detected as invalid.\n", invalid_control_value)
		}
	}

	invalid_control_value2 := `{"and":[{"name2":"prop1", "value":"val1"} ]}`
	if rp = create_RP(invalid_control_value2, t); rp != nil {
		if err := rp.IsValid(); err == nil {
			t.Errorf("Error: %v is an invalid RequiredProperty value, but it was not detected as invalid.\n", invalid_control_value2)
		}
	}

	invalid_control_value3 := `{"and":[{"name":"prop1", "value2":"val1"} ]}`
	if rp = create_RP(invalid_control_value3, t); rp != nil {
		if err := rp.IsValid(); err == nil {
			t.Errorf("Error: %v is an invalid RequiredProperty value, but it was not detected as invalid.\n", invalid_control_value3)
		}
	}

	invalid_control_value4 := `{"and":[{"name":"prop1"} ]}`
	if rp = create_RP(invalid_control_value4, t); rp != nil {
		if err := rp.IsValid(); err == nil {
			t.Errorf("Error: %v is an invalid RequiredProperty value, but it was not detected as invalid.\n", invalid_control_value4)
		}
	}

	invalid_control_value5 := `{"and":[{"value":"val1"} ]}`
	if rp = create_RP(invalid_control_value5, t); rp != nil {
		if err := rp.IsValid(); err == nil {
			t.Errorf("Error: %v is an invalid RequiredProperty value, but it was not detected as invalid.\n", invalid_control_value5)
		}
	}

	invalid_control_value6 := `{"and":[{"name":"prop1", "value":"val1", "op":"a"}]}`
	if rp = create_RP(invalid_control_value6, t); rp != nil {
		if err := rp.IsValid(); err == nil {
			t.Errorf("Error: %v is an invalid RequiredProperty value, but it was not detected as invalid.\n", invalid_control_value6)
		}
	}
}

// Test that simple expressions satisfy a single property value.
func Test_satisfy_simple1(t *testing.T) {
	var rp *RequiredProperty
	var pa *[]Property

	prop_list := `[{"name":"prop1", "value":true}]`
	simple_and := `{"and":[{"name":"prop1", "value":true}]}`

	if rp = create_RP(simple_and, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Error(err)
			}
		}
	}

}

// Test that simple expressions satisfy a single property value.
func Test_satisfy_simple2(t *testing.T) {
	var rp *RequiredProperty
	var pa *[]Property

	simple_and := `{}`
	prop_list := `[{"name":"prop1", "value":"val1"}]`

	if rp = create_RP(simple_and, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Error(err)
			}
		}
	}

	simple_and = `{"and":[{"name":"prop1", "value":"val1"}]}`

	if rp = create_RP(simple_and, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Error(err)
			}
		}
	}

	simple_and = `{"and":[{"name":"prop1", "value":"val2", "op":"!="}]}`
	if rp = create_RP(simple_and, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Error(err)
			}
		}
	}

	simple_or := `{"or":[{"name":"prop1", "value":"val1"}]}`
	if rp = create_RP(simple_or, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Error(err)
			}
		}
	}

	simple_or = `{"or":[{"name":"prop1", "value":9, "op":">"}]}`
	prop_list = `[{"name":"prop1", "value":10}]`
	if rp = create_RP(simple_or, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Error(err)
			}
		}
	}

	simple_or = `{"or":[{"name":"prop1", "value":11, "op":"<"}]}`
	if rp = create_RP(simple_or, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Error(err)
			}
		}
	}

	simple_or = `{"or":[{"name":"prop1", "value":11, "op":"<="}]}`
	if rp = create_RP(simple_or, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Error(err)
			}
		}
	}

	simple_or = `{"or":[{"name":"prop1", "value":10, "op":"<="}]}`
	if rp = create_RP(simple_or, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Error(err)
			}
		}
	}

	simple_or = `{"or":[{"name":"prop1", "value":10, "op":">="}]}`
	if rp = create_RP(simple_or, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Error(err)
			}
		}
	}

	simple_or = `{"or":[{"name":"prop1", "value":9, "op":">="}]}`
	if rp = create_RP(simple_or, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Error(err)
			}
		}
	}

	simple_or = `{"or":[{"name":"prop1", "value":10, "op":"=="}]}`
	if rp = create_RP(simple_or, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Error(err)
			}
		}
	}

	simple_or = `{"or":[{"name":"prop1", "value":5, "op":"!="}]}`
	if rp = create_RP(simple_or, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Error(err)
			}
		}
	}
}

// Test that multiple property requirement expressions can be satisfied.
func Test_satisfy_multiple1(t *testing.T) {
	var rp *RequiredProperty
	var pa *[]Property

	multiple_and := `{"and":[{"name":"prop1", "value":"val1"},{"name":"prop2", "value":"val2", "op":"=="}]}`
	prop_list := `[{"name":"prop1", "value":"val1"},{"name":"prop2", "value":"val2"},{"name":"prop3", "value":"val3"}]`

	if rp = create_RP(multiple_and, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Error(err)
			}
		}
	}

	multiple_or := `{"or":[{"name":"prop1", "value":"val1"},{"name":"prop2", "value":"val2"}]}`
	prop_list = `[{"name":"prop2", "value":"val2"}]`

	if rp = create_RP(multiple_or, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Error(err)
			}
		}
	}
}

// Test that simple expressions satisfy a single property value.
func Test_not_satisfy_simple1(t *testing.T) {
	var rp *RequiredProperty
	var pa *[]Property

	prop_list := `[{"name":"prop1", "value":true}]`
	simple_and := `{"and":[{"name":"prop1", "value":false}]}`

	if rp = create_RP(simple_and, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err == nil {
				t.Errorf("Error: %v should not satisfy %v, but it did.\n", prop_list, simple_and)
			}
		}
	}
}

// Test that simple expressions dont satisfy a single property value.
func Test_not_satisfy_simple2(t *testing.T) {
	var rp *RequiredProperty
	var pa *[]Property

	simple_and := `{"and":[{"name":"prop1", "value":"val1"}]}`
	prop_list := `[{"name":"prop2", "value":"val1"}]`

	if rp = create_RP(simple_and, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err == nil {
				t.Errorf("Error: %v should not satisfy %v, but it did.\n", prop_list, simple_and)
			}
		}
	}

	two_prop_and := `{"and":[{"name":"prop1", "value":"val1"},{"name":"prop2", "value":"val2"}]}`
	prop_list = `[{"name":"prop1", "value":"val1"}]`

	if rp = create_RP(two_prop_and, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err == nil {
				t.Errorf("Error: %v should not satisfy %v, but it did.\n", prop_list, two_prop_and)
			}
		}
	}

	simple_or := `{"or":[{"name":"prop1", "value":"val1"}]}`
	prop_list = `[{"name":"prop2", "value":"val1"}]`

	if rp = create_RP(simple_or, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err == nil {
				t.Errorf("Error: %v should not satisfy %v, but it did.\n", prop_list, simple_or)
			}
		}
	}

	two_prop_or := `{"or":[{"name":"prop1", "value":"val1"},{"name":"prop2", "value":"val2"}]}`
	prop_list = `[{"name":"prop1", "value":"val3"}]`

	if rp = create_RP(two_prop_or, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err == nil {
				t.Errorf("Error: %v should not satisfy %v, but it did.\n", prop_list, two_prop_or)
			}
		}
	}

	simple_or = `{"or":[{"name":"prop1", "value":"val1", "op":"!="}]}`
	prop_list = `[{"name":"prop1", "value":"val1"}]`

	if rp = create_RP(simple_or, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err == nil {
				t.Errorf("Error: %v should not satisfy %v, but it did.\n", prop_list, two_prop_or)
			}
		}
	}

	simple_or = `{"or":[{"name":"prop1", "value":"val1", "op":"="}]}`
	prop_list = `[{"name":"prop1", "value":"val2"}]`

	if rp = create_RP(simple_or, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err == nil {
				t.Errorf("Error: %v should not satisfy %v, but it did.\n", prop_list, two_prop_or)
			}
		}
	}

	simple_or = `{"or":[{"name":"prop1", "value":10, "op":">"}]}`
	prop_list = `[{"name":"prop1", "value":10}]`

	if rp = create_RP(simple_or, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err == nil {
				t.Errorf("Error: %v should not satisfy %v, but it did.\n", prop_list, two_prop_or)
			}
		}
	}

	simple_or = `{"or":[{"name":"prop1", "value":10, "op":"<"}]}`
	if rp = create_RP(simple_or, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err == nil {
				t.Errorf("Error: %v should not satisfy %v, but it did.\n", prop_list, two_prop_or)
			}
		}
	}

	simple_or = `{"or":[{"name":"prop1", "value":11, "op":">="}]}`
	if rp = create_RP(simple_or, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err == nil {
				t.Errorf("Error: %v should not satisfy %v, but it did.\n", prop_list, two_prop_or)
			}
		}
	}

	simple_or = `{"or":[{"name":"prop1", "value":9, "op":"<="}]}`
	if rp = create_RP(simple_or, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err == nil {
				t.Errorf("Error: %v should not satisfy %v, but it did.\n", prop_list, two_prop_or)
			}
		}
	}

	simple_or = `{"or":[{"name":"prop1", "value":9, "op":"="}]}`
	if rp = create_RP(simple_or, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err == nil {
				t.Errorf("Error: %v should not satisfy %v, but it did.\n", prop_list, two_prop_or)
			}
		}
	}

	simple_or = `{"or":[{"name":"prop1", "value":10, "op":"!="}]}`
	if rp = create_RP(simple_or, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err == nil {
				t.Errorf("Error: %v should not satisfy %v, but it did.\n", prop_list, two_prop_or)
			}
		}
	}
}

// Test that multiple property expressions arent satisfied.
func Test_not_satisfy_multiple1(t *testing.T) {
	var rp *RequiredProperty
	var pa *[]Property

	simple_and := `{"and":[{"name":"prop1", "value":"val1"},{"name":"prop2", "value":"val2"}]}`
	prop_list := `[{"name":"prop2", "value":"val1"}]`

	if rp = create_RP(simple_and, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err == nil {
				t.Errorf("Error: %v should not satisfy %v, but it did.\n", prop_list, simple_and)
			}
		}
	}

	two_prop_and := `{"and":[{"name":"prop1", "value":"val1"},{"name":"prop2", "value":"val2"}]}`
	prop_list = `[{"name":"prop1", "value":"val1"},{"name":"prop2", "value":"val1"},{"name":"prop3", "value":"val1"}]`

	if rp = create_RP(two_prop_and, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err == nil {
				t.Errorf("Error: %v should not satisfy %v, but it did.\n", prop_list, two_prop_and)
			}
		}
	}

	simple_or := `{"or":[{"name":"prop1", "value":"val1"},{"name":"prop2", "value":"val2"},{"name":"prop3", "value":"val3"}]}`
	prop_list = `[{"name":"prop1", "value":"val2"},{"name":"prop2", "value":"val1"}]`

	if rp = create_RP(simple_or, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err == nil {
				t.Errorf("Error: %v should not satisfy %v, but it did.\n", prop_list, simple_or)
			}
		}
	}

	two_prop_or := `{"or":[{"name":"prop1", "value":"val1"},{"name":"prop2", "value":"val2"},{"name":"prop3", "value":"val3"}]}`
	prop_list = `[{"name":"prop2", "value":"val1"},{"name":"prop3", "value":"val1"}]`

	if rp = create_RP(two_prop_or, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err == nil {
				t.Errorf("Error: %v should not satisfy %v, but it did.\n", prop_list, two_prop_or)
			}
		}
	}

}

// Tests that use complex expressions (with multiple control operators)
func Test_satisfy_complex1(t *testing.T) {
	var rp *RequiredProperty
	var pa *[]Property

	ex := `{"and":[{"name":"prop1", "value":"val1"},{"or":[{"name":"prop3", "value":"val3"},{"name":"prop4", "value":"val4"}]} ]}`
	prop_list := `[{"name":"prop1", "value":"val1"},{"name":"prop2", "value":"val2"},{"name":"prop3", "value":"val3"}]`

	if rp = create_RP(ex, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Error(err)
			}
		}
	}

	ex = `{"and":[{"and":[{"name":"prop1", "value":"val1"},{"name":"prop2", "value":"val2"}]} ,{"or":[{"name":"prop3", "value":"val3"},{"name":"prop4", "value":"val4"}]} ]}`
	if rp = create_RP(ex, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Error(err)
			}
		}
	}

	ex = `{"or":[{"and":[{"name":"prop1", "value":"val1"},{"name":"prop2", "value":"val2"}]} ,{"or":[{"name":"prop4", "value":"val4"},{"name":"prop5", "value":"val5"}]} ]}`
	if rp = create_RP(ex, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Error(err)
			}
		}
	}

	ex = `{"or":[{"and":[{"name":"prop4", "value":"val4"},{"name":"prop5", "value":"val5"}]} ,{"or":[{"name":"prop4", "value":"val4"},{"name":"prop3", "value":"val3"}]} ]}`
	if rp = create_RP(ex, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Error(err)
			}
		}
	}
}

// Tests that use complex expressions (with multiple control operators) that are not satisfied
func Test_not_satisfy_complex1(t *testing.T) {
	var rp *RequiredProperty
	var pa *[]Property

	ex := `{"and":[{"and":[{"name":"prop1", "value":"val1"},{"name":"prop2", "value":"val2"}]} ,{"or":[{"name":"prop3", "value":"val3"},{"name":"prop4", "value":"val4"}]} ]}`
	prop_list := `[{"name":"prop1", "value":"val1"},{"name":"prop3", "value":"val3"}]`

	if rp = create_RP(ex, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err == nil {
				t.Errorf("Error: %v should not satisfy %v, but it did.\n", prop_list, ex)
			}
		}
	}

	ex = `{"and":[{"and":[{"name":"prop1", "value":"val1"},{"name":"prop3", "value":"val3"}]} ,{"and":[{"name":"prop3", "value":"val3"},{"name":"prop4", "value":"val4"}]} ]}`

	if rp = create_RP(ex, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err == nil {
				t.Errorf("Error: %v should not satisfy %v, but it did.\n", prop_list, ex)
			}
		}
	}

	ex = `{"or":[{"and":[{"name":"prop1", "value":"val3"},{"name":"prop3", "value":"val3"}]} ,{"and":[{"name":"prop3", "value":"val3"},{"name":"prop4", "value":"val4"}]} ]}`

	if rp = create_RP(ex, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err == nil {
				t.Errorf("Error: %v should not satisfy %v, but it did.\n", prop_list, ex)
			}
		}
	}

	ex = `{"or":[{"or":[{"name":"prop1", "value":"val3"},{"name":"prop3", "value":"val1"}]} ,{"and":[{"name":"prop3", "value":"val3"},{"name":"prop4", "value":"val4"}]} ]}`

	if rp = create_RP(ex, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err == nil {
				t.Errorf("Error: %v should not satisfy %v, but it did.\n", prop_list, ex)
			}
		}
	}

	ex = `{"or":[{"or":[{"name":"prop1", "value":"val3"},{"name":"prop3", "value":"val1"}]} ,{"or":[{"name":"prop5", "value":"val5"},{"name":"prop4", "value":"val4"}]} ]}`

	if rp = create_RP(ex, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err == nil {
				t.Errorf("Error: %v should not satisfy %v, but it did.\n", prop_list, ex)
			}
		}
	}

	ex = `{"or":[{"and":[{"name":"prop1", "value":"val3"},{"name":"prop3", "value":"val3"}]} ,{"or":[{"name":"prop5", "value":"val5"},{"name":"prop4", "value":"val4"}]} ]}`

	if rp = create_RP(ex, t); rp != nil {
		if pa = create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err == nil {
				t.Errorf("Error: %v should not satisfy %v, but it did.\n", prop_list, ex)
			}
		}
	}
}

// This test verified that two Required Property expressions can be merged correctly.
func Test_merge1(t *testing.T) {

	var rp1 *RequiredProperty
	var rp2 *RequiredProperty

	simple1 := `{"and":[{"name":"prop1", "value":"val1"},{"name":"prop2", "value":"val2"}]}`
	simple2 := `{"and":[{"name":"prop3", "value":"val3"},{"name":"prop4", "value":"val4"}]}`
	if rp1 = create_RP(simple1, t); rp1 != nil {
		if rp2 = create_RP(simple2, t); rp2 != nil {
			if rp3 := rp1.Merge(rp2); rp3 == nil {
				t.Errorf("Error: Merged RequiredProperty expression not returned.\n")
			} else {
				var pa *[]Property
				prop_list := `[{"name":"prop1", "value":"val1"},{"name":"prop2", "value":"val2"},{"name":"prop3", "value":"val3"},{"name":"prop4", "value":"val4"}]`

				if pa = create_property_list(prop_list, t); pa != nil {
					if err := rp3.IsSatisfiedBy(*pa); err != nil {
						t.Error(err)
					}
				}
			}
		}
	}
}

func Test_merge2(t *testing.T) {

	var rp1 *RequiredProperty
	var rp2 *RequiredProperty

	simple1 := `{}`
	simple2 := `{}`
	if rp1 = create_RP(simple1, t); rp1 != nil {
		if rp2 = create_RP(simple2, t); rp2 != nil {
			if rp3 := rp1.Merge(rp2); rp3 == nil {
				t.Errorf("Error: Merged RequiredProperty expression not returned.\n")
			} else if len(*rp3) != 0 {
				t.Errorf("Error: Merged RequiredProperty should be empty, is %v.\n", *rp3)
			}
		}
	}
}

func Test_merge3(t *testing.T) {

	var rp1 *RequiredProperty
	var rp2 *RequiredProperty

	simple1 := `{"and":[{"name":"prop1", "value":"val1"},{"name":"prop2", "value":"val2"}]}`
	simple2 := `{}`
	if rp1 = create_RP(simple1, t); rp1 != nil {
		if rp2 = create_RP(simple2, t); rp2 != nil {
			if rp3 := rp1.Merge(rp2); rp3 == nil {
				t.Errorf("Error: Merged RequiredProperty expression not returned.\n")
			} else if len(*rp3) != 1 {
				t.Errorf("Error: Merged RequiredProperty should have 1 element, but it has %v.\n", len(*rp3))
			} else {
				var pa *[]Property
				prop_list := `[{"name":"prop1", "value":"val1"},{"name":"prop2", "value":"val2"}]`

				if pa = create_property_list(prop_list, t); pa != nil {
					if err := rp3.IsSatisfiedBy(*pa); err != nil {
						t.Error(err)
					}
				}
			}
		}
	}
}

func Test_merge4(t *testing.T) {

	var rp1 *RequiredProperty
	var rp2 *RequiredProperty

	simple1 := `{}`
	simple2 := `{"and":[{"name":"prop1", "value":"val1"},{"name":"prop2", "value":"val2"}]}`
	if rp1 = create_RP(simple1, t); rp1 != nil {
		if rp2 = create_RP(simple2, t); rp2 != nil {
			if rp3 := rp1.Merge(rp2); rp3 == nil {
				t.Errorf("Error: Merged RequiredProperty expression not returned.\n")
			} else if len(*rp3) != 1 {
				t.Errorf("Error: Merged RequiredProperty should have 1 element, but it has %v.\n", len(*rp3))
			} else {
				var pa *[]Property
				prop_list := `[{"name":"prop1", "value":"val1"},{"name":"prop2", "value":"val2"}]`

				if pa = create_property_list(prop_list, t); pa != nil {
					if err := rp3.IsSatisfiedBy(*pa); err != nil {
						t.Error(err)
					}
				}
			}
		}
	}
}

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

func Test_complex_IsSatisfiedBy(t *testing.T) {
	rp_list := `{"or":[{"name":"prop1", "value":"val1, val2, val3", "op":"in"}]}`
	prop_list := `[{"name":"prop1", "value":"val2"}]`

	if rp := create_RP(rp_list, t); rp != nil {
		if pa := create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Errorf("Error: %v should satisfy %v, but it did not: %v.\n", prop_list, rp_list, err)
			}
		}
	}

	rp_list = `{"or":[{"name":"prop1", "value":"val1, \"val2\", val3", "op":"in"}]}`
	prop_list = `[{"name":"prop1", "value":"val2"}]`

	if rp := create_RP(rp_list, t); rp != nil {
		if pa := create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Errorf("Error: %v should satisfy %v, but it did not: %v.\n", prop_list, rp_list, err)
			}
		}
	}

	rp_list = `{"or":[{"name":"prop1", "value":"(1.0.1,INFINITY]", "op":"in"}]}`
	prop_list = `[{"name":"prop1", "value":"1.4.5", "type":"version"}]`

	if rp := create_RP(rp_list, t); rp != nil {
		if pa := create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Errorf("Error: %v should satisfy %v, but it did not: %v.\n", prop_list, rp_list, err)
			}
		}
	}

	rp_list = `{"or":[{"name":"prop1", "value":"\"a bc def\""}]}`
	prop_list = `[{"name":"prop1", "value":"a bc def"}]`

	if rp := create_RP(rp_list, t); rp != nil {
		if pa := create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Errorf("Error: %v should satisfy %v, but it did not: %v.\n", prop_list, rp_list, err)
			}
		}
	}

	rp_list = `{"or":[{"name":"prop1", "value":"abc"}]}`
	prop_list = `[{"name":"prop1", "value":"abc,def,ghi", "type":"list of string"}]`

	if rp := create_RP(rp_list, t); rp != nil {
		if pa := create_property_list(prop_list, t); pa != nil {
			if err := rp.IsSatisfiedBy(*pa); err != nil {
				t.Errorf("Error: %v should satisfy %v, but it did not: %v.\n", prop_list, rp_list, err)
			}
		}
	}
}

// ================================================================================================================
// Helper functions used by all tests
//
// Create a RequiredProperty object from a JSON serialization. The JSON serialization
// does not have to be a valid RequiredProperty serialization, just has to be a valid
// JSON serialization.
func create_RP(jsonString string, t *testing.T) *RequiredProperty {
	rp := new(RequiredProperty)

	if err := json.Unmarshal([]byte(jsonString), &rp); err != nil {
		t.Errorf("Error unmarshalling RequiredProperty json string: %v error:%v\n", jsonString, err)
		return nil
	} else {
		return rp
	}
}

// Create an array of Property objects from a JSON serialization. The JSON serialization
// does not have to be a valid Property serialization, just has to be a valid
// JSON serialization.
func create_property_list(jsonString string, t *testing.T) *[]Property {
	pa := make([]Property, 0, 10)

	if err := json.Unmarshal([]byte(jsonString), &pa); err != nil {
		t.Errorf("Error unmarshalling Property json string: %v error:%v\n", jsonString, err)
		return nil
	} else {
		return &pa
	}
}
