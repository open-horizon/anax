package exchange

import (
	"fmt"
	"github.com/golang/glog"
	"strings"
	"time"
)

type VaultSecretExistsResponse struct {
	Exists bool `json:"exists"`
}

// Calls the agbot secure API to check if the given vault secret exists or not
func VaultSecretExists(ec ExchangeContext, agbotURL string, org string, userName string, nodeName string, secretName string) (bool, error) {

	var resp interface{}
	resp = new(VaultSecretExistsResponse)

	url := strings.TrimRight(agbotURL, "/")

	if nodeName != "" && userName != "" {
		url += fmt.Sprintf("/org/%v/secrets/user/%v/node/%v/%v", org, userName, nodeName, secretName)
	} else if nodeName != "" {
		url += fmt.Sprintf("/org/%v/secrets/node/%v/%v", org, nodeName, secretName)
	} else if userName != "" {
		url += fmt.Sprintf("/org/%v/secrets/user/%v/%v", org, userName, secretName)
	} else {
		url += fmt.Sprintf("/org/%v/secrets/%v", org, secretName)
	}

	retryCount := ec.GetHTTPFactory().RetryCount
	retryInterval := ec.GetHTTPFactory().GetRetryInterval()
	for {
		if err, tpErr := InvokeExchange(ec.GetHTTPFactory().NewHTTPClient(nil), "LIST", url, ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp); err != nil {
			glog.Errorf(rpclogString(fmt.Sprintf(err.Error())))
			return false, err
		} else if tpErr != nil {
			glog.Warningf(rpclogString(fmt.Sprintf(tpErr.Error())))
			if ec.GetHTTPFactory().RetryCount == 0 {
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else if retryCount == 0 {
				return false, fmt.Errorf("Exceeded %v retries for error: %v", ec.GetHTTPFactory().RetryCount, tpErr)
			} else {
				retryCount--
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			}
		} else {
			ret := resp.(*VaultSecretExistsResponse)
			return ret.Exists, nil
		}
	}
}
