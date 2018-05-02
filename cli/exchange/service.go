package exchange

import (
	"encoding/json"
	"errors"
	"fmt"
	dockerclient "github.com/fsouza/go-dockerclient"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/containermessage"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/rsapss-tool/sign"
	"github.com/open-horizon/rsapss-tool/verify"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// This can't be a const because a map literal isn't a const in go
var VALID_DEPLOYMENT_FIELDS = map[string]int8{"image": 1, "privileged": 1, "cap_add": 1, "environment": 1, "devices": 1, "binds": 1, "specific_ports": 1, "command": 1, "ports": 1}

type AbstractServiceFile interface {
	GetOrg() string
	GetURL() string
	GetVersion() string
	GetArch() string
	GetUserInputs() []exchange.UserInput
}

// This is used when reading json file the user gives us as input to create the service struct
type ServiceFile struct {
	Org                 string                       `json:"org"` // optional
	Label               string                       `json:"label"`
	Description         string                       `json:"description"`
	Public              bool                         `json:"public"`
	URL                 string                       `json:"url"`
	Version             string                       `json:"version"`
	Arch                string                       `json:"arch"`
	Sharable            string                       `json:"sharable"`
	MatchHardware       map[string]interface{}       `json:"matchHardware"`
	RequiredServices    []exchange.ServiceDependency `json:"requiredServices"`
	UserInputs          []exchange.UserInput         `json:"userInput"`
	Deployment          map[string]interface{}       `json:"deployment"`
	DeploymentSignature string                       `json:"deploymentSignature"`
	ImageStore          map[string]interface{}       `json:"imageStore"`
}

type GetServicesResponse struct {
	Services  map[string]ServiceExch `json:"services"`
	LastIndex int                    `json:"lastIndex"`
}

// This is used as the input/output to the exchange to create/read the service. The main differences are: no org, deployment is an escaped string, and optional owner and last updated
type ServiceExch struct {
	Owner               string                       `json:"owner,omitempty"`
	Label               string                       `json:"label"`
	Description         string                       `json:"description"`
	Public              bool                         `json:"public"`
	URL                 string                       `json:"url"`
	Version             string                       `json:"version"`
	Arch                string                       `json:"arch"`
	Sharable            string                       `json:"sharable"`
	MatchHardware       map[string]interface{}       `json:"matchHardware"`
	RequiredServices    []exchange.ServiceDependency `json:"requiredServices"`
	UserInputs          []exchange.UserInput         `json:"userInput"`
	Deployment          string                       `json:"deployment"`
	DeploymentSignature string                       `json:"deploymentSignature"`
	ImageStore          map[string]interface{}       `json:"imageStore"`
	LastUpdated         string                       `json:"lastUpdated,omitempty"`
}

type ServiceDockAuthExch struct {
	Registry string `json:"registry"`
	Token    string `json:"token"`
}

func (sf *ServiceFile) GetOrg() string {
	return sf.Org
}

func (sf *ServiceFile) GetURL() string {
	return sf.URL
}

func (sf *ServiceFile) GetVersion() string {
	return sf.Version
}

func (sf *ServiceFile) GetArch() string {
	return sf.Arch
}

func (sf *ServiceFile) GetUserInputs() []exchange.UserInput {
	return sf.UserInputs
}

// Returns true if the service definition userinputs define the variable.
func (sf *ServiceFile) DefinesVariable(name string) string {
	for _, ui := range sf.UserInputs {
		if ui.Name == name && ui.Type != "" {
			return ui.Type
		}
	}
	return ""
}

// Returns true if the service definition has required services.
func (sf *ServiceFile) HasDependencies() bool {
	if len(sf.RequiredServices) == 0 {
		return false
	}
	return true
}

// Return true if the service definition is a dependency in the input list of service references.
func (sf *ServiceFile) IsDependent(deps []exchange.ServiceDependency) bool {
	for _, dep := range deps {
		if sf.URL == dep.URL && sf.Org == dep.Org {
			return true
		}
	}
	return false
}

// Convert the Deployment Configuration to a full Deployment Description.
func (sf *ServiceFile) ConvertToDeploymentDescription(agreementService bool) (*DeploymentConfig, *containermessage.DeploymentDescription, error) {
	depConfig := ConvertToDeploymentConfig(sf.Deployment)
	infra := !agreementService
	return depConfig, &containermessage.DeploymentDescription{
		Services: depConfig.Services,
		ServicePattern: containermessage.Pattern{
			Shared: map[string][]string{},
		},
		Infrastructure: infra,
		Overrides:      map[string]*containermessage.Service{},
	}, nil
}

// Verify that non default user inputs are set in the input map.
func (sf *ServiceFile) RequiredVariablesAreSet(setVars map[string]interface{}) error {
	for _, ui := range sf.UserInputs {
		if ui.DefaultValue == "" && ui.Name != "" {
			if _, ok := setVars[ui.Name]; !ok {
				return errors.New(fmt.Sprintf("user input %v has no default value and is not set", ui.Name))
			}
		}
	}
	return nil
}

func ServiceList(org, userPw, service string, namesOnly bool) {
	cliutils.SetWhetherUsingApiKey(userPw)
	org, service = cliutils.TrimOrg(org, service)
	if namesOnly && service == "" {
		// Only display the names
		var resp GetServicesResponse
		cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/services"+cliutils.AddSlash(service), cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &resp)
		services := []string{}

		for k := range resp.Services {
			services = append(services, k)
		}
		jsonBytes, err := json.MarshalIndent(services, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn exchange service list' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		// Display the full resources
		var services GetServicesResponse

		httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/services"+cliutils.AddSlash(service), cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &services)
		if httpCode == 404 && service != "" {
			cliutils.Fatal(cliutils.NOT_FOUND, "service '%s' not found in org %s", service, org)
		}
		jsonBytes, err := json.MarshalIndent(services.Services, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn exchange service list' output: %v", err)
		}
		fmt.Println(string(jsonBytes))
	}
}

// ServicePublish signs the MS def and puts it in the exchange
func ServicePublish(org, userPw, jsonFilePath, keyFilePath, pubKeyFilePath string, dontTouchImage bool, registryTokens []string) {
	cliutils.SetWhetherUsingApiKey(userPw)
	// Read in the service metadata
	newBytes := cliutils.ReadJsonFile(jsonFilePath)
	var svcFile ServiceFile
	err := json.Unmarshal(newBytes, &svcFile)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to unmarshal json input file %s: %v", jsonFilePath, err)
	}
	if svcFile.Org != "" && svcFile.Org != org {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "the org specified in the input file (%s) must match the org specified on the command line (%s)", svcFile.Org, org)
	}
	svcFile.SignAndPublish(org, userPw, keyFilePath, pubKeyFilePath, dontTouchImage, registryTokens)
}

// CheckDeploymentService verifies it has the required 'image' key, and checks for keys we don't recognize.
// For now it only prints a warning for unrecognized keys, in case we recently added a key to anax and haven't updated hzn yet.
func CheckDeploymentService(svcName string, depSvc map[string]interface{}) {
	if _, ok := depSvc["image"]; !ok {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "service '%s' defined under 'deployment.services' does not have mandatory 'image' field", svcName)
	}

	// Check the rest of the keys for unrecognized ones
	for k := range depSvc {
		if _, ok := VALID_DEPLOYMENT_FIELDS[k]; !ok {
			cliutils.Warning("service '%s' defined under 'deployment.services' has unrecognized field '%s'. See https://github.com/open-horizon/anax/blob/master/doc/deployment_string.md", svcName, k)
		}
	}
}

// SignImagesFromDeploymentMap finds the images in this deployment structure (if any) and appends them to the imageList
func SignImagesFromDeploymentMap(deployment map[string]interface{}, dontTouchImage bool) (imageList []string) {
	// The deployment string should include: {"services":{"cpu2wiotp":{"image":"openhorizon/example_wl_x86_cpu2wiotp:1.1.2",...}}}
	// Since we have to parse the deployment structure anyway, we do some validity checking while we are at it
	// Note: in the code below we are exploiting the golang map feature that it returns the zero value when a key does not exist in the map.
	if len(deployment) == 0 {
		return imageList // an empty deployment structure is valid
	}
	var client *dockerclient.Client
	switch services := deployment["services"].(type) {
	case map[string]interface{}:
		for k, svc := range services {
			switch s := svc.(type) {
			case map[string]interface{}:
				CheckDeploymentService(k, s)
				switch image := s["image"].(type) {
				case string:
					domain, path, tag, digest := cutil.ParseDockerImagePath(image)
					cliutils.Verbose("%s parsed into: domain=%s, path=%s, tag=%s", image, domain, path, tag)
					if path == "" {
						fmt.Printf("Warning: could not parse image path '%v'. Not pushing it to a docker registry, just including it in the 'deployment' field as-is.\n", image)
					} else if digest == "" {
						// This image has a tag, or default tag
						if dontTouchImage {
							imageList = append(imageList, image)
						} else {
							// Push it, get the repo digest, and modify the imagePath to use the digest
							if client == nil {
								client = cliutils.NewDockerClient()
							}
							digest := cliutils.PushDockerImage(client, domain, path, tag) // this will error out if the push fails or can't get the digest
							if domain != "" {
								domain = domain + "/"
							}
							newImage := domain + path + "@" + digest
							fmt.Printf("Using '%s' in 'deployment' field instead of '%s'\n", newImage, image)
							s["image"] = newImage
						}
					}
				}
			default:
				cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "each service defined under 'deployment.services' must be a json object (with strings as the keys)")
			}
		}
	default:
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "the 'deployment' field must contain the 'services' field, whose value must be a json object (with strings as the keys)")
	}
	return
}

// Sign and publish the service definition. This is a function that is reusable across different hzn commands.
func (sf *ServiceFile) SignAndPublish(org, userPw, keyFilePath, pubKeyFilePath string, dontTouchImage bool, registryTokens []string) {
	svcInput := ServiceExch{Label: sf.Label, Description: sf.Description, Public: sf.Public, URL: sf.URL, Version: sf.Version, Arch: sf.Arch, Sharable: sf.Sharable, MatchHardware: sf.MatchHardware, RequiredServices: sf.RequiredServices, UserInputs: sf.UserInputs, ImageStore: sf.ImageStore}

	// Go thru the docker image paths to push/get sha256 tag and/or gather list of images that user needs to push
	var imageList []string
	if storeType, ok := svcInput.ImageStore["storeType"]; !ok || storeType != "imageServer" {
		imageList = SignImagesFromDeploymentMap(sf.Deployment, dontTouchImage)
	}
	// else the images are in the deprecated horizon image svr, don't do anything with them

	// Marshal and sign the deployment string
	fmt.Println("Signing service...")
	//cliutils.Verbose("signing deployment string %d", i+1)
	// Convert the deployment field from map[string]interface{} to []byte (i think treating it as type DeploymentConfig is too inflexible for future additions)
	deployment, err := json.Marshal(sf.Deployment)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal deployment string: %v", err)
	}
	svcInput.Deployment = string(deployment)
	// We know we need to sign the deployment config, so make sure a real key file was provided.
	if keyFilePath == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "must specify --private-key-file so that the deployment string can be signed")
	}
	svcInput.DeploymentSignature, err = sign.Input(keyFilePath, deployment)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "problem signing deployment string with %s: %v", keyFilePath, err)
	}

	//todo: when we support something in the ImageStore map, process it here

	// Create or update resource in the exchange
	exchId := cliutils.FormExchangeId(svcInput.URL, svcInput.Version, svcInput.Arch)
	var output string
	httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/services/"+exchId, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
	if httpCode == 200 {
		// Service exists, update it
		fmt.Printf("Updating %s in the exchange...\n", exchId)
		cliutils.ExchangePutPost(http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+org+"/services/"+exchId, cliutils.OrgAndCreds(org, userPw), []int{201}, svcInput)
	} else {
		// Service not there, create it
		fmt.Printf("Creating %s in the exchange...\n", exchId)
		cliutils.ExchangePutPost(http.MethodPost, cliutils.GetExchangeUrl(), "orgs/"+org+"/services", cliutils.OrgAndCreds(org, userPw), []int{201}, svcInput)
	}

	// Store the public key in the exchange, if they gave it to us
	if pubKeyFilePath != "" {
		// Note: the CLI framesvc already verified the file exists
		bodyBytes := cliutils.ReadFile(pubKeyFilePath)
		baseName := filepath.Base(pubKeyFilePath)
		fmt.Printf("Storing %s with the service in the exchange...\n", baseName)
		cliutils.ExchangePutPost(http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+org+"/services/"+exchId+"/keys/"+baseName, cliutils.OrgAndCreds(org, userPw), []int{201}, bodyBytes)
	}

	// Store registry auth tokens in the exchange, if they gave us some
	for _, regTok := range registryTokens {
		parts := strings.SplitN(regTok, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			fmt.Printf("Error: registry-token value of '%s' is not in the required format: registry:token. Not storing that in the Horizon exchange.\n", regTok)
			continue
		}
		fmt.Printf("Storing %s with the service in the exchange...\n", regTok)
		regTokExch := ServiceDockAuthExch{Registry: parts[0], Token: parts[1]}
		cliutils.ExchangePutPost(http.MethodPost, cliutils.GetExchangeUrl(), "orgs/"+org+"/services/"+exchId+"/dockauths", cliutils.OrgAndCreds(org, userPw), []int{201}, regTokExch)
	}

	// Tell the user to push the images to the docker registry
	if len(imageList) > 0 {
		//todo: should we just push the docker images for them?
		fmt.Println("If you haven't already, push your docker images to the registry:")
		for _, image := range imageList {
			fmt.Printf("  docker push %s\n", image)
		}
	}
	return
}

// ServiceVerify verifies the deployment strings of the specified service resource in the exchange.
func ServiceVerify(org, userPw, service, keyFilePath string) {
	cliutils.SetWhetherUsingApiKey(userPw)
	org, service = cliutils.TrimOrg(org, service)
	// Get service resource from exchange
	var output GetServicesResponse
	httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/services/"+service, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, "service '%s' not found in org %s", service, org)
	}

	// Loop thru services array, checking the deployment string signature
	svc, ok := output.Services[org+"/"+service]
	if !ok {
		cliutils.Fatal(cliutils.INTERNAL_ERROR, "key '%s' not found in resources returned from exchange", org+"/"+service)
	}
	someInvalid := false
	verified, err := verify.Input(keyFilePath, svc.DeploymentSignature, []byte(svc.Deployment))
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "problem verifying deployment string with %s: %v", keyFilePath, err)
	} else if !verified {
		fmt.Println("Deployment string was not signed with the private key associated with this public key.")
		someInvalid = true
	}
	// else if they all turned out to be valid, we will tell them that at the end

	if someInvalid {
		os.Exit(cliutils.SIGNATURE_INVALID)
	} else {
		fmt.Println("All signatures verified")
	}
}

func ServiceRemove(org, userPw, service string, force bool) {
	cliutils.SetWhetherUsingApiKey(userPw)
	org, service = cliutils.TrimOrg(org, service)
	if !force {
		cliutils.ConfirmRemove("Are you sure you want to remove service '" + org + "/" + service + "' from the Horizon Exchange?")
	}

	httpCode := cliutils.ExchangeDelete(cliutils.GetExchangeUrl(), "orgs/"+org+"/services/"+service, cliutils.OrgAndCreds(org, userPw), []int{204, 404})
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, "service '%s' not found in org %s", service, org)
	}
}

func ServiceListKey(org, userPw, service, keyName string) {
	cliutils.SetWhetherUsingApiKey(userPw)
	org, service = cliutils.TrimOrg(org, service)
	if keyName == "" {
		// Only display the names
		var output string
		httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/services/"+service+"/keys", cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
		if httpCode == 404 {
			cliutils.Fatal(cliutils.NOT_FOUND, "keys not found", keyName)
		}
		fmt.Printf("%s\n", output)
	} else {
		// Display the content of the key
		var output []byte
		httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/services/"+service+"/keys/"+keyName, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
		if httpCode == 404 {
			cliutils.Fatal(cliutils.NOT_FOUND, "key '%s' not found", keyName)
		}
		fmt.Printf("%s", string(output))
	}
}

func ServiceRemoveKey(org, userPw, service, keyName string) {
	cliutils.SetWhetherUsingApiKey(userPw)
	org, service = cliutils.TrimOrg(org, service)
	httpCode := cliutils.ExchangeDelete(cliutils.GetExchangeUrl(), "orgs/"+org+"/services/"+service+"/keys/"+keyName, cliutils.OrgAndCreds(org, userPw), []int{204, 404})
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, "key '%s' not found", keyName)
	}
}

func ServiceListAuth(org, userPw, service string, authId uint) {
	cliutils.SetWhetherUsingApiKey(userPw)
	org, service = cliutils.TrimOrg(org, service)
	var authIdStr string
	if authId != 0 {
		authIdStr = "/" + strconv.Itoa(int(authId))
	}
	var output string
	httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/services/"+service+"/dockauths"+authIdStr, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
	if httpCode == 404 {
		if authId != 0 {
			cliutils.Fatal(cliutils.NOT_FOUND, "docker auth %d not found", authId)
		} else {
			cliutils.Fatal(cliutils.NOT_FOUND, "docker auths not found")
		}
	}
	fmt.Printf("%s\n", output)
}

func ServiceRemoveAuth(org, userPw, service string, authId uint) {
	cliutils.SetWhetherUsingApiKey(userPw)
	org, service = cliutils.TrimOrg(org, service)
	authIdStr := strconv.Itoa(int(authId))
	httpCode := cliutils.ExchangeDelete(cliutils.GetExchangeUrl(), "orgs/"+org+"/services/"+service+"/dockauths/"+authIdStr, cliutils.OrgAndCreds(org, userPw), []int{204, 404})
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, "docker auth %d not found", authId)
	}
}
