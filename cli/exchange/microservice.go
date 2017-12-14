package exchange

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"net/http"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/rsapss-tool/sign"
)

type ExchangeMicroservices struct {
	Microservices  map[string]MicroserviceOutput `json:"microservices"`
	LastIndex int                    `json:"lastIndex"`
}

//todo: the only thing keeping me from using exchange.MicroserviceDefinition is dave adding the Public field to it
type MicroserviceOutput struct {
	Owner         string               `json:"owner"`
	Label         string               `json:"label"`
	Description   string               `json:"description"`
	Public   bool               `json:"public"`
	SpecRef       string               `json:"specRef"`
	Version       string               `json:"version"`
	Arch          string               `json:"arch"`
	Sharable      string               `json:"sharable"`
	DownloadURL   string               `json:"downloadUrl"`
	MatchHardware map[string]string        `json:"matchHardware"`
	UserInputs    []exchange.UserInput          `json:"userInput"`
	Workloads     []exchange.WorkloadDeployment `json:"workloads"`
	LastUpdated   string               `json:"lastUpdated"`
}

type MicroserviceInput struct {
	Label         string               `json:"label"`
	Description   string               `json:"description"`
	Public   bool               `json:"public"`
	SpecRef       string               `json:"specRef"`
	Version       string               `json:"version"`
	Arch          string               `json:"arch"`
	Sharable      string               `json:"sharable"`
	DownloadURL   string               `json:"downloadUrl"`
	MatchHardware map[string]string        `json:"matchHardware"`
	UserInputs    []exchange.UserInput          `json:"userInput"`
	Workloads     []exchange.WorkloadDeployment `json:"workloads"`
}

func MicroserviceList(org string, userPw string, microservice string, namesOnly bool) {
	if microservice != "" {
		microservice = "/" + microservice
	}
	if namesOnly {
		// Only display the names
		var resp ExchangeMicroservices
		cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/microservices"+microservice, org+"/"+userPw, []int{200}, &resp)
		var microservices []string
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
		var output ExchangeMicroservices
		cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/microservices"+microservice, org+"/"+userPw, []int{200}, &output)
		jsonBytes, err := json.MarshalIndent(output, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'hzn exchange microservice list' output: %v", err)
		}
		fmt.Println(string(jsonBytes))
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
	for i := range microInput.Workloads {
		cliutils.Verbose("signing deployment string %d", i+1)
		var err error
		microInput.Workloads[i].DeploymentSignature, err = sign.Input(keyFilePath, []byte(microInput.Workloads[i].Deployment))
		if err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "problem signing the deployment string with %s: %v", keyFilePath, err)
		}

		// Verify the torrent field is the form necessary for the containers that are stored in a docker registry (because that is all we support right now)
		torrentErrorString := `currently the torrent field must be like this to indicate the images are stored in a docker registry: {\"url\":\"\",\"signature\":\"\"}`
		if microInput.Workloads[i].Torrent == "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, torrentErrorString)
		}
		var torrentMap map[string]string
		if err := json.Unmarshal([]byte(microInput.Workloads[i].Torrent), torrentMap); err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "failed to unmarshal torrent string number %d: %v", i+1, err)
		}
		if url, ok := torrentMap["url"]; !ok || url != "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, torrentErrorString)
		}
		if signature, ok := torrentMap["signature"]; !ok || signature != "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, torrentErrorString)
		}
	}

	// Create of update resource in the exchange
	exchId := cliutils.FormExchangeId(microInput.SpecRef, microInput.Version, microInput.Arch)
	var output string
	httpCode := cliutils.ExchangeGet(cliutils.GetExchangeUrl(), "orgs/"+org+"/microservices/"+exchId, org+"/"+userPw, []int{200,404}, &output)
	if httpCode == 200 {
		// MS exists, update it
		fmt.Printf("Updating %s in the exchange...\n", exchId)
		cliutils.ExchangePutPost(http.MethodPut, cliutils.GetExchangeUrl(), "orgs/"+org+"/microservices/"+exchId, org+"/"+userPw, []int{201}, microInput)
	} else {
		// MS not there, create it
		fmt.Printf("Creating %s in the exchange...\n", exchId)
		cliutils.ExchangePutPost(http.MethodPost, cliutils.GetExchangeUrl(), "orgs/"+org+"/microservices", org+"/"+userPw, []int{201}, microInput)
	}
}
