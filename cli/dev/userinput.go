package dev

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/cli/cliutils"
	cliexchange "github.com/open-horizon/anax/cli/exchange"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/persistence"
	"os"
	"path"
	"path/filepath"
)

const USERINPUT_FILE = "userinput.json"

const DEFAULT_GLOBALSET_TYPE = ""

// These structs are used to parse the user input file
type GlobalSet struct {
	Type       string                 `json:"type"`
	SensorUrls []string               `json:"sensor_urls"`
	Variables  map[string]interface{} `json:"variables"`
}

func (g GlobalSet) String() string {
	return fmt.Sprintf("Global Array element, type: %v, sensor_urls: %v, variables: %v", g.Type, g.SensorUrls, g.Variables)
}

// Use for both microservices and workloads
type MicroWork struct {
	Org          string                 `json:"org"`
	Url          string                 `json:"url"`
	VersionRange string                 `json:"versionRange"`
	Variables    map[string]interface{} `json:"variables"`
}

func (m MicroWork) String() string {
	return fmt.Sprintf("Org: %v, URL: %v, VersionRange: %v, Variables: %v", m.Org, m.Url, m.VersionRange, m.Variables)
}

type InputFile struct {
	Global        []GlobalSet `json:"global"`
	Microservices []MicroWork `json:"microservices"`
	Workloads     []MicroWork `json:"workloads"`
}

func ReadInputFile(filePath string, inputFileStruct *InputFile) {
	newBytes := cliutils.ReadJsonFile(filePath)
	err := json.Unmarshal(newBytes, inputFileStruct)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to unmarshal json input file %s: %v", filePath, err)
	}
}

// Sort of like a constructor, it creates an in memory object except that it is created from the user input config
// file in the current project. This function assumes the caller has determined the exact location of the file.
func GetUserInputs(homeDirectory string, userInputFile string) (*InputFile, string, error) {

	userInputFilePath := path.Join(homeDirectory, USERINPUT_FILE)
	if userInputFile != "" {
		var err error
		if userInputFilePath, err = filepath.Abs(userInputFile); err != nil {
			return nil, "", err
		}
	}
	userInputs := new(InputFile)

	fileBytes := cliutils.ReadJsonFile(userInputFilePath)

	// We decode this JSON file using a decoder with the UseNumber flag set so that the attribute API code we reuse for parsing
	// the GlobalSet attributes will have the right metadata.
	decoder := json.NewDecoder(bytes.NewReader(fileBytes))
	decoder.UseNumber()

	if err := decoder.Decode(userInputs); err != nil {
		return nil, "", errors.New(fmt.Sprintf("unable to demarshal %v file, error: %v", userInputFilePath, err))
	}

	return userInputs, userInputFilePath, nil

}

// Sort of like a constructor, it creates a skeletal user input config object and writes it to the project
// in the file system.
func CreateUserInputs(directory string, workload bool) error {

	// Create a skeletal user input config object with fillins/place-holders for configuration.
	res := new(InputFile)
	res.Global = []GlobalSet{
		GlobalSet{
			Type: "",
			Variables: map[string]interface{}{
				"attribute_variable": "some_value",
			},
		},
	}

	if workload {
		res.Workloads = []MicroWork{
			MicroWork{
				Org:          os.Getenv(DEVTOOL_HZN_ORG),
				Url:          "",
				VersionRange: "[0.0.0,INFINITY)",
				Variables: map[string]interface{}{
					"my_variable": "some_value",
				},
			},
		}
	} else {
		res.Microservices = []MicroWork{
			MicroWork{
				Org:          os.Getenv(DEVTOOL_HZN_ORG),
				Url:          "",
				VersionRange: "[0.0.0,INFINITY)",
				Variables: map[string]interface{}{
					"my_variable": "some_value",
				},
			},
		}
	}

	// Convert the object to JSON and write it into the project.
	return CreateFile(directory, USERINPUT_FILE, res)

}

// Check for the existence of the user input config file in the project.
func UserInputExists(directory string) (bool, error) {
	return FileExists(directory, USERINPUT_FILE)
}

// Convert user input variables and values (for a workload or microservice) to environment variables and add them to an env var map.
func (i *InputFile) AddWorkloadUserInputs(workloadDef *cliexchange.WorkloadInput, envvars map[string]string) error {
	for _, wlVars := range i.Workloads {
		// Only add vars that are configuring the current workload definition.
		if wlVars.Url == workloadDef.WorkloadURL {
			for varName, varValue := range wlVars.Variables {
				if err := cutil.NativeToEnvVariableMap(envvars, varName, varValue); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (i *InputFile) AddMicroserviceUserInputs(envvars map[string]string) error {
	for _, msVars := range i.Microservices {
		for varName, varValue := range msVars.Variables {
			if err := cutil.NativeToEnvVariableMap(envvars, varName, varValue); err != nil {
				return err
			}
		}
	}
	return nil
}

// Convert each attribute in the global set of attributes to a persistent attribute. This enables us to reuse the validation
// logic and to reuse the logic that converts persistent attributes to environment variables.
func (i *InputFile) GlobalSetAsAttributes() ([]persistence.Attribute, error) {

	// Establish an error handler to catch errors that occurr in the API functions.
	var passthruError error
	errorhandler := api.GetPassThroughErrorHandler(&passthruError)

	attributes := make([]persistence.Attribute, 0, 10)

	// Run through each attribute in the global set of attributes and convert them into an API attributes, as if they are
	// coming through the anax REST API.
	apiAttrs := make([]api.Attribute, 0, 10)
	for _, gs := range i.Global {
		attr := api.NewAttribute("", []string{}, "Global variables", false, false, map[string]interface{}{})
		attr.Type = &gs.Type
		attr.SensorUrls = &gs.SensorUrls
		attr.Mappings = &gs.Variables
		// apiAttrs = append(apiAttrs, *attr)
		cliutils.Verbose("Converted userinput attribute: %v to API attribute: %v", gs, attr)

		// Validate the attribute and convert to a persistent attribute.
		persistAttr, errorHandled, err := api.ValidateAndConvertAPIAttribute(errorhandler, false, *attr)
		if errorHandled {
			return nil, errors.New(fmt.Sprintf("%v encountered error: %v", gs, passthruError.Error()))
		} else if err != nil {
			return nil, err
		}
		attributes = append(attributes, persistAttr)

	}
	cliutils.Verbose("Converted API Attributes: %v to persistent attributes: %v", apiAttrs, attributes)

	return attributes, nil
}

// Validate that the userinputs file is complete and coherent with the rest of the definitions in the project.
// If the file is not valid the reason will be returned in the error.
func (i *InputFile) Validate(directory string, originalUserInputFilePath string) error {

	// 1. type is non-empty and one of the valid types
	// 2. workloads/microservices - variables refer to valid workloadURL from workload def.

	for _, gs := range i.Global {
		if gs.Type == DEFAULT_GLOBALSET_TYPE {
			return errors.New(fmt.Sprintf("%v: global array element (%v) has an empty type, must be one of the supported attribute types. See the Horizon agent /attribute API.", originalUserInputFilePath, gs))
		}
	}

	// Validity check the attributes by running them through the converter.
	_, err := i.GlobalSetAsAttributes()
	if err != nil {
		return errors.New(fmt.Sprintf("%v has error: %v ", USERINPUT_FILE, err))
	}

	// Validity check the workload array.
	if IsWorkloadProject(directory) {
		// Get the workload definition, so that we can look at the user input variable definitions.
		workloadDef, wderr := GetWorkloadDefinition(directory)
		if wderr != nil {
			return wderr
		}
		for ix, wl := range i.Workloads {
			if wl.Org == "" {
				return errors.New(fmt.Sprintf("%v: workload array element at index %v is %v has empty Org, must be set to the name of the organization that owns the workload.", originalUserInputFilePath, ix, wl))
			} else if wl.VersionRange == "" {
				return errors.New(fmt.Sprintf("%v: workload array element at index %v is %v has empty VersionRange. Use [0.0.0,INFINITY) to cover all version ranges.", originalUserInputFilePath, ix, wl))
			} else if workloadDef.WorkloadURL == DEFAULT_WLDEF_URL {
				return errors.New(fmt.Sprintf("%v: workload array element at index %v is %v has incorrect Url. Must set workloadUrl in workload definition.", originalUserInputFilePath, ix, wl))
			} else if wl.Url != workloadDef.WorkloadURL {
				return errors.New(fmt.Sprintf("%v: workload array element at index %v is %v has incorrect Url, must be set to %v.", originalUserInputFilePath, ix, wl, workloadDef.WorkloadURL))
			}
			// For every variable that is set in the userinput file, make sure that variable is defined in the workload definition.
			for varName, varValue := range wl.Variables {
				if expectedType := workloadDef.DefinesVariable(varName); expectedType != "" {
					if err := cutil.VerifyWorkloadVarTypes(varValue, expectedType); err != nil {
						return errors.New(fmt.Sprintf("%v: workload array element at index %v is %v sets variable %v using a value of %v.", originalUserInputFilePath, ix, wl, varName, err))
					}
				} else {
					return errors.New(fmt.Sprintf("%v: workload array element at index %v is %v sets variable %v of type %T that is not defined in the workload definition.", originalUserInputFilePath, ix, wl, varName, varValue))
				}
			}

		}

	} else {
		// Validity check the microservice array.
		// Get the microservice definition, so that we can look at the user input variable definitions.
		msDef, mserr := GetMicroserviceDefinition(directory)
		if mserr != nil {
			return mserr
		}
		for ix, ms := range i.Microservices {
			if ms.Org == "" {
				return errors.New(fmt.Sprintf("%v: microservice array element at index %v is %v has empty Org, must be set to the name of the organization that owns the microservice.", originalUserInputFilePath, ix, ms))
			} else if ms.VersionRange == "" {
				return errors.New(fmt.Sprintf("%v: microservice array element at index %v is %v has empty VersionRange. Use [0.0.0,INFINITY) to cover all version ranges.", originalUserInputFilePath, ix, ms))
			} else if msDef.SpecRef == DEFAULT_MSDEF_URL {
				return errors.New(fmt.Sprintf("%v: microservice array element at index %v is %v has incorrect Url. Must set specRef in microservice definition.", originalUserInputFilePath, ix, ms))
			} else if ms.Url != msDef.SpecRef {
				return errors.New(fmt.Sprintf("%v: microservice array element at index %v is %v has incorrect Url, must be set to %v.", originalUserInputFilePath, ix, ms, msDef.SpecRef))
			}
			// For every variable that is set in the userinput file, make sure that variable is defined in the microservice definition.
			for varName, varValue := range ms.Variables {
				if expectedType := msDef.DefinesVariable(varName); expectedType != "" {
					if err := cutil.VerifyWorkloadVarTypes(varValue, expectedType); err != nil {
						return errors.New(fmt.Sprintf("%v: microservice array element at index %v is %v sets variable %v using a value of %v.", originalUserInputFilePath, ix, ms, varName, err))
					}
				} else {
					return errors.New(fmt.Sprintf("%v: microservice array element at index %v is %v sets variable %v of type %T that is not defined in the microservice definition.", originalUserInputFilePath, ix, ms, varName, varValue))
				}
			}
		}

	}
	return nil

}
