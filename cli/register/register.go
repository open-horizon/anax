package register

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/cli/cliutils"
	cliexchange "github.com/open-horizon/anax/cli/exchange"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/policy"
	"io/ioutil"
	"net/http"
)

// These structs are used to parse the registration input file. These are also used by the hzn dev code.
type GlobalSet struct {
	Type       string                 `json:"type"`
	SensorUrls []string               `json:"sensor_urls"`
	Variables  map[string]interface{} `json:"variables"`
}

func (g GlobalSet) String() string {
	return fmt.Sprintf("Global Array element, type: %v, sensor_urls: %v, variables: %v", g.Type, g.SensorUrls, g.Variables)
}

// Use for both microservices and workloads
type MicroWork struct {
	Org          string                 `json:"org"`
	Url          string                 `json:"url"`
	VersionRange string                 `json:"versionRange"`
	Variables    map[string]interface{} `json:"variables"`
}

func (m MicroWork) String() string {
	return fmt.Sprintf("Org: %v, URL: %v, VersionRange: %v, Variables: %v", m.Org, m.Url, m.VersionRange, m.Variables)
}

type InputFile struct {
	Global        []GlobalSet `json:"global,omitempty"`
	Microservices []MicroWork `json:"microservices,omitempty"`
	Workloads     []MicroWork `json:"workloads,omitempty"`
}

func ReadInputFile(filePath string, inputFileStruct *InputFile) {
	newBytes := cliutils.ReadJsonFile(filePath)
	err := json.Unmarshal(newBytes, inputFileStruct)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to unmarshal json input file %s: %v", filePath, err)
	}
}

// DoIt registers this node to Horizon with a pattern
func DoIt(org, pattern, nodeIdTok, userPw, email, inputFile string) {
	cliutils.SetWhetherUsingApiKey(nodeIdTok) // if we have to use userPw later in NodeCreate(), it will set this appropriately for userPw
	// Read input file 1st, so we don't get half way thru registration before finding the problem
	inputFileStruct := InputFile{}
	if inputFile != "" {
		fmt.Printf("Reading input file %s...\n", inputFile)
		ReadInputFile(inputFile, &inputFileStruct)
	}

	// Get the exchange url from the anax api
	exchUrlBase := cliutils.GetExchangeUrl()
	fmt.Printf("Horizon Exchange base URL: %s\n", exchUrlBase)

	// Default node id and token if necessary
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
	nodeIdTok = nodeId + ":" + nodeToken

	// See if the node exists in the exchange, and create if it doesn't
	httpCode := cliutils.ExchangeGet(exchUrlBase, "orgs/"+org+"/nodes/"+nodeId, cliutils.OrgAndCreds(org, nodeIdTok), nil, nil)
	if httpCode != 200 {
		if userPw == "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "node '%s/%s' does not exist in the exchange with the specified token, and the -u flag was not specified to provide exchange user credentials to create/update it.", org, nodeId)
		}
		fmt.Printf("Node %s/%s does not exist in the exchange with the specified token, creating/updating it...\n", org, nodeId)
		cliexchange.NodeCreate(org, nodeIdTok, userPw, email)
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
	configState := api.Configstate{State: &configuredStr}
	cliutils.HorizonPutPost(http.MethodPut, "node/configstate", []int{201, 200}, configState)

	fmt.Println("Horizon node is registered. Workload agreement negotiation should begin shortly. Run 'hzn agreement list' to view.")
}

// GetHighestMicroservice queries the exchange for all versions of this MS and returns the highest version, or an error
func GetHighestMicroservice(exchangeUrl, credOrg, nodeIdTok, org, url, versionRange, arch string) exchange.MicroserviceDefinition {
	route := "orgs/" + org + "/microservices?specRef=" + url + "&arch=" + arch // get all MS of this org, url, and arch
	var microOutput exchange.GetMicroservicesResponse
	cliutils.ExchangeGet(exchangeUrl, route, cliutils.OrgAndCreds(credOrg, nodeIdTok), []int{200}, &microOutput)
	if len(microOutput.Microservices) == 0 {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "found no microservices in the exchange matched: org=%s, specRef=%s, arch=%s", org, url, arch)
	}

	// Loop thru the returned MSs and pick out the highest version that is within versionRange range
	highestKey := "" // key to the MS def in the map that so far has the highest valid version
	vRange, err := policy.Version_Expression_Factory(versionRange)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "invalid version range '%s': %v", versionRange, err)
	}
	for microKey, micro := range microOutput.Microservices {
		if inRange, err := vRange.Is_within_range(micro.Version); err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "unable to verify that %v is within %v, error %v", micro.Version, vRange, err)
		} else if !inRange {
			continue // not within the specified version range, so ignore it
		}
		if highestKey == "" {
			highestKey = microKey // 1st MS found within the range
			continue
		}
		// else see if this version is higher than the previous highest version
		c, err := policy.CompareVersions(microOutput.Microservices[highestKey].Version, micro.Version)
		if err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "error compairing version %v with version %v. %v", microOutput.Microservices[highestKey], micro.Version, err)
		} else if c == -1 {
			highestKey = microKey
		}
	}

	if highestKey == "" {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "found no microservices in the exchange matched: org=%s, specRef=%s, version range=%s, arch=%s", org, url, versionRange, arch)
	}
	cliutils.Verbose("selected %s for version range %s", highestKey, versionRange)
	return microOutput.Microservices[highestKey]
}

// CreateInputFile runs thru the workloads and microservices used by this pattern and collects the user input needed
func CreateInputFile(org, pattern, arch, nodeIdTok, inputFile string) {
	// Get the pattern
	exchangeUrl := cliutils.GetExchangeUrl()
	var patOutput exchange.GetPatternResponse
	cliutils.ExchangeGet(exchangeUrl, "orgs/"+org+"/patterns/"+pattern, cliutils.OrgAndCreds(org, nodeIdTok), []int{200}, &patOutput)
	patKey := cliutils.OrgAndCreds(org, pattern)
	if _, ok := patOutput.Patterns[patKey]; !ok {
		cliutils.Fatal(cliutils.INTERNAL_ERROR, "did not find pattern '%s' as expected", patKey)
	}

	// Loop thru the workloads gathering their user input and microservices
	templateFile := InputFile{Global: []GlobalSet{{Type: "LocationAttributes", Variables: map[string]interface{}{"lat": 0.0, "lon": 0.0, "use_gps": false, "location_accuracy_km": 0.0}}}}
	if arch == "" {
		arch = cutil.ArchString()
	}
	completeAPISpecList := new(policy.APISpecList) // list of all MSs the workloads require (will filter out MS refs with exact same version range, but not overlapping ranges (that comes later)
	for _, work := range patOutput.Patterns[patKey].Workloads {
		if work.WorkloadArch != arch { // filter out workloads that are not our arch
			fmt.Printf("Ignoring workload that is a different architecture: %s, %s, %s\n", work.WorkloadOrg, work.WorkloadURL, work.WorkloadArch)
			continue
		}

		for _, workVersion := range work.WorkloadVersions {
			// Get the workload
			exchId := cliutils.FormExchangeId(work.WorkloadURL, workVersion.Version, work.WorkloadArch)
			var workOutput exchange.GetWorkloadsResponse
			cliutils.ExchangeGet(exchangeUrl, "orgs/"+work.WorkloadOrg+"/workloads/"+exchId, cliutils.OrgAndCreds(org, nodeIdTok), []int{200}, &workOutput)
			workKey := cliutils.OrgAndCreds(work.WorkloadOrg, exchId)
			if _, ok := workOutput.Workloads[workKey]; !ok {
				cliutils.Fatal(cliutils.INTERNAL_ERROR, "did not find workload '%s' as expected", workKey)
			}

			// Get the user input from this workload
			userInputs := workOutput.Workloads[workKey].UserInputs
			if len(userInputs) > 0 {
				workInput := MicroWork{Org: work.WorkloadOrg, Url: work.WorkloadURL, VersionRange: "[0.0.0,INFINITY)", Variables: make(map[string]interface{})}
				for _, u := range userInputs {
					workInput.Variables[u.Name] = u.DefaultValue
				}
				templateFile.Workloads = append(templateFile.Workloads, workInput)
			}

			// Loop thru this workload's microservices, adding them to our list
			micros := workOutput.Workloads[workKey].APISpecs
			apiSpecList := new(policy.APISpecList)
			for _, m := range micros {
				newAPISpec := policy.APISpecification_Factory(m.SpecRef, m.Org, m.Version, m.Arch)
				*apiSpecList = append(*apiSpecList, *newAPISpec)
			}
			// MergeWith will will filter out MS refs with exact same version range, but not overlapping ranges (that comes later)
			*completeAPISpecList = completeAPISpecList.MergeWith(apiSpecList)
		}
	}

	// Loop thru the referenced MSs, get the highest version of each range, and then get the user input for it
	// For now, anax only allows one microservice version, so we need to get the common version range for each microservice.
	common_apispec_list, err := completeAPISpecList.GetCommonVersionRanges()
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "problem getting the common list of microservice version ranges: %v", err)
	}
	// these 2 lines are for GetMicroservice()
	//nodeId, nodeToken := cliutils.SplitIdToken(nodeIdTok)
	//httpClientFactory := &config.HTTPClientFactory{NewHTTPClient: func(overrideTimeoutS *uint) *http.Client { return &http.Client{} }}
	for _, m := range *common_apispec_list {
		micro := GetHighestMicroservice(exchangeUrl, org, nodeIdTok, m.Org, m.SpecRef, m.Version, m.Arch)
		/* we could use exchange.GetMicroservice() instead of GetHighestMicroservice(), but it assumes an anax context so logs errors in a way not clear for hzn users...
		var versionRangeStr string		// need to expand the version string to a full version range, but still as a string
		if versionRange, err := policy.Version_Expression_Factory(m.Version); err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "problem creating version expression of %s: %v", m.Version, err)
		} else {
			versionRangeStr = versionRange.Get_expression()
		}
		micro, err := exchange.GetMicroservice(httpClientFactory, m.SpecRef, m.Org, versionRangeStr, m.Arch, exchangeUrl+"/", cliutils.OrgAndCreds(org, nodeId), nodeToken)
		if err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "problem getting the highest version microservice for %s %s %s: %v", m.Org, m.SpecRef, m.Arch, err)
		}
		cliutils.Verbose("for %s %s %s selected %s for version range %s", m.Org, m.SpecRef, m.Arch, micro.Version, m.Version)
		*/

		// Get the user input from this microservice
		userInputs2 := micro.UserInputs
		if len(userInputs2) > 0 {
			microInput := MicroWork{Org: m.Org, Url: m.SpecRef, VersionRange: "[0.0.0,INFINITY)", Variables: make(map[string]interface{})}
			for _, u := range userInputs2 {
				microInput.Variables[u.Name] = u.DefaultValue
			}
			templateFile.Microservices = append(templateFile.Microservices, microInput)
		}
	}

	// Output the template file
	jsonBytes, err := json.MarshalIndent(templateFile, "", cliutils.JSON_INDENT)
	if err != nil {
		cliutils.Fatal(cliutils.INTERNAL_ERROR, "failed to marshal the user input template file: %v", err)
	}
	//fmt.Printf("Would write to %s\n", inputFile)
	//fmt.Println(string(jsonBytes))
	if err := ioutil.WriteFile(inputFile, jsonBytes, 0644); err != nil {
		cliutils.Fatal(cliutils.FILE_IO_ERROR, "problem writing the user input template file: %v", err)
	}
	fmt.Printf("Wrote %s\n", inputFile)
}
