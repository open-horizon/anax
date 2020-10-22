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
const RESOURCE_NODE_MSG = "nodemsgs"              // A message was delivered to the node
const RESOURCE_AGBOT_MSG = "agbotmsgs"            // A message was delivered to the agbot
const RESOURCE_NODE = "node"                      // A change was made to the node
const RESOURCE_AGBOT = "agbot"                    // A change was made to the agbot
const RESOURCE_NODE_POLICY = "nodepolicies"       // A change was made to the node policy
const RESOURCE_NODE_AGREEMENTS = "nodeagreements" // A change was made to one of the agreements on the node
const RESOURCE_NODE_STATUS = "nodestatus"
const RESOURCE_NODE_ERROR = "nodeerrors" // A change was made to the node errors
const RESOURCE_NODE_SERVICES_CONFIGSTATE = "services_configstate"
const RESOURCE_SERVICE = "service"                       // A change was made to a service
const RESOURCE_AGBOT_SERVED_POLICY = "agbotbusinesspols" // A served deployment policy change occurred
const RESOURCE_AGBOT_SERVED_PATTERN = "agbotpatterns"    // A served pattern change occurred
const RESOURCE_AGBOT_PATTERN = "pattern"                 // A pattern change occurred
const RESOURCE_AGBOT_POLICY = "policy"                   // A policy change occurred
const RESOURCE_AGBOT_SERVICE_POLICY = "servicepolicies"  // A service policy changed
const RESOURCE_AGBOT_AGREEMENTS = "agbotagreements"      // A change was made to one of the agreements on the agbot
const RESOURCE_ORG = "org"                               // A change was made to the org

// constants for operation values
const CHANGE_OPERATION_CREATED = "created"
const CHANGE_OPERATION_CREATED_MODIFIED = "created/modified"
const CHANGE_OPERATION_MODIFIED = "modified"
const CHANGE_OPERATION_DELETED = "deleted"

// functions for interrogating change types
func (e ExchangeChange) IsMessage(node string) bool {
	changeNode := fmt.Sprintf("%v/%v", e.OrgID, e.ID)
	return changeNode == node && e.Resource == RESOURCE_NODE_MSG
}

func (e ExchangeChange) IsAgbotMessage(agbot string) bool {
	changeAgbot := fmt.Sprintf("%v/%v", e.OrgID, e.ID)
	return changeAgbot == agbot && e.Resource == RESOURCE_AGBOT_MSG
}

func (e ExchangeChange) IsAgbotAgreement(agbot string) bool {
	changeAgbot := fmt.Sprintf("%v/%v", e.OrgID, e.ID)
	return changeAgbot == agbot && e.Resource == RESOURCE_AGBOT_AGREEMENTS
}

func (e ExchangeChange) IsNode(node string) bool {
	if node != "" {
		changeNode := fmt.Sprintf("%v/%v", e.OrgID, e.ID)
		return changeNode == node && e.Resource == RESOURCE_NODE
	} else {
		return e.Resource == RESOURCE_NODE
	}
}

func (e ExchangeChange) IsAgbot(agbot string) bool {
	changeAgbot := fmt.Sprintf("%v/%v", e.OrgID, e.ID)
	return changeAgbot == agbot && e.Resource == RESOURCE_AGBOT
}

func (e ExchangeChange) IsNodePolicy(node string) bool {
	if node != "" {
		changeNode := fmt.Sprintf("%v/%v", e.OrgID, e.ID)
		return changeNode == node && e.Resource == RESOURCE_NODE_POLICY
	} else {
		return e.Resource == RESOURCE_NODE_POLICY
	}
}

func (e ExchangeChange) IsNodeAgreement(node string) bool {
	if node != "" {
		changeNode := fmt.Sprintf("%v/%v", e.OrgID, e.ID)
		return changeNode == node && e.Resource == RESOURCE_NODE_AGREEMENTS
	} else {
		return e.Resource == RESOURCE_NODE_AGREEMENTS
	}
}

func (e ExchangeChange) IsNodeStatus(node string) bool {
	if node != "" {
		changeNode := fmt.Sprintf("%v/%v", e.OrgID, e.ID)
		return changeNode == node && e.Resource == RESOURCE_NODE_STATUS
	} else {
		return e.Resource == RESOURCE_NODE_STATUS
	}
}

func (e ExchangeChange) IsNodeServiceConfigState(node string) bool {
	if node != "" {
		changeNode := fmt.Sprintf("%v/%v", e.OrgID, e.ID)
		return changeNode == node && e.Resource == RESOURCE_NODE_SERVICES_CONFIGSTATE
	} else {
		return e.Resource == RESOURCE_NODE_SERVICES_CONFIGSTATE
	}
}

func (e ExchangeChange) IsNodeError(node string) bool {
	changeNode := fmt.Sprintf("%v/%v", e.OrgID, e.ID)
	return changeNode == node && e.Resource == RESOURCE_NODE_ERROR
}

func (e ExchangeChange) IsOrg() bool {
	return e.Resource == RESOURCE_ORG
}

func (e ExchangeChange) IsService() bool {
	return e.Resource == RESOURCE_SERVICE
}

func (e ExchangeChange) IsAgbotServedPolicy(agbot string) bool {
	changeAgbot := fmt.Sprintf("%v/%v", e.OrgID, e.ID)
	return changeAgbot == agbot && e.Resource == RESOURCE_AGBOT_SERVED_POLICY
}

func (e ExchangeChange) IsAgbotServedPattern(agbot string) bool {
	changeAgbot := fmt.Sprintf("%v/%v", e.OrgID, e.ID)
	return changeAgbot == agbot && e.Resource == RESOURCE_AGBOT_SERVED_PATTERN
}

func (e ExchangeChange) IsPattern() bool {
	return e.Resource == RESOURCE_AGBOT_PATTERN
}

func (e ExchangeChange) IsDeploymentPolicy() bool {
	return e.Resource == RESOURCE_AGBOT_POLICY
}

func (e ExchangeChange) IsServicePolicy() bool {
	return e.Resource == RESOURCE_AGBOT_SERVICE_POLICY
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
	ChangeId   uint64   `json:"changeId"`
	MaxRecords int      `json:"maxRecords,omitempty"`
	Orgs       []string `json:"orgList,omitempty"`
}

type ExchangeChangeIDResponse struct {
	MaxChangeID uint64 `json:"maxChangeId,omitempty"`
}

// Retrieve the latest change ID from the exchange.
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
func GetExchangeChanges(ec ExchangeContext, changeId uint64, maxRecords int, orgList []string) (*ExchangeChanges, error) {

	number_orgs := 0
	if orgList != nil {
		number_orgs = len(orgList)
	}
	if number_orgs > 10 {
		glog.V(3).Infof(rpclogString(fmt.Sprintf("getting %v changes since change ID %v in %v orgs.", maxRecords, changeId, number_orgs)))
	} else {
		glog.V(3).Infof(rpclogString(fmt.Sprintf("getting %v changes since change ID %v in orgs %v", maxRecords, changeId, orgList)))
	}

	var resp interface{}
	resp = new(ExchangeChanges)
	req := GetExchangeChangesRequest{
		ChangeId:   changeId,
		MaxRecords: maxRecords,
		Orgs:       orgList,
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

			if number_orgs > 10 {
				glog.V(3).Infof(rpclogString(fmt.Sprintf("found %v changes since ID %v with latest change ID %v in %v orgs", len(changes.Changes), changeId, changes.MostRecentChangeID, number_orgs)))
			} else {
				glog.V(3).Infof(rpclogString(fmt.Sprintf("found %v changes since ID %v with latest change ID %v in orgs %v", len(changes.Changes), changeId, changes.MostRecentChangeID, orgList)))
			}
			glog.V(5).Infof(rpclogString(fmt.Sprintf("Raw changes response: %v", changes)))
			return changes, nil
		}
	}
}
