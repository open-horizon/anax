package persistence

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
)

const NODE_SURFACEERR = "nodesurfaceerror"

// The format for node eventlog errors surfaced to the exchange
type SurfaceError struct {
	Record_id  string `json:"record_id"`
	Message    string `json:"message"`
	Event_code string `json:"event_code"`
	Hidden     bool   `json:"hidden"`
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
	if !IsSurfaceType(eventLog.EventCode) {
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
			currentErrors[i] = newSurfaceError(eventLog)
			currentErrors[i].Hidden = hiddenField
			found = true
		}
	}
	if !found {
		currentErrors = append(currentErrors, newSurfaceError(eventLog))
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
	source1, ok1 := error1.Source.(AgreementEventSource)
	source2, ok2 := error2.Source.(AgreementEventSource)

	if !ok1 || !ok2 {
		return false
	}

	return (source1.RunningWorkload.Arch == source2.RunningWorkload.Arch && source1.RunningWorkload.Org == source2.RunningWorkload.Org && source1.RunningWorkload.URL == source2.RunningWorkload.URL && source1.RunningWorkload.Version == source2.RunningWorkload.Version)
}

// newSurfaceError returns a surface error from the eventlog parameter
func newSurfaceError(eventLog EventLog) SurfaceError {
	return SurfaceError{Record_id: eventLog.Id, Message: fmt.Sprintf("%s: %v", eventLog.MessageMeta.MessageKey, eventLog.MessageMeta.MessageArgs), Event_code: eventLog.EventCode, Hidden: false}
}
