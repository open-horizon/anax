package deploycheck

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/compcheck"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/i18n"
	"os"
)

func readExternalPolicyFile(filePath string, inputFileStruct *externalpolicy.ExternalPolicy) {
	newBytes := cliconfig.ReadJsonFileWithLocalConfig(filePath)
	err := json.Unmarshal(newBytes, inputFileStruct)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, i18n.GetMessagePrinter().Sprintf("failed to unmarshal json input file %s: %v", filePath, err))
	}
}

// check if the policies are compatible
func PolicyCompatible(org string, userPw string, nodeId string, nodeArch string, nodePolFile string, businessPolId string, businessPolFile string, servicePolFile string, checkAllSvcs bool, showDetail bool) {

	msgPrinter := i18n.GetMessagePrinter()

	// check the input and get the defaults
	userOrg, credToUse, nId, useNodeId := verifyPolicyCompatibleParamters(org, userPw, nodeId, nodePolFile, businessPolId, businessPolFile, servicePolFile)

	policyCheckInput := compcheck.PolicyCheck{}
	policyCheckInput.NodeArch = nodeArch

	// formalize node id or get node policy
	bUseLocalNode := false
	if useNodeId {
		// add credentials'org to node id if the node id does not have an org
		nId = cliutils.AddOrg(userOrg, nId)
		policyCheckInput.NodeId = nId
	} else if nodePolFile != "" {
		// read the node policy from file
		var np externalpolicy.ExternalPolicy
		readExternalPolicyFile(nodePolFile, &np)
		policyCheckInput.NodePolicy = &np
	} else {
		msgPrinter.Printf("Neither node id nor node policy is specified. Getting node policy from the local node.")
		msgPrinter.Println()
		bUseLocalNode = true
	}

	if bUseLocalNode {
		// get id from local node, check arch
		policyCheckInput.NodeId, policyCheckInput.NodeArch = getLocalNodeInfo(nodeArch)

		// get node policy from local node
		var np externalpolicy.ExternalPolicy
		cliutils.HorizonGet("node/policy", []int{200}, &np, false)
		policyCheckInput.NodePolicy = &np
	}

	// get business policy
	bp := getBusinessPolicy(userOrg, credToUse, businessPolId, businessPolFile)
	policyCheckInput.BusinessPolicy = bp

	if servicePolFile != "" {
		// read the service policy from file
		var sp externalpolicy.ExternalPolicy
		readExternalPolicyFile(servicePolFile, &sp)

		policyCheckInput.ServicePolicy = &sp
	}

	cliutils.Verbose(msgPrinter.Sprintf("Using compatibility checking input: %v", policyCheckInput))

	// get exchange context
	ec := cliutils.GetUserExchangeContext(userOrg, credToUse)

	// compcheck.PolicyCompatible function calls the exchange package that calls glog.
	// set the glog stderrthreshold to 3 (fatal) in order for glog error messages not showing up in the output
	flag.Set("stderrthreshold", "3")
	flag.Parse()

	// now we can call the real code to check if the policies are compatible.
	// the policy validation are done wthin the calling function.
	compOutput, err := compcheck.PolicyCompatible(ec, &policyCheckInput, checkAllSvcs, msgPrinter)
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

// make sure -n and --node-pol, -b and -B, pairs are mutually compatible.
// get default credential, node id and org if they are not set.
func verifyPolicyCompatibleParamters(org string, userPw string, nodeId string, nodePolFile string,
	businessPolId string, businessPolFile string, servicePolFile string) (string, string, string, bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	useNodeId := false
	nodeIdToUse := nodeId
	if nodeId != "" {
		if nodePolFile == "" {
			// true means will use exchange call
			useNodeId = true
		} else {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("-n and --node-pol are mutually exclusive."))
		}
	} else {
		if nodePolFile == "" {
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
	if businessPolId != "" {
		if businessPolFile == "" {
			// true means will use exchange call
			useBPolId = true
		} else {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("-b and -B are mutually exclusive."))
		}
	} else {
		if businessPolFile == "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Either -b or -B must be specified."))
		}
	}

	useSPolId := false
	if servicePolFile == "" {
		// true means will use exchange call
		useSPolId = true
	}

	// if user credential is not given, then use the node auth env HZN_EXCHANGE_NODE_AUTH if it is defined.
	credToUse := cliutils.WithDefaultEnvVar(&userPw, "HZN_EXCHANGE_NODE_AUTH")
	orgToUse := org
	if useNodeId || useBPolId || useSPolId {
		if *credToUse == "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Please specify the exchange credential with -u for querying the node, deployment policy and service policy."))
		} else {
			// get the org from credToUse
			if org == "" {
				id, _ := cliutils.SplitIdToken(*credToUse)
				if id != "" {
					orgToUse, _ = cliutils.TrimOrg("", id)
					if orgToUse == "" {
						cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Please specify the organization with -o for the exchange credentials: %v.", *credToUse))
					}
				}
			}
		}
	}

	return orgToUse, *credToUse, nodeIdToUse, useNodeId
}
