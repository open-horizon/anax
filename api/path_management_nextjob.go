package api

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/persistence"
	"time"
)

func FindManagementNextJobForOutput(jobType, ready string, errorHandler ErrorHandler, db *bolt.DB) (bool, map[string]*exchangecommon.NodeManagementPolicyStatus) {
	var err error
	managementStatuses := make(map[string]*exchangecommon.NodeManagementPolicyStatus, 0)
	returnStatus := make(map[string]*exchangecommon.NodeManagementPolicyStatus, 0)

	// The jobType filter currently only supports "agentUpgrade"
	if jobType == "agentUpgrade" || jobType == "" {

		// Only get statuses that are "downloaded" (ready)
		if ready == "true" {
			filters := []persistence.NMStatusFilter{persistence.StatusNMSFilter(exchangecommon.STATUS_DOWNLOADED)}
			if managementStatuses, err = persistence.FindNMPStatusWithFilters(db, filters); err != nil {
				return errorHandler(NewSystemError(fmt.Sprintf("unable to read management status object, error %v", err))), nil
			} 
		// Only get statuses that are NOT "downloaded" (not ready)
		} else if ready == "false" {
			filters := []persistence.NMStatusFilter {persistence.StatusNMSFilter(exchangecommon.STATUS_NEW)}
			if managementStatuses, err = persistence.FindNMPStatusWithFilters(db, filters); err != nil {
				return errorHandler(NewSystemError(fmt.Sprintf("unable to read management status object, error %v", err))), nil
			} 
		// Get all statuses
		} else if ready == "" {
			var errHandled bool
			if errHandled, managementStatuses = FindManagementStatusForOutput("", "", errorHandler, db); errHandled {
				return false, nil
			}
		} else {
			return errorHandler(NewSystemError(fmt.Sprintf("invalid value for \"ready\" argument."))), nil
		}

		// Loop through found statuses
		if managementStatuses != nil && len(managementStatuses) > 0 {
			var nextTime time.Time
			var nmpNameToReturn string

			// Loop through all NMP statuses and set next job to return
			for nmpName, nmpStatus := range managementStatuses {
				if nmpStatus.AgentUpgrade.ScheduledTime == "" {
					continue
				}
				if scheduledTime, err := time.Parse(time.RFC3339, nmpStatus.AgentUpgrade.ScheduledTime); err != nil {
					return errorHandler(NewSystemError(fmt.Sprintf("unable to read management status scheduled time, error %v", err))), nil
				} else if nmpNameToReturn == "" {
					nextTime = scheduledTime
					nmpNameToReturn = nmpName
				} else if scheduledTime.Before(nextTime) {
					nextTime = scheduledTime
					nmpNameToReturn = nmpName
				}
			}
			if nmpNameToReturn != "" {
				returnStatus[nmpNameToReturn] = managementStatuses[nmpNameToReturn]
			}
		}
	} else {
		return errorHandler(NewSystemError(fmt.Sprintf("jobType currently only supports \"agentUpgrade\"."))), nil
	}

	return false, returnStatus
}
