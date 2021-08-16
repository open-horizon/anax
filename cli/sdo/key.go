package sdo

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/i18n"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

type Key struct {
	Name      string `json:"name"`
	Orgid     string `json:"orgid"`
	Owner     string `json:"owner"`
	Filename  string `json:"filename"`
	IsExpired bool   `json:"isExpired"`
}

type KeyFile struct {
	Name       string `json:"key_name"`
	CommonName string `json:"common_name"`
	Email      string `json:"email_name"`
	Company    string `json:"company_name"`
	Country    string `json:"country_name"`
	State      string `json:"state_name"`
	Locale     string `json:"locale_name"`
}

// Create key in SDO owner services from given keyFile
func KeyCreate(org, userCreds string, keyFile *os.File, outputFile string, overwrite bool) {
	defer keyFile.Close()
	msgPrinter := i18n.GetMessagePrinter()
	cliutils.Verbose(msgPrinter.Sprintf("Importing key file name: %s", keyFile.Name()))

	// Don't attempt to create key if unable to put key in specified file
	if !overwrite {
		if _, err := os.Stat(outputFile); !os.IsNotExist(err) {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("File %s already exists. Please specify a different file path or file name. To overwrite the existing file, use the '--overwrite' flag.", outputFile))
		}
	}

	// Determine key file type, and handle it accordingly
	var returnBody []byte
	if strings.HasSuffix(keyFile.Name(), ".json") {
		returnBody = import1Key(org, userCreds, bufio.NewReader(keyFile), keyFile.Name())
	} else {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("unsupported key file type extension: %s", keyFile.Name()))
	}

	var fileExtension string
	if outputFile != "" {
		fmt.Printf("Key \"%s\" successfully added to the SDO owner services.\n", keyFile.Name())
		fileName := cliutils.DownloadToFile(outputFile, keyFile.Name(), returnBody, fileExtension, 0600, overwrite)
		fmt.Printf("Key \"%s\" successfully downloaded to %s from the SDO owner services.\n", keyFile.Name(), fileName)
	} else {
		fmt.Printf("%s\n", string(returnBody))
	}
}

// List keys that are stored in SDO owner services
func KeyList(org, userCreds, keyName string) {
	msgPrinter := i18n.GetMessagePrinter()
	var respBodyBytes []byte
	var emptyBody []byte
	var apiMsg string
	var emptyKeyName string

	// Retrieve keys from SDO owner services API
	cliutils.Verbose(msgPrinter.Sprintf("Listing SDO keys."))
	respBodyBytes, apiMsg = sendSdoKeysApiRequest(org, userCreds, emptyKeyName, http.MethodGet, emptyBody, []int{200, 404})

	// Put response in Key object for formatting
	output := []Key{}
	err := json.Unmarshal(respBodyBytes, &output)
	if err != nil {
		cliutils.Fatal(cliutils.HTTP_ERROR, msgPrinter.Sprintf("json unmarshalling HTTP response '%s' from %s: %v", string(respBodyBytes), apiMsg, err))
	}

	// look for given keyName
	var jsonBytes []byte
	if keyName != "" {
		var foundKey Key
		found := false
		for i, key := range output {
			if key.Name == keyName {
				foundKey = output[i]
				found = true
				break
			}
		}
		if !found {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("SDO key name %s not found", keyName))
		} else {
			jsonBytes, err = json.MarshalIndent(foundKey, "", cliutils.JSON_INDENT)
			if err != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn sdo keys list' output: %v", err))
			}
		}

	// use all the keys in SDO owner services
	} else {
		jsonBytes, err = json.MarshalIndent(output, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn sdo keys list' output: %v", err))
		}
	}

	// List key(s) on screen
	fmt.Printf("%s\n", jsonBytes)
}

// Download specified key from SDO owner services to file
func KeyDownload(org, userCreds, keyName, outputFile string, overwrite bool) {
	msgPrinter := i18n.GetMessagePrinter()
	var emptyBody []byte
	var fileExtension string

	// Request key information from SDO owner services API
	cliutils.Verbose(msgPrinter.Sprintf("Downloading SDO key \"%s\".", keyName))
	respBodyBytes, _ := sendSdoKeysApiRequest(org, userCreds, keyName, http.MethodGet, emptyBody, []int{200})

	// Download response body directly to file
	if outputFile != "" {
		fileName := cliutils.DownloadToFile(outputFile, keyName, respBodyBytes, fileExtension, 0600, overwrite)
		fmt.Printf("Key \"%s\" successfully downloaded to %s from the SDO owner services.\n", keyName, fileName)
	} else {
		// List keys on screen
		fmt.Printf("%s\n", respBodyBytes)
	}
}

// Remove sepcified key form SDO owner services
func KeyRemove(org, userCreds, keyName string) {
	msgPrinter := i18n.GetMessagePrinter()
	cliutils.Verbose(msgPrinter.Sprintf("Removing SDO key \"%s\".", keyName))

	// Tell SDO owner services API to remove specified key, if it exists
	var emptyBody []byte
	sendSdoKeysApiRequest(org, userCreds, keyName, http.MethodDelete, emptyBody, []int{204})

	fmt.Printf("Key \"%s\" successfully deleted from the SDO owner services.\n", keyName)
}

// Download a sample key template. If file path specified, template will be written to given file, otherwise "sample-sdo-key.json"
func KeyNew(outputFile string, overwrite bool) {
	msgPrinter := i18n.GetMessagePrinter()
	cliutils.Verbose(msgPrinter.Sprintf("Creating SDO key template at \"%s\".", outputFile))

	// Create template json by marshalling empty KeyFile struct
	body, err := json.MarshalIndent(KeyFile{}, "", cliutils.JSON_INDENT)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("parsing the json: %v", err))
	}

	// Download the template to file
	if outputFile != "" {
		fileExtension := ".json"
		defaultFileName := "sample-sdo-key"
		fileName := cliutils.DownloadToFile(outputFile, defaultFileName, body, fileExtension, 0600, overwrite)
		fmt.Printf("Key template successfully written to %s.\n", fileName)
	} else {
		fmt.Printf("%s\n", body)
	}
}

// Helper function to send API requests to SDO owner services API
func sendSdoKeysApiRequest(org, userCreds, keyName, method string, body interface{}, goodHttpCodes []int) ([]byte, string) {
	msgPrinter := i18n.GetMessagePrinter()

	// setup HTTP parameters and URL
	var respBodyBytes []byte
	var sdoURL string
	url := cliutils.GetSdoSvcUrl()

	sdoURL = url + "/orgs/" + org + "/keys" + cliutils.AddSlash(keyName)

	creds := cliutils.OrgAndCreds(org, userCreds)
	apiMsg := method + " " + sdoURL
	httpClient := cliutils.GetHTTPClient(config.HTTPRequestTimeoutS)

	resp := cliutils.InvokeRestApi(httpClient, method, sdoURL, creds, body, "SDO Owner Service", apiMsg)
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
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Invalid key name. Key \"%s\" does not exist in org \"%s\".\n", keyName, org))
	} else if httpCode == 400 && method == http.MethodPost {
		key, ok := body.(Key)
		if ok {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Invalid key file. Key \"%s\" already exists in SDO owner services for org \"%s\".\n", key, org))
		} else {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Invalid key file. Key already exists in SDO owner services for org \"%s\".", org))
		}
	} else if httpCode == 401 {
		user, _ := cliutils.SplitIdToken(userCreds)
		if keyName == "" {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Invalid credentials. User \"%s\" cannot access keys in org \"%s\" with given credentials.\n", user, org))
		} else {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Invalid credentials. User \"%s\" cannot access key \"%s\" in org \"%s\" with given credentials.\n", user, keyName, org))
		}
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

// Helper function to POST a key to SDO owner services
func import1Key(org, userCreds string, keyFileReader io.Reader, keyFileName string) []byte{
	msgPrinter := i18n.GetMessagePrinter()

	// Parse the voucher so we can tell them what we are doing
	key := KeyFile{}
	keyBytes, err := ioutil.ReadAll(keyFileReader)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("reading the bytes from %s: %v", keyFileName, err))
	} else if err = json.Unmarshal(keyBytes, &key); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("parsing the json from %s: %v", keyFileName, err))
	}

	// Check for empty fields in key file
	if err := checkEmptyKeyFields(key); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("given key %s has missing fields:\n%s", keyFileName, err))
	}

	// Remarshal key so that it is always in the right format
	if keyBytes, err = json.Marshal(key); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("parsing the json from %s: %v", keyFileName, err))
	}

	// Import the key to the SDO owner service
	var emptyName string
	cliutils.Verbose(msgPrinter.Sprintf("JSON Body:%s\n", string(keyBytes)))
	respBodyBytes, _ := sendSdoKeysApiRequest(org, userCreds, emptyName, http.MethodPost, key, []int{201})

	return respBodyBytes
}

// Check if keyfile stored in KeyFile struct as any empty fields
func checkEmptyKeyFields(key KeyFile) error {
	errMsg := ""
	if key.Name == "" {
		errMsg += "\tfield \"key_name\" is missing\n"
	}
	if key.CommonName == "" {
		errMsg += "\tfield \"common_name\" is missing\n"
	}
	if key.Email == "" {
		errMsg += "\tfield \"email_name\" is missing\n"
	}
	if key.Company == "" {
		errMsg += "\tfield \"company_name\" is missing\n"
	}
	if key.Country == "" {
		errMsg += "\tfield \"country_name\" is missing\n"
	}
	if key.State == "" {
		errMsg += "\tfield \"state_name\" is missing\n"
	}
	if key.Locale == "" {
		errMsg += "\tfield \"locale_name\" is missing\n"
	}
	if errMsg != "" {
		errMsg += "Please fill these and try again.\n"
		return errors.New(errMsg)
	}
	return nil
}
