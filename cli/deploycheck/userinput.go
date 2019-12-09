package deploycheck

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/open-horizon/anax/businesspolicy"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	cliexchange "github.com/open-horizon/anax/cli/exchange"
	"github.com/open-horizon/anax/compcheck"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/semanticversion"
	"os"
)

func readServiceFile(filePath string, inputFileStruct *cliexchange.ServiceFile) {
	newBytes := cliconfig.ReadJsonFileWithLocalConfig(filePath)
	err := json.Unmarshal(newBytes, inputFileStruct)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, i18n.GetMessagePrinter().Sprintf("Error unmarshaling service json file %s: %v", filePath, err))
	}
}

func readUserInputFile(filePath string, inputFileStruct *[]policy.UserInput) {
	newBytes := cliconfig.ReadJsonFileWithLocalConfig(filePath)
	err := json.Unmarshal(newBytes, inputFileStruct)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, i18n.GetMessagePrinter().Sprintf("Error unmarshaling userInput json file: %v", err))
	}
}

// check if the user inputs for services are compatible
func UserInputCompatible(org string, userPw string, nodeId string, nodeArch string, nodeUIFile string, businessPolId string, businessPolFile string, svcDefFiles []string, checkAllSvcs bool, showDetail bool) {

	msgPrinter := i18n.GetMessagePrinter()

	// check the input and get the defaults
	userOrg, credToUse, nId, useNodeId, bp, serviceDefs := verifyUserInputCompatibleParamters(org, userPw, nodeId, nodeUIFile, businessPolId, businessPolFile, svcDefFiles)

	uiCheckInput := compcheck.UserInputCheck{}
	uiCheckInput.NodeArch = nodeArch
	uiCheckInput.BusinessPolicy = bp

	// formalize node id or get node policy
	bUseLocalNode := false
	if useNodeId {
		// add credentials'org to node id if the node id does not have an org
		nId = cliutils.AddOrg(userOrg, nId)
		uiCheckInput.NodeId = nId
	} else if nodeUIFile != "" {
		// read the node userinput from file
		var node_ui []policy.UserInput
		readUserInputFile(nodeUIFile, &node_ui)
		uiCheckInput.NodeUserInput = node_ui
	} else {
		msgPrinter.Printf("Neither node id nor node user input file is not specified. Getting node user input from the local node.")
		msgPrinter.Println()
		bUseLocalNode = true
	}

	if bUseLocalNode {
		// get id from local node, check arch
		uiCheckInput.NodeId, uiCheckInput.NodeArch = getLocalNodeInfo(nodeArch)

		// get node user input from local node
		var node_ui []policy.UserInput
		cliutils.HorizonGet("node/userinput", []int{200}, &node_ui, false)
		uiCheckInput.NodeUserInput = node_ui
	}

	// put the given service defs into the uiCheckInput
	if serviceDefs != nil || len(serviceDefs) != 0 {
		uiCheckInput.Service = serviceDefs
	}

	cliutils.Verbose(msgPrinter.Sprintf("Using compatibility checking input: %v", uiCheckInput))

	// get exchange context
	ec := getUserExchangeContext(userOrg, credToUse)

	// compcheck.PolicyCompatible function calls the exchange package that calls glog.
	// set the glog stderrthreshold to 3 (fatal) in order for glog error messages not showing up in the output
	flag.Set("stderrthreshold", "3")
	flag.Parse()

	// now we can call the real code to check if the policies are compatible.
	// the policy validation are done wthin the calling function.
	compOutput, err := compcheck.UserInputCompatible(ec, &uiCheckInput, checkAllSvcs, msgPrinter)
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

// Make sure the url, arch and version are correct in the service definition file.
func validateService(service *cliexchange.ServiceFile) error {
	msgPrinter := i18n.GetMessagePrinter()

	if service.URL == "" {
		return fmt.Errorf(msgPrinter.Sprintf("URL must be specified in the service definition."))
	}
	if service.Version == "" {
		return fmt.Errorf(msgPrinter.Sprintf("Version must be specified in the service definition."))
	} else if !semanticversion.IsVersionString(service.Version) {
		return fmt.Errorf(msgPrinter.Sprintf("Invalide version format: %v.", service.Version))
	}
	if service.Arch == "" {
		return fmt.Errorf(msgPrinter.Sprintf("Arch must be specified in the service definition."))
	}

	return nil
}

// make sure -n and --node-pol, -b and --business-pol pairs are mutually compatible.
// get default credential, node id and org if they are not set.
func verifyUserInputCompatibleParamters(org string, userPw string, nodeId string, nodeUIFile string, businessPolId string, businessPolFile string, svcDefFiles []string) (string, string, string, bool, *businesspolicy.BusinessPolicy, []exchange.ServiceDefinition) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	useNodeId := false
	nodeIdToUse := nodeId
	if nodeId != "" {
		if nodeUIFile == "" {
			// true means will use exchange call
			useNodeId = true
		} else if nodeId != "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("-n and --node-ui are mutually exclusive."))
		}
	} else {
		if nodeUIFile == "" {
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
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("-b and --business-pol are mutually exclusive."))
		}
	} else {
		if businessPolFile == "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Either -b or --business-pol must be specified."))
		}
	}

	useSId := false
	serviceDefs := []exchange.ServiceDefinition{}
	svc_orgs := []string{}
	if svcDefFiles == nil || len(svcDefFiles) == 0 {
		// true means will use exchange call
		useSId = true
	} else {
		// check if the service has dependent services, if it does, then the code needs to
		// access the exchange
		for _, s_file := range svcDefFiles {
			var service cliexchange.ServiceFile
			readServiceFile(s_file, &service)
			if err := validateService(&service); err != nil {
				cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Error in service file %v. %v", s_file, err))
			}

			// save the service orgs for latet checkups because the org will be lost in the conversion
			svc_orgs = append(svc_orgs, service.Org)

			// convert ServiceFile to exchange.ServiceDefinition
			svcInput := exchange.ServiceDefinition{Label: service.Label, Description: service.Description, Public: service.Public, Documentation: service.Documentation, URL: service.URL, Version: service.Version, Arch: service.Arch, Sharable: service.Sharable, MatchHardware: service.MatchHardware, RequiredServices: service.RequiredServices, UserInputs: service.UserInputs}
			serviceDefs = append(serviceDefs, svcInput)

			if service.HasDependencies() {
				// true means will use exchange call
				useSId = true
			}
		}
	}

	// if user credential is not given, then use the node auth env HZN_EXCHANGE_NODE_AUTH if it is defined.
	credToUse := cliutils.WithDefaultEnvVar(&userPw, "HZN_EXCHANGE_NODE_AUTH")
	orgToUse := org
	if useNodeId || useBPolId || useSId {
		if *credToUse == "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Please specify the exchange credential with -u for querying the node, business policy and service policy."))
		} else {
			// get the org from credToUse
			if org == "" {
				id, _ := cliutils.SplitIdToken(*credToUse)
				if id != "" {
					orgToUse, _ = cliutils.TrimOrg("", id)
					if orgToUse == "" {
						cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Please specify the organization with -o for the exchange credentail: %v.", *credToUse))
					}
				}
			}
		}
	}

	// get business policy from file or exchange
	bp := getBusinessPolicy(orgToUse, *credToUse, businessPolId, businessPolFile)
	// the compcheck package does not need to get it again from the exchange.
	useBPolId = false

	// check if the orgs in the given service files matche the service in the business policy since the org info will be lost during the convertion.
	// Other parts will be checked later by the compcheck package.
	if svcDefFiles != nil && len(svcDefFiles) != 0 {
		for i, s_org := range svc_orgs {
			if s_org != "" && bp.Service.Org != s_org {
				cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("The service organization %v from service file %v does not match the service organization %v from business policy file %v.", s_org, svcDefFiles[i], bp.Service.Org, businessPolFile))
			}
		}
	}

	return orgToUse, *credToUse, nodeId, useNodeId, bp, serviceDefs
}
