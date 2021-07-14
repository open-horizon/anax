package deploycheck

import (
	"flag"
	"fmt"
	"github.com/open-horizon/anax/businesspolicy"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/common"
	"github.com/open-horizon/anax/compcheck"
	"github.com/open-horizon/anax/i18n"
	"os"
)

// check if the user inputs for services are compatible
func SecretBindingCompatible(org string, userPw string, nodeId string, nodeArch string, nodeType string, nodeOrg string,
	businessPolId string, businessPolFile string, patternId string, patternFile string,
	svcDefFiles []string, checkAllSvcs bool, showDetail bool) {

	msgPrinter := i18n.GetMessagePrinter()

	// check the input and get the defaults
	userOrg, credToUse, nId, useNodeId, bp, pattern, serviceDefs := verifySecretBindingCompatibleParameters(
		org, userPw, nodeId, nodeArch, nodeType, nodeOrg, businessPolId, businessPolFile, patternId, patternFile, svcDefFiles)

	sbCheckInput := compcheck.SecretBindingCheck{}
	sbCheckInput.NodeArch = nodeArch
	sbCheckInput.NodeType = nodeType
	sbCheckInput.NodeOrg = nodeOrg
	sbCheckInput.BusinessPolicy = bp
	sbCheckInput.PatternId = patternId
	sbCheckInput.Pattern = pattern

	// use the user org for the patternId if it does not include an org id
	if sbCheckInput.PatternId != "" {
		sbCheckInput.PatternId = cliutils.AddOrg(userOrg, sbCheckInput.PatternId)
	}

	if useNodeId {
		// add credentials'org to node id if the node id does not have an org
		nId = cliutils.AddOrg(userOrg, nId)
		sbCheckInput.NodeId = nId
	} else if nodeOrg == "" || nodeType == "" || nodeArch == "" {
		msgPrinter.Printf("Node architecture, organization or type is not specified and node ID is not specified. Getting the node information from the local node.")
		msgPrinter.Println()

		// get id from local node, check arch
		sbCheckInput.NodeId, sbCheckInput.NodeArch, sbCheckInput.NodeType, sbCheckInput.NodeOrg = getLocalNodeInfo(nodeArch, nodeType, nodeOrg)
	}

	// put the given service defs into the sbCheckInput
	if serviceDefs != nil || len(serviceDefs) != 0 {
		sbCheckInput.Service = serviceDefs
	}

	cliutils.Verbose(msgPrinter.Sprintf("Using compatibility checking input: %v", sbCheckInput))

	// get exchange context
	ec := cliutils.GetUserExchangeContext(userOrg, credToUse)
	agbotUrl := cliutils.GetAgbotSecureAPIUrlBase()

	// compcheck.UserInputCompatible function calls the exchange package that calls glog.
	// set the glog stderrthreshold to 3 (fatal) in order for glog error messages not showing up in the output
	flag.Set("stderrthreshold", "3")

	// now we can call the real code to check if the policies are compatible.
	// the policy validation are done wthin the calling function.
	compOutput, err := compcheck.SecretBindingCompatible(ec, agbotUrl, &sbCheckInput, checkAllSvcs, msgPrinter)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, err.Error())
	} else {
		if !showDetail {
			compOutput.Input = nil
		}

		// display the output
		output, err := cliutils.DisplayAsJson(compOutput)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn deploycheck secretbinding' output: %v", err))
		}

		fmt.Println(output)
	}
}

// Make sure -n and (-t, -O, -a) are mutually exclusive,
// -b and -B pairs are are mutually exclusive,
// and -p and -P pairs are mutually exclusive.
// Business policy and pattern are mutually exclusive.
// Get default credential, node id and org if they are not set.
func verifySecretBindingCompatibleParameters(org string, userPw string, nodeId string, nodeArch string, nodeType string, nodeOrg string,
	businessPolId string, businessPolFile string, patternId string, patternFile string,
	svcDefFiles []string) (string, string, string, bool, *businesspolicy.BusinessPolicy, common.AbstractPatternFile, []common.AbstractServiceFile) {

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// make sure the node type has correct value
	ValidateNodeType(nodeType)

	// check if calling to the exchange or use local node
	useNodeId := false
	nodeIdToUse := nodeId
	if nodeArch == "" || nodeOrg == "" || nodeType == "" {
		if nodeId != "" {
			useNodeId = true
		} else {
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

	useSId, serviceDefs := useExchangeForServiceDef(svcDefFiles)

	// if user credential is not given, then use the node auth env HZN_EXCHANGE_NODE_AUTH if it is defined.
	credToUse := cliutils.WithDefaultEnvVar(&userPw, "HZN_EXCHANGE_NODE_AUTH")
	orgToUse := org
	if useNodeId || useBPolId || usePatternId || useSId {
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
		pattern := getPattern(orgToUse, *credToUse, patternId, patternFile)

		// check if the specified the services are the ones that the pattern needs.
		// only check if the given services are valid or not.
		// Not checking the missing ones becaused it will be checked by the compcheck package.
		checkServiceDefsForPattern(pattern, serviceDefs, svcDefFiles)

		return orgToUse, *credToUse, nodeIdToUse, useNodeId, nil, pattern, serviceDefs
	}
}
