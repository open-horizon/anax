package exchange

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	_ "github.com/open-horizon/anax/externalpolicy/text_language"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/semanticversion"
)

type ExchangeNodes struct {
	LastIndex int                        `json:"lastIndex"`
	Nodes     map[string]exchange.Device `json:"nodes"`
}

func NodeList(org string, credToUse string, node string, namesOnly bool) {
	cliutils.SetWhetherUsingApiKey(credToUse)
	var nodeOrg string
	nodeOrg, node = cliutils.TrimOrg(org, node)
	if node == "*" {
		node = ""
	}
	if namesOnly && node == "" {
		// Only display the names
		var resp ExchangeNodes
		cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+nodeOrg+"/nodes"+cliutils.AddSlash(node), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &resp)
		nodes := []string{}
		for n := range resp.Nodes {
			nodes = append(nodes, n)
		}
		jsonBytes, err := json.MarshalIndent(nodes, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, i18n.GetMessagePrinter().Sprintf("failed to marshal 'exchange node list' output: %v", err))
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		// Display the full resources
		var nodes ExchangeNodes
		httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+nodeOrg+"/nodes"+cliutils.AddSlash(node), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &nodes)
		if httpCode == 404 && node != "" {
			cliutils.Fatal(cliutils.NOT_FOUND, i18n.GetMessagePrinter().Sprintf("node '%s' not found in org %s", node, nodeOrg))
		}
		output := cliutils.MarshalIndent(nodes.Nodes, "exchange node list")
		fmt.Println(output)
	}
}

func NodeCreate(org, nodeIdTok, node, token, userPw, email string, arch string, nodeName string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// They should specify either nodeIdTok (for backward compat) or node and token, but not both
	var nodeId, nodeToken string
	if node != "" || token != "" {
		if node == "" || token == "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("if node or token are specified then they both must be specified"))
		}
		// at this point we know both node and token were specified
		if nodeIdTok != "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("do not specify both the -n flag and the node and token positional arguments. They mean the same thing."))
		}
		nodeId = node
		nodeToken = token
	} else {
		// here we know neither node nor token were specified
		if nodeIdTok == "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("either the node and token positional arguments, or the -n flag must be specified."))
		}
		nodeId, nodeToken = cliutils.SplitIdToken(nodeIdTok)
	}

	if nodeName == "" {
		nodeName = nodeId
	}

	cliutils.SetWhetherUsingApiKey(userPw)
	exchUrlBase := cliutils.GetExchangeUrl()

	// Assume the user exists and try to create the node, but handle the error cases
	putNodeReq := exchange.PutDeviceRequest{Token: nodeToken, Name: nodeName, SoftwareVersions: make(map[string]string), PublicKey: []byte(""), Arch: arch} // we only need to set the token
	httpCode := cliutils.ExchangePutPost("Exchange", http.MethodPut, exchUrlBase, "orgs/"+org+"/nodes/"+nodeId, cliutils.OrgAndCreds(org, userPw), []int{201, 401, 403}, putNodeReq)
	if httpCode == 401 {
		// Invalid creds means the user doesn't exist, or pw is wrong, try to create it if we are in the public org
		user, pw := cliutils.SplitIdToken(userPw)
		if org == "public" && email != "" {
			// In the public org we can create a user anonymously, so try that
			msgPrinter.Printf("User %s/%s does not exist in the exchange with the specified password, creating it...", org, user)
			msgPrinter.Println()
			postUserReq := cliutils.UserExchangeReq{Password: pw, Admin: false, Email: email}
			httpCode = cliutils.ExchangePutPost("Exchange", http.MethodPost, exchUrlBase, "orgs/"+org+"/users/"+user, "", []int{201}, postUserReq)

			// User created, now try to create the node again
			httpCode = cliutils.ExchangePutPost("Exchange", http.MethodPut, exchUrlBase, "orgs/"+org+"/nodes/"+nodeId, cliutils.OrgAndCreds(org, userPw), []int{201}, putNodeReq)
		} else {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("user '%s/%s' does not exist with the specified password or -e was not specified to be able to create it in the 'public' org.", org, user))
		}
	} else if httpCode == 403 {
		// Access denied means the node exists and is owned by another user. Figure out who and tell the user
		var nodesOutput exchange.GetDevicesResponse
		httpCode = cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes/"+nodeId, cliutils.OrgAndCreds(org, userPw), []int{200}, &nodesOutput)
		var ok bool
		var ourNode exchange.Device
		if ourNode, ok = nodesOutput.Devices[cliutils.OrgAndCreds(org, nodeId)]; !ok {
			cliutils.Fatal(cliutils.INTERNAL_ERROR, msgPrinter.Sprintf("key '%s' not found in exchange nodes output", cliutils.OrgAndCreds(org, nodeId)))
		}
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("can not update existing node %s because it is owned by another user (%s)", nodeId, ourNode.Owner))
	}
}

func NodeUpdate(org string, credToUse string, node string, filePath string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	cliutils.SetWhetherUsingApiKey(credToUse)
	var nodeOrg string
	nodeOrg, node = cliutils.TrimOrg(org, node)

	//check that the node exists
	var nodeReq ExchangeNodes
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+nodeOrg+"/nodes/"+node, cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &nodeReq)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("Node %s/%s not found in the Horizon Exchange.", nodeOrg, node))
	}

	attribute := cliconfig.ReadJsonFileWithLocalConfig(filePath)

	findPatchType := make(map[string]interface{})
	json.Unmarshal([]byte(attribute), &findPatchType)

	// check invalid attributes
	for k, _ := range findPatchType {
		if k != "userInput" && k != "pattern" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Cannot update attribute %v. Supported attributes are: userInput, pattern.", k))
		}
	}

	updated := false
	for k, v := range findPatchType {
		bytes, err := json.Marshal(v)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal attribute input %s: %v", v, err))
		}
		patch := make(map[string]interface{})
		if k == "userInput" {
			ui := []policy.UserInput{}
			if err := json.Unmarshal(bytes, &ui); err != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal attribute input %s: %v", v, err))
			} else {
				// validate userInput
				// if node has a pattern, check the service is defined in top level services
				nodes := nodeReq.Nodes
				if len(nodes) != 1 {
					cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Expecting 1 exchange node, but get %d nodes", len(nodes)))
				}

				nodePatternString := ""
				for _, node := range nodes {
					nodePatternString = node.Pattern
				}
				var topLevelServices []ServiceReference
				patternUserInputsMap := make(map[string]policy.UserInput)
				userInputWithEmptyDefaultValueMap := make(map[string][]exchange.UserInput)

				if nodePatternString != "" {
					patOrg, patternName := cliutils.TrimOrg(org, nodePatternString)
					var patternsResp ExchangePatterns
					// Get exchange pattern
					httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+patOrg+"/patterns"+cliutils.AddSlash(patternName), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &patternsResp)
					if httpCode == 404 {
						cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Pattern %v of the node does not exist for org: %v \n", nodePatternString, patOrg))
					}

					patterns := patternsResp.Patterns
					if len(patterns) != 1 {
						cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Expecting 1 pattern with name %v, but get %d patterns", nodePatternString, len(nodes)))
					}

					pattern, _ := patterns[nodePatternString]
					topLevelServices = pattern.Services
					patternUserInputs := pattern.UserInput

					for _, u := range patternUserInputs {
						mapKey := fmt.Sprintf("%v/%v", u.ServiceOrgid, u.ServiceUrl)
						patternUserInputsMap[mapKey] = u
					}

					// if there are userinput with no default value, and value is not defined in pattern userinput, userinput from command line cannot be empty
					_, err := getUserInputWithEmptyDefaultValue(cliutils.OrgAndCreds(org, credToUse), topLevelServices, patternUserInputsMap, userInputWithEmptyDefaultValueMap)
					if err == nil && len(userInputWithEmptyDefaultValueMap) != 0 {
						if len(ui) == 0 {
							cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Userinput cannot be set to empty list because some userinputs have no default value from service and pattern "))
						}
					} else if err != nil {
						cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("%v", err))
					}
				}

				if len(ui) != 0 {
					// make a map
					userInputsMap := make(map[string]policy.UserInput)
					for _, u := range ui {
						// validate u has serviceOrg and serviceUrl
						if u.ServiceOrgid == "" || u.ServiceUrl == "" {
							cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("ServiceOrgid and ServiceUrl need to be set in userinput"))
						}
						mapKey := fmt.Sprintf("%v/%v", u.ServiceOrgid, u.ServiceUrl)
						userInputsMap[mapKey] = u
					}

					for _, u := range ui {
						uInput := u
						validated, warningMessage, err := ValidateUserInput(org, credToUse, uInput, nodePatternString, topLevelServices, userInputsMap, patternUserInputsMap, userInputWithEmptyDefaultValueMap)
						if !validated {
							cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Unable to validate exchange node userInput, error: %v", err))
						} else if warningMessage != "" { // validate == true, but give back warning error message
							cliutils.Warning(msgPrinter.Sprintf("validate exchange node/userinput %v ", warningMessage))
						}

					}
				}
				patch[k] = ui
			}
		} else if k == "pattern" {
			pattern := ""
			if err := json.Unmarshal(bytes, &pattern); err != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal attribute input %s: %v", v, err))
			} else {
				patch[k] = pattern
			}
		}

		msgPrinter.Printf("Updating %v for node %v/%v in the Horizon Exchange.", k, nodeOrg, node)
		msgPrinter.Println()
		cliutils.ExchangePutPost("Exchange", http.MethodPatch, cliutils.GetExchangeUrl(), "orgs/"+nodeOrg+"/nodes/"+node, cliutils.OrgAndCreds(org, credToUse), []int{200, 201}, patch)
		msgPrinter.Printf("Attribute %v updated.", k)
		msgPrinter.Println()

		updated = true
	}

	// Tell user that the device will re-evaluating the agreements based on the node update
	if updated {
		for _, v := range nodeReq.Nodes {
			var exchNode exchange.Device
			bytes, err := json.Marshal(v)
			if err != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal attribute input %s: %v", v, err))
			}
			if err := json.Unmarshal(bytes, &exchNode); err != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal exchange node %s: %v", v, err))
			}
			// the exchange node is registered with a device, then give user more info.
			if exchNode.PublicKey != nil && len(exchNode.PublicKey) != 0 {
				msgPrinter.Printf("Device will re-evaluate all agreements based on the update. Existing agreements might be cancelled and re-negotiated.")
				msgPrinter.Println()
			}
			break
		}
	}
}

type NodeExchangePatchToken struct {
	Token string `json:"token"`
}

func NodeSetToken(org, credToUse, node, token string) {
	cliutils.SetWhetherUsingApiKey(credToUse)
	var nodeOrg string
	nodeOrg, node = cliutils.TrimOrg(org, node)
	patchNodeReq := NodeExchangePatchToken{Token: token}
	cliutils.ExchangePutPost("Exchange", http.MethodPatch, cliutils.GetExchangeUrl(), "orgs/"+nodeOrg+"/nodes/"+node, cliutils.OrgAndCreds(org, credToUse), []int{201}, patchNodeReq)
}

func NodeConfirm(org, node, token string, nodeIdTok string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	cliutils.SetWhetherUsingApiKey("")

	// check the input
	if nodeIdTok != "" {
		if node != "" || token != "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("-n is mutually exclusive with <node> and <token> arguments."))
		}
	} else {
		if node == "" && token == "" {
			nodeIdTok = os.Getenv("HZN_EXCHANGE_NODE_AUTH")
		}
	}

	if nodeIdTok != "" {
		node, token = cliutils.SplitIdToken(nodeIdTok)
		if node != "" {
			// trim the org off the node id. the HZN_EXCHANGE_NODE_AUTH may contain the org id.
			_, node = cliutils.TrimOrg(org, node)
		}
	}

	if node == "" || token == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Please specify both node and token."))
	}

	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes/"+node, cliutils.OrgAndCreds(org, node+":"+token), []int{200}, nil)
	if httpCode == 200 {
		msgPrinter.Printf("Node id and token are valid.")
		msgPrinter.Println()
	}
	// else cliutils.ExchangeGet() already gave the error msg
}

func NodeRemove(org, credToUse, node string, force bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	cliutils.SetWhetherUsingApiKey(credToUse)
	var nodeOrg string
	nodeOrg, node = cliutils.TrimOrg(org, node)

	if !force {
		cliutils.ConfirmRemove(msgPrinter.Sprintf("Are you sure you want to remove node %v/%v from the Horizon Exchange (should not be done while an edge node is registered with this node id)?", nodeOrg, node))
	}

	httpCode := cliutils.ExchangeDelete("Exchange", cliutils.GetExchangeUrl(), "orgs/"+nodeOrg+"/nodes/"+node, cliutils.OrgAndCreds(org, credToUse), []int{204, 404})
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("node '%s' not found in org %s", node, nodeOrg))
	}
}

func NodeListPolicy(org string, credToUse string, node string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	cliutils.SetWhetherUsingApiKey(credToUse)
	var nodeOrg string
	nodeOrg, node = cliutils.TrimOrg(org, node)

	// check node exists first
	var nodes ExchangeNodes
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+nodeOrg+"/nodes"+cliutils.AddSlash(node), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &nodes)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("node '%v/%v' not found.", nodeOrg, node))
	}

	// list policy
	var policy exchange.ExchangePolicy
	cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+nodeOrg+"/nodes"+cliutils.AddSlash(node)+"/policy", cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &policy)
	output := cliutils.MarshalIndent(policy.GetExternalPolicy(), "exchange node listpolicy")
	fmt.Println(output)
}

func NodeAddPolicy(org string, credToUse string, node string, jsonFilePath string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	cliutils.SetWhetherUsingApiKey(credToUse)
	var nodeOrg string
	nodeOrg, node = cliutils.TrimOrg(org, node)

	// Read in the policy metadata
	newBytes := cliconfig.ReadJsonFileWithLocalConfig(jsonFilePath)
	var policyFile externalpolicy.ExternalPolicy
	err := json.Unmarshal(newBytes, &policyFile)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal json input file %s: %v", jsonFilePath, err))
	}

	//Check the policy file format
	err = policyFile.Validate()
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Incorrect policy format in file %s: %v", jsonFilePath, err))
	}

	// check node exists first
	var nodes ExchangeNodes
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+nodeOrg+"/nodes"+cliutils.AddSlash(node), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &nodes)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("node '%v/%v' not found.", nodeOrg, node))
	}

	// add/replce node policy
	msgPrinter.Printf("Updating Node policy and re-evaluating all agreements based on this policy. Existing agreements might be cancelled and re-negotiated.")
	msgPrinter.Println()
	cliutils.ExchangePutPost("Exchange", http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+nodeOrg+"/nodes/"+node+"/policy", cliutils.OrgAndCreds(org, credToUse), []int{201}, policyFile)

	msgPrinter.Printf("Node policy updated.")
	msgPrinter.Println()
}

func NodeUpdatePolicy(org, credToUse, node string, jsonfile string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	cliutils.SetWhetherUsingApiKey(credToUse)
	var nodeOrg string
	nodeOrg, node = cliutils.TrimOrg(org, node)

	attribute := cliconfig.ReadJsonFileWithLocalConfig(jsonfile)

	//Check if the node policy exists in the exchange
	var newPolicy externalpolicy.ExternalPolicy
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+nodeOrg+"/nodes"+cliutils.AddSlash(node)+"/policy", cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &newPolicy)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("Node policy not found for node %s/%s", nodeOrg, node))
	}

	findAttrType := make(map[string]interface{})
	err := json.Unmarshal([]byte(attribute), &findAttrType)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal attribute input %s: %v", attribute, err))
	}

	if _, ok := findAttrType["properties"]; ok {
		propertiesPatch := make(map[string]externalpolicy.PropertyList)
		err := json.Unmarshal([]byte(attribute), &propertiesPatch)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal attribute input %s: %v", attribute, err))
		}
		newProp := propertiesPatch["properties"]
		err = newProp.Validate()
		if err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Invalid property list %s: %v", attribute, err))
		}
		newPolicy.Properties = newProp
		msgPrinter.Printf("Updating Node %s policy properties in the horizon exchange and re-evaluating all agreements based on this policy. Existing agreements might be cancelled and re-negotiated.", node)
		msgPrinter.Println()
		cliutils.ExchangePutPost("Exchange", http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+nodeOrg+"/nodes/"+node+"/policy", cliutils.OrgAndCreds(org, credToUse), []int{200, 201}, newPolicy)
		msgPrinter.Printf("Node %s policy properties updated in the horizon exchange.", node)
		msgPrinter.Println()
	} else if _, ok = findAttrType["constraints"]; ok {
		constraintPatch := make(map[string]externalpolicy.ConstraintExpression)
		err := json.Unmarshal([]byte(attribute), &constraintPatch)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal attribute input %s: %v", attribute, err))
		}
		newConstr := constraintPatch["constraints"]
		_, err = newConstr.Validate()
		if err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Invalid constraint expression %s: %v", attribute, err))
		}
		newPolicy.Constraints = newConstr
		msgPrinter.Printf("Updating Node %s policy constraints in the horizon exchange and re-evaluating all agreements based on this policy. Existing agreements might be cancelled and re-negotiated.", node)
		msgPrinter.Println()
		cliutils.ExchangePutPost("Exchange", http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+nodeOrg+"/nodes/"+node+"/policy", cliutils.OrgAndCreds(org, credToUse), []int{200, 201}, newPolicy)
		msgPrinter.Printf("Node %s policy constraints updated in the horizon exchange.", node)
		msgPrinter.Println()
	} else {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Failed to find valid attribute to update in input %s. Attributes are constraints and properties.", attribute))
	}
}

func NodeRemovePolicy(org, credToUse, node string, force bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	cliutils.SetWhetherUsingApiKey(credToUse)
	var nodeOrg string
	nodeOrg, node = cliutils.TrimOrg(org, node)
	if !force {
		cliutils.ConfirmRemove(msgPrinter.Sprintf("Are you sure you want to remove node policy for %v/%v from the Horizon Exchange?", nodeOrg, node))
	}

	// check node exists first
	var nodes ExchangeNodes
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+nodeOrg+"/nodes"+cliutils.AddSlash(node), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &nodes)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("node '%v/%v' not found.", nodeOrg, node))
	}

	// remove policy
	msgPrinter.Printf("Removing Node policy and re-evaluating all agreements based on just the built-in node policy. Existing agreements might be cancelled and re-negotiated.")
	msgPrinter.Println()
	cliutils.ExchangeDelete("Exchange", cliutils.GetExchangeUrl(), "orgs/"+nodeOrg+"/nodes/"+node+"/policy", cliutils.OrgAndCreds(org, credToUse), []int{204, 404})
	msgPrinter.Printf("Node policy removed.")
	msgPrinter.Println()

}

// Format for outputting eventlog objects
type EventLog struct {
	Id         string           `json:"record_id"` // unique primary key for records
	Timestamp  string           `json:"timestamp"` // converted to "yyyy-mm-dd hh:mm:ss" format
	Severity   string           `json:"severity"`  // info, warning or error
	Message    string           `json:"message"`
	EventCode  string           `json:"event_code"`
	SourceType string           `json:"source_type"`  // the type of the source. It can be agreement, service, image, workload etc.
	Source     *json.RawMessage `json:"event_source"` // source involved for this event.
}

// NodeListErrors Displays the node errors currently surfaced to the exchange
func NodeListErrors(org string, credToUse string, node string, long bool) {
	msgPrinter := i18n.GetMessagePrinter()

	cliutils.SetWhetherUsingApiKey(credToUse)
	var nodeOrg string
	nodeOrg, node = cliutils.TrimOrg(org, node)

	// Check that the node specified exists in the exchange
	var nodes ExchangeNodes
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+nodeOrg+"/nodes"+cliutils.AddSlash(node), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &nodes)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("node '%v/%v' not found.", nodeOrg, node))
	}

	var resp exchange.ExchangeSurfaceError
	httpCode = cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+nodeOrg+"/nodes/"+node+"/errors", cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &resp)

	errorList := []persistence.SurfaceError{}
	if resp.ErrorList != nil {
		errorList = resp.ErrorList
	}

	if !long {
		jsonBytes, err := json.MarshalIndent(errorList, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn exchange node listerrors' output: %v", err))
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		long_output := make([]EventLog, len(errorList))
		for i, v := range errorList {
			var fullVSlice []persistence.EventLogRaw
			cliutils.HorizonGet(fmt.Sprintf("eventlog/all?record_id=%s", v.Record_id), []int{200}, &fullVSlice, false)
			if len(fullVSlice) == 0 {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, i18n.GetMessagePrinter().Sprintf("Error: event record could not be found"))
			}
			fullV := fullVSlice[0]
			long_output[i].Id = fullV.Id
			long_output[i].Timestamp = cliutils.ConvertTime(fullV.Timestamp)
			long_output[i].Severity = fullV.Severity
			long_output[i].Message = fullV.Message
			long_output[i].EventCode = fullV.EventCode
			long_output[i].SourceType = fullV.SourceType
			long_output[i].Source = fullV.Source
		}
		jsonBytes, err := json.MarshalIndent(long_output, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, i18n.GetMessagePrinter().Sprintf("failed to marshal 'hzn exchange node listerrors' output: %v", err))
		}
		fmt.Printf("%s\n", jsonBytes)
	}
}

func ValidateUserInput(org string, credToUse string, userInput policy.UserInput, nodePattern string, topLevelServices []ServiceReference, userInputsMap map[string]policy.UserInput, patternUserInputsMap map[string]policy.UserInput, userInputWithEmptyDefaultValueMap map[string][]exchange.UserInput) (bool, string, error) {
	msgPrinter := i18n.GetMessagePrinter()
	warningMessage := ""

	serviceOrg := userInput.ServiceOrgid
	serviceUrl := userInput.ServiceUrl
	serviceVersionRange := userInput.ServiceVersionRange
	serviceArch := userInput.ServiceArch

	msgPrinter.Printf("Start validating userinput %v, %v, %v, %v", serviceOrg, serviceUrl, serviceVersionRange, serviceArch)
	msgPrinter.Println()

	var errorString string
	if &userInput.ServiceVersionRange == nil || userInput.ServiceVersionRange == "" {
		def := "0.0.0"
		serviceVersionRange = def
	}

	// Convert the version to a version expression.
	vExp, err := semanticversion.Version_Expression_Factory(serviceVersionRange)
	if err != nil {
		errorString = msgPrinter.Sprintf("versionRange %v cannot be converted to a version expression, error %v", userInput.ServiceVersionRange, err)
		return false, warningMessage, errors.New(errorString)
	}
	serviceVersion := vExp.Get_expression()

	// service exist
	var sdef *exchange.ServiceDefinition

	searchVersion, err := getSearchVersion(serviceVersion)
	if err != nil {
		return false, warningMessage, err
	}

	sdef, err = getService(cliutils.OrgAndCreds(org, credToUse), serviceOrg, serviceUrl, serviceArch, serviceVersion, searchVersion)
	if sdef == nil || err != nil {
		return false, warningMessage, err
	}

	// compare ServiceDefinition.UserInput(array) with userInput.Inputs(array)
	var compareResult bool
	compareResult, err = compareUserInput(sdef.UserInputs, userInput.Inputs)
	if !compareResult {
		errorString = msgPrinter.Sprintf("Failed to compare given userInput with service userInput, error: %v", err)
		return false, warningMessage, errors.New(errorString)
	} else if err != nil {
		// give back a warning
		warningMessage = msgPrinter.Sprintf("For service %v %v %v, got warning message: %v", serviceOrg, serviceUrl, serviceArch, err.Error())
	}

	// Need to check:
	// 1) if service is top level service
	// 2) or recursively check if service defined in dependent service
	// 3) if service range of the dependent service has intersection with versionRange of service from userInput
	// 4) For services (toplevel service and required services) that have userinput without default value, it should be set in userinput from user
	foundService := false

	if &topLevelServices != nil && len(topLevelServices) != 0 {
		for _, topLevelService := range topLevelServices {
			topLevelServiceVersionNumber := topLevelService.ServiceVersions[0].Version
			sdef, err = getService(cliutils.OrgAndCreds(org, credToUse), topLevelService.ServiceOrg, topLevelService.ServiceURL, topLevelService.ServiceArch, topLevelServiceVersionNumber, "")
			if sdef == nil || err != nil {
				return false, warningMessage, errors.New(msgPrinter.Sprintf("toplevel service does not exist %v, %v, %v, %v", topLevelService.ServiceOrg, topLevelService.ServiceURL, topLevelService.ServiceArch, topLevelServiceVersionNumber))
			}

			mapKey := fmt.Sprintf("%v/%v", topLevelService.ServiceOrg, topLevelService.ServiceURL)
			emptyDefaultValueUserInput, ok := userInputWithEmptyDefaultValueMap[mapKey]

			// if !ok, means all the default value are set either from service or pattern userinput
			if ok {
				u, inCmdUserInput := userInputsMap[mapKey]
				if !inCmdUserInput {
					return false, warningMessage, errors.New(msgPrinter.Sprintf("%v %v has userInput that need to be set because they have no default value.", topLevelService.ServiceOrg, topLevelService.ServiceURL))
				} else {
					for _, emptyDefaultValue := range emptyDefaultValueUserInput {
						if definedInCmdUserInput, err := cmdInputsContains(u.Inputs, emptyDefaultValue.Name); !definedInCmdUserInput {
							return false, warningMessage, errors.New(msgPrinter.Sprintf("%v", err))
						}
					}
				}
			}

			var topLevelServiceVersion ServiceChoice
			if serviceOrg == topLevelService.ServiceOrg && serviceUrl == topLevelService.ServiceURL {
				for _, topLevelServiceVersion = range topLevelService.ServiceVersions {
					if withinRange, _ := vExp.Is_within_range(topLevelServiceVersion.Version); withinRange == true {
						sdef, err = getService(cliutils.OrgAndCreds(org, credToUse), topLevelService.ServiceOrg, topLevelService.ServiceURL, topLevelService.ServiceArch, topLevelServiceVersion.Version, "")
						if sdef == nil || err != nil {
							return false, warningMessage, errors.New(msgPrinter.Sprintf("toplevel service does not exist %v, %v, %v, %v", topLevelService.ServiceOrg, topLevelService.ServiceURL, topLevelService.ServiceArch, topLevelServiceVersion.Version))
						}
						foundService = true
						//return true, warningMessage, nil
					}
				}
			}

			// recursively check required services/dependent services
			foundMatchService := make([]bool, 1)
			findMatchInDepService, defaultValueCheck, err := checkDependentServices(cliutils.OrgAndCreds(org, credToUse), serviceOrg, serviceUrl, vExp, sdef, userInputsMap, patternUserInputsMap, userInputWithEmptyDefaultValueMap, foundMatchService)
			if !defaultValueCheck {
				return false, warningMessage, err
			}

			if !foundService && findMatchInDepService {
				foundService = true
			}

		}
		if !foundService {
			errorString = msgPrinter.Sprintf("Service %v %v %v %v in userInput is not defined in pattern %v or its dependent services", serviceOrg, serviceUrl, serviceArch, vExp.Get_expression(), nodePattern)
			return false, warningMessage, errors.New(errorString)
		}
	} else {
		if checkDefaultValueInServiceUserInput, err := checkDefaultValueInServiceUserInput(sdef.UserInputs, userInput.Inputs); !checkDefaultValueInServiceUserInput {
			warningMessage = msgPrinter.Sprintf("%vWarning: %v", warningMessage, err)
		}
	}

	return true, warningMessage, nil

}

// return bool, bool, err.
// first bool return - indicate if service is defined in the dependent service by recursively check
// second bool return - indicate if required service has userinputs without default values
func checkDependentServices(creds string, serviceOrg string, serviceUrl string, serviceVersionExp *semanticversion.Version_Expression, servToCheckDependentService *exchange.ServiceDefinition, userInputsMap map[string]policy.UserInput, patternUserInputsMap map[string]policy.UserInput, userInputWithEmptyDefaultValueMap map[string][]exchange.UserInput, foundMatchService []bool) (bool, bool, error) {
	msgPrinter := i18n.GetMessagePrinter()
	// versionRange is "" for the 1st interation

	dependentServices := servToCheckDependentService.RequiredServices
	if len(dependentServices) == 0 {
		if foundMatchService[0] {
			return true, true, nil
		} else {
			return false, true, errors.New(msgPrinter.Sprintf("No dependent service for service %v, %v, %v, %v", servToCheckDependentService.Owner, servToCheckDependentService.URL, servToCheckDependentService.Arch, servToCheckDependentService.Version))
		}

	}
	for _, dependentService := range dependentServices {
		dependentServiceVersionRange := dependentService.VersionRange
		if &dependentService.VersionRange == nil || dependentService.VersionRange == "" {
			def := "0.0.0"
			dependentServiceVersionRange = def
		}
		dependentServiceVersionExp, _ := semanticversion.Version_Expression_Factory(dependentServiceVersionRange)
		dependentServiceVersion := dependentServiceVersionExp.Get_expression()
		dependentServiceSearchVersion, _ := getSearchVersion(dependentServiceVersion)

		dsdef, err := getService(creds, dependentService.Org, dependentService.URL, dependentService.Arch, dependentServiceVersion, dependentServiceSearchVersion)
		if dsdef == nil || err != nil {
			return false, true, errors.New(msgPrinter.Sprintf("service does not exist %v, %v, %v, %v, %v", dependentService.Org, dependentService.URL, dependentService.Arch, dependentServiceVersion, dependentServiceSearchVersion))
		}

		mapKey := fmt.Sprintf("%v/%v", dependentService.Org, dependentService.URL)
		emptyDefaultValueUserInput, ok := userInputWithEmptyDefaultValueMap[mapKey]

		if ok {
			u, inCmdUserInput := userInputsMap[mapKey]
			if !inCmdUserInput {
				return foundMatchService[0], false, errors.New(msgPrinter.Sprintf("%v %v has userInput that need to be set because they have no default value.", dependentService.Org, dependentService.URL))
			} else {
				for _, emptyDefaultValue := range emptyDefaultValueUserInput {
					if definedInCmdUserInput, err := cmdInputsContains(u.Inputs, emptyDefaultValue.Name); !definedInCmdUserInput {
						return foundMatchService[0], false, errors.New(msgPrinter.Sprintf("%v", err))
					}
				}
			}
		}

		if serviceOrg == dependentService.Org && serviceUrl == dependentService.URL {
			// true if versionRange of the dependent service has intersection with versionRange of service from user input
			if err = serviceVersionExp.IntersectsWith(dependentServiceVersionExp); err == nil {
				foundMatchService[0] = true
			}

		}

		// recursive call
		_, defaultValueCheck, err := checkDependentServices(creds, serviceOrg, serviceUrl, serviceVersionExp, dsdef, userInputsMap, patternUserInputsMap, userInputWithEmptyDefaultValueMap, foundMatchService)
		if !defaultValueCheck {
			return foundMatchService[0], false, err
		}
	}

	if foundMatchService[0] {
		return true, true, nil
	}
	return false, true, errors.New(msgPrinter.Sprintf("Service %v, %v, %v does not define a dependent service", serviceOrg, serviceUrl, serviceVersionExp.Get_expression()))

}

func getSearchVersion(version string) (string, error) {
	msgPrinter := i18n.GetMessagePrinter()
	// The caller could pass a specific version or a version range, in the version parameter. If it's a version range
	// then it must be a full expression. That is, it must be expanded into the full syntax. For example; 1.2.3 is a specific
	// version, and [4.5.6, INFINITY) is the full expression corresponding to the shorthand form of "4.5.6".
	searchVersion := ""
	if version == "" || semanticversion.IsVersionExpression(version) {
		// search for all versions
	} else if semanticversion.IsVersionString(version) {
		// search for a specific version
		searchVersion = version
	} else {
		return "", errors.New(msgPrinter.Sprintf("input version %v is not a valid version string", version))
	}
	return searchVersion, nil
}

func getService(creds string, serviceOrg string, serviceUrl string, serviceArch string, serviceVersion string, searchVersion string) (*exchange.ServiceDefinition, error) {
	msgPrinter := i18n.GetMessagePrinter()
	var servicesResp exchange.GetServicesResponse
	targetURL := fmt.Sprintf("orgs/%v/services?url=%v", serviceOrg, serviceUrl)
	if searchVersion != "" {
		targetURL = fmt.Sprintf("%v&version=%v", targetURL, searchVersion)
	}

	if serviceArch != "" {
		targetURL = fmt.Sprintf("%v&arch=%v", targetURL, serviceArch)
	}

	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), targetURL, creds, []int{200, 404}, &servicesResp)
	if httpCode == 404 {
		errorString := msgPrinter.Sprintf("Service does not exist for org: %v, url: %v, version: %v, arch: %v \n", serviceOrg, serviceUrl, serviceVersion, serviceArch)
		return nil, errors.New(errorString)
	}

	// processGetServiceResponse to get the service with highest version
	return processGetServiceResponse(servicesResp, serviceUrl, serviceArch, serviceVersion, searchVersion)
}

func processGetServiceResponse(servicesResp exchange.GetServicesResponse, serviceUrl string, serviceArch string, serviceVersion string, searchVersion string) (*exchange.ServiceDefinition, error) {
	msgPrinter := i18n.GetMessagePrinter()
	services := servicesResp.Services

	var errorString string
	if searchVersion != "" {
		if serviceArch != "" {
			// return the given version, expecting 1 service
			if len(services) != 1 {
				errorString = msgPrinter.Sprintf("expecting 1 service for %v %v %v, but get %d services", serviceUrl, serviceVersion, searchVersion, len(services))
				return nil, errors.New(errorString)
			}
		}

		for _, serviceDef := range services {
			return &serviceDef, nil
		}

		return nil, errors.New("should not get here")

	} else {
		// get the hightest
		if len(services) == 0 {
			errorString = msgPrinter.Sprintf("expecting at least 1 service for %v %v %v, but get 0 service", serviceUrl, serviceVersion, searchVersion)
			return nil, errors.New(errorString)
		}

		vRange, _ := semanticversion.Version_Expression_Factory("0.0.0")
		var err error
		if serviceVersion != "" {
			if vRange, err = semanticversion.Version_Expression_Factory(serviceVersion); err != nil {
				errorString = msgPrinter.Sprintf("version range %v in error: %v", serviceVersion, err)
				return nil, errors.New(errorString)
			}
		}

		highest, serviceDef, _, err := exchange.GetHighestVersion(services, vRange)
		if err != nil {
			return nil, err
		}

		if highest == "" {
			errorString = msgPrinter.Sprintf("Cannot find service given version range: %v", serviceVersion)
			return nil, errors.New(errorString)
		}
		return &serviceDef, nil

	}
}

func compareUserInput(sdefUserInputs []exchange.UserInput, cmdUserInputs []policy.Input) (bool, error) {
	msgPrinter := i18n.GetMessagePrinter()
	serviceUserInputsMap := make(map[string]exchange.UserInput)
	var serviceInputName string
	var serviceInput exchange.UserInput
	var errorString string

	for _, serviceInput = range sdefUserInputs {
		serviceInputName = serviceInput.Name
		serviceUserInputsMap[serviceInputName] = serviceInput
	}

	// return true with warning message if the variables in userInput.Inputs are not defined in service, also check values are correct types
	var policyInputName string
	var policyInputValue interface{}
	var ok bool
	var inputNameNotDefinedInService []string

	for _, policyInput := range cmdUserInputs {
		policyInputName = policyInput.Name
		policyInputValue = policyInput.Value

		serviceInput, ok = serviceUserInputsMap[policyInputName]
		if !ok {
			// give back a warning
			inputNameNotDefinedInService = append(inputNameNotDefinedInService, policyInputName)
		} else {
			if err := cutil.VerifyWorkloadVarTypes(policyInputValue, serviceInput.Type); err != nil {
				errorString = msgPrinter.Sprintf("Error validating the type of userInput variable %v. error: %v", policyInputName, err)
				return false, errors.New(errorString)
			}
		}
	}

	if len(inputNameNotDefinedInService) != 0 {
		errorString = msgPrinter.Sprintf("The following variables are not defined in userInput for service in its highest version: %v \n", inputNameNotDefinedInService)
		return true, errors.New(errorString)
	}
	return true, nil

}

func sliceContains(sliceToCheck []string, elem string) bool {
	for _, e := range sliceToCheck {
		if e == elem {
			return true
		}
	}

	return false
}

// if service has a userinput with no default value AND userInput is not set in input body, return err
func checkDefaultValueInServiceUserInput(sdefUserInputs []exchange.UserInput, cmdUserInputs []policy.Input) (bool, error) {
	msgPrinter := i18n.GetMessagePrinter()
	var cmdUserInputNames []string
	var serviceInputName string
	var serviceInput exchange.UserInput

	for _, u := range cmdUserInputs {
		cmdUserInputNames = append(cmdUserInputNames, u.Name)
	}

	for _, serviceInput = range sdefUserInputs {
		serviceInputName = serviceInput.Name

		if serviceInput.DefaultValue == "" {
			valueDefined := sliceContains(cmdUserInputNames, serviceInputName)
			if !valueDefined {
				errorString := msgPrinter.Sprintf("%v needs to be set in userInput because it has no default value.", serviceInputName)
				return false, errors.New(errorString)
			}
		}
	}

	return true, nil
}

func checkUserInputDefaultValueNotEmpty(org string, sdef *exchange.ServiceDefinition, patternUserInput *policy.UserInput, userInputWithEmptyDefaultValueMap map[string][]exchange.UserInput) map[string][]exchange.UserInput {
	if sdef != nil {
		for _, u := range sdef.UserInputs {
			contains := false
			if u.DefaultValue == "" {
				if patternUserInput != nil {
					for _, patternInput := range patternUserInput.Inputs {
						if patternInput.Name == u.Name {
							contains = true
							break
						}
					}
				}
				if !contains {
					mapKey := fmt.Sprintf("%v/%v", org, sdef.URL)
					emptyDefaultValueNames, exist := userInputWithEmptyDefaultValueMap[mapKey]
					if exist {
						emptyDefaultValueNames = append(emptyDefaultValueNames, u)
					}

					userInputWithEmptyDefaultValueMap[mapKey] = emptyDefaultValueNames
				}
			}
		}
	}

	return userInputWithEmptyDefaultValueMap
}

func cmdInputsContains(cmdInputs []policy.Input, serviceInputName string) (bool, error) {
	msgPrinter := i18n.GetMessagePrinter()
	for _, input := range cmdInputs {
		if input.Name == serviceInputName {
			return true, nil
		}
	}

	errorString := msgPrinter.Sprintf("%v needs to be set in userInput because it has no default value.", serviceInputName)
	return false, errors.New(errorString)
}

func getUserInputWithEmptyDefaultValue(creds string, topLevelServices []ServiceReference, patternUserInputsMap map[string]policy.UserInput, userInputWithEmptyDefaultValueMap map[string][]exchange.UserInput) (*map[string][]exchange.UserInput, error) {
	msgPrinter := i18n.GetMessagePrinter()
	if &topLevelServices != nil && len(topLevelServices) != 0 {
		for _, topLevelService := range topLevelServices {
			topLevelServiceVersionNumber := topLevelService.ServiceVersions[0].Version
			sdef, err := getService(creds, topLevelService.ServiceOrg, topLevelService.ServiceURL, topLevelService.ServiceArch, topLevelServiceVersionNumber, "")
			if sdef == nil || err != nil {
				return nil, errors.New(msgPrinter.Sprintf("toplevel service does not exist %v, %v, %v, %v", topLevelService.ServiceOrg, topLevelService.ServiceURL, topLevelService.ServiceArch, topLevelServiceVersionNumber))
			}

			pattenUserInputMapKey := fmt.Sprintf("%v/%v", topLevelService.ServiceOrg, topLevelService.ServiceURL)
			patternUserInput, ok := patternUserInputsMap[pattenUserInputMapKey]
			if !ok {
				checkUserInputDefaultValueNotEmpty(topLevelService.ServiceOrg, sdef, nil, userInputWithEmptyDefaultValueMap)
			} else {
				checkUserInputDefaultValueNotEmpty(topLevelService.ServiceOrg, sdef, &patternUserInput, userInputWithEmptyDefaultValueMap)
			}

			// recursively check required services/dependent services
			_, err = checkDependentServiceEmptyDefaultValue(creds, sdef, patternUserInputsMap, userInputWithEmptyDefaultValueMap)
			if err != nil {
				return nil, err
			}

		}

	}

	return &userInputWithEmptyDefaultValueMap, nil

}

func checkDependentServiceEmptyDefaultValue(creds string, servToCheckDependentService *exchange.ServiceDefinition, patternUserInputsMap map[string]policy.UserInput, userInputWithEmptyDefaultValueMap map[string][]exchange.UserInput) (*map[string][]exchange.UserInput, error) {
	msgPrinter := i18n.GetMessagePrinter()
	// versionRange is "" for the 1st interation

	dependentServices := servToCheckDependentService.RequiredServices
	if len(dependentServices) == 0 {
		return &userInputWithEmptyDefaultValueMap, nil

	}

	for _, dependentService := range dependentServices {
		dependentServiceVersionRange := dependentService.VersionRange
		if &dependentService.VersionRange == nil || dependentService.VersionRange == "" {
			def := "0.0.0"
			dependentServiceVersionRange = def
		}
		dependentServiceVersionExp, _ := semanticversion.Version_Expression_Factory(dependentServiceVersionRange)
		dependentServiceVersion := dependentServiceVersionExp.Get_expression()
		dependentServiceSearchVersion, _ := getSearchVersion(dependentServiceVersion)

		dsdef, err := getService(creds, dependentService.Org, dependentService.URL, dependentService.Arch, dependentServiceVersion, dependentServiceSearchVersion)
		if dsdef == nil || err != nil {
			return nil, errors.New(msgPrinter.Sprintf("service does not exist %v, %v, %v, %v, %v", dependentService.Org, dependentService.URL, dependentService.Arch, dependentServiceVersion, dependentServiceSearchVersion))
		}
		pattenUserInputMapKey := fmt.Sprintf("%v/%v", dependentService.Org, dependentService.URL)
		patternUserInput, ok := patternUserInputsMap[pattenUserInputMapKey]

		if !ok {
			checkUserInputDefaultValueNotEmpty(dependentService.Org, dsdef, nil, userInputWithEmptyDefaultValueMap)
		} else {
			checkUserInputDefaultValueNotEmpty(dependentService.Org, dsdef, &patternUserInput, userInputWithEmptyDefaultValueMap)
		}

		// recursive call
		_, err = checkDependentServiceEmptyDefaultValue(creds, dsdef, patternUserInputsMap, userInputWithEmptyDefaultValueMap)
		if err != nil {
			return nil, err
		}

	}

	return &userInputWithEmptyDefaultValueMap, nil

}
