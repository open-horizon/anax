package agreementbot

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/businesspolicy"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/policy"
	"golang.org/x/crypto/sha3"
	"reflect"
	"sort"
	"sync"
	"time"
)

type ServicePolicyEntry struct {
	Policy  *externalpolicy.ExternalPolicy `json:"policy,omitempty"`      // the metadata for this service policy from the exchange.
	Updated uint64                         `json:"updatedTime,omitempty"` // the time when this entry was updated
	Hash    []byte                         `json:"hash,omitempty"`        // a hash of the service policy to compare for matadata changes in the exchange
}

func (p *ServicePolicyEntry) String() string {
	return fmt.Sprintf("ServicePolicyEntry: "+
		"Updated: %v "+
		"Hash: %x "+
		"Policy: %v",
		p.Updated, p.Hash, p.Policy)
}

func (p *ServicePolicyEntry) ShortString() string {
	return p.String()
}

// return a pointer to a copy of ServicePolicyEntry
func (p *ServicePolicyEntry) DeepCopy() *ServicePolicyEntry {
	var newPolicy *externalpolicy.ExternalPolicy
	if p.Policy == nil {
		newPolicy = nil
	} else {
		newPolicy = p.Policy.DeepCopy()
	}

	newUpdated := p.Updated

	var newHash []byte
	if p.Hash == nil {
		newHash = nil
	} else {
		newHash := make([]byte, len(p.Hash))
		copy(newHash, p.Hash)
	}

	copyP := ServicePolicyEntry{Policy: newPolicy, Updated: newUpdated, Hash: newHash}
	return &copyP

}

func NewServicePolicyEntry(p *externalpolicy.ExternalPolicy, svcId string) (*ServicePolicyEntry, error) {
	pSE := new(ServicePolicyEntry)
	pSE.Updated = uint64(time.Now().Unix())
	if hash, err := hashPolicy(p); err != nil {
		return nil, err
	} else {
		pSE.Hash = hash
	}
	pSE.Policy = p

	return pSE, nil
}

type BusinessPolicyEntry struct {
	Policy          *policy.Policy                 `json:"policy,omitempty"`          // the metadata for this business policy from the exchange, it is the converted to the internal policy format
	Updated         uint64                         `json:"updatedTime,omitempty"`     // the time when this entry was updated
	Hash            []byte                         `json:"hash,omitempty"`            // a hash of the business policy to compare for matadata changes in the exchange
	ServicePolicies map[string]*ServicePolicyEntry `json:"servicePolicies,omitempty"` // map of the service id and service policies
}

// return a pointer to a copy of BusinessPolicyEntry
func (p *BusinessPolicyEntry) DeepCopy() *BusinessPolicyEntry {
	var newPolicy *policy.Policy
	if p.Policy == nil {
		newPolicy = nil
	} else {
		newPolicy = p.Policy.DeepCopy()
	}

	newUpdated := p.Updated

	var newHash []byte
	if p.Hash == nil {
		newHash = nil
	} else {
		newHash := make([]byte, len(p.Hash))
		copy(newHash, p.Hash)
	}

	var newServePolicy map[string]*ServicePolicyEntry
	if p.ServicePolicies == nil {
		newServePolicy = nil
	} else {
		newServePolicy = make(map[string]*ServicePolicyEntry)
		for k, v := range p.ServicePolicies {
			newServePolicy[k] = v.DeepCopy()
		}
	}

	copyBusinessPolicyEntry := BusinessPolicyEntry{Policy: newPolicy, Updated: newUpdated, Hash: newHash, ServicePolicies: newServePolicy}
	return &copyBusinessPolicyEntry

}

// Create a new BusinessPolicyEntry. It converts the businesspolicy to internal policy format.
// the business policy exchange id (or/id) is the header name for the internal generated policy.
func NewBusinessPolicyEntry(pol *businesspolicy.BusinessPolicy, polId string) (*BusinessPolicyEntry, error) {
	pBE := new(BusinessPolicyEntry)
	pBE.Updated = uint64(time.Now().Unix())
	if hash, err := hashPolicy(pol); err != nil {
		return nil, err
	} else {
		pBE.Hash = hash
	}
	pBE.ServicePolicies = make(map[string]*ServicePolicyEntry, 0)

	// validate and convert the exchange business policy to internal policy format
	if err := pol.Validate(); err != nil {
		return nil, fmt.Errorf("Failed to validate the business policy %v. %v", *pol, err)
		//} else if pPolicy, err := pol.GenPolicyFromBusinessPolicy(polId); err != nil {
	} else if pPolicy, err := pol.GenPolicyFromBusinessPolicy(polId); err != nil {
		return nil, fmt.Errorf("Failed to convert the business policy to internal policy format: %v. %v", *pol, err)
	} else {
		pBE.Policy = pPolicy
	}

	return pBE, nil
}

func (p *BusinessPolicyEntry) String() string {
	return fmt.Sprintf("BusinessPolicyEntry: "+
		"Updated: %v "+
		"Hash: %x "+
		"Policy: %v"+
		"ServicePolicies: %v",
		p.Updated, p.Hash, p.Policy, p.ServicePolicies)
}

func (p *BusinessPolicyEntry) ShortString() string {
	keys := make([]string, 0, len(p.ServicePolicies))
	for k, _ := range p.ServicePolicies {
		keys = append(keys, k)
	}
	return fmt.Sprintf("BusinessPolicyEntry: "+
		"Updated: %v "+
		"Hash: %x "+
		"Policy: %v"+
		"ServicePolicies: %v",
		p.Updated, p.Hash, p.Policy.Header.Name, keys)
}

// hash the business policy or the service policy gotten from the exchange
func hashPolicy(p interface{}) ([]byte, error) {
	if ps, err := json.Marshal(p); err != nil {
		return nil, errors.New(fmt.Sprintf("unable to marshal poliy %v to a string, error %v", p, err))
	} else {
		hash := sha3.Sum256([]byte(ps))
		return hash[:], nil
	}
}

// Add a service policy to a BusinessPolicyEntry
// returns true if there is an existing entry for svcId and it is updated with the new policy with is different.
// If the old and new service policies are same, it returns false.
func (p *BusinessPolicyEntry) AddServicePolicy(svcPolicy *externalpolicy.ExternalPolicy, svcId string) (bool, error) {
	if svcPolicy == nil || svcId == "" {
		return false, nil
	}

	pSE, err := NewServicePolicyEntry(svcPolicy, svcId)
	if err != nil {
		return false, err
	}

	servicePol, found := p.ServicePolicies[svcId]
	if !found {
		p.ServicePolicies[svcId] = pSE
		return false, nil
	} else {
		if !bytes.Equal(pSE.Hash, servicePol.Hash) {
			p.ServicePolicies[svcId] = pSE
			p.Updated = uint64(time.Now().Unix())
			return true, nil
		} else {
			// same service policy exists, do nothing
			return false, nil
		}
	}
}

// Remove a service policy from a BusinessPolicyEntry
// It returns true if the service policy exists and is removed
func (p *BusinessPolicyEntry) RemoveServicePolicy(svcId string) bool {
	if svcId == "" {
		return false
	}

	spe, found := p.ServicePolicies[svcId]
	if !found {
		return false
	} else {
		// An empty polcy is also tracked in the business policy manager, this way we know if there is
		// new service policy added later.
		// The business policy manager does not track all the service policies referenced by a business policy.
		// It only tracks the ones that have agreements associated with it.
		tempPol := new(externalpolicy.ExternalPolicy)
		if !reflect.DeepEqual(*tempPol, *spe.Policy) {
			delete(p.ServicePolicies, svcId)

			// update the timestamp
			p.Updated = uint64(time.Now().Unix())
			return true
		} else {
			return false
		}
	}
}

func (pe *BusinessPolicyEntry) DeleteAllServicePolicies(org string) {
	pe.ServicePolicies = make(map[string]*ServicePolicyEntry, 0)
}

func (p *BusinessPolicyEntry) UpdateEntry(pol *businesspolicy.BusinessPolicy, polId string, newHash []byte) (*policy.Policy, error) {
	p.Hash = newHash
	p.Updated = uint64(time.Now().Unix())
	p.ServicePolicies = make(map[string]*ServicePolicyEntry, 0)

	// validate and convert the exchange business policy to internal policy format
	if err := pol.Validate(); err != nil {
		return nil, fmt.Errorf("Failed to validate the business policy %v. %v", *pol, err)
	} else if pPolicy, err := pol.GenPolicyFromBusinessPolicy(polId); err != nil {
		return nil, fmt.Errorf("Failed to convert the business policy to internal policy format: %v. %v", *pol, err)
	} else {
		p.Policy = pPolicy
		return pPolicy, nil
	}
}

type BusinessPolicyManager struct {
	spMapLock      sync.Mutex                                 // The lock that protects the map of ServedPolicies because it is referenced from another thread.
	polMapLock     sync.Mutex                                 // The lock that protects the map of BusinessPolicyEntry because it is referenced from another thread.
	eventChannel   chan events.Message                        // for sending policy change messages
	ServedPolicies map[string]exchange.ServedBusinessPolicy   // served node org, business policy org and business policy triplets. The key is the triplet exchange id.
	OrgPolicies    map[string]map[string]*BusinessPolicyEntry // all served policies by this agbot. The first key is org, the second key is business policy exchange id without org.
}

func (pm *BusinessPolicyManager) String() string {
	pm.polMapLock.Lock()
	defer pm.polMapLock.Unlock()

	res := "Policy Manager: "
	for org, orgMap := range pm.OrgPolicies {
		res += fmt.Sprintf("Org: %v ", org)
		for pat, pe := range orgMap {
			res += fmt.Sprintf("Business policy: %v %v ", pat, pe)
		}
	}

	pm.spMapLock.Lock()
	defer pm.spMapLock.Unlock()

	for _, served := range pm.ServedPolicies {
		res += fmt.Sprintf(" Serve: %v ", served)
	}
	return res
}

func (pm *BusinessPolicyManager) ShortString() string {
	pm.polMapLock.Lock()
	defer pm.polMapLock.Unlock()

	res := "Policy Manager: "
	for org, orgMap := range pm.OrgPolicies {
		res += fmt.Sprintf("Org: %v ", org)
		for pat, pe := range orgMap {
			s := ""
			if pe != nil {
				s = pe.ShortString()
			}
			res += fmt.Sprintf("Business policy: %v %v ", pat, s)
		}
	}
	return res
}

func NewBusinessPolicyManager(eventChannel chan events.Message) *BusinessPolicyManager {
	pm := &BusinessPolicyManager{
		OrgPolicies:  make(map[string]map[string]*BusinessPolicyEntry),
		eventChannel: eventChannel,
	}
	return pm
}

func (pm *BusinessPolicyManager) hasOrg(org string) bool {
	if _, ok := pm.OrgPolicies[org]; ok {
		return true
	}
	return false
}

func (pm *BusinessPolicyManager) hasBusinessPolicy(org string, polName string) bool {
	if pm.hasOrg(org) {
		if _, ok := pm.OrgPolicies[org][polName]; ok {
			return true
		}
	}
	return false
}

func (pm *BusinessPolicyManager) GetAllBusinessPolicyEntriesForOrg(org string) map[string]*BusinessPolicyEntry {
	pm.polMapLock.Lock()
	defer pm.polMapLock.Unlock()

	if pm.hasOrg(org) {
		return pm.OrgPolicies[org]
	}
	return nil
}

func (pm *BusinessPolicyManager) GetAllPoliciesOrderedForOrg(org string, newestFirst bool) []policy.Policy {
	pm.polMapLock.Lock()
	defer pm.polMapLock.Unlock()

	res := make([]policy.Policy, 0, 20)

	if pm.hasOrg(org) {

		// First, get a list of BPEs so we can sort them.
		tempList := make([]*BusinessPolicyEntry, 0, 20)
		for _, bpe := range pm.OrgPolicies[org] {
			tempList = append(tempList, bpe)
		}

		// Second, sort the BPE list based on the requested order.
		if newestFirst {
			sort.Slice(tempList, func(i, j int) bool {
				return tempList[i].Updated > tempList[j].Updated
			})
		} else {
			sort.Sort(BPEsByLastUpdatedTimeAscending(tempList))
		}

		// Last, extract the policy objects and return them.
		for _, bpe := range tempList {
			res = append(res, *bpe.Policy.DeepCopy())
		}

	}

	return res
}

// Helper functions for sorting business policy entries
type BPEsByLastUpdatedTimeAscending []*BusinessPolicyEntry

func (s BPEsByLastUpdatedTimeAscending) Len() int {
	return len(s)
}

func (s BPEsByLastUpdatedTimeAscending) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// The greater than check produces an ordering from highest to lowest.
func (s BPEsByLastUpdatedTimeAscending) Less(i, j int) bool {
	return s[i].Updated < s[j].Updated
}

func (pm *BusinessPolicyManager) GetBusinessPolicyEntry(org string, pol *policy.Policy) *BusinessPolicyEntry {
	pm.polMapLock.Lock()
	defer pm.polMapLock.Unlock()

	if orgMap, ok := pm.OrgPolicies[org]; ok {
		_, polName := cutil.SplitOrgSpecUrl(pol.Header.Name)
		if pBE, found := orgMap[polName]; found {
			return pBE
		}
	}
	return nil
}

func (pm *BusinessPolicyManager) GetAllPolicyOrgs() []string {
	pm.spMapLock.Lock()
	defer pm.spMapLock.Unlock()

	orgs := make([]string, 0)
	for _, sp := range pm.ServedPolicies {
		orgs = append(orgs, sp.BusinessPolOrg)
	}
	return orgs
}

// copy the given map of served business policies
func (pm *BusinessPolicyManager) setServedBusinessPolicies(servedPols map[string]exchange.ServedBusinessPolicy) {
	pm.spMapLock.Lock()
	defer pm.spMapLock.Unlock()

	// copy the input map
	pm.ServedPolicies = servedPols
}

// check if the agbot serves the given business policy or not.
func (pm *BusinessPolicyManager) serveBusinessPolicy(polOrg string, polName string) bool {
	pm.spMapLock.Lock()
	defer pm.spMapLock.Unlock()

	for _, sp := range pm.ServedPolicies {
		if sp.BusinessPolOrg == polOrg && (sp.BusinessPol == polName || sp.BusinessPol == "*") {
			return true
		}
	}
	return false
}

// check if the agbot service the given org or not.
func (pm *BusinessPolicyManager) serveOrg(polOrg string) bool {
	pm.spMapLock.Lock()
	defer pm.spMapLock.Unlock()

	for _, sp := range pm.ServedPolicies {
		if sp.BusinessPolOrg == polOrg {
			return true
		}
	}
	return false
}

// return an array of node orgs for the given served policy org and policy.
// this function is called from a different thread.
func (pm *BusinessPolicyManager) GetServedNodeOrgs(polOrg string, polName string) []string {
	pm.spMapLock.Lock()
	defer pm.spMapLock.Unlock()

	node_orgs := []string{}
	for _, sp := range pm.ServedPolicies {
		if sp.BusinessPolOrg == polOrg && (sp.BusinessPol == polName || sp.BusinessPol == "*") {
			node_org := sp.NodeOrg
			// the default node org is the policy org
			if node_org == "" {
				node_org = sp.BusinessPolOrg
			}
			node_orgs = append(node_orgs, node_org)
		}
	}
	return node_orgs
}

// Getters for BusinessPolicyManager
// return a copy of the ServedPolicies field
func (pm *BusinessPolicyManager) GetServedPolicies() map[string]exchange.ServedBusinessPolicy {
	pm.spMapLock.Lock()
	defer pm.spMapLock.Unlock()

	copyServedPol := make(map[string]exchange.ServedBusinessPolicy)
	for key, sp := range pm.ServedPolicies {
		copyServedPol[key] = sp
	}

	return copyServedPol
}

// return a copy of the OrgPolicies field
func (pm *BusinessPolicyManager) GetOrgPolicies() map[string]map[string]*BusinessPolicyEntry {
	pm.polMapLock.Lock()
	defer pm.polMapLock.Unlock()

	copyOrgPol := make(map[string]map[string]*BusinessPolicyEntry)
	for org, _ := range pm.OrgPolicies {
		OrgPolNames := make(map[string]*BusinessPolicyEntry)
		for n, polEntry := range pm.OrgPolicies[org] {
			OrgPolNames[n] = polEntry.DeepCopy()
		}
		copyOrgPol[org] = OrgPolNames
	}

	return copyOrgPol
}

// Given a list of policy_org/policy/node_org triplets that this agbot is supposed to serve, save that list and
// convert it to map of maps (keyed by org and policy name) to hold all the policy meta data. This
// will allow the BusinessPolicyManager to know when the policy metadata changes.
func (pm *BusinessPolicyManager) SetCurrentBusinessPolicies(servedPols map[string]exchange.ServedBusinessPolicy, polManager *policy.PolicyManager) error {
	pm.polMapLock.Lock()
	defer pm.polMapLock.Unlock()

	// Exit early if nothing to do
	if len(pm.ServedPolicies) == 0 && len(servedPols) == 0 {
		return nil
	}

	// save the served business policies in the pm
	pm.setServedBusinessPolicies(servedPols)

	// Create a new map of maps
	if len(pm.OrgPolicies) == 0 {
		pm.OrgPolicies = make(map[string]map[string]*BusinessPolicyEntry)
	}

	// For each org that this agbot is supposed to be serving, check if it is already in the pm.
	// If not add to it. The policies will be added later in the UpdatePolicies function.
	for _, served := range servedPols {
		// If we have encountered a new org in the served policy list, create a map of policies for it.
		if !pm.hasOrg(served.BusinessPolOrg) {
			pm.OrgPolicies[served.BusinessPolOrg] = make(map[string]*BusinessPolicyEntry)
		}
	}

	// For each org in the existing BusinessPolicyManager, check to see if its in the new map. If not, then
	// this agbot is no longer serving any business polices in that org, we can get rid of everything in that org.
	for org, _ := range pm.OrgPolicies {
		if !pm.serveOrg(org) {
			// delete org and all policy files in it.
			glog.V(5).Infof("Deleting the org %v from the policy manager because it is no longer hosted by the agbot.", org)
			if err := pm.deleteOrg(org, polManager); err != nil {
				return err
			}
		}
	}

	return nil
}

// For each org that the agbot is supporting, take the set of business policies defined within the org and save them into
// the BusinessPolicyManager. When new or updated policies are discovered, clear ServicePolicies for that BusinessPolicyEntry so that
// new businees polices can be filled later.
func (pm *BusinessPolicyManager) UpdatePolicies(org string, definedPolicies map[string]exchange.ExchangeBusinessPolicy, polManager *policy.PolicyManager) error {
	pm.polMapLock.Lock()
	defer pm.polMapLock.Unlock()

	// Exit early on error
	if !pm.hasOrg(org) {
		glog.Infof("org %v not found in policy manager", org)
		return nil
	}

	// If there is no business policy in the org, delete the org from the pm and all of the policy files in the org.
	// This is the case where business policy or the org has been deleted but the agbot still hosts the policy on the exchange.
	if definedPolicies == nil || len(definedPolicies) == 0 {
		// delete org and all policy files in it.
		glog.V(5).Infof("Org %v no longer has any policies in the policy manager", org)
		pm.deleteRemainingPolicies(org, polManager)
		return nil
	}

	// Delete the business policy from the pm if the policy does not exist on the exchange or the agbot
	// does not serve it any more.
	for polName, _ := range pm.OrgPolicies[org] {
		need_delete := true
		if pm.serveBusinessPolicy(org, polName) {
			for polId, _ := range definedPolicies {
				if exchange.GetId(polId) == polName {
					need_delete = false
					break
				}
			}
		}

		if need_delete {
			glog.V(5).Infof("Deleting business policy %v from the org %v from the policy manager because the policy no longer exists.", polName, org)
			if err := pm.deleteBusinessPolicy(org, polName, polManager); err != nil {
				glog.Errorf("Error deleting business policy %v from the org %v in the policy manager. Error: %v", polName, org, err)
				continue
			}
		}
	}

	// Now we just need to handle adding new business policies or update existing business policies
	for polId, exPol := range definedPolicies {
		pol := exPol.GetBusinessPolicy()
		if !pm.serveBusinessPolicy(org, exchange.GetId(polId)) {
			continue
		}
		if err := pm.updateBusinessPolicy(org, polId, &pol, polManager); err != nil {
			glog.Errorf("Error updating business policy %v from the org %v in the policy manager. Error: %v", polId, org, err)
			continue
		}
	}

	return nil
}

// Add or update the given service policy. Send an event message if it is updating so that the catcher can re-evaluate the agreements.
func (pm *BusinessPolicyManager) updateBusinessPolicy(org string, polId string, pol *businesspolicy.BusinessPolicy, polManager *policy.PolicyManager) error {
	need_new_entry := true
	if pm.hasBusinessPolicy(org, exchange.GetId(polId)) {
		if pe := pm.OrgPolicies[org][exchange.GetId(polId)]; pe != nil {
			need_new_entry = false

			// The PolicyEntry is already there, so check if the policy definition has changed.
			// If the policy has changed, Send a PolicyChangedMessage message. Otherwise the policy
			// definition we have is current.
			newHash, err := hashPolicy(pol)
			if err != nil {
				return errors.New(fmt.Sprintf("unable to hash the business policy %v for %v, error %v", pol, org, err))
			}

			if !bytes.Equal(pe.Hash, newHash) {
				// update the cache
				glog.V(5).Infof("Updating policy entry for %v of org %v because it is changed. ", polId, org)
				newPol, err := pe.UpdateEntry(pol, polId, newHash)
				if err != nil {
					return errors.New(fmt.Sprintf("error updating business policy entry for %v of org %v: %v", polId, org, err))
				}

				// notify the policy manager
				polManager.UpdatePolicy(org, newPol)

				// send a message so that other process can handle it by re-negotiating agreements
				glog.V(3).Infof(fmt.Sprintf("Policy manager detected changed business policy %v", polId))
				if policyString, err := policy.MarshalPolicy(newPol); err != nil {
					glog.Errorf(fmt.Sprintf("Error trying to marshal policy %v error: %v", newPol, err))
				} else {
					pm.eventChannel <- events.NewPolicyChangedMessage(events.CHANGED_POLICY, "", newPol.Header.Name, org, policyString)
				}
			}
		}
	}

	//If there's no BusinessPolicyEntry yet, create one
	if need_new_entry {
		if newPE, err := NewBusinessPolicyEntry(pol, polId); err != nil {
			return errors.New(fmt.Sprintf("unable to create business policy entry for %v, error %v", pol, err))
		} else {
			pm.OrgPolicies[org][exchange.GetId(polId)] = newPE

			// notify the policy manager
			polManager.AddPolicy(org, newPE.Policy)

			// send a message so that other process can handle it by re-negotiating agreements
			glog.V(3).Infof(fmt.Sprintf("Policy manager detected new business policy %v", polId))
			if policyString, err := policy.MarshalPolicy(newPE.Policy); err != nil {
				glog.Errorf(fmt.Sprintf("Error trying to marshal policy %v error: %v", newPE.Policy, err))
			} else {
				pm.eventChannel <- events.NewPolicyChangedMessage(events.CHANGED_POLICY, "", newPE.Policy.Header.Name, org, policyString)
			}

		}
	}

	return nil
}

// When an org is removed from the list of supported orgs and business policies, remove the org
// from the BusinessPolicyManager.
func (pm *BusinessPolicyManager) deleteOrg(org_in string, polManager *policy.PolicyManager) error {
	// Get rid of any policies remaining in the org. It is important to call this function so that
	// it will send out the policy delete events for any policies that are deleted.
	pm.deleteRemainingPolicies(org_in, polManager)

	// Get rid of the org map
	if pm.hasOrg(org_in) {
		delete(pm.OrgPolicies, org_in)
	}
	return nil
}

// Remove any policies that might be present in the policy manager for this org.
func (pm *BusinessPolicyManager) deleteRemainingPolicies(org_in string, polManager *policy.PolicyManager) {
	// send PolicyDeletedMessage message for each business policy in the org
	for org, orgMap := range pm.OrgPolicies {
		if org == org_in {
			for polName, pe := range orgMap {
				if pe != nil {
					glog.V(3).Infof(fmt.Sprintf("Policy manager detected deleted policy %v", polName))

					// notify the policy manager
					polManager.DeletePolicy(org, pe.Policy)

					if err := pm.deleteBusinessPolicy(org, polName, polManager); err != nil {
						glog.Errorf("Error deleting business policy %v from the org %v in the policy manager. Error: %v", polName, org, err)
						continue
					}

					if policyString, err := policy.MarshalPolicy(pe.Policy); err != nil {
						glog.Errorf(fmt.Sprintf("Policy manager error trying to marshal policy %v error: %v", polName, err))
					} else {
						pm.eventChannel <- events.NewPolicyDeletedMessage(events.DELETED_POLICY, "", pe.Policy.Header.Name, org, policyString)
					}
				}
			}
			break
		}
	}
}

// When a business policy is removed from the exchange, remove it from the BusinessPolicyManager, PolicyManager and send a PolicyDeletedMessage.
func (pm *BusinessPolicyManager) deleteBusinessPolicy(org string, polName string, polManager *policy.PolicyManager) error {
	// Get rid of the business policy from the pm
	if pm.hasOrg(org) {
		if pe, ok := pm.OrgPolicies[org][polName]; ok {
			if pe != nil {
				glog.V(3).Infof(fmt.Sprintf("Policy manager detected deleted policy %v", polName))

				// notify the policy manager
				polManager.DeletePolicy(org, pe.Policy)

				if policyString, err := policy.MarshalPolicy(pe.Policy); err != nil {
					glog.Errorf(fmt.Sprintf("Policy manager error trying to marshal policy %v error: %v", polName, err))
				} else {
					pm.eventChannel <- events.NewPolicyDeletedMessage(events.DELETED_POLICY, "", pe.Policy.Header.Name, org, policyString)
				}
			}

			delete(pm.OrgPolicies[org], polName)
		}
	}

	return nil
}

// Return all cached service policies for a business policy
func (pm *BusinessPolicyManager) GetServicePoliciesForPolicy(org string, polName string) map[string]externalpolicy.ExternalPolicy {
	pm.polMapLock.Lock()
	defer pm.polMapLock.Unlock()

	svcPols := make(map[string]externalpolicy.ExternalPolicy, 0)
	if pm.hasOrg(org) {
		if entry, ok := pm.OrgPolicies[org][polName]; ok {
			if entry != nil && entry.ServicePolicies != nil {
				for svcId, svcPolEntry := range entry.ServicePolicies {
					svcPols[svcId] = *svcPolEntry.Policy
				}
			}
		}
	}
	return svcPols
}

// Add or update the given marshaled service policy.
func (pm *BusinessPolicyManager) AddMarshaledServicePolicy(businessPolOrg, businessPolName, serviceId, servicePolString string) error {

	servicePol := new(externalpolicy.ExternalPolicy)
	if err := json.Unmarshal([]byte(servicePolString), servicePol); err != nil {
		return fmt.Errorf("Failed to unmashling the given service policy for service %v. %v", serviceId, err)
	}

	return pm.AddServicePolicy(businessPolOrg, businessPolName, serviceId, servicePol)
}

// Add or update the given service policy in all needed business policy entries. Send a message for each business policy if it is updating so that
// the event handler can reevaluating the agreements.
func (pm *BusinessPolicyManager) AddServicePolicy(businessPolOrg string, businessPolName string, serviceId string, servicePol *externalpolicy.ExternalPolicy) error {

	pm.polMapLock.Lock()
	defer pm.polMapLock.Unlock()

	orgMap, found := pm.OrgPolicies[businessPolOrg]
	if !found {
		return fmt.Errorf("No business polices found under org %v.", businessPolOrg)
	}

	pBE, found := orgMap[businessPolName]
	if !found {
		return fmt.Errorf("Cannnot find cached business policy %v/%v", businessPolOrg, businessPolName)
	}

	policyString := ""

	if updated, err := pBE.AddServicePolicy(servicePol, serviceId); err != nil {
		return fmt.Errorf("Faild to add service policy for service %v to the policy manager. %v", serviceId, err)
	} else {
		if updated {
			// send an event for service policy changed
			if polTemp, err := json.Marshal(servicePol); err != nil {
				return fmt.Errorf("Policy manager error trying to marshal service policy for service %v error: %v", serviceId, err)
			} else {
				policyString = string(polTemp)
			}
			pm.eventChannel <- events.NewServicePolicyChangedMessage(events.SERVICE_POLICY_CHANGED, businessPolOrg, businessPolName, serviceId, policyString)
		}

		// check if there are other business policies using the same service policy, we need to update them too
		for org, orgMap := range pm.OrgPolicies {
			if orgMap == nil {
				continue
			}
			for bpName, pbe := range orgMap {
				// this is the one that's just handled, skip it
				if bpName == businessPolName && org == businessPolOrg {
					continue
				}
				if pbe.ServicePolicies == nil {
					continue
				}
				for sId, _ := range pbe.ServicePolicies {
					if sId == serviceId {
						if updated, err := pbe.AddServicePolicy(servicePol, serviceId); err != nil {
							return fmt.Errorf("Faild to update service policy for service %v to the policy manager. %v", serviceId, err)
						} else if updated {
							// send an event for service policy changed
							if policyString == "" {
								if polTemp, err := json.Marshal(servicePol); err != nil {
									return fmt.Errorf("Policy manager error trying to marshal service policy for service %v error: %v", serviceId, err)
								} else {
									policyString = string(polTemp)
								}
							}
							pm.eventChannel <- events.NewServicePolicyChangedMessage(events.SERVICE_POLICY_CHANGED, org, bpName, serviceId, policyString)
						}
					}
				}
			}
		}

		return nil
	}

}

// Delete the given service policy in all the business policy entries. Send a message for each business policy so that
// the event handler can re-evaluating the agreements.
func (pm *BusinessPolicyManager) RemoveServicePolicy(businessPolOrg string, businessPolName string, serviceId string) error {

	pm.polMapLock.Lock()
	defer pm.polMapLock.Unlock()

	orgMap, found := pm.OrgPolicies[businessPolOrg]
	if !found {
		return nil
	}

	pBE, found := orgMap[businessPolName]
	if !found {
		return nil
	}

	if removed := pBE.RemoveServicePolicy(serviceId); removed {
		pm.eventChannel <- events.NewServicePolicyDeletedMessage(events.SERVICE_POLICY_DELETED, businessPolOrg, businessPolName, serviceId)
	}

	// check if there are other business policies using the samve service policy, we need to update them too
	for org, orgMap := range pm.OrgPolicies {
		if orgMap == nil {
			continue
		}
		for bpName, pbe := range orgMap {
			// this is the one that's just handled, skip it
			if bpName == businessPolName && org == businessPolOrg {
				continue
			}
			if pbe.ServicePolicies == nil {
				continue
			}
			for sId, _ := range pbe.ServicePolicies {
				if sId == serviceId {
					if removed := pbe.RemoveServicePolicy(serviceId); removed {
						pm.eventChannel <- events.NewServicePolicyDeletedMessage(events.SERVICE_POLICY_DELETED, businessPolOrg, businessPolName, serviceId)
					}
				}
			}
		}
	}
	return nil
}
