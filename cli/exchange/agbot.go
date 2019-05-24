package exchange

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
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
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'exchange agbot list' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		// Display the full resources
		var agbots ExchangeAgbots
		httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/agbots"+cliutils.AddSlash(agbot), cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &agbots)
		if httpCode == 404 && agbot != "" {
			cliutils.Fatal(cliutils.NOT_FOUND, "agbot '%s' not found in org %s", agbot, org)
		}
		output := cliutils.MarshalIndent(agbots.Agbots, "exchange agbots list")
		fmt.Println(output)
	}
}

func formPatternId(patternOrg, pattern, nodeOrg string) string {
	return patternOrg + "_" + pattern + "_" + nodeOrg
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
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "both patternorg and pattern must be specified (or neither)")
		}
		if nodeOrg == "" {
			nodeOrg = patternOrg
		}
		patternId = formPatternId(patternOrg, pattern, nodeOrg)
	}
	// Display the full resources
	var patterns ExchangeAgbotPatterns
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/agbots/"+agbot+"/patterns"+cliutils.AddSlash(patternId), cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &patterns)
	if httpCode == 404 && patternOrg != "" && pattern != "" {
		cliutils.Fatal(cliutils.NOT_FOUND, "pattern '%s' with org '%s' and node org '%s' not found in agbot '%s'", pattern, patternOrg, nodeOrg, agbot)
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
	//patternId := formPatternId(patternOrg, pattern, nodeOrg)
	input := ServedPattern{PatternOrg: patternOrg, Pattern: pattern, NodeOrg: nodeOrg}
	httpCode := cliutils.ExchangePutPost("Exchange", http.MethodPost, cliutils.GetExchangeUrl(), "orgs/"+org+"/agbots/"+agbot+"/patterns", cliutils.OrgAndCreds(org, userPw), []int{201, 409}, input)
	if httpCode == 409 {
		fmt.Printf("Pattern '%s' with org '%s' and node org '%s' already exists in agbot '%s'\n", pattern, patternOrg, nodeOrg, agbot)
		os.Exit(cliutils.CLI_INPUT_ERROR)
	}
}

func AgbotRemovePattern(org, userPw, agbot, patternOrg, pattern, nodeOrg string) {
	cliutils.SetWhetherUsingApiKey(userPw)
	org, agbot = cliutils.TrimOrg(org, agbot)
	if nodeOrg == "" {
		nodeOrg = patternOrg
	}
	patternId := formPatternId(patternOrg, pattern, nodeOrg)
	cliutils.ExchangeDelete("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/agbots/"+agbot+"/patterns/"+patternId, cliutils.OrgAndCreds(org, userPw), []int{204})
}
