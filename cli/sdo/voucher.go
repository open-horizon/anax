package sdo

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cli/exchange"
	"github.com/open-horizon/anax/config"
	anaxExchange "github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

// Sub-commands for inspecting and importing an Intel SDO ownership voucher for an SDO device.

// Structs for parsing the voucher
type OhStruct struct {
	R          []interface{} `json:"r"`
	Guid       []byte        `json:"g"` // making it type []byte will automatically base64 decode the json value
	DeviceType string        `json:"d"`
}
type Voucher struct {
	Oh OhStruct `json:"oh"`
}

// Structs for the inspect output
type InspectDevice struct {
	Uuid       string `json:"uuid"`
	DeviceType string `json:"deviceType"`
}
type InspectVoucher struct {
	RendezvousUrls []string `json:"rendezvousUrls"`
}
type InspectOutput struct {
	Device  InspectDevice  `json:"device"`
	Voucher InspectVoucher `json:"voucher"`
}

// hzn voucher inspect <voucher-file>
func VoucherInspect(voucherFile *os.File) {
	defer voucherFile.Close()
	cliutils.Verbose("Inspecting voucher file name: %s", voucherFile.Name())
	msgPrinter := i18n.GetMessagePrinter()

	outStruct := InspectOutput{}
	voucherBytes, err := ioutil.ReadAll(bufio.NewReader(voucherFile))
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("reading the bytes from %s: %v", voucherFile.Name(), err))
	}
	if err = parseVoucherBytes(voucherBytes, &outStruct); err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("parsing the json from %s: %v", voucherFile.Name(), err))
	}

	output := cliutils.MarshalIndent(outStruct, "voucher inspect")
	fmt.Println(output)
}

func parseVoucherBytes(voucherBytes []byte, outStruct *InspectOutput) error {
	// Read the voucher file and parse as json
	voucher := Voucher{}
	if err := json.Unmarshal(voucherBytes, &voucher); err != nil {
		return errors.New("parsing json: " + err.Error())
	}

	// Do further parsing of the json, for those parts that have varying types
	// The json in this section looks like: [ 1, [ 4, { "dn": "RVSDO", "po": 8040, "pow": 8040, "pr": "http" } ] ]
	for _, thing := range voucher.Oh.R {
		switch t := thing.(type) {
		case []interface{}:
			for _, thing2 := range t {
				switch t2 := thing2.(type) {
				case map[string]interface{}:
					url := getMapValueOrEmptyStr(t2, "pr") + "://" + getMapValueOrEmptyStr(t2, "dn")
					port := getMapValueOrEmptyStr(t2, "po") // Note: there is also a "pow" key that seems to have the same value, not sure the diff
					if port != "" {
						url += ":" + port
					}
					outStruct.Voucher.RendezvousUrls = append(outStruct.Voucher.RendezvousUrls, url)
				}
			}
		}
	}
	if len(outStruct.Voucher.RendezvousUrls) == 0 {
		return errors.New("did not find any rendezvous server URLs in the voucher")
	}

	// Get, decode, and convert the device uuid
	uu, err := uuid.FromBytes(voucher.Oh.Guid)
	if err != nil {
		return errors.New("decoding UUID: " + err.Error())
	}
	outStruct.Device.Uuid = uu.String()

	outStruct.Device.DeviceType = voucher.Oh.DeviceType
	return nil
}

func getMapValueOrEmptyStr(myMap map[string]interface{}, key string) string {
	if value, ok := myMap[key]; ok {
		return fmt.Sprintf("%v", value)
	} else {
		return ""
	}
}

type ImportResponse struct {
	NodeId    string `json:"deviceUuid"`
	NodeToken string `json:"nodeToken"`
}

// hzn voucher inspect <voucher-file>
func VoucherImport(org, userCreds string, voucherFile *os.File, example, policyFilePath, patternName string) {
	defer voucherFile.Close()
	msgPrinter := i18n.GetMessagePrinter()
	cliutils.Verbose(msgPrinter.Sprintf("Importing voucher file name: %s", voucherFile.Name()))

	// Check input
	sdoUrl := cliutils.GetSdoSvcUrl() // this looks in the environment or /etc/default/horizon, but hzn.go already sourced the hzn.json files
	if (example != "" && policyFilePath != "") || (example != "" && patternName != "") || (patternName != "" && policyFilePath != "") {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("-e, --policy, and -p are all mutually exclusive (can specify one of them)"))
	}
	if policyFilePath != "" {
		if _, err := os.Stat(policyFilePath); err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("accessing %s: %v", policyFilePath, err))
		}
	}

	// Determine voucher file type, and handle it accordingly
	if strings.HasSuffix(voucherFile.Name(), ".json") {
		import1Voucher(org, userCreds, sdoUrl, bufio.NewReader(voucherFile), voucherFile.Name(), example, policyFilePath, patternName, false)
	} else if strings.HasSuffix(voucherFile.Name(), ".tar") {
		importTar(org, userCreds, sdoUrl, bufio.NewReader(voucherFile), voucherFile.Name(), example, policyFilePath, patternName)
	} else if strings.HasSuffix(voucherFile.Name(), ".tar.gz") || strings.HasSuffix(voucherFile.Name(), ".tgz") {
		gzipReader, err := gzip.NewReader(voucherFile)
		if err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("reading voucher file %s: %v", voucherFile.Name(), err))
		}
		importTar(org, userCreds, sdoUrl, gzipReader, voucherFile.Name(), example, policyFilePath, patternName)
	} else if strings.HasSuffix(voucherFile.Name(), ".zip") {
		importZip(org, userCreds, sdoUrl, voucherFile, example, policyFilePath, patternName)
	} else {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("unsupported voucher file type extension: %s", voucherFile.Name()))
	}
}

func importTar(org, userCreds, sdoUrl string, voucherFileReader io.Reader, voucherFileName, example, policyFilePath, patternName string) {
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
			// Regular file, only process it if it's a json file
			if strings.HasPrefix(header.Name, ".") || !strings.HasSuffix(header.Name, ".json") {
				continue
			}
			import1Voucher(org, userCreds, sdoUrl, tarReader, header.Name, example, policyFilePath, patternName, true)
		}
	}
}

func importZip(org, userCreds, sdoUrl string, voucherFile *os.File, example, policyFilePath, patternName string) {
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
		if strings.HasPrefix(fileInfo.Name, ".") || !strings.HasSuffix(fileInfo.Name, ".json") {
			continue
		}
		zipFileReader, err := fileInfo.Open()
		if err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("opening file %s within zip for %s: %v", fileInfo.Name, voucherFile.Name(), err))
		}
		import1Voucher(org, userCreds, sdoUrl, zipFileReader, fileInfo.Name, example, policyFilePath, patternName, true)
		zipFileReader.Close()
	}
}

func import1Voucher(org, userCreds, sdoUrl string, voucherFileReader io.Reader, voucherFileName, example, policyFilePath, patternName string, quieter bool) {
	msgPrinter := i18n.GetMessagePrinter()

	// Parse the voucher so we can tell them what we are doing
	outStruct := InspectOutput{}
	voucherBytes, err := ioutil.ReadAll(voucherFileReader)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("reading the bytes from %s: %v", voucherFileName, err))
	}
	if err = parseVoucherBytes(voucherBytes, &outStruct); err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("parsing the json from %s: %v", voucherFileName, err))
	}
	// This is the single msg we want displayed even when quieter==true
	msgPrinter.Printf("Importing %s for device %s, using rendezvous servers %s ...", voucherFileName, outStruct.Device.Uuid, strings.Join(outStruct.Voucher.RendezvousUrls, ", "))
	msgPrinter.Println()

	// Import the voucher to the SDO owner service
	creds := cliutils.OrgAndCreds(org, userCreds)
	importResponse := ImportResponse{}
	SdoPostVoucher(sdoUrl+"/voucher", creds, voucherBytes, &importResponse)
	if !quieter {
		msgPrinter.Printf("Voucher imported. Node id: %s, token: %s", importResponse.NodeId, importResponse.NodeToken)
		msgPrinter.Println()
	}

	// Pre-create the node resource in the exchange, so it is already there when hzn register is run on the SDO device
	// Doing the equivalent of: hzn exchange node create -org "org" -n "$nodeId:$nodeToken" -u "user:pw" (with optional pattern)
	//todo: try to get the device arch from the voucher
	//exchange.NodeCreate(org, "", importResponse.NodeId, importResponse.NodeToken, userCreds, "amd64", "", persistence.DEVICE_TYPE_DEVICE, true)
	NodeAddDevice(org, importResponse.NodeId, importResponse.NodeToken, userCreds, "amd64", patternName, quieter)

	// Create the node policy in the exchange, if they specified it
	var policyStr string
	if policyFilePath != "" {
		policyBytes, err := ioutil.ReadFile(policyFilePath)
		if err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("reading the policy file %s: %v", policyFilePath, err))
		}
		policyStr = string(policyBytes)
	} else if example != "" {
		policyStr = `{ "properties": [ { "name": "openhorizon.example", "value": "` + example + `" } ] }`
		cliutils.Verbose(msgPrinter.Sprintf("Using node policy: %s", policyStr))
	}
	if policyStr != "" {
		NodeAddPolicyString(org, userCreds, importResponse.NodeId, policyStr, quieter)
	}
}

// Like cliutils.ExchangePutPost, except it gets a response body on success
func SdoPostVoucher(url string, creds string, requestBodyBytes []byte, respBody *ImportResponse) {
	msgPrinter := i18n.GetMessagePrinter()
	method := http.MethodPost
	apiMsg := method + " " + url
	httpClient := cliutils.GetHTTPClient(config.HTTPRequestTimeoutS)
	// Note: need to pass the request body in as a string, not []byte, so that it sets header: Content-Type, application/json
	resp := cliutils.InvokeRestApi(httpClient, method, url, creds, string(requestBodyBytes), "SDO Owner Service", apiMsg)
	defer resp.Body.Close()
	httpCode := resp.StatusCode
	cliutils.Verbose(msgPrinter.Sprintf("HTTP code: %d", httpCode))
	respBodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		cliutils.Fatal(cliutils.HTTP_ERROR, msgPrinter.Sprintf("failed to read exchange body response from %s: %v", apiMsg, err))
	}
	if httpCode != 201 {
		cliutils.Fatal(cliutils.HTTP_ERROR, msgPrinter.Sprintf("bad HTTP code %d from %s: %s", httpCode, apiMsg, string(respBodyBytes)))
	}
	err = json.Unmarshal(respBodyBytes, respBody)
	if err != nil {
		cliutils.Fatal(cliutils.HTTP_ERROR, msgPrinter.Sprintf("json unmarshalling HTTP response '%s' from %s: %v", string(respBodyBytes), apiMsg, err))
	}
}

// This is similar to exchange.NodeCreate(), except it can optionally set a pattern
func NodeAddDevice(org, nodeId, nodeToken, userPw, arch, patternName string, quieter bool) {
	msgPrinter := i18n.GetMessagePrinter()
	if !quieter {
		msgPrinter.Printf("Adding/updating node...")
		msgPrinter.Println()
	}

	putNodeReqBody := anaxExchange.PutDeviceRequest{Token: nodeToken, Name: nodeId, NodeType: persistence.DEVICE_TYPE_DEVICE, Pattern: patternName, PublicKey: []byte(""), Arch: arch}
	cliutils.ExchangePutPost("Exchange", http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes/"+nodeId, cliutils.OrgAndCreds(org, userPw), []int{201}, putNodeReqBody)
}

// This is the same as exchange.NodeAddPolicy(), except that the node policy is a string, not a file
func NodeAddPolicyString(org, credToUse, node, policyStr string, quieter bool) {
	msgPrinter := i18n.GetMessagePrinter()

	// Parse the policy metadata
	var policyFile externalpolicy.ExternalPolicy
	err := json.Unmarshal([]byte(policyStr), &policyFile)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal json string '%s': %v", policyStr, err))
	}

	//Check the policy file format
	err = policyFile.Validate()
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
	cliutils.ExchangePutPost("Exchange", http.MethodPut, exchangeUrl, "orgs/"+org+"/nodes/"+node+"/policy", cliutils.OrgAndCreds(org, credToUse), []int{201}, policyFile)

	//msgPrinter.Printf("Node policy updated.")
	//msgPrinter.Println()
}
