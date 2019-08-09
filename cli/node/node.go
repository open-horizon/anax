package node

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/apicommon"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/version"
)

type Configstate struct {
	State          *string `json:"state"`
	LastUpdateTime string  `json:"last_update_time"` // removed omitempty
}

// This is a combo of anax's HorizonDevice and Info (status) structs
type NodeAndStatus struct {
	// from api.HorizonDevice
	Id      *string `json:"id"`
	Org     *string `json:"organization"`
	Pattern *string `json:"pattern"` // a simple name, not prefixed with the org
	Name    *string `json:"name"`    // removed omitempty
	//Token              *string     `json:"token"`                 // removed omitempty
	TokenLastValidTime string      `json:"token_last_valid_time"` // removed omitempty
	TokenValid         *bool       `json:"token_valid"`           // removed omitempty
	HA                 *bool       `json:"ha"`                    // removed omitempty
	Config             Configstate `json:"configstate"`           // removed omitempty
	// from apicommon.Info
	Configuration *apicommon.Configuration `json:"configuration"`
	Connectivity  map[string]bool          `json:"connectivity"`
}

// CopyNodeInto copies the node info into our output struct and converts times in the process
func (n *NodeAndStatus) CopyNodeInto(horDevice *api.HorizonDevice) {
	//todo: I don't like having to repeat all of these fields, hard to maintain. Maybe use reflection?
	n.Id = horDevice.Id
	n.Org = horDevice.Org
	n.Pattern = horDevice.Pattern
	n.Name = horDevice.Name
	//n.Token = horDevice.Token  // <- the api always returns null for the token (as it should)
	if horDevice.TokenLastValidTime != nil {
		n.TokenLastValidTime = cliutils.ConvertTime(*horDevice.TokenLastValidTime)
	}
	n.TokenValid = horDevice.TokenValid
	n.HA = horDevice.HA
	n.Config.State = horDevice.Config.State
	if horDevice.Config.LastUpdateTime != nil {
		n.Config.LastUpdateTime = cliutils.ConvertTime(*horDevice.Config.LastUpdateTime)
	}
}

// CopyStatusInto copies the status info into our output struct
func (n *NodeAndStatus) CopyStatusInto(status *apicommon.Info) {
	//todo: I don't like having to repeat all of these fields, hard to maintain. Maybe use reflection?
	n.Configuration = status.Configuration
	n.Connectivity = status.Connectivity
}

func List() {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Get the node info
	horDevice := api.HorizonDevice{}
	cliutils.HorizonGet("node", []int{200}, &horDevice, false)
	if horDevice.Config == nil {
		cliutils.Fatal(cliutils.ANAX_NOT_CONFIGURED_YET, msgPrinter.Sprintf("Failed to get proper response from Horizon agent"))
	}
	nodeInfo := NodeAndStatus{} // the structure we will output
	nodeInfo.CopyNodeInto(&horDevice)

	// Get the horizon status info
	status := apicommon.Info{}
	cliutils.HorizonGet("status", []int{200}, &status, false)
	nodeInfo.CopyStatusInto(&status)

	// Output the combined info
	jsonBytes, err := json.MarshalIndent(nodeInfo, "", cliutils.JSON_INDENT)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn node list' output: %v", err))
	}
	fmt.Printf("%s\n", jsonBytes) //todo: is there a way to output with json syntax highlighting like jq does?
}

func Version() {
	// Show hzn version
	msgPrinter := i18n.GetMessagePrinter()

	msgPrinter.Printf("Horizon CLI version: %s", version.HORIZON_VERSION)
	msgPrinter.Println()

	// Show anax version
	status := apicommon.Info{}
	httpCode, err := cliutils.HorizonGet("status", []int{200}, &status, true)
	if err == nil && httpCode == 200 && status.Configuration != nil {
		msgPrinter.Printf("Horizon Agent version: %s", status.Configuration.HorizonVersion)
		msgPrinter.Println()
	} else {
		if err != nil {
			cliutils.Verbose(err.Error())
		}
		msgPrinter.Println("Horizon Agent version: failed to get.")
	}
}

func Architecture() {
	// Show client node architecture
	fmt.Printf("%s\n", cutil.ArchString())
}
