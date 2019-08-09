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
	var credOrg string
	credOrg, credToUse = cliutils.TrimOrg(org, credToUse)

	var polOrg string
	polOrg, policy = cliutils.TrimOrg(credOrg, policy)

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
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn exchange business listpolicy' output: %v", err))
		}
		fmt.Println(string(jsonBytes))
	} else {
		buf := new(bytes.Buffer)
		enc := json.NewEncoder(buf)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", cliutils.JSON_INDENT)
		err := enc.Encode(policyList.BusinessPolicy)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn exchange business listpolicy' output: %v", err))
		}
		fmt.Println(string(buf.String()))
	}
}

//BusinessAddPolicy will add a new policy or overwrite an existing policy byt he same name in the Horizon Exchange
func BusinessAddPolicy(org string, credToUse string, policy string, jsonFilePath string) {
	cliutils.SetWhetherUsingApiKey(credToUse)
	org, credToUse = cliutils.TrimOrg(org, credToUse)
	org, policy = cliutils.TrimOrg(org, policy)

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
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Incorrect business policy format in file %s: %v", jsonFilePath, err))
	}

	//add/overwrite business policy file
	httpCode := cliutils.ExchangePutPost("Exchange", http.MethodPost, cliutils.GetExchangeUrl(), "orgs/"+org+"/business/policies"+cliutils.AddSlash(policy), cliutils.OrgAndCreds(org, credToUse), []int{201, 403}, policyFile)
	if httpCode == 403 {
		cliutils.ExchangePutPost("Exchange", http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+org+"/business/policies"+cliutils.AddSlash(policy), cliutils.OrgAndCreds(org, credToUse), []int{201, 404}, policyFile)
		msgPrinter.Printf("Business policy: %v/%v updated in the Horizon Exchange", org, policy)
		msgPrinter.Println()
	} else {
		msgPrinter.Printf("Business policy: %v/%v added in the Horizon Exchange", org, policy)
		msgPrinter.Println()
	}
}

//BusinessUpdatePolicy will replace a single attribute of a business policy in the Horizon Exchange
func BusinessUpdatePolicy(org string, credToUse string, policyName string, filePath string) {
	cliutils.SetWhetherUsingApiKey(credToUse)
	org, credToUse = cliutils.TrimOrg(org, credToUse)
	org, policyName = cliutils.TrimOrg(org, policyName)

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	//Read in the file
	attribute := cliconfig.ReadJsonFileWithLocalConfig(filePath)

	//verify that the policy exists
	var exchangePolicy exchange.GetBusinessPolicyResponse
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/business/policies"+cliutils.AddSlash(policyName), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &exchangePolicy)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("Policy %s not found in org %s", policyName, org))
	}

	findPatchType := make(map[string]interface{})

	json.Unmarshal([]byte(attribute), &findPatchType)

	if _, ok := findPatchType["service"]; ok {
		patch := make(map[string]businesspolicy.ServiceRef)
		err := json.Unmarshal([]byte(attribute), &patch)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal attribute input %s: %v", attribute, err))
		}
		cliutils.ExchangePutPost("Exchange", http.MethodPatch, cliutils.GetExchangeUrl(), "orgs/"+org+"/business/policies"+cliutils.AddSlash(policyName), cliutils.OrgAndCreds(org, credToUse), []int{201}, patch)
		msgPrinter.Printf("Policy %v/%v updated in the Horizon Exchange", org, policyName)
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
		cliutils.ExchangePutPost("Exchange", http.MethodPatch, cliutils.GetExchangeUrl(), "orgs/"+org+"/business/policies"+cliutils.AddSlash(policyName), cliutils.OrgAndCreds(org, credToUse), []int{201}, patch)
		msgPrinter.Printf("Policy %v/%v updated in the Horizon Exchange", org, policyName)
		msgPrinter.Println()
	} else if _, ok := findPatchType["constraints"]; ok {
		var newValue externalpolicy.ConstraintExpression
		patch := make(map[string]externalpolicy.ConstraintExpression)
		err := json.Unmarshal([]byte(attribute), &patch)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal attribute input %s: %v", attribute, err))
		}
		err = newValue.Validate()
		if err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Invalid format for constraints: %v", err))
		}
		newValue = patch["constraints"]
		cliutils.ExchangePutPost("Exchange", http.MethodPatch, cliutils.GetExchangeUrl(), "orgs/"+org+"/business/policies"+cliutils.AddSlash(policyName), cliutils.OrgAndCreds(org, credToUse), []int{201}, patch)
		msgPrinter.Printf("Policy %v/%v updated in the Horizon Exchange", org, policyName)
		msgPrinter.Println()
	} else if _, ok := findPatchType["userInput"]; ok {
		patch := make(map[string][]policy.UserInput)
		err := json.Unmarshal([]byte(attribute), &patch)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal attribute input %s: %v", attribute, err))
		}
		cliutils.ExchangePutPost("Exchange", http.MethodPatch, cliutils.GetExchangeUrl(), "orgs/"+org+"/business/policies"+cliutils.AddSlash(policyName), cliutils.OrgAndCreds(org, credToUse), []int{201}, patch)
		msgPrinter.Printf("Policy %v/%v updated in the Horizon Exchange", org, policyName)
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
			cliutils.ExchangePutPost("Exchange", http.MethodPatch, cliutils.GetExchangeUrl(), "orgs/"+org+"/business/policies"+cliutils.AddSlash(policyName), cliutils.OrgAndCreds(org, credToUse), []int{201}, patch)
			msgPrinter.Printf("Policy %v/%v updated in the Horizon Exchange", org, policyName)
			msgPrinter.Println()
		} else {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Business policy attribute to be updated is not found in the input file. Supported attributes are: label, description, service, properties, constraints, and userInput."))
		}
	}
}

//BusinessRemovePolicy will remove an existing business policy in the Horizon Exchange
func BusinessRemovePolicy(org string, credToUse string, policy string, force bool) {
	cliutils.SetWhetherUsingApiKey(credToUse)
	org, credToUse = cliutils.TrimOrg(org, credToUse)

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if !force {
		cliutils.ConfirmRemove(msgPrinter.Sprintf("Are you sure you want to remove business policy %v for org %v from the Horizon Exchange?", policy, org))
	}

	//check if policy name is passed in as <org>/<service>
	org, policy = cliutils.TrimOrg(org, policy)

	//remove policy
	httpCode := cliutils.ExchangeDelete("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/business/policies"+cliutils.AddSlash(policy), cliutils.OrgAndCreds(org, credToUse), []int{204, 404})
	if httpCode == 404 {
		msgPrinter.Printf("Policy %v/%v not found in the Horizon Exchange", org, policy)
		msgPrinter.Println()
	} else {
		msgPrinter.Printf("Business policy %v/%v removed", org, policy)
		msgPrinter.Println()
	}
}

// Display an empty business policy template as an object.
func BusinessNewPolicy() {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	var business_policy_template = []string{
		`{`,
		`  "label": "",       /* ` + msgPrinter.Sprintf("Business policy label.") + ` */`,
		`  "description": "", /* ` + msgPrinter.Sprintf("Business policy description.") + ` */`,
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
		`  "properties": [   /* ` + msgPrinter.Sprintf("A list of policy properties that describe the service being dployed.") + ` */`,
		`    {`,
		`       "name": "",`,
		`       "value": nil`,
		`      }`,
		`  ],`,
		`  "constraints": [  /* ` + msgPrinter.Sprintf("A list of constraint expressions of the form <property name> <operator> <property value>,") + ` */`,
		`                    /* ` + msgPrinter.Sprintf("separated by boolean operators AND (&&) or OR (||).") + `*/`,
		`       "" `,
		`  ], `,
		`  "userInput": [    /* ` + msgPrinter.Sprintf("A list of userInput variables to set when the service runs, listed by service.") + ` */`,
		`    {            `,
		`      "serviceOrgid": "",         /* ` + msgPrinter.Sprintf("The org of the service.") + ` */`,
		`      "serviceUrl": "",           /* ` + msgPrinter.Sprintf("The name of the service.") + ` */`,
		`      "serviceVersionRange": "",  /* ` + msgPrinter.Sprintf("The service version range to which these variables should be applied.") + ` */`,
		`      "inputs": [                 /* ` + msgPrinter.Sprintf("The input variables to be set.") + `*/`,
		`        {`,
		`          "name": "",`,
		`          "value": nil`,
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
