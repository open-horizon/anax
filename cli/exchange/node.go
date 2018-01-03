package exchange

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/exchange"
	"net/http"
	"strings"
)

// We only care about handling the microservice names, so the rest is left as interface{} and will be passed from the exchange to the display
type ExchangeNodes struct {
	LastIndex int                    `json:"lastIndex"`
	Nodes     map[string]interface{} `json:"nodes"`
}

func NodeList(org string, userPw string, node string, namesOnly bool) {
	if node != "" {
		node = "/" + node
	}
	if namesOnly {
		// Only display the names
		var resp ExchangeNodes
		cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes"+node, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &resp)
		var nodes []string
		for n := range resp.Nodes {
			nodes = append(nodes, n)
		}
		jsonBytes, err := json.MarshalIndent(nodes, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'exchange node list' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		// Display the full resources
		var output string
		httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes"+node, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
		if httpCode == 404 && node != "" {
			cliutils.Fatal(cliutils.NOT_FOUND, "node '%s' not found in org %s", strings.TrimPrefix(node, "/"), org)
		}
		fmt.Println(output)
	}
}

func NodeCreate(org string, nodeIdTok string, userPw string, email string) {
	nodeId, nodeToken := cliutils.SplitIdToken(nodeIdTok)
	exchUrlBase := cliutils.GetExchangeUrl()
	putNodeReq := exchange.PutDeviceRequest{Token: nodeToken, Name: nodeId, SoftwareVersions: make(map[string]string), PublicKey: []byte("")} // we only need to set the token
	httpCode := cliutils.ExchangePutPost(http.MethodPut, exchUrlBase, "orgs/"+org+"/nodes/"+nodeId, cliutils.OrgAndCreds(org, userPw), []int{201, 401}, putNodeReq)
	if httpCode == 401 {
		user, pw := cliutils.SplitIdToken(userPw)
		if org == "public" && email != "" {
			// In the public org we can create a user anonymously, so try that
			fmt.Printf("User %s/%s does not exist in the exchange with the specified password, creating it...\n", org, user)
			postUserReq := cliutils.UserExchangeReq{Password: pw, Admin: false, Email: email}
			httpCode = cliutils.ExchangePutPost(http.MethodPost, exchUrlBase, "orgs/"+org+"/users/"+user, "", []int{201}, postUserReq)
			httpCode = cliutils.ExchangePutPost(http.MethodPut, exchUrlBase, "orgs/"+org+"/nodes/"+nodeId, cliutils.OrgAndCreds(org, userPw), []int{201}, putNodeReq)
		} else {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "user '%s/%s' does not exist with the specified password or -e was not specified to be able to create it in the 'public' org.", org, user)
		}
	}
}
