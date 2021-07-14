package secrets_manager

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/i18n"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

type SecretResponse struct {
	Exists bool `json:"exists"`
}

// Parses the raw bytes into the given structure, then prints the parsed structure
func printResponse(resp []byte, structure interface{}) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// parse into the structure type
	perr := json.Unmarshal(resp, &structure)
	if perr != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal REST API response: %v", perr))
	}

	// print the parsed structure
	jsonBytes, jerr := json.MarshalIndent(structure, "", cliutils.JSON_INDENT)
	if jerr != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'exchange node list' output: %v", jerr))
	}
	fmt.Printf("%s\n", jsonBytes)
}

// Retries a query (in the form a function returning the http code) <retryCount> times with <retryInterval> second delays
// as long as a 503 is returned. If a code other than 503 is returned, or the number of retries is reached, the code of the final
// query will be returned.
func queryWithRetry(query func() int, retryCount, retryInterval int) (httpCode int) {

	// on a 503, we want to retry a small number of times
	for i := 0; i < retryCount; i++ {
		httpCode = query()
		if httpCode != 503 {
			return httpCode
		}
		cliutils.Verbose("Vault component not found in the management hub. Retrying...")
		time.Sleep(time.Duration(retryInterval) * time.Second)
	}

	// maximum number of retries
	return httpCode
}

// If secretName is empty, lists all the org level secrets and non-empty directories for the specified org in the secrets manager
// If secretName is specified, prints a json object indicating whether the given secret exists or not in the secrets manager for the org
// If the name provided is a directory, lists all the secrets in the directory.
func SecretList(org, credToUse, secretName string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// get rid of trailing / from secret name
	if strings.HasSuffix(secretName, "/") {
		secretName = secretName[:len(secretName)-1]
	}

	// query the agbot secure api
	var resp []byte
	listQuery := func() int {
		return cliutils.AgbotGet("org"+cliutils.AddSlash(org)+"/secrets"+cliutils.AddSlash(secretName), cliutils.OrgAndCreds(org, credToUse),
			[]int{200, 401, 403, 404, 503}, &resp)
	}
	retCode := queryWithRetry(listQuery, 3, 1)

	// check if listing org/user secrets

	// listing org secrets - empty name
	isSecretDirectory := secretName == ""

	// listing user secrets - user/<user>
	if !isSecretDirectory {
		nameParts := strings.Split(secretName, "/")
		partsLength := len(nameParts)
		isSecretDirectory = nameParts[0] == "user" && partsLength == 2
	}

	// parse and print the response
	if retCode == 401 || retCode == 403 || retCode == 503 {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, string(resp))
	} else if isSecretDirectory { // list org/user secrets
		if retCode == 200 {
			// list secrets
			var secrets []string
			printResponse(resp, &secrets)
		} else if retCode == 404 {
			// no secrets found in the organization or user's directory
			fmt.Println("[]")
		}
	} else { // secret name provided, exists/does not exist
		if retCode == 200 {
			var secret SecretResponse
			printResponse(resp, &secret)
		} else if retCode == 404 {
			// secret doesn't exist, output exists: false for consistency
			secretDNE := SecretResponse{false}
			jsonBytes, jerr := json.MarshalIndent(secretDNE, "", cliutils.JSON_INDENT)
			if jerr != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'exchange node list' output: %v", jerr))
			}
			fmt.Printf("%s\n", jsonBytes)
		}
	}

}

// Adds or updates a secret in the secrets manager. Secret names are unique, if a secret already exists with the same name, the user
// will be prompted if they want to overwrite the existing secret, unless the secretOverwrite flag is set
func SecretAdd(org, credToUse, secretName, secretFile, secretKey, secretDetail string, secretOverwrite bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// check the input
	if secretFile != "" && secretDetail != "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("-f is mutually exclusive with --secretDetail."))
	}
	if secretFile == "" && secretDetail == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Must specify either -f or --secretDetail."))
	}

	// check if the secret already exists by querying the api
	secretExists := false
	var resp []byte
	checkQuery := func() int {
		return cliutils.AgbotGet("org"+cliutils.AddSlash(org)+"/secrets"+cliutils.AddSlash(secretName), cliutils.OrgAndCreds(org, credToUse),
			[]int{200, 401, 403, 404, 503}, &resp)
	}
	retCode := queryWithRetry(checkQuery, 3, 1)

	// check the response
	if retCode == 401 || retCode == 403 || retCode == 503 {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, string(resp))
	} else if retCode == 200 {
		var secret SecretResponse
		perr := json.Unmarshal(resp, &secret)
		if perr != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal REST API response: %v", perr))
		}
		secretExists = secret.Exists
	}

	// parse the key and details
	var newSecret struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	newSecret.Key = secretKey
	if secretDetail != "" {
		newSecret.Value = secretDetail
	} else {
		// parse in a file as bytes for the secret details
		var secretBytes []byte
		var err error
		if secretFile == "-" {
			secretBytes, err = ioutil.ReadAll(os.Stdin)
		} else {
			secretBytes, err = ioutil.ReadFile(secretFile)
		}
		if err != nil {
			cliutils.Fatal(cliutils.FILE_IO_ERROR, msgPrinter.Sprintf("reading %s failed: %v", secretFile, err))
		}
		newSecret.Value = string(secretBytes)
	}

	// prompt for overwrite if the secret already exists
	if secretExists && !secretOverwrite {
		cliutils.ConfirmRemove(msgPrinter.Sprintf("Secret \"%s\" already exists in the secrets manager. Do you want to overwrite?", secretName))
	}

	// add/replace the secret to the secrets manager
	var resp2 []byte
	addQuery := func() int {
		return cliutils.AgbotPutPost(http.MethodPut, "org"+cliutils.AddSlash(org)+"/secrets"+cliutils.AddSlash(secretName),
			cliutils.OrgAndCreds(org, credToUse), []int{201, 401, 403, 503}, newSecret, &resp2)
	}
	retCode = queryWithRetry(addQuery, 3, 1)

	// output success or failure
	if retCode == 201 {
		fmt.Printf("Secret \"%s\" successfully added to the secrets manager.\n", secretName)
	} else if retCode == 401 || retCode == 403 || retCode == 503 {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, string(resp2))
	}

}

// Removes a secret in the secrets manager. If the secret does not exist, an error (fatal) is raised
func SecretRemove(org, credToUse, secretName string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// query the agbot secure api
	removeQuery := func() int {
		return cliutils.AgbotDelete("org"+cliutils.AddSlash(org)+"/secrets"+cliutils.AddSlash(secretName), cliutils.OrgAndCreds(org, credToUse),
			[]int{204, 401, 403, 404, 503})
	}
	retCode := queryWithRetry(removeQuery, 3, 1)

	// output success or failure
	if retCode == 204 {
		fmt.Printf("Secret \"%v\" successfully deleted from the secrets manager.\n", secretName)
	} else if retCode == 401 || retCode == 403 {
		user, _ := cliutils.SplitIdToken(credToUse)
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Invalid credentials. User \"%s\" cannot access \"%s\".\n", user, secretName))
	} else if retCode == 404 {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Secret \"%s\" not found in the secrets manager, nothing to delete.\n", secretName))
	} else if retCode == 503 {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Vault component not found in the management hub."))
	}
}
