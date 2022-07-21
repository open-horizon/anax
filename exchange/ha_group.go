package exchange

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/exchangecommon"
)

// Get a single HA group with the given name
func GetHAGroupByName(ec ExchangeContext, orgId string, groupName string) (*exchangecommon.HAGroup, error) {
	glog.V(3).Infof("Getting HA group info for group %v in org %v.", groupName, orgId)

	var resp interface{}
	resp = new(exchangecommon.GetHAGroupResponse)

	targetURL := fmt.Sprintf("%vorgs/%v/hagroups/%v", ec.GetExchangeURL(), orgId, groupName)

	err := InvokeExchangeRetryOnTransportError(ec.GetHTTPFactory(), "GET", targetURL, ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp)
	if err != nil {
		return nil, err
	}

	if resp != nil {
		hagroups := resp.(*exchangecommon.GetHAGroupResponse).NodeGroups
		if hagroups != nil && len(hagroups) > 0 {
			return &hagroups[0], nil
		}
	}
	return nil, fmt.Errorf("The HA group \"%v\" does not exist in organization %v.", groupName, orgId)
}

// Get all HA groups in an organization
func GetAllHAGroups(ec ExchangeContext, orgId string) ([]exchangecommon.HAGroup, error) {
	glog.V(3).Infof("Getting all HA groups for org: %v.", orgId)

	var resp interface{}
	resp = new(exchangecommon.GetHAGroupResponse)

	targetURL := fmt.Sprintf("%vorgs/%v/hagroups", ec.GetExchangeURL(), orgId)
	err := InvokeExchangeRetryOnTransportError(ec.GetHTTPFactory(), "GET", targetURL, ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp)
	if err != nil {
		return nil, err
	}

	return resp.(*exchangecommon.GetHAGroupResponse).NodeGroups, nil
}
