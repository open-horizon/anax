package helm_deployment

import (
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cli/dev"
	"github.com/open-horizon/anax/cli/plugin_registry"
	"github.com/open-horizon/anax/helm"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/rsapss-tool/sign"
	"path/filepath"
)

func init() {
	plugin_registry.Register("helm", NewHelmDeploymentConfigPlugin())
}

type HelmDeploymentConfigPlugin struct {
}

func NewHelmDeploymentConfigPlugin() plugin_registry.DeploymentConfigPlugin {
	return new(HelmDeploymentConfigPlugin)
}

func (p *HelmDeploymentConfigPlugin) Sign(dep map[string]interface{}, privKey *rsa.PrivateKey, ctx plugin_registry.PluginContext) (bool, string, string, error) {

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if owned, err := p.Validate(dep, nil); !owned || err != nil {
		return owned, "", "", err
	}

	// Grab the archive file from the deployment config. The archive file might be relative to the
	// service definition file.
	filePath := dep["chart_archive"].(string)
	if filePath = filepath.Clean(filePath); filePath == "." {
		return true, "", "", errors.New(msgPrinter.Sprintf("cleaned %v resulted in an empty string.", dep["chart_archive"].(string)))
	}

	if currentDir, ok := (ctx.Get("currentDir")).(string); !ok {
		return true, "", "", errors.New(msgPrinter.Sprintf("plugin context must include 'currentDir' as the current directory of the service definition file"))
	} else if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(currentDir, filePath)
	}

	// Get the base 64 encoding of the Helm chart archive, and put it into the deployment config.
	if b64, err := helm.ConvertFileToB64String(filePath); err != nil {
		return true, "", "", errors.New(msgPrinter.Sprintf("unable to read chart archive %v, error %v", dep["chart_archive"], err))
	} else {
		dep["chart_archive"] = b64
	}

	// Stringify and sign the deployment string.
	deployment, err := json.Marshal(dep)
	if err != nil {
		return true, "", "", errors.New(msgPrinter.Sprintf("failed to marshal deployment string %v, error %v", dep, err))
	}
	depStr := string(deployment)

	hasher := sha256.New()
	_, err = hasher.Write(deployment)
	if err != nil {
		return true, "", "", err
	}
	sig, err := sign.Sha256HashOfInput(privKey, hasher)

	if err != nil {
		return true, "", "", errors.New(msgPrinter.Sprintf("problem signing deployment string: %v", err))
	}

	return true, depStr, sig, nil
}

// This function does not open the helm chart package contents to try to extract container images.
// This could be done in the future if necessary for this kind of deployment.
func (p *HelmDeploymentConfigPlugin) GetContainerImages(dep interface{}) (bool, []string, error) {
	owned, err := p.Validate(dep, nil)
	return owned, []string{}, err
}

func (p *HelmDeploymentConfigPlugin) DefaultConfig(imageInfo interface{}) interface{} {
	return map[string]interface{}{
		"chart_archive": "",
		"release_name":  "",
	}
}

// Return the default cluster config object, which is nil in this case.
func (p *HelmDeploymentConfigPlugin) DefaultClusterConfig() interface{} {
	return nil
}

func (p *HelmDeploymentConfigPlugin) Validate(dep interface{}, cdep interface{}) (bool, error) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if dc, ok := dep.(map[string]interface{}); !ok {
		return false, nil
	} else if c, ok := dc["chart_archive"]; !ok {
		return false, nil
	} else if r, ok := dc["release_name"]; !ok {
		return false, nil
	} else if ca, ok := c.(string); !ok {
		return true, errors.New(msgPrinter.Sprintf("chart_archive must have a string type value, has %T", c))
	} else if rn, ok := r.(string); !ok {
		return true, errors.New(msgPrinter.Sprintf("release_name must have a string type value, has %T", r))
	} else if len(ca) == 0 || len(rn) == 0 {
		return true, errors.New(msgPrinter.Sprintf("chart_archive and release_name must be non-empty strings"))
	} else {
		return true, nil
	}
}

func (p *HelmDeploymentConfigPlugin) StartTest(homeDirectory string, userInputFile string, configFiles []string, configType string, noFSS bool, userCreds string, secretsFiles map[string]string) bool {

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
	if owned, err := p.Validate(serviceDef.Deployment, nil); !owned || err != nil {
		return false
	}

	cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("'%v %v' not supported for Helm deployments", dev.SERVICE_COMMAND, dev.SERVICE_START_COMMAND))

	// For the compiler
	return true
}

func (p *HelmDeploymentConfigPlugin) StopTest(homeDirectory string) bool {

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
	if owned, err := p.Validate(serviceDef.Deployment, nil); !owned || err != nil {
		return false
	}

	cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("'%v %v' not supported for Helm deployments", dev.SERVICE_COMMAND, dev.SERVICE_START_COMMAND))

	// For the compiler
	return true
}
