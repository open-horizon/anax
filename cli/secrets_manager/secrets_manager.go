package secrets_manager

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/agreementbot/secrets"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/i18n"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
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

	// replace nil with empty slice if needed
	if _, ok := structure.([]string); ok && structure == nil {
		structure = make([]string, 0)
	}

	// print the parsed structure
	jsonBytes, jerr := json.MarshalIndent(structure, "", cliutils.JSON_INDENT)
	if jerr != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'agbot API' output: %v", jerr))
	}
	fmt.Printf("%s\n", jsonBytes)
}

// Retries a query (in the form a function returning the http code) <retryCount> times with <retryInterval> second delays
// as long as a 503 is returned. If a code other than 503 is returned, or the number of retries is reached, the code of the final
// query will be returned.
func queryWithRetry(query func() int, retryCount, retryInterval int) (httpCode int) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// on a 503, we want to retry a small number of times
	for i := 0; i < retryCount; i++ {
		httpCode = query()
		if httpCode != 503 {
			return httpCode
		}
		cliutils.Verbose(msgPrinter.Sprintf("Vault component not found in the management hub. Retrying..."))
		time.Sleep(time.Duration(retryInterval) * time.Second)
	}

	// maximum number of retries
	return httpCode
}

// If secretName is empty
// - nodeId is empty: lists all the org level secrets and non-empty directories for the specified org in the secrets manager
// - else: list all the node secrets for the specified org
// If secretName is
// - nodeId is empty: prints a json object indicating whether the given secret exists or not in the secrets manager for the org
// - else: prints node secrets as a json object indicating whether the given secret exists or not
// If the name provided is a directory,
// - nodeId is empty: lists all the secrets in the directory
// - else: list all the node secrets for the given directory
func SecretList(org, credToUse, secretName, secretNodeId string, listAll bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// get rid of trailing / from secret name
	if strings.HasSuffix(secretName, "/") {
		secretName = secretName[:len(secretName)-1]
	}

	if strings.HasPrefix(secretName, "/") {
		secretName = strings.TrimPrefix(secretName, "/")
	}

	if !strings.Contains(secretName, "node/") && secretNodeId != "" {
		// add node/{nodeID} to the path
		secretName = getSecretPathForNodeLevelSecret(secretName, secretNodeId)
	}
	// if given secretName := "", nodeId is specified, then secretName will be convert to "node/{nodeId}"
	// if given secretName is a directory: "user/userdevadmin", nodeId is specified, then secretName will be converted to "user/{userId}/node/{nodeId}"
	// if given secretName is "test-password", nodeId is specified, then secretname will be converted to "node/{nodeId}/test-password"
	// if given secretName is "user/userdevadmin/test-password", nodeId is specified, then secretName will be converted to "user/{userId}/node/{nodeId}/test-password":

	// query the agbot secure api
	var resp []byte
	listQuery := func() int {
		// call LIST: org/secrets/node/{nodeId}
		//            org/secrets/user/{userId}/node/{nodeId}
		//            org/secrets/node/{nodeId}/test-password
		//            org/secrets/user/{userId}/node/{nodeId}/test-password
		if listAll {
			return cliutils.AgbotList("org"+cliutils.AddSlash(org)+"/allsecrets", cliutils.OrgAndCreds(org, credToUse),
				[]int{200, 400, 401, 403, 404, 503, 504}, &resp)
		} else {
			return cliutils.AgbotList("org"+cliutils.AddSlash(org)+"/secrets"+cliutils.AddSlash(secretName), cliutils.OrgAndCreds(org, credToUse),
				[]int{200, 400, 401, 403, 404, 503, 504}, &resp)
		}
	}
	retCode := queryWithRetry(listQuery, 3, 1)

	// check if listing org/user secrets

	// listing org secrets - empty name
	isSecretDirectory := secretName == ""

	// listing user secrets - user/<user>
	// listing org node secrets - node/<nodeId>
	// listing user node secrets - user/<user>/node/<nodeId>
	if !isSecretDirectory {
		nameParts := strings.Split(secretName, "/")
		partsLength := len(nameParts)
		isUserSecretDirectory := nameParts[0] == "user" && (partsLength == 2 || partsLength == 4)
		isOrgNodeSecretDirectory := nameParts[0] == "node" && partsLength == 2
		isSecretDirectory = isUserSecretDirectory || isOrgNodeSecretDirectory
	}

	// parse and print the response
	if retCode == 400 || retCode == 401 || retCode == 403 || retCode == 503 || retCode == 504 {
		respString, _ := strconv.Unquote(string(resp))
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, respString)
	} else if isSecretDirectory {
		// list org/user secrets
		if retCode == 404 || strings.EqualFold("null", string(resp)) {
			// no secrets found in the organization or user's directory
			fmt.Println("[]")
		} else if retCode == 200 {
			// list secrets
			var secrets []string
			printResponse(resp, &secrets)
		}
	} else {
		// secret name provided, exists/does not exist
		// if the secret does not exist, exit with a non-zero return code
		if retCode == 200 {
			var secret SecretResponse
			printResponse(resp, &secret)
			if !secret.Exists {
				os.Exit(1)
			}
		} else if retCode == 404 {
			// secret doesn't exist, output exists: false for consistency
			secretDNE := SecretResponse{false}
			jsonBytes, jerr := json.MarshalIndent(secretDNE, "", cliutils.JSON_INDENT)
			if jerr != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'agbot API' output: %v", jerr))
			}
			fmt.Printf("%s\n", jsonBytes)
			os.Exit(1)
		}
	}

}

// Adds or updates a secret in the secrets manager. Secret names are unique, if a secret already exists with the same name, the user
// will be prompted if they want to overwrite the existing secret, unless the secretOverwrite flag is set
func SecretAdd(org, credToUse, secretName, secretNodeId, secretFile, secretKey, secretDetail string, secretOverwrite bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// get rid of trailing / from secret name
	if strings.HasSuffix(secretName, "/") {
		secretName = secretName[:len(secretName)-1]
	}

	if strings.HasPrefix(secretName, "/") {
		secretName = strings.TrimPrefix(secretName, "/")
	}

	if !strings.Contains(secretName, "node/") && secretNodeId != "" {
		// add node/{nodeID} to the path
		secretName = getSecretPathForNodeLevelSecret(secretName, secretNodeId)
	}

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
		return cliutils.AgbotList("org"+cliutils.AddSlash(org)+"/secrets"+cliutils.AddSlash(secretName), cliutils.OrgAndCreds(org, credToUse),
			[]int{200, 400, 401, 403, 404, 503}, &resp)
	}
	retCode := queryWithRetry(checkQuery, 3, 1)

	// check the response
	if retCode == 400 || retCode == 401 || retCode == 403 || retCode == 503 {
		respString, _ := strconv.Unquote(string(resp))
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, strings.Replace(respString, " list ", " add ", 1))
	} else if retCode == 200 {
		var secret SecretResponse
		perr := json.Unmarshal(resp, &secret)
		if perr != nil {
			// API returned an error or list
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Incorrect secret name given: %v", secretName))
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
			cliutils.OrgAndCreds(org, credToUse), []int{201, 400, 401, 403, 503}, newSecret, &resp2)
	}
	retCode = queryWithRetry(addQuery, 3, 1)

	// output success or failure
	if retCode == 201 {
		msgPrinter.Printf("Secret \"%s\" successfully added to the secrets manager.", secretName)
		msgPrinter.Println()
	} else if retCode == 400 || retCode == 401 || retCode == 403 || retCode == 503 {
		respString, _ := strconv.Unquote(string(resp2))
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, respString)
	}

}

// Removes a secret in the secrets manager. If the secret does not exist, an error (fatal) is raised
func SecretRemove(org, credToUse, secretName, secretNodeId string, forceRemoval bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// get rid of trailing / from secret name
	if strings.HasSuffix(secretName, "/") {
		secretName = secretName[:len(secretName)-1]
	}

	if strings.HasPrefix(secretName, "/") {
		secretName = strings.TrimPrefix(secretName, "/")
	}

	if !strings.Contains(secretName, "node/") && secretNodeId != "" {
		// add node/{nodeID} to the path
		secretName = getSecretPathForNodeLevelSecret(secretName, secretNodeId)
	}

	// confirm secret removal
	if !forceRemoval {
		cliutils.ConfirmRemove(msgPrinter.Sprintf("Are you sure you want to remove secret %s from the secrets manager?", secretName))
	}

	// query the agbot secure api
	removeQuery := func() int {
		return cliutils.AgbotDelete("org"+cliutils.AddSlash(org)+"/secrets"+cliutils.AddSlash(secretName), cliutils.OrgAndCreds(org, credToUse),
			[]int{204, 400, 401, 403, 404, 503})
	}
	retCode := queryWithRetry(removeQuery, 3, 1)

	// output success or failure
	if retCode == 204 {
		msgPrinter.Printf("Secret \"%v\" successfully deleted from the secrets manager.", secretName)
		msgPrinter.Println()
	} else if retCode == 400 {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Bad request, secret name \"%s\" invalid.", secretName))
	} else if retCode == 401 || retCode == 403 {
		user, _ := cliutils.SplitIdToken(credToUse)
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Permission denied, user \"%s\" cannot access \"%s\".\n", user, secretName))
	} else if retCode == 404 {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Secret \"%s\" not found in the secrets manager, nothing to remove.\n", secretName))
	} else if retCode == 503 {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Vault component not found in the management hub."))
	}
}

// Pulls secret details from the secrets manager. If the secret does not exist, an error (fatal) is raised
func SecretRead(org, credToUse, secretName, secretNodeId string) {
	// get rid of trailing / from secret name
	if strings.HasSuffix(secretName, "/") {
		secretName = secretName[:len(secretName)-1]
	}

	if strings.HasPrefix(secretName, "/") {
		secretName = strings.TrimPrefix(secretName, "/")
	}

	if !strings.Contains(secretName, "node/") && secretNodeId != "" {
		// add node/{nodeID} to the path
		secretName = getSecretPathForNodeLevelSecret(secretName, secretNodeId)
	}

	// query the agbot secure api
	var resp []byte
	listQuery := func() int {
		return cliutils.AgbotGet("org"+cliutils.AddSlash(org)+"/secrets"+cliutils.AddSlash(secretName), cliutils.OrgAndCreds(org, credToUse),
			[]int{200, 400, 401, 403, 404, 503}, &resp)
	}
	retCode := queryWithRetry(listQuery, 3, 1)

	// parse and print the response
	if retCode == 400 || retCode == 401 || retCode == 403 || retCode == 404 || retCode == 503 {
		respString, _ := strconv.Unquote(string(resp))
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, respString)
	} else {
		// retCode == 200
		var secretDetails secrets.SecretDetails

		// parse and print the response
		printResponse(resp, &secretDetails)
	}

}

func getSecretPathForNodeLevelSecret(secretName string, secretNodeId string) string {
	secretName = strings.TrimSpace(secretName)
	secretName = strings.TrimPrefix(secretName, "/")
	if secretName == "" {
		secretName = fmt.Sprintf("node/%v", secretNodeId)
	} else {
		parts := strings.Split(secretName, "/")
		if len(parts) == 1 {
			// org level, secretName: "test-password"
			secretName = fmt.Sprintf("node/%v/%v", secretNodeId, secretName)
		} else if len(parts) == 2 {
			// list secret given a directory, eg: user/userdevadmin
			secretName = fmt.Sprintf("%v/node/%v", secretName, secretNodeId)
		} else if len(parts) == 3 {
			// user level
			// parts: [user, ${userid}, test-password]
			secretName = fmt.Sprintf("%v/%v/node/%v/%v", parts[0], parts[1], secretNodeId, parts[2])
		}
	}

	return secretName
}
