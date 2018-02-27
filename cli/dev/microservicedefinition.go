package dev

import (
	"errors"
	"fmt"
	cliexchange "github.com/open-horizon/anax/cli/exchange"
	"github.com/open-horizon/anax/containermessage"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"path"
)

const MICROSERVICE_DEFINITION_FILE = "microservice.definition.json"

const DEFAULT_MSDEF_SPECIFIC_VERSION = "specific_version_number"
const DEFAULT_MSDEF_URL = ""

// Sort of like a constructor, it creates an in memory object except that it is created from the microservice definition config
// file in the current project. This function assumes the caller has determined the exact location of the file.
func GetMicroserviceDefinition(directory string, name string) (*cliexchange.MicroserviceFile, error) {

	res := new(cliexchange.MicroserviceFile)

	// GetFile will write to the res object, demarshalling the bytes into a json object that can be returned.
	if err := GetFile(directory, name, res); err != nil {
		return nil, err
	}
	return res, nil

}

// Sort of like a constructor, it creates a skeletal microservice definition config object and writes it to the project
// in the file system.
func CreateMicroserviceDefinition(directory string, org string) error {

	// Create a skeletal microservice definition config object with fillins/place-holders for configuration.
	res := new(cliexchange.MicroserviceFile)
	res.Label = ""
	res.Description = ""
	res.SpecRef = DEFAULT_MSDEF_URL
	res.DownloadURL = "not used yet"
	res.Version = DEFAULT_MSDEF_SPECIFIC_VERSION
	res.Arch = cutil.ArchString()
	res.Sharable = exchange.MS_SHARING_MODE_MULTIPLE
	res.UserInputs = []exchange.UserInput{
		exchange.UserInput{
			Name:         "",
			Label:        "",
			Type:         "",
			DefaultValue: "",
		},
	}
	res.MatchHardware = map[string]string{}
	res.Workloads = []cliexchange.WorkloadDeployment{
		cliexchange.WorkloadDeployment{
			Deployment: cliexchange.DeploymentConfig{
				Services: map[string]*containermessage.Service{
					"": &containermessage.Service{
						Image:       "",
						Environment: []string{"ENV_VAR_HERE=SOME_VALUE"},
					},
				},
			},
			DeploymentSignature: "",
			Torrent:             "",
		},
	}
	res.Org = org

	// Convert the object to JSON and write it into the project.
	return CreateFile(directory, MICROSERVICE_DEFINITION_FILE, res)

}

// Check for the existence of the microservice definition config file in the project.
func MicroserviceDefinitionExists(directory string) (bool, error) {
	return FileExists(directory, MICROSERVICE_DEFINITION_FILE)
}

// Validate that the microservice definition file is complete and coherent with the rest of the definitions in the project.
// If the file is not valid the reason will be returned in the error.
func ValidateMicroserviceDefinition(directory string, fileName string) error {

	msDef, mserr := GetMicroserviceDefinition(directory, fileName)
	if mserr != nil {
		return mserr
	}

	filePath := path.Join(directory, fileName)
	if msDef.SpecRef == DEFAULT_MSDEF_URL || msDef.SpecRef == "" {
		return errors.New(fmt.Sprintf("%v: specRef must be set.", filePath))
	} else if msDef.Version == DEFAULT_MSDEF_SPECIFIC_VERSION || msDef.Version == "" {
		return errors.New(fmt.Sprintf("%v: version must be set to a specific version, e.g. 1.0.0.", filePath))
	} else if msDef.Org == "" {
		return errors.New(fmt.Sprintf("%v: org must be set.", filePath))
	} else {
		for ix, wl := range msDef.Workloads {
			depConfig := cliexchange.ConvertToDeploymentConfig(wl.Deployment)
			if err := depConfig.CanStartStop(); err != nil {
				return errors.New(fmt.Sprintf("%v: workloads array element at index %v, %v", filePath, ix, err))
			}
		}
		for ix, ui := range msDef.UserInputs {
			if (ui.Name != "" && ui.Type == "") || (ui.Name == "" && (ui.Type != "" || ui.DefaultValue != "")) {
				return errors.New(fmt.Sprintf("%v: userInput array index %v does not have name and type specified.", filePath, ix))
			}
		}
	}
	return nil
}
