package kube_deployment

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cli/dev"
	"github.com/open-horizon/anax/cli/plugin_registry"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/rsapss-tool/sign"
	"io/ioutil"
	"os"
	"path/filepath"
)

func init() {
	plugin_registry.Register("kube-operator", NewKubeDeploymentConfigPlugin())
}

type KubeDeploymentConfigPlugin struct {
}

func NewKubeDeploymentConfigPlugin() plugin_registry.DeploymentConfigPlugin {
	return new(KubeDeploymentConfigPlugin)
}

func (p *KubeDeploymentConfigPlugin) Sign(dep map[string]interface{}, keyFilePath string, ctx plugin_registry.PluginContext) (bool, string, string, error) {

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if owned, err := p.Validate(dep); !owned || err != nil {
		return owned, "", "", err
	}

	// Grab the kube operator file from the deployment config. The file might be relative to the
	// service definition file.
	operatorFilePath := dep["yaml_archive"].(string)
	if operatorFilePath = filepath.Clean(operatorFilePath); operatorFilePath == "." {
		return true, "", "", errors.New(msgPrinter.Sprintf("cleaned %v resulted in an empty string.", dep["yaml_archive"].(string)))
	}

	if currentDir, ok := (ctx.Get("currentDir")).(string); !ok {
		return true, "", "", errors.New(msgPrinter.Sprintf("plugin context must include 'currentDir' as the current directory of the service definition file"))
	} else if !filepath.IsAbs(operatorFilePath) {
		operatorFilePath = filepath.Join(currentDir, operatorFilePath)
	}

	// Get the base 64 encoding of the kube operator, and put it into the deployment config.
	if b64, err := ConvertFileToB64String(operatorFilePath); err != nil {
		return true, "", "", errors.New(msgPrinter.Sprintf("unable to read kube operator %v, error %v", dep["yaml_archive"], err))
	} else {
		dep["yaml_archive"] = b64
	}

	// handle image pulling
	image := dep["operator_image"].(string)
	var dontTouchImage, pullImage, ok bool
	dontTouchImage, ok = (ctx.Get("dontTouchImage")).(bool)
	if !ok {
		dontTouchImage = false
	}
	pullImage, ok = (ctx.Get("pullImage")).(bool)
	if !ok {
		pullImage = false
	}

	newImage := cliutils.GetNewDockerImageName(image, dontTouchImage, pullImage)
	if newImage != image {
		msgPrinter.Printf("Using '%s' in 'operator_image' field instead of '%s'", newImage, image)
		msgPrinter.Println()
		dep["operator_image"] = newImage
	}

	// Stringify and sign the deployment string.
	deployment, err := json.Marshal(dep)
	if err != nil {
		return true, "", "", errors.New(msgPrinter.Sprintf("failed to marshal cluster deployment string %v, error %v", dep, err))
	}
	depStr := string(deployment)

	sig, err := sign.Input(keyFilePath, deployment)
	if err != nil {
		return true, "", "", errors.New(msgPrinter.Sprintf("problem signing cluster deployment string with %s: %v", keyFilePath, err))
	}

	return true, depStr, sig, nil
}

func (p *KubeDeploymentConfigPlugin) GetContainerImages(dep interface{}) (bool, []string, error) {
	var imageList []string
	if owned, err := p.Validate(dep); !owned || err != nil {
		return owned, imageList, err
	}

	// the Validate has verified the operator_image field
	dc := dep.(map[string]interface{})
	image := dc["operator_image"].(string)
	imageList = append(imageList, image)

	return true, imageList, nil
}

func (p *KubeDeploymentConfigPlugin) DefaultConfig(imageInfo interface{}) interface{} {
	return map[string]interface{}{
		"yaml_archive":   "",
		"operator_image": "",
	}
}

func (p *KubeDeploymentConfigPlugin) Validate(dep interface{}) (bool, error) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if dc, ok := dep.(map[string]interface{}); !ok {
		return false, nil
	} else if c, ok := dc["yaml_archive"]; !ok {
		return false, nil
	} else if r, ok := dc["operator_image"]; !ok {
		return false, nil
	} else if ca, ok := c.(string); !ok {
		return true, errors.New(msgPrinter.Sprintf("yaml_archive must have a string type value, has %T", c))
	} else if rn, ok := r.(string); !ok {
		return true, errors.New(msgPrinter.Sprintf("operator_image must have a string type value, has %T", r))
	} else if len(ca) == 0 || len(rn) == 0 {
		return true, errors.New(msgPrinter.Sprintf("yaml_archive and operator_image must be non-empty strings"))
	} else {
		return true, nil
	}
}

func (p *KubeDeploymentConfigPlugin) StartTest(homeDirectory string, userInputFile string, configFiles []string, configType string, noFSS bool, userCreds string) bool {

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Run verification before trying to start anything.
	dev.ServiceValidate(homeDirectory, userInputFile, configFiles, configType, userCreds)

	// Perform the common execution setup.
	dir, _, _ := dev.CommonExecutionSetup(homeDirectory, userInputFile, dev.SERVICE_COMMAND, dev.SERVICE_START_COMMAND)

	// Get the service definition, so that we can look at the user input variable definitions.
	serviceDef, sderr := dev.GetServiceDefinition(dir, dev.SERVICE_DEFINITION_FILE)
	if sderr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, fmt.Sprintf("'%v %v' %v", dev.SERVICE_COMMAND, dev.SERVICE_START_COMMAND, sderr))
	}

	// Now that we have the service def, we can check if we own the deployment config object.
	if owned, err := p.Validate(serviceDef.ClusterDeployment); !owned || err != nil {
		return false
	}

	cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("'%v %v' not supported for Kube operator deployments", dev.SERVICE_COMMAND, dev.SERVICE_START_COMMAND))

	// For the compiler
	return true
}

func (p *KubeDeploymentConfigPlugin) StopTest(homeDirectory string) bool {

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Perform the common execution setup.
	dir, _, _ := dev.CommonExecutionSetup(homeDirectory, "", dev.SERVICE_COMMAND, dev.SERVICE_START_COMMAND)

	// Get the service definition, so that we can look at the user input variable definitions.
	serviceDef, sderr := dev.GetServiceDefinition(dir, dev.SERVICE_DEFINITION_FILE)
	if sderr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, fmt.Sprintf("'%v %v' %v", dev.SERVICE_COMMAND, dev.SERVICE_START_COMMAND, sderr))
	}

	// Now that we have the service def, we can check if we own the deployment config object.
	if owned, err := p.Validate(serviceDef.ClusterDeployment); !owned || err != nil {
		return false
	}

	cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("'%v %v' not supported for Kube operator deployments", dev.SERVICE_COMMAND, dev.SERVICE_START_COMMAND))
	// For the compiler
	return true
}

// Convert a file into a base 64 encoded string. The input filepath is assumed to be absolute.
func ConvertFileToB64String(filePath string) (string, error) {

	// Make sure the file actually exists.
	if _, err := os.Stat(filePath); err != nil {
		return "", err
	}

	// Read in the file and convert the contents to a base 64 encoded string.
	if fileBytes, err := ioutil.ReadFile(filePath); err != nil {
		return "", err
	} else {
		b64String := base64.StdEncoding.EncodeToString(fileBytes)
		return b64String, nil
	}
}
