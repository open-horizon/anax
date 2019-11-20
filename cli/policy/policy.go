package policy

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
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/i18n"
	"net/http"
	"os"
)

func List() {
	// Get the node policy info
	nodePolicy := externalpolicy.ExternalPolicy{}
	cliutils.HorizonGet("node/policy", []int{200}, &nodePolicy, false)

	// Output the combined info
	output, err := cliutils.DisplayAsJson(nodePolicy)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, i18n.GetMessagePrinter().Sprintf("failed to marshal 'hzn policy list' output: %v", err))
	}

	fmt.Println(output)
}

func Update(fileName string) {

	ep := new(externalpolicy.ExternalPolicy)
	readInputFile(fileName, ep)

	msgPrinter := i18n.GetMessagePrinter()
	msgPrinter.Printf("Updating Horizon node policy and re-evaluating all agreements based on this node policy. Existing agreements might be cancelled and re-negotiated.")
	msgPrinter.Println()
	cliutils.HorizonPutPost(http.MethodPost, "node/policy", []int{201, 200}, ep)

	msgPrinter.Printf("Horizon node policy updated.")
	msgPrinter.Println()

}

func Patch(patch string) {
	cliutils.HorizonPutPost(http.MethodPatch, "node/policy", []int{201, 200}, patch)

	msgPrinter := i18n.GetMessagePrinter()
	msgPrinter.Printf("Horizon node policy updated.")
	msgPrinter.Println()
}

func readInputFile(filePath string, inputFileStruct *externalpolicy.ExternalPolicy) {
	newBytes := cliconfig.ReadJsonFileWithLocalConfig(filePath)
	err := json.Unmarshal(newBytes, inputFileStruct)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, i18n.GetMessagePrinter().Sprintf("failed to unmarshal json input file %s: %v", filePath, err))
	}
}

func Remove(force bool) {
	if !force {
		cliutils.ConfirmRemove(i18n.GetMessagePrinter().Sprintf("Are you sure you want to remove the node policy?"))
	}

	msgPrinter := i18n.GetMessagePrinter()
	msgPrinter.Printf("Removing Horizon node policy and re-evaluating all agreements based on just the built-in node policy. Existing agreements might be cancelled and re-negotiated.")
	msgPrinter.Println()
	cliutils.HorizonDelete("node/policy", []int{200, 204}, false)

	msgPrinter.Printf("Horizon node policy deleted.")
	msgPrinter.Println()
}

// Display an empty policy template as an object.
func New() {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	var policy_template = []string{
		`{`,
		`  "properties": [   /* ` + msgPrinter.Sprintf("A list of policy properties that describe the object.") + ` */`,
		`    {`,
		`       "name": "",`,
		`       "value": nil`,
		`      }`,
		`  ],`,
		`  "constraints": [  /* ` + msgPrinter.Sprintf("A list of constraint expressions of the form <property name> <operator> <property value>,") + ` */`,
		`                    /* ` + msgPrinter.Sprintf("separated by boolean operators AND (&&) or OR (||).") + `*/`,
		`       "" `,
		`  ], `,
		`}`,
	}

	for _, s := range policy_template {
		fmt.Println(s)
	}
}

// check if the policies are compatible
func Compatible(org string, userPw string, nodeId string, nodePolFile string, businessPolId string, businessPolFile string, servicePolFile string, showDetail bool) {

	msgPrinter := i18n.GetMessagePrinter()

	// check the input and get the defaults
	userOrg, credToUse, nId, useNodeId, useBPolId := verifyCompatibleParamters(org, userPw, nodeId, nodePolFile, businessPolId, businessPolFile, servicePolFile)
	cliutils.Verbose(msgPrinter.Sprintf("Exchange credentials to use: %v, organizationnode id: %v", credToUse, userOrg))

	policyCheckInput := compcheck.PolicyCompInput{}

	// formalize node id or get node policy
	nodeOrg := userOrg
	if useNodeId {
		// add credentials'org to node id if the node id does not have an org
		nId = cliutils.AddOrg(userOrg, nId)

		policyCheckInput.SetNodeId(nId)

		// get node org for later use
		nodeOrg, _ = cliutils.TrimOrg(nodeOrg, nId)
	} else if nodePolFile == "" {
		msgPrinter.Printf("Neither node id nor node policy is not specified. Getting node policy from the local node.")
		msgPrinter.Println()

		var np externalpolicy.ExternalPolicy
		cliutils.HorizonGet("node/policy", []int{200}, &np, false)

		policyCheckInput.SetNodePolicy(&np)

		// get node org for later use
		horDevice := api.HorizonDevice{}
		cliutils.HorizonGet("node", []int{200}, &horDevice, false)
		if horDevice.Org != nil {
			nodeOrg = *horDevice.Org
		}
	} else {
		// read the node policy from file
		var np externalpolicy.ExternalPolicy
		readInputFile(nodePolFile, &np)

		policyCheckInput.SetNodePolicy(&np)
	}

	// formalize business policy id or get business policy
	if useBPolId {
		// add node id org to the business policy id if the id does not have an org
		businessPolId = cliutils.AddOrg(nodeOrg, businessPolId)

		policyCheckInput.SetBusinessPolId(businessPolId)
	} else {
		// get business policy from file
		var bp businesspolicy.BusinessPolicy
		newBytes := cliconfig.ReadJsonFileWithLocalConfig(businessPolFile)
		if err := json.Unmarshal(newBytes, &bp); err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal json input file %s: %v", businessPolFile, err))
		}

		policyCheckInput.SetBusinessPolicy(&bp)
	}

	if servicePolFile != "" {
		// read the service policy from file
		var sp externalpolicy.ExternalPolicy
		readInputFile(servicePolFile, &sp)

		policyCheckInput.SetServicePolicy(&sp)
	}

	cliutils.Verbose(msgPrinter.Sprintf("Using compatibility checking input: %v", policyCheckInput))

	// get exchange context
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

	// compcheck.PolicyCompatible function calls the exchange package that calls glog.
	// set the glog stderrthreshold to 3 (fatal) in order for glog error messages not showing up in the output
	flag.Set("stderrthreshold", "3")
	flag.Parse()

	// now we can call the real code to check if the policies are compatible.
	// the policy validation are done wthin the calling function.
	compOutput, err := compcheck.PolicyCompatible(ec, &policyCheckInput)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, err.Error())
	} else {
		if !showDetail {
			compOutput.ProducerPolicy = nil
			compOutput.ConsumerPolicy = nil
		}

		// display the output
		output, err := cliutils.DisplayAsJson(compOutput)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, i18n.GetMessagePrinter().Sprintf("failed to marshal 'hzn policy compatible' output: %v", err))
		}

		fmt.Println(output)
	}
}

// make sure -n and --node-pol, -b and --business-pol, -s and --service-pol pairs are mutually compatible.
// get default credential, node id and org if they are not set.
func verifyCompatibleParamters(org string, userPw string, nodeId string, nodePolFile string, businessPolId string, businessPolFile string, servicePolFile string) (string, string, string, bool, bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	useNodeId := false
	nodeIdToUse := nodeId
	if nodeId != "" {
		if nodePolFile == "" {
			// true means will use exchange call
			useNodeId = true
		} else if nodeId != "" {
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

	// if user credential is not given, then use the node auth env HZN_EXCHANGE_NODE_AUTH if it is defined.
	credToUse := cliutils.WithDefaultEnvVar(&userPw, "HZN_EXCHANGE_NODE_AUTH")
	orgToUse := org
	if useNodeId || useBPolId || useSPolId {
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

	return orgToUse, *credToUse, nodeId, useNodeId, useBPolId
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
	return &config.HTTPClientFactory{
		NewHTTPClient: clientFunc,
	}
}
