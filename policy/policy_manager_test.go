// +build unit

package policy

import (
	"encoding/json"
	"strings"
	"testing"
)

func Test_Payloadmanager_init_success1(t *testing.T) {

	if pm, err := Initialize("./test/pffindtest/", make(map[string]string), nil, nil, true); err != nil {
		t.Error(err)
	} else {

		stringDump := pm.String()
		if !strings.Contains(stringDump, "Org: testorg Name: find test policy Workload:") {
			t.Errorf("String format of policy manager is incorrect, returned %v", stringDump)
		} else if pm.GetPolicy("testorg", "find test policy") == nil {
			t.Errorf("Did not find policy by name.")
		} else if pm.AgreementCounts["testorg"]["http://mycompany.com/microservice/spec"].Count != 0 || len(pm.AgreementCounts["testorg"]["http://mycompany.com/microservice/spec"].AgreementIds) != 0 {
			t.Errorf("Contract Count map is not initialized correctly %v", pm)
		} else if atMax, err := pm.ReachedMaxAgreements(pm.GetAllPolicies("testorg"), "testorg"); err != nil {
			t.Error(err)
		} else if atMax {
			t.Errorf("Policy max agreements is %v, should not be reporting reached max contracts.", pm.Policies["testorg"][0].MaxAgreements)
		}

	}
}

func Test_Payloadmanager_init_success2(t *testing.T) {

	if pm, err := Initialize("./test/pfmultiorg/", make(map[string]string), nil, nil, true); err != nil {
		t.Error(err)
	} else {

		stringDump := pm.String()
		if !strings.Contains(stringDump, "Org: testorg Name: find test policy Workload:") {
			t.Errorf("String format of policy manager is incorrect, returned %v", stringDump)
		} else if pm.GetPolicy("testorg", "find test policy") == nil {
			t.Errorf("Did not find policy by name.")
		} else if pm.GetPolicy("testorg2", "find test policy") == nil {
			t.Errorf("Did not find policy by name.")
		} else if num := pm.NumberPolicies(); num != 3 {
			t.Errorf("Did not find 3 policies, found %v", num)
		} else if orgs := pm.GetAllPolicyOrgs(); len(orgs) != 2 {
			t.Errorf("Expected 2 orgs, but got %v", len(orgs))
		}

	}
}

func Test_Payloadmanager_dup_policy_name(t *testing.T) {

	if pm, err := Initialize("./test/pfduptest/", make(map[string]string), nil, nil, true); err != nil {
		t.Errorf("Duplicate policies are handled by this function and it should not return error here. Error: %v.", err)
	} else {
		c := len(pm.WatcherContent.AllWatches["testorg"])
		if c != 1 {
			t.Errorf("Expecting 1 policy file added to the content watcher but got %v", c)
		}
	}
}

func Test_contractCounter_success(t *testing.T) {
	if pm, err := Initialize("./test/pfmatchtest/", make(map[string]string), nil, nil, true); err != nil {
		t.Error(err)
	} else {

		if serialPol, err := pm.GetSerializedPolicies("testorg"); err != nil {
			t.Error(err)
		} else if len(serialPol) != 3 {
			t.Errorf("There should be 3 policies, there are only %v returned.", len(serialPol))
		}

		pol := pm.GetPolicy("testorg", "find policy2")
		if atMax, err := pm.ReachedMaxAgreements([]Policy{*pol}, "testorg"); err != nil {
			t.Error(err)
		} else if atMax {
			t.Errorf("Policy max agreements is %v, should not be reporting reached max contracts.", pol.MaxAgreements)
		}

		if err := pm.AttemptingAgreement([]Policy{*pol}, "0x12345a", "testorg"); err != nil {
			t.Error(err)
		} else if atMax, err := pm.ReachedMaxAgreements([]Policy{*pol}, "testorg"); err != nil {
			t.Error(err)
		} else if atMax {
			t.Errorf("Policy max agreements is %v, should not be reporting reached max contracts.", pol.MaxAgreements)
		} else if err := pm.AttemptingAgreement([]Policy{*pol}, "0x12345b", "testorg"); err != nil {
			t.Error(err)
		} else if atMax, err := pm.ReachedMaxAgreements([]Policy{*pol}, "testorg"); err != nil {
			t.Error(err)
		} else if atMax {
			t.Errorf("Policy max agreements is %v, should not be reporting reached max contracts.", pol.MaxAgreements)
		} else if err := pm.AttemptingAgreement([]Policy{*pol}, "0x12345c", "testorg"); err != nil {
			t.Error(err)
		} else if atMax, err := pm.ReachedMaxAgreements([]Policy{*pol}, "testorg"); err != nil {
			t.Error(err)
		} else if !atMax {
			t.Errorf("Policy max agreements is %v, should be reporting reached max contracts.", pol.MaxAgreements)
		} else if err := pm.FinalAgreement([]Policy{*pol}, "0x12345a", "testorg"); err != nil {
			t.Error(err)
		} else if err := pm.FinalAgreement([]Policy{*pol}, "0x12345b", "testorg"); err != nil {
			t.Error(err)
		} else if err := pm.FinalAgreement([]Policy{*pol}, "0x12345c", "testorg"); err != nil {
			t.Error(err)
		} else if atMax, err := pm.ReachedMaxAgreements([]Policy{*pol}, "testorg"); err != nil {
			t.Error(err)
		} else if !atMax {
			t.Errorf("Policy max agreements is %v, should be reporting reached max contracts.", pol.MaxAgreements)
		} else if err := pm.CancelAgreement([]Policy{*pol}, "0x12345a", "testorg"); err != nil {
			t.Error(err)
		} else if err := pm.CancelAgreement([]Policy{*pol}, "0x12345b", "testorg"); err != nil {
			t.Error(err)
		} else if err := pm.CancelAgreement([]Policy{*pol}, "0x12345c", "testorg"); err != nil {
			t.Error(err)
		} else if atMax, err := pm.ReachedMaxAgreements([]Policy{*pol}, "testorg"); err != nil {
			t.Error(err)
		} else if atMax {
			t.Errorf("Policy max agreements is %v, should NOT be reporting reached max contracts.", pol.MaxAgreements)
		}
	}
}

func Test_contractCounter_failure1(t *testing.T) {

	var wrongPol *Policy
	if pm, err := Initialize("./test/pffindtest/", make(map[string]string), nil, nil, true); err != nil {
		t.Error(err)
	} else {
		// Grab the wrong policy so that we can do error tests
		// Simple match, one attribute for a single group
		wrongPol = pm.GetPolicy("testorg", "find test policy")

	}

	if wrongPol == nil {
		t.Errorf("Should have returned policy pointer.")
	}

	if pm, err := Initialize("./test/pfmatchtest/", make(map[string]string), nil, nil, true); err != nil {
		t.Error(err)
	} else {

		if err := pm.AttemptingAgreement([]Policy{*wrongPol}, "0x12345a", "testorg"); err == nil {
			t.Errorf("Should have returned error attempting agreement on unknown policy.")
		} else if err := pm.FinalAgreement([]Policy{*wrongPol}, "0x12345a", "testorg"); err == nil {
			t.Errorf("Should have returned error finalizing agreement on unknown policy.")
		} else if err := pm.CancelAgreement([]Policy{*wrongPol}, "0x12345a", "testorg"); err == nil {
			t.Errorf("Should have returned error cancelling agreement on unknown policy.")
		} else if atMax, err := pm.ReachedMaxAgreements([]Policy{*wrongPol}, "testorg"); err == nil {
			t.Errorf("Should have returned error checking for max agreement on unknown policy.")
		} else if atMax {
			t.Errorf("Should have returned false when checking for max agreement due to error.")
		} else if err := pm.AttemptingAgreement(nil, "0x12345a", "testorg"); err == nil {
			t.Errorf("Should have returned error attempting agreement on nil policy.")
		} else if err := pm.FinalAgreement(nil, "0x12345a", "testorg"); err == nil {
			t.Errorf("Should have returned error finalizing agreement on nil policy.")
		} else if err := pm.CancelAgreement(nil, "0x12345a", "testorg"); err == nil {
			t.Errorf("Should have returned error cancelling agreement on nil policy.")
		} else if atMax, err := pm.ReachedMaxAgreements(nil, "testorg"); err == nil {
			t.Errorf("Should have returned error checking for max agreement on nil policy.")
		} else if atMax {
			t.Errorf("Should have returned false when checking for max agreement due to error.")
		} else if err := pm.AttemptingAgreement([]Policy{*wrongPol}, "", "testorg"); err == nil {
			t.Errorf("Should have returned error attempting agreement with empty contract address.")
		} else if err := pm.FinalAgreement([]Policy{*wrongPol}, "", "testorg"); err == nil {
			t.Errorf("Should have returned error finalizing agreement with empty contract address.")
		} else if err := pm.CancelAgreement([]Policy{*wrongPol}, "", "testorg"); err == nil {
			t.Errorf("Should have returned error cancelling agreement with empty contract address.")
		}

		// Grab the right policy
		pol := pm.GetPolicy("testorg", "find policy2")
		if err := pm.AttemptingAgreement([]Policy{*pol}, "0x12345a", "testorg"); err != nil {
			t.Error(err)
		} else if err := pm.AttemptingAgreement([]Policy{*pol}, "0x12345a", "testorg"); err == nil {
			t.Errorf("Should have returned error attempting agreement twice on same contract.")
		} else if err := pm.AttemptingAgreement([]Policy{*pol}, "0x12345b", "testorg"); err != nil {
			t.Error(err)
		} else if err := pm.AttemptingAgreement([]Policy{*pol}, "0x12345c", "testorg"); err != nil {
			t.Error(err)
		} else if max, err := pm.ReachedMaxAgreements([]Policy{*pol}, "testorg"); !max || err != nil {
			t.Errorf("Should have returned true, reached max agreements of %v.", pol.MaxAgreements)
		} else if err := pm.FinalAgreement([]Policy{*pol}, "0x12345d", "testorg"); err == nil {
			t.Errorf("Should have returned error attempting to finalize non-existent agreement %v.", pm)
		} else if err := pm.FinalAgreement([]Policy{*pol}, "0x12345c", "testorg"); err != nil {
			t.Error(err)
		} else if err := pm.FinalAgreement([]Policy{*pol}, "0x12345c", "testorg"); err == nil {
			t.Errorf("Should have returned error attempting to finalize already final agreement %v.", pm)
		} else if err := pm.CancelAgreement([]Policy{*pol}, "0x12345d", "testorg"); err == nil {
			t.Errorf("Should have returned error cancelling non-existent agreement %v.", pol.MaxAgreements)
		} else if err := pm.CancelAgreement([]Policy{*pol}, "0x12345b", "testorg"); err != nil {
			t.Error(err)
		} else if atMax, err := pm.ReachedMaxAgreements([]Policy{*pol}, "testorg"); err != nil {
			t.Error(err)
		} else if atMax {
			t.Errorf("Policy max agreements is %v, should NOT be reporting reached max contracts.", pol.MaxAgreements)
		} else if err := pm.AttemptingAgreement([]Policy{*pol}, "0x12345b", "testorg"); err != nil {
			t.Error(err)
		} else if err := pm.FinalAgreement([]Policy{*pol}, "0x12345b", "testorg"); err != nil {
			t.Error(err)
		} else if atMax, err := pm.ReachedMaxAgreements([]Policy{*pol}, "testorg"); err != nil {
			t.Error(err)
		} else if !atMax {
			t.Errorf("Policy max agreements is %v, should be reporting reached max contracts.", pol.MaxAgreements)
		}
	}
}

func Test_find_by_apispec1(t *testing.T) {

	if pm, err := Initialize("./test/pfcompat1/", make(map[string]string), nil, nil, true); err != nil {
		t.Error(err)
	} else {
		searchURL := "http://mycompany.com/dm/gps"
		pols := pm.GetPolicyByURL("testorg", searchURL, "otherorg", "1.0.0")
		if len(pols) != 0 {
			t.Errorf("Expected to find 0 policy, found %v", len(pols))
		}

		searchURL = "http://mycompany.com/dm/cpu_temp"
		pols = pm.GetPolicyByURL("testorg", searchURL, "otherorg", "1.0.1")
		if len(pols) != 1 {
			t.Errorf("Expected to find 1 policies, found %v", len(pols))
		}

		searchURL = ""
		pols = pm.GetPolicyByURL("testorg", searchURL, "otherorg", "1.0.0")
		if len(pols) != 0 {
			t.Errorf("Expected to find 0 policies, found %v", len(pols))
		}
	}
}

func Test_add_policy(t *testing.T) {
	if pm, err := Initialize("./test/pffindtest/", make(map[string]string), nil, nil, false); err != nil {
		t.Error(err)
	} else {

		// Add a new policy file
		newPolicyContent := `{"header":{"name":"new policy","version":"1.0"}}`
		newPolicy := new(Policy)
		if err := json.Unmarshal([]byte(newPolicyContent), newPolicy); err != nil {
			t.Errorf("Error demarshalling new policy: %v", err)
		} else if err := pm.AddPolicy("testorg", newPolicy); err != nil {
			t.Errorf("Error adding new policy: %v", err)
		} else if err := pm.AddPolicy("testorg", newPolicy); err == nil {
			t.Errorf("Should have been an error adding duplicate policy: %v", err)
		} else if num := pm.NumberPolicies(); num != 2 {
			t.Errorf("Expecting 2 policies, have %v", num)
		}

		// update existing policy
		newPolicyContent = `{"header":{"name":"new policy","version":"2.0"}}`
		newPolicy = new(Policy)
		if err := json.Unmarshal([]byte(newPolicyContent), newPolicy); err != nil {
			t.Errorf("Error demarshalling new policy: %v", err)
		} else {
			pm.UpdatePolicy("testorg", newPolicy)
			if num := pm.NumberPolicies(); num != 2 {
				t.Errorf("Expecting 2 policies, have %v", num)
			}
		}

		// add this policy
		newPolicyContent = `{"header":{"name":"new 2 policy","version":"1.0"}}`
		newPolicy = new(Policy)
		if err := json.Unmarshal([]byte(newPolicyContent), newPolicy); err != nil {
			t.Errorf("Error demarshalling new policy: %v", err)
		} else {
			pm.UpdatePolicy("testorg", newPolicy)
			if num := pm.NumberPolicies(); num != 3 {
				t.Errorf("Expecting 3 policies, have %v", num)
			}
		}

		// nothing to delete
		newPolicyContent = `{"header":{"name":"new 3 policy","version":"1.0"}}`
		newPolicy = new(Policy)
		if err := json.Unmarshal([]byte(newPolicyContent), newPolicy); err != nil {
			t.Errorf("Error demarshalling new policy: %v", err)
		} else {
			pm.DeletePolicy("testorg", newPolicy)
			if num := pm.NumberPolicies(); num != 3 {
				t.Errorf("Expecting 3 policies, have %v", num)
			}
		}

		// delete 1 policy
		newPolicyContent = `{"header":{"name":"new policy","version":"2.0"}}`
		newPolicy = new(Policy)
		if err := json.Unmarshal([]byte(newPolicyContent), newPolicy); err != nil {
			t.Errorf("Error demarshalling new policy: %v", err)
		} else {
			pm.DeletePolicy("testorg", newPolicy)
			if num := pm.NumberPolicies(); num != 2 {
				t.Errorf("Expecting 2 policies, have %v", num)
			}
		}

		// delete the other new policy
		newPolicyContent = `{"header":{"name":"new 2 policy","version":"1.0"}}`
		newPolicy = new(Policy)
		if err := json.Unmarshal([]byte(newPolicyContent), newPolicy); err != nil {
			t.Errorf("Error demarshalling new policy: %v", err)
		} else {
			pm.DeletePolicy("testorg", newPolicy)
			if num := pm.NumberPolicies(); num != 1 {
				t.Errorf("Expecting 1 policies, have %v", num)
			}
		}

	}
}

func Test_MergeAllProducers1(t *testing.T) {

	pa := `{"header":{"name":"ms1 policy","version": "2.0"},` +
		`"apiSpec":[{"specRef":"http://mycompany.com/dm/ms1","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"}],` +
		`"agreementProtocols":[{"name":"Basic","protocolVersion":1}],` +
		`"dataVerification":{"enabled":true,"URL":"","interval":0,"metering":{"tokens":2,"per_time_unit":"hour","notification_interval":3600}},` +
		`"maxAgreements":1}`
	pb := `{"header":{"name":"ms2 policy","version": "2.0"},` +
		`"apiSpec":[{"specRef":"http://mycompany.com/dm/ms2","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"}],` +
		`"agreementProtocols":[{"name":"Basic","protocolVersion":1}],` +
		`"dataVerification":{"enabled":true,"URL":"","interval":0,"metering":{"tokens":1,"per_time_unit":"min","notification_interval":120}},` +
		`"maxAgreements":1}`

	pc := `{"header":{"name":"ms1 policy merged with ms2 policy","version":"2.0"},` +
		`"apiSpec":[{"specRef":"http://mycompany.com/dm/ms1","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"},{"specRef":"http://mycompany.com/dm/ms2","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"}],` +
		`"agreementProtocols":[{"name":"Basic","protocolVersion":1}],` +
		`"dataVerification":{"enabled":true,"URL":"","interval":600,"metering":{"tokens":60,"per_time_unit":"hour","notification_interval":120}},` +
		`"maxAgreements":1}`

	if p1 := create_Policy(pa, t); p1 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p1, pa)
	} else if p2 := create_Policy(pb, t); p2 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p2, pb)
	} else if p3 := create_Policy(pc, t); p3 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p3, pc)
	} else {
		policies := []Policy{*p1, *p2}
		pm := PolicyManager_Factory(true)
		if mergedPol, err := pm.MergeAllProducers(&policies, p3); err != nil {
			t.Errorf("Error: %v merging %v and %v\n", err, p1, p2)
		} else if _, err := Are_Compatible_Producers(p3, mergedPol, 600); err != nil {
			t.Errorf("Error: %v merging %v and %v are not compatible\n", err, p3, mergedPol)
		} else {
			t.Logf("Merged Policy from 2 producer policies: %v", mergedPol)
		}
	}
}

func Test_MergeAllProducers2(t *testing.T) {
	var p3 *Policy
	policies := []Policy{}
	pm := PolicyManager_Factory(true)
	if _, err := pm.MergeAllProducers(&policies, p3); err == nil {
		t.Errorf("No policies, should return an error\n")
	}
}

func Test_MergeAllProducers3(t *testing.T) {

	pa := `{"header":{"name":"ms1 policy","version": "2.0"},` +
		`"apiSpec":[{"specRef":"http://mycompany.com/dm/ms1","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"}],` +
		`"agreementProtocols":[{"name":"Basic","protocolVersion":1}],` +
		`"dataVerification":{"enabled":true,"URL":"","interval":0,"metering":{"tokens":2,"per_time_unit":"hour","notification_interval":3600}},` +
		`"maxAgreements":1}`

	pc := `{"header":{"name":"ms1 policy","version": "2.0"},` +
		`"apiSpec":[{"specRef":"http://mycompany.com/dm/ms1","version":"1.0.0","exclusiveAccess":true,"arch":"amd64"}],` +
		`"agreementProtocols":[{"name":"Basic","protocolVersion":1}],` +
		`"dataVerification":{"enabled":true,"URL":"","interval":600,"metering":{"tokens":2,"per_time_unit":"hour","notification_interval":3600}},` +
		`"maxAgreements":1}`

	if p1 := create_Policy(pa, t); p1 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p1, pa)
	} else if p3 := create_Policy(pc, t); p3 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", p3, pc)
	} else {
		policies := []Policy{*p1}
		pm := PolicyManager_Factory(true)
		if mergedPol, err := pm.MergeAllProducers(&policies, p3); err != nil {
			t.Errorf("Error: %v merging %v with nothing\n", err, p1)
		} else if _, err := Are_Compatible_Producers(p3, mergedPol, 600); err != nil {
			t.Errorf("Error: %v merging %v and %v are not compatible\n", err, p3, mergedPol)
		} else {
			t.Logf("Merged Policy from 1 producer policies: %v", mergedPol)
		}
	}
}
