package exchange

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/exchangecommon"
)

const NMPExchangeResource = "managementpolicies"

// Get the single node management policy from the exchange
func GetSingleExchangeNodeManagementPolicy(ec ExchangeContext, policyOrg string, policyName string) (*exchangecommon.ExchangeNodeManagementPolicy, error) {
	glog.V(3).Infof("Getting node management policy : %v/%v", policyOrg, policyName)

	var resp interface{}
	resp = new(exchangecommon.ExchangeNodeManagementPolicy)

	targetURL := fmt.Sprintf("%vorgs/%v/%v/%v", ec.GetExchangeURL(), policyOrg, NMPExchangeResource, policyName)

	err := InvokeExchangeRetryOnTransportError(ec.GetHTTPFactory(), "GET", targetURL, ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp)
	if err != nil {
		return nil, err
	}

	nmp := resp.(*exchangecommon.ExchangeNodeManagementPolicy)
	return nmp, nil
}

type ExchangeNodeManagementPolicyResponse struct {
	Policies  map[string]exchangecommon.ExchangeNodeManagementPolicy `json:"managementPolicy"`
	LastIndex int `json:"lastIndex,omitempty"`
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
