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
	"strings"
)

//todo: only using these instead of exchange.GetPatternResponse because exchange.Pattern is missing the Owner and LastUpdated fields
type ExchangePatterns struct {
	Patterns  map[string]PatternOutput `json:"patterns"`
	LastIndex int                      `json:"lastIndex"`
}

type PatternOutput struct {
	Owner       string `json:"owner"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Public      bool   `json:"public"`
	//Workloads          []exchange.WorkloadReference `json:"workloads"`
	Workloads          []WorkloadReference          `json:"workloads"`
	AgreementProtocols []exchange.AgreementProtocol `json:"agreementProtocols"`
	LastUpdated        string                       `json:"lastUpdated"`
}

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
type PatternInput struct {
	Label              string                       `json:"label"`
	Description        string                       `json:"description"`
	Public             bool                         `json:"public"`
	Workloads          []WorkloadReference          `json:"workloads"`
	AgreementProtocols []exchange.AgreementProtocol `json:"agreementProtocols"`
}

func PatternList(org string, userPw string, pattern string, namesOnly bool) {
	if pattern != "" {
		pattern = "/" + pattern
	}
	if namesOnly {
		// Only display the names
		var resp ExchangePatterns
		cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/patterns"+pattern, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &resp)
		var patterns []string
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
		//var output string
		var output ExchangePatterns
		httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/patterns"+pattern, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
		if httpCode == 404 && pattern != "" {
			cliutils.Fatal(cliutils.NOT_FOUND, "pattern '%s' not found in org %s", strings.TrimPrefix(pattern, "/"), org)
		}
		jsonBytes, err := json.MarshalIndent(output, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn exchange pattern list' output: %v", err)
		}
		fmt.Println(string(jsonBytes))
	}
}

// PatternPublish signs the MS def and puts it in the exchange
func PatternPublish(org string, userPw string, jsonFilePath string, keyFilePath string) {
	// Read in the pattern metadata
	newBytes := cliutils.ReadJsonFile(jsonFilePath)
	var patInput PatternInput
	err := json.Unmarshal(newBytes, &patInput)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to unmarshal json input file %s: %v", jsonFilePath, err)
	}

	// Loop thru the workloads array and the workloadVersions array and sign the deployment_overrides strings
	fmt.Println("Signing pattern...")
	for i := range patInput.Workloads {
		for j := range patInput.Workloads[i].WorkloadVersions {
			cliutils.Verbose("signing deployment_overrides string in workload %d, workloadVersion number %d", i+1, j+1)
			var err error
			patInput.Workloads[i].WorkloadVersions[j].DeploymentOverridesSignature, err = sign.Input(keyFilePath, []byte(patInput.Workloads[i].WorkloadVersions[j].DeploymentOverrides))
			if err != nil {
				cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "problem signing the deployment_overrides string with %s: %v", keyFilePath, err)
			}
		}
	}

	// Create of update resource in the exchange
	exchId := filepath.Base(jsonFilePath)                     // remove the leading path
	exchId = strings.TrimSuffix(exchId, filepath.Ext(exchId)) // strip suffix if there
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
}

func PatternVerify(org, userPw, pattern, keyFilePath string) {
	// Get pattern resource from exchange
	var output ExchangePatterns
	httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/patterns/"+pattern, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, "pattern '%s' not found in org %s", pattern, org)
	}

	// Loop thru workloads array, checking the deployment string signature
	pat, ok := output.Patterns[org+"/"+pattern]
	if !ok {
		cliutils.Fatal(cliutils.INTERNAL_ERROR, "key '%s' not found in resources returned from exchange", org+"/"+pattern)
	}
	someInvalid := false
	for i := range pat.Workloads {
		for j := range pat.Workloads[i].WorkloadVersions {
			cliutils.Verbose("verifying deployment_overrides string in workload %d, workloadVersion number %d", i+1, j+1)
			//pat.Workloads[i].WorkloadVersions[j].DeploymentOverridesSignature, err = sign.Input(keyFilePath, []byte(pat.Workloads[i].WorkloadVersions[j].DeploymentOverrides))
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
func PatternAddWorkload(org string, userPw string, pattern string, workloadFilePath string, keyFilePath string) {
	// Read in the workload metadata
	newBytes := cliutils.ReadJsonFile(workloadFilePath)
	var workInput WorkloadReference
	err := json.Unmarshal(newBytes, &workInput)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to unmarshal json input file %s: %v", workloadFilePath, err)
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

	// Sign the workload being added
	for i := range workInput.WorkloadVersions {
		cliutils.Verbose("signing deployment_overrides string in workloadVersion element number %d", i+1)
		var err error
		workInput.WorkloadVersions[i].DeploymentOverridesSignature, err = sign.Input(keyFilePath, []byte(workInput.WorkloadVersions[i].DeploymentOverrides))
		if err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "problem signing the deployment_overrides string with %s: %v", keyFilePath, err)
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
}
