package policy

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"reflect"
	"sync"
)

// The PolicyManager's purpose is to manage an in memory representation of all the policies in use
// by the system at runtime. It build on top of the policy_file code to provide abstractions and APIs
// for reusable functions.

type PolicyManager struct {
	Policies        []*Policy                       // The policies in effect at this time
	PolicyLock      sync.Mutex                      // The lock that protects modification of the Policy array
	ALock           sync.Mutex                      // The lock that protects the contract counts map
	AgreementCounts map[string]*AgreementCountEntry // A map of all policies (by name) that have an agreement with a given device
	WatcherContent  map[string]*WatchEntry          // The contents of the policy file watcher
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
	for pn, cce := range self.AgreementCounts {
		res += fmt.Sprintf("Agreement counts policy %v entry: %v\n", pn, cce.String())
	}
	return res
}

func (self *PolicyManager) String() string {
	res := ""
	for _, pol := range self.Policies {
		res += fmt.Sprintf("Name: %v Workload: %v\n", pol.Header.Name, pol.Workloads)
	}
	res += self.AgreementCountString()
	return res
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

// This function creates AgreementProtocol objects
func PolicyManager_Factory() *PolicyManager {
	pm := new(PolicyManager)
	pm.AgreementCounts = make(map[string]*AgreementCountEntry)

	return pm
}

// Add a new policy to the policy manager. If the policy is already there (by name), an error is returned.
// The caller might be ok ignoring the error.
func (self *PolicyManager) AddPolicy(newPolicy *Policy) error {
	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()

	return self.addPolicy(newPolicy)
}

func (self *PolicyManager) addPolicy(newPolicy *Policy) error {

	for _, pol := range self.Policies {
		if pol.Header.Name == newPolicy.Header.Name {
			return errors.New(fmt.Sprintf("Policy already known to the PolicyManager"))
		}
	}
	self.Policies = append(self.Policies, newPolicy)
	agc := make(map[string]string)
	cce := new(AgreementCountEntry)
	cce.AgreementIds = agc
	self.AgreementCounts[newPolicy.Header.Name] = cce
	return nil
}

// Update a policy in the policy manager. If the policy is already there (by name), update it, otherwise it is just added.
func (self *PolicyManager) UpdatePolicy(newPolicy *Policy) {
	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()

	for ix, pol := range self.Policies {
		if pol.Header.Name == newPolicy.Header.Name {
			// Replace existing policy
			self.Policies[ix] = newPolicy
			return
		}
	}
	self.addPolicy(newPolicy)
	return
}

// Delete a policy in the policy manager (by name).
func (self *PolicyManager) DeletePolicy(delPolicy *Policy) {
	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()

	for ix, pol := range self.Policies {
		if pol.Header.Name == delPolicy.Header.Name {
			// Remove existing policy
			self.Policies = append(self.Policies[:ix], self.Policies[ix+1:]...)
			return
		}
	}
	return
}

func (self *PolicyManager) UpgradeAgreementProtocols() {
	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()

	for pix, pol := range self.Policies {
		for aix, agp := range pol.AgreementProtocols {
			if agp.Name == CitizenScientist && agp.ProtocolVersion != 2 {
				self.Policies[pix].AgreementProtocols[aix].ProtocolVersion = 2
			}
		}
	}

}

// This function is used to get the policy manager up and running. When this function returns, all the current policies
// have been read into memory. It can be used instead of the factory method for convenience.
func Initialize(policyPath string, workloadResolver func(wURL string, wVersion string, wArch string) (*APISpecList, error)) (*PolicyManager, error) {

	glog.V(1).Infof("Initializing Policy Manager with %v.", policyPath)
	pm := PolicyManager_Factory()
	numberFiles := 0

	// Setup the callback functions for the policy file watcher. The change notification is the only callback
	// that should be invoked at this time.
	changeNotify := func(fileName string, policy *Policy) {
		numberFiles += 1
		pm.AddPolicy(policy)
		glog.V(3).Infof("Found policy file %v containing %v.", fileName, policy.Header.Name)
	}

	deleteNotify := func(fileName string, policy *Policy) {
		glog.Errorf("Policy Watcher detected file %v deletion during initialization.", fileName)
	}

	errorNotify := func(fileName string, err error) {
		glog.Errorf("Policy Watcher detected error consuming policy file %v during initialization, error: %v", fileName, err)
	}

	// Call the policy file watcher once to load up the initial set of policy files
	contents := make(map[string]*WatchEntry)
	if cons, err := PolicyFileChangeWatcher(policyPath, contents, changeNotify, deleteNotify, errorNotify, workloadResolver, 0); err != nil {
		return nil, err
	} else if len(pm.Policies) != numberFiles {
		return nil, errors.New(fmt.Sprintf("Policy Names must be unique, found %v files, but %v unique policies", numberFiles, len(pm.Policies)))
	} else {
		pm.WatcherContent = cons
		return pm, nil
	}
}

func (self *PolicyManager) MatchesMine(matchPolicy *Policy) error {

	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()

	if matches, err := self.hasPolicy(matchPolicy); err != nil {
		return errors.New(fmt.Sprintf("Policy matching %v not found in %v, error is %v", matchPolicy, self.Policies, err))
	} else if matches {
		return nil
	} else {
		return errors.New(fmt.Sprintf("Policy matching %v not found in %v, no error found", matchPolicy, self.Policies))
	}

}

// This function runs unlocked so that APIs in this module can manage the locks. It returns true
// if the policy manager already has this policy.
func (self *PolicyManager) hasPolicy(matchPolicy *Policy) (bool, error) {

	errString := ""
	for _, pol := range self.Policies {
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
		} else if !reflect.DeepEqual(pol.CounterPartyProperties, matchPolicy.CounterPartyProperties) {
			errString = fmt.Sprintf("CounterPartyProperties %v mismatch with %v", pol.CounterPartyProperties, matchPolicy.CounterPartyProperties)
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
func (self *PolicyManager) AttemptingAgreement(policies []Policy, agreement string) error {
	self.ALock.Lock()
	defer self.ALock.Unlock()

	if policies == nil || len(policies) == 0 {
		return errors.New(fmt.Sprintf("Input policy is nil, agreement: %v", agreement))
	} else if agreement == "" {
		return errors.New(fmt.Sprintf("Input agreement is nil"))
	}

	for _, pol := range policies {
		if cce, there := self.AgreementCounts[pol.Header.Name]; !there {
			return errors.New(fmt.Sprintf("Unable to find policy name %v in agreement counter: %v", pol.Header.Name, self))
		} else if _, there := cce.AgreementIds[agreement]; there {
			return errors.New(fmt.Sprintf("Agreement %v already in agreement counter: %v", agreement, cce))
		} else {
			cce.AgreementIds[agreement] = AGREEMENT_PENDING
			cce.Count = len(cce.AgreementIds)
			glog.V(3).Infof("Policy Manager: Agreement tracking %v", self.AgreementCounts)
		}
	}
	return nil
}

// This function is used to indicate an agreement is finalized for these policies.
func (self *PolicyManager) FinalAgreement(policies []Policy, agreement string) error {
	self.ALock.Lock()
	defer self.ALock.Unlock()

	if policies == nil || len(policies) == 0 {
		return errors.New(fmt.Sprintf("Input policy is nil, agreement: %v", agreement))
	} else if agreement == "" {
		return errors.New(fmt.Sprintf("Input agreement is nil"))
	}

	for _, pol := range policies {
		if cce, there := self.AgreementCounts[pol.Header.Name]; !there {
			return errors.New(fmt.Sprintf("Unable to find policy name %v in agreement counter: %v", pol.Header.Name, self))
		} else if status, there := cce.AgreementIds[agreement]; !there {
			return errors.New(fmt.Sprintf("agreement %v NOT in agreement counter: %v", agreement, cce))
		} else if status != AGREEMENT_PENDING {
			return errors.New(fmt.Sprintf("agreement %v NOT in pending status: %v", agreement, status))
		} else {
			cce.AgreementIds[agreement] = AGREEMENT_FINAL
			glog.V(3).Infof("Policy Manager: Agreement tracking %v", self.AgreementCounts)
		}
	}
	return nil
}

// This function is used to indicate an agreement is cancelled.
func (self *PolicyManager) CancelAgreement(policies []Policy, agreement string) error {
	self.ALock.Lock()
	defer self.ALock.Unlock()

	if policies == nil || len(policies) == 0 {
		return errors.New(fmt.Sprintf("Input policy is nil, agreement: %v", agreement))
	} else if agreement == "" {
		return errors.New(fmt.Sprintf("Input agreement is nil"))
	}

	for _, pol := range policies {
		if cce, there := self.AgreementCounts[pol.Header.Name]; !there {
			return errors.New(fmt.Sprintf("Unable to find policy name %v in agreement counter: %v", pol.Header.Name, self))
		} else if _, there := cce.AgreementIds[agreement]; !there {
			return errors.New(fmt.Sprintf("agreement %v NOT in agreement counter: %v", agreement, cce))
		} else {
			delete(cce.AgreementIds, agreement)
			cce.Count = len(cce.AgreementIds)
			glog.V(3).Infof("Policy Manager: Agreement tracking %v", self.AgreementCounts)
		}
	}
	return nil
}

// This function returns true if any of the policies has reached its maximum number of agreements.
func (self *PolicyManager) ReachedMaxAgreements(policies []Policy) (bool, error) {
	self.ALock.Lock()
	defer self.ALock.Unlock()

	if policies == nil || len(policies) == 0 {
		return false, errors.New(fmt.Sprintf("Input policy is nil"))
	}

	for _, pol := range policies {
		if cce, there := self.AgreementCounts[pol.Header.Name]; !there {
			return false, errors.New(fmt.Sprintf("Unable to find policy name %v in agreement counter: %v", pol.Header.Name, self))
		} else {
			if reachedMax := self.unlockedReachedMaxAgreements(&pol, cce.Count); reachedMax {
				return true, nil
			}
		}
	}
	return false, nil
}

// This function returns true if the policy has reached its maximum number of agreements.
func (self *PolicyManager) GetNumberAgreements(policy *Policy) (int, error) {
	self.ALock.Lock()
	defer self.ALock.Unlock()
	if policy == nil {
		return 0, errors.New(fmt.Sprintf("Input policy is nil"))
	} else if cce, there := self.AgreementCounts[policy.Header.Name]; !there {
		return 0, errors.New(fmt.Sprintf("Unable to find policy name in agreement counter: %v", self))
	} else {
		return cce.Count, nil
	}
}

// This is an internal function that runs unlocked and returns true if the policy has reached its maximum number of agreements.
func (self *PolicyManager) unlockedReachedMaxAgreements(policy *Policy, current int) bool {
	if policy.MaxAgreements != 0 && current > policy.MaxAgreements {
		return true
	} else {
		return false
	}
}

// This function returns an array of json serialized policies in the PM, as strings.
func (self *PolicyManager) GetSerializedPolicies() (map[string]string, error) {

	res := make(map[string]string)
	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()
	for _, pol := range self.Policies {
		if serialPol, err := json.Marshal(pol); err != nil {
			return res, errors.New(fmt.Sprintf("Failed to serialize policy %v. Error: %v", *pol, err))
		} else {
			res[pol.Header.Name] = string(serialPol)
		}
	}
	return res, nil
}

// This function returns the policy object of a given name.
func (self *PolicyManager) GetPolicy(name string) *Policy {

	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()
	for _, pol := range self.Policies {
		if pol.Header.Name == name {
			return pol
		}
	}
	return nil
}

// This function returns the set of policy objects that contain the input API spec URL.
// It runs outside the PM lock to make it reusable. It should ONLY be used by functions
// that already hold the PM lock.
func (self *PolicyManager) unlockedGetPolicyByURL(url string, version string) *Policy {

	for _, pol := range self.Policies {
		if pol.APISpecs.ContainsSpecRef(url, version) {
			return pol
		}
	}
	return nil
}

// This function returns the set of policy objects that contain the input API spec URL.
// It returns copies so that the caller doesnt have to worry about them changing
// underneath him.
func (self *PolicyManager) GetPolicyByURL(url string, version string) []Policy {

	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()
	res := make([]Policy, 0, 10)
	pol := self.unlockedGetPolicyByURL(url, version)
	if pol != nil {
		res = append(res, *pol)
	}
	return res
}

// This function deletes a policy by URL
func (self *PolicyManager) DeletePolicyByURL(url string, version string) error {

	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()
	for _, pol := range self.Policies {
		if pol.APISpecs.ContainsSpecRef(url, version) {
			// save the policy name
			// Delete this policy - compress the array
			// Remove the agreement counts by policy name
		}
	}
	return errors.New("Not implemented yet")
}

func (self *PolicyManager) GetAllAgreementProtocols() map[string]BlockchainList {
	protocols := make(map[string]BlockchainList)
	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()
	for _, pol := range self.Policies {
		for _, agp := range pol.AgreementProtocols {
			protocols[agp.Name] = agp.Blockchains
		}
	}
	return protocols
}

func (self *PolicyManager) GetAllPolicies() []Policy {
	policies := make([]Policy, 0, 10)
	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()
	for _, pol := range self.Policies {
		policies = append(policies, *pol)
	}
	return policies
}

func (self *PolicyManager) GetAllAvailablePolicies() []Policy {
	policies := make([]Policy, 0, 10)
	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()
	for _, pol := range self.Policies {
		if self.unlockedReachedMaxAgreements(pol, self.AgreementCounts[pol.Header.Name].Count) {
			glog.V(3).Infof("Skipping policy %v, reached maximum of %v agreements.", pol.Header.Name, self.AgreementCounts[pol.Header.Name].Count)
		} else {
			policies = append(policies, *pol)
		}
	}
	return policies
}

func (self *PolicyManager) NumberPolicies() int {
	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()
	return len(self.Policies)
}

// This function is used by a producer to find the original policies that make up
// a merged policy that it received from a consumer.
func (self *PolicyManager) GetPolicyList(inPolicy *Policy) ([]Policy, error) {

	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()

	res := make([]Policy, 0, 10)

	// Policies that have more than 1 APISpec are policies that have been merged together from more than
	// 1 individual policy. These are producer side policies that represent a request for more than 1
	// microservice.
	if len(inPolicy.APISpecs) > 1 {
		for _, apiSpec := range inPolicy.APISpecs {
			pol := self.unlockedGetPolicyByURL(apiSpec.SpecRef, apiSpec.Version)
			if pol != nil {
				res = append(res, *pol)
			} else {
				return nil, errors.New(fmt.Sprintf("could not find policy for %v %v", apiSpec.SpecRef, apiSpec.Version))
			}
		}
	} else {
		res = append(res, *inPolicy)
	}
	return res, nil
}

// Simple function to merge a list of producer policies and return a single merged producer policy.
func (self *PolicyManager) MergeAllProducers(policies *[]Policy, previouslyMerged *Policy) (*Policy, error) {

	if len(*policies) == 0 {
		return nil, errors.New(fmt.Sprintf("list is empty, no policies to merge"))
	}

	// Merge all the input policies together
	var mergedPolicy *Policy
	for _, pol := range (*policies) {
		if mergedPolicy == nil {
			mergedPolicy = &pol
		} else if newPolicy, err := Are_Compatible_Producers(mergedPolicy, &pol, uint64(previouslyMerged.DataVerify.Interval)); err != nil {
			return nil, errors.New(fmt.Sprintf("could not merge policies %v and %v, error: %v", mergedPolicy, pol, err))
		} else {
			mergedPolicy = newPolicy
		}
	}
	return mergedPolicy, nil
}
