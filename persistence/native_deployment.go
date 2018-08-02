package persistence

import (
	"fmt"
	docker "github.com/fsouza/go-dockerclient"
)

type DeploymentConfig interface {
	ToPersistentForm() (map[string]interface{}, error)
	FromPersistentForm(pf map[string]interface{}) error
	IsNative() bool
	ToString() string
}

type NativeDeploymentConfig struct {
	Services map[string]ServiceConfig
}

func (n *NativeDeploymentConfig) ToPersistentForm() (map[string]interface{}, error) {
	ret := make(map[string]interface{}, 5)
	for k, v := range n.Services {
		ret[k] = v
	}
	return ret, nil
}

func (n *NativeDeploymentConfig) FromPersistentForm(pf map[string]interface{}) error {
	return nil
}

func (n *NativeDeploymentConfig) IsNative() bool {
	return true
}

func (n *NativeDeploymentConfig) ToString() string {
	depStr := ""
	if n != nil {
		for key, _ := range n.Services {
			depStr = depStr + key + ","
		}
	}
	return depStr
}

// the internal representation of this lib; *this is the one persisted using the persistence lib*
type ServiceConfig struct {
	Config     docker.Config     `json:"config"`
	HostConfig docker.HostConfig `json:"host_config"`
}

func ServiceConfigNames(serviceConfigs *map[string]ServiceConfig) []string {
	names := []string{}

	if serviceConfigs != nil {
		for name, _ := range *serviceConfigs {
			names = append(names, name)
		}
	}

	return names
}

func (c ServiceConfig) String() string {
	return fmt.Sprintf("Config: %v, HostConfig: %v", c.Config, c.HostConfig)
}
