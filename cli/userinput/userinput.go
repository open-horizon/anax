package userinput

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/policy"
	"net/http"
)

//Display a list of the current userInputs of the node
func List() {
	var inputs []policy.UserInput
	cliutils.HorizonGet("node/userinput", []int{200}, &inputs, false)

	output, err := cliutils.DisplayAsJson(inputs)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "Unable to marshal userinput object: %v", err)
	}
	fmt.Println(output)
}

//Add or overwrite the userinputs for this node
func Add(filePath string) {
	var inputs []policy.UserInput
	inputString := cliconfig.ReadJsonFileWithLocalConfig(filePath)

	err := json.Unmarshal([]byte(inputString), &inputs)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "Error unmarshaling userInput json file: %v", err)
	}

	cliutils.HorizonPutPost(http.MethodPost, "node/userinput", []int{200, 201}, inputs)
	fmt.Println("Horizon node user inputs updated.")
}

//Update the userinputs for this node
func Update(filePath string) {
	var inputs []policy.UserInput
	inputString := cliconfig.ReadJsonFileWithLocalConfig(filePath)

	err := json.Unmarshal([]byte(inputString), &inputs)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "Error unmarshaling userInput json file: %v", err)
	}

	cliutils.HorizonPutPost(http.MethodPatch, "node/userinput", []int{200, 201}, inputs)
	fmt.Println("Horizon node user inputs updated.")
}

//Remove the user inputs for this nose
func Remove(force bool) {
	if !force {
		cliutils.ConfirmRemove("Are you sure you want to remove the node user inputs?")
	}

	cliutils.HorizonDelete("node/userinput", []int{200, 204}, false)

	fmt.Println("Horizon user inputs removed.")
}
