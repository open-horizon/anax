package dev

import (
	"errors"
	"fmt"
	cliexchange "github.com/open-horizon/anax/cli/exchange"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"path"
)

const WORKLOAD_DEFINITION_FILE = "workload.definition.json"

const DEFAULT_WLDEF_SPECIFIC_VERSION = "specific_version_number"
const DEFAULT_WLDEF_URL = ""

// Sort of like a constructor, it creates an in memory object except that it is created from the workload definition config
// file in the current project. This function assumes the caller has determined the exact location of the file.
func GetWorkloadDefinition(directory string) (*cliexchange.WorkloadInput, error) {

	res := new(cliexchange.WorkloadInput)

	// GetFile will write to the res object, demarshalling the bytes into a json object that can be returned.
	if err := GetFile(directory, WORKLOAD_DEFINITION_FILE, res); err != nil {
		return nil, err
	}
	return res, nil

}

// Sort of like a constructor, it creates a skeletal workload definition config object and writes it to the project
// in the file system.
func CreateWorkloadDefinition(directory string) error {

	// Create a skeletal workload definition config object with fillins/place-holders for configuration.
	res := new(cliexchange.WorkloadInput)
	res.Label = ""
	res.Description = ""
	res.WorkloadURL = DEFAULT_WLDEF_URL
	res.DownloadURL = "not used yet"
	res.Version = DEFAULT_WLDEF_SPECIFIC_VERSION
	res.Arch = cutil.ArchString()
	res.UserInputs = []exchange.UserInput{
		exchange.UserInput{
			Name:         "",
			Label:        "",
			Type:         "",
			DefaultValue: "",
		},
	}
	res.APISpecs = []exchange.APISpec{}
	res.Workloads = []exchange.WorkloadDeployment{}

	// Convert the object to JSON and write it into the project.
	return CreateFile(directory, WORKLOAD_DEFINITION_FILE, res)

}

// Check for the existence of the workload definition config file in the project.
func WorkloadDefinitionExists(directory string) (bool, error) {
	return FileExists(directory, WORKLOAD_DEFINITION_FILE)
}

// Validate that the workload definition file is complete and coherent with the rest of the definitions in the project.
// If the file is not valid the reason will be returned in the error.
func ValidateWorkloadDefinition(directory string) error {

	workloadDef, wderr := GetWorkloadDefinition(directory)
	if wderr != nil {
		return wderr
	}

	filePath := path.Join(directory, WORKLOAD_DEFINITION_FILE)
	if workloadDef.WorkloadURL == DEFAULT_WLDEF_URL || workloadDef.WorkloadURL == "" {
		return errors.New(fmt.Sprintf("%v: workloadUrl must be set.", filePath))
	} else if workloadDef.Version == DEFAULT_WLDEF_SPECIFIC_VERSION || workloadDef.Version == "" {
		return errors.New(fmt.Sprintf("%v: version must be set to a specific version, e.g. 1.0.0.", filePath))
	}
	return nil
}
