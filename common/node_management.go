package common

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/version"
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
// It returns true if the status is changed. False if the status stays the same.
func SetNodeManagementPolicyStatus(db *bolt.DB, pDevice *persistence.ExchangeDevice, nmp_id string,
	newStatus *exchangecommon.NodeManagementPolicyStatus,
	dbStatus *exchangecommon.NodeManagementPolicyStatus,
	putStatusHandler exchange.PutNodeManagementPolicyStatusHandler,
	getDeviceHandler exchange.DeviceHandler,
	patchDeviceHandler exchange.PatchDeviceHandler) (bool, error) {

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
				cert_version := ""
				config_version := ""
				agent_version := ""
				sw_version := pDevice.SoftwareVersions
				if sw_version != nil {
					cert_version, _ = sw_version[persistence.CERT_VERSION]
					config_version, _ = sw_version[persistence.CONFIG_VERSION]
					agent_version, _ = sw_version[persistence.AGENT_VERSION]
				}

				// Use the versions in the status to set the device versions
				updateExch := false
				newCertVer := dbStatus.AgentUpgrade.UpgradedVersions.CertVersion
				newConfigVer := dbStatus.AgentUpgrade.UpgradedVersions.ConfigVersion
				if newCertVer != "" {
					pDevice.SetCertVersion(db, pDevice.Id, newCertVer)
					if newCertVer != cert_version {
						updateExch = true
					}
				}
				if newConfigVer != "" {
					pDevice.SetConfigVersion(db, pDevice.Id, newConfigVer)
					if newConfigVer != config_version {
						updateExch = true
					}
				}

				// Update the agent software version 
				pDevice.SetAgentVersion(db, pDevice.Id, version.HORIZON_VERSION)
				if version.HORIZON_VERSION != agent_version {
					updateExch = true
				}

				// path the node with new cert and config versions
				if updateExch {
					if err := UpdateExchNodeSoftwareVersions(newCertVer, newConfigVer,
						fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id), pDevice.Token,
						getDeviceHandler, patchDeviceHandler); err != nil {
						return true, fmt.Errorf("Failed to update the node SoftwareVersion attribute in the Exchange. %v", err)
					}
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

// update the node.SoftwareVersion with the given certficate and config versions.
func UpdateExchNodeSoftwareVersions(newCertVer string, newConfigVer string,
	id_with_org string, token string,
	getDeviceHandler exchange.DeviceHandler,
	patchDeviceHandler exchange.PatchDeviceHandler) error {

	exchNode, err := getDeviceHandler(id_with_org, token)
	if err != nil {
		return fmt.Errorf("Failed to get the node %v from the exchange. %v", id_with_org, err)
	}

	if exchNode != nil {
		// patch the device with new software versions
		versions := exchNode.SoftwareVersions
		if versions == nil {
			versions = make(map[string]string, 0)
		}
		versions[exchangecommon.HORIZON_VERSION] = version.HORIZON_VERSION
		versions[exchangecommon.CERT_VERSION] = newCertVer
		versions[exchangecommon.CONFIG_VERSION] = newConfigVer

		if err = patchDeviceHandler(id_with_org, token, &exchange.PatchDeviceRequest{SoftwareVersions: versions}); err != nil {
			return fmt.Errorf("Failed to patch the exchange node %v with correct software versions. %v", id_with_org, err)
		}
	}

	return nil
}
