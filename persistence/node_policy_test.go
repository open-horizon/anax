// +build unit

package persistence

import (
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/externalpolicy"
	"testing"
)

// Verify that FindNodePolicy works when there is no node policy in the database.
func Test_ReadNodePolicy(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	if np, err := FindNodePolicy(db); err != nil {
		t.Errorf("failed to find node policy in db, error %v", err)
	} else if np != nil {
		t.Errorf("incorrect result, there should not be a node policy: %v", *np)
	}

}

// Verify that a Node Policy Object can be created and saved.
func Test_WriteNodePolicy1(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	propName := "prop1"
	propList := new(externalpolicy.PropertyList)
	propList.Add_Property(externalpolicy.Property_Factory(propName, "val1"), false)
	propName2 := "prop2"
	propList_Deploy := new(externalpolicy.PropertyList)
	propList_Deploy.Add_Property(externalpolicy.Property_Factory(propName2, "val2"), false)
	propName3 := "prop3"
	propList_Manage := new(externalpolicy.PropertyList)
	propList_Manage.Add_Property(externalpolicy.Property_Factory(propName3, "val3"), false)

	extPolicy := externalpolicy.ExternalPolicy{
		Properties:  *propList,
		Constraints: []string{},
	}
	extPolicy_Deploy := externalpolicy.ExternalPolicy{
		Properties:  *propList_Deploy,
		Constraints: []string{`prop3 == "some value"`},
	}
	extPolicy_Manage := externalpolicy.ExternalPolicy{
		Properties:  *propList_Manage,
		Constraints: []string{`prop4 == "some other value"`},
	}
	nodePolicy := exchangecommon.NodePolicy{ExternalPolicy: extPolicy, Deployment: extPolicy_Deploy, Management: extPolicy_Manage}

	err = SaveNodePolicy(db, &nodePolicy)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if fnp, err := FindNodePolicy(db); err != nil {
		t.Errorf("failed to find node policy in db, error %v", err)
	} else if len(fnp.Properties) != 1 {
		t.Errorf("incorrect node policy, there should be 1 property defined, found: %v", len(fnp.Properties))
	} else if fnp.Properties[0].Name != propName {
		t.Errorf("expected property %v, but received %v", propName, fnp.Properties[0].Name)
	} else if len(fnp.Constraints) != 0 {
		t.Errorf("incorrect node policy, there should be 0 constraint defined, found: %v", len(fnp.Constraints))
	} else if len(fnp.Deployment.Properties) != 1 {
		t.Errorf("incorrect deployment node policy, there should be 1 property defined, found: %v", len(fnp.Deployment.Properties))
	} else if fnp.Deployment.Properties[0].Name != propName2 {
		t.Errorf("expected deployment property %v, but received %v", propName2, fnp.Deployment.Properties[0].Name)
	} else if len(fnp.Deployment.Constraints) != 1 {
		t.Errorf("incorrect deployment node policy, there should be 1 constraint defined, found: %v", len(fnp.Deployment.Constraints))
	} else if len(fnp.Management.Properties) != 1 {
		t.Errorf("incorrect management node policy, there should be 1 property defined, found: %v", len(fnp.Management.Properties))
	} else if fnp.Management.Properties[0].Name != propName3 {
		t.Errorf("expected management property %v, but received %v", propName3, fnp.Management.Properties[0].Name)
	} else if len(fnp.Management.Constraints) != 1 {
		t.Errorf("incorrect management node policy, there should be 1 constraint defined, found: %v", len(fnp.Management.Constraints))
	}
}

// Verify that a Node Policy Object can be created and saved, and then updated.
func Test_UpdateNodePolicy1(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	propName := "prop1"
	propList := new(externalpolicy.PropertyList)
	propList.Add_Property(externalpolicy.Property_Factory(propName, "val1"), false)
	propName2 := "prop2"
	propList_Deploy := new(externalpolicy.PropertyList)
	propList_Deploy.Add_Property(externalpolicy.Property_Factory(propName2, "val2"), false)
	propName3 := "prop3"
	propList_Manage := new(externalpolicy.PropertyList)
	propList_Manage.Add_Property(externalpolicy.Property_Factory(propName3, "val3"), false)

	extPolicy := externalpolicy.ExternalPolicy{
		Properties:  *propList,
		Constraints: []string{},
	}
	extPolicy_Deploy := externalpolicy.ExternalPolicy{
		Properties:  *propList_Deploy,
		Constraints: []string{`prop3 == "some value"`},
	}
	extPolicy_Manage := externalpolicy.ExternalPolicy{
		Properties:  *propList_Manage,
		Constraints: []string{`prop4 == "some other value"`},
	}
	nodePolicy := exchangecommon.NodePolicy{ExternalPolicy: extPolicy, Deployment: extPolicy_Deploy, Management: extPolicy_Manage}

	err = SaveNodePolicy(db, &nodePolicy)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if fnp, err := FindNodePolicy(db); err != nil {
		t.Errorf("failed to find node policy in db, error %v", err)
	} else if len(fnp.Properties) != 1 {
		t.Errorf("incorrect node policy, there should be 1 property defined, found: %v", *fnp)
	} else if fnp.Properties[0].Name != propName {
		t.Errorf("expected property %v, but received %v", propName, fnp.Properties[0].Name)
	}

	// Now change the property specified in the policy.
	propName = "prop2"
	propList = new(externalpolicy.PropertyList)
	propList.Add_Property(externalpolicy.Property_Factory(propName, "val2"), false)

	nodePolicy.Properties = *propList

	err = SaveNodePolicy(db, &nodePolicy)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if fnp, err := FindNodePolicy(db); err != nil {
		t.Errorf("failed to find node policy in db, error %v", err)
	} else if len(fnp.Properties) != 1 {
		t.Errorf("incorrect node policy, there should be 1 property defined, found: %v", *fnp)
	} else if fnp.Properties[0].Name != propName {
		t.Errorf("expected property %v, but received %v", propName, fnp.Properties[0].Name)
	}

}

// Verify that a Node Policy Object can be created and then deleted.
func Test_DeleteNodePolicy1(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	propName := "prop1"
	propList := new(externalpolicy.PropertyList)
	propList.Add_Property(externalpolicy.Property_Factory(propName, "val1"), false)
	propName2 := "prop2"
	propList_Deploy := new(externalpolicy.PropertyList)
	propList_Deploy.Add_Property(externalpolicy.Property_Factory(propName2, "val2"), false)
	propName3 := "prop3"
	propList_Manage := new(externalpolicy.PropertyList)
	propList_Manage.Add_Property(externalpolicy.Property_Factory(propName3, "val3"), false)

	extPolicy := externalpolicy.ExternalPolicy{
		Properties:  *propList,
		Constraints: []string{},
	}
	extPolicy_Deploy := externalpolicy.ExternalPolicy{
		Properties:  *propList_Deploy,
		Constraints: []string{`prop3 == "some value"`},
	}
	extPolicy_Manage := externalpolicy.ExternalPolicy{
		Properties:  *propList_Manage,
		Constraints: []string{`prop4 == "some other value"`},
	}
	nodePolicy := exchangecommon.NodePolicy{ExternalPolicy: extPolicy, Deployment: extPolicy_Deploy, Management: extPolicy_Manage}

	err = SaveNodePolicy(db, &nodePolicy)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if fnp, err := FindNodePolicy(db); err != nil {
		t.Errorf("failed to find node policy in db, error %v", err)
	} else if len(fnp.Properties) != 1 {
		t.Errorf("incorrect node policy, there should be 1 property defined, found: %v", *fnp)
	} else if fnp.Properties[0].Name != propName {
		t.Errorf("expected property %v, but received %v", propName, fnp.Properties[0].Name)
	}

	// Now delete the object.

	err = DeleteNodePolicy(db)

	if np, err := FindNodePolicy(db); err != nil {
		t.Errorf("failed to find node policy in db, error %v", err)
	} else if np != nil {
		t.Errorf("incorrect result, there should not be a node policy: %v", *np)
	}

}

// Verify that GetNodePolicyLastUpdated_Exch works when there is no data in the database.
func Test_GetNodePolicyLastUpdated_Exch(t *testing.T) {
	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	if lastUpdated, err := GetNodePolicyLastUpdated_Exch(db); err != nil {
		t.Errorf("failed to find node policy in db, error %v", err)
	} else if lastUpdated != "" {
		t.Errorf("incorrect result, should have returned an empty string but got: %v", lastUpdated)
	}

}

func Test_SaveNodePolicyLastUpdated_Exch(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	lastUpdated := "2019-05-06T21:52:50.010Z[UTC]"
	err = SaveNodePolicyLastUpdated_Exch(db, lastUpdated)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if lastUpdated1, err := GetNodePolicyLastUpdated_Exch(db); err != nil {
		t.Errorf("failed to find last updated value for exchange node policy in local db, error %v", err)
	} else if lastUpdated != lastUpdated1 {
		t.Errorf("incorrect last updated value saved, expecting %v found: %v", lastUpdated, lastUpdated1)
	}
}

// Verify that a exchange node lastUpdated value can be created and then deleted.
func Test_DeleteNodePolicyLastUpdated_Exch(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	lastUpdated := "2019-05-06T21:52:50.010Z[UTC]"
	err = SaveNodePolicyLastUpdated_Exch(db, lastUpdated)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if lastUpdated1, err := GetNodePolicyLastUpdated_Exch(db); err != nil {
		t.Errorf("failed to find last updated value for exchange node policy in local db, error %v", err)
	} else if lastUpdated != lastUpdated1 {
		t.Errorf("incorrect last updated value saved, expecting %v found: %v", lastUpdated, lastUpdated1)
	}

	// Now delete the object.
	err = DeleteNodePolicyLastUpdated_Exch(db)
	if err != nil {
		t.Errorf("Failed to delete saved lastUpdated value from the local db, error %v", err)
	}

	if lastUpdated2, err := GetNodePolicyLastUpdated_Exch(db); err != nil {
		t.Errorf("failed to find last updated value for exchange node policy in local db, error %v", err)
	} else if lastUpdated2 != "" {
		t.Errorf("incorrect result, expecting an empty string but got: %v", lastUpdated2)
	}

	// delete again, should have no error
	err = DeleteNodePolicyLastUpdated_Exch(db)
	if err != nil {
		t.Errorf("Failed to delete saved lastUpdated value from the local db, error %v", err)
	}
}
