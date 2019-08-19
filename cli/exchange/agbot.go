package exchange

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/i18n"
	"net/http"
	"os"
)

// We only care about handling the agbot names, so the rest is left as interface{} and will be passed from the exchange to the display
type ExchangeAgbots struct {
	LastIndex int                    `json:"lastIndex"`
	Agbots    map[string]interface{} `json:"agbots"`
}

func AgbotList(org string, userPw string, agbot string, namesOnly bool) {
	cliutils.SetWhetherUsingApiKey(userPw)
	org, agbot = cliutils.TrimOrg(org, agbot)
	if agbot == "*" {
		agbot = ""
	}
	if namesOnly && agbot == "" {
		// Only display the names
		var resp ExchangeAgbots
		cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/agbots"+cliutils.AddSlash(agbot), cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &resp)
		agbots := []string{}
		for a := range resp.Agbots {
			agbots = append(agbots, a)
		}
		jsonBytes, err := json.MarshalIndent(agbots, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, i18n.GetMessagePrinter().Sprintf("failed to marshal 'exchange agbot list' output: %v", err))
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		// Display the full resources
		var agbots ExchangeAgbots
		httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/agbots"+cliutils.AddSlash(agbot), cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &agbots)
		if httpCode == 404 && agbot != "" {
			cliutils.Fatal(cliutils.NOT_FOUND, i18n.GetMessagePrinter().Sprintf("agbot '%s' not found in org %s", agbot, org))
		}
		output := cliutils.MarshalIndent(agbots.Agbots, "exchange agbots list")
		fmt.Println(output)
	}
}

func formServicedObjectId(objOrg, obj, nodeOrg string) string {
	return objOrg + "_" + obj + "_" + nodeOrg
}

type ExchangeAgbotPatterns struct {
	Patterns map[string]interface{} `json:"patterns"`
}

func AgbotListPatterns(org, userPw, agbot, patternOrg, pattern, nodeOrg string) {
	cliutils.SetWhetherUsingApiKey(userPw)
	org, agbot = cliutils.TrimOrg(org, agbot)
	var patternId string
	if patternOrg != "" || pattern != "" {
		if patternOrg == "" || pattern == "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, i18n.GetMessagePrinter().Sprintf("both patternorg and pattern must be specified (or neither)"))
		}
		if nodeOrg == "" {
			nodeOrg = patternOrg
		}
		patternId = formServicedObjectId(patternOrg, pattern, nodeOrg)
	}
	// Display the full resources
	var patterns ExchangeAgbotPatterns
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/agbots/"+agbot+"/patterns"+cliutils.AddSlash(patternId), cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &patterns)
	if httpCode == 404 && patternOrg != "" && pattern != "" {
		cliutils.Fatal(cliutils.NOT_FOUND, i18n.GetMessagePrinter().Sprintf("pattern '%s' with org '%s' and node org '%s' not found in agbot '%s'", pattern, patternOrg, nodeOrg, agbot))
	}
	output := cliutils.MarshalIndent(patterns.Patterns, "exchange agbot listpattern")
	fmt.Println(output)
}

type ServedPattern struct {
	PatternOrg string `json:"patternOrgid"`
	Pattern    string `json:"pattern"`
	NodeOrg    string `json:"nodeOrgid"`
}

func AgbotAddPattern(org, userPw, agbot, patternOrg, pattern, nodeOrg string) {
	cliutils.SetWhetherUsingApiKey(userPw)
	org, agbot = cliutils.TrimOrg(org, agbot)
	if nodeOrg == "" {
		nodeOrg = patternOrg
	}
	input := ServedPattern{PatternOrg: patternOrg, Pattern: pattern, NodeOrg: nodeOrg}
	httpCode := cliutils.ExchangePutPost("Exchange", http.MethodPost, cliutils.GetExchangeUrl(), "orgs/"+org+"/agbots/"+agbot+"/patterns", cliutils.OrgAndCreds(org, userPw), []int{201, 409}, input)
	if httpCode == 409 {
		i18n.GetMessagePrinter().Printf("Pattern '%s' with org '%s' and node org '%s' already exists in agbot '%s'", pattern, patternOrg, nodeOrg, agbot)
		i18n.GetMessagePrinter().Println()
		os.Exit(cliutils.CLI_INPUT_ERROR)
	}
}

func AgbotRemovePattern(org, userPw, agbot, patternOrg, pattern, nodeOrg string) {
	cliutils.SetWhetherUsingApiKey(userPw)
	org, agbot = cliutils.TrimOrg(org, agbot)
	if nodeOrg == "" {
		nodeOrg = patternOrg
	}
	patternId := formServicedObjectId(patternOrg, pattern, nodeOrg)
	cliutils.ExchangeDelete("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/agbots/"+agbot+"/patterns/"+patternId, cliutils.OrgAndCreds(org, userPw), []int{204})
}

type ServedBusinessPolicy struct {
	BusinessPolOrg string `json:"businessPolOrgid"` // defaults to nodeOrgid
	BusinessPol    string `json:"businessPol"`      // '*' means all
	NodeOrg        string `json:"nodeOrgid"`
	LastUpdated    string `json:"lastUpdated"`
}

func AgbotListBusinessPolicy(org, userPw, agbot string) {
	cliutils.SetWhetherUsingApiKey(userPw)
	org, agbot = cliutils.TrimOrg(org, agbot)
	// Display the full resources
	resp := new(exchange.GetAgbotsBusinessPolsResponse)
	cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/agbots/"+agbot+"/businesspols", cliutils.OrgAndCreds(org, userPw), []int{200, 404}, resp)
	output := cliutils.MarshalIndent(resp.BusinessPols, "exchange agbot listbusinesspol")
	fmt.Println(output)
}

// Add the business policy to the agot supporting list. Currently
// all the patterns are open to all the nodes within the same organization.
func AgbotAddBusinessPolicy(org, userPw, agbot, polOrg string) {
	cliutils.SetWhetherUsingApiKey(userPw)
	org, agbot = cliutils.TrimOrg(org, agbot)

	input := exchange.ServedBusinessPolicy{BusinessPolOrg: polOrg, BusinessPol: "*", NodeOrg: polOrg}
        i18n.GetMessagePrinter().Printf("Adding Business policy org %s' to agbot '%s' ...", polOrg, agbot)
	i18n.GetMessagePrinter().Println()	
	httpCode := cliutils.ExchangePutPost("Exchange", http.MethodPost, cliutils.GetExchangeUrl(), "orgs/"+org+"/agbots/"+agbot+"/businesspols", cliutils.OrgAndCreds(org, userPw), []int{201, 409}, input)
	if httpCode == 409 {
		i18n.GetMessagePrinter().Printf("Business policy org %s' already exists in agbot '%s'", polOrg, agbot)
		i18n.GetMessagePrinter().Println()
		os.Exit(cliutils.CLI_INPUT_ERROR)
	}
}

// Remove the business policy from the agot supporting list. Currently
// only supporting removing all the policies from a organization.
func AgbotRemoveBusinessPolicy(org, userPw, agbot, PolOrg string) {
	cliutils.SetWhetherUsingApiKey(userPw)
	org, agbot = cliutils.TrimOrg(org, agbot)
	polId := formServicedObjectId(PolOrg, "*", PolOrg)
	i18n.GetMessagePrinter().Printf("Removing Business policy org %s' from agbot '%s' ...", PolOrg, agbot)
	i18n.GetMessagePrinter().Println()	
	cliutils.ExchangeDelete("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/agbots/"+agbot+"/businesspols/"+polId, cliutils.OrgAndCreds(org, userPw), []int{204})
}
