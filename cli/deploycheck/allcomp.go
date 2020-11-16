package deploycheck

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/businesspolicy"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/common"
	"github.com/open-horizon/anax/compcheck"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"os"
)

// check if the policies are compatible
func AllCompatible(org string, userPw string, nodeId string, nodeArch string, nodeType string,
	nodePolFile string, nodeUIFile string, businessPolId string, businessPolFile string,
	patternId string, patternFile string, servicePolFile string, svcDefFiles []string,
	checkAllSvcs bool, showDetail bool) {

	msgPrinter := i18n.GetMessagePrinter()

	// check the input and get the defaults
	userOrg, credToUse, nId, useNodeId, bp, pattern, serviceDefs := verifyCompCheckParameters(
		org, userPw, nodeId, nodeType, nodePolFile, nodeUIFile, businessPolId, businessPolFile,
		patternId, patternFile, servicePolFile, svcDefFiles)

	compCheckInput := compcheck.CompCheck{}
	compCheckInput.NodeArch = nodeArch
	compCheckInput.NodeType = nodeType
	compCheckInput.BusinessPolicy = bp
	compCheckInput.PatternId = patternId
	compCheckInput.Pattern = pattern

	// use the user org for the patternId if it does not include an org id
	if compCheckInput.PatternId != "" {
		compCheckInput.PatternId = cliutils.AddOrg(userOrg, compCheckInput.PatternId)
	}

	if useNodeId {
		// add credentials'org to node id if the node id does not have an org
		nId = cliutils.AddOrg(userOrg, nId)
		compCheckInput.NodeId = nId
	}

	// formalize node id or get node policy
	bUseLocalNodeForPolicy := false
	bUseLocalNodeForUI := false
	if bp != nil || pattern != nil || patternId != "" {
		if nodePolFile != "" {
			// read the node policy from file
			var np externalpolicy.ExternalPolicy
			readExternalPolicyFile(nodePolFile, &np)
			compCheckInput.NodePolicy = &np
		} else if !useNodeId {
			bUseLocalNodeForPolicy = true
		}
	}

	if nodeUIFile != "" {
		// read the node userinput from file
		var node_ui []policy.UserInput
		readUserInputFile(nodeUIFile, &node_ui)
		compCheckInput.NodeUserInput = node_ui
	} else if !useNodeId {
		bUseLocalNodeForUI = true
	}

	if bUseLocalNodeForPolicy {
		msgPrinter.Printf("Neither node id nor node policy is specified. Getting node policy from the local node.")
		msgPrinter.Println()

		// get id from local node, check arch
		compCheckInput.NodeId, compCheckInput.NodeArch = getLocalNodeInfo(nodeArch)

		// get node policy from local node
		var np externalpolicy.ExternalPolicy
		cliutils.HorizonGet("node/policy", []int{200}, &np, false)
		compCheckInput.NodePolicy = &np
	}

	if bUseLocalNodeForUI {
		msgPrinter.Printf("Neither node id nor node user input file is specified. Getting node user input from the local node.")
		msgPrinter.Println()
		// get node user input from local node
		var node_ui []policy.UserInput
		cliutils.HorizonGet("node/userinput", []int{200}, &node_ui, false)
		compCheckInput.NodeUserInput = node_ui
	}

	if nodeType == "" && compCheckInput.NodeId != "" {
		cliutils.Verbose(msgPrinter.Sprintf("No node type has been provided: node type of '%v' node will be used", compCheckInput.NodeId))
	}

	// read the service policy from file for the policy case
	if servicePolFile != "" {
		var sp externalpolicy.ExternalPolicy
		readExternalPolicyFile(servicePolFile, &sp)
		compCheckInput.ServicePolicy = &sp
	}

	// put the given service defs into the compCheckInput
	if serviceDefs != nil || len(serviceDefs) != 0 {
		compCheckInput.Service = serviceDefs
	}

	cliutils.Verbose(msgPrinter.Sprintf("Using compatibility checking input: %v", compCheckInput))

	// get exchange context
	ec := cliutils.GetUserExchangeContext(userOrg, credToUse)

	// compcheck.Compatible function calls the exchange package that calls glog.
	// set the glog stderrthreshold to 3 (fatal) in order for glog error messages not showing up in the output
	flag.Set("stderrthreshold", "3")
	flag.Parse()

	// now we can call the real code to check if the policies are compatible.
	// the policy validation are done wthin the calling function.
	compOutput, err := compcheck.DeployCompatible(ec, &compCheckInput, checkAllSvcs, msgPrinter)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, err.Error())
	} else {
		if !showDetail {
			compOutput.Input = nil
		}

		// display the output
		output, err := cliutils.DisplayAsJson(compOutput)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn policy compatible' output: %v", err))
		}

		fmt.Println(output)
	}
}

// Make sure -n and --node-pol, -b and -B pairs
// and -p and -P pairs are mutually exclusive.
// Business policy and pattern are mutually exclusive.
// Get default credential, node id and org if they are not set.
func verifyCompCheckParameters(org string, userPw string, nodeId string, nodeType string, nodePolFile string, nodeUIFile string,
	businessPolId string, businessPolFile string, patternId string, patternFile string, servicePolFile string,
	svcDefFiles []string) (string, string, string, bool, *businesspolicy.BusinessPolicy, *common.PatternFile, []common.ServiceFile) {

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// make sure the node type has correct value
	ValidateNodeType(nodeType)

	// make sure only specify one: business policy or pattern
	useBPol := false
	if businessPolId != "" || businessPolFile != "" {
		useBPol = true
		if patternId != "" || patternFile != "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Please specify either deployment policy or pattern."))
		}
	} else {
		if patternId == "" && patternFile == "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("One of these flags must be specified: -b, -B, -p, or -P."))
		}
	}

	useNodeId := false
	nodeIdToUse := nodeId
	if nodeId != "" {
		if (useBPol && nodePolFile == "") || (!useBPol && nodeUIFile == "") {
			// true means will use exchange call
			useNodeId = true
		}
		if nodePolFile != "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("-n and --node-pol are mutually exclusive."))
		}
		if nodeUIFile != "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("-n and --node-ui are mutually exclusive."))
		}
	} else {
		if (useBPol && nodePolFile == "") || (!useBPol && nodeUIFile == "") {
			// get node id from HZN_EXCHANGE_NODE_AUTH
			if nodeIdTok := os.Getenv("HZN_EXCHANGE_NODE_AUTH"); nodeIdTok != "" {
				nodeIdToUse, _ = cliutils.SplitIdToken(nodeIdTok)
				if nodeIdToUse != "" {
					// true means will use exchange call
					useNodeId = true
				}
			}
		}
	}

	useBPolId := false
	usePatternId := false
	if useBPol {
		if businessPolId != "" {
			if businessPolFile == "" {
				// true means will use exchange call
				useBPolId = true
			} else {
				cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("-b and -B are mutually exclusive."))
			}
		}
	} else {
		if patternId != "" {
			if patternFile == "" {
				// true means will use exchange call
				usePatternId = true
			} else {
				cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("-p and -P are mutually exclusive."))
			}
		}
	}

	useSPolId := false
	if servicePolFile == "" {
		// true means will use exchange call
		useSPolId = true
	}

	useSId, serviceDefs := useExchangeForServiceDef(svcDefFiles)

	// if user credential is not given, then use the node auth env HZN_EXCHANGE_NODE_AUTH if it is defined.
	credToUse := cliutils.WithDefaultEnvVar(&userPw, "HZN_EXCHANGE_NODE_AUTH")
	orgToUse := org
	if useNodeId || useBPolId || useSPolId || usePatternId || useSId {
		if *credToUse == "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Please specify the Exchange credential with -u for querying the node, deployment policy and service policy."))
		} else {
			// get the org from credToUse
			if org == "" {
				id, _ := cliutils.SplitIdToken(*credToUse)
				if id != "" {
					orgToUse, _ = cliutils.TrimOrg("", id)
					if orgToUse == "" {
						cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Please specify the organization with -o for the Exchange credentials: %v.", *credToUse))
					}
				}
			}
		}
	}

	if useBPol {
		// get business policy from file or exchange
		bp := getBusinessPolicy(orgToUse, *credToUse, businessPolId, businessPolFile)
		// the compcheck package does not need to get it again from the exchange.
		useBPolId = false

		// check if the given service files specify correct services.
		// Other parts will be checked later by the compcheck package.
		checkServiceDefsForBPol(bp, serviceDefs, svcDefFiles)

		return orgToUse, *credToUse, nodeIdToUse, useNodeId, bp, nil, serviceDefs
	} else {
		// get pattern from file or exchange
		pattern, pf := getPattern(orgToUse, *credToUse, patternId, patternFile)

		// check if the specified the services are the ones that the pattern needs.
		// only check if the given services are valid or not.
		// Not checking the missing ones becaused it will be checked by the compcheck package.
		checkServiceDefsForPattern(pattern, serviceDefs, svcDefFiles)

		return orgToUse, *credToUse, nodeIdToUse, useNodeId, nil, pf, serviceDefs
	}
}

// get node info and check node arch against the input arch
func getLocalNodeInfo(inputArch string) (string, string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	id := ""
	arch := cutil.ArchString()

	horDevice := api.HorizonDevice{}
	cliutils.HorizonGet("node", []int{200}, &horDevice, false)

	// check node current state
	if horDevice.Config == nil || horDevice.Config.State == nil || *horDevice.Config.State != persistence.CONFIGSTATE_CONFIGURED {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Cannot use the local node because it is not registered."))
	}

	// get node id
	if horDevice.Org != nil && horDevice.Id != nil {
		id = cliutils.AddOrg(*horDevice.Org, *horDevice.Id)
	}

	// get/check node architecture
	if inputArch != "" {
		if inputArch != arch {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("The node architecture %v specified by -a does not match the architecture of the local node %v.", inputArch, arch))
		}
	}

	return id, arch
}

// get business policy from exchange or from file.
func getBusinessPolicy(defaultOrg string, credToUse string, businessPolId string, businessPolFile string) *businesspolicy.BusinessPolicy {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	var bp businesspolicy.BusinessPolicy
	if businessPolFile != "" {
		// get business policy from file
		newBytes := cliconfig.ReadJsonFileWithLocalConfig(businessPolFile)
		if err := json.Unmarshal(newBytes, &bp); err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal deployment policy json input file %s: %v", businessPolFile, err))
		}
	} else {
		// get business policy from the exchange
		var policyList exchange.GetBusinessPolicyResponse
		polOrg, polId := cliutils.TrimOrg(defaultOrg, businessPolId)

		httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+polOrg+"/business/policies"+cliutils.AddSlash(polId), cliutils.OrgAndCreds(defaultOrg, credToUse), []int{200, 404}, &policyList)
		if httpCode == 404 || policyList.BusinessPolicy == nil || len(policyList.BusinessPolicy) == 0 {
			cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("Deployment policy not found for %v/%v", polOrg, polId))
		} else {
			for _, exchPol := range policyList.BusinessPolicy {
				bp = exchPol.GetBusinessPolicy()
				break
			}
		}
	}
	return &bp
}

// make sure that the node type has correct value.
func ValidateNodeType(nodeType string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if nodeType != "" && nodeType != persistence.DEVICE_TYPE_DEVICE && nodeType != persistence.DEVICE_TYPE_CLUSTER {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Wrong node type specified: %v. It must be 'device' or 'cluster'.", nodeType))
	}

}
