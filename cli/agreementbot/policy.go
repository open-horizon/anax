package agreementbot

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/policy"
	"os"
)

// get the policy names that the agbot hosts
func getPolicyNames(org string) (map[string][]string, int) {
	// set env to call agbot url
	os.Setenv("HORIZON_URL", cliutils.AGBOT_HZN_API)

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
	os.Setenv("HORIZON_URL", cliutils.AGBOT_HZN_API)

	// Get horizon api policy output
	var apiOutput policy.Policy
	httpCode, _ := cliutils.HorizonGet(fmt.Sprintf("policy/%v/%v", org, name), []int{200, 400}, &apiOutput, false)

	return &apiOutput, httpCode
}

func PolicyList(org string, name string) {
	if name == "" {
		policies, httpCode := getPolicyNames(org)
		if httpCode == 200 {
			jsonBytes, err := json.MarshalIndent(policies, "", cliutils.JSON_INDENT)
			if err != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'policy list' output: %v", err)
			}
			fmt.Printf("%s\n", jsonBytes)
		} else if httpCode == 400 {
			fmt.Printf("Error: The organization '%v' does not exist.\n", org)
		}
	} else {
		pol, httpCode := getPolicy(org, name)
		if httpCode == 200 {
			jsonBytes, err := json.MarshalIndent(pol, "", cliutils.JSON_INDENT)
			if err != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'policy list' output: %v", err)
			}
			fmt.Printf("%s\n", jsonBytes)
		} else if httpCode == 400 {
			fmt.Printf("Error: Either the organization '%v' does not exist or the policy '%v' is not hosted by this agbot.\n", org, name)
		}
	}
}
