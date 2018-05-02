package dev

import (
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cutil"
	"os"
)

// These constants define the hzn dev subcommands supported by this module.
const SERVICE_COMMAND = "service"
const SERVICE_CREATION_COMMAND = "new"
const SERVICE_START_COMMAND = "start"
const SERVICE_STOP_COMMAND = "stop"
const SERVICE_VERIFY_COMMAND = "verify"
const SERVICE_DEPLOY_COMMAND = "publish"

// Create skeletal horizon metadata files to establish a new microservice project.
func ServiceNew(homeDirectory string, org string) {

	// Verify that env vars are set properly and determine the working directory.
	dir, err := VerifyEnvironment(homeDirectory, false, false, "")
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
	}

	if org == "" && os.Getenv(DEVTOOL_HZN_ORG) == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' must specify either --org or set the %v environment variable.", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, DEVTOOL_HZN_ORG)
	}

	// Create the working directory.
	if err := CreateWorkingDir(dir); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
	}

	// If there are any horizon metadata files already in the directory then we wont create any files.
	cmd := fmt.Sprintf("%v %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND)
	FileNotExist(dir, cmd, USERINPUT_FILE, UserInputExists)
	FileNotExist(dir, cmd, SERVICE_DEFINITION_FILE, ServiceDefinitionExists)

	if org == "" {
		org = os.Getenv(DEVTOOL_HZN_ORG)
	}

	// Create the metadata files.
	if err := CreateUserInputs(dir, false, true, org); err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
	} else if err := CreateServiceDefinition(dir, org); err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
	}

	fmt.Printf("Created horizon metadata files in %v. Edit these files to define and configure your new %v.\n", dir, SERVICE_COMMAND)
}

func ServiceStartTest(homeDirectory string, userInputFile string) {

	// Run verification before trying to start anything.
	ServiceValidate(homeDirectory, userInputFile)

	// Perform the common execution setup.
	dir, userInputs, cw := commonExecutionSetup(homeDirectory, userInputFile, SERVICE_COMMAND, SERVICE_START_COMMAND)

	// Get the service definition, so that we can look at the user input variable definitions.
	serviceDef, sderr := GetServiceDefinition(dir, SERVICE_DEFINITION_FILE)
	if sderr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_START_COMMAND, sderr)
	}

	// Get the metadata for each dependency. The metadata is returned as a list of service definition files from
	// the project's dependency directory.
	deps, derr := GetServiceDependencies(dir, serviceDef.RequiredServices)
	if derr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' unable to get service dependencies, %v", SERVICE_COMMAND, SERVICE_START_COMMAND, derr)
	}

	// Log the starting of dependencies if there are any.
	if len(deps) != 0 {
		cliutils.Verbose("Starting dependencies.")
	}

	// If the service has dependencies, get them started first.
	msNetworks, perr := processStartDependencies(dir, deps, userInputs.Global, userInputs.Services, cw)
	if perr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' unable to start service dependencies, %v", SERVICE_COMMAND, SERVICE_START_COMMAND, perr)
	}

	// Get the service's deployment description from the deployment config in the definition.
	dc, deployment, cerr := serviceDef.ConvertToDeploymentDescription(true)
	if cerr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_START_COMMAND, cerr)
	}

	// Generate an agreement id for testing purposes.
	agreementId, aerr := cutil.GenerateAgreementId()
	if aerr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' unable to generate test agreementId, %v", SERVICE_COMMAND, SERVICE_START_COMMAND, aerr)
	}

	// Now we can start the service container.
	_, err := startContainers(deployment, serviceDef.URL, serviceDef.Version, userInputs.Global, serviceDef.UserInputs, userInputs.Services, serviceDef.Org, dc, cw, msNetworks, true, true, agreementId)
	//    _, err := startService(deployment, serviceDef, deps, userInputs.Global, userInputs.Services, dc, cw, map[string]docker.ContainerNetwork{})
	//    _, err := startService(deployment, serviceDef.URL, serviceDef.Version, userInputs.Global, serviceDef.UserInputs, userInputs.Services, serviceDef.Org, dc, cw, map[string]docker.ContainerNetwork{})
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v.", SERVICE_COMMAND, SERVICE_START_COMMAND, err)
	}

}

// Services are stopped in the reverse order they were started, parents first and then leaf nodes last in order
// to minimize the possibility of a parent throwing an error during execution because a leaf node is gone.
func ServiceStopTest(homeDirectory string) {

	// Perform the common execution setup.
	dir, _, cw := commonExecutionSetup(homeDirectory, "", SERVICE_COMMAND, SERVICE_STOP_COMMAND)

	// Get the service definition for this project.
	serviceDef, wderr := GetServiceDefinition(dir, SERVICE_DEFINITION_FILE)
	if wderr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_STOP_COMMAND, wderr)
	}

	// Get the deployment config. This is a top-level service because it's the one being launched, so it is treated as
	// if it is managed by an agreement.
	dc, _, cerr := serviceDef.ConvertToDeploymentDescription(true)
	if cerr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_STOP_COMMAND, cerr)
	}

	// Stop the service.
	err := stopService(dc, cw)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_STOP_COMMAND, err)
	}

	// Get the metadata for each dependency. The metadata is returned as a list of service definition files from
	// the project's dependency directory.
	deps, derr := GetServiceDependencies(dir, serviceDef.RequiredServices)
	if derr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' unable to get service dependencies, %v", SERVICE_COMMAND, SERVICE_STOP_COMMAND, derr)
	}

	// If the service has dependencies, stop them.
	if err := processStopDependencies(dir, deps, cw); err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' unable to stop service dependencies, %v", SERVICE_COMMAND, SERVICE_STOP_COMMAND, err)
	}

	fmt.Printf("Stopped service.\n")
}

func ServiceValidate(homeDirectory string, userInputFile string) {

	// Get the setup info and context for running the command.
	dir, err := setup(homeDirectory, true, false, "")
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_VERIFY_COMMAND, err)
	}

	if err := AbstractServiceValidation(dir, true); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_VERIFY_COMMAND, err)
	}

	CommonProjectValidation(dir, userInputFile, SERVICE_COMMAND, SERVICE_VERIFY_COMMAND)

	fmt.Printf("Service project %v verified.\n", dir)
}

func ServiceDeploy(homeDirectory string, keyFile string, pubKeyFilePath string, userCreds string, dontTouchImage bool, registryTokens []string) {

	// Validate the inputs
	if keyFile == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' must specify a keyFile. See hzn dev %v %v --help.", SERVICE_COMMAND, SERVICE_DEPLOY_COMMAND, SERVICE_COMMAND, SERVICE_DEPLOY_COMMAND)
	} else if _, err := os.Stat(keyFile); err != nil && !os.IsNotExist(err) {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' error checking existence of %v, error: %v.", SERVICE_COMMAND, SERVICE_DEPLOY_COMMAND, keyFile, err)
	} else if err != nil && os.IsNotExist(err) {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' keyFile %v does not exist.", SERVICE_COMMAND, SERVICE_DEPLOY_COMMAND, keyFile)
	}

	// Get the setup info and context for running the command.
	dir, err := setup(homeDirectory, true, true, userCreds)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_DEPLOY_COMMAND, err)
	}

	// Make sure we're in a service project.
	if !IsServiceProject(dir) {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' current project is not a microservice project.", SERVICE_COMMAND, SERVICE_DEPLOY_COMMAND)
	}

	// Run verification to make sure the project is complete and consistent.
	ServiceValidate(dir, "")

	// Now we can deploy it.

	// First get the service definition.
	serviceDef, wderr := GetServiceDefinition(dir, SERVICE_DEFINITION_FILE)
	if wderr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_DEPLOY_COMMAND, wderr)
	}

	// Sign and publish the project
	if userCreds == "" {
		userCreds = os.Getenv(DEVTOOL_HZN_USER)
	}

	// Parse creds to figure out if an API key is in use
	cliutils.SetWhetherUsingApiKey(userCreds)

	// Invoke the re-usable part of hzn exchange service publish to actually do the publish.
	serviceDef.SignAndPublish(serviceDef.Org, userCreds, keyFile, pubKeyFilePath, dontTouchImage, registryTokens)

	fmt.Printf("Service project %v deployed.\n", dir)
}
