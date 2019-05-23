package register

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/apicommon"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	cliexchange "github.com/open-horizon/anax/cli/exchange"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
)

// These structs are used to parse the registration input file. These are also used by the hzn dev code.
type GlobalSet struct {
	Type         string                   `json:"type"`
	ServiceSpecs persistence.ServiceSpecs `json:"service_specs,omitempty"`
	Variables    map[string]interface{}   `json:"variables"`
}

func (g GlobalSet) String() string {
	return fmt.Sprintf("Global Array element, type: %v, service_specs: %v, variables: %v", g.Type, g.ServiceSpecs, g.Variables)
}

// Use for services. This is used by other cli sub-cmds too.
type MicroWork struct {
	Org          string                 `json:"org"`
	Url          string                 `json:"url"`
	VersionRange string                 `json:"versionRange,omitempty"` //optional
	Variables    map[string]interface{} `json:"variables"`
}

func (m MicroWork) String() string {
	return fmt.Sprintf("Org: %v, URL: %v, VersionRange: %v, Variables: %v", m.Org, m.Url, m.VersionRange, m.Variables)
}

type InputFile struct {
	Global   []GlobalSet `json:"global,omitempty"`
	Services []MicroWork `json:"services,omitempty"`
}

func ReadInputFile(filePath string, inputFileStruct *InputFile) {
	newBytes := cliconfig.ReadJsonFileWithLocalConfig(filePath)
	err := json.Unmarshal(newBytes, inputFileStruct)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to unmarshal json input file %s: %v", filePath, err)
	}
}

// DoIt registers this node to Horizon with a pattern
func DoIt(org, pattern, nodeIdTok, userPw, email, inputFile string, nodeOrgFromFlag string, patternFromFlag string, nodeName string) {
	// check the input
	if nodeOrgFromFlag != "" || patternFromFlag != "" {
		if org != "" || pattern != "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "-o and -p are mutually exclusive with <nodeorg> and <pattern> arguments.")
		} else {
			org = nodeOrgFromFlag
			pattern = patternFromFlag
		}
	}

	// get default org if needed
	if org == "" {
		org = os.Getenv("HZN_ORG_ID")
	}

	if org == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "Please specify the node organization id.")
	}
	if pattern == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "Please specify a pattern name.")
	}

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

	// get the arch from anax
	statusInfo := apicommon.Info{}
	cliutils.HorizonGet("status", []int{200}, &statusInfo, false)
	anaxArch := (*statusInfo.Configuration).Arch
	fmt.Printf("Get Arch from anax: %s\n", anaxArch)

	// Default node id and token if necessary
	nodeId, nodeToken := cliutils.SplitIdToken(nodeIdTok)
	if nodeId == "" {
		// Get the id from anax
		horDevice := api.HorizonDevice{}
		cliutils.HorizonGet("node", []int{200}, &horDevice, false)
		if horDevice.Id == nil {
			cliutils.Fatal(cliutils.ANAX_NOT_CONFIGURED_YET, "Failed to get proper response from the Horizon agent")
		}
		nodeId = *horDevice.Id

		if nodeId == "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "Please specify the node id and token using -n flag or HZN_EXCHANGE_NODE_AUTH environment variable.")
		} else {
			fmt.Printf("Using node ID '%s' from the Horizon agent\n", nodeId)
		}
	} else {
		// trim the org off the node id. the HZN_EXCHANGE_NODE_AUTH may contain the org id.
		_, nodeId = cliutils.TrimOrg(org, nodeId)
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

	if nodeName == "" {
		nodeName = nodeId
	}

	// See if the node exists in the exchange, and create if it doesn't
	httpCode := cliutils.ExchangeGet(exchUrlBase, "orgs/"+org+"/nodes/"+nodeId, cliutils.OrgAndCreds(org, nodeIdTok), nil, nil)

	if httpCode != 200 {
		if userPw == "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "node '%s/%s' does not exist in the exchange with the specified token, and the -u flag was not specified to provide exchange user credentials to create/update it.", org, nodeId)
		}
		fmt.Printf("Node %s/%s does not exist in the exchange with the specified token, creating/updating it...\n", org, nodeId)
		cliexchange.NodeCreate(org, "", nodeId, nodeToken, userPw, email, anaxArch, nodeName)
	} else {
		fmt.Printf("Node %s/%s exists in the exchange\n", org, nodeId)
	}

	// Initialize the Horizon device (node)
	fmt.Println("Initializing the Horizon node...")
	//nd := Node{Id: nodeId, Token: nodeToken, Org: org, Pattern: pattern, Name: nodeId, HA: false}
	falseVal := false
	nd := api.HorizonDevice{Id: &nodeId, Token: &nodeToken, Org: &org, Pattern: &pattern, Name: &nodeName, HA: &falseVal} //todo: support HA config
	httpCode, _ = cliutils.HorizonPutPost(http.MethodPost, "node", []int{201, 200, cliutils.ANAX_ALREADY_CONFIGURED}, nd)
	if httpCode == cliutils.ANAX_ALREADY_CONFIGURED {
		// Note: I wanted to make `hzn register` idempotent, but the anax api doesn't support changing existing settings once in configuring state (to maintain internal consistency).
		//		And i can't query ALL the existing settings to make sure they are what we were going to set, because i can't query the node token.
		cliutils.Fatal(cliutils.HTTP_ERROR, "this Horizon node is already registered or in the process of being registered. If you want to register it differently, run 'hzn unregister' first.")
	}

	// Process the input file and call /attribute, /service/config to set the specified variables
	if inputFile != "" {
		// Set the global variables as attributes with no url (or in the case of HTTPSBasicAuthAttributes, with url equal to image svr)
		// Technically the AgreementProtocolAttributes can be set, but it has no effect on anax if a pattern is being used.
		attr := api.NewAttribute("", "Global variables", false, false, map[string]interface{}{}) // we reuse this for each GlobalSet
		if len(inputFileStruct.Global) > 0 {
			fmt.Println("Setting global variables...")
		}
		for _, g := range inputFileStruct.Global {
			attr.Type = &g.Type
			attr.ServiceSpecs = &g.ServiceSpecs
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
		attr = api.NewAttribute("UserInputAttributes", "service", false, false, map[string]interface{}{}) // we reuse this for each service
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
			httpCode, respBody := cliutils.HorizonPutPost(http.MethodPost, "service/config", []int{201, 200, 400}, service)
			if httpCode == 400 {
				if matches := parseRegisterInputError(respBody); matches != nil && len(matches) > 2 {
					cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "Registration failed because %v Please update the services section in the input file %v. Run 'hzn unregister' and then 'hzn register...' again", matches[0], inputFile)
				}
				cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "%v", respBody)
			}
		}

	} else {
		// Technically an input file is not required, but it is not the common case, so warn them
		fmt.Println("Warning: no input file was specified. This is only valid if none of the services need variables set (including GPS coordinates).")
	}

	// Set the pattern and register the node
	fmt.Println("Changing Horizon state to configured to register this node with Horizon...")
	configuredStr := "configured"
	configState := api.Configstate{State: &configuredStr}
	httpCode, respBody := cliutils.HorizonPutPost(http.MethodPut, "node/configstate", []int{201, 200, 400}, configState)
	if httpCode == 400 {
		if matches := parseRegisterInputError(respBody); matches != nil && len(matches) > 2 {
			err_string := fmt.Sprintf("Registration failed because %v", matches[0])
			if inputFile != "" {
				cliutils.Fatal(cliutils.CLI_INPUT_ERROR, err_string+" Please define variables for service %v in the input file %v. Run 'hzn unregister' and then 'hzn register...' again", matches[2], inputFile)
			} else {
				cliutils.Fatal(cliutils.CLI_INPUT_ERROR, err_string+" Please create an input file, define variables for service %v. Run 'hzn unregister' and then 'hzn register...' again with the -f flag to specify the input file.", matches[2])
			}
		}
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "%v", respBody)
	}
	var getDevicesResp exchange.GetDevicesResponse
	cliutils.ExchangeGet(exchUrlBase, "orgs/"+org+"/nodes/"+nodeId, cliutils.OrgAndCreds(org, nodeIdTok), []int{200}, &getDevicesResp)
	fmt.Printf("getDevicesResp: %v", getDevicesResp)

	// if arch is not set, set the arch with anax's arch
	devices := getDevicesResp.Devices
	node := org + "/" + nodeId
	device := devices[node]
	archFromExchange := device.Arch

	fmt.Printf("device: %v", device)
	fmt.Printf("archFromExchange: %v", archFromExchange)

	if archFromExchange == "" {
		// update node arch with anax arch
		fmt.Printf("archFromNode is empty, update node arch with anax arch %v", anaxArch)
		putDeviceReq := exchange.PutDeviceRequest{device.Token, device.Name, device.Pattern, device.RegisteredServices, device.MsgEndPoint, device.SoftwareVersions, device.PublicKey, anaxArch}
		cliutils.ExchangePutPost(http.MethodPut, exchUrlBase, "orgs/"+org+"/nodes/"+nodeId, cliutils.OrgAndCreds(org, userPw), []int{200, 201}, putDeviceReq)
	} else if archFromExchange != anaxArch {
		cliutils.Fatal(cliutils.INTERNAL_ERROR, "node arch from Exchange does not match arch from Anax")
	}

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
	return false // was not within any of the ranges
}

// GetHighestService queries the exchange for all versions of this service and returns the highest version that is within at least 1 of the version ranges
func GetHighestService(nodeCreds, org, url, arch string, versionRanges []string) exchange.ServiceDefinition {
	route := "orgs/" + org + "/services?url=" + url + "&arch=" + arch // get all services of this org, url, and arch
	var svcOutput exchange.GetServicesResponse
	cliutils.SetWhetherUsingApiKey(nodeCreds)
	cliutils.ExchangeGet(cliutils.GetExchangeUrl(), route, nodeCreds, []int{200}, &svcOutput)
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
	Org            string
	URL            string
	Arch           string
	VersionRanges  []string // all the version ranges we find for this service as we descend thru the required services
	HighestVersion string   // filled in when we have to find the highest service to get its required services. Is valid at the end if len(VersionRanges)==1
	UserInputs     []exchange.UserInput
}

// AddAllRequiredSvcs
func AddAllRequiredSvcs(nodeCreds, org, url, arch, versionRange string, allRequiredSvcs map[string]*SvcMapValue) {
	// Add this service to the service map
	cliutils.Verbose("found: %s, %s, %s, %s", org, url, arch, versionRange)
	svcKey := formSvcKey(org, url, arch)
	if s, ok := allRequiredSvcs[svcKey]; ok {
		// To protect against circular service references, check if we've already seen this exact svc version range
		for _, v := range s.VersionRanges {
			if v == versionRange {
				return
			}
		}
	} else {
		allRequiredSvcs[svcKey] = &SvcMapValue{Org: org, URL: url, Arch: arch} // this must be a ptr to the struct or go won't let us modify it in the map
	}
	allRequiredSvcs[svcKey].VersionRanges = append(allRequiredSvcs[svcKey].VersionRanges, versionRange) // add this version to this service in our map

	// Get the service from the exchange so we can get its required services
	highestSvc := GetHighestService(nodeCreds, org, url, arch, []string{versionRange})
	allRequiredSvcs[svcKey].HighestVersion = highestSvc.Version // in case we don't encounter this service again, we already know the highest version for getting the user input from
	allRequiredSvcs[svcKey].UserInputs = highestSvc.UserInputs

	// Loop thru this service's required services, adding them to our map
	for _, s := range highestSvc.RequiredServices {
		// This will add this svc to our map and keep descending down the required services
		AddAllRequiredSvcs(nodeCreds, s.Org, s.URL, s.Arch, s.Version, allRequiredSvcs)
	}
}

// CreateInputFile runs thru the services used by this pattern (descending into all required services) and collects the user input needed
func CreateInputFile(nodeOrg, pattern, arch, nodeIdTok, inputFile string) {
	var patOrg string
	patOrg, pattern = cliutils.TrimOrg(nodeOrg, pattern) // patOrg will either get the prefix from pattern, or default to nodeOrg
	nodeCreds := cliutils.OrgAndCreds(nodeOrg, nodeIdTok)

	// Get the pattern
	var patOutput exchange.GetPatternResponse
	cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+patOrg+"/patterns/"+pattern, nodeCreds, []int{200}, &patOutput)
	patKey := cliutils.OrgAndCreds(patOrg, pattern)
	if _, ok := patOutput.Patterns[patKey]; !ok {
		cliutils.Fatal(cliutils.INTERNAL_ERROR, "did not find pattern '%s' as expected", patKey)
	}
	if arch == "" {
		arch = cutil.ArchString()
	}

	// Recursively go thru the services and their required services, collecting them in a map.
	// Afterward we will process them to figure out the highest version of each before getting their input.
	allRequiredSvcs := make(map[string]*SvcMapValue) // the key is the combined org, url, arch. The value is the org, url, arch and a list of the versions.
	for _, svc := range patOutput.Patterns[patKey].Services {
		if svc.ServiceArch != arch { // filter out services that are not our arch
			fmt.Printf("Ignoring service that is a different architecture: %s, %s, %s\n", svc.ServiceOrg, svc.ServiceURL, svc.ServiceArch)
			continue
		}

		for _, svcVersion := range svc.ServiceVersions {
			// This will add this svc to our map and keep descending down the required services
			AddAllRequiredSvcs(nodeCreds, svc.ServiceOrg, svc.ServiceURL, svc.ServiceArch, svcVersion.Version, allRequiredSvcs) // svcVersion.Version is a version range
		}
	}

	// Loop thru each service, find the highest version of that service, and then record the user input for it
	// Note: if the pattern references multiple versions of the same service (directly or indirectly), we create input for the highest version of the service.
	templateFile := InputFile{Global: []GlobalSet{}} // to add the global loc attrs: Type: "LocationAttributes", Variables: map[string]interface{}{"lat": 0.0, "lon": 0.0, "use_gps": false, "location_accuracy_km": 0.0}
	for _, s := range allRequiredSvcs {
		var userInput []exchange.UserInput
		if s.HighestVersion != "" && len(s.VersionRanges) <= 1 {
			// When we were finding the required services we only encountered this service once, so the user input we found then is valid
			userInput = s.UserInputs
		} else {
			svc := GetHighestService(nodeCreds, s.Org, s.URL, s.Arch, s.VersionRanges)
			userInput = svc.UserInputs
		}

		// Get the user input from this service
		if len(userInput) > 0 {
			svcInput := MicroWork{Org: s.Org, Url: s.URL, VersionRange: "[0.0.0,INFINITY)", Variables: make(map[string]interface{})}
			for _, u := range userInput {
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

// this function parses the error returned by the registration process to see if the error
// is due to the missing input variable or wrong input variable type for a service.
// It returns a string array in the following format:
// [error, variable name, service org/service url]
// if it returns an empty array, then there is no match.
func parseRegisterInputError(resp string) []string {
	// match the ANAX_SVC_MISSING_VARIABLE
	tmplt_miss_var := strings.Replace(cutil.ANAX_SVC_MISSING_VARIABLE, "%v", "([^\\s]*)", -1)
	re1 := regexp.MustCompile(tmplt_miss_var)
	matches := re1.FindStringSubmatch(resp)

	// match ANAX_SVC_WRONG_TYPE
	if matches == nil || len(matches) == 0 {
		tmplt_wrong_type := strings.Replace(cutil.ANAX_SVC_WRONG_TYPE, "%v", "([^\\s]*)", -1) + "type [^.]*[.]"
		re2 := regexp.MustCompile(tmplt_wrong_type)
		matches = re2.FindStringSubmatch(resp)
	}

	// match ANAX_SVC_MISSING_CONFIG
	if matches == nil || len(matches) == 0 {
		tmplt_missing_config := strings.Replace(cutil.ANAX_SVC_MISSING_CONFIG, "%v", "([^\\s]*)", -1)
		re3 := regexp.MustCompile(tmplt_missing_config)
		matches = re3.FindStringSubmatch(resp)
		if matches != nil && len(matches) > 2 {
			// no variable name for this
			matches[1] = ""
		}
	}
	return matches
}
