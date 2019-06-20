package userinput

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/policy"
	"net/http"
)

const USERINPUT_TEMPLATE_OBJECT = `[                                /* A list of objects, each one containing the user inputs required for a specified service. */
  {
    "serviceOrgid": "",          /* The horizon org of the specified service. */
    "serviceUrl": "",            /* The unique string used to identify the specified service. */
    "serviceArch": "",           /* The service architecture that these inputs apply to. Omit or leave blank to mean all architectures. */
    "serviceVersionRange": "",   /* The service versions that these inputs apply to. Omit or specify "[0.0.0,INFINITY)" to mean all versions. */
    "inputs": [                  /* A list of objects with the names and values for the user inputs used by this service. */
      {
        "name": "",
        "value": null
      }
    ]
  }
]`

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

func New() {
	fmt.Println(USERINPUT_TEMPLATE_OBJECT)
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
