package agreementbot

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/cutil"
	"time"
)

// Retry all the deployment policy and node combinations that should have made agreements but did not.
// There are many reasons why this could happen, all of which are the result of async events that
// occur out of order. These combinations will be retried.
func (w *AgreementBotWorker) handleRetryAgreements() {

	glog.V(3).Infof(AWlogString(fmt.Sprintf("Handling retries: %v", w.retryAgreements.Dump())))

	// Search policies as if they just changed. This will return more results than we need, but
	// the results will be filtered based on the nodes that we know we need to retry.
	now := uint64(time.Now().Unix())

	// Do a destructive get on the list of policies and nodes to retry. From this point onward,
	// any newly discovered agreement failures will start queueing up again.
	retryMap := w.retryAgreements.GetAll()
	if len(retryMap) == 0 {
		glog.V(5).Infof(AWlogString("agreement retry is empty"))
		return
	}

	searchError := false

	// Iterate through all the policy orgs. Usually there is only 1 org in this list.
	allOrgs := w.pm.GetAllPolicyOrgs()
	for _, org := range allOrgs {
		// Get a copy of all policies in the policy manager that pulls from the policy files so that we can safely iterate the list.
		policies := w.pm.GetAllAvailablePolicies(org)
		for _, consumerPolicy := range policies {
			if _, ok := retryMap[consumerPolicy.Header.Name]; !ok {
				// Ignore policies that are not in the retry list.
				continue
			}
			if consumerPolicy.PatternId != "" {
				if err := w.searchNodesAndMakeAgreements(&consumerPolicy, org, "", 0, nil); err != nil {
					searchError = true
					break
				}
			} else if pBE := w.BusinessPolManager.GetBusinessPolicyEntry(org, &consumerPolicy); pBE != nil {
				_, polName := cutil.SplitOrgSpecUrl(consumerPolicy.Header.Name)
				if err := w.searchNodesAndMakeAgreements(&consumerPolicy, org, polName, now, nodeFilter(retryMap[consumerPolicy.Header.Name])); err != nil {
					searchError = true
					break
				}
			}
		}
		if searchError {
			break
		}
	}

	// If there was a search error, requeue all the deployment policies that should be retried.
	if searchError {
		for polId, nodeMap := range retryMap {
			for nodeId, _ := range nodeMap {
				w.retryAgreements.AddRetry(polId, nodeId)
			}
		}
	}

	glog.V(3).Infof(AWlogString(fmt.Sprintf("Done handling retries: %v", w.retryAgreements.Dump())))

}

func nodeFilter(nodeMap map[string]bool) SearchFilter {
	return func(nodeId string) bool {
		// Return true if the input nodeId should be filtered out. The nodeMap contains the nodes that are supposed to be retried.
		_, ok := nodeMap[nodeId]
		return !ok
	}
}
