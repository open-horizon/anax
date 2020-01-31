package exchange

import (
	"fmt"
	"github.com/golang/glog"
	"time"
)

// The LastUpdated field is explicitly omitted due to a pending change to the datatype of the field.
// When the field datatype becomes and int, we can add it back.
type ResourceChange struct {
	ChangeID uint64 `json:"changeid,omitempty"`
}

type ExchangeChange struct {
	OrgID           string           `json:"orgid,omitempty"`
	Resource        string           `json:"resource,omitempty"` // The type of the resource
	ID              string           `json:"id,omitempty"`
	Operation       string           `json:"operation,omitempty"`
	ResourceChanges []ResourceChange `json:"resourceChanges,omitempty"`
}

func (e ExchangeChange) String() string {
	detailedChanges := fmt.Sprintf("%v", len(e.ResourceChanges))
	if glog.V(5) {
		detailedChanges = fmt.Sprintf("%v", e.ResourceChanges)
	}

	return fmt.Sprintf("Exchange Change Resource type: %v, "+
		"Resource: %v/%v, "+
		"Operation: %v, "+
		"Detailed changes: %v",
		e.Resource, e.OrgID, e.ID, e.Operation, detailedChanges)
}

// constants for resource type values
const RESOURCE_NODE_MSG = "nodemsgs"        // A message was delivered to the node
const RESOURCE_NODE = "node"                // A change was made to the node
const RESOURCE_NODE_POLICY = "nodepolicies" // A change was made to the node policy
const RESOURCE_NODE_ERROR = "nodeerrors"    // A change was made to the node errors
const RESOURCE_SERVICE = "service"          // A change was made to a service

// functions for interrogating change types
func (e ExchangeChange) IsMessage(node string) bool {
	changeNode := fmt.Sprintf("%v/%v", e.OrgID, e.ID)
	return changeNode == node && e.Resource == RESOURCE_NODE_MSG
}

func (e ExchangeChange) IsNode(node string) bool {
	changeNode := fmt.Sprintf("%v/%v", e.OrgID, e.ID)
	return changeNode == node && e.Resource == RESOURCE_NODE
}

func (e ExchangeChange) IsNodePolicy(node string) bool {
	changeNode := fmt.Sprintf("%v/%v", e.OrgID, e.ID)
	return changeNode == node && e.Resource == RESOURCE_NODE_POLICY
}

func (e ExchangeChange) IsNodeError(node string) bool {
	changeNode := fmt.Sprintf("%v/%v", e.OrgID, e.ID)
	return changeNode == node && e.Resource == RESOURCE_NODE_ERROR
}

func (e ExchangeChange) IsService() bool {
	return e.Resource == RESOURCE_SERVICE
}

// This is the struct we get back from the exchange API call.
type ExchangeChanges struct {
	Changes            []ExchangeChange `json:"changes,omitempty"`
	MostRecentChangeID uint64           `json:"mostRecentChangeId,omitempty"`
	ExchangeVersion    string           `json:"exchangeVersion,omitempty"`
}

func (e ExchangeChanges) String() string {
	detailedChanges := fmt.Sprintf("%v", len(e.Changes))
	if glog.V(5) {
		detailedChanges = fmt.Sprintf("%v", e.Changes)
	}

	return fmt.Sprintf("Changes: %v, "+
		"MostRecentChangeID: %v, "+
		"ExchangeVersion: %v, ",
		detailedChanges, e.MostRecentChangeID, e.ExchangeVersion)
}

func (e *ExchangeChanges) GetMostRecentChangeID() uint64 {
	return e.MostRecentChangeID
}

func (e *ExchangeChanges) GetExchangeVersion() string {
	return e.ExchangeVersion
}

// This is the request body for the changes API call.
type GetExchangeChangesRequest struct {
	ChangeId   uint64 `json:"changeId"`
	MaxRecords int    `json:"maxRecords,omitempty"`
}

type ExchangeChangeIDResponse struct {
	MaxChangeID uint64 `json:"maxChangeId,omitempty"`
}

// Retrieve the latest changes from the exchange.
func GetExchangeChangeID(ec ExchangeContext) (*ExchangeChangeIDResponse, error) {

	glog.V(3).Infof(rpclogString(fmt.Sprintf("getting current max change ID")))

	var resp interface{}
	resp = new(ExchangeChangeIDResponse)

	// Get resource changes in the exchange
	targetURL := fmt.Sprintf("%vchanges/maxchangeid", ec.GetExchangeURL())

	retryCount := ec.GetHTTPFactory().RetryCount
	retryInterval := ec.GetHTTPFactory().GetRetryInterval()
	for {
		if err, tpErr := InvokeExchange(ec.GetHTTPFactory().NewHTTPClient(nil), "GET", targetURL, ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			if ec.GetHTTPFactory().RetryCount == 0 {
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else if retryCount == 0 {
				return nil, fmt.Errorf("Exceeded %v retries for error: %v", ec.GetHTTPFactory().RetryCount, tpErr)
			} else {
				retryCount--
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			}
		} else {
			changeResp := resp.(*ExchangeChangeIDResponse)

			glog.V(3).Infof(rpclogString(fmt.Sprintf("found max changes ID %v", changeResp)))
			return changeResp, nil
		}
	}
}

// Retrieve the latest changes from the exchange.
func GetExchangeChanges(ec ExchangeContext, changeId uint64, maxRecords int) (*ExchangeChanges, error) {

	glog.V(3).Infof(rpclogString(fmt.Sprintf("getting %v changes since change ID %v", maxRecords, changeId)))

	var resp interface{}
	resp = new(ExchangeChanges)
	req := GetExchangeChangesRequest{
		ChangeId:   changeId,
		MaxRecords: maxRecords,
	}

	// Get resource changes in the exchange
	targetURL := fmt.Sprintf("%vorgs/%v/changes", ec.GetExchangeURL(), GetOrg(ec.GetExchangeId()))

	retryCount := ec.GetHTTPFactory().RetryCount
	retryInterval := ec.GetHTTPFactory().GetRetryInterval()
	for {
		if err, tpErr := InvokeExchange(ec.GetHTTPFactory().NewHTTPClient(nil), "POST", targetURL, ec.GetExchangeId(), ec.GetExchangeToken(), &req, &resp); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			if ec.GetHTTPFactory().RetryCount == 0 {
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else if retryCount == 0 {
				return nil, fmt.Errorf("Exceeded %v retries for error: %v", ec.GetHTTPFactory().RetryCount, tpErr)
			} else {
				retryCount--
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			}
		} else {
			changes := resp.(*ExchangeChanges)

			glog.V(3).Infof(rpclogString(fmt.Sprintf("found %v changes since ID %v with latest change ID %v", len(changes.Changes), changeId, changes.MostRecentChangeID)))
			glog.V(5).Infof(rpclogString(fmt.Sprintf("Raw changes response: %v", changes)))
			return changes, nil
		}
	}
}
