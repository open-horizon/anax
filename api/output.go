package api

import (
	dockerclient "github.com/fsouza/go-dockerclient"
	"github.com/open-horizon/anax/persistence"
	"strconv"
)

// The output format for microservice config
type MicroserviceConfig struct {
	SensorUrl     string                  `json:"sensor_url"`     // uniquely identifying
	SensorOrg     string                  `json:"sensor_org"`     // The org that holds the ms definition
	SensorVersion string                  `json:"sensor_version"` // added for ms split. It is only used for microsevice. If it is omitted, old behavior is assumed.
	AutoUpgrade   bool                    `json:"auto_upgrade"`   // added for ms split. The default is true. If the sensor (microservice) should be automatically upgraded when new versions become available.
	ActiveUpgrade bool                    `json:"active_upgrade"` // added for ms split. The default is false. If horizon should actively terminate agreements when new versions become available (active) or wait for all the associated agreements terminated before making upgrade.
	Attributes    []persistence.Attribute `json:"attributes"`
}

type APIMicroserviceConfig struct {
	SensorUrl     string        `json:"sensor_url"`     // uniquely identifying
	SensorOrg     string        `json:"sensor_org"`     // The org that holds the ms definition
	SensorVersion string        `json:"sensor_version"` // added for ms split. It is only used for microsevice. If it is omitted, old behavior is assumed.
	AutoUpgrade   bool          `json:"auto_upgrade"`   // added for ms split. The default is true. If the sensor (microservice) should be automatically upgraded when new versions become available.
	ActiveUpgrade bool          `json:"active_upgrade"` // added for ms split. The default is false. If horizon should actively terminate agreements when new versions become available (active) or wait for all the associated agreements terminated before making upgrade.
	Attributes    []interface{} `json:"attributes"`
}

func NewMicroserviceConfig(url string, org string, version string) *MicroserviceConfig {
	return &MicroserviceConfig{
		SensorUrl:     url,
		SensorOrg:     org,
		SensorVersion: version,
	}
}

// The output format for GET microservice
type AllMicroservices struct {
	Config      []MicroserviceConfig     `json:"config"`      // the microservice configurations
	Instances   map[string][]interface{} `json:"instances"`   // the microservice instances that are running
	Definitions map[string][]interface{} `json:"definitions"` // the definitions of microservices from the exchange
}

func NewMicroserviceOutput() *AllMicroservices {
	return &AllMicroservices{
		Config:      make([]MicroserviceConfig, 0, 10),
		Instances:   make(map[string][]interface{}, 0),
		Definitions: make(map[string][]interface{}, 0),
	}
}

// The output format for microservice instances
type MicroserviceInstanceOutput struct {
	persistence.MicroserviceInstance                               // an embedded field
	Containers                       *[]dockerclient.APIContainers `json:"containers"` // the docker info for a running container
}

func NewMicroserviceInstanceOutput(mi persistence.MicroserviceInstance, containers *[]dockerclient.APIContainers) *MicroserviceInstanceOutput {
	return &MicroserviceInstanceOutput{
		MicroserviceInstance: mi,
		Containers:           containers,
	}
}

// The output format for GET workload
type AllWorkloads struct {
	Containers *[]dockerclient.APIContainers `json:"containers"` // the docker info for a running container
}

func NewWorkloadOutput() *AllWorkloads {
	return &AllWorkloads{}
}

// The output format for GET service
type AllServices struct {
	Config      []MicroserviceConfig                            `json:"config"`      // the service configurations
	Instances   map[string][]*MicroserviceInstanceOutput        `json:"instances"`   // the microservice instances that are running
	Definitions map[string][]persistence.MicroserviceDefinition `json:"definitions"` // the definitions of services from the exchange
}

func NewServiceOutput() *AllServices {
	return &AllServices{
		Config:      make([]MicroserviceConfig, 0, 10),
		Instances:   make(map[string][]*MicroserviceInstanceOutput, 0),
		Definitions: make(map[string][]persistence.MicroserviceDefinition, 0),
	}
}

// Functions and types that plug into the go sorting feature
type EstablishedAgreementsByAgreementCreationTime []persistence.EstablishedAgreement

func (s EstablishedAgreementsByAgreementCreationTime) Len() int {
	return len(s)
}

func (s EstablishedAgreementsByAgreementCreationTime) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s EstablishedAgreementsByAgreementCreationTime) Less(i, j int) bool {
	return s[i].AgreementCreationTime < s[j].AgreementCreationTime
}

type EstablishedAgreementsByAgreementTerminatedTime []persistence.EstablishedAgreement

func (s EstablishedAgreementsByAgreementTerminatedTime) Len() int {
	return len(s)
}

func (s EstablishedAgreementsByAgreementTerminatedTime) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s EstablishedAgreementsByAgreementTerminatedTime) Less(i, j int) bool {
	return s[i].AgreementTerminatedTime < s[j].AgreementTerminatedTime
}

type MicroserviceDefById []persistence.MicroserviceDefinition

func (s MicroserviceDefById) Len() int {
	return len(s)
}

func (s MicroserviceDefById) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s MicroserviceDefById) Less(i, j int) bool {
	return s[i].Id < s[j].Id
}

type MicroserviceDefByUpgradeStartTime []persistence.MicroserviceDefinition

func (s MicroserviceDefByUpgradeStartTime) Len() int {
	return len(s)
}

func (s MicroserviceDefByUpgradeStartTime) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s MicroserviceDefByUpgradeStartTime) Less(i, j int) bool {
	return s[i].UpgradeStartTime < s[j].UpgradeStartTime
}

type MicroserviceInstanceByMicroserviceDefId []*MicroserviceInstanceOutput

func (s MicroserviceInstanceByMicroserviceDefId) Len() int {
	return len(s)
}

func (s MicroserviceInstanceByMicroserviceDefId) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s MicroserviceInstanceByMicroserviceDefId) Less(i, j int) bool {
	return s[i].MicroserviceDefId < s[j].MicroserviceDefId
}

type MicroserviceInstanceByCleanupStartTime []*MicroserviceInstanceOutput

func (s MicroserviceInstanceByCleanupStartTime) Len() int {
	return len(s)
}

func (s MicroserviceInstanceByCleanupStartTime) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s MicroserviceInstanceByCleanupStartTime) Less(i, j int) bool {
	return s[i].CleanupStartTime < s[j].CleanupStartTime
}

type EventLogByRecordId []persistence.EventLog

func (s EventLogByRecordId) Len() int {
	return len(s)
}

func (s EventLogByRecordId) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s EventLogByRecordId) Less(i, j int) bool {
	if id_i, err := strconv.ParseUint(s[i].Id, 10, 64); err == nil {
		if id_j, err := strconv.ParseUint(s[j].Id, 10, 64); err == nil {
			return id_i < id_j
		}
	}
	return s[i].Id < s[j].Id
}
