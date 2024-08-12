package deploycheck

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/common"
	"github.com/open-horizon/anax/compcheck"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
	"os"
)

func readNodePolicyFile(filePath string, inputFileStruct *exchangecommon.NodePolicy) {
	newBytes := cliconfig.ReadJsonFileWithLocalConfig(filePath)
	err := json.Unmarshal(newBytes, inputFileStruct)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, i18n.GetMessagePrinter().Sprintf("failed to unmarshal json input file %s: %v", filePath, err))
	}
}

func readServicePolicyFile(filePath string, inputFileStruct *exchangecommon.ServicePolicy) {
	newBytes := cliconfig.ReadJsonFileWithLocalConfig(filePath)
	err := json.Unmarshal(newBytes, inputFileStruct)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, i18n.GetMessagePrinter().Sprintf("failed to unmarshal json input file %s: %v", filePath, err))
	}
}

// check if the policies are compatible
func PolicyCompatible(org string, userPw string, nodeIds []string, haGroupName string, nodeArch string, nodeType string, nodeNamespace string, nodeIsNamespaceScoped bool, nodePolFile string, businessPolId string, businessPolFile string, servicePolFile string, svcDefFiles []string, checkAllSvcs bool, showDetail bool) {

	msgPrinter := i18n.GetMessagePrinter()

	// check the input and get the defaults
	userOrg, credToUse, nIds, useNodeId, serviceDefs := verifyPolicyCompatibleParameters(org, userPw, nodeIds, haGroupName, nodeType, nodeNamespace, nodePolFile, businessPolId, businessPolFile, servicePolFile, svcDefFiles)

	// get exchange context
	ec := cliutils.GetUserExchangeContext(userOrg, credToUse)

	// get business policy
	bp := getBusinessPolicy(userOrg, credToUse, businessPolId, businessPolFile)

	if serviceDefs != nil || len(serviceDefs) != 0 {
		// check if the given service files specify correct services.
		// Other parts will be checked later by the compcheck package.
		checkServiceDefsForBPol(bp, serviceDefs, svcDefFiles)
	}

	// read the service policy files if provided
	var sp exchangecommon.ServicePolicy
	if servicePolFile != "" {
		// read the service policy from file
		readServicePolicyFile(servicePolFile, &sp)
	}

	if nIds == nil || len(nIds) == 0 {
		// this is the case where the local node will be used, useNodeId is false.
		// add a fake node to make it easier to process
		nIds = []string{"fake_fake_node"}
	}

	// compcheck.PolicyCompatible function calls the exchange package that calls glog.
	// set glog to log to /dev/null so glog errors will not be printed
	flag.Set("log_dir", "/dev/null")

	totalOutput := make(map[string]*compcheck.CompCheckOutput)
	for _, nId := range nIds {
		policyCheckInput := compcheck.PolicyCheck{}
		policyCheckInput.NodeArch = nodeArch
		policyCheckInput.NodeType = nodeType
		policyCheckInput.NodeClusterNS = nodeNamespace
		policyCheckInput.NodeNamespaceScoped = nodeIsNamespaceScoped
		policyCheckInput.BusinessPolicy = bp

		// formalize node id or get node policy
		bUseLocalNode := false
		if useNodeId {
			// add credentials'org to node id if the node id does not have an org
			nId = cliutils.AddOrg(userOrg, nId)
			policyCheckInput.NodeId = nId
		} else if nodePolFile != "" {
			// read the node policy from file
			var np exchangecommon.NodePolicy
			readNodePolicyFile(nodePolFile, &np)
			policyCheckInput.NodePolicy = &np
		} else {
			msgPrinter.Printf("Neither node id nor node policy is specified. Getting node policy from the local node.")
			msgPrinter.Println()
			bUseLocalNode = true
		}

		if bUseLocalNode {
			// get id from local node, check arch
			policyCheckInput.NodeId, policyCheckInput.NodeArch, policyCheckInput.NodeType, policyCheckInput.NodeClusterNS, policyCheckInput.NodeNamespaceScoped, _ = getLocalNodeInfo(nodeArch, nodeType, nodeNamespace, nodeIsNamespaceScoped, "")

			// get node policy from local node
			var np exchangecommon.NodePolicy
			cliutils.HorizonGet("node/policy", []int{200}, &np, false)
			policyCheckInput.NodePolicy = &np
		}

		if nodeType == "" && policyCheckInput.NodeId != "" {
			cliutils.Verbose(msgPrinter.Sprintf("No node type has been provided: node type of '%v' node will be used", policyCheckInput.NodeId))
		}

		if servicePolFile != "" {
			policyCheckInput.ServicePolicy = sp.GetExternalPolicy()
		}

		// put the given service defs into the uiCheckInput
		if serviceDefs != nil || len(serviceDefs) != 0 {
			policyCheckInput.Service = serviceDefs
		}

		cliutils.Verbose(msgPrinter.Sprintf("Using compatibility checking input: %v", policyCheckInput))

		// now we can call the real code to check if the policies are compatible.
		// the policy validation are done wthin the calling function.
		compOutput, err := compcheck.PolicyCompatible(ec, &policyCheckInput, checkAllSvcs, msgPrinter)
		if err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, err.Error())
		} else {
			if !showDetail {
				compOutput.Input = nil
			}
			totalOutput[nId] = compOutput
		}
	}

	// display the output
	var output string
	var err error
	if haGroupName == "" && len(nIds) == 1 {
		for _, o := range totalOutput {
			output, err = cliutils.DisplayAsJson(o)
			break
		}
	} else {
		output, err = cliutils.DisplayAsJson(totalOutput)
	}

	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn deploycheck policy' output: %v", err))
	}
	fmt.Println(output)
}

// make sure -n and --node-pol, -b and -B, pairs are mutually compatible.
// get default credential, node id and org if they are not set.
func verifyPolicyCompatibleParameters(org string, userPw string,
	nodeIds []string, haGroupName string,
	nodeType string, nodeNamespace string,
	nodePolFile string,
	businessPolId string, businessPolFile string,
	servicePolFile string,
	svcDefFiles []string) (string, string, []string, bool, []common.AbstractServiceFile) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// make sure the node type has correct value
	ValidateNodeType(nodeType)

	// make sure the namespace is only specified for cluster node
	if nodeType == persistence.DEVICE_TYPE_DEVICE && nodeNamespace != "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("-s can only be specified when the node type sepcified by -t is 'cluster'."))
	}

	credToUse := cliutils.WithDefaultEnvVar(&userPw, "HZN_EXCHANGE_NODE_AUTH")
	orgToUse := org

	nodeIdToUse := []string{}
	useNodeId := false
	if haGroupName != "" {
		if len(nodeIds) != 0 {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("-n and --ha-group are mutually exclusive."))
		}
		if nodePolFile != "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("--ha-group and --node-pol are mutually exclusive."))
		}

		if orgToUse == "" {
			orgToUse = GetOrgFromCred(org, *credToUse)
		}

		// get the HA group members
		haMembers := GetHAGroupMembers(orgToUse, *credToUse, haGroupName)
		if haMembers != nil && len(haMembers) > 0 {
			useNodeId = true
			for _, m := range haMembers {
				nodeIdToUse = append(nodeIdToUse, m)
			}
		} else {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("The HA group %v does not have members.", haGroupName))
		}
	} else if len(nodeIds) == 0 {
		if nodePolFile == "" {
			// get node id from HZN_EXCHANGE_NODE_AUTH
			if nodeIdTok := os.Getenv("HZN_EXCHANGE_NODE_AUTH"); nodeIdTok != "" {
				id, _ := cliutils.SplitIdToken(nodeIdTok)
				if id != "" {
					// true means will use exchange call
					useNodeId = true
					nodeIdToUse = append(nodeIdToUse, id)
				}
			}
		}
	} else {
		nodeIdToUse = append(nodeIdToUse, nodeIds...)
		if nodePolFile == "" {
			// true means will use exchange call
			useNodeId = true
		} else {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("-n and --node-pol are mutually exclusive."))
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

	if businessPolId != "" && svcDefFiles != nil && len(svcDefFiles) > 0 {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("-b and --service are mutually exclusive."))
	}

	useSId, serviceDefs := useExchangeForServiceDef(svcDefFiles)

	// if user credential is not given, then use the node auth env HZN_EXCHANGE_NODE_AUTH if it is defined.
	if useNodeId || useBPolId || useSPolId || useSId {
		if orgToUse == "" {
			orgToUse = GetOrgFromCred(org, *credToUse)
		}
	}

	return orgToUse, *credToUse, nodeIdToUse, useNodeId, serviceDefs
}

func GetOrgFromCred(org string, credToUse string) string {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	orgToUse := org
	if credToUse == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Please specify the Exchange credential with -u for querying the node, deployment policy, service and service policy."))
	} else {
		// get the org from credToUse
		if orgToUse == "" {
			id, _ := cliutils.SplitIdToken(credToUse)
			if id != "" {
				orgToUse, _ = cliutils.TrimOrg("", id)
				if orgToUse == "" {
					cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Please specify the organization with -o for the Exchange credentials: %v.", credToUse))
				}
			}
		}
	}

	return orgToUse
}
