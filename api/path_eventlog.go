package api

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/eventlog"
	"github.com/open-horizon/anax/persistence"
	"golang.org/x/text/message"
	"sort"
)

// This API returns the event logs saved on the db.
func FindEventLogsForOutput(db *bolt.DB, all_logs bool, selections map[string][]string, msgPrinter *message.Printer) ([]persistence.EventLog, error) {

	glog.V(5).Infof(apiLogString(fmt.Sprintf("Getting event logs from the db. The selectors are: %v.", selections)))

	//convert to selectors
	s, err := persistence.ConvertToSelectors(selections)
	if err != nil {
		return nil, fmt.Errorf(msgPrinter.Sprintf("Error converting the selections into Selectors: %v", err))
	} else {
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Converted selections into a map of persistence.Selector arrays: %v.", s)))
	}

	// get the event logs
	if event_logs, err := eventlog.GetEventLogs(db, all_logs, s, msgPrinter); err != nil {
		return nil, err
	} else {
		sort.Sort(EventLogByRecordId(event_logs))
		return event_logs, nil
	}
}

func FindSurfaceLogsForOutput(db *bolt.DB, msgPrinter *message.Printer) ([]persistence.EventLog, error) {
	eventLogs := make([]persistence.EventLog, 0)

	if surfaceLogs, err := persistence.FindSurfaceErrors(db); err != nil {
		return nil, err
	} else {
		for _, log := range surfaceLogs {
			if !log.Hidden {
				recordSelector := []persistence.Selector{persistence.Selector{Op: "=", MatchValue: log.Record_id}}
				recordSelectorMap := make(map[string][]persistence.Selector)
				recordSelectorMap["record_id"] = recordSelector
				fullEventlog, err := eventlog.GetEventLogs(db, true, recordSelectorMap, msgPrinter)
				if err == nil && len(fullEventlog) > 0 {
					eventLogs = append(eventLogs, fullEventlog[0])
				}
			}
		}
	}
	return eventLogs, nil
}
