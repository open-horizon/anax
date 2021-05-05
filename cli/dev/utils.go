package dev

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/common"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/container"
	"github.com/open-horizon/anax/containermessage"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/imagefetch"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/semanticversion"
	"github.com/satori/go.uuid"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// Constants that hold the name of env vars used with the context of the hzn dev commands.
const DEVTOOL_HZN_ORG = "HZN_ORG_ID"
const DEVTOOL_HZN_USER = "HZN_EXCHANGE_USER_AUTH"
const DEVTOOL_HZN_EXCHANGE_URL = "HZN_EXCHANGE_URL"
const DEVTOOL_HZN_DEVICE_ID = "HZN_DEVICE_ID"
const DEVTOOL_HZN_PATTERN = "HZN_PATTERN"

const DEVTOOL_HZN_FSS_IMAGE_TAG = "HZN_DEV_FSS_IMAGE_TAG"
const DEVTOOL_HZN_FSS_IMAGE_REPO = "HZN_DEV_FSS_IMAGE_REPO"
const DEVTOOL_HZN_FSS_CSS_PORT = "HZN_DEV_FSS_CSS_PORT"
const DEVTOOL_HZN_FSS_WORKING_DIR = "HZN_DEV_FSS_WORKING_DIR"
const DEFAULT_DEVTOOL_HZN_FSS_WORKING_DIR = "/tmp/hzndev/"

const DEFAULT_WORKING_DIR = "horizon"
const DEFAULT_DEPENDENCY_DIR = "dependencies"

// The current working directory could be specified via input (as an absolute or relative path) or
// it could be defaulted if there is no input. If it must exist but does not, return an error.
func GetWorkingDir(dashD string, verifyExists bool) (string, error) {
	dir := dashD
	var err error
	if dir == "" {
		dir = DEFAULT_WORKING_DIR
	}

	if dir, err = filepath.Abs(dir); err != nil {
		return "", err
	} else if verifyExists {
		if _, err := os.Stat(dir); err != nil {
			return "", err
		}
	}

	return dir, nil
}

// Create the working directory if needed.
func CreateWorkingDir(dir string) error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Create the working directory with the dependencies and pattern directories in one shot. If it already exists, just keep going.
	newDepDir := path.Join(dir, DEFAULT_DEPENDENCY_DIR)
	if _, err := os.Stat(newDepDir); os.IsNotExist(err) {
		if err := os.MkdirAll(newDepDir, 0755); err != nil {
			return errors.New(msgPrinter.Sprintf("could not create directory %v, error: %v", newDepDir, err))
		}
	} else if err != nil {
		return errors.New(msgPrinter.Sprintf("could not get status of directory %v, error: %v", newDepDir, err))
	}

	cliutils.Verbose(msgPrinter.Sprintf("Using working directory: %v", dir))
	return nil
}

// Check for a file's existence or error out of the command. This is just a way to consolidate the error handling because
// we have several files that we're dealing with.
func FileNotExist(dir string, cmd string, fileName string, check func(string) (bool, error)) {
	if exists, err := check(dir); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v' %v", cmd, err)
	} else if exists {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v' %v", cmd, i18n.GetMessagePrinter().Sprintf("horizon project in %v already contains %v.", dir, fileName))
	}
}

// Check for file existence and return any errors.
func FileExists(directory string, fileName string) (bool, error) {
	filePath := path.Join(directory, fileName)
	if _, err := os.Stat(filePath); err != nil && !os.IsNotExist(err) {
		return false, errors.New(i18n.GetMessagePrinter().Sprintf("error checking for %v: %v", fileName, err))
	} else if err == nil {
		return true, nil
	}
	return false, nil
}

// This function demarshals the file bytes into the input obj structure. The contents of what obj
// points to will be modified by this function.
func GetFile(directory string, fileName string, obj interface{}) error {
	filePath := path.Join(directory, fileName)

	fileBytes := cliutils.ReadJsonFile(filePath)
	if err := json.Unmarshal(fileBytes, obj); err != nil {
		return errors.New(i18n.GetMessagePrinter().Sprintf("failed to unmarshal %s, error: %v", filePath, err))
	}
	return nil
}

// This function takes one of the project json objects and writes it to a file in the project.
func CreateFile(directory string, fileName string, obj interface{}) error {
	// Convert the object to JSON and write it.
	filePath := path.Join(directory, fileName)
	if jsonBytes, err := json.MarshalIndent(obj, "", "    "); err != nil {
		return errors.New(i18n.GetMessagePrinter().Sprintf("failed to create json object for %v, error: %v", fileName, err))
	} else if err := ioutil.WriteFile(filePath, jsonBytes, 0664); err != nil {
		return errors.New(i18n.GetMessagePrinter().Sprintf("unable to write json object for %v to file %v, error: %v", fileName, filePath, err))
	} else {
		return nil
	}
}

// This function takes the common UserInput object marshals it into JSON and writes it to a file in the project.
func CreateUserInputFile(directory string, ui *common.UserInputFile) error {
	// Convert the object to JSON and write it.
	filePath := path.Join(directory, USERINPUT_FILE)
	if bytes, err := ui.GetOutputJsonBytes(false); err != nil {
		return err
	} else if err := ioutil.WriteFile(filePath, bytes, 0664); err != nil {
		return errors.New(i18n.GetMessagePrinter().Sprintf("unable to write json object for userinput to file %v, error: %v", filePath, err))
	}
	return nil
}

// Common verification before executing a sub command.
func VerifyEnvironment(homeDirectory string, mustExist bool, needExchange bool, userCreds string) (string, error) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Make sure the env vars needed by the dev tools are setup
	if needExchange && userCreds != "" {
		id, _ := cliutils.SplitIdToken(userCreds) // only look for the / in the id, because the token is more likely to have special chars
		if !strings.Contains(id, "/") && os.Getenv(DEVTOOL_HZN_ORG) == "" {
			return "", errors.New(msgPrinter.Sprintf("Must set environment variable %v or specify the user as 'org/user' on the --user-pw flag", DEVTOOL_HZN_ORG))
		}
	} else if needExchange && userCreds == "" {
		id, _ := cliutils.SplitIdToken(os.Getenv(DEVTOOL_HZN_USER)) // only look for the / in the id, because the token is more likely to have special chars
		if !strings.Contains(id, "/") && os.Getenv(DEVTOOL_HZN_ORG) == "" {
			return "", errors.New(msgPrinter.Sprintf("Must set environment variable %v or specify the user as 'org/user' on the --user-pw flag", DEVTOOL_HZN_ORG))
		}
	}

	if needExchange && os.Getenv(DEVTOOL_HZN_USER) == "" && userCreds == "" {
		return "", errors.New(msgPrinter.Sprintf("Must set environment variable %v or specify user exchange credentials with --user-pw", DEVTOOL_HZN_USER))
	} else if os.Getenv(DEVTOOL_HZN_EXCHANGE_URL) == "" {
		exchangeUrl := cliutils.GetExchangeUrl()
		if exchangeUrl != "" {
			if err := os.Setenv(DEVTOOL_HZN_EXCHANGE_URL, exchangeUrl); err != nil {
				cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Unable to set env var %v to %v, error %v", DEVTOOL_HZN_EXCHANGE_URL, exchangeUrl, err))
			}
		} else {
			return "", errors.New(msgPrinter.Sprintf("Environment variable %v must be set.", DEVTOOL_HZN_EXCHANGE_URL))
		}
	}

	// Get the directory we're working in
	dir, err := GetWorkingDir(homeDirectory, mustExist)
	if err != nil {
		return "", errors.New(msgPrinter.Sprintf("project has no horizon metadata directory. Use hzn dev to create a new project. Error: %v", err))
	} else {
		return dir, nil
	}

}

// Indicates whether or not the given project is a service project.
func IsServiceProject(directory string) bool {
	if ex, err := ServiceDefinitionExists(directory); !ex || err != nil {
		return false
	} else if ex, err := DependenciesExists(directory, true); !ex || err != nil {
		return false
	}
	return true
}

// autoAddDep -- if true, the dependent services will be automatically added if they can be found from the exchange
func CommonProjectValidation(dir string, userInputFile string, projectType string, cmd string, userCreds string, autoAddDep bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Get the Userinput file, so that we can validate it.
	userInputs, userInputsFilePath, uierr := GetUserInputs(dir, userInputFile)
	if uierr != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", projectType, cmd, uierr)
	}

	// Validate Dependencies
	if derr := ValidateDependencies(dir, userInputs, userInputsFilePath, projectType, userCreds, autoAddDep); derr != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("'%v %v' project does not validate. %v", projectType, cmd, derr))
	}

	if verr := ValidateUserInput(userInputs, dir, userInputsFilePath, projectType); verr != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("'%v %v' project does not validate. %v ", projectType, cmd, verr))
	}
}

// Validate that the input list of files actually exist.
func FileValidation(configFiles []string, configType string, projectType string, cmd string) []string {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if len(configFiles) > 0 && configType == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("'%v %v' Must specify configuration file type (-t) when a configuration file is specified (-m).", projectType, cmd))
	}

	absoluteFiles := make([]string, 0, 5)

	for _, fileRef := range configFiles {
		if absFileRef, err := filepath.Abs(fileRef); err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("'%v %v' configuration file %v error %v", projectType, cmd, fileRef, err))
		} else if _, err := os.Stat(absFileRef); err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("'%v %v' configuration file %v error %v", projectType, cmd, fileRef, err))
		} else {
			absoluteFiles = append(absoluteFiles, absFileRef)
		}
	}

	return absoluteFiles
}

func AbstractServiceValidation(dir string) error {
	if verr := ValidateServiceDefinition(dir, SERVICE_DEFINITION_FILE); verr != nil {
		return errors.New(i18n.GetMessagePrinter().Sprintf("project does not validate. %v ", verr))
	}
	return nil
}

// Sort of like a constructor, it creates an in memory object except that it is created from a service
// definition config file in the current project. This function assumes the caller has determined the exact location of the file.
// This function also assumes that the project pointed to by the directory parameter is assumed to contain the kind of definition
// the caller expects.
func GetAbstractDefinition(directory string) (common.AbstractServiceFile, error) {

	tryDefinitionName := SERVICE_DEFINITION_FILE
	res := new(common.ServiceFile)

	// GetFile will write to the res object, demarshalling the bytes into a json object that can be returned.
	if err := GetFile(directory, tryDefinitionName, res); err != nil {
		return nil, err
	}
	return res, nil

}

// Common setup processing for handling workload related commands.
func setup(homeDirectory string, mustExist bool, needExchange bool, userCreds string) (string, error) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Shut off the Anax runtime logging.
	flag.Set("v", "0")

	// Verify that the environment and inputs are usable.
	dir, err := VerifyEnvironment(homeDirectory, mustExist, needExchange, userCreds)
	if err != nil {
		return "", err
	}

	cliutils.Verbose(msgPrinter.Sprintf("Reading Horizon metadata from %s", dir))

	// Verify that the project is a service project.
	if !IsServiceProject(dir) {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("project in %v is not a horizon project.", dir))
	}

	return dir, nil
}

func makeByValueAttributes(attrs []persistence.Attribute) []persistence.Attribute {
	byValueAttrs := make([]persistence.Attribute, 0, 10)
	for _, a := range attrs {
		switch a.(type) {
		case *persistence.HAAttributes:
			p := a.(*persistence.HAAttributes)
			byValueAttrs = append(byValueAttrs, *p)
		case *persistence.HTTPSBasicAuthAttributes:
			p := a.(*persistence.HTTPSBasicAuthAttributes)
			byValueAttrs = append(byValueAttrs, *p)
		case *persistence.DockerRegistryAuthAttributes:
			p := a.(*persistence.DockerRegistryAuthAttributes)
			byValueAttrs = append(byValueAttrs, *p)
		}
	}
	return byValueAttrs
}

// Create the environment variable map needed by the container worker to hold the environment variables that are passed to the
// workload container.
func createEnvVarMap(agreementId string,
	workloadPW string,
	global []common.GlobalSet,
	msURL string,
	configVar map[string]interface{},
	defaultVar []exchange.UserInput,
	org string,
	cw *container.ContainerWorker,
	attrConverter func(attributes []persistence.Attribute,
		envvars map[string]string,
		prefix string,
		defaultRAM int64,
		nodePol *externalpolicy.ExternalPolicy, isCluster bool) (map[string]string, error),
) (map[string]string, error) {

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// First, add in the Horizon platform env vars.
	envvars := make(map[string]string)

	// Set the env vars that will be passed to the services.
	cutil.SetPlatformEnvvars(envvars,
		config.ENVVAR_PREFIX,
		agreementId,
		GetNodeId(),
		org,
		workloadPW,
		os.Getenv(DEVTOOL_HZN_EXCHANGE_URL),
		os.Getenv(DEVTOOL_HZN_PATTERN),
		cw.Config.GetFileSyncServiceProtocol(),
		cw.Config.GetFileSyncServiceAPIListen(),
		strconv.Itoa(int(cw.Config.GetFileSyncServiceAPIPort())))

	// Second, add the Horizon system env vars. Some of these can come from the global section of a user inputs file. To do this we have to
	// convert the attributes in the userinput file into API attributes so that they can be validity checked. Then they are converted to
	// persistence attributes so that they can be further converted to environment variables. This is the progression that anax uses when
	// running real workloads so the same progression is used here.

	// The set of global attributes in the project's userinput file might not all be applicable to all services, so we will
	// create a shortened list of global attribute that only apply to this service.
	shortGlobals := make([]common.GlobalSet, 0, 10)
	for _, inputGlobal := range global {
		if len(inputGlobal.ServiceSpecs) == 0 || (inputGlobal.ServiceSpecs[0].Url == msURL && inputGlobal.ServiceSpecs[0].Org == org) {
			shortGlobals = append(shortGlobals, inputGlobal)
		}
	}

	// Now convert the reduced global attribute set to API attributes.
	attrs, err := GlobalSetAsAttributes(shortGlobals)
	if err != nil {
		return nil, errors.New(msgPrinter.Sprintf("%v has error: %v ", USERINPUT_FILE, err))
	}

	// Third, add in default system attributes if not already present.
	attrs = api.FinalizeAttributesSpecifiedInService(persistence.NewServiceSpec(msURL, org), attrs)

	cliutils.Verbose(msgPrinter.Sprintf("Final Attributes: %v", attrs))

	// The conversion to persistent attributes produces an array of pointers to attributes, we need a by-value
	// array of attributes because that's what the functions which convert attributes to env vars expect. This is
	// because at runtime, the attributes are serialized to a database and then read out again before converting to env vars.

	byValueAttrs := makeByValueAttributes(attrs)

	// Get the node policy info
	nodePolicy := externalpolicy.ExternalPolicy{}
	cliutils.HorizonGet("node/policy", []int{200}, &nodePolicy, true)
	// Fourth, convert all attributes to system env vars.
	var cerr error
	envvars, cerr = attrConverter(byValueAttrs, envvars, config.ENVVAR_PREFIX, cw.Config.Edge.DefaultServiceRegistrationRAM, &nodePolicy, false)
	if cerr != nil {
		return nil, errors.New(msgPrinter.Sprintf("global attribute conversion error: %v", cerr))
	}

	// Last, now that the system and attribute based env vars are in place, we can convert the workload defined variables to env
	// vars and add them into the env var map.
	// Add in default variables from the workload definition.
	AddDefaultUserInputs(defaultVar, envvars)

	// Then add in the configured variable values from the workload section of the user input file.
	if err := AddConfiguredUserInputs(configVar, envvars); err != nil {
		return nil, err
	}

	return envvars, nil
}

func createContainerWorker() (*container.ContainerWorker, error) {

	workloadStorageDir := "/tmp/hzn"
	if err := os.MkdirAll(workloadStorageDir, 0755); err != nil {
		return nil, err
	}

	config := &config.HorizonConfig{
		Edge: config.Config{
			ServiceStorage:                workloadStorageDir,
			DefaultServiceRegistrationRAM: 0,
			FileSyncService: config.FSSConfig{
				AuthenticationPath: path.Join(GetDevWorkingDirectory(), "auth"),
				APIListen:          path.Join(GetDevWorkingDirectory(), "essapi.sock"),
				APIProtocol:        "unix",
			},
		},
		AgreementBot:  config.AGConfig{},
		Collaborators: config.Collaborators{},
	}

	// Create the folder for SSL certificates (under authentication path)
	if err := os.MkdirAll(config.GetESSSSLClientCertPath(), 0755); err != nil {
		return nil, err
	}

	return container.CreateCLIContainerWorker(config)
}

// This function is used to setup context to execute a service container.
func CommonExecutionSetup(homeDirectory string, userInputFile string, projectType string, cmd string) (string, *common.UserInputFile, *container.ContainerWorker) {

	// Get the setup info and context for running the command.
	dir, err := setup(homeDirectory, true, false, "")
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", projectType, cmd, err)
	}

	// Get the userinput file, so that we can get the userinput variables.
	userInputs, _, err := GetUserInputs(dir, userInputFile)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", projectType, cmd, err)
	}

	// Create the containerWorker
	cw, cerr := createContainerWorker()
	if cerr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, i18n.GetMessagePrinter().Sprintf("'%v %v' unable to create Container Worker, %v", projectType, cmd, cerr))
	}

	return dir, userInputs, cw
}

// This function is used to clear all service's files & folders (such as UDS socket and auth folder) that will not be needed anymore
func ExecutionTearDown(cw *container.ContainerWorker) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Remove the UDS socket file
	err := os.RemoveAll(cw.Config.GetFileSyncServiceAPIListen())
	if err != nil {
		msgPrinter.Printf("Failed to remove the UDS socket file: %v", err)
		msgPrinter.Println()
	}

	// Clear the File Sync Service API authentication credential folder
	if err := os.RemoveAll(cw.GetAuthenticationManager().AuthPath); err != nil {
		msgPrinter.Printf("Failed to remove FSS Authentication credential folder, error: %v", err)
		msgPrinter.Println()
	}
}

func findContainers(serviceName string, instancePrefix string, cw *container.ContainerWorker) ([]docker.APIContainers, error) {
	dcService := docker.ListContainersOptions{
		All: true,
		Filters: map[string][]string{
			"label": []string{
				fmt.Sprintf("%v.service_name=%v", container.LABEL_PREFIX, serviceName),
				fmt.Sprintf("%v.dev_service", container.LABEL_PREFIX),
			},
		},
	}

	serviceContainers, err := cw.GetClient().ListContainers(dcService)
	if err != nil {
		return nil, errors.New(i18n.GetMessagePrinter().Sprintf("unable to list containers, %v", err))
	}

	// Ensure that the container being returned belongs to the right service instance.
	containers := make([]docker.APIContainers, 0)
	for _, sc := range serviceContainers {
		if instanceName, ok := sc.Labels[container.LABEL_PREFIX+".agreement_id"]; ok {
			if instancePrefix == "" || instancePrefix != "" && strings.HasPrefix(instanceName, instancePrefix) {
				containers = append(containers, sc)
			}
		}
	}

	return containers, nil
}

func getContainerNetworks(depConfig *common.DeploymentConfig, instancePrefix string, cw *container.ContainerWorker) (map[string]string, error) {
	containerNetworks := make(map[string]string)
	for serviceName, _ := range depConfig.Services {
		containers, err := findContainers(serviceName, instancePrefix, cw)
		if err != nil {
			return nil, errors.New(i18n.GetMessagePrinter().Sprintf("unable to list existing containers: %v", err))
		}

		for _, msc := range containers {
			if agreementId, ok := msc.Labels[container.LABEL_PREFIX+".agreement_id"]; ok {
				if nw, ok := msc.Networks.Networks[agreementId]; ok {
					containerNetworks[agreementId] = nw.NetworkID
					cliutils.Verbose(i18n.GetMessagePrinter().Sprintf("Found main network for service %v, %v", agreementId, nw))
				}
			}
		}
	}
	return containerNetworks, nil
}

func ProcessStartDependencies(dir string, deps []*common.ServiceFile, globals []common.GlobalSet, configUserInputs []policy.AbstractUserInput, cw *container.ContainerWorker, serviceInstance string) (map[string]string, error) {

	// Collect all the service networks that have to be connected to the caller's container.
	ms_networks := make(map[string]string)

	for _, depDef := range deps {

		msn, startErr := startDependent(dir, depDef, globals, configUserInputs, cw, serviceInstance)

		// If there were errors, cleanup any services that are already started.
		if startErr != nil {

			// Stop any services that might already be started.
			ServiceStopTest(dir)

			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, i18n.GetMessagePrinter().Sprintf("'%v %v' %v for dependency %v", SERVICE_COMMAND, SERVICE_START_COMMAND, startErr, depDef.URL))

		} else {

			// When the dependency is started, a default network is created. It is returned in a name to id mapping. Dependencies need to be connected
			// to parent specific networks, so get rid of the default network and replace with a parent specific network. If the dependency didn't
			// need to be started, then no networks are returned.

			var containers []docker.APIContainers
			if msn != nil {
				for nwName, _ := range msn {
					// Get APIContainers given a network name
					var err error
					serviceContainers, err := cw.GetClient().ListContainers(docker.ListContainersOptions{Filters: map[string][]string{"network": []string{nwName}}})
					if err != nil {
						return nil, fmt.Errorf("unable to get list of containers in network %v, error %v", nwName, err)
					} else {
						containers = append(containers, serviceContainers...)
					}
				}
			} else {

				depConfig, _, derr := depDef.ConvertToDeploymentDescription(false)
				if derr != nil {
					return nil, derr
				}

				for serviceName, _ := range depConfig.Services {
					serviceContainers, err := findContainers(serviceName, cutil.MakeMSInstanceKey(depDef.URL, depDef.Org, depDef.Version, ""), cw)
					if err != nil {
						cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, i18n.GetMessagePrinter().Sprintf("'%v %v' unable to list existing containers: %v", SERVICE_COMMAND, SERVICE_START_COMMAND, err))
					}
					containers = append(containers, serviceContainers...)
				}
			}

			// Create networks specific to each parent and dependency, and connect the containers to them.
			newDependencyNetworks := cw.GatherAndCreateDependencyNetworks(containers, serviceInstance)

			// Add the dependency's new networks to the map of networks to be connected to the input service.
			for netName, net := range newDependencyNetworks {
				cliutils.Verbose(i18n.GetMessagePrinter().Sprintf("Containers for service %v/%v are in network %v", depDef.Org, depDef.URL, netName))
				ms_networks[netName] = net
			}

		}

	}

	return ms_networks, nil
}

func startDependent(dir string,
	serviceDef *common.ServiceFile,
	globals []common.GlobalSet, // API Attributes
	configUserInputs []policy.AbstractUserInput, // indicates configured variables
	cw *container.ContainerWorker,
	parentServiceInstance string) (map[string]string, error) {

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// The docker networks of any dependencies that the input service has.
	msNetworks := make(map[string]string)

	// Convert the deployment config into a full DeploymentDescription.
	depConfig, deployment, derr := serviceDef.ConvertToDeploymentDescription(false)
	if derr != nil {
		return nil, derr
	}

	// Start the service containers
	if !depConfig.HasAnyServices() {
		cliutils.Verbose(msgPrinter.Sprintf("Skipping service because it has no deployment configuration: %v", depConfig))
		return nil, nil
	}

	// Get a service instance id for this service. The service could be a singleton that is already started.
	var sId string
	singletonAlreadyStarted := false

	if serviceDef.Sharable == exchange.MS_SHARING_MODE_SINGLETON || serviceDef.Sharable == exchange.MS_SHARING_MODE_SINGLE {
		if serviceContainers, err := findContainers(depConfig.AnyServiceName(), cutil.MakeMSInstanceKey(serviceDef.URL, serviceDef.Org, serviceDef.Version, ""), cw); err != nil {
			return nil, errors.New(msgPrinter.Sprintf("unable to list existing containers: %v", err))
		} else if len(serviceContainers) > 0 {
			if instanceId, ok := serviceContainers[0].Labels[container.LABEL_PREFIX+".agreement_id"]; ok {
				sId = instanceId
				singletonAlreadyStarted = true
			}
		}
	}

	// If a service id was not assigned, generate one.
	if sId == "" {

		// Make a new instance id the same way the runtime makes them.
		id, err := uuid.NewV4()
		if err != nil {
			return nil, errors.New(msgPrinter.Sprintf("unable to generate instance ID: %v", err))
		}
		sId = cutil.MakeMSInstanceKey(serviceDef.URL, serviceDef.Org, serviceDef.Version, id.String())
	}

	cliutils.Verbose(msgPrinter.Sprintf("Working on service %v", sId))

	// Work our way down the dependency tree. If the service we want to start has dependencies, recursively process them
	// until we get to a leaf node. Leaf node services are started first, parents are started last.
	if serviceDef.HasDependencies() {

		if deps, err := GetServiceDependencies(dir, serviceDef.RequiredServices); err != nil {
			return nil, errors.New(msgPrinter.Sprintf("unable to retrieve dependency metadata: %v", err))
			// Start this service's dependencies
		} else if msn, err := ProcessStartDependencies(dir, deps, globals, configUserInputs, cw, sId); err != nil {
			return nil, errors.New(msgPrinter.Sprintf("unable to start dependencies: %v", err))
		} else {
			msNetworks = msn
		}
	}

	// If the service we need to start is a sharable singleton then it might already be started. If it is then just return.
	if (serviceDef.Sharable == exchange.MS_SHARING_MODE_SINGLETON || serviceDef.Sharable == exchange.MS_SHARING_MODE_SINGLE) && singletonAlreadyStarted {
		return nil, nil
	}

	// Start the service containers.
	return StartContainers(deployment, serviceDef.URL, globals, serviceDef.UserInputs, configUserInputs, serviceDef.Org, depConfig, cw, msNetworks, true, false, sId)

}

func StartContainers(deployment *containermessage.DeploymentDescription,
	specRef string,
	globals []common.GlobalSet, // API attributes
	defUserInputs []exchange.UserInput, // indicates variable defaults
	configUserInputs []policy.AbstractUserInput, // indicates configured variables
	org string,
	dc *common.DeploymentConfig,
	cw *container.ContainerWorker,
	msNetworks map[string]string,
	service bool,
	agreementBased bool,
	id string) (map[string]string, error) {

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Establish logging context
	logName := "microservice"
	if service {
		logName = "service"
	}

	agId := ""
	wlpw := ""
	if agreementBased {
		agId = id
		wlpw = "deprecated"
	}

	// Dependencies that require userinput variables to be set must have those variables set in the current userinput file,
	// which is either the input userinput file or the default userinput file from the current project.
	configVars := getConfiguredVariables(configUserInputs, specRef)

	// Now that we have the configured variables, turn everything into environment variables for the container.
	environmentAdditions, enverr := createEnvVarMap(agId, wlpw, globals, specRef, configVars, defUserInputs, org, cw, persistence.AttributesToEnvvarMap)
	if enverr != nil {
		return nil, errors.New(msgPrinter.Sprintf("unable to create environment variables"))
	}

	cliutils.Verbose(msgPrinter.Sprintf("Passing environment variables: %v", environmentAdditions))

	// Start the dpendent service

	msgPrinter.Printf("Start %v: %v with instance id prefix %v", logName, dc.CLIString(), id)
	msgPrinter.Println()

	// Start the dependent service container.
	_, startErr := cw.ResourcesCreate(id, "", deployment, []byte(""), environmentAdditions, msNetworks, cutil.FormOrgSpecUrl(cutil.NormalizeURL(specRef), org), "")
	if startErr != nil {
		return nil, errors.New(msgPrinter.Sprintf("unable to start container using %v, error: %v", dc.CLIString(), startErr))
	}

	msgPrinter.Printf("Running %v.", logName)
	msgPrinter.Println()

	return getContainerNetworks(dc, id, cw)
}

func ProcessStopDependencies(dir string, deps []*common.ServiceFile, cw *container.ContainerWorker) error {

	// Log the stopping of dependencies if there are any.
	if len(deps) != 0 {
		cliutils.Verbose(i18n.GetMessagePrinter().Sprintf("Stopping dependencies."))
	}

	for _, depDef := range deps {
		if err := stopDependent(dir, depDef, cw); err != nil {
			return err
		}
	}

	return nil
}

func stopDependent(dir string, serviceDef *common.ServiceFile, cw *container.ContainerWorker) error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Convert the deployment config into a full DeploymentDescription.
	depConfig, _, derr := serviceDef.ConvertToDeploymentDescription(false)
	if derr != nil {
		return derr
	}

	// Stop the service containers
	if !depConfig.HasAnyServices() {
		msgPrinter.Printf("Skipping service because it has no deployment configuration: %v", depConfig)
		msgPrinter.Println()
	} else if err := stopContainers(depConfig, cutil.MakeMSInstanceKey(serviceDef.URL, serviceDef.Org, serviceDef.Version, ""), cw, true); err != nil {
		return err
	}

	// Work our way down the dependency tree. If the service we want to stop has dependencies, recursively process them
	// until we get to a leaf node. Parents are stopped first, leaf nodes are stopped last.
	if serviceDef.HasDependencies() {

		if deps, err := GetServiceDependencies(dir, serviceDef.RequiredServices); err != nil {
			return errors.New(msgPrinter.Sprintf("unable to retrieve dependency metadata: %v", err))
			// Stop this service's dependencies
		} else if err := ProcessStopDependencies(dir, deps, cw); err != nil {
			return errors.New(msgPrinter.Sprintf("unable to stop dependencies: %v", err))
		}
	}

	return nil
}

func StopService(dc *common.DeploymentConfig, cw *container.ContainerWorker) error {
	return stopContainers(dc, "", cw, true)
}

func stopContainers(dc *common.DeploymentConfig, instanceId string, cw *container.ContainerWorker, service bool) error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Establish logging context
	logName := "service"
	if !service {
		logName = "microservice"
	}

	// Stop each container in the deployment config.
	for serviceName, _ := range dc.Services {
		containers, err := findContainers(serviceName, instanceId, cw)
		if err != nil {
			return errors.New(msgPrinter.Sprintf("unable to list containers, %v", err))
		}

		cliutils.Verbose(msgPrinter.Sprintf("Found containers %v", containers))

		// Locate the dev container(s) and stop it.
		for _, c := range containers {
			msId := c.Labels[container.LABEL_PREFIX+".agreement_id"]
			msgPrinter.Printf("Stop %v: %v with instance id prefix %v", logName, dc.CLIString(), msId)
			msgPrinter.Println()
			cw.ResourcesRemove([]string{msId})
		}
	}
	return nil
}

// Get the images into the local docker server for services
func getContainerImages(containerConfig *events.ContainerConfig, currentUIs *common.UserInputFile) error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Create a temporary anax config object to hold config for the shared runtime functions.
	cfg := &config.HorizonConfig{
		Edge: config.Config{
			TrustSystemCACerts:     true,
			TrustDockerAuthFromOrg: true,
		},
		AgreementBot:  config.AGConfig{},
		Collaborators: config.Collaborators{},
	}

	col, _ := config.NewCollaborators(*cfg)
	cfg.Collaborators = *col

	// Create a docker client so that we can convert the downloaded images into docker images.
	dockerEP := "unix:///var/run/docker.sock"
	client, derr := docker.NewClient(dockerEP)
	if derr != nil {
		return errors.New(msgPrinter.Sprintf("failed to create docker client, error: %v", derr))
	}

	// This is the image server authentication configuration. First get any anax attributes and convert them into
	// anax attributes.
	attributes, err := GlobalSetAsAttributes(currentUIs.Global)
	if err != nil {
		return errors.New(msgPrinter.Sprintf("failed to convert global attributes in %v, error: %v ", USERINPUT_FILE, err))
	}
	byValueAttrs := makeByValueAttributes(attributes)

	// Then extract the HTTPS authentication attributes.
	dockerAuthConfigurations := make(map[string][]docker.AuthConfiguration, 0)
	authErr := imagefetch.ExtractAuthAttributes(byValueAttrs, dockerAuthConfigurations)
	if authErr != nil {
		return errors.New(msgPrinter.Sprintf("failed to extract authentication attribute from %v, error: %v ", USERINPUT_FILE, err))
	}

	msgPrinter.Printf("getting container images into docker.")
	msgPrinter.Println()
	if err := imagefetch.ProcessImageFetch(cfg, client, containerConfig, dockerAuthConfigurations); err != nil {
		return errors.New(msgPrinter.Sprintf("failed to get container images, error: %v", err))
	}

	return nil
}

func CreateNetwork(client *docker.Client, name string) (*docker.Network, error) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	bridge, err := container.MakeBridge(client, name, true, false, true)
	if err != nil {
		return nil, err
	}
	cliutils.Verbose(msgPrinter.Sprintf("Created network %v", name))

	return bridge, nil
}

func RemoveNetwork(client *docker.Client, name string) error {

	// Remove named network
	networks, err := client.ListNetworks()
	if err != nil {
		return errors.New(fmt.Sprintf("unable to list docker networks, error %v", err))
	}

	for _, net := range networks {
		if net.Name == name {
			if err := client.RemoveNetwork(net.ID); err != nil {
				return errors.New(fmt.Sprintf("unable to remove docker network %v, error %v", name, err))
			} else {
				return nil
			}
		}
	}

	return nil
}

func GetNodeId() string {
	// Allow device id override if the env var is set.
	testDeviceId, _ := os.Hostname()
	if os.Getenv(DEVTOOL_HZN_DEVICE_ID) != "" {
		testDeviceId = os.Getenv(DEVTOOL_HZN_DEVICE_ID)
	}
	return testDeviceId
}

func GetDevWorkingDirectory() string {
	wd := os.Getenv(DEVTOOL_HZN_FSS_WORKING_DIR)
	if wd == "" {
		wd = DEFAULT_DEVTOOL_HZN_FSS_WORKING_DIR
	}
	return wd
}

// It is used by "hzn dev service new" when the specRef is an empty string.
// This function generates a service specRef and version from the image name provided by the user.
func GetServiceSpecFromImage(image string) (string, string, error) {
	if image == "" {
		return "", "", nil
	}

	specRef := ""
	version := ""

	// parse the image
	_, path, tag, _ := cutil.ParseDockerImagePath(image)
	if path == "" {
		return "", "", errors.New(fmt.Sprintf("invalid image format: %v", image))
	} else {
		// get last part as the service ref
		s := strings.Split(path, "/")
		specRef = s[len(s)-1]
	}

	if tag != "" && semanticversion.IsVersionString(tag) {
		version = tag
	}

	return specRef, version, nil
}

// This function extracts the image names from the image list and returns a map of name~image pairs.
// If the image does not have version tag specified, this function will add $SERVICE_VERSION as the tag
// so that it's easy for the user to update the version later. And it will append_$ARCH to the image name so
// distiguash images from different arch.
func GetImageInfoFromImageList(images []string, version string, noImageGen bool) (map[string]string, string, error) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	imageInfo := make(map[string]string)
	image_base := ""

	if len(images) == 0 {
		imageInfo["$SERVICE_NAME"] = "${DOCKER_IMAGE_BASE}_$ARCH:$SERVICE_VERSION"
		return imageInfo, image_base, nil
	}

	for _, image := range images {
		host, path, tag, digest := cutil.ParseDockerImagePath(image)
		if path == "" {
			return nil, "", errors.New(msgPrinter.Sprintf("invalid image format: %v", image))
		}
		s := strings.Split(path, "/")

		if !noImageGen {
			// only one image will be specified if noImageGen is flase.
			// In this case, remove the tag and digest, use SERVICE_VERSION for tag
			// The real image name will be "${DOCKER_IMAGE_BASE}_$ARCH:$SERVICE_VERSION"
			imageInfo[s[len(s)-1]] = "${DOCKER_IMAGE_BASE}_$ARCH:$SERVICE_VERSION"
			image_base = cutil.FormDockerImageName(host, path, "", "")
			return imageInfo, image_base, nil
		} else {
			if tag == "" && digest == "" {
				if len(images) == 1 {
					imageInfo[s[len(s)-1]] = "${DOCKER_IMAGE_BASE}_$ARCH:$SERVICE_VERSION"
					image_base = cutil.FormDockerImageName(host, path, "", "")
					return imageInfo, image_base, nil
				} else {
					// append _$ARCH in the image name and add SERVICE_VERSION as tag if the tag is not specified or the tag equals to the service version
					image = cutil.FormDockerImageName(host, fmt.Sprintf("%v_$ARCH", path), "$SERVICE_VERSION", "")
				}
			}
		}
		imageInfo[s[len(s)-1]] = image
	}

	return imageInfo, "", nil
}

// check the file existance, substitute the variables and save the file
func CreateFileWithConent(directory string, filename string, content string, substitutes map[string]string, perm_exec bool) error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	filePath := path.Join(directory, filename)

	// make sure the file does not exist
	found, err := FileExists(directory, filename)
	if err != nil {
		return err
	}
	if found {
		return errors.New(msgPrinter.Sprintf("file %v exists already", filePath))
	}

	// do the substitution
	if substitutes != nil {
		for key, val := range substitutes {
			content = strings.Replace(content, key, val, -1)
		}
	}

	// save the file
	var perm os.FileMode
	if perm_exec {
		// executable file
		perm = 0755
	} else {
		// regular file
		perm = 0644
	}
	if err := ioutil.WriteFile(filePath, []byte(content), perm); err != nil {
		return errors.New(msgPrinter.Sprintf("unable to write content to file %v, error: %v", filePath, err))
	} else {
		return nil
	}
}
