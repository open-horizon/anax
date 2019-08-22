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
	i18n.GetMessagePrinter().Println("Updating Horizon node policy and re-evaluating all agreements based on this node policy. Existing agreements might be cancelled and re-negotiated.")
	i18n.GetMessagePrinter().Println()
	cliutils.HorizonPutPost(http.MethodPost, "node/policy", []int{201, 200}, ep)

	i18n.GetMessagePrinter().Println("Horizon node policy updated.")

}

func Patch(patch string) {
	cliutils.HorizonPutPost(http.MethodPatch, "node/policy", []int{201, 200}, patch)

	i18n.GetMessagePrinter().Println("Horizon node policy updated.")
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
	i18n.GetMessagePrinter().Println("Removing Horizon node policy and re-evaluating all agreements based on just the built-in node policy. Existing agreements might be cancelled and re-negotiated.")
	i18n.GetMessagePrinter().Println()
	cliutils.HorizonDelete("node/policy", []int{200, 204}, false)

	i18n.GetMessagePrinter().Println("Horizon node policy deleted.")
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
