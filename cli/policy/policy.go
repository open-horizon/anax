package policy

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/externalpolicy"
	"net/http"
)

const POLICY_TEMPLATE_OBJECT = `{
  "properties": [   /* A list of policy properties that describe the object. */
    {
      "name": "",
      "value": nil
    }
  ],
  "constraints": [  /* A list of constraint expressions of the form <property name> <operator> <property value>, */
                    /* separated by boolean operators AND (&&) or OR (||). */
    ""
  ]
}`

func List() {
	// Get the node policy info
	nodePolicy := externalpolicy.ExternalPolicy{}
	cliutils.HorizonGet("node/policy", []int{200}, &nodePolicy, false)

	// Output the combined info
	output, err := cliutils.DisplayAsJson(nodePolicy)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn policy list' output: %v", err)
	}

	fmt.Println(output)
}

func Update(fileName string) {

	ep := new(externalpolicy.ExternalPolicy)
	readInputFile(fileName, ep)

	cliutils.HorizonPutPost(http.MethodPost, "node/policy", []int{201, 200}, ep)

	fmt.Println("Horizon node policy updated.")

}

func Patch(patch string) {
	cliutils.HorizonPutPost(http.MethodPatch, "node/policy", []int{201, 200}, patch)

	fmt.Println("Horizon node policy updated.")
}

func readInputFile(filePath string, inputFileStruct *externalpolicy.ExternalPolicy) {
	newBytes := cliconfig.ReadJsonFileWithLocalConfig(filePath)
	err := json.Unmarshal(newBytes, inputFileStruct)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to unmarshal json input file %s: %v", filePath, err)
	}
}

func Remove(force bool) {
	if !force {
		cliutils.ConfirmRemove("Are you sure you want to remove the node policy?")
	}

	cliutils.HorizonDelete("node/policy", []int{200, 204}, false)

	fmt.Println("Horizon node policy deleted.")
}

// Display an empty policy template as an object.
func New() {
	fmt.Println(POLICY_TEMPLATE_OBJECT)
}
