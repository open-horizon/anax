// +build unit

package api

import (
	"flag"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/externalpolicy"
	_ "github.com/open-horizon/anax/externalpolicy/text_language"
	"github.com/open-horizon/anax/persistence"
	"testing"
)

const NUM_BUILT_INS = 7

func init() {
	flag.Set("alsologtostderr", "true")
	flag.Set("v", "7")
	// no need to parse flags, that's done by test framework
}

// Verify that FindNodePolicyForOutput works when there is no node policy defined yet.
func Test_FindNPForOutput0(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	if np, err := FindNodePolicyForOutput(db); err != nil {
		t.Errorf("failed to find node policy in db, error %v", err)
	} else if len(np.Properties) != 0 {
		t.Errorf("incorrect node policy, there should be %v properties defined, found: %v", 0, *np)
	}

}

// Verify that a Node Policy Object can be created and saved the first time.
func Test_SaveNodePolicy1(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)
	node_policy_error_handler := func(device interface{}, err error) bool {
		return errorhandler(err)
	}

	_, err = persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", "device", false, "myOrg", "", persistence.CONFIGSTATE_CONFIGURING, persistence.SoftwareVersion{persistence.AGENT_VERSION: "1.0.0"})
	if err != nil {
		t.Errorf("failed to create persisted device, error %v", err)
	}

	propName1 := "prop1"
	propList := new(externalpolicy.PropertyList)
	propList.Add_Property(externalpolicy.Property_Factory(propName1, "val1"), false)
	constraints := []string{`prop3 == "some value"`}

	propName2 := "prop2"
	propName3 := "prop3"
	propList_Deploy := new(externalpolicy.PropertyList)
	propList_Deploy.Add_Property(externalpolicy.Property_Factory(propName2, "val2"), false)
	propList_Deploy.Add_Property(externalpolicy.Property_Factory(propName3, "val3"), false)
	constraints_Deploy := []string{`prop4 == "some value4"`}

	propName4 := "prop4"
	propName5 := "prop5"
	propList_Manage := new(externalpolicy.PropertyList)
	propList_Manage.Add_Property(externalpolicy.Property_Factory(propName4, "val4"), false)
	propList_Manage.Add_Property(externalpolicy.Property_Factory(propName5, "val5"), false)
	constraints_Manage := []string{`prop5 == "some value5"`}

	extNodePolicy := &exchangecommon.NodePolicy{
		ExternalPolicy: externalpolicy.ExternalPolicy{
			Properties:  *propList,
			Constraints: constraints,
		},
		Deployment: externalpolicy.ExternalPolicy{
			Properties:  *propList_Deploy,
			Constraints: constraints_Deploy,
		},
		Management: externalpolicy.ExternalPolicy{
			Properties:  *propList_Manage,
			Constraints: constraints_Manage,
		},
	}

	ExchangeNodePolicyLastUpdated = ""

	errHandled, np, msgs := UpdateNodePolicy(extNodePolicy, node_policy_error_handler, getDummyNodePolicyHandler(extNodePolicy), getDummyPutNodePolicyHandler(), db)

	if errHandled {
		t.Errorf("Unexpected error handled: %v", myError)
	} else if myError != nil && len(myError.Error()) != 0 {
		t.Errorf("myError set unexpectedly (%T) %v", myError, myError)
	} else if np == nil {
		t.Errorf("no node policy returned")
	} else if fnp, err := FindNodePolicyForOutput(db); err != nil {
		t.Errorf("failed to find node policy in db, error %v", err)
	} else if len(fnp.Properties) != len(*propList)+NUM_BUILT_INS {
		t.Errorf("incorrect node policy, there should be %v property defined, found: %v", len(*propList)+NUM_BUILT_INS, len(fnp.Properties))
	} else if fnp.Properties[0].Name != propName1 {
		t.Errorf("expected property %v, but received %v", propName1, fnp.Properties[0].Name)
	} else if len(fnp.Constraints) != len(constraints) {
		t.Errorf("incorrect node policy, there should be %v constraints defined, found: %v", len(constraints), len(fnp.Constraints))
	} else if len(fnp.Deployment.Properties) != len(*propList_Deploy) {
		t.Errorf("incorrect deployment node policy, there should be %v property defined, found: %v", len(*propList_Deploy), len(fnp.Deployment.Properties))
	} else if fnp.Deployment.Properties[0].Name != propName2 {
		t.Errorf("expected deployment property %v, but received %v", propName2, fnp.Deployment.Properties[0].Name)
	} else if len(fnp.Deployment.Constraints) != len(constraints_Deploy) {
		t.Errorf("incorrect deployment node policy, there should be %v constraints defined, found: %v", len(constraints_Deploy), len(fnp.Deployment.Constraints))
	} else if len(fnp.Management.Properties) != len(*propList_Manage) {
		t.Errorf("incorrect management node policy, there should be %v property defined, found: %v", len(*propList_Manage), len(fnp.Management.Properties))
	} else if fnp.Management.Properties[0].Name != propName4 {
		t.Errorf("expected management property %v, but received %v", propName4, fnp.Management.Properties[0].Name)
	} else if len(fnp.Management.Constraints) != len(constraints_Manage) {
		t.Errorf("incorrect deployment node policy, there should be %v constraints defined, found: %v", len(constraints_Manage), len(fnp.Management.Constraints))
	} else if len(msgs) != 1 {
		t.Errorf("there should be 1 message, returned %v", len(msgs))
	}

}

// Verify that a Node Policy Object can be created and saved, and then updated.
func Test_UpdateNodePolicy1(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)
	node_policy_error_handler := func(device interface{}, err error) bool {
		return errorhandler(err)
	}

	_, err = persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", "cluster", false, "myOrg", "", persistence.CONFIGSTATE_CONFIGURING, persistence.SoftwareVersion{persistence.AGENT_VERSION: "1.0.0"})
	if err != nil {
		t.Errorf("failed to create persisted device, error %v", err)
	}

	propName := "prop1"
	propList := new(externalpolicy.PropertyList)
	propList.Add_Property(externalpolicy.Property_Factory(propName, "val1"), false)

	extNodePolicy := &exchangecommon.NodePolicy{
		ExternalPolicy: externalpolicy.ExternalPolicy{
			Properties:  *propList,
			Constraints: []string{`prop3 == "some value"`},
		},
	}
	ExchangeNodePolicyLastUpdated = ""

	errHandled, np, msgs := UpdateNodePolicy(extNodePolicy, node_policy_error_handler, getDummyNodePolicyHandler(extNodePolicy), getDummyPutNodePolicyHandler(), db)

	if errHandled {
		t.Errorf("Unexpected error handled: %v", myError)
	} else if myError != nil && len(myError.Error()) != 0 {
		t.Errorf("myError set unexpectedly (%T) %v", myError, myError)
	} else if np == nil {
		t.Errorf("no node policy returned")
	} else if fnp, err := FindNodePolicyForOutput(db); err != nil {
		t.Errorf("failed to find node policy in db, error %v", err)
	} else if len(fnp.Properties) != len(*propList)+NUM_BUILT_INS-3 {
		t.Errorf("incorrect node policy, there should be %v property defined, found: %v", NUM_BUILT_INS, *fnp)
	} else if fnp.Properties[0].Name != propName {
		t.Errorf("expected property %v, but received %v", propName, fnp.Properties[0].Name)
	} else if len(msgs) != 1 {
		t.Errorf("there should be 1 message, returned %v", len(msgs))
	}

	// Now change the property specified in the policy.
	propName = "prop2"
	propList = new(externalpolicy.PropertyList)
	propList.Add_Property(externalpolicy.Property_Factory(propName, "val2"), false)

	extNodePolicy.Properties = *propList

	errHandled, np, msgs = UpdateNodePolicy(extNodePolicy, node_policy_error_handler, getDummyNodePolicyHandler(extNodePolicy), getDummyPutNodePolicyHandler(), db)

	if errHandled {
		t.Errorf("Unexpected error handled: %v", myError)
	} else if myError != nil && len(myError.Error()) != 0 {
		t.Errorf("myError set unexpectedly (%T) %v", myError, myError)
	} else if np == nil {
		t.Errorf("no node policy returned")
	} else if fnp, err := FindNodePolicyForOutput(db); err != nil {
		t.Errorf("failed to find node policy in db, error %v", err)
	} else if len(fnp.Properties) != len(*propList)+NUM_BUILT_INS-3 {
		t.Errorf("incorrect node policy, there should be %v property defined, found: %v", NUM_BUILT_INS, *fnp)
	} else if fnp.Properties[0].Name != propName {
		t.Errorf("expected property %v, but received %v", propName, fnp.Properties[0].Name)
	} else if len(msgs) != 1 {
		t.Errorf("there should be 1 message, returned %v", len(msgs))
	}

}

// Verify that a Node Policy Object can be created and then deleted.
func Test_DeleteNodePolicy1(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)
	node_policy_error_handler := func(device interface{}, err error) bool {
		return errorhandler(err)
	}

	_, err = persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", "", false, "myOrg", "", persistence.CONFIGSTATE_CONFIGURING, persistence.SoftwareVersion{persistence.AGENT_VERSION: "1.0.0"})
	if err != nil {
		t.Errorf("failed to create persisted device, error %v", err)
	}

	propName := "prop1"
	propList := new(externalpolicy.PropertyList)
	propList.Add_Property(externalpolicy.Property_Factory(propName, "val1"), false)

	extNodePolicy := &exchangecommon.NodePolicy{
		ExternalPolicy: externalpolicy.ExternalPolicy{
			Properties:  *propList,
			Constraints: []string{`prop3 == "some value"`},
		},
	}

	ExchangeNodePolicyLastUpdated = ""

	errHandled, np, msgs := UpdateNodePolicy(extNodePolicy, node_policy_error_handler, getDummyNodePolicyHandler(extNodePolicy), getDummyPutNodePolicyHandler(), db)

	if errHandled {
		t.Errorf("Unexpected error handled: %v", myError)
	} else if myError != nil && len(myError.Error()) != 0 {
		t.Errorf("myError set unexpectedly (%T) %v", myError, myError)
	} else if np == nil {
		t.Errorf("no node policy returned")
	} else if fnp, err := FindNodePolicyForOutput(db); err != nil {
		t.Errorf("failed to find node policy in db, error %v", err)
	} else if len(fnp.Properties) != 1+NUM_BUILT_INS {
		t.Errorf("incorrect node policy, there should be %v property defined, found: %v", 1+NUM_BUILT_INS, *fnp)
	} else if fnp.Properties[0].Name != propName {
		t.Errorf("expected property %v, but received %v", propName, fnp.Properties[0].Name)
	} else if len(msgs) != 1 {
		t.Errorf("there should be 1 message, returned %v", len(msgs))
	}

	// Now delete the object.

	errHandled, msgs = DeleteNodePolicy(node_policy_error_handler, db, getDummyNodePolicyHandler(nil), getDummyDeleteNodePolicyHandler())

	if errHandled {
		t.Errorf("Unexpected error handled: %v", myError)
	} else if myError != nil && len(myError.Error()) != 0 {
		t.Errorf("myError set unexpectedly (%T) %v", myError, myError)
	} else if np == nil {
		t.Errorf("no node policy returned")
	} else if fnp, err := FindNodePolicyForOutput(db); err != nil {
		t.Errorf("failed to find node policy in db, error %v", err)
	} else if len(fnp.Properties) != 0 {
		t.Errorf("incorrect node policy, there should be no properties defined, found: %v", *fnp)
	} else if len(msgs) != 1 {
		t.Errorf("there should be 1 message, returned %v", len(msgs))
	}

}
