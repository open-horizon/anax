package exchange

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/semanticversion"
	"net/http"
	"strings"
)

const USER_EMAIL_OPTIONAL_EXCHANGE_VERSION = "2.61.0"

type ExchangeUsers struct {
	Users     map[string]ExchangeUser `json:"users"`
	ListIndex int                     `json:"lastIndex"`
}

type ExchangeUser struct {
	Password    string `json:"password"`
	Email       string `json:"email,omitempty"`
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

	var emailIsOptional bool
	var err error
	if emailIsOptional, err = checkExchangeVersionForOptionalUserEmail(org, userPwCreds); err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("failed to check exchange version, error: %v", err))
	}

	if email == "" && strings.Contains(user, "@") {
		email = user
	} else if email == "" && !emailIsOptional {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("an email must be specified because the Exchange API version is less than %v.", USER_EMAIL_OPTIONAL_EXCHANGE_VERSION))
	}

	if isHubAdmin && org != "root" {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Only users in the root org can be hubadmins."))
	}

	if !isHubAdmin && org == "root" {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Users in the root org must be hubadmins. Omit the -H option."))
	}

	cliutils.SetWhetherUsingApiKey(userPwCreds)

	postUserReq := cliutils.UserExchangeReq{Password: pw, Admin: isAdmin, HubAdmin: isHubAdmin, Email: email}
	cliutils.ExchangePutPost("Exchange", http.MethodPost, cliutils.GetExchangeUrl(), "orgs/"+org+"/users/"+user, cliutils.OrgAndCreds(org, userPwCreds), []int{201}, postUserReq, nil)
}

type UserExchangePatchAdmin struct {
	Admin bool `json:"admin"`
}

func UserSetAdmin(org, userPwCreds, user string, isAdmin bool) {
	msgPrinter := i18n.GetMessagePrinter()

	if isAdmin && org == "root" {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("A user in the root org cannot be an org admin."))
	}

	cliutils.SetWhetherUsingApiKey(userPwCreds)
	patchUserReq := UserExchangePatchAdmin{Admin: isAdmin}
	cliutils.ExchangePutPost("Exchange", http.MethodPatch, cliutils.GetExchangeUrl(), "orgs/"+org+"/users/"+user, cliutils.OrgAndCreds(org, userPwCreds), []int{201}, patchUserReq, nil)
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

// check if current exchange version is greater than 2.61.0, which makes user email optional.
// return (true, nil) if user email is optional
// return (false, nil) if user email is required
func checkExchangeVersionForOptionalUserEmail(org, userPwCreds string) (bool, error) {
	cliutils.SetWhetherUsingApiKey(userPwCreds)

	var output []byte
	cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "admin/version", cliutils.OrgAndCreds(org, userPwCreds), []int{200}, &output)

	exchangeVersion := strings.TrimSpace(string(output))

	if !semanticversion.IsVersionString(exchangeVersion) {
		return false, fmt.Errorf(i18n.GetMessagePrinter().Sprintf("The current exchange version %v is not a valid version string.", exchangeVersion))
	} else if comp, err := semanticversion.CompareVersions(exchangeVersion, USER_EMAIL_OPTIONAL_EXCHANGE_VERSION); err != nil {
		return false, fmt.Errorf(i18n.GetMessagePrinter().Sprintf("Failed to compare the versions. %v", err))
	} else if comp < 0 {
		// current exchange version < 2.61.0. User email is required
		return false, nil
	} else {
		// current exchange version >= 2.61.0. User email is optional
		return true, nil
	}

}
