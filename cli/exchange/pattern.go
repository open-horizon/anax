package exchange

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
)

// We only care about the pattern names, so the rest is left as interface{}
type ExchangePattern struct {
	LastIndex int                    `json:"lastIndex"`
	Patterns  map[string]interface{} `json:"patterns"`
}

func PatternList(org string, nodeIdTok string, pattern string, namesOnly bool) {
	if pattern != "" {
		pattern = "/" + pattern
	}
	if namesOnly {
		// Only display the names
		var resp ExchangePattern
		cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/patterns"+pattern, org+"/"+nodeIdTok, []int{200}, &resp)
		var patterns []string
		for p := range resp.Patterns {
			patterns = append(patterns, p)
		}
		jsonBytes, err := json.MarshalIndent(patterns, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'exchange pattern list' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		// Display the full resources
		var output string
		cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/patterns"+pattern, org+"/"+nodeIdTok, []int{200}, &output)
		fmt.Println(output)
	}
}
