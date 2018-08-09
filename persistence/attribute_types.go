package persistence

import (
	"fmt"
)

// particular service preferences
type LocationAttributes struct {
	Meta               *AttributeMeta `json:"meta"`
	Lat                float64        `json:"lat"`
	Lon                float64        `json:"lon"`
	LocationAccuracyKM float64        `json:"location_accuracy_km"` // a fudge factor so as not to reveal exact lat lon location
	UseGps             bool           `json:"use_gps"`              // a statement of preference; does not indicate that there is a GPS device
}

func (a LocationAttributes) GetMeta() *AttributeMeta {
	return a.Meta
}

func (a LocationAttributes) GetGenericMappings() map[string]interface{} {
	return map[string]interface{}{
		"lat": a.Lat,
		"lon": a.Lon,
		"location_accuracy_km": a.LocationAccuracyKM,
		"use_gps":              a.UseGps,
	}
}

func (a LocationAttributes) Update(other Attribute) error {
	return fmt.Errorf("Update not implemented for type: %T", a)
}

func (a LocationAttributes) String() string {
	return fmt.Sprintf("Meta: %v, lat: %v, lon: %v, LocationAccuracyKM: %v, UseGps: %v", a.Meta, a.Lat, a.Lon, a.LocationAccuracyKM, a.UseGps)
}

type ArchitectureAttributes struct {
	Meta         *AttributeMeta `json:"meta"`
	Architecture string         `json:"architecture"`
}

func (a ArchitectureAttributes) GetMeta() *AttributeMeta {
	return a.Meta
}

func (a ArchitectureAttributes) GetGenericMappings() map[string]interface{} {
	return map[string]interface{}{
		"architecture": a.Architecture,
	}
}

func (a ArchitectureAttributes) Update(other Attribute) error {
	return fmt.Errorf("Update not implemented for type: %T", a)
}

func (a ArchitectureAttributes) String() string {
	return fmt.Sprintf("Meta: %v, Arch: %v", a.Meta, a.Architecture)
}

type ComputeAttributes struct {
	Meta *AttributeMeta `json:"meta"`
	CPUs int64          `json:"cpus"`
	RAM  int64          `json:"ram"`
}

func (a ComputeAttributes) GetMeta() *AttributeMeta {
	return a.Meta
}

func (a ComputeAttributes) GetGenericMappings() map[string]interface{} {
	return map[string]interface{}{
		"cpus": a.CPUs,
		"ram":  a.RAM,
	}
}

// TODO: duplicate this for the others too
func (a ComputeAttributes) Update(other Attribute) error {
	return fmt.Errorf("Update not implemented for type: %T", a)
}

func (a ComputeAttributes) String() string {
	return fmt.Sprintf("Meta: %v, CPUs: %v, RAM: %v", a.Meta, a.CPUs, a.RAM)
}

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
	return fmt.Sprintf("Meta: %v, Tokens: %v, PerTimeUnit: %v, NotificationIntervalS: %v", a.Meta, a.Tokens, a.PerTimeUnit, a.NotificationIntervalS)
}

type CounterPartyPropertyAttributes struct {
	Meta       *AttributeMeta         `json:"meta"`
	Expression map[string]interface{} `json:"expression"`
}

func (a CounterPartyPropertyAttributes) GetMeta() *AttributeMeta {
	return a.Meta
}

func (a CounterPartyPropertyAttributes) GetGenericMappings() map[string]interface{} {
	return map[string]interface{}{
		"expression": a.Expression,
	}
}

// TODO: duplicate this for the others too
func (a CounterPartyPropertyAttributes) Update(other Attribute) error {
	return fmt.Errorf("Update not implemented for type: %T", a)
}

func (a CounterPartyPropertyAttributes) String() string {
	return fmt.Sprintf("Meta: %v, Expression: %v", a.Meta, a.Expression)
}

type PropertyAttributes struct {
	Meta     *AttributeMeta         `json:"meta"`
	Mappings map[string]interface{} `json:"mappings"`
}

func (a PropertyAttributes) GetMeta() *AttributeMeta {
	return a.Meta
}

func (a PropertyAttributes) GetGenericMappings() map[string]interface{} {
	out := map[string]interface{}{}

	for k, v := range a.Mappings {
		out[k] = v
	}

	return out
}

// TODO: duplicate this for the others too
func (a PropertyAttributes) Update(other Attribute) error {
	return fmt.Errorf("Update not implemented for type: %T", a)
}

func (a PropertyAttributes) String() string {
	return fmt.Sprintf("Meta: %v, Mappings: %v", a.Meta, a.Mappings)
}

type UserInputAttributes struct {
	Meta     *AttributeMeta         `json:"meta"`
	Mappings map[string]interface{} `json:"mappings"`
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
	return fmt.Sprintf("Meta: %v, Mappings: %v", a.Meta, a.Mappings)
}

type AgreementProtocolAttributes struct {
	Meta      *AttributeMeta `json:"meta"`
	Protocols interface{}    `json:"protocols"`
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
	return fmt.Sprintf("Meta: %v, Protocols: %v", a.Meta, a.Protocols)
}

type HTTPSBasicAuthAttributes struct {
	Meta     *AttributeMeta `json:"meta"`
	Username string         `json:"username"`
	Password string         `json:"password"`
}

func (a HTTPSBasicAuthAttributes) String() string {
	return fmt.Sprintf("meta: %v, username: %v, password: <withheld>", a.GetMeta(), a.Username)
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
	User  string `json:"user"`  // The name of the user, the default is 'token'
	Token string `json:"token"` // It can be a token, a password, an api key etc.
}

type DockerRegistryAuthAttributes struct {
	Meta  *AttributeMeta `json:"meta"`
	Auths []Auth         `json:"auths"`
}

func (a DockerRegistryAuthAttributes) String() string {
	auths_show := make([]Auth, 0)
	for _, au := range a.Auths {
		auths_show = append(auths_show, Auth{User: au.User, Token: "********"})
	}

	return fmt.Sprintf("meta: %v, auths: %v", a.GetMeta(), auths_show)
}

func (a DockerRegistryAuthAttributes) GetMeta() *AttributeMeta {
	return a.Meta
}

func (a DockerRegistryAuthAttributes) GetGenericMappings() map[string]interface{} {

	auths_show := make([]Auth, 0)
	for _, au := range a.Auths {
		auths_show = append(auths_show, Auth{User: au.User, Token: "********"})
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
		if auth.User == auth_new.User && auth.Token == auth_new.Token {
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
		if auth.User == auth_in.User && auth.Token == auth_in.Token {
			a.Auths = append(a.Auths[:i], a.Auths[i+1:]...)
			break
		}
	}
}
