package api

import (
	"fmt"
	"github.com/open-horizon/anax/persistence"
	"reflect"
	"strconv"
)

type HorizonDevice struct {
	Id                 *string `json:"id"`
	Org                *string `json:"organization"`
	Pattern            *string `json:"pattern"` // a simple name, not prefixed with the org
	Name               *string `json:"name,omitempty"`
	Token              *string `json:"token,omitempty"`
	TokenLastValidTime *uint64 `json:"token_last_valid_time,omitempty"`
	TokenValid         *bool   `json:"token_valid,omitempty"`
	HADevice           *bool   `json:"ha_device,omitempty"`
}

func (h HorizonDevice) String() string {
	cred := "not set"
	if h.Token != nil && *h.Token != "" {
		cred = "set"
	}

	return fmt.Sprintf("Id: %v, Org: %v, Name: %v, Token: [%v], TokenLastValidTime: %v, TokenValid: %v", h.Id, h.Org, h.Name, cred, h.TokenLastValidTime, h.TokenValid)
}

// This is a type conversion function but note that the token field within the persistent
// is explicitly omitted so that it's not exposed in the API.
func ConvertFromPersistentHorizonDevice(pDevice *persistence.ExchangeDevice) *HorizonDevice {
	return &HorizonDevice{
		Id:                 &pDevice.Id,
		Org:                &pDevice.Org,
		Pattern:            &pDevice.Pattern,
		Name:               &pDevice.Name,
		TokenValid:         &pDevice.TokenValid,
		TokenLastValidTime: &pDevice.TokenLastValidTime,
		HADevice:           &pDevice.HADevice,
	}
}

type Attribute struct {
	Id          *string                 `json:"id"`
	Type        *string                 `json:"type"`
	SensorUrls  *[]string               `json:"sensor_urls"`
	Label       *string                 `json:"label"`
	Publishable *bool                   `json:"publishable"`
	HostOnly    *bool                   `json:"host_only"`
	Mappings    *map[string]interface{} `json:"mappings"`
}

func (a Attribute) String() string {
	// function to make sure the nil pointers get printed without 'invalid memory address' error
	getString := func(v interface{}) string {
		if reflect.ValueOf(v).IsNil() {
			return "<nil>"
		} else {
			return fmt.Sprintf("%v", reflect.Indirect(reflect.ValueOf(v)))
		}
	}

	return fmt.Sprintf("Id: %v, Type: %v, SensorUrls: %v, Label: %v, Publishable: %v, HostOnly: %v, Mappings: %v",
		getString(a.Id), getString(a.Type), getString(a.SensorUrls), getString(a.Label), getString(a.Publishable), getString(a.HostOnly), getString(a.Mappings))
}

// uses pointers for members b/c it allows nil-checking at deserialization; !Important!: the json field names here must not change w/out changing the error messages returned from the API, they are not programmatically determined
type Service struct {
	SensorUrl     *string      `json:"sensor_url"`     // uniquely identifying
	SensorOrg     *string      `json:"sensor_org"`     // The org that holds the ms definition
	SensorName    *string      `json:"sensor_name"`    // may not be uniquely identifying
	SensorVersion *string      `json:"sensor_version"` // added for ms split. It is only used for microsevice. If it is omitted, old behavior is asumed.
	AutoUpgrade   *bool        `json:"auto_upgrade"`   // added for ms split. The default is false. If the sensor (microservice) should be automatically upgraded when new versions become available.
	ActiveUpgrade *bool        `json:"active_upgrade"` // added for ms split. The default is false. If horizon should actively terminate agreements when new versions become available (active) or wait for all the associated agreements terminated before making upgrade.
	Attributes    *[]Attribute `json:"attributes"`
}

func (s *Service) String() string {
	sURL := ""
	sOrg := ""
	sName := ""
	sVersion := ""
	auto_upgrade := ""
	active_upgrade := ""

	if s.SensorUrl != nil {
		sURL = *s.SensorUrl
	}

	if s.SensorOrg != nil {
		sOrg = *s.SensorOrg
	}

	if s.SensorName != nil {
		sName = *s.SensorName
	}

	if s.SensorVersion != nil {
		sVersion = *s.SensorVersion
	}

	if s.AutoUpgrade != nil {
		auto_upgrade = strconv.FormatBool(*s.AutoUpgrade)
	}

	if s.ActiveUpgrade != nil {
		active_upgrade = strconv.FormatBool(*s.ActiveUpgrade)
	}

	return fmt.Sprintf("SensorUrl: %v, SensorOrg: %v, SensorName: %v, SensorVersion: %v, AutoUpgrade: %v, ActiveUpgrade: %v, Attributes: %v", sURL, sOrg, sName, sVersion, auto_upgrade, active_upgrade, s.Attributes)
}

// This section is for handling the workloadConfig API input
type WorkloadConfig struct {
	WorkloadURL string                 `json:"workload_url"`
	Org         string                 `json:"organization"`
	Version     string                 `json:"workload_version"` // This is a version range
	Variables   map[string]interface{} `json:"variables"`
}

func (w WorkloadConfig) String() string {
	return fmt.Sprintf("WorkloadURL: %v, "+
		"Org: %v, "+
		"Version: %v, "+
		"Variables: %v",
		w.WorkloadURL, w.Org, w.Version, w.Variables)
}
