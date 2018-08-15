package api

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/eventlog"
	"github.com/open-horizon/anax/persistence"
	"sort"
)

// This API returns the event logs saved on the db.
func FindEventLogsForOutput(db *bolt.DB, all_logs bool, selections map[string][]string) ([]persistence.EventLog, error) {

	glog.V(5).Infof(apiLogString(fmt.Sprintf("Getting event logs from the db. The selectors are: %v.", selections)))

	//convert to selectors
	s, err := persistence.ConvertToSelectors(selections)
	if err != nil {
		return nil, fmt.Errorf("Error converting the selections into Selectors: %v", err)
	} else {
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Converted selections into a map of persistence.Selector arrays: %v.", s)))
	}

	// get the event logs
	if event_logs, err := eventlog.GetEventLogs(db, all_logs, s); err != nil {
		return nil, err
	} else {
		sort.Sort(EventLogByRecordId(event_logs))
		return event_logs, nil
	}
}
