// +build unit

package exchangecommon

import (
	"github.com/open-horizon/anax/externalpolicy"
	"reflect"
	"testing"
)

func Test_CompareWith(t *testing.T) {

	// compare with nil
	pol1 := &NodePolicy{}
	rc_d, rc_m := pol1.CompareWith(nil)
	if rc_d != externalpolicy.EP_COMPARE_DELETED || rc_m != externalpolicy.EP_COMPARE_DELETED {
		t.Errorf("CompareWith should have returned (%v, %v), but got (%v, %v).", externalpolicy.EP_COMPARE_DELETED, externalpolicy.EP_COMPARE_DELETED, rc_d, rc_m)
	}

	// compare 2 empty policies
	pol1 = &NodePolicy{}
	pol2 := &NodePolicy{}
	rc_d, rc_m = pol1.CompareWith(pol2)
	if rc_d != externalpolicy.EP_COMPARE_NOCHANGE || rc_m != externalpolicy.EP_COMPARE_NOCHANGE {
		t.Errorf("CompareWith should have returned (%v, %v), but got (%v, %v).", externalpolicy.EP_COMPARE_NOCHANGE, externalpolicy.EP_COMPARE_NOCHANGE, rc_d, rc_m)
	}

	// compare with a policy that has the same property and constraints.
	propList1 := new(externalpolicy.PropertyList)
	propList1.Add_Property(externalpolicy.Property_Factory("prop1", "val1"), false)
	propList1.Add_Property(externalpolicy.Property_Factory("prop2", "val2"), false)
	propList1d := new(externalpolicy.PropertyList)
	propList1d.Add_Property(externalpolicy.Property_Factory("prop3", "val3"), false)
	propList1d.Add_Property(externalpolicy.Property_Factory("prop4", "val4"), false)
	propList1m := new(externalpolicy.PropertyList)
	propList1m.Add_Property(externalpolicy.Property_Factory("prop5", "val5"), false)
	propList1m.Add_Property(externalpolicy.Property_Factory("prop6", "val6"), false)
	constr1d := externalpolicy.ConstraintExpression{"version in [1.1.1,INFINITY) OR cert == USDA", `prop3 == "some value"`}
	constr1m := externalpolicy.ConstraintExpression{"version in [1.1.1,INFINITY) OR cert == USDA", `prop4 == "val4"`}

	propList2 := new(externalpolicy.PropertyList)
	propList2.Add_Property(externalpolicy.Property_Factory("prop2", "val2"), false)
	propList2.Add_Property(externalpolicy.Property_Factory("prop1", "val1"), false)
	propList2d := new(externalpolicy.PropertyList)
	propList2d.Add_Property(externalpolicy.Property_Factory("prop4", "val4"), false)
	propList2d.Add_Property(externalpolicy.Property_Factory("prop3", "val3"), false)
	propList2m := new(externalpolicy.PropertyList)
	propList2m.Add_Property(externalpolicy.Property_Factory("prop6", "val6"), false)
	propList2m.Add_Property(externalpolicy.Property_Factory("prop5", "val5"), false)
	constr2d := externalpolicy.ConstraintExpression{`prop3 == "some value"`, "version in [1.1.1,INFINITY) OR cert == USDA"}
	constr2m := externalpolicy.ConstraintExpression{"version in [1.1.1,INFINITY) OR cert == USDA", `prop4 == "val4"`}

	pol1 = &NodePolicy{
		ExternalPolicy: externalpolicy.ExternalPolicy{Properties: *propList1, Constraints: externalpolicy.ConstraintExpression{}},
		Deployment:     externalpolicy.ExternalPolicy{Properties: *propList1d, Constraints: constr1d},
		Management:     externalpolicy.ExternalPolicy{Properties: *propList1m, Constraints: constr1m},
	}
	pol2 = &NodePolicy{
		ExternalPolicy: externalpolicy.ExternalPolicy{Properties: *propList2, Constraints: externalpolicy.ConstraintExpression{}},
		Deployment:     externalpolicy.ExternalPolicy{Properties: *propList2d, Constraints: constr2d},
		Management:     externalpolicy.ExternalPolicy{Properties: *propList2m, Constraints: constr2m},
	}

	rc_d, rc_m = pol1.CompareWith(pol2)
	if rc_d != externalpolicy.EP_COMPARE_NOCHANGE || rc_m != externalpolicy.EP_COMPARE_NOCHANGE {
		t.Errorf("CompareWith should have returned (%v, %v), but got (%v, %v).", externalpolicy.EP_COMPARE_NOCHANGE, externalpolicy.EP_COMPARE_NOCHANGE, rc_d, rc_m)
	}

	// the top level constraints does not take effect if constraints are defined
	// under deployment and management.
	pol1.Constraints = externalpolicy.ConstraintExpression{"prop7 == \"some value\""}
	pol2.Constraints = externalpolicy.ConstraintExpression{"prop6 == \"some value\""}
	rc_d, rc_m = pol1.CompareWith(pol2)
	if rc_d != externalpolicy.EP_COMPARE_NOCHANGE || rc_m != externalpolicy.EP_COMPARE_NOCHANGE {
		t.Errorf("CompareWith should have returned (%v, %v), but got (%v, %v).", externalpolicy.EP_COMPARE_NOCHANGE, externalpolicy.EP_COMPARE_NOCHANGE, rc_d, rc_m)
	}

	// comapre with a policy that has different properties
	pol1.Properties.Add_Property(externalpolicy.Property_Factory("prop7", "val7"), false)
	rc_d, rc_m = pol1.CompareWith(pol2)
	if rc_d != externalpolicy.EP_COMPARE_PROPERTY_CHANGED || rc_m != externalpolicy.EP_COMPARE_PROPERTY_CHANGED {
		t.Errorf("CompareWith should have returned (%v, %v), but got (%v, %v).", externalpolicy.EP_COMPARE_PROPERTY_CHANGED, externalpolicy.EP_COMPARE_PROPERTY_CHANGED, rc_d, rc_m)
	}

	// compare with a policy that has different properties and constrains
	pol1.Deployment.Constraints = externalpolicy.ConstraintExpression{"prop7 == \"some value\"",
		"version in [1.1.1,INFINITY) OR cert == USDA_PART1"}
	pol2.Deployment.Constraints = externalpolicy.ConstraintExpression{"prop6 == \"some value\"",
		"version in [1.1.1,INFINITY) OR cert == USDA_PART1"}
	rc_d, rc_m = pol1.CompareWith(pol2)
	if rc_d != externalpolicy.EP_COMPARE_PROPERTY_CHANGED|externalpolicy.EP_COMPARE_CONSTRAINT_CHANGED || rc_m != externalpolicy.EP_COMPARE_PROPERTY_CHANGED {
		t.Errorf("CompareWith should have returned (%v, %v), but got (%v, %v).", externalpolicy.EP_COMPARE_PROPERTY_CHANGED|externalpolicy.EP_COMPARE_CONSTRAINT_CHANGED, externalpolicy.EP_COMPARE_PROPERTY_CHANGED, rc_d, rc_m)
	}
}

func Test_GetPolicies(t *testing.T) {
	// old style poilicy
	propList1 := new(externalpolicy.PropertyList)
	propList1.Add_Property(externalpolicy.Property_Factory("prop1", "val1"), false)
	propList1.Add_Property(externalpolicy.Property_Factory("prop2", "val2"), false)
	constr1 := externalpolicy.ConstraintExpression{"a==b", "c==d"}

	pol := &NodePolicy{
		ExternalPolicy: externalpolicy.ExternalPolicy{Properties: *propList1, Constraints: constr1},
	}

	extPol := pol.GetDeploymentPolicy()
	if !reflect.DeepEqual(*propList1, extPol.Properties) {
		t.Errorf("GetDeploymentPolicy does not get the correct properties.")
	} else if !reflect.DeepEqual(constr1, extPol.Constraints) {
		t.Errorf("GetDeploymentPolicy does not get the correct constraints.")
	}

	extPol = pol.GetManagementPolicy()
	if !reflect.DeepEqual(*propList1, extPol.Properties) {
		t.Errorf("GetManagementPolicy does not get the correct properties.")
	} else if !reflect.DeepEqual(constr1, extPol.Constraints) {
		t.Errorf("GetManagementPolicy does not get the correct constraints.")
	}

	// new style policy
	propList1d := new(externalpolicy.PropertyList)
	propList1d.Add_Property(externalpolicy.Property_Factory("prop3", "val3"), false)
	propList1d.Add_Property(externalpolicy.Property_Factory("prop4", "val4"), false)
	propList1m := new(externalpolicy.PropertyList)
	propList1m.Add_Property(externalpolicy.Property_Factory("prop5", "val5"), false)
	propList1m.Add_Property(externalpolicy.Property_Factory("prop6", "val6"), false)
	propList1m.Add_Property(externalpolicy.Property_Factory("prop7", "val7"), false)
	constr1d := externalpolicy.ConstraintExpression{"version in [1.1.1,INFINITY) OR cert == USDA", `prop3 == "some value"`}
	constr1m := externalpolicy.ConstraintExpression{"version in [1.1.1,INFINITY) OR cert == USDA", `prop4 == "val4"`}
	pol = &NodePolicy{
		ExternalPolicy: externalpolicy.ExternalPolicy{Properties: *propList1, Constraints: constr1},
		Deployment:     externalpolicy.ExternalPolicy{Properties: *propList1d, Constraints: constr1d},
		Management:     externalpolicy.ExternalPolicy{Properties: *propList1m, Constraints: constr1m},
	}

	extPol = pol.GetDeploymentPolicy()
	if len(extPol.Properties) != len(*propList1)+len(*propList1d) {
		t.Errorf("GetDeploymentPolicy returns wrong number of properties.")
	} else if !reflect.DeepEqual(constr1d, extPol.Constraints) {
		t.Errorf("GetDeploymentPolicy does not get the correct constraints.")
	}

	extPol = pol.GetManagementPolicy()
	if len(extPol.Properties) != len(*propList1)+len(*propList1m) {
		t.Errorf("GetManagementPolicy returns wrong number of properties.")
	} else if !reflect.DeepEqual(constr1m, extPol.Constraints) {
		t.Errorf("GetManagementPolicy does not get the correct constraints.")
	}
}

func Test_DeepCopy(t *testing.T) {

	propList1 := new(externalpolicy.PropertyList)
	propList1.Add_Property(externalpolicy.Property_Factory("prop1", "val1"), false)
	propList1.Add_Property(externalpolicy.Property_Factory("prop2", "val2"), false)
	propList1d := new(externalpolicy.PropertyList)
	propList1d.Add_Property(externalpolicy.Property_Factory("prop3", "val3"), false)
	propList1d.Add_Property(externalpolicy.Property_Factory("prop4", "val4"), false)
	propList1m := new(externalpolicy.PropertyList)
	propList1m.Add_Property(externalpolicy.Property_Factory("prop5", "val5"), false)
	propList1m.Add_Property(externalpolicy.Property_Factory("prop6", "val6"), false)

	constr1 := externalpolicy.ConstraintExpression{"a==b", "c==d"}
	constr1d := externalpolicy.ConstraintExpression{"version in [1.1.1,INFINITY) OR cert == USDA", `prop3 == "some value"`}
	constr1m := externalpolicy.ConstraintExpression{"version in [1.1.1,INFINITY) OR cert == USDA", `prop4 == "val4"`}

	pol1 := &NodePolicy{
		ExternalPolicy: externalpolicy.ExternalPolicy{Properties: *propList1, Constraints: constr1},
		Deployment:     externalpolicy.ExternalPolicy{Properties: *propList1d, Constraints: constr1d},
		Management:     externalpolicy.ExternalPolicy{Properties: *propList1m, Constraints: constr1m},
	}

	pol2 := pol1.DeepCopy()

	if !reflect.DeepEqual(pol1, pol2) {
		t.Errorf("policy2 do not equal to policy 1")
	} else if !reflect.DeepEqual(*propList1, pol2.Properties) {
		t.Errorf("Properties in policy2 do not equal to properties in policy 1")
	} else if !reflect.DeepEqual(*propList1d, pol2.Deployment.Properties) {
		t.Errorf("Deployment properties in policy2 does not equal to deployment properties in policy 1")
	} else if !reflect.DeepEqual(*propList1m, pol2.Management.Properties) {
		t.Errorf("Management properties in policy2 does not equal to management properties in policy 1")
	} else if !reflect.DeepEqual(constr1, pol2.Constraints) {
		t.Errorf("Constraints in policy2 does not equal to constraints in policy 1")
	} else if !reflect.DeepEqual(constr1d, pol2.Deployment.Constraints) {
		t.Errorf("Deployment constraints in policy2 does not equal to deployment constraints in policy 1")
	} else if !reflect.DeepEqual(constr1m, pol2.Management.Constraints) {
		t.Errorf("Management constraints in policy2 does not equal to management constraints in policy 1")
	}
}

func Test_ConvertNodePolicy_v1Tov2(t *testing.T) {

	// old style poilicy with built-in node properties
	propList1 := new(externalpolicy.PropertyList)
	propList1.Add_Property(externalpolicy.Property_Factory("prop1", "val1"), false)
	propList1.Add_Property(externalpolicy.Property_Factory("prop2", "val2"), false)
	propList1.Add_Property(externalpolicy.Property_Factory("openhorizon.hardwareId", "blah"), false)
	propList1.Add_Property(externalpolicy.Property_Factory("openhorizon.cpu", 2), false)
	propList1.Add_Property(externalpolicy.Property_Factory("openhorizon.arch", "amd64"), false)
	propList1.Add_Property(externalpolicy.Property_Factory("openhorizon.memory", 11234), false)
	propList1.Add_Property(externalpolicy.Property_Factory("openhorizon.allowPrivileged", false), false)
	constr1 := externalpolicy.ConstraintExpression{"a==b", "c==d"}
	pol := externalpolicy.ExternalPolicy{Properties: *propList1, Constraints: constr1}

	// convert to new format
	newPol := ConvertNodePolicy_v1Tov2(pol)

	newPropList1 := new(externalpolicy.PropertyList)
	newPropList1.Add_Property(externalpolicy.Property_Factory("openhorizon.hardwareId", "blah"), false)
	newPropList1.Add_Property(externalpolicy.Property_Factory("openhorizon.cpu", 2), false)
	newPropList1.Add_Property(externalpolicy.Property_Factory("openhorizon.arch", "amd64"), false)
	newPropList1.Add_Property(externalpolicy.Property_Factory("openhorizon.memory", 11234), false)
	newPropList1.Add_Property(externalpolicy.Property_Factory("openhorizon.allowPrivileged", false), false)
	newPropList1d := new(externalpolicy.PropertyList)
	newPropList1d.Add_Property(externalpolicy.Property_Factory("prop1", "val1"), false)
	newPropList1d.Add_Property(externalpolicy.Property_Factory("prop2", "val2"), false)

	if !newPol.Properties.IsSame(*newPropList1) {
		t.Errorf("new policy properties are not correct.")
	} else if len(newPol.Constraints) != 0 {
		t.Errorf("new policy constraints are not correct.")
	} else if !newPol.Deployment.Properties.IsSame(*newPropList1d) {
		t.Errorf("new policy deployment properties are not correct.")
	} else if !newPol.Deployment.Constraints.IsSame(constr1) {
		t.Errorf("new policy deployment constraintes are not correct.")
	} else if newPol.Management.Properties != nil {
		t.Errorf("new policy management properties are not correct.")
	} else if newPol.Management.Constraints != nil {
		t.Errorf("new policy management constraints are not correct.")
	}
}
