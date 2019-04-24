package dev

import (
	"github.com/open-horizon/anax/cli/cliconfig"
)

const HZNENV_FILE = "hzn.json"

// It creates a hzn.json file that contains the enviromental variables needed for this project.
func CreateHznEnvFile(directory string, org string, specRef string, version string, image_base string) error {
	var config cliconfig.HorizonCliConfig
	config.HZN_ORG_ID = org
	config.MetadataVars = map[string]string{}
	config.MetadataVars["SERVICE_NAME"] = specRef
	config.MetadataVars["SERVICE_VERSION"] = version
	config.MetadataVars["DOCKER_IMAGE_BASE"] = image_base

	return CreateFile(directory, HZNENV_FILE, config)
}
