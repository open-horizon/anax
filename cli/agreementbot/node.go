package agreementbot

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/agreementbot"
	"github.com/open-horizon/anax/apicommon"
	"github.com/open-horizon/anax/cli/cliutils"
	"os"
)

// This is a combo of anax's HorizonDevice and Info (status) structs
type AgbotAndStatus struct {
	// from agreementbot.HorizonAgbot
	Id  string `json:"agbot_id"`
	Org string `json:"organization"`
	// from apicommon.Info
	Configuration *apicommon.Configuration `json:"configuration"`
	Connectivity  map[string]bool          `json:"connectivity"`
}

// CopyNodeInto copies the node info into our output struct
func (n *AgbotAndStatus) CopyNodeInto(horDevice *agreementbot.HorizonAgbot) {
	//todo: I don't like having to repeat all of these fields, hard to maintain. Maybe use reflection?
	n.Id = horDevice.Id
	n.Org = horDevice.Org
}

// CopyStatusInto copies the status info into our output struct
func (n *AgbotAndStatus) CopyStatusInto(status *apicommon.Info) {
	//todo: I don't like having to repeat all of these fields, hard to maintain. Maybe use reflection?
	n.Configuration = status.Configuration
	n.Connectivity = status.Connectivity
}

func List() {
	// set env to call agbot url
	os.Setenv("HORIZON_URL", cliutils.AGBOT_HZN_API)

	// Get the agbot info
	horDevice := agreementbot.HorizonAgbot{}
	cliutils.HorizonGet("node", []int{200}, &horDevice)
	nodeInfo := AgbotAndStatus{} // the structure we will output
	nodeInfo.CopyNodeInto(&horDevice)

	// Get the horizon status info
	status := apicommon.Info{}
	cliutils.HorizonGet("status", []int{200}, &status)
	nodeInfo.CopyStatusInto(&status)

	// Output the combined info
	jsonBytes, err := json.MarshalIndent(nodeInfo, "", cliutils.JSON_INDENT)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn node list' output: %v", err)
	}
	fmt.Printf("%s\n", jsonBytes) //todo: is there a way to output with json syntax highlighting like jq does?
}
