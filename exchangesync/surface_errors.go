package exchangesync

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"golang.org/x/text/message"
	"time"
)

// UpdateSurfaceErrors is called periodically by a subwoker under the agreement worker.
// This will close any surfaced errors that have persistent related agreements and update the local and exchange copies.
// The node copy is the master EXCEPT for the hidden field of each error.
func UpdateSurfaceErrors(db *bolt.DB, pDevice persistence.ExchangeDevice, getErrors exchange.SurfaceErrorsHandler, putErrors exchange.PutSurfaceErrorsHandler, errorTimeout int, agreementPersistentTime int) int {
	var updatedExchLogs []persistence.SurfaceError

	dbErrors, err := persistence.FindSurfaceErrors(db)
	if err != nil {
		glog.Infof("Error getting surface errors from the local db. %v", err)
		return 0
	}

	err, exchErrors := GetExchangeSurfaceErrors(&pDevice, getErrors)
	if err != nil {
		for _, dbError := range dbErrors {
			if errorTimeout == 0 || time.Since(time.Unix(int64(persistence.GetEventLogObject(db, nil, dbError.Record_id).Timestamp), 0)).Seconds() < float64(errorTimeout) {
				if !HasPersistentAgreement(db, pDevice, nil, dbError.Record_id, agreementPersistentTime) {
					updatedExchLogs = append(updatedExchLogs, dbError)

				}
			}
		}

		persistence.SaveSurfaceErrors(db, updatedExchLogs)
		return 0
	}

	for _, dbError := range dbErrors {
		fullDbError := persistence.GetEventLogObject(db, nil, dbError.Record_id)
		if errorTimeout == 0 || time.Since(time.Unix(int64(persistence.GetEventLogObject(db, nil, dbError.Record_id).Timestamp), 0)).Seconds() < float64(errorTimeout) {
			if !HasPersistentAgreement(db, pDevice, nil, dbError.Record_id, agreementPersistentTime) {
				for _, exchError := range exchErrors {
					if persistence.MatchWorkload(fullDbError, persistence.GetEventLogObject(db, nil, exchError.Record_id)) && dbError.Event_code == dbError.Event_code {
						dbError.Hidden = exchError.Hidden
					}
				}
				updatedExchLogs = append(updatedExchLogs, dbError)
			}
		}
	}

	persistence.SaveSurfaceErrors(db, updatedExchLogs)
	PutExchangeSurfaceErrors(&pDevice, putErrors, updatedExchLogs)
	return 0
}

// PersistingEAFilter is a filter for searching for agreements that have persisted for a certain period.
func PersistingEAFilter(agreementPersistentTime int) persistence.EAFilter {
	return func(e persistence.EstablishedAgreement) bool {
		if e.AgreementFinalizedTime == 0 {
			return false
		}

		return time.Since(time.Unix(int64(e.AgreementFinalizedTime), 0)).Seconds() > float64(agreementPersistentTime)
	}
}

// HasPersistentAgreement takes a recordID and returns true if there is a persistent agreement with the same workload on the node.
func HasPersistentAgreement(db *bolt.DB, pDevice persistence.ExchangeDevice, msgPrinter *message.Printer, recordID string, agreementPersistentTime int) bool {
	eventLog := persistence.GetEventLogObject(db, msgPrinter, recordID)

	acitvePersistAgreements, err := persistence.FindEstablishedAgreements(db, "Basic", []persistence.EAFilter{persistence.UnarchivedEAFilter(), PersistingEAFilter(agreementPersistentTime)})
	if err != nil {
		return false
	}

	for _, agreement := range acitvePersistAgreements {
		urlSelector := make(map[string][]persistence.Selector)
		orgSelector := make(map[string][]persistence.Selector)
		versSelector := make(map[string][]persistence.Selector)
		archSelector := make(map[string][]persistence.Selector)
		urlSelector["workload_to_run.url"] = []persistence.Selector{persistence.Selector{Op: "=", MatchValue: agreement.RunningWorkload.URL}}
		orgSelector["workload_to_run.org"] = []persistence.Selector{persistence.Selector{Op: "=", MatchValue: agreement.RunningWorkload.Org}}
		versSelector["workload_to_run.version"] = []persistence.Selector{persistence.Selector{Op: "=", MatchValue: agreement.RunningWorkload.Version}}
		archSelector["workload_to_run.arch"] = []persistence.Selector{persistence.Selector{Op: "=", MatchValue: agreement.RunningWorkload.Arch}}
		if eventLog.Matches(urlSelector) && eventLog.Matches(orgSelector) && eventLog.Matches(versSelector) && eventLog.Matches(archSelector) {
			return true
		}
	}
	return false
}

// This function returns the node errors currently surfaced to the exchange
func GetExchangeSurfaceErrors(pDevice *persistence.ExchangeDevice, getErrors exchange.SurfaceErrorsHandler) (error, []persistence.SurfaceError) {
	exchErrors, err := getErrors(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id))
	if err != nil {
		return err, nil
	}

	return nil, exchErrors.ErrorList
}

func PutExchangeSurfaceErrors(pDevice *persistence.ExchangeDevice, putErrors exchange.PutSurfaceErrorsHandler, errors []persistence.SurfaceError) error {
	errorList := exchange.ExchangeSurfaceError{ErrorList: errors}
	_, err := putErrors(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id), &errorList)
	if err != nil {
		return err
	}

	return nil
}
