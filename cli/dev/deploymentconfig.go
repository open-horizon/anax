package dev

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/open-horizon/anax/containermessage"
	"io/ioutil"
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

	filePath := path.Join(directory, DEPLOYMENT_CONFIG_FILE)

	res := new(DeploymentConfig)
	if fileBytes, err := ioutil.ReadFile(filePath); err != nil {
		return nil, errors.New(fmt.Sprintf("reading %s failed: %v", filePath, err))
	} else if err := json.Unmarshal(fileBytes, res); err != nil {
		return nil, errors.New(fmt.Sprintf("failed to unmarshal %s as deployment config file, error: %v", filePath, err))
	}
	return res, nil

}

// A validation method. Is there enough info in the deployment config to start a container? If not, the
// missing info is returned in the error message.
func (self *DeploymentConfig) CanStartStop() error {
	if len(self.Services) == 0 {
		return errors.New(fmt.Sprintf("no services defined"))
	} else {
		for serviceName, service := range self.Services {
			if len(service.Image) == 0 {
				return errors.New(fmt.Sprintf("no image for service %s", serviceName))
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
