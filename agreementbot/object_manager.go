package agreementbot

import (
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"sync"
	"time"
)

// This is the main object that manages the cache of object policies. It uses the agbot's served business policies configuration
// to figure out which orgs it is going to serve objects from.
type MMSObjectPolicyManager struct {
	orgMapLock     sync.Mutex                                   // The lock that protects the org map.
	spMapLock      sync.Mutex                                   // The lock that protects the map of ServedPolicies because it is referenced from another thread.
	orgMap         map[string]map[string][]MMSObjectPolicyEntry // The list of object policies in the cache.
	ServedPolicies map[string]exchange.ServedBusinessPolicy     // served node org, business policy org and business policy triplets. The key is the triplet exchange id.
}

func NewMMSObjectPolicyManager() *MMSObjectPolicyManager {
	m := &MMSObjectPolicyManager{
		orgMap: make(map[string]map[string][]MMSObjectPolicyEntry),
	}
	return m
}

func (m *MMSObjectPolicyManager) String() string {
	m.orgMapLock.Lock()
	defer m.orgMapLock.Unlock()

	res := fmt.Sprintf("MMS Object Policy Manager: Org MAP %v", m.orgMap)
	return res
}

// This is an internal function that assumes it is running with the org map lock.
func (m *MMSObjectPolicyManager) hasOrg(org string) bool {
	if _, ok := m.orgMap[org]; ok {
		return true
	}
	return false
}

// Retrieve the object policy from the map of policies. The input serviceName is assumed to be org qualified.
func (m *MMSObjectPolicyManager) GetObjectPolicies(org string, serviceName string) *exchange.ObjectDestinationPolicies {
	m.orgMapLock.Lock()
	defer m.orgMapLock.Unlock()

	objPolicies := new(exchange.ObjectDestinationPolicies)

	if serviceMap, ok := m.orgMap[org]; ok {
		if entryList, found := serviceMap[serviceName]; found {
			for _, entry := range entryList {
				(*objPolicies) = append((*objPolicies), entry.Policy)
			}
		}
		// Query the MMS to see if the object needs to be brought into the cache.
		// TBD
	}
	// Query the MMS to see if the object needs to be brought into the cache.
	// TBD
	return objPolicies
}

func (m *MMSObjectPolicyManager) GetAllPolicyOrgs() []string {
	m.orgMapLock.Lock()
	defer m.orgMapLock.Unlock()

	orgs := make([]string, 0)
	for org, _ := range m.orgMap {
		orgs = append(orgs, org)
	}
	return orgs
}

// copy the given map of served business policies
func (m *MMSObjectPolicyManager) setServedBusinessPolicies(servedPols map[string]exchange.ServedBusinessPolicy) {
	m.spMapLock.Lock()
	defer m.spMapLock.Unlock()

	// copy the input map
	m.ServedPolicies = servedPols
}

// check if the agbot serves the given org or not.
func (m *MMSObjectPolicyManager) serveOrg(polOrg string) bool {
	m.spMapLock.Lock()
	defer m.spMapLock.Unlock()

	for _, sp := range m.ServedPolicies {
		if sp.BusinessPolOrg == polOrg {
			return true
		}
	}
	return false
}

// Given a list of policy_org/policy/node_org triplets that this agbot is supposed to serve, save that list and
// convert it to map of maps (keyed by org and service name) to hold all the policy meta data. This
// will allow the MMSObjectPolicyManager to know when the policy metadata changes.
func (m *MMSObjectPolicyManager) SetCurrentPolicyOrgs(servedPols map[string]exchange.ServedBusinessPolicy) error {
	m.orgMapLock.Lock()
	defer m.orgMapLock.Unlock()

	// Exit early if nothing to do
	if len(m.ServedPolicies) == 0 && len(servedPols) == 0 {
		return nil
	}

	// Save the served business policies.
	m.setServedBusinessPolicies(servedPols)

	// For each org that this agbot is supposed to be serving, check if it is already known.
	// If not add to it. The policies will be added later in the UpdatePolicies function.
	for _, served := range servedPols {
		// If we have encountered a new org in the served policy list, create a map of policies for it.
		if !m.hasOrg(served.BusinessPolOrg) {
			m.orgMap[served.BusinessPolOrg] = make(map[string][]MMSObjectPolicyEntry)
		}
	}

	// For each org in the existing MMSObjectPolicyManager, check to see if its in the new map. If not, then
	// this agbot is no longer serving that org, we can get rid of everything in that org.
	for org, _ := range m.orgMap {
		if !m.serveOrg(org) {
			// delete org and all object policies in it.
			glog.V(5).Infof("Deleting the org %v from the MMS Object Policy manager because it is no longer hosted by the agbot.", org)
			if err := m.deleteOrg(org); err != nil {
				return err
			}
		}
	}

	return nil
}

// This function gets called when object policy updates are detected by the agbot. It will be common for no updates
// to be received most of the time. It should be invoked on a regular basis.
func (m *MMSObjectPolicyManager) UpdatePolicies(org string, updatedPolicies *exchange.ObjectDestinationPolicies, objReceivedHandler exchange.ObjectPolicyUpdateReceivedHandler) ([]events.Message, error) {
	m.orgMapLock.Lock()
	defer m.orgMapLock.Unlock()

	changeEvents := make([]events.Message, 0, 5)

	if updatedPolicies == nil || len(*updatedPolicies) == 0 {
		return changeEvents, nil
	}

	// Exit early on error
	if !m.hasOrg(org) {
		return changeEvents, errors.New(fmt.Sprintf("org %v not found in object policy manager", org))
	}

	// If there are object policies that have been deleted, we wont know until we ask the MMS if the object still exists.
	// Loop through all the cached object polices checking to see if they still exist.
	// TBD...

	// Now we just need to handle adding new or updated object policies. Collect the changes so that we can send out events when we're done.
	var policyReplaced exchange.ObjectDestinationPolicy
	for _, objPol := range *updatedPolicies {

		glog.V(5).Infof(mmsLogString(fmt.Sprintf("Updated policy received %v", objPol)))

		for _, serviceID := range objPol.DestinationPolicy.Services {
			serviceMapKey := cutil.FormOrgSpecUrl(serviceID.ServiceName, serviceID.OrgID)

			if _, ok := m.orgMap[objPol.OrgID][serviceMapKey]; !ok {
				entry := NewMMSObjectPolicyEntry(&objPol)
				entryArray := make([]MMSObjectPolicyEntry, 0, 2)
				entryArray = append(entryArray, *entry)
				m.orgMap[objPol.OrgID][serviceMapKey] = entryArray
			} else {
				// The object policy might already have 1 or more entries in the org map cache, so we need to find and update them.
				found := false
				for ix, existingEntry := range m.orgMap[objPol.OrgID][serviceMapKey] {
					if existingEntry.Policy.OrgID == objPol.OrgID && existingEntry.Policy.ObjectID == objPol.ObjectID && existingEntry.Policy.ObjectType == objPol.ObjectType {
						// Replace the entry
						policyReplaced = existingEntry.Policy
						m.orgMap[objPol.OrgID][serviceMapKey][ix].Policy = objPol
						found = true
						break
					}
				}
				// For the current service in the updated policy object, create a new entry and add it to the map.
				if !found {
					entry := NewMMSObjectPolicyEntry(&objPol)
					m.orgMap[objPol.OrgID][serviceMapKey] = append(m.orgMap[objPol.OrgID][serviceMapKey], *entry)
				}
			}

		}

		// Tell the MMS that the policy has been received
		if err := objReceivedHandler(&objPol); err != nil {
			glog.Errorf(mmsLogString(fmt.Sprintf("unable to update policy received in Model Management System, error %v", err)))
		}

		// Create an event to tell the other workers that a model policy has changed.
		var ev events.Message
		if policyReplaced.OrgID != "" {
			ev = events.NewMMSObjectPolicyMessage(events.OBJECT_POLICY_CHANGED, objPol, policyReplaced)
		} else {
			ev = events.NewMMSObjectPolicyMessage(events.OBJECT_POLICY_NEW, objPol, nil)
		}
		changeEvents = append(changeEvents, ev)

	}

	glog.V(5).Infof(mmsLogString(fmt.Sprintf("Object Policy org map %v", m.orgMap)))
	glog.V(5).Infof(mmsLogString(fmt.Sprintf("produced events %v", changeEvents)))

	return changeEvents, nil
}

// When an org is removed from the list of supported orgs, remove it from the MMSObjectPolicyManager.
func (m *MMSObjectPolicyManager) deleteOrg(org_in string) error {
	// No need to send messages, the business policy manager will do it, and we can respond to those events.

	// Get rid of the org map
	if m.hasOrg(org_in) {
		delete(m.orgMap, org_in)
	}
	return nil
}

type MMSObjectPolicyEntry struct {
	Policy  exchange.ObjectDestinationPolicy `json:"policy,omitempty"`      // the metadata for this object policy in the MMS
	Updated uint64                           `json:"updatedTime,omitempty"` // the time when this entry was updated
}

// Create a new MMSObjectPolicyEntry. It converts the businesspolicy to internal policy format.
// the business policy exchange id (or/id) is the header name for the internal generated policy.
func NewMMSObjectPolicyEntry(pol *exchange.ObjectDestinationPolicy) *MMSObjectPolicyEntry {
	pE := new(MMSObjectPolicyEntry)
	pE.Updated = uint64(time.Now().Unix())
	pE.Policy = (*pol)

	return pE
}

func (p *MMSObjectPolicyEntry) String() string {
	return fmt.Sprintf("MMSObjectPolicyEntry: "+
		"Updated: %v "+
		"Policy: %v",
		p.Updated, p.Policy)
}

func (p *MMSObjectPolicyEntry) ShortString() string {
	return fmt.Sprintf("MMSObjectPolicyEntry: "+
		"Updated: %v "+
		"Policy: %v",
		p.Updated, p.Policy)
}

func (p *MMSObjectPolicyEntry) UpdateEntry(pol *exchange.ObjectDestinationPolicy) (*MMSObjectPolicyEntry, error) {
	p.Updated = uint64(time.Now().Unix())
	p.Policy = (*pol)
	return p, nil
}

// =============================================================================================================
var mmsLogString = func(v interface{}) string {
	return fmt.Sprintf("MMS Object Policy Manager: %v", v)
}
