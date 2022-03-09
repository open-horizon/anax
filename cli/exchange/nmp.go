package exchange

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/i18n"
	"net/http"
)

type ExchangeNMPStatus struct {
	LastIndex    int                                                `json:"lastIndex"`
	AgentUpgrade map[string]exchangecommon.AgentUpgradePolicyStatus `json:"agentUpgradePolicyStatus"`
}

func NMPList(org, credToUse, nmpName string, namesOnly, listNodes bool) {

	// If user specifies --nodes flag, return list of applicable nodes for given nmp instead of the nmp itself.
	if listNodes {
		NMPListNodes(org, credToUse, nmpName)
		return
	}

	cliutils.SetWhetherUsingApiKey(credToUse)

	var nmpOrg string
	nmpOrg, nmpName = cliutils.TrimOrg(org, nmpName)

	if nmpName == "*" {
		nmpName = ""
	}

	// Get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Call the exchange to get the NMP's
	var nmpList exchange.ExchangeNodeManagementPolicyResponse
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+nmpOrg+"/managementpolicies"+cliutils.AddSlash(nmpName), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &nmpList)

	if httpCode == 404 {
		// Throw an error if the given NMP name does not exist
		if nmpName != "" {
			cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("NMP %s not found in org %s", nmpName, nmpOrg))
		} else if httpCode == 404 {
			fmt.Println("[]")
		}

		// If --long was not specified and an NMP name was not given, output only the NMP names
	} else if namesOnly && nmpName == "" {
		nmpNameList := []string{}
		for nmp := range nmpList.Policies {
			nmpNameList = append(nmpNameList, nmp)
		}
		jsonBytes, err := json.MarshalIndent(nmpNameList, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn exchange nmp list' output: %v", err))
		}
		fmt.Println(string(jsonBytes))

		// Otherwise, output full NMP details
	} else {
		output := cliutils.MarshalIndent(nmpList.Policies, "exchange nmp list")
		fmt.Println(output)
	}
}

func NMPNew() {
	// Get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Assemple NMP template file
	var nmp_template = []string{
		`{`,
		`  "label": "",                               /* ` + msgPrinter.Sprintf("A short description of the policy.") + ` */`,
		`  "description": "",                         /* ` + msgPrinter.Sprintf("(Optional) A much longer description of the policy.") + ` */`,
		`  "constraints": [                           /* ` + msgPrinter.Sprintf("(Optional) A list of constraint expressions of the form <property name> <operator> <property value>,") + ` */`,
		`    "myproperty == myvalue"                  /* ` + msgPrinter.Sprintf("separated by boolean operators AND (&&) or OR (||).") + `*/`,
		`  ],`,
		`  "properties": [                            /* ` + msgPrinter.Sprintf("(Optional) A list of policy properties that describe this policy.") + ` */`,
		`    {`,
		`      "name": "",`,
		`      "value": null`,
		`    }`,
		`  ],`,
		`  "patterns": [                              /* ` + msgPrinter.Sprintf("(Optional) This policy applies to nodes using one of these patterns.") + ` */`,
		`    ""`,
		`  ],`,
		`  "enabled": false,                          /* ` + msgPrinter.Sprintf("Is this policy enabled or disabled.") + ` */`,
		`  "start": "<RFC3339 timestamp> | now",      /* ` + msgPrinter.Sprintf("When to start an upgrade, default \"now\".") + ` */`,
		`  "startWindow": 0,                          /* ` + msgPrinter.Sprintf("Enable agents to randomize upgrade start time within start + startWindow, default 0.") + ` */`,
		`  "agentUpgradePolicy": {                    /* ` + msgPrinter.Sprintf("(Optional) Assertions on how the agent should update itself.") + ` */`,
		`    "manifest": "",                          /* ` + msgPrinter.Sprintf("The manifest file containing the software, config and cert files to upgrade.") + ` */`,
		`    "allowDowngrade": false                  /* ` + msgPrinter.Sprintf("Is this policy allowed to perform a downgrade to a previous version.") + ` */`,
		`  }`,
		`}`,
	}

	// Output template file to stdout
	for _, s := range nmp_template {
		fmt.Println(s)
	}
}

func NMPAdd(org, credToUse, nmpName, jsonFilePath string, appliesTo, noConstraints bool) {
	// Check for ExchangeUrl early on
	var exchUrl = cliutils.GetExchangeUrl()

	cliutils.SetWhetherUsingApiKey(credToUse)

	var nmpOrg string
	nmpOrg, nmpName = cliutils.TrimOrg(org, nmpName)

	// Get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Read in the new NMP from file
	newBytes := cliconfig.ReadJsonFileWithLocalConfig(jsonFilePath)
	var nmpFile exchangecommon.ExchangeNodeManagementPolicy
	err := json.Unmarshal(newBytes, &nmpFile)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal json input file %s: %v", jsonFilePath, err))
	}

	// Validate the format of the nmp
	err = nmpFile.Validate()
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Incorrect node management policy format in file %s: %v", jsonFilePath, err))
	}

	// If the --no-constraints flag is not specified and the given nmp has no constraints, alert the user.
	if !noConstraints && nmpFile.HasNoConstraints() && nmpFile.HasNoPatterns() {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("The node management policy has no constraints which might result in the management policy being deployed to all nodes. Please specify --no-constraints to confirm that this is acceptable."))
	}

	// Struct to hold exchange PUT/POST response info
	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	}

	// First try to add a new NMP
	httpCode := cliutils.ExchangePutPost("Exchange", http.MethodPost, exchUrl, "orgs/"+nmpOrg+"/managementpolicies"+cliutils.AddSlash(nmpName), cliutils.OrgAndCreds(org, credToUse), []int{201, 403}, nmpFile, &resp)

	// If the NMP already exists, try to update it
	if httpCode == 403 {
		httpCode = cliutils.ExchangePutPost("Exchange", http.MethodPut, exchUrl, "orgs/"+nmpOrg+"/managementpolicies"+cliutils.AddSlash(nmpName), cliutils.OrgAndCreds(org, credToUse), []int{201, 404}, nmpFile, nil)
		if httpCode == 201 {
			msgPrinter.Printf("Node management policy: %v/%v updated in the Horizon Exchange", nmpOrg, nmpName)
			msgPrinter.Println()
		} else if httpCode == 404 {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Cannot create node management policy %v/%v: %v", nmpOrg, nmpName, resp.Msg))
		}

		// If update was successful, output success message
	} else if !cliutils.IsDryRun() {
		msgPrinter.Printf("Node management policy: %v/%v added in the Horizon Exchange", nmpOrg, nmpName)
		msgPrinter.Println()
	}

	// If the user specified the --appliesTo flag, get a list of nodes that are compatible with this NMP and output list
	if appliesTo {
		nodes := determineCompatibleNodes(org, credToUse, nmpName, nmpFile)
		if nodes != nil && len(nodes) > 0 {
			output := cliutils.MarshalIndent(nodes, "exchange nmp add")
			fmt.Printf(output)
		}
		msgPrinter.Println()
	}
}

func NMPRemove(org, credToUse, nmpName string, force bool) {
	cliutils.SetWhetherUsingApiKey(credToUse)

	var nmpOrg string
	nmpOrg, nmpName = cliutils.TrimOrg(org, nmpName)

	// Get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Prompt are you sure message to user if --force was not specified
	if !force {
		cliutils.ConfirmRemove(msgPrinter.Sprintf("Are you sure you want to remove node management policy %v for org %v from the Horizon Exchange?", nmpName, nmpOrg))
	}

	// Remove NMP from the exchange
	httpCode := cliutils.ExchangeDelete("Exchange", cliutils.GetExchangeUrl(), "orgs/"+nmpOrg+"/managementpolicies"+cliutils.AddSlash(nmpName), cliutils.OrgAndCreds(org, credToUse), []int{204, 404})
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("Node management policy %s not found in org %s", nmpName, nmpOrg))
	} else if httpCode == 204 {
		msgPrinter.Printf("Removing node management policy %v/%v and re-evaluating all agreements. Existing agreements might be cancelled and re-negotiated", nmpOrg, nmpName)
		msgPrinter.Println()
		msgPrinter.Printf("Node management policy %v/%v removed", nmpOrg, nmpName)
		msgPrinter.Println()
	}
}

func NMPListNodes(org, credToUse, nmpName string) {
	cliutils.SetWhetherUsingApiKey(credToUse)

	var nmpOrg string
	nmpOrg, nmpName = cliutils.TrimOrg(org, nmpName)

	if nmpName == "*" {
		nmpName = ""
	}

	// Get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Store list of compatible nodes in map indexed by NMP names in the exchange
	compatibleNodeMap := make(map[string][]string)

	// Get a list of all NMP's that the user has access to from the exchange
	var nmpList exchange.ExchangeNodeManagementPolicyResponse
	var output string
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+nmpOrg+"/managementpolicies"+cliutils.AddSlash(nmpName), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &nmpList)
	if httpCode == 404 && nmpName != "" {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("NMP %s not found in org %s", nmpName, nmpOrg))
	} else if httpCode == 404 {
		output = "{}"
	} else {
		// Loop over all the NMP's and determine compatible nodes
		for nmp, nmpPolicy := range nmpList.Policies {
			nodes := determineCompatibleNodes(org, credToUse, nmpName, nmpPolicy)
			compatibleNodeMap[nmp] = nodes
		}
		output = cliutils.MarshalIndent(compatibleNodeMap, "management nmp list --nodes")
	}

	// Output compatibleNodeMap
	fmt.Printf(output)
	msgPrinter.Println()
}

func NMPStatus(org, credToUse, nmpName string) {
	cliutils.SetWhetherUsingApiKey(credToUse)

	var nmpOrg string
	nmpOrg, nmpName = cliutils.TrimOrg(org, nmpName)

	if nmpName == "*" {
		nmpName = ""
	}

	// Get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Get names of nodes user can access from the Exchange
	var resp ExchangeNodes
	cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+nmpOrg+"/nodes", cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &resp)

	// Map to store NMP statuses across all nodes
	allNMPStatuses := make(map[string]exchangecommon.AgentUpgradePolicyStatus, 0)

	// Loop over each node
	for nodeName := range resp.Nodes {

		// Get the list of NMP statuses, or try to find the given nmpName, if applicable
		var nmpStatusList ExchangeNMPStatus
		_, nodeName = cliutils.TrimOrg(org, nodeName)
		httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+nmpOrg+"/nodes/"+nodeName+"/managementStatus"+cliutils.AddSlash(nmpName), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &nmpStatusList)
		if httpCode == 404 {
			continue
		}

		// Add all the found statuses to the map
		for nmpStatusName, nmpStatus := range nmpStatusList.AgentUpgrade {
			allNMPStatuses[nmpStatusName] = nmpStatus
		}
	}

	// Format output and print
	if len(allNMPStatuses) == 0 {
		if nmpName == "" {
			fmt.Println("[]")
		} else {
			cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("Status for NMP %s not found in org %s", nmpName, nmpOrg))
		}

	} else {
		fmt.Println(cliutils.MarshalIndent(allNMPStatuses, "management nmp status"))
	}
}

func determineCompatibleNodes(org, credToUse, nmpName string, nmpPolicy exchangecommon.ExchangeNodeManagementPolicy) []string {
	var nmpOrg string
	nmpOrg, nmpName = cliutils.TrimOrg(org, nmpName)

	// Get node names from the exchange
	var resp ExchangeNodes
	cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+nmpOrg+"/nodes", cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &resp)

	// Loop over all nodes that user has access to in the exchange
	compatibleNodes := []string{}
	for nodeNameEx, node := range resp.Nodes {
		// Skip unregistered nodes
		if node.PublicKey == "" {
			continue
		}
		// If node is registered with pattern, check if given NMP has the same pattern specified
		if node.Pattern != "" {
			if cutil.SliceContains(nmpPolicy.Patterns, node.Pattern) {
				compatibleNodes = append(compatibleNodes, nodeNameEx)
			}

			// If node is registered with policy, check properties and constraints for compatibility
		} else {
			// Get the policy registered with the current node
			var nodePolicy exchange.ExchangeNodePolicy
			_, nodeName := cliutils.TrimOrg(org, nodeNameEx)
			cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+nmpOrg+"/nodes"+cliutils.AddSlash(nodeName)+"/policy", cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &nodePolicy)
			nodeManagementPolicy := nodePolicy.GetManagementPolicy()

			// Compare properties/constraints
			if err := nmpPolicy.Constraints.IsSatisfiedBy(nodeManagementPolicy.Properties); err != nil {
				continue
			} else if err = nodeManagementPolicy.Constraints.IsSatisfiedBy(nmpPolicy.Properties); err != nil {
				continue
			} else {
				compatibleNodes = append(compatibleNodes, nodeNameEx)
			}
		}
	}

	return compatibleNodes
}
