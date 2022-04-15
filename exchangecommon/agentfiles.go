package exchangecommon

import (
	"fmt"
)

const (
	AU_AGENTFILE_TYPE_SOFTWARE = "agent_software_files"
	AU_AGENTFILE_TYPE_CONFIG   = "agent_config_files"
	AU_AGENTFILE_TYPE_CERT     = "agent_cert_files"
	AU_MANIFEST_TYPE           = "agent_upgrade_manifests"
)

type ValidAgentFileTypes []string

var (
	// Right now, there are only agent upgrade types, but there may be more types in future
	// which should be added to this list
	ValidFileTypes = ValidAgentFileTypes{AU_AGENTFILE_TYPE_SOFTWARE, AU_AGENTFILE_TYPE_CONFIG, AU_AGENTFILE_TYPE_CERT}
)

func (a ValidAgentFileTypes) Contains(element string) bool {
	for _, t := range a {
		if t == element {
			return true
		}
	}
	return false
}

func (a ValidAgentFileTypes) String() string {
	str := ""
	for _, t := range a {
		str += fmt.Sprintf("%v, ", t)
	}
	return str[:len(str)-2]
}
