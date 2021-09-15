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

// Compare 2 bound secrets to ensure they are the same.
func (w BoundSecret) IsSame(other BoundSecret) bool {
	if len(w) != len(other) {
		return false
	}

	for thisSecretName, thisBoundSecretName := range w {
		if otherBoundSecretName, ok := other[thisSecretName]; !ok {
			return false
		} else if otherBoundSecretName != thisBoundSecretName {
			return false
		}
	}

	return true
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

// Compare 2 secret bindings to ensure they are the same.
func (w SecretBinding) IsSame(other SecretBinding) bool {
	if w.ServiceOrgid != other.ServiceOrgid {
		return false
	}
	if w.ServiceUrl != other.ServiceUrl {
		return false
	}
	if w.ServiceVersionRange != other.ServiceVersionRange {
		return false
	}
	if w.ServiceArch != "" && other.ServiceArch != "" && w.ServiceArch != other.ServiceArch {
		return false
	}
	return SecretArrayIsSame(w.Secrets, other.Secrets)
}

// Compare 2 secret binding arrays to make sure they are the same.
func SecretBindingIsSame(this []SecretBinding, other []SecretBinding) bool {
	if len(this) != len(other) {
		return false
	}

	if len(this) > 0 {
		for _, thisSB := range this {
			found := false
			for _, otherSB := range other {
				if thisSB.IsSame(otherSB) {
					found = true
				}
			}
			if !found {
				return false
			}
		}
	}

	return true
}

// Compare 2 arrays of bound secrets to make sure they are the same.
func SecretArrayIsSame(this []BoundSecret, other []BoundSecret) bool {
	if len(this) != len(other) {
		return false
	}

	if len(this) > 0 {
		for _, thisBS := range this {
			found := false
			for _, otherBS := range other {
				if thisBS.IsSame(otherBS) {
					found = true
				}
			}
			if !found {
				return false
			}
		}
	}

	return true
}
