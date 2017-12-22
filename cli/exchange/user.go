package exchange

import (
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"net/http"
)

func UserList(org string, userPw string) {
	exchUrlBase := cliutils.GetExchangeUrl()
	user, _ := cliutils.SplitIdToken(userPw)
	var output string
	httpCode := cliutils.ExchangeGet(exchUrlBase, "orgs/"+org+"/users/"+user, cliutils.OrgAndCreds(org,userPw), []int{200,404}, &output)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, "user '%s' not found in org %s", user, org)
	}
	fmt.Println(output)
}

func UserCreate(org string, userPw string, email string) {
	if org != "public" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "users can only be created in the Horizon Exchange for the 'public' organization.")
	}
	user, pw := cliutils.SplitIdToken(userPw)
	postUserReq := cliutils.UserExchangeReq{Password: pw, Admin: false, Email: email}
	cliutils.ExchangePutPost(http.MethodPost, cliutils.GetExchangeUrl(), "orgs/"+org+"/users/"+user, "", []int{201}, postUserReq)
}
