package policy

import (
	"encoding/json"
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

	simple_or = `{"or":[{"name":"prop1", "value":10, "op":"="}]}`
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

	multiple_and := `{"and":[{"name":"prop1", "value":"val1"},{"name":"prop2", "value":"val2", "op":"="}]}`
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
