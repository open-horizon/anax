// +build unit

package externalpolicy

import (
	"github.com/open-horizon/anax/policy"
	"reflect"
	"testing"
)

func Test_GenPolicyFromExternalPolicy(t *testing.T) {
	propList := new(policy.PropertyList)
	propList.Add_Property(policy.Property_Factory("prop1", "val1"))
	propList.Add_Property(policy.Property_Factory("prop2", "val2"))

	extNodePolicy := &ExternalPolicy{
		Properties:  *propList,
		Constraints: []string{"prop3 == val3"},
	}

	if pol, err := extNodePolicy.GenPolicyFromExternalPolicy("mydevice"); err != nil {
		t.Errorf("GenPolicyFromExternalPolicy should not have returned error but got: %v", err)
	} else {
		if pol.Header.Name != "Policy for mydevice" {
			t.Errorf("Wrong policy name generated: %v", pol.Header.Name)
		}
		if !reflect.DeepEqual(pol.Properties, *propList) {
			t.Errorf("Error converting external properties %v to policy properties: %v", propList, pol.Properties)
		}

		// check counterparty property
		// this part is tested heavily in constaint_expression_test.go
		propList3 := new(policy.PropertyList)
		propList3.Add_Property(policy.Property_Factory("prop3", "val3"))

		if err := pol.CounterPartyProperties.IsSatisfiedBy(*propList3); err != nil {
			t.Errorf("Couterparty property check should not have returned error but got: %v", err)
		}
	}
}
