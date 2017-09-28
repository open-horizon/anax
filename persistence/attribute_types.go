package persistence

import (
	"fmt"
)

// particular service preferences
type LocationAttributes struct {
	Meta               *AttributeMeta `json:"meta"`
	Lat                string         `json:"lat"`
	Lon                string         `json:"lon"`
	UserProvidedCoords bool           `json:"user_provided_coords"` // if true, the lat / lon could have been edited by the user
	UseGps             bool           `json:"use_gps"`              // a statement of preference; does not indicate that there is a GPS device
}

func (a LocationAttributes) GetMeta() *AttributeMeta {
	return a.Meta
}

func (a LocationAttributes) GetGenericMappings() map[string]interface{} {
	return map[string]interface{}{
		"lat": a.Lat,
		"lon": a.Lon,
		"user_provided_coords": a.UserProvidedCoords,
		"use_gps":              a.UseGps,
	}
}

func (a LocationAttributes) Update(other Attribute) error {
	return fmt.Errorf("Update not implemented for type: %T", a)
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

type MappedAttributes struct {
	Meta     *AttributeMeta    `json:"meta"`
	Mappings map[string]string `json:"mappings"`
}

func (a MappedAttributes) GetMeta() *AttributeMeta {
	return a.Meta
}

func (a MappedAttributes) GetGenericMappings() map[string]interface{} {
	out := map[string]interface{}{}

	for k, v := range a.Mappings {
		out[k] = v
	}

	return out
}

func (a MappedAttributes) Update(other Attribute) error {
	switch other.(type) {
	case *MappedAttributes:
		o := other.(*MappedAttributes)
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

type HTTPSBasicAuthAttributes struct {
	Meta     *AttributeMeta `json:"meta"`
	Username string         `json:"username"`
	Password string         `json:"password"`
}

func (a HTTPSBasicAuthAttributes) GetMeta() *AttributeMeta {
	return a.Meta
}

func (a HTTPSBasicAuthAttributes) GetGenericMappings() map[string]interface{} {
	return map[string]interface{}{
		"username": a.Username,
		"password": a.Password,
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
