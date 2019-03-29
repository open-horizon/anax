package dev

import (
	"fmt"
)

const HZNENV_FILE = "hzn.cfg"

// It creates a hzn.env file that contains the enviromental variables needed for this project.
func CreateHznEnvFile(directory string, org string, specRef string, version string, image_base string) error {
	content := "# Settings needed to build, publish, and run the Horizon helloworld example edge service.\n# This file will be automatically read by hzn and make.\n\n"
	content += fmt.Sprintf("HZN_ORG_ID=%v\n", org)
	content += fmt.Sprintf("SERVICE_NAME=%v\n", specRef)
	content += fmt.Sprintf("SERVICE_VERSION=%v\n\n", version)

	if specRef == "" {
		content += fmt.Sprintf("# ARCH and SERVICE_VERSION will be added to this when used\n")
		content += fmt.Sprintf("DOCKER_IMAGE_BASE=\n")
	} else if image_base != "" {
		content += fmt.Sprintf("# ARCH and SERVICE_VERSION will be added to this when used\n")
		content += fmt.Sprintf("DOCKER_IMAGE_BASE=%v\n", image_base)
	} else {
		// this is the case when image is generated in other place and no image base could be obtained
		//because user insists to certain image version or digest.
	}

	content += `
# Soon this will not be needed
export HZN_ORG_ID SERVICE_NAME SERVICE_VERSION DOCKER_IMAGE_BASE
`

	return CreateFileWithConent(directory, HZNENV_FILE, content, nil, false)
}
