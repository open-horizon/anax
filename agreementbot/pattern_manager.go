package agreementbot

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/policy"
	"golang.org/x/crypto/sha3"
	"sync"
	"time"
)

type PatternEntry struct {
	Pattern         *exchange.Pattern `json:"pattern,omitempty"`         // the metadata for this pattern from the exchange
	Updated         uint64            `json:"updatedTime,omitempty"`     // the time when this entry was updated
	Hash            []byte            `json:"hash,omitempty"`            // a hash of the current entry to compare for matadata changes in the exchange
	PolicyFileNames []string          `json:"policyFileNames,omitempty"` // the list of policy names generated for this pattern
}

func (p *PatternEntry) String() string {
	return fmt.Sprintf("Pattern Entry: "+
		"Updated: %v "+
		"Hash: %x "+
		"Files: %v"+
		"Pattern: %v",
		p.Updated, p.Hash, p.PolicyFileNames, p.Pattern)
}

func (p *PatternEntry) ShortString() string {
	return fmt.Sprintf("Files: %v", p.PolicyFileNames)
}

// return a pointer to a copy of PatternEntry
func (p *PatternEntry) DeepCopy() *PatternEntry {
	newEntry := PatternEntry{Updated: p.Updated}

	if p.Pattern != nil {
		newEntry.Pattern = p.Pattern.DeepCopy()
	}

	if p.Hash != nil {
		newHash := make([]byte, len(p.Hash))
		copy(newHash, p.Hash)
		newEntry.Hash = newHash
	}

	if p.PolicyFileNames != nil {
		newPolicyFileNames := make([]string, len(p.PolicyFileNames))
		copy(newPolicyFileNames, p.PolicyFileNames)
		newEntry.PolicyFileNames = newPolicyFileNames
	}

	return &newEntry

}

func hashPattern(p *exchange.Pattern) ([]byte, error) {
	if ps, err := json.Marshal(p); err != nil {
		return nil, errors.New(fmt.Sprintf("unable to marshal pattern %v to a string, error %v", p, err))
	} else {
		hash := sha3.Sum256([]byte(ps))
		return hash[:], nil
	}
}

func NewPatternEntry(p *exchange.Pattern) (*PatternEntry, error) {
	pe := new(PatternEntry)
	pe.Pattern = p
	pe.Updated = uint64(time.Now().Unix())
	if hash, err := hashPattern(p); err != nil {
		return nil, err
	} else {
		pe.Hash = hash
	}
	pe.PolicyFileNames = make([]string, 0, 10)
	return pe, nil
}

func (pe *PatternEntry) AddPolicyFileName(fileName string) {
	pe.PolicyFileNames = append(pe.PolicyFileNames, fileName)
}

func (pe *PatternEntry) DeleteAllPolicyFiles(policyPath string, org string) error {

	for _, fileName := range pe.PolicyFileNames {
		if err := policy.DeletePolicyFile(fileName); err != nil {
			return err
		}
	}
	return nil
}

func (pe *PatternEntry) UpdateEntry(pattern *exchange.Pattern, newHash []byte) {
	pe.Pattern = pattern
	pe.Hash = newHash
	pe.Updated = uint64(time.Now().Unix())
	pe.PolicyFileNames = make([]string, 0, 10)
}

type PatternManager struct {
	spMapLock      sync.Mutex                          // The lock that protects the map of ServedPatterns because it is referenced from another thread.
	patMapLock     sync.Mutex                          // The lock that protects the map of PatternEntry because it is referenced from another thread.
	ServedPatterns map[string]exchange.ServedPattern   // served node org, pattern org and pattern triplets
	OrgPatterns    map[string]map[string]*PatternEntry // all served paterns by this agbot
}

func (pm *PatternManager) String() string {
	pm.patMapLock.Lock()
	defer pm.patMapLock.Unlock()

	res := "Pattern Manager: "
	for org, orgMap := range pm.OrgPatterns {
		res += fmt.Sprintf("Org: %v ", org)
		for pat, pe := range orgMap {
			res += fmt.Sprintf("Pattern: %v %v ", pat, pe)
		}
	}

	pm.spMapLock.Lock()
	defer pm.spMapLock.Unlock()

	for _, served := range pm.ServedPatterns {
		res += fmt.Sprintf(" Serve: %v ", served)
	}
	return res
}

func (p *PatternManager) ShortString() string {
	p.patMapLock.Lock()
	defer p.patMapLock.Unlock()

	res := "Pattern Manager: "
	for org, orgMap := range p.OrgPatterns {
		res += fmt.Sprintf("Org: %v ", org)
		for pat, pe := range orgMap {
			s := ""
			if pe != nil {
				s = pe.ShortString()
			}
			res += fmt.Sprintf("Pattern: %v %v ", pat, s)
		}
	}
	return res
}

func NewPatternManager() *PatternManager {
	pm := &PatternManager{
		OrgPatterns: make(map[string]map[string]*PatternEntry),
	}
	return pm
}

func (pm *PatternManager) hasOrg(org string) bool {
	if _, ok := pm.OrgPatterns[org]; ok {
		return true
	}
	return false
}

func (pm *PatternManager) hasPattern(org string, pattern string) bool {
	if pm.hasOrg(org) {
		if _, ok := pm.OrgPatterns[org][pattern]; ok {
			return true
		}
	}
	return false
}

// copy the given map of served patterns
func (pm *PatternManager) setServedPatterns(servedPatterns map[string]exchange.ServedPattern) {
	pm.spMapLock.Lock()
	defer pm.spMapLock.Unlock()

	// copy the input map
	pm.ServedPatterns = servedPatterns
}

// chek if the agbot serves the given pattern or not.
func (pm *PatternManager) servePattern(pattern_org string, pattern string) bool {
	pm.spMapLock.Lock()
	defer pm.spMapLock.Unlock()

	for _, sp := range pm.ServedPatterns {
		if sp.PatternOrg == pattern_org && (sp.Pattern == pattern || sp.Pattern == "*") {
			return true
		}
	}
	return false
}

// check if the agbot service the given org or not.
func (pm *PatternManager) serveOrg(pattern_org string) bool {
	pm.spMapLock.Lock()
	defer pm.spMapLock.Unlock()

	for _, sp := range pm.ServedPatterns {
		if sp.PatternOrg == pattern_org {
			return true
		}
	}
	return false
}

// return an array of node orgs for the given served pattern org and pattern.
// this function is called from a different thread.
func (pm *PatternManager) GetServedNodeOrgs(pattten_org string, pattern string) []string {
	pm.spMapLock.Lock()
	defer pm.spMapLock.Unlock()

	node_orgs := []string{}
	for _, sp := range pm.ServedPatterns {
		if sp.PatternOrg == pattten_org && (sp.Pattern == pattern || sp.Pattern == "*") {
			node_org := sp.NodeOrg
			// the default node org is the pattern org
			if node_org == "" {
				node_org = sp.PatternOrg
			}
			node_orgs = append(node_orgs, node_org)
		}
	}
	return node_orgs
}

func (pm *PatternManager) GetAllPatternOrgs() []string {
	pm.spMapLock.Lock()
	defer pm.spMapLock.Unlock()

	orgs := make([]string, 0)
	for _, sp := range pm.ServedPatterns {
		orgs = append(orgs, sp.PatternOrg)
	}
	return orgs
}

// Getters for PatternManager
// return a copy of the ServedPatterns field
func (pm *PatternManager) GetServedPatterns() map[string]exchange.ServedPattern {
	pm.spMapLock.Lock()
	defer pm.spMapLock.Unlock()

	copyServedPat := make(map[string]exchange.ServedPattern)
	for key, sp := range pm.ServedPatterns {
		copyServedPat[key] = sp
	}

	return copyServedPat
}

// return a copy of the OrgPatterns field
func (pm *PatternManager) GetOrgPatterns() map[string]map[string]*PatternEntry {
	pm.patMapLock.Lock()
	defer pm.patMapLock.Unlock()

	copyOrgPat := make(map[string]map[string]*PatternEntry)

	for org, _ := range pm.OrgPatterns {
		OrgPatNames := make(map[string]*PatternEntry)
		for n, patEntry := range pm.OrgPatterns[org] {
			OrgPatNames[n] = patEntry.DeepCopy()
		}
		copyOrgPat[org] = OrgPatNames
	}

	return copyOrgPat
}

// Given a list of pattern_org/pattern/node_org triplets that this agbot is supposed to serve, save that list and
// convert it to map of maps (keyed by org and pattern name) to hold all the pattern meta data. This
// will allow the PatternManager to know when the pattern metadata changes.
func (pm *PatternManager) SetCurrentPatterns(servedPatterns map[string]exchange.ServedPattern, policyPath string) error {
	pm.patMapLock.Lock()
	defer pm.patMapLock.Unlock()

	// Exit early if nothing to do
	if len(pm.OrgPatterns) == 0 && len(pm.ServedPatterns) == 0 && len(servedPatterns) == 0 {
		return nil
	}

	// save the served patterns in the pm
	pm.setServedPatterns(servedPatterns)

	// Create a new map of maps
	if len(pm.OrgPatterns) == 0 {
		pm.OrgPatterns = make(map[string]map[string]*PatternEntry)
	}

	// For each org that this agbot is supposed to be serving, check if it is already in the pm.
	// If not add to it. The patterns will be added later in the UpdatePatternPolicies function.
	for _, served := range servedPatterns {
		// If we have encountered a new org in the served pattern list, create a map of patterns for it.
		if !pm.hasOrg(served.PatternOrg) {
			pm.OrgPatterns[served.PatternOrg] = make(map[string]*PatternEntry)
		}
	}

	// For each org in the existing PatternManager, check to see if its in the new map. If not, then
	// this agbot is no longer serving any patterns in that org, we can get rid of everything in that org.
	for org, _ := range pm.OrgPatterns {
		if !pm.serveOrg(org) {
			// delete org and all policy files in it.
			glog.V(5).Infof("Deleting the org %v from the pattern manager and all its policy files because it is no longer hosted by the agbot.", org)
			if err := pm.deleteOrg(policyPath, org); err != nil {
				return err
			}
		}
	}

	return nil
}

// Create all the policy files for the input pattern
func createPolicyFiles(pe *PatternEntry, patternId string, pattern *exchange.Pattern, policyPath string, org string) error {
	if policies, err := exchange.ConvertToPolicies(patternId, pattern); err != nil {
		return errors.New(fmt.Sprintf("error converting pattern to policies, error %v", err))
	} else {
		for _, pol := range policies {
			if fileName, err := policy.CreatePolicyFile(policyPath, org, pol.Header.Name, pol); err != nil {
				return errors.New(fmt.Sprintf("error creating policy file, error %v", err))
			} else {
				pe.AddPolicyFileName(fileName)
			}
		}
	}
	return nil
}

// For each org that the agbot is supporting, take the set of patterns defined within the org and save them into
// the PatternManager. When new or updated patterns are discovered, generate policy files for each pattern so that
// the agbot can start serving the workloads and services.
func (pm *PatternManager) UpdatePatternPolicies(org string, definedPatterns map[string]exchange.Pattern, policyPath string) error {
	pm.patMapLock.Lock()
	defer pm.patMapLock.Unlock()

	// If there is no pattern in the org, delete the pattern entries for the org from the pm and all of the policy files in the org.
	// This is the case where pattern or the org has been deleted but the agbot still hosts the pattern on the exchange.
	if definedPatterns == nil || len(definedPatterns) == 0 {
		glog.V(5).Infof("Clear pattern entries and deleting all policy files from org %v.", org)
		// remove all the pattern entries from the pattern manager
		if pm.hasOrg(org) && len(pm.OrgPatterns[org]) > 0 {
			pm.OrgPatterns[org] = make(map[string]*PatternEntry)
		}
		// delete all policy files from the org
		return policy.DeletePolicyFilesForOrg(policyPath, org, true)
	}

	// just in case the entry for the given org is not created yet
	if !pm.hasOrg(org) {
		pm.OrgPatterns[org] = make(map[string]*PatternEntry)
	}

	// Delete the pattern from the pm and all of its policy files if the pattern does not exist on the exchange or the agbot
	// does not serve it any more.
	for pattern, _ := range pm.OrgPatterns[org] {
		need_delete := true
		if pm.servePattern(org, pattern) {
			for patternId, _ := range definedPatterns {
				if exchange.GetId(patternId) == pattern {
					need_delete = false
					break
				}
			}
		}

		if need_delete {
			glog.V(5).Infof("Deleting pattern %v and its policy files from the org %v from the pattern manager because the pattern no longer exists.", pattern, org)
			if err := pm.deletePattern(policyPath, org, pattern); err != nil {
				return err
			}
		}
	}

	// Now we just need to handle adding new patterns or update existing patterns
	for patternId, pattern := range definedPatterns {
		if !pm.servePattern(org, exchange.GetId(patternId)) {
			continue
		}

		need_new_entry := true
		if pm.hasPattern(org, exchange.GetId(patternId)) {
			if pe := pm.OrgPatterns[org][exchange.GetId(patternId)]; pe != nil {
				need_new_entry = false

				// The PatternEntry is already there, so check if the pattern definition has changed.
				// If the pattern has changed, recreate all policy files. Otherwise the pattern
				// definition we have is current.
				newHash, err := hashPattern(&pattern)
				if err != nil {
					return errors.New(fmt.Sprintf("unable to hash pattern %v for %v, error %v", pattern, org, err))
				}
				if !bytes.Equal(pe.Hash, newHash) {
					glog.V(5).Infof("Deleting all the policy files for org %v because the old pattern %v does not match the new pattern %v", org, pe.Pattern, pattern)
					if err := pe.DeleteAllPolicyFiles(policyPath, org); err != nil {
						return errors.New(fmt.Sprintf("unable to delete policy files for %v, error %v", org, err))
					}
					pe.UpdateEntry(&pattern, newHash)
					glog.V(5).Infof("Creating the policy files for pattern %v.", patternId)
					if err := createPolicyFiles(pe, patternId, &pattern, policyPath, org); err != nil {
						return errors.New(fmt.Sprintf("unable to create policy files for %v, error %v", pattern, err))
					}
				}
			}
		}

		//If there's no PatternEntry yet, create one and then create the policy files.
		if need_new_entry {
			if newPE, err := NewPatternEntry(&pattern); err != nil {
				return errors.New(fmt.Sprintf("unable to create pattern entry for %v, error %v", pattern, err))
			} else {
				pm.OrgPatterns[org][exchange.GetId(patternId)] = newPE
				glog.V(5).Infof("Creating the policy files for pattern %v.", patternId)
				if err := createPolicyFiles(newPE, patternId, &pattern, policyPath, org); err != nil {
					return errors.New(fmt.Sprintf("unable to create policy files for %v, error %v", pattern, err))
				}
			}
		}
	}

	return nil
}

// When an org is removed from the list of supported orgs and patterns, remove the org
// from the PatternManager and delete all the policy files for it.
func (pm *PatternManager) deleteOrg(policyPath string, org string) error {
	// Delete all the policy files that are pattern based for the org
	if err := policy.DeletePolicyFilesForOrg(policyPath, org, true); err != nil {
		glog.Errorf("Error deleting policy files for org %v. %v", org, err)
	}

	// Get rid of the org map
	if pm.hasOrg(org) {
		delete(pm.OrgPatterns, org)
	}

	return nil
}

// When a pattern is removed, remove the pattern from the PatternManager and delete all the policy files for it.
func (pm *PatternManager) deletePattern(policyPath string, org string, pattern string) error {
	// delete the policy files
	if err := policy.DeletePolicyFilesForPattern(policyPath, org, pattern); err != nil {
		glog.Errorf("Error deleting policy files for pattern %v/%v. %v", org, pattern, err)
	}

	// Get rid of the pattern from the pm
	if pm.hasOrg(org) {
		if _, ok := pm.OrgPatterns[org][pattern]; ok {
			delete(pm.OrgPatterns[org], pattern)
		}
	}

	return nil
}
