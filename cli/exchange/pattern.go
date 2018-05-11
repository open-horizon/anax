package exchange

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/exchange"
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
	Workloads          []WorkloadReference          `json:"workloads"`
	AgreementProtocols []exchange.AgreementProtocol `json:"agreementProtocols"`
	LastUpdated        string                       `json:"lastUpdated"`
}

// These 5 structs are used when reading json file the user gives us as input to create the pattern struct
type ServiceOverrides struct {
	Environment []string `json:"environment,omitempty"`
}
type DeploymentOverrides struct {
	Services map[string]ServiceOverrides `json:"services"`
}
type WorkloadChoiceFile struct {
	Version  string                    `json:"version"`  // the version of the workload
	Priority exchange.WorkloadPriority `json:"priority"` // the highest priority workload is tried first for an agreement, if it fails, the next priority is tried. Priority 1 is the highest, priority 2 is next, etc.
	Upgrade  exchange.UpgradePolicy    `json:"upgradePolicy"`
	//DeploymentOverrides          DeploymentOverrides       `json:"deployment_overrides"`           // env var overrides for the workload
	DeploymentOverrides          interface{} `json:"deployment_overrides"`           // env var overrides for the workload
	DeploymentOverridesSignature string      `json:"deployment_overrides_signature"` // signature of env var overrides
}
type WorkloadReferenceFile struct {
	WorkloadURL      string                    `json:"workloadUrl"`      // refers to a workload definition in the exchange
	WorkloadOrg      string                    `json:"workloadOrgid"`    // the org holding the workload definition
	WorkloadArch     string                    `json:"workloadArch"`     // the hardware architecture of the workload definition
	WorkloadVersions []WorkloadChoiceFile      `json:"workloadVersions"` // a list of workload version for rollback
	DataVerify       exchange.DataVerification `json:"dataVerification"` // policy for verifying that the node is sending data
	NodeH            exchange.NodeHealth       `json:"nodeHealth"`       // policy for determining when a node's health is violating its agreements
}
type ServiceChoiceFile struct {
	Version                      string                    `json:"version"`  // the version of the service
	Priority                     exchange.WorkloadPriority `json:"priority"` // the highest priority service is tried first for an agreement, if it fails, the next priority is tried. Priority 1 is the highest, priority 2 is next, etc.
	Upgrade                      exchange.UpgradePolicy    `json:"upgradePolicy"`
	DeploymentOverrides          interface{}               `json:"deployment_overrides"`           // env var overrides for the service
	DeploymentOverridesSignature string                    `json:"deployment_overrides_signature"` // signature of env var overrides
}
type ServiceReferenceFile struct {
	ServiceURL      string                    `json:"serviceUrl"`              // refers to a service definition in the exchange
	ServiceOrg      string                    `json:"serviceOrgid"`            // the org holding the service definition
	ServiceArch     string                    `json:"serviceArch"`             // the hardware architecture of the service definition
	AgreementLess   bool                      `json:"agreementLess,omitempty"` // a special case where this service will also be required by others
	ServiceVersions []ServiceChoiceFile       `json:"serviceVersions"`         // a list of service version for rollback
	DataVerify      exchange.DataVerification `json:"dataVerification"`        // policy for verifying that the node is sending data
	NodeH           exchange.NodeHealth       `json:"nodeHealth"`              // policy for determining when a node's health is violating its agreements
}
type PatternFile struct {
	Org                string                       `json:"org"` // optional
	Label              string                       `json:"label"`
	Description        string                       `json:"description"`
	Public             bool                         `json:"public"`
	Services           []ServiceReferenceFile       `json:"services"`
	Workloads          []WorkloadReferenceFile      `json:"workloads"`
	AgreementProtocols []exchange.AgreementProtocol `json:"agreementProtocols"`
}

// These 3 structs are used as the input to the exchange to create the pattern
//todo: can't use exchange.Pattern (and some sub-structs) because it has omitempty on several fields required by the exchange
type WorkloadChoice struct {
	Version                      string                    `json:"version"`  // the version of the workload
	Priority                     exchange.WorkloadPriority `json:"priority"` // the highest priority workload is tried first for an agreement, if it fails, the next priority is tried. Priority 1 is the highest, priority 2 is next, etc.
	Upgrade                      exchange.UpgradePolicy    `json:"upgradePolicy"`
	DeploymentOverrides          string                    `json:"deployment_overrides"`           // env var overrides for the workload
	DeploymentOverridesSignature string                    `json:"deployment_overrides_signature"` // signature of env var overrides
}
type WorkloadReference struct {
	WorkloadURL      string                    `json:"workloadUrl"`      // refers to a workload definition in the exchange
	WorkloadOrg      string                    `json:"workloadOrgid"`    // the org holding the workload definition
	WorkloadArch     string                    `json:"workloadArch"`     // the hardware architecture of the workload definition
	WorkloadVersions []WorkloadChoice          `json:"workloadVersions"` // a list of workload version for rollback
	DataVerify       exchange.DataVerification `json:"dataVerification"` // policy for verifying that the node is sending data
	NodeH            exchange.NodeHealth       `json:"nodeHealth"`       // policy for determining when a node's health is violating its agreements
}
type ServiceChoice struct {
	Version                      string                    `json:"version"`  // the version of the service
	Priority                     exchange.WorkloadPriority `json:"priority"` // the highest priority service is tried first for an agreement, if it fails, the next priority is tried. Priority 1 is the highest, priority 2 is next, etc.
	Upgrade                      exchange.UpgradePolicy    `json:"upgradePolicy"`
	DeploymentOverrides          string                    `json:"deployment_overrides"`           // env var overrides for the service
	DeploymentOverridesSignature string                    `json:"deployment_overrides_signature"` // signature of env var overrides
}
type ServiceReference struct {
	ServiceURL      string                    `json:"serviceUrl"`              // refers to a service definition in the exchange
	ServiceOrg      string                    `json:"serviceOrgid"`            // the org holding the service definition
	ServiceArch     string                    `json:"serviceArch"`             // the hardware architecture of the service definition
	AgreementLess   bool                      `json:"agreementLess,omitempty"` // a special case where this service will also be required by others
	ServiceVersions []ServiceChoice           `json:"serviceVersions"`         // a list of service version for rollback
	DataVerify      exchange.DataVerification `json:"dataVerification"`        // policy for verifying that the node is sending data
	NodeH           exchange.NodeHealth       `json:"nodeHealth"`              // policy for determining when a node's health is violating its agreements
}
type PatternInput struct {
	Label              string                       `json:"label"`
	Description        string                       `json:"description"`
	Public             bool                         `json:"public"`
	Services           []ServiceReference           `json:"services"`
	Workloads          []WorkloadReference          `json:"workloads"`
	AgreementProtocols []exchange.AgreementProtocol `json:"agreementProtocols"`
}

func PatternList(org string, userPw string, pattern string, namesOnly bool) {
	cliutils.SetWhetherUsingApiKey(userPw)
	org, pattern = cliutils.TrimOrg(org, pattern)
	if namesOnly && pattern == "" {
		// Only display the names
		var resp ExchangePatterns
		cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/patterns"+cliutils.AddSlash(pattern), cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &resp)
		patterns := []string{} // this is important (instead of leaving it nil) so json marshaling displays it as [] instead of null
		for p := range resp.Patterns {
			patterns = append(patterns, p)
		}
		jsonBytes, err := json.MarshalIndent(patterns, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'exchange pattern list' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		// Display the full resources
		var patterns ExchangePatterns
		httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/patterns"+cliutils.AddSlash(pattern), cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &patterns)
		if httpCode == 404 && pattern != "" {
			cliutils.Fatal(cliutils.NOT_FOUND, "pattern '%s' not found in org %s", pattern, org)
		}
		jsonBytes, err := json.MarshalIndent(patterns.Patterns, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn exchange pattern list' output: %v", err)
		}
		fmt.Println(string(jsonBytes))
	}
}

// Take the deployment overrides field, which we have told the json unmarshaller was unknown type (so we can handle both escaped string and struct)
// and turn it into the DeploymentOverrides struct we really want.
func ConvertToDeploymentOverrides(deployment interface{}) *DeploymentOverrides {
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
	depOver := new(DeploymentOverrides)
	err = json.Unmarshal(jsonBytes, depOver)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to unmarshal json for deployment overrides field %s: %v", string(jsonBytes), err)
	}

	return depOver
}

// PatternPublish signs the MS def and puts it in the exchange
func PatternPublish(org, userPw, jsonFilePath, keyFilePath, pubKeyFilePath, patName string) {
	cliutils.SetWhetherUsingApiKey(userPw)
	// Read in the pattern metadata
	newBytes := cliutils.ReadJsonFile(jsonFilePath)
	var patFile PatternFile
	err := json.Unmarshal(newBytes, &patFile)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to unmarshal json input file %s: %v", jsonFilePath, err)
	}
	if patFile.Org != "" && patFile.Org != org {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "the org specified in the input file (%s) must match the org specified on the command line (%s)", patFile.Org, org)
	}
	if patFile.Workloads != nil && len(patFile.Workloads) > 0 && patFile.Services != nil && len(patFile.Services) > 0 {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "you can not specify both the 'workloads' and 'services' fields.")
	}
	patInput := PatternInput{Label: patFile.Label, Description: patFile.Description, Public: patFile.Public, AgreementProtocols: patFile.AgreementProtocols}

	// Loop thru the services/workloads array and the servicesVersions/workloadVersions array and sign the deployment_overrides fields
	if patFile.Services != nil && len(patFile.Services) > 0 {
		patInput.Services = make([]ServiceReference, len(patFile.Services))
		for i := range patFile.Services {
			patInput.Services[i].ServiceURL = patFile.Services[i].ServiceURL
			patInput.Services[i].ServiceOrg = patFile.Services[i].ServiceOrg
			patInput.Services[i].ServiceArch = patFile.Services[i].ServiceArch
			patInput.Services[i].AgreementLess = patFile.Services[i].AgreementLess
			patInput.Services[i].ServiceVersions = make([]ServiceChoice, len(patFile.Services[i].ServiceVersions))
			patInput.Services[i].DataVerify = patFile.Services[i].DataVerify
			patInput.Services[i].NodeH = patFile.Services[i].NodeH
			for j := range patFile.Services[i].ServiceVersions {
				patInput.Services[i].ServiceVersions[j].Version = patFile.Services[i].ServiceVersions[j].Version
				patInput.Services[i].ServiceVersions[j].Priority = patFile.Services[i].ServiceVersions[j].Priority
				patInput.Services[i].ServiceVersions[j].Upgrade = patFile.Services[i].ServiceVersions[j].Upgrade

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
					fmt.Printf("Signing deployment_overrides field in service %d, serviceVersion number %d\n", i+1, j+1)
					deployment, err = json.Marshal(depOver)
					if err != nil {
						cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal deployment_overrides field in service %d, serviceVersion number %d: %v", i+1, j+1, err)
					}
					patInput.Services[i].ServiceVersions[j].DeploymentOverrides = string(deployment)
					// We know we need to sign the overrides, so make sure a real key file was provided.
					if keyFilePath == "" {
						cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "must specify --private-key-file so that the deployment_overrides can be signed")
					}
					patInput.Services[i].ServiceVersions[j].DeploymentOverridesSignature, err = sign.Input(keyFilePath, deployment)
					if err != nil {
						cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "problem signing the deployment_overrides string with %s: %v", keyFilePath, err)
					}
				}
			}
		}
	} else if patFile.Workloads != nil && len(patFile.Workloads) > 0 {
		patInput.Workloads = make([]WorkloadReference, len(patFile.Workloads))
		for i := range patFile.Workloads {
			patInput.Workloads[i].WorkloadURL = patFile.Workloads[i].WorkloadURL
			patInput.Workloads[i].WorkloadOrg = patFile.Workloads[i].WorkloadOrg
			patInput.Workloads[i].WorkloadArch = patFile.Workloads[i].WorkloadArch
			patInput.Workloads[i].WorkloadVersions = make([]WorkloadChoice, len(patFile.Workloads[i].WorkloadVersions))
			patInput.Workloads[i].DataVerify = patFile.Workloads[i].DataVerify
			patInput.Workloads[i].NodeH = patFile.Workloads[i].NodeH
			for j := range patFile.Workloads[i].WorkloadVersions {
				patInput.Workloads[i].WorkloadVersions[j].Version = patFile.Workloads[i].WorkloadVersions[j].Version
				patInput.Workloads[i].WorkloadVersions[j].Priority = patFile.Workloads[i].WorkloadVersions[j].Priority
				patInput.Workloads[i].WorkloadVersions[j].Upgrade = patFile.Workloads[i].WorkloadVersions[j].Upgrade

				var err error
				var deployment []byte
				depOver := ConvertToDeploymentOverrides(patFile.Workloads[i].WorkloadVersions[j].DeploymentOverrides)
				// If the input deployment overrides are already in string form and signed, then use them as is.
				if patFile.Workloads[i].WorkloadVersions[j].DeploymentOverrides != nil && reflect.TypeOf(patFile.Workloads[i].WorkloadVersions[j].DeploymentOverrides).String() == "string" && patFile.Workloads[i].WorkloadVersions[j].DeploymentOverridesSignature != "" {
					patInput.Workloads[i].WorkloadVersions[j].DeploymentOverrides = patFile.Workloads[i].WorkloadVersions[j].DeploymentOverrides.(string)
					patInput.Workloads[i].WorkloadVersions[j].DeploymentOverridesSignature = patFile.Workloads[i].WorkloadVersions[j].DeploymentOverridesSignature
				} else if depOver == nil {
					// If the input deployment override is an object that is nil, then there are no overrides, so no signing necessary.
					patInput.Workloads[i].WorkloadVersions[j].DeploymentOverrides = ""
					patInput.Workloads[i].WorkloadVersions[j].DeploymentOverridesSignature = ""
				} else {
					fmt.Printf("Signing deployment_overrides field in workload %d, workloadVersion number %d\n", i+1, j+1)
					deployment, err = json.Marshal(depOver)
					if err != nil {
						cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal deployment_overrides field in workload %d, workloadVersion number %d: %v", i+1, j+1, err)
					}
					patInput.Workloads[i].WorkloadVersions[j].DeploymentOverrides = string(deployment)
					// We know we need to sign the overrides, so make sure a real key file was provided.
					if keyFilePath == "" {
						cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "must specify --private-key-file so that the deployment_overrides can be signed")
					}
					patInput.Workloads[i].WorkloadVersions[j].DeploymentOverridesSignature, err = sign.Input(keyFilePath, deployment)
					if err != nil {
						cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "problem signing the deployment_overrides string with %s: %v", keyFilePath, err)
					}
				}
			}
		}
	}

	// Create or update resource in the exchange
	var exchId string
	if patName != "" {
		exchId = patName
	} else {
		// Use the json file base name as the default for the pattern name
		exchId = filepath.Base(jsonFilePath)                      // remove the leading path
		exchId = strings.TrimSuffix(exchId, filepath.Ext(exchId)) // strip suffix if there
	}
	var output string
	httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/patterns/"+exchId, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
	if httpCode == 200 {
		// Pattern exists, update it
		fmt.Printf("Updating %s in the exchange...\n", exchId)
		cliutils.ExchangePutPost(http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+org+"/patterns/"+exchId, cliutils.OrgAndCreds(org, userPw), []int{201}, patInput)
	} else {
		// Pattern not there, create it
		fmt.Printf("Creating %s in the exchange...\n", exchId)
		cliutils.ExchangePutPost(http.MethodPost, cliutils.GetExchangeUrl(), "orgs/"+org+"/patterns/"+exchId, cliutils.OrgAndCreds(org, userPw), []int{201}, patInput)
	}

	// Store the public key in the exchange, if they gave it to us
	if pubKeyFilePath != "" {
		// Note: the CLI framework already verified the file exists
		bodyBytes := cliutils.ReadFile(pubKeyFilePath)
		baseName := filepath.Base(pubKeyFilePath)
		fmt.Printf("Storing %s with the pattern in the exchange...\n", baseName)
		cliutils.ExchangePutPost(http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+org+"/patterns/"+exchId+"/keys/"+baseName, cliutils.OrgAndCreds(org, userPw), []int{201}, bodyBytes)
	}
}

func PatternVerify(org, userPw, pattern, keyFilePath string) {
	cliutils.SetWhetherUsingApiKey(userPw)
	org, pattern = cliutils.TrimOrg(org, pattern)
	// Get pattern resource from exchange
	var output ExchangePatterns
	httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/patterns/"+pattern, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, "pattern '%s' not found in org %s", pattern, org)
	}
	pat, ok := output.Patterns[org+"/"+pattern]
	if !ok {
		cliutils.Fatal(cliutils.INTERNAL_ERROR, "key '%s' not found in resources returned from exchange", org+"/"+pattern)
	}

	// Loop thru services array, checking the deployment string signature
	someInvalid := false
	for i := range pat.Services {
		for j := range pat.Services[i].ServiceVersions {
			cliutils.Verbose("verifying deployment_overrides string in service %d, serviceVersion number %d", i+1, j+1)
			if pat.Services[i].ServiceVersions[j].DeploymentOverrides == "" && pat.Services[i].ServiceVersions[j].DeploymentOverridesSignature == "" {
				continue // there was nothing to sign, so nothing to verify
			}
			verified, err := verify.Input(keyFilePath, pat.Services[i].ServiceVersions[j].DeploymentOverridesSignature, []byte(pat.Services[i].ServiceVersions[j].DeploymentOverrides))
			if err != nil {
				cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "problem verifying deployment_overrides string in service %d, serviceVersion number %d with %s: %v", i+1, j+1, keyFilePath, err)
			} else if !verified {
				fmt.Printf("Deployment_overrides string in service %d, serviceVersion number %d was not signed with the private key associated with this public key.\n", i+1, j+1)
				someInvalid = true
			}
			// else if they all turned out to be valid, we will tell them that at the end
		}
	}
	for i := range pat.Workloads {
		for j := range pat.Workloads[i].WorkloadVersions {
			cliutils.Verbose("verifying deployment_overrides string in workload %d, workloadVersion number %d", i+1, j+1)
			if pat.Workloads[i].WorkloadVersions[j].DeploymentOverrides == "" && pat.Workloads[i].WorkloadVersions[j].DeploymentOverridesSignature == "" {
				continue // there was nothing to sign, so nothing to verify
			}
			verified, err := verify.Input(keyFilePath, pat.Workloads[i].WorkloadVersions[j].DeploymentOverridesSignature, []byte(pat.Workloads[i].WorkloadVersions[j].DeploymentOverrides))
			if err != nil {
				cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "problem verifying deployment_overrides string in workload %d, workloadVersion number %d with %s: %v", i+1, j+1, keyFilePath, err)
			} else if !verified {
				fmt.Printf("Deployment_overrides string in workload %d, workloadVersion number %d was not signed with the private key associated with this public key.\n", i+1, j+1)
				someInvalid = true
			}
			// else if they all turned out to be valid, we will tell them that at the end
		}
	}

	if someInvalid {
		os.Exit(cliutils.SIGNATURE_INVALID)
	} else {
		fmt.Println("All signatures verified")
	}
}

func PatternRemove(org, userPw, pattern string, force bool) {
	cliutils.SetWhetherUsingApiKey(userPw)
	org, pattern = cliutils.TrimOrg(org, pattern)
	if !force {
		cliutils.ConfirmRemove("Are you sure you want to remove pattern '" + org + "/" + pattern + "' from the Horizon Exchange?")
	}

	httpCode := cliutils.ExchangeDelete(cliutils.GetExchangeUrl(), "orgs/"+org+"/patterns/"+pattern, cliutils.OrgAndCreds(org, userPw), []int{204, 404})
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, "pattern '%s' not found in org %s", pattern, org)
	}
}

/*
func copyPatternOutputToInput(output *PatternOutput, input *PatternInput) {
	input.Label = output.Label
	input.Description = output.Description
	input.Public = output.Public
	input.AgreementProtocols = output.AgreementProtocols
	//input.Workloads = output.Workloads
}
*/

// PatternAddWorkload reads json for 1 element of the workloads array of a pattern, gets the named pattern from the
// exchange, and then either replaces that workload array element (if it already exists), or adds it.
func PatternAddWorkload(org, userPw, pattern, workloadFilePath, keyFilePath, pubKeyFilePath string) {
	cliutils.SetWhetherUsingApiKey(userPw)
	org, pattern = cliutils.TrimOrg(org, pattern)
	// Read in the workload metadata
	newBytes := cliutils.ReadJsonFile(workloadFilePath)
	var workFile WorkloadReferenceFile
	err := json.Unmarshal(newBytes, &workFile)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to unmarshal json input file %s: %v", workloadFilePath, err)
	}

	// Check that the critical values in the workload are not empty
	if workFile.WorkloadOrg == "" || workFile.WorkloadURL == "" || workFile.WorkloadArch == "" || len(workFile.WorkloadVersions) == 0 {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "the workloadOrgid, workloadUrl, workloadArch, or workloadVersions field can not be empty.")
	}
	for _, wv := range workFile.WorkloadVersions {
		if wv.Version == "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "none of the workloadVersions.version fields can be.")
		}
	}

	// Get the pattern from the exchange
	var output ExchangePatterns
	cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/patterns/"+pattern, cliutils.OrgAndCreds(org, userPw), []int{200}, &output)
	key := org + "/" + pattern
	if _, ok := output.Patterns[key]; !ok {
		cliutils.Fatal(cliutils.INTERNAL_ERROR, "horizon exchange api pattern output did not include '%s' key", pattern)
	}
	// Convert it to the structure to put it back into the exchange
	patInput := PatternInput{Label: output.Patterns[key].Label, Description: output.Patterns[key].Description, Public: output.Patterns[key].Public, Workloads: output.Patterns[key].Workloads, AgreementProtocols: output.Patterns[key].AgreementProtocols}

	// Make a copy of the workload, ready for input to the exchange, add sign it
	var workInput WorkloadReference
	workInput.WorkloadURL = workFile.WorkloadURL
	workInput.WorkloadOrg = workFile.WorkloadOrg
	workInput.WorkloadArch = workFile.WorkloadArch
	workInput.WorkloadVersions = make([]WorkloadChoice, len(workFile.WorkloadVersions))
	workInput.DataVerify = workFile.DataVerify
	workInput.NodeH = workFile.NodeH
	for i := range workFile.WorkloadVersions {
		cliutils.Verbose("signing deployment_overrides string in workloadVersion element number %d", i+1)
		workInput.WorkloadVersions[i].Version = workFile.WorkloadVersions[i].Version
		workInput.WorkloadVersions[i].Priority = workFile.WorkloadVersions[i].Priority
		workInput.WorkloadVersions[i].Upgrade = workFile.WorkloadVersions[i].Upgrade

		var err error
		var deployment []byte
		depOver := ConvertToDeploymentOverrides(workFile.WorkloadVersions[i].DeploymentOverrides)
		// If the input deployment overrides are already in string form and signed, then use them as is.
		if workFile.WorkloadVersions[i].DeploymentOverrides != nil && reflect.TypeOf(workFile.WorkloadVersions[i].DeploymentOverrides).String() == "string" && workFile.WorkloadVersions[i].DeploymentOverridesSignature != "" {
			workInput.WorkloadVersions[i].DeploymentOverrides = workFile.WorkloadVersions[i].DeploymentOverrides.(string)
			workInput.WorkloadVersions[i].DeploymentOverridesSignature = workFile.WorkloadVersions[i].DeploymentOverridesSignature
		} else if depOver == nil {
			// If the input deployment override is an object that is nil, then there are no overrides.
			workInput.WorkloadVersions[i].DeploymentOverrides = ""
			workInput.WorkloadVersions[i].DeploymentOverridesSignature = ""
		} else {
			deployment, err = json.Marshal(depOver)
			if err != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal deployment_overrides field in workloadVersion element number %d: %v", i+1, err)
			}
			workInput.WorkloadVersions[i].DeploymentOverrides = string(deployment)
			// We know we need to sign the overrides, so make sure a real key file was provided.
			if keyFilePath == "" {
				cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "must specify --private-key-file so that the deployment_overrides can be signed")
			}
			workInput.WorkloadVersions[i].DeploymentOverridesSignature, err = sign.Input(keyFilePath, deployment)
			if err != nil {
				cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "problem signing the deployment_overrides string with %s: %v", keyFilePath, err)
			}
		}
	}

	// Find the workload entry in the pattern that matches the 1 being added (if any)
	foundMatch := false
	for i := range patInput.Workloads {
		if patInput.Workloads[i].WorkloadOrg == workInput.WorkloadOrg && patInput.Workloads[i].WorkloadURL == workInput.WorkloadURL && patInput.Workloads[i].WorkloadArch == workInput.WorkloadArch {
			// Found it, replace this entry
			fmt.Printf("Replacing workload element number %d\n", i+1)
			patInput.Workloads[i] = workInput
			foundMatch = true
		}
	}
	if !foundMatch {
		// Didn't find a matching element above, so append it
		fmt.Println("Adding workload to the end of the workload array")
		patInput.Workloads = append(patInput.Workloads, workInput)
	}

	// Finally put it back in the exchange
	fmt.Printf("Updating %s in the exchange...\n", pattern)
	cliutils.ExchangePutPost(http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+org+"/patterns/"+pattern, cliutils.OrgAndCreds(org, userPw), []int{201}, patInput)

	// Store the public key in the exchange, if they gave it to us
	if pubKeyFilePath != "" {
		// Note: the CLI framework already verified the file exists
		bodyBytes := cliutils.ReadFile(pubKeyFilePath)
		baseName := filepath.Base(pubKeyFilePath)
		fmt.Printf("Storing %s with the pattern in the exchange...\n", baseName)
		cliutils.ExchangePutPost(http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+org+"/patterns/"+pattern+"/keys/"+baseName, cliutils.OrgAndCreds(org, userPw), []int{201}, bodyBytes)
	}
}

func PatternDelWorkload(org, userPw, pattern, workloadOrg, workloadUrl, workloadArch string) {
	cliutils.SetWhetherUsingApiKey(userPw)
	org, pattern = cliutils.TrimOrg(org, pattern)
	// Get the pattern from the exchange
	var output ExchangePatterns
	cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/patterns/"+pattern, cliutils.OrgAndCreds(org, userPw), []int{200}, &output)
	key := org + "/" + pattern
	if _, ok := output.Patterns[key]; !ok {
		cliutils.Fatal(cliutils.INTERNAL_ERROR, "horizon exchange api pattern output did not include '%s' key", pattern)
	}
	// Convert it to the structure to put it back into the exchange
	patInput := PatternInput{Label: output.Patterns[key].Label, Description: output.Patterns[key].Description, Public: output.Patterns[key].Public, Workloads: output.Patterns[key].Workloads, AgreementProtocols: output.Patterns[key].AgreementProtocols}

	// Find the workload entry in the pattern
	matchIndex := -1
	for i := range patInput.Workloads {
		if patInput.Workloads[i].WorkloadOrg == workloadOrg && patInput.Workloads[i].WorkloadURL == workloadUrl && patInput.Workloads[i].WorkloadArch == workloadArch {
			// Found it, record which one
			matchIndex = i
		}
	}

	// Delete it if we found it
	if matchIndex >= 0 {
		fmt.Printf("Deleting workload element number %d\n", matchIndex+1)
		patInput.Workloads = append(patInput.Workloads[:matchIndex], patInput.Workloads[matchIndex+1:]...)
	} else {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "did not find the specified workload in the pattern")
	}

	// Finally put it back in the exchange
	fmt.Printf("Updating %s in the exchange...\n", pattern)
	cliutils.ExchangePutPost(http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+org+"/patterns/"+pattern, cliutils.OrgAndCreds(org, userPw), []int{201}, patInput)
}

func PatternListKey(org, userPw, pattern, keyName string) {
	cliutils.SetWhetherUsingApiKey(userPw)
	org, pattern = cliutils.TrimOrg(org, pattern)
	if keyName == "" {
		// Only display the names
		var output string
		cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/patterns/"+pattern+"/keys", cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
		fmt.Printf("%s\n", output)
	} else {
		// Display the content of the key
		var output []byte
		httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/patterns/"+pattern+"/keys/"+keyName, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
		if httpCode == 404 && pattern != "" {
			cliutils.Fatal(cliutils.NOT_FOUND, "key '%s' not found", keyName)
		}
		fmt.Printf("%s", string(output))
	}
}

func PatternRemoveKey(org, userPw, pattern, keyName string) {
	cliutils.SetWhetherUsingApiKey(userPw)
	org, pattern = cliutils.TrimOrg(org, pattern)
	httpCode := cliutils.ExchangeDelete(cliutils.GetExchangeUrl(), "orgs/"+org+"/patterns/"+pattern+"/keys/"+keyName, cliutils.OrgAndCreds(org, userPw), []int{204, 404})
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, "key '%s' not found", keyName)
	}
}
