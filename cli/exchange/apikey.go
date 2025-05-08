package exchange

import (
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/i18n"
	"net/http"
)


type ApiKeyCreateRequest struct {
	Description string `json:"description"`
}

type ApiKeyListResponse struct {
	ApiKeys []ApiKey `json:"apikeys"`
}

type ApiKey struct {
	Id          string `json:"id"`
	Description string `json:"description"`
	Owner       string `json:"owner"`
	LastUpdated string `json:"lastUpdated"`
}

type ApiKeyCreateResponse struct {
	Id          string `json:"id"`
	Description string `json:"description"`
	Owner       string `json:"owner"`
	Value       string `json:"value"`
	LastUpdated string `json:"lastUpdated"`
}
func ApiKeyGetById(org, userPwCreds, username, keyId string) {
	msgPrinter := i18n.GetMessagePrinter()
	cliutils.SetWhetherUsingApiKey(userPwCreds)

	if org == "" || username == "" || keyId == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR,
		msgPrinter.Sprintf("Organization, username, and key ID must be specified."))
	}

	url := fmt.Sprintf("orgs/%s/users/%s/apikeys/%s", org, username, keyId)
	var single ApiKey
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), url,
		cliutils.OrgAndCreds(org, userPwCreds), []int{http.StatusOK, http.StatusNotFound }, &single)

	if httpCode == http.StatusNotFound  {
		cliutils.Fatal(cliutils.NOT_FOUND,
			msgPrinter.Sprintf("API key '%s' not found for user '%s' in org '%s'", keyId, username, org))
	}

	output := cliutils.MarshalIndent(single, "exchange apikey get")
	fmt.Println(output)
}

func ApiKeyCreate(org, userPwCreds, username, description string) {
	cliutils.SetWhetherUsingApiKey(userPwCreds)

	postReq := ApiKeyCreateRequest{Description: description}
	var resp ApiKeyCreateResponse
	url := fmt.Sprintf("orgs/%s/users/%s/apikeys", org, username)

	httpCode := cliutils.ExchangePutPost("Exchange", http.MethodPost, cliutils.GetExchangeUrl(), url,
		cliutils.OrgAndCreds(org, userPwCreds), []int{http.StatusCreated}, postReq, &resp)
	if httpCode == http.StatusCreated {
		output := cliutils.MarshalIndent(resp, "exchange apikey create")
		fmt.Println(output)
	}
}


func ApiKeyRemove(org, userPwCreds, username, keyId string, force bool) {
	msgPrinter := i18n.GetMessagePrinter()
	cliutils.SetWhetherUsingApiKey(userPwCreds)

	if !force {
		cliutils.ConfirmRemove(
			msgPrinter.Sprintf("Are you sure you want to remove apikey %v from user %v/%v?", keyId, org, username))
	}

	url := fmt.Sprintf("orgs/%s/users/%s/apikeys/%s", org, username, keyId)
	httpCode := cliutils.ExchangeDelete("Exchange", cliutils.GetExchangeUrl(), url,
		cliutils.OrgAndCreds(org, userPwCreds), []int{http.StatusNoContent, http.StatusNotFound })

	if httpCode == http.StatusNotFound  {
		cliutils.Fatal(cliutils.NOT_FOUND,
			msgPrinter.Sprintf("apikey '%s' not found for user '%s' in org '%s'", keyId, username, org))
	}
}
