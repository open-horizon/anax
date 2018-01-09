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

type BXDockerRegistryAuthAttributes struct {
	Meta  *AttributeMeta `json:"meta"`
	Token string         `json:"token"`
}

func (a BXDockerRegistryAuthAttributes) String() string {
	return fmt.Sprintf("meta: %v, token: <withheld>", a.GetMeta())
}

func (a BXDockerRegistryAuthAttributes) GetMeta() *AttributeMeta {
	return a.Meta
}

func (a BXDockerRegistryAuthAttributes) GetGenericMappings() map[string]interface{} {
	var obf string

	if a.Token != "" {
		obf = "**********"
	}

	return map[string]interface{}{
		"token": obf,
	}
}

func (a BXDockerRegistryAuthAttributes) Update(other Attribute) error {
	switch other.(type) {
	case *BXDockerRegistryAuthAttributes:
		o := other.(*BXDockerRegistryAuthAttributes)
		a.GetMeta().Update(*o.GetMeta())

		a.Token = o.Token
	default:
		return fmt.Errorf("Concrete type of attribute (%T) provided to Update() is incompatible with this Attribute's type (%T)", a, other)
	}

	return nil
}
