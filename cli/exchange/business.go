package exchange

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/businesspolicy"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/policy"
	"net/http"
)

//BusinessListPolicy lists all the policies in the org or only the specified policy if one is given
func BusinessListPolicy(org string, credToUse string, policy string, namesOnly bool) {
	cliutils.SetWhetherUsingApiKey(credToUse)

	var polOrg string
	polOrg, policy = cliutils.TrimOrg(org, policy)

	if policy == "*" {
		policy = ""
	}

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	//get policy list from Horizon Exchange
	var policyList exchange.GetBusinessPolicyResponse
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+polOrg+"/business/policies"+cliutils.AddSlash(policy), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &policyList)
	if httpCode == 404 && policy != "" {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("Policy %s not found in org %s", policy, polOrg))
	} else if httpCode == 404 {
		policyNameList := []string{}
		fmt.Println(policyNameList)
	} else if namesOnly && policy == "" {
		policyNameList := []string{}
		for bPolicy := range policyList.BusinessPolicy {
			policyNameList = append(policyNameList, bPolicy)
		}
		jsonBytes, err := json.MarshalIndent(policyNameList, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn exchange deployment listpolicy' output: %v", err))
		}
		fmt.Println(string(jsonBytes))
	} else {
		buf := new(bytes.Buffer)
		enc := json.NewEncoder(buf)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", cliutils.JSON_INDENT)
		err := enc.Encode(policyList.BusinessPolicy)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn exchange deployment listpolicy' output: %v", err))
		}
		fmt.Println(string(buf.String()))
	}
}

//BusinessAddPolicy will add a new policy or overwrite an existing policy byt he same name in the Horizon Exchange
func BusinessAddPolicy(org string, credToUse string, policy string, jsonFilePath string, noConstraints bool) {

	//check for ExchangeUrl early on
	var exchUrl = cliutils.GetExchangeUrl()

	cliutils.SetWhetherUsingApiKey(credToUse)
	var polOrg string
	polOrg, policy = cliutils.TrimOrg(org, policy)

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	//read in the new business policy from file
	newBytes := cliconfig.ReadJsonFileWithLocalConfig(jsonFilePath)
	var policyFile businesspolicy.BusinessPolicy
	err := json.Unmarshal(newBytes, &policyFile)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal json input file %s: %v", jsonFilePath, err))
	}

	//validate the format of the business policy
	err = policyFile.Validate()
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Incorrect deployment policy format in file %s: %v", jsonFilePath, err))
	}

	// if the --no-constraints flag is not specified and the given policy has no constraints, alert the user.
	if (!noConstraints) && policyFile.HasNoConstraints() {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("The deployment policy has no constraints which might result in the service being deployed to all nodes. Please specify --no-constraints to confirm that this is acceptable."))
	}

	//add/overwrite business policy file
	httpCode := cliutils.ExchangePutPost("Exchange", http.MethodPost, exchUrl, "orgs/"+polOrg+"/business/policies"+cliutils.AddSlash(policy), cliutils.OrgAndCreds(org, credToUse), []int{201, 403}, policyFile, nil)
	if httpCode == 403 {
		cliutils.ExchangePutPost("Exchange", http.MethodPut, exchUrl, "orgs/"+polOrg+"/business/policies"+cliutils.AddSlash(policy), cliutils.OrgAndCreds(org, credToUse), []int{201, 404}, policyFile, nil)
		msgPrinter.Printf("Deployment policy: %v/%v updated in the Horizon Exchange", polOrg, policy)
		msgPrinter.Println()
	} else {
		msgPrinter.Printf("Deployment policy: %v/%v added in the Horizon Exchange", polOrg, policy)
		msgPrinter.Println()
	}
}

//BusinessUpdatePolicy will replace a single attribute of a business policy in the Horizon Exchange
func BusinessUpdatePolicy(org string, credToUse string, policyName string, filePath string) {

	//check for ExchangeUrl early on
	var exchUrl = cliutils.GetExchangeUrl()

	cliutils.SetWhetherUsingApiKey(credToUse)
	var polOrg string
	polOrg, policyName = cliutils.TrimOrg(org, policyName)

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	//Read in the file
	attribute := cliconfig.ReadJsonFileWithLocalConfig(filePath)

	//verify that the policy exists
	var exchangePolicy exchange.GetBusinessPolicyResponse
	httpCode := cliutils.ExchangeGet("Exchange", exchUrl, "orgs/"+polOrg+"/business/policies"+cliutils.AddSlash(policyName), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &exchangePolicy)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("Policy %s not found in org %s", policyName, polOrg))
	}

	findPatchType := make(map[string]interface{})

	json.Unmarshal([]byte(attribute), &findPatchType)

	if _, ok := findPatchType["service"]; ok {
		patch := make(map[string]businesspolicy.ServiceRef)
		err := json.Unmarshal([]byte(attribute), &patch)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal attribute input %s: %v", attribute, err))
		}
		msgPrinter.Printf("Updating Policy %v/%v in the Horizon Exchange and re-evaluating all agreements based on this deployment policy. Existing agreements might be cancelled and re-negotiated.", polOrg, policyName)
		msgPrinter.Println()
		cliutils.ExchangePutPost("Exchange", http.MethodPatch, exchUrl, "orgs/"+polOrg+"/business/policies"+cliutils.AddSlash(policyName), cliutils.OrgAndCreds(org, credToUse), []int{201}, patch, nil)
		msgPrinter.Printf("Policy %v/%v updated in the Horizon Exchange", polOrg, policyName)
		msgPrinter.Println()
	} else if _, ok := findPatchType["properties"]; ok {
		var newValue externalpolicy.PropertyList
		patch := make(map[string]externalpolicy.PropertyList)
		err := json.Unmarshal([]byte(attribute), &patch)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal attribute input %s: %v", attribute, err))
		}
		newValue = patch["properties"]
		err = newValue.Validate()
		if err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Invalid format for properties: %v", err))
		}
		patch["properties"] = newValue
		msgPrinter.Printf("Updating Policy %v/%v in the Horizon Exchange and re-evaluating all agreements based on this deployment policy. Existing agreements might be cancelled and re-negotiated.", polOrg, policyName)
		msgPrinter.Println()
		cliutils.ExchangePutPost("Exchange", http.MethodPatch, exchUrl, "orgs/"+polOrg+"/business/policies"+cliutils.AddSlash(policyName), cliutils.OrgAndCreds(org, credToUse), []int{201}, patch, nil)
		msgPrinter.Printf("Policy %v/%v updated in the Horizon Exchange", polOrg, policyName)
		msgPrinter.Println()
	} else if _, ok := findPatchType["constraints"]; ok {
		var newValue externalpolicy.ConstraintExpression
		patch := make(map[string]externalpolicy.ConstraintExpression)
		err := json.Unmarshal([]byte(attribute), &patch)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal attribute input %s: %v", attribute, err))
		}
		_, err = newValue.Validate()
		if err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Invalid format for constraints: %v", err))
		}
		newValue = patch["constraints"]
		msgPrinter.Printf("Updating Policy %v/%v in the Horizon Exchange and re-evaluating all agreements based on this deployment policy. Existing agreements might be cancelled and re-negotiated.", polOrg, policyName)
		msgPrinter.Println()
		cliutils.ExchangePutPost("Exchange", http.MethodPatch, exchUrl, "orgs/"+polOrg+"/business/policies"+cliutils.AddSlash(policyName), cliutils.OrgAndCreds(org, credToUse), []int{201}, patch, nil)
		msgPrinter.Printf("Policy %v/%v updated in the Horizon Exchange", polOrg, policyName)
		msgPrinter.Println()
	} else if _, ok := findPatchType["userInput"]; ok {
		patch := make(map[string][]policy.UserInput)
		err := json.Unmarshal([]byte(attribute), &patch)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal attribute input %s: %v", attribute, err))
		}
		cliutils.ExchangePutPost("Exchange", http.MethodPatch, exchUrl, "orgs/"+polOrg+"/business/policies"+cliutils.AddSlash(policyName), cliutils.OrgAndCreds(org, credToUse), []int{201}, patch, nil)
		msgPrinter.Printf("Updating Policy %v/%v in the Horizon Exchange and re-evaluating all agreements based on this deployment policy. Existing agreements might be cancelled and re-negotiated.", polOrg, policyName)
		msgPrinter.Println()
		msgPrinter.Printf("Policy %v/%v updated in the Horizon Exchange", polOrg, policyName)
		msgPrinter.Println()
	} else {
		_, ok := findPatchType["label"]
		_, ok2 := findPatchType["description"]
		if ok || ok2 {
			patch := make(map[string]string)
			err := json.Unmarshal([]byte(attribute), &patch)
			if err != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal attribute input %s: %v", attribute, err))
			}
			msgPrinter.Printf("Updating Policy %v/%v in the Horizon Exchange and re-evaluating all agreements based on this deployment policy. Existing agreements might be cancelled and re-negotiated.", polOrg, policyName)
			msgPrinter.Println()
			cliutils.ExchangePutPost("Exchange", http.MethodPatch, exchUrl, "orgs/"+polOrg+"/business/policies"+cliutils.AddSlash(policyName), cliutils.OrgAndCreds(org, credToUse), []int{201}, patch, nil)
			msgPrinter.Printf("Policy %v/%v updated in the Horizon Exchange", polOrg, policyName)
			msgPrinter.Println()
		} else {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Deployment policy attribute to be updated is not found in the input file. Supported attributes are: label, description, service, properties, constraints, and userInput."))
		}
	}
}

//BusinessRemovePolicy will remove an existing business policy in the Horizon Exchange
func BusinessRemovePolicy(org string, credToUse string, policy string, force bool) {
	cliutils.SetWhetherUsingApiKey(credToUse)
	var polOrg string
	polOrg, policy = cliutils.TrimOrg(org, policy)

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if !force {
		cliutils.ConfirmRemove(msgPrinter.Sprintf("Are you sure you want to remove deployment policy %v for org %v from the Horizon Exchange?", policy, polOrg))
	}

	//remove policy
	httpCode := cliutils.ExchangeDelete("Exchange", cliutils.GetExchangeUrl(), "orgs/"+polOrg+"/business/policies"+cliutils.AddSlash(policy), cliutils.OrgAndCreds(org, credToUse), []int{204, 404})
	if httpCode == 404 {
		msgPrinter.Printf("Policy %v/%v not found in the Horizon Exchange", polOrg, policy)
		msgPrinter.Println()
	} else {
		msgPrinter.Printf("Removing deployment policy %v/%v and re-evaluating all agreements. Existing agreements might be cancelled and re-negotiated", polOrg, policy)
		msgPrinter.Println()
		msgPrinter.Printf("Deployment policy %v/%v removed", polOrg, policy)
		msgPrinter.Println()
	}
}

// Display an empty business policy template as an object.
func BusinessNewPolicy() {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	var business_policy_template = []string{
		`{`,
		`  "label": "",       /* ` + msgPrinter.Sprintf("Deployment policy label.") + ` */`,
		`  "description": "", /* ` + msgPrinter.Sprintf("Deployment policy description.") + ` */`,
		`  "service": {       `,
		`    "name": "",      /* ` + msgPrinter.Sprintf("The name of the service.") + ` */`,
		`    "org": "",       /* ` + msgPrinter.Sprintf("The org of the service.") + ` */`,
		`    "arch": "",      /* ` + msgPrinter.Sprintf("Set to '*' to use services of any hardware architecture.") + ` */`,
		`    "serviceVersions": [  /* ` + msgPrinter.Sprintf("A list of service versions.") + ` */`,
		`      {`,
		`        "version": "",`,
		`        "priority":{}`,
		`      }`,
		`    ]`,
		`  },`,
		`  "properties": [   /* ` + msgPrinter.Sprintf("A list of policy properties that describe the service being deployed.") + ` */`,
		`    {`,
		`       "name": "",`,
		`       "value": null`,
		`      }`,
		`  ],`,
		`  "constraints": [  /* ` + msgPrinter.Sprintf("A list of constraint expressions of the form <property name> <operator> <property value>,") + ` */`,
		`                    /* ` + msgPrinter.Sprintf("separated by boolean operators AND (&&) or OR (||).") + `*/`,
		`       "myproperty == myvalue" `,
		`  ], `,
		`  "userInput": [    /* ` + msgPrinter.Sprintf("A list of userInput variables to set when the service runs, listed by service.") + ` */`,
		`    {            `,
		`      "serviceOrgid": "",         /* ` + msgPrinter.Sprintf("The org of the service.") + ` */`,
		`      "serviceUrl": "",           /* ` + msgPrinter.Sprintf("The name of the service.") + ` */`,
		`      "serviceVersionRange": "",  /* ` + msgPrinter.Sprintf("The service version range to which these variables should be applied.") + ` */`,
		`      "inputs": [                 /* ` + msgPrinter.Sprintf("The input variables to be set.") + `*/`,
		`        {`,
		`          "name": "",`,
		`          "value": null`,
		`        }`,
		`      ]`,
		`    }`,
		`  ]`,
		`}`,
	}

	for _, s := range business_policy_template {
		fmt.Println(s)
	}
}
