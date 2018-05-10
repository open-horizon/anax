package dev

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	cliexchange "github.com/open-horizon/anax/cli/exchange"
	"github.com/open-horizon/anax/cli/register"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/policy"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
)

const DEPENDENCY_COMMAND = "dependency"
const DEPENDENCY_FETCH_COMMAND = "fetch"
const DEPENDENCY_LIST_COMMAND = "list"
const DEPENDENCY_REMOVE_COMMAND = "remove"

// This function assumes that 1 of specRef or url is set, and that org is set. Everything else is optional.
func createLogMessage(specRef string, url string, org string, version string, arch string) string {
	// Create the right log message.
	target := fmt.Sprintf("specRef: %v, org: %v", specRef, org)
	if url != "" {
		target = fmt.Sprintf("url: %v, org: %v", url, org)
	}
	if version != "" {
		target += fmt.Sprintf(", version: %v", version)
	}
	if arch != "" {
		target += fmt.Sprintf(", arch: %v", arch)
	}
	return target
}

// This is the entry point for the hzn dev dependency fetch command.
func DependencyFetch(homeDirectory string, project string, specRef string, url string, org string, version string, arch string, userCreds string, keyFiles []string, userInputFile string) {

	// Check input parameters for correctness.
	dir, err := verifyFetchInput(homeDirectory, project, specRef, url, org, version, arch, userCreds)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'dependency %v' %v", DEPENDENCY_FETCH_COMMAND, err)
	}

	target := project

	// Go get the dependency metadata.
	if project != "" {
		if err := fetchLocalProjectDependency(dir, project, userInputFile); err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'dependency %v' %v", DEPENDENCY_FETCH_COMMAND, err)
		}
	} else {
		if err := fetchExchangeProjectDependency(dir, specRef, url, org, version, arch, userCreds, keyFiles, userInputFile); err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'dependency %v' %v", DEPENDENCY_FETCH_COMMAND, err)
		}

		// Create the right log message.
		target = createLogMessage(specRef, url, org, version, arch)
	}

	fmt.Printf("New dependency on %v created.\n", target)
}

// This is the entry point for the hzn dev dependency list command.
func DependencyList(homeDirectory string) {

	dir, err := setup(homeDirectory, true, false, "")
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", DEPENDENCY_COMMAND, DEPENDENCY_LIST_COMMAND, err)
	}

	if IsMicroserviceProject(dir) || IsWorkloadProject(dir) {
		deps, err := GetDependencies(dir)
		if err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", DEPENDENCY_COMMAND, DEPENDENCY_LIST_COMMAND, err)
		}

		marshalListOut(deps)

	} else if IsServiceProject(dir) {
		// Get the service definition, so that we can look at the service dependencies.
		serviceDef, sderr := GetServiceDefinition(dir, SERVICE_DEFINITION_FILE)
		if sderr != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", DEPENDENCY_COMMAND, DEPENDENCY_LIST_COMMAND, sderr)
		}

		// Now get all the dependencies
		deps, err := GetServiceDependencies(dir, serviceDef.RequiredServices)
		if err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", DEPENDENCY_COMMAND, DEPENDENCY_LIST_COMMAND, err)
		}

		marshalListOut(deps)
	}

}

func marshalListOut(deps interface{}) {
	jsonBytes, err := json.MarshalIndent(deps, "", "    ")
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "'%v %v' unable to create json object from dependencies, %v", DEPENDENCY_COMMAND, DEPENDENCY_LIST_COMMAND, err)
	}
	fmt.Printf("%v\n", string(jsonBytes))
}

// This is the entry point for the hzn dev dependency remove command.
func DependencyRemove(homeDirectory string, specRef string, url string, version string, arch string) {

	// Check input parameters for correctness.
	dir, err := verifyRemoveInput(homeDirectory, specRef, url, version, arch)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'dependency %v' %v", DEPENDENCY_REMOVE_COMMAND, err)
	}

	var theDep cliexchange.AbstractServiceFile
	var depFileInfo os.FileInfo
	uniqueDep := true

	// Indicate the kinds of dependency files we need to look for.
	fileSuffix := SERVICE_DEFINITION_FILE
	if IsWorkloadProject(dir) || IsMicroserviceProject(dir) {
		fileSuffix = MICROSERVICE_DEFINITION_FILE
	}

	// Grab the dependency files from the filesystem.
	deps, err := GetDependencyFiles(dir, fileSuffix)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", DEPENDENCY_COMMAND, DEPENDENCY_REMOVE_COMMAND, err)
	}

	// Make sure we can uniquely identify the dependency to be removed.
	var tempDep cliexchange.AbstractServiceFile
	for _, fileInfo := range deps {

		tempDep = nil
		if IsWorkloadProject(dir) || IsMicroserviceProject(dir) {
			if dep, err := GetMicroserviceDefinition(path.Join(dir, DEFAULT_DEPENDENCY_DIR), fileInfo.Name()); err != nil {
				cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", DEPENDENCY_COMMAND, DEPENDENCY_REMOVE_COMMAND, err)
			} else {
				tempDep = dep
			}
		} else {
			if dep, err := GetServiceDefinition(path.Join(dir, DEFAULT_DEPENDENCY_DIR), fileInfo.Name()); err != nil {
				cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", DEPENDENCY_COMMAND, DEPENDENCY_REMOVE_COMMAND, err)
			} else {
				tempDep = dep
			}
		}

		if (tempDep.GetURL() == specRef || tempDep.GetURL() == url) && (version == "" || (version != "" && tempDep.GetVersion() == version)) && (arch == "" || (arch != "" && tempDep.GetArch() == arch)) {
			if theDep != nil {
				uniqueDep = false
				break
			}
			theDep = tempDep
			depFileInfo = fileInfo
		}
	}

	// If we did not find the dependency, then return the error. If the input did not uniquely identify the dependency, then return
	// the error. Otherwise remove the dependency.
	if theDep == nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'dependency %v' dependency not found.", DEPENDENCY_REMOVE_COMMAND)
	} else if !uniqueDep {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'dependency %v' dependency %v is not unique. Please specify version and/or architecture to uniquely identify the dependency.", DEPENDENCY_REMOVE_COMMAND, specRef)
	} else {
		cliutils.Verbose("Found dependency: %v", depFileInfo.Name())

		// We know which dependency to remove, so remove it.
		if err := os.Remove(path.Join(dir, DEFAULT_DEPENDENCY_DIR, depFileInfo.Name())); err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'dependency %v' dependency %v could not be removed, error: %v", DEPENDENCY_REMOVE_COMMAND, depFileInfo.Name(), err)
		}

		// Update the service definition with the new dependencies.
		if err := RemoveServiceDependency(dir, theDep); err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'dependency %v' error updating project definition: %v", DEPENDENCY_REMOVE_COMMAND, err)
		}

		// Update the default userinputs removing any configured variables.
		if err := RemoveConfiguredVariables(dir, theDep); err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'dependency %v' error updating userinputs: %v", DEPENDENCY_REMOVE_COMMAND, err)
		}

	}

	// Create the right log message.
	fmt.Printf("Removed dependency %v.\n", createLogMessage(specRef, url, theDep.GetOrg(), version, arch))
}

// Returns an os.FileInfo object for each dependency file. This function assumes the caller has
// determined the exact location of the files.
func GetDependencyFiles(directory string, fileSuffix string) ([]os.FileInfo, error) {

	res := make([]os.FileInfo, 0, 10)
	depPath := path.Join(directory, DEFAULT_DEPENDENCY_DIR)
	if files, err := ioutil.ReadDir(depPath); err != nil {
		return res, errors.New(fmt.Sprintf("unable to get list of dependency files in %v, error: %v", depPath, err))
	} else {
		for _, fileInfo := range files {
			if strings.HasSuffix(fileInfo.Name(), fileSuffix) && !fileInfo.IsDir() {
				res = append(res, fileInfo)
			}
		}
	}

	return res, nil

}

func GetDependencies(directory string) ([]*cliexchange.MicroserviceFile, error) {
	res := make([]*cliexchange.MicroserviceFile, 0, 10)
	depFiles, err := GetDependencyFiles(directory, MICROSERVICE_DEFINITION_FILE)
	if err != nil {
		return res, err
	}

	for _, fileInfo := range depFiles {
		d, err := GetMicroserviceDefinition(path.Join(directory, DEFAULT_DEPENDENCY_DIR), fileInfo.Name())
		if err != nil {
			return res, err
		} else {
			res = append(res, d)
		}
	}

	return res, nil
}

func GetServiceDependencies(directory string, deps []exchange.ServiceDependency) ([]*cliexchange.ServiceFile, error) {
	res := make([]*cliexchange.ServiceFile, 0, 10)
	depFiles, err := GetDependencyFiles(directory, SERVICE_DEFINITION_FILE)
	if err != nil {
		return res, err
	}

	for _, fileInfo := range depFiles {
		d, err := GetServiceDefinition(path.Join(directory, DEFAULT_DEPENDENCY_DIR), fileInfo.Name())
		if err != nil {
			return res, err
		} else if d.IsDependent(deps) {
			res = append(res, d)
		}
	}

	return res, nil
}

// Check for the existence of the dependency directory in the project.
func DependenciesExists(directory string) (bool, error) {
	return FileExists(directory, DEFAULT_DEPENDENCY_DIR)
}

// Validate that the dependencies are complete and coherent with the rest of the definitions in the project.
// Any errors will be returned to the caller.
func ValidateDependencies(directory string, userInputs *register.InputFile, userInputsFilePath string) error {

	if IsWorkloadProject(directory) {

		// Get the current project's definition file.
		d, err := GetWorkloadDefinition(directory)
		if err != nil {
			return err
		}

		// For each microservice definition file in the dependencies directory, verify it.
		deps, err := GetDependencyFiles(directory, MICROSERVICE_DEFINITION_FILE)
		if err != nil {
			return err
		}

		// Validate each dependency, not just the direct dependencies of the current project.
		for _, fileInfo := range deps {
			if err := ValidateMicroserviceDefinition(path.Join(directory, DEFAULT_DEPENDENCY_DIR), fileInfo.Name()); err != nil {
				return errors.New(fmt.Sprintf("dependency %v did not validate, error: %v", fileInfo.Name(), err))
			} else if err := Validate(directory, fileInfo, userInputs, userInputsFilePath); err != nil {
				return errors.New(fmt.Sprintf("dependency %v did not validate, error: %v", fileInfo.Name(), err))
			}
		}

		// Validate that the project defintion's dependencies are present in the dependencies directory.
		for _, rs := range d.APISpecs {
			fName := createDependencyFileName(rs.SpecRef, rs.Version, MICROSERVICE_DEFINITION_FILE)
			found := false
			for _, fileInfo := range deps {
				if fName == fileInfo.Name() {
					found = true
					break
				}
			}
			if !found {
				return errors.New(fmt.Sprintf("dependency %v does not exist in %v.", rs.SpecRef, path.Join(directory, DEFAULT_DEPENDENCY_DIR)))
			}
		}

	} else if IsServiceProject(directory) {

		d, err := GetServiceDefinition(directory, SERVICE_DEFINITION_FILE)
		if err != nil {
			return err
		}

		// For each service definition file in the dependencies directory, verify it.
		deps, err := GetDependencyFiles(directory, SERVICE_DEFINITION_FILE)
		if err != nil {
			return err
		}

		for _, fileInfo := range deps {
			if err := ValidateServiceDefinition(path.Join(directory, DEFAULT_DEPENDENCY_DIR), fileInfo.Name()); err != nil {
				return errors.New(fmt.Sprintf("dependency %v did not validate, error: %v", fileInfo.Name(), err))
			} else if err := ValidateService(directory, fileInfo, userInputs, userInputsFilePath); err != nil {
				return errors.New(fmt.Sprintf("dependency %v did not validate, error: %v", fileInfo.Name(), err))
			}
		}

		// Validate that the project defintion's dependencies are present in the dependencies directory.
		for _, rs := range d.RequiredServices {
			fName := createDependencyFileName(rs.URL, rs.Version, SERVICE_DEFINITION_FILE)
			found := false
			for _, fileInfo := range deps {
				if fName == fileInfo.Name() {
					found = true
					break
				}
			}
			if !found {
				return errors.New(fmt.Sprintf("dependency %v does not exist in %v.", rs.URL, path.Join(directory, DEFAULT_DEPENDENCY_DIR)))
			}
		}
	}

	return nil
}

func Validate(directory string, fInfo os.FileInfo, userInputs *register.InputFile, userInputsFilePath string) error {

	d, err := GetMicroserviceDefinition(path.Join(directory, DEFAULT_DEPENDENCY_DIR), fInfo.Name())
	if err != nil {
		return err
	}

	// Userinputs from the dependency without a default value must be set in the userinput file.
	return validateDependencyUserInputs(d, d.GetUserInputs(), userInputs.Microservices, userInputsFilePath)

}

func ValidateService(directory string, fInfo os.FileInfo, userInputs *register.InputFile, userInputsFilePath string) error {
	d, err := GetServiceDefinition(path.Join(directory, DEFAULT_DEPENDENCY_DIR), fInfo.Name())
	if err != nil {
		return err
	}

	// Userinputs from the dependency without a default value must be set in the userinput file.
	return validateDependencyUserInputs(d, d.GetUserInputs(), userInputs.Services, userInputsFilePath)
}

func validateDependencyUserInputs(d cliexchange.AbstractServiceFile, uis []exchange.UserInput, configUserInputs []register.MicroWork, userInputsFilePath string) error {
	for _, ui := range uis {
		if ui.DefaultValue == "" {
			found := false
			for _, msUI := range configUserInputs {
				if d.GetURL() == msUI.Url {
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
	return nil
}

func verifyFetchInput(homeDirectory string, project string, specRef string, url string, org string, version string, arch string, userCreds string) (string, error) {

	// Shut off the Anax runtime logging.
	flag.Set("v", "0")

	// Verify that the environment and inputs are usable.
	dir, err := VerifyEnvironment(homeDirectory, true, true, userCreds)
	if err != nil {
		return "", err
	}

	// Valid inputs are either project or the others, but not both. version and arch are optional when specref and org are used.
	// url and specRef are mutually exclusive with each other.
	if specRef != "" && url != "" {
		return "", errors.New(fmt.Sprintf("--specRef is mutually exclusive with --url."))
	} else if project != "" && (specRef != "" || org != "" || url != "") {
		return "", errors.New(fmt.Sprintf("--project is mutually exclusive with --specRef, --org and --url."))
	} else if project == "" && specRef == "" && org == "" && url == "" {
		return "", errors.New(fmt.Sprintf("one of --project, or --specRef and --org, or --url and --org must be specified."))
	} else if (specRef != "" && org == "") || (specRef == "" && org != "" && url == "") || (url != "" && org == "") {
		return "", errors.New(fmt.Sprintf("either --specRef and --org, or --url and --org must be specified."))
	}

	// Verify that the inputs match with the project type.
	if specRef != "" && IsServiceProject(dir) {
		return "", errors.New(fmt.Sprintf("use --url with service projects."))
	} else if url != "" && IsWorkloadProject(dir) {
		return "", errors.New(fmt.Sprintf("use --specRef with workload projects."))
	} else if IsMicroserviceProject(dir) {
		return "", errors.New(fmt.Sprintf("microservice projects cannot have dependencies."))
	}

	// Verify that if --project was specified, it points to a valid horizon project directory.
	if project != "" {
		if !IsMicroserviceProject(project) && !IsServiceProject(project) {
			return "", errors.New(fmt.Sprintf("--project %v does not contain Horizon service or microservice metadata.", project))
		} else if IsMicroserviceProject(project) {
			if err := ValidateMicroserviceDefinition(project, MICROSERVICE_DEFINITION_FILE); err != nil {
				return "", err
			}
		} else {
			if err := ValidateServiceDefinition(project, SERVICE_DEFINITION_FILE); err != nil {
				return "", err
			}
		}
	}

	cliutils.Verbose("Reading Horizon metadata from %s", dir)

	return dir, nil
}

func verifyRemoveInput(homeDirectory string, specRef string, url string, version string, arch string) (string, error) {

	// Shut off the Anax runtime logging.
	flag.Set("v", "0")

	// Verify that the environment and inputs are usable.
	dir, err := VerifyEnvironment(homeDirectory, true, false, "")
	if err != nil {
		return "", err
	}

	// Valid inputs are specRef with the others being optional.
	if specRef == "" && url == "" {
		return "", errors.New(fmt.Sprintf("--specRef or --url is required for remove."))
	} else if specRef != "" && url != "" {
		return "", errors.New(fmt.Sprintf("--specRef and --url are mutually exclusive."))
	}

	cliutils.Verbose("Reading Horizon metadata from %s", dir)

	return dir, nil
}

// The caller is trying to use a local project (i.e. a project that is on the same machine) as a dependency.
// If the dependency is a local project then we can validate it and copy the project metadata.
func fetchLocalProjectDependency(homeDirectory string, project string, userInputFile string) error {

	// Get the setup info and context for running the command.
	dir, err := setup(homeDirectory, true, false, "")
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", DEPENDENCY_COMMAND, DEPENDENCY_FETCH_COMMAND, err)
	}

	// If the dependent project is not validate-able then we cant reliably use it as a dependency.
	// Validate the dependency project based on what kind of project we are currently working with.
	dependentProjectType := "Microservice"
	if IsWorkloadProject(dir) {
		if err := AbstractServiceValidation(project, false); err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", DEPENDENCY_COMMAND, DEPENDENCY_FETCH_COMMAND, err)
		}
	} else if IsServiceProject(dir) {
		dependentProjectType = "Service"
		if err := AbstractServiceValidation(project, true); err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", DEPENDENCY_COMMAND, DEPENDENCY_FETCH_COMMAND, err)
		}
	} else {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' microservice projects cannot have dependencies", DEPENDENCY_COMMAND, DEPENDENCY_FETCH_COMMAND)
	}

	CommonProjectValidation(project, userInputFile, DEPENDENCY_COMMAND, DEPENDENCY_FETCH_COMMAND)

	fmt.Printf("%v project %v verified.\n", dependentProjectType, dir)

	// The rest of this function gets the dependency's user input and adds it to this project's user input, and it reads
	// this project's workload definition and updates it with the reference to the ms. In the files that are read and
	// then written we want those to preserve the env vars as env vars.
	envVarSetting := os.Getenv("HZN_DONT_SUBST_ENV_VARS")
	os.Setenv("HZN_DONT_SUBST_ENV_VARS", "1")

	// Pull the metadata from the dependent project. Log the filesystem location of the dependent metadata.
	if absProject, err := filepath.Abs(project); err != nil {
		return err
	} else {
		cliutils.Verbose("Reading Horizon metadata from dependency: %v", absProject)
	}

	// Get the dependency's definition.
	sDef, err := GetAbstractDefinition(project)
	if err != nil {
		return err
	}

	// Get the dependency's variable configurations.
	depVarConfig, err := GetUserInputsVariableConfiguration(project, userInputFile)
	if err != nil {
		return err
	}

	cliutils.Verbose("Found dependency %v, Org: %v", sDef.GetURL(), sDef.GetOrg())

	// Harden the new dependency in a file in this project's dependency store.
	if err := UpdateDependencyFile(homeDirectory, sDef); err != nil {
		return err
	}

	// Harden the dependent's dependencies so that the current project will be able to get all
	// the dependencies running.
	if err := UpdateDependentDependencies(homeDirectory, project); err != nil {
		return err
	}

	// Update the project's definition dependencies to make sure the dependency is included.
	if err := RefreshServiceDependencies(homeDirectory, sDef); err != nil {
		return err
	}

	// Update this project's userinputs with variable configuration from the dependency's userinputs.
	currentUIs, uerr := UpdateVariableConfiguration(homeDirectory, sDef, depVarConfig)
	if uerr != nil {
		return uerr
	}

	// Get the dependency's userinputs to get the global attribute settings.
	depUserInputs, _, uierr := GetUserInputs(project, userInputFile)
	if uierr != nil {
		return uierr
	}

	// Find the global attributes in the dependency and move them into this project.
	for _, depGlobal := range depUserInputs.Global {
		found := false
		for _, currentUIGlobal := range currentUIs.Global {
			if currentUIGlobal.Type == depGlobal.Type && reflect.DeepEqual(currentUIGlobal.Variables, depGlobal.Variables) {
				found = true
				break
			}
		}
		// If the global setting was already in the current project, then dont copy anything from the dependency.
		if found {
			continue
		} else {
			// Copy the global setting so that the dependency continues to work correctly. Also tag the global setting with the
			// dependencies URL so that the system knows it only applies to this dependency.
			if len(depGlobal.SensorUrls) == 0 {
				depGlobal.SensorUrls = append(depGlobal.SensorUrls, sDef.GetURL())
			}
			currentUIs.Global = append(currentUIs.Global, depGlobal)
		}
	}

	// Update the user input file in the filesystem.
	if err := CreateFile(homeDirectory, USERINPUT_FILE, currentUIs); err != nil {
		return err
	}

	cliutils.Verbose("Updated %v/%v with the dependency's variable and global attribute configuration.", homeDirectory, USERINPUT_FILE)
	os.Setenv("HZN_DONT_SUBST_ENV_VARS", envVarSetting) // restore this setting

	return nil
}

func fetchExchangeProjectDependency(homeDirectory string, specRef string, url string, org string, version string, arch string, userCreds string, keyFiles []string, userInputFile string) error {

	projectType := "service"
	if IsWorkloadProject(homeDirectory) {
		projectType = "microservice"
	}

	// Pull the metadata from the exchange, including any of this dependency's dependencies.
	sDef, err := getExchangeDefinition(homeDirectory, specRef, url, org, version, arch, userCreds, keyFiles, userInputFile)
	if err != nil {
		return err
	}

	// Harden the new dependency in the file.
	if err := UpdateDependencyFile(homeDirectory, sDef); err != nil {
		return err
	}

	// The rest of this function gets the dependency's user input and adds it to this project's user input, and it reads
	// this project's workload definition and updates it with the reference to the ms. In the files that are read and
	// then written we want those to preserve the env vars as env vars.
	envVarSetting := os.Getenv("HZN_DONT_SUBST_ENV_VARS")
	os.Setenv("HZN_DONT_SUBST_ENV_VARS", "1")

	// Update the workload definition dependencies to make sure the dependency is included. The APISpec array
	// in the workload definition is rebuilt from the dependencies.
	if err := RefreshServiceDependencies(homeDirectory, sDef); err != nil {
		return err
	}

	// Loop through this project's variable configurations and add skeletal non-default variables that
	// are defined by the new dependency.
	foundUIs := false
	varConfigs, err := GetUserInputsVariableConfiguration(homeDirectory, "")
	for _, currentUI := range varConfigs {
		if currentUI.Url == sDef.GetURL() && currentUI.Org == org && currentUI.VersionRange == sDef.GetVersion() {
			// The new dependency already has userinputs configured in this project.
			cliutils.Verbose("The current project already has userinputs defined for this dependency.")
			foundUIs = true
			break
		}
	}

	// If there are no variables already defined, and there are non-defaulted variables, then add skeletal variables.
	if !foundUIs {
		foundNonDefault := false
		vars := make(map[string]interface{})
		for _, ui := range sDef.GetUserInputs() {
			if ui.DefaultValue == "" {
				foundNonDefault = true
				vars[ui.Name] = ""
			}
		}

		if foundNonDefault {
			skelVarConfig := register.MicroWork{
				Org:          org,
				Url:          sDef.GetURL(),
				VersionRange: sDef.GetVersion(),
				Variables:    vars,
			}
			if err := SetUserInputsVariableConfiguration(homeDirectory, sDef, []register.MicroWork{skelVarConfig}); err != nil {
				return err
			}

			cliutils.Verbose("Updated %v/%v with the dependency's variable configuration.", homeDirectory, USERINPUT_FILE)
			fmt.Printf("Please provide a value for the dependency's non-default variables in the %v section of this project's userinput file to ensure that the dependency operates correctly. The userInputs section of the new dependency contains a definition for each user input variable.\n", projectType)

		}
	}

	fmt.Printf("Please add Horizon attributes to the global section of the new dependency to ensure that the dependency operates correctly.\n")
	os.Setenv("HZN_DONT_SUBST_ENV_VARS", envVarSetting) // restore this setting

	return nil
}

func getExchangeDefinition(homeDirectory string, specRef string, surl string, org string, version string, arch string, userCreds string, keyFiles []string, userInputFile string) (cliexchange.AbstractServiceFile, error) {

	if IsWorkloadProject(homeDirectory) {

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
		cliutils.SetWhetherUsingApiKey(userCreds)
		cliutils.ExchangeGet(cliutils.GetExchangeUrl(), resSuffix, cliutils.OrgAndCreds(os.Getenv(DEVTOOL_HZN_ORG), userCreds), []int{200}, resp)

		// Parse the response and extract the 1 microservice definition or return an error if not 1 ms.
		var microserviceDef exchange.MicroserviceDefinition
		if len(resp.Microservices) > 1 {
			listed := ""
			for _, msDef := range resp.Microservices {
				listed += fmt.Sprintf("version: %v arch: %v, ", msDef.Version, msDef.Arch)
			}
			listed = listed[:len(listed)-2]
			return nil, errors.New(fmt.Sprintf("more than 1 microservice found in the exchange, please specify version and/or hardware architecture to narrow the results: %v", listed))
		} else if len(resp.Microservices) == 0 {
			return nil, errors.New(fmt.Sprintf("no microservices found in the exchange."))
		} else {
			for _, msDef := range resp.Microservices {
				microserviceDef = msDef
				break
			}
		}

		cliutils.Verbose("Creating dependency %v, Org: %v", microserviceDef, org)

		msDef := new(cliexchange.MicroserviceFile)

		// Get this project's userinputs.
		currentUIs, _, err := GetUserInputs(homeDirectory, "")
		if err != nil {
			return nil, err
		}

		// Get the dependency's deployment container images
		dc := new(cliexchange.DeploymentConfig)
		var torrent policy.Torrent
		if len(microserviceDef.Workloads) > 0 {
			ms_workload := microserviceDef.Workloads[0]
			if err := json.Unmarshal([]byte(ms_workload.Deployment), dc); err != nil {
				return nil, errors.New(fmt.Sprintf("failed to unmarshal deployment %v: %v", ms_workload.Deployment, err))
			}

			// convert to torrent structure only if the torrent string exists on the exchange
			if ms_workload.Torrent != "" {
				if err := json.Unmarshal([]byte(ms_workload.Torrent), &torrent); err != nil {
					return nil, fmt.Errorf("The torrent definition for microservice %v has error: %v", microserviceDef.SpecRef, err)
				}
			}

			url1, err := url.Parse(torrent.Url)
			if err != nil {
				return nil, fmt.Errorf("ill-formed URL: %v, error %v", torrent.Url, err)
			}

			cc := events.NewContainerConfig(*url1, torrent.Signature, ms_workload.Deployment, ms_workload.DeploymentSignature, "", "", make([]events.ImageDockerAuth, 0))

			if err := getContainerImages(cc, keyFiles, currentUIs); err != nil {
				return nil, errors.New(fmt.Sprintf("failed to get images for %v/%v: %v", org, specRef, err))
			}
		}

		// Fill in the parts of the dependency that come from the microservice definition.
		msDef.Org = org
		msDef.SpecRef = microserviceDef.SpecRef
		msDef.Version = microserviceDef.Version
		msDef.Arch = microserviceDef.Arch
		msDef.Label = microserviceDef.Label
		msDef.Description = microserviceDef.Description
		msDef.Public = microserviceDef.Public
		msDef.Sharable = microserviceDef.Sharable
		msDef.DownloadURL = microserviceDef.DownloadURL
		msDef.UserInputs = microserviceDef.UserInputs
		msDef.Workloads = []cliexchange.WorkloadDeployment{
			cliexchange.WorkloadDeployment{
				Deployment:          dc,
				DeploymentSignature: "",
				Torrent:             "",
			},
		}

		return msDef, nil

	} else if IsServiceProject(homeDirectory) {

		return getServiceDefinition(homeDirectory, surl, org, version, arch, userCreds, keyFiles)

	} else {
		return nil, errors.New(fmt.Sprintf("unsupported project type"))
	}

}

func UpdateDependencyFile(homeDirectory string, sDef cliexchange.AbstractServiceFile) error {

	// Determine what the file suffix should be
	fileSuffix := MICROSERVICE_DEFINITION_FILE
	switch sDef.(type) {
	case *cliexchange.ServiceFile:
		fileSuffix = SERVICE_DEFINITION_FILE
	}

	fileName := createDependencyFileName(sDef.GetURL(), sDef.GetVersion(), fileSuffix)

	filePath := path.Join(homeDirectory, DEFAULT_DEPENDENCY_DIR)
	if err := CreateFile(filePath, fileName, sDef); err != nil {
		return err
	}

	cliutils.Verbose("Created %v/%v as a new dependency.", filePath, fileName)

	return nil
}

func createDependencyFileName(url string, version string, suffix string) string {
	// Create the dependency filename.
	re := regexp.MustCompile(`^[A-Za-z0-9+.-]*?://`)
	url2 := re.ReplaceAllLiteralString(cliutils.ExpandEnv(url), "")
	re = regexp.MustCompile(`[$!*,;/?@&~=%]`)
	url3 := re.ReplaceAllLiteralString(url2, "-")

	return fmt.Sprintf("%v_%v.%v", url3, cliutils.ExpandEnv(version), suffix)
}

// Copy the dependency files out, validate them and write them back.
func UpdateDependentDependencies(homeDirectory string, depProject string) error {

	// Return early for non-service projects
	if !IsServiceProject(homeDirectory) {
		return nil
	}

	// If there is a local project dependency, get the local dependency files.
	if depProject != "" {

		deps, err := GetDependencyFiles(depProject, SERVICE_DEFINITION_FILE)
		if err != nil {
			return err
		}

		for _, dep := range deps {
			if sDef, err := GetServiceDefinition(path.Join(depProject, DEFAULT_DEPENDENCY_DIR), dep.Name()); err != nil {
				return err
			} else if err := ValidateServiceDefinition(path.Join(depProject, DEFAULT_DEPENDENCY_DIR), dep.Name()); err != nil {
				return errors.New(fmt.Sprintf("dependency %v did not validate, error: %v", dep.Name(), err))
			} else if err := CreateFile(path.Join(homeDirectory, DEFAULT_DEPENDENCY_DIR), dep.Name(), sDef); err != nil {
				return err
			}
		}
	}

	return nil

}

// Iterate through the dependencies of the given service and create a dependency for each one.
func getServiceDefinitionDependencies(homeDirectory string, serviceDef *cliexchange.ServiceFile, userCreds string, keyFiles []string) error {
	for _, rs := range serviceDef.RequiredServices {
		// Get the service definition for each required service. Dependencies refer to each other by version range, so the
		// service we're looking for might not be at the exact version specified in the required service element.
		if sDef, err := getServiceDefinition(homeDirectory, rs.URL, rs.Org, "", rs.Arch, userCreds, keyFiles); err != nil {
			return err
		} else if err := UpdateDependencyFile(homeDirectory, sDef); err != nil {
			return err
		}
	}
	return nil
}

func getServiceDefinition(homeDirectory, surl string, org string, version string, arch string, userCreds string, keyFiles []string) (*cliexchange.ServiceFile, error) {

	// Construct the resource URL suffix.
	resSuffix := fmt.Sprintf("orgs/%v/services?url=%v", org, surl)
	if version != "" {
		resSuffix += fmt.Sprintf("&version=%v", version)
	}
	if arch != "" {
		resSuffix += fmt.Sprintf("&arch=%v", arch)
	}

	// Create an object to hold the response.
	resp := new(exchange.GetServicesResponse)

	// Call the exchange to get the service definition.
	if userCreds == "" {
		userCreds = os.Getenv(DEVTOOL_HZN_USER)
	}
	cliutils.SetWhetherUsingApiKey(userCreds)
	cliutils.ExchangeGet(cliutils.GetExchangeUrl(), resSuffix, cliutils.OrgAndCreds(os.Getenv(DEVTOOL_HZN_ORG), userCreds), []int{200}, resp)

	// Parse the response and extract the highest version service definition or return an error.
	var serviceDef exchange.ServiceDefinition
	var serviceId string
	if len(resp.Services) > 1 {
		highest, sDef, sId, err := exchange.GetHighestVersion(resp.Services, nil)
		if err != nil {
			return nil, err
		} else if highest == "" {
			return nil, errors.New(fmt.Sprintf("unable to find highest version of %v %v in the exchange: %v", surl, org, resp.Services))
		} else {
			serviceDef = sDef
			serviceId = sId
		}

	} else if len(resp.Services) == 0 {
		return nil, errors.New(fmt.Sprintf("no services found in the exchange."))
	} else {
		for sId, sDef := range resp.Services {
			serviceDef = sDef
			serviceId = sId
			break
		}
	}

	cliutils.Verbose("Creating dependency on: %v, Org: %v", serviceDef, org)

	sDef_cliex := new(cliexchange.ServiceFile)

	// Get container images into the local docker
	dc := make(map[string]interface{})
	if serviceDef.Deployment != "" {
		if err := json.Unmarshal([]byte(serviceDef.Deployment), &dc); err != nil {
			return nil, errors.New(fmt.Sprintf("failed to unmarshal deployment %v: %v", serviceDef.Deployment, err))
		}

		// Get this project's userinputs so that the downloader can use any special authorization attributes that might
		// be specified in the global section of the user inputs.
		currentUIs, _, err := GetUserInputs(homeDirectory, "")
		if err != nil {
			return nil, err
		}

		// convert the image server info into torrent
		torrent := getImageReferenceAsTorrent(&serviceDef)

		// verify the image server url
		url1, err := url.Parse(torrent.Url)
		if err != nil {
			return nil, fmt.Errorf("ill-formed URL: %v, error %v", torrent.Url, err)
		}

		// Get docker auth for the service
		auth_url := fmt.Sprintf("orgs/%v/services/%v/dockauths", org, exchange.GetId(serviceId))
		docker_auths := make([]exchange.ImageDockerAuth, 0)
		cliutils.SetWhetherUsingApiKey(userCreds)
		cliutils.ExchangeGet(cliutils.GetExchangeUrl(), auth_url, cliutils.OrgAndCreds(os.Getenv(DEVTOOL_HZN_ORG), userCreds), []int{200, 404}, &docker_auths)

		img_auths := make([]events.ImageDockerAuth, 0)
		if docker_auths != nil {
			for _, iau_temp := range docker_auths {
				img_auths = append(img_auths, events.ImageDockerAuth{Registry: iau_temp.Registry, UserName: "token", Password: iau_temp.Token})
			}
		}
		cliutils.Verbose("The image docker auths for the service %v/%v are: %v", org, surl, img_auths)

		cc := events.NewContainerConfig(*url1, torrent.Signature, serviceDef.Deployment, serviceDef.DeploymentSignature, "", "", img_auths)

		// get the images
		if err := getContainerImages(cc, keyFiles, currentUIs); err != nil {
			return nil, errors.New(fmt.Sprintf("failed to get images for %v/%v: %v", org, surl, err))
		}
	}

	// Fill in the parts of the dependency that come from the microservice definition.
	sDef_cliex.Org = org
	sDef_cliex.URL = serviceDef.URL
	sDef_cliex.Version = serviceDef.Version
	sDef_cliex.Arch = serviceDef.Arch
	sDef_cliex.Label = serviceDef.Label
	sDef_cliex.Description = serviceDef.Description
	sDef_cliex.Public = serviceDef.Public
	sDef_cliex.Sharable = serviceDef.Sharable
	sDef_cliex.UserInputs = serviceDef.UserInputs
	sDef_cliex.Deployment = dc
	sDef_cliex.MatchHardware = serviceDef.MatchHardware
	sDef_cliex.RequiredServices = serviceDef.RequiredServices
	sDef_cliex.ImageStore = serviceDef.ImageStore

	// If this service has dependencies, bring them in.
	if serviceDef.HasDependencies() {
		if err := getServiceDefinitionDependencies(homeDirectory, sDef_cliex, userCreds, keyFiles); err != nil {
			return nil, err
		}
	}

	return sDef_cliex, nil
}
