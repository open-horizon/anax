package api

import (
	"github.com/open-horizon/anax/policy"
)

func FindPoliciesForOutput(pm *policy.PolicyManager) (map[string]policy.Policy, error) {

	out := make(map[string]policy.Policy)

	// Policies are kept in org specific directories
	allOrgs := pm.GetAllPolicyOrgs()
	for _, org := range allOrgs {

		allPolicies := pm.GetAllPolicies(org)
		for _, pol := range allPolicies {

			out[pol.Header.Name] = pol
		}
	}

	return out, nil
}
