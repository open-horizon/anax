package dev

import (
	"errors"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	cliexchange "github.com/open-horizon/anax/cli/exchange"
	"github.com/open-horizon/anax/cli/plugin_registry"
	"github.com/open-horizon/anax/exchange"
	"path"
)

const SERVICE_DEFINITION_FILE = "service.definition.json"

const DEFAULT_SDEF_SPECIFIC_VERSION = "specific_version_number"
const DEFAULT_SDEF_URL = ""

// Sort of like a constructor, it creates an in memory object except that it is created from the service definition config
// file in the current project. This function assumes the caller has determined the exact location of the file.
func GetServiceDefinition(directory string, name string) (*cliexchange.ServiceFile, error) {

	res := new(cliexchange.ServiceFile)

	// GetFile will write to the res object, demarshalling the bytes into a json object that can be returned.
	if err := GetFile(directory, name, res); err != nil {
		return nil, err
	}
	return res, nil

}

// Sort of like a constructor, it creates a service definition config object and writes it to the project
// in the file system.
func CreateServiceDefinition(directory string, specRef string, imageInfo map[string]string, deploymentType string) error {

	// Create a skeletal service definition config object with fillins/place-holders for configuration.
	res := new(cliexchange.ServiceFile)

	res.Org = "$HZN_ORG_ID"
	res.Label = "$SERVICE_NAME for $ARCH"
	res.Public = true
	res.URL = "$SERVICE_NAME"
	res.Version = "$SERVICE_VERSION"
	res.Arch = "$ARCH"
	res.Description = ""
	res.Sharable = exchange.MS_SHARING_MODE_MULTIPLE
	if specRef == "" {
		res.UserInputs = []exchange.UserInput{
			exchange.UserInput{
				Name:         "",
				Label:        "",
				Type:         "",
				DefaultValue: "",
			},
		}
	} else {
		res.UserInputs = []exchange.UserInput{
			exchange.UserInput{
				Name:         "HW_WHO",
				Label:        "Who to say hello to",
				Type:         "string",
				DefaultValue: "World",
			},
		}
	}
	res.RequiredServices = []exchange.ServiceDependency{}

	// Use the deployment plugin registry to obtain the default deployment config map.
	if plugin_registry.DeploymentConfigPlugins.HasPlugin(deploymentType) {
		if deploymentType == "native" {
			res.Deployment = plugin_registry.DeploymentConfigPlugins.Get(deploymentType).DefaultConfig(imageInfo)
		} else {
			res.Deployment = plugin_registry.DeploymentConfigPlugins.Get(deploymentType).DefaultConfig(nil)
		}
	} else {
		return errors.New(fmt.Sprintf("unknown deployment type: %v", deploymentType))
	}

	res.DeploymentSignature = ""
	res.ImageStore = map[string]interface{}{}

	// Convert the object to JSON and write it into the project.
	return CreateFile(directory, SERVICE_DEFINITION_FILE, res)

}

// Check for the existence of the service definition config file in the project.
func ServiceDefinitionExists(directory string) (bool, error) {
	return FileExists(directory, SERVICE_DEFINITION_FILE)
}

// Validate that the service definition file is complete and coherent with the rest of the definitions in the project.
// If the file is not valid the reason will be returned in the error.
func ValidateServiceDefinition(directory string, fileName string) error {

	sDef, mserr := GetServiceDefinition(directory, fileName)
	if mserr != nil {
		return mserr
	}

	filePath := path.Join(directory, fileName)
	if sDef.URL == DEFAULT_SDEF_URL || sDef.URL == "" {
		return errors.New(fmt.Sprintf("%v: URL must be set.", filePath))
	} else if sDef.Version == DEFAULT_SDEF_SPECIFIC_VERSION || sDef.Version == "" {
		return errors.New(fmt.Sprintf("%v: version must be set to a specific version, e.g. 1.0.0.", filePath))
	} else if sDef.Org == "" {
		return errors.New(fmt.Sprintf("%v: org must be set.", filePath))
	} else {
		if err := plugin_registry.DeploymentConfigPlugins.ValidatedByOne(sDef.Deployment); err != nil {
			return errors.New(fmt.Sprintf("%v: deployment configuration, %v", filePath, err))
		}
		for ix, ui := range sDef.UserInputs {
			if (ui.Name != "" && ui.Type == "") || (ui.Name == "" && (ui.Type != "" || ui.DefaultValue != "")) {
				return errors.New(fmt.Sprintf("%v: userInput array index %v does not have name and type specified.", filePath, ix))
			}
		}
	}
	return nil
}

// Refresh the RequiredServices dependencies in the definition. This is called when new dependencies are added or removed.
func RefreshServiceDependencies(homeDirectory string, newDepDef cliexchange.AbstractServiceFile) error {

	// Update the service definition dependencies.
	serviceDef, err := GetServiceDefinition(homeDirectory, SERVICE_DEFINITION_FILE)
	if err != nil {
		return err
	}

	found := false
	for _, dep := range serviceDef.RequiredServices {
		if dep.URL == newDepDef.GetURL() {
			found = true
			break
		}
	}

	// If the dependency is already present, no need to add it.
	if !found {
		newSD := exchange.ServiceDependency{
			URL:     newDepDef.GetURL(),
			Org:     newDepDef.GetOrg(),
			Version: newDepDef.GetVersion(),
			Arch:    newDepDef.GetArch(),
		}
		serviceDef.RequiredServices = append(serviceDef.RequiredServices, newSD)

		// Harden the updated service definition.
		if err := CreateFile(homeDirectory, SERVICE_DEFINITION_FILE, serviceDef); err != nil {
			return err
		}

		cliutils.Verbose("Updated %v/%v dependencies.", homeDirectory, SERVICE_DEFINITION_FILE)
	} else {
		cliutils.Verbose("No need to update %v/%v dependencies.", homeDirectory, SERVICE_DEFINITION_FILE)
	}

	return nil
}

func RemoveServiceDependency(homeDirectory string, theDepDef cliexchange.AbstractServiceFile) error {

	// Update the service definition dependencies.
	serviceDef, err := GetServiceDefinition(homeDirectory, SERVICE_DEFINITION_FILE)
	if err != nil {
		return err
	}

	for ix, dep := range serviceDef.RequiredServices {
		if dep.URL == theDepDef.GetURL() {
			serviceDef.RequiredServices = append(serviceDef.RequiredServices[:ix], serviceDef.RequiredServices[ix+1:]...)

			// Harden the updated service definition.
			if err := CreateFile(homeDirectory, SERVICE_DEFINITION_FILE, serviceDef); err != nil {
				return err
			}

			cliutils.Verbose("Updated %v/%v dependencies.", homeDirectory, SERVICE_DEFINITION_FILE)
			return nil
		}
	}

	cliutils.Verbose("No need to update %v/%v dependencies.", homeDirectory, SERVICE_DEFINITION_FILE)
	return nil
}
