package dev

import (
	"fmt"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/open-horizon/anax/cli/cliutils"
	"os"
)

// These constants define the hzn dev subcommands supported by this module.
const MICROSERVICE_COMMAND = "microservice"
const MICROSERVICE_CREATION_COMMAND = "new"
const MICROSERVICE_START_COMMAND = "start"
const MICROSERVICE_STOP_COMMAND = "stop"
const MICROSERVICE_VERIFY_COMMAND = "verify"
const MICROSERVICE_DEPLOY_COMMAND = "publish"

// Create skeletal horizon metadata files to establish a new microservice project.
func MicroserviceNew(homeDirectory string, org string) {

	// Verify that env vars are set properly and determine the working directory.
	dir, err := VerifyEnvironment(homeDirectory, false, false, "")
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", MICROSERVICE_COMMAND, MICROSERVICE_CREATION_COMMAND, err)
	}

	if org == "" && os.Getenv(DEVTOOL_HZN_ORG) == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' must specify either --org or set the %v environment variable.", WORKLOAD_COMMAND, WORKLOAD_CREATION_COMMAND, DEVTOOL_HZN_ORG)
	}

	// Create the working directory.
	if err := CreateWorkingDir(dir); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", MICROSERVICE_COMMAND, MICROSERVICE_CREATION_COMMAND, err)
	}

	// If there are any horizon metadata files already in the directory then we wont create any files.
	cmd := fmt.Sprintf("%v %v", MICROSERVICE_COMMAND, MICROSERVICE_CREATION_COMMAND)
	FileNotExist(dir, cmd, USERINPUT_FILE, UserInputExists)
	FileNotExist(dir, cmd, MICROSERVICE_DEFINITION_FILE, MicroserviceDefinitionExists)
	FileNotExist(dir, cmd, DEPENDENCIES_FILE, DependenciesExists)

	if org == "" {
		org = os.Getenv(DEVTOOL_HZN_ORG)
	}

	// Create the metadata files.
	if err := CreateUserInputs(dir, false, org); err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", MICROSERVICE_COMMAND, MICROSERVICE_CREATION_COMMAND, err)
	} else if err := CreateMicroserviceDefinition(dir, org); err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", MICROSERVICE_COMMAND, MICROSERVICE_CREATION_COMMAND, err)
	} else if err := CreateDependencies(dir); err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", MICROSERVICE_COMMAND, MICROSERVICE_CREATION_COMMAND, err)
	}

	fmt.Printf("Created horizon metadata files in %v. Edit these files to define and configure your new %v.\n", dir, MICROSERVICE_COMMAND)
}

func MicroserviceStartTest(homeDirectory string, userInputFile string) {

	// Run verification before trying to start anything.
	MicroserviceValidate(homeDirectory, userInputFile)

	// Perform the common execution setup.
	dir, userInputs, cw := commonExecutionSetup(homeDirectory, userInputFile, MICROSERVICE_COMMAND, MICROSERVICE_START_COMMAND)

	// Get the microservice definition, so that we can look at the user input variable definitions.
	microserviceDef, wderr := GetMicroserviceDefinition(dir)
	if wderr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", MICROSERVICE_COMMAND, MICROSERVICE_START_COMMAND, wderr)
	}

	dc, deployment, cerr := microserviceDef.ConvertToDeploymentDescription()
	if cerr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", MICROSERVICE_COMMAND, MICROSERVICE_START_COMMAND, cerr)
	}

	// Now we can start the microservice container.
	_, err := startMicroservice(deployment, microserviceDef.SpecRef, microserviceDef.Version, userInputs.Global, microserviceDef.UserInputs, userInputs, microserviceDef.Org, dc, cw, map[string]docker.ContainerNetwork{})
	if err != nil {

		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v.", MICROSERVICE_COMMAND, MICROSERVICE_START_COMMAND, err)
	}

}

func MicroserviceStopTest(homeDirectory string) {

	// Perform the common execution setup.
	dir, _, cw := commonExecutionSetup(homeDirectory, "", MICROSERVICE_COMMAND, MICROSERVICE_STOP_COMMAND)

	// Get the microservice definition.
	microserviceDef, wderr := GetMicroserviceDefinition(dir)
	if wderr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", MICROSERVICE_COMMAND, MICROSERVICE_STOP_COMMAND, wderr)
	}

	// Get the deployment config.
	dc, _, cerr := microserviceDef.ConvertToDeploymentDescription()
	if cerr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", MICROSERVICE_COMMAND, MICROSERVICE_STOP_COMMAND, cerr)
	}

	// Stop the microservice
	err := stopMicroservice(dc, cw)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", MICROSERVICE_COMMAND, MICROSERVICE_STOP_COMMAND, err)
	}

	fmt.Printf("Stopped microservice.\n")
}

func MicroserviceValidate(homeDirectory string, userInputFile string) {

	// Get the setup info and context for running the command.
	dir, err := setup(homeDirectory, true, false, "")
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", MICROSERVICE_COMMAND, MICROSERVICE_VERIFY_COMMAND, err)
	}

	// Make sure we're in a microservice project.
	if !IsMicroserviceProject(dir) {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' current project is not a microservice project.", MICROSERVICE_COMMAND, MICROSERVICE_VERIFY_COMMAND)
	}

	// Validate Microservice Definition
	if verr := ValidateMicroserviceDefinition(dir); verr != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' project does not validate. %v ", MICROSERVICE_COMMAND, MICROSERVICE_VERIFY_COMMAND, verr)
	}

	CommonProjectValidation(dir, userInputFile, MICROSERVICE_COMMAND, MICROSERVICE_VERIFY_COMMAND)

	fmt.Printf("Microservice project %v verified.\n", dir)
}

func MicroserviceDeploy(homeDirectory string, keyFile string, userCreds string) {

	// Validate the inputs
	if keyFile == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' must specify a keyFile. See hzn dev %v %v --help.", MICROSERVICE_COMMAND, MICROSERVICE_DEPLOY_COMMAND, MICROSERVICE_COMMAND, MICROSERVICE_DEPLOY_COMMAND)
	} else if _, err := os.Stat(keyFile); err != nil && !os.IsNotExist(err) {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' error checking existence of %v, error: %v.", MICROSERVICE_COMMAND, MICROSERVICE_DEPLOY_COMMAND, keyFile, err)
	} else if err != nil && os.IsNotExist(err) {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' keyFile %v does not exist.", MICROSERVICE_COMMAND, MICROSERVICE_DEPLOY_COMMAND, keyFile)
	}

	// Get the setup info and context for running the command.
	dir, err := setup(homeDirectory, true, true, userCreds)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", MICROSERVICE_COMMAND, MICROSERVICE_DEPLOY_COMMAND, err)
	}

	// Make sure we're in a microservice project.
	if !IsMicroserviceProject(dir) {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' current project is not a microservice project.", MICROSERVICE_COMMAND, MICROSERVICE_DEPLOY_COMMAND)
	}

	// Run verification to make sure the project is complete and consistent.
	MicroserviceValidate(dir, "")

	// Now we can deploy it.

	// First get the microservice definition.
	microserviceDef, wderr := GetMicroserviceDefinition(dir)
	if wderr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", MICROSERVICE_COMMAND, MICROSERVICE_DEPLOY_COMMAND, wderr)
	}

	// Sign and publish the project
	if userCreds == "" {
		userCreds = os.Getenv(DEVTOOL_HZN_USER)
	}
	microserviceDef.SignAndPublish(microserviceDef.Org, userCreds, keyFile)

	fmt.Printf("Microservice project %v deployed.\n", dir)
}
