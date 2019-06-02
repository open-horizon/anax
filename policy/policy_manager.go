package policy

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"sync"
)

// The PolicyManager's purpose is to manage an in memory representation of all the policies in use
// by the system at runtime. It build on top of the policy_file code to provide abstractions and APIs
// for reusable functions.

type PolicyManager struct {
	AgreementTracking bool                                       // Whether or not to count agreements inside the AgreementCounts map. See PolicyManager_Factory().
	APISpecCounts     bool                                       // How to count agreements inside the AgreementCounts map. See PolicyManager_Factory().
	Policies          map[string][]*Policy                       // The policies in effect at this time, by organization
	PolicyLock        sync.Mutex                                 // The lock that protects modification of the Policies
	ALock             sync.Mutex                                 // The lock that protects the contract counts map
	AgreementCounts   map[string]map[string]*AgreementCountEntry // A map of all policies (by org and name) that have an agreement with a given device
	WatcherContent    *Contents                                  // The contents of the policy file watcher
}

// The ContractCountEntry is used to track which device addresses (contract addresses) are in agreement for a given policy name. The
// status of the agreement is also tracked.
type AgreementCountEntry struct {
	Count        int               // The number of agreements using this policy
	AgreementIds map[string]string // Map of agreement id to agreement status
}

const AGREEMENT_PENDING = "pending"
const AGREEMENT_FINAL = "final"

// A simple function used to return a human readable string representation of the policies that
// the policy manager knows about.
func (self *PolicyManager) AgreementCountString() string {
	res := ""
	for org, orgMap := range self.AgreementCounts {
		for pn, cce := range orgMap {
			res += fmt.Sprintf("Org: %v, Agreement counts policy %v entry: %v\n", org, pn, cce.String())
		}
	}
	return res
}

func (self *PolicyManager) String() string {
	res := ""
	for org, orgArray := range self.Policies {
		res += fmt.Sprintf("Org: %v ", org)
		for _, pol := range orgArray {
			res += fmt.Sprintf("Name: %v Workload: %v\n", pol.Header.Name, pol.Workloads)
		}
	}
	res += self.AgreementCountString()
	return res
}

// A simple function to turn off agreement tracking
func (self *PolicyManager) SetNoAgreementTracking() {
	self.AgreementTracking = false
}

// A simple function used to return a human readable string representation of the agreement counts that
// the policy manager knows about.
func (self *AgreementCountEntry) String() string {
	res := fmt.Sprintf("Count: %v ", self.Count)
	for con, status := range self.AgreementIds {
		res += fmt.Sprintf("Agreement: %v Status: %v ", con, status)
	}
	return res
}

// This function creates the Policy Manager object. The apiSpecCounts input is used to
// tell the policy manager how to count agreements; true - count based on the first API spec in the policy
// object, or false - count by the policy name.  True is used by the device side of anax, false is used
// by the agbot side of anax.
func PolicyManager_Factory(agreementTracking bool, apiSpecCounts bool) *PolicyManager {
	pm := new(PolicyManager)
	pm.APISpecCounts = apiSpecCounts
	pm.AgreementTracking = agreementTracking
	pm.Policies = make(map[string][]*Policy)
	pm.AgreementCounts = make(map[string]map[string]*AgreementCountEntry)

	return pm
}

// Add a new policy to the policy manager. If the policy is already there (by name), an error is returned.
// The caller might be ok ignoring the error.
func (self *PolicyManager) AddPolicy(org string, newPolicy *Policy) error {
	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()

	return self.addPolicy(org, newPolicy)
}

func (self *PolicyManager) addPolicy(org string, newPolicy *Policy) error {

	orgArray, ok := self.Policies[org]
	if !ok {
		self.Policies[org] = make([]*Policy, 0, 10)
		orgArray = self.Policies[org]
	}

	for _, pol := range orgArray {
		if pol.Header.Name == newPolicy.Header.Name {
			return errors.New(fmt.Sprintf("Policy already known to the PolicyManager"))
		}
	}

	self.Policies[org] = append(self.Policies[org], newPolicy)

	if self.AgreementTracking {
		agc := make(map[string]string)
		cce := new(AgreementCountEntry)
		cce.AgreementIds = agc

		if _, ok := self.AgreementCounts[org]; !ok {
			self.AgreementCounts[org] = make(map[string]*AgreementCountEntry)
		}

		keyName := newPolicy.Header.Name
		if self.APISpecCounts && len(newPolicy.APISpecs) > 0 {
			keyName = cutil.FormOrgSpecUrl(newPolicy.APISpecs[0].SpecRef, newPolicy.APISpecs[0].Org)
		}

		self.AgreementCounts[org][keyName] = cce
	}
	return nil
}

// Update a policy in the policy manager. If the policy is already there (by name), update it, otherwise it is just added.
func (self *PolicyManager) UpdatePolicy(org string, newPolicy *Policy) {
	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()

	orgArray, ok := self.Policies[org]
	if !ok {
		self.Policies[org] = make([]*Policy, 0, 10)
		orgArray = self.Policies[org]
	}

	for ix, pol := range orgArray {
		if pol.Header.Name == newPolicy.Header.Name {
			// Replace existing policy
			orgArray[ix] = newPolicy
			return
		}
	}
	self.addPolicy(org, newPolicy)
	return
}

// Delete a policy in the policy manager (by name).
func (self *PolicyManager) DeletePolicy(org string, delPolicy *Policy) {
	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()

	orgArray, ok := self.Policies[org]
	if !ok {
		return
	}

	for ix, pol := range orgArray {
		if pol.Header.Name == delPolicy.Header.Name {
			// Remove existing policy
			orgArray = append(orgArray[:ix], orgArray[ix+1:]...)
			self.Policies[org] = orgArray
			return
		}
	}
	return
}

// Delete a policy in the policy manager (by name).
func (self *PolicyManager) DeletePolicyByName(org string, polName string) {
	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()

	orgArray, ok := self.Policies[org]
	if !ok {
		return
	}

	for ix, pol := range orgArray {
		if pol.Header.Name == polName {
			// Remove existing policy
			orgArray = append(orgArray[:ix], orgArray[ix+1:]...)
			self.Policies[org] = orgArray
			return
		}
	}
	return
}

// This function is used to get the policy manager up and running. When this function returns, all the current policies
// have been read into memory. It can be used instead of the factory method for convenience.
// The agreementTracking boolean is used to tell the policy manager to track agreement counts for each policy name.
// The apiSpecCounts boolean is used to tell the policy manager to track agreement counts by policy apiSpec ref name.
func Initialize(policyPath string,
	arch_synonymns config.ArchSynonyms,
	workloadOrServiceResolver func(wURL string, wOrg string, wVersion string, wArch string) (*APISpecList, error),
	agreementTracking bool,
	apiSpecCounts bool) (*PolicyManager, error) {

	glog.V(1).Infof("Initializing Policy Manager with %v.", policyPath)
	pm := PolicyManager_Factory(agreementTracking, apiSpecCounts)
	numberFiles := 0

	// Setup the callback functions for the policy file watcher. The change notification is the only callback
	// that should be invoked at this time.
	changeNotify := func(org string, fileName string, policy *Policy) {
		numberFiles += 1
		pm.AddPolicy(org, policy)
		glog.V(3).Infof("Found policy file %v/%v containing %v.", org, fileName, policy.Header.Name)
	}

	deleteNotify := func(org string, fileName string, policy *Policy) {
		glog.Errorf("Policy Watcher detected file %v/%v deletion during initialization.", org, fileName)
	}

	errorNotify := func(org string, fileName string, err error) {
		glog.Errorf("Policy Watcher detected error consuming policy file %v/%v during initialization, error: %v", org, fileName, err)
	}

	// Call the policy file watcher once to load up the initial set of policy files
	contents := NewContents()
	if cons, err := PolicyFileChangeWatcher(policyPath, contents, arch_synonymns, changeNotify, deleteNotify, errorNotify, workloadOrServiceResolver, 0); err != nil {
		return nil, err
	} else if pm.NumberPolicies() != numberFiles {
		return nil, errors.New(fmt.Sprintf("Policy Names must be unique, found %v files, but %v unique policies", numberFiles, pm.NumberPolicies()))
	} else {
		pm.WatcherContent = cons
		return pm, nil
	}
}

func (self *PolicyManager) MatchesMine(org string, matchPolicy *Policy) error {

	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()

	if matches, err := self.hasPolicy(org, matchPolicy); err != nil {
		return errors.New(fmt.Sprintf("Policy matching %v not found in %v, error is %v", matchPolicy, self.Policies, err))
	} else if matches {
		return nil
	} else {
		return errors.New(fmt.Sprintf("Policy matching %v not found in %v, no error found", matchPolicy, self.Policies))
	}

}

// This function runs unlocked so that APIs in this module can manage the locks. It returns true
// if the policy manager already has this policy.
func (self *PolicyManager) hasPolicy(org string, matchPolicy *Policy) (bool, error) {

	errString := ""

	orgArray, ok := self.Policies[org]
	if !ok {
		return false, errors.New(fmt.Sprintf("organization %v not found", org))
	}

	for _, pol := range orgArray {
		if errString != "" {
			glog.V(5).Infof("Policy Manager: Previous search loop returned: %v", errString)
		}
		if !pol.Header.IsSame(matchPolicy.Header) {
			errString = fmt.Sprintf("Header %v mismatch with %v", pol.Header, matchPolicy.Header)
			continue
		} else if (len(matchPolicy.Workloads) == 0 || (len(matchPolicy.Workloads) != 0 && matchPolicy.Workloads[0].WorkloadURL == "")) && !pol.APISpecs.IsSame(matchPolicy.APISpecs, true) {
			errString = fmt.Sprintf("API Spec %v mismatch with %v", pol.APISpecs, matchPolicy.APISpecs)
			continue
		} else if !pol.AgreementProtocols.IsSame(matchPolicy.AgreementProtocols) {
			errString = fmt.Sprintf("AgreementProtocol %v mismatch with %v", pol.AgreementProtocols, matchPolicy.AgreementProtocols)
			continue
		} else if !pol.IsSameWorkload(matchPolicy) {
			errString = fmt.Sprintf("Workload %v mismatch with %v", pol.Workloads, matchPolicy.Workloads)
			continue
		} else if !pol.DataVerify.IsSame(matchPolicy.DataVerify) {
			errString = fmt.Sprintf("DataVerify %v mismatch with %v", pol.DataVerify, matchPolicy.DataVerify)
			continue
		} else if !pol.Properties.IsSame(matchPolicy.Properties) {
			errString = fmt.Sprintf("Properties %v mismatch with %v", pol.Properties, matchPolicy.Properties)
			continue
		} else if !pol.Constraints.IsSame(matchPolicy.Constraints) {
			errString = fmt.Sprintf("Constraints %v mismatch with %v", pol.Constraints, matchPolicy.Constraints)
			continue
		} else if pol.RequiredWorkload != matchPolicy.RequiredWorkload {
			errString = fmt.Sprintf("RequiredWorkload %v mismatch with %v", pol.RequiredWorkload, matchPolicy.RequiredWorkload)
			continue
		} else if pol.MaxAgreements != matchPolicy.MaxAgreements {
			errString = fmt.Sprintf("MaxAgreement %v mismatch with %v", pol.MaxAgreements, matchPolicy.MaxAgreements)
			continue
		} else {
			errString = ""
			break
		}

	}

	if errString == "" {
		return true, nil
	} else {
		return false, errors.New(errString)
	}
}

func MarshalPolicy(pol *Policy) (string, error) {
	if polString, err := json.Marshal(pol); err != nil {
		return "", err
	} else {
		return string(polString), nil
	}
}

func DemarshalPolicy(policyString string) (*Policy, error) {
	pol := new(Policy)
	if err := json.Unmarshal([]byte(policyString), pol); err != nil {
		return nil, err
	}

	return pol, nil
}

// Policies can specify that they only want to make agreements with a specific number of counterparties.
// In order to track this, the policy manager has APIs that allow the caller to update it when agreements
// are started, finalized and cancelled. The following APIs are used for this purpose.

// This function is used to indicate a new agreement is in progress for these policies.
func (self *PolicyManager) AttemptingAgreement(policies []Policy, agreement string, org string) error {
	self.ALock.Lock()
	defer self.ALock.Unlock()

	if self.AgreementTracking {
		if policies == nil || len(policies) == 0 {
			return errors.New(fmt.Sprintf("Input policy is nil, agreement: %v", agreement))
		} else if agreement == "" {
			return errors.New(fmt.Sprintf("Input agreement is empty"))
		} else if org == "" {
			return errors.New(fmt.Sprintf("Input org is empty"))
		}

		orgMap, ok := self.AgreementCounts[org]
		if !ok {
			return errors.New(fmt.Sprintf("Unable to find organization %v in agreement counter: %v", org, self))
		}

		for _, pol := range policies {
			keyName := pol.Header.Name
			if self.APISpecCounts && len(pol.APISpecs) > 0 {
				keyName = cutil.FormOrgSpecUrl(pol.APISpecs[0].SpecRef, pol.APISpecs[0].Org)
			}

			if cce, there := orgMap[keyName]; !there {
				return errors.New(fmt.Sprintf("Unable to find policy name %v/%v in agreement counter: %v", org, keyName, self))
			} else if _, there := cce.AgreementIds[agreement]; there {
				return errors.New(fmt.Sprintf("Agreement %v already in agreement counter: %v", agreement, cce))
			} else {
				cce.AgreementIds[agreement] = AGREEMENT_PENDING
				cce.Count = len(cce.AgreementIds)
				glog.V(3).Infof("Policy Manager: Agreement tracking %v", self.AgreementCounts)
			}
		}
	}
	return nil
}

// This function is used to indicate an agreement is finalized for these policies.
func (self *PolicyManager) FinalAgreement(policies []Policy, agreement string, org string) error {
	self.ALock.Lock()
	defer self.ALock.Unlock()

	if self.AgreementTracking {
		if policies == nil || len(policies) == 0 {
			return errors.New(fmt.Sprintf("Input policy is nil, agreement: %v", agreement))
		} else if agreement == "" {
			return errors.New(fmt.Sprintf("Input agreement is empty"))
		} else if org == "" {
			return errors.New(fmt.Sprintf("Input org is empty"))
		}

		orgMap, ok := self.AgreementCounts[org]
		if !ok {
			return errors.New(fmt.Sprintf("Unable to find organization %v in agreement counter: %v", org, self))
		}

		for _, pol := range policies {

			keyName := pol.Header.Name
			if self.APISpecCounts && len(pol.APISpecs) > 0 {
				keyName = cutil.FormOrgSpecUrl(pol.APISpecs[0].SpecRef, pol.APISpecs[0].Org)
			}

			if cce, there := orgMap[keyName]; !there {
				return errors.New(fmt.Sprintf("Unable to find policy name %v/%v in agreement counter: %v", org, keyName, self))
			} else if status, there := cce.AgreementIds[agreement]; !there {
				return errors.New(fmt.Sprintf("agreement %v NOT in agreement counter: %v", agreement, cce))
			} else if status != AGREEMENT_PENDING {
				return errors.New(fmt.Sprintf("agreement %v NOT in pending status: %v", agreement, status))
			} else {
				cce.AgreementIds[agreement] = AGREEMENT_FINAL
				glog.V(3).Infof("Policy Manager: Agreement tracking %v", self.AgreementCounts)
			}
		}
	}
	return nil
}

// This function is used to indicate an agreement is cancelled.
func (self *PolicyManager) CancelAgreement(policies []Policy, agreement string, org string) error {
	self.ALock.Lock()
	defer self.ALock.Unlock()

	if self.AgreementTracking {
		if policies == nil || len(policies) == 0 {
			return errors.New(fmt.Sprintf("Input policy is nil, agreement: %v", agreement))
		} else if agreement == "" {
			return errors.New(fmt.Sprintf("Input agreement is empty"))
		} else if org == "" {
			return errors.New(fmt.Sprintf("Input org is empty"))
		}

		orgMap, ok := self.AgreementCounts[org]
		if !ok {
			return errors.New(fmt.Sprintf("Unable to find organization %v in agreement counter: %v", org, self))
		}

		for _, pol := range policies {

			keyName := pol.Header.Name
			if self.APISpecCounts && len(pol.APISpecs) > 0 {
				keyName = cutil.FormOrgSpecUrl(pol.APISpecs[0].SpecRef, pol.APISpecs[0].Org)
			}

			if cce, there := orgMap[keyName]; !there {
				return errors.New(fmt.Sprintf("Unable to find policy name %v/%v in agreement counter: %v", org, keyName, self))
			} else if _, there := cce.AgreementIds[agreement]; !there {
				return errors.New(fmt.Sprintf("agreement %v NOT in agreement counter: %v", agreement, cce))
			} else {
				delete(cce.AgreementIds, agreement)
				cce.Count = len(cce.AgreementIds)
				glog.V(3).Infof("Policy Manager: Agreement tracking %v", self.AgreementCounts)
			}
		}
	}
	return nil
}

// This function returns true if any of the policies has reached its maximum number of agreements.
func (self *PolicyManager) ReachedMaxAgreements(policies []Policy, org string) (bool, error) {
	self.ALock.Lock()
	defer self.ALock.Unlock()

	if self.AgreementTracking {
		if policies == nil || len(policies) == 0 {
			return false, errors.New(fmt.Sprintf("Input policy is nil"))
		} else if org == "" {
			return false, errors.New(fmt.Sprintf("Input org is empty"))
		}

		orgMap, ok := self.AgreementCounts[org]
		if !ok {
			return false, errors.New(fmt.Sprintf("Unable to find organization %v in agreement counter: %v", org, self))
		}

		for _, pol := range policies {

			keyName := pol.Header.Name
			if self.APISpecCounts && len(pol.APISpecs) > 0 {
				keyName = cutil.FormOrgSpecUrl(pol.APISpecs[0].SpecRef, pol.APISpecs[0].Org)
			}

			if cce, there := orgMap[keyName]; !there {
				return false, errors.New(fmt.Sprintf("Unable to find policy name %v in agreement counter: %v", keyName, self))
			} else {
				if reachedMax := self.unlockedReachedMaxAgreements(&pol, cce.Count); reachedMax {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

// This is an internal function that runs unlocked and returns true if the policy has reached its maximum number of agreements.
func (self *PolicyManager) unlockedReachedMaxAgreements(policy *Policy, current int) bool {
	if policy.MaxAgreements != 0 && current > policy.MaxAgreements {
		return true
	} else {
		return false
	}
}

// This function returns a map of json serialized policies in the PM, as strings.
func (self *PolicyManager) GetSerializedPolicies(org string) (map[string]string, error) {

	res := make(map[string]string)
	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()

	orgArray, ok := self.Policies[org]
	if !ok {
		return res, errors.New(fmt.Sprintf("organization %v not found", org))
	}

	for _, pol := range orgArray {
		if serialPol, err := json.Marshal(pol); err != nil {
			return res, errors.New(fmt.Sprintf("Failed to serialize policy %v. Error: %v", *pol, err))
		} else {
			res[pol.Header.Name] = string(serialPol)
		}
	}
	return res, nil
}

// This function returns the policy object of a given name.
func (self *PolicyManager) GetPolicy(org string, name string) *Policy {

	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()

	orgArray, ok := self.Policies[org]
	if !ok {
		return nil
	}

	for _, pol := range orgArray {
		if pol.Header.Name == name {
			return pol
		}
	}
	return nil
}

// This function returns the first policy object that contains the input API spec URL.
// It runs outside the PM lock to make it reusable. It should ONLY be used by functions
// that already hold the PM lock.
func (self *PolicyManager) unlockedGetPolicyByURL(homeOrg string, url string, org string, version string) *Policy {

	orgArray, ok := self.Policies[homeOrg]
	if !ok {
		return nil
	}

	for _, pol := range orgArray {
		glog.Infof("iterating policy: %v %v", pol.Header, pol.APISpecs)
		if pol.APISpecs.ContainsSpecRef(url, org, version) {
			return pol
		}
	}
	return nil
}

// This function returns the first policy object that contains given name.
// It runs outside the PM lock to make it reusable. It should ONLY be used by functions
// that already hold the PM lock.
func (self *PolicyManager) unlockedGetPolicyByName(homeOrg string, polName string) *Policy {

	orgArray, ok := self.Policies[homeOrg]
	if !ok {
		return nil
	}

	for _, pol := range orgArray {
		if pol.Header.Name == polName {
			return pol
		}
	}
	return nil
}

// This function returns the first policy objects that contains the input API spec URL.
// It returns copies so that the caller doesnt have to worry about them changing
// underneath him.
func (self *PolicyManager) GetPolicyByURL(homeOrg string, url string, org string, version string) []Policy {

	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()
	res := make([]Policy, 0, 10)
	pol := self.unlockedGetPolicyByURL(homeOrg, url, org, version)
	if pol != nil {
		res = append(res, *pol)
	}
	return res
}

func (self *PolicyManager) GetAllAgreementProtocols() map[string]BlockchainList {
	protocols := make(map[string]BlockchainList)
	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()

	for _, orgArray := range self.Policies {
		for _, pol := range orgArray {
			for _, agp := range pol.AgreementProtocols {
				protocols[agp.Name] = agp.Blockchains
			}
		}
	}
	return protocols
}

func (self *PolicyManager) GetAllPolicies(org string) []Policy {
	policies := make([]Policy, 0, 10)
	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()

	orgArray, ok := self.Policies[org]
	if !ok {
		return policies
	}

	for _, pol := range orgArray {
		policies = append(policies, *pol)
	}
	return policies
}

func (self *PolicyManager) GetAllPolicyOrgs() []string {
	orgs := make([]string, 0, 10)
	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()

	for org, _ := range self.Policies {
		orgs = append(orgs, org)
	}

	return orgs
}

// returns all the policy names keyed by organization
func (self *PolicyManager) GetAllPolicyNames() map[string][]string {
	ret := make(map[string][]string, 0)
	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()

	if self.Policies != nil {
		for org, p_array := range self.Policies {
			names := make([]string, 0)
			if p_array != nil {
				for _, policy := range p_array {
					names = append(names, policy.Header.Name)
				}
			}
			ret[org] = names
		}
	}

	return ret
}

// returns the policy names for the given organization
func (self *PolicyManager) GetPolicyNamesForOrg(org string) map[string][]string {
	ret := make(map[string][]string, 0)

	p_array := self.GetAllPolicies(org)
	if p_array != nil {
		names := make([]string, 0)
		for _, policy := range p_array {
			names = append(names, policy.Header.Name)
		}
		ret[org] = names
	}

	return ret
}

func (self *PolicyManager) GetAllAvailablePolicies(org string) []Policy {
	policies := make([]Policy, 0, 10)
	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()

	orgArray, ok := self.Policies[org]
	if !ok {
		return policies
	}

	for _, pol := range orgArray {

		keyName := pol.Header.Name
		if self.APISpecCounts && len(pol.APISpecs) > 0 {
			keyName = cutil.FormOrgSpecUrl(pol.APISpecs[0].SpecRef, pol.APISpecs[0].Org)
		}

		if self.AgreementTracking && self.unlockedReachedMaxAgreements(pol, self.AgreementCounts[org][keyName].Count) {
			glog.V(3).Infof("Skipping policy %v, reached maximum of %v agreements.", pol.Header.Name, self.AgreementCounts[org][keyName].Count)
		} else {
			policies = append(policies, *pol)
		}
	}
	return policies
}

func (self *PolicyManager) NumberPolicies() int {
	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()
	res := 0
	for _, orgMap := range self.Policies {
		res += len(orgMap)
	}
	return res
}

// This function is used by a producer to find the original policies that make up
// a merged policy that it received from a consumer. This function is only usable
// from a producer node.
func (self *PolicyManager) GetPolicyList(homeOrg string, inPolicy *Policy) ([]Policy, error) {

	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()

	res := make([]Policy, 0, 10)

	// Get node policy that matches the inPolicy, this is the non-pattern case
	if len(inPolicy.APISpecs) == 0 {
		pol := self.unlockedGetPolicyByName(homeOrg, inPolicy.Header.Name)
		if pol != nil {
			res = append(res, *pol)
		}
	} else { // pattern case

		// Policies that have more than 1 APISpec are policies that have been merged together from more than
		// 1 individual policy. These are producer side policies that represent a request for more than 1
		// microservice.
		for _, apiSpec := range inPolicy.APISpecs {
			pol := self.unlockedGetPolicyByURL(homeOrg, apiSpec.SpecRef, apiSpec.Org, apiSpec.Version)
			if pol != nil {
				res = append(res, *pol)
			} else {
				return nil, errors.New(fmt.Sprintf("could not find policy for %v %v %v", apiSpec.SpecRef, apiSpec.Org, apiSpec.Version))
			}
		}
	}

	return res, nil
}

// Simple function to merge a list of producer policies and return a single merged producer policy.
func (self *PolicyManager) MergeAllProducers(policies *[]Policy, previouslyMerged *Policy) (*Policy, error) {

	if len(*policies) == 0 {
		return nil, nil
	}

	// Merge all the input policies together
	var mergedPolicy *Policy
	for _, pol := range *policies {
		if mergedPolicy == nil {
			mergedPolicy = new(Policy)
			(*mergedPolicy) = pol
		} else if newPolicy, err := Are_Compatible_Producers(mergedPolicy, &pol, uint64(previouslyMerged.DataVerify.Interval)); err != nil {
			return nil, errors.New(fmt.Sprintf("could not merge policies %v and %v, error: %v", mergedPolicy, pol, err))
		} else {
			(*mergedPolicy) = *newPolicy
		}
	}
	return mergedPolicy, nil
}
