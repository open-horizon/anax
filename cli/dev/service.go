package dev

import (
	"errors"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cli/plugin_registry"
	"github.com/open-horizon/anax/common"
	"github.com/open-horizon/anax/container"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/semanticversion"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// These constants define the hzn dev subcommands supported by this module.
const SERVICE_COMMAND = "service"
const SERVICE_CREATION_COMMAND = "new"
const SERVICE_START_COMMAND = "start"
const SERVICE_STOP_COMMAND = "stop"
const SERVICE_VERIFY_COMMAND = "verify"
const SERVICE_LOG_COMMAND = "log"

const SERVICE_NEW_DEFAULT_VERSION = "0.0.1"

// Create skeletal horizon metadata files to establish a new service project.
func ServiceNew(homeDirectory string, org string, specRef string, version string, images []string, noImageGen bool, dconfig []string, noPattern bool, noPolicy bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

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
				cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("'%v %v' Failed to get the service name from the image name. %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err))
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
	FileNotExist(dir, cmd, SECRETS_FILE, SecretsFileExists)
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
	cliutils.Verbose(msgPrinter.Sprintf("Creating config file for environmental variables: %v/%v", dir, HZNENV_FILE))
	err = CreateHznEnvFile(dir, org, specRef, version, image_base)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
	}

	// Create the metadata files.
	cliutils.Verbose(msgPrinter.Sprintf("Creating user input file: %v/%v", dir, USERINPUT_FILE))
	err = CreateUserInputs(dir, specRef)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
	}

	cliutils.Verbose(msgPrinter.Sprintf("Creating secrets file: %v/%v", dir, SECRETS_FILE))
	err = CreateSecretsFile(dir)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
	}

	cliutils.Verbose(msgPrinter.Sprintf("Creating service definition file: %v/%v", dir, SERVICE_DEFINITION_FILE))
	err = CreateServiceDefinition(dir, specRef, imageInfo, noImageGen, dconfig)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
	}

	if !noPattern {
		cliutils.Verbose(msgPrinter.Sprintf("Creating pattern definition file: %v/%v", dir, PATTERN_DEFINITION_FILE))
		err = CreatePatternDefinition(dir)
		if err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
		}
		if cutil.SliceContains(dconfig, "native") {
			cliutils.Verbose(msgPrinter.Sprintf("Creating pattern definition file: %v/%v", dir, PATTERN_DEFINITION_ALL_ARCHES_FILE))
			err = CreatePatternDefinitionAllArches(dir)
			if err != nil {
				cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
			}
		}
	}

	// Create default service policy file
	if !noPolicy {
		cliutils.Verbose(msgPrinter.Sprintf("Creating service policy file: %v/%v", dir, SERVICE_POLICY_FILE))
		err = CreateServicePolicy(dir)
		if err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
		}
	}

	// create files for source code control.
	cliutils.Verbose(msgPrinter.Sprintf("Creating .gitignore files for source code management."))
	err = CreateSourceCodeManagementFiles(dir)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
	}

	// create the image related files under current direcotry.
	if !noImageGen && specRef != "" && cutil.SliceContains(dconfig, "native") {
		if current_dir, err := os.Getwd(); err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
		} else {
			cliutils.Verbose(msgPrinter.Sprintf("Creating image generation files under %v directory.", current_dir))
			if err := CreateServiceImageFiles(current_dir, dir); err != nil {
				cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_CREATION_COMMAND, err)
			} else {
				msgPrinter.Printf("Created image generation files in %v and horizon metadata files in %v. Edit these files to define and configure your new %v.", current_dir, dir, SERVICE_COMMAND)
				msgPrinter.Println()
			}
		}
	} else {
		msgPrinter.Printf("Created horizon metadata files in %v. Edit these files to define and configure your new %v.", dir, SERVICE_COMMAND)
		msgPrinter.Println()
	}
}

// verify the input parameter for the 'hzn service new' command.
func verifyNewServiceInputs(homeDirectory string, org string, specRef string, version string, images []string, noImageGen bool, dconfig []string, noPattern bool) (string, error) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Verify that env vars are set properly and determine the working directory.
	dir, err := VerifyEnvironment(homeDirectory, false, false, "")
	if err != nil {
		return "", err
	}

	if org == "" && os.Getenv(DEVTOOL_HZN_ORG) == "" {
		return "", fmt.Errorf(msgPrinter.Sprintf("must specify either --org or set the %v environment variable.", DEVTOOL_HZN_ORG))
	}

	// check if the input version is a valid version string
	if version != "" {
		if !semanticversion.IsVersionString(version) {
			return "", fmt.Errorf(msgPrinter.Sprintf("invalid version string: %v", version))
		}
	}

	if len(images) != 0 {
		if len(images) > 1 && !noImageGen {
			return "", fmt.Errorf(msgPrinter.Sprintf("only support one image for a service unless --noImageGen flag is specified."))
		}

		// validate the image
		for _, image := range images {
			if _, path, _, _ := cutil.ParseDockerImagePath(image); path == "" {
				return "", fmt.Errorf(msgPrinter.Sprintf("image %v has invalid format.", image))
			}
		}
	} else {
		if specRef != "" && cutil.SliceContains(dconfig, "native") {
			return "", fmt.Errorf(msgPrinter.Sprintf("please specify the image name with -i flag."))
		}
	}

	// Make sure that the input deployment config type is supported.
	for _, dc := range dconfig {
		if !plugin_registry.DeploymentConfigPlugins.HasPlugin(dc) {
			return "", fmt.Errorf(msgPrinter.Sprintf("unsupported deployment config type: %v", dconfig))
		}
	}

	return dir, nil
}

// Take the filepaths given by --secrets flags
// map the filename which is the same as the secret name to the filepath
func mapSecNameToSecPath(secretPaths []string) map[string]string {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	finalMap := make(map[string]string, len(secretPaths))
	for _, secPath := range secretPaths {
		if _, err := os.Stat(secPath); err != nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Error verifying filepath %v: %v", secPath, err))
		}
		finalMap[filepath.Base(secPath)] = secPath
	}
	return finalMap
}

func ServiceStartTest(homeDirectory string, userInputFile string, configFiles []string, configType string, noFSS bool, userCreds string, secretsFilePaths []string) {
	secretsFilePathsMap := mapSecNameToSecPath(secretsFilePaths)

	// Allow the right plugin to start a test of this service.
	startErr := plugin_registry.DeploymentConfigPlugins.StartTest(homeDirectory, userInputFile, configFiles, configType, noFSS, userCreds, secretsFilePathsMap)
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

func ServiceValidate(homeDirectory string, userInputFile string, configFiles []string, configType string, userCreds string) []string {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Get the setup info and context for running the command.
	dir, err := setup(homeDirectory, true, false, "")
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_VERIFY_COMMAND, err)
	}

	if err := AbstractServiceValidation(dir); err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_VERIFY_COMMAND, err)
	}

	CommonProjectValidation(dir, userInputFile, SERVICE_COMMAND, SERVICE_VERIFY_COMMAND, userCreds, true)

	absFiles := FileValidation(configFiles, configType, SERVICE_COMMAND, SERVICE_VERIFY_COMMAND)

	msgPrinter.Printf("Service project %v verified.", dir)
	msgPrinter.Println()

	return absFiles
}

func searchDependencies(dir string, serviceDef *common.ServiceFile, targetService string) (*common.ServiceFile, error) {

	// check the current service
	if serviceDef.URL == targetService {
		return serviceDef, nil
	}

	// generate dependencies
	serviceDeps, derr := GetServiceDependencies(dir, serviceDef.RequiredServices)
	if derr != nil {
		return nil, derr
	}

	// search dependencies (depth first search)
	for _, dep := range serviceDeps {
		res, rerr := searchDependencies(dir, dep, targetService)
		if rerr != nil {
			return nil, rerr
		}
		if res != nil {
			return res, nil
		}
	}

	// unsuccessful search
	return nil, nil

}

func ServiceLog(homeDirectory string, serviceName string, containerName string, tailing bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Perform the common execution setup.
	dir, _, cw := CommonExecutionSetup(homeDirectory, "", SERVICE_COMMAND, SERVICE_LOG_COMMAND)

	// Get the service definition for this project.
	serviceDef, wderr := GetServiceDefinition(dir, SERVICE_DEFINITION_FILE)
	if wderr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_LOG_COMMAND, wderr)
	}

	targetServiceDef := serviceDef
	if serviceName != "" {
		// Search for the specified service URL
		foundServiceDef, sderr := searchDependencies(dir, serviceDef, serviceName)
		if sderr != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_LOG_COMMAND, sderr)
		}

		// Service URL not found
		if foundServiceDef == nil {
			err := errors.New(i18n.GetMessagePrinter().Sprintf("failed to find the service %v in the current project. If this is a new dependent "+
				"service, please update the dependency list with the 'hzn dev dependency fetch' command.", serviceName))
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_LOG_COMMAND, err)
		} else {
			targetServiceDef = foundServiceDef
		}
	} else {
		serviceName = serviceDef.URL
	}

	// Get the deployment config. This is a top-level service because it's the one being launched, so it is treated as
	// if it is managed by an agreement.
	dc, _, cerr := targetServiceDef.ConvertToDeploymentDescription(true, msgPrinter)
	if cerr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", SERVICE_COMMAND, SERVICE_LOG_COMMAND, cerr)
	}

	// find the log driver for the container (default syslog)
	var logDriver string
	if containerName == "" && len(dc.Services) > 1 {

		// collect the container names
		var containerNames []string
		for name, _ := range dc.Services {
			containerNames = append(containerNames, name)
		}
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("'%v %v' More than one container has been found for deployment: %v. Please specify the service name by -c flag", SERVICE_COMMAND, SERVICE_LOG_COMMAND, strings.Join(containerNames, ", ")))
	} else if containerName != "" {
		found := false
		for name, svc := range dc.Services {
			if name == containerName {
				found = true
				logDriver = svc.LogDriver
			}
		}
		if !found {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("'%v %v': container %v not found in service %v", SERVICE_COMMAND, SERVICE_LOG_COMMAND, containerName, serviceName))
		}
	} else { // containerName == "" && len(dc.Services) == 1
		for name, svc := range dc.Services {
			containerName = name
			logDriver = svc.LogDriver
		}
	}
	if logDriver == "" {
		logDriver = "syslog"
	}

	// Locate the dev container(s) and show logs
	containers, err := findContainers(containerName, "", cw)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("'%v %v' Unable to list containers: %v", SERVICE_COMMAND, SERVICE_LOG_COMMAND), err)
	}
	cliutils.Verbose(msgPrinter.Sprintf("Found containers %v", containers))

	for _, c := range containers {
		if _, isDevService := c.Labels[container.LABEL_PREFIX+".dev_service"]; isDevService {
			msId := c.Labels[container.LABEL_PREFIX+".agreement_id"]

			nonDefaultLogDriverUsed := false
			if nonDefaultLogDriverUsed, err = cliutils.ChekServiceLogPossibility(logDriver); err != nil {
				cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Unable to display log messages: %v", err))
			}

			msgPrinter.Printf("Displaying log messages for dev service %v with instance id prefix %v.", serviceName, msId)
			msgPrinter.Println()
			if tailing {
				msgPrinter.Printf("Use ctrl-C to terminate this command.")
				msgPrinter.Println()
			}

			if runtime.GOOS == "darwin" || nonDefaultLogDriverUsed {
				cliutils.LogMac(msId+"-"+containerName, tailing)
			} else {
				cliutils.LogLinux(strings.ToLower(msId)+"_"+containerName, tailing)
			}
			return
		}
	}

	cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("'%v %v' Cannot find any running container for dev service %s", SERVICE_COMMAND, SERVICE_LOG_COMMAND, serviceName))
}
