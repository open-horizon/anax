package api

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/persistence"
	"time"
)

func FindManagementStatusForOutput(nmpName, orgName string, errorHandler ErrorHandler, db *bolt.DB) (bool, map[string]*exchangecommon.NodeManagementPolicyStatus) {
	var err error
	managementStatuses := make(map[string]*exchangecommon.NodeManagementPolicyStatus, 0)

	var fullName string
	if orgName == "" {
		// Find exchange device in DB
		pDevice, err := persistence.FindExchangeDevice(db)
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

func UpdateManagementStatus(nmStatus managementStatusInput, errorHandler ErrorHandler, statusHandler exchange.PutNodeManagementPolicyStatusHandler, nmpName string, db *bolt.DB) (bool, string, []*events.NMStatusChangedMessage) {
	if nmStatus.Type == "agentUpgrade" {
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
			return errorHandler(NewSystemError(fmt.Sprintf("unable to read management status object, error %v", err))), "", nil
		} else if managementStatus == nil {
			return errorHandler(NewNotFoundError(fmt.Sprintf("The nmp %v cannot be found", nmpName), "management")), "", nil
		}

		// Initialize agent upgrade status if it does not exist
		if managementStatus.AgentUpgrade == nil {
			managementStatus.AgentUpgrade = new(exchangecommon.AgentUpgradePolicyStatus)
		}

		// Set the actual start time and completion time depending on new status
		if nmStatus.Status == exchangecommon.STATUS_INITIATED && managementStatus.AgentUpgrade.ActualStartTime == "" {
			managementStatus.AgentUpgrade.ActualStartTime = time.Now().Format(time.RFC3339)
		} else if nmStatus.Status == exchangecommon.STATUS_SUCCESSFUL && managementStatus.AgentUpgrade.CompletionTime == "" {
			managementStatus.AgentUpgrade.CompletionTime = time.Now().Format(time.RFC3339)
		}
		managementStatus.AgentUpgrade.ErrorMessage = nmStatus.ErrorMessage

		// Send NM_STATUS_CHANGED message if status was changed
		if nmStatus.Status != "" && managementStatus.AgentUpgrade.Status != nmStatus.Status {
			managementStatus.AgentUpgrade.Status = nmStatus.Status
			msgs = append(msgs, events.NewNMStatusChangedMessage(events.NM_STATUS_CHANGED, nmStatus.Status))
		}
		
		// Update the NMP status in the local db
		if err := persistence.SaveOrUpdateNMPStatus(db, fullName, *managementStatus); err != nil {
			return errorHandler(NewSystemError(fmt.Sprintf("Unable to update node management status object, error %v", err))), "", nil
		} 

		// Update the status of the NMP in the exchange
		// if _, err := statusHandler(pDevice.Org, pDevice.Id, nmpName, managementStatus); err != nil {
		// 	return errorhandler(NewSystemError(fmt.Sprintf("Unable to update node management status object in the exchange, error %v", err))), "", nil
		// }

		// Return message
		return false, fmt.Sprintf("Updated status for NMP %v.", fullName), msgs

	} else if nmStatus.Type != "" {
		return errorHandler(NewBadRequestError(fmt.Sprintf("Input's \"type\" field currently only supports \"agentUpgrade\"."))), "", nil
	}

	return errorHandler(NewBadRequestError(fmt.Sprintf("Input does not include \"type\" field."))), "", nil
}
