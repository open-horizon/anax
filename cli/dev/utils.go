package dev

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/cli/cliutils"
	cliexchange "github.com/open-horizon/anax/cli/exchange"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/container"
	"github.com/open-horizon/anax/containermessage"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/torrent"
	fetch "github.com/open-horizon/horizon-pkg-fetch"
	"github.com/satori/go.uuid"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const DEVTOOL_HZN_ORG = "HZN_ORG_ID"
const DEVTOOL_HZN_USER = "HZN_EXCHANGE_USER_AUTH"
const DEVTOOL_HZN_EXCHANGE_URL = "HZN_EXCHANGE_URL"
const DEVTOOL_HZN_DEVICE_ID = "HZN_DEVICE_ID"

const DEFAULT_WORKING_DIR = "horizon"

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
	// Create the working directory. If it already exists, just keep going.
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return errors.New(fmt.Sprintf("could not create working directory %v, error: %v", dir, err))
		}
	} else if err != nil {
		return errors.New(fmt.Sprintf("could not get status of working directory %v, error: %v", dir, err))
	}
	cliutils.Verbose("Using working directory: %v", dir)
	return nil
}

// Check for a file's existence or error out of the command. This is just a way to consolidate the error handling because
// we have several files that we're dealing with.
func FileNotExist(dir string, cmd string, fileName string, check func(string) (bool, error)) {
	if exists, err := check(dir); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v' %v", cmd, err)
	} else if exists {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v' %v", cmd, fmt.Sprintf("horizon project in %v, already contains %v.", dir, fileName))
	}
}

// Check for file existence and return any errors.
func FileExists(directory string, fileName string) (bool, error) {
	filePath := path.Join(directory, fileName)
	if _, err := os.Stat(filePath); err != nil && !os.IsNotExist(err) {
		return false, errors.New(fmt.Sprintf("error checking for %v: %v", fileName, err))
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
		return errors.New(fmt.Sprintf("failed to unmarshal %s, error: %v", filePath, err))
	}
	return nil
}

// This function takes one of the project json objects and writes it to a file int he project.
func CreateFile(directory string, fileName string, obj interface{}) error {
	// Convert the object to JSON and write it.
	filePath := path.Join(directory, fileName)
	if jsonBytes, err := json.MarshalIndent(obj, "", "    "); err != nil {
		return errors.New(fmt.Sprintf("failed to create json object for %v, error: %v", fileName, err))
	} else if err := ioutil.WriteFile(filePath, jsonBytes, 0664); err != nil {
		return errors.New(fmt.Sprintf("unable to write json object for %v to file %v, error: %v", fileName, filePath, err))
	} else {
		return nil
	}
}

// Common verification before executing a sub command.
func VerifyEnvironment(homeDirectory string, mustExist bool, needExchange bool, userCreds string) (string, error) {

	// Make sure the env vars needed by the dev tools are setup
	if needExchange && userCreds != "" {
		id, _ := cliutils.SplitIdToken(userCreds) // only look for the / in the id, because the token is more likely to have special chars
		if !strings.Contains(id, "/") && os.Getenv(DEVTOOL_HZN_ORG) == "" {
			return "", errors.New(fmt.Sprintf("Must set environment variable %v or specify the user as 'org/user' on the --user-pw flag", DEVTOOL_HZN_ORG))
		}
	} else if needExchange && userCreds == "" {
		id, _ := cliutils.SplitIdToken(os.Getenv(DEVTOOL_HZN_USER)) // only look for the / in the id, because the token is more likely to have special chars
		if !strings.Contains(id, "/") && os.Getenv(DEVTOOL_HZN_ORG) == "" {
			return "", errors.New(fmt.Sprintf("Must set environment variable %v or specify the user as 'org/user' on the --user-pw flag", DEVTOOL_HZN_ORG))
		}
	}

	if needExchange && os.Getenv(DEVTOOL_HZN_USER) == "" && userCreds == "" {
		return "", errors.New(fmt.Sprintf("Must set environment variable %v or specify user exchange credentials with --user-pw", DEVTOOL_HZN_USER))
	} else if os.Getenv(DEVTOOL_HZN_EXCHANGE_URL) == "" {
		return "", errors.New(fmt.Sprintf("Environment variable %v must be set.", DEVTOOL_HZN_EXCHANGE_URL))
	}

	// Get the directory we're working in
	dir, err := GetWorkingDir(homeDirectory, mustExist)
	if err != nil {
		return "", errors.New(fmt.Sprintf("project has no horizon metadata directory. Use hzn dev to create a new project. Error: %v", err))
	} else {
		return dir, nil
	}

}

// Indicates whether or not the given project is a workload project
func IsWorkloadProject(directory string) bool {
	if ex, err := UserInputExists(directory); !ex || err != nil {
		return false
	} else if ex, err := WorkloadDefinitionExists(directory); !ex || err != nil {
		return false
	} else if ex, err := DependenciesExists(directory); !ex || err != nil {
		return false
	}
	return true
}

// Indicates whether or not the given project is a microservice project
func IsMicroserviceProject(directory string) bool {
	if ex, err := UserInputExists(directory); !ex || err != nil {
		return false
	} else if ex, err := MicroserviceDefinitionExists(directory); !ex || err != nil {
		return false
	} else if ex, err := DependenciesExists(directory); !ex || err != nil {
		return false
	}
	return true
}

func CommonProjectValidation(dir string, userInputFile string, projectType string, cmd string) {
	// Get the Userinput file, so that we can validate it.
	userInputs, userInputsFilePath, uierr := GetUserInputs(dir, userInputFile)
	if uierr != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", projectType, cmd, uierr)
	}

	if verr := userInputs.Validate(dir, userInputsFilePath); verr != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' project does not validate. %v ", projectType, cmd, verr)
	}

	// Validate Dependencies
	if derr := ValidateDependencies(dir, userInputs, userInputsFilePath); derr != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' project does not validate. %v", projectType, cmd, derr)
	}
}

// Common setup processing for handling workload related commands.
func setup(homeDirectory string, mustExist bool, needExchange bool, userCreds string) (string, error) {

	// Shut off the Anax runtime logging.
	flag.Set("v", "0")

	// Verify that the environment and inputs are usable.
	dir, err := VerifyEnvironment(homeDirectory, mustExist, needExchange, userCreds)
	if err != nil {
		return "", err
	}

	cliutils.Verbose("Reading Horizon metadata from %s", dir)

	// Verify that the project is a workload project or a microservice.
	if !IsWorkloadProject(dir) && !IsMicroserviceProject(dir) {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "project in %v is not a horizon project.", dir)
	}

	return dir, nil
}

func makeByValueAttributes(attrs []persistence.Attribute) []persistence.Attribute {
	byValueAttrs := make([]persistence.Attribute, 0, 10)
	for _, a := range attrs {
		switch a.(type) {
		case *persistence.LocationAttributes:
			p := a.(*persistence.LocationAttributes)
			byValueAttrs = append(byValueAttrs, *p)
		case *persistence.ComputeAttributes:
			p := a.(*persistence.ComputeAttributes)
			byValueAttrs = append(byValueAttrs, *p)
		case *persistence.ArchitectureAttributes:
			p := a.(*persistence.ArchitectureAttributes)
			byValueAttrs = append(byValueAttrs, *p)
		case *persistence.HAAttributes:
			p := a.(*persistence.HAAttributes)
			byValueAttrs = append(byValueAttrs, *p)
		case *persistence.HTTPSBasicAuthAttributes:
			p := a.(*persistence.HTTPSBasicAuthAttributes)
			byValueAttrs = append(byValueAttrs, *p)
		case *persistence.BXDockerRegistryAuthAttributes:
			p := a.(*persistence.BXDockerRegistryAuthAttributes)
			byValueAttrs = append(byValueAttrs, *p)
		}
	}
	return byValueAttrs
}


// Create the environment variable map needed by the container worker to hold the environment variables that are passed to the
// workload container.
func createEnvVarMap(agreementId string,
	workloadPW string,
	global []GlobalSet,
	configVar map[string]interface{},
	defaultVar []exchange.UserInput,
	org string,
	attrConverter func(attributes []persistence.Attribute, envvars map[string]string, prefix string) (map[string]string, error)) (map[string]string, error) {

	// First, add in the Horizon platform env vars.
	envvars := make(map[string]string)

	// Allow device id override if the env var is set.
	testDeviceId, _ := os.Hostname()
	if os.Getenv(DEVTOOL_HZN_DEVICE_ID) != "" {
		testDeviceId = os.Getenv(DEVTOOL_HZN_DEVICE_ID)
	}

	exchangeURL := os.Getenv(DEVTOOL_HZN_EXCHANGE_URL)
	cutil.SetPlatformEnvvars(envvars, config.ENVVAR_PREFIX, agreementId, testDeviceId, org, workloadPW, exchangeURL)

	// Second, add the Horizon system env vars. Some of these can come from the global section of a user inputs file. To do this we have to
	// convert the attributes in the userinput file into API attributes so that they can be validity checked. Then they are converted to
	// persistence attributes so that they can be further converted to environment variables. This is the progression that anax uses when
	// running real workloads so the same progression is used here.
	attrs, err := GlobalSetAsAttributes(global)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("%v has error: %v ", USERINPUT_FILE, err))
	}

	// Third, add in default system attributes if not already present.
	attrs = api.FinalizeAttributesSpecifiedInService(1024, "", attrs)

	cliutils.Verbose("Final Attributes: %v", attrs)

	// The conversion to persistent attributes produces an array of pointers to attributes, we need a by-value
	// array of attributes because that's what the functions which convert attributes to env vars expect. This is
	// because at runtime, the attributes are serialized to a database and then read out again before converting to env vars.

	byValueAttrs := makeByValueAttributes(attrs)

	// Fourth, convert all attributes to system env vars.
	var cerr error
	envvars, cerr = attrConverter(byValueAttrs, envvars, config.ENVVAR_PREFIX)
	if cerr != nil {
		return nil, errors.New(fmt.Sprintf("global attribute conversion error: %v", cerr))
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

	workloadRODir := "/tmp/hzn"
	if err := os.MkdirAll(workloadRODir, 0755); err != nil {
		return nil, err
	}

	config := &config.HorizonConfig{
		Edge: config.Config{
			WorkloadROStorage:             workloadRODir,
			DefaultServiceRegistrationRAM: 128,
		},
		AgreementBot:  config.AGConfig{},
		Collaborators: config.Collaborators{},
	}

	return container.CreateCLIContainerWorker(config)
}

// This function is used to setup context to execute a microservice or workload container.
func commonExecutionSetup(homeDirectory string, userInputFile string, projectType string, cmd string) (string, *InputFile, *container.ContainerWorker) {

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
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' unable to create Container Worker, %v", projectType, cmd, cerr)
	}

	return dir, userInputs, cw
}

func startMicroservice(deployment *containermessage.DeploymentDescription,
	specRef string,
	version string,
	globals []GlobalSet, // API attributes
	defUserInputs []exchange.UserInput, // indicates variable defaults
	configUserInputs *InputFile, // indciates configured variables
	org string,
	dc *cliexchange.DeploymentConfig,
	cw *container.ContainerWorker,
	msNetworks map[string]docker.ContainerNetwork) (map[string]docker.ContainerNetwork, error) {

	// Dependencies that require userinput variables to be set must have those variables set in the current userinput file,
	// which is either the input userinput file or the default userinput file from the current project.
	configVars := getConfiguredVariables(configUserInputs.Microservices, specRef)

	// Now that we have the configured variables, turn everything into environment variables for the container.
	environmentAdditions, enverr := createEnvVarMap("", "", globals, configVars, defUserInputs, org, persistence.AttributesToEnvvarMap)
	if enverr != nil {
		return nil, errors.New(fmt.Sprintf("unable to create environment variables"))
	}

	cliutils.Verbose("Passing environment variables: %v", environmentAdditions)

	// Start the dependency microservice

	// Make an instance id the same way the runtime makes them.
	msId := cutil.MakeMSInstanceKey(specRef, version, uuid.NewV4().String())

	fmt.Printf("Start microservice: %v with instance id prefix %v\n", dc.CLIString(), msId)

	// Start the microservice container.
	_, startErr := cw.ResourcesCreate(msId, nil, deployment, []byte(""), environmentAdditions, map[string]docker.ContainerNetwork{})
	if startErr != nil {
		return nil, errors.New(fmt.Sprintf("unable to start container using %v, error: %v", dc.CLIString(), startErr))
	}

	fmt.Printf("Running microservice.\n")

	// Locate the microservice network(s) and return them so that a workload can be hooked in if necessary.
	for serviceName, _ := range dc.Services {
		dcService := docker.ListContainersOptions{
			All: true,
			Filters: map[string][]string{
				"label": []string{
					fmt.Sprintf("%v.service_name=%v", container.LABEL_PREFIX, serviceName),
				},
			},
		}

		containers, err := cw.GetClient().ListContainers(dcService)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("unable to list containers, %v", err))
		}

		// Return all the networks on this microservice.
		for _, msc := range containers {
			for nw_name, nw := range msc.Networks.Networks {
				msNetworks[nw_name] = nw
				cliutils.Verbose("Found network %v", nw)
			}
		}
	}

	return msNetworks, nil
}

func stopMicroservice(dc *cliexchange.DeploymentConfig, cw *container.ContainerWorker) error {
	for serviceName, _ := range dc.Services {
		dcService := docker.ListContainersOptions{
			All: true,
			Filters: map[string][]string{
				"label": []string{
					fmt.Sprintf("%v.service_name=%v", container.LABEL_PREFIX, serviceName),
				},
			},
		}

		containers, err := cw.GetClient().ListContainers(dcService)
		if err != nil {
			return errors.New(fmt.Sprintf("unable to list containers, %v", err))
		}

		cliutils.Verbose("Found containers %v", containers)

		// Locate the microservice container and stop it.
		for _, c := range containers {
			msId := c.Labels[container.LABEL_PREFIX+".agreement_id"]
			fmt.Printf("Stop microservice: %v with instance id prefix  %v\n", dc.CLIString(), msId)
			cw.ResourcesRemove([]string{msId})
		}
	}
	return nil
}

// For workloads and microservices that have their images on an image server, download the image(s)
// and load them into the local docker.
func downloadFromImageServer(torrentInfo string, keyFile string, currentUIs *InputFile) error {

	torrObj := new(policy.Torrent)
	if err := json.Unmarshal([]byte(torrentInfo), torrObj); err != nil {
		return errors.New(fmt.Sprintf("failed to unmarshal torrent field: %v, error: %v", torrentInfo, err))
	} else if torrObj.Url == "" {
		return nil
	} else if torrentUrl, err := url.Parse(torrObj.Url); err != nil {
		return errors.New(fmt.Sprintf("failed to parse torrent.url %v, error: %v", torrObj, err))
	} else if torrentUrl != nil {

		fmt.Printf("Dependency has container images on an image server, downloading the images now.\n")
		cliutils.Verbose("Downloading container images from image server: %s", torrentUrl)

		torrentSig := torrObj.Signature

		// Create a temporary anax config object to hold the HTTP config we need to contact the Image server.
		cfg := &config.HorizonConfig{
			Edge: config.Config{
				TrustSystemCACerts: true,
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
			return errors.New(fmt.Sprintf("failed to create docker client, error: %v", derr))
		}

		// This function prevents a download for something that is already downloaded.
		skipCheckFn := torrent.SkipCheckFn(client)

		// This is the image server authentication configuration. First get any anax attributes and convert them into
		// anax attributes.
		attributes, err := GlobalSetAsAttributes(currentUIs.Global)
		if err != nil {
			return errors.New(fmt.Sprintf("failed to convert global attributes in %v, error: %v ", USERINPUT_FILE, err))
		}
		byValueAttrs := makeByValueAttributes(attributes)

		// Then extract the HTTPS authentication attributes.
		httpAuthAttrs := make(map[string]map[string]string, 0)
		dockerAuthConfigurations := make(map[string]docker.AuthConfiguration, 0)
		httpAuth, _, authErr := torrent.ExtractAuthAttributes(byValueAttrs, httpAuthAttrs, dockerAuthConfigurations)
		if authErr != nil {
			return errors.New(fmt.Sprintf("failed to extract authentication attribute from %v, error: %v ", USERINPUT_FILE, err))
		}

		cliutils.Verbose("Using HTTPS Basic authorization: %v", httpAuth)

		// A public key is needed to verify the signature of the image parts.
		pemFiles := []string{keyFile}

		// Download to a temporary location.
		torrentDir := "/tmp"

		// Call the package fetcher library to download and verify the image parts.
		imageFiles, fetchErr := fetch.PkgFetch(cfg.Collaborators.HTTPClientFactory.WrappedNewHTTPClient(), &skipCheckFn, *torrentUrl, torrentSig, torrentDir, pemFiles, httpAuth)
		if fetchErr != nil {
			return errors.New(fmt.Sprintf("failed to fetch %v, error: %v", torrentUrl, fetchErr))
		}

		fmt.Printf("Loading container images into docker.\n")
		cliutils.Verbose("Loading container images into docker: %v", imageFiles)

		// Now that the images are downloaded, load them into docker.
		loadErr := torrent.LoadImagesFromPkgParts(client, imageFiles)
		if loadErr != nil {
			return errors.New(fmt.Sprintf("failed to load images %v from images server, error: %v", imageFiles, loadErr))
		}
	}
	return nil
}