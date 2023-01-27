package fdo

import (
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/i18n"
	"io/ioutil"
	"net/http"
	"strings"
)

// List keys that are stored in FDO owner services
func KeyList(org, userCreds, keyName string) {
	supportedAliases := []string{"SECP256R1", "SECP384R1", "RSAPKCS3072", "RSAPKCS2048", "RSA2048RESTR"}

	msgPrinter := i18n.GetMessagePrinter()

	if keyName == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Please sepcify the device alias received from the manufacturer. The supported device aliases are: %v which are all cryptography standards.", strings.Join(supportedAliases, ",")))
	} else {
		found := false
		for _, sa := range supportedAliases {
			if keyName == sa {
				found = true
			}
		}
		if !found {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Invalid alias. The supported device aliases are: %v which are all cryptography standards.", strings.Join(supportedAliases, ",")))
		}

	}

	// Retrieve keys from FDO owner services API
	cliutils.Verbose(msgPrinter.Sprintf("Listing FDO keys."))
	respBodyBytes, _ := sendFdoKeysApiRequest(org, userCreds, keyName, http.MethodGet, []byte{}, []int{200, 404})

	// List the public key
	fmt.Printf("%s\n", respBodyBytes)
}

// Helper function to send API requests to FDO owner services API
func sendFdoKeysApiRequest(org, userCreds, keyName, method string, body interface{}, goodHttpCodes []int) ([]byte, string) {
	msgPrinter := i18n.GetMessagePrinter()

	// setup HTTP parameters and URL
	fdoURL := cliutils.GetFdoSvcUrl()

	url := fdoURL + "/orgs/" + org + "/fdo/certificate" + cliutils.AddSlash(keyName)

	creds := cliutils.OrgAndCreds(org, userCreds)
	apiMsg := method + " " + fdoURL
	cliutils.Verbose("url: %v", url)

	httpClient := cliutils.GetHTTPClient(config.HTTPRequestTimeoutS)
	resp := cliutils.InvokeRestApi(httpClient, method, url, creds, body, "FDO Owner Service", apiMsg, make(map[string]string), true)
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	httpCode := resp.StatusCode
	cliutils.Verbose(msgPrinter.Sprintf("HTTP code: %d", httpCode))

	respBodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		cliutils.Fatal(cliutils.HTTP_ERROR, msgPrinter.Sprintf("failed to read exchange body response from %s: %v", apiMsg, err))
	}
	if httpCode == 404 && keyName != "" {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Invalid alias name. The key for device alias \"%s\" does not exist in organization \"%s\".", keyName, org))
	} else if httpCode == 401 {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Invalid credentials. The user cannot access the public key for device alias \"%s\" in organization \"%s\" with the given credentials.", keyName, org))
	} else {
		isGoodCode := false
		for _, goodCode := range goodHttpCodes {
			if goodCode == httpCode {
				isGoodCode = true
			}
		}
		if !isGoodCode {
			cliutils.Fatal(cliutils.HTTP_ERROR, msgPrinter.Sprintf("bad HTTP code %d from %s: %s", httpCode, apiMsg, string(respBodyBytes)))
		}
	}
	return respBodyBytes, apiMsg
}
