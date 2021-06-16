package exchangecommon

import (
	"fmt"
)

// a binding that maps a secret name to a secret manager secret name.
type BoundSecret map[string]string

// return both service secret name and secret manager secret name
func (w BoundSecret) GetBinding() (string, string) {
	for k, v := range w {
		return k, v
	}
	return "", ""
}

// Make a deep copy of the binding and return it.
func (w BoundSecret) MakeCopy() (out BoundSecret) {
	out = make(BoundSecret)
	for k, v := range w {
		out[k] = v
	}
	return
}

// The secret binding that maps service secret names to secret manager secret names
type SecretBinding struct {
	ServiceOrgid        string        `json:"serviceOrgid"`
	ServiceUrl          string        `json:"serviceUrl"`
	ServiceArch         string        `json:"serviceArch,omitempty"`         // empty string means it applies to all arches
	ServiceVersionRange string        `json:"serviceVersionRange,omitempty"` // version range such as [0.0.0,INFINITY). empty string means it applies to all versions
	Secrets             []BoundSecret `json:"secrets"`                       // maps a service secret name to a secret manager secret name
}

func (w SecretBinding) String() string {
	return fmt.Sprintf("ServiceUrl: %v, ServiceOrgid: %v, ServiceArch: %v, ServiceVersionRange: %v, Secrets: %v",
		w.ServiceUrl,
		w.ServiceOrgid,
		w.ServiceArch,
		w.ServiceVersionRange,
		w.Secrets)
}

// Make a deep copy of the binding and return it.
func (w SecretBinding) MakeCopy() (out SecretBinding) {

	out = w
	out.Secrets = make([]BoundSecret, 0)
	for _, binding := range w.Secrets {
		nb := make(BoundSecret)
		for k, v := range binding {
			nb[k] = v
		}
		out.Secrets = append(out.Secrets, nb)
	}

	return
}
