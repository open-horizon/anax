package exchange

import (
	"fmt"
	"net/http"
	"strings"
)

func HANodeCanExecuteNMP(ec ExchangeContext, agbotURL string, haGroupName string, nmpName string) (bool, error) {
	exNodeId := ec.GetExchangeId()
	node := GetId(exNodeId)
	org := GetOrg(exNodeId)

	targetURL := strings.TrimRight(agbotURL, "/")
	targetURL += fmt.Sprintf("/org/%s/hagroup/%s/nodemanagement/%s/%s", org, haGroupName, node, nmpName)

	var resp interface{}
	resp = new(PutPostDeleteStandardResponse)

	if err := InvokeExchangeRetryOnTransportError(ec.GetHTTPFactory(), "POST", targetURL, ec.GetExchangeId(), ec.GetExchangeToken(), nil, &resp); err != nil {
		return false, err
	} else if putResp, ok := resp.(*PutPostDeleteStandardResponse); !ok {
		return false, fmt.Errorf("Failed to unmarshal agbot response %v to the expected format.", resp)
	} else if putResp.Code == fmt.Sprintf("%v", http.StatusCreated) {
		return true, nil
	} else if putResp.Code == fmt.Sprintf("%v", http.StatusConflict) {
		return false, nil
	} else {
		return false, fmt.Errorf("Error: unexpected status code returned by call to %v. Code was %v. Resp was %v", targetURL, putResp.Code, putResp.Msg)
	}
}
