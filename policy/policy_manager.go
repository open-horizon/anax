package policy

import (
    "encoding/json"
    "errors"
    "fmt"
    "github.com/golang/glog"
    "sync"
)

// The PolicyManager's purpose is to manage an in memory representation of all the policies in use
// by the system at runtime. It build on top of the policy_file code to provide abstractions and APIs
// for reusable functions.

type PolicyManager struct {
    Policies          []*Policy                      // The policies in effect at this time
    PolicyLock        sync.Mutex                     // The lock that protects modification of the Policy array
    CCLock            sync.Mutex                     // The lock that protects the contract counts map
    ContractCounts    map[string]*ContractCountEntry // A map of all policies (by name) that have an agreement with a given device
}

// The ContractCountEntry is used to track which device addresses (contract addresses) are in agrement for a given policy name. The
// status of the agreement is also tracked.
type ContractCountEntry struct {
    Count              int               // The number of contracts using this policy
    AgreementContracts map[string]string // Map of device address to agreement status
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
    for pn, cce := range self.ContractCounts {
        res += fmt.Sprintf("Contract counts policy %v entry: %v\n", pn, cce.String())
    }
    return res
}

// A simple function used to return a human readable string representation of the contract counts that
// the policy manager knows about.
func (self *ContractCountEntry) String() string {
    res := fmt.Sprintf("Count: %v ", self.Count)
    for con, status := range self.AgreementContracts {
        res += fmt.Sprintf("Contract: %v Status: %v ", con, status)
    }
    return res
}

// This function creates AgreementProtocol objects
func PolicyManager_Factory() *PolicyManager {
    pm := new(PolicyManager)
    pm.ContractCounts = make(map[string]*ContractCountEntry)

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
        agc := make(map[string]string)
        cce := new(ContractCountEntry)
        cce.AgreementContracts = agc
        pm.ContractCounts[policy.Header.Name] = cce
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
        return err
    } else if matches {
        return nil
    } else {
        return errors.New(fmt.Sprintf("Policy matching %v not found in %v", *matchPolicy, self.Policies))
    }

}

// This function runs unlocked so that APIs in this module can manage the locks. It returns true
// if the policy manager already has this policy.
func (self *PolicyManager) hasPolicy(matchPolicy *Policy) (bool, error) {
    if matchString, err := MarshalPolicy(matchPolicy); err != nil {
        return false, err
    } else {
        for _, pol := range self.Policies {
            if polString, err := MarshalPolicy(pol); err != nil {
                return false, err
                continue
            } else if matchString == polString {
                return true, nil
            }
        }
        return false, nil
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
func (self *PolicyManager) AttemptingAgreement(policy *Policy, contract string) error {
    self.CCLock.Lock()
    defer self.CCLock.Unlock()
    if policy == nil {
        return errors.New(fmt.Sprintf("Input policy for device %v is nil", contract))
    } else if contract == "" {
        return errors.New(fmt.Sprintf("Input contract is nil"))
    } else if cce, there := self.ContractCounts[policy.Header.Name]; !there {
        return errors.New(fmt.Sprintf("Unable to find policy name in contract counter: %v", self))
    } else if _, there := cce.AgreementContracts[contract]; there {
        return errors.New(fmt.Sprintf("Contract %v already in contract counter: %v", contract, cce))
    } else if self.unlockedReachedMaxAgreements(policy, cce.Count) {
        return errors.New(fmt.Sprintf("Policy violation: Contract counter %v already in max agreements: %v", cce, policy.MaxAgreements))
    } else {
        cce.AgreementContracts[contract] = AGREEMENT_PENDING
        cce.Count = len(cce.AgreementContracts)
        glog.V(4).Infof("Policy Manager: Agreement tracking %v", self.ContractCounts)
    }
    return nil
}

// This function is used to indicate an agreement is finalized for this policy.
func (self *PolicyManager) FinalAgreement(policy *Policy, contract string) error {
    self.CCLock.Lock()
    defer self.CCLock.Unlock()
    if policy == nil {
        return errors.New(fmt.Sprintf("Input policy for device %v is nil", contract))
    } else if contract == "" {
        return errors.New(fmt.Sprintf("Input contract is nil"))
    } else if cce, there := self.ContractCounts[policy.Header.Name]; !there {
        return errors.New(fmt.Sprintf("Unable to find policy name in contract counter: %v", self))
    } else if status, there := cce.AgreementContracts[contract]; !there {
        return errors.New(fmt.Sprintf("Contract %v NOT in contract counter: %v", contract, cce))
    } else if status != AGREEMENT_PENDING {
        return errors.New(fmt.Sprintf("Contract %v NOT in pending status: %v", contract, status))
    } else {
        cce.AgreementContracts[contract] = AGREEMENT_FINAL
        glog.V(4).Infof("Policy Manager: Agreement tracking %v", self.ContractCounts)
    }
    return nil
}

// This function is used to indicate an agreement is cancelled.
func (self *PolicyManager) CancelAgreement(policy *Policy, contract string) error {
    self.CCLock.Lock()
    defer self.CCLock.Unlock()
    if policy == nil {
        return errors.New(fmt.Sprintf("Input policy for device %v is nil", contract))
    } else if contract == "" {
        return errors.New(fmt.Sprintf("Input contract is nil"))
    } else if cce, there := self.ContractCounts[policy.Header.Name]; !there {
        return errors.New(fmt.Sprintf("Unable to find policy name in contract counter: %v", self))
    } else if _, there := cce.AgreementContracts[contract]; !there {
        return errors.New(fmt.Sprintf("Contract %v NOT in contract counter: %v", contract, cce))
    } else {
        delete(cce.AgreementContracts, contract)
        cce.Count = len(cce.AgreementContracts)
        glog.V(4).Infof("Policy Manager: Agreement tracking %v", self.ContractCounts)
    }
    return nil
}

// This function returns true if the policy has reached its maximum number of agreements.
func (self *PolicyManager) ReachedMaxAgreements(policy *Policy) (bool, error) {
    self.CCLock.Lock()
    defer self.CCLock.Unlock()
    if policy == nil {
        return false, errors.New(fmt.Sprintf("Input policy is nil"))
    } else if cce, there := self.ContractCounts[policy.Header.Name]; !there {
        return false, errors.New(fmt.Sprintf("Unable to find policy name in contract counter: %v", self))
    } else {
        return self.unlockedReachedMaxAgreements(policy, cce.Count), nil
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

func (self *PolicyManager) NumberPolicies() int {
    self.PolicyLock.Lock()
    defer self.PolicyLock.Unlock()
    return len(self.Policies)
}
