package key

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/rsapss-tool/generatekeys"
	"time"
)

func List() {
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
}


// Create generates a private/public key pair
func Create(x509Org, x509CN, outputDir string, keyLength, daysValid int) {
	// Note: the cli parse already verifies outputDir exists and keyLength and daysValid are ints
	fmt.Println("Creating key pair...")
	newKeys, err := generatekeys.Write(outputDir, keyLength, x509CN, x509Org, time.Now().AddDate(0, 0, daysValid))
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "failed to create a new key pair: %v", err)
	}
	fmt.Println("Created keys:")
	for _, key := range newKeys {
		fmt.Printf("\t%v\n", key)
	}
}
