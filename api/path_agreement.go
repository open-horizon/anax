package api

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"sort"
)

func FindAgreementsForOutput(db *bolt.DB) (map[string]map[string][]persistence.EstablishedAgreement, error) {

	agreements, err := persistence.FindEstablishedAgreementsAllProtocols(db, policy.AllAgreementProtocols(), []persistence.EAFilter{})
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to read agreement objects, error %v", err))
	}

	// The output is map that contains a set of active agreements and a set of archived agreements.
	var agreementsKey = "agreements"
	var archivedKey = "archived"
	var activeKey = "active"

	wrap := make(map[string]map[string][]persistence.EstablishedAgreement, 0)
	wrap[agreementsKey] = make(map[string][]persistence.EstablishedAgreement, 0)
	wrap[agreementsKey][archivedKey] = []persistence.EstablishedAgreement{}
	wrap[agreementsKey][activeKey] = []persistence.EstablishedAgreement{}

	for _, agreement := range agreements {
		// The archived agreements and the agreements being terminated are returned as archived.
		if agreement.Archived || agreement.AgreementTerminatedTime != 0 {
			wrap[agreementsKey][archivedKey] = append(wrap[agreementsKey][archivedKey], agreement)
		} else {
			wrap[agreementsKey][activeKey] = append(wrap[agreementsKey][activeKey], agreement)
		}
	}

	// do sorts
	sort.Sort(EstablishedAgreementsByAgreementCreationTime(wrap[agreementsKey][activeKey]))
	sort.Sort(EstablishedAgreementsByAgreementTerminatedTime(wrap[agreementsKey][archivedKey]))

	return wrap, nil
}

func DeleteAgreement(errorhandler ErrorHandler, agreementId string, db *bolt.DB) (bool, *events.ApiAgreementCancelationMessage) {

	glog.V(3).Infof("Handling DELETE of agreement: %v", agreementId)

	var filters []persistence.EAFilter
	filters = append(filters, persistence.UnarchivedEAFilter())
	filters = append(filters, persistence.IdEAFilter(agreementId))

	agreements, err := persistence.FindEstablishedAgreementsAllProtocols(db, policy.AllAgreementProtocols(), filters)
	if err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("unable to read agreement objects, error %v", err))), nil
	} else if len(agreements) == 0 {
		return errorhandler(NewNotFoundError(fmt.Sprintf("no agreements in local database"))), nil
	}

	// Deletion is actually handled asynchronously. If the agreement is already terminating there is nothing to do.
	var msg *events.ApiAgreementCancelationMessage
	if agreements[0].AgreementTerminatedTime == 0 {
		msg = events.NewApiAgreementCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, agreements[0].AgreementProtocol, agreements[0].CurrentAgreementId, agreements[0].CurrentDeployment)
	}

	return false, msg
}
