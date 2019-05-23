package governance

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/eventlog"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/microservice"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/producer"
)

// Each microservice def in the database has a policy generated for it, that needs to be re-generated and published back to the exchange.
func (w *GovernanceWorker) handleUpdatePolicy(cmd *UpdatePolicyCommand) {

	// for the non pattern case, no policy files are needed. TODO-New: what should we do here?
	if w.devicePattern == "" {
		return
	}

	// Find the microservice definitions in our database so that we can update the policy for each one.
	msDefs, err := persistence.FindMicroserviceDefs(w.db, []persistence.MSFilter{persistence.UnarchivedMSFilter()})
	if err != nil {
		glog.Errorf(logString(fmt.Sprintf("Unable to update policies, find service definitions from the database, error %v", err)))
		return
	}

	// For each microservice def, generate a new policy. The GenMicroservicePolicy function will write the new policy to disk and
	// it will issue events that trigger the node to update its service advertisement in the exchange.
	for _, msdef := range msDefs {

		glog.V(5).Infof(logString(fmt.Sprintf("Working on msdef: %v", msdef)))

		if err := microservice.GenMicroservicePolicy(&msdef, w.BaseWorker.Manager.Config.Edge.PolicyPath, w.db, w.BaseWorker.Manager.Messages, exchange.GetOrg(w.GetExchangeId()), w.devicePattern); err != nil {
			glog.Errorf(logString(fmt.Sprintf("Unable to update policy for %v, error %v", msdef, err)))
		}
	}

	glog.V(5).Infof(logString(fmt.Sprintf("Policies updated")))

}

// When node policy gets updated or deleted, all the agreements wil need
// to be canceled so that new negotiation can start
func (w *GovernanceWorker) handleNodePolicyUpdated() {
	glog.V(5).Infof(logString(fmt.Sprintf("handling node policy changes")))

	// get all the unarchived agreements
	agreements, err := persistence.FindEstablishedAgreementsAllProtocols(w.db, policy.AllAgreementProtocols(), []persistence.EAFilter{persistence.UnarchivedEAFilter()})
	if err != nil {
		glog.Errorf(logString(fmt.Sprintf("Unable to retrieve all the  from the database, error %v", err)))
		return
	}

	for _, ag := range agreements {
		agreementId := ag.CurrentAgreementId
		if ag.AgreementTerminatedTime != 0 && ag.AgreementForceTerminatedTime == 0 {
			glog.V(3).Infof(logString(fmt.Sprintf("skip agreement %v, it is already terminating", agreementId)))
		} else {
			glog.V(3).Infof(logString(fmt.Sprintf("ending the agreement: %v", agreementId)))

			reason := w.producerPH[ag.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_POLICY_CHANGED)

			eventlog.LogAgreementEvent(
				w.db,
				persistence.SEVERITY_INFO,
				fmt.Sprintf("Start terminating agreement for %v. Termination reason: %v", ag.RunningWorkload.URL, w.producerPH[ag.AgreementProtocol].GetTerminationReason(reason)),
				persistence.EC_CANCEL_AGREEMENT,
				ag)

			w.cancelAgreement(agreementId, ag.AgreementProtocol, reason, w.producerPH[ag.AgreementProtocol].GetTerminationReason(reason))

			// send the event to the container in case it has started the workloads.
			w.Messages() <- events.NewGovernanceWorkloadCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, ag.AgreementProtocol, agreementId, ag.GetDeploymentConfig())
			// clean up microservice instances if needed
			w.handleMicroserviceInstForAgEnded(agreementId, false)
		}
	}
}
