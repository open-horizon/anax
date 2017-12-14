package exchange

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/rsapss-tool/sign"
	"net/http"
	"path/filepath"
	"strings"
)

//todo: only using these instead of exchange.GetPatternResponse because exchange.Pattern is missing the Owner and LastUpdated fields
type ExchangePatterns struct {
	Patterns  map[string]PatternOutput `json:"patterns"`
	LastIndex int                    `json:"lastIndex"`
}

type PatternOutput struct {
	Owner         string               `json:"owner"`
	Label              string              `json:"label"`
	Description        string              `json:"description"`
	Public             bool                `json:"public"`
	Workloads          []exchange.WorkloadReference `json:"workloads"`
	AgreementProtocols []exchange.AgreementProtocol `json:"agreementProtocols"`
	LastUpdated   string               `json:"lastUpdated"`
}

//todo: can't use exchange.Pattern (and some sub-structs) because it has omitempty on several fields required by the exchange
type WorkloadChoice struct {
	Version                      string           `json:"version"`  // the version of the workload
	Priority                     exchange.WorkloadPriority `json:"priority"` // the highest priority workload is tried first for an agreement, if it fails, the next priority is tried. Priority 1 is the highest, priority 2 is next, etc.
	Upgrade                      exchange.UpgradePolicy    `json:"upgradePolicy"`
	DeploymentOverrides          string           `json:"deployment_overrides"`           // env var overrides for the workload
	DeploymentOverridesSignature string           `json:"deployment_overrides_signature"` // signature of env var overrides
}
type WorkloadReference struct {
	WorkloadURL      string           `json:"workloadUrl"`      // refers to a workload definition in the exchange
	WorkloadOrg      string           `json:"workloadOrgid"`    // the org holding the workload definition
	WorkloadArch     string           `json:"workloadArch"`     // the hardware architecture of the workload definition
	WorkloadVersions []WorkloadChoice `json:"workloadVersions"` // a list of workload version for rollback
	DataVerify       exchange.DataVerification `json:"dataVerification"`           // policy for verifying that the node is sending data
	NodeH            exchange.NodeHealth       `json:"nodeHealth"`                 // policy for determining when a node's health is violating its agreements
}
type PatternInput struct {
	Label              string              `json:"label"`
	Description        string              `json:"description"`
	Public             bool                `json:"public"`
	Workloads          []WorkloadReference `json:"workloads"`
	AgreementProtocols []exchange.AgreementProtocol `json:"agreementProtocols"`
}

func PatternList(org string, userPw string, pattern string, namesOnly bool) {
	if pattern != "" {
		pattern = "/" + pattern
	}
	if namesOnly {
		// Only display the names
		var resp ExchangePatterns
		cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/patterns"+pattern, org+"/"+userPw, []int{200}, &resp)
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
		cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/patterns"+pattern, org+"/"+userPw, []int{200}, &output)
		jsonBytes, err := json.MarshalIndent(output, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn exchange pattern list' output: %v", err)
		}
		fmt.Println(string(jsonBytes))
	}
}


// PatternPublish signs the MS def and puts it in the exchange
func PatternPublish(org string, userPw string, jsonFilePath string, keyFilePath string) {
	// Read in the MS metadata
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
	exchId := filepath.Base(jsonFilePath)	// remove the leading path
	exchId = strings.TrimSuffix(exchId, filepath.Ext(exchId))   // strip suffix if there
	var output string
	httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/patterns/"+exchId, org+"/"+userPw, []int{200,404}, &output)
	if httpCode == 200 {
		// Pattern exists, update it
		fmt.Printf("Updating %s in the exchange...\n", exchId)
		cliutils.ExchangePutPost(http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+org+"/patterns/"+exchId, org+"/"+userPw, []int{201}, patInput)
	} else {
		// Pattern not there, create it
		fmt.Printf("Creating %s in the exchange...\n", exchId)
		cliutils.ExchangePutPost(http.MethodPost, cliutils.GetExchangeUrl(), "orgs/"+org+"/patterns/"+exchId, org+"/"+userPw, []int{201}, patInput)
	}
}
