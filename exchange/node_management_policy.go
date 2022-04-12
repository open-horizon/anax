package exchange

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/exchangecommon"
)

const NMPExchangeResource = "managementpolicies"

type GetNMPResponse struct {
	exchangecommon.ExchangeNodeManagementPolicy
	LastUpdated string `json:"lastUpdated"`
	Created     string `json:"created"`
}

// Get the single node management policy from the exchange
func GetSingleExchangeNodeManagementPolicy(ec ExchangeContext, policyOrg string, policyName string) (*exchangecommon.ExchangeNodeManagementPolicy, error) {
	glog.V(3).Infof("Getting node management policy : %v/%v", policyOrg, policyName)

	var resp interface{}
	resp = new(GetNMPResponse)

	targetURL := fmt.Sprintf("%vorgs/%v/%v/%v", ec.GetExchangeURL(), policyOrg, NMPExchangeResource, policyName)

	err := InvokeExchangeRetryOnTransportError(ec.GetHTTPFactory(), "GET", targetURL, ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp)
	if err != nil {
		return nil, err
	}

	nmp := resp.(GetNMPResponse).ExchangeNodeManagementPolicy
	return &nmp, nil
}

type ExchangeNodeManagementPolicyResponse struct {
	Policies  map[string]exchangecommon.ExchangeNodeManagementPolicy `json:"managementPolicy,omitempty"`
	LastIndex int                                                    `json:"lastIndex,omitempty"`
}

// Get all node management policies in the given org
func GetAllExchangeNodeManagementPolicy(ec ExchangeContext, policyOrg string) (*map[string]exchangecommon.ExchangeNodeManagementPolicy, error) {
	glog.V(3).Infof("Getting all node management policies for org %v", policyOrg)

	var resp interface{}
	resp = new(ExchangeNodeManagementPolicyResponse)

	targetURL := fmt.Sprintf("%vorgs/%v/%v", ec.GetExchangeURL(), policyOrg, NMPExchangeResource)

	err := InvokeExchangeRetryOnTransportError(ec.GetHTTPFactory(), "GET", targetURL, ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp)
	if err != nil {
		return nil, err
	}

	policies := resp.(*ExchangeNodeManagementPolicyResponse).Policies
	return &policies, nil
}

// Get lists of the agent file versions currently availible in the css
func GetNodeUpgradeVersions(ec ExchangeContext) (*exchangecommon.AgentFileVersions, error) {
	glog.V(3).Infof("Getting the availible versions for agent upgrade packages from the exchange.")

	var resp interface{}
	resp = new(exchangecommon.AgentFileVersions)

	targetURL := fmt.Sprintf("%vorgs/IBM/AgentFileVersion", ec.GetExchangeURL())

	err := InvokeExchangeRetryOnTransportError(ec.GetHTTPFactory(), "GET", targetURL, ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp)
	if err != nil {
		return nil, err
	}

	return resp.(*exchangecommon.AgentFileVersions), nil
}

// Sets lists of the agent file versions currently availible in the css
func PutNodeUpgradeVersions(ec ExchangeContext, afv *exchangecommon.AgentFileVersions) error {
	glog.V(3).Infof("Putting the availible versions for agent upgrade packages to the exchange. %v", afv)

	var resp interface{}
	resp = ""

	targetURL := fmt.Sprintf("%vorgs/IBM/AgentFileVersion", ec.GetExchangeURL())

	err := InvokeExchangeRetryOnTransportError(ec.GetHTTPFactory(), "PUT", targetURL, ec.GetExchangeId(), ec.GetExchangeToken(), afv, &resp)
	if err != nil {
		return err
	}

	return nil
}
