package deploycheck

import (
	"encoding/json"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/businesspolicy"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/compcheck"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
	"net/http"
)

// check if the policies are compatible
func AllCompatible(org string, userPw string, nodeId string, nodeArch string, nodePolFile string, nodeUIFile string, businessPolId string, businessPolFile string, servicePolFile string, serviceFiles []string, checkAllSvcs bool, showDetail bool) {

	msgPrinter := i18n.GetMessagePrinter()
	msgPrinter.Printf("AllCompatible called")
	msgPrinter.Println()

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
	return &config.HTTPClientFactory{
		NewHTTPClient: clientFunc,
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
