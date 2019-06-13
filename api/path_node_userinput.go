package api

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangesync"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
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
	db *bolt.DB) (bool, []policy.UserInput, []*events.NodeUserInputMessage) {

	// Check for the device in the local database. If there are errors, they will be written
	// to the HTTP response.
	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		return errorhandler(nil, NewSystemError(fmt.Sprintf("Unable to read node object, error %v", err))), nil, nil
	} else if pDevice == nil {
		return errorhandler(nil, NewNotFoundError("Exchange registration not recorded. Complete account and node registration with an exchange and then record node registration using this API's /node path.", "node")), nil, nil
	}

	if changedSvcs, err := exchangesync.UpdateNodeUserInput(pDevice, db, userInput, getDevice, patchDevice); err != nil {
		return errorhandler(pDevice, NewSystemError(fmt.Sprintf("Unable to update the node user input. %v", err))), nil, nil
	} else {
		LogDeviceEvent(db, persistence.SEVERITY_INFO, fmt.Sprintf("New node user input: %v", userInput), persistence.EC_NODE_USERINPUT_UPDATED, pDevice)

		nodeUserInputUpdated := events.NewNodeUserInputMessage(events.UPDATE_NODE_USERINPUT, changedSvcs)
		return false, userInput, []*events.NodeUserInputMessage{nodeUserInputUpdated}
	}
}

// Update a single field of the UserInput object in the local node db and in the exchange
func PatchNodeUserInput(patchObject []policy.UserInput,
	errorhandler DeviceErrorHandler,
	getDevice exchange.DeviceHandler,
	patchDevice exchange.PatchDeviceHandler,
	db *bolt.DB) (bool, []policy.UserInput, []*events.NodeUserInputMessage) {

	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		return errorhandler(nil, NewSystemError(fmt.Sprintf("Unable to read node object, error %v", err))), nil, nil
	} else if pDevice == nil {
		return errorhandler(nil, NewNotFoundError("Exchange registration not recorded. Complete account and node registration with an exchange and then record node registration using this API's /node path.", "node")), nil, nil
	}

	if err := exchangesync.PatchNodeUserInput(pDevice, db, patchObject, getDevice, patchDevice); err != nil {
		return errorhandler(pDevice, NewSystemError(fmt.Sprintf("Unable patch the user input. %v", err))), nil, nil
	} else {
		LogDeviceEvent(db, persistence.SEVERITY_INFO, fmt.Sprintf("New node user input: %v", patchObject), persistence.EC_NODE_USERINPUT_UPDATED, pDevice)

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
		LogDeviceEvent(db, persistence.SEVERITY_INFO, fmt.Sprintf("No node user input to detele"), persistence.EC_NODE_USERINPUT_UPDATED, pDevice)
		return false, []*events.NodeUserInputMessage{}
	}

	// delete the node policy from both exchange the local db
	if err := exchangesync.DeleteNodeUserInput(pDevice, db, getDevice, patchDevice); err != nil {
		return errorhandler(pDevice, NewSystemError(fmt.Sprintf("Node user input could not be deleted. %v", err))), nil
	}

	LogDeviceEvent(db, persistence.SEVERITY_INFO, fmt.Sprintf("Deleted all node user input"), persistence.EC_NODE_USERINPUT_UPDATED, pDevice)

	chnagedSvcSpecs := new(persistence.ServiceSpecs)
	for _, ui := range userInput {
		chnagedSvcSpecs.AppendServiceSpec(persistence.ServiceSpec{Url: ui.ServiceUrl, Org: ui.ServiceOrgid})
	}
	nodeUserInputUpdated := events.NewNodeUserInputMessage(events.UPDATE_NODE_USERINPUT, *chnagedSvcSpecs)
	return false, []*events.NodeUserInputMessage{nodeUserInputUpdated}
}
