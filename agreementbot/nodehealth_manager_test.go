// +build unit

package agreementbot

import (
	"flag"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"testing"
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

	nhHandler := getVariableStatusHandler(mynode, agid, lastHB)
	err := nhm.SetUpdatedStatus(mypattern, "theorg", nhHandler)

	if err != nil {
		t.Errorf("unexpected error %v", err)
	} else if len(nhm.Patterns) != 1 {
		t.Errorf("patterns map should have something in it, is %v", nhm)
	} else if op := nhm.NodeOutOfPolicy(mypattern, "theorg", mynode, 300); !op {
		t.Errorf("node %v was not detected as out of policy %v", mynode, nhm)
	} else if op := nhm.AgreementOutOfPolicy(mypattern, "theorg", mynode, agid); op {
		t.Errorf("agreement %v was detected as out of policy, %v", agid, nhm)
	}

	badagid := "ag2"
	if op := nhm.AgreementOutOfPolicy(mypattern, "theorg", mynode, badagid); !op {
		t.Errorf("agreement %v was not detected as out of policy, %v", agid, nhm)
	}

	badnode := "org/node2"
	if op := nhm.NodeOutOfPolicy(mypattern, "theorg", badnode, 300); !op {
		t.Errorf("node %v was not detected as out of policy %v", mynode, nhm)
	}

	if op := nhm.AgreementOutOfPolicy(mypattern, "theorg", badnode, agid); !op {
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

	nhHandler := getVariableStatusHandler(mynode, agid, lastHB)
	err := nhm.SetUpdatedStatus(mypattern, "theorg", nhHandler)

	if err != nil {
		t.Errorf("unexpected error %v", err)
	} else if len(nhm.Patterns) != 1 {
		t.Errorf("patterns map should have something in it, is %v", nhm)
	} else if op := nhm.NodeOutOfPolicy(mypattern, "theorg", mynode, 300); op {
		t.Errorf("node %v was detected as out of policy %v", mynode, nhm)
	} else if op := nhm.AgreementOutOfPolicy(mypattern, "theorg", mynode, agid); op {
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
	mynode := "org/node1"
	agid := "ag1"
	lastHB := cutil.FormattedTime() // Now in the exchange string format

	nhHandler := getVariableStatusHandler(mynode, agid, lastHB)
	err := nhm.SetUpdatedStatus(mypattern, myorg, nhHandler)

	if err != nil {
		t.Errorf("unexpected error %v", err)
	} else if len(nhm.Patterns) != 1 {
		t.Errorf("patterns map should have something in it, is %v", nhm)
	} else if op := nhm.NodeOutOfPolicy(mypattern, myorg, mynode, 300); op {
		t.Errorf("node %v was detected as out of policy %v", mynode, nhm)
	} else if op := nhm.AgreementOutOfPolicy(mypattern, myorg, mynode, agid); op {
		t.Errorf("agreement %v was detected as out of policy, %v", agid, nhm)
	}

	mypattern = "mypattern"
	myorg = ""
	mynode = "org2/node2"
	agid = "ag2"
	lastHB = cutil.FormattedTime() // Now in the exchange string format

	nhHandler = getVariableStatusHandler(mynode, agid, lastHB)
	err = nhm.SetUpdatedStatus(mypattern, myorg, nhHandler)

	if err != nil {
		t.Errorf("unexpected error %v", err)
	} else if len(nhm.Patterns) != 2 {
		t.Errorf("patterns map should have something in it, is %v", nhm)
	} else if op := nhm.NodeOutOfPolicy(mypattern, myorg, mynode, 300); op {
		t.Errorf("node %v was detected as out of policy %v", mynode, nhm)
	} else if op := nhm.AgreementOutOfPolicy(mypattern, myorg, mynode, agid); op {
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
	mynode := "org/node1"
	mynode2 := "org/node2"
	agid := "ag1"
	agid2 := "ag2"
	lastHB := cutil.FormattedTime() // Now in the exchange string format

	nhHandler := getVariableStatusHandler(mynode, agid, lastHB)
	err := nhm.SetUpdatedStatus(mypattern, myorg, nhHandler)

	if err != nil {
		t.Errorf("unexpected error %v", err)
	} else if len(nhm.Patterns) != 1 {
		t.Errorf("patterns map should have something in it, is %v", nhm)
	} else if op := nhm.NodeOutOfPolicy(mypattern, myorg, mynode, 300); op {
		t.Errorf("node %v was detected as out of policy %v", mynode, nhm)
	} else if op := nhm.AgreementOutOfPolicy(mypattern, myorg, mynode, agid); op {
		t.Errorf("agreement %v was detected as out of policy, %v", agid, nhm)
	}

	// Reset update state and get a non-nil update from the status handler, same pattern.
	nhm.ResetUpdateStatus()
	if lastCall, isUpdated := nhm.hasUpdatedStatus(mypattern, myorg); isUpdated {
		t.Errorf("unexpected updated state %v", nhm)
	} else if lastCall == "" {
		t.Errorf("expected a last call string %v", nhm)
	}

	nhHandler = getVariableStatusHandler(mynode2, agid2, lastHB)
	err = nhm.SetUpdatedStatus(mypattern, myorg, nhHandler)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	} else if len(nhm.Patterns) != 1 {
		t.Errorf("patterns map should have something in it, is %v", nhm)
	} else if op := nhm.NodeOutOfPolicy(mypattern, myorg, mynode2, 300); op {
		t.Errorf("node %v was detected as out of policy %v", mynode2, nhm)
	} else if op := nhm.AgreementOutOfPolicy(mypattern, myorg, mynode2, agid2); op {
		t.Errorf("agreement %v was detected as out of policy, %v", agid2, nhm)
	} else if len(nhm.Patterns[mypattern].Nodes.Nodes) != 2 {
		t.Errorf("lost a node somewhere, should be 2, is %v", nhm)
	}

	// Reset update state and get a nil update from the status handler.
	nhHandler = getNilUpdateStatusHandler(mynode, agid, lastHB)
	nhm.ResetUpdateStatus()
	err = nhm.SetUpdatedStatus(mypattern, myorg, nhHandler)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	} else if len(nhm.Patterns) != 1 {
		t.Errorf("patterns map should have something in it, is %v", nhm)
	} else if op := nhm.NodeOutOfPolicy(mypattern, myorg, mynode, 300); op {
		t.Errorf("node %v was detected as out of policy %v", mynode, nhm)
	} else if op := nhm.AgreementOutOfPolicy(mypattern, myorg, mynode, agid); op {
		t.Errorf("agreement %v was detected as out of policy, %v", agid, nhm)
	} else if len(nhm.Patterns[mypattern].Nodes.Nodes) != 2 {
		t.Errorf("lost a node somewhere, should be 2, is %v", nhm)
	}

	// Reset update state and get a nil update from the status handler.
	nhHandler = getNoUpdateStatusHandler(mynode, agid, lastHB)
	nhm.ResetUpdateStatus()
	err = nhm.SetUpdatedStatus(mypattern, myorg, nhHandler)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	} else if len(nhm.Patterns) != 1 {
		t.Errorf("patterns map should have something in it, is %v", nhm)
	} else if op := nhm.NodeOutOfPolicy(mypattern, myorg, mynode, 300); op {
		t.Errorf("node %v was detected as out of policy %v", mynode, nhm)
	} else if op := nhm.AgreementOutOfPolicy(mypattern, myorg, mynode, agid); op {
		t.Errorf("agreement %v was detected as out of policy, %v", agid, nhm)
	} else if len(nhm.Patterns[mypattern].Nodes.Nodes) != 2 {
		t.Errorf("lost a node somewhere, should be 2, is %v", nhm)
	}

}

func getVariableStatusHandler(node string, agreementId string, lastHB string) func(pattern string, org string, lastCall string) (*exchange.NodeHealthStatus, error) {
	return func(pattern string, org string, lastCall string) (*exchange.NodeHealthStatus, error) {
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

func getNilUpdateStatusHandler(node string, agreementId string, lastHB string) func(pattern string, org string, lastCall string) (*exchange.NodeHealthStatus, error) {
	return func(pattern string, org string, lastCall string) (*exchange.NodeHealthStatus, error) {
		return nil, nil
	}
}

func getNoUpdateStatusHandler(node string, agreementId string, lastHB string) func(pattern string, org string, lastCall string) (*exchange.NodeHealthStatus, error) {
	return func(pattern string, org string, lastCall string) (*exchange.NodeHealthStatus, error) {
		o := &exchange.NodeHealthStatus{
			Nodes: map[string]exchange.NodeInfo{},
		}
		return o, nil
	}
}
