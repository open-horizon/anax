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

// This API deletes the selected event logs saved on the db.
func DeleteEventLogs(db *bolt.DB, prune bool, selections map[string][]string, msgPrinter *message.Printer) (int, error) {
	s := map[string][]persistence.Selector{}
	if prune {
		lastUnreg, err := persistence.GetLastUnregistrationTime(db)
		if err != nil {
			return 0, fmt.Errorf("Failed to get the last unregistration time stamp from db. %v", err)
		}

		s["timestamp"] = []persistence.Selector{persistence.Selector{Op: "<", MatchValue: lastUnreg}}

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Selectors for pruning are: %v.", s)))
	} else {
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Deleting event logs from the db. The selectors are: %v.", selections)))

		//convert to selectors
		var err error
		s, err = persistence.ConvertToSelectors(selections)
		if err != nil {
			return 0, fmt.Errorf(msgPrinter.Sprintf("Error converting the selections into Selectors: %v", err))
		} else {
			glog.V(5).Infof(apiLogString(fmt.Sprintf("Converted selections into a map of persistence.Selector arrays: %v.", s)))
		}
	}

	count, err := eventlog.DeleteEventLogs(db, s, msgPrinter)
	return count, err
}

func FindSurfaceLogsForOutput(db *bolt.DB, msgPrinter *message.Printer) ([]persistence.SurfaceError, error) {
	outputLogs := make([]persistence.SurfaceError, 0)
	surfaceLogs, err := persistence.FindSurfaceErrors(db)
	if err != nil {
		return nil, err
	}
	for _, log := range surfaceLogs {
		if !log.Hidden {
			outputLogs = append(outputLogs, log)
		}
	}
	return outputLogs, nil
}
