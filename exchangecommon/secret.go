package exchangecommon

import (
	"fmt"
)

// a binding that maps a secret name to a vault secret name.
type VaultBinding struct {
	Value       string `json:"value"`
	VaultSecret string `json:"vaultSecret"`
}

// The secret binding that maps service secret names to vault secret names
type SecretBinding struct {
	ServiceOrgid        string         `json:"serviceOrgid"`
	ServiceUrl          string         `json:"serviceUrl"`
	ServiceArch         string         `json:"serviceArch,omitempty"`         // empty string means it applies to all arches
	ServiceVersionRange string         `json:"serviceVersionRange,omitempty"` // version range such as [0.0.0,INFINITY). empty string means it applies to all versions
	Secrets             []VaultBinding `json:"secrets"`                       // maps a service secret name to a vault secret name
}

func (w SecretBinding) String() string {
	return fmt.Sprintf("ServiceUrl: %v, ServiceOrgid: %v, ServiceArch: %v, ServiceVersionRange: %v, Secrets: %v",
		w.ServiceUrl,
		w.ServiceOrgid,
		w.ServiceArch,
		w.ServiceVersionRange,
		w.Secrets)
}
