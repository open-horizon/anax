package dev

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/cli/cliutils"
	cliexchange "github.com/open-horizon/anax/cli/exchange"
	"github.com/open-horizon/anax/cli/register"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"path"
	"path/filepath"
)

const USERINPUT_FILE = "userinput.json"

const DEFAULT_GLOBALSET_TYPE = ""

// Sort of like a constructor, it creates an in memory object except that it is created from the user input config
// file in the current project. This function assumes the caller has determined the exact location of the file.
func GetUserInputs(homeDirectory string, userInputFile string) (*register.InputFile, string, error) {

	userInputFilePath := path.Join(homeDirectory, USERINPUT_FILE)
	if userInputFile != "" {
		var err error
		if userInputFilePath, err = filepath.Abs(userInputFile); err != nil {
			return nil, "", err
		}
	}
	userInputs := new(register.InputFile)

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

// Given a userinput file, extract the configured variables based on the type of project.
func GetUserInputsVariableConfiguration(homeDirectory string, userInputFile string) ([]register.MicroWork, error) {
	if uif, _, err := GetUserInputs(homeDirectory, userInputFile); err != nil {
		return nil, err
	} else if IsMicroserviceProject(homeDirectory) {
		return uif.Microservices, nil
	} else {
		return uif.Services, nil
	}
}

// Sort of like a constructor, it creates a skeletal user input config object and writes it to the project
// in the file system.
func CreateUserInputs(directory string, workload bool, service bool, org string) error {

	// Create a skeletal user input config object with fillins/place-holders for configuration.
	res := new(register.InputFile)
	res.Global = []register.GlobalSet{
		register.GlobalSet{
			Type: "",
			Variables: map[string]interface{}{
				"attribute_variable": "some_value",
			},
		},
	}

	// Create a skeletal array with one element for variable configuration.
	mw := []register.MicroWork{
		register.MicroWork{
			Org:          org,
			Url:          "",
			VersionRange: "[0.0.0,INFINITY)",
			Variables: map[string]interface{}{
				"my_variable": "some_value",
			},
		},
	}

	if workload {
		res.Workloads = mw
	} else if service {
		res.Services = mw
	} else {
		res.Microservices = mw
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
func GlobalSetAsAttributes(global []register.GlobalSet) ([]persistence.Attribute, error) {

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
func ValidateUserInput(i *register.InputFile, directory string, originalUserInputFilePath string) error {

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

	} else if IsServiceProject(directory) {
		// Get the service definition, so that we can look at the user input variable definitions.
		sDef, wderr := GetServiceDefinition(directory, SERVICE_DEFINITION_FILE)
		if wderr != nil {
			return wderr
		}
		foundDefinitionTuple := false
		for ix, ms := range i.Services {
			// Validate the tuple identifiers. With services, there can be tuples for the service in this project as well as services
			// that are dependencies. Only the tuple for the current project's definition is validated here. The tuples for the
			// dependencies are validated in dependency validation functions.
			if ms.Url == sDef.URL {
				foundDefinitionTuple = true
				// For every variable that is set in the userinput file, make sure that variable is defined in the service definition.
				if err := validateConfiguredVariables(ms.Variables, sDef.DefinesVariable); err != nil {
					return errors.New(fmt.Sprintf("%v: services array element at index %v is %v %v", originalUserInputFilePath, ix, ms, err))
				}
				// For every variable that is defined without a default, make sure it is set.
				if err := sDef.RequiredVariablesAreSet(ms.Variables); err != nil {
					return errors.New(fmt.Sprintf("%v: %v", originalUserInputFilePath, err))
				}
			}

			if err := validateServiceTuple(ms.Org, ms.VersionRange, ms.Url); err != nil {
				return errors.New(fmt.Sprintf("%v: services array element at index %v is %v %v", originalUserInputFilePath, ix, ms, err))
			}

		}
		if !foundDefinitionTuple {
			return errors.New(fmt.Sprintf("%v: services array does not contain an element for %v.", originalUserInputFilePath, sDef.URL))
		}

	} else {
		// Validity check the microservice array.
		// Get the microservice definition, so that we can look at the user input variable definitions.
		msDef, mserr := GetMicroserviceDefinition(directory, MICROSERVICE_DEFINITION_FILE)
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
		return errors.New(fmt.Sprintf("has empty org, must be set to the name of the organization that owns the workload or microservice."))
	} else if vers == "" {
		return errors.New(fmt.Sprintf("has empty versionRange. Use [0.0.0,INFINITY) to cover all version ranges."))
	} else if url != definitionUrl {
		return errors.New(fmt.Sprintf("has incorrect url, must be set to %v.", definitionUrl))
	}
	return nil
}

func validateServiceTuple(org string, vers string, url string) error {
	if org == "" {
		return errors.New(fmt.Sprintf("has empty org, must be set to the name of the organization that owns the service."))
	} else if vers == "" {
		return errors.New(fmt.Sprintf("has empty versionRange. Use [0.0.0,INFINITY) to cover all version ranges."))
	} else if url == "" {
		return errors.New(fmt.Sprintf("has empty url. Must be set to this service's url or a dependency's url."))
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

func getConfiguredVariables(configEntries []register.MicroWork, url string) map[string]interface{} {
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

// Given a userinput file, a dependency definition and a set of configured user input variables, copy the configured variables
// into the userinput file.
func UpdateVariableConfiguration(homeDirectory string, sDef cliexchange.AbstractServiceFile, configuredVars []register.MicroWork) (*register.InputFile, error) {

	currentUIs, _, err := GetUserInputs(homeDirectory, "")
	if err != nil {
		return nil, err
	}

	// If there are any user inputs, append them to this project's user inputs. If this dependency already has
	// some configuration in this project, then no changes will be made to this project.
	if IsWorkloadProject(homeDirectory) {
		// Find the configured variables for this dependency.
		var depVarConfig register.MicroWork
		for _, depUI := range configuredVars {
			if depUI.Url == sDef.GetURL() {
				depVarConfig = depUI
				break
			}
		}

		if depVarConfig.Url != "" {
			found := false
			for _, currentUI := range currentUIs.Microservices {
				if currentUI.Url == depVarConfig.Url && currentUI.Org == depVarConfig.Org {
					found = true
					break
				}
			}
			if !found {
				currentUIs.Microservices = append(currentUIs.Microservices, depVarConfig)
			}
		}
	} else {
		// For services, copy all the variable configurations because we want to get the variable config for the
		// dependencies too.
		for _, currentCV := range configuredVars {
			found := false
			for _, currentUI := range currentUIs.Services {
				if currentUI.Url == currentCV.Url && currentUI.Org == currentCV.Org {
					found = true
					break
				}
			}
			if !found {
				currentUIs.Services = append(currentUIs.Services, currentCV)
			}
		}
	}

	return currentUIs, nil

}

func SetUserInputsVariableConfiguration(homeDirectory string, sDef cliexchange.AbstractServiceFile, configuredVars []register.MicroWork) error {

	if currentUIs, err := UpdateVariableConfiguration(homeDirectory, sDef, configuredVars); err != nil {
		return err
	} else {
		return CreateFile(homeDirectory, USERINPUT_FILE, currentUIs)
	}
}

// Remove configured variables from the userinputs file
func RemoveConfiguredVariables(homeDirectory string, theDep cliexchange.AbstractServiceFile) error {

	// Update the service definition dependencies.
	userInputs, _, err := GetUserInputs(homeDirectory, "")
	if err != nil {
		return err
	}

	if IsServiceProject(homeDirectory) {
		for ix, dep := range userInputs.Services {
			if dep.Url == theDep.GetURL() {
				userInputs.Services = append(userInputs.Services[:ix], userInputs.Services[ix+1:]...)
				// Harden the updated user inputs.
				if err := CreateFile(homeDirectory, USERINPUT_FILE, userInputs); err != nil {
					return err
				}

				cliutils.Verbose("Updated %v/%v.", homeDirectory, USERINPUT_FILE)
				return nil
			}
		}
		cliutils.Verbose("No need to update %v/%v.", homeDirectory, USERINPUT_FILE)

	} else if IsWorkloadProject(homeDirectory) {
		for ix, dep := range userInputs.Microservices {
			if dep.Url == theDep.GetURL() {
				userInputs.Microservices = append(userInputs.Microservices[:ix], userInputs.Microservices[ix+1:]...)
				// Harden the updated user inputs.
				if err := CreateFile(homeDirectory, USERINPUT_FILE, userInputs); err != nil {
					return err
				}

				cliutils.Verbose("Updated %v/%v.", homeDirectory, USERINPUT_FILE)
				return nil
			}
		}
		cliutils.Verbose("No need to update %v/%v.", homeDirectory, USERINPUT_FILE)
	}

	return nil

}
