package exchange

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/common"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/rsapss-tool/sign"
	"github.com/open-horizon/rsapss-tool/verify"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

//todo: only using these instead of exchange.GetPatternResponse because exchange.Pattern is missing the LastUpdated field
type ExchangePatterns struct {
	Patterns  map[string]PatternOutput `json:"patterns"`
	LastIndex int                      `json:"lastIndex"`
}

type PatternOutput struct {
	Owner              string                       `json:"owner"`
	Label              string                       `json:"label"`
	Description        string                       `json:"description"`
	Public             bool                         `json:"public"`
	Services           []ServiceReference           `json:"services"`
	AgreementProtocols []exchange.AgreementProtocol `json:"agreementProtocols"`
	UserInput          []policy.UserInput           `json:"userInput,omitempty"`
	LastUpdated        string                       `json:"lastUpdated,omitempty"`
}

// These 5 structs are used when reading json file the user gives us as input to create the pattern struct
type ServiceOverrides struct {
	Environment []string `json:"environment,omitempty"`
}
type DeploymentOverrides struct {
	Services map[string]ServiceOverrides `json:"services"`
}

type ServiceChoice struct {
	Version                      string                    `json:"version"`            // the version of the service
	Priority                     exchange.WorkloadPriority `json:"priority,omitempty"` // the highest priority service is tried first for an agreement, if it fails, the next priority is tried. Priority 1 is the highest, priority 2 is next, etc.
	Upgrade                      exchange.UpgradePolicy    `json:"upgradePolicy,omitempty"`
	DeploymentOverrides          string                    `json:"deployment_overrides,omitempty"`           // env var overrides for the service
	DeploymentOverridesSignature string                    `json:"deployment_overrides_signature,omitempty"` // signature of env var overrides
}
type ServiceReference struct {
	ServiceURL      string                    `json:"serviceUrl"`                 // refers to a service definition in the exchange
	ServiceOrg      string                    `json:"serviceOrgid"`               // the org holding the service definition
	ServiceArch     string                    `json:"serviceArch"`                // the hardware architecture of the service definition
	AgreementLess   bool                      `json:"agreementLess,omitempty"`    // a special case where this service will also be required by others
	ServiceVersions []ServiceChoice           `json:"serviceVersions,omitempty"`  // a list of service version for rollback
	DataVerify      exchange.DataVerification `json:"dataVerification,omitempty"` // policy for verifying that the node is sending data
	NodeH           *exchange.NodeHealth      `json:"nodeHealth,omitempty"`       // this needs to be a ptr so it will be omitted if not specified, so exchange will default it
}
type PatternInput struct {
	Label              string                       `json:"label"`
	Description        string                       `json:"description,omitempty"`
	Public             bool                         `json:"public"`
	Services           []ServiceReference           `json:"services,omitempty"`
	AgreementProtocols []exchange.AgreementProtocol `json:"agreementProtocols,omitempty"`
	UserInput          []policy.UserInput           `json:"userInput,omitempty"`
}

// List the pattern resources for the given org.
// The userPw can be the userId:password auth or the nodeId:token auth.
func PatternList(org string, userPw string, pattern string, namesOnly bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	//check for ExchangeUrl early on
	var exchUrl = cliutils.GetExchangeUrl()

	cliutils.SetWhetherUsingApiKey(userPw)
	var patOrg string
	patOrg, pattern = cliutils.TrimOrg(org, pattern)
	if pattern == "*" {
		pattern = ""
	}
	if namesOnly && pattern == "" {
		// Only display the names
		var resp ExchangePatterns
		cliutils.ExchangeGet("Exchange", exchUrl, "orgs/"+patOrg+"/patterns"+cliutils.AddSlash(pattern), cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &resp)
		patterns := []string{} // this is important (instead of leaving it nil) so json marshaling displays it as [] instead of null
		for p := range resp.Patterns {
			patterns = append(patterns, p)
		}
		jsonBytes, err := json.MarshalIndent(patterns, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'exchange pattern list' output: %v", err))
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		// Display the full resources
		var patterns ExchangePatterns
		httpCode := cliutils.ExchangeGet("Exchange", exchUrl, "orgs/"+patOrg+"/patterns"+cliutils.AddSlash(pattern), cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &patterns)
		if httpCode == 404 && pattern != "" {
			cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("pattern '%s' not found in org %s", pattern, patOrg))
		}
		jsonBytes, err := json.MarshalIndent(patterns.Patterns, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn exchange pattern list' output: %v", err))
		}
		fmt.Println(string(jsonBytes))
	}
}

// This function updates an attribute for the given pattern
func PatternUpdate(org string, credToUse string, pattern string, filePath string) {

	//check for ExchangeUrl early on
	var exchUrl = cliutils.GetExchangeUrl()

	cliutils.SetWhetherUsingApiKey(credToUse)
	var patOrg string
	patOrg, pattern = cliutils.TrimOrg(org, pattern)

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	//Read in the file
	attribute := cliconfig.ReadJsonFileWithLocalConfig(filePath)

	//verify that the pattern exists
	var exchPatterns ExchangePatterns
	httpCode := cliutils.ExchangeGet("Exchange", exchUrl, "orgs/"+patOrg+"/patterns"+cliutils.AddSlash(pattern), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &exchPatterns)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("Pattern %s not found in org %s", pattern, patOrg))
	}

	findPatchType := make(map[string]interface{})

	json.Unmarshal([]byte(attribute), &findPatchType)

	var patch interface{}
	var err error
	if _, ok := findPatchType["services"]; ok {
		patch = make(map[string][]ServiceReference)
		err = json.Unmarshal([]byte(attribute), &patch)
	} else if _, ok := findPatchType["userInput"]; ok {
		patch = make(map[string][]policy.UserInput)
		err = json.Unmarshal([]byte(attribute), &patch)
	} else {
		_, ok := findPatchType["label"]
		_, ok2 := findPatchType["description"]
		if ok || ok2 {
			patch = make(map[string]string)
			err = json.Unmarshal([]byte(attribute), &patch)
		} else {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Pattern attribute to be updated is not found in the input file. Supported attributes are: label, description, services, and userInput."))
		}
	}

	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal attribute input %s: %v", attribute, err))
	}
	cliutils.ExchangePutPost("Exchange", http.MethodPatch, exchUrl, "orgs/"+patOrg+"/patterns"+cliutils.AddSlash(pattern), cliutils.OrgAndCreds(org, credToUse), []int{201}, patch)
	msgPrinter.Printf("Pattern %v/%v updated in the Horizon Exchange", patOrg, pattern)
	msgPrinter.Println()
}

// Take the deployment overrides field, which we have told the json unmarshaller was unknown type (so we can handle both escaped string and struct)
// and turn it into the DeploymentOverrides struct we really want.
func ConvertToDeploymentOverrides(deployment interface{}) *DeploymentOverrides {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

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
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal body for %v: %v", d, err))
		}
	}

	// Now unmarshal the bytes into the struct we have wanted all along
	depOver := new(DeploymentOverrides)
	err = json.Unmarshal(jsonBytes, depOver)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal json for deployment overrides field %s: %v", string(jsonBytes), err))
	}

	return depOver
}

// PatternPublish signs the MS def and puts it in the exchange
func PatternPublish(org, userPw, jsonFilePath, keyFilePath, pubKeyFilePath, patName string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	//Get ExchangeUrl value early on
	var exchUrl = cliutils.GetExchangeUrl()

	cliutils.SetWhetherUsingApiKey(userPw)
	// Read in the pattern metadata
	newBytes := cliconfig.ReadJsonFileWithLocalConfig(jsonFilePath)
	var patFile common.PatternFile
	err := json.Unmarshal(newBytes, &patFile)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal json input file %s: %v", jsonFilePath, err))
	}
	if patFile.Org != "" && patFile.Org != org {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("the org specified in the input file (%s) must match the org specified on the command line (%s)", patFile.Org, org))
	}
	if patFile.Org == "" {
		patFile.Org = org
	}
	patInput := PatternInput{Label: patFile.Label, Description: patFile.Description, Public: patFile.Public, AgreementProtocols: patFile.AgreementProtocols, UserInput: patFile.UserInput}

	//issue 924: Patterns with no services are not allowed
	if patFile.Services == nil || len(patFile.Services) == 0 {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("the pattern definition (%s) must contain services, unable to proceed", patFile.Services))
	}

	keyVerified := false
	// Loop thru the services array and the servicesVersions array and sign the deployment_overrides fields
	if patFile.Services != nil && len(patFile.Services) > 0 {
		patInput.Services = make([]ServiceReference, len(patFile.Services))
		for i := range patFile.Services {
			patInput.Services[i].ServiceURL = patFile.Services[i].ServiceURL
			patInput.Services[i].ServiceOrg = patFile.Services[i].ServiceOrg
			patInput.Services[i].ServiceArch = patFile.Services[i].ServiceArch
			patInput.Services[i].AgreementLess = patFile.Services[i].AgreementLess
			patInput.Services[i].ServiceVersions = make([]ServiceChoice, len(patFile.Services[i].ServiceVersions))
			if patFile.Services[i].DataVerify != nil {
				patInput.Services[i].DataVerify = *patFile.Services[i].DataVerify
			}
			patInput.Services[i].NodeH = patFile.Services[i].NodeH
			for j := range patFile.Services[i].ServiceVersions {
				patInput.Services[i].ServiceVersions[j].Version = patFile.Services[i].ServiceVersions[j].Version
				if patFile.Services[i].ServiceVersions[j].Priority != nil {
					patInput.Services[i].ServiceVersions[j].Priority = *patFile.Services[i].ServiceVersions[j].Priority
				}
				if patFile.Services[i].ServiceVersions[j].Upgrade != nil {
					patInput.Services[i].ServiceVersions[j].Upgrade = *patFile.Services[i].ServiceVersions[j].Upgrade
				}
				var err error
				var deployment []byte
				depOver := ConvertToDeploymentOverrides(patFile.Services[i].ServiceVersions[j].DeploymentOverrides)
				// If the input deployment overrides are already in string form and signed, then use them as is.
				if patFile.Services[i].ServiceVersions[j].DeploymentOverrides != nil && reflect.TypeOf(patFile.Services[i].ServiceVersions[j].DeploymentOverrides).String() == "string" && patFile.Services[i].ServiceVersions[j].DeploymentOverridesSignature != "" {
					patInput.Services[i].ServiceVersions[j].DeploymentOverrides = patFile.Services[i].ServiceVersions[j].DeploymentOverrides.(string)
					patInput.Services[i].ServiceVersions[j].DeploymentOverridesSignature = patFile.Services[i].ServiceVersions[j].DeploymentOverridesSignature
				} else if depOver == nil {
					// If the input deployment override is an object that is nil, then there are no overrides, so no signing necessary.
					patInput.Services[i].ServiceVersions[j].DeploymentOverrides = ""
					patInput.Services[i].ServiceVersions[j].DeploymentOverridesSignature = ""
				} else {
					msgPrinter.Printf("Signing deployment_overrides field in service %d, serviceVersion number %d", i+1, j+1)
					msgPrinter.Println()
					deployment, err = json.Marshal(depOver)
					if err != nil {
						cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal deployment_overrides field in service %d, serviceVersion number %d: %v", i+1, j+1, err))
					}
					patInput.Services[i].ServiceVersions[j].DeploymentOverrides = string(deployment)
					// We know we need to sign the overrides, so make sure a real key file was provided.
					if !keyVerified {
						keyFilePath, pubKeyFilePath = cliutils.GetSigningKeys(keyFilePath, pubKeyFilePath)
						keyVerified = true
					}

					patInput.Services[i].ServiceVersions[j].DeploymentOverridesSignature, err = sign.Input(keyFilePath, deployment)
					if err != nil {
						cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("problem signing the deployment_overrides string with %s: %v", keyFilePath, err))
					}
				}
			}
		}
	}

	// Create or update resource in the exchange
	var exchId string
	if patName != "" {
		exchId = patName
	} else if patFile.Name != "" {
		exchId = patFile.Name
	} else {
		// Use the json file base name as the default for the pattern name
		exchId = filepath.Base(jsonFilePath)                      // remove the leading path
		exchId = strings.TrimSuffix(exchId, filepath.Ext(exchId)) // strip suffix if there
	}
	// replace the unwanted charactors from the id with '-'
	exchId = cutil.FormExchangeId(exchId)

	var output string
	httpCode := cliutils.ExchangeGet("Exchange", exchUrl, "orgs/"+patFile.Org+"/patterns/"+exchId, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
	if httpCode == 200 {
		// Pattern exists, update it
		msgPrinter.Printf("Updating %s in the exchange...", exchId)
		msgPrinter.Println()
		cliutils.ExchangePutPost("Exchange", http.MethodPut, exchUrl, "orgs/"+patFile.Org+"/patterns/"+exchId, cliutils.OrgAndCreds(org, userPw), []int{201}, patInput)
	} else {
		// Pattern not there, create it
		msgPrinter.Printf("Creating %s in the exchange...", exchId)
		msgPrinter.Println()
		cliutils.ExchangePutPost("Exchange", http.MethodPost, exchUrl, "orgs/"+patFile.Org+"/patterns/"+exchId, cliutils.OrgAndCreds(org, userPw), []int{201}, patInput)
	}

	// Store the public key in the exchange, if they gave it to us
	if pubKeyFilePath != "" {
		// Note: already verified the file exists
		if !keyVerified {
			pubKeyFilePath = cliutils.GetAndVerifyPublicKey(pubKeyFilePath)
		}
		bodyBytes := cliutils.ReadFile(pubKeyFilePath)
		baseName := filepath.Base(pubKeyFilePath)
		msgPrinter.Printf("Storing %s with the pattern in the exchange...", baseName)
		msgPrinter.Println()
		cliutils.ExchangePutPost("Exchange", http.MethodPut, exchUrl, "orgs/"+patFile.Org+"/patterns/"+exchId+"/keys/"+baseName, cliutils.OrgAndCreds(org, userPw), []int{201}, bodyBytes)
	}
}

// Verify that the deployment_overrides_signature is valid for the given key.
// The userPw can be the userId:password auth or the nodeId:token auth.
func PatternVerify(org, userPw, pattern, keyFilePath string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	//check for ExchangeUrl early on
	var exchUrl = cliutils.GetExchangeUrl()

	cliutils.SetWhetherUsingApiKey(userPw)
	var patorg string
	patorg, pattern = cliutils.TrimOrg(org, pattern)
	// Get pattern resource from exchange
	var output ExchangePatterns
	httpCode := cliutils.ExchangeGet("Exchange", exchUrl, "orgs/"+patorg+"/patterns/"+pattern, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("pattern '%s' not found in org %s", pattern, org))
	}
	pat, ok := output.Patterns[org+"/"+pattern]
	if !ok {
		cliutils.Fatal(cliutils.INTERNAL_ERROR, msgPrinter.Sprintf("key '%s' not found in resources returned from exchange", org+"/"+pattern))
	}

	// Loop thru services array, checking the deployment string signature
	someInvalid := false
	keyVerified := false
	for i := range pat.Services {
		for j := range pat.Services[i].ServiceVersions {
			cliutils.Verbose(msgPrinter.Sprintf("verifying deployment_overrides string in service %d, serviceVersion number %d", i+1, j+1))
			if pat.Services[i].ServiceVersions[j].DeploymentOverrides == "" && pat.Services[i].ServiceVersions[j].DeploymentOverridesSignature == "" {
				continue // there was nothing to sign, so nothing to verify
			}
			if !keyVerified {
				//take default key if empty, make sure the key exists
				keyFilePath = cliutils.GetAndVerifyPublicKey(keyFilePath)
				keyVerified = true
			}
			verified, err := verify.Input(keyFilePath, pat.Services[i].ServiceVersions[j].DeploymentOverridesSignature, []byte(pat.Services[i].ServiceVersions[j].DeploymentOverrides))
			if err != nil {
				cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("problem verifying deployment_overrides string in service %d, serviceVersion number %d with %s: %v", i+1, j+1, keyFilePath, err))
			} else if !verified {
				msgPrinter.Printf("Deployment_overrides string in service %d, serviceVersion number %d was not signed with the private key associated with this public key %v.", i+1, j+1, keyFilePath)
				msgPrinter.Println()
				someInvalid = true
			}
			// else if they all turned out to be valid, we will tell them that at the end
		}
	}

	if someInvalid {
		os.Exit(cliutils.SIGNATURE_INVALID)
	} else {
		msgPrinter.Printf("All signatures verified")
		msgPrinter.Println()
	}
}

func PatternRemove(org, userPw, pattern string, force bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	//check for ExchangeUrl early on
	var exchUrl = cliutils.GetExchangeUrl()

	cliutils.SetWhetherUsingApiKey(userPw)
	var patorg string
	patorg, pattern = cliutils.TrimOrg(org, pattern)
	if !force {
		cliutils.ConfirmRemove(msgPrinter.Sprintf("Are you sure you want to remove pattern %v/%v from the Horizon Exchange?", org, pattern))
	}

	httpCode := cliutils.ExchangeDelete("Exchange", exchUrl, "orgs/"+patorg+"/patterns/"+pattern, cliutils.OrgAndCreds(org, userPw), []int{204, 404})
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("pattern '%s' not found in org %s", pattern, org))
	}
}

// List the public keys that can be used to verify the deployment_overrides_signature for a pattern.
// The userPw can be the userId:password auth or the nodeId:token auth.
func PatternListKey(org, userPw, pattern, keyName string) {
	//check for ExchangeUrl early on
	var exchUrl = cliutils.GetExchangeUrl()

	cliutils.SetWhetherUsingApiKey(userPw)
	var patorg string
	patorg, pattern = cliutils.TrimOrg(org, pattern)
	if keyName == "" {
		// Only display the names
		var output string
		cliutils.ExchangeGet("Exchange", exchUrl, "orgs/"+patorg+"/patterns/"+pattern+"/keys", cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
		fmt.Printf("%s\n", output)
	} else {
		// Display the content of the key
		var output []byte
		httpCode := cliutils.ExchangeGet("Exchange", exchUrl, "orgs/"+patorg+"/patterns/"+pattern+"/keys/"+keyName, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
		if httpCode == 404 && pattern != "" {
			cliutils.Fatal(cliutils.NOT_FOUND, i18n.GetMessagePrinter().Sprintf("key '%s' not found", keyName))
		}
		fmt.Printf("%s", string(output))
	}
}

func PatternRemoveKey(org, userPw, pattern, keyName string) {
	//check for ExchangeUrl early on
	var exchUrl = cliutils.GetExchangeUrl()

	cliutils.SetWhetherUsingApiKey(userPw)
	var patorg string
	patorg, pattern = cliutils.TrimOrg(org, pattern)
	httpCode := cliutils.ExchangeDelete("Exchange", exchUrl, "orgs/"+patorg+"/patterns/"+pattern+"/keys/"+keyName, cliutils.OrgAndCreds(org, userPw), []int{204, 404})
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, i18n.GetMessagePrinter().Sprintf("key '%s' not found", keyName))
	}
}
