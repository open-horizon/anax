package register

import (
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/api"
	"strings"
	"github.com/open-horizon/anax/exchange"
	"fmt"
	"io/ioutil"
	"encoding/json"
)

type HorizonDevice struct {
	Id                 string      `json:"id"`
	Org                string      `json:"organization"`
	Pattern            string      `json:"pattern"` // a simple name, not prefixed with the org
	Name               string      `json:"name,omitempty"`
	Token              string      `json:"token,omitempty"`
	HADevice           bool        `json:"ha_device,omitempty"`
}

type GlobalSet struct {
	Type string `json:"type"`
	Variables map[string]interface{} `json:"variables"`
}

// Use for both microservices and workloads
type MicroWork struct {
	Org string `json:"org"`
	Url string `json:"url"`
	VersionRange string `json:"versionRange"`
	Variables map[string]interface{} `json:"variables"`
}

type InputFile struct {
	Global []GlobalSet `json:"global"`
	Microservices []MicroWork     `json:"microservices"`
	Workloads []MicroWork     `json:"workloads"`
}


func readInputFile(filePath string, inputFileStruct *InputFile) {
	fileBytes, err := ioutil.ReadFile(filePath)
	if err != nil { cliutils.Fatal(cliutils.READ_FILE_ERROR, "reading %s failed: %v", filePath, err) }
	//todo: remove comments from file
	err = json.Unmarshal(fileBytes, inputFileStruct)
	if err != nil { cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to unmarshal json input file %s: %v", filePath, err) }
}


// Note: a structure like this exists in the api pkg, but has the id and everything is ptrs, so it is not convenient to use
type Attribute struct {
	Type        string                 `json:"type"`
	SensorUrls  []string               `json:"sensor_urls"`
	Label       string                 `json:"label"`
	Publishable bool                   `json:"publishable"`
	HostOnly    bool                   `json:"host_only"`
	Mappings    map[string]interface{} `json:"mappings"`
}

type Service struct {
	SensorUrl     string      `json:"sensor_url"`
	SensorOrg     string      `json:"sensor_org"`
	SensorName    string      `json:"sensor_name"`
	SensorVersion string      `json:"sensor_version"`
	AutoUpgrade   bool        `json:"auto_upgrade"`
	ActiveUpgrade bool        `json:"active_upgrade"`
	Attributes    []Attribute `json:"attributes"`
}


// DoIt registers this node to Horizon with a pattern
func DoIt(org string, nodeId string, nodeToken string, pattern string, userPw string, inputFile string) {
	// Get the exchange url from the anax api
	status := api.Info{}
	cliutils.HorizonGet("status", 200, &status)
	exchUrlBase := strings.TrimSuffix(status.Configuration.ExchangeAPI, "/")
	fmt.Printf("Horizon Exchange base URL: %s\n", exchUrlBase)

	// See if the node exists in the exchange, and create if it doesn't
	node := exchange.GetDevicesResponse{}
	httpCode := cliutils.ExchangeGet(exchUrlBase, "orgs/"+org+"/nodes/"+nodeId, org+"/"+nodeId+":"+nodeToken, 0, &node)
	//cliutils.Verbose("node: %v", node)
	if httpCode != 200 {
		if userPw == "" { cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "node %s/%s does not exist in the exchange and the -u flag was not specified to provide exchange user credentials to create it.", org, nodeId) }
		fmt.Printf("Node %s/%s does not exists in the exchange, creating it...\n", org, nodeId)
		putNodeReq := exchange.PutDeviceRequest{Token: nodeToken, Name: nodeId, SoftwareVersions: make(map[string]string), PublicKey: []byte("")}		// we only need to set the token
		cliutils.ExchangePutPost("PUT", exchUrlBase, "orgs/"+org+"/nodes/"+nodeId, org+"/"+userPw, 201, putNodeReq)
	} else {
		fmt.Printf("Node %s/%s exists in the exchange\n", org, nodeId)
	}

	// Initialize the Horizon device (node)
	fmt.Println("Initializing the Horizon node...")
	hd := HorizonDevice{Id: nodeId, Token: nodeToken, Org: org, Pattern: pattern, Name: nodeId, HADevice: false}	//todo: support HA config
	cliutils.HorizonPutPost("POST", "horizondevice", []int{201,200}, hd)

	// Read input file and call /attribute, /service, and /workloadconfig to set the specified variables
	if inputFile != "" {
		fmt.Printf("Reading input file %s...\n", inputFile)
		inputFileStruct := InputFile{}
		readInputFile(inputFile, &inputFileStruct)

		// Set the global variables as attributes with no url
		fmt.Println("Setting global variables...")
		attr := Attribute{SensorUrls: []string{}, Label: "Global variables", Publishable: false, HostOnly: false}	// we reuse this for each GlobalSet
		for _, g := range inputFileStruct.Global {
			//todo: in input file change the LocationAttributes to MappedAttributes (and only HZN_LAT and HZN_LON)
			attr.Type = g.Type
			attr.Mappings = g.Variables
			cliutils.HorizonPutPost("POST", "attribute", []int{201,200}, attr)
		}

		// Set the microservice variables
		fmt.Println("Setting microservice variables...")
		attr = Attribute{Type: "MappedAttributes", SensorUrls: []string{}, Label: "app", Publishable: false, HostOnly: false}	// we reuse this for each microservice
		service := Service{Attributes: []Attribute{attr}}
		for _, m := range inputFileStruct.Microservices {
			service.SensorOrg = m.Org
			service.SensorUrl = m.Url
			service.SensorVersion = m.VersionRange
			attr.Mappings = m.Variables
			service.Attributes[0] = attr
			cliutils.HorizonPutPost("POST", "service", []int{201,200}, service)
		}

		// Set the workload variables
		fmt.Println("Setting workload variables...")
		for _, w := range inputFileStruct.Workloads {
			workload := api.WorkloadConfig{Org: w.Org, WorkloadURL: w.Url, Version: w.VersionRange, Variables: w.Variables}
			cliutils.HorizonPutPost("POST", "workloadconfig", []int{201,200}, workload)
		}

	} else {
		// Technically an input file is not required, but it is not the common case, so warn them
		fmt.Println("Warning: no input file was specified. This is only valid if none of the microservices or workloads need variables set (including GPS coordinates).")
	}

	// Set the pattern and register the node
	fmt.Println("Note: the implementation of register is not yet complete. At this point it should set configstate to configured to register this node with Horizon.")
}
