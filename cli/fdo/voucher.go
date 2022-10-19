package fdo

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cli/exchange"
	"github.com/open-horizon/anax/cli/register"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	anaxExchange "github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

// Sub-commands for inspecting and importing an Intel FDO ownership voucher for an FDO device.

type ImportResponse struct {
	NodeId string `json:"deviceUuid"`
}

// list the all the uploaded FDO vouchers, or a single voucher
func VoucherList(org, userCreds, voucher string) {
	msgPrinter := i18n.GetMessagePrinter()
	cliutils.Verbose(msgPrinter.Sprintf("Listing imported FDO vouchers."))

	// call the ocs-api to get the uploaded vouchers
	respBodyBytes, _ := getVouchers(org, userCreds, voucher)

	fmt.Printf("%s", respBodyBytes)

	// add a new line for voucher returned with content
	if voucher != "" {
		msgPrinter.Println()
	}
}

// download the specified device-id voucher to file on disk
func VoucherDownload(org, userCreds, device, outputFile string, overwrite bool) {
	msgPrinter := i18n.GetMessagePrinter()
	cliutils.Verbose(msgPrinter.Sprintf("Listing imported FDO vouchers."))

	// call the ocs-api to get the uploaded vouchers
	var respBodyBytes []byte
	respBodyBytes, _ = getVouchers(org, userCreds, device)

	// Download response body directly to file
	if outputFile != "" {
		fileName := cliutils.DownloadToFile(outputFile, device, respBodyBytes, "", 0600, overwrite)
		msgPrinter.Printf("Voucher \"%s\" successfully downloaded to %s from the FDO owner services.", device, fileName)
		msgPrinter.Println()
	} else {
		// List voucher on screen
		fmt.Printf("%s\n", respBodyBytes)
	}
}

// hzn fdo voucher import <voucher-file>
func VoucherImport(org, userCreds string, voucherFile *os.File, example, policyFilePath, patternName, userInputFileName, haGroupName string) {
	defer voucherFile.Close()
	msgPrinter := i18n.GetMessagePrinter()
	cliutils.Verbose(msgPrinter.Sprintf("Importing voucher file name: %s", voucherFile.Name()))

	// Check input
	if (example != "" && policyFilePath != "") || (example != "" && patternName != "") || (patternName != "" && policyFilePath != "") {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("-e, --policy, and -p are all mutually exclusive (can specify one of them)"))
	}
	if policyFilePath != "" {
		if _, err := os.Stat(policyFilePath); err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("accessing %s: %v", policyFilePath, err))
		}
	}
	if userInputFileName != "" {
		if _, err := os.Stat(userInputFileName); err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("accessing %s: %v", userInputFileName, err))
		}
	}

	// make sure pattern name is in the format of org/pattern
	if patternName != "" {
		var patternOrg string
		patternOrg, patternName = cliutils.TrimOrg(org, patternName)
		patternName = fmt.Sprintf("%v/%v", patternOrg, patternName)
	}

	// make sure the HA group org is the same as the node org and the group name does not contain the org.
	if haGroupName != "" {
		var haGroupOrg string
		haGroupOrg, haGroupName = cliutils.TrimOrg(org, haGroupName)
		if haGroupOrg != org {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("the HA group organization ID '%v' is different from the node organization ID '%v'.", haGroupOrg, org))
		}
	}

	// Determine voucher file type, and handle it accordingly
	if strings.HasSuffix(voucherFile.Name(), ".tar") {
		importTar(org, userCreds, bufio.NewReader(voucherFile), voucherFile.Name(), example, policyFilePath, patternName, userInputFileName, haGroupName)
	} else if strings.HasSuffix(voucherFile.Name(), ".tar.gz") || strings.HasSuffix(voucherFile.Name(), ".tgz") {
		gzipReader, err := gzip.NewReader(voucherFile)
		if err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("reading voucher file %s: %v", voucherFile.Name(), err))
		}
		importTar(org, userCreds, gzipReader, voucherFile.Name(), example, policyFilePath, patternName, userInputFileName, haGroupName)
	} else if strings.HasSuffix(voucherFile.Name(), ".zip") {
		importZip(org, userCreds, voucherFile, example, policyFilePath, patternName, userInputFileName, haGroupName)
	} else {
		import1Voucher(org, userCreds, bufio.NewReader(voucherFile), voucherFile.Name(), example, policyFilePath, patternName, userInputFileName, haGroupName, false)
	}
}

// get the uploaded vouchers
func getVouchers(org, userCreds, voucher string) ([]byte, string) {
	msgPrinter := i18n.GetMessagePrinter()
	cliutils.Verbose(msgPrinter.Sprintf("Listing imported FDO vouchers."))

	// setup HTTP parameters and URL
	var respBodyBytes []byte
	var requestBodyBytes []byte
	var url string
	fdoURL := cliutils.GetFdoSvcUrl()

	if voucher == "" {
		url = fdoURL + "/orgs/" + org + "/fdo/vouchers"
	} else {
		url = fdoURL + "/orgs/" + org + "/fdo/vouchers" + "/" + voucher
	}

	cliutils.Verbose("url: %v", url)

	creds := cliutils.OrgAndCreds(org, userCreds)
	method := http.MethodGet
	apiMsg := method + " " + url
	httpClient := cliutils.GetHTTPClient(config.HTTPRequestTimeoutS)

	resp := cliutils.InvokeRestApi(httpClient, method, url, creds, requestBodyBytes, "FDO Owner Service", apiMsg, map[string]string{"Accept": "text/plain"}, true)
	if resp.Body != nil {
		defer resp.Body.Close()
	}

	httpCode := resp.StatusCode
	cliutils.Verbose(msgPrinter.Sprintf("HTTP code: %d", httpCode))

	respBodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		cliutils.Fatal(cliutils.HTTP_ERROR, msgPrinter.Sprintf("failed to read exchange body response from %s: %v", apiMsg, err))
	}
	if httpCode == 404 || httpCode == 403 {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Invalid voucher name. Voucher \"%s\" does not exist in org \"%s\".\n", voucher, org))
	} else if httpCode == 401 {
		user, _ := cliutils.SplitIdToken(userCreds)
		if voucher == "" {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Invalid credentials. User \"%s\" cannot access vouchers in org \"%s\" with given credentials.\n", user, org))
		} else {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Invalid credentials. User \"%s\" cannot access voucher \"%s\" in org \"%s\" with given credentials.\n", user, voucher, org))
		}
	} else if httpCode != 200 {
		cliutils.Fatal(cliutils.HTTP_ERROR, msgPrinter.Sprintf("bad HTTP code %d from %s: %s", httpCode, apiMsg, string(respBodyBytes)))
	}

	return respBodyBytes, apiMsg
}

func importTar(org string, userCreds string, voucherFileReader io.Reader, voucherFileName, example, policyFilePath, patternName, userInputFileName, haGroupName string) {
	msgPrinter := i18n.GetMessagePrinter()
	tarReader := tar.NewReader(voucherFileReader)
	for {
		header, err := tarReader.Next() // get the next file in the tar, and turn tarReader into a reader for that file
		if err == io.EOF {
			break // this means we hit the end of the tar file
		} else if err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("reading the bytes from %s: %v", voucherFileName, err))
		}
		switch header.Typeflag {
		case tar.TypeDir:
			continue // just ignore the directories
		case tar.TypeReg:
			// Regular file, only process it if it's a .txt file
			if strings.HasPrefix(header.Name, ".") || !strings.HasSuffix(header.Name, ".txt") {
				continue
			}
			import1Voucher(org, userCreds, tarReader, header.Name, example, policyFilePath, patternName, userInputFileName, haGroupName, false)
		}
	}
}

func importZip(org string, userCreds string, voucherFile *os.File, example, policyFilePath, patternName, userInputFileName, haGroupName string) {
	msgPrinter := i18n.GetMessagePrinter()
	voucherBytes, err := ioutil.ReadAll(bufio.NewReader(voucherFile))
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("reading the bytes from %s: %v", voucherFile.Name(), err))
	}
	zipReader, err := zip.NewReader(bytes.NewReader(voucherBytes), int64(len(voucherBytes)))
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("creating zip reader for %s: %v", voucherFile.Name(), err))
	}
	for _, fileInfo := range zipReader.File {
		if strings.HasPrefix(fileInfo.Name, ".") || !strings.HasSuffix(fileInfo.Name, ".txt") {
			continue
		}
		zipFileReader, err := fileInfo.Open()
		if err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("opening file %s within zip for %s: %v", fileInfo.Name, voucherFile.Name(), err))
		}
		import1Voucher(org, userCreds, zipFileReader, fileInfo.Name, example, policyFilePath, patternName, userInputFileName, haGroupName, false)
		zipFileReader.Close()
	}
}

func import1Voucher(org string, userCreds string, voucherFileReader io.Reader, voucherFileName, example, policyFilePath, patternName, userInputFileName, haGroupName string, quieter bool) {
	msgPrinter := i18n.GetMessagePrinter()

	// Parse the voucher so we can tell them what we are doing
	voucherBytes, err := ioutil.ReadAll(voucherFileReader)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("reading the bytes from %s: %v", voucherFileName, err))
	}

	// Import the voucher to the FDO owner service
	creds := cliutils.OrgAndCreds(org, userCreds)
	importResponse := ImportResponse{}
	FdoPostVoucher(creds, org, voucherBytes, &importResponse)
	if !quieter {
		msgPrinter.Printf("Voucher imported. Node id: %s", importResponse.NodeId)
		msgPrinter.Println()
	}

	// Pre-create the node resource in the exchange, so it is already there when hzn register is run on the FDO device
	// Doing the equivalent of: hzn exchange node create -org "org" -n "$nodeId" -u "user:pw" (with optional pattern)
	// todo: try to get the device arch from the voucher
	// exchange.NodeCreate(org, "", importResponse.NodeId, importResponse.NodeToken, userCreds, "amd64", "", persistence.DEVICE_TYPE_DEVICE, true)
	nodeToken, err := cutil.SecureRandomString()
	if err != nil {
		cliutils.Fatal(cliutils.INTERNAL_ERROR, msgPrinter.Sprintf("could not create a random token"))
	}
	NodeAddDevice(org, importResponse.NodeId, nodeToken, userCreds, "", patternName, userInputFileName, quieter)

	// Create the node policy in the exchange, if they specified it
	var policyStr string
	if policyFilePath != "" {
		policyBytes, err := ioutil.ReadFile(policyFilePath)
		if err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("reading the policy file %s: %v", policyFilePath, err))
		}
		policyStr = string(policyBytes)
	} else if example != "" {
		policyStr = `{ "deployment": { "properties": [ { "name": "openhorizon.example", "value": "` + example + `" } ] }}`
		cliutils.Verbose(msgPrinter.Sprintf("Using node policy: %s", policyStr))
	}
	if policyStr != "" {
		NodeAddPolicyString(org, userCreds, importResponse.NodeId, policyStr, quieter)
	}

	// add the node in the HA group, if specified
	if haGroupName != "" {
		// now get the node from the exchange
		var devicesResp anaxExchange.GetDevicesResponse
		existingHagrName := ""
		cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes/"+importResponse.NodeId, cliutils.OrgAndCreds(org, userCreds), []int{200}, &devicesResp)
		for _, node := range devicesResp.Devices {
			existingHagrName = node.HAGroup
			break
		}
		if existingHagrName != "" {
			if existingHagrName != haGroupName {
				cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Cannot proceed with node %v because the node is in a different HA group: %v.", importResponse.NodeId, existingHagrName))
			} else if !quieter {
				msgPrinter.Printf("The node is already in the HA group '%v'", haGroupName)
				msgPrinter.Println()
			}
		} else {
			register.AddNodeToHAGroup(org, importResponse.NodeId, haGroupName, userCreds)
		}
	}
}

// Like cliutils.ExchangePutPost, except it gets a response body on success
func FdoPostVoucher(creds, org string, requestBodyBytes []byte, response *ImportResponse) {
	msgPrinter := i18n.GetMessagePrinter()
	method := http.MethodPost

	var url string
	fdoURL := cliutils.GetFdoSvcUrl()
	url = fdoURL + "/orgs/" + org + "/fdo/vouchers"
	apiMsg := method + " " + url

	cliutils.Verbose("url: %v", url)

	httpClient := cliutils.GetHTTPClient(config.HTTPRequestTimeoutS)
	// Note: need to pass the request body in as a string, not []byte, so that it sets header: Content-Type, application/json
	resp := cliutils.InvokeRestApi(httpClient, method, url, creds, requestBodyBytes, "FDO Owner Service", apiMsg, map[string]string{"Accept": "text/plain", "Content-Type": "text/plain"}, true)
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	httpCode := resp.StatusCode
	cliutils.Verbose(msgPrinter.Sprintf("HTTP code: %d", httpCode))
	respBodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		cliutils.Fatal(cliutils.HTTP_ERROR, msgPrinter.Sprintf("failed to read exchange body response from %s: %v", apiMsg, err))
	}
	if httpCode == 400 {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Invalid voucher file format: %s.\n", string(respBodyBytes)))
	} else if httpCode == 401 {
		user, _ := cliutils.SplitIdToken(creds)
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Invalid credentials. User \"%s\" cannot access vouchers in org \"%s\" with given credentials.\n", user, org))
	} else if httpCode != 201 && httpCode != 200 {
		cliutils.Fatal(cliutils.HTTP_ERROR, msgPrinter.Sprintf("bad HTTP code %d from %s: %s", httpCode, apiMsg, string(respBodyBytes)))
	}

	response.NodeId = fmt.Sprintf("%s", respBodyBytes)
}

// This is similar to exchange.NodeCreate(), except it can optionally set a pattern
func NodeAddDevice(org, nodeId, nodeToken, userPw, arch, patternName, userInputFileName string, quieter bool) {
	msgPrinter := i18n.GetMessagePrinter()
	if !quieter {
		msgPrinter.Printf("Adding/updating node...")
		msgPrinter.Println()
	}

	var inputs []policy.UserInput
	if userInputFileName != "" {
		userinputBytes, err := ioutil.ReadFile(userInputFileName)
		if err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("reading the service cofiguration user input file %s: %v", userInputFileName, err))
		}

		err = json.Unmarshal([]byte(userinputBytes), &inputs)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("Error unmarshaling userInput json file: %v", err))
		}
	} else {
		inputs = []policy.UserInput{}
	}

	putNodeReqBody := anaxExchange.PutDeviceRequest{Token: nodeToken, Name: nodeId, NodeType: persistence.DEVICE_TYPE_DEVICE, Pattern: patternName, UserInput: inputs, PublicKey: []byte(""), Arch: arch}
	cliutils.ExchangePutPost("Exchange", http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes/"+nodeId+"?"+cliutils.NOHEARTBEAT_PARAM, cliutils.OrgAndCreds(org, userPw), []int{201}, putNodeReqBody, nil)
}

// This is the same as exchange.NodeAddPolicy(), except that the node policy is a string, not a file
func NodeAddPolicyString(org, credToUse, node, policyStr string, quieter bool) {
	msgPrinter := i18n.GetMessagePrinter()

	// Parse the policy metadata
	var policyFile exchangecommon.NodePolicy
	err := json.Unmarshal([]byte(policyStr), &policyFile)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal json string '%s': %v", policyStr, err))
	}

	//Check the policy file format
	err = policyFile.ValidateAndNormalize()
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Incorrect policy format in '%s': %v", policyStr, err))
	}

	// ensure the node exists first
	exchangeUrl := cliutils.GetExchangeUrl()
	var nodes exchange.ExchangeNodes
	httpCode := cliutils.ExchangeGet("Exchange", exchangeUrl, "orgs/"+org+"/nodes"+cliutils.AddSlash(node), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &nodes)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("node '%v/%v' not found.", org, node))
	}

	// add/replace node policy
	if !quieter {
		msgPrinter.Printf("Adding/updating node policy...")
		msgPrinter.Println()
	}
	exchNodePolicy := anaxExchange.ExchangeNodePolicy{NodePolicy: policyFile, NodePolicyVersion: exchangecommon.NODEPOLICY_VERSION_VERSION_2}
	cliutils.ExchangePutPost("Exchange", http.MethodPut, exchangeUrl, "orgs/"+org+"/nodes/"+node+"/policy"+"?"+cliutils.NOHEARTBEAT_PARAM, cliutils.OrgAndCreds(org, credToUse), []int{201}, exchNodePolicy, nil)
}
