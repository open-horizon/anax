package plugin_registry

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"github.com/open-horizon/anax/i18n"
)

// Each deployment config plugin implements this interface.
type DeploymentConfigPlugin interface {
	Sign(dep map[string]interface{}, keyFilePath *rsa.PrivateKey, ctx PluginContext) (bool, string, string, error)
	GetContainerImages(dep interface{}) (bool, []string, error)
	DefaultConfig(imageInfo interface{}) interface{}
	DefaultClusterConfig() interface{}
	Validate(dep interface{}, cdep interface{}) (bool, error)
	StartTest(homeDirectory string, userInputFile string, configFiles []string, configType string, noFSS bool, userCreds string, secretsFiles map[string]string) bool
	StopTest(homeDirectory string) bool
}

// Global deployment config registry.
type DeploymentConfigRegistry map[string]DeploymentConfigPlugin

var DeploymentConfigPlugins = DeploymentConfigRegistry{}

// Plugin instances call this function to register themselves in the global registry.
func Register(name string, p DeploymentConfigPlugin) {
	DeploymentConfigPlugins[name] = p
}

// Ask each plugin to attempt to sign the deployment config. Plugins are called
// until one of them claims ownership of the deployment config. If no error is
// returned, then one of the plugins has signed the deployment config, and returns
// the deployment config as a string and the signature of the string.
func (d DeploymentConfigRegistry) SignByOne(dep map[string]interface{}, privKey *rsa.PrivateKey, ctx PluginContext) (string, string, error) {
	for _, p := range d {
		if owned, depStr, sig, err := p.Sign(dep, privKey, ctx); owned {
			return depStr, sig, err
		}
	}

	return "", "", errors.New(i18n.GetMessagePrinter().Sprintf("deployment config %v is not supported", dep))
}

// Ask each plugin to return all the images mentioned in the deployment config. Plugins are called
// until one of them claims ownership of the deployment config. If no error is
// returned, then one of the plugins has claimed ownership, and returns
// the list of container images in the deployment config.
func (d DeploymentConfigRegistry) GetContainerImages(dep interface{}) ([]string, error) {
	for _, p := range d {
		if owned, images, err := p.GetContainerImages(dep); owned {
			return images, err
		}
	}

	return []string{}, errors.New(i18n.GetMessagePrinter().Sprintf("deployment config %v is not supported", dep))
}

// Ask each plugin to attempt to validate the deployment config. Plugins are called
// until one of them claims ownership of the deployment config. If no error is
// returned, then one of the plugins has validated the deployment config.
func (d DeploymentConfigRegistry) ValidatedByOne(dep interface{}, cdep interface{}) error {
	for _, p := range d {
		if owned, err := p.Validate(dep, cdep); owned {
			return err
		}
	}

	return errors.New(i18n.GetMessagePrinter().Sprintf("deployment config %v is not supported", dep))
}

// Ask each plugin to attempt to start the project in test mode. Plugins are called
// until one of them claims ownership of the deployment config. If no error is
// returned, then one of the plugins has claimed the deployment config.
func (d DeploymentConfigRegistry) StartTest(homeDirectory string, userInputFile string, configFiles []string, configType string, noFSS bool, userCreds string, secretsFiles map[string]string) error {
	for _, p := range d {
		if owned := p.StartTest(homeDirectory, userInputFile, configFiles, configType, noFSS, userCreds, secretsFiles); owned {
			return nil
		}
	}

	return errors.New(i18n.GetMessagePrinter().Sprintf("starting test mode is not supported for this project"))
}

// Ask each plugin to attempt to stop the project in test mode. Plugins are called
// until one of them claims ownership of the deployment config. If no error is
// returned, then one of the plugins has claimed the deployment config.
func (d DeploymentConfigRegistry) StopTest(homeDirectory string) error {
	for _, p := range d {
		if owned := p.StopTest(homeDirectory); owned {
			return nil
		}
	}

	return errors.New(fmt.Sprintf("stopping test mode is not supported for this project"))
}

func (d DeploymentConfigRegistry) HasPlugin(name string) bool {
	if _, ok := d[name]; ok {
		return true
	}
	return false
}

func (d DeploymentConfigRegistry) Get(name string) DeploymentConfigPlugin {
	if val, ok := d[name]; ok {
		return val
	}
	return nil
}
