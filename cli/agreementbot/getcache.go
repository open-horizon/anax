package agreementbot

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/agreementbot"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/i18n"
	"os"
)

// Display served pattern orgs and deployment policy orgs cached by aggrement bot.
func GetServedOrgs() {
	msgPrinter := i18n.GetMessagePrinter()
	// set env to call agbot url
	if err := os.Setenv("HORIZON_URL", cliutils.GetAgbotUrlBase()); err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("unable to set env var 'HORIZON_URL', error %v", err))
	}

	// Get the agbot servedorgs info
	servedOrgsInfo := agreementbot.ServedOrgs{} // the structure we will output
	cliutils.HorizonGet("cache/servedorg", []int{200}, &servedOrgsInfo, false)

	// Output the combined info
	jsonBytes, err := json.MarshalIndent(servedOrgsInfo, "", cliutils.JSON_INDENT)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn node list' output: %v", err))
	}
	fmt.Printf("%s\n", jsonBytes)

}

// Display patterns cached by agreement bot. If no org or org/name is specified, display all. Show detailed info if long
func GetPatterns(org string, name string, long bool) {
	msgPrinter := i18n.GetMessagePrinter()

	// set env to call agbot url
	if err := os.Setenv("HORIZON_URL", cliutils.GetAgbotUrlBase()); err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("unable to set env var 'HORIZON_URL', error %v", err))
	}

	if name != "" && org == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("org must be specified with -o when pattern name is specified."))
	}

	// base url upon which to add arguments and flag
	patUrl := "cache/pattern"
	// if org is specified add it to url
	// if name is specified add it to url and ignore value for long
	if org != "" {
		patUrl = fmt.Sprintf("%v/%v", patUrl, org)
		if name != "" {
			patUrl = fmt.Sprintf("%v/%v", patUrl, name)
			long = false
		}
	}

	if long {
		// Show detailed info if long, else only orgs and names
		patUrl = fmt.Sprintf("%v?long==1", patUrl)
		if org == "" {
			// Get the agbot servedorgs info
			patInfo := map[string]map[string]*agreementbot.PatternEntry{} //the structure we will output

			cliutils.HorizonGet(patUrl, []int{200}, &patInfo, false)

			// Output the combined info
			jsonBytes, err := json.MarshalIndent(patInfo, "", cliutils.JSON_INDENT)
			if err != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal output: %v", err))
			}
			fmt.Printf("%s\n", jsonBytes)

		} else if name == "" {
			// Get the agbot servedorgs info
			patInfo := map[string]*agreementbot.PatternEntry{} //the structure we will output

			if httpCode, _ := cliutils.HorizonGet(patUrl, []int{200, 404}, &patInfo, false); httpCode == 404 {
				msgPrinter.Printf("%v does not exist in the pattern management cache.", org)
				msgPrinter.Println()
			} else {
				// Output the combined info
				jsonBytes, err := json.MarshalIndent(patInfo, "", cliutils.JSON_INDENT)
				if err != nil {
					cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal output: %v", err))
				}
				fmt.Printf("%s\n", jsonBytes)
			}
		}

	} else if org != "" {
		// display detailed info of pattern if name is specified, else only list names
		if name != "" {
			var patInfo *agreementbot.PatternEntry //the structure we will output
			if httpCode, _ := cliutils.HorizonGet(patUrl, []int{200, 404}, &patInfo, false); httpCode == 404 {
				msgPrinter.Printf("%v/%v does not exist in the pattern management cache.", org, name)
				msgPrinter.Println()
			} else {
				// Output the combined info
				jsonBytes, err := json.MarshalIndent(patInfo, "", cliutils.JSON_INDENT)
				if err != nil {
					cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal output: %v", err))
				}
				fmt.Printf("%s\n", jsonBytes)
			}
		} else {

			patInfo := []string{} //the structure we will output
			if httpCode, _ := cliutils.HorizonGet(patUrl, []int{200, 404}, &patInfo, false); httpCode == 404 {
				msgPrinter.Printf("%v does not exist in the pattern management cache.", org)
				msgPrinter.Println()
			} else {
				// Output the combined info
				jsonBytes, err := json.MarshalIndent(patInfo, "", cliutils.JSON_INDENT)
				if err != nil {
					cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal output: %v", err))
				}
				fmt.Printf("%s\n", jsonBytes)
			}
		}
	} else {
		// Get the agbot servedorgs info
		patInfo := map[string][]string{} //the structure we will output
		cliutils.HorizonGet(patUrl, []int{200}, &patInfo, false)

		// Output the combined info
		jsonBytes, err := json.MarshalIndent(patInfo, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal output: %v", err))
		}
		fmt.Printf("%s\n", jsonBytes)
	}

}

// Display deployment policies cached by agreement bot. If no org or org/name is specified, display all. Show detailed info if long
func GetPolicies(org string, name string, long bool) {
	msgPrinter := i18n.GetMessagePrinter()
	// set env to call agbot url
	if err := os.Setenv("HORIZON_URL", cliutils.GetAgbotUrlBase()); err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("unable to set env var 'HORIZON_URL', error %v", err))
	}

	if name != "" && org == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("org must be specified with -o when deployment policy name is specified."))
	}

	// base Url from which to add other flags and arguments
	polUrl := "cache/deploymentpol"

	// if org is specified, add to url
	// if name is specified as well, add to url and ignore value of long
	if org != "" {
		polUrl = fmt.Sprintf("%v/%v", polUrl, org)
		if name != "" {
			polUrl = fmt.Sprintf("%v/%v", polUrl, name)
			long = false
		}
	}

	// Show detailed info if long, else only orgs and names
	if long {
		polUrl = fmt.Sprintf("%v?long==1", polUrl)
		if org == "" {
			// Get the agbot servedorgs info
			polInfo := map[string]map[string]*agreementbot.BusinessPolicyEntry{} //the structure we will output

			cliutils.HorizonGet(polUrl, []int{200}, &polInfo, false)

			// Output the combined info
			jsonBytes, err := json.MarshalIndent(polInfo, "", cliutils.JSON_INDENT)
			if err != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal output: %v", err))
			}
			fmt.Printf("%s\n", jsonBytes)

		} else if name == "" {
			// Get the agbot servedorgs info
			polInfo := map[string]*agreementbot.BusinessPolicyEntry{} //the structure we will output

			if httpCode, _ := cliutils.HorizonGet(polUrl, []int{200, 404}, &polInfo, false); httpCode == 404 {
				msgPrinter.Printf("%v does not exist in the deployment policy management cache.", org)
				msgPrinter.Println()
			} else {
				// Output the combined info
				jsonBytes, err := json.MarshalIndent(polInfo, "", cliutils.JSON_INDENT)
				if err != nil {
					cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal output: %v", err))
				}
				fmt.Printf("%s\n", jsonBytes)
			}
		}

	} else if org != "" {
		// display detailed info of policies if name is specified, else only list names
		if name != "" {
			var polInfo *agreementbot.BusinessPolicyEntry //the structure we will output
			if httpCode, _ := cliutils.HorizonGet(polUrl, []int{200, 404}, &polInfo, false); httpCode == 404 {
				msgPrinter.Printf("%v/%v does not exist in the deployment policy management cache.", org, name)
				msgPrinter.Println()
			} else {
				// Output the combined info
				jsonBytes, err := json.MarshalIndent(polInfo, "", cliutils.JSON_INDENT)
				if err != nil {
					cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal output: %v", err))
				}
				fmt.Printf("%s\n", jsonBytes)
			}
		} else {
			polInfo := []string{} //the structure we will output
			if httpCode, _ := cliutils.HorizonGet(polUrl, []int{200, 404}, &polInfo, false); httpCode == 404 {
				msgPrinter.Printf("%v does not exist in the deployment policy management cache.", org)
				msgPrinter.Println()
			} else {
				// Output the combined info
				jsonBytes, err := json.MarshalIndent(polInfo, "", cliutils.JSON_INDENT)
				if err != nil {
					cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal output: %v", err))
				}
				fmt.Printf("%s\n", jsonBytes)
			}
		}
	} else {
		// Get the agbot servedorgs info
		polInfo := map[string][]string{} //the structure we will output
		cliutils.HorizonGet(polUrl, []int{200}, &polInfo, false)

		// Output the combined info
		jsonBytes, err := json.MarshalIndent(polInfo, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal output: %v", err))
		}
		fmt.Printf("%s\n", jsonBytes)
	}
}
