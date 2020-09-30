package policy

import (
	"fmt"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/semanticversion"
	"reflect"
	"strings"
)

type AbstractUserInput interface {
	GetServiceOrgid() string
	GetServiceUrl() string
	GetServiceArch() string
	GetServiceVersionRange() string
	GetInputLength() int
	GetInputNames() []string
	GetInputValue(string) (interface{}, error)
	GetInputMap() map[string]interface{}
}

type Input struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

func (s Input) String() string {
	return fmt.Sprintf("Name: %v, "+
		"Value: %v",
		s.Name, s.Value)
}

func (s Input) ShortString() string {
	return fmt.Sprintf("%v: %v", s.Name, s.Value)
}

// compare two Input's
func (s Input) IsSame(input Input) bool {
	if s.Name != input.Name {
		return false
	}
	if !reflect.DeepEqual(s.Value, input.Value) {
		return false
	}
	return true
}

type UserInput struct {
	ServiceOrgid        string  `json:"serviceOrgid"`
	ServiceUrl          string  `json:"serviceUrl"`
	ServiceArch         string  `json:"serviceArch,omitempty"`         // empty string means it applies to all arches
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

func (s UserInput) ShortString() string {
	inputs := []string{}
	if s.Inputs != nil && len(s.Inputs) != 0 {
		for _, ui := range s.Inputs {
			inputs = append(inputs, ui.ShortString())
		}
	}
	return fmt.Sprintf("Service: %v/%v %v %v, "+
		"Inputs: %v",
		s.ServiceOrgid, s.ServiceUrl, s.ServiceArch, s.ServiceVersionRange, strings.Join(inputs, ","))
}

// The following functions implement AbstractUserInput interface
func (s UserInput) GetServiceOrgid() string {
	return s.ServiceOrgid
}

func (s UserInput) GetServiceUrl() string {
	return s.ServiceUrl
}

func (s UserInput) GetServiceArch() string {
	return s.ServiceArch
}

func (s UserInput) GetServiceVersionRange() string {
	if s.ServiceVersionRange == "" {
		return "[0.0.0,INFINITY)"
	} else {
		return s.ServiceVersionRange
	}
}

func (s UserInput) GetInputLength() int {
	if s.Inputs != nil {
		return len(s.Inputs)
	}
	return 0
}

func (s UserInput) GetInputNames() []string {
	names := []string{}
	if s.Inputs != nil {
		for _, ui := range s.Inputs {
			names = append(names, ui.Name)
		}
	}
	return names
}

// return value nil means the given attribute name is not found
func (s UserInput) GetInputValue(name string) (interface{}, error) {
	if s.Inputs != nil {
		for _, ui := range s.Inputs {
			if ui.Name == name {
				return ui.Value, nil
			}
		}
	}
	return nil, fmt.Errorf("Variable %v is not in the user input.", name)
}


func (s UserInput) GetInputMap() map[string]interface{} {
	inputs := make(map[string]interface{})
	if s.Inputs != nil {
		for _, ui := range s.Inputs {
			inputs[ui.Name] = ui.Value
		}
	}
	return inputs
}

// Make a copy of this object
func (s UserInput) Copy() UserInput {
	out_ui := s
	if s.Inputs == nil || len(s.Inputs) == 0 {
		out_ui.Inputs = []Input{}
	} else {
		out_ui.Inputs = make([]Input, len(s.Inputs))
		copy(out_ui.Inputs, s.Inputs)
	}

	return out_ui
}

// compare the user inputs
func (s UserInput) IsSame(userInput UserInput) bool {
	if s.ServiceOrgid != userInput.ServiceOrgid {
		return false
	}
	if s.ServiceUrl != userInput.ServiceUrl {
		return false
	}
	if s.ServiceVersionRange != userInput.ServiceVersionRange {
		return false
	}
	if s.ServiceArch != "" && userInput.ServiceArch != "" && s.ServiceArch != userInput.ServiceArch {
		return false
	}
	return InputArrayIsSame(s.Inputs, userInput.Inputs)
}

// Get the input given the name of the variable
func (s UserInput) FindInput(name string) *Input {
	if s.Inputs == nil || len(s.Inputs) == 0 {
		return nil
	}

	for _, u := range s.Inputs {
		if u.Name == name {
			return &u
		}
	}
	return nil
}

// ui1 is the default, ui2 is the input form the user. ui2 overwrites ui1.
func MergeUserInput(ui1, ui2 UserInput, checkService bool) (*UserInput, error) {
	// handle corner conditions first
	if ui2.Inputs == nil || len(ui2.Inputs) == 0 {
		output_ui := ui1.Copy()
		return &output_ui, nil
	}
	if ui1.Inputs == nil || len(ui1.Inputs) == 0 {
		output_ui := ui2.Copy()
		return &output_ui, nil
	}

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
	output_ui := ui1.Copy()

	// overwrite with the second
	for _, u2 := range ui2.Inputs {
		found := false
		for i, o := range ui1.Inputs {
			// replace with the values from ui2 if same variable exists
			if o.Name == u2.Name {
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

// If there are 2 UserInput for the same service, take the one from ui2 if deepMerge is false.
// If deepMerge is true, then merge the content from ui2 into ui1, ui2 take precedence.
func MergeUserInputArrays(ui1, ui2 []UserInput, deepMerge bool) []UserInput {
	// check cornor conditions
	if ui1 == nil || len(ui1) == 0 {
		if ui2 == nil {
			return []UserInput{}
		} else {
			output_ui := make([]UserInput, len(ui2))
			copy(output_ui, ui2)
			return output_ui
		}
	}

	if ui2 == nil || len(ui2) == 0 {
		if ui1 == nil {
			return []UserInput{}
		} else {
			output_ui := make([]UserInput, len(ui1))
			copy(output_ui, ui1)
			return output_ui
		}
	}

	// Now do the merge
	userInput := make([]UserInput, len(ui1))
	copy(userInput, ui1)
	for _, u2 := range ui2 {
		found := false
		for i1, u1 := range ui1 {
			if u1.ServiceOrgid != u2.ServiceOrgid || u1.ServiceUrl != u2.ServiceUrl {
				continue
			}
			if !(u1.ServiceArch == u2.ServiceArch || u1.ServiceArch == "" || u2.ServiceArch == "") {
				continue
			}
			found = true
			if deepMerge {
				new_u, _ := MergeUserInput(u1, u2, false)
				if new_u != nil {
					userInput[i1] = *new_u
				}
			} else {
				userInput[i1] = u2
			}
			break
		}
		if !found {
			userInput = append(userInput, u2)
		}
	}
	return userInput
}

// Get the user input that fits this given service spec
// if arch is an empty string, it means any arch.
// if service version is an empty string, it means any version is be ok.
func FindUserInput(svcName, svcOrg, svcVersion, svcArch string, userInput []UserInput) (*UserInput, int, error) {
	if userInput == nil || len(userInput) == 0 {
		return nil, -1, nil
	}

	index := -1
	for _, u1 := range userInput {
		index++

		if u1.ServiceOrgid == svcOrg && u1.ServiceUrl == svcName && (u1.ServiceArch == svcArch || u1.ServiceArch == "" || svcArch == "") {

			if svcVersion != "" {
				if u1.ServiceVersionRange == "" {
					u1.ServiceVersionRange = "[0.0.1,INFINITY)"
				}
				if vExp, err := semanticversion.Version_Expression_Factory(u1.ServiceVersionRange); err != nil {
					return nil, -1, fmt.Errorf("Wrong version string %v specified in user input for service %v/%v %v %v, error %v", u1.ServiceVersionRange, svcOrg, svcName, svcVersion, svcArch, err)
				} else if inRange, err := vExp.Is_within_range(svcVersion); err != nil {
					return nil, -1, fmt.Errorf("Error checking version range %v in user input for service %v/%v %v %v . %v", vExp, svcOrg, svcName, svcVersion, svcArch, err)
				} else if !inRange {
					continue
				}
			}

			u_tmp := UserInput(u1)
			return &u_tmp, index, nil
		}
	}

	return nil, -1, nil
}

// Gets the and update the existing settings if the name does not exist.
func UpdateSettingsWithUserInputs(userInputs []UserInput, existingUserSettings map[string]string, svcUrl string, svcOrg string) (map[string]string, error) {
	userSettings := existingUserSettings
	if userInputs != nil && len(userInputs) > 0 {
		for _, ui := range userInputs {
			if ui.Inputs != nil && len(ui.Inputs) > 0 {
				if ui.ServiceUrl == svcUrl && ui.ServiceOrgid == svcOrg {
					for _, item := range ui.Inputs {
						found := false
						for k, _ := range existingUserSettings {
							if item.Name == k {
								found = true
								break
							}
						}
						if !found {
							if err := cutil.NativeToEnvVariableMap(userSettings, item.Name, item.Value); err != nil {
								return nil, fmt.Errorf("Error converting value %v of %v to string for service %v %v. %v", item.Value, item.Name, svcUrl, svcOrg, err)
							}
						}
					}
				}
			}
		}
	}

	return userSettings, nil
}

// compare the UserInput arrays
func UserInputArrayIsSame(userInput1 []UserInput, userInput2 []UserInput) bool {
	if len(userInput1) != len(userInput2) {
		return false
	}

	if len(userInput1) > 0 {
		for _, ui1 := range userInput1 {
			found := false
			for _, ui2 := range userInput2 {
				if ui1.IsSame(ui2) {
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

// compare two Input arrays
func InputArrayIsSame(input1 []Input, input2 []Input) bool {
	if len(input1) != len(input1) {
		return false
	}

	if len(input1) > 0 {
		for _, ui1 := range input1 {
			found := false
			for _, ui2 := range input2 {
				if ui1.IsSame(ui2) {
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
