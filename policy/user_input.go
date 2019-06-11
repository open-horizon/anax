package policy

import (
	"fmt"
	"github.com/open-horizon/anax/semanticversion"
)

type Input struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
	Type  string      `json:"type,omitempty"`
}

func (s Input) String() string {
	return fmt.Sprintf("Name: %v, "+
		"Value: %v",
		s.Name, s.Value)
}

type UserInput struct {
	ServiceOrgid        string  `json:"serviceOrgid"`
	ServiceUrl          string  `json:"serviceUrl"`
	ServiceArch         string  `json:"retry_durations,omitempty"`     // empty string means it applies to all arches
	ServiceVersionRange string  `json:"serviceVersionRange,omitempty"` // version range such as [0.0.0,INFINITY). empty string means it applies to all versions
	Inputs              []Input `json:"inputs"`
}

func (s UserInput) String() string {
	return fmt.Sprintf("ServiceOrgid: %v, "+
		"ServiceUrl: %v, "+
		"ServiceArch: %v, "+
		"ServiceVersionRange: %v, "+
		"Inputs: %v",
		s.ServiceOrgid, s.ServiceUrl, s.ServiceArch, s.ServiceVersionRange, s.Inputs)
}

// ui1 is the default, ui2 is the input form the user. ui2 overwrites ui1.
func MergeUserInput(ui1, ui2 UserInput, checkService bool) (*UserInput, error) {
	if checkService {
		if ui1.ServiceOrgid != ui2.ServiceOrgid || ui1.ServiceUrl != ui2.ServiceUrl {
			return nil, fmt.Errorf("The two user input structure are for different services: %v/%v %v/%v", ui1.ServiceOrgid, ui1.ServiceUrl, ui2.ServiceOrgid, ui2.ServiceUrl)
		}

		if !(ui1.ServiceArch == ui2.ServiceArch || ui1.ServiceArch == "" || ui2.ServiceArch == "") {
			return nil, fmt.Errorf("The two user input structure are for different service arches: %v %v", ui1.ServiceArch, ui2.ServiceArch)
		}

		// we will not check the version for now.
	}

	// make a copy of the first one
	output_ui := UserInput(ui1)

	// overwrite with the second
	for _, u2 := range ui2.Inputs {
		found := false
		for i, o := range output_ui.Inputs {
			// replace with the values from ui2 if same variable exists
			if o.Name == u2.Name && o.Value != u2.Value {
				output_ui.Inputs[i] = Input(u2)
				found = true
				break
			}
		}
		if !found {
			output_ui.Inputs = append(output_ui.Inputs, Input(u2))
		}
	}
	return &output_ui, nil
}

// Get the user input that fits this given service spec
func FindUserInput(svcName, svcOrg, svcVersion, svcArch string, userInput []UserInput) (*UserInput, error) {
	for _, u1 := range userInput {
		if u1.ServiceOrgid == svcOrg && u1.ServiceUrl == svcName && (u1.ServiceArch == svcArch || u1.ServiceArch == "") {

			if u1.ServiceVersionRange == "" {
				u1.ServiceVersionRange = "[0.0.1, INFINITY)"
			}
			if vExp, err := semanticversion.Version_Expression_Factory(u1.ServiceVersionRange); err != nil {
				return nil, fmt.Errorf("Wrong version string %v specified in user input for service %v/%v %v %v, error %v", u1.ServiceVersionRange, svcOrg, svcName, svcVersion, svcArch, err)
			} else if inRange, err := vExp.Is_within_range(svcVersion); err != nil {
				return nil, fmt.Errorf("Error checking version range %v in user input for service %v/%v %v %v . %v", vExp, svcOrg, svcName, svcVersion, svcArch, err)
			} else if !inRange {
				return nil, fmt.Errorf("Version range %v in user input for service %v/%v %v %v does not match service version.", u1.ServiceVersionRange, svcOrg, svcName, svcVersion, svcArch)
			}

			u_tmp := UserInput(u1)
			return &u_tmp, nil
		}
	}

	return nil, nil
}
