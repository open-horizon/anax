package sdo

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/base64"
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

// hzn sdo voucher inspect <voucher-file>
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

func DeprecatedVoucherInspect(voucherFile *os.File) {
	fmt.Fprintf(os.Stderr, "WARNING: \"hzn voucher inspect\" is deprecated and will be removed in a future release. Please use \"hzn sdo voucher inspect\" instead.\n")
	VoucherInspect(voucherFile)
}

func parseVoucherBytes(voucherBytes []byte, outStruct *InspectOutput) error {
	// Read the voucher file and parse as json
	msgPrinter := i18n.GetMessagePrinter()
	voucher := Voucher{}
	if err := json.Unmarshal(voucherBytes, &voucher); err != nil {
		return errors.New("parsing json: " + err.Error())
	}

	// Do further parsing of the json, for those parts that have varying types
	// The json in this section is like 1 of these 2 cases:
	//		[ 1, [ 4, { "dn": "RVSDO", "po": 8040, "pow": 8040, "pr": "http" } ] ]
	//		[ 1, [ 4, { "ip": [4, "qS0yUg=="], "po": 8040, "pow": 8040, "pr": "http" } ] ]
	for _, thing := range voucher.Oh.R {
		switch t := thing.(type) {
		case []interface{}:
			for _, thing2 := range t {
				switch t2 := thing2.(type) {
				case map[string]interface{}:
					host := getMapValueOrEmptyStr(t2, "dn")
					if host == "" {
						// It must be an IP address. Loop thru array looking for encoded string
						if thing3, ok := t2["ip"]; ok {
							switch ipArray := thing3.(type) {
							case []interface{}:
								for _, thing4 := range ipArray {
									switch t4 := thing4.(type) {
									case string:
										// Found the IP encoded string
										var ipBytes []byte
										ipBytes = make([]byte, 4, 4)
										if n, err := base64.StdEncoding.Decode(ipBytes, []byte(t4)); err != nil {
											cliutils.Warning(msgPrinter.Sprintf("base64 decoding %s: %v", t4, err))
										} else {
											// The decoded value is a byte array of length 4. Each byte is 1 of the numbers of the IP address
											cliutils.Verbose("decoding %s yielded %d bytes", t4, n)
											if n == 4 && len(ipBytes) == 4 {
												host = fmt.Sprintf("%d.%d.%d.%d", int(ipBytes[0]), int(ipBytes[1]), int(ipBytes[2]), int(ipBytes[3]))
											}
										}
									}
								}
							}
						}
					}
					url := getMapValueOrEmptyStr(t2, "pr") + "://" + host
					port := getMapValueOrEmptyStr(t2, "po") // Note: "po" is the device port, "pow" the owner port, not sure which one i want
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

// call GET sdoURL/vouchers/[<device-uuid>] to get the uploaded vouchers
func getVouchers(org, userCreds, apiMsg string, voucher string) ([]byte, string) {
	msgPrinter := i18n.GetMessagePrinter()
	cliutils.Verbose(msgPrinter.Sprintf("Listing imported SDO vouchers."))

	// setup HTTP parameters and URL
	var respBodyBytes []byte
	var requestBodyBytes []byte
	var sdoURL string
	url := cliutils.GetSdoSvcUrl()

	if voucher == "" {
		sdoURL = url + "/orgs/" + org + "/vouchers"
	} else {
		sdoURL = url + "/orgs/" + org + "/vouchers" + "/" + voucher
	}

	creds := cliutils.OrgAndCreds(org, userCreds)
	method := http.MethodGet
	apiMsg = method + " " + sdoURL
	httpClient := cliutils.GetHTTPClient(config.HTTPRequestTimeoutS)

	resp := cliutils.InvokeRestApi(httpClient, method, sdoURL, creds, requestBodyBytes, "SDO Owner Service", apiMsg)
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

// called when the -l flag is used to list the full voucher
func listFullVoucher(respBodyBytes []byte, apiMsg string) []byte {
	msgPrinter := i18n.GetMessagePrinter()
	cliutils.Verbose(msgPrinter.Sprintf("Listing imported SDO vouchers."))

	// unmarshalling to interface{} to catch the full voucher json due to its varying element types within each array
	var output interface{}
	err := json.Unmarshal(respBodyBytes, &output)
	if err != nil {
		cliutils.Fatal(cliutils.HTTP_ERROR, msgPrinter.Sprintf("json unmarshalling HTTP response '%s' from %s: %v", string(respBodyBytes), apiMsg, err))
	}

	jsonBytes, err := json.MarshalIndent(output, "", cliutils.JSON_INDENT)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn exchange service list' output: %v", err))
	}

	return jsonBytes
}

// list the all the uploaded SDO vouchers, or a single voucher
func VoucherList(org, userCreds, voucher string, namesOnly bool) {
	msgPrinter := i18n.GetMessagePrinter()
	cliutils.Verbose(msgPrinter.Sprintf("Listing imported SDO vouchers."))

	// call the ocs-api to get the uploaded vouchers
	var respBodyBytes []byte
	var apiMsg string
	respBodyBytes, apiMsg = getVouchers(org, userCreds, apiMsg, voucher)

	// no voucher was specified, list all uploaded vouchers
	if voucher == "" {
		output := []string{}
		err := json.Unmarshal(respBodyBytes, &output)
		if err != nil {
			cliutils.Fatal(cliutils.HTTP_ERROR, msgPrinter.Sprintf("json unmarshalling HTTP response '%s' from %s: %v", string(respBodyBytes), apiMsg, err))
		}

		jsonBytes, err := json.MarshalIndent(output, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn sdo voucher list' output: %v", err))
		}

		// list only the uuid's of imported vouchers
		if namesOnly {
			fmt.Printf("%s\n", jsonBytes)

		} else { // list full details of all imported vouchers
			for i := range output {
				respBodyBytes, apiMsg = getVouchers(org, userCreds, apiMsg, output[i])
				jsonBytes := listFullVoucher(respBodyBytes, apiMsg)
				fmt.Printf("%s\n", jsonBytes)
				fmt.Printf("\n")
			}
		}

	} else { // list the details of a single imported voucher
		vouch := InspectOutput{}
		err := json.Unmarshal(respBodyBytes, &vouch)
		if err != nil {
			cliutils.Fatal(cliutils.HTTP_ERROR, msgPrinter.Sprintf("json unmarshalling HTTP response '%s' from %s: %v", string(respBodyBytes), apiMsg, err))
		}
		if namesOnly { // list only short relavent details of voucher
			if err = parseVoucherBytes(respBodyBytes, &vouch); err != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("parsing the json from %s: %v", voucher, err))
			}

			jsonBytes, err := json.MarshalIndent(vouch, "", cliutils.JSON_INDENT)
			if err != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn sdo voucher list' output: %v", err))
			}
			fmt.Printf("%s\n", jsonBytes)

		} else { // list full details of voucher
			jsonBytes := listFullVoucher(respBodyBytes, apiMsg)
			fmt.Printf("%s\n", jsonBytes)
		}
	}
}

func DeprecatedVoucherList(org, userCreds, voucher string, namesOnly bool) {
	fmt.Fprintf(os.Stderr, "WARNING: \"hzn voucher list\" is deprecated and will be removed in a future release. Please use \"hzn sdo voucher list\" instead.\n")
	VoucherList(org, userCreds, voucher, namesOnly)
}

// download the specified device-id voucher to file on disk
func VoucherDownload(org, userCreds, device, outputFile string, overwrite bool) {
	msgPrinter := i18n.GetMessagePrinter()
	cliutils.Verbose(msgPrinter.Sprintf("Listing imported SDO vouchers."))

	// call the ocs-api to get the uploaded vouchers
	var respBodyBytes []byte
	var apiMsg string
	respBodyBytes, _ = getVouchers(org, userCreds, apiMsg, device)

	// Download response body directly to file
	if outputFile != "" {
		fileName := cliutils.DownloadToFile(outputFile, device, respBodyBytes, ".json", 0600, overwrite)
		fmt.Printf("Voucher \"%s\" successfully downloaded to %s from the SDO owner services.\n", device, fileName)
	} else {
		// List voucher on screen
		fmt.Printf("%s\n", respBodyBytes)
	}
}

// hzn sdo voucher import <voucher-file>
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

func DeprecatedVoucherImport(org, userCreds string, voucherFile *os.File, example, policyFilePath, patternName string) {
	fmt.Fprintf(os.Stderr, "WARNING: \"hzn voucher import\" is deprecated and will be removed in a future release. Please use \"hzn sdo voucher import\" instead.\n")
	VoucherImport(org, userCreds, voucherFile, example, policyFilePath, patternName)
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
	SdoPostVoucher(sdoUrl+"/orgs/" + org + "/vouchers", creds, org, voucherBytes, &importResponse)
	if !quieter {
		msgPrinter.Printf("Voucher imported. Node id: %s, token: %s", importResponse.NodeId, importResponse.NodeToken)
		msgPrinter.Println()
	}

	// Pre-create the node resource in the exchange, so it is already there when hzn register is run on the SDO device
	// Doing the equivalent of: hzn exchange node create -org "org" -n "$nodeId:$nodeToken" -u "user:pw" (with optional pattern)
	//todo: try to get the device arch from the voucher
	//exchange.NodeCreate(org, "", importResponse.NodeId, importResponse.NodeToken, userCreds, "amd64", "", persistence.DEVICE_TYPE_DEVICE, true)
	NodeAddDevice(org, importResponse.NodeId, importResponse.NodeToken, userCreds, "", patternName, quieter)

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
func SdoPostVoucher(url, creds, org string, requestBodyBytes []byte, respBody *ImportResponse) {
	msgPrinter := i18n.GetMessagePrinter()
	method := http.MethodPost
	apiMsg := method + " " + url
	httpClient := cliutils.GetHTTPClient(config.HTTPRequestTimeoutS)
	// Note: need to pass the request body in as a string, not []byte, so that it sets header: Content-Type, application/json
	resp := cliutils.InvokeRestApi(httpClient, method, url, creds, string(requestBodyBytes), "SDO Owner Service", apiMsg)
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
	} else if httpCode != 201 {
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
	cliutils.ExchangePutPost("Exchange", http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+org+"/nodes/"+nodeId+"?"+cliutils.NOHEARTBEAT_PARAM, cliutils.OrgAndCreds(org, userPw), []int{201}, putNodeReqBody, nil)
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
	cliutils.ExchangePutPost("Exchange", http.MethodPut, exchangeUrl, "orgs/"+org+"/nodes/"+node+"/policy"+"?"+cliutils.NOHEARTBEAT_PARAM, cliutils.OrgAndCreds(org, credToUse), []int{201}, policyFile, nil)

	//msgPrinter.Printf("Node policy updated.")
	//msgPrinter.Println()
}
