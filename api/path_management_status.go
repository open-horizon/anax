package api

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/common"
	"github.com/open-horizon/anax/eventlog"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/persistence"
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

func UpdateManagementStatus(nmStatus exchangecommon.NodeManagementPolicyStatus, errorHandler ErrorHandler, statusHandler exchange.PutNodeManagementPolicyStatusHandler, nmpName string, db *bolt.DB) (bool, string, []*events.NMStatusChangedMessage) {
	// Slice to store events
	msgs := make([]*events.NMStatusChangedMessage, 0, 10)

	// Find exchange device in DB
	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		return errorHandler(NewSystemError(fmt.Sprintf("Unable to read node object, error %v", err))), "", nil
	} else if pDevice == nil {
		return errorHandler(NewNotFoundError("Exchange registration not recorded. Complete account and node registration with an exchange and then record node registration using this API's /node path.", "management")), "", nil
	}

	var managementStatus *exchangecommon.NodeManagementPolicyStatus
	fullName := string(pDevice.Org) + "/" + string(nmpName)

	// Check to see if status is already being stored
	if managementStatus, err = persistence.FindNMPStatus(db, fullName); err != nil {
		return errorHandler(NewSystemError(fmt.Sprintf("Unable to read management status object, error %v", err))), "", nil
	} else if managementStatus == nil {
		return errorHandler(NewNotFoundError(fmt.Sprintf("The nmp %v cannot be found", nmpName), "management")), "", nil
	}

	// save the new status to local db and the exchange
	status_changed, err := common.SetNodeManagementPolicyStatus(db, pDevice, fullName, &nmStatus, managementStatus, statusHandler)
	if err != nil {
		return errorHandler(NewSystemError(fmt.Sprintf("Error saving nmp status for %v: %v", fullName, err))), "", nil
	}

	// Send NM_STATUS_CHANGED message if status was changed, log the new status to the event log
	if status_changed {
		msgs = append(msgs, events.NewNMStatusChangedMessage(events.NM_STATUS_CHANGED, fullName, nmStatus.AgentUpgrade.Status))
		newNMPStatus := managementStatus.AgentUpgrade.Status
		if managementStatus.AgentUpgrade.ErrorMessage != "" {
			newNMPStatus += fmt.Sprintf(", ErrorMessage: %v", managementStatus.AgentUpgrade.ErrorMessage)
		}
		eventlog.LogNodeEvent(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_API_NMP_STATUS_CHANGE, pDevice.Org, nmpName, newNMPStatus), persistence.EC_NMP_STATUS_UPDATE_COMPLETE, pDevice.Id, pDevice.Org, pDevice.Pattern, pDevice.Config.State)
	}

	// Return message
	return false, fmt.Sprintf("Updated status for NMP %v.", fullName), msgs
}
