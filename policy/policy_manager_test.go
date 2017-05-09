package policy

import (
	"encoding/json"
	// "runtime"
	"strings"
	"testing"
	// "time"
)

func Test_Payloadmanager_init_success1(t *testing.T) {

	if pm, err := Initialize("./test/pffindtest/"); err != nil {
		t.Error(err)
	} else {

		stringDump := pm.String()
		if !strings.Contains(stringDump, "Name: find test policy Workload:") {
			t.Errorf("String format of policy manager is incorrect, returned %v", stringDump)
		} else if pm.GetPolicy("find test policy") == nil {
			t.Errorf("Did not find policy by name.")
		} else if pm.AgreementCounts["find test policy"].Count != 0 || len(pm.AgreementCounts["find test policy"].AgreementIds) != 0 {
			t.Errorf("Contract Count map is not initialized correctly %v", pm)
		} else if atMax, err := pm.ReachedMaxAgreements(pm.Policies[0]); err != nil {
			t.Error(err)
		} else if atMax {
			t.Errorf("Policy max agreements is %v, should not be reporting reached max contracts.", pm.Policies[0].MaxAgreements)
		}

	}
}

func Test_Payloadmanager_dup_policy_name(t *testing.T) {

	if _, err := Initialize("./test/pfduptest/"); err == nil {
		t.Errorf("Should have found duplicate policy names but did not.")
	}
}

func Test_contractCounter_success(t *testing.T) {
	if pm, err := Initialize("./test/pfmatchtest/"); err != nil {
		t.Error(err)
	} else {

		if serialPol, err := pm.GetSerializedPolicies(); err != nil {
			t.Error(err)
		} else if len(serialPol) != 3 {
			t.Errorf("There should be 3 policies, there are only %v returned.", len(serialPol))
		}

		pol := pm.GetPolicy("find policy2")
		if atMax, err := pm.ReachedMaxAgreements(pol); err != nil {
			t.Error(err)
		} else if atMax {
			t.Errorf("Policy max agreements is %v, should not be reporting reached max contracts.", pol.MaxAgreements)
		}

		if err := pm.AttemptingAgreement(pol, "0x12345a"); err != nil {
			t.Error(err)
		} else if atMax, err := pm.ReachedMaxAgreements(pol); err != nil {
			t.Error(err)
		} else if atMax {
			t.Errorf("Policy max agreements is %v, should not be reporting reached max contracts.", pol.MaxAgreements)
		} else if err := pm.AttemptingAgreement(pol, "0x12345b"); err != nil {
			t.Error(err)
		} else if atMax, err := pm.ReachedMaxAgreements(pol); err != nil {
			t.Error(err)
		} else if atMax {
			t.Errorf("Policy max agreements is %v, should not be reporting reached max contracts.", pol.MaxAgreements)
		} else if err := pm.AttemptingAgreement(pol, "0x12345c"); err != nil {
			t.Error(err)
		} else if atMax, err := pm.ReachedMaxAgreements(pol); err != nil {
			t.Error(err)
		} else if !atMax {
			t.Errorf("Policy max agreements is %v, should be reporting reached max contracts.", pol.MaxAgreements)
		} else if err := pm.FinalAgreement(pol, "0x12345a"); err != nil {
			t.Error(err)
		} else if err := pm.FinalAgreement(pol, "0x12345b"); err != nil {
			t.Error(err)
		} else if err := pm.FinalAgreement(pol, "0x12345c"); err != nil {
			t.Error(err)
		} else if atMax, err := pm.ReachedMaxAgreements(pol); err != nil {
			t.Error(err)
		} else if !atMax {
			t.Errorf("Policy max agreements is %v, should be reporting reached max contracts.", pol.MaxAgreements)
		} else if err := pm.CancelAgreement(pol, "0x12345a"); err != nil {
			t.Error(err)
		} else if err := pm.CancelAgreement(pol, "0x12345b"); err != nil {
			t.Error(err)
		} else if err := pm.CancelAgreement(pol, "0x12345c"); err != nil {
			t.Error(err)
		} else if atMax, err := pm.ReachedMaxAgreements(pol); err != nil {
			t.Error(err)
		} else if atMax {
			t.Errorf("Policy max agreements is %v, should NOT be reporting reached max contracts.", pol.MaxAgreements)
		}
	}
}

func Test_contractCounter_failure1(t *testing.T) {

	var wrongPol *Policy
	if pm, err := Initialize("./test/pffindtest/"); err != nil {
		t.Error(err)
	} else {
		// Grab the wrong policy so that we can do error tests
		// Simple match, one attribute for a single group
		wrongPol = pm.GetPolicy("find test policy")

	}

	if wrongPol == nil {
		t.Errorf("Should have returned policy pointer.")
	}

	if pm, err := Initialize("./test/pfmatchtest/"); err != nil {
		t.Error(err)
	} else {

		if err := pm.AttemptingAgreement(wrongPol, "0x12345a"); err == nil {
			t.Errorf("Should have returned error attempting agreement on unknown policy.")
		} else if err := pm.FinalAgreement(wrongPol, "0x12345a"); err == nil {
			t.Errorf("Should have returned error finalizing agreement on unknown policy.")
		} else if err := pm.CancelAgreement(wrongPol, "0x12345a"); err == nil {
			t.Errorf("Should have returned error cancelling agreement on unknown policy.")
		} else if atMax, err := pm.ReachedMaxAgreements(wrongPol); err == nil {
			t.Errorf("Should have returned error checking for max agreement on unknown policy.")
		} else if atMax {
			t.Errorf("Should have returned false when checking for max agreement due to error.")
		} else if err := pm.AttemptingAgreement(nil, "0x12345a"); err == nil {
			t.Errorf("Should have returned error attempting agreement on nil policy.")
		} else if err := pm.FinalAgreement(nil, "0x12345a"); err == nil {
			t.Errorf("Should have returned error finalizing agreement on nil policy.")
		} else if err := pm.CancelAgreement(nil, "0x12345a"); err == nil {
			t.Errorf("Should have returned error cancelling agreement on nil policy.")
		} else if atMax, err := pm.ReachedMaxAgreements(nil); err == nil {
			t.Errorf("Should have returned error checking for max agreement on nil policy.")
		} else if atMax {
			t.Errorf("Should have returned false when checking for max agreement due to error.")
		} else if err := pm.AttemptingAgreement(wrongPol, ""); err == nil {
			t.Errorf("Should have returned error attempting agreement with empty contract address.")
		} else if err := pm.FinalAgreement(wrongPol, ""); err == nil {
			t.Errorf("Should have returned error finalizing agreement with empty contract address.")
		} else if err := pm.CancelAgreement(wrongPol, ""); err == nil {
			t.Errorf("Should have returned error cancelling agreement with empty contract address.")
		}

		// Grab the right policy
		pol := pm.GetPolicy("find policy2")
		if err := pm.AttemptingAgreement(pol, "0x12345a"); err != nil {
			t.Error(err)
		} else if err := pm.AttemptingAgreement(pol, "0x12345a"); err == nil {
			t.Errorf("Should have returned error attempting agreement twice on same contract.")
		} else if err := pm.AttemptingAgreement(pol, "0x12345b"); err != nil {
			t.Error(err)
		} else if err := pm.AttemptingAgreement(pol, "0x12345c"); err != nil {
			t.Error(err)
		} else if max, err := pm.ReachedMaxAgreements(pol); !max || err != nil {
			t.Errorf("Should have returned true, reached max agreements of %v.", pol.MaxAgreements)
		} else if err := pm.FinalAgreement(pol, "0x12345d"); err == nil {
			t.Errorf("Should have returned error attempting to finalize non-existent agreement %v.", pm)
		} else if err := pm.FinalAgreement(pol, "0x12345c"); err != nil {
			t.Error(err)
		} else if err := pm.FinalAgreement(pol, "0x12345c"); err == nil {
			t.Errorf("Should have returned error attempting to finalize already final agreement %v.", pm)
		} else if err := pm.CancelAgreement(pol, "0x12345d"); err == nil {
			t.Errorf("Should have returned error cancelling non-existent agreement %v.", pol.MaxAgreements)
		} else if err := pm.CancelAgreement(pol, "0x12345b"); err != nil {
			t.Error(err)
		} else if atMax, err := pm.ReachedMaxAgreements(pol); err != nil {
			t.Error(err)
		} else if atMax {
			t.Errorf("Policy max agreements is %v, should NOT be reporting reached max contracts.", pol.MaxAgreements)
		} else if err := pm.AttemptingAgreement(pol, "0x12345b"); err != nil {
			t.Error(err)
		} else if err := pm.FinalAgreement(pol, "0x12345b"); err != nil {
			t.Error(err)
		} else if atMax, err := pm.ReachedMaxAgreements(pol); err != nil {
			t.Error(err)
		} else if !atMax {
			t.Errorf("Policy max agreements is %v, should be reporting reached max contracts.", pol.MaxAgreements)
		}
	}
}

func Test_find_by_apispec1(t *testing.T) {

	if pm, err := Initialize("./test/pfcompat1/"); err != nil {
		t.Error(err)
	} else {
		searchURL := "http://mycompany.com/dm/gps"
		pols := pm.GetPolicyByURL(searchURL)
		if len(pols) != 1 {
			t.Errorf("Expected to find 1 policy, found %v", len(pols))
		}

		searchURL = "http://mycompany.com/dm/cpu_temp"
		pols = pm.GetPolicyByURL(searchURL)
		if len(pols) != 2 {
			t.Errorf("Expected to find 2 policies, found %v", len(pols))
		}

		searchURL = ""
		pols = pm.GetPolicyByURL(searchURL)
		if len(pols) != 0 {
			t.Errorf("Expected to find 0 policies, found %v", len(pols))
		}
	}
}

func Test_add_policy(t *testing.T) {
	if pm, err := Initialize("./test/pffindtest/"); err != nil {
		t.Error(err)
	} else {

		// Add a new policy file
		newPolicyContent := `{"header":{"name":"new policy","version":"1.0"}}`
		newPolicy := new(Policy)
		if err := json.Unmarshal([]byte(newPolicyContent), newPolicy); err != nil {
			t.Errorf("Error demarshalling new policy: %v", err)
		} else if err := pm.AddPolicy(newPolicy); err != nil {
			t.Errorf("Error adding new policy: %v", err)
		} else if err := pm.AddPolicy(newPolicy); err == nil {
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
			pm.UpdatePolicy(newPolicy)
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
			pm.UpdatePolicy(newPolicy)
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
			pm.DeletePolicy(newPolicy)
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
			pm.DeletePolicy(newPolicy)
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
			pm.DeletePolicy(newPolicy)
			if num := pm.NumberPolicies(); num != 1 {
				t.Errorf("Expecting 1 policies, have %v", num)
			}
		}

	}
}
