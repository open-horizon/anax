package dev

import (
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
)

const MICROSERVICE_CREATION_COMMAND = "new"

// Create skeletal horizon metadata files to establish a new microservice project.
func MicroserviceNew(homeDirectory string) {

	// Verify that env vars are set properly and determine the working directory.
	dir, err := VerifyEnvironment(homeDirectory, false)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'microservice %v' %v", MICROSERVICE_CREATION_COMMAND, err)
	}

	// Create the working directory.
	if err := CreateWorkingDir(dir); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'microservice %v' %v", MICROSERVICE_CREATION_COMMAND, err)
	}

	// If there are any horizon metadata files already in the directory then we wont create any files.
	cmd := fmt.Sprintf("microservice %v", MICROSERVICE_CREATION_COMMAND)
	FileNotExist(dir, cmd, DEPLOYMENT_CONFIG_FILE, DeploymentConfigExists)
	FileNotExist(dir, cmd, USERINPUT_FILE, UserInputExists)
	FileNotExist(dir, cmd, MICROSERVICE_DEFINITION_FILE, MicroserviceDefinitionExists)
	FileNotExist(dir, cmd, DEPENDENCIES_FILE, DependenciesExists)

	// Create the metadata files.
	if err := CreateDeploymentConfig(dir); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'microservice %v' %v", MICROSERVICE_CREATION_COMMAND, err)
	} else if err := CreateUserInputs(dir, false); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'microservice %v' %v", MICROSERVICE_CREATION_COMMAND, err)
	} else if err := CreateMicroserviceDefinition(dir); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'microservice %v' %v", MICROSERVICE_CREATION_COMMAND, err)
	} else if err := CreateDependencies(dir); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'microservice %v' %v", MICROSERVICE_CREATION_COMMAND, err)
	}

	cliutils.Verbose("Created horizon metadata files. Edit these files to define and configure your new microservice.")
}

func MicroserviceStartTest(homeDirectory string, userInputFile string) {
	cliutils.Fatal(cliutils.INTERNAL_ERROR, "'microservice start' not supported yet.")
}

func MicroserviceStopTest(homeDirectory string) {
	cliutils.Fatal(cliutils.INTERNAL_ERROR, "'microservice stop' not supported yet.")
}

func MicroserviceValidate(homeDirectory string, userInputFile string) {
	cliutils.Fatal(cliutils.INTERNAL_ERROR, "'microservice verify' not supported yet.")
}

func MicroserviceDeploy(homeDirectory string) {
	cliutils.Fatal(cliutils.INTERNAL_ERROR, "'microservice deploy' not supported yet.")
}
