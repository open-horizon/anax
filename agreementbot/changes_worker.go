package agreementbot

import (
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/version"
	"github.com/open-horizon/anax/worker"
	"time"
)

type ChangesWorker struct {
	worker.BaseWorker          // embedded field
	changeID          uint64   // The current change Id in the exchange.
	orgList           []string // The list of orgs for which this worker should see changes.
	noworkDispatch    int64    // The last time the NoWorkHandler was dispatched.
	mmsObjectPollTime int64    // The last time the MMS was polled for changes
}

func NewChangesWorker(name string, cfg *config.HorizonConfig) *ChangesWorker {

	ec := worker.NewExchangeContext(cfg.AgreementBot.ExchangeId, cfg.AgreementBot.ExchangeToken, cfg.AgreementBot.ExchangeURL, cfg.AgreementBot.CSSURL, cfg.Collaborators.HTTPClientFactory)
	worker := &ChangesWorker{
		BaseWorker:     worker.NewBaseWorker(name, cfg, ec),
		changeID:       0,
		orgList:        make([]string, 0, 5),
		noworkDispatch: time.Now().Unix(),
	}

	glog.Info(chglog(fmt.Sprintf("Starting ExchangeChanges worker")))

	worker.Start(worker, int(cfg.AgreementBot.ExchangeHeartbeat))
	return worker
}

func (w *ChangesWorker) Messages() chan events.Message {
	return w.BaseWorker.Manager.Messages
}

func (w *ChangesWorker) Initialize() bool {

	// If we havent picked up changes yet, make sure to broadcast all change events just to make sure the agbot is
	// up to date with what's in the exchange.
	if w.changeID == 0 {
		if err := w.getChangeId(); err != nil {
			// If the change Id was unavailable, the nowork handler will try to obtain it later.
			return true
		}
	}

	// Grab the list of orgs this agbot is supposed to be serving and set it into the worker's org list cache.
	w.orgList = w.gatherServedOrgs(nil)

	return true
}

// Handle events that are propogated to this worker from the internal event bus.
func (w *ChangesWorker) NewEvent(incoming events.Message) {

	switch incoming.(type) {

	case *events.NodeShutdownCompleteMessage:
		msg, _ := incoming.(*events.NodeShutdownCompleteMessage)
		switch msg.Event().Id {
		case events.UNCONFIGURE_COMPLETE:
			w.Commands <- worker.NewTerminateCommand("shutdown")
		}

	default: //nothing

	}

	return
}

// Handle commands that are placed on the command queue.
func (w *ChangesWorker) CommandHandler(command worker.Command) bool {

	// Be sure to call findAndProcessChanges() if it hasnt been called in a while.
	// switch command.(type) {

	// default:
	// 	return false
	// }

	return true

}

// This function gets called when the worker framework has found nothing to do for the "no work interval"
// that was set when the worker was started.
func (w *ChangesWorker) NoWorkHandler() {

	// Heartbeat and check for changes.
	w.findAndProcessChanges()

	return
}

// Go get the latest changes and process them, notifying other workers that they might have work to do.
func (w *ChangesWorker) findAndProcessChanges() {

	w.noworkDispatch = time.Now().Unix()

	// If there is no last known change id, then we havent initialized yet,so do nothing.
	if w.changeID == 0 {
		glog.Warningf(chglog(fmt.Sprintf("No starting change ID")))

		// Get the latest change ID, if it's available.
		if err := w.getChangeId(); err != nil {
			return
		}

		// Grab the list of orgs this agbot is supposed to be serving and set it into the worker's org list cache.
		w.orgList = w.gatherServedOrgs(nil)

	}

	glog.V(3).Infof(chglog(fmt.Sprintf("looking for changes starting from ID %v", w.changeID)))

	// Call the exchange to retrieve any changes since our last known change id.
	changes, err := exchange.GetHTTPExchangeChangeHandler(w)(w.changeID, w.Config.AgreementBot.MaxExchangeChanges, w.orgList)

	// Handle heartbeat state changes and errors. Returns true if there was an error to be handled.
	if w.handleHeartbeatStateAndError(changes, err) {
		return
	}

	// Keep a map of changes that can be batched together into 1 event in order to reduce the load on
	// the agbot worker.
	batchedEvents := make(map[events.EventId]bool)

	agbotMessages := 0

	// Loop through each change to identify resources that we are interested in, and then send out event messages
	// to notify the other workers that they have some work to do.
	for _, change := range changes.Changes {
		exchange.DeleteCacheResourceFromChange(change, "")
		if glog.V(5) {
			glog.Infof(chglog(fmt.Sprintf("Change: %v", change)))
		}

		if change.IsAgbotMessage(w.GetExchangeId()) {

			if change.Operation == "created" {
				agbotMessages += len(change.ResourceChanges)
			}

			batchedEvents[events.CHANGE_AGBOT_MESSAGE_TYPE] = true

		} else if change.IsAgbotServedPolicy(w.GetExchangeId()) || change.IsAgbotServedPattern(w.GetExchangeId()) {
			w.updateServedOrgs(&change)

		} else if change.IsAgbotAgreement(w.GetExchangeId()) {
			batchedEvents[events.CHANGE_AGBOT_AGREEMENT_TYPE] = true

		} else if change.IsPattern() {
			ev := events.NewExchangeChangeMessage(events.CHANGE_AGBOT_PATTERN)
			ev.SetChange(change)
			w.Messages() <- ev

		} else if change.IsDeploymentPolicy() {
			ev := events.NewExchangeChangeMessage(events.CHANGE_AGBOT_POLICY)
			ev.SetChange(change)
			w.Messages() <- ev

		} else if change.IsServicePolicy() {
			ev := events.NewExchangeChangeMessage(events.CHANGE_SERVICE_POLICY_TYPE)
			ev.SetChange(change)
			w.Messages() <- ev

		} else if change.IsNode("") {
			batchedEvents[events.CHANGE_NODE_TYPE] = true

		} else if change.IsNodePolicy("") {
			batchedEvents[events.CHANGE_NODE_POLICY_TYPE] = true

		} else if change.IsNodeAgreement("") {
			batchedEvents[events.CHANGE_NODE_AGREEMENT_TYPE] = true

		} else if change.IsNodeServiceConfigState("") {
			batchedEvents[events.CHANGE_NODE_CONFIGSTATE_TYPE] = true

		} else {
			glog.V(5).Infof(chglog(fmt.Sprintf("Unhandled change: %v %v/%v", change.Resource, change.OrgID, change.ID)))
		}
	}

	// Publish any batched events
	w.emitChangeMessages(batchedEvents, agbotMessages)

	// Record the most recent change id.
	w.postProcessChanges(changes)

	glog.V(3).Infof(chglog(fmt.Sprintf("done looking for changes")))

	// Poll the CSS for object policy changes. This is a different changes mechanism than the one used by the exchange.
	oldTime := w.mmsObjectPollTime
	w.mmsObjectPollTime = time.Now().UTC().UnixNano()

	for _, org := range w.orgList {

		// Query MMS for all object policies in the org since the last time we called.
		if mmsObjPolicies, err := exchange.GetHTTPObjectPolicyUpdatesQueryHandler(w)(org, oldTime); err != nil {
			glog.Errorf(fmt.Sprintf("unable to get object polices for org %v, error %v", org, err))
		} else if len(*mmsObjPolicies) != 0 {
			w.Messages() <- events.NewMMSObjectPoliciesMessage(events.OBJECT_POLICIES_CHANGED, (*mmsObjPolicies))
		}

	}
	glog.V(3).Infof(chglog(fmt.Sprintf("done looking for object policy changes")))

}

// Get the current change ID from the exchange, which gives this worker a place to start. Once there
// is a change ID available, tell the agbot worker to init itself and start scanning.
func (w *ChangesWorker) getChangeId() error {

	// Call the exchange to retrieve the current max change id.
	if maxChangeID, err := exchange.GetHTTPExchangeMaxChangeIDHandler(w)(); err != nil {
		msg := fmt.Sprintf("Error retrieving max change ID, error: %v", err)
		glog.Errorf(chglog(msg))
		return errors.New(msg)

	} else {
		w.changeID = maxChangeID.MaxChangeID
	}

	// Ensure the agbot does initial scans across resources.
	w.Messages() <- events.NewExchangeChangeMessage(events.CHANGE_AGBOT_MESSAGE_TYPE)
	w.Messages() <- events.NewExchangeChangeMessage(events.CHANGE_AGBOT_SERVED_POLICY)
	w.Messages() <- events.NewExchangeChangeMessage(events.CHANGE_AGBOT_SERVED_PATTERN)
	return nil
}

// Send change message for each change type in the map that is set to true.
func (w *ChangesWorker) emitChangeMessages(resChanges map[events.EventId]bool, agbotMessages int) {
	for changeType, _ := range resChanges {
		if changeType == events.CHANGE_AGBOT_MESSAGE_TYPE {
			ev := events.NewExchangeChangeMessage(changeType)
			ev.SetChange(events.MessageCount{Count: agbotMessages})
			w.Messages() <- ev
		} else {
			w.Messages() <- events.NewExchangeChangeMessage(changeType)
		}
	}
}

// Record the most recent change id based on the changes that were found.
func (w *ChangesWorker) postProcessChanges(changes *exchange.ExchangeChanges) {
	// If there were changes found, even uninteresting changes, we need to keep the most recent change id current.
	if changes.GetMostRecentChangeID() != 0 {
		w.changeID = changes.GetMostRecentChangeID() + 1
	}
}

// Process any error from the /changes API and update the heartbeat state appropriately. Return true if the
// caller should not proceeed to process the response.
func (w *ChangesWorker) handleHeartbeatStateAndError(changes *exchange.ExchangeChanges, err error) bool {
	if err != nil {
		glog.Errorf(chglog(fmt.Sprintf("heartbeat and change retrieval failed, error %v", err)))
		return true
	} else {

		// There is no error, but also no response object, that's a problem that needs to be logged.
		if changes == nil {
			glog.Errorf(chglog(fmt.Sprintf("Exchange /changes API returned no error and no response.")))
			return true
		}

		// Log an error if the current exchange version does not meet the minimum version requirement.
		if changes.GetExchangeVersion() != "" {
			if err := version.VerifyExchangeVersion1(changes.GetExchangeVersion(), false); err != nil {
				glog.Errorf(chglog(fmt.Sprintf("Error verifiying exchange version, error: %v", err)))
				return true
			}
		}

	}
	return false
}

// Query the agbot's list of configured orgs and policies in order to obtain the complete list of orgs served
// by this agbot.
func (w *ChangesWorker) gatherServedOrgs(change *exchange.ExchangeChange) []string {

	// Collect the served orgs in a map so that it's easier to handle duplicate org names.
	orgs := make(map[string]bool)

	// The map always includes the agbot's org.
	orgs[exchange.GetOrg(w.GetExchangeId())] = true

	// Get the orgs related to serving policies, and verify that the org exists in the exchange.
	if pols, err := exchange.GetHTTPAgbotServedDeploymentPolicy(w)(); err != nil {
		glog.Errorf(chglog(fmt.Sprintf("Error retrieving served deployment policies, error: %v", err)))
	} else {
		for _, servedOrg := range pols {
			// If the org is not already in the map, make sure it is a valid org.
			if _, ok := orgs[servedOrg.BusinessPolOrg]; !ok && w.verifyOrg(servedOrg.BusinessPolOrg) {
				orgs[servedOrg.BusinessPolOrg] = true
			}
			if _, ok := orgs[servedOrg.NodeOrg]; !ok && w.verifyOrg(servedOrg.NodeOrg) {
				orgs[servedOrg.NodeOrg] = true
			}
		}
	}

	// Get the orgs related to serving patterns.
	if pats, err := exchange.GetHTTPAgbotServedPattern(w)(); err != nil {
		glog.Errorf(chglog(fmt.Sprintf("Error retrieving served patterns, error: %v", err)))
	} else {
		for _, servedOrg := range pats {
			// If the org is not already in the map, make sure it is a valid org.
			if _, ok := orgs[servedOrg.PatternOrg]; !ok && w.verifyOrg(servedOrg.PatternOrg) {
				orgs[servedOrg.PatternOrg] = true
			}
			if _, ok := orgs[servedOrg.NodeOrg]; !ok && w.verifyOrg(servedOrg.NodeOrg) {
				orgs[servedOrg.NodeOrg] = true
			}
		}
	}

	glog.V(5).Infof(chglog(fmt.Sprintf("Previously serving orgs: %v, now serving orgs: %v", w.orgList, orgs)))

	if change != nil {
		if change.IsAgbotServedPolicy(w.GetExchangeId()) {
			w.Messages() <- events.NewExchangeChangeMessage(events.CHANGE_AGBOT_SERVED_POLICY)
		} else {
			w.Messages() <- events.NewExchangeChangeMessage(events.CHANGE_AGBOT_SERVED_PATTERN)
		}
	}

	// Given the orgs collected so far, convert them into a list and return them to the caller.
	orgList := []string{}
	for org, _ := range orgs {
		orgList = append(orgList, org)
	}

	glog.V(3).Infof(chglog(fmt.Sprintf("Agbot serving orgs: %v", orgList)))
	return orgList
}

// this function updates the worker's served org list and delete the cached resources related to any org that has changed
func (w *ChangesWorker) updateServedOrgs(change *exchange.ExchangeChange) {
	oldServedOrgs := w.orgList
	newServedOrgs := w.gatherServedOrgs(change)

	changedOrgs := []string{}

	for _, oldOrg := range oldServedOrgs {
		found := false
		for _, newOrg := range newServedOrgs {
			if oldOrg == newOrg {
				found = true
				break
			}
		}
		if !found {
			// oldOrg is no longer being served by this agbot
			changedOrgs = append(changedOrgs, oldOrg)
		}
	}
	for _, newOrg := range newServedOrgs {
		found := false
		for _, oldOrg := range oldServedOrgs {
			if newOrg == oldOrg {
				found = true
				break
			}
		}
		if !found {
			// newOrg was not previously served by this agbot, but is now
			changedOrgs = append(changedOrgs, newOrg)
		}
	}

	// we need to remove the cached exchange resources from the orgs that have been added or removed from the served list
	for _, changedOrg := range changedOrgs {
		exchange.DeleteOrgCachedResources(changedOrg)
	}

	// update the agbot's served org list
	w.orgList = newServedOrgs
}

// Verify that an org exists
func (w *ChangesWorker) verifyOrg(org string) bool {
	if _, err := exchange.GetHTTPExchangeOrgHandler(w)(org); err != nil {
		glog.Errorf(chglog(fmt.Sprintf("Error verifying org %v, error: %v", org, err)))
		return false
	}
	return true
}

// Utility logging function
var chglog = func(v interface{}) string {
	return fmt.Sprintf("Exchange Changes Worker: %v", v)
}
