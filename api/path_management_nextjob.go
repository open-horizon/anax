package api

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/persistence"
	"time"
)

func FindManagementNextJobForOutput(jobType, ready string, db *bolt.DB) (map[string]*exchangecommon.NodeManagementPolicyStatus, error) {
	var err error
	managementStatuses := make(map[string]*exchangecommon.NodeManagementPolicyStatus, 0)
	returnStatus := make(map[string]*exchangecommon.NodeManagementPolicyStatus, 0)

	// The jobType filter currently only supports "agentUpgrade"
	if jobType == "agentUpgrade" || jobType == "" {

		// Only get statuses that are "downloaded" (ready)
		if ready == "true" {
			filters := []persistence.NMStatusFilter{persistence.StatusNMSFilter(exchangecommon.STATUS_DOWNLOADED)}
			if managementStatuses, err = persistence.FindNMPStatusWithFilters(db, filters); err != nil {
				return nil, errors.New(fmt.Sprintf("unable to read management status object, error %v", err))
			} 
		// Only get statuses that are NOT "downloaded" (not ready)
		} else if ready == "false" {
			filters := []persistence.NMStatusFilter {persistence.StatusNMSFilter(exchangecommon.STATUS_NEW)}
			if managementStatuses, err = persistence.FindNMPStatusWithFilters(db, filters); err != nil {
				return nil, errors.New(fmt.Sprintf("unable to read management status object, error %v", err))
			} 
		// Get all statuses
		} else if ready == "" {
			if managementStatuses, err = FindManagementStatusForOutput("", "", db); err != nil {
				return nil, err
			}
		} else {
			return nil, errors.New(fmt.Sprintf("invalid value for \"ready\" argument."))
		}

		// Loop through found statuses
		if len(managementStatuses) > 0 {
			var nextTime time.Time
			var nmpNameToReturn string

			// Look at first NMP status and set it to return
			for nmpName, nmpStatus := range managementStatuses {
				if nextTime, err = time.Parse(time.RFC3339, nmpStatus.AgentUpgrade.ScheduledTime); err != nil {
					return nil, errors.New(fmt.Sprintf("unable to read management status scheduled time, error %v", err))
				}
				nmpNameToReturn = nmpName
				break
			}

			// Loop through all NMP statuses and set next job to return
			for nmpName, nmpStatus := range managementStatuses {
				if scheduledTime, err := time.Parse(time.RFC3339, nmpStatus.AgentUpgrade.ScheduledTime); err != nil {
					return nil, errors.New(fmt.Sprintf("unable to read management status scheduled time, error %v", err))
				} else if scheduledTime.Before(nextTime) {
					nextTime = scheduledTime
					nmpNameToReturn = nmpName
				}
			}
			returnStatus[nmpNameToReturn] = managementStatuses[nmpNameToReturn]
		}
	} else {
		return nil, errors.New(fmt.Sprintf("jobType currently only supports \"agentUpgrade\"."))
	}

	return returnStatus, nil
}
