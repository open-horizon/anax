package exchange

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	_ "github.com/open-horizon/anax/externalpolicy/text_language"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
)

// We only care about handling the node names, so the rest is left as interface{} and will be passed from the exchange to the display
type ExchangeNodes struct {
	LastIndex int                    `json:"lastIndex"`
	Nodes     map[string]interface{} `json:"nodes"`
}

func NodeList(org string, credToUse string, node string, namesOnly bool) {
	cliutils.SetWhetherUsingApiKey(credToUse)
	org, node = cliutils.TrimOrg(org, node)
	if node == "*" {
		node = ""
	}
	if namesOnly && node == "" {
		// Only display the names
		var resp ExchangeNodes
		cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes"+cliutils.AddSlash(node), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &resp)
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
		httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes"+cliutils.AddSlash(node), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &nodes)
		if httpCode == 404 && node != "" {
			cliutils.Fatal(cliutils.NOT_FOUND, i18n.GetMessagePrinter().Sprintf("node '%s' not found in org %s", node, org))
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
	org, node = cliutils.TrimOrg(org, node)

	attribute := cliconfig.ReadJsonFileWithLocalConfig(filePath)

	//check that the node exists
	var nodeReq ExchangeNodes
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes/"+node, cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &nodeReq)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("Node %s/%s not found in the Horizon Exchange.", org, node))
	}

	cliutils.ExchangePutPost("Exchange", http.MethodPatch, cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes/"+node, cliutils.OrgAndCreds(org, credToUse), []int{200, 201}, attribute)
	msgPrinter.Printf("Node %s updated in the Horizon Exchange.", node)
	msgPrinter.Println()
}

type NodeExchangePatchToken struct {
	Token string `json:"token"`
}

func NodeSetToken(org, credToUse, node, token string) {
	cliutils.SetWhetherUsingApiKey(credToUse)
	patchNodeReq := NodeExchangePatchToken{Token: token}
	cliutils.ExchangePutPost("Exchange", http.MethodPatch, cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes/"+node, cliutils.OrgAndCreds(org, credToUse), []int{201}, patchNodeReq)
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
		msgPrinter.Println("Node id and token are valid.")
	}
	// else cliutils.ExchangeGet() already gave the error msg
}

func NodeRemove(org, credToUse, node string, force bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	cliutils.SetWhetherUsingApiKey(credToUse)
	org, node = cliutils.TrimOrg(org, node)
	if !force {
		cliutils.ConfirmRemove(msgPrinter.Sprintf("Are you sure you want to remove node %v/%v from the Horizon Exchange (should not be done while an edge node is registered with this node id)?", org, node))
	}

	httpCode := cliutils.ExchangeDelete("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes/"+node, cliutils.OrgAndCreds(org, credToUse), []int{204, 404})
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("node '%s' not found in org %s", node, org))
	}
}

func NodeListPolicy(org string, credToUse string, node string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	cliutils.SetWhetherUsingApiKey(credToUse)
	org, node = cliutils.TrimOrg(org, node)

	// check node exists first
	var nodes ExchangeNodes
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes"+cliutils.AddSlash(node), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &nodes)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("node '%v/%v' not found.", org, node))
	}

	// list policy
	var policy exchange.ExchangePolicy
	cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes"+cliutils.AddSlash(node)+"/policy", cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &policy)
	output := cliutils.MarshalIndent(policy.GetExternalPolicy(), "exchange node listpolicy")
	fmt.Println(output)
}

func NodeAddPolicy(org string, credToUse string, node string, jsonFilePath string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	cliutils.SetWhetherUsingApiKey(credToUse)
	org, node = cliutils.TrimOrg(org, node)

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
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes"+cliutils.AddSlash(node), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &nodes)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("node '%v/%v' not found.", org, node))
	}

	// add/replce node policy
	msgPrinter.Println("Updating Node policy and re-evaluating all agreements based on this policy. Existing agreements might be cancelled and re-negotiated.")
	msgPrinter.Println()
	cliutils.ExchangePutPost("Exchange", http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes/"+node+"/policy", cliutils.OrgAndCreds(org, credToUse), []int{201}, policyFile)

	msgPrinter.Println("Node policy updated.")
}

func NodeUpdatePolicy(org, credToUse, node string, jsonfile string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	cliutils.SetWhetherUsingApiKey(credToUse)
	org, node = cliutils.TrimOrg(org, node)

	attribute := cliconfig.ReadJsonFileWithLocalConfig(jsonfile)

	//Check if the node policy exists in the exchange
	var newPolicy externalpolicy.ExternalPolicy
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes"+cliutils.AddSlash(node)+"/policy", cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &newPolicy)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("Node policy not found for node %s/%s", node, org))
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
		cliutils.ExchangePutPost("Exchange", http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes/"+node+"/policy", cliutils.OrgAndCreds(org, credToUse), []int{200, 201}, newPolicy)
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
		cliutils.ExchangePutPost("Exchange", http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes/"+node+"/policy", cliutils.OrgAndCreds(org, credToUse), []int{200, 201}, newPolicy)
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
	org, node = cliutils.TrimOrg(org, node)
	if !force {
		cliutils.ConfirmRemove(msgPrinter.Sprintf("Are you sure you want to remove node policy for %v/%v from the Horizon Exchange?", org, node))
	}

	// check node exists first
	var nodes ExchangeNodes
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes"+cliutils.AddSlash(node), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &nodes)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("node '%v/%v' not found.", org, node))
	}

	// remove policy
	msgPrinter.Printf("Removing Node policy and re-evaluating all agreements based on just the built-in node policy. Existing agreements might be cancelled and re-negotiated.")
	msgPrinter.Println()
	cliutils.ExchangeDelete("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes/"+node+"/policy", cliutils.OrgAndCreds(org, credToUse), []int{204, 404})
	msgPrinter.Println("Node policy removed.")

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
	org, node = cliutils.TrimOrg(org, node)

	var resp exchange.ExchangeSurfaceError
	cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes/"+node+"/errors", cliutils.OrgAndCreds(org, credToUse), []int{200}, &resp)
	errorList := resp.ErrorList

	if !long {
		jsonBytes, err := json.MarshalIndent(errorList, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn exchange node listerrors' output: %v", err))
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		output := make([]EventLog, len(errorList))
		for i, surfError := range errorList {
			var fullEventLogSlice []persistence.EventLogRaw
			cliutils.HorizonGet(fmt.Sprintf("eventlog/all?record_id=%s", surfError.Record_id), []int{200, 404}, fullEventLogSlice, false)
			if len(fullEventLogSlice) > 0 {
				var fullEventLog persistence.EventLogRaw
				fullEventLog = fullEventLogSlice[0]
				output[i] = EventLog{Id: fullEventLog.Id, Timestamp: cliutils.ConvertTime(fullEventLog.Timestamp), Severity: fullEventLog.Severity, Message: fullEventLog.Message, EventCode: fullEventLog.EventCode, SourceType: fullEventLog.SourceType, Source: fullEventLog.Source}
			}
		}
		jsonBytes, err := json.MarshalIndent(output, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn exchange node listerrors' output: %v", err))
		}
		fmt.Printf("%s\n", jsonBytes)
	}
}
