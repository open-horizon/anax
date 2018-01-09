package dev

import (
	"github.com/open-horizon/anax/cli/cliutils"
)

const DEPENDENCIES_FILE = "dependency.definition.json"

type Dependencies struct {
}

func DependencyFetch(homeDirectory string) {
	cliutils.Fatal(cliutils.INTERNAL_ERROR, "'dependency fetch' not supported yet.")
}

func DependencyList(homeDirectory string) {
	cliutils.Fatal(cliutils.INTERNAL_ERROR, "'dependency list' not supported yet.")
}

// Sort of like a constructor, it creates an in memory object except that it is created from the dependency config
// file in the current project. This function assumes the caller has determined the exact location of the file.
func GetDependencies(directory string) (*Dependencies, error) {

	res := new(Dependencies)

	// GetFile will write to the res object, demarshalling the bytes into a json object that can be returned.
	if err := GetFile(directory, DEPENDENCIES_FILE, res); err != nil {
		return nil, err
	}
	return res, nil

}

// Sort of like a constructor, it creates a skeletal dependency config object and writes it to the project
// in the file system.
func CreateDependencies(directory string) error {

	// Create a skeletal dependency config object with fillins/place-holders for configuration.
	res := new(Dependencies)

	// Convert the object to JSON and write it into the project.
	return CreateFile(directory, DEPENDENCIES_FILE, res)

}

// Check for the existence of the dependency config file in the project.
func DependenciesExists(directory string) (bool, error) {
	return FileExists(directory, DEPENDENCIES_FILE)
}

// Validate that the microservice definition file is complete and coherent with the rest of the definitions in the project.
// If the file is not valid the reason will be returned in the error.
func ValidateDependencies(directory string) error {

	_, deperr := GetDependencies(directory)
	if deperr != nil {
		return deperr
	}

	// filePath := path.Join(directory, DEPENDENCIES_FILE)

	return nil
}
