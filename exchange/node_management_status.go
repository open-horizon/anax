package exchange

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchangecommon"
)

// Get a single node management policy status from the exchange
func GetNodeManagementPolicyStatus(ec ExchangeContext, orgId string, nodeId string, policyName string) (*exchangecommon.NodeManagementPolicyStatus, error) {
	glog.V(3).Infof("Getting node management policy status for node %v/%v and policy %v", orgId, nodeId, policyName)

	var resp interface{}
	resp = new(exchangecommon.NodeManagementPolicyStatus)

	_, policyName = cutil.SplitOrgSpecUrl(policyName)

	targetURL := fmt.Sprintf("%vorgs/%v/nodes/%v/managementStatus/%v", ec.GetExchangeURL(), orgId, nodeId, policyName)

	err := InvokeExchangeRetryOnTransportError(ec.GetHTTPFactory(), "GET", targetURL, ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp)
	if err != nil {
		return nil, err
	}

	nmpStatus := resp.(*exchangecommon.NodeManagementPolicyStatus)
	return nmpStatus, nil
}

// Update/Create a single node management policy status in the exchange
func PutNodeManagementPolicyStatus(ec ExchangeContext, orgId string, nodeId string, policyName string, nmpStatusFull *exchangecommon.NodeManagementPolicyStatus) (*PutPostDeleteStandardResponse, error) {
	// allowdowngrade and manifest are not in the exchange status schema. remove them here
	nmpStatus := nmpStatusFull.DeepCopy()
	nmpStatus.AgentUpgradeInternal = nil
	glog.V(3).Infof("Putting node management policy status for node %v/%v and policy %v. Status is: %v.", orgId, nodeId, policyName, nmpStatus)

	var resp interface{}
	resp = new(PutPostDeleteStandardResponse)

	org, name := cutil.SplitOrgSpecUrl(policyName)
	if name == "" {
		name = org
	}
	policyName = name

	targetURL := fmt.Sprintf("%vorgs/%v/nodes/%v/managementStatus/%v", ec.GetExchangeURL(), orgId, nodeId, policyName)

	// set the working directory to an empty string as this is not in the exchange schema
	nmpStatus.AgentUpgrade.BaseWorkingDirectory = ""

	err := InvokeExchangeRetryOnTransportError(ec.GetHTTPFactory(), "PUT", targetURL, ec.GetExchangeId(), ec.GetExchangeToken(), nmpStatus, &resp)
	if err != nil {
		return nil, err
	}

	return resp.(*PutPostDeleteStandardResponse), nil
}

// Delete the specifies node management policy status from the exchange
func DeleteNodeManagementPolicyStatus(ec ExchangeContext, orgId string, nodeId string, policyName string) error {
	glog.V(3).Infof("Delete node management policy status for policy %v and node %v/%v.", policyName, orgId, nodeId)

	var resp interface{}
	resp = new(PutPostDeleteStandardResponse)

	_, policyName = cutil.SplitOrgSpecUrl(policyName)

	targetURL := fmt.Sprintf("%vorgs/%v/nodes/%v/managementStatus/%v", ec.GetExchangeURL(), orgId, nodeId, policyName)

	err := InvokeExchangeRetryOnTransportError(ec.GetHTTPFactory(), "DELETE", targetURL, ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp)
	if err != nil {
		return err
	}

	return nil
}

type NodeManagementAllStatuses struct {
	PolicyStatuses map[string]exchangecommon.NodeManagementPolicyStatus `json:"managementStatus"`
	LastUpdated    string                                               `json:"lastUpdated"`
}

func (n NodeManagementAllStatuses) String() string {
	return fmt.Sprintf("PolicyStatuses: %v, LastUpdated: %v", n.PolicyStatuses, n.LastUpdated)
}

// Get all the node management policy statuses in the exchange for a given node
func GetNodeManagementAllStatuses(ec ExchangeContext, orgId string, nodeId string) (*NodeManagementAllStatuses, error) {
	glog.V(3).Infof("Getting all node management policy statuses for node: %v/%v.", orgId, nodeId)

	var resp interface{}
	resp = new(NodeManagementAllStatuses)

	targetURL := fmt.Sprintf("%vorgs/%v/nodes/%v/managementStatus", ec.GetExchangeURL(), orgId, nodeId)
	err := InvokeExchangeRetryOnTransportError(ec.GetHTTPFactory(), "GET", targetURL, ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp)
	if err != nil {
		return nil, err
	}

	return resp.(*NodeManagementAllStatuses), nil
}

// (the exchange does not support it now, please delete one by one)
// Delete all the node management policy statuses in the exchange for a given node
func DeleteNodeManagementAllStatuses(ec ExchangeContext, orgId string, nodeId string) error {
	glog.V(3).Infof("Deleting all node management policy statuses for node: %v/%v.", orgId, nodeId)

	var resp interface{}
	resp = new(PutPostDeleteStandardResponse)

	targetURL := fmt.Sprintf("%vorgs/%v/nodes/%v/managementStatus", ec.GetExchangeURL(), orgId, nodeId)

	err := InvokeExchangeRetryOnTransportError(ec.GetHTTPFactory(), "DELETE", targetURL, ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp)
	if err != nil {
		return err
	}

	return nil
}
