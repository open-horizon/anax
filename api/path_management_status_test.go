package api

import (
	"flag"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/persistence"
	"testing"
)

func init() {
	flag.Set("alsologtostderr", "true")
	flag.Set("v", "7")
	// no need to parse flags, that's done by test framework
}

func Test_FindManagementStatusForOutput(t *testing.T) {
	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	var myError error
	errorHandler := GetPassThroughErrorHandler(&myError)

	// Create dummy exchange device in local DB
	pDevice, err := persistence.SaveNewExchangeDevice(db, "id", "token", "name", "nodeType", true, "org", "pattern", "configstate", persistence.SoftwareVersion{persistence.AGENT_VERSION: "1.0.0"})
	if err != nil {
		t.Errorf("Unable to read node object, error %v", err)
	} else if pDevice == nil {
		t.Errorf("Exchange registration not recorded. Complete account and node registration with an exchange and then record node registration using this API's /node path.")
	}

	// Save a few example statuses in local db
	testnmp := create_test_status(exchangecommon.STATUS_NEW, dir)
	persistence.SaveOrUpdateNMPStatus(db, "org/testnmp", *testnmp)
	testnmp2 := create_test_status(exchangecommon.STATUS_DOWNLOADED, dir)
	persistence.SaveOrUpdateNMPStatus(db, "org/testnmp2", *testnmp2)
	testnmp3 := create_test_status(exchangecommon.STATUS_SUCCESSFUL, dir)
	persistence.SaveOrUpdateNMPStatus(db, "org/testnmp3", *testnmp3)

	// Test #1 - Look for a specific NMP Status
	if errHandled, statuses := FindManagementStatusForOutput("testnmp", "org", errorHandler, db); errHandled {
		t.Errorf("failed to find node management status in db, error %v", err)
	} else if statuses != nil && len(statuses) != 1 {
		t.Errorf("incorrect number of management statuses returned from db, expected: %v, actual: %v", 1, len(statuses))
	} else if !equal_statuses(testnmp, statuses["org/testnmp"]) {
		t.Errorf("incorrect node management status for \"testnmp\" returned from db, error %v", err)
	}

	// Test #2 - Get all NMP Statuses
	if errHandled, statuses := FindManagementStatusForOutput("", "", errorHandler, db); errHandled {
		t.Errorf("failed to find node management status in db, error %v", err)
	} else if statuses != nil && len(statuses) != 3 {
		t.Errorf("incorrect number of management statuses returned from db, expected: %v, actual: %v", 3, len(statuses))
	} else if !equal_statuses(testnmp, statuses["org/testnmp"]) {
		t.Errorf("incorrect node management status for \"testnmp\" returned from db, error %v", err)
	} else if !equal_statuses(testnmp2, statuses["org/testnmp2"]) {
		t.Errorf("incorrect node management status for \"testnmp2\" returned from db, error %v", err)
	} else if !equal_statuses(testnmp3, statuses["org/testnmp3"]) {
		t.Errorf("incorrect node management status for \"testnmp3\" returned from db, error %v", err)
	}
}

func Test_UpdateManagementStatus(t *testing.T) {
	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	// Create passthrough error handler
	var mainError error
	errorHandler := GetPassThroughErrorHandler(&mainError)

	// Create dummy management status
	nmStatus := exchangecommon.NodeManagementPolicyStatus{
		AgentUpgrade: &exchangecommon.AgentUpgradePolicyStatus{
			Status:       exchangecommon.STATUS_DOWNLOADED,
			ErrorMessage: "",
		},
	}

	// Create dummy exchange device in local DB
	pDevice, err := persistence.SaveNewExchangeDevice(db, "id", "token", "name", "nodeType", true, "org", "pattern", "configstate", persistence.SoftwareVersion{persistence.AGENT_VERSION: "1.0.0"})
	if err != nil {
		t.Errorf("Unable to read node object, error %v", err)
	} else if pDevice == nil {
		t.Errorf("Exchange registration not recorded. Complete account and node registration with an exchange and then record node registration using this API's /node path.")
	}

	// Create dummy handler for putting updated NMP status in the exchange
	statusHandler := func(orgId string, nodeId string, policyName string, nmpStatus *exchangecommon.NodeManagementPolicyStatus) (*exchange.PutPostDeleteStandardResponse, error) {
		return &exchange.PutPostDeleteStandardResponse{Code: "401", Msg: "nil"}, nil
	}

	// Save a couple example statuses in local db
	testnmp := create_test_status(exchangecommon.STATUS_NEW, dir)
	persistence.SaveOrUpdateNMPStatus(db, "org/testnmp", *testnmp)
	testnmp2 := create_test_status(exchangecommon.STATUS_DOWNLOADED, dir)
	persistence.SaveOrUpdateNMPStatus(db, "org/testnmp2", *testnmp2)

	var newNMPStatus *exchangecommon.NodeManagementPolicyStatus

	// Test #1 - Update a specific NMP Status to STATUS_DOWNLOADED
	if errHandled, out, msgs := UpdateManagementStatus(nmStatus, errorHandler, statusHandler, "testnmp", db); errHandled {
		t.Errorf("failed to update node management status in db, error %v", mainError)
	} else if out != "Updated status for NMP org/testnmp." {
		t.Errorf("incorrect return response, expected: %v, actual: %v", "Updated status for NMP org/testnmp.", out)
	} else if msgs[0].Status != nmStatus.AgentUpgrade.Status {
		t.Errorf("incorrect event(s) sent, expected: %v, actual: %v", nmStatus.AgentUpgrade.Status, msgs[0].Status)
	}
	newNMPStatus, _ = persistence.FindNMPStatus(db, "org/testnmp")
	if newNMPStatus.AgentUpgrade.Status != nmStatus.AgentUpgrade.Status {
		t.Errorf("NMP status was not updated: %v", newNMPStatus)
	}

	// Test #2 - Update a specific NMP Status to STATUS_INITIATED
	nmStatus.AgentUpgrade.Status = exchangecommon.STATUS_INITIATED
	oldNMPStatus, _ := persistence.FindNMPStatus(db, "org/testnmp2")
	oldStartTime := oldNMPStatus.AgentUpgrade.ActualStartTime
	if errHandled, out, msgs := UpdateManagementStatus(nmStatus, errorHandler, statusHandler, "testnmp2", db); errHandled {
		t.Errorf("failed to update node management status in db, error %v", mainError)
	} else if out != "Updated status for NMP org/testnmp2." {
		t.Errorf("incorrect return response, expected: %v, actual: %v", "Updated status for NMP org/testnmp2.", out)
	} else if msgs[0].Status != nmStatus.AgentUpgrade.Status {
		t.Errorf("incorrect event(s) sent, expected: %v, actual: %v", nmStatus.AgentUpgrade.Status, msgs[0].Status)
	}
	newNMPStatus, _ = persistence.FindNMPStatus(db, "org/testnmp2")
	if newNMPStatus.AgentUpgrade.Status != nmStatus.AgentUpgrade.Status {
		t.Errorf("NMP status was not updated, expected: %v, actual: %v", nmStatus.AgentUpgrade.Status, newNMPStatus.AgentUpgrade.Status)
	} else if oldStartTime == newNMPStatus.AgentUpgrade.ActualStartTime {
		t.Errorf("NMP actual start time was not updated, expected: %v, actual: %v", nmStatus.AgentUpgrade.Status, newNMPStatus.AgentUpgrade.Status)
	}

	// Test #3 - Update a specific NMP Status to STATUS_SUCCESSFUL
	nmStatus.AgentUpgrade.Status = exchangecommon.STATUS_SUCCESSFUL
	oldNMPStatus, _ = persistence.FindNMPStatus(db, "org/testnmp2")
	oldCompletionTime := oldNMPStatus.AgentUpgrade.CompletionTime
	if errHandled, out, msgs := UpdateManagementStatus(nmStatus, errorHandler, statusHandler, "testnmp2", db); errHandled {
		t.Errorf("failed to update node management status in db, error %v", mainError)
	} else if out != "Updated status for NMP org/testnmp2." {
		t.Errorf("incorrect return response, expected: %v, actual: %v", "Updated status for NMP org/testnmp2.", out)
	} else if msgs[0].Status != nmStatus.AgentUpgrade.Status {
		t.Errorf("incorrect event(s) sent, expected: %v, actual: %v", nmStatus.AgentUpgrade.Status, msgs[0].Status)
	}
	newNMPStatus, _ = persistence.FindNMPStatus(db, "org/testnmp2")
	if newNMPStatus.AgentUpgrade.Status != nmStatus.AgentUpgrade.Status {
		t.Errorf("NMP status was not updated, expected: %v, actual: %v", nmStatus.AgentUpgrade.Status, newNMPStatus.AgentUpgrade.Status)
	} else if oldCompletionTime == newNMPStatus.AgentUpgrade.CompletionTime {
		t.Errorf("NMP completion time was not updated, expected: %v, actual: %v", nmStatus.AgentUpgrade.Status, newNMPStatus.AgentUpgrade.Status)
	}
}
