package agreementbot

import (
	"fmt"
	"sync"
)

// A 2 level map to accumulate combinations of deployment policies and nodes that should
// have made agreements. Use of this object is thread safe.
type RetryAgreements struct {
	retryPolicies map[string]map[string]bool // A map of policy ids to a map of node ids.
	mapLock       sync.Mutex
}

func NewRetryAgreements() *RetryAgreements {
	return &RetryAgreements{
		retryPolicies: make(map[string]map[string]bool, 10),
	}
}

func (r *RetryAgreements) AddRetry(depPolId string, nodeId string) {
	r.mapLock.Lock()
	defer r.mapLock.Unlock()

	if _, ok := r.retryPolicies[depPolId]; !ok || r.retryPolicies[depPolId] == nil {
		r.retryPolicies[depPolId] = make(map[string]bool, 10)
	}
	r.retryPolicies[depPolId][nodeId] = true
}

// This is a destructive get that returns the current map of policies and nodes, and
// then starts a new internal map for accumulating more retry candidates.
func (r *RetryAgreements) GetAll() map[string]map[string]bool {
	r.mapLock.Lock()
	defer r.mapLock.Unlock()

	ret := r.retryPolicies
	r.retryPolicies = make(map[string]map[string]bool, 10)
	return ret
}

func (r *RetryAgreements) Dump() string {
	r.mapLock.Lock()
	defer r.mapLock.Unlock()

	res := "retry agreements: "
	for polId, nodes := range r.retryPolicies {
		res += fmt.Sprintf("%v: ", polId)
		for nodeId, _ := range nodes {
			res += fmt.Sprintf("%v,", nodeId)
		}
	}
	return res
}
