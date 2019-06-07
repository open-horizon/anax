package persistence

import (
	"fmt"
)

type HAAttributes struct {
	Meta     *AttributeMeta `json:"meta"`
	Partners []string       `json:"partners"`
}

func (a HAAttributes) GetMeta() *AttributeMeta {
	return a.Meta
}

func (a HAAttributes) GetGenericMappings() map[string]interface{} {
	return map[string]interface{}{
		"partnerID": a.Partners,
	}
}

// TODO: duplicate this for the others too
func (a HAAttributes) Update(other Attribute) error {
	return fmt.Errorf("Update not implemented for type: %T", a)
}

func (a HAAttributes) PartnersContains(id string) bool {
	for _, p := range a.Partners {
		if p == id {
			return true
		}
	}
	return false
}

func (a HAAttributes) String() string {
	return fmt.Sprintf("Meta: %v, Partners: %v", a.Meta, a.Partners)
}

type MeteringAttributes struct {
	Meta                  *AttributeMeta `json:"meta"`
	ServiceSpecs          *ServiceSpecs  `json:"service_specs"`
	Tokens                uint64         `json:"tokens"`
	PerTimeUnit           string         `json:"per_time_unit"`
	NotificationIntervalS int            `json:"notification_interval"`
}

func (a MeteringAttributes) GetMeta() *AttributeMeta {
	return a.Meta
}

func (a MeteringAttributes) GetGenericMappings() map[string]interface{} {
	return map[string]interface{}{
		"tokens":                a.Tokens,
		"per_time_unit":         a.PerTimeUnit,
		"notification_interval": a.NotificationIntervalS,
	}
}

// TODO: duplicate this for the others too
func (a MeteringAttributes) Update(other Attribute) error {
	return fmt.Errorf("Update not implemented for type: %T", a)
}

func (a MeteringAttributes) String() string {
	if a.ServiceSpecs == nil {
		return fmt.Sprintf("Meta: %v, ServiceSpecs: %v, Tokens: %v, PerTimeUnit: %v, NotificationIntervalS: %v", a.Meta, nil, a.Tokens, a.PerTimeUnit, a.NotificationIntervalS)
	} else {
		return fmt.Sprintf("Meta: %v, ServiceSpecs: %v, Tokens: %v, PerTimeUnit: %v, NotificationIntervalS: %v", a.Meta, *(a.ServiceSpecs), a.Tokens, a.PerTimeUnit, a.NotificationIntervalS)
	}
}

func (a MeteringAttributes) GetServiceSpecs() *ServiceSpecs {
	if a.ServiceSpecs == nil {
		a.ServiceSpecs = new(ServiceSpecs)
	}
	return a.ServiceSpecs
}

type UserInputAttributes struct {
	Meta         *AttributeMeta         `json:"meta"`
	ServiceSpecs *ServiceSpecs          `json:"service_specs"`
	Mappings     map[string]interface{} `json:"mappings"`
}

func (a UserInputAttributes) GetMeta() *AttributeMeta {
	return a.Meta
}

func (a UserInputAttributes) GetGenericMappings() map[string]interface{} {
	out := map[string]interface{}{}

	for k, v := range a.Mappings {
		out[k] = v
	}

	return out
}

func (a UserInputAttributes) Update(other Attribute) error {
	switch other.(type) {
	case *UserInputAttributes:
		o := other.(*UserInputAttributes)
		a.GetMeta().Update(*o.GetMeta())
		a.GetServiceSpecs().Update(*o.GetServiceSpecs())
		// update a's members with any in o that are specified

		for k, v := range o.Mappings {
			a.Mappings[k] = v
		}
	default:
		return fmt.Errorf("Concrete type of attribute (%T) provided to Update() is incompatible with this Attribute's type (%T)", a, other)
	}

	return nil
}

func (a UserInputAttributes) String() string {
	if a.ServiceSpecs == nil {
		return fmt.Sprintf("Meta: %v, ServiceSpecs: %v, Mappings: %v", a.Meta, nil, a.Mappings)
	} else {
		return fmt.Sprintf("Meta: %v, ServiceSpecs: %v, Mappings: %v", a.Meta, *a.ServiceSpecs, a.Mappings)
	}
}

func (a UserInputAttributes) GetServiceSpecs() *ServiceSpecs {
	if a.ServiceSpecs == nil {
		a.ServiceSpecs = new(ServiceSpecs)
	}
	return a.ServiceSpecs
}

type AgreementProtocolAttributes struct {
	Meta         *AttributeMeta `json:"meta"`
	ServiceSpecs *ServiceSpecs  `json:"service_specs"`
	Protocols    interface{}    `json:"protocols"`
}

func (a AgreementProtocolAttributes) GetMeta() *AttributeMeta {
	return a.Meta
}

func (a AgreementProtocolAttributes) GetGenericMappings() map[string]interface{} {
	return map[string]interface{}{
		"protocols": a.Protocols,
	}
}

// TODO: duplicate this for the others too
func (a AgreementProtocolAttributes) Update(other Attribute) error {
	return fmt.Errorf("Update not implemented for type: %T", a)
}

func (a AgreementProtocolAttributes) String() string {
	if a.ServiceSpecs == nil {
		return fmt.Sprintf("Meta: %v, ServiceSpecs: %v, Protocols: %v", a.Meta, nil, a.Protocols)
	} else {
		return fmt.Sprintf("Meta: %v, ServiceSpecs: %v, Protocols: %v", a.Meta, *a.ServiceSpecs, a.Protocols)
	}
}

func (a AgreementProtocolAttributes) GetServiceSpecs() *ServiceSpecs {
	if a.ServiceSpecs == nil {
		a.ServiceSpecs = new(ServiceSpecs)
	}
	return a.ServiceSpecs
}

type HTTPSBasicAuthAttributes struct {
	Meta     *AttributeMeta `json:"meta"`
	Url      string         `json:"url"`
	Username string         `json:"username"`
	Password string         `json:"password"`
}

func (a HTTPSBasicAuthAttributes) String() string {
	return fmt.Sprintf("meta: %v, url: %v, username: %v, password: <withheld>", a.GetMeta(), a.Url, a.Username)
}

func (a HTTPSBasicAuthAttributes) GetMeta() *AttributeMeta {
	return a.Meta
}

func (a HTTPSBasicAuthAttributes) GetGenericMappings() map[string]interface{} {
	var obf string

	if a.Password != "" {
		obf = "**********"
	}

	return map[string]interface{}{
		"url":      a.Url,
		"username": a.Username,
		"password": obf,
	}
}

func (a HTTPSBasicAuthAttributes) Update(other Attribute) error {
	switch other.(type) {
	case *HTTPSBasicAuthAttributes:
		o := other.(*HTTPSBasicAuthAttributes)
		a.GetMeta().Update(*o.GetMeta())

		a.Username = o.Username
		a.Password = o.Password
	default:
		return fmt.Errorf("Concrete type of attribute (%T) provided to Update() is incompatible with this Attribute's type (%T)", a, other)
	}

	return nil
}

type Auth struct {
	Registry string `json:"registry"`
	UserName string `json:"username"` // The name of the user, the default is 'token'
	Token    string `json:"token"`    // It can be a token, a password, an api key etc.
}

type DockerRegistryAuthAttributes struct {
	Meta  *AttributeMeta `json:"meta"`
	Auths []Auth         `json:"auths"`
}

func (a DockerRegistryAuthAttributes) String() string {
	auths_show := make([]Auth, 0)
	for _, au := range a.Auths {
		auths_show = append(auths_show, Auth{Registry: au.Registry, UserName: au.UserName, Token: "********"})
	}

	return fmt.Sprintf("meta: %v, auths: %v", a.GetMeta(), auths_show)
}

func (a DockerRegistryAuthAttributes) GetMeta() *AttributeMeta {
	return a.Meta
}

func (a DockerRegistryAuthAttributes) GetGenericMappings() map[string]interface{} {

	auths_show := make([]Auth, 0)
	for _, au := range a.Auths {
		auths_show = append(auths_show, Auth{UserName: au.UserName, Token: "********"})
	}

	return map[string]interface{}{
		"auths": auths_show,
	}
}

func (a DockerRegistryAuthAttributes) Update(other Attribute) error {
	switch other.(type) {
	case *DockerRegistryAuthAttributes:
		o := other.(*DockerRegistryAuthAttributes)
		a.GetMeta().Update(*o.GetMeta())

		a.Auths = o.Auths
	default:
		return fmt.Errorf("Concrete type of attribute (%T) provided to Update() is incompatible with this Attribute's type (%T)", a, other)
	}

	return nil
}

// add a new auth token to this registery
func (a DockerRegistryAuthAttributes) AddAuth(auth_new Auth) {
	found := false
	for _, auth := range a.Auths {
		if auth.UserName == auth_new.UserName && auth.Token == auth_new.Token {
			found = true
			break
		}
	}

	if !found {
		a.Auths = append(a.Auths, auth_new)
	}
}

// delete the given auth from this registery
func (a DockerRegistryAuthAttributes) DeleteAuth(auth_in Auth) {
	for i, auth := range a.Auths {
		if auth.UserName == auth_in.UserName && auth.Token == auth_in.Token {
			a.Auths = append(a.Auths[:i], a.Auths[i+1:]...)
			break
		}
	}
}
