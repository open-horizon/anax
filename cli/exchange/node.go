package exchange

import (
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/exchange"
	"net/http"
)

func NodeList(org string, nodeIdTok string) {
	nodeId, _ := cliutils.SplitIdToken(nodeIdTok)
	var output string
	cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes/"+nodeId, org+"/"+nodeIdTok, []int{200}, &output)
	fmt.Println(output)
}

func NodeCreate(org string, nodeIdTok string, userPw string, email string) {
	nodeId, nodeToken := cliutils.SplitIdToken(nodeIdTok)
	exchUrlBase := cliutils.GetExchangeUrl()
	putNodeReq := exchange.PutDeviceRequest{Token: nodeToken, Name: nodeId, SoftwareVersions: make(map[string]string), PublicKey: []byte("")} // we only need to set the token
	httpCode := cliutils.ExchangePutPost(http.MethodPut, exchUrlBase, "orgs/"+org+"/nodes/"+nodeId, org+"/"+userPw, []int{201, 401}, putNodeReq)
	if httpCode == 401 {
		user, pw := cliutils.SplitIdToken(userPw)
		if org == "public" && email != "" {
			// In the public org we can create a user anonymously, so try that
			fmt.Printf("User %s/%s does not exist in the exchange with the specified password, creating it...\n", org, user)
			postUserReq := cliutils.UserExchangeReq{Password: pw, Admin: false, Email: email}
			httpCode = cliutils.ExchangePutPost(http.MethodPost, exchUrlBase, "orgs/"+org+"/users/"+user, "", []int{201}, postUserReq)
			httpCode = cliutils.ExchangePutPost(http.MethodPut, exchUrlBase, "orgs/"+org+"/nodes/"+nodeId, org+"/"+userPw, []int{201}, putNodeReq)
		} else {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "user '%s/%s' does not exist with the specified password or -e was not specified to be able to create it in the 'public' org.", org, user)
		}
	}
}
