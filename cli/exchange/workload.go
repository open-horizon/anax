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
	"strings"
)

type WorkloadInput struct {
	Label       string                        `json:"label"`
	Description string                        `json:"description"`
	Public      bool                          `json:"public"`
	WorkloadURL string                        `json:"workloadUrl"`
	Version     string                        `json:"version"`
	Arch        string                        `json:"arch"`
	DownloadURL string                        `json:"downloadUrl"`
	APISpecs    []exchange.APISpec            `json:"apiSpec"`
	UserInputs  []exchange.UserInput          `json:"userInput"`
	Workloads   []exchange.WorkloadDeployment `json:"workloads"`
}

func WorkloadList(org, userPw, workload string, namesOnly bool) {
	if workload != "" {
		workload = "/" + workload
	}
	if namesOnly && workload == "" {
		// Only display the names
		var resp exchange.GetWorkloadsResponse
		cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/workloads"+workload, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &resp)
		workloads := []string{}
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
		var output exchange.GetWorkloadsResponse
		httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/workloads"+workload, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
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
func WorkloadPublish(org, userPw, jsonFilePath, keyFilePath string) {
	// Read in the workload metadata
	newBytes := cliutils.ReadJsonFile(jsonFilePath)
	var workInput WorkloadInput
	err := json.Unmarshal(newBytes, &workInput)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to unmarshal json input file %s: %v", jsonFilePath, err)
	}

	// Loop thru the workloads array and sign the deployment strings
	fmt.Println("Signing workload...")
	var imageList []string
	for i := range workInput.Workloads {
		cliutils.Verbose("signing deployment string %d", i+1)
		var err error
		workInput.Workloads[i].DeploymentSignature, err = sign.Input(keyFilePath, []byte(workInput.Workloads[i].Deployment))
		if err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "problem signing deployment string %d with %s: %v", i+1, keyFilePath, err)
		}

		// Gather the docker image paths to instruct to docker push at the end
		imageList = AppendImagesFromDeploymentField(workInput.Workloads[i].Deployment, i+1, imageList)

		CheckTorrentField(workInput.Workloads[i].Torrent, i)
	}

	// Create or update resource in the exchange
	exchId := cliutils.FormExchangeId(workInput.WorkloadURL, workInput.Version, workInput.Arch)
	var output string
	httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/workloads/"+exchId, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
	if httpCode == 200 {
		// Workload exists, update it
		fmt.Printf("Updating %s in the exchange...\n", exchId)
		cliutils.ExchangePutPost(http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+org+"/workloads/"+exchId, cliutils.OrgAndCreds(org, userPw), []int{201}, workInput)
	} else {
		// Workload not there, create it
		fmt.Printf("Creating %s in the exchange...\n", exchId)
		cliutils.ExchangePutPost(http.MethodPost, cliutils.GetExchangeUrl(), "orgs/"+org+"/workloads", cliutils.OrgAndCreds(org, userPw), []int{201}, workInput)
	}

	// Tell the to push the images to the docker registry
	if len(imageList) > 0 {
		//todo: should we just push the docker images for them?
		fmt.Println("If you haven't already, push your docker images to the registry:")
		for _, image := range imageList {
			fmt.Printf("  docker push %s\n", image)
		}
	}
}

// WorkloadVerify verifies the deployment strings of the specified workload resource in the exchange.
func WorkloadVerify(org, userPw, workload, keyFilePath string) {
	// Get workload resource from exchange
	var output exchange.GetWorkloadsResponse
	httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/workloads/"+workload, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, "workload '%s' not found in org %s", workload, org)
	}

	// Loop thru workloads array, checking the deployment string signature
	work, ok := output.Workloads[org+"/"+workload]
	if !ok {
		cliutils.Fatal(cliutils.INTERNAL_ERROR, "key '%s' not found in resources returned from exchange", org+"/"+workload)
	}
	someInvalid := false
	for i := range work.Workloads {
		cliutils.Verbose("verifying deployment string %d", i+1)
		verified, err := verify.Input(keyFilePath, work.Workloads[i].DeploymentSignature, []byte(work.Workloads[i].Deployment))
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

func WorkloadRemove(org, userPw, workload string, force bool) {
	if !force {
		cliutils.ConfirmRemove("Are you sure you want to remove workload '" + org + "/" + workload + "' from the Horizon Exchange?")
	}

	httpCode := cliutils.ExchangeDelete(cliutils.GetExchangeUrl(), "orgs/"+org+"/workloads/"+workload, cliutils.OrgAndCreds(org, userPw), []int{204, 404})
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, "workload '%s' not found in org %s", workload, org)
	}
}
