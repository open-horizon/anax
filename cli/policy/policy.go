package policy

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/i18n"
	"net/http"
)

func List() {
	// Get the node policy info
	nodePolicy := externalpolicy.ExternalPolicy{}
	cliutils.HorizonGet("node/policy", []int{200}, &nodePolicy, false)

	// Output the combined info
	output, err := cliutils.DisplayAsJson(nodePolicy)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, i18n.GetMessagePrinter().Sprintf("failed to marshal 'hzn policy list' output: %v", err))
	}

	fmt.Println(output)
}

func Update(fileName string) {

	ep := new(externalpolicy.ExternalPolicy)
	readInputFile(fileName, ep)

	msgPrinter := i18n.GetMessagePrinter()
	msgPrinter.Printf("Updating Horizon node policy and re-evaluating all agreements based on this node policy. Existing agreements might be cancelled and re-negotiated.")
	msgPrinter.Println()
	cliutils.HorizonPutPost(http.MethodPost, "node/policy", []int{201, 200}, ep)

	msgPrinter.Printf("Horizon node policy updated.")
	msgPrinter.Println()

}

func Patch(patch string) {
	cliutils.HorizonPutPost(http.MethodPatch, "node/policy", []int{201, 200}, patch)

	msgPrinter := i18n.GetMessagePrinter()
	msgPrinter.Printf("Horizon node policy updated.")
	msgPrinter.Println()
}

func readInputFile(filePath string, inputFileStruct *externalpolicy.ExternalPolicy) {
	newBytes := cliconfig.ReadJsonFileWithLocalConfig(filePath)
	err := json.Unmarshal(newBytes, inputFileStruct)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, i18n.GetMessagePrinter().Sprintf("failed to unmarshal json input file %s: %v", filePath, err))
	}
}

func Remove(force bool) {
	if !force {
		cliutils.ConfirmRemove(i18n.GetMessagePrinter().Sprintf("Are you sure you want to remove the node policy?"))
	}

	msgPrinter := i18n.GetMessagePrinter()
	msgPrinter.Printf("Removing Horizon node policy and re-evaluating all agreements based on just the built-in node policy. Existing agreements might be cancelled and re-negotiated.")
	msgPrinter.Println()
	cliutils.HorizonDelete("node/policy", []int{200, 204}, false)

	msgPrinter.Printf("Horizon node policy deleted.")
	msgPrinter.Println()
}

// Display an empty policy template as an object.
func New() {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	var policy_template = []string{
		`{`,
		`  "properties": [   /* ` + msgPrinter.Sprintf("A list of policy properties that describe the object.") + ` */`,
		`    {`,
		`       "name": "",`,
		`       "value": nil`,
		`      }`,
		`  ],`,
		`  "constraints": [  /* ` + msgPrinter.Sprintf("A list of constraint expressions of the form <property name> <operator> <property value>,") + ` */`,
		`                    /* ` + msgPrinter.Sprintf("separated by boolean operators AND (&&) or OR (||).") + `*/`,
		`       "" `,
		`  ], `,
		`}`,
	}

	for _, s := range policy_template {
		fmt.Println(s)
	}
}
