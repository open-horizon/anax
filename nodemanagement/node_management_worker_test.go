//go:build unit
// +build unit

package nodemanagement

import (
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/externalpolicy"
	_ "github.com/open-horizon/anax/externalpolicy/text_language"
	"github.com/open-horizon/anax/persistence"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"
)

func Test_ProcessAllNMPS(t *testing.T) {
	dir, db, err := setupDB()
	if err != nil {
		t.Errorf("Error setting up db for tests: %v", err)
	}
	defer cleanupDB(dir)

	w := NewNodeManagementWorker("nmpworker", &config.HorizonConfig{}, db)

	nodeDepProps := externalpolicy.PropertyList{*externalpolicy.Property_Factory("prop1", false), *externalpolicy.Property_Factory("prop2", 10)}
	nodeDepConstr := externalpolicy.ConstraintExpression{"nmpProp1 == Ontario || nmpProp2 > 5"}
	nodeDepPol := externalpolicy.ExternalPolicy{Properties: nodeDepProps, Constraints: nodeDepConstr}
	nodeMgmtProps := externalpolicy.PropertyList{*externalpolicy.Property_Factory("prop2", 5)}
	nodeMgmtConstr := externalpolicy.ConstraintExpression{"nmpProp1 == Toronto"}
	nodeMgmtPol := externalpolicy.ExternalPolicy{Properties: nodeMgmtProps, Constraints: nodeMgmtConstr}
	nodeProps := externalpolicy.PropertyList{*externalpolicy.Property_Factory("prop3", "yes")}
	nodeConstr := externalpolicy.ConstraintExpression{"nmpProp3 == green"}
	nodeTopLvlPol := externalpolicy.ExternalPolicy{Properties: nodeProps, Constraints: nodeConstr}
	nodePol := exchangecommon.NodePolicy{Deployment: nodeDepPol, Management: nodeMgmtPol, ExternalPolicy: nodeTopLvlPol}

	if err = persistence.SaveNodePolicy(db, &nodePol); err != nil {
		t.Errorf("Error saving node policy: %v", err)
	}

	// device set to  use policy
	_, err = persistence.SaveNewExchangeDevice(db, "testNode", "testNodeTok", "testNode", persistence.DEVICE_TYPE_DEVICE, "userdev", "", persistence.CONFIGSTATE_CONFIGURED, persistence.SoftwareVersion{persistence.AGENT_VERSION: "2.1.1", persistence.CONFIG_VERSION: "", persistence.CERT_VERSION: "1.2.3"})
	if err != nil {
		t.Errorf("Error saving exchange device in db: %v", err)
	}

	// save a policy to remove
	err = persistence.SaveOrUpdateNMPStatus(db, "userdev/noMoreNMP", exchangecommon.NodeManagementPolicyStatus{})

	// matches mgmt policy
	nmp1 := exchangecommon.ExchangeNodeManagementPolicy{
		Patterns:               []string{},
		Enabled:                true,
		PolicyUpgradeTime:      "now",
		AgentAutoUpgradePolicy: &exchangecommon.ExchangeAgentUpgradePolicy{Manifest: "manifest", AllowDowngrade: false},
		Constraints:            externalpolicy.ConstraintExpression{"prop2 < 9 && prop3 == yes"},
		Properties:             externalpolicy.PropertyList{*externalpolicy.Property_Factory("nmpProp1", "Toronto"), *externalpolicy.Property_Factory("nmpProp3", "green")},
	}

	// matches dep policy
	nmp2 := exchangecommon.ExchangeNodeManagementPolicy{
		Patterns:               []string{},
		Enabled:                true,
		PolicyUpgradeTime:      "now",
		AgentAutoUpgradePolicy: &exchangecommon.ExchangeAgentUpgradePolicy{Manifest: "manifest", AllowDowngrade: false},
		Constraints:            externalpolicy.ConstraintExpression{"prop2 > 9 && prop1 == false"},
		Properties:             externalpolicy.PropertyList{*externalpolicy.Property_Factory("nmpProp1", false), *externalpolicy.Property_Factory("nmpProp2", 10)},
	}

	// matches mgmt policy  but disabled
	nmp3 := exchangecommon.ExchangeNodeManagementPolicy{
		Patterns:               []string{},
		Enabled:                false,
		PolicyUpgradeTime:      "now",
		AgentAutoUpgradePolicy: &exchangecommon.ExchangeAgentUpgradePolicy{Manifest: "manifest", AllowDowngrade: false},
		Constraints:            externalpolicy.ConstraintExpression{"prop2 < 9 && prop1 == true"},
		Properties:             externalpolicy.PropertyList{*externalpolicy.Property_Factory("nmpProp1", "Toronto"), *externalpolicy.Property_Factory("nmpProp3", "green")},
	}

	allPols := map[string]exchangecommon.ExchangeNodeManagementPolicy{"userdev/nmp1": nmp1, "userdev/nmp2": nmp2, "userdev/nmp3": nmp3}

	err = w.ProcessAllNMPS("", getAllNMPSHandler(&allPols), getDeleteNMPStatusHandler(), getPutNMPStatusHandler(), getAllNodeManagementPolicyStatusHandler())
	if err != nil {
		t.Errorf("Unexpected error while processing nmps: %v.", err)
	}
	statuses, err := persistence.FindAllNMPStatus(db)
	if err != nil {
		t.Errorf("Unexpected error while getting statuses from db: %v.", err)
	} else if _, ok := statuses["userdev/noMoreNMP"]; ok {
		t.Errorf("Policy status for \"noMoreNMP\" should have been deleted from db but was not.")
	} else if _, ok := statuses["userdev/nmp1"]; !ok {
		t.Errorf("Policy status for \"userdev/nmp1\" should have been saved to db but was not.")
	} else if _, ok := statuses["userdev/nmp2"]; ok {
		t.Errorf("Policy status for \"userdev/nmp2\" should not have been saved to db but was.")
	} else if _, ok := statuses["userdev/nmp3"]; ok {
		t.Errorf("Policy status for disabled nmp \"userdev/nmp3\" should not have been saved to db but was.")
	}
}

func getAllNMPSHandler(pols *map[string]exchangecommon.ExchangeNodeManagementPolicy) exchange.AllNodeManagementPoliciesHandler {
	return func(policyOrg string) (*map[string]exchangecommon.ExchangeNodeManagementPolicy, error) {
		return pols, nil
	}
}

func getDeleteNMPStatusHandler() exchange.DeleteNodeManagementPolicyStatusHandler {
	return func(orgId string, nodeId string, policyName string) error {
		return nil
	}
}

func getPutNMPStatusHandler() exchange.PutNodeManagementPolicyStatusHandler {
	return func(orgId string, nodeId string, policyName string, nmpStatus *exchangecommon.NodeManagementPolicyStatus) (*exchange.PutPostDeleteStandardResponse, error) {
		return nil, nil
	}
}

func getAllNodeManagementPolicyStatusHandler() exchange.AllNodeManagementPolicyStatusHandler {
	return func(orgId string, nodeId string) (*exchange.NodeManagementAllStatuses, error) {
		return nil, nil
	}
}

func Test_getEarliest(t *testing.T) {
	status1 := exchangecommon.NodeManagementPolicyStatus{AgentUpgradeInternal: &exchangecommon.AgentUpgradeInternalStatus{ScheduledUnixTime: time.Unix(1649212221, 0)}}
	status2 := exchangecommon.NodeManagementPolicyStatus{AgentUpgradeInternal: &exchangecommon.AgentUpgradeInternalStatus{ScheduledUnixTime: time.Unix(1649211221, 0)}}
	status3 := exchangecommon.NodeManagementPolicyStatus{AgentUpgradeInternal: &exchangecommon.AgentUpgradeInternalStatus{ScheduledUnixTime: time.Unix(1649112221, 0)}}
	statusList := &map[string]*exchangecommon.NodeManagementPolicyStatus{"status1": &status1, "status2": &status2, "status3": &status3}

	nextName, _ := getLatest(statusList)
	if nextName != "status1" {
		t.Errorf("The next nmp to run should have been \"status1\": found %s instead.", nextName)
	} else if len(*statusList) != 2 {
		t.Errorf("The status list should contain 2 remaining statuses. Found  %v.", statusList)
	}
	nextName, _ = getLatest(statusList)
	if nextName != "status2" {
		t.Errorf("The next nmp to run should have been \"status2\": found %s instead.", nextName)
	} else if len(*statusList) != 1 {
		t.Errorf("The status list should contain 1 remaining statuses. Found  %v.", statusList)
	}
	nextName, _ = getLatest(statusList)
	if nextName != "status3" {
		t.Errorf("The next nmp to run should have been \"status3\": found %s instead.", nextName)
	} else if len(*statusList) != 0 {
		t.Errorf("The status list should contain 0 remaining statuses. Found  %v.", statusList)
	}

	nextName, _ = getLatest(statusList)
	if nextName != "" {
		t.Errorf("The next nmp to run should have been \"\": found %s instead.", nextName)
	} else if len(*statusList) != 0 {
		t.Errorf("The status list should contain 0 remaining statuses. Found  %v.", statusList)
	}

	statusList = nil
	nextName, _ = getLatest(statusList)
	if nextName != "" {
		t.Errorf("The next nmp to run should have been \"\": found %s instead.", nextName)
	} else if statusList != nil {
		t.Errorf("The status list should be nil. Found  %v.", statusList)
	}
}

func setupDB() (string, *bolt.DB, error) {
	dir, err := ioutil.TempDir("", "container-")
	if err != nil {
		return "", nil, err
	}

	db, err := bolt.Open(path.Join(dir, "anax-int.db"), 0600, &bolt.Options{Timeout: 10 * time.Second})
	if err != nil {
		return dir, nil, err
	}

	return dir, db, nil
}

func cleanupDB(dir string) error {
	return os.RemoveAll(dir)
}
