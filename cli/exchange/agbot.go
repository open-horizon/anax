package exchange

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"net/http"
	"strings"
)

// We only care about handling the microservice names, so the rest is left as interface{} and will be passed from the exchange to the display
type ExchangeAgbots struct {
	LastIndex int                    `json:"lastIndex"`
	Agbots    map[string]interface{} `json:"agbots"`
}

func AgbotList(org string, userPw string, agbot string, namesOnly bool) {
	cliutils.SetWhetherUsingApiKey(userPw)
	if agbot != "" {
		agbot = "/" + agbot
	}
	if namesOnly && agbot == "" {
		// Only display the names
		var resp ExchangeAgbots
		cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/agbots"+agbot, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &resp)
		agbots := []string{}
		for a := range resp.Agbots {
			agbots = append(agbots, a)
		}
		jsonBytes, err := json.MarshalIndent(agbots, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'exchange agbot list' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		// Display the full resources
		var output string
		httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/agbots"+agbot, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
		if httpCode == 404 && agbot != "" {
			cliutils.Fatal(cliutils.NOT_FOUND, "agbot '%s' not found in org %s", strings.TrimPrefix(agbot, "/"), org)
		}
		fmt.Println(output)
	}
}

func formPatternId(patternOrg, pattern string) string {
	return patternOrg + "_" + pattern
}

func AgbotListPatterns(org, userPw, agbot, patternOrg, pattern string) {
	cliutils.SetWhetherUsingApiKey(userPw)
	var patternId string
	if patternOrg != "" || pattern != "" {
		if patternOrg == "" || pattern == "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "both patternorg and pattern must be specified (or neither)")
		}
		patternId = "/" + formPatternId(patternOrg, pattern)
	}
	// Display the full resources
	var output string
	httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/agbots/"+agbot+"/patterns"+patternId, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
	if httpCode == 404 && patternOrg != "" && pattern != "" {
		cliutils.Fatal(cliutils.NOT_FOUND, "pattern '%s' with org '%s' not found in agbot '%s'", pattern, patternOrg, agbot)
	}
	fmt.Println(output)
}

type ServedPattern struct {
	Org     string `json:"patternOrgid"`
	Pattern string `json:"pattern"`
}

func AgbotAddPattern(org, userPw, agbot, patternOrg, pattern string) {
	cliutils.SetWhetherUsingApiKey(userPw)
	patternId := formPatternId(patternOrg, pattern)
	input := ServedPattern{Org: patternOrg, Pattern: pattern}
	cliutils.ExchangePutPost(http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+org+"/agbots/"+agbot+"/patterns/"+patternId, cliutils.OrgAndCreds(org, userPw), []int{201}, input)
}

func AgbotRemovePattern(org, userPw, agbot, patternOrg, pattern string) {
	cliutils.SetWhetherUsingApiKey(userPw)
	patternId := formPatternId(patternOrg, pattern)
	cliutils.ExchangeDelete(cliutils.GetExchangeUrl(), "orgs/"+org+"/agbots/"+agbot+"/patterns/"+patternId, cliutils.OrgAndCreds(org, userPw), []int{204})
}
