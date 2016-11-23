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
}

// The ContractCountEntry is used to track which device addresses (contract addresses) are in agrement for a given policy name. The
// status of the agreement is also tracked.
type AgreementCountEntry struct {
	Count        int               // The number of agreements using this policy
	AgreementIds map[string]string // Map of agreement id to agreement status
}

const AGREEMENT_PENDING = "pending"
const AGREEMENT_FINAL = "final"

// A simple function used to return a human readable string representation of the policies that
// the policy manager knows about.
func (self *PolicyManager) String() string {
	res := ""
	for _, pol := range self.Policies {
		res += fmt.Sprintf("Name: %v Workload: %v\n", pol.Header.Name, pol.Workloads)
	}
	for pn, cce := range self.AgreementCounts {
		res += fmt.Sprintf("Agreement counts policy %v entry: %v\n", pn, cce.String())
	}
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

// This function is used to get the policy manager up and running. When this function returns, all the current policies
// have been read into memory. It can be used instead of the factory method for convenience.
func Initialize(policyPath string) (*PolicyManager, error) {

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
		glog.Errorf("Policy Watcher detected error consuming policy file %v during initialization.", fileName)
	}

	// Call the policy file watcher once to load up the initial set of policy files
	if err := PolicyFileChangeWatcher(policyPath, changeNotify, deleteNotify, errorNotify, 0); err != nil {
		return nil, err
	} else if len(pm.Policies) != numberFiles {
		return nil, errors.New(fmt.Sprintf("Policy Names must be unique, found %v files, but %v unique policies", numberFiles, len(pm.Policies)))
	} else {
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
			glog.V(3).Infof("Policy Manager: Previous search loop returned: %v", errString)
		}
		if !pol.Header.IsSame(matchPolicy.Header) {
			errString = fmt.Sprintf("Header %v mismatch with %v", pol.Header, matchPolicy.Header)
			continue
		} else if !pol.APISpecs.IsSame(matchPolicy.APISpecs) {
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
		} else if !pol.Blockchains.IsSame(matchPolicy.Blockchains) {
			errString = fmt.Sprintf("Blockchain %v mismatch with %v", pol.Blockchains, matchPolicy.Blockchains)
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
// In order to track this, the policy manager has APIs that allow the gov to update it when agreements
// are started, finalized and cancelled. The following APIs are used for this purpose.

// This function is used to indicate a new agreement is in progress for this policy
func (self *PolicyManager) AttemptingAgreement(policy *Policy, agreement string) error {
	self.ALock.Lock()
	defer self.ALock.Unlock()
	if policy == nil {
		return errors.New(fmt.Sprintf("Input policy for device %v is nil", agreement))
	} else if agreement == "" {
		return errors.New(fmt.Sprintf("Input agreement is nil"))
	} else if cce, there := self.AgreementCounts[policy.Header.Name]; !there {
		return errors.New(fmt.Sprintf("Unable to find policy name in agreement counter: %v", self))
	} else if _, there := cce.AgreementIds[agreement]; there {
		return errors.New(fmt.Sprintf("Contract %v already in agreement counter: %v", agreement, cce))
	} else {
		cce.AgreementIds[agreement] = AGREEMENT_PENDING
		cce.Count = len(cce.AgreementIds)
		glog.V(3).Infof("Policy Manager: Agreement tracking %v", self.AgreementCounts)
	}
	return nil
}

// This function is used to indicate an agreement is finalized for this policy.
func (self *PolicyManager) FinalAgreement(policy *Policy, agreement string) error {
	self.ALock.Lock()
	defer self.ALock.Unlock()
	if policy == nil {
		return errors.New(fmt.Sprintf("Input policy for device %v is nil", agreement))
	} else if agreement == "" {
		return errors.New(fmt.Sprintf("Input agreement is nil"))
	} else if cce, there := self.AgreementCounts[policy.Header.Name]; !there {
		return errors.New(fmt.Sprintf("Unable to find policy name in agreement counter: %v", self))
	} else if status, there := cce.AgreementIds[agreement]; !there {
		return errors.New(fmt.Sprintf("agreement %v NOT in agreement counter: %v", agreement, cce))
	} else if status != AGREEMENT_PENDING {
		return errors.New(fmt.Sprintf("agreement %v NOT in pending status: %v", agreement, status))
	} else {
		cce.AgreementIds[agreement] = AGREEMENT_FINAL
		glog.V(3).Infof("Policy Manager: Agreement tracking %v", self.AgreementCounts)
	}
	return nil
}

// This function is used to indicate an agreement is cancelled.
func (self *PolicyManager) CancelAgreement(policy *Policy, agreement string) error {
	self.ALock.Lock()
	defer self.ALock.Unlock()
	if policy == nil {
		return errors.New(fmt.Sprintf("Input policy for device %v is nil", agreement))
	} else if agreement == "" {
		return errors.New(fmt.Sprintf("Input agreement is nil"))
	} else if cce, there := self.AgreementCounts[policy.Header.Name]; !there {
		return errors.New(fmt.Sprintf("Unable to find policy name in agreement counter: %v", self))
	} else if _, there := cce.AgreementIds[agreement]; !there {
		return errors.New(fmt.Sprintf("agreement %v NOT in agreement counter: %v", agreement, cce))
	} else {
		delete(cce.AgreementIds, agreement)
		cce.Count = len(cce.AgreementIds)
		glog.V(3).Infof("Policy Manager: Agreement tracking %v", self.AgreementCounts)
	}
	return nil
}

// This function returns true if the policy has reached its maximum number of agreements.
func (self *PolicyManager) ReachedMaxAgreements(policy *Policy) (bool, error) {
	self.ALock.Lock()
	defer self.ALock.Unlock()
	if policy == nil {
		return false, errors.New(fmt.Sprintf("Input policy is nil"))
	} else if cce, there := self.AgreementCounts[policy.Header.Name]; !there {
		return false, errors.New(fmt.Sprintf("Unable to find policy name in contract counter: %v", self))
	} else {
		return self.unlockedReachedMaxAgreements(policy, cce.Count), nil
	}
}

// This function returns true if the policy has reached its maximum number of agreements.
func (self *PolicyManager) GetNumberAgreements(policy *Policy) (int, error) {
	self.ALock.Lock()
	defer self.ALock.Unlock()
	if policy == nil {
		return 0, errors.New(fmt.Sprintf("Input policy is nil"))
	} else if cce, there := self.AgreementCounts[policy.Header.Name]; !there {
		return 0, errors.New(fmt.Sprintf("Unable to find policy name in contract counter: %v", self))
	} else {
		return cce.Count, nil
	}
}

// This is an internal function that runs unlocked and returns true if the policy has reached its maximum number of agreements.
func (self *PolicyManager) unlockedReachedMaxAgreements(policy *Policy, current int) bool {
	if policy.MaxAgreements != 0 && current >= policy.MaxAgreements {
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

// This function returns the set of policy objects that contains contain the input device URL.
func (self *PolicyManager) GetPolicyByURL(url string) []Policy {

	res := make([]Policy, 0, 10)
	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()
	for _, pol := range self.Policies {
		if pol.APISpecs.ContainsSpecRef(url) {
			res = append(res, *pol)
		}
	}
	return res
}

// This function deletes a policy by URL
func (self *PolicyManager) DeletePolicyByURL(url string) error {

	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()
	for _, pol := range self.Policies {
		if pol.APISpecs.ContainsSpecRef(url) {
			// save the policy name
			// Delete this policy - compress the array
			// Remove the agreement counts by policy name
		}
	}
	return errors.New("Not implemented yet")
}

func (self *PolicyManager) GetAllAgreementProtocols() map[string]bool {
	protocols := make(map[string]bool)
	self.PolicyLock.Lock()
	defer self.PolicyLock.Unlock()
	for _, pol := range self.Policies {
		agps := pol.AgreementProtocols.As_String_Array()
		for _, agp := range agps {
			protocols[agp] = true
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
