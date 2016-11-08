/*
This utility cmd allows the horizon external developer to specify governor config information
in a simple way and then expands it to create a complete gov policy file for 1 contract proposal.
It also understands some predefined patterns that enables the user to specify even less, higher
level info, that is then translated to the base policy file properties.  See below for the cmd usage.
*/

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/open-horizon/anax/citizenscientist"
	"github.com/open-horizon/anax/policy"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
)

// This is the struct that can be input to stdin, and is also used to hold the cmd line values.
// Several of these structs come from the policy mgr.

type QuarksStruct struct {
	CloudMsgBrokerHost       string `json:"cloudMsgBrokerHost"`
	DataVerificationInterval int    `json:"dataVerificationInterval"`
}

type Input struct {
	Name             string                    `json:"name"`
	APISpecs         []policy.APISpecification `json:"apiSpec"`
	Arch             string                    `json:"arch"`
	EthereumAccounts []string                  `json:"ethereumAccounts"`
	Quarks           QuarksStruct              `json:"quarks"`
	ResourceLimits   policy.ResourceLimit      `json:"resourceLimits"`
}

// The output will go in the Policy struct

const FOOTER = "# vim: set ts=4 sw=4 expandtab:"
const IOTFHOSTSUFFIX = "internetofthings.ibmcloud.com"
const NEUTRONHOSTPROD = "bluehorizon.network"
const NEUTRONHOSTSTG = "staging.bluehorizon.hovitos.engineering"

func usage(exitCode int) {
	usageStr1 := "" +
		"Usage:\n" +
		"  cat <input json> | bhgovconfig [-d <output-dir>] [-t <template-dir>] [-root-dir <root-dir>] [-ethereum-accounts <acct-ids>]\n" +
		"\n" +
		"Create the blue horizon governor config files based on the specified json properties piped to stdin,\n" +
		"and the commands options.  This command understands some of the common workload patterns like quarks apps\n" +
		"and expands input into the full governor policy file and config file.  These files are placed in\n" +
		"specified <output-dir>.  The command also needs access to the governor policy template files, which should\n" +
		"be in the specified <template-dir>.\n" +
		"\n" +
		"Options:\n"
		// "  -d <output-dir>    The directory to put the config files in. Defaults to the current directory.\n"+
	usageStr2 := "\n" +
		"Examples:\n" +
		"  cat input.json | bhgovconfig -d /tmp\n" +
		"\n" +
		"Example input.json file:\n" +
		"{\n" +
		"    \"name\": \"netspeed\",\n" +
		"    \"apiSpec\": [\n" +
		"        {\n" +
		"            \"specRef\": \"https://bluehorizon.network/documentation/netspeed-device-api\"\n" +
		"        }\n" +
		"    ],\n" +
		"    \"arch\": \"arm\",\n" +
		"    \"ethereumAccounts\": [ \"1a2b3c\", \"4d5e6f\" ],\n" +
		"    \"quarks\": {\n" +
		"        \"cloudMsgBrokerHost\": \"123abc.messaging.internetofthings.ibmcloud.com\",\n" +
		"        \"dataVerificationInterval\": 300\n" +
		"    }\n" +
		"}\n"
	if exitCode > 0 {
		fmt.Fprintf(os.Stderr, usageStr1) // send it to stderr
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, usageStr2) // send it to stderr
	} else {
		fmt.Printf(usageStr1) // send it to stdout
		flag.PrintDefaults()  //todo: do not yet know how to get this to print to stdout
		fmt.Printf(usageStr2) // send it to stdout
	}
	os.Exit(exitCode)
}

/*
Get the json from stdin and populate the fields of in the input struct.
*/
func loadStdinFile(input *Input) {
	// deserialize the json from stdin
	buf := bytes.NewBuffer([]byte{})
	buf.ReadFrom(os.Stdin)

	err := json.Unmarshal(buf.Bytes(), &input)
	if err != nil {
		log.Fatalf("error reading json properties from stdin: %v", err)
	}

	checkInput(input) // this will exit with error if something is wrong
}

/*
Check the input properties and make sure all require values are there.
*/
func checkInput(input *Input) {
	// When loadStdinFile() unmarshalled the json, it checked its syntax, but not for the specific field names

	if input.Name == "" {
		log.Fatal("error: the name of the workload must be specified\n")
	}

	if len(input.APISpecs) == 0 {
		log.Fatalf("error: no 'apiSpec' entries specified\n")
	}
	for i, spec := range input.APISpecs {
		if spec.SpecRef == "" {
			log.Fatalf("error: apiSpec %v does not have a 'specRef' entry.\n", i)
		}
	}

	// If we get here, everything checked out
}

/*
Return true if a file is piped into stdin.
*/
func isStdinFile() bool {
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		return true
	} else {
		return false
	}
}

/*
Get policy properties via the cmd line options
*/
func getCliProperties(input *Input) {
	name := flag.Lookup("name").Value.String()
	if name != "" {
		input.Name = name
	}
}

/*
Get an http.Response to a url. The caller must close the reader when done with it.
*/
func getHttpResponse(url string) *http.Response {
	fmt.Printf("Downloading %v...\n", url)
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("error getting %v: %v", url, err)
	}
	return resp
}

/*
Download a file via http or https and return the byte array
*/
func getHttpFile(url string) []byte {

	resp := getHttpResponse(url)
	defer resp.Body.Close()
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("error reading %v: %v", url, err)
	}
	return bytes

}

/*
Create the appropriate policy content for the quarks workload pattern
*/
func createQuarksPolicyContent(input *Input, pol *policy.Policy, templateDir string) {
	// Only currently support iotf and neutron star as the data aggregation point
	host := strings.ToLower(input.Quarks.CloudMsgBrokerHost)
	if len(input.EthereumAccounts) == 0 {
		// the host is only required when not in dev mode
		if !strings.HasSuffix(host, "."+IOTFHOSTSUFFIX) && host != NEUTRONHOSTPROD && host != NEUTRONHOSTSTG {
			log.Fatalf("error: quarks.cloudMsgBrokerHost value %v must be '<org-id>.%v', '%v', or '%v'", host, IOTFHOSTSUFFIX, NEUTRONHOSTPROD, NEUTRONHOSTSTG)
		}
	}

	// Find the template file to use for workload info
	// there are 2 different forms of arch and specRef:
	apiUrl := input.APISpecs[0].SpecRef // eventually need to support more than 1 device api
	var deviceName, arch string
	if input.Arch != "" {
		// Arch is specified separately and specRef is like:  https://bluehorizon.network/documentation/cpu-temperature-device-api
		base := path.Base(apiUrl)
		if !strings.HasSuffix(base, "-device-api") {
			log.Fatalf("error: specRef does not end with '-device-api': %v", apiUrl)
		}
		deviceName = strings.TrimSuffix(base, "-device-api")
		arch = input.Arch
	} else {
		// Arch is embedded in specRef:  https://bluehorizon.network/device-api/arm/netspeed (or amd64 instead of arm)
		var dir string
		dir, deviceName = path.Split(apiUrl)
		arch = path.Base(dir)
	}

	templateName := "quarks-" + arch + "-" + deviceName
	templatePath := templateDir + "/" + templateName + ".json"

	var bytes []byte
	if strings.HasPrefix(templatePath, "http:") || strings.HasPrefix(templatePath, "https:") {
		// Its a url, download it
		bytes = getHttpFile(templatePath)
	} else {
		// Its a local file, read it
		var err error
		bytes, err = ioutil.ReadFile(templatePath)
		if err != nil {
			log.Fatalf("error reading template file %v: %v", templatePath, err)
		}
	}

	var template policy.Policy
	err := json.Unmarshal(bytes, &template)
	if err != nil {
		log.Fatalf("error parsing template file %v into json: %v", templatePath, err)
	}
	pol.Workloads = template.Workloads

	// Create the deployment_user_info string that contains deployment info the external developer is allowed to specify
	var depUserInfo QuarksStruct
	depUserInfo.CloudMsgBrokerHost = input.Quarks.CloudMsgBrokerHost
	depUserInfoBytes, err := json.Marshal(depUserInfo)
	if err != nil {
		log.Fatalf("error marshaling the user deployment info to json: %v", err)
	}
	depUserInfoStr := string(depUserInfoBytes)
	pol.Workloads[0].DeploymentUserInfo = depUserInfoStr

	// Create the data verification info
	interval := input.Quarks.DataVerificationInterval
	if interval != 0 {
		// They want data verification of the contract
		pol.DataVerify.Interval = interval
		pol.DataVerify.Enabled = false //TODO: change to true when we have a data checking service for iotf topics
		pol.DataVerify.URL = host      //TODO: develop a service that can be used to check mqtt topics and include creds here
	}
}

/*
From the input info, create the appropriate policy content in output variable pol
*/
func createPolicyContent(input *Input, pol *policy.Policy, templateDir string) {
	pol.Header.Name = input.Name
	pol.Header.Version = policy.CurrentVersion

	// Device api spec
	pol.APISpecs = make([]policy.APISpecification, len(input.APISpecs))
	for i, spec := range input.APISpecs {
		var version string
		if spec.Version == "" {
			version = "0.0.0" // this means any version >= to this value, so any version
		} else {
			version = spec.Version
		}
		pol.APISpecs[i] = *policy.APISpecification_Factory(spec.SpecRef, version, input.Arch)
	}

	pol.AgreementProtocols = make([]policy.AgreementProtocol, 1)
	pol.AgreementProtocols[0] = policy.AgreementProtocol{Name: citizenscientist.PROTOCOL_NAME} // for now we always use this contract

	// Find the correct workload template/pattern and fill it in and copy to pol
	if input.Quarks.CloudMsgBrokerHost != "" || input.Quarks.CloudMsgBrokerHost == "" { //todo: need a way to recognize they want quarks
		createQuarksPolicyContent(input, pol, templateDir) // quarks workload
	} else {
		// it is not a predefined workload we understand
		log.Fatalf("error: input does not specify a recognized predefined pattern")
	}

	// Put all the ethereum accounts into an "and"ed array
	if len(input.EthereumAccounts) > 0 {

		rp := policy.RequiredProperty_Factory()
		(*rp)["and"] = make([]interface{}, 0, 10)

		for _, a := range input.EthereumAccounts {
			pe := policy.PropertyExpression_Factory("ethereum_account", a, "=")
			(*rp)["and"] = append((*rp)["and"].([]interface{}), pe)
		}
		pol.CounterPartyProperties = *rp
	}
}

/*
Write the contents of the policy struct to a file in the specified output dir
*/
func writePolicyFile(input *Input, pol *policy.Policy, policyOutputDir string) {
	name := pol.Header.Name //todo: remove chars that are bad for file names

	// there are 2 different forms of arch and specRef:
	apiUrl := pol.APISpecs[0].SpecRef // eventually need to support more than 1 device api
	var arch string
	if input.Arch != "" {
		// Arch is specified separately
		arch = input.Arch
	} else {
		// Arch is embedded in specRef:  https://bluehorizon.network/device-api/arm/netspeed (or amd64 instead of arm)
		dir, _ := path.Split(apiUrl)
		arch = path.Base(dir)
	}

	filename := policyOutputDir + "/" + name + "-" + arch + ".policy"

	newJson, err := json.MarshalIndent(pol, "", "    ")
	if err != nil {
		log.Fatalf("error marshaling the policy info to json: %v", err)
	}

	err = ioutil.WriteFile(filename, []byte(newJson), 0664)
	if err != nil {
		log.Fatalf("error writing the policy info to %v: %v", filename, err)
	}
}

/*
From the input info, create the appropriate provider.config info
*/
// func createConfigContent(input *Input, templateDir string) (*config.Config) {
// 	// templatePath := flag.Lookup("t").Value.String() + "/provider.config"
// 	templatePath := templateDir + "/provider.config"

// 	// Get the provider.config template file
// 	var configStruct *config.Config
// 	if strings.HasPrefix(templatePath, "http:") || strings.HasPrefix(templatePath, "https:") {
// 		// Its a url, download it and parse it
// 		resp := getHttpResponse(templatePath)
// 		defer resp.Body.Close()
// 		configStruct = &config.Config{}
// 		err := json.NewDecoder(resp.Body).Decode(configStruct)
// 		if err != nil { log.Fatalf("error decoding content of config file %v: %v", templatePath, err)
// 		}
// 	} else {
// 		// Its a local file, read it and parse it
// 		var err error
// 		configStruct, err = config.Read(templatePath)
// 		if err != nil { log.Fatalf("error parsing template policy file %v into json: %v", templatePath, err) }
// 	}

// 	// if flag.Lookup("ethereum-accounts").Value.String() != "" {
// 	if len(input.EthereumAccounts) > 0 {
// 		// In development mode we pay attention to the eth acct ids that the rpi advertises, so we can contract with our own.
// 		// (We also added the list of eth acct ids to the workload match groups.)
// 		configStruct.IgnoreContractWithAttribs = ""
// 		// configStruct.PayloadPath = ""   // the new template has this blanked
// 	}

// 	// In the specific case that this is our team and on x86 we want to leave EtcdUrl set.
// 	// Otherwise we blank it out
// 	if !( (runtime.GOARCH == "amd64" || runtime.GOARCH == "386") && (strings.ToLower(input.Quarks.CloudMsgBrokerHost)==NEUTRONHOSTPROD || strings.ToLower(input.Quarks.CloudMsgBrokerHost)==NEUTRONHOSTSTG) ) {
// 		configStruct.EtcdUrl = ""
// 	}

// 	//todo: fill in the valueExchange section, but it is not read yet

// 	return configStruct
// }

/*
Write the contents of the provider.config struct to a file in the specified output dir
*/
// func writeConfigFile(config *config.Config, outputDir string) {
// 	filename := outputDir + "/provider.config"

// 	// this does not indent the json
// 	// file, err := os.OpenFile(filepath.Clean(filename), os.O_WRONLY | os.O_CREATE | os.O_TRUNC, 0664)
// 	// if err != nil { log.Fatalf("error: unable to open %s for write: %v", filename, err) }
// 	// err = json.NewEncoder(file).Encode(config)
// 	// if err != nil { log.Fatalf("error: unable to encode content of %v: %v", filename, err) }

// 	newJson, err := json.MarshalIndent(config, "", "    ")
// 	if err != nil { log.Fatalf("error marshaling %v to json: %v", filename, err) }

// 	err = ioutil.WriteFile(filename, []byte(newJson), 0664)
// 	if err != nil { log.Fatalf("error writing %v: %v", filename, err) }
// }

func main() {
	// Get and check cmd line options
	var name, outputDir, templateDir, rootDir, ethAccounts string
	// var developmentMode bool
	flag.StringVar(&name, "name", "", "name of this workload")
	flag.StringVar(&outputDir, "d", "/vol/provider-tremor/etc/provider-tremor", "output directory for the config files, and location for the templates")
	flag.StringVar(&templateDir, "t", "https://tor01.objectstorage.softlayer.net/v1/AUTH_bd05f276-e42f-4fa1-b7b3-780e8544769f/policy-templates", "url or directory that contains workload templates/patterns. Defaults to the standard Blue Horizon location in SoftLayer object store.")
	flag.StringVar(&rootDir, "root-dir", "/vol/provider-tremor/root", "directory that will be /root for the governor container")
	flag.StringVar(&ethAccounts, "ethereum-accounts", "", "comma separated list of ethereum account IDs that this governor should only contract with. Used for 'development mode'.")
	flag.Usage = func() { usage(0) }
	flag.Parse()

	// Currently they need to specify the properties in stdin
	if !isStdinFile() {
		usage(0)
	}

	// Create output dirs if they do not exist
	policyOutputDir := outputDir + "/policy.d"
	err := os.MkdirAll(policyOutputDir, 0775)
	if err != nil {
		log.Fatalf("error creating directory %v: %v", policyOutputDir, err)
	}
	err = os.MkdirAll(rootDir+"/.colonus", 0775)
	if err != nil {
		log.Fatalf("error creating directory %v: %v", rootDir+"/.colonus", err)
	}
	err = os.MkdirAll(rootDir+"/.ethereum", 0775)
	if err != nil {
		log.Fatalf("error creating directory %v: %v", rootDir+"/.ethereum", err)
	}

	// If they are piping a json file to stdin, load that as default values
	var input Input
	if isStdinFile() {
		loadStdinFile(&input)
	}
	// overlay any cli args that overlap with stdin values
	if ethAccounts != "" {
		accts := strings.Split(ethAccounts, ",")
		input.EthereumAccounts = accts
	}

	// Get properties specified thru cmd line options and overlay them on the input struct
	getCliProperties(&input)

	// Create the policy file content
	var policy policy.Policy // once we fill this in, we will marshal it into the output file
	createPolicyContent(&input, &policy, templateDir)

	// Write the policy info to a file in the output dir
	writePolicyFile(&input, &policy, policyOutputDir)

	// Create the provider.config file content
	// config := createConfigContent(&input, templateDir)

	// Write the provider.config to a file in the output dir
	// writeConfigFile(config, outputDir)

	// Create required directory_version file, but do not touch it if it exists
	dirVersionFilename := path.Join(os.Getenv("SNAP_COMMON"), "eth", "directory_version")
	if _, err := os.Stat(dirVersionFilename); os.IsNotExist(err) {
		err = ioutil.WriteFile(dirVersionFilename, []byte("0\n"), 0664)
		if err != nil {
			log.Fatalf("error writing %v: %v", dirVersionFilename, err)
		}
	}
}
