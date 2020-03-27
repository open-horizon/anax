package exchange

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cli/plugin_registry"
	"github.com/open-horizon/anax/cli/policy"
	"github.com/open-horizon/anax/common"
	"github.com/open-horizon/anax/cutil"
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

type ServiceDockAuthExch struct {
	Registry string `json:"registry"`
	UserName string `json:"username"`
	Token    string `json:"token"`
}

type ServicePolicyFile struct {
	Properties  externalpolicy.PropertyList         `json:"properties"`
	Constraints externalpolicy.ConstraintExpression `json:"constraints"`
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
		var resp exchange.GetServicesResponse
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
		var services exchange.GetServicesResponse

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
	var svcFile common.ServiceFile
	err := json.Unmarshal(newBytes, &svcFile)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal json input file %s: %v", jsonFilePath, err))
	}
	if svcFile.Org != "" && svcFile.Org != org {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("the org specified in the input file (%s) must match the org specified on the command line (%s)", svcFile.Org, org))
	}

	// Compensate for old service definition files
	svcFile.SupportVersionRange()

	// validate the service attributes
	ec := cliutils.GetUserExchangeContext(org, userPw)
	if err := common.ValidateService(exchange.GetHTTPServiceDefResolverHandler(ec), &svcFile, msgPrinter); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Error validating the input service: %v", err))
	}

	SignAndPublish(&svcFile, org, userPw, jsonFilePath, keyFilePath, pubKeyFilePath, dontTouchImage, pullImage, registryTokens, !overwrite)

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
func SignAndPublish(sf *common.ServiceFile, org, userPw, jsonFilePath, keyFilePath, pubKeyFilePath string, dontTouchImage bool, pullImage bool, registryTokens []string, promptForOverwrite bool) {

	//check for ExchangeUrl early on
	var exchUrl = cliutils.GetExchangeUrl()

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	svcInput := exchange.ServiceDefinition{Label: sf.Label, Description: sf.Description, Public: sf.Public, Documentation: sf.Documentation, URL: sf.URL, Version: sf.Version, Arch: sf.Arch, Sharable: sf.Sharable, MatchHardware: sf.MatchHardware, RequiredServices: sf.RequiredServices, UserInputs: sf.UserInputs}

	baseDir := filepath.Dir(jsonFilePath)
	svcInput.Deployment, svcInput.DeploymentSignature = SignDeployment(sf.Deployment, sf.DeploymentSignature, baseDir, false, keyFilePath, pubKeyFilePath, dontTouchImage, pullImage)
	svcInput.ClusterDeployment, svcInput.ClusterDeploymentSignature = SignDeployment(sf.ClusterDeployment, sf.ClusterDeploymentSignature, baseDir, true, keyFilePath, pubKeyFilePath, dontTouchImage, pullImage)

	// Create or update resource in the exchange
	exchId := cutil.FormExchangeIdForService(svcInput.URL, svcInput.Version, svcInput.Arch)
	var output string
	httpCode := cliutils.ExchangeGet("Exchange", exchUrl, "orgs/"+org+"/services/"+exchId, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
	if httpCode == 200 {
		// check if the service exists with the same version, ask user if -O is not specified.
		if promptForOverwrite {
			cliutils.ConfirmRemove(msgPrinter.Sprintf("Service %v/%v exists in the exchange, do you want to overwrite it?", org, exchId))
		}
		// Service exists, update it
		msgPrinter.Printf("Updating %s in the exchange...", exchId)
		msgPrinter.Println()
		cliutils.ExchangePutPost("Exchange", http.MethodPut, exchUrl, "orgs/"+org+"/services/"+exchId, cliutils.OrgAndCreds(org, userPw), []int{201}, svcInput)
	} else {
		// Service not there, create it
		msgPrinter.Printf("Creating %s in the exchange...", exchId)
		msgPrinter.Println()
		cliutils.ExchangePutPost("Exchange", http.MethodPost, exchUrl, "orgs/"+org+"/services", cliutils.OrgAndCreds(org, userPw), []int{201}, svcInput)
	}

	// Store the public key in the exchange, if they gave it to us
	if pubKeyFilePath != "" {
		bodyBytes := cliutils.ReadFile(pubKeyFilePath)
		baseName := filepath.Base(pubKeyFilePath)
		msgPrinter.Printf("Storing %s with the service in the exchange...", baseName)
		msgPrinter.Println()
		cliutils.ExchangePutPost("Exchange", http.MethodPut, exchUrl, "orgs/"+org+"/services/"+exchId+"/keys/"+baseName, cliutils.OrgAndCreds(org, userPw), []int{201}, bodyBytes)
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
		cliutils.ExchangePutPost("Exchange", http.MethodPost, exchUrl, "orgs/"+org+"/services/"+exchId+"/dockauths", cliutils.OrgAndCreds(org, userPw), []int{201}, regTokExch)
	}

	// If necessary, tell the user to push the container images to the docker registry. Get the list of images they need to manually push
	// from the appropriate deployment config plugin.
	//
	// We will NOT tell the user to manually push images if the publish command has already pushed the images. By default, the act
	// of publishing a service will also cause the docker images used by the service to be pushed to a docker repo. The dontTouchImage flag tells
	// the publish command to skip pushing the images.
	if dontTouchImage {
		imageMap := map[string]bool{}
		for _, deployment := range []interface{}{sf.Deployment, sf.ClusterDeployment} {
			if deployment != nil && deployment != "" {
				if images, err := plugin_registry.DeploymentConfigPlugins.GetContainerImages(deployment); err != nil {
					cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("unable to get container images from deployment or cluster deployment string: %v", err))
				} else if images != nil {
					for _, img := range images {
						imageMap[img] = true
					}
				}
			}
		}

		if len(imageMap) > 0 {
			msgPrinter.Printf("If you haven't already, push your docker images to the registry:")
			msgPrinter.Println()
			for image, _ := range imageMap {
				fmt.Printf("  docker push %s\n", image)
			}
		}
	}
	return
}

func SignDeployment(deployment interface{}, deploymentSignature string, baseDir string, isCluster bool, keyFilePath, pubKeyFilePath string, dontTouchImage bool, pullImage bool) (string, string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// tags for related filed name in the service, for proper output messages
	tag_d := "deployment"
	tag_dsig := "deploymentSignature"
	if isCluster {
		tag_d = "clusterDeployment"
		tag_dsig = "clusterDeploymentSignature"
	}

	// The deployment field can be json object (map), string (for pre-signed), or nil
	var newDeployment, newDeploymentSignature string
	switch dep := deployment.(type) {
	case nil:
		deployment = ""
		if deploymentSignature != "" {
			cliutils.Warning(msgPrinter.Sprintf("the '%v' field is non-blank, but being ignored, because the '%v' field is null", tag_dsig, tag_d))
		}
		newDeploymentSignature = ""

	case map[string]interface{}:
		// We know we need to sign the deployment config, so make sure a real key file was provided.
		keyFilePath, pubKeyFilePath = cliutils.GetSigningKeys(keyFilePath, pubKeyFilePath)

		// Construct and sign the deployment string.
		msgPrinter.Printf("Signing service...")
		msgPrinter.Println()

		// Setup the Plugin context with variables that might be needed by 1 or more of the plugins.
		ctx := plugin_registry.NewPluginContext()
		ctx.Add("currentDir", baseDir)
		ctx.Add("dontTouchImage", dontTouchImage)
		ctx.Add("pullImage", pullImage)

		// Allow the right plugin to sign the deployment configuration.
		depStr, sig, err := plugin_registry.DeploymentConfigPlugins.SignByOne(dep, keyFilePath, ctx)
		if err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("unable to sign deployment config: %v", err))
		}

		newDeployment = depStr
		newDeploymentSignature = sig

	case string:
		// Means this service is pre-signed
		if deployment != "" && deploymentSignature == "" {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("the '%v' field is a non-empty string, which implies this service is pre-signed, but the '%v' field is empty", tag_d, tag_dsig))
		}
		newDeployment = dep
		newDeploymentSignature = deploymentSignature

	default:
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("'%v' field is invalid type. It must be either a json object or a string (for pre-signed)", tag_d))
	}

	return newDeployment, newDeploymentSignature
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
	var output exchange.GetServicesResponse
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

	// verify the deployment
	if svc.Deployment != "" {
		verified, err := verify.Input(keyFilePath, svc.DeploymentSignature, []byte(svc.Deployment))
		if err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("error verifying deployment string with %s: %v", keyFilePath, err))
		} else if !verified {
			msgPrinter.Printf("Deployment string was not signed with the private key associated with this public key %v.", keyFilePath)
			msgPrinter.Println()
			someInvalid = true
		}
	}
	// verify the cluster deployment
	if svc.ClusterDeployment != "" {
		verified, err := verify.Input(keyFilePath, svc.ClusterDeploymentSignature, []byte(svc.ClusterDeployment))
		if err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("error verifying cluster deployment string with %s: %v", keyFilePath, err))
		} else if !verified {
			msgPrinter.Printf("Cluster deployment string was not signed with the private key associated with this public key %v.", keyFilePath)
			msgPrinter.Println()
			someInvalid = true
		}
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
	var services exchange.ServiceDefinition
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

	//check for ExchangeUrl early on
	var exchUrl = cliutils.GetExchangeUrl()

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
	var services exchange.GetServicesResponse
	httpCode := cliutils.ExchangeGet("Exchange", exchUrl, "orgs/"+svcorg+"/services"+cliutils.AddSlash(service), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &services)
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
	cliutils.ExchangePutPost("Exchange", http.MethodPut, exchUrl, "orgs/"+svcorg+"/services/"+service+"/policy", cliutils.OrgAndCreds(org, credToUse), []int{201}, policyFile)

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
	var services exchange.ServiceDefinition
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
