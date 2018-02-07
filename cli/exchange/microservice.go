package exchange

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/containermessage"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/rsapss-tool/sign"
	"github.com/open-horizon/rsapss-tool/verify"
	"net/http"
	"os"
	"reflect"
	"strings"
)

type DeploymentConfig struct {
	Services map[string]*containermessage.Service `json:"services"`
}

func (dc DeploymentConfig) CLIString() string {
	servs := ""
	for serviceName, _ := range dc.Services {
		servs += serviceName + ", "
	}
	servs = servs[:len(servs)-2]
	return fmt.Sprintf("service(s) %v", servs)
}

func (dc DeploymentConfig) String() string {

	res := ""
	for serviceName, deployConfig := range dc.Services {
		res += fmt.Sprintf("service: %v, config: %v", serviceName, deployConfig)
	}

	return res
}

func (dc DeploymentConfig) HasAnyServices() bool {
	if len(dc.Services) == 0 {
		return false
	}
	return true
}

// A validation method. Is there enough info in the deployment config to start a container? If not, the
// missing info is returned in the error message. Note that when there is a complete absence of deployment
// config metadata, that's ok too for microservices.
func (dc DeploymentConfig) CanStartStop() error {
	if len(dc.Services) == 0 {
		return nil
		// return errors.New(fmt.Sprintf("no services defined"))
	} else {
		for serviceName, service := range dc.Services {
			if len(serviceName) == 0 {
				return errors.New(fmt.Sprintf("no service name"))
			} else if len(service.Image) == 0 {
				return errors.New(fmt.Sprintf("no docker image for service %s", serviceName))
			}
		}
	}
	return nil
}

type WorkloadDeployment struct {
	//Deployment          DeploymentConfig `json:"deployment"`
	Deployment          interface{} `json:"deployment"`
	DeploymentSignature string      `json:"deployment_signature"`
	Torrent             string      `json:"torrent"`
}

type MicroserviceFile struct {
	Org           string               `json:"org"` // optional
	Label         string               `json:"label"`
	Description   string               `json:"description"`
	Public        bool                 `json:"public"`
	SpecRef       string               `json:"specRef"`
	Version       string               `json:"version"`
	Arch          string               `json:"arch"`
	Sharable      string               `json:"sharable"`
	DownloadURL   string               `json:"downloadUrl"`
	MatchHardware map[string]string    `json:"matchHardware"`
	UserInputs    []exchange.UserInput `json:"userInput"`
	Workloads     []WorkloadDeployment `json:"workloads"`
}

// Take the deployment field, which we have told the json unmarshaller was unknown type (so we can handle both escaped string and struct)
// and turn it into the DeploymentConfig struct we really want.
func ConvertToDeploymentConfig(deployment interface{}) *DeploymentConfig {
	var jsonBytes []byte
	var err error

	// Take whatever type the deployment field is and convert it to marshalled json bytes
	switch d := deployment.(type) {
	case string:
		if len(d) == 0 {
			return nil
		}
		// In the original input file this was escaped json as a string, but the original unmarshal removed the escapes
		jsonBytes = []byte(d)
	case nil:
		return nil
	default:
		// The only other valid input is regular json in DeploymentConfig structure. Marshal it back to bytes so we can unmarshal it in a way that lets Go know it is a DeploymentConfig
		jsonBytes, err = json.Marshal(d)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal body for %v: %v", d, err)
		}
	}

	// Now unmarshal the bytes into the struct we have wanted all along
	depConfig := new(DeploymentConfig)
	err = json.Unmarshal(jsonBytes, depConfig)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to unmarshal json for deployment field %s: %v", string(jsonBytes), err)
	}

	return depConfig
}

// Convert the first Deployment Configuration to a full Deployment Description.
func (mf *MicroserviceFile) ConvertToDeploymentDescription() (*DeploymentConfig, *containermessage.DeploymentDescription, error) {
	for _, wl := range mf.Workloads {
		depConfig := ConvertToDeploymentConfig(wl.Deployment)
		return depConfig, &containermessage.DeploymentDescription{
			Services: depConfig.Services,
			ServicePattern: containermessage.Pattern{
				Shared: map[string][]string{},
			},
			Infrastructure: true,
			Overrides:      map[string]*containermessage.Service{},
		}, nil
	}
	return nil, nil, errors.New(fmt.Sprintf("has no containers to execute"))
}

// Verify that non default user inputs are set in the input map.
func (mf *MicroserviceFile) RequiredVariablesAreSet(setVars map[string]interface{}) error {
	for _, ui := range mf.UserInputs {
		if ui.DefaultValue == "" && ui.Name != "" {
			if _, ok := setVars[ui.Name]; !ok {
				return errors.New(fmt.Sprintf("user input %v has no default value and is not set", ui.Name))
			}
		}
	}
	return nil
}

// Returns true if the microservice definition userinputs define the variable.
func (mf *MicroserviceFile) DefinesVariable(name string) string {
	for _, ui := range mf.UserInputs {
		if ui.Name == name && ui.Type != "" {
			return ui.Type
		}
	}
	return ""
}

type MicroserviceInput struct {
	Label         string                        `json:"label"`
	Description   string                        `json:"description"`
	Public        bool                          `json:"public"`
	SpecRef       string                        `json:"specRef"`
	Version       string                        `json:"version"`
	Arch          string                        `json:"arch"`
	Sharable      string                        `json:"sharable"`
	DownloadURL   string                        `json:"downloadUrl"`
	MatchHardware map[string]string             `json:"matchHardware"`
	UserInputs    []exchange.UserInput          `json:"userInput"`
	Workloads     []exchange.WorkloadDeployment `json:"workloads"`
}

func MicroserviceList(org string, userPw string, microservice string, namesOnly bool) {
	cliutils.SetWhetherUsingApiKey(userPw)
	if microservice != "" {
		microservice = "/" + microservice
	}
	if namesOnly && microservice == "" {
		// Only display the names
		var resp exchange.GetMicroservicesResponse
		cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/microservices"+microservice, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &resp)
		microservices := []string{}
		for k := range resp.Microservices {
			microservices = append(microservices, k)
		}
		jsonBytes, err := json.MarshalIndent(microservices, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn exchange microservice list' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		// Display the full resources
		//var output string
		var output exchange.GetMicroservicesResponse
		httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/microservices"+microservice, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
		if httpCode == 404 && microservice != "" {
			cliutils.Fatal(cliutils.NOT_FOUND, "microservice '%s' not found in org %s", strings.TrimPrefix(microservice, "/"), org)
		}
		jsonBytes, err := json.MarshalIndent(output, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn exchange microservice list' output: %v", err)
		}
		fmt.Println(string(jsonBytes))
	}
}

func AppendImagesFromDeploymentField(deployment *DeploymentConfig, imageList []string) []string {
	// The deployment string should include: {"services":{"cpu2wiotp":{"image":"openhorizon/example_wl_x86_cpu2wiotp:1.1.2",...}}}
	for _, s := range deployment.Services {
		if s.Image != "" {
			imageList = append(imageList, s.Image)
		}
	}
	return imageList
}

func CheckTorrentField(torrent string, index int) {
	// Verify the torrent field is the form necessary for the containers that are stored in a docker registry (because that is all we support from hzn right now)
	torrentErrorString := `currently the torrent field must either be empty or be like this to indicate the images are stored in a docker registry: {\"url\":\"\",\"signature\":\"\"}`
	if torrent == "" {
		//cliutils.Fatal(cliutils.CLI_INPUT_ERROR, torrentErrorString)
		return
	}
	var torrentMap map[string]string
	if err := json.Unmarshal([]byte(torrent), &torrentMap); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "failed to unmarshal torrent string number %d: %v", index+1, err)
	}
	if url, ok := torrentMap["url"]; !ok || url != "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, torrentErrorString)
	}
	if signature, ok := torrentMap["signature"]; !ok || signature != "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, torrentErrorString)
	}
}

// MicroservicePublish signs the MS def and puts it in the exchange
func MicroservicePublish(org, userPw, jsonFilePath, keyFilePath string) {
	cliutils.SetWhetherUsingApiKey(userPw)
	// Read in the MS metadata
	newBytes := cliutils.ReadJsonFile(jsonFilePath)
	var microFile MicroserviceFile
	err := json.Unmarshal(newBytes, &microFile)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to unmarshal json input file %s: %v", jsonFilePath, err)
	}
	if microFile.Org != "" && microFile.Org != org {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "the org specified in the input file (%s) must match the org specified on the command line (%s)", microFile.Org, org)
	}

	microFile.SignAndPublish(org, userPw, keyFilePath)
}

// Sign and publish the microservice definition. This is a function that is reusable across different hzn commands.
func (mf *MicroserviceFile) SignAndPublish(org, userPw, keyFilePath string) {
	microInput := MicroserviceInput{Label: mf.Label, Description: mf.Description, Public: mf.Public, SpecRef: mf.SpecRef, Version: mf.Version, Arch: mf.Arch, Sharable: mf.Sharable, DownloadURL: mf.DownloadURL, MatchHardware: mf.MatchHardware, UserInputs: mf.UserInputs, Workloads: make([]exchange.WorkloadDeployment, len(mf.Workloads))}

	// Loop thru the workloads array, sign the deployment strings, and copy all 3 fields to microInput
	fmt.Println("Signing microservice...")
	var imageList []string
	for i := range mf.Workloads {
		var err error
		var deployment []byte
		depConfig := ConvertToDeploymentConfig(mf.Workloads[i].Deployment)
		if mf.Workloads[i].Deployment != nil && reflect.TypeOf(mf.Workloads[i].Deployment).String() == "string" && mf.Workloads[i].DeploymentSignature != "" {
			microInput.Workloads[i].Deployment = mf.Workloads[i].Deployment.(string)
			microInput.Workloads[i].DeploymentSignature = mf.Workloads[i].DeploymentSignature
		} else if depConfig == nil {
			microInput.Workloads[i].Deployment = ""
			microInput.Workloads[i].DeploymentSignature = ""
		} else {
			cliutils.Verbose("signing deployment string %d", i+1)
			deployment, err = json.Marshal(depConfig)
			if err != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal deployment string %d: %v", i+1, err)
			}
			microInput.Workloads[i].Deployment = string(deployment)
			// We know we need to sign the deployment config, so make sure a real key file was provided.
			if keyFilePath == "" {
				cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "must specify --private-key-file so that the deployment string can be signed")
			}
			microInput.Workloads[i].DeploymentSignature, err = sign.Input(keyFilePath, deployment)
			if err != nil {
				cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "problem signing the deployment string with %s: %v", keyFilePath, err)
			}
		}

		microInput.Workloads[i].Torrent = mf.Workloads[i].Torrent

		// Gather the docker image paths to instruct the user to docker push at the end
		imageList = AppendImagesFromDeploymentField(depConfig, imageList)

		CheckTorrentField(microInput.Workloads[i].Torrent, i)
	}

	// Create or update resource in the exchange
	exchId := cliutils.FormExchangeId(microInput.SpecRef, microInput.Version, microInput.Arch)
	var output string
	httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/microservices/"+exchId, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
	if httpCode == 200 {
		// MS exists, update it
		fmt.Printf("Updating %s in the exchange...\n", exchId)
		cliutils.ExchangePutPost(http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+org+"/microservices/"+exchId, cliutils.OrgAndCreds(org, userPw), []int{201}, microInput)
	} else {
		// MS not there, create it
		fmt.Printf("Creating %s in the exchange...\n", exchId)
		cliutils.ExchangePutPost(http.MethodPost, cliutils.GetExchangeUrl(), "orgs/"+org+"/microservices", cliutils.OrgAndCreds(org, userPw), []int{201}, microInput)
	}

	// Tell them to push the images to the docker registry
	if len(imageList) > 0 {
		//todo: should we just push the docker images for them?
		fmt.Println("If you haven't already, push your docker images to the registry:")
		for _, image := range imageList {
			fmt.Printf("  docker push %s\n", image)
		}
	}
}

// MicroserviceVerify verifies the deployment strings of the specified microservice resource in the exchange.
func MicroserviceVerify(org, userPw, microservice, keyFilePath string) {
	cliutils.SetWhetherUsingApiKey(userPw)
	// Get microservice resource from exchange
	var output exchange.GetMicroservicesResponse
	httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/microservices/"+microservice, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, "microservice '%s' not found in org %s", microservice, org)
	}

	// Loop thru microservices array, checking the deployment string signature
	micro, ok := output.Microservices[org+"/"+microservice]
	if !ok {
		cliutils.Fatal(cliutils.INTERNAL_ERROR, "key '%s' not found in resources returned from exchange", org+"/"+microservice)
	}
	someInvalid := false
	for i := range micro.Workloads {
		cliutils.Verbose("verifying deployment string %d", i+1)
		verified, err := verify.Input(keyFilePath, micro.Workloads[i].DeploymentSignature, []byte(micro.Workloads[i].Deployment))
		if err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "problem verifying deployment string %d with %s: %v", i+1, keyFilePath, err)
		} else if !verified {
			fmt.Printf("Deployment string %d was not signed with the private key associated with this public key.\n", i+1)
			someInvalid = true
		}
		// else if they all turned out to be valid, we will tell them that at the end
	}

	if someInvalid {
		os.Exit(cliutils.SIGNATURE_INVALID)
	} else {
		fmt.Println("All signatures verified")
	}
}

func MicroserviceRemove(org, userPw, microservice string, force bool) {
	cliutils.SetWhetherUsingApiKey(userPw)
	if !force {
		cliutils.ConfirmRemove("Are you sure you want to remove microservice '" + org + "/" + microservice + "' from the Horizon Exchange?")
	}

	httpCode := cliutils.ExchangeDelete(cliutils.GetExchangeUrl(), "orgs/"+org+"/microservices/"+microservice, cliutils.OrgAndCreds(org, userPw), []int{204, 404})
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, "microservice '%s' not found in org %s", microservice, org)
	}
}
