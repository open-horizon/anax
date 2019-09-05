package persistence

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"time"
)

const NODE_SURFACEERR = "nodesurfaceerror"

// The format for node eventlog errors surfaced to the exchange
type SurfaceError struct {
	Record_id  string       `json:"record_id"`
	Message    string       `json:"message"`
	Event_code string       `json:"event_code"`
	Hidden     bool         `json:"hidden"`
	Workload   WorkloadInfo `json:"workload"`
	Timestamp  string       `json:"timestamp"`
}

// FindSurfaceErrors returns the surface errors currently in the local db
func FindSurfaceErrors(db *bolt.DB) ([]SurfaceError, error) {
	var surfaceErrors []SurfaceError

	readErr := db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(NODE_SURFACEERR)); b != nil {
			return b.ForEach(func(k, v []byte) error {

				if err := json.Unmarshal(v, &surfaceErrors); err != nil {
					return fmt.Errorf("Unable to deserialize node surface error record: %v", v)
				}

				return nil
			})
		}

		return nil // end transaction
	})

	if readErr != nil {
		return nil, readErr
	}
	return surfaceErrors, nil
}

// SaveSurfaceErrors saves the provided list of surface errors to the local db
func SaveSurfaceErrors(db *bolt.DB, surfaceErrors []SurfaceError) error {
	writeErr := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(NODE_SURFACEERR))
		if err != nil {
			return err
		}

		if serial, err := json.Marshal(surfaceErrors); err != nil {
			return fmt.Errorf("Failed to serialize surface errors: %v. Error: %v", surfaceErrors, err)
		} else {
			return b.Put([]byte(NODE_SURFACEERR), serial)
		}
	})

	return writeErr
}

// NewErrorLog takes an eventLog object and puts it in the local db and exchange if it should be surfaced
func NewErrorLog(db *bolt.DB, eventLog EventLog) bool {
	if !IsSurfaceType(eventLog.EventCode) || !(eventLog.SourceType == SRC_TYPE_AG || eventLog.SourceType == SRC_TYPE_SVC) {
		return false
	}
	currentErrors, err := FindSurfaceErrors(db)
	if err != nil {
		glog.V(3).Infof("Error getting surface errors from local db. %v", err)
		return false
	}
	found := false
	for i, currentError := range currentErrors {
		if MatchWorkload(GetEventLogObject(db, nil, currentError.Record_id), GetEventLogObject(db, nil, eventLog.Id)) {
			hiddenField := currentError.Hidden
			if eventLog.EventCode != currentError.Event_code {
				hiddenField = false
			}
			currentErrors[i] = NewSurfaceError(eventLog)
			currentErrors[i].Hidden = hiddenField
			found = true
		}
	}
	if !found {
		currentErrors = append(currentErrors, NewSurfaceError(eventLog))
	}
	if err = SaveSurfaceErrors(db, currentErrors); err != nil {
		glog.V(3).Infof("Error saving surface errors to local db. %v", err)
	}
	return true
}

// getErrorTypeList returns a slice containing the error types to surface to the exchange
func getErrorTypeList() []string {
	return []string{EC_ERROR_IMAGE_LOADE, EC_ERROR_IN_DEPLOYMENT_CONFIG, EC_ERROR_START_CONTAINER}
}

// IsSurfaceType returns true if the string parameter is a type to surface to the exchange
func IsSurfaceType(errorType string) bool {
	for _, surfaceType := range getErrorTypeList() {
		if errorType == surfaceType {
			return true
		}
	}
	return false
}

// MatchWorkload function checks if the 2 eventlog parameters have matching workloads
func MatchWorkload(error1 EventLog, error2 EventLog) bool {
	var source1Workload WorkloadInfo
	var source2Workload WorkloadInfo
	if source1, ok := error1.Source.(AgreementEventSource); ok {
		source1Workload = WorkloadInfo{URL: source1.RunningWorkload.URL, Org: source1.RunningWorkload.Org, Arch: source1.RunningWorkload.Arch, Version: source1.RunningWorkload.Version}
	} else if source1, ok := error1.Source.(ServiceEventSource); ok {
		source1Workload = WorkloadInfo{URL: source1.ServiceUrl, Org: source1.Org, Arch: source1.Arch, Version: source1.Version}
	} else {
		return false
	}
	if source2, ok := error2.Source.(AgreementEventSource); ok {
		source2Workload = WorkloadInfo{URL: source2.RunningWorkload.URL, Org: source2.RunningWorkload.Org, Arch: source2.RunningWorkload.Arch, Version: source2.RunningWorkload.Version}
	} else if source2, ok := error2.Source.(ServiceEventSource); ok {
		source2Workload = WorkloadInfo{URL: source2.ServiceUrl, Org: source2.Org, Arch: source2.Arch, Version: source2.Version}
	} else {
		return false
	}

	return (source1Workload.URL == source2Workload.URL && source1Workload.Org == source2Workload.Org)
}

// NewSurfaceError returns a surface error from the eventlog parameter
func NewSurfaceError(eventLog EventLog) SurfaceError {
	timestamp := time.Unix((int64)(eventLog.Timestamp), 0).String()
	return SurfaceError{Record_id: eventLog.Id, Message: fmt.Sprintf("%s: %v", eventLog.MessageMeta.MessageKey, eventLog.MessageMeta.MessageArgs), Event_code: eventLog.EventCode, Hidden: false, Workload: GetWorkloadInfo(eventLog), Timestamp: timestamp}
}

func GetWorkloadInfo(eventLog EventLog) WorkloadInfo {
	if source, ok := eventLog.Source.(AgreementEventSource); ok {
		return WorkloadInfo{URL: source.RunningWorkload.URL, Org: source.RunningWorkload.Org, Arch: source.RunningWorkload.Arch, Version: source.RunningWorkload.Version}
	} else if source, ok := eventLog.Source.(*AgreementEventSource); ok {
		return WorkloadInfo{URL: source.RunningWorkload.URL, Org: source.RunningWorkload.Org, Arch: source.RunningWorkload.Arch, Version: source.RunningWorkload.Version}
	} else if source, ok := eventLog.Source.(ServiceEventSource); ok {
		return WorkloadInfo{URL: source.ServiceUrl, Org: source.Org, Arch: source.Arch, Version: source.Version}
	} else if source, ok := eventLog.Source.(*ServiceEventSource); ok {
		return WorkloadInfo{URL: source.ServiceUrl, Org: source.Org, Arch: source.Arch, Version: source.Version}
	}
	return WorkloadInfo{}
}
