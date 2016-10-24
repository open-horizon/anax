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
	"fmt"
	"flag"
	"log"
	"os"
	"path"
	// "path/filepath"
	// "io"
	"io/ioutil"
	// "reflect"
	"strings"
	"net/http"
	"runtime"

	// "github.com/davecgh/go-spew/spew"

	"repo.hovitos.engineering/MTN/go-policy"
	"repo.hovitos.engineering/MTN/go-policy/payload"
	"repo.hovitos.engineering/MTN/provider-tremor/config"
)

// This is the struct that can be input to stdin, and is also used to hold the cmd line values.
// Several of these structs come from the policy mgr.
// type CloudMsgBrokerTopicsStruct struct {
// 	Apps string `json:"apps"`
// 	PublishData []string `json:"publishData"`
// 	Control string `json:"control"`
// }

// type CloudMsgBrokerCredentialsStruct struct {
// 	User string `json:"user"`
// 	Password string `json:"password"`
// }

type QuarksStruct struct {
	CloudMsgBrokerHost string `json:"cloudMsgBrokerHost"`
	// CloudMsgBrokerTopics CloudMsgBrokerTopicsStruct `json:"cloudMsgBrokerTopics"`
	// CloudMsgBrokerCredentials CloudMsgBrokerCredentialsStruct `json:"cloudMsgBrokerCredentials"`
	DataVerificationInterval int `json:"dataVerificationInterval"`
}

type Input struct {
    Name                string              `json:"name"`
    APISpecs         	[]policy.APISpecification  `json:"apiSpec"`
    Arch                string  `json:"arch"`
    EthereumAccounts    []string  `json:"ethereumAccounts"`
    Quarks				QuarksStruct	`json:"quarks"`
    ResourceLimits    	policy.ResourceLimit       `json:"resourceLimits"`
}

// The output will go in the Policy struct

const FOOTER = "# vim: set ts=4 sw=4 expandtab:"
const IOTFHOSTSUFFIX = "internetofthings.ibmcloud.com"
const NEUTRONHOSTPROD = "bluehorizon.network"
const NEUTRONHOSTSTG = "staging.bluehorizon.hovitos.engineering"

func usage(exitCode int) {
	usageStr1 := "" +
		"Usage:\n" +
		"  cat <input json> | bhgovconfig [-d <output-dir>] [-t <template-dir>] [-root-dir <root-dir>] [-ethereum-accounts <acct-ids>]\n"+
		"\n"+
		"Create the blue horizon governor config files based on the specifief json properties piped to stdin,\n"+
		"and the commands options.  This command understands some of the common workload patterns like quarks apps\n"+
		"and expands input into the full governor policy file and provider.config file.  These files are placed in\n"+
		"specified <output-dir>.  The command also needs access to the governor policy template files, which should\n"+
		"be in the specified <template-dir>.\n"+
		"\n"+
		"Options:\n"
		// "  -d <output-dir>    The directory to put the config files in. Defaults to the current directory.\n"+
	usageStr2 := "\n" +
		"Examples:\n"+
		"  cat input.json | bhgovconfig -d /tmp\n"+
		"\n"+
		"Example input.json file:\n"+
		"{\n"+
		"    \"name\": \"netspeed\",\n"+
		"    \"apiSpec\": [\n"+
		"        {\n"+
		"            \"specRef\": \"https://bluehorizon.network/device-api/arm/netspeed\"\n"+
		"        }\n"+
		"    ],\n"+
		"    \"arch\": \"arm\",\n"+
		"    \"ethereumAccounts\": [ \"1a2b3c\", \"4d5e6f\" ],\n"+
		"    \"quarks\": {\n"+
		"        \"cloudMsgBrokerHost\": \"123abc.messaging.internetofthings.ibmcloud.com\",\n"+
		"        \"dataVerificationInterval\": 300\n"+
		"    }\n"+
		"}\n"
	if exitCode > 0 {
		fmt.Fprintf(os.Stderr, usageStr1)		// send it to stderr
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, usageStr2)		// send it to stderr
	} else {
		fmt.Printf(usageStr1)		// send it to stdout
		flag.PrintDefaults()		//todo: do not yet know how to get this to print to stdout
		fmt.Printf(usageStr2)		// send it to stdout
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
	if err != nil { log.Fatalf("error reading json properties from stdin: %v", err) }

	checkInput(input) 		// this will exit with error if something is wrong
}


/*
Check the input properties and make sure all require values are there.
*/
func checkInput(input *Input) {
	// When loadStdinFile() unmarshalled the json, it checked its syntax, but not for the specific field names

	if input.Name == "" { log.Fatal("error: the name of the contract workload must be specified\n") }

	if len(input.APISpecs) == 0 { log.Fatalf("error: no 'apiSpec' entries specified\n")	}
	for i, spec := range input.APISpecs {
		if spec.SpecRef == "" { log.Fatalf("error: apiSpec %v does not have a 'specRef' entry.\n", i) }
	}

	// if input.Quarks.CloudMsgBrokerHost == "" { log.Fatalf("error: no 'cloudMsgBrokerHost' entry specified in the 'quarks' object\n") } 		// in dev mode, empty host is appropriate
	// if input.Quarks.CloudMsgBrokerTopics.Apps == "" { log.Fatalf("error: no 'apps' entry specified in the 'quarks.cloudMsgBrokerTopics' object\n") }
	// if len(input.Quarks.CloudMsgBrokerTopics.PublishData) == 0 { log.Fatalf("error: no 'publishData' entry specified in the 'quarks.cloudMsgBrokerTopics' object\n") }
	// if input.Quarks.CloudMsgBrokerTopics.Control == "" { log.Fatalf("error: no 'control' entry specified in the 'quarks.cloudMsgBrokerTopics' object\n") }
	// if input.Quarks.CloudMsgBrokerCredentials.User == "" { log.Fatalf("error: no 'user' entry specified in the 'quarks.cloudMsgBrokerCredentials' object\n") }
	// if input.Quarks.CloudMsgBrokerCredentials.Password == "" { log.Fatalf("error: no 'password' entry specified in the 'quarks.cloudMsgBrokerCredentials' object\n") }
	// dataVerificationInterval is optional, if not specified it means do not check the data

	// typeStr := reflect.TypeOf(payload.MatchGroups)
	// field := reflect.ValueOf(payload).FieldByName("MatchGroups")
	// spew.Dump(field)

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
	// spew.Dump(flag.Lookup("name"))
	if name != "" { input.Name = name }
}

/*
Get an http.Response to a url. The caller must close the reader when done with it.
*/
func getHttpResponse(url string) (*http.Response) {
	fmt.Printf("Downloading %v...\n", url)
	resp, err := http.Get(url)
	if err != nil { log.Fatalf("error getting %v: %v", url, err) }
	return resp
}


/*
Download a file via http or https and return the byte array
*/
func getHttpFile(url string) ([]byte) {
	// out, err := os.Create(localFile)
	// if err != nil { log.Fatalf("error creating local file %v: %v", localFile, err) }
	// defer out.Close()
	// fmt.Printf("Downloading %v...\n", url)
	// resp, err := http.Get(url)
	// if err != nil { log.Fatalf("error getting %v: %v", url, err) }
	resp := getHttpResponse(url)
	defer resp.Body.Close()
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil { log.Fatalf("error reading %v: %v", url, err) }
	return bytes
	// n, err := io.Copy(out, resp.Body)
	// if err != nil { log.Fatalf("error writing file %v: %v, %v", localFile, n, err) }
}


/*
Create the appropriate policy content for the quarks workload pattern
*/
func createQuarksPolicyContent(input *Input, pol *policy.Policy, templateDir string) {
	// Only currently support iotf and neutron star as the data aggregation point
	host := strings.ToLower(input.Quarks.CloudMsgBrokerHost)
	if len(input.EthereumAccounts) == 0 {
		// the host is only required when not in dev mode
		if !strings.HasSuffix(host,"."+IOTFHOSTSUFFIX) && host != NEUTRONHOSTPROD && host != NEUTRONHOSTSTG {
			log.Fatalf("error: quarks.cloudMsgBrokerHost value %v must be '<org-id>.%v', '%v', or '%v'", host, IOTFHOSTSUFFIX, NEUTRONHOSTPROD, NEUTRONHOSTSTG)
		}
	}

	// Find the template file to use for workload info
	// there are 2 different forms of arch and specRef:
	apiUrl := input.APISpecs[0].SpecRef 		// eventually need to support more than 1 device api
	var deviceName, arch string
	if input.Arch != "" {
		// Arch is specified separately and specRef is like:  https://bluehorizon.network/documentation/cpu-temperature-device-api
		base := path.Base(apiUrl)
		if !strings.HasSuffix(base, "-device-api") { log.Fatalf("error: specRef does not end with '-device-api': %v", apiUrl) }
		deviceName = strings.TrimSuffix(base, "-device-api")
		arch = input.Arch
	} else {
		// Arch is embedded in specRef:  https://bluehorizon.network/device-api/arm/netspeed (or amd64 instead of arm)
		var dir string
		dir, deviceName = path.Split(apiUrl)
		arch = path.Base(dir)
	}
	// var hostType string
	// if strings.HasSuffix(host,"."+IOTFHOSTSUFFIX) {
	// 	hostType = "iotf"
	// } else {
	// 	hostType = "neutron"
	// }
	// templateName := "quarks-" + hostType + "-" + arch + "-" + deviceName
	templateName := "quarks-" + arch + "-" + deviceName
	// templatePath := flag.Lookup("t").Value.String() + "/" + templateName + ".json"
	templatePath := templateDir + "/" + templateName + ".json"

	var bytes []byte
	if strings.HasPrefix(templatePath, "http:") || strings.HasPrefix(templatePath, "https:") {
		// Its a url, download it
		bytes = getHttpFile(templatePath)
	} else {
		// Its a local file, read it
		var err error
		bytes, err = ioutil.ReadFile(templatePath)
		if err != nil { log.Fatalf("error reading template file %v: %v", templatePath, err) }
	}

	var template policy.Policy
	err := json.Unmarshal(bytes, &template)
	if err != nil { log.Fatalf("error parsing template file %v into json: %v", templatePath, err) }
	pol.Workloads = template.Workloads

	// Create the deployment_user_info string that contains deployment info the external developer is allowed to specify
	var depUserInfo QuarksStruct
	depUserInfo.CloudMsgBrokerHost = input.Quarks.CloudMsgBrokerHost
	// depUserInfo.CloudMsgBrokerTopics = input.Quarks.CloudMsgBrokerTopics
	// depUserInfo.CloudMsgBrokerCredentials = input.Quarks.CloudMsgBrokerCredentials
	depUserInfoBytes, err := json.Marshal(depUserInfo)
	if err != nil { log.Fatalf("error marshaling the user deployment info to json: %v", err) }
	depUserInfoStr := string(depUserInfoBytes)
	// spew.Dump(depUserInfoStr)
	// fmt.Printf("deployment_user_info: %v\n", depUserInfoStr)
	pol.Workloads[0].DeploymentUserInfo = depUserInfoStr

	// Create the data verification info
	interval := input.Quarks.DataVerificationInterval
	if interval != 0 {
		// They want data verification of the contract
		pol.DataVerify.Interval = interval
		pol.DataVerify.Enabled = false 		//todo: change to true when we have a data checking service for iotf topics
		pol.DataVerify.URL = host 		//todo: develop a service that can be used to check mqtt topics and include creds here
	}
}


/*
From the input info, create the appropriate policy content in output variable pol
*/
func createPolicyContent(input *Input, pol *policy.Policy, templateDir string) {
	pol.Header.Name = input.Name
	pol.Header.Version = "1.0"

	// Device api spec
	pol.APISpecs = make([]policy.APISpecification, len(input.APISpecs))
	for i, spec := range input.APISpecs {
		var version string
		if spec.Version == "" {
			version = "0.0.0" 	// this means any version >= to this value, so any version
		} else {
			version = spec.Version
		}
		pol.APISpecs[i] = policy.APISpecification{SpecRef: spec.SpecRef, Version: version, ExclusiveAccess: false}
	}

	pol.AgreementProtocols = make([]policy.AgreementProtocol, 1)
	pol.AgreementProtocols[0] = policy.AgreementProtocol{Name: "2PartyDataUse"} 	// for now we always use this contract

	// Find the correct workload template/pattern and fill it in and copy to pol
	if input.Quarks.CloudMsgBrokerHost != "" || input.Quarks.CloudMsgBrokerHost == "" { 		//todo: need a way to recognize they want quarks
		createQuarksPolicyContent(input, pol, templateDir) 		// quarks workload
	} else {
		// it is not a predefined workload we understand
		log.Fatalf("error: input does not specify a recognized predefined pattern")
	}

	// if ethAccts := flag.Lookup("ethereum-accounts").Value.String(); ethAccts != "" {
	if len(input.EthereumAccounts) > 0 {
		// Add the ethereum account ids to pol.Workloads.MatchGroups
		// accts := strings.Split(ethAccts, ",")
		// workload := pol.Workloads[0] 		//todo: support more that 1 workload in a policy
		// mGroups is a 2-d array.  Create the new inner entry, then we will append it to the outer array
		mInner := make([]payload.Match, len(input.EthereumAccounts))
		for i, a := range input.EthereumAccounts {
			 mInner[i] = payload.Match{Attr: "ethereum_account", Value: a}
		}
		pol.Workloads[0].MatchGroups = append(pol.Workloads[0].MatchGroups, mInner)
	}
}


/*
Write the contents of the policy struct to a file in the specified output dir
*/
func writePolicyFile(input *Input, pol *policy.Policy, policyOutputDir string) {
	name := pol.Header.Name 		//todo: remove chars that are bad for file names

	// there are 2 different forms of arch and specRef:
	apiUrl := pol.APISpecs[0].SpecRef 		// eventually need to support more than 1 device api
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
	if err != nil { log.Fatalf("error marshaling the policy info to json: %v", err) }

	err = ioutil.WriteFile(filename, []byte(newJson), 0664)
	if err != nil { log.Fatalf("error writing the policy info to %v: %v", filename, err) }
}


/*
From the input info, create the appropriate provider.config info
*/
func createConfigContent(input *Input, templateDir string) (*config.Config) {
	// templatePath := flag.Lookup("t").Value.String() + "/provider.config"
	templatePath := templateDir + "/provider.config"

	// Get the provider.config template file
	var configStruct *config.Config
	if strings.HasPrefix(templatePath, "http:") || strings.HasPrefix(templatePath, "https:") {
		// Its a url, download it and parse it
		resp := getHttpResponse(templatePath)
		defer resp.Body.Close()
		configStruct = &config.Config{}
		err := json.NewDecoder(resp.Body).Decode(configStruct)
		if err != nil { log.Fatalf("error decoding content of config file %v: %v", templatePath, err)
		}
	} else {
		// Its a local file, read it and parse it
		var err error
		configStruct, err = config.Read(templatePath)
		if err != nil { log.Fatalf("error parsing template policy file %v into json: %v", templatePath, err) }
	}

	// if flag.Lookup("ethereum-accounts").Value.String() != "" {
	if len(input.EthereumAccounts) > 0 {
		// In development mode we pay attention to the eth acct ids that the rpi advertises, so we can contract with our own.
		// (We also added the list of eth acct ids to the workload match groups.)
		configStruct.IgnoreContractWithAttribs = ""
		// configStruct.PayloadPath = ""   // the new template has this blanked
	}

	// In the specific case that this is our team and on x86 we want to leave EtcdUrl set.
	// Otherwise we blank it out
	if !( (runtime.GOARCH == "amd64" || runtime.GOARCH == "386") && (strings.ToLower(input.Quarks.CloudMsgBrokerHost)==NEUTRONHOSTPROD || strings.ToLower(input.Quarks.CloudMsgBrokerHost)==NEUTRONHOSTSTG) ) {
		configStruct.EtcdUrl = ""
	}

	//todo: fill in the valueExchange section, but it is not read yet

	return configStruct
}


/*
Write the contents of the provider.config struct to a file in the specified output dir
*/
func writeConfigFile(config *config.Config, outputDir string) {
	filename := outputDir + "/provider.config"

	// this does not indent the json
	// file, err := os.OpenFile(filepath.Clean(filename), os.O_WRONLY | os.O_CREATE | os.O_TRUNC, 0664)
	// if err != nil { log.Fatalf("error: unable to open %s for write: %v", filename, err) }
	// err = json.NewEncoder(file).Encode(config)
	// if err != nil { log.Fatalf("error: unable to encode content of %v: %v", filename, err) }

	newJson, err := json.MarshalIndent(config, "", "    ")
	if err != nil { log.Fatalf("error marshaling %v to json: %v", filename, err) }

	err = ioutil.WriteFile(filename, []byte(newJson), 0664)
	if err != nil { log.Fatalf("error writing %v: %v", filename, err) }
}


func main() {
	// Get and check cmd line options
	var name, outputDir, templateDir, rootDir, ethAccounts string
	// var developmentMode bool
	flag.StringVar(&name, "name", "", "name of this contract workload")
	flag.StringVar(&outputDir, "d", "/vol/provider-tremor/etc/provider-tremor", "output directory for the config files, and location for the templates")
	flag.StringVar(&templateDir, "t", "https://tor01.objectstorage.softlayer.net/v1/AUTH_bd05f276-e42f-4fa1-b7b3-780e8544769f/policy-templates", "url or directory that contains workload templates/patterns. Defaults to the standard Blue Horizon location in SoftLayer object store.")
	flag.StringVar(&rootDir, "root-dir", "/vol/provider-tremor/root", "directory that will be /root for the governor container")
	flag.StringVar(&ethAccounts, "ethereum-accounts", "", "comma separated list of ethereum account IDs that this governor should only contract with. Used for 'development mode'.")
	// flag.BoolVar(&developmentMode, "devel", false, "development mode - only contract with the RPi I am running on")
	flag.Usage = func() { usage(0) }
	flag.Parse()
	// spew.Dump(flag.Lookup("devel").Value.String())
	// fName := "quarks-neutron-netspeed.json"
	// getHttpFile(templateDir+"/"+fName, "/tmp/"+fName)
	// os.Exit(0)
	// Currently they need to specify the properties  stdin
	// if name == "" && !isStdinFile() { usage(0) }
	if !isStdinFile() { usage(0) }

	// Create output dirs if they do not exist
	policyOutputDir := outputDir + "/policy.d"
	err := os.MkdirAll(policyOutputDir, 0775)
	if err != nil { log.Fatalf("error creating directory %v: %v", policyOutputDir, err) }
	err = os.MkdirAll(rootDir+"/.colonus", 0775)
	if err != nil { log.Fatalf("error creating directory %v: %v", rootDir+"/.colonus", err) }
	err = os.MkdirAll(rootDir+"/.ethereum", 0775)
	if err != nil { log.Fatalf("error creating directory %v: %v", rootDir+"/.ethereum", err) }

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
	var policy policy.Policy 		// once will fill this in, we will marshal it into the output file
	createPolicyContent(&input, &policy, templateDir)

	// Write the policy info to a file in the output dir
	writePolicyFile(&input, &policy, policyOutputDir)

	// Create the provider.config file content
	config := createConfigContent(&input, templateDir)

	// Write the provider.config to a file in the output dir
	writeConfigFile(config, outputDir)

	// Create required directory_version file, but do not touch it if it exists
	dirVersionFilename := rootDir+"/.colonus/directory_version"
	if _, err := os.Stat(dirVersionFilename); os.IsNotExist(err) {
		err = ioutil.WriteFile(dirVersionFilename, []byte("0\n"), 0664)
		if err != nil { log.Fatalf("error writing %v: %v", rootDir+"/.colonus/directory_version", err) }
	}
}
