package dev

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	cliexchange "github.com/open-horizon/anax/cli/exchange"
	"github.com/open-horizon/anax/cli/register"
	"github.com/open-horizon/anax/exchange"
	"io/ioutil"
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

// This is the entry point for the hzn dev dependency fetch command.
func DependencyFetch(homeDirectory string, project string, specRef string, org string, version string, arch string, userCreds string, keyFile string, userInputFile string) {

	// Check input parameters for correctness.
	dir, err := verifyFetchInput(homeDirectory, project, specRef, org, version, arch, userCreds)
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
		if err := fetchExchangeProjectDependency(dir, specRef, org, version, arch, userCreds, keyFile, userInputFile); err != nil {
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
		fmt.Printf("%v\n", string(jsonBytes))
	}

}

// This is the entry point for the hzn dev dependency remove command.
func DependencyRemove(homeDirectory string, specRef string, version string, arch string) {

	// Check input parameters for correctness.
	dir, err := verifyRemoveInput(homeDirectory, specRef, version, arch)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'dependency %v' %v", DEPENDENCY_REMOVE_COMMAND, err)
	}

	// Grab the dependency files from the filesystem.
	deps, err := GetDependencyFiles(dir)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", DEPENDENCY_COMMAND, DEPENDENCY_REMOVE_COMMAND, err)
	}

	// Make sure we can uniquely identify the dependency to be removed.
	var theDep *cliexchange.MicroserviceFile
	var depFileInfo os.FileInfo

	uniqueDep := true
	for _, fileInfo := range deps {

		dep, err := GetMicroserviceDefinition(path.Join(dir, DEFAULT_DEPENDENCY_DIR), fileInfo.Name())
		if err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", DEPENDENCY_COMMAND, DEPENDENCY_REMOVE_COMMAND, err)
		}

		if dep.SpecRef == specRef && (version == "" || (version != "" && dep.Version == version)) && (arch == "" || (arch != "" && dep.Arch == arch)) {
			if theDep != nil {
				uniqueDep = false
				break
			}
			theDep = dep
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

		// Update the workload definition with the new dependencies.
		if err := RefreshWorkloadDependencies(dir); err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'dependency %v' error updating workload definition: %v", DEPENDENCY_REMOVE_COMMAND, err)
		}

		cliutils.Verbose("Updated workload dependencies.")
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

// Returns an os.FileInfo object for each dependency file. This function assumes the caller has
// determined the exact location of the files.
func GetDependencyFiles(directory string) ([]os.FileInfo, error) {

	res := make([]os.FileInfo, 0, 10)
	depPath := path.Join(directory, DEFAULT_DEPENDENCY_DIR)
	if files, err := ioutil.ReadDir(depPath); err != nil {
		return res, errors.New(fmt.Sprintf("unable to get list of dependency files in %v, error: %v", depPath, err))
	} else {
		for _, fileInfo := range files {
			if strings.HasSuffix(fileInfo.Name(), MICROSERVICE_DEFINITION_FILE) && !fileInfo.IsDir() {
				res = append(res, fileInfo)
			}
		}
	}

	return res, nil

}

func GetDependencies(directory string) ([]*cliexchange.MicroserviceFile, error) {
	res := make([]*cliexchange.MicroserviceFile, 0, 10)
	depFiles, err := GetDependencyFiles(directory)
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

// Check for the existence of the dependency directory in the project.
func DependenciesExists(directory string) (bool, error) {
	return FileExists(directory, DEFAULT_DEPENDENCY_DIR)
}

// Validate that the dependencies are complete and coherent with the rest of the definitions in the project.
// Any errors will be returned to the caller.
func ValidateDependencies(directory string, userInputs *register.InputFile, userInputsFilePath string) error {

	// For each definition file in the dependencies directory, verify it.
	deps, err := GetDependencyFiles(directory)
	if err != nil {
		return err
	}

	for _, fileInfo := range deps {
		if err := ValidateMicroserviceDefinition(path.Join(directory, DEFAULT_DEPENDENCY_DIR), fileInfo.Name()); err != nil {
			return errors.New(fmt.Sprintf("dependency %v did not validate, error: %v", fileInfo.Name(), err))
		} else if err := Validate(directory, fileInfo, userInputs, userInputsFilePath); err != nil {
			return errors.New(fmt.Sprintf("dependency %v did not validate, error: %v", fileInfo.Name(), err))
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
		} else if err := ValidateMicroserviceDefinition(project, MICROSERVICE_DEFINITION_FILE); err != nil {
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

func fetchLocalProjectDependency(homeDirectory string, project string, userInputFile string) error {

	// If the dependency is a local project then we can validate it and copy the project metadata.

	// If the dependent project is not validate-able then we cant reliably use it as a dependency. This function
	// handles the 'hzn dev microservice verify' command so it will exit if there is an error.
	MicroserviceValidate(project, "")

	// The rest of this function gets the dependency's user input and adds it to this project's user input, and it reads
	// this project's workload definition and updates it with the reference to the ms. In the files that are read and
	// then written we want those to preserve the env vars as env vars.
	envVarSetting := os.Getenv("HZN_DONT_SUBST_ENV_VARS")
	os.Setenv("HZN_DONT_SUBST_ENV_VARS", "1")

	// Pull the metadata from the dependent project.
	if absProject, err := filepath.Abs(project); err != nil {
		return err
	} else {
		cliutils.Verbose("Reading Horizon metadata from dependency: %v", absProject)
	}

	// Get the dependency's definition.
	msDef, err := GetMicroserviceDefinition(project, MICROSERVICE_DEFINITION_FILE)
	if err != nil {
		return err
	}

	// Get the dependency's userinputs to get the global attribute settings and any variable configuration.
	ui, _, err := GetUserInputs(project, userInputFile)
	if err != nil {
		return err
	}

	cliutils.Verbose("Found dependency %v, Org: %v", msDef.SpecRef, msDef.Org)

	// Harden the new dependency in the file.
	if err := UpdateDependencyFile(homeDirectory, msDef); err != nil {
		return err
	}

	// Update the workload definition dependencies to make sure the dependency is included. The APISpec array
	// in the workload definition is rebuilt from the dependencies.
	if err := RefreshWorkloadDependencies(homeDirectory); err != nil {
		return err
	}

	// Update this project's userinputs with variable configuration from the dependency's userinputs and with global
	// attributes from the dependency.

	// Get this project's userinputs.
	currentUIs, _, err := GetUserInputs(homeDirectory, "")
	if err != nil {
		return err
	}

	// Find the configured variables for this dependency.
	var depVarConfig register.MicroWork
	for _, depUI := range ui.Microservices {
		if depUI.Url == msDef.SpecRef {
			depVarConfig = depUI
			break
		}
	}

	// If there are any user inputs, append them to this project's user inputs.
	if depVarConfig.Url != "" {
		found := false
		for _, currentUI := range currentUIs.Microservices {
			if currentUI.Url == depVarConfig.Url && currentUI.Org == depVarConfig.Org {
				found = true
				break
			}
		}
		if !found {
			currentUIs.Microservices = append(currentUIs.Microservices, depVarConfig)
		}
	}

	// Find the global attributes in the dependency and move them into this project.
	for _, depGlobal := range ui.Global {
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
			// dependencies spec ref URL so that the system knows it only applies to this dependency.
			if len(depGlobal.SensorUrls) == 0 {
				depGlobal.SensorUrls = append(depGlobal.SensorUrls, msDef.SpecRef)
			}
			currentUIs.Global = append(currentUIs.Global, depGlobal)
		}
	}

	if err := CreateFile(homeDirectory, USERINPUT_FILE, currentUIs); err != nil {
		return err
	}

	cliutils.Verbose("Updated %v/%v with the dependency's variable and global attribute configuration.", homeDirectory, USERINPUT_FILE)
	os.Setenv("HZN_DONT_SUBST_ENV_VARS", envVarSetting) // restore this setting

	return nil
}

func fetchExchangeProjectDependency(homeDirectory string, specRef string, org string, version string, arch string, userCreds string, keyFile string, userInputFile string) error {

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
		return errors.New(fmt.Sprintf("more than 1 microservice found in the exchange, please specify version and/or hardware architecture to narrow the results: %v", listed))
	} else if len(resp.Microservices) == 0 {
		return errors.New(fmt.Sprintf("no microservices found in the exchange."))
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
		return err
	}

	// Get the dependency's deployment config and container package info (if any).
	dc := new(cliexchange.DeploymentConfig)
	for _, wl := range microserviceDef.Workloads {
		if err := json.Unmarshal([]byte(wl.Deployment), dc); err != nil {
			return errors.New(fmt.Sprintf("failed to unmarshal deployment %v: %v", microserviceDef.Workloads[0].Deployment, err))
		}

		// Now that we have the deployment config, we can retrieve the container packages and download into docker.
		if wl.Torrent != "" {
			if err := downloadFromImageServer(wl.Torrent, keyFile, currentUIs); err != nil {
				return err
			}
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

	// Harden the new dependency in the file.
	if err := UpdateDependencyFile(homeDirectory, msDef); err != nil {
		return err
	}

	// The rest of this function gets the dependency's user input and adds it to this project's user input, and it reads
	// this project's workload definition and updates it with the reference to the ms. In the files that are read and
	// then written we want those to preserve the env vars as env vars.
	envVarSetting := os.Getenv("HZN_DONT_SUBST_ENV_VARS")
	os.Setenv("HZN_DONT_SUBST_ENV_VARS", "1")

	// Update the workload definition dependencies to make sure the dependency is included. The APISpec array
	// in the workload definition is rebuilt from the dependencies.
	if err := RefreshWorkloadDependencies(homeDirectory); err != nil {
		return err
	}

	// Add skeletal userinputs to this project's userinput file.

	// Get this project's userinputs again, this time w/o replacing the env vars, so we can write it out with those intact.
	currentUIs, _, err = GetUserInputs(homeDirectory, "")
	if err != nil {
		return err
	}

	// Loop through this project's microservice variable configurations and add skeletal non-default variables that
	// are defined by the new dependency.
	foundUIs := false
	for _, currentUI := range currentUIs.Microservices {
		if currentUI.Url == msDef.SpecRef && currentUI.Org == org && currentUI.VersionRange == msDef.Version {
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
		for _, ui := range msDef.UserInputs {
			if ui.DefaultValue == "" {
				foundNonDefault = true
				vars[ui.Name] = ""
			}
		}

		if foundNonDefault {
			skelVarConfig := register.MicroWork{
				Org:          org,
				Url:          msDef.SpecRef,
				VersionRange: msDef.Version,
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
	os.Setenv("HZN_DONT_SUBST_ENV_VARS", envVarSetting) // restore this setting

	return nil
}

func UpdateDependencyFile(homeDirectory string, msDef *cliexchange.MicroserviceFile) error {

	// Create the dependency filename.
	re := regexp.MustCompile(`^[A-Za-z0-9+.-]*?://`)
	url2 := re.ReplaceAllLiteralString(cliutils.ExpandEnv(msDef.SpecRef), "")
	re = regexp.MustCompile(`[$!*,;/?@&~=%]`)
	url3 := re.ReplaceAllLiteralString(url2, "-")

	fileName := fmt.Sprintf("%v_%v.%v", url3, cliutils.ExpandEnv(msDef.Version), MICROSERVICE_DEFINITION_FILE)

	filePath := path.Join(homeDirectory, DEFAULT_DEPENDENCY_DIR)
	if err := CreateFile(filePath, fileName, msDef); err != nil {
		return err
	}

	cliutils.Verbose("Created %v/%v as a new dependency.", filePath, fileName)

	return nil
}
