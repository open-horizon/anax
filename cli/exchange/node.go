package exchange

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/common"
	"github.com/open-horizon/anax/compcheck"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	_ "github.com/open-horizon/anax/externalpolicy/text_language"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/semanticversion"
	"net/http"
	"os"
)

type ExchangeNodes struct {
	LastIndex int                        `json:"lastIndex"`
	Nodes     map[string]exchange.Device `json:"nodes"`
}

type ContainerStat struct {
	Name    string `json:"name"`
	Image   string `json:"image"`
	Created int    `json:"created"`
	State   string `json:"state"`
}

type ExNodeStatusService struct {
	AgreementId     string          `json:"agreementId"`
	ServiceUrl      string          `json:"serviceUrl"`
	OrgId           string          `json:"orgid"`
	Version         string          `json:"version"`
	Arch            string          `json:"arch"`
	ContainerStatus []ContainerStat `json:"containerStatus"`
	OperatorStatus  interface{}     `json:"operatorStatus,omitempty"`
}

type ExchangeNodeStatus struct {
	Connectivity    map[string]bool       `json:"connectivity,omitempty"`
	Services        []ExNodeStatusService `json:"services"`
	RunningServices string                `json:"runningServices"`
	LastUpdated     string                `json:"lastUpdated,omitempty"`
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

// Create a node with the given information.
// The function will check if the node exists. If it exists, then only the token will be updated.
// If checkNode is false, it means that the node existance has been check somewhere else, the function will
// assume the node does not exist.
func NodeCreate(org, nodeIdTok, node, token, userPw, arch string, nodeName string, nodeType string, checkNode bool) {
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

	// validate the node type
	if nodeType != "" && nodeType != persistence.DEVICE_TYPE_DEVICE && nodeType != persistence.DEVICE_TYPE_CLUSTER {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Wrong node type specified: %v. It must be 'device' or 'cluster'.", nodeType))
	}

	//Check if the node exists or not
	var nodes ExchangeNodes
	nodeExists := false
	if checkNode {
		httpCode := cliutils.ExchangeGet("Exchange", exchUrlBase, "orgs/"+org+"/nodes/"+nodeId, cliutils.OrgAndCreds(org, userPw), []int{200, 404, 401}, &nodes)
		if httpCode == 401 {
			// Invalid creds means the user doesn't exist, or pw is wrong
			user, _ := cliutils.SplitIdToken(userPw)
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("user '%s' does not exist with the specified password.", user))
		} else if httpCode == 200 {
			nodeExists = true
		}
	}

	var httpCode int
	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	}
	if nodeExists {
		msgPrinter.Printf("Node %v exists. Only the node token will be updated.", nodeId)
		msgPrinter.Println()
		if arch != "" || nodeType != "" {
			msgPrinter.Printf("The node arch and node type will be ignored if they are specified.")
			msgPrinter.Println()
		}
		patchNodeReq := NodeExchangePatchToken{Token: nodeToken}
		httpCode = cliutils.ExchangePutPost("Exchange", http.MethodPatch, cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes/"+nodeId, cliutils.OrgAndCreds(org, userPw), []int{201, 401, 403}, patchNodeReq, &resp)
	} else {
		// create the node with given node type
		if nodeType == "" {
			nodeType = persistence.DEVICE_TYPE_DEVICE
		}
		putNodeReq := exchange.PutDeviceRequest{Token: nodeToken, Name: nodeName, NodeType: nodeType, SoftwareVersions: make(map[string]string), PublicKey: []byte(""), Arch: arch}
		httpCode = cliutils.ExchangePutPost("Exchange", http.MethodPut, exchUrlBase, "orgs/"+org+"/nodes/"+nodeId, cliutils.OrgAndCreds(org, userPw), []int{201, 401, 403}, putNodeReq, &resp)
	}

	if httpCode == 401 {
		// Invalid creds means the user doesn't exist, or pw is wrong
		user, _ := cliutils.SplitIdToken(userPw)
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("user '%s' does not exist with the specified password.", user))
	} else if httpCode == 403 {
		// Access denied means either the node exists and is owned by another user or it doesn't exist but user reached the maxNodes threshold.
		var nodesOutput exchange.GetDevicesResponse
		httpCode = cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes/"+nodeId, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &nodesOutput)
		if httpCode == 200 {
			// Node exists. Figure out who is the owner and tell the user
			var ok bool
			var ourNode exchange.Device
			if ourNode, ok = nodesOutput.Devices[cliutils.OrgAndCreds(org, nodeId)]; !ok {
				cliutils.Fatal(cliutils.INTERNAL_ERROR, msgPrinter.Sprintf("key '%s' not found in exchange nodes output", cliutils.OrgAndCreds(org, nodeId)))
			}
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("can not update existing node %s because it is owned by another user (%s)", nodeId, ourNode.Owner))
		} else if httpCode == 404 {
			// Node doesn't exist. MaxNodes reached or 403 means real access denied, display the message from the exchange
			if resp.Msg != "" {
				fmt.Println(resp.Msg)
			}
		}
	} else if httpCode == 201 {
		if nodeExists {
			msgPrinter.Printf("Node %v updated.", nodeId)
			msgPrinter.Println()
		} else {
			if resp.Msg != "" {
				fmt.Println(resp.Msg)
			}
			msgPrinter.Printf("Node %v created.", nodeId)
			msgPrinter.Println()
		}
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
	if err := json.Unmarshal([]byte(attribute), &findPatchType); err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal attribute: %v", err))
	}

	// check invalid attributes
	for k, _ := range findPatchType {
		if k != "userInput" && k != "pattern" && k != "heartbeatIntervals" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Cannot update attribute %v. Supported attributes are: userInput, pattern, heartbeatIntervals.", k))
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
			}
			// validate userInput
			// if node has a pattern, check the service is defined in top level services
			nodes := nodeReq.Nodes
			if len(nodes) != 1 {
				cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Expecting 1 exchange node, but get %d nodes", len(nodes)))
			}

			var node exchange.Device
			nId := ""
			for id, n := range nodes {
				node = n
				nId = id
				break
			}

			// verify node userinput for pattern case
			verifyNodeUserInput(org, credToUse, node, nId, ui)

			// verify node userinput for policy case
			verifyNodeUserInputsForPolicy(org, credToUse, node, ui)

			patch[k] = ui
		} else if k == "pattern" {
			pattern := ""
			if err := json.Unmarshal(bytes, &pattern); err != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal pattern attribute input %s: %v", v, err))
			} else {
				patch[k] = pattern
			}
		} else if k == "heartbeatIntervals" {
			hb := exchange.HeartbeatIntervals{}
			if err := json.Unmarshal(bytes, &hb); err != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal heartbeat input %s: %v", v, err))
			} else {
				patch[k] = hb
			}
		}

		msgPrinter.Printf("Updating %v for node %v/%v in the Horizon Exchange.", k, nodeOrg, node)
		msgPrinter.Println()
		cliutils.ExchangePutPost("Exchange", http.MethodPatch, cliutils.GetExchangeUrl(), "orgs/"+nodeOrg+"/nodes/"+node, cliutils.OrgAndCreds(org, credToUse), []int{200, 201}, patch, nil)
		msgPrinter.Printf("Attribute %v updated.", k)
		msgPrinter.Println()

		updated = true
	}

	// Tell user that the device will re-evaluating the agreements based on the node update, if necessary.
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

			// the exchange node is registered with a device and the update is not heartbeat interval, then give user more info.
			skipReEval := false
			if _, ok := findPatchType["heartbeatIntervals"]; ok && len(findPatchType) == 1 {
				skipReEval = true
			}

			if exchNode.PublicKey != "" && !skipReEval {
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
	cliutils.ExchangePutPost("Exchange", http.MethodPatch, cliutils.GetExchangeUrl(), "orgs/"+nodeOrg+"/nodes/"+node, cliutils.OrgAndCreds(org, credToUse), []int{201}, patchNodeReq, nil)
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

	// display
	output, err := cliutils.DisplayAsJson(policy)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, i18n.GetMessagePrinter().Sprintf("failed to marshal node policy output: %v", err))
	}

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
	err = policyFile.ValidateAndNormalize()
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
	cliutils.ExchangePutPost("Exchange", http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+nodeOrg+"/nodes/"+node+"/policy", cliutils.OrgAndCreds(org, credToUse), []int{201}, policyFile, nil)

	msgPrinter.Printf("Node policy updated.")
	msgPrinter.Println()
}

func NodeUpdatePolicy(org, credToUse, node string, jsonfile string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	msgPrinter.Printf("Warning: This command is deprecated. It will continue to be supported until the next major release. Please use 'hzn exchange node addpolicy' to update the node policy.")
	msgPrinter.Println()

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
		cliutils.ExchangePutPost("Exchange", http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+nodeOrg+"/nodes/"+node+"/policy", cliutils.OrgAndCreds(org, credToUse), []int{200, 201}, newPolicy, nil)
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
		cliutils.ExchangePutPost("Exchange", http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+nodeOrg+"/nodes/"+node+"/policy", cliutils.OrgAndCreds(org, credToUse), []int{200, 201}, newPolicy, nil)
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
	msgPrinter.Printf("Removing Node policy and re-evaluating all agreements. Existing agreements might be cancelled and re-negotiated.")
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

// NodeListStatus list the node run time status, for example service container status.
func NodeListStatus(org string, credToUse string, node string) {
	msgPrinter := i18n.GetMessagePrinter()

	cliutils.SetWhetherUsingApiKey(credToUse)
	var nodeOrg string
	nodeOrg, node = cliutils.TrimOrg(org, node)

	var nodeStatus ExchangeNodeStatus
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+nodeOrg+"/nodes"+cliutils.AddSlash(node)+"/status", cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &nodeStatus)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("node status not found for node '%v/%v'.", nodeOrg, node))
	}

	output := cliutils.MarshalIndent(nodeStatus, "exchange node liststatus")
	fmt.Println(output)

}

// Verify the node user input for the pattern case. Make sure that the given
// user input are compatible with the pattern.
func verifyNodeUserInput(org string, credToUse string, node exchange.Device, nId string, ui []policy.UserInput) {
	msgPrinter := i18n.GetMessagePrinter()

	if node.Pattern == "" {
		return
	}

	msgPrinter.Printf("Verifying userInputs with the node pattern %v.", node.Pattern)
	msgPrinter.Println()

	// get exchange context
	ec := cliutils.GetUserExchangeContext(org, credToUse)

	// compcheck.UserInputCompatible function calls the exchange package that calls glog.
	// set the glog stderrthreshold to 3 (fatal) in order for glog error messages not showing up in the output
	flag.Set("stderrthreshold", "3")
	flag.Parse()

	uiCheckInput := compcheck.UserInputCheck{}
	uiCheckInput.NodeArch = node.Arch
	uiCheckInput.PatternId = node.Pattern
	uiCheckInput.NodeId = nId
	uiCheckInput.NodeUserInput = ui

	// now we can call the real code to check if the policies are compatible.
	// the policy validation are done wthin the calling function.
	compOutput, err := compcheck.UserInputCompatible(ec, &uiCheckInput, true, msgPrinter)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, err.Error())
	} else if !compOutput.Compatible {
		msgPrinter.Printf("Error varifying the given user input with the node pattern %v:", node.Pattern)
		msgPrinter.Println()
		if compOutput.Reason != nil {
			for id, reason := range compOutput.Reason {
				if reason != msgPrinter.Sprintf("Compatible") {
					fmt.Printf("  %v: %v\n", id, reason)
				}
			}
		}
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Unable to update the node because of the above error."))
	}
}

// Verify the node user input for the policy case. Make sure that the given
// user input are compatible with the service in exchange
func verifyNodeUserInputsForPolicy(org string, credToUse string, node exchange.Device, ui []policy.UserInput) {
	msgPrinter := i18n.GetMessagePrinter()
	msgPrinter.Printf("Verifying userInputs for node using policy.")
	msgPrinter.Println()

	if len(ui) != 0 {
		all_services := []common.AbstractServiceFile{}
		for _, u := range ui {
			uInput := u

			serviceOrg := uInput.ServiceOrgid
			serviceUrl := uInput.ServiceUrl
			serviceVersionRange := uInput.ServiceVersionRange
			serviceArch := uInput.ServiceArch

			if &uInput.ServiceVersionRange == nil || uInput.ServiceVersionRange == "" {
				serviceVersionRange = "0.0.0"
			}

			msgPrinter.Printf("Start validating userinput %v, %v, %v, %v", serviceOrg, serviceUrl, serviceVersionRange, serviceArch)
			msgPrinter.Println()

			var vExp *semanticversion.Version_Expression
			var err error
			if vExp, err = semanticversion.Version_Expression_Factory(serviceVersionRange); err != nil {
				cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Unable to validate exchange node userInput, error: %v", "", err))
			}
			versionRange := vExp.Get_expression()

			if &uInput.ServiceArch == nil || uInput.ServiceArch == "" {
				serviceArch = node.Arch
			}

			// get exchange context
			ec := cliutils.GetUserExchangeContext(org, credToUse)

			serviceDefResolverHandler := exchange.GetHTTPServiceDefResolverHandler(ec)
			var bpUserInput []policy.UserInput

			svcSpec := compcheck.NewServiceSpec(serviceUrl, serviceOrg, versionRange, serviceArch)
			if validated, message, sDefs, err := compcheck.VerifyUserInputForService(svcSpec, nil, serviceDefResolverHandler, bpUserInput, ui, node.GetNodeType(), msgPrinter); !validated {
				if err != nil && message != "" {
					cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Unable to validate exchange node userInput, message: %v, error: %v", message, err))
				} else if err != nil {
					cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Unable to validate exchange node userInput, error: %v", err))
				} else {
					cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Unable to validate exchange node userInput, message: %v", message))
				}
			} else {
				all_services = append(all_services, sDefs...)

				// check service type and node type compatibility
				if compatible_t, reason_t := compcheck.CheckTypeCompatibility(node.NodeType, sDefs[0], msgPrinter); compatible_t != true {
					cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Unable to validate exchange node userInput: %v", reason_t))
				}
			}

		}

		// check redundant userinput for service
		if err := compcheck.CheckRedundantUserinput(all_services, ui, msgPrinter); err != nil {
			cliutils.Warning(msgPrinter.Sprintf("validate exchange node/userinput %v ", err))
		}
	}
}
