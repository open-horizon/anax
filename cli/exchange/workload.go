package exchange

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/containermessage"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/rsapss-tool/sign"
	"github.com/open-horizon/rsapss-tool/verify"
	"net/http"
	"os"
	"strings"
)

// This is used when reading json file the user gives us as input to create the workload struct
type WorkloadFile struct {
	Org         string               `json:"org"` // optional
	Label       string               `json:"label"`
	Description string               `json:"description"`
	Public      bool                 `json:"public"`
	WorkloadURL string               `json:"workloadUrl"`
	Version     string               `json:"version"`
	Arch        string               `json:"arch"`
	DownloadURL string               `json:"downloadUrl"`
	APISpecs    []exchange.APISpec   `json:"apiSpec"`
	UserInputs  []exchange.UserInput `json:"userInput"`
	Workloads   []WorkloadDeployment `json:"workloads"`
}

// Returns true if the workload definition userinputs define the variable.
func (w *WorkloadFile) DefinesVariable(name string) string {
	for _, ui := range w.UserInputs {
		if ui.Name == name && ui.Type != "" {
			return ui.Type
		}
	}
	return ""
}

// Convert the first Deployment Configuration to a full Deployment Description.
func (self *WorkloadFile) ConvertToDeploymentDescription() (*DeploymentConfig, *containermessage.DeploymentDescription, error) {
	for _, wl := range self.Workloads {
		return &wl.Deployment, &containermessage.DeploymentDescription{
			Services: wl.Deployment.Services,
			ServicePattern: containermessage.Pattern{
				Shared: map[string][]string{},
			},
			Infrastructure: true,
			Overrides:      map[string]*containermessage.Service{},
		}, nil
	}
	return nil, nil, errors.New(fmt.Sprintf("has no containers to execute"))
}

// Verify that non default user inputs are set in the input map.
func (self *WorkloadFile) RequiredVariablesAreSet(setVars map[string]interface{}) error {
	for _, ui := range self.UserInputs {
		if ui.DefaultValue == "" && ui.Name != "" {
			if _, ok := setVars[ui.Name]; !ok {
				return errors.New(fmt.Sprintf("user input %v has no default value and is not set", ui.Name))
			}
		}
	}
	return nil
}

// This is used as the input to the exchange to create the workload
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
	cliutils.SetWhetherUsingApiKey(userPw)
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
	cliutils.SetWhetherUsingApiKey(userPw)
	// Read in the workload metadata
	newBytes := cliutils.ReadJsonFile(jsonFilePath)
	var workFile WorkloadFile
	err := json.Unmarshal(newBytes, &workFile)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to unmarshal json input file %s: %v", jsonFilePath, err)
	}
	if workFile.Org != "" && workFile.Org != org {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "the org specified in the input file (%s) must match the org specified on the command line (%s)", workFile.Org, org)
	}
	workFile.SignAndPublish(org, userPw, keyFilePath)

}

// Sign and publish the workload definition. This is a function that is reusable across different hzn commands.
func (workFile *WorkloadFile) SignAndPublish(org, userPw, keyFilePath string) {
	workInput := WorkloadInput{Label: workFile.Label, Description: workFile.Description, Public: workFile.Public, WorkloadURL: workFile.WorkloadURL, Version: workFile.Version, Arch: workFile.Arch, DownloadURL: workFile.DownloadURL, APISpecs: workFile.APISpecs, UserInputs: workFile.UserInputs, Workloads: make([]exchange.WorkloadDeployment, len(workFile.Workloads))}

	// Loop thru the workloads array and sign the deployment strings
	fmt.Println("Signing workload...")
	var imageList []string
	for i := range workFile.Workloads {
		cliutils.Verbose("signing deployment string %d", i+1)
		workInput.Workloads[i].Torrent = workFile.Workloads[i].Torrent
		var err error
		var deployment []byte
		deployment, err = json.Marshal(workFile.Workloads[i].Deployment)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal deployment string %d: %v", i+1, err)
		}
		workInput.Workloads[i].Deployment = string(deployment)
		workInput.Workloads[i].DeploymentSignature, err = sign.Input(keyFilePath, deployment)
		if err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "problem signing deployment string %d with %s: %v", i+1, keyFilePath, err)
		}

		// Gather the docker image paths to instruct to docker push at the end
		imageList = AppendImagesFromDeploymentField(workFile.Workloads[i].Deployment, imageList)

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

	// Tell the user to push the images to the docker registry
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
	cliutils.SetWhetherUsingApiKey(userPw)
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
	cliutils.SetWhetherUsingApiKey(userPw)
	if !force {
		cliutils.ConfirmRemove("Are you sure you want to remove workload '" + org + "/" + workload + "' from the Horizon Exchange?")
	}

	httpCode := cliutils.ExchangeDelete(cliutils.GetExchangeUrl(), "orgs/"+org+"/workloads/"+workload, cliutils.OrgAndCreds(org, userPw), []int{204, 404})
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, "workload '%s' not found in org %s", workload, org)
	}
}
