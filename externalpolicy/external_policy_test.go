//go:build unit
// +build unit

package externalpolicy

import (
	_ "github.com/open-horizon/anax/externalpolicy/text_language"
	"reflect"
	"testing"
)

func ExternalPolicy_Test_MergeWith(t *testing.T) {
	propList1 := new(PropertyList)
	propList1.Add_Property(Property_Factory("prop1", "val1"), false)

	propList2 := new(PropertyList)
	propList2.Add_Property(Property_Factory("prop1", "val1"), false)

	pol1 := &ExternalPolicy{
		Properties:  *propList1,
		Constraints: []string{`prop3 == "some value"`},
	}

	pol2 := &ExternalPolicy{
		Properties:  *propList2,
		Constraints: []string{`prop3 == "some value"`},
	}

	pol1.MergeWith(pol2, false)
	if len(pol1.Properties) != 1 {
		t.Errorf("Error: Properties %v should have 1 element but got %v", pol1.Properties, len(pol1.Properties))
	}
	if len(pol1.Constraints) != 1 {
		t.Errorf("Error: Properties %v should have 1 element but got %v", pol1.Constraints, len(pol1.Constraints))
	}

	propList1 = new(PropertyList)
	propList1.Add_Property(Property_Factory("prop1", "val1"), false)
	propList1.Add_Property(Property_Factory("prop2", "val2"), false)

	propList2 = new(PropertyList)
	propList2.Add_Property(Property_Factory("prop1", "val1"), false)
	propList2.Add_Property(Property_Factory("prop3", "val3"), false)
	propList2.Add_Property(Property_Factory("prop4", "val4"), false)

	pol1 = &ExternalPolicy{
		Properties: *propList1,
		Constraints: []string{"version == 1.1.1 OR USDA in \"United,States,Department,of,Agriculture\"",
			"version in [1.1.1,INFINITY) OR cert == USDA",
			"color == \"orange\""},
	}

	pol2 = &ExternalPolicy{
		Properties: *propList2,
		Constraints: []string{"prop3 == \"some value\"",
			"version == 1.1.1 OR USDA in \"United,States,Department,of,Agriculture\"",
			"version in [1.1.1,INFINITY) OR cert == USDA_PART1"},
	}

	pol1.MergeWith(pol2, false)
	if len(pol1.Properties) != 4 {
		t.Errorf("Error: Properties %v should have 4 elements but got %v", pol1.Properties, len(pol1.Properties))
	}
	if len(pol1.Constraints) != 5 {
		t.Errorf("Error: Properties %v should have 5 elements but got %v", pol1.Constraints, len(pol1.Constraints))
	}
}

func ExternalPolicy_Test_CompareWith(t *testing.T) {

	// compare with nil
	pol1 := &ExternalPolicy{}
	rc := pol1.CompareWith(nil)
	if rc != EP_COMPARE_DELETED {
		t.Errorf("CompareWith should have returned %v, but got %v.", EP_COMPARE_DELETED, rc)
	}

	// compare 2 empty policies
	pol1 = &ExternalPolicy{}
	pol2 := &ExternalPolicy{}
	rc = pol1.CompareWith(pol2)
	if rc != EP_COMPARE_NOCHANGE {
		t.Errorf("CompareWith should have returned %v, but got %v.", EP_COMPARE_NOCHANGE, rc)
	}

	// compare with a policy that has the same property and constraints.
	propList1 := new(PropertyList)
	propList1.Add_Property(Property_Factory("prop1", "val1"), false)
	propList1.Add_Property(Property_Factory("prop2", "val2"), false)
	pol1 = &ExternalPolicy{
		Properties:  *propList1,
		Constraints: []string{"version in [1.1.1,INFINITY) OR cert == USDA", `prop3 == "some value"`},
	}
	propList2 := new(PropertyList)
	propList2.Add_Property(Property_Factory("prop2", "val2"), false)
	propList2.Add_Property(Property_Factory("prop1", "val1"), false)
	pol2 = &ExternalPolicy{
		Properties:  *propList2,
		Constraints: []string{`prop3 == "some value"`, "version in [1.1.1,INFINITY) OR cert == USDA"},
	}

	rc = pol1.CompareWith(pol2)
	if rc != EP_COMPARE_NOCHANGE {
		t.Errorf("CompareWith should have returned %v, but got %v.", EP_COMPARE_NOCHANGE, rc)
	}

	// comapre with a policy that has different properties
	propList1 = new(PropertyList)
	propList1.Add_Property(Property_Factory("prop1", "val1"), false)
	propList1.Add_Property(Property_Factory("prop2", "val2"), false)

	propList2 = new(PropertyList)
	propList2.Add_Property(Property_Factory("prop3", "val3"), false)
	propList2.Add_Property(Property_Factory("prop4", "val4"), false)

	pol1 = &ExternalPolicy{
		Properties: *propList1,
		Constraints: []string{"version == 1.1.1 OR USDA in \"United,States,Department,of,Agriculture\"",
			"version in [1.1.1,INFINITY) OR cert == USDA",
			"color == \"orange\""},
	}

	pol2 = &ExternalPolicy{
		Properties: *propList2,
		Constraints: []string{"prop3 == \"some value\"",
			"version == 1.1.1 OR USDA in \"United,States,Department,of,Agriculture\"",
			"version in [1.1.1,INFINITY) OR cert == USDA_PART1"},
	}
	rc = pol1.CompareWith(pol2)
	if rc != EP_COMPARE_PROPERTY_CHANGED {
		t.Errorf("CompareWith should have returned %v, but got %v.", EP_COMPARE_PROPERTY_CHANGED, rc)
	}

	// compare with a policy that has different properties and constrains
	pol2 = &ExternalPolicy{
		Properties: *propList2,
		Constraints: []string{"prop3 == \"some value\"",
			"version in [1.1.1,INFINITY) OR cert == USDA_PART1"},
	}
	rc = pol1.CompareWith(pol2)
	if rc != EP_COMPARE_PROPERTY_CHANGED|EP_COMPARE_CONSTRAINT_CHANGED {
		t.Errorf("CompareWith should have returned %v, but got %v.", EP_COMPARE_PROPERTY_CHANGED|EP_COMPARE_CONSTRAINT_CHANGED, rc)
	}
}

func ExternalPolicy_Test_CopyProperties(t *testing.T) {

	propList1 := new(PropertyList)
	propList1.Add_Property(Property_Factory("prop1", "val1"), false)
	propList1.Add_Property(Property_Factory("prop2", "val2"), false)
	propList1.Add_Property(Property_Factory("prop3", false), false)
	propList1.Add_Property(Property_Factory("prop4", 1), false)

	propList2 := CopyProperties(*propList1)
	if !reflect.DeepEqual(propList1, &propList2) {
		t.Errorf("CopyProperties does not create same properties.")
	}
}

func ExternalPolicy_Test_CopyConstraints(t *testing.T) {

	c := []string{"version in [1.1.1,INFINITY) OR cert == USDA", `prop3 == "some value"`}
	cCopy := CopyConstraints(c)
	if !reflect.DeepEqual(c, cCopy) {
		t.Errorf("CopyConstraints does not create same constraints.")
	}
}

func ExternalPolicy_Test_DeepCopy(t *testing.T) {

	propList1 := new(PropertyList)
	propList1.Add_Property(Property_Factory("prop1", "val1"), false)
	propList1.Add_Property(Property_Factory("prop2", "val2"), false)
	propList1.Add_Property(Property_Factory("prop3", false), false)
	propList1.Add_Property(Property_Factory("prop4", 1), false)
	constr := []string{"version in [1.1.1,INFINITY) OR cert == USDA", `prop3 == "some value"`}

	extPol1 := &ExternalPolicy{Properties: *propList1, Constraints: constr}
	extPol2 := extPol1.DeepCopy()

	if !reflect.DeepEqual(extPol1.Properties, extPol2.Properties) {
		t.Errorf("DeepCopy does not create same properties.")
	} else if !reflect.DeepEqual(extPol1.Constraints, extPol2.Constraints) {
		t.Errorf("DeepCopy does not create same constraints.")
	}
}
