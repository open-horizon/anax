package common

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
)

// These structs are used to parse the registration input file with old format.
type GlobalSet struct {
	Type         string                   `json:"type"`
	ServiceSpecs persistence.ServiceSpecs `json:"service_specs,omitempty"`
	Variables    map[string]interface{}   `json:"variables"`
}

func (g GlobalSet) String() string {
	return fmt.Sprintf("Global Array element, type: %v, service_specs: %v, variables: %v", g.Type, g.ServiceSpecs, g.Variables)
}

// This structure encapsultes both formats of registration input file.
// For the old format Services is an array of MicroWork.
// For the new format Global is empty and Services is an array of policy.UserInput.
type UserInputFile struct {
	Global   []GlobalSet                `json:"global,omitempty"`
	Services []policy.AbstractUserInput `json:"services,omitempty"`
}

func (self UserInputFile) String() string {
	return fmt.Sprintf("Global: %v, Services: %v", self.Global, self.Services)
}

// create a UserInputFile object using the new uerinput format
func NewUserInputFile(userinput []policy.UserInput) *UserInputFile {
	uif := UserInputFile{}

	uif.Services = []policy.AbstractUserInput{}

	if userinput != nil {
		for _, ui := range userinput {
			uif.Services = append(uif.Services, ui)
		}
	}

	return &uif
}

// create a UserInputFile object using the old userinput format
func NewUserInputFileFromOldFormat(userinput UserInputFile_Old) *UserInputFile {
	uif := UserInputFile{Global: userinput.Global}

	uif.Services = []policy.AbstractUserInput{}

	if userinput.Services != nil {
		for _, ui := range userinput.Services {
			uif.Services = append(uif.Services, ui)
		}
	}

	return &uif
}

// Given the json bytes from a file, this function parse the input and into a UserInputFile object.
// Caller is reponsible to open the file and read the contents.
// The input can be old user input file format or new format.
// The old format is the json reprentation of UserInputFile_Old. It contains both "global" and "sevies" attributes.
// The new format is the json reprentation of []policy.UserInput.
/*
Old userinput format:
{
	"global": [
		{
			...
		}
	],
	"services": [
		{
			"org": "myorg",
			"url": "myservice",
			"versionRange": "[1.0.0,INFINITY)",
			"variables": {
				"myvar": "myvalue"
			}
		}
	]
}

New userinput format:
[
  {
    "serviceOrgid": "myorg",
    "serviceUrl": "myservice",
    "serviceVersionRange": "[1.0.0,INFINITY)",
    "inputs": [
      {
        "name": "myvar",
        "value": "myvalue"
      }
    ]
  }
]
*/
func NewUserInputFileFromJsonBytes(jsonBytes []byte) (*UserInputFile, error) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()
	uif := UserInputFile{}

	if jsonBytes == nil {
		return &uif, nil
	}

	// try new user input format first
	policyInputFileList := []policy.UserInput{}
	if err := json.Unmarshal(jsonBytes, &policyInputFileList); err == nil {
		return NewUserInputFile(policyInputFileList), nil
	}

	// old format, it contains both global and services sections
	inputFile_Old := UserInputFile_Old{}
	err := json.Unmarshal(jsonBytes, &inputFile_Old)
	if err != nil {
		return nil, fmt.Errorf(msgPrinter.Sprintf("failed to unmarshal json input %v: %v", jsonBytes, err))
	}

	return NewUserInputFileFromOldFormat(inputFile_Old), nil
}

// return the global settings for the user input.
// the new format does not contain global section
func (self UserInputFile) GetGlobal() []GlobalSet {
	if self.Global == nil && len(self.Global) == 0 {
		return nil
	} else {
		return self.Global
	}
}

// Get services section.
func (self UserInputFile) GetServiceUserInput() []policy.AbstractUserInput {
	return self.Services
}

// Check if the global attribute is empty or not
func (self UserInputFile) IsGlobalsEmpty() bool {
	if self.Global == nil || len(self.Global) == 0 {
		return true
	} else {
		for _, g := range self.Global {
			if g.Variables != nil && len(g.Variables) != 0 {
				return false
			}
		}
	}

	return true
}

// Returns the bytes that can be saved into a file. The caller is responsible for
// file opening and closing.
// If the Global part in the userinput is empty, it will return new format unless alwaysOldFormat is set to true.
// If the Global part in the userinput is not empty, it will return old format.
func (self UserInputFile) GetOutputJsonBytes(alwaysOldFormat bool) ([]byte, error) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// get the writing format
	write_old_format := false
	if alwaysOldFormat || !self.IsGlobalsEmpty() {
		write_old_format = true
	}

	// populate the data
	var err1 error
	var jsonBytes []byte
	if write_old_format {
		data_old_format := self.GetOldFormat()
		jsonBytes, err1 = json.MarshalIndent(data_old_format, "", "    ")
	} else {
		if data_new_format, err := self.GetNewFormat(false); err != nil {
			return nil, fmt.Errorf(msgPrinter.Sprintf("Error getting new user input format. %v", err1))
		} else {
			jsonBytes, err1 = json.MarshalIndent(data_new_format, "", "    ")
		}
	}

	if err1 != nil {
		return nil, fmt.Errorf(msgPrinter.Sprintf("Failed to create json object for user input. %v", err1))
	}

	return jsonBytes, nil
}

// convert it to old format
func (self UserInputFile) GetOldFormat() *UserInputFile_Old {
	old_format := UserInputFile_Old{Global: self.Global, Services: []MicroWork{}}

	if self.Services != nil {
		for _, ui := range self.Services {
			old_ui := MicroWork{
				Org:          ui.GetServiceOrgid(),
				Url:          ui.GetServiceUrl(),
				VersionRange: ui.GetServiceVersionRange(),
				Variables:    make(map[string]interface{}, 0),
			}
			for _, name := range ui.GetInputNames() {
				val, _ := ui.GetInputValue(name)
				old_ui.Variables[name] = val
			}
			old_format.Services = append(old_format.Services, old_ui)
		}
	}

	return &old_format
}

// convert to new format, returns error if the Global is not empty.
// If force is true, no error will be returned.
func (self UserInputFile) GetNewFormat(force bool) ([]policy.UserInput, error) {
	if !self.IsGlobalsEmpty() && !force {
		return nil, fmt.Errorf("Failed to convert the user input to new format because it contains global settings: %v", self.Global)
	}

	new_format := []policy.UserInput{}
	if self.Services != nil {
		for _, ui := range self.Services {
			new_ui := policy.UserInput{
				ServiceOrgid:        ui.GetServiceOrgid(),
				ServiceUrl:          ui.GetServiceUrl(),
				ServiceArch:         ui.GetServiceArch(),
				ServiceVersionRange: ui.GetServiceVersionRange(),
				Inputs:              []policy.Input{},
			}
			for _, name := range ui.GetInputNames() {
				inpt := policy.Input{Name: name}
				inpt.Value, _ = ui.GetInputValue(name)
				new_ui.Inputs = append(new_ui.Inputs, inpt)
			}
			new_format = append(new_format, new_ui)
		}
	}
	return new_format, nil
}

// old userinput file format
type UserInputFile_Old struct {
	Global   []GlobalSet `json:"global,omitempty"`
	Services []MicroWork `json:"services,omitempty"`
}

// old format, the new format is policy.UserInput.
// Both implement policy.AbstractUserInput inerface
type MicroWork struct {
	Org          string                 `json:"org"`
	Url          string                 `json:"url"`
	VersionRange string                 `json:"versionRange,omitempty"` //optional
	Variables    map[string]interface{} `json:"variables"`
}

func (s MicroWork) String() string {
	return fmt.Sprintf("Org: %v, URL: %v, VersionRange: %v, Variables: %v", s.Org, s.Url, s.VersionRange, s.Variables)
}

// The following functions implement policy.AbstractUserInput interface
func (s MicroWork) GetServiceOrgid() string {
	return s.Org
}

func (s MicroWork) GetServiceUrl() string {
	return s.Url
}

func (s MicroWork) GetServiceArch() string {
	return ""
}

func (s MicroWork) GetServiceVersionRange() string {
	if s.VersionRange == "" {
		return "[0.0.0,INFINITY)"
	} else {
		return s.VersionRange
	}
}

func (s MicroWork) GetInputLength() int {
	if s.Variables != nil {
		return len(s.Variables)
	}
	return 0
}

func (s MicroWork) GetInputNames() []string {
	names := []string{}
	if s.Variables != nil {
		for key, _ := range s.Variables {
			names = append(names, key)
		}
	}
	return names
}

// return value nil means the given attribute name is not found
func (s MicroWork) GetInputValue(name string) (interface{}, error) {
	if s.Variables != nil {
		for key, val := range s.Variables {
			if key == name {
				return val, nil
			}
		}
	}
	return nil, fmt.Errorf("Variable %v is not in the user input.", name)
}


func (s MicroWork) GetInputMap() map[string]interface{} {
	return s.Variables
}
