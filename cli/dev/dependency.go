package dev

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/common"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/semanticversion"
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

type ServiceDependency struct {
	Service    ServiceSpec
	TopSvcRefs []*ServiceSpec // the top services that eventually reference this service
	FileInfo   os.FileInfo
}

func NewServiceDependency(service *ServiceSpec, topService *ServiceSpec, fileInfo os.FileInfo) *ServiceDependency {
	dep := new(ServiceDependency)
	dep.Service = *service
	dep.FileInfo = fileInfo
	dep.TopSvcRefs = []*ServiceSpec{topService}
	return dep
}

// Add a top service reference to this dependency
func (sd *ServiceDependency) AddTopRef(topService *ServiceSpec) {
	foundTop := false
	for _, top_temp := range sd.TopSvcRefs {
		if top_temp.Matches(*topService) {
			foundTop = true
		}
	}
	if !foundTop {
		sd.TopSvcRefs = append(sd.TopSvcRefs, topService)
	}
}

func (sd ServiceDependency) String() string {
	return fmt.Sprintf("{Service: %v, TopSvcRefs: %v, FileInfo: %v}", sd.Service, sd.TopSvcRefs, sd.FileInfo.Name())
}

type ServiceSpec struct {
	SpecRef string
	Org     string
	Version string
	Arch    string
}

func NewServiceSpec(specref, org, version, arch string) *ServiceSpec {
	return &ServiceSpec{
		SpecRef: specref,
		Org:     org,
		Version: version,
		Arch:    arch,
	}
}

func (sp ServiceSpec) String() string {
	return fmt.Sprintf("{SpecRef: %v, Org: %v, Version: %v, Arch: %v}", sp.SpecRef, sp.Org, sp.Version, sp.Arch)
}

func (sp ServiceSpec) Matches(sp2 ServiceSpec) bool {
	return (sp.Org == "" || sp2.Org == "" || sp.Org == sp2.Org) &&
		(sp.SpecRef == "" || sp2.SpecRef == "" || sp.SpecRef == sp2.SpecRef) &&
		(sp.Version == "" || sp2.Version == "" || sp.Version == sp2.Version) &&
		(sp.Arch == "" || sp2.Arch == "" || sp.Arch == sp2.Arch)
}

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
func DependencyFetch(homeDirectory string, project string, specRef string, url string, org string, version string, arch string, userCreds string, userInputFile string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

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
		if err := fetchExchangeProjectDependency(dir, specRef, url, org, version, arch, userCreds, userInputFile); err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'dependency %v' %v", DEPENDENCY_FETCH_COMMAND, err)
		}

		// Create the right log message.
		target = createLogMessage(specRef, url, org, version, arch)
	}

	msgPrinter.Printf("New dependency created: %v .", target)
	msgPrinter.Println()
}

// This is the entry point for the hzn dev dependency list command.
func DependencyList(homeDirectory string) {

	dir, err := setup(homeDirectory, true, false, "")
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", DEPENDENCY_COMMAND, DEPENDENCY_LIST_COMMAND, err)
	}

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

func marshalListOut(deps interface{}) {
	jsonBytes, err := json.MarshalIndent(deps, "", "    ")
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, i18n.GetMessagePrinter().Sprintf("'%v %v' unable to create json object from dependencies, %v", DEPENDENCY_COMMAND, DEPENDENCY_LIST_COMMAND, err))
	}
	fmt.Printf("%v\n", string(jsonBytes))
}

// This is the entry point for the hzn dev dependency remove command.
func DependencyRemove(homeDirectory string, specRef string, url string, version string, arch string, org string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Check input parameters for correctness.
	dir, err := verifyRemoveInput(homeDirectory, specRef, url, version, arch)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'dependency %v' %v", DEPENDENCY_REMOVE_COMMAND, err)
	}

	envVarSetting := os.Getenv("HZN_DONT_SUBST_ENV_VARS")
	if err := os.Setenv("HZN_DONT_SUBST_ENV_VARS", "1"); err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Unable to set env var 'HZN_DONT_SUBST_ENV_VARS', error %v", err))
	}

	if url != "" {
		specRef = url
	}
	removeDependencyAndChildren(dir, specRef, org, version, arch)

	if err := os.Setenv("HZN_DONT_SUBST_ENV_VARS", envVarSetting); err != nil { // restore this setting
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Unable to restore env var 'HZN_DONT_SUBST_ENV_VARS', error %v", err))
	}
}

// Returns an os.FileInfo object for each dependency file. This function assumes the caller has
// determined the exact location of the files.
func GetDependencyFiles(directory string, fileSuffix string) ([]os.FileInfo, error) {

	res := make([]os.FileInfo, 0, 10)
	depPath := path.Join(directory, DEFAULT_DEPENDENCY_DIR)
	if files, err := ioutil.ReadDir(depPath); err != nil {
		return res, errors.New(i18n.GetMessagePrinter().Sprintf("unable to get list of dependency files in %v, error: %v", depPath, err))
	} else {
		for _, fileInfo := range files {
			if strings.HasSuffix(fileInfo.Name(), fileSuffix) && !fileInfo.IsDir() {
				res = append(res, fileInfo)
			}
		}
	}

	return res, nil

}

func GetServiceDependencies(directory string, deps []exchange.ServiceDependency) ([]*common.ServiceFile, error) {
	res := make([]*common.ServiceFile, 0, 10)
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
func DependenciesExists(directory string, okToCreate bool) (bool, error) {
	if exists, err := FileExists(directory, DEFAULT_DEPENDENCY_DIR); err != nil {
		return false, err
	} else if !exists && okToCreate {
		newDir := path.Join(directory, DEFAULT_DEPENDENCY_DIR)
		if err := os.MkdirAll(newDir, 0755); err != nil {
			return false, errors.New(i18n.GetMessagePrinter().Sprintf("could not create dependency directory %v, error: %v", newDir, err))
		}
	} else if !exists {
		return false, nil
	}
	return true, nil
}

// Validate that the dependencies are complete and coherent with the rest of the definitions in the project.
// Any errors will be returned to the caller.
func ValidateDependencies(directory string, userInputs *common.UserInputFile, userInputsFilePath string, projectType string, userCreds string, autoAddDep bool) error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if projectType == SERVICE_COMMAND || IsServiceProject(directory) {

		d, err := GetServiceDefinition(directory, SERVICE_DEFINITION_FILE)
		if err != nil {
			return err
		}

		// For each service definition file in the dependencies directory, verify it.
		deps, err := GetDependencyFiles(directory, SERVICE_DEFINITION_FILE)
		if err != nil {
			return err
		}

		// Validate that the project defintion's dependencies are present in the dependencies directory.
		hasNewDepFile := false
		for _, rs := range d.RequiredServices {
			found := false
			for _, fileInfo := range deps {
				if dDef, err := GetServiceDefinition(path.Join(directory, DEFAULT_DEPENDENCY_DIR), fileInfo.Name()); err != nil {
					return errors.New(msgPrinter.Sprintf("dependency validation failed, unable to read %v, error: %v", fileInfo.Name(), err))
				} else if vRange, err := semanticversion.Version_Expression_Factory(rs.VersionRange); err != nil {
					return errors.New(msgPrinter.Sprintf("dependency validation failed, dependency %v has an invalid version %v, error: %v", fileInfo.Name(), rs.Version, err))
				} else if inRange, err := vRange.Is_within_range(dDef.Version); err != nil {
					return errors.New(msgPrinter.Sprintf("dependency validation failed, unable to verify version range %v is within required range %v, error: %v", dDef.Version, vRange.Get_expression(), err))
				} else if inRange {
					found = true
					break
				}
			}
			if !found {
				if autoAddDep {
					DependencyFetch(directory, "", "", rs.URL, rs.Org, rs.VersionRange, rs.Arch, userCreds, userInputsFilePath)
					hasNewDepFile = true
				} else {
					return errors.New(msgPrinter.Sprintf("dependency %v at version %v does not exist in %v.", rs.URL, rs.VersionRange, path.Join(directory, DEFAULT_DEPENDENCY_DIR)))
				}
			}
		}

		// refetch the service definition and user inputs
		if hasNewDepFile {
			deps, err = GetDependencyFiles(directory, SERVICE_DEFINITION_FILE)
			if err != nil {
				return err
			}
			userInputs, _, err = GetUserInputs(directory, userInputsFilePath)
			if err != nil {
				return err
			}
		}

		for _, fileInfo := range deps {
			if err := ValidateServiceDefinition(path.Join(directory, DEFAULT_DEPENDENCY_DIR), fileInfo.Name()); err != nil {
				return errors.New(msgPrinter.Sprintf("dependency %v did not validate, error: %v", fileInfo.Name(), err))
			} else if err := ValidateService(directory, fileInfo, userInputs, userInputsFilePath); err != nil {
				return errors.New(msgPrinter.Sprintf("dependency %v did not validate, error: %v", fileInfo.Name(), err))
			}
		}
	}

	return nil
}

func ValidateService(directory string, fInfo os.FileInfo, userInputs *common.UserInputFile, userInputsFilePath string) error {
	d, err := GetServiceDefinition(path.Join(directory, DEFAULT_DEPENDENCY_DIR), fInfo.Name())
	if err != nil {
		return err
	}

	// Userinputs from the dependency without a default value must be set in the userinput file.
	return validateDependencyUserInputs(d, d.GetUserInputs(), userInputs.Services, userInputsFilePath)
}

func validateDependencyUserInputs(d common.AbstractServiceFile, uis []exchange.UserInput, configUserInputs []policy.AbstractUserInput, userInputsFilePath string) error {
	for _, ui := range uis {
		if ui.DefaultValue == "" {
			found := false
			for _, msUI := range configUserInputs {
				if d.GetURL() == msUI.GetServiceUrl() && (d.GetOrg() == "" || msUI.GetServiceOrgid() == "" || d.GetOrg() == msUI.GetServiceOrgid()) {
					if _, ok := msUI.GetInputMap()[ui.Name]; ok {
						found = true
						break
					}
				}
			}
			if !found {
				return errors.New(i18n.GetMessagePrinter().Sprintf("variable %v has no default and must be specified in %v", ui.Name, userInputsFilePath))
			}
		}
	}
	return nil
}

func verifyFetchInput(homeDirectory string, project string, specRef string, url string, org string, version string, arch string, userCreds string) (string, error) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

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
		return "", errors.New(msgPrinter.Sprintf("--specRef is mutually exclusive with --url."))
	} else if project != "" && (specRef != "" || org != "" || url != "") {
		return "", errors.New(msgPrinter.Sprintf("--project is mutually exclusive with --specRef, --org and --url."))
	} else if project == "" && specRef == "" && org == "" && url == "" {
		return "", errors.New(msgPrinter.Sprintf("one of --project, or --specRef and --org, or --url and --org must be specified."))
	} else if (specRef != "" && org == "") || (specRef == "" && org != "" && url == "") || (url != "" && org == "") {
		return "", errors.New(msgPrinter.Sprintf("either --specRef and --org, or --url and --org must be specified."))
	}

	// Verify that the inputs match with the project type.
	if specRef != "" && IsServiceProject(dir) {
		return "", errors.New(msgPrinter.Sprintf("use --url with service projects."))
	}

	// Verify that if --project was specified, it points to a valid horizon project directory.
	if project != "" {
		if !IsServiceProject(project) {
			return "", errors.New(msgPrinter.Sprintf("--project %v does not contain Horizon service metadata.", project))
		} else {
			if err := ValidateServiceDefinition(project, SERVICE_DEFINITION_FILE); err != nil {
				return "", err
			}
		}
	}

	cliutils.Verbose(msgPrinter.Sprintf("Reading Horizon metadata from %s", dir))

	return dir, nil
}

func verifyRemoveInput(homeDirectory string, specRef string, url string, version string, arch string) (string, error) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Shut off the Anax runtime logging.
	flag.Set("v", "0")

	// Verify that the environment and inputs are usable.
	dir, err := VerifyEnvironment(homeDirectory, true, false, "")
	if err != nil {
		return "", err
	}

	// Valid inputs are specRef with the others being optional.
	if specRef == "" && url == "" {
		return "", errors.New(msgPrinter.Sprintf("--specRef or --url is required for remove."))
	} else if specRef != "" && url != "" {
		return "", errors.New(msgPrinter.Sprintf("--specRef and --url are mutually exclusive."))
	}

	cliutils.Verbose(msgPrinter.Sprintf("Reading Horizon metadata from %s", dir))

	return dir, nil
}

// The caller is trying to use a local project (i.e. a project that is on the same machine) as a dependency.
// If the dependency is a local project then we can validate it and copy the project metadata.
func fetchLocalProjectDependency(homeDirectory string, project string, userInputFile string) error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Get the setup info and context for running the command.
	dir, err := setup(homeDirectory, true, false, "")
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", DEPENDENCY_COMMAND, DEPENDENCY_FETCH_COMMAND, err)
	}

	// If the dependent project is not validate-able then we cant reliably use it as a dependency.
	if err := AbstractServiceValidation(project); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", DEPENDENCY_COMMAND, DEPENDENCY_FETCH_COMMAND, err)
	}

	CommonProjectValidation(project, userInputFile, DEPENDENCY_COMMAND, DEPENDENCY_FETCH_COMMAND, "", false)

	msgPrinter.Printf("Service project %v verified.", dir)
	msgPrinter.Println()

	// The rest of this function gets the dependency's user input and adds it to this project's user input, and it reads
	// this project's workload definition and updates it with the reference to the ms. In the files that are read and
	// then written we want those to preserve the env vars as env vars.
	envVarSetting := os.Getenv("HZN_DONT_SUBST_ENV_VARS")
	if err := os.Setenv("HZN_DONT_SUBST_ENV_VARS", "0"); err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Unable to set env var 'HZN_DONT_SUBST_ENV_VARS' to zero, error %v", err))
	}

	// get original env vars
	orig_env_vars := cliconfig.GetEnvVars()

	// get configuration file under the same directory and export the variables as env vars
	hzn_vars := map[string]string{}
	metadata_vars := map[string]string{}
	proj_config_file, err := cliconfig.GetProjectConfigFile(project)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Failed to get the hzn.json configuration file under %v directory. Error: %v", project, err))
	}
	// make sure the dependency files are expended with the env vars of their own config file.
	if proj_config_file != "" {
		hzn_vars, metadata_vars, err = cliconfig.SetEnvVarsFromConfigFile(proj_config_file, orig_env_vars, true)
		if err != nil && !os.IsNotExist(err) {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Failed to set the environment variables from configuration file %v. Error: %v", proj_config_file, err))
		}
	}

	// Pull the metadata from the dependent project. Log the filesystem location of the dependent metadata.
	if absProject, err := filepath.Abs(project); err != nil {
		return err
	} else {
		cliutils.Verbose(msgPrinter.Sprintf("Reading Horizon metadata from dependency: %v", absProject))
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

	cliutils.Verbose(msgPrinter.Sprintf("Found dependency %v, Org: %v", sDef.GetURL(), sDef.GetOrg()))

	// restore the env vars
	if proj_config_file != "" {
		err = cliconfig.RestoreEnvVars(orig_env_vars, hzn_vars, metadata_vars)
		if err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Failed to restore the environment variables. %v", err))
		}
	}

	if err := os.Setenv("HZN_DONT_SUBST_ENV_VARS", "1"); err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Unable to set env var 'HZN_DONT_SUBST_ENV_VARS', error %v", err))
	}

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
			if len(depGlobal.ServiceSpecs) == 0 {
				depGlobal.ServiceSpecs = append(depGlobal.ServiceSpecs, *persistence.NewServiceSpec(sDef.GetURL(), sDef.GetOrg()))
			}
			currentUIs.Global = append(currentUIs.Global, depGlobal)
		}
	}

	// Update the user input file in the filesystem.
	if err := CreateUserInputFile(homeDirectory, currentUIs); err != nil {
		return err
	}

	cliutils.Verbose(msgPrinter.Sprintf("Updated %v/%v with the dependency's variable and global attribute configuration.", homeDirectory, USERINPUT_FILE))
	if err := os.Setenv("HZN_DONT_SUBST_ENV_VARS", envVarSetting); err != nil { // restore this setting
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Unable to restore env var 'HZN_DONT_SUBST_ENV_VARS', error %v", err))
	}

	return nil
}

func fetchExchangeProjectDependency(homeDirectory string, specRef string, url string, org string, version string, arch string, userCreds string, userInputFile string) error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Pull the metadata from the exchange, including any of this dependency's dependencies.
	sDef, err := getExchangeDefinition(homeDirectory, specRef, url, org, "", arch, userCreds, userInputFile)
	if err != nil {
		return err
	}

	// Harden the new dependency in the file.
	if err := UpdateDependencyFile(homeDirectory, sDef); err != nil {
		return err
	}

	//Add the service to the service.definition.json file and userinput.json file
	if err := UpdateServiceDefandUserInputFile(homeDirectory, sDef, false); err != nil {
		return err
	}

	msgPrinter.Printf("To ensure that the dependency operates correctly, please add variable values to the userinput.json file if this service needs any.")
	msgPrinter.Println()
	return nil
}

// update the service definition file and userinput file with this dependent service
func UpdateServiceDefandUserInputFile(homeDirectory string, sDef common.AbstractServiceFile, skipServcieDef bool) error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// gets the dependency's user input and adds it to this project's user input, and it reads
	// this project's workload definition and updates it with the reference to the ms. In the files that are read and
	// then written we want those to preserve the env vars as env vars.
	envVarSetting := os.Getenv("HZN_DONT_SUBST_ENV_VARS")
	if err := os.Setenv("HZN_DONT_SUBST_ENV_VARS", "1"); err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Unable to set env var 'HZN_DONT_SUBST_ENV_VARS', error %v", err))
	}
	defer os.Setenv("HZN_DONT_SUBST_ENV_VARS", envVarSetting) // restore this setting

	if !skipServcieDef {
		// Update the workload definition dependencies to make sure the dependency is included. The APISpec array
		// in the workload definition is rebuilt from the dependencies.
		if err := RefreshServiceDependencies(homeDirectory, sDef); err != nil {
			return err
		}
	}

	// Loop through this project's variable configurations and add skeletal non-default variables that
	// are defined by the new dependency.
	foundUIs := false
	varConfigs, err := GetUserInputsVariableConfiguration(homeDirectory, "")
	if err != nil {
		return err
	}
	for _, currentUI := range varConfigs {
		if currentUI.GetServiceUrl() == sDef.GetURL() && currentUI.GetServiceOrgid() == sDef.GetOrg() && currentUI.GetServiceVersionRange() == sDef.GetVersion() {
			// The new dependency already has userinputs configured in this project.
			cliutils.Verbose(msgPrinter.Sprintf("The current project already has userinputs defined for this dependency."))
			foundUIs = true
			break
		}
	}

	// If there are no variables already defined, and there are non-defaulted variables, then add skeletal variables.
	if !foundUIs {
		foundNonDefault := false
		inputs := make([]policy.Input, 0, len(sDef.GetUserInputs()))
		for _, ui := range sDef.GetUserInputs() {
			if ui.DefaultValue == "" {
				foundNonDefault = true
				inputs = append(inputs, policy.Input{
					Name:  ui.Name,
					Value: "",
				})
			}
		}

		if foundNonDefault {
			skelVarConfig := policy.UserInput{
				ServiceOrgid:        sDef.GetOrg(),
				ServiceUrl:          sDef.GetURL(),
				ServiceArch:         sDef.GetArch(),
				ServiceVersionRange: sDef.GetVersion(),
				Inputs:              inputs,
			}
			if err := SetUserInputsVariableConfiguration(homeDirectory, sDef, []policy.AbstractUserInput{skelVarConfig}); err != nil {
				return err
			}

			cliutils.Verbose(msgPrinter.Sprintf("Updated %v/%v with the dependency's variable configuration.", homeDirectory, USERINPUT_FILE))
		}
	}

	return nil
}

func getExchangeDefinition(homeDirectory string, specRef string, surl string, org string, version string, arch string, userCreds string, userInputFile string) (common.AbstractServiceFile, error) {

	if ex, err := ServiceDefinitionExists(homeDirectory); !ex || err != nil {
		return nil, errors.New(i18n.GetMessagePrinter().Sprintf("no service definition config file found in project"))
	} else if ex, err := DependenciesExists(homeDirectory, true); !ex || err != nil {
		return nil, errors.New(i18n.GetMessagePrinter().Sprintf("no dependency directory found in project"))
	} else {
		return getServiceDefinition(homeDirectory, surl, org, version, arch, userCreds)
	}
}

func UpdateDependencyFile(homeDirectory string, sDef common.AbstractServiceFile) error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	fileName := createDependencyFileName(sDef.GetOrg(), sDef.GetURL(), sDef.GetVersion(), SERVICE_DEFINITION_FILE)

	filePath := path.Join(homeDirectory, DEFAULT_DEPENDENCY_DIR)
	if err := CreateFile(filePath, fileName, sDef); err != nil {
		return err
	}

	cliutils.Verbose(msgPrinter.Sprintf("Created %v/%v as a new dependency.", filePath, fileName))

	return nil
}

func createDependencyFileName(org string, url string, version string, suffix string) string {
	// Create the dependency filename.
	re := regexp.MustCompile(`^[A-Za-z0-9+.-]*?://`)
	url2 := re.ReplaceAllLiteralString(cliutils.ExpandEnv(url), "")
	re = regexp.MustCompile(`[$!*,;/?@&~=%]`)
	url3 := re.ReplaceAllLiteralString(url2, "-")

	return fmt.Sprintf("%v_%v_%v.%v", org, url3, cliutils.ExpandEnv(version), suffix)
}

// Copy the dependency files out, validate them and write them back.
func UpdateDependentDependencies(homeDirectory string, depProject string) error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

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
				return errors.New(msgPrinter.Sprintf("dependency %v did not validate, error: %v", dep.Name(), err))
			} else if err := CreateFile(path.Join(homeDirectory, DEFAULT_DEPENDENCY_DIR), dep.Name(), sDef); err != nil {
				return err
			}
		}
	}

	return nil

}

// Iterate through the dependencies of the given service and create a dependency for each one.
func getServiceDefinitionDependencies(homeDirectory string, serviceDef *common.ServiceFile, userCreds string) error {
	for _, rs := range serviceDef.RequiredServices {
		// Get the service definition for each required service. Dependencies refer to each other by version range, so the
		// service we're looking for might not be at the exact version specified in the required service element.
		if sDef, err := getServiceDefinition(homeDirectory, rs.URL, rs.Org, "", rs.Arch, userCreds); err != nil {
			return err
		} else if err := UpdateDependencyFile(homeDirectory, sDef); err != nil {
			return err
		} else if err := UpdateServiceDefandUserInputFile(homeDirectory, sDef, true); err != nil {
			return err
		}
	}
	return nil
}

func getServiceDefinition(homeDirectory, surl string, org string, version string, arch string, userCreds string) (*common.ServiceFile, error) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Construct the resource URL suffix.
	resSuffix := fmt.Sprintf("orgs/%v/services?url=%v", org, surl)
	if version != "" {
		resSuffix += fmt.Sprintf("&version=%v", version)
	}
	if arch == "" {
		arch = cutil.ArchString()
	}
	resSuffix += fmt.Sprintf("&arch=%v", arch)

	// Create an object to hold the response.
	resp := new(exchange.GetServicesResponse)

	// Call the exchange to get the service definition.
	if userCreds == "" {
		userCreds = os.Getenv(DEVTOOL_HZN_USER)
	}
	cliutils.SetWhetherUsingApiKey(userCreds)
	cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), resSuffix, cliutils.OrgAndCreds(os.Getenv(DEVTOOL_HZN_ORG), userCreds), []int{200}, resp)

	// Parse the response and extract the highest version service definition or return an error.
	var serviceDef exchange.ServiceDefinition
	var serviceId string
	if len(resp.Services) > 1 {
		highest, sDef, sId, err := exchange.GetHighestVersion(resp.Services, nil)
		if err != nil {
			return nil, err
		} else if highest == "" {
			return nil, errors.New(msgPrinter.Sprintf("unable to find highest version of %v %v in the Exchange: %v", surl, org, resp.Services))
		} else {
			serviceDef = sDef
			serviceId = sId
		}

	} else if len(resp.Services) == 0 {
		return nil, errors.New(msgPrinter.Sprintf("no services found in the Exchange."))
	} else {
		for sId, sDef := range resp.Services {
			serviceDef = sDef
			serviceId = sId
			break
		}
	}

	cliutils.Verbose(msgPrinter.Sprintf("Creating dependency on: %v, Org: %v", serviceDef, org))

	sDef_cliex := new(common.ServiceFile)

	// Get container images into the local docker
	dc := make(map[string]interface{})
	if serviceDef.Deployment != "" {
		if err := json.Unmarshal([]byte(serviceDef.Deployment), &dc); err != nil {
			return nil, errors.New(msgPrinter.Sprintf("failed to unmarshal deployment %v: %v", serviceDef.Deployment, err))
		}

		// Get this project's userinputs so that the downloader can use any special authorization attributes that might
		// be specified in the global section of the user inputs.
		currentUIs, _, err := GetUserInputs(homeDirectory, "")
		if err != nil {
			return nil, err
		}

		// Get docker auth for the service
		auth_url := fmt.Sprintf("orgs/%v/services/%v/dockauths", org, exchange.GetId(serviceId))
		docker_auths := make([]exchange.ImageDockerAuth, 0)
		cliutils.SetWhetherUsingApiKey(userCreds)
		cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), auth_url, cliutils.OrgAndCreds(os.Getenv(DEVTOOL_HZN_ORG), userCreds), []int{200, 404}, &docker_auths)

		img_auths := make([]events.ImageDockerAuth, 0)
		if docker_auths != nil {
			for _, iau_temp := range docker_auths {
				user_name := "token"
				if iau_temp.UserName != "" {
					user_name = iau_temp.UserName
				}
				img_auths = append(img_auths, events.ImageDockerAuth{Registry: iau_temp.Registry, UserName: user_name, Password: iau_temp.Token})
			}
		}
		cliutils.Verbose(msgPrinter.Sprintf("The image docker auths for the service %v/%v are: %v", org, surl, img_auths))

		cc := events.NewContainerConfig(serviceDef.Deployment, serviceDef.DeploymentSignature, "", "", "", "", img_auths)

		// get the images
		if err := getContainerImages(cc, currentUIs); err != nil {
			return nil, errors.New(msgPrinter.Sprintf("failed to get images for %v/%v: %v", org, surl, err))
		}
	}

	// Fill in the parts of the dependency that come from the service definition.
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

	// If this service has dependencies, bring them in.
	if serviceDef.HasDependencies() {
		if err := getServiceDefinitionDependencies(homeDirectory, sDef_cliex, userCreds); err != nil {
			return nil, err
		}
	}

	return sDef_cliex, nil
}

// get all the dependencies, store them in an array. Each element points to an array of top level services.
func getDependencyInfo(dir string) ([]*ServiceDependency, error) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	deps := []*ServiceDependency{}
	svc, err := GetServiceDefinition(dir, SERVICE_DEFINITION_FILE)
	if err != nil {
		return deps, err
	}

	depFiles, err := GetDependencyFiles(dir, SERVICE_DEFINITION_FILE)
	if err != nil {
		return deps, err
	}

	for _, s := range svc.RequiredServices {
		sp := NewServiceSpec(s.URL, s.Org, s.VersionRange, s.Arch)
		if err := getDependencyDependencyInfo(dir, sp, sp, depFiles, &deps); err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Unable to get dependency info for %v , error %v", sp, err))
		}
	}

	return deps, nil

}

// Recursively get dependencies of the dependency
func getDependencyDependencyInfo(dir string, spTop *ServiceSpec, sp *ServiceSpec, depFiles []os.FileInfo, deps *[]*ServiceDependency) error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// find out if the service is already in the list
	var theItem *ServiceDependency
	for _, d := range *deps {
		if d.Service.Matches(*sp) {
			theItem = d
			break
		}
	}

	// find the service file from the /dependencies directory
	for _, fileInfo := range depFiles {

		dep, err := GetServiceDefinition(path.Join(dir, DEFAULT_DEPENDENCY_DIR), fileInfo.Name())
		if err != nil {
			return err
		}

		// add this service in the dependency list
		sp_temp := NewServiceSpec(dep.GetURL(), dep.GetOrg(), dep.GetVersion(), dep.GetArch())
		if sp_temp.Matches(*sp) {
			if theItem == nil {
				foundDep := NewServiceDependency(sp_temp, spTop, fileInfo)
				(*deps) = append(*deps, foundDep)
			} else {
				theItem.AddTopRef(spTop)
			}

			// get dependent's dependencies
			for _, s := range dep.RequiredServices {
				sp_dep_temp := NewServiceSpec(s.URL, s.Org, s.VersionRange, s.Arch)
				if err := getDependencyDependencyInfo(dir, spTop, sp_dep_temp, depFiles, deps); err != nil {
					cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Unable to get dependency info for %v , error %v", sp_dep_temp, err))
				}
			}
		}
	}

	return nil
}

// Remove the service dependency and dependency's dependencies.
func removeDependencyAndChildren(dir string, specRef string, org string, version string, arch string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	topDep := NewServiceSpec(specRef, org, version, arch)
	deps, err := getDependencyInfo(dir)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("'dependency %v' failed to get a list of dependecies. Error %v", DEPENDENCY_REMOVE_COMMAND, err))
	}

	cliutils.Verbose(msgPrinter.Sprintf("All dependencies are: %v", deps))

	found := false
	for _, dep := range deps {
		for _, dep_tmp := range dep.TopSvcRefs {
			if dep_tmp.Matches(*topDep) {
				found = true
				// remove this one
				cliutils.Verbose(msgPrinter.Sprintf("Found dependency: %v", dep.FileInfo.Name()))
				if len(dep.TopSvcRefs) <= 1 {
					// the dependent service is only refrenced once, safe to remove it
					removeDependencyFromProject(dir, dep, false)
				} else if len(dep.TopSvcRefs) > 1 {
					if dep.Service.Matches(*topDep) {
						// the dep service itself is the top level dep service, so we just remove it from the RequiredServices
						// part of service.definition.json. Still keep the userinput.json and dependencies file because it has
						// other references.
						removeDependencyFromProject(dir, dep, true)
					} else {
						// keep it because it has other references
						msgPrinter.Printf("Will not remove dependency %v because it is referenced by other services.", dep.FileInfo.Name())
						msgPrinter.Println()
					}
				}
			}
		}
	}

	if !found {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("'dependency %v' dependency not found.", DEPENDENCY_REMOVE_COMMAND))
	}
}

// Remove all the references to the the given dependency from the project
// onlyRemoveFromSvcDef is true when the top level dependent service is reference by another dependent service.
// This is a corner case, but we need to handel it.
func removeDependencyFromProject(dir string, sd *ServiceDependency, onlyRemoveFromSvcDef bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Use error string to accumulate the errors so that it can move on to clean other parts even when one part failed.
	err_string := ""

	// open the service dependency file
	theDep, err := GetServiceDefinition(path.Join(dir, DEFAULT_DEPENDENCY_DIR), sd.FileInfo.Name())
	if err != nil {
		err_string += msgPrinter.Sprintf("Could not read the dependency file %v. Error: %v\n", sd.FileInfo.Name(), err)
	}

	// Update the service definition with the new dependencies.
	if err := RemoveServiceDependency(dir, theDep); err != nil {
		err_string += msgPrinter.Sprintf("Error updating project definition: %v\n", err)
	}

	if !onlyRemoveFromSvcDef {
		// We know which dependency to remove, so remove it.
		if err := os.Remove(path.Join(dir, DEFAULT_DEPENDENCY_DIR, sd.FileInfo.Name())); err != nil {
			err_string += msgPrinter.Sprintf("Dependency %v could not be removed. Error: %v\n", sd.FileInfo.Name(), err)
		}

		// Update the default userinputs removing any configured variables.
		if err := RemoveConfiguredVariables(dir, theDep); err != nil {
			err_string += msgPrinter.Sprintf("Error updating userinputs: %v", err)
		}
	}

	if err_string != "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'dependency %v' %v", DEPENDENCY_REMOVE_COMMAND, err_string)
	}

	// Create the right log message.
	if len(sd.TopSvcRefs) > 0 && sd.TopSvcRefs[0].Matches(sd.Service) {
		msgPrinter.Printf("Removed dependency %v.", createLogMessage(sd.Service.SpecRef, "", sd.Service.Org, sd.Service.Version, sd.Service.Arch))
		msgPrinter.Println()
	} else {
		msgPrinter.Printf("Removed dependency's dependency %v.", createLogMessage(sd.Service.SpecRef, "", sd.Service.Org, sd.Service.Version, sd.Service.Arch))
		msgPrinter.Println()
	}
}
