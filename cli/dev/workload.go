package dev

import (
	"fmt"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/container"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/persistence"
	"os"
)

// These constants define the hzn dev subcommands supported by this module.
const WORKLOAD_COMMAND = "workload"
const WORKLOAD_CREATION_COMMAND = "new"
const WORKLOAD_START_COMMAND = "start"
const WORKLOAD_STOP_COMMAND = "stop"
const WORKLOAD_VERIFY_COMMAND = "verify"
const WORKLOAD_DEPLOY_COMMAND = "publish"

// Create skeletal horizon metadata files to establish a new workload project.
func WorkloadNew(homeDirectory string, org string) {

	// Verify that env vars are set properly and determine the working directory.
	dir, err := VerifyEnvironment(homeDirectory, false, false, "")
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", WORKLOAD_COMMAND, WORKLOAD_CREATION_COMMAND, err)
	}

	if org == "" && os.Getenv(DEVTOOL_HZN_ORG) == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' must specify either --org or set the %v environment variable.", WORKLOAD_COMMAND, WORKLOAD_CREATION_COMMAND, DEVTOOL_HZN_ORG)
	}

	// Create the working directory.
	if err := CreateWorkingDir(dir); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", WORKLOAD_COMMAND, WORKLOAD_CREATION_COMMAND, err)
	}

	// If there are any horizon metadata files already in the directory then we wont create any files.
	cmd := fmt.Sprintf("%v %v", WORKLOAD_COMMAND, WORKLOAD_CREATION_COMMAND)
	FileNotExist(dir, cmd, USERINPUT_FILE, UserInputExists)
	FileNotExist(dir, cmd, WORKLOAD_DEFINITION_FILE, WorkloadDefinitionExists)
	FileNotExist(dir, cmd, DEPENDENCIES_FILE, DependenciesExists)

	if org == "" {
		org = os.Getenv(DEVTOOL_HZN_ORG)
	}

	// Create the metadata files.
	if err := CreateUserInputs(dir, true, org); err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", WORKLOAD_COMMAND, WORKLOAD_CREATION_COMMAND, err)
	} else if err := CreateWorkloadDefinition(dir, org); err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", WORKLOAD_COMMAND, WORKLOAD_CREATION_COMMAND, err)
	} else if err := CreateDependencies(dir); err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", WORKLOAD_COMMAND, WORKLOAD_CREATION_COMMAND, err)
	}

	fmt.Printf("Created horizon metadata files in %v. Edit these files to define and configure your new %v.\n", dir, WORKLOAD_COMMAND)

}

func WorkloadStartTest(homeDirectory string, userInputFile string) {

	// Run verification before trying to start anything.
	WorkloadValidate(homeDirectory, userInputFile)

	// Perform the common execution setup.
	dir, userInputs, cw := commonExecutionSetup(homeDirectory, userInputFile, WORKLOAD_COMMAND, WORKLOAD_START_COMMAND)

	// Collect all the microservice networks that have to be connected.
	ms_networks := map[string]docker.ContainerNetwork{}

	// Get the workload definition, so that we can look at the user input variable definitions.
	workloadDef, wderr := GetWorkloadDefinition(dir)
	if wderr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", WORKLOAD_COMMAND, WORKLOAD_START_COMMAND, wderr)
	}

	// Generate an agreement id for testing purposes.
	agreementId, aerr := cutil.GenerateAgreementId()
	if aerr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' unable to generate test agreementId, %v", WORKLOAD_COMMAND, WORKLOAD_START_COMMAND, aerr)
	}

	// Start any dependencies. Dependencies should be listed in the APISpec array of the workload definition and in
	// the dependencies json file.

	// Log the starting of dependencies if there are any.
	if len(workloadDef.APISpecs) != 0 {
		cliutils.Verbose("Starting dependencies.")
	}

	// Loop through each dependency to get the metadata we need to start the dependency.
	deps, err := GetDependencies(dir)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' unable to get dependencies, %v", WORKLOAD_COMMAND, WORKLOAD_START_COMMAND, err)
	}

	for depId, dep := range deps {

		if dep.DeployConfig.HasAnyServices() {
			// Convert the deployment config into a full DeploymentDescription.
			deployment, derr := dep.ConvertToDeploymentDescription()
			if derr != nil {
				cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' unable to create Deployment Description for dependency, %v", WORKLOAD_COMMAND, WORKLOAD_START_COMMAND, derr)
			}

			var err error
			ms_networks, err = startMicroservice(deployment, dep.SpecRef, dep.Version, dep.Global, dep.UserInputs, userInputs, workloadDef.Org, &dep.DeployConfig, cw, ms_networks)
			if err != nil {

				// Stop any microservices that might already be started.
				WorkloadStopTest(homeDirectory)

				cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v for dependency %v", WORKLOAD_COMMAND, WORKLOAD_START_COMMAND, err, dep)
			}
		} else {
			fmt.Printf("Skipping microservice %v because it has no deployment.\n", depId)
		}

	}

	// Now we can start the workload container.

	// Get the variables intended to configure this dependency from this project's userinput file.
	configVars := getConfiguredVariables(userInputs.Workloads, workloadDef.WorkloadURL)

	environmentAdditions, enverr := createEnvVarMap(agreementId, "deprecated", userInputs.Global, configVars, workloadDef.UserInputs, workloadDef.Org, persistence.ConvertWorkloadPersistentNativeToEnv)
	if enverr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' unable to create environment variables, %v", WORKLOAD_COMMAND, WORKLOAD_START_COMMAND, enverr)
	}

	cliutils.Verbose("Passing environment variables: %v", environmentAdditions)

	dc, deployment, cerr := workloadDef.ConvertToDeploymentDescription()
	if cerr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", WORKLOAD_COMMAND, WORKLOAD_START_COMMAND, cerr)
	}

	fmt.Printf("Starting workload: %v in agreement id %v\n", dc.CLIString(), agreementId)

	// Start the workload container image
	_, startErr := cw.ResourcesCreate(agreementId, nil, deployment, []byte(""), environmentAdditions, ms_networks)
	if startErr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' unable to start container using %v, %v", WORKLOAD_COMMAND, WORKLOAD_START_COMMAND, dc.CLIString(), startErr)
	}

	fmt.Printf("Running workload.\n")

}

func WorkloadStopTest(homeDirectory string) {

	// Perform the common execution setup.
	dir, _, cw := commonExecutionSetup(homeDirectory, "", WORKLOAD_COMMAND, WORKLOAD_STOP_COMMAND)

	// Loop through each dependency to get the metadata we need to start the dependency.
	deps, err := GetDependencies(dir)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' unable to get dependencies, %v", WORKLOAD_COMMAND, WORKLOAD_STOP_COMMAND, err)
	}

	for _, dep := range deps {
		err := stopMicroservice(&dep.DeployConfig, cw)
		if err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v for dependency %v", WORKLOAD_COMMAND, WORKLOAD_STOP_COMMAND, err, dep)
		}
	}

	// Get the workload definition.
	workloadDef, wderr := GetWorkloadDefinition(dir)
	if wderr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", WORKLOAD_COMMAND, WORKLOAD_STOP_COMMAND, wderr)
	}

	// Get the deployment config.
	dc, _, cerr := workloadDef.ConvertToDeploymentDescription()
	if cerr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", WORKLOAD_COMMAND, WORKLOAD_STOP_COMMAND, cerr)
	}

	// Locate the containers that match the services in our deployment config.
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
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' unable to list containers, %v", WORKLOAD_COMMAND, WORKLOAD_STOP_COMMAND, err)
		}

		cliutils.Verbose("Found containers %v", containers)

		// Locate the workload container and stop it.
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
	dir, err := setup(homeDirectory, true, false, "")
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", WORKLOAD_COMMAND, WORKLOAD_VERIFY_COMMAND, err)
	}

	// Make sure we're in a workload project.
	if !IsWorkloadProject(dir) {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' current project is not a workload project.", WORKLOAD_COMMAND, WORKLOAD_VERIFY_COMMAND)
	}

	// Validate Workload Definition
	if verr := ValidateWorkloadDefinition(dir); verr != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' project does not validate. %v ", WORKLOAD_COMMAND, WORKLOAD_VERIFY_COMMAND, verr)
	}

	CommonProjectValidation(dir, userInputFile, WORKLOAD_COMMAND, WORKLOAD_VERIFY_COMMAND)

	fmt.Printf("Workload project %v verified.\n", dir)
}

func WorkloadDeploy(homeDirectory string, keyFile string, userCreds string) {

	// Validate the inputs
	if keyFile == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' must specify a keyFile. See hzn dev %v %v --help.", WORKLOAD_COMMAND, WORKLOAD_DEPLOY_COMMAND, WORKLOAD_COMMAND, WORKLOAD_DEPLOY_COMMAND)
	} else if _, err := os.Stat(keyFile); err != nil && !os.IsNotExist(err) {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' error checking existence of %v, error: %v.", WORKLOAD_COMMAND, WORKLOAD_DEPLOY_COMMAND, keyFile, err)
	} else if err != nil && os.IsNotExist(err) {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' keyFile %v does not exist.", WORKLOAD_COMMAND, WORKLOAD_DEPLOY_COMMAND, keyFile)
	}

	// Get the setup info and context for running the command.
	dir, err := setup(homeDirectory, true, true, userCreds)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", WORKLOAD_COMMAND, WORKLOAD_DEPLOY_COMMAND, err)
	}

	// Make sure we're in a workload project.
	if !IsWorkloadProject(dir) {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' current project is not a workload project.", WORKLOAD_COMMAND, WORKLOAD_DEPLOY_COMMAND)
	}

	// Run verification to make sure the project is complete and consistent.
	WorkloadValidate(dir, "")

	// Now we can deploy it.

	// First get the workload definition.
	workloadDef, wderr := GetWorkloadDefinition(dir)
	if wderr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", WORKLOAD_COMMAND, WORKLOAD_DEPLOY_COMMAND, wderr)
	}

	if userCreds == "" {
		userCreds = os.Getenv(DEVTOOL_HZN_USER)
	}
	workloadDef.SignAndPublish(workloadDef.Org, userCreds, keyFile)

	fmt.Printf("Workload project %v deployed.\n", dir)
}

// ========================= private functions ======================================================
