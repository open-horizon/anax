package dev

import (
	"errors"
	"flag"
	"fmt"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cli/register"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/container"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/persistence"
	"os"
	"reflect"
	"strconv"
)

func WorkloadNew(homeDirectory string) {
	cliutils.Fatal(cliutils.INTERNAL_ERROR, "'workload new' not supported yet.")
}

func WorkloadStartTest(homeDirectory string, userInputFile string) {

	// Get the setup info and context for running the command.
	dir, dc, err := setup(homeDirectory)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'workload start' %v", err)
	}

	// Get the workload definition, so that we can look at the user input variable definitions.

	// Get the userinput file, so that we can get the userinput variables.
	userInputs, uierr := GetUserInputs(dir, userInputFile)
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
	environmentAdditions, enverr := createEnvVarMap(agreementId, userInputs)
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
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'workload start' unable to start workload container, %v", startErr)
	}

	fmt.Printf("Running workload.\n")

}

func WorkloadStopTest(homeDirectory string) {

	// Get the setup info and context for running the command.
	_, dc, err := setup(homeDirectory)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'workload stop' %v", err)
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

func WorkloadValidate(homeDirectory string) {
	cliutils.Fatal(cliutils.INTERNAL_ERROR, "'workload verify' not supported yet.")
}

func WorkloadDeploy(homeDirectory string) {
	cliutils.Fatal(cliutils.INTERNAL_ERROR, "'workload deploy' not supported yet.")
}

// ========================= private functions ======================================================

// Common setup processing for handling workload related commands.
func setup(homeDirectory string) (string, *DeploymentConfig, error) {

	// Make sure the env vars needed by the dev tools are setup
	if os.Getenv(DEVTOOL_HZN_ORG) == "" {
		return "", nil, errors.New(fmt.Sprintf("Environment variable %v must be set to use the 'workload start/stop' commands.", DEVTOOL_HZN_ORG))
	} else if os.Getenv(DEVTOOL_HZN_EXCHANGE_URL) == "" {
		return "", nil, errors.New(fmt.Sprintf("Environment variable %v must be set to use the 'workload start/stop' commands.", DEVTOOL_HZN_EXCHANGE_URL))
	}

	// Get the directory we're working in
	dir, err := GetWorkingDir(homeDirectory)
	if err != nil {
		return "", nil, errors.New(fmt.Sprintf("cannot determine working directory, error: %v", err))
	}

	cliutils.Verbose("Reading Horizon metadata from %s", dir)

	// Open the deployment config and verify that it can be used to start a workload container.
	dc, err := GetDeploymentConfig(dir)
	if err != nil {
		return "", nil, errors.New(fmt.Sprintf("cannot read deployment config file, %v", err))
	} else if err := dc.CanStartStop(); err != nil {
		return "", nil, errors.New(fmt.Sprintf("cannot start/stop workload, %v", err))
	}

	// Shut off the Anax runtime logging.
	flag.Set("v", "0")

	return dir, dc, nil
}

// Create the environment variable map needed by the container worker to hold the environment variables that are passed to the
// workload container.
func createEnvVarMap(agreementId string, userInputs *register.InputFile) (map[string]string, error) {

	// First add in the Horizon platform env vars.
	environmentAdditions := make(map[string]string)
	testDeviceId, _ := os.Hostname()
	org := os.Getenv(DEVTOOL_HZN_ORG)
	workloadPW := "deprecated"
	exchangeURL := os.Getenv(DEVTOOL_HZN_EXCHANGE_URL)
	container.SetPlatformEnvvars(environmentAdditions, agreementId, testDeviceId, org, workloadPW, exchangeURL)

	// Now add the Horizon system env vars. Some of these can come from the global section of the user inputs file.
	var lat, lon, cpus, ram string
	ram = "128"
	cpus = "1"
	for _, g := range userInputs.Global {
		if g.Type == reflect.TypeOf(persistence.LocationAttributes{}).Name() {
			if fs, err := floatVariableToString(g.Variables, "lat"); err != nil {
				return environmentAdditions, errors.New(fmt.Sprintf("in %v %v: %v", USERINPUT_FILE, reflect.TypeOf(persistence.LocationAttributes{}).Name(), err))
			} else {
				lat = fs
			}
			if fs, err := floatVariableToString(g.Variables, "lon"); err != nil {
				return environmentAdditions, errors.New(fmt.Sprintf("in %v %v: %v", USERINPUT_FILE, reflect.TypeOf(persistence.LocationAttributes{}).Name(), err))
			} else {
				lon = fs
			}
		} else if g.Type == reflect.TypeOf(persistence.ComputeAttributes{}).Name() {
			if is, err := intVariableToString(g.Variables, "ram"); err != nil {
				return environmentAdditions, errors.New(fmt.Sprintf("in %v %v: %v", USERINPUT_FILE, reflect.TypeOf(persistence.ComputeAttributes{}).Name(), err))
			} else {
				ram = is
			}
			if is, err := intVariableToString(g.Variables, "cpus"); err != nil {
				return environmentAdditions, errors.New(fmt.Sprintf("in %v %v: %v", USERINPUT_FILE, reflect.TypeOf(persistence.ComputeAttributes{}).Name(), err))
			} else {
				cpus = is
			}
		}

	}

	// The last parameter is the hardware architecture. Let it default for the test environment.
	// This function is used to set the system env vars so that if an anax developer tries to add more
	// env vars, the CLI will not compile, alerting the anax developer to update the CLI with the new env var.
	container.SetSystemEnvvars(environmentAdditions, lat, lon, cpus, ram, "")

	// The add in the userInputs from the project.

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
