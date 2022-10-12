package governance

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/eventlog"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/microservice"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
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

// Handles node policy updated or deleted. The node policy contains 2 parts:
// deployment and management.
// ucDeployment -- the node policy change code for deployment.
// ucManagement -- the node policy change code for management.
// They are defined in the externalpolicy.ExternalPolicy.go (EP_COMPARE_*)
func (w *GovernanceWorker) handleNodePolicyUpdated(ucDeployment int, ucManagement int) {
	glog.V(5).Infof(logString(fmt.Sprintf("handling node policy changes, change code: %v, %v", ucDeployment, ucManagement)))

	// handle management policy updated/deleted
	if ucManagement != externalpolicy.EP_COMPARE_NOCHANGE {
		glog.V(3).Infof(logString(fmt.Sprintf("handling management node policy updates. change code: %v", ucManagement)))
		w.handleNodePolicyUpdateForManagement(ucManagement)
	}

	// handle deployment policy updated/deleted
	if ucDeployment != externalpolicy.EP_COMPARE_NOCHANGE {
		glog.V(3).Infof(logString(fmt.Sprintf("handling deployment node policy updates. change code: %v", ucDeployment)))
		w.handleNodePolicyUpdateForDeployment(ucDeployment)
	}
}

// Handles the node policy changes for deployment.
// The updateCode is the OR of
//
//	EP_COMPARE_PROPERTY_CHANGED,
//	EP_COMPARE_CONSTRAINT_CHANGED,
//	EP_ALLOWPRIVILEGED_CHANGED
//
// defined in externalpolicy/ExternalPolicy.go
func (w *GovernanceWorker) handleNodePolicyUpdateForDeployment(updateCode int) {

	// node policy deleted
	if updateCode&externalpolicy.EP_COMPARE_DELETED == externalpolicy.EP_COMPARE_DELETED {
		// cancel all agreements for the policy case
		if w.devicePattern == "" {
			w.pm.DeletePolicyByName(exchange.GetOrg(w.GetExchangeId()), policy.MakeExternalPolicyHeaderName(w.GetExchangeId()))
			w.cancelAllAgreements()
		}
		return
	}

	// now handle the policy updated case, update the policy in policy manager
	// for the non-pattern case
	if w.devicePattern == "" {
		// get the node policy
		nodePolicy, err := persistence.FindNodePolicy(w.db)
		if err != nil {
			glog.Errorf(logString(fmt.Sprintf("unable to read node policy from the local database. %v", err)))
			eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
				persistence.NewMessageMeta(EL_GOV_ERR_RETRIEVE_NODE_POL_FROM_DB, err.Error()),
				persistence.EC_DATABASE_ERROR)
			return
		}

		// get the deployment policy from the node policy now that the node policy
		// containts both deployment and management policies.
		var deploy_pol *externalpolicy.ExternalPolicy
		if nodePolicy != nil {
			deploy_pol = nodePolicy.GetDeploymentPolicy()
		} else {
			deploy_pol = nil
		}

		// add the node policy to the policy manager
		newPol, err := policy.GenPolicyFromExternalPolicy(deploy_pol, policy.MakeExternalPolicyHeaderName(w.GetExchangeId()))
		if err != nil {
			glog.Errorf(logString(fmt.Sprintf("Failed to convert node policy to policy file format: %v", err)))
			return
		}
		w.pm.UpdatePolicy(exchange.GetOrg(w.GetExchangeId()), newPol)
	}

	// If node's allowPrivileged built-in property is changed, cancel all agreements
	if updateCode&externalpolicy.EP_ALLOWPRIVILEGED_CHANGED == externalpolicy.EP_ALLOWPRIVILEGED_CHANGED {
		w.cancelAllAgreements()
	}

	// Let governAgreements() function handle the policy re-evaluation and the rest
	// for other cases
}

// Handles the node policy changes for node management.
// The updateCode is the OR of
//
//	EP_COMPARE_PROPERTY_CHANGED,
//	EP_COMPARE_CONSTRAINT_CHANGED,
//	EP_COMPARE_DELETED,
//	EP_ALLOWPRIVILEGED_CHANGED
//
// defined in externalpolicy/ExternalPolicy.go
func (w *GovernanceWorker) handleNodePolicyUpdateForManagement(updateCode int) {
}
