package policy

type ServiceSecret struct {
	ServiceOrgid   string            `json:"serviceOrgid"`
	ServiceUrl     string            `json:"serviceUrl"`
	ServiceArch    string            `json:"serviceArch,omitempty"`
	ServiceVersion string            `json:"serviceVersion,omitempty"`
	ServiceSecrets map[string]string `json:"serviceSecret"`
}
