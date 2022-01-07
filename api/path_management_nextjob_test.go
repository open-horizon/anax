package api

import (
	"flag"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/persistence"
	"testing"
	"time"
)

func init() {
	flag.Set("alsologtostderr", "true")
	flag.Set("v", "7")
	// no need to parse flags, that's done by test framework
}

func Test_FindManagementNextJobForOutput(t *testing.T) {
	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	var myError error
	errorHandler := GetPassThroughErrorHandler(&myError)

	// Create dummy exchange device in local DB
	pDevice, err := persistence.SaveNewExchangeDevice(db, "id", "token", "name", "nodeType", true, "org", "pattern", "configstate")
	if err != nil {
		t.Errorf("Unable to read node object, error %v", err)
	} else if pDevice == nil {
		t.Errorf("Exchange registration not recorded. Complete account and node registration with an exchange and then record node registration using this API's /node path.")
	}

	// test #1 - No NMP Statuses in local db
	if errHandled, statuses := FindManagementNextJobForOutput("agentUpgrade", "true", errorHandler, db); errHandled {
		t.Errorf("failed to find node management status in db, error %v", err)
	} else if statuses != nil && len(statuses) != 0 {
		t.Errorf("incorrect number of management statuses returned from db, expected: %v, actual: %v", 0, len(statuses))
	}

	testnmp := create_test_status(exchangecommon.STATUS_NEW, dir)
	persistence.SaveOrUpdateNMPStatus(db, "org/testnmp", *testnmp)

	// test #2 - Only "waiting" statuses in local db, search for "downloaded"
	if errHandled, statuses := FindManagementNextJobForOutput("agentUpgrade", "true", errorHandler, db); errHandled {
		t.Errorf("failed to find node management status in db, error %v", err)
	} else if statuses != nil && len(statuses) != 0 {
		t.Errorf("incorrect number of management statuses returned from db, expected: %v, actual: %v", 0, len(statuses))
	}

	// test #3 - Only "waiting" statuses in local db, search for not "downloaded"
	if errHandled, statuses := FindManagementNextJobForOutput("agentUpgrade", "false", errorHandler, db); errHandled {
		t.Errorf("failed to find node management status in db, error %v", err)
	} else if statuses != nil && len(statuses) != 1 {
		t.Errorf("incorrect number of management statuses returned from db, expected: %v, actual: %v", 1, len(statuses))
	} else if !equal_statuses(testnmp, statuses["org/testnmp"]) {
		t.Errorf("incorrect node management status for \"testnmp\" returned from db, error %v", err)
	}

	testnmp2 := create_test_status(exchangecommon.STATUS_DOWNLOADED, dir)
	persistence.SaveOrUpdateNMPStatus(db, "org/testnmp2", *testnmp2)

	// test #4 - Add "downloaded" status to local db, search for "downloaded"
	if errHandled, statuses := FindManagementNextJobForOutput("agentUpgrade", "true", errorHandler, db); errHandled {
		t.Errorf("failed to find node management status in db, error %v", err)
	} else if statuses != nil && len(statuses) != 1 {
		t.Errorf("incorrect number of management statuses returned from db, expected: %v, actual: %v", 1, len(statuses))
	} else if !equal_statuses(testnmp2, statuses["org/testnmp2"]) {
		t.Errorf("incorrect node management status for \"testnmp2\" returned from db, error %v", err)
	}

	// test #5 - Search for all statuses
	if errHandled, statuses := FindManagementNextJobForOutput("agentUpgrade", "", errorHandler, db); errHandled {
		t.Errorf("failed to find node management status in db, error %v", err)
	} else if statuses != nil && len(statuses) != 1 {
		t.Errorf("incorrect number of management statuses returned from db, expected: %v, actual: %v", 1, len(statuses))
	} else if !equal_statuses(testnmp, statuses["org/testnmp"]) {
		t.Errorf("incorrect node management status for \"testnmp\" returned from db, error %v", err)
	}

	// test #5 - Search for not "downloaded" statuses
	if errHandled, statuses := FindManagementNextJobForOutput("agentUpgrade", "false", errorHandler, db); errHandled {
		t.Errorf("failed to find node management status in db, error %v", err)
	} else if statuses != nil && len(statuses) != 1 {
		t.Errorf("incorrect number of management statuses returned from db, expected: %v, actual: %v", 1, len(statuses))
	} else if !equal_statuses(testnmp, statuses["org/testnmp"]) {
		t.Errorf("incorrect node management status for \"testnmp\" returned from db, error %v", err)
	}

	// test #6 - Search for any status type
	if errHandled, statuses := FindManagementNextJobForOutput("", "", errorHandler, db); errHandled {
		t.Errorf("failed to find node management status in db, error %v", err)
	} else if statuses != nil && len(statuses) != 1 {
		t.Errorf("incorrect number of management statuses returned from db, expected: %v, actual: %v", 1, len(statuses))
	} else if !equal_statuses(testnmp, statuses["org/testnmp"]) {
		t.Errorf("incorrect node management status for \"testnmp\" returned from db, error %v", err)
	}

	// test #7 - Search for incorrect status type
	if errHandled, statuses := FindManagementNextJobForOutput("wrongType", "", errorHandler, db); !errHandled {
		t.Errorf("did not receive expected error.")
	} else if statuses != nil && len(statuses) != 0 {
		t.Errorf("incorrect number of management statuses returned from db, expected: %v, actual: %v", 0, len(statuses))
	}
}

func equal_statuses(a, b *exchangecommon.NodeManagementPolicyStatus) bool {
	if a.AgentUpgrade != nil && b.AgentUpgrade != nil {
		if a.AgentUpgrade.ScheduledTime != b.AgentUpgrade.ScheduledTime {
			return false
		}
		if a.AgentUpgrade.ActualStartTime != b.AgentUpgrade.ActualStartTime {
			return false
		}
		if a.AgentUpgrade.CompletionTime != b.AgentUpgrade.CompletionTime {
			return false
		}
		if a.AgentUpgrade.Status != b.AgentUpgrade.Status {
			return false
		}
		if a.AgentUpgrade.ErrorMessage != b.AgentUpgrade.ErrorMessage {
			return false
		}
		if a.AgentUpgrade.BaseWorkingDirectory != b.AgentUpgrade.BaseWorkingDirectory {
			return false
		}
		return true
	} else if a.AgentUpgrade != nil || b.AgentUpgrade != nil {
		return false
	} else {
		return true
	}
}

func create_test_status(status, dir string) *exchangecommon.NodeManagementPolicyStatus {
	switch(status) {
	case exchangecommon.STATUS_DOWNLOADED:
		return &exchangecommon.NodeManagementPolicyStatus {
			AgentUpgrade: &exchangecommon.AgentUpgradePolicyStatus {
				ScheduledTime: time.Now().Format(time.RFC3339),
				ActualStartTime: "",
				CompletionTime: "",
				Status: exchangecommon.STATUS_DOWNLOADED,
				ErrorMessage: "",
				BaseWorkingDirectory: dir,
			},
		}
	case exchangecommon.STATUS_NEW:
		return &exchangecommon.NodeManagementPolicyStatus {
			AgentUpgrade: &exchangecommon.AgentUpgradePolicyStatus {
				ScheduledTime: time.Now().Format(time.RFC3339),
				ActualStartTime: "",
				CompletionTime: "",
				Status: exchangecommon.STATUS_NEW,
				ErrorMessage: "",
				BaseWorkingDirectory: dir,
			},
		}
	case exchangecommon.STATUS_INITIATED:
		return &exchangecommon.NodeManagementPolicyStatus {
			AgentUpgrade: &exchangecommon.AgentUpgradePolicyStatus {
				ScheduledTime: time.Now().Format(time.RFC3339),
				ActualStartTime: time.Now().Format(time.RFC3339),
				CompletionTime: "",
				Status: exchangecommon.STATUS_INITIATED,
				ErrorMessage: "",
				BaseWorkingDirectory: dir,
			},
		}
	case exchangecommon.STATUS_SUCCESSFUL:
		return &exchangecommon.NodeManagementPolicyStatus {
			AgentUpgrade: &exchangecommon.AgentUpgradePolicyStatus {
				ScheduledTime: time.Now().Format(time.RFC3339),
				ActualStartTime: time.Now().Format(time.RFC3339),
				CompletionTime: time.Now().Format(time.RFC3339),
				Status: exchangecommon.STATUS_SUCCESSFUL,
				ErrorMessage: "",
				BaseWorkingDirectory: dir,
			},
		}
	case exchangecommon.STATUS_DOWNLOAD_FAILED:
		return &exchangecommon.NodeManagementPolicyStatus {
			AgentUpgrade: &exchangecommon.AgentUpgradePolicyStatus {
				ScheduledTime: time.Now().Format(time.RFC3339),
				ActualStartTime: time.Now().Format(time.RFC3339),
				CompletionTime: "",
				Status: exchangecommon.STATUS_DOWNLOAD_FAILED,
				ErrorMessage: "Download failed.",
				BaseWorkingDirectory: dir,
			},
		}
	case exchangecommon.STATUS_FAILED_JOB:
		return &exchangecommon.NodeManagementPolicyStatus {
			AgentUpgrade: &exchangecommon.AgentUpgradePolicyStatus {
				ScheduledTime: time.Now().Format(time.RFC3339),
				ActualStartTime: time.Now().Format(time.RFC3339),
				CompletionTime: "",
				Status: exchangecommon.STATUS_FAILED_JOB,
				ErrorMessage: "Failed job.",
				BaseWorkingDirectory: dir,
			},
		}
	case exchangecommon.STATUS_UNKNOWN:
		return &exchangecommon.NodeManagementPolicyStatus {
			AgentUpgrade: &exchangecommon.AgentUpgradePolicyStatus {
				ScheduledTime: time.Now().Format(time.RFC3339),
				ActualStartTime: "",
				CompletionTime: "",
				Status: exchangecommon.STATUS_UNKNOWN,
				ErrorMessage: "Unknown.",
				BaseWorkingDirectory: dir,
			},
		}
	default:
		return nil
	}
}
