package api

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/common"
	"github.com/open-horizon/anax/eventlog"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/persistence"
	"strings"
)

func FindManagementStatusForOutput(nmpName, orgName string, errorHandler ErrorHandler, db *bolt.DB) (bool, map[string]*exchangecommon.NodeManagementPolicyStatus) {
	var err error
	managementStatuses := make(map[string]*exchangecommon.NodeManagementPolicyStatus, 0)

	var fullName string
	var pDevice *persistence.ExchangeDevice
	if orgName == "" {
		// Find exchange device in DB
		pDevice, err = persistence.FindExchangeDevice(db)
		if err != nil {
			return errorHandler(NewSystemError(fmt.Sprintf("Unable to read node object, error %v", err))), nil
		} else if pDevice == nil {
			return errorHandler(NewNotFoundError("Exchange registration not recorded. Complete account and node registration with an exchange and then record node registration using this API's /node path.", "management")), nil
		}
		fullName = string(pDevice.Org) + "/" + string(nmpName)
	} else {
		fullName = string(orgName) + "/" + string(nmpName)
	}

	// Look for given NMP status
	if nmpName != "" {
		var managementStatus *exchangecommon.NodeManagementPolicyStatus
		if managementStatus, err = persistence.FindNMPStatus(db, fullName); err != nil {
			return errorHandler(NewSystemError(fmt.Sprintf("unable to read management status object, error %v", err))), nil
		} else if managementStatus != nil {
			managementStatuses[fullName] = managementStatus
		} else {
			return errorHandler(NewNotFoundError(fmt.Sprintf("The nmp %v cannot be found", nmpName), "management")), nil
		}

		// Return all stored NMP statuses
	} else {
		if managementStatuses, err = persistence.FindAllNMPStatus(db); err != nil {
			return errorHandler(NewSystemError(fmt.Sprintf("unable to read management status object, error %v", err))), nil
		}
	}

	return false, managementStatuses
}

func UpdateManagementStatus(nmStatus exchangecommon.NodeManagementPolicyStatus, errorHandler ErrorHandler,
	statusHandler exchange.PutNodeManagementPolicyStatusHandler,
	getDeviceHandler exchange.DeviceHandler,
	patchDeviceHandler exchange.PatchDeviceHandler,
	nmpName string, orgName string, db *bolt.DB) (bool, string) {

	// Find exchange device in DB
	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		return errorHandler(NewSystemError(fmt.Sprintf("Unable to read node object, error %v", err))), ""
	} else if pDevice == nil {
		return errorHandler(NewNotFoundError("Exchange registration not recorded. Complete account and node registration with an exchange and then record node registration using this API's /node path.", "management")), ""
	}

	var managementStatus *exchangecommon.NodeManagementPolicyStatus
	if orgName == "" {
		orgName = pDevice.Org
	}
	fullName := string(orgName) + "/" + string(nmpName)

	// Check to see if status is already being stored
	if managementStatus, err = persistence.FindNMPStatus(db, fullName); err != nil {
		return errorHandler(NewSystemError(fmt.Sprintf("Unable to read management status object, error %v", err))), ""
	} else if managementStatus == nil {
		return errorHandler(NewNotFoundError(fmt.Sprintf("The nmp %v cannot be found", nmpName), "management")), ""
	}

	// save the new status to local db and the exchange
	status_changed, err := common.SetNodeManagementPolicyStatus(db, pDevice, fullName, &nmStatus, managementStatus, statusHandler, getDeviceHandler, patchDeviceHandler)
	if err != nil {
		return errorHandler(NewSystemError(fmt.Sprintf("Error saving nmp status for %v: %v", fullName, err))), ""
	}

	// Send NM_STATUS_CHANGED message if status was changed, log the new status to the event log
	if status_changed {
		newNMPStatus := managementStatus.AgentUpgrade.Status
		if managementStatus.AgentUpgrade.ErrorMessage != "" {
			newNMPStatus += fmt.Sprintf(", ErrorMessage: %v", managementStatus.AgentUpgrade.ErrorMessage)
		}
		eventlog.LogNodeEvent(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_API_NMP_STATUS_CHANGE, pDevice.Org, nmpName, newNMPStatus), persistence.EC_NMP_STATUS_UPDATE_COMPLETE, pDevice.Id, pDevice.Org, pDevice.Pattern, pDevice.Config.State)
	}

	// Return message
	return false, fmt.Sprintf("Updated status for NMP %v.", fullName)
}

func ResetManagementStatus(nmpName string, orgName string, errorHandler ErrorHandler,
	statusHandler exchange.PutNodeManagementPolicyStatusHandler,
	getDeviceHandler exchange.DeviceHandler,
	patchDeviceHandler exchange.PatchDeviceHandler,
	db *bolt.DB) (bool, string) {

	nmStatus := new(exchangecommon.NodeManagementPolicyStatus)
	nmStatus.AgentUpgrade = new(exchangecommon.AgentUpgradePolicyStatus)
	nmStatus.SetStatus(exchangecommon.STATUS_NEW)

	names := []string{}
	if nmpName != "" {
		names = append(names, nmpName)
		return UpdateManagementStatus(*nmStatus, errorHandler, statusHandler, getDeviceHandler, patchDeviceHandler, nmpName, orgName, db)
	} else {
		if allMgmtStatuses, err := persistence.FindAllNMPStatus(db); err != nil {
			return errorHandler(NewSystemError(fmt.Sprintf("unable to read management status object, error %v", err))), ""
		} else if allMgmtStatuses != nil && len(allMgmtStatuses) != 0 {
			for nmpStatusKey, _ := range allMgmtStatuses {
				names = append(names, nmpStatusKey)
				if hasError, rsrc := UpdateManagementStatus(*nmStatus, errorHandler, statusHandler, getDeviceHandler, patchDeviceHandler, exchange.GetId(nmpStatusKey), exchange.GetOrg(nmpStatusKey), db); hasError {
					return hasError, rsrc
				}
			}
		}
	}

	// Return message
	if len(names) == 0 {
		return false, fmt.Sprintf("No nmp status found")
	} else if len(names) == 1 {
		return false, fmt.Sprintf("Updated the status for NMP %v.", names[0])
	} else {
		return false, fmt.Sprintf("Updated the status for NMPs %v.", strings.Join(names, ","))
	}
}
