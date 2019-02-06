package exchange

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"net/http"
	"strings"
)

type ExchangeUsers struct {
	Users     map[string]interface{} `json:"users"`
	ListIndex int                    `json:"lastIndex"`
}

func UserList(org, userPwCreds, theUser string, allUsers, namesOnly bool) {
	cliutils.SetWhetherUsingApiKey(userPwCreds)

	// Decide which users should be shown
	exchUrlBase := cliutils.GetExchangeUrl()
	if allUsers {
		theUser = ""
	} else if theUser == "" {
		theUser, _ = cliutils.SplitIdToken(userPwCreds)
	} // else we list the user specified in theUser

	// Get users
	var users ExchangeUsers
	httpCode := cliutils.ExchangeGet(exchUrlBase, "orgs/"+org+"/users"+cliutils.AddSlash(theUser), cliutils.OrgAndCreds(org, userPwCreds), []int{200, 404}, &users)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, "theUser '%s' not found in org %s", strings.TrimPrefix(theUser, "/"), org)
	}

	// Decide how much of each user should be shown
	if namesOnly {
		usernames := []string{} // this is important (instead of leaving it nil) so json marshaling displays it as [] instead of null
		for u := range users.Users {
			usernames = append(usernames, u)
		}
		jsonBytes, err := json.MarshalIndent(usernames, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'exchange user list' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	} else { // show full resources
		output := cliutils.MarshalIndent(users.Users, "exchange users list")
		fmt.Println(output)
	}
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
		cliutils.ConfirmRemove("Warning: this will also delete all Exchange resources owned by this user (nodes, services, patterns, etc). Are you sure you want to remove user '" + org + "/" + user + "' from the Horizon Exchange?")
	}

	httpCode := cliutils.ExchangeDelete(cliutils.GetExchangeUrl(), "orgs/"+org+"/users/"+user, cliutils.OrgAndCreds(org, userPwCreds), []int{204, 404})
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, "user '%s' not found in org %s", user, org)
	}
}
