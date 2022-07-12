package exchange

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/persistence"
	"strings"
	"time"
)

type SearchResultDevice struct {
	Id        string `json:"id"`
	NodeType  string `json:"nodeType"`
	PublicKey string `json:"publicKey"`
}

func (d SearchResultDevice) String() string {
	return fmt.Sprintf("Id: %v, NodeType: %v", d.Id, d.NodeType)
}

func (d SearchResultDevice) ShortString() string {
	return d.String()
}

func (d *SearchResultDevice) GetNodeType() string {
	if d.NodeType == "" {
		return persistence.DEVICE_TYPE_DEVICE
	} else {
		return d.NodeType
	}
}

// Structs and types for working with business policy based exchange searches
type SearchExchBusinessPolRequest struct {
	NodeOrgIds   []string `json:"nodeOrgids,omitempty"`
	ChangedSince uint64   `json:"changedSince"`
	Session      string   `json:"session"`
	NumEntries   uint64   `json:"numEntries"`
}

func (a SearchExchBusinessPolRequest) String() string {
	return fmt.Sprintf("NodeOrgIds: %v, ChangedSince: %v, Session: %v, NumEntries: %v", a.NodeOrgIds, time.Unix(int64(a.ChangedSince), 0).Format(cutil.ExchangeTimeFormat), a.Session, a.NumEntries)
}

// This struct is a merge of 2 possible responses that can come from the policy search API.
type SearchExchBusinessPolResponse struct {
	Devices   []SearchResultDevice `json:"nodes"`
	LastIndex int                  `json:"lastIndex"`
	AgbotId   string               `json:"agbot"`    // The internal agbot id (UUID)
	Offset    string               `json:"offset"`   // The current timestamp wihtin the exchange that marks the latest changed node given to an agbot
	Session   string               `jsaon:"session"` // The session token used by the agbot for this series of searches
}

func (r SearchExchBusinessPolResponse) String() string {
	return fmt.Sprintf("Devices: %v, LastIndex: %v, AgbotId: %v, Exchange offset: %v, Session: %v", r.Devices, r.LastIndex, r.AgbotId, r.Offset, r.Session)
}

func GetPolicyNodes(ec ExchangeContext, policyOrg string, policyName string, req *SearchExchBusinessPolRequest) (*SearchExchBusinessPolResponse, error) {
	// Invoke the exchange
	var resp interface{}
	resp = new(SearchExchBusinessPolResponse)
	targetURL := ec.GetExchangeURL() + "orgs/" + policyOrg + "/business/policies/" + policyName + "/search"
	for {
		// TODO: Need special handling for a 409 because the session is invalid (or old).
		if err, tpErr := InvokeExchange(ec.GetHTTPFactory().NewHTTPClient(nil), "POST", targetURL, ec.GetExchangeId(), ec.GetExchangeToken(), *req, &resp); err != nil {
			if !strings.Contains(err.Error(), "status: 404") {
				return nil, err
			} else {
				return resp.(*SearchExchBusinessPolResponse), nil
			}
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			return resp.(*SearchExchBusinessPolResponse), nil
		}
	}
}

// Structs and types for working with pattern based exchange searches
type SearchExchangePatternRequest struct {
	ServiceURL   string   `json:"serviceUrl,omitempty"`
	Arch         string   `json:"arch,omitempty"`
	NodeOrgIds   []string `json:"nodeOrgids,omitempty"`
	SecondsStale int      `json:"secondsStale"`
	NumEntries   int      `json:"numEntries"`
}

func (a SearchExchangePatternRequest) String() string {
	return fmt.Sprintf("ServiceURL: %v, SecondsStale: %v, NumEntries: %v", a.ServiceURL, a.SecondsStale, a.NumEntries)
}

type SearchExchangePatternResponse struct {
	Devices   []SearchResultDevice `json:"nodes"`
	LastIndex int                  `json:"lastIndex"`
}

func (r SearchExchangePatternResponse) String() string {
	return fmt.Sprintf("Devices: %v, LastIndex: %v", r.Devices, r.LastIndex)
}

// This function creates the exchange search message body.
func CreateSearchPatternRequest() *SearchExchangePatternRequest {

	ser := &SearchExchangePatternRequest{
		NumEntries: 0,
	}

	return ser
}

func GetPatternNodes(ec ExchangeContext, policyOrg string, patternId string, req *SearchExchangePatternRequest) (*[]SearchResultDevice, error) {
	// Invoke the exchange
	var resp interface{}
	resp = new(SearchExchangePatternResponse)
	targetURL := ec.GetExchangeURL() + "orgs/" + policyOrg + "/patterns/" + GetId(patternId) + "/search"
	for {
		if err, tpErr := InvokeExchange(ec.GetHTTPFactory().NewHTTPClient(nil), "POST", targetURL, ec.GetExchangeId(), ec.GetExchangeToken(), *req, &resp); err != nil {
			if !strings.Contains(err.Error(), "status: 404") {
				return nil, err
			} else {
				empty := make([]SearchResultDevice, 0, 0)
				return &empty, nil
			}
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			dev := resp.(*SearchExchangePatternResponse).Devices
			return &dev, nil
		}
	}
}

// This section is for types related to querying the exchange for node health

type AgreementObject struct {
}

type NodeInfo struct {
	LastHeartbeat string                     `json:"lastHeartbeat"`
	Agreements    map[string]AgreementObject `json:"agreements"`
}

func (n NodeInfo) String() string {
	return fmt.Sprintf("LastHeartbeat: %v, Agreements: %v", n.LastHeartbeat, n.Agreements)
}

type NodeHealthStatus struct {
	Nodes map[string]NodeInfo `json:"nodes"`
}

type NodeHealthStatusRequest struct {
	NodeOrgIds []string `json:"nodeOrgids,omitempty"`
	LastCall   string   `json:"lastTime"`
}

// Return the current status of nodes in a given pattern. This function can return nil and no error if the exchange has no
// updated status to return.
func GetNodeHealthStatus(httpClientFactory *config.HTTPClientFactory, pattern string, org string, nodeOrgs []string, lastCallTime string, exURL string, id string, token string) (*NodeHealthStatus, error) {

	glog.V(3).Infof(rpclogString(fmt.Sprintf("getting node health status for %v", pattern)))

	// to save time, do not make a rpc call if the nodeOrgs is empty
	if len(nodeOrgs) == 0 {
		var nh NodeHealthStatus
		nh.Nodes = make(map[string]NodeInfo, 0)
		return &nh, nil
	}

	params := &NodeHealthStatusRequest{
		NodeOrgIds: nodeOrgs,
		LastCall:   lastCallTime,
	}

	var resp interface{}
	resp = new(NodeHealthStatus)

	// Search the exchange for the node health status
	targetURL := fmt.Sprintf("%vorgs/%v/search/nodehealth", exURL, org)
	if pattern != "" {
		targetURL = fmt.Sprintf("%vorgs/%v/patterns/%v/nodehealth", exURL, GetOrg(pattern), GetId(pattern))
	}

	retryCount := httpClientFactory.RetryCount
	retryInterval := httpClientFactory.GetRetryInterval()
	for {
		if err, tpErr := InvokeExchange(httpClientFactory.NewHTTPClient(nil), "POST", targetURL, id, token, &params, &resp); err != nil && !strings.Contains(err.Error(), "status: 404") {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			if httpClientFactory.RetryCount == 0 {
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else if retryCount == 0 {
				return nil, fmt.Errorf("Exceeded %v retries for error: %v", httpClientFactory.RetryCount, tpErr)
			} else {
				retryCount--
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			}
		} else {
			status := resp.(*NodeHealthStatus)
			glog.V(3).Infof(rpclogString(fmt.Sprintf("found nodehealth status for %v, status %v", pattern, status)))
			return status, nil
		}
	}
}
