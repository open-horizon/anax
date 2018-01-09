package dev

import (
	"errors"
	"flag"
	"fmt"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/cli/cliutils"
	cliexchange "github.com/open-horizon/anax/cli/exchange"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/container"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/persistence"
	"os"
	"strconv"
)

const WORKLOAD_CREATION_COMMAND = "new"
const WORKLOAD_VERIFY_COMMAND = "verify"

// Create skeletal horizon metadata files to establish a new workload project.
func WorkloadNew(homeDirectory string) {

	// Verify that env vars are set properly and determine the working directory.
	dir, err := VerifyEnvironment(homeDirectory, false)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'workload %v' %v", WORKLOAD_CREATION_COMMAND, err)
	}

	// Create the working directory.
	if err := CreateWorkingDir(dir); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'workload %v' %v", WORKLOAD_CREATION_COMMAND, err)
	}

	// If there are any horizon metadata files already in the directory then we wont create any files.
	cmd := fmt.Sprintf("workload %v", WORKLOAD_CREATION_COMMAND)
	FileNotExist(dir, cmd, DEPLOYMENT_CONFIG_FILE, DeploymentConfigExists)
	FileNotExist(dir, cmd, USERINPUT_FILE, UserInputExists)
	FileNotExist(dir, cmd, WORKLOAD_DEFINITION_FILE, WorkloadDefinitionExists)
	FileNotExist(dir, cmd, DEPENDENCIES_FILE, DependenciesExists)

	// Create the metadata files.
	if err := CreateDeploymentConfig(dir); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'workload %v' %v", WORKLOAD_CREATION_COMMAND, err)
	} else if err := CreateUserInputs(dir, true); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'workload %v' %v", WORKLOAD_CREATION_COMMAND, err)
	} else if err := CreateWorkloadDefinition(dir); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'workload %v' %v", WORKLOAD_CREATION_COMMAND, err)
	} else if err := CreateDependencies(dir); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'workload %v' %v", WORKLOAD_CREATION_COMMAND, err)
	}

	cliutils.Verbose("Created horizon metadata files. Edit these files to define and configure your new workload.")

}

func WorkloadStartTest(homeDirectory string, userInputFile string) {

	// Run verification before trying to start anything
	WorkloadValidate(homeDirectory, userInputFile)

	// Get the setup info and context for running the command.
	dir, err := setup(homeDirectory, true)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'workload start' %v", err)
	}

	// Get the setup info and context for executing a container.
	dc, err := setupToExecute(dir)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'workload start' %v", err)
	}

	// Get the workload definition, so that we can look at the user input variable definitions.
	workloadDef, wderr := GetWorkloadDefinition(dir)
	if wderr != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'workload start' %v", wderr)
	}

	// Get the userinput file, so that we can get the userinput variables.
	userInputs, _, uierr := GetUserInputs(dir, userInputFile)
	if uierr != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'workload start' %v", uierr)
	}

	// Generate an agreement id for testing purposes.
	agreementId, aerr := cutil.GenerateAgreementId()
	if aerr != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'workload start' unable to generate test agreementId, %v", aerr)
	}

	// Convert the deployment config into a full DeploymentDescription.
	deployment, derr := dc.ConvertToDeploymentDescription()
	if derr != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'workload start' unable to create Deployment Description, %v", derr)
	}

	// Create the set of environment variables that are passed into the container.
	environmentAdditions, enverr := createEnvVarMap(agreementId, userInputs, workloadDef)
	if enverr != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'workload start' unable to create environment variables, %v", enverr)
	}

	cliutils.Verbose("Passing environment variables: %v", environmentAdditions)

	// Collect all the microservice networks that have to be connected.
	ms_networks := map[string]docker.ContainerNetwork{}

	// Create the containerWorker
	cw, cerr := createContainerWorker()
	if cerr != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'workload start' unable to create Container Worker, %v", cerr)
	}

	fmt.Printf("Starting workload: %v in agreement id %v\n", dc.CLIString(), agreementId)

	// Start the workload container image
	_, startErr := cw.ResourcesCreate(agreementId, nil, deployment, []byte(""), environmentAdditions, ms_networks)
	if startErr != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'workload start' unable to start workload container using %v %v, %v", DEPLOYMENT_CONFIG_FILE, dc.CLIString(), startErr)
	}

	fmt.Printf("Running workload.\n")

}

func WorkloadStopTest(homeDirectory string) {

	// Get the setup info and context for running the command.
	dir, err := setup(homeDirectory, true)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'workload stop' %v", err)
	}

	// Get the setup info and context for executing a container.
	dc, err := setupToExecute(dir)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'workload start' %v", err)
	}

	// Create the containerWorker that we can use to remove the running workload
	cw, cerr := createContainerWorker()
	if cerr != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'workload stop' unable to create Container Worker, %v", cerr)
	}

	// Locate the containers that match the services in our deployment config
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
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'workload stop' unable to list containers, %v", err)
		}

		cliutils.Verbose("Found containers %v", containers)

		for _, c := range containers {
			agreementId := c.Labels[container.LABEL_PREFIX+".agreement_id"]
			fmt.Printf("Stopping workload: %v in agreement id %v\n", dc.CLIString(), agreementId)
			cw.ResourcesRemove([]string{agreementId})
		}
	}

	fmt.Printf("Stopped workload.\n")

}

func WorkloadValidate(homeDirectory string, userInputFile string) {

	// Get the setup info and context for running the command.
	dir, err := setup(homeDirectory, true)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'workload %v' %v", WORKLOAD_VERIFY_COMMAND, err)
	}

	// Validate Workload Definition
	if verr := ValidateWorkloadDefinition(dir); verr != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'workload %v' project does not validate. %v ", WORKLOAD_VERIFY_COMMAND, verr)
	}

	// Validate Deployment config
	if dcerr := ValidateDeploymentConfig(dir); dcerr != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'workload %v' project does not validate. %v", WORKLOAD_VERIFY_COMMAND, dcerr)
	}

	// Validate Dependencies
	if derr := ValidateDependencies(dir); derr != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'workload %v' project does not validate. %v", WORKLOAD_VERIFY_COMMAND, derr)
	}

	// Get the Userinput file, so that we can validate it.
	userInputs, userInputsFilePath, uierr := GetUserInputs(dir, userInputFile)
	if uierr != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'workload %v' %v", WORKLOAD_VERIFY_COMMAND, uierr)
	}

	if verr := userInputs.Validate(dir, userInputsFilePath); verr != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'workload %v' project does not validate. %v ", WORKLOAD_VERIFY_COMMAND, verr)
	}

	fmt.Printf("Workload project verified.\n")
}

func WorkloadDeploy(homeDirectory string) {
	cliutils.Fatal(cliutils.INTERNAL_ERROR, "'workload deploy' not supported yet.")
}

// ========================= private functions ======================================================

// Common setup processing for handling workload related commands.
func setup(homeDirectory string, mustExist bool) (string, error) {

	// Shut off the Anax runtime logging.
	flag.Set("v", "0")

	// Verify that the environment and inputs are usable.
	dir, err := VerifyEnvironment(homeDirectory, mustExist)
	if err != nil {
		return "", err
	}

	cliutils.Verbose("Reading Horizon metadata from %s", dir)

	// Verify that the project is a workload project.
	if !IsWorkloadProject(dir) {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "project in %v is not a workload project.", dir)
	}

	return dir, nil
}

// Get the deployment config
func setupToExecute(directory string) (*DeploymentConfig, error) {

	// Open the deployment config and verify that it can be used to start a workload container.
	dc, err := GetDeploymentConfig(directory)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("cannot read deployment config file, %v", err))
	} else if err := dc.CanStartStop(); err != nil {
		return nil, errors.New(fmt.Sprintf("cannot start/stop workload, %v", err))
	}

	return dc, nil
}

// Create the environment variable map needed by the container worker to hold the environment variables that are passed to the
// workload container.
func createEnvVarMap(agreementId string, userInputs *InputFile, workloadDef *cliexchange.WorkloadInput) (map[string]string, error) {

	// First add in the Horizon platform env vars.
	environmentAdditions := make(map[string]string)
	testDeviceId, _ := os.Hostname()
	org := os.Getenv(DEVTOOL_HZN_ORG)
	workloadPW := "deprecated"
	exchangeURL := os.Getenv(DEVTOOL_HZN_EXCHANGE_URL)
	cutil.SetPlatformEnvvars(environmentAdditions, config.ENVVAR_PREFIX, agreementId, testDeviceId, org, workloadPW, exchangeURL)

	// Now add the Horizon system env vars. Some of these can come from the global section of the user inputs file. To do this we have to
	// convert the attributes in the userinput file into API attributes so that they can be validity checked. Then they are converted to
	// persistence attributes so that they can be further converted to environment variables. This is the progression that anax uses when
	// running real workloads so the same progression is used here.
	attrs, err := userInputs.GlobalSetAsAttributes()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("%v has error: %v ", USERINPUT_FILE, err))
	}

	// Add in default system attributes if not already present.
	attrs = api.FinalizeAttributesSpecifiedInService(1024, "", attrs)

	cliutils.Verbose("Final Attributes: %v", attrs)

	// The conversion to persistent attributes produces an array of pointers to attributes, we need a by-value
	// array of attributes because that's what the functions which convert attributes to env vars expect. This is
	// because at runtime, the attributes are serialized to a database and then read out again before converting to env vars.

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
		}
	}

	// Convert all attributes to system env vars.
	persistence.ConvertWorkloadPersistentNativeToEnv(byValueAttrs, environmentAdditions)

	// Now that the system and attribute based env vars are in place, we can convert the workload defined variables to env
	// vars and add them into the env var map.
	// Add in default variables from the workload definition.
	workloadDef.AddDefaultUserInputs(environmentAdditions)

	// Then add in the configured variable values from the workload section of the user input file.
	userInputs.AddWorkloadUserInputs(workloadDef, environmentAdditions)

	return environmentAdditions, nil
}

func floatVariableToString(vars map[string]interface{}, varName string) (string, error) {
	if v, ok := vars[varName]; !ok {
		return "", errors.New(fmt.Sprintf("%v is not specified.", varName))
	} else if fv, ok := v.(float64); !ok {
		return "", errors.New(fmt.Sprintf("%v must have a floating point value, is %T.", varName, v))
	} else {
		return strconv.FormatFloat(fv, 'f', 6, 64), nil
	}
}

func intVariableToString(vars map[string]interface{}, varName string) (string, error) {
	if v, ok := vars[varName]; !ok {
		return "", errors.New(fmt.Sprintf("%v is not specified.", varName))
	} else if fv, ok := v.(float64); !ok {
		return "", errors.New(fmt.Sprintf("%v must have an integer value, is %T.", varName, v))
	} else {
		iv := int64(fv)
		return strconv.FormatInt(iv, 10), nil
	}
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
