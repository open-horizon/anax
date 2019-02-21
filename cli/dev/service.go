package dev

import (
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cli/plugin_registry"
	"os"
)

// These constants define the hzn dev subcommands supported by this module.
const SERVICE_COMMAND = "service"
const SERVICE_CREATION_COMMAND = "new"
const SERVICE_START_COMMAND = "start"
const SERVICE_STOP_COMMAND = "stop"
const SERVICE_VERIFY_COMMAND = "verify"

// Create skeletal horizon metadata files to establish a new service project.
func ServiceNew(homeDirectory string, org string, dconfig string) {

	// Verify that env vars are set properly and determine the working directory.
	dir, err := VerifyEnvironment(homeDirectory, false, false, "")
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
	}

	if org == "" && os.Getenv(DEVTOOL_HZN_ORG) == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' must specify either --org or set the %v environment variable.", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, DEVTOOL_HZN_ORG)
	}

	// Make sure that the input deployment config type is supported.
	if !plugin_registry.DeploymentConfigPlugins.HasPlugin(dconfig) {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, fmt.Sprintf("unsupported deployment config type: %v", dconfig))
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
	if err := CreateUserInputs(dir, org); err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
	} else if err := CreateServiceDefinition(dir, org, dconfig); err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
	}

	fmt.Printf("Created horizon metadata files in %v. Edit these files to define and configure your new %v.\n", dir, SERVICE_COMMAND)
}

func ServiceStartTest(homeDirectory string, userInputFile string, configFiles []string, configType string) {

	// Allow the right plugin to start a test of this service.
	startErr := plugin_registry.DeploymentConfigPlugins.StartTest(homeDirectory, userInputFile, configFiles, configType)
	if startErr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "%v", startErr)
	}

}

// Services are stopped in the reverse order they were started, parents first and then leaf nodes last in order
// to minimize the possibility of a parent throwing an error during execution because a leaf node is gone.
func ServiceStopTest(homeDirectory string) {

	// Allow the right plugin to stop a test of this service.
	stopErr := plugin_registry.DeploymentConfigPlugins.StopTest(homeDirectory)
	if stopErr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "%v", stopErr)
	}

}

func ServiceValidate(homeDirectory string, userInputFile string, configFiles []string, configType string) []string {

	// Get the setup info and context for running the command.
	dir, err := setup(homeDirectory, true, false, "")
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_VERIFY_COMMAND, err)
	}

	if err := AbstractServiceValidation(dir); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_VERIFY_COMMAND, err)
	}

	CommonProjectValidation(dir, userInputFile, SERVICE_COMMAND, SERVICE_VERIFY_COMMAND)

	absFiles := FileValidation(configFiles, configType, SERVICE_COMMAND, SERVICE_VERIFY_COMMAND)

	fmt.Printf("Service project %v verified.\n", dir)

	return absFiles
}
