package dev

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/common"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/containermessage"
	"github.com/open-horizon/anax/i18n"
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

// Add binds for the provided service secret files if the secret name is specified in a dependent service
func AddDependentServiceSecretBinds(deps []*common.ServiceFile, secretsFiles map[string]string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	for _, dep := range deps {
		typedDeploy := common.DeploymentConfig{}

		if deployBytes, err := json.Marshal(dep.Deployment); err != nil {
			msgPrinter.Printf("Failed to marshal deps: %v", err)
			msgPrinter.Println()
		} else if err = json.Unmarshal(deployBytes, &typedDeploy); err != nil {
			msgPrinter.Printf("Failed to unmarshal deps: %v", err)
			msgPrinter.Println()
		} else {
			for svcName, svcInfo := range typedDeploy.Services {
				for secName, _ := range svcInfo.Secrets {
					if svcPath, ok := secretsFiles[secName]; ok {
						svcInfo.Binds = append(svcInfo.Binds, fmt.Sprintf("%v:%v/%v", svcPath, config.HZN_SECRETS_MOUNT, secName))
					} else {
						msgPrinter.Printf("Warning: Secret %v for service %v not specified with %v command.\n", secName, svcName, SERVICE_START_COMMAND)
						msgPrinter.Println()
					}
				}
			}
		}
		dep.Deployment = typedDeploy
	}
}

// Add binds for the provided service secret files to the top-level service container if the secret name is in the service deployment
func AddTopLevelServiceSecretBinds(deployment *containermessage.DeploymentDescription, secretsFiles map[string]string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	for svcName, svcInfo := range deployment.Services {
		for secName, _ := range svcInfo.Secrets {
			if svcPath, ok := secretsFiles[secName]; ok {
				svcInfo.Binds = append(svcInfo.Binds, fmt.Sprintf("%v:%v/%v", svcPath, config.HZN_SECRETS_MOUNT, secName))
			} else {
				msgPrinter.Printf("Warning: Secret %v for service %v not specified with %v command.\n", secName, svcName, SERVICE_START_COMMAND)
				msgPrinter.Println()
			}
		}
	}
}
