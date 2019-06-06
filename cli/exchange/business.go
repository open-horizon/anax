package exchange

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/businesspolicy"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	"net/http"
)

//BusinessListPolicy lists all the policies in the org or only the specified policy if one is given
func BusinessListPolicy(org string, credToUse string, policy string) {
	cliutils.SetWhetherUsingApiKey(credToUse)
	var credOrg string
	credOrg, credToUse = cliutils.TrimOrg(org, credToUse)

	var polOrg string
	polOrg, policy = cliutils.TrimOrg(credOrg, policy)

	if policy == "*" {
		policy = ""
	}
	//get policy list from Horizon Exchange
	var policyList exchange.GetBusinessPolicyResponse
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+polOrg+"/business/policies"+cliutils.AddSlash(policy), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &policyList)
	if httpCode == 404 && policy != "" {
		cliutils.Fatal(cliutils.NOT_FOUND, "Policy %s not found in org %s", policy, polOrg)
	} else if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, "Business policy for organization %s not found", polOrg)
	}

	jsonBytes, err := json.MarshalIndent(policyList.BusinessPolicy, "", cliutils.JSON_INDENT)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn exchange business listpolicy' output: %v", err)
	}
	fmt.Println(string(jsonBytes))
}

//BusinessAddPolicy will add a new policy or overwrite an existing policy byt he same name in the Horizon Exchange
func BusinessAddPolicy(org string, credToUse string, policy string, jsonFilePath string) {
	cliutils.SetWhetherUsingApiKey(credToUse)
	org, credToUse = cliutils.TrimOrg(org, credToUse)

	//read in the new business policy from file
	newBytes := cliconfig.ReadJsonFileWithLocalConfig(jsonFilePath)
	var policyFile businesspolicy.BusinessPolicy
	err := json.Unmarshal(newBytes, &policyFile)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to unmarshal json input file %s: %v", jsonFilePath, err)
	}

	//validate the format of the business policy
	err = policyFile.Validate()
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "Incorrect business policy format in file %s: %v", jsonFilePath, err)
	}

	//add/overwrite business policy file
	httpCode := cliutils.ExchangePutPost("Exchange", http.MethodPost, cliutils.GetExchangeUrl(), "orgs/"+org+"/business/policies"+cliutils.AddSlash(policy), cliutils.OrgAndCreds(org, credToUse), []int{201, 403}, policyFile)
	if httpCode == 403 {
		cliutils.ExchangePutPost("Exchange", http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+org+"/business/policies"+cliutils.AddSlash(policy), cliutils.OrgAndCreds(org, credToUse), []int{201, 404}, policyFile)
		fmt.Println("Business policy: " + org + "/" + policy + " updated in the Horizon Exchange")
	} else {
		fmt.Println("Business policy: " + org + "/" + policy + " added in the Horizon Exchange")
	}
}

//BusinessUpdatePolicy will replace a single attribute of a business policy in the Horizon Exchange
func BusinessUpdatePolicy(org string, credToUse string, policy string, attribute string, valueFilePath string) {
	cliutils.SetWhetherUsingApiKey(credToUse)
	org, credToUse = cliutils.TrimOrg(org, credToUse)

	//verify that the policy exists
	var exchangePolicy exchange.GetBusinessPolicyResponse
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/business/policies"+cliutils.AddSlash(policy), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &exchangePolicy)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, "Policy %s not found in org %s", policy, org)
	}

	//Read in patch and send to the exchange if format is correct
	newBytes := cliconfig.ReadJsonFileWithLocalConfig(valueFilePath)
	switch attribute {
	case "service":
		var newValue businesspolicy.ServiceRef
		patch := make(map[string]businesspolicy.ServiceRef)
		err := json.Unmarshal(newBytes, &newValue)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to unmarshal json input file %s: %v", valueFilePath, err)
		}
		patch[attribute] = newValue
		cliutils.ExchangePutPost("Exchange", http.MethodPatch, cliutils.GetExchangeUrl(), "orgs/"+org+"/business/policies"+cliutils.AddSlash(policy), cliutils.OrgAndCreds(org, credToUse), []int{201}, patch)
		fmt.Println("Policy " + org + "/" + policy + " updated in the Horizon Exchange")
	case "properties":
		var newValue externalpolicy.PropertyList
		patch := make(map[string]externalpolicy.PropertyList)
		err := json.Unmarshal(newBytes, &newValue)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to unmarshal json input file %s: %v", valueFilePath, err)
		}
		err = newValue.Validate()
		if err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "Invalid format for properties")
		}
		patch[attribute] = newValue
		cliutils.ExchangePutPost("Exchange", http.MethodPatch, cliutils.GetExchangeUrl(), "orgs/"+org+"/business/policies"+cliutils.AddSlash(policy), cliutils.OrgAndCreds(org, credToUse), []int{201}, patch)
		fmt.Println("Policy " + org + "/" + policy + " updated in the Horizon Exchange")
	case "constraints":
		var newValue externalpolicy.ConstraintExpression
		patch := make(map[string]externalpolicy.ConstraintExpression)
		err := json.Unmarshal(newBytes, &newValue)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to unmarshal json input file %s: %v", valueFilePath, err)
		}
		err = newValue.Validate()
		if err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "Invalid format for properties")
		}
		patch[attribute] = newValue
		cliutils.ExchangePutPost("Exchange", http.MethodPatch, cliutils.GetExchangeUrl(), "orgs/"+org+"/business/policies"+cliutils.AddSlash(policy), cliutils.OrgAndCreds(org, credToUse), []int{201}, patch)
		fmt.Println("Policy " + org + "/" + policy + " updated in the Horizon Exchange")
	case "label", "description":
		var newValue string
		patch := make(map[string]string)
		err := json.Unmarshal(newBytes, &newValue)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to unmarshal json input file %s: %v", valueFilePath, err)
		}
		patch[attribute] = newValue
		cliutils.ExchangePutPost("Exchange", http.MethodPatch, cliutils.GetExchangeUrl(), "orgs/"+org+"/business/policies"+cliutils.AddSlash(policy), cliutils.OrgAndCreds(org, credToUse), []int{201}, patch)
		fmt.Println("Policy " + org + "/" + policy + " updated in the Horizon Exchange")
	default:
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "Business policy attribute not specified. Attributes are: label, description, service, properties, and constraints")
	}
}

//BusinessRemovePolicy will remove an existing business policy in the Horizon Exchange
func BusinessRemovePolicy(org string, credToUse string, policy string, force bool) {
	cliutils.SetWhetherUsingApiKey(credToUse)
	org, credToUse = cliutils.TrimOrg(org, credToUse)
	if !force {
		cliutils.ConfirmRemove("Are you sure you want to remove business policy " + policy + " for org " + org + " from the Horizon Exchange?")
	}

	//remove policy
	httpCode := cliutils.ExchangeDelete("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/business/policies"+cliutils.AddSlash(policy), cliutils.OrgAndCreds(org, credToUse), []int{204, 404})
	if httpCode == 404 {
		fmt.Println("Policy " + org + "/" + policy + " not found in the Horizon Exchange")
	} else {
		fmt.Println("Business policy " + org + "/" + policy + " removed")
	}
}
