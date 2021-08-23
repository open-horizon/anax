package exchangecommon

import (
	"fmt"
)

// service types, they are node defined in the exchange.
// they are derived from the Deployment and ClusterDeployment attributes of a service definition.
const SERVICE_TYPE_DEVICE = "device"
const SERVICE_TYPE_CLUSTER = "cluster"
const SERVICE_TYPE_BOTH = "both"

// dependent service sharing mode
const SERVICE_SHARING_MODE_EXCLUSIVE = "exclusive"
const SERVICE_SHARING_MODE_SINGLE = "single" // deprecated, use singleton instead. but leave it here for backward compatibility
const SERVICE_SHARING_MODE_SINGLETON = "singleton"
const SERVICE_SHARING_MODE_MULTIPLE = "multiple"

// Types and functions used to work with the exchange's service objects.

// This type is a tuple used to refer to a specific service that is a dependency for the referencing service.
type ServiceDependency struct {
	URL          string `json:"url"`
	Org          string `json:"org"`
	Version      string `json:"version,omitempty"`
	VersionRange string `json:"versionRange"`
	Arch         string `json:"arch"`
}

func (sd ServiceDependency) String() string {
	return fmt.Sprintf("{URL: %v, Org: %v, Version: %v, VersionRange: %v, Arch: %v}", sd.URL, sd.Org, sd.Version, sd.VersionRange, sd.Arch)
}

func (sd ServiceDependency) GetVersionRange() string {
	if sd.VersionRange != "" {
		return sd.VersionRange
	} else if sd.Version != "" {
		return sd.Version
	} else {
		return "[0.0.0,INFINITY)"
	}
}

func NewServiceDependency(url string, org string, version string, arch string) *ServiceDependency {
	return &ServiceDependency{
		URL:          url,
		Org:          org,
		Version:      version,
		VersionRange: version,
		Arch:         arch,
	}
}

// UserInput This type is used to describe a configuration variable that the node owner/user has to set before the service is able to execute on the edge node.
// swagger:model
type UserInput struct {
	Name         string `json:"name"`
	Label        string `json:"label"`
	Type         string `json:"type"` // Valid values are "string", "int", "float", "boolean", "list of strings"
	DefaultValue string `json:"defaultValue"`
}

func (ui UserInput) String() string {
	return fmt.Sprintf("{Name: %v, :Label: %v, Type: %v, DefaultValue: %v}", ui.Name, ui.Label, ui.Type, ui.DefaultValue)
}

func NewUserInput(name string, label string, stype string, default_value string) *UserInput {
	return &UserInput{
		Name:         name,
		Label:        label,
		Type:         stype,
		DefaultValue: default_value,
	}
}
