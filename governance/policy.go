package governance

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/microservice"
	"github.com/open-horizon/anax/persistence"
)

// Each microservice def in the database has a policy generated for it, that needs to be re-generated and published back to the exchange.
func (w *GovernanceWorker) handleUpdatePolicy(cmd *UpdatePolicyCommand) {

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
