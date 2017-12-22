package exchange

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/rsapss-tool/sign"
	"net/http"
	"strings"
)

// We only care about the workload names, so the rest is left as interface{}
type ExchangeWorkloads struct {
	Workloads  map[string]WorkloadOutput `json:"workloads"`
	LastIndex int                    `json:"lastIndex"`
}

//todo: the only thing keeping me from using exchange.WorkloadDefinition is dave adding the Public field to it
type WorkloadOutput struct {
	Owner       string               `json:"owner"`
	Label       string               `json:"label"`
	Description string               `json:"description"`
	Public   bool               `json:"public"`
	WorkloadURL string               `json:"workloadUrl"`
	Version     string               `json:"version"`
	Arch        string               `json:"arch"`
	DownloadURL string               `json:"downloadUrl"`
	APISpecs    []exchange.APISpec            `json:"apiSpec"`
	UserInputs  []exchange.UserInput          `json:"userInput"`
	Workloads   []exchange.WorkloadDeployment `json:"workloads"`
	LastUpdated string               `json:"lastUpdated"`
}

type WorkloadInput struct {
	Label       string               `json:"label"`
	Description string               `json:"description"`
	Public   bool               `json:"public"`
	WorkloadURL string               `json:"workloadUrl"`
	Version     string               `json:"version"`
	Arch        string               `json:"arch"`
	DownloadURL string               `json:"downloadUrl"`
	APISpecs    []exchange.APISpec            `json:"apiSpec"`
	UserInputs  []exchange.UserInput          `json:"userInput"`
	Workloads   []exchange.WorkloadDeployment `json:"workloads"`
}

func WorkloadList(org string, userPw string, workload string, namesOnly bool) {
	if workload != "" {
		workload = "/" + workload
	}
	if namesOnly {
		// Only display the names
		var resp ExchangeWorkloads
		cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/workloads"+workload, cliutils.OrgAndCreds(org,userPw), []int{200,404}, &resp)
		var workloads []string
		for k := range resp.Workloads {
			workloads = append(workloads, k)
		}
		jsonBytes, err := json.MarshalIndent(workloads, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn exchange workload list' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		// Display the full resources
		//var output string
		var output ExchangeWorkloads
		httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/workloads"+workload, cliutils.OrgAndCreds(org,userPw), []int{200,404}, &output)
		if httpCode == 404 && workload != "" {
			cliutils.Fatal(cliutils.NOT_FOUND, "workload '%s' not found in org %s", strings.TrimPrefix(workload, "/"), org)
		}
		jsonBytes, err := json.MarshalIndent(output, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn exchange workload list' output: %v", err)
		}
		fmt.Println(string(jsonBytes))
	}
}


// WorkloadPublish signs the MS def and puts it in the exchange
func WorkloadPublish(org string, userPw string, jsonFilePath string, keyFilePath string) {
	// Read in the workload metadata
	newBytes := cliutils.ReadJsonFile(jsonFilePath)
	var workInput WorkloadInput
	err := json.Unmarshal(newBytes, &workInput)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to unmarshal json input file %s: %v", jsonFilePath, err)
	}

	// Loop thru the workloads array and sign the deployment strings
	fmt.Println("Signing workload...")
	for i := range workInput.Workloads {
		cliutils.Verbose("signing deployment string %d", i+1)
		var err error
		workInput.Workloads[i].DeploymentSignature, err = sign.Input(keyFilePath, []byte(workInput.Workloads[i].Deployment))
		if err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "problem signing the deployment string with %s: %v", keyFilePath, err)
		}
		//todo: gather the docker image paths to instruct to docker push at the end

		CheckTorrentField(workInput.Workloads[i].Torrent, i)
	}

	// Create of update resource in the exchange
	exchId := cliutils.FormExchangeId(workInput.WorkloadURL, workInput.Version, workInput.Arch)
	var output string
	httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/workloads/"+exchId, cliutils.OrgAndCreds(org,userPw), []int{200,404}, &output)
	if httpCode == 200 {
		// Workload exists, update it
		fmt.Printf("Updating %s in the exchange...\n", exchId)
		cliutils.ExchangePutPost(http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+org+"/workloads/"+exchId, cliutils.OrgAndCreds(org,userPw), []int{201}, workInput)
	} else {
		// Workload not there, create it
		fmt.Printf("Creating %s in the exchange...\n", exchId)
		cliutils.ExchangePutPost(http.MethodPost, cliutils.GetExchangeUrl(), "orgs/"+org+"/workloads", cliutils.OrgAndCreds(org,userPw), []int{201}, workInput)
	}
}
