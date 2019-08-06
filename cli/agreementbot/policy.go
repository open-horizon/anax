package agreementbot

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/policy"
	"os"
)

// get the policy names that the agbot hosts
func getPolicyNames(org string) (map[string][]string, int) {
	// set env to call agbot url
	if err := os.Setenv("HORIZON_URL", cliutils.GetAgbotUrlBase()); err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, i18n.GetMessagePrinter().Sprintf("unable to set env var 'HORIZON_URL', error %v", err))
	}

	// Get horizon api policy output
	apiOutput := make(map[string][]string, 0)
	httpCode := 200

	if org != "" {
		// get the policy names for the given org
		httpCode, _ = cliutils.HorizonGet(fmt.Sprintf("policy/%v", org), []int{200, 400}, &apiOutput, false)
	} else {
		// get all the policy names
		httpCode, _ = cliutils.HorizonGet("policy", []int{200}, &apiOutput, false)
	}

	return apiOutput, httpCode
}

// get the policy with the given name for the given org
func getPolicy(org string, name string) (*policy.Policy, int) {
	// set env to call agbot url
	if err := os.Setenv("HORIZON_URL", cliutils.GetAgbotUrlBase()); err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, i18n.GetMessagePrinter().Sprintf("unable to set env var 'HORIZON_URL', error %v", err))
	}

	// Get horizon api policy output
	var apiOutput policy.Policy
	httpCode, _ := cliutils.HorizonGet(fmt.Sprintf("policy/%v/%v", org, name), []int{200, 400}, &apiOutput, false)

	return &apiOutput, httpCode
}

func PolicyList(org string, name string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if name == "" {
		policies, httpCode := getPolicyNames(org)
		if httpCode == 200 {
			jsonBytes, err := json.MarshalIndent(policies, "", cliutils.JSON_INDENT)
			if err != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'policy list' output: %v", err))
			}
			fmt.Printf("%s\n", jsonBytes)
		} else if httpCode == 400 {
			msgPrinter.Printf("Error: The organization '%v' does not exist.", org)
			msgPrinter.Println()
		}
	} else {
		pol, httpCode := getPolicy(org, name)
		if httpCode == 200 {
			jsonBytes, err := json.MarshalIndent(pol, "", cliutils.JSON_INDENT)
			if err != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'policy list' output: %v", err))
			}
			fmt.Printf("%s\n", jsonBytes)
		} else if httpCode == 400 {
			msgPrinter.Printf("Error: Either the organization '%v' does not exist or the policy '%v' is not hosted by this agbot.", org, name)
			msgPrinter.Println()
		}
	}
}
