package helm_deployment

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cli/dev"
	"github.com/open-horizon/anax/cli/plugin_registry"
	"github.com/open-horizon/anax/helm"
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

func (p *HelmDeploymentConfigPlugin) Sign(dep map[string]interface{}, keyFilePath string, ctx plugin_registry.PluginContext) (bool, string, string, error) {

	if owned, err := p.Validate(dep); !owned || err != nil {
		return owned, "", "", err
	}

	// Grab the archive file from the deployment config. The archive file might be relative to the
	// service definition file.
	filePath := dep["chart_archive"].(string)
	if currentDir, ok := (ctx.Get("currentDir")).(string); !ok {
		return true, "", "", errors.New(fmt.Sprintf("plugin context must include 'currentDir' as the current directory of the service definition file"))
	} else if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(currentDir, filePath)
	}

	// Get the base 64 encoding of the Helm chart archive, and put it into the deployment config.
	if b64, err := helm.ConvertFileToB64String(filePath); err != nil {
		return true, "", "", errors.New(fmt.Sprintf("unable to read chart archive %v, error %v", dep["chart_archive"], err))
	} else {
		dep["chart_archive"] = b64
	}

	// Stringify and sign the deployment string.
	deployment, err := json.Marshal(dep)
	if err != nil {
		return true, "", "", errors.New(fmt.Sprintf("failed to marshal deployment string %v, error %v", dep, err))
	}
	depStr := string(deployment)

	sig, err := sign.Input(keyFilePath, deployment)
	if err != nil {
		return true, "", "", errors.New(fmt.Sprintf("problem signing deployment string with %s: %v", keyFilePath, err))
	}

	return true, depStr, sig, nil
}

// This function does not open the helm chart package contents to try to extract container images.
// This could be done in the future if necessary for this kind of deployment.
func (p *HelmDeploymentConfigPlugin) GetContainerImages(dep interface{}) (bool, []string, error) {
	owned, err := p.Validate(dep)
	return owned, []string{}, err
}

func (p *HelmDeploymentConfigPlugin) DefaultConfig() interface{} {
	return map[string]interface{}{
		"chart_archive": "",
		"release_name":  "",
	}
}

func (p *HelmDeploymentConfigPlugin) Validate(dep interface{}) (bool, error) {
	if dc, ok := dep.(map[string]interface{}); !ok {
		return false, nil
	} else if c, ok := dc["chart_archive"]; !ok {
		return false, nil
	} else if r, ok := dc["release_name"]; !ok {
		return false, nil
	} else if ca, ok := c.(string); !ok {
		return true, errors.New(fmt.Sprintf("chart_archive must have a string type value, has %T", c))
	} else if rn, ok := r.(string); !ok {
		return true, errors.New(fmt.Sprintf("release_name must have a string type value, has %T", r))
	} else if len(ca) == 0 || len(rn) == 0 {
		return true, errors.New(fmt.Sprintf("chart_archive and release_name must be non-empty strings"))
	} else {
		return true, nil
	}
}

func (p *HelmDeploymentConfigPlugin) StartTest(homeDirectory string, userInputFile string, configFiles []string, configType string) bool {

	// Run verification before trying to start anything.
	dev.ServiceValidate(homeDirectory, userInputFile, configFiles, configType)

	// Perform the common execution setup.
	dir, _, _ := dev.CommonExecutionSetup(homeDirectory, userInputFile, dev.SERVICE_COMMAND, dev.SERVICE_START_COMMAND)

	// Get the service definition, so that we can look at the user input variable definitions.
	serviceDef, sderr := dev.GetServiceDefinition(dir, dev.SERVICE_DEFINITION_FILE)
	if sderr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", dev.SERVICE_COMMAND, dev.SERVICE_START_COMMAND, sderr)
	}

	// Now that we have the service def, we can check if we own the deployment config object.
	if owned, err := p.Validate(serviceDef.Deployment); !owned || err != nil {
		return false
	}

	cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' not supported for Helm deployments", dev.SERVICE_COMMAND, dev.SERVICE_START_COMMAND)

	// For the compiler
	return true
}

func (p *HelmDeploymentConfigPlugin) StopTest(homeDirectory string) bool {

	// Perform the common execution setup.
	dir, _, _ := dev.CommonExecutionSetup(homeDirectory, "", dev.SERVICE_COMMAND, dev.SERVICE_START_COMMAND)

	// Get the service definition, so that we can look at the user input variable definitions.
	serviceDef, sderr := dev.GetServiceDefinition(dir, dev.SERVICE_DEFINITION_FILE)
	if sderr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", dev.SERVICE_COMMAND, dev.SERVICE_START_COMMAND, sderr)
	}

	// Now that we have the service def, we can check if we own the deployment config object.
	if owned, err := p.Validate(serviceDef.Deployment); !owned || err != nil {
		return false
	}

	cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' not supported for Helm deployments", dev.SERVICE_COMMAND, dev.SERVICE_START_COMMAND)

	// For the compiler
	return true
}
