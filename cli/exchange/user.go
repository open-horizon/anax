package exchange

import (
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"net/http"
	"strings"
)

type ExchangeUsers struct {
	Users     map[string]interface{} `json:"users"`
	ListIndex int                    `json:"lastIndex"`
}

func UserList(org, userPwCreds string, allUsers bool) {
	cliutils.SetWhetherUsingApiKey(userPwCreds)
	exchUrlBase := cliutils.GetExchangeUrl()
	var user string
	if !allUsers {
		user, _ = cliutils.SplitIdToken(userPwCreds)
		user = "/" + user
	}
	var users ExchangeUsers
	httpCode := cliutils.ExchangeGet(exchUrlBase, "orgs/"+org+"/users"+user, cliutils.OrgAndCreds(org, userPwCreds), []int{200, 404}, &users)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, "user '%s' not found in org %s", strings.TrimPrefix(user, "/"), org)
	}
	output := cliutils.MarshalIndent(users.Users, "exchange users list")
	fmt.Println(output)
}

func UserCreate(org, userPwCreds, user, pw, email string, isAdmin bool) {
	if email == "" {
		if strings.Contains(user, "@") {
			email = user
		} else {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "the email must be specified via -e if the username is not an email address.")
		}
	}
	cliutils.SetWhetherUsingApiKey(userPwCreds)
	postUserReq := cliutils.UserExchangeReq{Password: pw, Admin: isAdmin, Email: email}
	cliutils.ExchangePutPost(http.MethodPost, cliutils.GetExchangeUrl(), "orgs/"+org+"/users/"+user, cliutils.OrgAndCreds(org, userPwCreds), []int{201}, postUserReq)
}

type UserExchangePatchAdmin struct {
	Admin bool `json:"admin"`
}

func UserSetAdmin(org, userPwCreds, user string, isAdmin bool) {
	cliutils.SetWhetherUsingApiKey(userPwCreds)
	patchUserReq := UserExchangePatchAdmin{Admin: isAdmin}
	cliutils.ExchangePutPost(http.MethodPatch, cliutils.GetExchangeUrl(), "orgs/"+org+"/users/"+user, cliutils.OrgAndCreds(org, userPwCreds), []int{201}, patchUserReq)
}

func UserRemove(org, userPwCreds, user string, force bool) {
	cliutils.SetWhetherUsingApiKey(userPwCreds)
	if !force {
		cliutils.ConfirmRemove("Warning: this will also delete all Exchange resources owned by this user (nodes, microservices, workloads, patterns, etc). Are you sure you want to remove user '" + org + "/" + user + "' from the Horizon Exchange?")
	}

	httpCode := cliutils.ExchangeDelete(cliutils.GetExchangeUrl(), "orgs/"+org+"/users/"+user, cliutils.OrgAndCreds(org, userPwCreds), []int{204, 404})
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, "user '%s' not found in org %s", user, org)
	}
}
