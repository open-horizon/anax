package dev

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
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
func CreateUserInputs(directory string, workload bool, org string) error {

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
				Org:          org,
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
				Org:          org,
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

// Convert default user inputs to environment variables in a map. The input map is modified
// by this function. If a variable is already in the input map, it is not modified.
func AddDefaultUserInputs(uis []exchange.UserInput, envmap map[string]string) {
	for _, ui := range uis {
		if ui.Name != "" && ui.DefaultValue != "" {
			if _, ok := envmap[ui.Name]; !ok {
				envmap[ui.Name] = ui.DefaultValue
			}
		}
	}
}

// Convert user input variables and values (for a workload or microservice) to environment variables and add them to an env var map.
func AddConfiguredUserInputs(configVars map[string]interface{}, envvars map[string]string) error {

	for varName, varValue := range configVars {
		if err := cutil.NativeToEnvVariableMap(envvars, varName, varValue); err != nil {
			return err
		}
	}
	return nil
}

// Convert each attribute in the global set of attributes to a persistent attribute. This enables us to reuse the validation
// logic and to reuse the logic that converts persistent attributes to environment variables.
func GlobalSetAsAttributes(global []GlobalSet) ([]persistence.Attribute, error) {

	// Establish an error handler to catch errors that occurr in the API functions.
	var passthruError error
	errorhandler := api.GetPassThroughErrorHandler(&passthruError)

	attributes := make([]persistence.Attribute, 0, 10)

	// Run through each attribute in the global set of attributes and convert them into an API attributes, as if they are
	// coming through the anax REST API.
	for _, gs := range global {
		attr := api.NewAttribute("", []string{}, "Global variables", false, false, map[string]interface{}{})
		attr.Type = &gs.Type
		attr.SensorUrls = &gs.SensorUrls
		attr.Mappings = &gs.Variables
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
	cliutils.Verbose("Converted API Attributes: %v to persistent attributes: %v", global, attributes)

	return attributes, nil
}

// Validate that the userinputs file is complete and coherent with the rest of the definitions in the project.
// If the file is not valid the reason will be returned in the error.
func (i *InputFile) Validate(directory string, originalUserInputFilePath string) error {

	// 1. type is non-empty and one of the valid types
	// 2. workloads/microservices - variables refer to valid variable definitions.

	for _, gs := range i.Global {
		if gs.Type == DEFAULT_GLOBALSET_TYPE {
			return errors.New(fmt.Sprintf("%v: global array element (%v) has an empty type, must be one of the supported attribute types. See the Horizon agent /attribute API.", originalUserInputFilePath, gs))
		}
	}

	// Validity check the attributes by running them through the converter.
	_, err := GlobalSetAsAttributes(i.Global)
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
			// Validate the tuple identifiers.
			if err := validateTuple(wl.Org, wl.VersionRange, wl.Url, workloadDef.WorkloadURL); err != nil {
				return errors.New(fmt.Sprintf("%v: workloads array element at index %v is %v %v", originalUserInputFilePath, ix, wl, err))
			}
			// For every variable that is set in the userinput file, make sure that variable is defined in the workload definition.
			if err := validateConfiguredVariables(wl.Variables, workloadDef.DefinesVariable); err != nil {
				return errors.New(fmt.Sprintf("%v: workloads array element at index %v is %v %v", originalUserInputFilePath, ix, wl, err))
			}
			// For every variable that is defined without a default, make sure it is set.
			if err := workloadDef.RequiredVariablesAreSet(wl.Variables); err != nil {
				return errors.New(fmt.Sprintf("%v: %v", originalUserInputFilePath, err))
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
			// Validate the tuple identifiers.
			if err := validateTuple(ms.Org, ms.VersionRange, ms.Url, msDef.SpecRef); err != nil {
				return errors.New(fmt.Sprintf("%v: microservices array element at index %v is %v %v", originalUserInputFilePath, ix, ms, err))
			}
			// For every variable that is set in the userinput file, make sure that variable is defined in the microservice definition.
			if err := validateConfiguredVariables(ms.Variables, msDef.DefinesVariable); err != nil {
				return errors.New(fmt.Sprintf("%v: microservices array element at index %v is %v %v", originalUserInputFilePath, ix, ms, err))
			}
			// For every variable that is defined without a default, make sure it is set.
			if err := msDef.RequiredVariablesAreSet(ms.Variables); err != nil {
				return errors.New(fmt.Sprintf("%v: %v", originalUserInputFilePath, err))
			}
		}

	}
	return nil

}

func validateTuple(org string, vers string, url string, definitionUrl string) error {
	if org == "" {
		return errors.New(fmt.Sprintf("has empty Org, must be set to the name of the organization that owns the microservice."))
	} else if vers == "" {
		return errors.New(fmt.Sprintf("has empty VersionRange. Use [0.0.0,INFINITY) to cover all version ranges."))
	} else if url != definitionUrl {
		return errors.New(fmt.Sprintf("has incorrect Url, must be set to %v.", definitionUrl))
	}
	return nil
}

func validateConfiguredVariables(variables map[string]interface{}, definesVar func(varName string) string) error {
	for varName, varValue := range variables {
		if expectedType := definesVar(varName); expectedType != "" {
			if err := cutil.VerifyWorkloadVarTypes(varValue, expectedType); err != nil {
				return errors.New(fmt.Sprintf("sets variable %v using a value of %v.", varName, err))
			}
		} else {
			return errors.New(fmt.Sprintf("sets variable %v of type %T that is not defined.", varName, varValue))
		}
	}
	return nil
}

func getConfiguredVariables(configEntries []MicroWork, url string) map[string]interface{} {
	// Get the variables intended to configure this dependency from this project's userinput file.
	var configVars map[string]interface{}

	// Run through the list looking for the element that matches the input URL.
	for _, ce := range configEntries {
		if ce.Url == url {
			configVars = ce.Variables
			break
		}
	}
	return configVars
}
