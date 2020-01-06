package userinput

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/policy"
	"net/http"
)

//Display a list of the current userInputs of the node
func List() {
	var inputs []policy.UserInput
	cliutils.HorizonGet("node/userinput", []int{200}, &inputs, false)

	output, err := cliutils.DisplayAsJson(inputs)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, i18n.GetMessagePrinter().Sprintf("Unable to marshal userinput object: %v", err))
	}
	fmt.Println(output)
}

func New() {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	var uerinput_template = []string{
		`[                         /* ` + msgPrinter.Sprintf("A list of objects, each one containing the user inputs required for a specified service.") + ` */`,
		`  {`,
		`    "serviceOrgid": "",   /* ` + msgPrinter.Sprintf("The horizon org of the specified service.") + ` */`,
		`    "serviceUrl": "",     /* ` + msgPrinter.Sprintf("The unique string used to identify the specified service.") + ` */`,
		`    "serviceArch": "",    /* ` + msgPrinter.Sprintf("The service architecture that these inputs apply to. Omit or leave blank to mean all architectures.") + ` */`,
		`    "serviceVersionRange": "", /* ` + msgPrinter.Sprintf("The service versions that these inputs apply to. Omit or specify \"[0.0.0,INFINITY)\" to mean all versions.") + ` */`,
		`    "inputs": [           /* ` + msgPrinter.Sprintf("A list of objects with the names and values for the user inputs used by this service.") + ` */`,
		`      {`,
		`        "name": "",`,
		`        "value": null`,
		`      }`,
		`    ]`,
		`  }`,
		`]`,
	}

	for _, s := range uerinput_template {
		fmt.Println(s)
	}
}

//Add or overwrite the userinputs for this node
func Add(filePath string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	var inputs []policy.UserInput
	inputString := cliconfig.ReadJsonFileWithLocalConfig(filePath)

	err := json.Unmarshal([]byte(inputString), &inputs)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("Error unmarshaling userInput json file: %v", err))
	}

	cliutils.HorizonPutPost(http.MethodPost, "node/userinput", []int{200, 201}, inputs)
	msgPrinter.Printf("Horizon node user inputs updated.")
	msgPrinter.Println()
}

//Update the userinputs for this node
func Update(filePath string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	var inputs []policy.UserInput
	inputString := cliconfig.ReadJsonFileWithLocalConfig(filePath)

	err := json.Unmarshal([]byte(inputString), &inputs)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("Error unmarshaling userInput json file: %v", err))
	}

	cliutils.HorizonPutPost(http.MethodPatch, "node/userinput", []int{200, 201}, inputs)
	msgPrinter.Printf("Horizon node user inputs updated.")
	msgPrinter.Println()
}

//Remove the user inputs for this nose
func Remove(force bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if !force {
		cliutils.ConfirmRemove(msgPrinter.Sprintf("Are you sure you want to remove the node user inputs?"))
	}

	cliutils.HorizonDelete("node/userinput", []int{200, 204}, []int{}, false)

	msgPrinter.Printf("Horizon user inputs removed.")
	msgPrinter.Println()
}
