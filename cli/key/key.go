package key

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/rsapss-tool/generatekeys"
	"time"
	"net/http"
	"path/filepath"
	"strings"
)

func List(keyName string) {
	if keyName == "" {
		// Getting all of the keys only returns the names
		apiOutput := make(map[string][]string, 0)
		// Note: it is allowed to get /publickey before post /node is called, so we don't have to check for that error
		cliutils.HorizonGet("publickey", []int{200}, &apiOutput)
		var ok bool
		if _, ok = apiOutput["pem"]; !ok {
			cliutils.Fatal(cliutils.HTTP_ERROR, "horizon api publickey output did not include 'pem' key")
		}
		jsonBytes, err := json.MarshalIndent(apiOutput["pem"], "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'key list' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		// Get the content of 1 key, which is not json
		var apiOutput string
		cliutils.HorizonGet("publickey/"+keyName, []int{200}, &apiOutput)
		fmt.Printf("%s", apiOutput)
	}
}


// Create generates a private/public key pair
func Create(x509Org, x509CN, outputDir string, keyLength, daysValid int, importKey bool) {
	// Note: the cli parse already verifies outputDir exists and keyLength and daysValid are ints
	fmt.Println("Creating key pair, this may take a minute...")
	newKeys, err := generatekeys.Write(outputDir, keyLength, x509CN, x509Org, time.Now().AddDate(0, 0, daysValid))
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "failed to create a new key pair: %v", err)
	}
	var pubKeyName string 		// capture this in case they want us to import it
	fmt.Println("Created keys:")
	for _, key := range newKeys {
		fmt.Printf("\t%v\n", key)
		if strings.Contains(key, "public") {	// this seems like a better check than blindly getting the 2nd key in the list
			pubKeyName = key
		}
	}

	// Import the key to anax if they requested that
	if importKey {
		if pubKeyName == "" {
			cliutils.Fatal(cliutils.INTERNAL_ERROR, "asked to import the created public key, but can not determine the name.")
		}
		Import(pubKeyName)
		fmt.Printf("%s imported to the Horizon agent", pubKeyName)
	}
}


func Import(pubKeyFile string) {
	// Note: the CLI framework already verified the file exists
	bodyBytes := cliutils.ReadFile(pubKeyFile)
	baseName := filepath.Base(pubKeyFile)
	cliutils.HorizonPutPost(http.MethodPut, "publickey/"+baseName, []int{201, 200}, bodyBytes)
}


func Remove(keyName string) {
	cliutils.HorizonDelete("publickey/"+keyName, []int{200, 204})
	fmt.Printf("Public key '%s' removed from the Horizon agent.\n", keyName)
}