package dev

import (
	"fmt"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/containermessage"
	"github.com/open-horizon/anax/i18n"
	"path/filepath"
)

const SECRETS_FILE = "servicesecret"

// Check for the existence of the user input config file in the project.
func SecretsFileExists(directory string) (bool, error) {
	return FileExists(directory, SECRETS_FILE)
}

type ServiceSecret struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func CreateSecretsFile(directory string) error {
	fileContents := ServiceSecret{Key: "My Secret Type", Value: "My Secret Value"}

	return CreateFile(directory, SECRETS_FILE, fileContents)
}

// Add binds for the provided service secret files to the top-level service container if the secret name is in the service deployment
func AddServiceSecretBinds(deployment *containermessage.DeploymentDescription, secretsFiles map[string]string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	for svcName, svcInfo := range deployment.Services {
		for secName, _ := range svcInfo.Secrets {
			if svcPath, ok := secretsFiles[secName]; ok {
				// get full path
				fullPath, err := filepath.Abs(svcPath)
				if err != nil {
					msgPrinter.Printf("Warning: Failed to convert file name %v to absolute path. %v", svcPath, err)
					msgPrinter.Println()
				}

				// add binds
				svcInfo.Binds = append(svcInfo.Binds, fmt.Sprintf("%v:%v/%v", fullPath, config.HZN_SECRETS_MOUNT, secName))
			} else {
				msgPrinter.Printf("Warning: Secret %v for service %v not specified with %v command.", secName, svcName, SERVICE_START_COMMAND)
				msgPrinter.Println()
			}
		}
	}
}
