package policy

import (
	"fmt"
)

type UserInput []ServiceUserInput

type Input struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

func (s Input) String() string {
	return fmt.Sprintf("Name: %v, "+
		"Value: %v",
		s.Name, s.Value)
}

type ServiceUserInput struct {
	ServiceOrgid        string  `json:"serviceOrgid"`
	ServiceUrl          string  `json:"serviceUrl"`
	ServiceArch         string  `json:"retry_durations,omitempty"`     // empty string means it applies to all arches
	ServiceVersionRange string  `json:"serviceVersionRange,omitempty"` // version range such as [0.0.0,INFINITY). empty string means it applies to all versions
	Inputs              []Input `json:"inputs"`
}

func (s ServiceUserInput) String() string {
	return fmt.Sprintf("ServiceOrgid: %v, "+
		"ServiceUrl: %v, "+
		"ServiceArch: %v, "+
		"ServiceVersionRange: %v, "+
		"Inputs: %v",
		s.ServiceOrgid, s.ServiceUrl, s.ServiceArch, s.ServiceVersionRange, s.Inputs)
}
