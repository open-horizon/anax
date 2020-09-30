package register

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/apicommon"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	cliexchange "github.com/open-horizon/anax/cli/exchange"
	"github.com/open-horizon/anax/cli/unregister"
	"github.com/open-horizon/anax/common"
	"github.com/open-horizon/anax/compcheck"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/semanticversion"
	"io/ioutil"
	"k8s.io/client-go/rest"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// read the user input file that contains the global attributes and service userinput settings.
// it supports both old and new format.
func ReadUserInputFile(filePath string) *common.UserInputFile {

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// read the file
	newBytes := cliconfig.ReadJsonFileWithLocalConfig(filePath)

	// create UserInputFile object
	if uif, err := common.NewUserInputFileFromJsonBytes(newBytes); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Unable to create UserInputFile object from file %s. %v", filePath, err))
	} else {
		return uif
	}

	return nil
}

// read and verify a node policy file
func ReadAndVerifyPolicFile(jsonFilePath string, nodePol *externalpolicy.ExternalPolicy) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	newBytes := cliconfig.ReadJsonFileWithLocalConfig(jsonFilePath)
	err := json.Unmarshal(newBytes, nodePol)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal json input file %s: %v", jsonFilePath, err))
	}

	//Check the policy file format
	err = nodePol.ValidateAndNormalize()
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Incorrect node policy format in file %s: %v", jsonFilePath, err))
	}
}

// DoIt registers this node to Horizon with a pattern
func DoIt(org, pattern, nodeIdTok, userPw, inputFile string, nodeOrgFromFlag string, patternFromFlag string, nodeName string, nodepolicyFlag string, waitService string, waitOrg string, waitTimeout int) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// check the input
	org, pattern, waitService, waitOrg = verifyRegisterParamters(org, pattern, nodeOrgFromFlag, patternFromFlag, waitService, waitOrg)

	cliutils.SetWhetherUsingApiKey(nodeIdTok) // if we have to use userPw later in NodeCreate(), it will set this appropriately for userPw
	var userInputFileObj *common.UserInputFile
	if inputFile != "" {
		msgPrinter.Printf("Reading input file %s...", inputFile)
		msgPrinter.Println()
		userInputFileObj = ReadUserInputFile(inputFile)
		cliutils.Verbose(msgPrinter.Sprintf("Retrieved user input object from file %v: %v", inputFile, userInputFileObj))
	}

	// read and verify the node policy if it specified
	var nodePol externalpolicy.ExternalPolicy
	if nodepolicyFlag != "" {
		ReadAndVerifyPolicFile(nodepolicyFlag, &nodePol)
	}

	// get the arch from anax
	statusInfo := apicommon.Info{}
	cliutils.HorizonGet("status", []int{200}, &statusInfo, false)
	anaxArch := (*statusInfo.Configuration).Arch

	// Get the exchange url from the anax api and the cli. Display a warning if they do not match.
	exchUrlBase := cliutils.GetExchangeUrl()
	anaxExchUrlBase := strings.TrimSuffix(cliutils.GetExchangeUrlFromAnax(), "/")
	if exchUrlBase != anaxExchUrlBase && exchUrlBase != "" && anaxExchUrlBase != "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("hzn cli is configured with exchange url %s from %s and the horizon agent is configured with exchange url %s from %s. hzn register will not work with mismatched exchange urls.", exchUrlBase, cliutils.GetExchangeUrlLocation(), anaxExchUrlBase, cliutils.GetExchangeUrlLocationFromAnax()))
	} else {
		msgPrinter.Printf("Horizon Exchange base URL: %s", exchUrlBase)
		msgPrinter.Println()
	}

	timeout := 60
	var timeoutStr string
	cliutils.WithDefaultEnvVar(&timeoutStr, "HZN_REGISTER_HTTP_TIMEOUT")
	if timeoutStr != "" {
		if val, err := strconv.Atoi(timeoutStr); err != nil {
			timeout = val
		} else {
			msgPrinter.Printf("Failed to read value for HZN_REGISTER_HTTP_TIMEOUT from config file. Continuing with default value.")
			msgPrinter.Println()
		}
	}

	// Get node info from anax
	horDevice := api.HorizonDevice{}
	cliutils.HorizonGet("node", []int{200}, &horDevice, false)

	// exit if the node is already registered
	if horDevice.Config != nil && horDevice.Config.State != nil && (*horDevice.Config.State != persistence.CONFIGSTATE_UNCONFIGURED) {
		cliutils.Fatal(cliutils.HTTP_ERROR, msgPrinter.Sprintf("this Horizon node is already registered or in the process of being registered. If you want to register it differently, run 'hzn unregister' first."))
	}

	// Default node id and token if necessary
	nodeId, nodeToken := cliutils.SplitIdToken(nodeIdTok)
	if nodeId == "" {
		// Get the id from anax
		if horDevice.Id == nil {
			cliutils.Fatal(cliutils.ANAX_NOT_CONFIGURED_YET, msgPrinter.Sprintf("Failed to get proper response from the Horizon agent"))
		}
		nodeId = *horDevice.Id

		if nodeId == "" {
			// Generate a node id using the machine's serial number, if available.
			var msErr error
			nodeId, msErr = cutil.GetMachineSerial("")
			if msErr != nil {
				cliutils.Verbose(msgPrinter.Sprintf("Unable to read machine serial number, error: %v. Continuing device registration.", msErr))
			}
			if nodeId != "" {
				msgPrinter.Printf("Node ID not specified, using machine serial number %v as node ID.", nodeId)
				msgPrinter.Println()
			} else {
				cliutils.Verbose(msgPrinter.Sprintf("Node ID not specified, and machine serial number not found, generating random node ID."))

				// Generate a random string of 40 characters, consisting of numbers and letters.
				var err error
				if nodeId, err = cutil.GenerateRandomNodeId(); err != nil {
					cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Unable to generate random node id, error: %v", err))
				} else {
					msgPrinter.Printf("Generated random node ID: %v.", nodeId)
					msgPrinter.Println()
				}
			}

		} else {
			msgPrinter.Printf("Using node ID '%s' from the Horizon agent", nodeId)
			msgPrinter.Println()
		}
	} else {
		// trim the org off the node id. the HZN_EXCHANGE_NODE_AUTH may contain the org id.
		_, nodeId = cliutils.TrimOrg(org, nodeId)
	}
	if nodeToken == "" {
		// Create a random token
		var err error
		nodeToken, err = cutil.SecureRandomString()
		if err != nil {
			cliutils.Fatal(cliutils.INTERNAL_ERROR, msgPrinter.Sprintf("could not create a random token"))
		}
		msgPrinter.Printf("Generated random node token")
		msgPrinter.Println()
	}
	nodeIdTok = nodeId + ":" + nodeToken

	if nodeName == "" {
		nodeName = nodeId
	}

	// validate the node type
	nodeType := persistence.DEVICE_TYPE_DEVICE
	_, err1 := rest.InClusterConfig()
	if err1 == nil {
		nodeType = persistence.DEVICE_TYPE_CLUSTER
	}

	// See if the node exists in the exchange, and create if it doesn't
	var devicesResp exchange.GetDevicesResponse
	exchangePattern := ""
	httpCode := cliutils.ExchangeGet("Exchange", exchUrlBase, "orgs/"+org+"/nodes/"+nodeId, cliutils.OrgAndCreds(org, nodeIdTok), nil, &devicesResp)

	if httpCode != 200 {
		if userPw == "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("node '%s/%s' does not exist in the Exchange with the specified token, and the -u flag was not specified to provide exchange user credentials to create/update it.", org, nodeId))
		}

		cliutils.SetWhetherUsingApiKey(userPw)
		userOrg, userAuth := cliutils.TrimOrg(org, userPw)
		httpCode1 := cliutils.ExchangeGet("Exchange", exchUrlBase, "orgs/"+org+"/nodes/"+nodeId, cliutils.OrgAndCreds(userOrg, userAuth), nil, &devicesResp)
		if httpCode1 != 200 {
			// node does not exist, create it
			msgPrinter.Printf("Node %s/%s does not exist in the Exchange with the specified token, creating/updating it...", org, nodeId)
			msgPrinter.Println()
			cliexchange.NodeCreate(org, "", nodeId, nodeToken, userPw, anaxArch, nodeName, nodeType, false)
		} else {
			// node exists but the token is new, update the node token
			msgPrinter.Printf("Updating node token...")
			msgPrinter.Println()
			patchNodeReq := cliexchange.NodeExchangePatchToken{Token: nodeToken}
			cliutils.ExchangePutPost("Exchange", http.MethodPatch, cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes/"+nodeId, cliutils.OrgAndCreds(userOrg, userAuth), []int{201}, patchNodeReq, nil)
			for nId, n := range devicesResp.Devices {
				exchangePattern = n.Pattern

				// check if the node type matches. The node type from the exchange will never be empty, the exchange returns 'device' if empty.
				if nodeType != n.GetNodeType() {
					cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Node type mismatch. The node type '%v' does not match the node type '%v' of the Exchange node %v.", nodeType, n.GetNodeType(), nId))
				}
				break
			}
		}
	} else {
		msgPrinter.Printf("Node %s/%s exists in the Exchange", org, nodeId)
		msgPrinter.Println()
		for nId, n := range devicesResp.Devices {
			exchangePattern = n.Pattern

			// check if the node type matches. The node type from the exchange will never be empty, the exchange returns 'device' if empty.
			if nodeType != n.GetNodeType() {
				cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Node type mismatch. The node type '%v' does not match the node type '%v' of the Exchange node %v.", nodeType, n.GetNodeType(), nId))
			}
			break
		}
	}

	checkPattern := false
	// Use the exchange node pattern if any
	if pattern == "" {
		if exchangePattern == "" {
			if nodepolicyFlag == "" {
				msgPrinter.Printf("No pattern or node policy is specified. Will proceeed with the existing node policy.")
				msgPrinter.Println()
			} else {
				msgPrinter.Printf("Will proceeed with the given node policy.")
				msgPrinter.Println()
			}
		} else {
			msgPrinter.Printf("Pattern %s defined for the node on the Exchange.", exchangePattern)
			msgPrinter.Println()
			pattern = exchangePattern
			checkPattern = true
		}
	} else {
		if exchangePattern != "" && cliutils.AddOrg(org, pattern) != cliutils.AddOrg(org, exchangePattern) {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Cannot proceed with the given pattern %s because it is different from the pattern %s defined for the node in the Exchange.\nTo correct the problem, please do one of the following: \n\t- Remove the node from the Exchange \n\t- Remove the pattern from the node in the Exchange \n\t- Register without a pattern (the pattern defined on the node in the Exchange will be used)", pattern, exchangePattern))
		} else {
			checkPattern = true
		}
	}

	var pat exchange.Pattern
	if checkPattern {
		var output exchange.GetPatternResponse
		var patorg, patname string
		patorg, patname = cliutils.TrimOrg(org, pattern)
		httpCode := cliutils.ExchangeGet("Exchange", exchUrlBase, "orgs/"+patorg+"/patterns"+cliutils.AddSlash(patname), cliutils.OrgAndCreds(org, nodeIdTok), []int{200, 404, 405}, &output)
		if httpCode != 200 {
			cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("pattern '%s/%s' not found from the Exchange.", patorg, patname))
		}
		pat = output.Patterns[patorg+"/"+patname]
		if len(pat.Services) == 0 {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Cannot proceed with the given pattern %s because it does not include any services.", pattern))
		} else {
			msgPrinter.Printf("Will proceeed with the given pattern %s.", pattern)
			msgPrinter.Println()
		}
	}

	// Update node policy if specified
	if nodepolicyFlag != "" {
		msgPrinter.Printf("Updating the node policy...")
		msgPrinter.Println()
		cliutils.ExchangePutPost("Exchange", http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes/"+nodeId+"/policy", cliutils.OrgAndCreds(org, nodeIdTok), []int{201}, nodePol, nil)
	}

	// Initialize the Horizon device (node)
	msgPrinter.Printf("Initializing the Horizon node with node type '%v'...", nodeType)
	msgPrinter.Println()
	//nd := Node{Id: nodeId, Token: nodeToken, Org: org, Pattern: pattern, Name: nodeId, HA: false}
	falseVal := false
	nd := api.HorizonDevice{Id: &nodeId, Token: &nodeToken, Org: &org, Pattern: &pattern, Name: &nodeName, NodeType: &nodeType, HA: &falseVal} //todo: support HA config

	err := CreateNode(nd, timeout)
	if err != nil {
		msgPrinter.Printf("Error initializing the node: %v", err)
		msgPrinter.Println()
		RegistrationFailure()
	}

	// Process the input file and call /attribute to set the specified variables
	if userInputFileObj != nil {
		if !userInputFileObj.IsGlobalsEmpty() {
			// Set the global variables as attributes with no url (or in the case of HTTPSBasicAuthAttributes, with url equal to image svr)
			msgPrinter.Printf("Setting global variables...")
			msgPrinter.Println()
			attr := api.NewAttribute("", "Global variables", false, false, map[string]interface{}{}) // we reuse this for each GlobalSet
			for _, g := range userInputFileObj.GetGlobal() {
				attr.Type = &g.Type
				attr.ServiceSpecs = &g.ServiceSpecs
				attr.Mappings = &g.Variables

				// set HostOnly to true for these 2 types
				switch g.Type {
				case "HTTPSBasicAuthAttributes", "DockerRegistryAuthAttributes":
					host_only := true
					attr.HostOnly = &host_only
				}
				//cliutils.HorizonPutPost(http.MethodPost, "attribute", []int{201, 200}, attr)
				err := SetUserInput(timeout, "attribute", attr)
				if err != nil {
					msgPrinter.Printf("Error setting user input variables: %v", err)
					msgPrinter.Println()
					RegistrationFailure()
				}
			}
		}

		// Set the service variables using new format
		newUserInputs, _ := userInputFileObj.GetNewFormat(true)
		if newUserInputs != nil && len(newUserInputs) > 0 {
			// use policy.UserInput struct
			err := SetUserInput(timeout, "node/userinput", newUserInputs)
			if err != nil {
				msgPrinter.Printf("Error setting user input variables: %v", err)
				msgPrinter.Println()
				RegistrationFailure()
			}
		}
	}

	if inputFile == "" {
		// Technically an input file is not required, but it is not the common case, so warn them
		msgPrinter.Printf("Note: no input file was specified. This is only valid if none of the services need variables set.")
		msgPrinter.Println()
		msgPrinter.Printf("However, if there is 'userInput' specified in the node already in the Exchange, the userInput will be used.")
		msgPrinter.Println()
	}

	// Set the pattern and register the node
	msgPrinter.Printf("Changing Horizon state to configured to register this node with Horizon...")
	msgPrinter.Println()
	err = SetConfigState(timeout, inputFile)
	if err != nil {
		msgPrinter.Printf("Error setting node state to configured: %v", err)
		msgPrinter.Println()
		RegistrationFailure()
	}

	// Now drop into the long wait for a service to get started on the node.
	if waitService != "" {
		if nodeType == persistence.DEVICE_TYPE_CLUSTER {
			msgPrinter.Printf("NOTE: The -s flag is currently not supported for nodeType: %s. Node registration will complete without waiting for the service to start.", nodeType)
			msgPrinter.Println()
			msgPrinter.Printf("Horizon node is registered. Workload agreement negotiation should begin shortly. Run 'hzn agreement list' to view.")
			msgPrinter.Println()
		} else {
			msgPrinter.Printf("Horizon node is registered. Workload services should begin executing shortly.")
			msgPrinter.Println()

			// Wait for the service to be started.
			WaitForService(waitOrg, waitService, waitTimeout, pattern, pat, nodeType, anaxArch, org, nodeIdTok)
		}
	} else {
		msgPrinter.Printf("Horizon node is registered. Workload agreement negotiation should begin shortly. Run 'hzn agreement list' to view.")
		msgPrinter.Println()
	}

}

// RegistrationFailure attempts to unregister the node if a critical error is encountered during registration.
// This function will not return. It ends with a call to cliutils.Fatal
func RegistrationFailure() {
	msgPrinter := i18n.GetMessagePrinter()

	msgPrinter.Printf("Critical error encountered in registration. Attempting to undo registration steps to leave node in the unregistered state.")
	msgPrinter.Println("")

	retries := 3
	for i := 0; i < retries; i++ {
		unregister.DeleteHorizonNode(false, false, 15)
		err := unregister.CheckNodeConfigState(15)
		if err == nil {
			break
		}
		if i < retries-1 {
			msgPrinter.Printf("Error unregistering node. Retrying.")
			msgPrinter.Println()
			time.Sleep(time.Second * 5)
		} else {
			msgPrinter.Printf("Failed to unregister node. Attempting a deep clean of the node.")
			msgPrinter.Println()
			unregister.DeleteHorizonNode(false, true, 15)
			err = unregister.CheckNodeConfigState(15)
			if err != nil {
				cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Failed to deep clean the node. %v", err))
			}
		}
	}

	cliutils.Fatal(cliutils.INTERNAL_ERROR, msgPrinter.Sprintf("Registration failed. Node successfully returned to unregistered state."))
}

// CreateNode will create the node locally during registration.
// Timeout is the seconds to wait before returning an error if the call does not return
func CreateNode(nodeDevice api.HorizonDevice, timeout int) error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	c := make(chan string, 1)
	go func() {
		httpCode, body, err := cliutils.HorizonPutPost(http.MethodPost, "node", []int{}, nodeDevice, false)
		if err != nil {
			c <- err.Error()
		} else if httpCode != 200 && httpCode != 201 && body != "" {
			c <- fmt.Sprintf("%v", body)
		}
		c <- fmt.Sprintf("%d", httpCode)
	}()

	channelWait := 15
	totalWait := timeout

	for {
		select {
		case httpReturn := <-c:
			if httpCode, err := strconv.Atoi(httpReturn); err == nil {
				if httpCode == cliutils.ANAX_ALREADY_CONFIGURED {
					cliutils.Fatal(cliutils.HTTP_ERROR, msgPrinter.Sprintf("this Horizon node is already registered or in the process of being registered. If you want to register it differently, run 'hzn unregister' first."))
				} else if httpCode != 200 && httpCode != 201 {
					return fmt.Errorf("Bad HTTP code %d returned from node.", httpCode)
				} else {
					return nil
				}
			} else {
				return fmt.Errorf(httpReturn)
			}
		case <-time.After(time.Duration(timeout) * time.Second):
			totalWait = totalWait - channelWait
			if totalWait <= 0 {
				return fmt.Errorf(msgPrinter.Sprintf("Call to anax to create node timed out."))
			}
		}
	}
}

// SetUserInput sets the given user inputs locally on the node.
// The timeout is the seconds to wait before returning an error if the call does not return.
func SetUserInput(timeout int, resource string, value interface{}) error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	c := make(chan string, 1)
	go func() {
		_, _, err := cliutils.HorizonPutPost(http.MethodPost, resource, []int{200, 201}, value, false)
		if err != nil {
			c <- err.Error()
		} else {
			c <- "done"
		}
	}()

	channelWait := 15
	totalWait := timeout
	for {
		select {
		case httpReturn := <-c:
			if httpReturn == "done" {
				return nil
			} else {
				return fmt.Errorf(httpReturn)
			}
		case <-time.After(time.Duration(channelWait) * time.Second):
			totalWait = totalWait - channelWait
			if totalWait <= 0 {
				return fmt.Errorf(msgPrinter.Sprintf("Call to %s timed out.", resource))
			}
		}
	}
}

// SetServiceConfig sets the service variables provided at registration locally in the node.
// timeout parameter is the time in seconds to wait before returning an error if the call does not return.
func SetServiceConfig(timeout int, inputFile string, value interface{}) (error, int) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	c := make(chan string, 1)
	go func() {
		httpCode, respBody, err := cliutils.HorizonPutPost(http.MethodPost, "service/config", []int{200, 201, 400}, value, false)
		if err != nil {
			c <- err.Error()
		}
		if httpCode == 400 {
			if matches := parseRegisterInputError(respBody); matches != nil && len(matches) > 2 {
				c <- msgPrinter.Sprintf("Registration failed because %v Please update the services section in the input file %v. Run 'hzn unregister' and then 'hzn register...' again", matches[0], inputFile)
			}
			c <- msgPrinter.Sprintf("Error setting service variables from user input file: %v", respBody)
		}
		c <- "done"
	}()

	channelWait := 15
	totalWait := timeout

	for {
		select {
		case output := <-c:
			if output == "done" {
				return nil, 0
			}
			return fmt.Errorf(output), cliutils.CLI_INPUT_ERROR
		case <-time.After(time.Duration(channelWait) * time.Second):
			totalWait = totalWait - channelWait
			if totalWait <= 0 {
				return fmt.Errorf(msgPrinter.Sprintf("Call to set service config resource timed out.")), cliutils.INTERNAL_ERROR
			}
		}
	}
}

// SetConfigState changes the node config state to configured.
// timeout parameter is the time in seconds to wait before returning an error if the call does not return
func SetConfigState(timeout int, inputFile string) error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	c := make(chan string, 1)
	go func() {
		configuredStr := "configured"
		configState := api.Configstate{State: &configuredStr}
		httpCode, respBody, err := cliutils.HorizonPutPost(http.MethodPut, "node/configstate", []int{201, 200, 400}, configState, false)
		if err != nil {
			c <- err.Error()
		}
		if matches := parseRegisterInputError(respBody); matches != nil && len(matches) > 2 && httpCode == 400 {
			err_string := fmt.Sprintf("Registration failed because %v", matches[0])
			if inputFile != "" {
				c <- msgPrinter.Sprintf("%v. Please define variables for service %v in the input file %v. Run 'hzn unregister' and then 'hzn register...' again", err_string, matches[2], inputFile)
			} else {
				c <- msgPrinter.Sprintf("%v. Please create an input file, define variables for service %v. Run 'hzn unregister' and then 'hzn register...' again with the -f flag to specify the input file.", err_string, matches[2])
			}
		} else if httpCode == 400 {
			c <- respBody
		} else {
			c <- "done"
		}
	}()

	channelWait := 15
	totalWait := timeout

	for {
		select {
		case output := <-c:
			if output == "done" {
				cliutils.Verbose("Call to node to change state to configured executed successfully.")
				return nil
			} else {
				return fmt.Errorf("%v", output)
			}

		case <-time.After(time.Duration(channelWait) * time.Second):
			totalWait = totalWait - channelWait
			if totalWait <= 0 {
				cliutils.Verbose("Timeout on the call to update node config state. Checking if it is updated.")
				state := api.Configstate{}
				cliutils.HorizonGet("node/configstate", []int{200, 201}, &state, true)
				if *state.State == "unconfigured" {
					cliutils.Verbose("Node state is unconfigured.")
					return nil
				}
				return fmt.Errorf("Timeout waiting for node config state call to return.")
			}
			cliutils.Verbose(msgPrinter.Sprintf("Waiting for node config state update call to return. %d seconds until timeout.", totalWait))
		}
	}
}

func verifyRegisterParamters(org, pattern, nodeOrgFromFlag string, patternFromFlag string, waitService string, waitOrg string) (string, string, string, string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if nodeOrgFromFlag != "" || patternFromFlag != "" {
		if org != "" || pattern != "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("-o and -p are mutually exclusive with <nodeorg> and <pattern> arguments."))
		} else {
			org = nodeOrgFromFlag
			pattern = patternFromFlag
		}
	}

	// get default org if needed
	if org == "" {
		org = os.Getenv("HZN_ORG_ID")
	}

	if org == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Please specify the node organization id."))
	}

	if waitService == "" && waitOrg != "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("-s must be specified if --serviceorg is specified"))
	}

	// if waitOrg is omitted, default to '*'
	if waitOrg == "" {
		waitOrg = "*"
	}

	// for the policy case, waitService = '*' is not supported
	if waitService == "*" {
		if pattern == "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("When registering with a node policy, '*' is not a valid value for -s."))
		} else if waitOrg != "*" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("When registering with a pattern, if -s is '*' (i.e. all the top-level services in the pattern will be monitored), --serviceorg must be omitted."))
		}
	}

	return org, pattern, waitService, waitOrg
}

// isWithinRanges returns true if version is within at least 1 of the ranges in versionRanges
func isWithinRanges(version string, versionRanges []string) bool {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	for _, vr := range versionRanges {
		vRange, err := semanticversion.Version_Expression_Factory(vr)
		if err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("invalid version range '%s': %v", vr, err))
		}
		if inRange, err := vRange.Is_within_range(version); err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("unable to verify that %v is within %v, error %v", version, vRange, err))
		} else if inRange {
			return true
		}
	}
	return false // was not within any of the ranges
}

// GetHighestService queries the exchange for all versions of this service and returns the highest version that is within at least 1 of the version ranges
func GetHighestService(nodeCreds, org, url, arch string, versionRanges []string) exchange.ServiceDefinition {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	route := "orgs/" + org + "/services?url=" + url + "&arch=" + arch // get all services of this org, url, and arch
	var svcOutput exchange.GetServicesResponse
	cliutils.SetWhetherUsingApiKey(nodeCreds)
	cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), route, nodeCreds, []int{200}, &svcOutput)
	if len(svcOutput.Services) == 0 {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("found no services in the Exchange matching: org=%s, url=%s, arch=%s", org, url, arch))
	}

	// Loop thru the returned services and pick out the highest version that is within one of the versionRanges
	highestKey := "" // key to the service def in the map that so far has the highest valid version
	for svcKey, svc := range svcOutput.Services {
		if !isWithinRanges(svc.Version, versionRanges) {
			continue // not within any of the specified version ranges, so ignore it
		}
		if highestKey == "" {
			highestKey = svcKey // 1st svc found that is within the range
			continue
		}
		// else see if this version is higher than the previous highest version
		c, err := semanticversion.CompareVersions(svcOutput.Services[highestKey].Version, svc.Version)
		if err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("error comparing version %v with version %v. %v", svcOutput.Services[highestKey], svc.Version, err))
		} else if c == -1 {
			highestKey = svcKey
		}
	}

	if highestKey == "" {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("found no services in the Exchange matched: org=%s, specRef=%s, version range=%s, arch=%s", org, url, versionRanges, arch))
	}
	return svcOutput.Services[highestKey]
}

func formSvcKey(org, url, arch string) string {
	return org + "_" + url + "_" + arch
}

type SvcMapValue struct {
	Org            string
	URL            string
	Arch           string
	VersionRanges  []string // all the version ranges we find for this service as we descend thru the required services
	HighestVersion string   // filled in when we have to find the highest service to get its required services. Is valid at the end if len(VersionRanges)==1
	UserInputs     []exchange.UserInput
	Privileged     bool
}

// AddAllRequiredSvcs
func AddAllRequiredSvcs(nodeCreds, org, url, arch, versionRange string, allRequiredSvcs map[string]*SvcMapValue) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Get the service from the exchange so we can get its required services
	highestSvc := GetHighestService(nodeCreds, org, url, arch, []string{versionRange})

	nodeOrg, nodeIdTok := cliutils.TrimOrg(org, nodeCreds)
	nodeId, _ := cliutils.SplitIdToken(nodeIdTok)

	var nodes exchange.GetDevicesResponse
	cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+nodeOrg+"/nodes/"+nodeId, nodeCreds, []int{200}, &nodes)

	// Make sure that the node type and the service type match
	serviceType := highestSvc.GetServiceType()
	for nId, n := range nodes.Devices {
		if serviceType != exchange.SERVICE_TYPE_BOTH && n.GetNodeType() != serviceType {
			msgPrinter.Printf("Ignoring version %s of service %s/%s with node type mismatch: the service type '%v' does not match the node type '%v' of the Exchange node %v.", versionRange, org, url, serviceType, n.GetNodeType(), nId)
			msgPrinter.Println()
			return
		}
	}

	// Add this service to the service map
	cliutils.Verbose(msgPrinter.Sprintf("found: %s, %s, %s, %s", org, url, arch, versionRange))
	svcKey := formSvcKey(org, url, arch)
	if s, ok := allRequiredSvcs[svcKey]; ok {
		// To protect against circular service references, check if we've already seen this exact svc version range
		for _, v := range s.VersionRanges {
			if v == versionRange {
				return
			}
		}
	} else {
		allRequiredSvcs[svcKey] = &SvcMapValue{Org: org, URL: url, Arch: arch} // this must be a ptr to the struct or go won't let us modify it in the map
	}
	allRequiredSvcs[svcKey].VersionRanges = append(allRequiredSvcs[svcKey].VersionRanges, versionRange) // add this version to this service in our map
	allRequiredSvcs[svcKey].HighestVersion = highestSvc.Version                                         // in case we don't encounter this service again, we already know the highest version for getting the user input from
	allRequiredSvcs[svcKey].UserInputs = highestSvc.UserInputs

	// check if the service contains privileged
	if priv, err := compcheck.DeploymentRequiresPrivilege(highestSvc.Deployment, msgPrinter); err != nil {
		cliutils.Fatal(cliutils.INTERNAL_ERROR, msgPrinter.Sprintf("Failed to check if deployment requires privileged: %v", err))
	} else if priv {
		allRequiredSvcs[svcKey].Privileged = true
	}

	// Loop thru this service's required services, adding them to our map
	for _, s := range highestSvc.RequiredServices {
		// This will add this svc to our map and keep descending down the required services
		AddAllRequiredSvcs(nodeCreds, s.Org, s.URL, s.Arch, s.VersionRange, allRequiredSvcs)
	}
}

// CreateInputFile runs thru the services used by this pattern (descending into all required services) and collects the user input needed
func CreateInputFile(nodeOrg, pattern, arch, nodeIdTok, inputFile string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	var patOrg string
	patOrg, pattern = cliutils.TrimOrg(nodeOrg, pattern) // patOrg will either get the prefix from pattern, or default to nodeOrg
	nodeCreds := cliutils.OrgAndCreds(nodeOrg, nodeIdTok)

	// Get the pattern
	var patOutput exchange.GetPatternResponse
	cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+patOrg+"/patterns/"+pattern, nodeCreds, []int{200}, &patOutput)
	patKey := cliutils.OrgAndCreds(patOrg, pattern)
	if _, ok := patOutput.Patterns[patKey]; !ok {
		cliutils.Fatal(cliutils.INTERNAL_ERROR, msgPrinter.Sprintf("did not find pattern '%s' as expected", patKey))
	}
	if arch == "" {
		arch = cutil.ArchString()
	}

	// Recursively go thru the services and their required services, collecting them in a map.
	// Afterward we will process them to figure out the highest version of each before getting their input.
	allRequiredSvcs := make(map[string]*SvcMapValue) // the key is the combined org, url, arch. The value is the org, url, arch and a list of the versions.
	for _, svc := range patOutput.Patterns[patKey].Services {
		if svc.ServiceArch != arch { // filter out services that are not our arch
			msgPrinter.Printf("Ignoring service that is a different architecture: %s, %s, %s", svc.ServiceOrg, svc.ServiceURL, svc.ServiceArch)
			msgPrinter.Println()
			continue
		}

		for _, svcVersion := range svc.ServiceVersions {
			// This will add this svc to our map and keep descending down the required services
			AddAllRequiredSvcs(nodeCreds, svc.ServiceOrg, svc.ServiceURL, svc.ServiceArch, svcVersion.Version, allRequiredSvcs) // svcVersion.Version is a version range
		}
	}

	// Loop thru each service, find the highest version of that service, and then record the user input for it
	// Note: if the pattern references multiple versions of the same service (directly or indirectly), we create input for the highest version of the service.
	svcInputs := make([]policy.UserInput, 0, len(allRequiredSvcs))

	containsPrivilegedSvc := false
	for _, s := range allRequiredSvcs {
		var userInput []exchange.UserInput
		if s.HighestVersion != "" && len(s.VersionRanges) <= 1 {
			// When we were finding the required services we only encountered this service once, so the user input we found then is valid
			userInput = s.UserInputs
		} else {
			svc := GetHighestService(nodeCreds, s.Org, s.URL, s.Arch, s.VersionRanges)
			userInput = svc.UserInputs
		}

		// Get the user input from this service
		if len(userInput) > 0 {
			svcInput := policy.UserInput{ServiceOrgid: s.Org, ServiceUrl: s.URL, ServiceVersionRange: "[0.0.0,INFINITY)", Inputs: make([]policy.Input, len(userInput))}
			for i, u := range userInput {
				svcInput.Inputs[i] = policy.Input{Name: u.Name, Value: u.DefaultValue}
			}
			svcInputs = append(svcInputs, svcInput)
		}

		if s.Privileged {
			containsPrivilegedSvc = true
		}
	}

	// Output the file
	jsonBytes, err := json.MarshalIndent(svcInputs, "", cliutils.JSON_INDENT)
	if err != nil {
		cliutils.Fatal(cliutils.INTERNAL_ERROR, msgPrinter.Sprintf("failed to marshal the user input template file: %v", err))
	}

	if err := ioutil.WriteFile(inputFile, jsonBytes, 0644); err != nil {
		cliutils.Fatal(cliutils.FILE_IO_ERROR, msgPrinter.Sprintf("problem writing the user input template file: %v", err))
	}

	msgPrinter.Printf("Wrote %s", inputFile)
	msgPrinter.Println()

	// Create example node policy file if at least one service contains privileged in the deployment string
	if containsPrivilegedSvc {
		allowPrivilegedPolicyExample := externalpolicy.ExternalPolicy{
			Properties: externalpolicy.PropertyList{
				externalpolicy.Property{
					Name:  "openhorizon.allowPrivileged",
					Value: true,
				},
			},
		}
		nodePolicySampleFile := inputFile + "_np.json"
		// Output the file
		jsonBytes, err := json.MarshalIndent(allowPrivilegedPolicyExample, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.INTERNAL_ERROR, msgPrinter.Sprintf("failed to marshal the example node policy file: %v", err))
		}
		if err := ioutil.WriteFile(nodePolicySampleFile, jsonBytes, 0644); err != nil {
			cliutils.Fatal(cliutils.FILE_IO_ERROR, msgPrinter.Sprintf("problem writing the example node policy file: %v", err))
		} else {
			msgPrinter.Printf("One or more of services contain privileged in the deployment string. Make sure your node policy file allows privileged. A sample node policy file has been created: %s", nodePolicySampleFile)
			msgPrinter.Println()
		}
	}
}

// this function parses the error returned by the registration process to see if the error
// is due to the missing input variable or wrong input variable type for a service.
// It returns a string array in the following format:
// [error, variable name, service org/service url]
// if it returns an empty array, then there is no match.
func parseRegisterInputError(resp string) []string {
	// match the ANAX_SVC_MISSING_VARIABLE
	tmplt_miss_var := strings.Replace(cutil.ANAX_SVC_MISSING_VARIABLE, "%v", "([^\\s]*)", -1)
	re1 := regexp.MustCompile(tmplt_miss_var)
	matches := re1.FindStringSubmatch(resp)

	// match ANAX_SVC_WRONG_TYPE
	if matches == nil || len(matches) == 0 {
		tmplt_wrong_type := strings.Replace(cutil.ANAX_SVC_WRONG_TYPE, "%v", "([^\\s]*)", -1) + "type [^.]*[.]"
		re2 := regexp.MustCompile(tmplt_wrong_type)
		matches = re2.FindStringSubmatch(resp)
	}

	// match ANAX_SVC_MISSING_CONFIG
	if matches == nil || len(matches) == 0 {
		tmplt_missing_config := strings.Replace(cutil.ANAX_SVC_MISSING_CONFIG, "%v", "([^\\s]*)", -1)
		re3 := regexp.MustCompile(tmplt_missing_config)
		matches = re3.FindStringSubmatch(resp)
		if matches != nil && len(matches) > 2 {
			// no variable name for this
			matches[1] = ""
		}
	}
	return matches
}
