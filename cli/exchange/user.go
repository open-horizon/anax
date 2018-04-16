package exchange

import (
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"net/http"
)

type ExchangeUsers struct {
	Users     map[string]interface{} `json:"users"`
	ListIndex int                    `json:"lastIndex"`
}

func UserList(org string, userPw string) {
	cliutils.SetWhetherUsingApiKey(userPw)
	exchUrlBase := cliutils.GetExchangeUrl()
	user, _ := cliutils.SplitIdToken(userPw)
	var users ExchangeUsers
	httpCode := cliutils.ExchangeGet(exchUrlBase, "orgs/"+org+"/users/"+user, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &users)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, "user '%s' not found in org %s", user, org)
	}
	output := cliutils.MarshalIndent(users.Users, "exchange users list")
	fmt.Println(output)
}

func UserCreate(org string, userPw string, email string) {
	if org != "public" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "users can only be created in the Horizon Exchange for the 'public' organization.")
	}
	cliutils.SetWhetherUsingApiKey(userPw)
	user, pw := cliutils.SplitIdToken(userPw)
	postUserReq := cliutils.UserExchangeReq{Password: pw, Admin: false, Email: email}
	cliutils.ExchangePutPost(http.MethodPost, cliutils.GetExchangeUrl(), "orgs/"+org+"/users/"+user, "", []int{201}, postUserReq)
}

func UserRemove(org, userPw, user string, force bool) {
	cliutils.SetWhetherUsingApiKey(userPw)
	if !force {
		cliutils.ConfirmRemove("Warning: this will also delete all Exchange resources owned by this user (nodes, microservices, workloads, patterns, etc). Are you sure you want to remove user '" + org + "/" + user + "' from the Horizon Exchange?")
	}

	httpCode := cliutils.ExchangeDelete(cliutils.GetExchangeUrl(), "orgs/"+org+"/users/"+user, cliutils.OrgAndCreds(org, userPw), []int{204, 404})
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, "user '%s' not found in org %s", user, org)
	}
}
