package kube_deployment

import (
	"crypto/rsa"
	"crypto/sha256"
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

const KUBE_DEPLOYMENT_CONFIG_TYPE = "cluster"

func init() {
	plugin_registry.Register(KUBE_DEPLOYMENT_CONFIG_TYPE, NewKubeDeploymentConfigPlugin())
}

type KubeDeploymentConfigPlugin struct {
}

func NewKubeDeploymentConfigPlugin() plugin_registry.DeploymentConfigPlugin {
	return new(KubeDeploymentConfigPlugin)
}

func (p *KubeDeploymentConfigPlugin) Sign(dep map[string]interface{}, privKey *rsa.PrivateKey, ctx plugin_registry.PluginContext) (bool, string, string, error) {

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if owned, err := p.Validate(nil, dep); !owned || err != nil {
		return owned, "", "", err
	}

	// Grab the kube operator file from the deployment config. The file might be relative to the
	// service definition file.
	operatorFilePath := dep["operatorYamlArchive"].(string)
	if operatorFilePath = filepath.Clean(operatorFilePath); operatorFilePath == "." {
		return true, "", "", errors.New(msgPrinter.Sprintf("cleaned %v resulted in an empty string.", dep["operatorYamlArchive"].(string)))
	}

	if currentDir, ok := (ctx.Get("currentDir")).(string); !ok {
		return true, "", "", errors.New(msgPrinter.Sprintf("plugin context must include 'currentDir' as the current directory of the service definition file"))
	} else if !filepath.IsAbs(operatorFilePath) {
		operatorFilePath = filepath.Join(currentDir, operatorFilePath)
	}

	// Get the base 64 encoding of the kube operator, and put it into the deployment config.
	if b64, err := ConvertFileToB64String(operatorFilePath); err != nil {
		return true, "", "", errors.New(msgPrinter.Sprintf("unable to read kube operator %v, error %v", dep["operatorYamlArchive"], err))
	} else {
		dep["operatorYamlArchive"] = b64
	}

	// Stringify and sign the deployment string.
	deployment, err := json.Marshal(dep)
	if err != nil {
		return true, "", "", errors.New(msgPrinter.Sprintf("failed to marshal %v deployment string %v, error %v", KUBE_DEPLOYMENT_CONFIG_TYPE, dep, err))
	}
	depStr := string(deployment)

	hasher := sha256.New()
	_, err = hasher.Write(deployment)
	if err != nil {
		return true, "", "", err
	}
	sig, err := sign.Sha256HashOfInput(privKey, hasher)
	
	if err != nil {
		return true, "", "", errors.New(msgPrinter.Sprintf("problem signing %v deployment string: %v", KUBE_DEPLOYMENT_CONFIG_TYPE, err))
	}

	return true, depStr, sig, nil
}

func (p *KubeDeploymentConfigPlugin) GetContainerImages(dep interface{}) (bool, []string, error) {
	return false, []string{}, nil
}

// Return the default config object, which is nil in this case.
func (p *KubeDeploymentConfigPlugin) DefaultConfig(imageInfo interface{}) interface{} {
	return nil
}

// Return the default cluster config object.
func (p *KubeDeploymentConfigPlugin) DefaultClusterConfig() interface{} {
	return map[string]interface{}{
		"operatorYamlArchive": "",
	}
}

func (p *KubeDeploymentConfigPlugin) Validate(dep interface{}, cdep interface{}) (bool, error) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// If there is a native deployment config, defer to that plugin.
	if dep != nil {
		return false, nil
	}

	if dc, ok := cdep.(map[string]interface{}); !ok {
		return false, nil
	} else if c, ok := dc["operatorYamlArchive"]; !ok {
		return false, nil
	} else if ca, ok := c.(string); !ok {
		return true, errors.New(msgPrinter.Sprintf("operatorYamlArchive must have a string type value, has %T", c))
	} else if len(ca) == 0 {
		return true, errors.New(msgPrinter.Sprintf("operatorYamlArchive must be non-empty strings"))
	} else {
		return true, nil
	}
}

func (p *KubeDeploymentConfigPlugin) StartTest(homeDirectory string, userInputFile string, configFiles []string, configType string, noFSS bool, userCreds string, secretsFiles map[string]string) bool {

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
	// If there is a deployment config that we dont own, then return false, we dont own this service def.
	// This allows another plugin to claim ownership of the service def and start a test.
	// Otherwise, if the cluster config is ours, then we own the service but since this plugin doesnt
	// support start and stop, terminate with a fatal error.
	if serviceDef.Deployment != nil {
		return false
	} else {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("'%v %v' not supported for services using a %v deployment configuration", dev.SERVICE_COMMAND, dev.SERVICE_START_COMMAND, KUBE_DEPLOYMENT_CONFIG_TYPE))
	}

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
	if serviceDef.Deployment != nil {
		return false
	} else if owned, err := p.Validate(serviceDef.Deployment, serviceDef.ClusterDeployment); !owned || err != nil {
		return false
	}

	cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("'%v %v' not supported for services using a %v deployment configuration", dev.SERVICE_COMMAND, dev.SERVICE_START_COMMAND, KUBE_DEPLOYMENT_CONFIG_TYPE))
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
