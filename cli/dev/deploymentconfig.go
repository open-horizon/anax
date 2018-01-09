package dev

import (
	"errors"
	"fmt"
	"github.com/open-horizon/anax/containermessage"
	"path"
)

const DEPLOYMENT_CONFIG_FILE = "deployment.config.json"

type DeploymentConfig struct {
	Services map[string]*containermessage.Service `json:"services"`
}

func (dc DeploymentConfig) CLIString() string {
	for serviceName, service := range dc.Services {
		return fmt.Sprintf("service %v from image %v", serviceName, service.Image)
	}
	// This is for the compiler
	return ""
}

// Sort of like a constructor, it creates an in memory object except that it is created from the deployment config
// file in the current project. This function assumes the caller has determined the exact location of the
// deployment config file.
func GetDeploymentConfig(directory string) (*DeploymentConfig, error) {

	res := new(DeploymentConfig)

	// GetFile will write to the res object, demarshalling the bytes into a json object that can be returned.
	if err := GetFile(directory, DEPLOYMENT_CONFIG_FILE, res); err != nil {
		return nil, err
	}
	return res, nil
}

// Sort of like a constructor, it creates a skeletal deployment config object and writes it to the project
// in the file system.
func CreateDeploymentConfig(directory string) error {

	// Create a skeletal deployment config object with fillins/place-holders for configuration.
	res := new(DeploymentConfig)
	res.Services = make(map[string]*containermessage.Service)
	res.Services[""] = &containermessage.Service{
		Image:       "",
		Environment: []string{"ENV_VAR_HERE=SOME_VALUE"},
	}

	// Convert the object to JSON and write it into the project.
	return CreateFile(directory, DEPLOYMENT_CONFIG_FILE, res)

}

// Check for the existence of the deployment config file in the project.
func DeploymentConfigExists(directory string) (bool, error) {
	return FileExists(directory, DEPLOYMENT_CONFIG_FILE)
}

// A validation method. Is there enough info in the deployment config to start a container? If not, the
// missing info is returned in the error message.
func (self *DeploymentConfig) CanStartStop() error {
	if len(self.Services) == 0 {
		return errors.New(fmt.Sprintf("no services defined"))
	} else {
		for serviceName, service := range self.Services {
			if len(serviceName) == 0 {
				return errors.New(fmt.Sprintf("no service name"))
			} else if len(service.Image) == 0 {
				return errors.New(fmt.Sprintf("no docker image for service %s", serviceName))
			}
		}
	}
	return nil
}

// Convert a Deployment Configuration to a full Deployment Description.
func (self *DeploymentConfig) ConvertToDeploymentDescription() (*containermessage.DeploymentDescription, error) {
	return &containermessage.DeploymentDescription{
		Services: self.Services,
		ServicePattern: containermessage.Pattern{
			Shared: map[string][]string{},
		},
		Infrastructure: false,
		Overrides:      map[string]*containermessage.Service{},
	}, nil
}

// Validate that the deployment config file is complete and coherent with the rest of the definitions in the project.
// If the file is not valid the reason will be returned in the error.
func ValidateDeploymentConfig(directory string) error {

	dc, dcerr := GetDeploymentConfig(directory)
	if dcerr != nil {
		return dcerr
	}

	filePath := path.Join(directory, DEPLOYMENT_CONFIG_FILE)
	if err := dc.CanStartStop(); err != nil {
		return errors.New(fmt.Sprintf("%v: %v", filePath, err))
	}
	return nil
}
