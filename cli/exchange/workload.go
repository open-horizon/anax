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
	"reflect"
	"strings"
	"path/filepath"
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
func (wf *WorkloadFile) DefinesVariable(name string) string {
	for _, ui := range wf.UserInputs {
		if ui.Name == name && ui.Type != "" {
			return ui.Type
		}
	}
	return ""
}

// Convert the first Deployment Configuration to a full Deployment Description.
func (wf *WorkloadFile) ConvertToDeploymentDescription() (*DeploymentConfig, *containermessage.DeploymentDescription, error) {
	for _, wl := range wf.Workloads {
		depConfig := ConvertToDeploymentConfig(wl.Deployment)
		return depConfig, &containermessage.DeploymentDescription{
			Services: depConfig.Services,
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
func (wf *WorkloadFile) RequiredVariablesAreSet(setVars map[string]interface{}) error {
	for _, ui := range wf.UserInputs {
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
func WorkloadPublish(org, userPw, jsonFilePath, keyFilePath, pubKeyFilePath string) {
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
	exchId := workFile.SignAndPublish(org, userPw, keyFilePath)

	// Store the public key in the exchange, if they gave it to us
	if pubKeyFilePath != "" {
		// Note: the CLI framework already verified the file exists
		bodyBytes := cliutils.ReadFile(pubKeyFilePath)
		baseName := filepath.Base(pubKeyFilePath)
		fmt.Printf("Storing %s with the workload in the exchange...\n", baseName)
		cliutils.ExchangePutPost(http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+org+"/workloads/"+exchId+"/keys/"+baseName, cliutils.OrgAndCreds(org, userPw), []int{201}, bodyBytes)
	}
}

// Sign and publish the workload definition. This is a function that is reusable across different hzn commands.
func (wf *WorkloadFile) SignAndPublish(org, userPw, keyFilePath string) (exchId string) {
	workInput := WorkloadInput{Label: wf.Label, Description: wf.Description, Public: wf.Public, WorkloadURL: wf.WorkloadURL, Version: wf.Version, Arch: wf.Arch, DownloadURL: wf.DownloadURL, APISpecs: wf.APISpecs, UserInputs: wf.UserInputs, Workloads: make([]exchange.WorkloadDeployment, len(wf.Workloads))}

	// Loop thru the workloads array and sign the deployment strings
	fmt.Println("Signing workload...")
	var imageList []string
	if len(wf.Workloads) > 1 {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "the 'workloads' array can not have more than 1 element in it")
	}
	for i := range wf.Workloads {
		var err error
		var deployment []byte
		depConfig := ConvertToDeploymentConfig(wf.Workloads[i].Deployment)
		if wf.Workloads[i].Deployment != nil && reflect.TypeOf(wf.Workloads[i].Deployment).String() == "string" && wf.Workloads[i].DeploymentSignature != "" {
			workInput.Workloads[i].Deployment = wf.Workloads[i].Deployment.(string)
			workInput.Workloads[i].DeploymentSignature = wf.Workloads[i].DeploymentSignature
		} else if depConfig == nil {
			workInput.Workloads[i].Deployment = ""
			workInput.Workloads[i].DeploymentSignature = ""
		} else {
			cliutils.Verbose("signing deployment string %d", i+1)
			deployment, err = json.Marshal(depConfig)
			if err != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal deployment string %d: %v", i+1, err)
			}
			workInput.Workloads[i].Deployment = string(deployment)
			// We know we need to sign the deployment config, so make sure a real key file was provided.
			if keyFilePath == "" {
				cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "must specify --private-key-file so that the deployment string can be signed")
			}
			workInput.Workloads[i].DeploymentSignature, err = sign.Input(keyFilePath, deployment)
			if err != nil {
				cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "problem signing deployment string %d with %s: %v", i+1, keyFilePath, err)
			}
		}

		workInput.Workloads[i].Torrent = wf.Workloads[i].Torrent

		// Gather the docker image paths to instruct the user to docker push at the end
		imageList = AppendImagesFromDeploymentField(depConfig, imageList)

		CheckTorrentField(workInput.Workloads[i].Torrent, i)
	}

	// Create or update resource in the exchange
	exchId = cliutils.FormExchangeId(workInput.WorkloadURL, workInput.Version, workInput.Arch)
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
	return
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

func WorkloadListKey(org, userPw, workload, keyName string) {
	cliutils.SetWhetherUsingApiKey(userPw)
	if keyName == "" {
		// Only display the names
		var output string
		cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/workloads/"+workload+"/keys", cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
		fmt.Printf("%s\n", output)
	} else {
		// Display the content of the key
		var output []byte
		httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/workloads/"+workload+"/keys/"+keyName, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
		if httpCode == 404 && workload != "" {
			cliutils.Fatal(cliutils.NOT_FOUND, "key '%s' not found", keyName)
		}
		fmt.Printf("%s", string(output))
	}
}

func WorkloadRemoveKey(org, userPw, workload, keyName string) {
	cliutils.SetWhetherUsingApiKey(userPw)
	httpCode := cliutils.ExchangeDelete(cliutils.GetExchangeUrl(), "orgs/"+org+"/workloads/"+workload+"/keys/"+keyName, cliutils.OrgAndCreds(org, userPw), []int{204, 404})
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, "key '%s' not found", keyName)
	}
}
