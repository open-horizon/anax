package exchange

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cli/plugin_registry"
	"github.com/open-horizon/anax/cli/policy"
	"github.com/open-horizon/anax/containermessage"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	_ "github.com/open-horizon/anax/externalpolicy/text_language"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/rsapss-tool/verify"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

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
	Documentation       string                       `json:"documentation"`
	URL                 string                       `json:"url"`
	Version             string                       `json:"version"`
	Arch                string                       `json:"arch"`
	Sharable            string                       `json:"sharable"`
	MatchHardware       map[string]interface{}       `json:"matchHardware,omitempty"`
	RequiredServices    []exchange.ServiceDependency `json:"requiredServices"`
	UserInputs          []exchange.UserInput         `json:"userInput"`
	Deployment          interface{}                  `json:"deployment"` // interface{} because pre-signed services can be stringified json
	DeploymentSignature string                       `json:"deploymentSignature,omitempty"`
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
	Documentation       string                       `json:"documentation"`
	URL                 string                       `json:"url"`
	Version             string                       `json:"version"`
	Arch                string                       `json:"arch"`
	Sharable            string                       `json:"sharable"`
	MatchHardware       map[string]interface{}       `json:"matchHardware"`
	RequiredServices    []exchange.ServiceDependency `json:"requiredServices"`
	UserInputs          []exchange.UserInput         `json:"userInput"`
	Deployment          string                       `json:"deployment"`
	DeploymentSignature string                       `json:"deploymentSignature"`
	LastUpdated         string                       `json:"lastUpdated,omitempty"`
}

type ServiceDockAuthExch struct {
	Registry string `json:"registry"`
	UserName string `json:"username"`
	Token    string `json:"token"`
}

type ServicePolicyFile struct {
	Properties  externalpolicy.PropertyList         `json:"properties"`
	Constraints externalpolicy.ConstraintExpression `json:"constraints"`
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
				return errors.New(i18n.GetMessagePrinter().Sprintf("user input %v has no default value and is not set", ui.Name))
			}
		}
	}
	return nil
}

func (sf *ServiceFile) SupportVersionRange() {
	for ix, sdep := range sf.RequiredServices {
		if sdep.VersionRange == "" {
			sf.RequiredServices[ix].VersionRange = sf.RequiredServices[ix].Version
		}
	}
}

// List the the service resources for the given org.
// The userPw can be the userId:password auth or the nodeId:token auth.
func ServiceList(credOrg, userPw, service string, namesOnly bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	cliutils.SetWhetherUsingApiKey(userPw)
	var svcOrg string
	svcOrg, service = cliutils.TrimOrg(credOrg, service)
	if service == "*" {
		service = ""
	}
	if namesOnly && service == "" {
		// Only display the names
		var resp GetServicesResponse
		cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+svcOrg+"/services"+cliutils.AddSlash(service), cliutils.OrgAndCreds(credOrg, userPw), []int{200, 404}, &resp)
		services := []string{}

		for k := range resp.Services {
			services = append(services, k)
		}
		jsonBytes, err := json.MarshalIndent(services, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn exchange service list' output: %v", err))
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		// Display the full resources
		var services GetServicesResponse

		httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+svcOrg+"/services"+cliutils.AddSlash(service), cliutils.OrgAndCreds(credOrg, userPw), []int{200, 404}, &services)
		if httpCode == 404 && service != "" {
			cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("service '%s' not found in org %s", service, svcOrg))
		}
		jsonBytes, err := json.MarshalIndent(services.Services, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn exchange service list' output: %v", err))
		}
		fmt.Println(string(jsonBytes))
	}
}

// ServicePublish signs the MS def and puts it in the exchange
func ServicePublish(org, userPw, jsonFilePath, keyFilePath, pubKeyFilePath string, dontTouchImage bool, pullImage bool, registryTokens []string, overwrite bool, servicePolicyFilePath string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if dontTouchImage && pullImage {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Flags -I and -P are mutually exclusive."))
	}
	cliutils.SetWhetherUsingApiKey(userPw)

	// Read in the service metadata
	newBytes := cliconfig.ReadJsonFileWithLocalConfig(jsonFilePath)
	var svcFile ServiceFile
	err := json.Unmarshal(newBytes, &svcFile)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal json input file %s: %v", jsonFilePath, err))
	}
	if svcFile.Org != "" && svcFile.Org != org {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("the org specified in the input file (%s) must match the org specified on the command line (%s)", svcFile.Org, org))
	}

	// Compensate for old service definition files
	svcFile.SupportVersionRange()

	svcFile.SignAndPublish(org, userPw, jsonFilePath, keyFilePath, pubKeyFilePath, dontTouchImage, pullImage, registryTokens, !overwrite)

	// create service policy if servicePolicyFilePath is defined
	if servicePolicyFilePath != "" {
		serviceAddPolicyService := fmt.Sprintf("%s/%s_%s_%s", svcFile.Org, svcFile.URL, svcFile.Version, svcFile.Arch) //svcFile.URL + "_" + svcFile.Version + "_" +
		msgPrinter.Printf("Adding service policy for service: %v", serviceAddPolicyService)
		msgPrinter.Println()
		ServiceAddPolicy(org, userPw, serviceAddPolicyService, servicePolicyFilePath)
		msgPrinter.Printf("Service policy added for service: %v", serviceAddPolicyService)
		msgPrinter.Println()
	}
}

// Sign and publish the service definition. This is a function that is reusable across different hzn commands.
func (sf *ServiceFile) SignAndPublish(org, userPw, jsonFilePath, keyFilePath, pubKeyFilePath string, dontTouchImage bool, pullImage bool, registryTokens []string, promptForOverwrite bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	svcInput := ServiceExch{Label: sf.Label, Description: sf.Description, Public: sf.Public, Documentation: sf.Documentation, URL: sf.URL, Version: sf.Version, Arch: sf.Arch, Sharable: sf.Sharable, MatchHardware: sf.MatchHardware, RequiredServices: sf.RequiredServices, UserInputs: sf.UserInputs}

	// The deployment field can be json object (map), string (for pre-signed), or nil
	switch dep := sf.Deployment.(type) {
	case nil:
		svcInput.Deployment = ""
		if sf.DeploymentSignature != "" {
			cliutils.Warning(msgPrinter.Sprintf("the 'deploymentSignature' field is non-blank, but being ignored, because the 'deployment' field is null"))
		}
		svcInput.DeploymentSignature = ""

	case map[string]interface{}:
		// We know we need to sign the deployment config, so make sure a real key file was provided.
		keyFilePath, pubKeyFilePath = cliutils.GetSigningKeys(keyFilePath, pubKeyFilePath)

		// Construct and sign the deployment string.
		msgPrinter.Printf("Signing service...")
		msgPrinter.Println()

		// Setup the Plugin context with variables that might be needed by 1 or more of the plugins.
		ctx := plugin_registry.NewPluginContext()
		ctx.Add("currentDir", filepath.Dir(jsonFilePath))
		ctx.Add("dontTouchImage", dontTouchImage)
		ctx.Add("pullImage", pullImage)

		// Allow the right plugin to sign the deployment configuration.
		depStr, sig, err := plugin_registry.DeploymentConfigPlugins.SignByOne(dep, keyFilePath, ctx)
		if err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("unable to sign deployment config: %v", err))
		}

		// Update the exchange service input object with the deployment string and sig, so that the service
		// can be created in the exchange.
		svcInput.Deployment = depStr
		svcInput.DeploymentSignature = sig

	case string:
		// Means this service is pre-signed
		if sf.Deployment != "" && sf.DeploymentSignature == "" {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("the 'deployment' field is a non-empty string, which implies this service is pre-signed, but the 'deploymentSignature' field is empty"))
		}
		svcInput.Deployment = dep
		svcInput.DeploymentSignature = sf.DeploymentSignature

	default:
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("'deployment' field is invalid type. It must be either a json object or a string (for pre-signed)"))
	}

	// Create or update resource in the exchange
	exchId := cliutils.FormExchangeIdForService(svcInput.URL, svcInput.Version, svcInput.Arch)
	var output string
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+org+"/services/"+exchId, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
	if httpCode == 200 {
		// check if the service exists with the same version, ask user if -O is not specified.
		if promptForOverwrite {
			cliutils.ConfirmRemove(msgPrinter.Sprintf("Service %v/%v exists in the exchange, do you want to overwrite it?", org, exchId))
		}
		// Service exists, update it
		msgPrinter.Printf("Updating %s in the exchange...", exchId)
		msgPrinter.Println()
		cliutils.ExchangePutPost("Exchange", http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+org+"/services/"+exchId, cliutils.OrgAndCreds(org, userPw), []int{201}, svcInput)
	} else {
		// Service not there, create it
		msgPrinter.Printf("Creating %s in the exchange...", exchId)
		msgPrinter.Println()
		cliutils.ExchangePutPost("Exchange", http.MethodPost, cliutils.GetExchangeUrl(), "orgs/"+org+"/services", cliutils.OrgAndCreds(org, userPw), []int{201}, svcInput)
	}

	// Store the public key in the exchange, if they gave it to us
	if pubKeyFilePath != "" {
		bodyBytes := cliutils.ReadFile(pubKeyFilePath)
		baseName := filepath.Base(pubKeyFilePath)
		msgPrinter.Printf("Storing %s with the service in the exchange...", baseName)
		msgPrinter.Println()
		cliutils.ExchangePutPost("Exchange", http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+org+"/services/"+exchId+"/keys/"+baseName, cliutils.OrgAndCreds(org, userPw), []int{201}, bodyBytes)
	}

	// Store registry auth tokens in the exchange, if they gave us some
	for _, regTok := range registryTokens {
		parts := strings.SplitN(regTok, ":", 3)
		regstry := ""
		username := ""
		token := ""
		if len(parts) == 3 {
			regstry = parts[0]
			username = parts[1]
			token = parts[2]
		}

		if parts[0] == "" || len(parts) < 3 || parts[2] == "" {
			msgPrinter.Printf("Error: registry-token value of '%s' is not in the required format: registry:user:token. Not storing that in the Horizon exchange.", regTok)
			msgPrinter.Println()
			continue
		}
		msgPrinter.Printf("Storing %s with the service in the exchange...", regTok)
		msgPrinter.Println()
		regTokExch := ServiceDockAuthExch{Registry: regstry, UserName: username, Token: token}
		cliutils.ExchangePutPost("Exchange", http.MethodPost, cliutils.GetExchangeUrl(), "orgs/"+org+"/services/"+exchId+"/dockauths", cliutils.OrgAndCreds(org, userPw), []int{201}, regTokExch)
	}

	// If necessary, tell the user to push the container images to the docker registry. Get the list of images they need to manually push
	// from the appropriate deployment config plugin.
	//
	// We will NOT tell the user to manually push images if the publish command has already pushed the images. By default, the act
	// of publishing a service will also cause the docker images used by the service to be pushed to a docker repo. The dontTouchImage flag tells
	// the publish command to skip pushing the images.
	if dontTouchImage {
		if imageList, err := plugin_registry.DeploymentConfigPlugins.GetContainerImages(sf.Deployment); err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("unable to get container images from deployment: %v", err))
		} else if len(imageList) > 0 {
			msgPrinter.Printf("If you haven't already, push your docker images to the registry:")
			msgPrinter.Println()
			for _, image := range imageList {
				fmt.Printf("  docker push %s\n", image)
			}
		}
	}
	return
}

// ServiceVerify verifies the deployment strings of the specified service resource in the exchange.
// The userPw can be the userId:password auth or the nodeId:token auth.
func ServiceVerify(org, userPw, service, keyFilePath string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	cliutils.SetWhetherUsingApiKey(userPw)
	var svcorg string
	svcorg, service = cliutils.TrimOrg(org, service)
	// Get service resource from exchange
	var output GetServicesResponse
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+svcorg+"/services/"+service, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("service '%s' not found in org %s", service, svcorg))
	}

	// Loop thru services array, checking the deployment string signature
	svc, ok := output.Services[svcorg+"/"+service]
	if !ok {
		cliutils.Fatal(cliutils.INTERNAL_ERROR, msgPrinter.Sprintf("key '%s' not found in resources returned from exchange", svcorg+"/"+service))
	}
	someInvalid := false

	//take default key if empty, make sure the key exists
	keyFilePath = cliutils.VerifySigningKeyInput(keyFilePath, true)

	verified, err := verify.Input(keyFilePath, svc.DeploymentSignature, []byte(svc.Deployment))
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("error verifying deployment string with %s: %v", keyFilePath, err))
	} else if !verified {
		msgPrinter.Printf("Deployment string was not signed with the private key associated with this public key %v.", keyFilePath)
		msgPrinter.Println()
		someInvalid = true
	}
	// else if they all turned out to be valid, we will tell them that at the end

	if someInvalid {
		os.Exit(cliutils.SIGNATURE_INVALID)
	} else {
		msgPrinter.Printf("All signatures verified")
		msgPrinter.Println()
	}
}

func ServiceRemove(org, userPw, service string, force bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	cliutils.SetWhetherUsingApiKey(userPw)
	var svcorg string
	svcorg, service = cliutils.TrimOrg(org, service)
	if !force {
		cliutils.ConfirmRemove(msgPrinter.Sprintf("Are you sure you want to remove service %v/%v from the Horizon Exchange?", svcorg, service))
	}

	httpCode := cliutils.ExchangeDelete("Exchange", cliutils.GetExchangeUrl(), "orgs/"+svcorg+"/services/"+service, cliutils.OrgAndCreds(org, userPw), []int{204, 404})
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("service '%s' not found in org %s", service, svcorg))
	}
}

// List the public keys for a service that can be used to verify the deployment signature for the service
// The userPw can be the userId:password auth or the nodeId:token auth.
func ServiceListKey(org, userPw, service, keyName string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	cliutils.SetWhetherUsingApiKey(userPw)
	var svcorg string
	svcorg, service = cliutils.TrimOrg(org, service)
	if keyName == "" {
		// Only display the names
		var output string
		httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+svcorg+"/services/"+service+"/keys", cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
		if httpCode == 404 {
			cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("keys not found"))
		}
		fmt.Printf("%s\n", output)
	} else {
		// Display the content of the key
		var output []byte
		httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+svcorg+"/services/"+service+"/keys/"+keyName, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
		if httpCode == 404 {
			cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("key '%s' not found", keyName))
		}
		fmt.Printf("%s", string(output))
	}
}

func ServiceRemoveKey(org, userPw, service, keyName string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	cliutils.SetWhetherUsingApiKey(userPw)
	var svcorg string
	svcorg, service = cliutils.TrimOrg(org, service)
	httpCode := cliutils.ExchangeDelete("Exchange", cliutils.GetExchangeUrl(), "orgs/"+svcorg+"/services/"+service+"/keys/"+keyName, cliutils.OrgAndCreds(org, userPw), []int{204, 404})
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("key '%s' not found", keyName))
	}
}

// List the docker auth that can be used to get the images for the service
// The userPw can be the userId:password auth or the nodeId:token auth.
func ServiceListAuth(org, userPw, service string, authId uint) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	cliutils.SetWhetherUsingApiKey(userPw)
	var svcorg string
	svcorg, service = cliutils.TrimOrg(org, service)
	var authIdStr string
	if authId != 0 {
		authIdStr = "/" + strconv.Itoa(int(authId))
	}
	var output string
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+svcorg+"/services/"+service+"/dockauths"+authIdStr, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
	if httpCode == 404 {
		if authId != 0 {
			cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("docker auth %d not found", authId))
		} else {
			cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("docker auths not found"))
		}
	}
	fmt.Printf("%s\n", output)
}

func ServiceRemoveAuth(org, userPw, service string, authId uint) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	cliutils.SetWhetherUsingApiKey(userPw)
	var svcorg string
	svcorg, service = cliutils.TrimOrg(org, service)
	authIdStr := strconv.Itoa(int(authId))
	httpCode := cliutils.ExchangeDelete("Exchange", cliutils.GetExchangeUrl(), "orgs/"+svcorg+"/services/"+service+"/dockauths/"+authIdStr, cliutils.OrgAndCreds(org, userPw), []int{204, 404})
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("docker auth %d not found", authId))
	}
}

//ServiceListPolicy lists the policy for the service in the Horizon Exchange
func ServiceListPolicy(org string, credToUse string, service string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	cliutils.SetWhetherUsingApiKey(credToUse)
	var svcorg string
	svcorg, service = cliutils.TrimOrg(org, service)

	// Check that the service exists
	var services ServiceExch
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+svcorg+"/services"+cliutils.AddSlash(service), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &services)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("service '%v/%v' not found.", svcorg, service))
	}
	var policy exchange.ExchangePolicy
	cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+svcorg+"/services"+cliutils.AddSlash(service)+"/policy", cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &policy)

	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", cliutils.JSON_INDENT)
	err := enc.Encode(policy.GetExternalPolicy())
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn exchange service listpolicy' output: %v", err))
	}
	fmt.Println(string(buf.String()))
}

//ServiceAddPolicy adds a policy or replaces an existing policy for the service in the Horizon Exchange
func ServiceAddPolicy(org string, credToUse string, service string, jsonFilePath string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	cliutils.SetWhetherUsingApiKey(credToUse)
	var svcorg string
	svcorg, service = cliutils.TrimOrg(org, service)
	fullServiceName := fmt.Sprintf(svcorg + "/" + service)

	// Read in the policy metadata
	newBytes := cliconfig.ReadJsonFileWithLocalConfig(jsonFilePath)
	var policyFile externalpolicy.ExternalPolicy
	err := json.Unmarshal(newBytes, &policyFile)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal json input file %s: %v", jsonFilePath, err))
	}

	//Check the policy file format
	err = policyFile.Validate()
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Incorrect policy format in file %s: %v", jsonFilePath, err))
	}

	// Check that the service exists
	var services GetServicesResponse
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+svcorg+"/services"+cliutils.AddSlash(service), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &services)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("service '%v/%v' not found.", svcorg, service))
	}
	serviceFromExchange := services.Services[fullServiceName]

	serviceName := serviceFromExchange.URL
	serviceVersion := serviceFromExchange.Version
	serviceArch := serviceFromExchange.Arch

	// Set default built in properties before publishing to the exchange
	msgPrinter.Printf("Adding built-in property values...")
	msgPrinter.Println()
	msgPrinter.Printf("The following property value will be overriden: service.url, service.name, service.org, service.version, service.arch")
	msgPrinter.Println()

	properties := policyFile.Properties
	properties.Add_Property(externalpolicy.Property_Factory(externalpolicy.PROP_SVC_URL, serviceName), true)
	properties.Add_Property(externalpolicy.Property_Factory(externalpolicy.PROP_SVC_NAME, serviceName), true)
	properties.Add_Property(externalpolicy.Property_Factory(externalpolicy.PROP_SVC_ORG, svcorg), true)
	properties.Add_Property(externalpolicy.Property_Factory(externalpolicy.PROP_SVC_VERSION, serviceVersion), true)
	properties.Add_Property(externalpolicy.Property_Factory(externalpolicy.PROP_SVC_ARCH, serviceArch), true)

	policyFile.Properties = properties

	//Check the policy file format again
	err = policyFile.Validate()
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Incorrect policy format in file %s: %v", jsonFilePath, err))
	}

	// add/replace service policy
	msgPrinter.Printf("Updating Service policy  and re-evaluating all agreements based on this Service policy. Existing agreements might be cancelled and re-negotiated.")
	msgPrinter.Println()
	cliutils.ExchangePutPost("Exchange", http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+svcorg+"/services/"+service+"/policy", cliutils.OrgAndCreds(org, credToUse), []int{201}, policyFile)

	msgPrinter.Printf("Service policy updated.")
	msgPrinter.Println()
}

//ServiceRemovePolicy removes the service policy in the exchange
func ServiceRemovePolicy(org string, credToUse string, service string, force bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	cliutils.SetWhetherUsingApiKey(credToUse)
	var svcorg string
	svcorg, service = cliutils.TrimOrg(org, service)

	//confirm removal with user
	if !force {
		cliutils.ConfirmRemove(msgPrinter.Sprintf("Are you sure you want to remove service policy for %v/%v from the Horizon Exchange?", svcorg, service))
	}

	// Check that the service exists
	var services ServiceExch
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+svcorg+"/services"+cliutils.AddSlash(service), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &services)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("service '%v/%v' not found.", svcorg, service))
	}

	//remove service policy
	msgPrinter.Printf("Removing Service policy and re-evaluating all agreements based on just the built-in node policy. Existing agreements might be cancelled and re-negotiated.")
	msgPrinter.Println()
	cliutils.ExchangeDelete("Exchange", cliutils.GetExchangeUrl(), "orgs/"+svcorg+"/services/"+service+"/policy", cliutils.OrgAndCreds(org, credToUse), []int{204, 404})
	msgPrinter.Printf("Service policy removed.")
	msgPrinter.Println()
}

// Display an empty service policy template as an object.
func ServiceNewPolicy() {
	policy.New()
}
