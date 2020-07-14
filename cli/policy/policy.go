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
	msgPrinter := i18n.GetMessagePrinter()

	ep := new(externalpolicy.ExternalPolicy)
	readInputFile(fileName, ep)

	readOnlyBuiltIns := externalpolicy.ListReadOnlyProperties()
	includedBuiltIns := ""
	for _, builtInProp := range readOnlyBuiltIns {
		if ep.Properties.HasProperty(builtInProp) {
			if includedBuiltIns == "" {
				includedBuiltIns = builtInProp
			} else {
				includedBuiltIns = fmt.Sprintf("%s, %s", includedBuiltIns, builtInProp)
			}
		}
	}
	if includedBuiltIns != "" {
		msgPrinter.Printf("Warning: built-in properties %v are read-only. The given value will be ignored.", includedBuiltIns)
		msgPrinter.Println()
	}

	cliutils.HorizonPutPost(http.MethodPost, "node/policy", []int{201, 200}, ep, true)

	msgPrinter.Printf("Updating Horizon node policy and re-evaluating all agreements based on this node policy. Existing agreements might be cancelled and re-negotiated.")
	msgPrinter.Println()

}

func Patch(patch string) {
	msgPrinter := i18n.GetMessagePrinter()
	msgPrinter.Printf("Warning: This command is deprecated. It will continue to be supported until the next major release. Please use 'hzn policy update' to update the node policy.")
	msgPrinter.Println()

	cliutils.HorizonPutPost(http.MethodPatch, "node/policy", []int{201, 200}, patch, true)

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
	cliutils.HorizonDelete("node/policy", []int{200, 204}, []int{}, false)

	msgPrinter.Printf("Removing Horizon node policy and re-evaluating all agreements. Existing agreements might be cancelled and re-negotiated.")
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
		`       "value": null`,
		`      }`,
		`  ],`,
		`  "constraints": [  /* ` + msgPrinter.Sprintf("A list of constraint expressions of the form <property name> <operator> <property value>,") + ` */`,
		`                    /* ` + msgPrinter.Sprintf("separated by boolean operators AND (&&) or OR (||).") + `*/`,
		`       "" `,
		`  ] `,
		`}`,
	}

	for _, s := range policy_template {
		fmt.Println(s)
	}
}
