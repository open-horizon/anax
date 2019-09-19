package exchangesync

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"golang.org/x/text/message"
	"time"
)

// UpdateSurfaceErrors is called periodically by a subwoker under the agreement worker.
// This will close any surfaced errors that have persistent related agreements and update the local and exchange copies.
// The node copy is the master EXCEPT for the hidden field of each error.
func UpdateSurfaceErrors(db *bolt.DB, pDevice persistence.ExchangeDevice, getErrors exchange.SurfaceErrorsHandler, putErrors exchange.PutSurfaceErrorsHandler, serviceResolverHandler exchange.ServiceResolverHandler, errorTimeout int, agreementPersistentTime int) int {
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
				if !HasPersistentAgreement(db, serviceResolverHandler, pDevice, nil, dbError, agreementPersistentTime) {
					updatedExchLogs = append(updatedExchLogs, dbError)

				}
			}
		}

		err = persistence.SaveSurfaceErrors(db, updatedExchLogs)
		if err != nil {
			glog.Errorf("Error saving surface errors to local db. %v", err)
		}
		return 0
	}

	updated := false
	for _, dbError := range dbErrors {
		fullDbError := persistence.GetEventLogObject(db, nil, dbError.Record_id)
		if errorTimeout == 0 || time.Since(time.Unix(int64(persistence.GetEventLogObject(db, nil, dbError.Record_id).Timestamp), 0)).Seconds() < float64(errorTimeout) {
			if !HasPersistentAgreement(db, serviceResolverHandler, pDevice, nil, dbError, agreementPersistentTime) {
				match_found := false
				for _, exchError := range exchErrors {
					if persistence.MatchWorkload(fullDbError, persistence.GetEventLogObject(db, nil, exchError.Record_id)) && dbError.Event_code == dbError.Event_code {
						dbError.Hidden = exchError.Hidden
						if dbError.Record_id != exchError.Record_id {
							updated = true
						}
						match_found = true
					}
				}
				if !match_found {
					updated = true
				}
				updatedExchLogs = append(updatedExchLogs, dbError)
			} else {
				updated = true
			}
		} else {
			updated = true
		}
	}

	err = persistence.SaveSurfaceErrors(db, updatedExchLogs)
	if err != nil {
		glog.Errorf("Error saving surface errors to local db. %v", err)
	}
	if updated {
		PutExchangeSurfaceErrors(&pDevice, putErrors, updatedExchLogs)
	}
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
func HasPersistentAgreement(db *bolt.DB, serviceResolverHandler exchange.ServiceResolverHandler, pDevice persistence.ExchangeDevice, msgPrinter *message.Printer, errorLog persistence.SurfaceError, agreementPersistentTime int) bool {
	eventLog := persistence.GetEventLogObject(db, msgPrinter, errorLog.Record_id)
	workload := persistence.GetWorkloadInfo(eventLog)

	allServices, err := getAllServicesFromAgreements(db, serviceResolverHandler, agreementPersistentTime)
	if err != nil {
		glog.V(3).Infof("Error getting the services for active agreements.")
		return false
	}

	for _, service := range *allServices {
		if (service.SpecRef == workload.URL || workload.URL == "") && (service.Org == workload.Org || workload.Org == "") {
			return true
		}
	}

	return false
}

// get the all the top level and dependent services the given agreements are using
func getAllServicesFromAgreements(db *bolt.DB, serviceResolverHandler exchange.ServiceResolverHandler, agreementPersistentTime int) (*policy.APISpecList, error) {

	ags, err := persistence.FindEstablishedAgreementsAllProtocols(db, policy.AllAgreementProtocols(), []persistence.EAFilter{persistence.UnarchivedEAFilter(), PersistingEAFilter(agreementPersistentTime)})

	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve unarchived agreements from database. %v", err)
	}

	apiSpecs := new(policy.APISpecList)

	if ags != nil {
		for _, ag := range ags {
			workload := ag.RunningWorkload
			if workload.URL == "" || workload.Org == "" {
				continue
			}

			apiSpecs.Add_API_Spec(policy.APISpecification_Factory(workload.URL, workload.Org, "", workload.Arch))

			asl, _, _, err := serviceResolverHandler(workload.URL, workload.Org, workload.Version, workload.Arch)
			if err != nil {
				return nil, fmt.Errorf((fmt.Sprintf("error searching for service details %v, error: %v", workload, err)))
			}

			if asl != nil {
				for _, s := range *asl {
					apiSpecs.Add_API_Spec(policy.APISpecification_Factory(s.SpecRef, s.Org, "", s.Arch))
				}
			}
		}
	}

	return apiSpecs, nil
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
