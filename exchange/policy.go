package exchange

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/externalpolicy"
	"time"
)

// The node and service policy objects in the exchange are identical to the external policy object
// supported by the node/policy API, so it is embedded in the ExchangePolicy object.
type ExchangePolicy struct {
	externalpolicy.ExternalPolicy
	LastUpdated string `json:"lastUpdated,omitempty"`
}

func (e ExchangePolicy) String() string {
	return fmt.Sprintf("%v, "+
		"LastUpdated: %v",
		e.ExternalPolicy, e.LastUpdated)
}

func (e ExchangePolicy) ShortString() string {
	return e.String()
}

func (e *ExchangePolicy) GetExternalPolicy() externalpolicy.ExternalPolicy {
	return e.ExternalPolicy
}

func (e *ExchangePolicy) GetLastUpdated() string {
	return e.LastUpdated
}

// Retrieve the node policy object from the exchange. The input device Id is assumed to be prefixed with its org.
func GetNodePolicy(ec ExchangeContext, deviceId string) (*ExchangePolicy, error) {
	glog.V(3).Infof(rpclogString(fmt.Sprintf("getting node policy for %v.", deviceId)))

	// Get the node policy object. There should only be 1.
	var resp interface{}
	resp = new(ExchangePolicy)

	targetURL := fmt.Sprintf("%vorgs/%v/nodes/%v/policy", ec.GetExchangeURL(), GetOrg(deviceId), GetId(deviceId))
	for {
		if err, tpErr := InvokeExchange(ec.GetHTTPFactory().NewHTTPClient(nil), "GET", targetURL, ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(3).Infof(rpclogString(fmt.Sprintf("returning node policy for %v.", deviceId)))
			nodePolicy := resp.(*ExchangePolicy)
			return nodePolicy, nil
		}
	}

}

// Write an updated node policy to the exchange.
func PutNodePolicy(ec ExchangeContext, deviceId string, ep *ExchangePolicy) (*PutDeviceResponse, error) {
	// create PUT body
	var resp interface{}
	resp = new(PutDeviceResponse)
	targetURL := fmt.Sprintf("%vorgs/%v/nodes/%v/policy", ec.GetExchangeURL(), GetOrg(deviceId), GetId(deviceId))

	for {
		if err, tpErr := InvokeExchange(ec.GetHTTPFactory().NewHTTPClient(nil), "PUT", targetURL, ec.GetExchangeId(), ec.GetExchangeToken(), ep, &resp); err != nil {
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(3).Infof(rpclogString(fmt.Sprintf("put device policy for %v to exchange %v", deviceId, ep)))
			return resp.(*PutDeviceResponse), nil
		}
	}
}
