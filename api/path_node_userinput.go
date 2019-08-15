package api

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangesync"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/semanticversion"
)

// Return an empty user input object or the object that's in the local database.
func FindNodeUserInputForOutput(db *bolt.DB) ([]policy.UserInput, error) {

	if userInput, err := persistence.FindNodeUserInput(db); err != nil {
		return nil, errors.New(fmt.Sprintf("unable to read node user input object, error %v", err))
	} else if userInput == nil {
		return []policy.UserInput{}, nil
	} else {
		return userInput, nil
	}
}

// Update the user input object in the local node database and in the exchange.
func UpdateNodeUserInput(userInput []policy.UserInput,
	errorhandler DeviceErrorHandler,
	getDevice exchange.DeviceHandler,
	patchDevice exchange.PatchDeviceHandler,
	getService exchange.ServiceHandler,
	db *bolt.DB) (bool, []policy.UserInput, []*events.NodeUserInputMessage) {

	// Check for the device in the local database. If there are errors, they will be written
	// to the HTTP response.
	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		return errorhandler(nil, NewSystemError(fmt.Sprintf("Unable to read node object, error %v", err))), nil, nil
	} else if pDevice == nil {
		return errorhandler(nil, NewNotFoundError("Exchange registration not recorded. Complete account and node registration with an exchange and then record node registration using this API's /node path.", "node")), nil, nil
	}

	// verify userinput: 1) service exist, 2) variables definied in the service, otherwise return true but give warning message 3) values have correct type
	validated := false
	for _, u := range userInput {
		validated, err = ValidateUserInput(u, getService)
		if !validated {
			return errorhandler(nil, NewAPIUserInputError(fmt.Sprintf("Unable to validate node userInput, error: %v", err), "node.userinput")), nil, nil
		} else if err != nil { // validate == true, but give back warning error message
			glog.Warningf(apiLogString(fmt.Sprintf("UPDATE node/userinput %v ", err)))
		}
	}

	if changedSvcs, err := exchangesync.UpdateNodeUserInput(pDevice, db, userInput, getDevice, patchDevice); err != nil {
		return errorhandler(pDevice, NewSystemError(fmt.Sprintf("Unable to update the node user input. %v", err))), nil, nil
	} else {
		LogDeviceEvent(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_API_NEW_NODE_UI, userInput), persistence.EC_NODE_USERINPUT_UPDATED, pDevice)

		nodeUserInputUpdated := events.NewNodeUserInputMessage(events.UPDATE_NODE_USERINPUT, changedSvcs)
		return false, userInput, []*events.NodeUserInputMessage{nodeUserInputUpdated}
	}
}

// Update a single field of the UserInput object in the local node db and in the exchange
func PatchNodeUserInput(patchObject []policy.UserInput,
	errorhandler DeviceErrorHandler,
	getDevice exchange.DeviceHandler,
	patchDevice exchange.PatchDeviceHandler,
	getService exchange.ServiceHandler,
	db *bolt.DB) (bool, []policy.UserInput, []*events.NodeUserInputMessage) {

	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		return errorhandler(nil, NewSystemError(fmt.Sprintf("Unable to read node object, error %v", err))), nil, nil
	} else if pDevice == nil {
		return errorhandler(nil, NewNotFoundError("Exchange registration not recorded. Complete account and node registration with an exchange and then record node registration using this API's /node path.", "node")), nil, nil
	}

	// verify userinput: 1) service exist, 2) variables definied in the service, otherwise return true but give warning message 3) values have correct type
	validated := false
	for _, u := range patchObject {
		validated, err = ValidateUserInput(u, getService)
		if !validated {
			return errorhandler(nil, NewAPIUserInputError(fmt.Sprintf("Unable to validate node userInput, error: %v", err), "node.userinput")), nil, nil
		} else if err != nil { // validate == true, but give back warning error message
			glog.Warningf(apiLogString(fmt.Sprintf("PATCH node/userinput %v ", err)))
		}
	}

	if err := exchangesync.PatchNodeUserInput(pDevice, db, patchObject, getDevice, patchDevice); err != nil {
		return errorhandler(pDevice, NewSystemError(fmt.Sprintf("Unable patch the user input. %v", err))), nil, nil
	} else {
		LogDeviceEvent(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_API_NEW_NODE_UI, patchObject), persistence.EC_NODE_USERINPUT_UPDATED, pDevice)

		chnagedSvcSpecs := new(persistence.ServiceSpecs)
		for _, ui := range patchObject {
			chnagedSvcSpecs.AppendServiceSpec(persistence.ServiceSpec{Url: ui.ServiceUrl, Org: ui.ServiceOrgid})

		}
		nodeUserInputUpdated := events.NewNodeUserInputMessage(events.UPDATE_NODE_USERINPUT, *chnagedSvcSpecs)
		return false, patchObject, []*events.NodeUserInputMessage{nodeUserInputUpdated}
	}
}

// Delete the node policy object.
func DeleteNodeUserInput(errorhandler DeviceErrorHandler, db *bolt.DB,
	getDevice exchange.DeviceHandler,
	patchDevice exchange.PatchDeviceHandler) (bool, []*events.NodeUserInputMessage) {

	// Check for the device in the local database. If there are errors, they will be written
	// to the HTTP response.
	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		return errorhandler(nil, NewSystemError(fmt.Sprintf("Unable to read node object, error %v", err))), nil
	} else if pDevice == nil {
		return errorhandler(nil, NewNotFoundError("Exchange registration not recorded. Complete account and node registration with an exchange and then record node registration using this API's /node path.", "node")), nil
	}

	userInput, err := persistence.FindNodeUserInput(db)
	if err != nil {
		return errorhandler(pDevice, NewSystemError(fmt.Sprintf("unable to read node user input object, error %v", err))), nil
	}
	if userInput == nil || len(userInput) == 0 {
		LogDeviceEvent(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_API_NO_NODE_UI_TO_DEL), persistence.EC_NODE_USERINPUT_UPDATED, pDevice)
		return false, []*events.NodeUserInputMessage{}
	}

	// delete the node policy from both exchange the local db
	if err := exchangesync.DeleteNodeUserInput(pDevice, db, getDevice, patchDevice); err != nil {
		return errorhandler(pDevice, NewSystemError(fmt.Sprintf("Node user input could not be deleted. %v", err))), nil
	}

	LogDeviceEvent(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_API_DELETED_ALL_NODE_UI), persistence.EC_NODE_USERINPUT_UPDATED, pDevice)

	chnagedSvcSpecs := new(persistence.ServiceSpecs)
	for _, ui := range userInput {
		chnagedSvcSpecs.AppendServiceSpec(persistence.ServiceSpec{Url: ui.ServiceUrl, Org: ui.ServiceOrgid})
	}
	nodeUserInputUpdated := events.NewNodeUserInputMessage(events.UPDATE_NODE_USERINPUT, *chnagedSvcSpecs)
	return false, []*events.NodeUserInputMessage{nodeUserInputUpdated}
}

// Validate 1) service exist; 2) variables defined in the service; 3) values have correct type
func ValidateUserInput(userInput policy.UserInput, getService exchange.ServiceHandler) (bool, error) {
	glog.V(3).Infof(apiLogString(fmt.Sprintf("Start validate userinput .... \n")))
	serviceOrg := userInput.ServiceOrgid
	serviceUrl := userInput.ServiceUrl
	serviceVersionRange := userInput.ServiceVersionRange
	serviceArch := userInput.ServiceArch

	nodeArch := cutil.ArchString()
	var errorString string
	if serviceArch == "" {
		serviceArch = nodeArch
	} else if serviceArch != nodeArch {
		errorString = fmt.Sprintf("serviceArch: %v in userinput file should match node arch: %v if serviceArch is not empty", serviceArch, nodeArch)
		return false, errors.New(errorString)
	}

	// The versionRange field is checked for valid characters by the Version_Expression_Factory, it has a very
	// specific syntax and allows a subset of normally valid characters.

	// Use a default version that allows all version if not specified.
	if &userInput.ServiceVersionRange == nil || userInput.ServiceVersionRange == "" {
		def := "0.0.0"
		serviceVersionRange = def
	}

	// Convert the version to a version expression.
	vExp, err := semanticversion.Version_Expression_Factory(serviceVersionRange)
	if err != nil {
		errorString = fmt.Sprintf("versionRange %v cannot be converted to a version expression, error %v", userInput.ServiceVersionRange, err)
		return false, errors.New(errorString)
	}

	// service exist
	var sdef *exchange.ServiceDefinition
	sdef, _, err = getService(serviceUrl, serviceOrg, vExp.Get_expression(), serviceArch)
	if sdef == nil {
		errorString = fmt.Sprintf("Service does not exist for org: %v, url: %v, version: %v, arch: %v, get error: %v \n", serviceOrg, serviceUrl, vExp.Get_expression(), serviceArch, err)
		return false, errors.New(errorString)
	} else if err != nil {
		errorString = fmt.Sprintf("Error from get exchange service: %v \n", err)
		return false, errors.New(errorString)
	}

	// compare ServiceDefinition.Userinput (array) with userInput.Inputs (array)
	serviceUserInputs := sdef.UserInputs
	policyUserInputs := userInput.Inputs

	serviceUserinputsMap := make(map[string]exchange.UserInput)
	var serviceInputName string
	var serviceInput exchange.UserInput

	for _, serviceInput = range serviceUserInputs {
		serviceInputName = serviceInput.Name
		serviceUserinputsMap[serviceInputName] = serviceInput
	}

	// the variables in userInput.Inputs are defined in the service, if not, return true with warning message, check values are correct types
	var policyInputName string
	var policyInputValue interface{}
	var ok bool
	var inputNameNotDefinedInService []string

	for _, policyInput := range policyUserInputs {
		policyInputName = policyInput.Name
		policyInputValue = policyInput.Value

		serviceInput, ok = serviceUserinputsMap[policyInputName]
		if !ok {
			// give back a warning with this errorString
			inputNameNotDefinedInService = append(inputNameNotDefinedInService, policyInputName)
		} else {
			if err := cutil.VerifyWorkloadVarTypes(policyInputValue, serviceInput.Type); err != nil {
				return false, err
			}
		}

	}

	if len(inputNameNotDefinedInService) != 0 {
		errorString = fmt.Sprintf("The following variables are not defined in userInput for service in its highest version: %v \n", inputNameNotDefinedInService)
		return true, errors.New(errorString)
	}

	return true, nil
}
