package register

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
)

// These structs are used to parse the registration input file
type GlobalSet struct {
	Type       string                 `json:"type"`
	SensorUrls []string               `json:"sensor_urls"`
	Variables  map[string]interface{} `json:"variables"`
}

// Use for both microservices and workloads
type MicroWork struct {
	Org          string                 `json:"org"`
	Url          string                 `json:"url"`
	VersionRange string                 `json:"versionRange"`
	Variables    map[string]interface{} `json:"variables"`
}

type InputFile struct {
	Global        []GlobalSet `json:"global"`
	Microservices []MicroWork `json:"microservices"`
	Workloads     []MicroWork `json:"workloads"`
}

func ReadInputFile(filePath string, inputFileStruct *InputFile) {
	var fileBytes []byte
	var err error
	if filePath == "-" {
		fileBytes, err = ioutil.ReadAll(os.Stdin)
	} else {
		fileBytes, err = ioutil.ReadFile(filePath)
	}
	if err != nil {
		cliutils.Fatal(cliutils.READ_FILE_ERROR, "reading %s failed: %v", filePath, err)
	}
	// remove /* */ comments
	re := regexp.MustCompile(`(?s)/\*.*?\*/`)
	newBytes := re.ReplaceAll(fileBytes, nil)

	err = json.Unmarshal(newBytes, inputFileStruct)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to unmarshal json input file %s: %v", filePath, err)
	}
}

// DoIt registers this node to Horizon with a pattern
func DoIt(org string, pattern string, nodeIdTok string, userPw string, email string, inputFile string) {
	// Read input file 1st, so we don't get half way thru registration before finding the problem
	inputFileStruct := InputFile{}
	if inputFile != "" {
		fmt.Printf("Reading input file %s...\n", inputFile)
		ReadInputFile(inputFile, &inputFileStruct)
	}

	// Get the exchange url from the anax api
	exchUrlBase := cliutils.GetExchangeUrl()
	fmt.Printf("Horizon Exchange base URL: %s\n", exchUrlBase)

	// See if the node exists in the exchange, and create if it doesn't
	nodeId, nodeToken := cliutils.SplitIdToken(nodeIdTok)
	if nodeId == "" {
		// Get the id from anax
		horDevice := api.HorizonDevice{}
		cliutils.HorizonGet("node", []int{200}, &horDevice)
		nodeId = *horDevice.Id
		fmt.Printf("Using node ID '%s' from the Horizon agent\n", nodeId)
	}
	if nodeToken == "" {
		// Create a random token
		var err error
		nodeToken, err = cutil.SecureRandomString()
		if err != nil {
			cliutils.Fatal(cliutils.INTERNAL_ERROR, "could not create a random token")
		}
		fmt.Println("Generated random node token")
	}
	node := exchange.GetDevicesResponse{}
	httpCode := cliutils.ExchangeGet(exchUrlBase, "orgs/"+org+"/nodes/"+nodeId, org+"/"+nodeId+":"+nodeToken, nil, &node)
	if httpCode != 200 {
		if userPw == "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "node '%s/%s' does not exist in the exchange with the specified token and the -u flag was not specified to provide exchange user credentials to create/update it.", org, nodeId)
		}
		fmt.Printf("Node %s/%s does not exist in the exchange with the specified token, creating/updating it...\n", org, nodeId)
		putNodeReq := exchange.PutDeviceRequest{Token: nodeToken, Name: nodeId, SoftwareVersions: make(map[string]string), PublicKey: []byte("")} // we only need to set the token
		httpCode = cliutils.ExchangePutPost(http.MethodPut, exchUrlBase, "orgs/"+org+"/nodes/"+nodeId, org+"/"+userPw, []int{201, 401}, putNodeReq)
		if httpCode == 401 {
			user, pw := cliutils.SplitIdToken(userPw)
			if org == "public" && email != "" {
				// In the public org we can create a user anonymously, so try that
				fmt.Printf("User %s/%s does not exist in the exchange with the specified password, creating it...\n", org, user)
				postUserReq := cliutils.UserExchangeReq{Password: pw, Admin: false, Email: email}
				httpCode = cliutils.ExchangePutPost(http.MethodPost, exchUrlBase, "orgs/"+org+"/users/"+user, "", []int{201}, postUserReq)
				fmt.Printf("Trying again to create/update %s/%s ...\n", org, nodeId)
				httpCode = cliutils.ExchangePutPost(http.MethodPut, exchUrlBase, "orgs/"+org+"/nodes/"+nodeId, org+"/"+userPw, []int{201}, putNodeReq)
			} else {
				cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "node '%s/%s' does not exist in the exchange with the specified token and user '%s/%s' does not exist with the specified password.", org, nodeId, org, user)
			}
		}
	} else {
		fmt.Printf("Node %s/%s exists in the exchange\n", org, nodeId)
	}

	// Initialize the Horizon device (node)
	fmt.Println("Initializing the Horizon node...")
	//nd := Node{Id: nodeId, Token: nodeToken, Org: org, Pattern: pattern, Name: nodeId, HA: false}
	falseVal := false
	nd := api.HorizonDevice{Id: &nodeId, Token: &nodeToken, Org: &org, Pattern: &pattern, Name: &nodeId, HA: &falseVal} //todo: support HA config
	httpCode = cliutils.HorizonPutPost(http.MethodPost, "node", []int{201, 200, cliutils.ANAX_ALREADY_CONFIGURED}, nd)
	if httpCode == cliutils.ANAX_ALREADY_CONFIGURED {
		// Note: I wanted to make `hzn register` idempotent, but the anax api doesn't support changing existing settings once in configuring state (to maintain internal consistency).
		//		And i can't query ALL the existing settings to make sure they are what we were going to set, because i can't query the node token.
		cliutils.Fatal(cliutils.HTTP_ERROR, "this Horizon node is already registered or in the process of being registered. If you want to register it differently, run 'hzn unregister' first.")
	}

	// Process the input file and call /attribute, /microservice/config, and /workload/config to set the specified variables
	if inputFile != "" {
		// Set the global variables as attributes with no url (or in the case of HTTPSBasicAuthAttributes, with url equal to image svr)
		// Technically the AgreementProtocolAttributes can be set, but it has no effect on anax if a pattern is being used.
		fmt.Println("Setting global variables...")
		attr := api.NewAttribute("", []string{}, "Global variables", false, false, map[string]interface{}{}) // we reuse this for each GlobalSet
		for _, g := range inputFileStruct.Global {
			attr.Type = &g.Type
			attr.SensorUrls = &g.SensorUrls
			attr.Mappings = &g.Variables
			cliutils.HorizonPutPost(http.MethodPost, "attribute", []int{201, 200}, attr)
		}

		// Set the microservice variables
		fmt.Println("Setting microservice variables...")
		attr = api.NewAttribute("UserInputAttributes", []string{}, "microservice", false, false, map[string]interface{}{}) // we reuse this for each microservice
		emptyStr := ""
		service := api.Service{SensorName: &emptyStr} // we reuse this too
		for _, m := range inputFileStruct.Microservices {
			service.SensorOrg = &m.Org
			service.SensorUrl = &m.Url
			service.SensorVersion = &m.VersionRange
			attr.Mappings = &m.Variables
			attrSlice := []api.Attribute{*attr}
			service.Attributes = &attrSlice
			cliutils.HorizonPutPost(http.MethodPost, "microservice/config", []int{201, 200}, service)
		}

		// Set the workload variables
		fmt.Println("Setting workload variables...")
		attr = api.NewAttribute("UserInputAttributes", []string{}, "workload", false, false, map[string]interface{}{})
		for _, w := range inputFileStruct.Workloads {
			attr.Mappings = &w.Variables
			workload := api.WorkloadConfig{Org: w.Org, WorkloadURL: w.Url, Version: w.VersionRange, Attributes: []api.Attribute{*attr}}
			cliutils.HorizonPutPost(http.MethodPost, "workload/config", []int{201, 200}, workload)
		}

	} else {
		// Technically an input file is not required, but it is not the common case, so warn them
		fmt.Println("Warning: no input file was specified. This is only valid if none of the microservices or workloads need variables set (including GPS coordinates).")
	}

	// Set the pattern and register the node
	fmt.Println("Changing Horizon state to configured to register this node with Horizon...")
	configuredStr := "configured"
	config := api.Configstate{State: &configuredStr}
	cliutils.HorizonPutPost(http.MethodPut, "node/configstate", []int{201, 200}, config)

	fmt.Println("Horizon node is registered. Workload agreement negotiation should begin shortly. Run 'hzn agreement list' to view.")
}
