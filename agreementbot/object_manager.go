package agreementbot

import (
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/semanticversion"
	"github.com/open-horizon/edge-sync-service/common"
	"sync"
	"time"
)

// This is the main object that manages the cache of object policies. It uses the agbot's served business policies configuration
// to figure out which orgs it is going to serve objects from.
type MMSObjectPolicyManager struct {
	orgMapLock        sync.Mutex                                   // The lock that protects the org map.
	spMapLock         sync.Mutex                                   // The lock that protects the map of ServedOrgs because it is referenced from another thread.
	orgMap            map[string]map[string][]MMSObjectPolicyEntry // The list of object policies in the cache.
	ServedOrgs        map[string]exchange.ServedBusinessPolicy     // The served node org, business policy org and business policy triplets. The key is the triplet exchange id.
	garbageCollection int64                                        // Last time garbage collection was done.
	config            *config.HorizonConfig
}

func NewMMSObjectPolicyManager(cfg *config.HorizonConfig) *MMSObjectPolicyManager {
	m := &MMSObjectPolicyManager{
		orgMap:            make(map[string]map[string][]MMSObjectPolicyEntry),
		garbageCollection: time.Now().Unix(),
		config:            cfg,
	}
	return m
}

func (m *MMSObjectPolicyManager) String() string {
	m.orgMapLock.Lock()
	defer m.orgMapLock.Unlock()

	res := fmt.Sprintf("MMS Object Policy Manager: Org MAP %v", m.orgMap)
	return res
}

func (m *MMSObjectPolicyManager) ShowOrgMapUnlocked() string {
	res := fmt.Sprintf("Org cache: ")
	for org, orgMap := range m.orgMap {
		res += fmt.Sprintf("Org: %v ", org)
		for k, v := range orgMap {
			res += fmt.Sprintf("Service: %v [", k)
			for _, entry := range v {
				res += fmt.Sprintf(" %v", entry.ShortString())
			}
			res += "] "
		}
	}

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
func (m *MMSObjectPolicyManager) GetObjectPolicies(org string, serviceName string, arch string, version string) *exchange.ObjectDestinationPolicies {
	m.orgMapLock.Lock()
	defer m.orgMapLock.Unlock()

	glog.V(5).Infof(mmsLogString(fmt.Sprintf("retrieving objects for %v %v %v %v", org, serviceName, arch, version)))

	objPolicies := new(exchange.ObjectDestinationPolicies)

	if serviceMap, ok := m.orgMap[org]; ok {
		if entryList, found := serviceMap[serviceName]; found {
			for _, entry := range entryList {

				glog.V(5).Infof(mmsLogString(fmt.Sprintf("examining entry %v", entry)))
				// Filter out the objects that don't meet the arch and version specification of the service.
				// The arch in the entry's service ID has already been canonicalized.
				if entry.ServiceID.Arch != "*" && entry.ServiceID.Arch != arch {
					continue
				}

				if ok, err := entry.VersionExpression.Is_within_range(version); err != nil {
					glog.Errorf(mmsLogString(fmt.Sprintf("unable to check version %v against range %v for %v, error %v", version, entry.VersionExpression, serviceName, err)))
					continue
				} else if !ok {
					continue
				}

				// This object passes the filters, so include it.
				(*objPolicies) = append((*objPolicies), entry.Policy)
			}
		}
	}
	glog.V(5).Infof(mmsLogString(fmt.Sprintf("returning objects for %v %v %v %v, %v", org, serviceName, arch, version, objPolicies)))
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
func (m *MMSObjectPolicyManager) setServedBusinessPolicies(servedOrgs map[string]exchange.ServedBusinessPolicy) {
	m.spMapLock.Lock()
	defer m.spMapLock.Unlock()

	// copy the input map
	m.ServedOrgs = servedOrgs
}

// check if the agbot serves the given org or not.
func (m *MMSObjectPolicyManager) serveOrg(polOrg string) bool {
	m.spMapLock.Lock()
	defer m.spMapLock.Unlock()

	for _, sp := range m.ServedOrgs {
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
	if len(m.ServedOrgs) == 0 && len(servedPols) == 0 {
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
			glog.V(5).Infof(mmsLogString(fmt.Sprintf("Deleting the org %v from the MMS Object Policy manager because it is no longer hosted by the agbot.", org)))
			if err := m.deleteOrg(org); err != nil {
				return err
			}
		}
	}

	return nil
}

// This function gets called when object policy updates are detected by the agbot. It will be common for no updates
// to be received most of the time. It should be invoked on a regular basis.
func (m *MMSObjectPolicyManager) UpdatePolicies(org string, updatedPolicies *exchange.ObjectDestinationPolicies, objQueryHandler exchange.ObjectQueryHandler) ([]events.Message, error) {
	m.orgMapLock.Lock()
	defer m.orgMapLock.Unlock()

	changeEvents := make([]events.Message, 0, 5)

	// Exit early on error
	if !m.hasOrg(org) {
		return changeEvents, errors.New(fmt.Sprintf("org %v not found in object policy manager", org))
	}

	// If there are object policies that have been deleted, we wont know until we ask the MMS if the object still exists.
	// Loop through all the cached object policies checking to see if they still exist.
	diff := time.Now().Unix() - m.garbageCollection
	if diff >= m.config.AgreementBot.MMSGarbageCollectionInterval {
		m.garbageCollection = time.Now().Unix()
		glog.V(5).Infof(mmsLogString(fmt.Sprintf("Starting object policy garbage collection")))
		for org, serviceMap := range m.orgMap {
			for service, peList := range serviceMap {
				for ix, pe := range peList {
					if obj, err := objQueryHandler(pe.Policy.OrgID, pe.Policy.ObjectID, pe.Policy.ObjectType); err != nil {
						glog.Errorf(mmsLogString(fmt.Sprintf("error reading object %v %v %v, %v", pe.Policy.OrgID, pe.Policy.ObjectID, pe.Policy.ObjectType, err)))
					} else if obj == nil {
						glog.V(3).Infof(mmsLogString(fmt.Sprintf("object %v/%v %v has been deleted", pe.Policy.OrgID, pe.Policy.ObjectID, pe.Policy.ObjectType)))
						m.orgMap[org][service] = append(m.orgMap[org][service][:ix], m.orgMap[org][service][ix+1:]...)
					}
				}
			}
		}
	}

	// Now we just need to handle adding new or updated object policies. Collect the changes so that we can send out events when we're done.
	if updatedPolicies == nil || len(*updatedPolicies) == 0 {
		return changeEvents, nil
	}

	var policyReplaced exchange.ObjectDestinationPolicy
	foundService := false
	for _, objPol := range *updatedPolicies {

		glog.V(5).Infof(mmsLogString(fmt.Sprintf("Updated policy received %v", objPol)))

		// Find services in the cache that are not referenced by a given object id any more. This can happen if the service
		// reference in the object policy is changed.
		for service, peList := range m.orgMap[objPol.OrgID] {
			for ix, pe := range peList {
				if pe.Policy.OrgID == objPol.OrgID && pe.Policy.ObjectID == objPol.ObjectID && pe.Policy.ObjectType == objPol.ObjectType {
					glog.V(5).Infof(mmsLogString(fmt.Sprintf("Obj %v found in %v map", objPol.ObjectID, service)))
					for _, serviceID := range objPol.DestinationPolicy.Services {
						if service == cutil.FormOrgSpecUrl(serviceID.ServiceName, serviceID.OrgID) {
							foundService = true
							break
						}
					}
					if !foundService {
						policyReplaced = pe.Policy
						glog.V(3).Infof(mmsLogString(fmt.Sprintf("object %v/%v %v policy removed from %v cache.", objPol.OrgID, objPol.ObjectID, objPol.ObjectType, service)))
						m.orgMap[objPol.OrgID][service] = append(m.orgMap[objPol.OrgID][service][:ix], m.orgMap[objPol.OrgID][service][ix+1:]...)

					}
				}
			}
		}

		// Now run through each service in the updated policy and figure out if there is a change or if it's new.
		for _, serviceID := range objPol.DestinationPolicy.Services {

			// Within each org, there is a map keyed by service names (org/service-name).
			serviceMapKey := cutil.FormOrgSpecUrl(serviceID.ServiceName, serviceID.OrgID)

			// If the object's version is invalid, do not include it in the cache.
			versionExp, err := semanticversion.Version_Expression_Factory(serviceID.Version)
			if err != nil {
				glog.Errorf(mmsLogString(fmt.Sprintf("object %v %v %v has unrecognized version expression %v in service %v, error %v", objPol.OrgID, objPol.ObjectID, objPol.ObjectType, serviceID.Version, serviceID.ServiceName, err)))
				continue
			}

			// If one of this object's services has not been seen before, add it to the map.
			if _, ok := m.orgMap[objPol.OrgID][serviceMapKey]; !ok {
				entry := m.NewMMSObjectPolicyEntry(&objPol, serviceID, versionExp)
				entryArray := make([]MMSObjectPolicyEntry, 0, 2)
				entryArray = append(entryArray, *entry)
				m.orgMap[objPol.OrgID][serviceMapKey] = entryArray
			} else {

				// The object policy references a service that already has at least one entry in the map. The object policy update
				// could be a new policy or an update to one that is already cached.
				found := false
				for ix, existingEntry := range m.orgMap[objPol.OrgID][serviceMapKey] {
					if existingEntry.Policy.OrgID == objPol.OrgID && existingEntry.Policy.ObjectID == objPol.ObjectID && existingEntry.Policy.ObjectType == objPol.ObjectType {
						// Replace the entry.
						policyReplaced = existingEntry.Policy

						// Canonicalize the arch in the policy update's service ID.
						if canonicalArch := m.config.ArchSynonyms.GetCanonicalArch(serviceID.Arch); canonicalArch != "" {
							serviceID.Arch = canonicalArch
						}

						m.orgMap[objPol.OrgID][serviceMapKey][ix].UpdateEntry(&objPol, serviceID, versionExp)
						found = true
						break
					}
				}
				// For the current service in the updated policy object, create a new entry and add it to the map.
				if !found {
					entry := m.NewMMSObjectPolicyEntry(&objPol, serviceID, versionExp)
					m.orgMap[objPol.OrgID][serviceMapKey] = append(m.orgMap[objPol.OrgID][serviceMapKey], *entry)
				}
			}

		}

		// Create an event to tell the other workers that a model policy has changed.
		var ev events.Message
		if !foundService || policyReplaced.OrgID != "" {
			ev = events.NewMMSObjectPolicyMessage(events.OBJECT_POLICY_CHANGED, objPol, policyReplaced)
		} else {
			ev = events.NewMMSObjectPolicyMessage(events.OBJECT_POLICY_NEW, objPol, nil)
		}
		changeEvents = append(changeEvents, ev)

	}

	glog.V(3).Infof(mmsLogString(m.ShowOrgMapUnlocked()))
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
	Policy            exchange.ObjectDestinationPolicy    `json:"policy,omitempty"`      // the metadata for this object policy in the MMS
	ServiceID         common.ServiceID                    `json:"service,omitempty"`     // the service id for which we created this entry
	VersionExpression *semanticversion.Version_Expression `json:"version,omitempty"`     // the service version expression
	Updated           uint64                              `json:"updatedTime,omitempty"` // the time when this entry was updated
}

// Create a new MMSObjectPolicyEntry. It converts the businesspolicy to internal policy format.
// the business policy exchange id (or/id) is the header name for the internal generated policy.
func (m *MMSObjectPolicyManager) NewMMSObjectPolicyEntry(pol *exchange.ObjectDestinationPolicy, serviceID common.ServiceID, ve *semanticversion.Version_Expression) *MMSObjectPolicyEntry {
	pE := new(MMSObjectPolicyEntry)
	pE.Updated = uint64(time.Now().Unix())
	pE.Policy = (*pol)
	pE.VersionExpression = ve
	if canonicalArch := m.config.ArchSynonyms.GetCanonicalArch(serviceID.Arch); canonicalArch != "" {
		serviceID.Arch = canonicalArch
	}
	pE.ServiceID = serviceID
	return pE
}

func (p *MMSObjectPolicyEntry) String() string {
	return fmt.Sprintf("MMSObjectPolicyEntry: "+
		"Updated: %v "+
		"Policy: %v "+
		"ServiceID: %v "+
		"Version Exp: %v",
		p.Updated, p.Policy, p.ServiceID, p.VersionExpression)
}

func (p *MMSObjectPolicyEntry) ShortString() string {
	return fmt.Sprintf("Policy: %v", p.Policy)
}

func (p *MMSObjectPolicyEntry) UpdateEntry(pol *exchange.ObjectDestinationPolicy, serviceID common.ServiceID, ve *semanticversion.Version_Expression) (*MMSObjectPolicyEntry, error) {
	p.Updated = uint64(time.Now().Unix())
	p.Policy = (*pol)
	p.VersionExpression = ve
	p.ServiceID = serviceID
	return p, nil
}

// =============================================================================================================
var mmsLogString = func(v interface{}) string {
	return fmt.Sprintf("MMS Object Policy Manager: %v", v)
}
