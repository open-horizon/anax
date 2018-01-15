package node

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/cli/cliutils"
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
	// from api.Info
	Geths         []api.Geth         `json:"geth"`
	Configuration *api.Configuration `json:"configuration"`
	Connectivity  map[string]bool    `json:"connectivity"`
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
func (n *NodeAndStatus) CopyStatusInto(status *api.Info) {
	//todo: I don't like having to repeat all of these fields, hard to maintain. Maybe use reflection?
	n.Geths = status.Geths
	n.Configuration = status.Configuration
	n.Connectivity = status.Connectivity
}

func List() {
	// Get the node info
	horDevice := api.HorizonDevice{}
	cliutils.HorizonGet("node", []int{200}, &horDevice)
	nodeInfo := NodeAndStatus{} // the structure we will output
	nodeInfo.CopyNodeInto(&horDevice)

	// Get the horizon status info
	status := api.Info{}
	cliutils.HorizonGet("status", []int{200}, &status)
	nodeInfo.CopyStatusInto(&status)

	// Output the combined info
	jsonBytes, err := json.MarshalIndent(nodeInfo, "", cliutils.JSON_INDENT)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn node list' output: %v", err)
	}
	fmt.Printf("%s\n", jsonBytes) //todo: is there a way to output with json syntax highlighting like jq does?
}

func Version() {
	status := api.Info{}
	cliutils.HorizonGet("status", []int{200}, &status)
	fmt.Println(status.Configuration.HorizonVersion)
}
