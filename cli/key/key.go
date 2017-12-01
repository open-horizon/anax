package key

import (
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"encoding/json"
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
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'show pem' output: %v", err)
	}
	fmt.Printf("%s\n", jsonBytes)
}
