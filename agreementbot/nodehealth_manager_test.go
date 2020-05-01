// +build unit

package agreementbot

import (
	"flag"
	"github.com/open-horizon/anax/agreementbot/persistence"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/policy"
	"testing"
	"time"
)

func init() {
	flag.Set("alsologtostderr", "true")
	flag.Set("v", "7")
	// no need to parse flags, that's done by test framework
}

func Test_NodeHealthStatus_constructor(t *testing.T) {
	nhm := NewNodeHealthManager()
	if nhm == nil {
		t.Errorf("constructor should return non-nil object")
	}

	nhpe := NewNHPatternEntry()
	if nhpe == nil {
		t.Errorf("constructor should return non-nil object")
	}

}

func Test_NodeHealthStatus_firstpass(t *testing.T) {

	nhm := NewNodeHealthManager()
	if len(nhm.Patterns) != 0 {
		t.Errorf("patterns map should be empty")
	}

	nhm.ResetUpdateStatus()

	mypattern := "mypattern"
	mynode := "org/node1"
	agid := "ag1"
	lastHB := "2006-01-02T15:04:05.999Z[UTC]"

	nhHandler := getVariableStatusHandler(mynode, agid, []string{}, lastHB)
	err := nhm.SetUpdatedStatus(mypattern, "theorg", nhHandler)

	if err != nil {
		t.Errorf("unexpected error %v", err)
	} else if len(nhm.Patterns) != 1 {
		t.Errorf("patterns map should have something in it, is %v", nhm)
	} else if op := nhm.NodeOutOfPolicy(mypattern, "theorg", mynode, 300); !op {
		t.Errorf("node %v was not detected as out of policy %v", mynode, nhm)
	} else if op := nhm.AgreementOutOfPolicy(mypattern, "theorg", mynode, agid, uint64(time.Now().Unix()), 10); op {
		t.Errorf("agreement %v was detected as out of policy, %v", agid, nhm)
	}

	badagid := "ag2"
	if op := nhm.AgreementOutOfPolicy(mypattern, "theorg", mynode, badagid, uint64(time.Now().Unix()-100), 10); !op {
		t.Errorf("agreement %v was not detected as out of policy, %v", agid, nhm)
	} else if op := nhm.AgreementOutOfPolicy(mypattern, "theorg", mynode, badagid, uint64(time.Now().Unix()), 10); op {
		t.Errorf("agreement %v was note detected as out of policy, %v", agid, nhm)
	}

	badnode := "org/node2"
	if op := nhm.NodeOutOfPolicy(mypattern, "theorg", badnode, 300); !op {
		t.Errorf("node %v was not detected as out of policy %v", mynode, nhm)
	}

	if op := nhm.AgreementOutOfPolicy(mypattern, "theorg", badnode, agid, uint64(time.Now().Unix()-100), 10); !op {
		t.Errorf("agreement %v was not detected as out of policy, %v", agid, nhm)
	}

}

func Test_NodeHealthStatus_recentnode(t *testing.T) {

	nhm := NewNodeHealthManager()
	if len(nhm.Patterns) != 0 {
		t.Errorf("patterns map should be empty")
	}

	nhm.ResetUpdateStatus()

	mypattern := "mypattern"
	mynode := "org/node1"
	agid := "ag1"
	lastHB := cutil.FormattedTime() // Now in the exchange string format

	nhHandler := getVariableStatusHandler(mynode, agid, []string{"theorg", "theorg1"}, lastHB)
	err := nhm.SetUpdatedStatus(mypattern, "theorg", nhHandler)

	if err != nil {
		t.Errorf("unexpected error %v", err)
	} else if len(nhm.Patterns) != 1 {
		t.Errorf("patterns map should have something in it, is %v", nhm)
	} else if op := nhm.NodeOutOfPolicy(mypattern, "theorg", mynode, 300); op {
		t.Errorf("node %v was detected as out of policy %v", mynode, nhm)
	} else if op := nhm.AgreementOutOfPolicy(mypattern, "theorg", mynode, agid, uint64(time.Now().Unix()), 10); op {
		t.Errorf("agreement %v was detected as out of policy, %v", agid, nhm)
	}

}

func Test_NodeHealthStatus_recentnode_org(t *testing.T) {

	nhm := NewNodeHealthManager()
	if len(nhm.Patterns) != 0 {
		t.Errorf("patterns map should be empty")
	}

	nhm.ResetUpdateStatus()

	mypattern := ""
	myorg := "org"
	mynode := "org1/node1"
	agid := "ag1"
	lastHB := cutil.FormattedTime() // Now in the exchange string format

	nhHandler := getVariableStatusHandler(mynode, agid, []string{"org", "org2"}, lastHB)
	err := nhm.SetUpdatedStatus(mypattern, myorg, nhHandler)

	if err != nil {
		t.Errorf("unexpected error %v", err)
	} else if len(nhm.Patterns) != 1 {
		t.Errorf("patterns map should have something in it, is %v", nhm)
	} else if op := nhm.NodeOutOfPolicy(mypattern, myorg, mynode, 300); op {
		t.Errorf("node %v was detected as out of policy %v", mynode, nhm)
	} else if op := nhm.AgreementOutOfPolicy(mypattern, myorg, mynode, agid, uint64(time.Now().Unix()), 10); op {
		t.Errorf("agreement %v was detected as out of policy, %v", agid, nhm)
	}

	mypattern = "mypattern"
	myorg = ""
	mynode = "org2/node2"
	agid = "ag2"
	lastHB = cutil.FormattedTime() // Now in the exchange string format

	nhHandler = getVariableStatusHandler(mynode, agid, []string{"org", "org2"}, lastHB)
	err = nhm.SetUpdatedStatus(mypattern, myorg, nhHandler)

	if err != nil {
		t.Errorf("unexpected error %v", err)
	} else if len(nhm.Patterns) != 2 {
		t.Errorf("patterns map should have something in it, is %v", nhm)
	} else if op := nhm.NodeOutOfPolicy(mypattern, myorg, mynode, 300); op {
		t.Errorf("node %v was detected as out of policy %v", mynode, nhm)
	} else if op := nhm.AgreementOutOfPolicy(mypattern, myorg, mynode, agid, uint64(time.Now().Unix()), 10); op {
		t.Errorf("agreement %v was detected as out of policy, %v", agid, nhm)
	}

	t.Log(nhm)

}

func Test_NodeHealthStatus_multiupdate(t *testing.T) {

	nhm := NewNodeHealthManager()
	if len(nhm.Patterns) != 0 {
		t.Errorf("patterns map should be empty")
	}

	nhm.ResetUpdateStatus()

	mypattern := "mypattern"
	myorg := "org"
	mynode := "org1/node1"
	mynode2 := "org2/node2"
	agid := "ag1"
	agid2 := "ag2"
	lastHB := cutil.FormattedTime() // Now in the exchange string format

	nhHandler := getVariableStatusHandler(mynode, agid, []string{"org1", "org2"}, lastHB)
	err := nhm.SetUpdatedStatus(mypattern, myorg, nhHandler)

	if err != nil {
		t.Errorf("unexpected error %v", err)
	} else if len(nhm.Patterns) != 1 {
		t.Errorf("patterns map should have something in it, is %v", nhm)
	} else if op := nhm.NodeOutOfPolicy(mypattern, myorg, mynode, 300); op {
		t.Errorf("node %v was detected as out of policy %v", mynode, nhm)
	} else if op := nhm.AgreementOutOfPolicy(mypattern, myorg, mynode, agid, uint64(time.Now().Unix()), 10); op {
		t.Errorf("agreement %v was detected as out of policy, %v", agid, nhm)
	}

	// Reset update state and get a non-nil update from the status handler, same pattern.
	nhm.ResetUpdateStatus()
	if lastCall, isUpdated := nhm.hasUpdatedStatus(mypattern, myorg); isUpdated {
		t.Errorf("unexpected updated state %v", nhm)
	} else if lastCall == "" {
		t.Errorf("expected a last call string %v", nhm)
	}

	nhHandler = getVariableStatusHandler(mynode2, agid2, []string{"org1", "org2"}, lastHB)
	err = nhm.SetUpdatedStatus(mypattern, myorg, nhHandler)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	} else if len(nhm.Patterns) != 1 {
		t.Errorf("patterns map should have something in it, is %v", nhm)
	} else if op := nhm.NodeOutOfPolicy(mypattern, myorg, mynode2, 300); op {
		t.Errorf("node %v was detected as out of policy %v", mynode2, nhm)
	} else if op := nhm.AgreementOutOfPolicy(mypattern, myorg, mynode2, agid2, uint64(time.Now().Unix()), 10); op {
		t.Errorf("agreement %v was detected as out of policy, %v", agid2, nhm)
	} else if len(nhm.Patterns[mypattern].Nodes.Nodes) != 2 {
		t.Errorf("lost a node somewhere, should be 2, is %v", nhm)
	}

	// Reset update state and get a nil update from the status handler.
	nhHandler = getNilUpdateStatusHandler(mynode, agid, []string{}, lastHB)
	nhm.ResetUpdateStatus()
	err = nhm.SetUpdatedStatus(mypattern, myorg, nhHandler)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	} else if len(nhm.Patterns) != 1 {
		t.Errorf("patterns map should have something in it, is %v", nhm)
	} else if op := nhm.NodeOutOfPolicy(mypattern, myorg, mynode, 300); op {
		t.Errorf("node %v was detected as out of policy %v", mynode, nhm)
	} else if op := nhm.AgreementOutOfPolicy(mypattern, myorg, mynode, agid, uint64(time.Now().Unix()), 10); op {
		t.Errorf("agreement %v was detected as out of policy, %v", agid, nhm)
	} else if len(nhm.Patterns[mypattern].Nodes.Nodes) != 2 {
		t.Errorf("lost a node somewhere, should be 2, is %v", nhm)
	}

	// Reset update state and get a nil update from the status handler.
	nhHandler = getNoUpdateStatusHandler(mynode, agid, []string{"org1", "org2"}, lastHB)
	nhm.ResetUpdateStatus()
	err = nhm.SetUpdatedStatus(mypattern, myorg, nhHandler)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	} else if len(nhm.Patterns) != 1 {
		t.Errorf("patterns map should have something in it, is %v", nhm)
	} else if op := nhm.NodeOutOfPolicy(mypattern, myorg, mynode, 300); op {
		t.Errorf("node %v was detected as out of policy %v", mynode, nhm)
	} else if op := nhm.AgreementOutOfPolicy(mypattern, myorg, mynode, agid, uint64(time.Now().Unix()), 10); op {
		t.Errorf("agreement %v was detected as out of policy, %v", agid, nhm)
	} else if len(nhm.Patterns[mypattern].Nodes.Nodes) != 2 {
		t.Errorf("lost a node somewhere, should be 2, is %v", nhm)
	}

}

func Test_SetNodeOrgs(t *testing.T) {
	nhm := NewNodeHealthManager()
	if len(nhm.Patterns) != 0 {
		t.Errorf("patterns map should be empty")
	}

	ag1, _ := persistence.NewAgreement("agreement_id1", "pattern_org1", "node_org1/device1", "device", "", "", "", "", "basic", "pattern_org1/sall", []string{""}, policy.NodeHealth{})
	ag2, _ := persistence.NewAgreement("agreement_id2", "pattern_org1", "node_org1/device2", "device", "", "", "", "", "basic", "pattern_org1/sall", []string{""}, policy.NodeHealth{})
	ag3, _ := persistence.NewAgreement("agreement_id3", "pattern_org1", "node_org2/device1", "device", "", "", "", "", "basic", "pattern_org1/sall", []string{""}, policy.NodeHealth{})
	ag4, _ := persistence.NewAgreement("agreement_id4", "pattern_org1", "node_org2/device2", "device", "", "", "", "", "basic", "pattern_org1/sall", []string{""}, policy.NodeHealth{})
	ag5, _ := persistence.NewAgreement("agreement_id5", "pattern_org2", "node_org1/device1", "device", "", "", "", "", "basic", "pattern_org2/sall", []string{""}, policy.NodeHealth{})
	ag6, _ := persistence.NewAgreement("agreement_id6", "pattern_org2", "node_org1/device1", "device", "", "", "", "", "basic", "pattern_org2/sall", []string{""}, policy.NodeHealth{})
	ag7, _ := persistence.NewAgreement("agreement_id7", "pattern_org1", "node_org1/device2", "device", "", "", "", "", "basic", "pattern_org1/netspeed", []string{""}, policy.NodeHealth{})
	ag8, _ := persistence.NewAgreement("agreement_id8", "pattern_org1", "node_org2/device2", "device", "", "", "", "", "basic", "pattern_org1/netspeed", []string{""}, policy.NodeHealth{})
	ag9, _ := persistence.NewAgreement("agreement_id9", "org1", "node_org2/device3", "device", "", "", "", "", "basic", "", []string{""}, policy.NodeHealth{})
	ag10, _ := persistence.NewAgreement("agreement_id10", "org1", "node_org3/device3", "device", "", "", "", "", "basic", "", []string{""}, policy.NodeHealth{})

	agreements := []persistence.Agreement{*ag1, *ag2, *ag3, *ag4, *ag5, *ag6, *ag7, *ag8, *ag9, *ag10}

	nhm.SetNodeOrgs(agreements, "basic")

	patternNodeOrgs := nhm.NodeOrgs

	if len(patternNodeOrgs) != 4 {
		t.Errorf("expected patternNodeOrgs has 4 keys but found %v", len(patternNodeOrgs))
	}

	if nodeOrgs, ok := patternNodeOrgs["pattern_org1/sall"]; !ok {
		t.Errorf("expected pattern_org1/sall found in map but not.")
	} else if len(nodeOrgs) != 2 {
		t.Errorf("expected pattern_org1/sall has 2 node orgs in map but found %v.", nodeOrgs)
	} else if !stringSliceContains(nodeOrgs, "node_org1") {
		t.Errorf("expected org1 has node org called node_org1 in map but not.")
	} else if !stringSliceContains(nodeOrgs, "node_org2") {
		t.Errorf("expected org1 has node org called node_org2 in map but not.")
	}

	if nodeOrgs, ok := patternNodeOrgs["pattern_org2/sall"]; !ok {
		t.Errorf("expected pattern_org2/sall found in map but not.")
	} else if len(nodeOrgs) != 1 {
		t.Errorf("expected pattern_org2/sall has 1 node org in map but found %v.", nodeOrgs)
	} else if nodeOrgs[0] != "node_org1" {
		t.Errorf("expected pattern_org2/sall has 1 node org called node_org1 in map but found %v.", nodeOrgs[0])
	}

	if nodeOrgs, ok := patternNodeOrgs["pattern_org1/netspeed"]; !ok {
		t.Errorf("expected pattern_org1/netspeed found in map but not.")
	} else if len(nodeOrgs) != 2 {
		t.Errorf("expected pattern_org1/netspeed has 2 node orgs in map but found %v.", nodeOrgs)
	} else if !stringSliceContains(nodeOrgs, "node_org1") {
		t.Errorf("expected org1 has node org called node_org1 in map but not.")
	} else if !stringSliceContains(nodeOrgs, "node_org2") {
		t.Errorf("expected org1 has node org called node_org2 in map but not.")
	}

	if nodeOrgs, ok := patternNodeOrgs["org1"]; !ok {
		t.Errorf("expected org1 found in map but not.")
	} else if len(nodeOrgs) != 2 {
		t.Errorf("expected org1 has 2 node orgs in map but found %v.", nodeOrgs)
	} else if !stringSliceContains(nodeOrgs, "node_org2") {
		t.Errorf("expected org1 has node org called node_org2 in map but not.")
	} else if !stringSliceContains(nodeOrgs, "node_org3") {
		t.Errorf("expected org1 has node org called node_org3 in map but not.")
	}

	nhm.SetNodeOrgs([]persistence.Agreement{}, "basic")
	if len(nhm.NodeOrgs) != 0 {
		t.Errorf("expected patternNodeOrgs has 0 keys but found %v", len(nhm.NodeOrgs))
	}
}

func Test_stringSliceContains(t *testing.T) {
	if !stringSliceContains([]string{"s1", "s2", "s3"}, "s1") {
		t.Errorf("expected slice contains s1 but not.")
	} else if !stringSliceContains([]string{"s1", "s2", "s3"}, "s2") {
		t.Errorf("expected slice contains s2 but not.")
	} else if !stringSliceContains([]string{"s1", "s2", "s3"}, "s2") {
		t.Errorf("expected slice contains s2 but not.")
	} else if stringSliceContains([]string{"s1", "s3"}, "s2") {
		t.Errorf("expected slice not contain s2 but it does.")
	} else if stringSliceContains([]string{"s3"}, "s2") {
		t.Errorf("expected slice not contain s3 but it does.")
	}
}

func getVariableStatusHandler(node string, agreementId string, nodeOrgs []string, lastHB string) func(pattern string, org string, nodeOrgs []string, lastCall string) (*exchange.NodeHealthStatus, error) {
	return func(pattern string, org string, nodeOrgs []string, lastCall string) (*exchange.NodeHealthStatus, error) {
		o := &exchange.NodeHealthStatus{
			Nodes: map[string]exchange.NodeInfo{
				node: exchange.NodeInfo{
					LastHeartbeat: lastHB,
					Agreements: map[string]exchange.AgreementObject{
						agreementId: exchange.AgreementObject{},
					},
				},
			},
		}
		return o, nil
	}
}

func getNilUpdateStatusHandler(node string, agreementId string, nodeOrgs []string, lastHB string) func(pattern string, org string, nodeOrgs []string, lastCall string) (*exchange.NodeHealthStatus, error) {
	return func(pattern string, org string, nodeOrgs []string, lastCall string) (*exchange.NodeHealthStatus, error) {
		return nil, nil
	}
}

func getNoUpdateStatusHandler(node string, agreementId string, nodeOrgs []string, lastHB string) func(pattern string, org string, nodeOrgs []string, lastCall string) (*exchange.NodeHealthStatus, error) {
	return func(pattern string, org string, nodeOrgs []string, lastCall string) (*exchange.NodeHealthStatus, error) {
		o := &exchange.NodeHealthStatus{
			Nodes: map[string]exchange.NodeInfo{},
		}
		return o, nil
	}
}
