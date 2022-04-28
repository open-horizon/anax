package common

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/persistence"
	"os"
	"path"
	"time"
)

// The input data newStatus is either from the status.json file or from the /nodemanagement/status/nmp
// agent API call. The newStatus mainly contains the status and the error message.
// This function will add the start and completion time accordingly.
// It will:
// 1. save the status to local db
// 2. put the status to the exchange
// 3. update the node software, config and cert versions locally and exchange.
// 4. remove the working directory if the status is 'successful'

// nmp_id is "org/nmp_name".
func SetNodeManagementPolicyStatus(db *bolt.DB, pDevice *persistence.ExchangeDevice, nmp_id string,
	newStatus *exchangecommon.NodeManagementPolicyStatus,
	dbStatus *exchangecommon.NodeManagementPolicyStatus,
	putStatusHandler exchange.PutNodeManagementPolicyStatusHandler) (bool, error) {

	if newStatus.AgentUpgrade == nil {
		return false, nil
	}

	if dbStatus.AgentUpgrade == nil {
		dbStatus.AgentUpgrade = new(exchangecommon.AgentUpgradePolicyStatus)
	}

	// Set the actual start time and completion time depending on new status
	statusString := newStatus.AgentUpgrade.Status
	if statusString == "" {
		statusString = exchangecommon.STATUS_UNKNOWN
	}

	if statusString == exchangecommon.STATUS_INITIATED && (dbStatus.AgentUpgrade.ActualStartTime == "" || dbStatus.AgentUpgrade.ActualStartTime == "0") {
		dbStatus.AgentUpgrade.ActualStartTime = time.Now().Format(time.RFC3339)
	} else if statusString == exchangecommon.STATUS_SUCCESSFUL && (dbStatus.AgentUpgrade.CompletionTime == "" || dbStatus.AgentUpgrade.CompletionTime == "0") {
		dbStatus.AgentUpgrade.CompletionTime = time.Now().Format(time.RFC3339)
	}
	dbStatus.AgentUpgrade.ErrorMessage = newStatus.AgentUpgrade.ErrorMessage

	// Only set when the old and new status are different
	if dbStatus.AgentUpgrade.Status != statusString {
		dbStatus.AgentUpgrade.Status = statusString

		// Update the NMP status in the local db
		if err := persistence.SaveOrUpdateNMPStatus(db, nmp_id, *dbStatus); err != nil {
			return true, fmt.Errorf("Unable to update node management status object in local database, error %v", err)
		}

		// Update the status of the NMP in the exchange
		if pDevice != nil {
			_, nmpName := cutil.SplitOrgSpecUrl(nmp_id)
			if _, err := putStatusHandler(pDevice.Org, pDevice.Id, nmpName, dbStatus); err != nil {
				return true, fmt.Errorf("Unable to update node management status object in the exchange, error %v", err)
			}
		}

		if statusString == exchangecommon.STATUS_SUCCESSFUL {
			if pDevice != nil {
				// Use the versions in the status to set the device versions
				if dbStatus.AgentUpgrade.UpgradedVersions.ConfigVersion != "" {
					pDevice.SetConfigVersion(db, pDevice.Id, dbStatus.AgentUpgrade.UpgradedVersions.ConfigVersion)
				}
				if dbStatus.AgentUpgrade.UpgradedVersions.CertVersion != "" {
					pDevice.SetCertVersion(db, pDevice.Id, dbStatus.AgentUpgrade.UpgradedVersions.CertVersion)
				}
			}

			// Status has been read-in and updated sucessfully. Can now remove the working dirctory for the job.
			if err := os.RemoveAll(path.Join(dbStatus.AgentUpgrade.BaseWorkingDirectory, nmp_id)); err != nil {
				return true, fmt.Errorf("Failed to remove the working directory for management job %v. Error was: %v", nmp_id, err)
			}
		}
		return true, nil
	}
	return false, nil
}
