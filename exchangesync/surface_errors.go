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

// UpdateSurfaceErrors is called when a node's errors need to be surfaced to the exchange.
// This will close any surfaced errors that have persistent related agreements and update the local and exchange copies.
// The node copy is the master EXCEPT for the hidden field of each error.
func UpdateSurfaceErrors(db *bolt.DB, pDevice persistence.ExchangeDevice, exchErrors []persistence.SurfaceError, putErrors exchange.PutSurfaceErrorsHandler, serviceResolverHandler exchange.ServiceResolverHandler, errorTimeout int, agreementPersistentTime int) int {
	updatedExchLogs := make([]persistence.SurfaceError, 0, 5)

	glog.V(5).Infof("Checking on errors to surface")

	// Get the list of surfaced errors from the local DB. This list of errors should also be in the exchange, but
	// be careful because it might not be there.
	dbErrors, err := persistence.FindSurfaceErrors(db)
	if err != nil {
		glog.Errorf("Error getting surface errors from the local db. %v", err)
		return 0
	}

	// Run through all the currently logged errors to see if any of them have been resolved, or hidden. Or, they might
	// be new and need to be written to the exchange.
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
				// If the current error log record in the local DB has not been written to the exchange yet, then make sure the
				// current list of errors is written to the exchange.
				if !match_found {
					updated = true
				}
				updatedExchLogs = append(updatedExchLogs, dbError)
				glog.V(5).Infof("Collecting errors to surface: %v", dbError)
			} else {
				// We found an error in the local DB that has a persistent agreement, so it should be removed from the list of
				// errors surface to the exchange. Bye not adding it to the current running list, it will be removed when the
				// list is saved.
				updated = true
			}
		} else {
			// The error is too old to be included anymore so omit it from the running list of errors.
			updated = true
		}
	}

	glog.V(5).Infof("Saving errors to surface locally: %v", updatedExchLogs)
	err = persistence.SaveSurfaceErrors(db, updatedExchLogs)
	if err != nil {
		glog.Errorf("Error saving surface errors to local db. %v", err)
	}
	if updated {
		err := PutExchangeSurfaceErrors(&pDevice, putErrors, updatedExchLogs)
		if err != nil {
			glog.Errorf("Error putting surface errors to the exchange. %v", err)
		}
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

func PutExchangeSurfaceErrors(pDevice *persistence.ExchangeDevice, putErrors exchange.PutSurfaceErrorsHandler, errors []persistence.SurfaceError) error {
	errorList := exchange.ExchangeSurfaceError{ErrorList: errors}
	_, err := putErrors(fmt.Sprintf("%v/%v", pDevice.Org, pDevice.Id), &errorList)
	if err != nil {
		return err
	}

	return nil
}
