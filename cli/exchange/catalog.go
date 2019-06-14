package exchange

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
)

const orgType = "IBM"

type CatalogServiceWithMediumInfo struct {
	Description   string `json:"description"`
	Documentation string `json:"documentation"`
}

type CatalogPatternWithMediumInfo struct {
	Description string `json:"description"`
}

// List the public service resources for orgType:IBM.
// The userPw can be the userId:password auth or the nodeId:token auth.
func CatalogServiceList(credOrg string, userPw string, displayShort bool, displayLong bool) {
	cliutils.SetWhetherUsingApiKey(userPw)
	if displayShort && displayLong {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "Flags -s and -l are mutually exclusive.")
	}

	var resp GetServicesResponse
	cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "catalog/services?orgtype="+orgType, cliutils.OrgAndCreds(credOrg, userPw), []int{200}, &resp)

	if displayLong {
		jsonBytes, err := json.MarshalIndent(resp.Services, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn exchange catalog servicelist -l' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	} else if displayShort {
		serviceNames := []string{}
		for k := range resp.Services {
			serviceNames = append(serviceNames, k)
		}
		jsonBytes, err := json.MarshalIndent(serviceNames, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn exchange catalog servicelist -s' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		// display medium information about public services
		var servicesMedium = make(map[string]CatalogServiceWithMediumInfo)
		for k, v := range resp.Services {
			catalogServiceMedium := CatalogServiceWithMediumInfo{
				Description:   v.Description,
				Documentation: v.Documentation,
			}
			servicesMedium[k] = catalogServiceMedium
		}
		jsonBytes, err := json.MarshalIndent(servicesMedium, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn exchange catalog servicelist' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)

	}

}

// List the public pattern resources for orgType:IBM.
func CatalogPatternList(credOrg string, userPw string, displayShort bool, displayLong bool) {
	cliutils.SetWhetherUsingApiKey(userPw)
	if displayShort && displayLong {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "Flags -s and -l are mutually exclusive.")
	}

	var resp ExchangePatterns
	cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "catalog/patterns?orgtype="+orgType, cliutils.OrgAndCreds(credOrg, userPw), []int{200}, &resp)

	if displayLong {
		jsonBytes, err := json.MarshalIndent(resp.Patterns, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn exchange catalog patternlist -l' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	} else if displayShort {
		patternNames := []string{}
		for k := range resp.Patterns {
			patternNames = append(patternNames, k)
		}
		jsonBytes, err := json.MarshalIndent(patternNames, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn exchange catalog patternlist -s' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		// display medium information about public patterns
		var patternsMedium = make(map[string]CatalogPatternWithMediumInfo)
		for k, v := range resp.Patterns {
			catalogPatternMedium := CatalogPatternWithMediumInfo{
				Description: v.Description,
			}
			patternsMedium[k] = catalogPatternMedium
		}
		jsonBytes, err := json.MarshalIndent(patternsMedium, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn exchange catalog patternlist' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)

	}
}
