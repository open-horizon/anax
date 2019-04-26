package dev

import (
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cli/plugin_registry"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/policy"
	"os"
)

// These constants define the hzn dev subcommands supported by this module.
const SERVICE_COMMAND = "service"
const SERVICE_CREATION_COMMAND = "new"
const SERVICE_START_COMMAND = "start"
const SERVICE_STOP_COMMAND = "stop"
const SERVICE_VERIFY_COMMAND = "verify"

const SERVICE_NEW_DEFAULT_VERSION = "0.0.1"

// Create skeletal horizon metadata files to establish a new service project.
func ServiceNew(homeDirectory string, org string, specRef string, version string, images []string, noImageGen bool, dconfig string, noPattern bool) {

	// validate the parameters
	dir, err := verifyNewServiceInputs(homeDirectory, org, specRef, version, images, noImageGen, dconfig, noPattern)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
	}

	// fill unspecified parameters witht the default
	if len(images) != 0 {
		// get the specRef and version from the image name if not specified
		if specRef == "" {
			specRef1, version1, err := GetServiceSpecFromImage(images[0])
			if err != nil {
				cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' Failed to get the service name from the image name. %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
			} else {
				specRef = specRef1
			}

			if version == "" && version1 != "" {
				version = version1
			}
		}
	}
	if specRef != "" && version == "" {
		version = SERVICE_NEW_DEFAULT_VERSION
	}

	// Create the working directory.
	if err := CreateWorkingDir(dir); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
	}

	// If there are any horizon metadata files already in the directory then we wont create any files.
	cmd := fmt.Sprintf("%v %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND)
	FileNotExist(dir, cmd, USERINPUT_FILE, UserInputExists)
	FileNotExist(dir, cmd, SERVICE_DEFINITION_FILE, ServiceDefinitionExists)
	FileNotExist(dir, cmd, PATTERN_DEFINITION_FILE, PatternDefinitionExists)
	FileNotExist(dir, cmd, PATTERN_DEFINITION_ALL_ARCHES_FILE, PatternDefinitionAllArchesExists)

	if org == "" {
		org = os.Getenv(DEVTOOL_HZN_ORG)
	}

	imageInfo, image_base, err := GetImageInfoFromImageList(images, version, noImageGen)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
	}

	// create env var file
	cliutils.Verbose(fmt.Sprintf("Creating config file for environmental variables: %v/%v", dir, HZNENV_FILE))
	err = CreateHznEnvFile(dir, org, specRef, version, image_base)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
	}

	// Create the metadata files.
	cliutils.Verbose(fmt.Sprintf("Creating user input file: %v/%v", dir, USERINPUT_FILE))
	err = CreateUserInputs(dir, specRef)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
	}

	cliutils.Verbose(fmt.Sprintf("Creating service definition file: %v/%v", dir, SERVICE_DEFINITION_FILE))
	err = CreateServiceDefinition(dir, specRef, imageInfo, dconfig)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
	}

	if !noPattern {
		cliutils.Verbose(fmt.Sprintf("Creating pattern definition file: %v/%v", dir, PATTERN_DEFINITION_FILE))
		err = CreatePatternDefinition(dir)
		if err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
		}
		cliutils.Verbose(fmt.Sprintf("Creating pattern definition file: %v/%v", dir, PATTERN_DEFINITION_ALL_ARCHES_FILE))
		err = CreatePatternDefinitionAllArches(dir)
		if err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
		}
	}

	// create files for source code control.
	cliutils.Verbose(fmt.Sprintf("Creating .gitignore files for source code management."))
	err = CreateSourceCodeManagementFiles(dir)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
	}

	// create the image related files under current direcotry.
	if !noImageGen && specRef != "" {
		if current_dir, err := os.Getwd(); err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
		} else {
			cliutils.Verbose(fmt.Sprintf("Creating image generation files under %v directory.", current_dir))
			if err := CreateServiceImageFiles(current_dir, dir); err != nil {
				cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
			} else {
				fmt.Printf("Created image generation files in %v and horizon metadata files in %v. Edit these files to define and configure your new %v.\n", current_dir, dir, SERVICE_COMMAND)
			}
		}
	} else {
		fmt.Printf("Created horizon metadata files in %v. Edit these files to define and configure your new %v.\n", dir, SERVICE_COMMAND)
	}
}

// verify the input parameter for the 'hzn service new' command.
func verifyNewServiceInputs(homeDirectory string, org string, specRef string, version string, images []string, noImageGen bool, dconfig string, noPattern bool) (string, error) {

	// Verify that env vars are set properly and determine the working directory.
	dir, err := VerifyEnvironment(homeDirectory, false, false, "")
	if err != nil {
		return "", err
	}

	if org == "" && os.Getenv(DEVTOOL_HZN_ORG) == "" {
		return "", fmt.Errorf("must specify either --org or set the %v environment variable.", DEVTOOL_HZN_ORG)
	}

	// check if the input version is a valid version string
	if version != "" {
		if !policy.IsVersionString(version) {
			return "", fmt.Errorf("invalid version string: %v", version)
		}
	}

	if len(images) != 0 {
		if len(images) > 1 && !noImageGen {
			return "", fmt.Errorf("only support one image for a service unless --noImageGen flag is specified.")
		}

		// validate the image
		for _, image := range images {
			if _, path, _, _ := cutil.ParseDockerImagePath(image); path == "" {
				return "", fmt.Errorf("image %v has invalid format.", image)
			}
		}
	} else {
		if specRef != "" {
			return "", fmt.Errorf("please specify the image name with -i flag.")
		}
	}

	// Make sure that the input deployment config type is supported.
	if !plugin_registry.DeploymentConfigPlugins.HasPlugin(dconfig) {
		return "", fmt.Errorf("unsupported deployment config type: %v", dconfig)
	}

	return dir, nil
}

func ServiceStartTest(homeDirectory string, userInputFile string, configFiles []string, configType string, noFSS bool, userCreds string, keyFiles []string) {

	// Allow the right plugin to start a test of this service.
	startErr := plugin_registry.DeploymentConfigPlugins.StartTest(homeDirectory, userInputFile, configFiles, configType, noFSS, userCreds, keyFiles)
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

func ServiceValidate(homeDirectory string, userInputFile string, configFiles []string, configType string, userCreds string, keyFiles []string) []string {

	// Get the setup info and context for running the command.
	dir, err := setup(homeDirectory, true, false, "")
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_VERIFY_COMMAND, err)
	}

	if err := AbstractServiceValidation(dir); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_VERIFY_COMMAND, err)
	}

	CommonProjectValidation(dir, userInputFile, SERVICE_COMMAND, SERVICE_VERIFY_COMMAND, userCreds, keyFiles, true)

	absFiles := FileValidation(configFiles, configType, SERVICE_COMMAND, SERVICE_VERIFY_COMMAND)

	fmt.Printf("Service project %v verified.\n", dir)

	return absFiles
}
