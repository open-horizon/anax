package dev

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	cliexchange "github.com/open-horizon/anax/cli/exchange"
	"github.com/open-horizon/anax/containermessage"
	"github.com/open-horizon/anax/exchange"
	"os"
	"path"
	"path/filepath"
)

const DEPENDENCIES_FILE = "dependency.definition.json"

const DEPENDENCY_COMMAND = "dependency"
const DEPENDENCY_FETCH_COMMAND = "fetch"
const DEPENDENCY_LIST_COMMAND = "list"
const DEPENDENCY_REMOVE_COMMAND = "remove"

// Holds the parameters used to capture the dependency metadata so that the dependency can be refreshed.
type MetadataReference struct {
	Project string `json:"project"`
	SpecRef string `json:"specRef"`
	Version string `json:"version"`
	Org     string `json:"org"`
	Arch    string `json:"arch"`
}

func (m MetadataReference) Validate() error {
	if (m.Project != "") && (m.SpecRef != "" || m.Org != "" || m.Version != "" || m.Arch != "") {
		return errors.New(fmt.Sprintf("can contain project or specRef and org, but not both"))
	} else if (m.Project == "") && m.SpecRef == "" && m.Org == "" {
		return errors.New(fmt.Sprintf("must contain either project or specRef and org"))
	} else if (m.Project == "") && (m.SpecRef == "" || m.Org == "") {
		return errors.New(fmt.Sprintf("must specify specRef and org"))
	}
	return nil
}

// Describes a service dependency.
type Dependency struct {
	SpecRef      string                       `json:"specRef"`
	Version      string                       `json:"version"`
	Arch         string                       `json:"arch"`
	Sharable     string                       `json:"sharable"`
	Global       []GlobalSet                  `json:"global"`
	UserInputs   []exchange.UserInput         `json:"userInput"` // These come from the dependency's definition file, so they might not have a default.
	DeployConfig cliexchange.DeploymentConfig `json:"deployment.config"`
	MetaRef      MetadataReference            `json:"metadata.reference"`
}

func (d Dependency) String() string {
	return fmt.Sprintf("SpecRef: %v, "+
		"Version: %v, "+
		"Arch: %v, "+
		"Sharable: %v, "+
		"Global: %v, "+
		"UserInputs: %v, "+
		"DeployConfig: %v",
		d.SpecRef, d.Version, d.Arch, d.Sharable, d.Global, d.UserInputs, d.DeployConfig)
}

func (d Dependency) ShortString() string {
	return fmt.Sprintf("SpecRef: %v, Version: %v, Arch: %v", d.SpecRef, d.Version, d.Arch)
}

func (d Dependency) ConvertToDeploymentDescription() (*containermessage.DeploymentDescription, error) {
	return &containermessage.DeploymentDescription{
		Services: d.DeployConfig.Services,
		ServicePattern: containermessage.Pattern{
			Shared: map[string][]string{},
		},
		Infrastructure: false,
		Overrides:      map[string]*containermessage.Service{},
	}, nil
}

// All the Horizon dependencies that this project has.
type Dependencies map[string]Dependency

func (d Dependencies) AddNewDependency(id string, newDep *Dependency) error {
	d[id] = *newDep
	return nil
}

// This is the entry point for the hzn dev dependency fetch command.
func DependencyFetch(homeDirectory string, project string, specRef string, org string, version string, arch string, userCreds string) {

	// Check input parameters for correctness.
	dir, err := verifyFetchInput(homeDirectory, project, specRef, org, version, arch, userCreds)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'dependency %v' %v", DEPENDENCY_FETCH_COMMAND, err)
	}

	target := project

	// Go get the dependency metadata.
	if project != "" {
		if err := fetchLocalProjectDependency(dir, project); err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'dependency %v' %v", DEPENDENCY_FETCH_COMMAND, err)
		}
	} else {
		if err := fetchExchangeProjectDependency(dir, specRef, org, version, arch, userCreds); err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'dependency %v' %v", DEPENDENCY_FETCH_COMMAND, err)
		}
		target = fmt.Sprintf("specRef: %v, org: %v", specRef, org)
		if version != "" {
			target += fmt.Sprintf(", version: %v", version)
		}
		if arch != "" {
			target += fmt.Sprintf(", arch: %v", arch)
		}
	}

	fmt.Printf("New dependency on %v created.\n", target)
}

// This is the entry point for the hzn dev dependency list command.
func DependencyList(homeDirectory string) {

	dir, err := setup(homeDirectory, true, false, "")
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", DEPENDENCY_COMMAND, DEPENDENCY_LIST_COMMAND, err)
	}

	deps, err := GetDependencies(dir)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", DEPENDENCY_COMMAND, DEPENDENCY_LIST_COMMAND, err)
	}

	if jsonBytes, err := json.MarshalIndent(deps, "", "    "); err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "'%v %v' to create json object from dependencies, %v", DEPENDENCY_COMMAND, DEPENDENCY_LIST_COMMAND, err)
	} else {
		fmt.Printf("%v", string(jsonBytes))
	}

}

// This is the entry point for the hzn dev dependency remove command.
func DependencyRemove(homeDirectory string, specRef string, version string, arch string) {

	// Check input parameters for correctness.
	dir, err := verifyRemoveInput(homeDirectory, specRef, version, arch)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'dependency %v' %v", DEPENDENCY_REMOVE_COMMAND, err)
	}

	// Grab the dependency object from the filesystem.
	deps, err := GetDependencies(dir)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", DEPENDENCY_COMMAND, DEPENDENCY_REMOVE_COMMAND, err)
	}

	// Make sure we can uniquely identify the dependency to be removed.
	var theDep Dependency
	var theDepKey string
	uniqueDep := true
	for depKey, dep := range deps {
		if dep.SpecRef == specRef && (version == "" || (version != "" && dep.Version == version)) && (arch == "" || (arch != "" && dep.Arch == arch)) {
			if theDep.SpecRef != "" {
				uniqueDep = false
				break
			}
			theDep = dep
			theDepKey = depKey
		}
	}

	// If we did not find the dependency, then return the error. If the input did not uniquely identify the dependency, then return
	// the error. Otherwise remove the dependency.
	if theDep.SpecRef == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'dependency %v' dependency not found.", DEPENDENCY_REMOVE_COMMAND)
	} else if !uniqueDep {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'dependency %v' dependency %v is not unique. Please specify version and/or architecture to uniquely identify the dependency.", DEPENDENCY_REMOVE_COMMAND, specRef)
	} else {
		cliutils.Verbose("Found dependency: %v", theDep)

		// We know which dependency to remove, so remove it.
		delete(deps, theDepKey)

		// Update the workload definition with the new dependencies.
		if err := RefreshWorkloadDependencies(dir, deps); err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'dependency %v' error updating workload definition: %v", DEPENDENCY_REMOVE_COMMAND, err)
		}

		// Write out the new list of dependencies.
		if err := CreateFile(dir, DEPENDENCIES_FILE, deps); err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'dependency %v' error updating dependency file: %v", DEPENDENCY_REMOVE_COMMAND, err)
		}

		cliutils.Verbose("Updated %v/%v for removed dependency.", dir, DEPENDENCIES_FILE)
	}

	// Construct meaningful completion message.
	target := fmt.Sprintf("specRef: %v", specRef)
	if version != "" {
		target += fmt.Sprintf(", version: %v", version)
	}
	if arch != "" {
		target += fmt.Sprintf(", arch: %v", arch)
	}
	fmt.Printf("Removed dependency %v.\n", target)
}

// Sort of like a constructor, it creates an in memory object except that it is created from the dependency config
// file in the current project. This function assumes the caller has determined the exact location of the file.
func GetDependencies(directory string) (Dependencies, error) {

	res := make(Dependencies)

	filePath := path.Join(directory, DEPENDENCIES_FILE)
	fileBytes := cliutils.ReadJsonFile(filePath)

	// We decode this JSON file using a decoder with the UseNumber flag set so that the attribute API code we reuse for parsing
	// the GlobalSet attributes will have the right metadata.
	decoder := json.NewDecoder(bytes.NewReader(fileBytes))
	decoder.UseNumber()

	if err := decoder.Decode(&res); err != nil {
		return nil, errors.New(fmt.Sprintf("unable to demarshal %v file, error: %v", filePath, err))
	}

	return res, nil

}

// Sort of like a constructor, it creates a skeletal dependency config object and writes it to the project
// in the file system.
func CreateDependencies(directory string) error {

	// Create a skeletal dependency config object with fillins/place-holders for configuration.
	res := make(Dependencies)

	// Convert the object to JSON and write it into the project.
	return CreateFile(directory, DEPENDENCIES_FILE, res)

}

// Check for the existence of the dependency config file in the project.
func DependenciesExists(directory string) (bool, error) {
	return FileExists(directory, DEPENDENCIES_FILE)
}

// Validate that the microservice definition file is complete and coherent with the rest of the definitions in the project.
// If the file is not valid the reason will be returned in the error.
func ValidateDependencies(directory string, userInputs *InputFile, userInputsFilePath string) error {

	deps, deperr := GetDependencies(directory)
	if deperr != nil {
		return deperr
	}

	// Loop through the entire dependency map structure and validate each entry.
	for depId, dep := range deps {
		if depId == "" {
			return errors.New(fmt.Sprintf("dependency %v has an empty key", dep))
		} else if err := dep.Validate(userInputs, userInputsFilePath); err != nil {
			return errors.New(fmt.Sprintf("dependency %v has a validation error: %v", depId, err))
		}
	}

	return nil
}

func (d Dependency) Validate(userInputs *InputFile, userInputsFilePath string) error {

	// Validate the tuple info.
	if d.SpecRef == "" {
		return errors.New(fmt.Sprintf("specRef is empty"))
	} else if d.Version == "" {
		return errors.New(fmt.Sprintf("version is empty"))
	} else if d.Arch == "" {
		return errors.New(fmt.Sprintf("arch is empty"))
	}

	// Validate the global section by running it through the attribute converter.
	_, err := GlobalSetAsAttributes(d.Global)
	if err != nil {
		return errors.New(fmt.Sprintf("dependency %v has an error in the global section: %v ", d, err))
	}

	// Userinputs from the dependency without a default value must be set in the userinput file.
	for _, ui := range d.UserInputs {
		if ui.DefaultValue == "" {
			found := false
			for _, msUI := range userInputs.Microservices {
				if d.SpecRef == msUI.Url {
					if _, ok := msUI.Variables[ui.Name]; ok {
						found = true
						break
					}
				}
			}
			if !found {
				return errors.New(fmt.Sprintf("variable %v has no default and must be specified in %v", ui.Name, userInputsFilePath))
			}
		}
	}

	// Validate the inner deployment.config section.
	if err := d.DeployConfig.CanStartStop(); err != nil {
		return errors.New(fmt.Sprintf("dependency %v has an error in the deployment.config: %v", d, err))
	}

	// Validate the metadata reference.
	if err := (&d.MetaRef).Validate(); err != nil {
		return errors.New(fmt.Sprintf("dependency %v has an error in the metadata.reference: %v", d, err))
	}

	return nil
}

func verifyFetchInput(homeDirectory string, project string, specRef string, org string, version string, arch string, userCreds string) (string, error) {

	// Shut off the Anax runtime logging.
	flag.Set("v", "0")

	// Verify that the environment and inputs are usable.
	dir, err := VerifyEnvironment(homeDirectory, true, true, userCreds)
	if err != nil {
		return "", err
	}

	// Valid inputs are either project or the others, but not both. version and arch are optional when specref and org are used.
	if project != "" && (specRef != "" || org != "") {
		return "", errors.New(fmt.Sprintf("-project is mutually exclusive with -specRef and -org."))
	} else if project == "" && specRef == "" && org == "" {
		return "", errors.New(fmt.Sprintf("one of -project or -specRef and -org must be specified."))
	} else if (specRef != "" && org == "") || (specRef == "" && org != "") {
		return "", errors.New(fmt.Sprintf("both -specRef and -org must be specified."))
	}

	// Verify that if -project was specified, it points to a valid horizon project directory.
	if project != "" {
		if !IsMicroserviceProject(project) {
			return "", errors.New(fmt.Sprintf("-project %v does not contain Horizon microservice metadata.", project))
		} else if err := ValidateMicroserviceDefinition(project); err != nil {
			return "", err
		}
	}

	cliutils.Verbose("Reading Horizon metadata from %s", dir)

	return dir, nil
}

func verifyRemoveInput(homeDirectory string, specRef string, version string, arch string) (string, error) {

	// Shut off the Anax runtime logging.
	flag.Set("v", "0")

	// Verify that the environment and inputs are usable.
	dir, err := VerifyEnvironment(homeDirectory, true, false, "")
	if err != nil {
		return "", err
	}

	// Valid inputs are specRef with the others being optional.
	if specRef == "" {
		return "", errors.New(fmt.Sprintf("-specRef is required for remove."))
	}

	cliutils.Verbose("Reading Horizon metadata from %s", dir)

	return dir, nil
}

func fetchLocalProjectDependency(homeDirectory string, project string) error {

	// If the dependency is a local project then we can validate it and extract the project metadata.

	// If the dependent project is not validate-able then we cant reliably use it as a dependency. This function
	// handles the 'hzn dev microservice verify' command so it will exit if there is an error.
	MicroserviceValidate(project, "")

	// Create a new dependency
	newDep := new(Dependency)

	// Pull the metadata from the dependent project.
	// Save the full file path of the pointer to the source of the dependency.
	if absProject, err := filepath.Abs(project); err != nil {
		return err
	} else {
		newDep.MetaRef = MetadataReference{
			Project: absProject,
		}
		cliutils.Verbose("Reading Horizon metadata from dependency: %v", absProject)
	}

	// Get the dependency's definition.
	msDef, err := GetMicroserviceDefinition(project)
	if err != nil {
		return err
	}

	newDep.SpecRef = msDef.SpecRef
	newDep.Version = msDef.Version
	newDep.Arch = msDef.Arch
	newDep.Sharable = msDef.Sharable
	newDep.UserInputs = msDef.UserInputs
	for _, wl := range msDef.Workloads {
		newDep.DeployConfig = wl.Deployment
		break
	}

	// Get the dependency's userinputs to get the global attribute settings and any variable configuration.
	ui, _, err := GetUserInputs(project, "")
	if err != nil {
		return err
	}

	newDep.Global = ui.Global

	cliutils.Verbose("Found dependency %v, Org: %v", newDep.ShortString(), msDef.Org)

	// Harden the new dependency in the file.
	if err := UpdateDependencyFile(homeDirectory, newDep, msDef.Org); err != nil {
		return err
	}

	// Update the workload definition dependencies to make sure the dependency is included. The APISpec array
	// in the workload definition is rebuilt from the dependencies.
	if err := RefreshWorkloadDependencies(homeDirectory, nil); err != nil {
		return err
	}

	// Update this project's userinputs with variable configuration from the dependency's userinputs.
	// Find the configured variables for this dependency.
	var depVarConfig MicroWork
	for _, depUI := range ui.Microservices {
		if depUI.Url == newDep.SpecRef {
			depVarConfig = depUI
			break
		}
	}

	// If there are any user inputs, append them to this project's user inputs.
	if depVarConfig.Url != "" {
		// Get this project's userinputs.
		currentUIs, _, err := GetUserInputs(homeDirectory, "")
		if err != nil {
			return err
		}

		found := false
		for _, currentUI := range currentUIs.Microservices {
			if currentUI.Url == depVarConfig.Url && currentUI.Org == depVarConfig.Org {
				found = true
				break
			}
		}
		if !found {
			currentUIs.Microservices = append(currentUIs.Microservices, depVarConfig)

			if err := CreateFile(homeDirectory, USERINPUT_FILE, currentUIs); err != nil {
				return err
			}

			cliutils.Verbose("Updated %v/%v with the dependency's variable configuration.", homeDirectory, USERINPUT_FILE)
		}
	}
	return nil
}

func fetchExchangeProjectDependency(homeDirectory string, specRef string, org string, version string, arch string, userCreds string) error {

	// Pull the metadata from the exchange.

	// Construct the resource URL suffix.
	resSuffix := fmt.Sprintf("orgs/%v/microservices?specRef=%v", org, specRef)
	if version != "" {
		resSuffix += fmt.Sprintf("&version=%v", version)
	}
	if arch != "" {
		resSuffix += fmt.Sprintf("&arch=%v", arch)
	}

	// Create an object to hold the response.
	resp := new(exchange.GetMicroservicesResponse)

	// Call the exchange to get the microservice definition.
	if userCreds == "" {
		userCreds = os.Getenv(DEVTOOL_HZN_USER)
	}
	cliutils.ExchangeGet(os.Getenv(DEVTOOL_HZN_EXCHANGE_URL), resSuffix, cliutils.OrgAndCreds(os.Getenv(DEVTOOL_HZN_ORG), userCreds), []int{200}, resp)

	cliutils.Verbose("Response: %v", resp)

	// Parse the response and extract the 1 microservice definition or return an error if not 1 ms.
	var microserviceDef exchange.MicroserviceDefinition
	if len(resp.Microservices) > 1 {
		listed := ""
		for _, msDef := range resp.Microservices {
			listed += fmt.Sprintf("version: %v arch: %v, ", msDef.Version, msDef.Arch)
		}
		listed = listed[:len(listed)-2]
		return errors.New(fmt.Sprintf("more than 1 microservice found in the exchange, please specify version and/or hardware architecture to narrow the results: %v", listed))
	} else if len(resp.Microservices) == 0 {
		return errors.New(fmt.Sprintf("no microservices found in the exchange."))
	} else {
		for _, msDef := range resp.Microservices {
			microserviceDef = msDef
			break
		}
	}

	// Create a new dependency object.
	newDep := new(Dependency)

	// Save the source of the dependency.
	newDep.MetaRef = MetadataReference{
		SpecRef: specRef,
		Org:     org,
		Version: version,
		Arch:    arch,
	}

	// Get the dependency's deployment config.
	dc := new(cliexchange.DeploymentConfig)
	for _, wl := range microserviceDef.Workloads {
		if err := json.Unmarshal([]byte(wl.Deployment), dc); err != nil {
			return errors.New(fmt.Sprintf("failed to unmarshal deployment %v: %v", microserviceDef.Workloads[0].Deployment, err))
		}
		newDep.DeployConfig = *dc
	}

	// Fill in the parts of the dependency that come from the microservice definition.
	newDep.SpecRef = microserviceDef.SpecRef
	newDep.Version = microserviceDef.Version
	newDep.Arch = microserviceDef.Arch
	newDep.Sharable = microserviceDef.Sharable
	newDep.UserInputs = microserviceDef.UserInputs
	newDep.Global = []GlobalSet{}

	cliutils.Verbose("Found dependency %v, Org: %v", newDep.ShortString(), org)

	// Harden the new dependency in the file.
	if err := UpdateDependencyFile(homeDirectory, newDep, org); err != nil {
		return err
	}

	// Update the workload definition dependencies to make sure the dependency is included. The APISpec array
	// in the workload definition is rebuilt from the dependencies.
	if err := RefreshWorkloadDependencies(homeDirectory, nil); err != nil {
		return err
	}

	// Add skeletal userinputs to this project's userinput file.

	// Get this project's userinputs.
	currentUIs, _, err := GetUserInputs(homeDirectory, "")
	if err != nil {
		return err
	}

	// Loop through this project's microservice variable configurations and add skeletal non-default variables that
	// are defined by the new dependency.
	foundUIs := false
	for _, currentUI := range currentUIs.Microservices {
		if currentUI.Url == newDep.SpecRef && currentUI.Org == org && currentUI.VersionRange == newDep.Version {
			// The new dependency already has userinputs configured in this project.
			cliutils.Verbose("The current project already has userinputs defined for this dependency.")
			foundUIs = true
			break
		}
	}

	// If there are no variables already defined, add skeletal variables.
	if !foundUIs {
		foundNonDefault := false
		vars := make(map[string]interface{})
		for _, ui := range newDep.UserInputs {
			if ui.DefaultValue == "" {
				foundNonDefault = true
				vars[ui.Name] = ""
			}
		}

		if foundNonDefault {
			skelVarConfig := MicroWork{
				Org:          org,
				Url:          newDep.SpecRef,
				VersionRange: newDep.Version,
				Variables:    vars,
			}
			currentUIs.Microservices = append(currentUIs.Microservices, skelVarConfig)

			if err := CreateFile(homeDirectory, USERINPUT_FILE, currentUIs); err != nil {
				return err
			}

			cliutils.Verbose("Updated %v/%v with the dependency's variable configuration.", homeDirectory, USERINPUT_FILE)
			fmt.Printf("Please provide a value for the dependency's non-default variables in the microservices section of this project's userinput file to ensure that the dependency operates correctly. The userInputs section of the new dependency contains a definition for each user input variable.\n")

		}
	}

	fmt.Printf("Please add Horizon attributes to the global section of the new dependency to ensure that the dependency operates correctly.\n")

	return nil
}

func UpdateDependencyFile(homeDirectory string, newDep *Dependency, org string) error {

	// Create the dependency key.
	key := fmt.Sprintf("%v/%v", org, cliutils.FormExchangeId(newDep.SpecRef, newDep.Version, newDep.Arch))

	// Get the current set of dependencies
	deps, err := GetDependencies(homeDirectory)
	if err != nil {
		return err
	}

	// Add the new dependency.
	if err := deps.AddNewDependency(key, newDep); err != nil {
		return err
	}

	// Write file back to the project.
	if err := CreateFile(homeDirectory, DEPENDENCIES_FILE, deps); err != nil {
		return err
	}

	cliutils.Verbose("Updated %v/%v with the new dependency.", homeDirectory, DEPENDENCIES_FILE)

	return nil
}
