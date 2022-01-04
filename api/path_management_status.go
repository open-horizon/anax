package api

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/persistence"
	"time"
)

func FindManagementStatusForOutput(nmpName, org string, db *bolt.DB) (map[string]*exchangecommon.NodeManagementPolicyStatus, error) {
	var err error
	managementStatuses := make(map[string]*exchangecommon.NodeManagementPolicyStatus, 0)

	// Look for given NMP status
	if nmpName != "" {
		fullName := org + "/" + nmpName
		var managementStatus *exchangecommon.NodeManagementPolicyStatus
		if managementStatus, err = persistence.FindNMPStatus(db, fullName); err != nil {
			return nil, errors.New(fmt.Sprintf("unable to read management status object, error %v", err))
		} else if managementStatus != nil {
			managementStatuses[fullName] = managementStatus
		}
		
	// Return all stored NMP statuses
	} else {
		if managementStatuses, err = persistence.FindAllNMPStatus(db); err != nil {
			return nil, errors.New(fmt.Sprintf("unable to read management status object, error %v", err))
		}
	}

	return managementStatuses, nil
}

func UpdateManagementStatus(nmStatus managementStatusInput, errorhandler ErrorHandler, statusHandler exchange.PutNodeManagementPolicyStatusHandler, nmpName string, pDevice *persistence.ExchangeDevice, db *bolt.DB) (bool, string, []*events.NMStatusChangedMessage) {
	if nmStatus.Type == "agentUpgrade" {
		// Slice to store events
		msgs := make([]*events.NMStatusChangedMessage, 0, 10)
	
		var err error
		var managementStatus *exchangecommon.NodeManagementPolicyStatus
		exists := false
		fullName := string(pDevice.Org) + "/" + string(nmpName)

		// Check to see if status is already being stored
		if managementStatus, err = persistence.FindNMPStatus(db, fullName); err != nil {
			return errorhandler(NewSystemError(fmt.Sprintf("unable to read management status object, error %v", err))), "", nil
		} else if managementStatus != nil {
			exists = true
		}

		// Create new status if one does not exist
		if managementStatus == nil {
			managementStatus = &exchangecommon.NodeManagementPolicyStatus{}
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
			return errorhandler(NewSystemError(fmt.Sprintf("Unable to update node management status object, error %v", err))), "", nil
		} 

		// Update the status of the NMP in the exchange
		// if _, err := statusHandler(pDevice.Org, pDevice.Id, nmpName, managementStatus); err != nil {
		// 	return errorhandler(NewSystemError(fmt.Sprintf("Unable to update node management status object in the exchange, error %v", err))), "", nil
		// }

		// Return message
		if exists {
			return false, fmt.Sprintf("Updated status for NMP %v.", fullName), msgs
		}
		return false, fmt.Sprintf("Added status for NMP %v.", fullName), msgs

	} else if nmStatus.Type != "" {
		return errorhandler(NewBadRequestError(fmt.Sprintf("Input's \"type\" field currently only supports \"agentUpgrade\"."))), "", nil
	}

	return errorhandler(NewBadRequestError(fmt.Sprintf("Input does not include \"type\" field."))), "", nil
}
