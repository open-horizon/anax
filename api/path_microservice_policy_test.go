// +build unit

package api

import (
	"flag"
	"github.com/open-horizon/anax/policy"
	"testing"
)

func init() {
	flag.Set("alsologtostderr", "true")
	flag.Set("v", "7")
	// no need to parse flags, that's done by test framework
}

func Test_FindPoliciesForOutput0(t *testing.T) {

	// Use the policy library APIs to build up policy objects.
	org := "testorg"
	p1name := "name1"
	p2name := "name2"

	specRef1 := "specurl1"
	specRef2 := "specurl2"
	specVer := "1.0.0"
	specArch := "amd64"
	spec1 := policy.APISpecification_Factory(specRef1, org, specVer, specArch)
	spec2 := policy.APISpecification_Factory(specRef2, org, specVer, specArch)

	newPolicy := policy.Policy_Factory(p1name)
	newPolicy.Add_API_Spec(spec1)

	pm := policy.PolicyManager_Factory(true, true)

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	// An empty PM should return no results
	if out, err := FindPoliciesForOutput(pm, db); err != nil {
		t.Errorf("unexpected err: %v", err)
	} else if len(out) != 0 {
		t.Errorf("expecting no policies in result, but got %v", out)
	}

	// Add 1 policy to the PM
	pm.AddPolicy(org, newPolicy)

	if out, err := FindPoliciesForOutput(pm, db); err != nil {
		t.Errorf("unexpected err: %v", err)
	} else if len(out) != 1 {
		t.Errorf("expecting one policy in result, but got %v", out)
	} else if _, ok := out[p1name]; !ok {
		t.Errorf("expected policy name %v in the output, but got %v", p1name, out)
	}

	// Add 2nd policy to the PM
	newPolicy = policy.Policy_Factory(p2name)
	newPolicy.Add_API_Spec(spec2)
	pm.AddPolicy(org, newPolicy)

	if out, err := FindPoliciesForOutput(pm, db); err != nil {
		t.Errorf("unexpected err: %v", err)
	} else if len(out) != 2 {
		t.Errorf("expecting one policy in result, but got %v", out)
	} else if _, ok := out[p2name]; !ok {
		t.Errorf("expected policy name %v in the output, but got %v", p2name, out)
	}

}
