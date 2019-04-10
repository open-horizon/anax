package key

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/rsapss-tool/generatekeys"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

type KeyPairSimpleOutput struct {
	ID               string `json:"id"`
	CommonName       string `json:"common_name"`
	OrganizationName string `json:"organization_name"`
	SerialNumber     string `json:"serial_number"`
	NotValidBefore   string `json:"not_valid_before"`
	NotValidAfter    string `json:"not_valid_after"`
}

type KeyList struct {
	Pem []string `json:"pem"`
}

func List(keyName string, listAll bool) {
	if keyName == "" && listAll {
		var apiOutput KeyList
		cliutils.HorizonGet("trust", []int{200}, &apiOutput, false)
		jsonBytes, err := json.MarshalIndent(apiOutput.Pem, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'key list' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	} else if keyName == "" {
		// Getting all of the keys only returns the names
		var apiOutput map[string][]api.KeyPairSimpleRecord
		// Note: it is allowed to get /trust before post /node is called, so we don't have to check for that error
		cliutils.HorizonGet("trust?verbose=true", []int{200}, &apiOutput, false)
		cliutils.Verbose("apiOutput: %v", apiOutput)

		var output []api.KeyPairSimpleRecord
		var ok bool
		if output, ok = apiOutput["pem"]; !ok {
			cliutils.Fatal(cliutils.HTTP_ERROR, "horizon api trust output did not include 'pem' key")
		}

		certsSimpleOutput := []KeyPairSimpleOutput{}
		for _, kps := range output {
			certsSimpleOutput = append(certsSimpleOutput, KeyPairSimpleOutput{
				ID:               kps.ID,
				SerialNumber:     kps.SerialNumber,
				CommonName:       kps.SubjectNames["commonName (CN)"].(string),
				OrganizationName: kps.SubjectNames["organizationName (O)"].(string),
				NotValidBefore:   kps.NotValidBefore.String(),
				NotValidAfter:    kps.NotValidAfter.String(),
			})
		}

		jsonBytes, err := json.MarshalIndent(certsSimpleOutput, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'key list' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		// Get the content of 1 key, which is not json
		var apiOutput string
		cliutils.HorizonGet("trust/"+keyName, []int{200}, &apiOutput, false)
		fmt.Printf("%s", apiOutput)
	}
}

// Create generates a private/public key pair
func Create(x509Org, x509CN, outputDir string, keyLength, daysValid int, importKey bool) {
	// Note: the cli parse already verifies outputDir exists and keyLength and daysValid are ints
	fmt.Println("Creating RSA PSS private and public keys, and an x509 certificate for distribution. This is a CPU-intensive operation and, depending on key length and platform, may take a while. Key generation on an amd64 or ppc64 system using the default key length will complete in less than 1 minute.")
	newKeys, err := generatekeys.Write(outputDir, keyLength, x509CN, x509Org, time.Now().AddDate(0, 0, daysValid))
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "failed to create a new key pair: %v", err)
	}
	var pubKeyName string // capture this in case they want us to import it
	fmt.Println("Created keys:")
	for _, key := range newKeys {
		fmt.Printf("\t%v\n", key)
		if strings.Contains(key, "public") { // this seems like a better check than blindly getting the 2nd key in the list
			pubKeyName = key
		}
	}

	// Import the key to anax if they requested that
	if importKey {
		if pubKeyName == "" {
			cliutils.Fatal(cliutils.INTERNAL_ERROR, "asked to import the created public key, but can not determine the name.")
		}
		Import(pubKeyName)
		fmt.Printf("%s imported to the Horizon agent\n", pubKeyName)
	}
}

func Import(pubKeyFile string) {
	// Note: the CLI framework already verified the file exists
	bodyBytes := cliutils.ReadFile(pubKeyFile)
	baseName := filepath.Base(pubKeyFile)
	cliutils.HorizonPutPost(http.MethodPut, "trust/"+baseName, []int{201, 200}, bodyBytes)
}

func Remove(keyName string) {
	cliutils.HorizonDelete("trust/"+keyName, []int{200, 204})
	fmt.Printf("Public key '%s' removed from the Horizon agent.\n", keyName)
}
