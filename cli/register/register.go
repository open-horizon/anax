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

// Use for services, microservices, and workloads
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
	Global        []GlobalSet    `json:"global,omitempty"`
	Services      []MicroWork `json:"services,omitempty"`
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
		cliexchange.NodeCreate(org, "", nodeId, nodeToken, userPw, email)
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
		attr := api.NewAttribute("", []string{}, "Global variables", false, false, map[string]interface{}{}) // we reuse this for each GlobalSet
		if len(inputFileStruct.Global) > 0 {
			fmt.Println("Setting global variables...")
		}
		for _, g := range inputFileStruct.Global {
			attr.Type = &g.Type
			attr.SensorUrls = &g.SensorUrls
			attr.Mappings = &g.Variables

			// set HostOnly to true for these 2 types
			switch g.Type {
			case "HTTPSBasicAuthAttributes", "DockerRegistryAuthAttributes":
				host_only := true
				attr.HostOnly = &host_only
			}
			cliutils.HorizonPutPost(http.MethodPost, "attribute", []int{201, 200}, attr)
		}

		// Set the service variables
		attr = api.NewAttribute("UserInputAttributes", []string{}, "service", false, false, map[string]interface{}{}) // we reuse this for each service
		emptyStr := ""
		service := api.Service{Name: &emptyStr} // we reuse this too
		if len(inputFileStruct.Services) > 0 {
			fmt.Println("Setting service variables...")
		}
		for _, m := range inputFileStruct.Services {
			service.Org = &m.Org
			service.Url = &m.Url
			service.VersionRange = &m.VersionRange
			attr.Mappings = &m.Variables
			attrSlice := []api.Attribute{*attr}
			service.Attributes = &attrSlice
			cliutils.HorizonPutPost(http.MethodPost, "service/config", []int{201, 200}, service)
		}

		// Set the microservice variables
		attr = api.NewAttribute("UserInputAttributes", []string{}, "microservice", false, false, map[string]interface{}{}) // we reuse this for each microservice
		microservice := api.MicroService{SensorName: &emptyStr}                                                            // we reuse this too
		if len(inputFileStruct.Microservices) > 0 {
			fmt.Println("Setting microservice variables...")
		}
		for _, m := range inputFileStruct.Microservices {
			microservice.SensorOrg = &m.Org
			microservice.SensorUrl = &m.Url
			microservice.SensorVersion = &m.VersionRange
			attr.Mappings = &m.Variables
			attrSlice := []api.Attribute{*attr}
			microservice.Attributes = &attrSlice
			cliutils.HorizonPutPost(http.MethodPost, "microservice/config", []int{201, 200}, microservice)
		}

		// Set the workload variables
		attr = api.NewAttribute("UserInputAttributes", []string{}, "workload", false, false, map[string]interface{}{})
		if len(inputFileStruct.Workloads) > 0 {
			fmt.Println("Setting workload variables...")
		}
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

// isWithinRanges returns true if version is within at least 1 of the ranges in versionRanges
func isWithinRanges(version string, versionRanges []string) bool {
	for _, vr := range versionRanges {
		vRange, err := policy.Version_Expression_Factory(vr)
		if err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "invalid version range '%s': %v", vr, err)
		}
		if inRange, err := vRange.Is_within_range(version); err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "unable to verify that %v is within %v, error %v", version, vRange, err)
		} else if inRange {
			return true
		}
	}
	return false 	// was not within any of the ranges
}

// GetHighestService queries the exchange for all versions of this service and returns the highest version that is within at least 1 of the version ranges
func GetHighestService(exchangeUrl, credOrg, nodeIdTok, org, url, arch string, versionRanges []string) exchange.ServiceDefinition {
	route := "orgs/" + org + "/services?url=" + url + "&arch=" + arch // get all services of this org, url, and arch
	var svcOutput exchange.GetServicesResponse
	cliutils.SetWhetherUsingApiKey(nodeIdTok)
	cliutils.ExchangeGet(exchangeUrl, route, cliutils.OrgAndCreds(credOrg, nodeIdTok), []int{200}, &svcOutput)
	if len(svcOutput.Services) == 0 {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "found no services in the exchange matching: org=%s, url=%s, arch=%s", org, url, arch)
	}

	// Loop thru the returned services and pick out the highest version that is within one of the versionRanges
	highestKey := "" // key to the service def in the map that so far has the highest valid version
	for svcKey, svc := range svcOutput.Services {
		if !isWithinRanges(svc.Version, versionRanges) {
			continue // not within any of the specified version ranges, so ignore it
		}
		if highestKey == "" {
			highestKey = svcKey // 1st svc found that is within the range
			continue
		}
		// else see if this version is higher than the previous highest version
		c, err := policy.CompareVersions(svcOutput.Services[highestKey].Version, svc.Version)
		if err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "error comparing version %v with version %v. %v", svcOutput.Services[highestKey], svc.Version, err)
		} else if c == -1 {
			highestKey = svcKey
		}
	}

	if highestKey == "" {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "found no services in the exchange matched: org=%s, specRef=%s, version range=%s, arch=%s", org, url, versionRanges, arch)
	}
	return svcOutput.Services[highestKey]
}

func formSvcKey(org, url, arch string) string {
	return org + "_" + url + "_" + arch
}

type SvcMapValue struct {
	Org           string
	URL           string
	VersionRanges []string
	Arch          string
}

// CreateInputFile runs thru the services used by this pattern and collects the user input needed
func CreateInputFile(nodeOrg, pattern, arch, nodeIdTok, inputFile string) {
	var patOrg string
	patOrg, pattern = cliutils.TrimOrg(nodeOrg, pattern)	// patOrg will either get the prefix from pattern, or default to nodeOrg
	// Get the pattern
	exchangeUrl := cliutils.GetExchangeUrl()
	var patOutput exchange.GetPatternResponse
	cliutils.ExchangeGet(exchangeUrl, "orgs/"+patOrg+"/patterns/"+pattern, cliutils.OrgAndCreds(nodeOrg, nodeIdTok), []int{200}, &patOutput)
	patKey := cliutils.OrgAndCreds(patOrg, pattern)
	if _, ok := patOutput.Patterns[patKey]; !ok {
		cliutils.Fatal(cliutils.INTERNAL_ERROR, "did not find pattern '%s' as expected", patKey)
	}
	if arch == "" {
		arch = cutil.ArchString()
	}

	//todo: this needs to be recursive!!
	// Recursively go thru the services and their required services, collecting them in a map.
	// Afterward we will process them to figure out the highest version of each before getting their input.
	allRequiredSvcs := make(map[string]*SvcMapValue)	// the key is the combined org, url, arch. The value is the org, url, arch and a list of the versions.
	for _, svc := range patOutput.Patterns[patKey].Services {
		if svc.ServiceArch != arch { // filter out services that are not our arch
			fmt.Printf("Ignoring service that is a different architecture: %s, %s, %s\n", svc.ServiceOrg, svc.ServiceURL, svc.ServiceArch)
			continue
		}

		svcKey := formSvcKey(svc.ServiceOrg, svc.ServiceURL, svc.ServiceArch)
		if _, ok := allRequiredSvcs[svcKey]; !ok {
			allRequiredSvcs[svcKey] = &SvcMapValue{Org: svc.ServiceOrg, URL: svc.ServiceURL, Arch: svc.ServiceArch}		// this must be a ptr to the struct or go won't let us modify it in the map
		}
		for _, svcVersion := range svc.ServiceVersions {
			cliutils.Verbose("found: %s, %s, %s, %s", svc.ServiceOrg, svc.ServiceURL, svc.ServiceArch, svcVersion.Version)
			versionRange := "[" + svcVersion.Version + "," + svcVersion.Version + "]"	// the pattern specifies an exact version, turn that into a range
			allRequiredSvcs[svcKey].VersionRanges = append(allRequiredSvcs[svcKey].VersionRanges, versionRange) // add this version to this service in our map

			// Get the service from the exchange so we can get its required services
			exchId := cliutils.FormExchangeId(svc.ServiceURL, svcVersion.Version, svc.ServiceArch)
			var svcOutput exchange.GetServicesResponse
			cliutils.ExchangeGet(exchangeUrl, "orgs/"+svc.ServiceOrg+"/services/"+exchId, cliutils.OrgAndCreds(nodeOrg, nodeIdTok), []int{200}, &svcOutput)
			exSvcKey := cliutils.OrgAndCreds(svc.ServiceOrg, exchId)
			if _, ok := svcOutput.Services[exSvcKey]; !ok {
				cliutils.Fatal(cliutils.INTERNAL_ERROR, "did not find service '%s' in exchange as expected", exSvcKey)
			}

			// Loop thru this service's required services, adding them to our map
			for _, s := range svcOutput.Services[exSvcKey].RequiredServices {
				//todo: this is where we need to recurse
				cliutils.Verbose("found: %s, %s, %s, %s", s.Org, s.URL, s.Arch, s.Version)
				sKey := formSvcKey(s.Org, s.URL, s.Arch)
				if _, ok := allRequiredSvcs[sKey]; !ok {
					allRequiredSvcs[sKey] = &SvcMapValue{Org: svc.ServiceOrg, URL: svc.ServiceURL, Arch: svc.ServiceArch}
				}
				allRequiredSvcs[sKey].VersionRanges = append(allRequiredSvcs[sKey].VersionRanges, s.Version)	// s.Version is already a range
			}
		}
	}

	// Loop thru each service, find the highest version of that service, and then record the user input for it
	// Note: if the pattern references multiple versions of the same service (directly or indirectly), we create input for the highest version of the service.
	templateFile := InputFile{Global: []GlobalSet{}}
	//templateFile := InputFile{Global: []GlobalSet{{Type: "LocationAttributes", Variables: map[string]interface{}{"lat": 0.0, "lon": 0.0, "use_gps": false, "location_accuracy_km": 0.0}}}}
	for _, s := range allRequiredSvcs {
		svc := GetHighestService(exchangeUrl, nodeOrg, nodeIdTok, s.Org, s.URL, s.Arch, s.VersionRanges)

		// Get the user input from this service
		if len(svc.UserInputs) > 0 {
			svcInput := MicroWork{Org: s.Org, Url: s.URL, VersionRange: "[0.0.0,INFINITY)", Variables: make(map[string]interface{})}
			for _, u := range svc.UserInputs {
				svcInput.Variables[u.Name] = u.DefaultValue
			}
			templateFile.Services = append(templateFile.Services, svcInput)
		}
	}

	// Output the template file
	jsonBytes, err := json.MarshalIndent(templateFile, "", cliutils.JSON_INDENT)
	if err != nil {
		cliutils.Fatal(cliutils.INTERNAL_ERROR, "failed to marshal the user input template file: %v", err)
	}
	if err := ioutil.WriteFile(inputFile, jsonBytes, 0644); err != nil {
		cliutils.Fatal(cliutils.FILE_IO_ERROR, "problem writing the user input template file: %v", err)
	}
	fmt.Printf("Wrote %s\n", inputFile)
}
