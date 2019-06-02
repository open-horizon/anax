// +build unit

package externalpolicy

import (
	_ "github.com/open-horizon/anax/externalpolicy/text_language"
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
		t.Errorf("Error: Properties %v should have 4 element but got %v", pol1.Properties, len(pol1.Properties))
	}
	if len(pol1.Constraints) != 5 {
		t.Errorf("Error: Properties %v should have 5 element but got %v", pol1.Constraints, len(pol1.Constraints))
	}
}
