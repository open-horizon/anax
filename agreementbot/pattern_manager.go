package agreementbot

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/policy"
	"golang.org/x/crypto/sha3"
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
	OrgPatterns map[string]map[string]*PatternEntry
}

func (p *PatternManager) String() string {
	res := "Pattern Manager: "
	for org, orgMap := range p.OrgPatterns {
		res += fmt.Sprintf("Org: %v ", org)
		for pat, pe := range orgMap {
			res += fmt.Sprintf("Pattern: %v %v ", pat, pe)
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

// Given a list of org/pattern pairs that this agbot is supported to be serving, take that list and
// convert it to map of maps (keyed by org and pattern name) to hold all the pattern metadata. This
// will allow the PatternManager to know when the pattern metadata changes.
func (pm *PatternManager) SetCurrentPatterns(servedPatterns *[]exchange.ServedPatterns, policyPath string) error {

	// Exit early if nothing to do
	if len(pm.OrgPatterns) == 0 && len(*servedPatterns) == 0 {
		return nil
	}

	// Create a new map of maps
	newMap := make(map[string]map[string]*PatternEntry)

	// For each org/pattern pair that this agbot is supposed to be serving copy the map entries from the
	// existing map or create new ones as necesssary.
	for _, served := range *servedPatterns {

		// If we have encountered a new org in the served pattern list, create a map of patterns for it.
		if _, ok := newMap[served.Org]; !ok {
			newMap[served.Org] = make(map[string]*PatternEntry)
		}

		// If the org and pattern have an entry in the old map, copy entry to new map. The PatternEntry
		// will be nil for patterns that are newly appearing in the agbot metadata. In that case, the
		// PatternEntry will be created later, once we have the pattern metadata from the exchange.
		if pm.hasPattern(served.Org, served.Pattern) {
			newMap[served.Org][served.Pattern] = pm.OrgPatterns[served.Org][served.Pattern]
		} else {
			newMap[served.Org][served.Pattern] = nil
		}
	}

	// For each org in the existing PatternManager, check to see if its in the new map. If not, then
	// this agbot is no longer serving any patterns in that org, we can get rid of everything in that org.
	// Same goes for a pattern that is no longer present in the new map.
	for org, orgMap := range pm.OrgPatterns {

		// If the org is not in the new map, then we need to get rid of it and all its patterns.
		if _, ok := newMap[org]; !ok {
			// delete org and all policy files in it.
			pm.deleteOrg(policyPath, org)
		} else {

			// If the pattern is not in the org any more, get rid of its policy files.
			for pattern, pe := range orgMap {
				if _, ok := newMap[org][pattern]; !ok && pe != nil {
					// Delete the policy files.
					if err := pe.DeleteAllPolicyFiles(policyPath, org); err != nil {
						return err
					}

				}
			}
		}
	}

	// The new map of patterns is current so save it as the PatternManager's new state.
	pm.OrgPatterns = newMap

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
// the agbot can start serving the workloads.
func (pm *PatternManager) UpdatePatternPolicies(org string, definedPatterns map[string]exchange.Pattern, policyPath string) error {

	// Exit early on error
	if !pm.hasOrg(org) {
		return errors.New(fmt.Sprintf("org %v not found in pattern manager", org))
	}

	// For each defined pattern, update it in the new PatternManager map
	for patternId, pattern := range definedPatterns {
		// If the PatternManager knows about this pattern, then its because this agbot is configured to serve it.
		if pm.hasPattern(org, exchange.GetId(patternId)) {

			// There might not be a PatternEntry for this pattern yet because the pattern might have just been
			// discovered by the query of the agbot config. If there's no PatternEntry yet, create one and then
			// create the policy files.
			if pe := pm.OrgPatterns[org][exchange.GetId(patternId)]; pe == nil {
				if newPE, err := NewPatternEntry(&pattern); err != nil {
					return errors.New(fmt.Sprintf("unable to create pattern entry for %v, error %v", pattern, err))
				} else {
					pm.OrgPatterns[org][exchange.GetId(patternId)] = newPE
					if err := createPolicyFiles(newPE, patternId, &pattern, policyPath, org); err != nil {
						return errors.New(fmt.Sprintf("unable to create policy files for %v, error %v", pattern, err))
					}
				}
			} else {
				// The PatternEntry was already there, so check if the pattern definition has changed.
				// If the pattern has changed, recreate all policy files. Otherwise the pattern
				// definition we have is current.
				newHash, err := hashPattern(&pattern)
				if err != nil {
					return errors.New(fmt.Sprintf("unable to hash pattern %v for %v, error %v", pattern, org, err))
				}
				if !bytes.Equal(pe.Hash, newHash) {
					if err := pe.DeleteAllPolicyFiles(policyPath, org); err != nil {
						return errors.New(fmt.Sprintf("unable to delete policy files for %v, error %v", org, err))
					}
					pe.UpdateEntry(&pattern, newHash)
					if err := createPolicyFiles(pe, patternId, &pattern, policyPath, org); err != nil {
						return errors.New(fmt.Sprintf("unable to create policy files for %v, error %v", pattern, err))
					}
				}
			}
		} else {
			// The PatternManager does not know the pattern therefore the agbot is not configured to serve this pattern.
			// We can safely ignore the pattern.
		}
	}

	return nil
}

// When an org is removed from the list of supported orgs and patterns, remove the org
// from the PatternManager and delete all the policy files for it.
func (pm *PatternManager) deleteOrg(policyPath string, org string) error {

	// Verify that we know about the org
	if !pm.hasOrg(org) {
		return errors.New(fmt.Sprintf("unable to delete org %v, org was not found", org))
	}

	// Delete the policy files for each pattern
	for _, pe := range pm.OrgPatterns[org] {
		if pe != nil {
			if err := pe.DeleteAllPolicyFiles(policyPath, org); err != nil {
				return errors.New(fmt.Sprintf("unable to delete policy files for %v, error %v", org, err))
			}
		}
	}

	// Get rid of the org map
	delete(pm.OrgPatterns, org)

	return nil
}
