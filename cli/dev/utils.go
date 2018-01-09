package dev

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
)

const DEVTOOL_HZN_ORG = "HZN_ORG"
const DEVTOOL_HZN_USER = "HZN_USER"
const DEVTOOL_HZN_PASSWORD = "HZN_PASSWORD"
const DEVTOOL_HZN_EXCHANGE_URL = "HZN_EXCHANGE_URL"

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
func VerifyEnvironment(homeDirectory string, mustExist bool) (string, error) {
	// Make sure the env vars needed by the dev tools are setup
	if os.Getenv(DEVTOOL_HZN_ORG) == "" {
		return "", errors.New(fmt.Sprintf("Environment variable %v must be set to use the 'workload start/stop' commands.", DEVTOOL_HZN_ORG))
	} else if os.Getenv(DEVTOOL_HZN_EXCHANGE_URL) == "" {
		return "", errors.New(fmt.Sprintf("Environment variable %v must be set to use the 'workload start/stop' commands.", DEVTOOL_HZN_EXCHANGE_URL))
	}

	// Get the directory we're working in
	dir, err := GetWorkingDir(homeDirectory, mustExist)
	if err != nil {
		return "", errors.New(fmt.Sprintf("has no working directory, error: %v", err))
	} else {
		return dir, nil
	}

}

// Indicates whether or not the given project is a workload project
func IsWorkloadProject(directory string) bool {
	if ex, err := DeploymentConfigExists(directory); !ex || err != nil {
		return false
	} else if ex, err := UserInputExists(directory); !ex || err != nil {
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
	if ex, err := DeploymentConfigExists(directory); !ex || err != nil {
		return false
	} else if ex, err := UserInputExists(directory); !ex || err != nil {
		return false
	} else if ex, err := MicroserviceDefinitionExists(directory); !ex || err != nil {
		return false
	} else if ex, err := DependenciesExists(directory); !ex || err != nil {
		return false
	}
	return true
}
