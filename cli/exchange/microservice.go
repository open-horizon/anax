package exchange

import (
	"encoding/json"
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

type MicroserviceInput struct {
	Label         string                        `json:"label"`
	Description   string                        `json:"description"`
	Public        bool                          `json:"public"`
	SpecRef       string                        `json:"specRef"`
	Version       string                        `json:"version"`
	Arch          string                        `json:"arch"`
	Sharable      string                        `json:"sharable"`
	DownloadURL   string                        `json:"downloadUrl"`
	MatchHardware map[string]string             `json:"matchHardware"`
	UserInputs    []exchange.UserInput          `json:"userInput"`
	Workloads     []exchange.WorkloadDeployment `json:"workloads"`
}

// Returns true if the microservice definition userinputs define the variable.
func (w *MicroserviceInput) DefinesVariable(name string) string {
	for _, ui := range w.UserInputs {
		if ui.Name == name && ui.Type != "" {
			return ui.Type
		}
	}
	return ""
}

func MicroserviceList(org string, userPw string, microservice string, namesOnly bool) {
	if microservice != "" {
		microservice = "/" + microservice
	}
	if namesOnly && microservice == "" {
		// Only display the names
		var resp exchange.GetMicroservicesResponse
		cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/microservices"+microservice, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &resp)
		microservices := []string{}
		for k := range resp.Microservices {
			microservices = append(microservices, k)
		}
		jsonBytes, err := json.MarshalIndent(microservices, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn exchange microservice list' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		// Display the full resources
		//var output string
		var output exchange.GetMicroservicesResponse
		httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/microservices"+microservice, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
		if httpCode == 404 && microservice != "" {
			cliutils.Fatal(cliutils.NOT_FOUND, "microservice '%s' not found in org %s", strings.TrimPrefix(microservice, "/"), org)
		}
		jsonBytes, err := json.MarshalIndent(output, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn exchange microservice list' output: %v", err)
		}
		fmt.Println(string(jsonBytes))
	}
}

type DeploymentConfig struct {
	Services map[string]containermessage.Service `json:"services"`
}

func AppendImagesFromDeploymentField(deployment string, deploymentNum int, imageList []string) []string {
	// The deployment string should include: "deployment": "{"services":{"cpu2wiotp":{"image":"openhorizon/example_wl_x86_cpu2wiotp:1.1.2",...}}}"
	var deploymentStr DeploymentConfig
	if err := json.Unmarshal([]byte(deployment), &deploymentStr); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "failed to unmarshal deployment string number %d: %v", deploymentNum+1, err)
	}
	for _, s := range deploymentStr.Services {
		if s.Image != "" {
			imageList = append(imageList, s.Image)
		}
	}
	return imageList
}

func CheckTorrentField(torrent string, index int) {
	// Verify the torrent field is the form necessary for the containers that are stored in a docker registry (because that is all we support from hzn right now)
	torrentErrorString := `currently the torrent field must either be empty or be like this to indicate the images are stored in a docker registry: {\"url\":\"\",\"signature\":\"\"}`
	if torrent == "" {
		//cliutils.Fatal(cliutils.CLI_INPUT_ERROR, torrentErrorString)
		return
	}
	var torrentMap map[string]string
	if err := json.Unmarshal([]byte(torrent), &torrentMap); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "failed to unmarshal torrent string number %d: %v", index+1, err)
	}
	if url, ok := torrentMap["url"]; !ok || url != "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, torrentErrorString)
	}
	if signature, ok := torrentMap["signature"]; !ok || signature != "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, torrentErrorString)
	}
}

// MicroservicePublish signs the MS def and puts it in the exchange
func MicroservicePublish(org string, userPw string, jsonFilePath string, keyFilePath string) {
	// Read in the MS metadata
	newBytes := cliutils.ReadJsonFile(jsonFilePath)
	var microInput MicroserviceInput
	err := json.Unmarshal(newBytes, &microInput)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to unmarshal json input file %s: %v", jsonFilePath, err)
	}

	// Loop thru the workloads array and sign the deployment strings
	fmt.Println("Signing microservice...")
	var imageList []string
	for i := range microInput.Workloads {
		cliutils.Verbose("signing deployment string %d", i+1)
		var err error
		microInput.Workloads[i].DeploymentSignature, err = sign.Input(keyFilePath, []byte(microInput.Workloads[i].Deployment))
		if err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "problem signing the deployment string with %s: %v", keyFilePath, err)
		}
		// Gather the docker image paths to instruct to docker push at the end
		imageList = AppendImagesFromDeploymentField(microInput.Workloads[i].Deployment, i+1, imageList)

		CheckTorrentField(microInput.Workloads[i].Torrent, i)
	}

	// Create of update resource in the exchange
	exchId := cliutils.FormExchangeId(microInput.SpecRef, microInput.Version, microInput.Arch)
	var output string
	httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/microservices/"+exchId, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
	if httpCode == 200 {
		// MS exists, update it
		fmt.Printf("Updating %s in the exchange...\n", exchId)
		cliutils.ExchangePutPost(http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+org+"/microservices/"+exchId, cliutils.OrgAndCreds(org, userPw), []int{201}, microInput)
	} else {
		// MS not there, create it
		fmt.Printf("Creating %s in the exchange...\n", exchId)
		cliutils.ExchangePutPost(http.MethodPost, cliutils.GetExchangeUrl(), "orgs/"+org+"/microservices", cliutils.OrgAndCreds(org, userPw), []int{201}, microInput)
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

// MicroserviceVerify verifies the deployment strings of the specified microservice resource in the exchange.
func MicroserviceVerify(org, userPw, microservice, keyFilePath string) {
	// Get microservice resource from exchange
	var output exchange.GetMicroservicesResponse
	httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/microservices/"+microservice, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &output)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, "microservice '%s' not found in org %s", microservice, org)
	}

	// Loop thru microservices array, checking the deployment string signature
	micro, ok := output.Microservices[org+"/"+microservice]
	if !ok {
		cliutils.Fatal(cliutils.INTERNAL_ERROR, "key '%s' not found in resources returned from exchange", org+"/"+microservice)
	}
	someInvalid := false
	for i := range micro.Workloads {
		cliutils.Verbose("verifying deployment string %d", i+1)
		verified, err := verify.Input(keyFilePath, micro.Workloads[i].DeploymentSignature, []byte(micro.Workloads[i].Deployment))
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

func MicroserviceRemove(org, userPw, microservice string, force bool) {
	if !force {
		cliutils.ConfirmRemove("Are you sure you want to remove microservice '" + org + "/" + microservice + "' from the Horizon Exchange?")
	}

	httpCode := cliutils.ExchangeDelete(cliutils.GetExchangeUrl(), "orgs/"+org+"/microservices/"+microservice, cliutils.OrgAndCreds(org, userPw), []int{204, 404})
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, "microservice '%s' not found in org %s", microservice, org)
	}
}
