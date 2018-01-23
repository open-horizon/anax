package dev

import (
	"errors"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	cliexchange "github.com/open-horizon/anax/cli/exchange"
	"github.com/open-horizon/anax/containermessage"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"path"
	"strings"
)

const WORKLOAD_DEFINITION_FILE = "workload.definition.json"

const DEFAULT_WLDEF_SPECIFIC_VERSION = "specific_version_number"
const DEFAULT_WLDEF_URL = ""

// Sort of like a constructor, it creates an in memory object except that it is created from the workload definition config
// file in the current project. This function assumes the caller has determined the exact location of the file.
func GetWorkloadDefinition(directory string) (*cliexchange.WorkloadFile, error) {

	res := new(cliexchange.WorkloadFile)

	// GetFile will write to the res object, demarshalling the bytes into a json object that can be returned.
	if err := GetFile(directory, WORKLOAD_DEFINITION_FILE, res); err != nil {
		return nil, err
	}
	return res, nil

}

// Sort of like a constructor, it creates a skeletal workload definition config object and writes it to the project
// in the file system.
func CreateWorkloadDefinition(directory string, org string) error {

	// Create a skeletal workload definition config object with fillins/place-holders for configuration.
	res := new(cliexchange.WorkloadFile)
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
	} else if len(workloadDef.Workloads) == 0 {
		return errors.New(fmt.Sprintf("%v: must contain at least 1 workload deployment configuration.", filePath))
	} else {
		for ix, wl := range workloadDef.Workloads {
			if len(wl.Deployment.Services) == 0 {
				return errors.New(fmt.Sprintf("%v: workloads array index %v must contain at least 1 deployment configuration.", filePath, ix))
			} else if err := wl.Deployment.CanStartStop(); err != nil {
				return errors.New(fmt.Sprintf("%v: Workloads index %v %v", filePath, ix, err))
			}
		}
		for ix, ui := range workloadDef.UserInputs {
			if (ui.Name != "" && ui.Type == "") || (ui.Name == "" && (ui.Type != "" || ui.DefaultValue != "")) {
				return errors.New(fmt.Sprintf("%v: userInput array index %v does not have name and type specified.", filePath, ix))
			}
		}
	}
	return nil
}

// Refresh the APISpec list dependencies in the definition. This is called when new dependencies are aded or removed.
func RefreshWorkloadDependencies(homeDirectory string, deps Dependencies) error {
	// Update the workload definition dependencies to make sure the dependency is included. The APISpec array
	// in the workload definition is rebuilt from the dependencies.
	workloadDef, err := GetWorkloadDefinition(homeDirectory)
	if err != nil {
		return err
	}

	// Get the current set of dependencies if not provided on input.
	if deps == nil {
		var err error
		deps, err = GetDependencies(homeDirectory)
		if err != nil {
			return err
		}
	}

	// Start with an empty array and then rebuild it.
	workloadDef.APISpecs = make([]exchange.APISpec, 0, 10)

	for id, dep := range deps {
		org := strings.Split(id, "/")[0]
		newAPISpec := exchange.APISpec{
			SpecRef: dep.SpecRef,
			Org:     org,
			Version: dep.Version,
			Arch:    dep.Arch,
		}
		workloadDef.APISpecs = append(workloadDef.APISpecs, newAPISpec)
	}

	// Harden the updated APISpec dependency list.
	if err := CreateFile(homeDirectory, WORKLOAD_DEFINITION_FILE, workloadDef); err != nil {
		return err
	}

	cliutils.Verbose("Updated %v/%v dependencies.", homeDirectory, WORKLOAD_DEFINITION_FILE)

	return nil
}
