package api

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
)

func FindPoliciesForOutput(pm *policy.PolicyManager, db *bolt.DB) (map[string]policy.Policy, error) {

	out := make(map[string]policy.Policy)

	// Policies are kept in org specific directories
	allOrgs := pm.GetAllPolicyOrgs()
	for _, org := range allOrgs {

		allPolicies := pm.GetAllPolicies(org)
		for _, pol := range allPolicies {

			// the arch of SPecRefs have been converted to canonical arch in the pm, we will switch to the ones defined in the pattern or by user for output
			if pol.APISpecs != nil {
				for i := 0; i < len(pol.APISpecs); i++ {
					api_spec := &pol.APISpecs[i]
					if pmsdef, err := persistence.FindMicroserviceDefs(db, []persistence.MSFilter{persistence.UnarchivedMSFilter(), persistence.UrlMSFilter(api_spec.SpecRef)}); err != nil {
						glog.Warningf(apiLogString(fmt.Sprintf("Failed to get microservice %v from db. %v", api_spec.SpecRef, err)))
					} else if pmsdef != nil && len(pmsdef) > 0 {
						api_spec.Arch = pmsdef[0].Arch
					}
				}
			}

			out[pol.Header.Name] = pol
		}
	}

	return out, nil
}
