package exchange

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/i18n"
	"net/http"
	"strings"
)

type ExchangeUsers struct {
	Users     map[string]ExchangeUser `json:"users"`
	ListIndex int                     `json:"lastIndex"`
}

type ExchangeUser struct {
	Password    string `json:"password"`
	Email       string `json:"email"`
	Admin       bool   `json:"admin"`
	HubAdmin    bool   `json:"hubAdmin"`
	LastUpdated string `json:"lastUpdated"`
	UpdatedBy   string `json:"updatedBy"`
}

func UserList(org, userPwCreds, theUser string, allUsers, namesOnly bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	cliutils.SetWhetherUsingApiKey(userPwCreds)

	// Decide which users should be shown
	exchUrlBase := cliutils.GetExchangeUrl()
	if allUsers {
		theUser = ""
	} else if theUser == "" {
		theUser, _ = cliutils.SplitIdToken(userPwCreds)
		_, theUser = cliutils.TrimOrg("", theUser)
	} // else we list the user specified in theUser

	// Get users
	var users ExchangeUsers
	httpCode := cliutils.ExchangeGet("Exchange", exchUrlBase, "orgs/"+org+"/users"+cliutils.AddSlash(theUser), cliutils.OrgAndCreds(org, userPwCreds), []int{200, 404}, &users)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("user '%s' not found in org %s", strings.TrimPrefix(theUser, "/"), org))
	}

	// Decide how much of each user should be shown
	if namesOnly {
		usernames := []string{} // this is important (instead of leaving it nil) so json marshaling displays it as [] instead of null
		for u := range users.Users {
			usernames = append(usernames, u)
		}
		jsonBytes, err := json.MarshalIndent(usernames, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'exchange user list' output: %v", err))
		}
		fmt.Printf("%s\n", jsonBytes)
	} else { // show full resources
		output := cliutils.MarshalIndent(users.Users, "exchange users list")
		fmt.Println(output)
	}
}

func UserCreate(org, userPwCreds, user, pw, email string, isAdmin bool, isHubAdmin bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if email == "" {
		if strings.Contains(user, "@") {
			email = user
		} else {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("the email must be specified via -e if the username is not an email address."))
		}
	}

	if isHubAdmin && org != "root" {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Only exchange users in the root org can be hubadmins."))
	}

	cliutils.SetWhetherUsingApiKey(userPwCreds)

	postUserReq := cliutils.UserExchangeReq{Password: pw, Admin: isAdmin, HubAdmin: isHubAdmin, Email: email}
	cliutils.ExchangePutPost("Exchange", http.MethodPost, cliutils.GetExchangeUrl(), "orgs/"+org+"/users/"+user, cliutils.OrgAndCreds(org, userPwCreds), []int{201}, postUserReq, nil)
}

type UserExchangePatchAdmin struct {
	Admin bool `json:"admin"`
}

func UserSetAdmin(org, userPwCreds, user string, isAdmin bool) {
	cliutils.SetWhetherUsingApiKey(userPwCreds)
	patchUserReq := UserExchangePatchAdmin{Admin: isAdmin}
	cliutils.ExchangePutPost("Exchange", http.MethodPatch, cliutils.GetExchangeUrl(), "orgs/"+org+"/users/"+user, cliutils.OrgAndCreds(org, userPwCreds), []int{201}, patchUserReq, nil)
}

func UserSetHubAdmin(org, userPwCreds, user string, isHubAdmin bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	_, userId := cliutils.TrimOrg(org, user)

	// Get the user from the exchange
	var userList ExchangeUsers
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/root/users/"+userId, cliutils.OrgAndCreds(org, userPwCreds), []int{200, 404, 403}, &userList)
	if httpCode == 200 {
		// If the user already exists in the root org, we need to update it with a PUT to set hubAdmin to correct state
		exchUser := cliutils.AddOrg("root", userId)
		if existingUser, ok := userList.Users[exchUser]; !ok {
			cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("user '%s' not found in list returned from exchange", exchUser))
		} else {
			updatedUser := cliutils.UserExchangeReq{Password: existingUser.Password, Admin: existingUser.Admin, HubAdmin: isHubAdmin, Email: existingUser.Email}
			cliutils.ExchangePutPost("Exchange", http.MethodPut, cliutils.GetExchangeUrl(), "orgs/root/users/"+userId, cliutils.OrgAndCreds(org, userPwCreds), []int{201}, updatedUser, nil)
		}
	} else if httpCode == 403 {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Only hubadmin users and the exchange root user can create hubadmins."))
	} else if isHubAdmin {
		// If the user does not exist in the root org and the user isHubAdmin, create them with hubAdmin set to true and no other fields
		postUserReq := cliutils.UserExchangeReq{HubAdmin: true}
		httpCode := cliutils.ExchangePutPost("Exchange", http.MethodPost, cliutils.GetExchangeUrl(), "orgs/root/users/"+userId, cliutils.OrgAndCreds(org, userPwCreds), []int{201, 400, 403}, postUserReq, nil)
		if httpCode == 400 {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Hubadmin status cannot be set because the user %s does not exist in the root org.", user))
		} else if httpCode == 403 {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Only hubadmin users and the exchange root user can create hubadmins."))
		}
	}
	// If the user does not exist in the root org and isHubAdmin is false, we don't need to do anything
}

func UserRemove(org, userPwCreds, user string, force bool) {
	cliutils.SetWhetherUsingApiKey(userPwCreds)
	if !force {
		cliutils.ConfirmRemove(i18n.GetMessagePrinter().Sprintf("Warning: this will also delete all Exchange resources owned by this user (nodes, services, patterns, etc). Are you sure you want to remove user %v/%v from the Horizon Exchange?", org, user))
	}

	httpCode := cliutils.ExchangeDelete("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/users/"+user, cliutils.OrgAndCreds(org, userPwCreds), []int{204, 404})
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, i18n.GetMessagePrinter().Sprintf("user '%s' not found in org %s", user, org))
	}
}
