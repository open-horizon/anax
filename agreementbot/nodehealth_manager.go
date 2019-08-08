package agreementbot

import (
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/agreementbot/persistence"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"time"
)

// The Node Health Pattern manager's job is to maintain a cache of node and agreement status as seen
// by the exchange. The info in the cache is used to determine if an agreement is out of policy
// because its node is misbehaving or because the agreement itself was removed from the exchange.
//
// The Node Health Pattern manager maintains a 2 level map of patterns (including no pattern == "")
// which is a map of nodes, that contains the node status info. Node info is refreshed from the exchange
// in pattern groups. That is, all the nodes using a given pattern are updated in the cache with one
// call to the exchange. An agbot will only obtain node status info for patterns which are used
// by agreements that it is managing.

type NodeHealthHandler func(pattern string, org string, nodeOrgs []string, lastCallTime string) (*exchange.NodeHealthStatus, error)

type NHPatternEntry struct {
	Nodes        *exchange.NodeHealthStatus // The node info from the exchange
	Updated      bool                       // Indicates whether or not the node info has been updated from the exchange. This field is set to false after governance iterates all agreements.
	LastCallTime string                     // The last time the exchange was called to obtain status
}

func (n *NHPatternEntry) String() string {
	return fmt.Sprintf("NH Pattern Entry Updated: %v, "+
		"LastCallTime: %v, "+
		"Nodes: %v",
		n.Updated, n.LastCallTime, n.Nodes)
}

func NewNHPatternEntry() *NHPatternEntry {
	nh := &NHPatternEntry{
		Nodes:        nil,
		Updated:      false,
		LastCallTime: "",
	}
	return nh
}

type NodeHealthManager struct {
	Patterns map[string]*NHPatternEntry // A map of patterns for which this agbot has agreements
	NodeOrgs map[string][]string        // a map of node orgs for each pattern used by current active agreements
}

func (n *NodeHealthManager) String() string {
	return fmt.Sprintf("Patterns: %v",
		n.Patterns)
}

func NewNodeHealthManager() *NodeHealthManager {
	nh := &NodeHealthManager{
		Patterns: make(map[string]*NHPatternEntry),
	}
	return nh
}

// Make sure the manager has the latest status info from the exchange.
func (m *NodeHealthManager) SetUpdatedStatus(pattern string, org string, nhHandler NodeHealthHandler) error {

	updatedAsOfNow := cutil.FormattedTime()
	if lastCallTime, isUpdated := m.hasUpdatedStatus(pattern, org); !isUpdated {
		if nhs, err := m.getNewStatus(pattern, org, lastCallTime, nhHandler); err != nil {
			return errors.New(fmt.Sprintf("unable to get updated node health for %v, error %v", pattern, err))
		} else {
			m.setNewStatus(pattern, org, updatedAsOfNow, nhs)
		}
	}
	return nil
}

// Clear the Updated flag in each pattern entry so that future requests for status will first go the
// exchange to get any updates.
func (m *NodeHealthManager) ResetUpdateStatus() {
	for _, pe := range m.Patterns {
		pe.Updated = false
	}
}

// Determine if the input node's heartbeat is overdue, i.e. beyond the policy interval. Return false (not
// out of policy) if the agrement is still present.
func (m *NodeHealthManager) NodeOutOfPolicy(pattern string, org string, deviceId string, interval int) bool {

	key := getKey(pattern, org)
	if pe, ok := m.Patterns[key]; !ok {
		return true
	} else if node, ok := pe.Nodes.Nodes[deviceId]; !ok {
		return true
	} else {
		lastHB := uint64(cutil.TimeInSeconds(node.LastHeartbeat, cutil.ExchangeTimeFormat))
		now := uint64(time.Now().Unix())
		if (lastHB < now) && ((now - lastHB) >= uint64(interval)) {
			return true
		}
	}

	return false
}

// Determine if the input agreement id is still present in the exchange. Return false (not out of policy)
// if the agreement is present. If the agreement is not present then give the node NHCheckAgreementStatus + agbot finalized time
// to get the agreement object into the exchange.
func (m *NodeHealthManager) AgreementOutOfPolicy(pattern string, org string, deviceId string, agreementId string, start uint64, interval int) bool {

	key := getKey(pattern, org)
	if pe, ok := m.Patterns[key]; !ok {
		return true
	} else if node, ok := pe.Nodes.Nodes[deviceId]; !ok {
		return true
	} else if _, ok := node.Agreements[agreementId]; ok {
		return false
	} else {
		// The agreement is out of policy if the agent has not set its agreement obj into the exchange within NHCheckAgreementStatus (interval)
		// seconds of the time when the agbot finalized the agreement (which occurs when it receives the positive proposal reply).
		now := uint64(time.Now().Unix())
		limit := start + uint64(interval)
		if now > limit {
			return true
		}
	}

	return false
}

// The manager has updated status if the pattern entry exists and has the Updated flag turned on.
func (m *NodeHealthManager) hasUpdatedStatus(pattern string, org string) (string, bool) {

	key := getKey(pattern, org)
	if pe, ok := m.Patterns[key]; !ok {
		return "", false
	} else {
		return pe.LastCallTime, pe.Updated
	}
}

// Assume the caller has called hasUpdatedStatus and knows they definitely want to call the exchange.
func (m *NodeHealthManager) getNewStatus(pattern string, org string, lastCall string, nhHandler NodeHealthHandler) (*exchange.NodeHealthStatus, error) {
	//get the node orgs for the pattern
	patternKey := getKey(pattern, org)
	nodeOrgs := []string{}
	if v, found := m.NodeOrgs[patternKey]; found {
		nodeOrgs = v
	}
	glog.V(5).Infof("Node Health Manager: node orgs for pattern %v or org %v: %v", pattern, org, nodeOrgs)

	return nhHandler(pattern, org, nodeOrgs, lastCall)
}

// Update the manager with the new node status.
func (m *NodeHealthManager) setNewStatus(pattern string, org string, lastCall string, nhs *exchange.NodeHealthStatus) {

	key := getKey(pattern, org)

	// Create the pattern entry if needed
	pe, ok := m.Patterns[key]
	if !ok {
		pe = NewNHPatternEntry()
		m.Patterns[key] = pe
	}

	// Save cache update metadata
	pe.LastCallTime = lastCall
	pe.Updated = true

	// Cache new status if there is any
	if nhs != nil {
		glog.V(5).Infof("Node Health Manager: consuming new status: %v", nhs)
		if pe.Nodes == nil {
			pe.Nodes = nhs
		} else {
			for nodeid, status := range nhs.Nodes {
				pe.Nodes.Nodes[nodeid] = status
			}
		}
	}

}

// set the node orgs for patterns for current active agreements under the given agreement protocol
func (m *NodeHealthManager) SetNodeOrgs(agreements []persistence.Agreement, agreementProtocol string) {

	tmpNodeOrgs := map[string][]string{}

	for _, ag := range agreements {
		patternKey := getKey(ag.Pattern, ag.Org)
		nodeOrg := exchange.GetOrg(ag.DeviceId)
		if nodeOrgs, ok := tmpNodeOrgs[patternKey]; !ok {
			tmpNodeOrgs[patternKey] = []string{nodeOrg}
		} else {
			if !stringSliceContains(nodeOrgs, nodeOrg) {
				nodeOrgs = append(nodeOrgs, nodeOrg)
				tmpNodeOrgs[patternKey] = nodeOrgs
			}
		}
	}

	m.NodeOrgs = tmpNodeOrgs
}

// check if a slice contains a string
func stringSliceContains(a []string, s string) bool {
	for _, v := range a {
		if s == v {
			return true
		}
	}
	return false
}

func getKey(pattern string, org string) string {
	if pattern != "" {
		return pattern
	} else {
		return org
	}
}
