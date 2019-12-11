package deploycheck

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/businesspolicy"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/compcheck"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"net/http"
	"os"
)

// check if the policies are compatible
func AllCompatible(org string, userPw string, nodeId string, nodeArch string, nodePolFile string, nodeUIFile string, businessPolId string, businessPolFile string, servicePolFile string, svcDefFiles []string, checkAllSvcs bool, showDetail bool) {

	msgPrinter := i18n.GetMessagePrinter()

	// check the input and get the defaults
	userOrg, credToUse, nId, useNodeId, bp, serviceDefs := verifyCompCheckParamters(org, userPw, nodeId, nodePolFile, nodeUIFile, businessPolId, businessPolFile, servicePolFile, svcDefFiles)

	compCheckInput := compcheck.CompCheck{}
	compCheckInput.NodeArch = nodeArch
	compCheckInput.BusinessPolicy = bp

	// formalize node id or get node policy
	bUseLocalNode := false
	if useNodeId {
		// add credentials'org to node id if the node id does not have an org
		nId = cliutils.AddOrg(userOrg, nId)
		compCheckInput.NodeId = nId
	} else if nodePolFile != "" {
		// read the node policy from file
		var np externalpolicy.ExternalPolicy
		readExternalPolicyFile(nodePolFile, &np)
		compCheckInput.NodePolicy = &np
	} else {
		bUseLocalNode = true
	}

	// get node user input
	if nodeUIFile != "" {
		// read the node userinput from file
		var node_ui []policy.UserInput
		readUserInputFile(nodeUIFile, &node_ui)
		compCheckInput.NodeUserInput = node_ui
	} else {
		// empty user input
		compCheckInput.NodeUserInput = []policy.UserInput{}
	}

	if bUseLocalNode {
		msgPrinter.Printf("Neither node id nor node policy is specified. Getting node policy from the local node.")
		msgPrinter.Println()

		// get id from local node, check arch
		compCheckInput.NodeId, compCheckInput.NodeArch = getLocalNodeInfo(nodeArch)

		// get node policy from local node
		var np externalpolicy.ExternalPolicy
		cliutils.HorizonGet("node/policy", []int{200}, &np, false)
		compCheckInput.NodePolicy = &np

		if nodeUIFile == "" {
			msgPrinter.Printf("Neither node id nor node user input file is specified. Getting node user input from the local node.")
			msgPrinter.Println()
			// get node user input from local node
			var node_ui []policy.UserInput
			cliutils.HorizonGet("node/userinput", []int{200}, &node_ui, false)
			compCheckInput.NodeUserInput = node_ui
		}
	} else {
		if nodeUIFile == "" {
			msgPrinter.Printf("Node user input file is not specified with --node-ui flag, assuming no user input.")
			msgPrinter.Println()
		}
	}

	// read the service policy from file
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
	ec := getUserExchangeContext(userOrg, credToUse)

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

// make sure -n and --node-pol, -b and --business-pol, -n and --node-ui pairs are mutually exclusive.
// get default credential, node id and org if they are not set.
func verifyCompCheckParamters(org string, userPw string,
	nodeId string, nodePolFile string, nodeUIFile string,
	businessPolId string, businessPolFile string,
	servicePolFile string, svcDefFiles []string) (string, string, string, bool, *businesspolicy.BusinessPolicy, []exchange.ServiceDefinition) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	useNodeId := false
	nodeIdToUse := nodeId
	if nodeId != "" {
		if nodePolFile == "" || nodeUIFile == "" {
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
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("-b and --business-pol are mutually exclusive."))
		}
	} else {
		if businessPolFile == "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Either -b or --business-pol must be specified."))
		}
	}

	useSPolId := false
	if servicePolFile == "" {
		// true means will use exchange call
		useSPolId = true
	}

	useSId, serviceDefs, svc_orgs := useExchangeForServiceDef(svcDefFiles)

	// if user credential is not given, then use the node auth env HZN_EXCHANGE_NODE_AUTH if it is defined.
	credToUse := cliutils.WithDefaultEnvVar(&userPw, "HZN_EXCHANGE_NODE_AUTH")
	orgToUse := org
	if useNodeId || useBPolId || useSPolId || useSId {
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

// create the exchange context with the given user credentail
func getUserExchangeContext(userOrg string, credToUse string) *compcheck.UserExchangeContext {
	var ec *compcheck.UserExchangeContext
	if credToUse != "" {
		cred, token := cliutils.SplitIdToken(credToUse)
		if userOrg != "" {
			cred = cliutils.AddOrg(userOrg, cred)
		}
		ec = createUserExchangeContext(cred, token)
	} else {
		ec = createUserExchangeContext("", "")
	}

	return ec
}

// create an exchange context based on the user Id and password.
func createUserExchangeContext(userId string, passwd string) *compcheck.UserExchangeContext {
	// GetExchangeUrl trims the last slash, we need to add it back for the exchange API calls.
	exchUrl := cliutils.GetExchangeUrl() + "/"
	return &compcheck.UserExchangeContext{
		UserId:      userId,
		Password:    passwd,
		URL:         exchUrl,
		CSSURL:      "",
		HTTPFactory: newHTTPClientFactory(),
	}
}

func newHTTPClientFactory() *config.HTTPClientFactory {
	clientFunc := func(overrideTimeoutS *uint) *http.Client {
		var timeoutS uint
		if overrideTimeoutS != nil {
			timeoutS = *overrideTimeoutS
		} else {
			timeoutS = config.HTTPRequestTimeoutS
		}

		return cliutils.GetHTTPClient(int(timeoutS))
	}

	// get retry count and retry interval from env
	maxRetries, retryInterval, err := cliutils.GetHttpRetryParameters(5, 2)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, err.Error())
	}

	return &config.HTTPClientFactory{
		NewHTTPClient: clientFunc,
		RetryCount:    maxRetries,
		RetryInterval: retryInterval,
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
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal business policy json input file %s: %v", businessPolFile, err))
		}
	} else {
		// get business policy from the exchange
		var policyList exchange.GetBusinessPolicyResponse
		polOrg, polId := cliutils.TrimOrg(defaultOrg, businessPolId)

		httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+polOrg+"/business/policies"+cliutils.AddSlash(polId), cliutils.OrgAndCreds(defaultOrg, credToUse), []int{200, 404}, &policyList)
		if httpCode == 404 || policyList.BusinessPolicy == nil || len(policyList.BusinessPolicy) == 0 {
			cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("Business policy not found for %v/%v", polOrg, polId))
		} else {
			for _, exchPol := range policyList.BusinessPolicy {
				bp = exchPol.GetBusinessPolicy()
				break
			}
		}
	}
	return &bp
}
