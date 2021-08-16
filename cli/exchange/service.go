package exchange

import (
	"bytes"
	"crypto/rsa"
	"encoding/base64"
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
	"github.com/open-horizon/anax/persistence"
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
func ServiceList(credOrg, userPw, service string, namesOnly bool, filePath string, exSvcOpYamlForce bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	cliutils.SetWhetherUsingApiKey(userPw)
	var svcOrg string
	svcOrg, service = cliutils.TrimOrg(credOrg, service)
	if service == "*" {
		service = ""
	}

	if service == "" && filePath != "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("-f can only be used when one service is specified."))
	}

	if exSvcOpYamlForce && filePath == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("-F can only be used when -f is specified."))
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

		exchServices := services.Services
		var clusterDeployment string
		var svcId string
		for sId, s := range exchServices {
			// when only one service is specified, save the ClusterDeployment and service id for later use
			if service != "" {
				clusterDeployment = s.ClusterDeployment
				svcId = sId
			}

			// only display 100 charactors for ClusterDeployment because it is usually very long
			if s.ClusterDeployment != "" && len(s.ClusterDeployment) > 100 {
				if service != "" && !namesOnly {
					// if user specify a service name and -l is specified, then display all of ClusterDeployment
					// this will give user a way to examine all of it.
					continue
				}
				s_copy := exchange.ServiceDefinition(s)
				s_copy.ClusterDeployment = s.ClusterDeployment[0:100] + "..."
				exchServices[sId] = s_copy
			}
		}
		jsonBytes, err := json.MarshalIndent(exchServices, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn exchange service list' output: %v", err))
		}
		fmt.Println(string(jsonBytes))

		// save the kube operator yaml archive to file if filePath is specified and one service is specified
		if filePath != "" {
			if clusterDeployment == "" {
				msgPrinter.Printf("Ignoring -f because the clusterDeployment attribute is empty for this service.")
				msgPrinter.Println()
			} else {
				SaveOpYamlToFile(svcId, clusterDeployment, filePath, exSvcOpYamlForce)
			}
		}
	}
}

// ServicePublish signs the MS def and puts it in the exchange
func ServicePublish(org, userPw, jsonFilePath, keyFilePath, pubKeyFilePath string, dontTouchImage bool, pullImage bool, registryTokens []string, overwrite bool, servicePolicyFilePath string, public string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if pubKeyFilePath != "" && keyFilePath == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Flag -K cannot be specified without -k flag."))
	}
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
	if public != "" {
		public = strings.ToLower(public)
		if public != "true" && public != "false" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Need to set 'true' or 'false' when specifying flag --public."))
		} else {
			// Override public key with key set in the hzn command
			svcFile.Public, err = strconv.ParseBool(public)
			if err != nil {
				cliutils.Fatal(cliutils.INTERNAL_ERROR, msgPrinter.Sprintf("failed to parse %s: %v", public, err))
			}

		}
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
	var usedPubKeyBytes []byte
	var usedPubKeyBytes_cluster []byte
	usedPubKeyName := ""
	usedPubKeyName_cluster := ""
	svcInput.Deployment, svcInput.DeploymentSignature, usedPubKeyBytes, usedPubKeyName = SignDeployment(sf.Deployment, sf.DeploymentSignature, baseDir, false, keyFilePath, pubKeyFilePath, dontTouchImage, pullImage)
	svcInput.ClusterDeployment, svcInput.ClusterDeploymentSignature, usedPubKeyBytes_cluster, usedPubKeyName_cluster = SignDeployment(sf.ClusterDeployment, sf.ClusterDeploymentSignature, baseDir, true, keyFilePath, pubKeyFilePath, dontTouchImage, pullImage)

	// Create or update resource in the exchange
	exchId := cutil.FormExchangeIdForService(svcInput.URL, svcInput.Version, svcInput.Arch)
	var output string
	httpCode := cliutils.ExchangeGet("Exchange", exchUrl, "orgs/"+org+"/services/"+exchId, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
	if httpCode == 200 {
		// check if the service exists with the same version, ask user if -O is not specified.
		if promptForOverwrite {
			cliutils.ConfirmRemove(msgPrinter.Sprintf("Service %v/%v exists in the Exchange, do you want to overwrite it?", org, exchId))
		}
		// Service exists, update it
		msgPrinter.Printf("Updating %s in the Exchange...", exchId)
		msgPrinter.Println()
		cliutils.ExchangePutPost("Exchange", http.MethodPut, exchUrl, "orgs/"+org+"/services/"+exchId, cliutils.OrgAndCreds(org, userPw), []int{201}, svcInput, nil)
	} else {
		// Service not there, create it
		msgPrinter.Printf("Creating %s in the Exchange...", exchId)
		msgPrinter.Println()
		cliutils.ExchangePutPost("Exchange", http.MethodPost, exchUrl, "orgs/"+org+"/services", cliutils.OrgAndCreds(org, userPw), []int{201}, svcInput, nil)
	}

	// Store the public key in the exchange
	var pubKeyNameToStore string
	var pubKeyToStore []byte
	if usedPubKeyName != "" {
		pubKeyNameToStore = usedPubKeyName
		pubKeyToStore = usedPubKeyBytes
	} else if usedPubKeyName_cluster != "" {
		pubKeyNameToStore = usedPubKeyName_cluster
		pubKeyToStore = usedPubKeyBytes_cluster
	}
	if pubKeyNameToStore != "" {
		msgPrinter.Printf("Storing %s with the service in the Exchange...", pubKeyNameToStore)
		msgPrinter.Println()
		cliutils.ExchangePutPost("Exchange", http.MethodPut, exchUrl, "orgs/"+org+"/services/"+exchId+"/keys/"+pubKeyNameToStore, cliutils.OrgAndCreds(org, userPw), []int{201}, pubKeyToStore, nil)
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
		msgPrinter.Printf("Storing %s with the service in the Exchange...", regTok)
		msgPrinter.Println()
		regTokExch := ServiceDockAuthExch{Registry: regstry, UserName: username, Token: token}
		cliutils.ExchangePutPost("Exchange", http.MethodPost, exchUrl, "orgs/"+org+"/services/"+exchId+"/dockauths", cliutils.OrgAndCreds(org, userPw), []int{201}, regTokExch, nil)
	}

	// If necessary, tell the user to push the container images to the docker registry. Get the list of images they need to manually push
	// from the appropriate deployment config plugin.
	//
	// We will NOT tell the user to manually push images if the publish command has already pushed the images. By default, the act
	// of publishing a service will also cause the docker images used by the service to be pushed to a docker repo. The dontTouchImage flag tells
	// the publish command to skip pushing the images.
	if dontTouchImage {
		imageMap := map[string]bool{}

		if sf.Deployment != nil && sf.Deployment != "" {
			if images, err := plugin_registry.DeploymentConfigPlugins.GetContainerImages(sf.Deployment); err != nil {
				cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("unable to get container images from deployment configuration: %v", err))
			} else if images != nil {
				for _, img := range images {
					imageMap[img] = true
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

// The function signs the given deployment if it is not empty abd not already signed. It returns the deployment, its signature
// and the public key whose matching private was used for signing the deployment.
func SignDeployment(deployment interface{}, deploymentSignature string, baseDir string, isCluster bool, keyFilePath string, pubKeyFilePath string, dontTouchImage bool, pullImage bool) (string, string, []byte, string) {
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
	var newDeployment, newDeploymentSignature, newPubKeyName string
	var newPubKeyToStore []byte
	var newPrivKeyToStore *rsa.PrivateKey
	switch dep := deployment.(type) {
	case nil:
		deployment = ""
		if deploymentSignature != "" {
			cliutils.Warning(msgPrinter.Sprintf("the '%v' field is non-blank, but being ignored, because the '%v' field is null", tag_dsig, tag_d))
		}
		newDeploymentSignature = ""

	case map[string]interface{}:
		// We know we need to sign the deployment config, so make sure a real key file was provided.
		newPrivKeyToStore, newPubKeyToStore, newPubKeyName = cliutils.GetSigningKeys(keyFilePath, pubKeyFilePath)

		// Construct and sign the deployment string.
		msgPrinter.Printf("Signing service...")
		msgPrinter.Println()

		// Setup the Plugin context with variables that might be needed by 1 or more of the plugins.
		ctx := plugin_registry.NewPluginContext()
		ctx.Add("currentDir", baseDir)
		ctx.Add("dontTouchImage", dontTouchImage)
		ctx.Add("pullImage", pullImage)

		// Allow the right plugin to sign the deployment configuration.
		depStr, sig, err := plugin_registry.DeploymentConfigPlugins.SignByOne(dep, newPrivKeyToStore, ctx)
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

	return newDeployment, newDeploymentSignature, newPubKeyToStore, newPubKeyName
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
	err = policyFile.ValidateAndNormalize()
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
	err = policyFile.ValidateAndNormalize()
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Incorrect policy format in file %s: %v", jsonFilePath, err))
	}

	// add/replace service policy
	msgPrinter.Printf("Updating Service policy  and re-evaluating all agreements based on this Service policy. Existing agreements might be cancelled and re-negotiated.")
	msgPrinter.Println()
	cliutils.ExchangePutPost("Exchange", http.MethodPut, exchUrl, "orgs/"+svcorg+"/services/"+service+"/policy", cliutils.OrgAndCreds(org, credToUse), []int{201}, policyFile, nil)

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
	msgPrinter.Printf("Removing Service policy and re-evaluating all agreements. Existing agreements might be cancelled and re-negotiated.")
	msgPrinter.Println()
	cliutils.ExchangeDelete("Exchange", cliutils.GetExchangeUrl(), "orgs/"+svcorg+"/services/"+service+"/policy", cliutils.OrgAndCreds(org, credToUse), []int{204, 404})
	msgPrinter.Printf("Service policy removed.")
	msgPrinter.Println()
}

// Display an empty service policy template as an object.
func ServiceNewPolicy() {
	policy.New()
}

// Get the Kubernetes operator yaml archive from the cluster deployment string and save it to a file
func SaveOpYamlToFile(sId string, clusterDeployment string, filePath string, forceOverwrite bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	archiveData := GetOpYamlArchiveFromClusterDepl(clusterDeployment)

	fileName := filePath
	if info, err := os.Stat(filePath); err == nil && info.IsDir() {
		_, id := cliutils.TrimOrg("", sId) // remove the org part
		fileName = filepath.Join(filePath, id+"_operator_yaml.tar.gz")
	}

	if _, err := os.Stat(fileName); err == nil {
		if !forceOverwrite {
			cliutils.ConfirmRemove(msgPrinter.Sprintf("File %v already exists, do you want to overwrite?", fileName))
		}
	}

	file, err := os.Create(fileName)
	if err != nil {
		cliutils.Fatal(cliutils.INTERNAL_ERROR, msgPrinter.Sprintf("Failed to create file %v. %v", fileName, err))
	}
	defer file.Close()

	if _, err := file.Write(archiveData); err != nil {
		cliutils.Fatal(cliutils.INTERNAL_ERROR, msgPrinter.Sprintf("Failed to save the clusterDeployment operator yaml to file %v. %v", fileName, err))
	}

	msgPrinter.Printf("The clusterDeployment operator yaml archive is saved to file %v", fileName)
	msgPrinter.Println()
}

// This function getst the operator yaml archive (in .tat.gz format) from the clusterDeployment
// string from a service
func GetOpYamlArchiveFromClusterDepl(deploymentConfig string) []byte {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// for non cluster type service, return nil
	if deploymentConfig == "" {
		return nil
	}

	if kd, err := persistence.GetKubeDeployment(deploymentConfig); err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("error getting kube deployment configuration: %v", err))
	} else {
		archiveData, err := base64.StdEncoding.DecodeString(kd.OperatorYamlArchive)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("error decoding the cluster deployment configuration: %v", err))
		}
		return archiveData
	}

	return nil
}

type ServiceNode struct {
	OrgId          string `json:"orgid"`
	ServiceUrl     string `json:"serviceURL"`
	ServiceVersion string `json:"serviceVersion"`
	ServiceArch    string `json:"serviceArch"`
}

// List the nodes that a service is running on.
func ListServiceNodes(org, userPw, svcId, nodeOrg string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	//check for ExchangeUrl early on
	var exchUrl = cliutils.GetExchangeUrl()

	// if nodeOrg is not specified, default to service org
	if nodeOrg == "" {
		nodeOrg = org
	}

	// extract service id
	var svcOrg string
	svcOrg, svcId = cliutils.TrimOrg(org, svcId)

	// get service from the Exchange
	var services exchange.GetServicesResponse
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+svcOrg+"/services"+cliutils.AddSlash(svcId), cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &services)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("service id does not exist in the Exchange: %v/%v", svcOrg, svcId))
	} else {
		// extract org id, service url, version, and arch from Exchange
		var svcNode ServiceNode
		for id, _ := range services.Services {
			svcNode = ServiceNode{OrgId: svcOrg, ServiceUrl: services.Services[id].URL, ServiceVersion: services.Services[id].Version, ServiceArch: services.Services[id].Arch}
			break
		}

		// structure we are writing to
		var listNodes map[string]interface{}
		cliutils.ExchangePutPost("Exchange", http.MethodPost, exchUrl, "orgs/"+nodeOrg+"/search/nodes/service", cliutils.OrgAndCreds(org, userPw), []int{201}, svcNode, &listNodes)

		// print list
		if nodes, ok := listNodes["nodes"]; !ok {
			fmt.Println("[]")
		} else {
			jsonBytes, err := json.MarshalIndent(nodes, "", cliutils.JSON_INDENT)
			if err != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn exchange service listnode' output: %v", err))
			}
			fmt.Printf("%s\n", jsonBytes)
		}
	}
}
