package api

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/eventlog"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"os"
	"time"
)

// Global "static" field to remember that unconfig is in progress. We can't tell from the configstate in the node
// object because it eventually gets deleted at the end of unconfiguration.
var Unconfiguring bool

func LogDeviceEvent(db *bolt.DB, severity string, message string, event_code string, device interface{}) {
	id := ""
	org := ""
	pattern := ""
	state := ""

	if device != nil {
		switch device.(type) {
		case *HorizonDevice:
			d, _ := (device).(*HorizonDevice)
			if d.Id != nil {
				id = *d.Id
			}
			if d.Org != nil {
				org = *d.Org
			}
			if d.Pattern != nil {
				pattern = fmt.Sprintf("%v/%v", org, *d.Pattern)
			}
			if d.Config != nil {
				state = *d.Config.State
			}
		case *persistence.ExchangeDevice:
			d, _ := (device).(*persistence.ExchangeDevice)
			id = d.Id
			org = d.Org
			pattern = d.Pattern
			state = d.Config.State
		}
	}

	eventlog.LogNodeEvent(db, severity, message, event_code, id, org, pattern, state)
}

func FindHorizonDeviceForOutput(db *bolt.DB) (*HorizonDevice, error) {

	var device *HorizonDevice

	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to read node object, error %v", err))
	} else if pDevice == nil {
		device_id := os.Getenv("HZN_DEVICE_ID")
		state := persistence.CONFIGSTATE_UNCONFIGURED
		if Unconfiguring {
			state = persistence.CONFIGSTATE_UNCONFIGURING
		}
		lut := uint64(0)
		cfg := &Configstate{
			State:          &state,
			LastUpdateTime: &lut,
		}
		device = &HorizonDevice{
			Id:     &device_id,
			Config: cfg,
		}
	} else {
		device = ConvertFromPersistentHorizonDevice(pDevice)
	}

	return device, nil
}

// Given a demarshalled HorizonDevice object, validate it and save it, returning any errors.
func CreateHorizonDevice(device *HorizonDevice,
	errorhandler ErrorHandler,
	getOrg exchange.OrgHandlerWithContext,
	getPatterns exchange.PatternHandlerWithContext,
	em *events.EventStateManager,
	db *bolt.DB) (bool, *HorizonDevice, *HorizonDevice) {

	// Reject the call if the node is restarting.
	se := events.NewNodeShutdownCompleteMessage(events.UNCONFIGURE_COMPLETE, "")
	if em.ReceivedEvent(se, nil) {
		return errorhandler(NewAPIUserInputError("Node is restarting, please wait a few seconds and try again.", "node")), nil, nil
	}

	// Check for the device in the local database. If there are errors, they will be written
	// to the HTTP response.

	if pDevice, err := persistence.FindExchangeDevice(db); err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("unable to read node object, error %v", err))), nil, nil
	} else if pDevice != nil {
		return errorhandler(NewConflictError("device is already registered")), nil, nil
	} else if Unconfiguring {
		return errorhandler(NewAPIUserInputError("Node is restarting, please wait a few seconds and try again.", "node")), nil, nil
	}

	glog.V(5).Infof(apiLogString(fmt.Sprintf("Create node payload: %v", device)))

	LogDeviceEvent(db, persistence.SEVERITY_INFO, fmt.Sprintf("Start node configuration/registration for node %v.", *device.Id), persistence.EC_START_NODE_CONFIG_REG, device)

	// There is no existing device registration in the database, so proceed to verifying the input device object.
	if device.Id == nil || *device.Id == "" {
		device_id := os.Getenv("HZN_DEVICE_ID")
		if device_id == "" {
			return errorhandler(NewAPIUserInputError("Either setup HZN_DEVICE_ID environmental variable or specify device.id.", "device.id")), nil, nil
		}
		device.Id = &device_id
	}

	if bail := checkInputString(errorhandler, "device.id", device.Id); bail {
		return true, nil, nil
	}

	if bail := checkInputString(errorhandler, "device.organization", device.Org); bail {
		return true, nil, nil
	}

	// Device pattern is optional
	if device.Pattern != nil && *device.Pattern != "" {
		if bail := checkInputString(errorhandler, "device.pattern", device.Pattern); bail {
			return true, nil, nil
		}
	}

	if bail := checkInputString(errorhandler, "device.name", device.Name); bail {
		return true, nil, nil
	}

	// No need to check the token for invalid input characters, it is not computed or parsed.
	if device.Token == nil {
		return errorhandler(NewAPIUserInputError("null and must not be", "device.token")), nil, nil
	}

	// HA validation. Since the HA declaration is a boolean, there is nothing to validate for HA.

	// Verify that the input organization exists in the exchange.
	deviceId := fmt.Sprintf("%v/%v", *device.Org, *device.Id)
	if _, err := getOrg(*device.Org, deviceId, *device.Token); err != nil {
		return errorhandler(NewAPIUserInputError(fmt.Sprintf("organization %v not found in exchange, error: %v", *device.Org, err), "device.organization")), nil, nil
	}

	// Verify the pattern org if the patter is not in the same org as the device.

	// Verify that the input pattern is defined in the exchange.
	// The input pattern is in the format of <pattern org>/<pattern name>
	if device.Pattern != nil && *device.Pattern != "" {
		// get the pattern name and the pattern org name.
		pattern_org, pattern_name, pattern := persistence.GetFormatedPatternString(*device.Pattern, *device.Org)
		device.Pattern = &pattern

		// verify pattern exists
		if patternDefs, err := getPatterns(pattern_org, pattern_name, deviceId, *device.Token); err != nil {
			return errorhandler(NewAPIUserInputError(fmt.Sprintf("error searching for pattern %v in exchange, error: %v", pattern, err), "device.pattern")), nil, nil
		} else if _, ok := patternDefs[pattern]; !ok {
			return errorhandler(NewAPIUserInputError(fmt.Sprintf("pattern %v not found in exchange.", pattern), "device.pattern")), nil, nil
		}
	}

	// So far everything checks out and verifies, so save the registration to the local database.
	haDevice := false
	if device.HA != nil && *device.HA == true {
		haDevice = true
	}

	pDev, err := persistence.SaveNewExchangeDevice(db, *device.Id, *device.Token, *device.Name, haDevice, *device.Org, *device.Pattern, persistence.CONFIGSTATE_CONFIGURING)
	if err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("error persisting new device registration: %v", err))), nil, nil
	}

	glog.V(5).Infof(apiLogString(fmt.Sprintf("Create node updated: %v", pDev)))

	// Return 2 device objects, the first is the fully populated newly created device object. The second is a device
	// object suitable for output (external consumption). Specifically the token is omitted.
	exDev := ConvertFromPersistentHorizonDevice(pDev)

	return false, device, exDev
}

// Handles the PATCH verb on this resource. Only the exchange token is updateable.
func UpdateHorizonDevice(device *HorizonDevice,
	errorhandler ErrorHandler,
	db *bolt.DB) (bool, *HorizonDevice, *HorizonDevice) {

	LogDeviceEvent(db, persistence.SEVERITY_INFO, fmt.Sprintf("Start updating node %v.", *device.Id), persistence.EC_START_NODE_UPDATE, device)

	// Check for the device in the local database. If there are errors, they will be written
	// to the HTTP response.
	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Unable to read node object, error %v", err))), nil, nil
	} else if pDevice == nil {
		return errorhandler(NewNotFoundError("Exchange registration not recorded. Complete account and device registration with an exchange and then record device registration using this API.", "node")), nil, nil
	} else if !pDevice.IsState(persistence.CONFIGSTATE_CONFIGURING) {
		return errorhandler(NewBadRequestError(fmt.Sprintf("The node must be in configuring state in order to PATCH."))), nil, nil
	}

	// Verify that the input id is ok.
	if bail := checkInputString(errorhandler, "device.id", device.Id); bail {
		return true, nil, nil
	}

	// We dont compute the token so there is no need to try to check it.

	// If there is no token, that's an error
	if device.Token == nil {
		return errorhandler(NewAPIUserInputError("null and must not be", "device.token")), nil, nil
	}

	updatedDev, err := pDevice.SetExchangeDeviceToken(db, *device.Id, *device.Token)
	if err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("error persisting token update on node object: %v", err))), nil, nil
	}

	// Return 2 device objects, the first is the fully populated newly updated device object. The second is a device
	// object suitable for output (external consumption). Specifically the token is omitted.
	exDev := ConvertFromPersistentHorizonDevice(updatedDev)

	LogDeviceEvent(db, persistence.SEVERITY_INFO, fmt.Sprintf("Complete node update for %v.", *device.Id), persistence.EC_NODE_UPDATE_COMPLETE, device)

	return false, device, exDev

}

// Handles the DELETE verb on this resource.
func DeleteHorizonDevice(removeNode string,
	block string,
	em *events.EventStateManager,
	msgQueue chan events.Message,
	errorhandler ErrorHandler,
	db *bolt.DB) bool {

	LogDeviceEvent(db, persistence.SEVERITY_INFO, fmt.Sprintf("Start node unregistration."), persistence.EC_START_NODE_UNREG, nil)

	// Check for the device in the local database. If there are errors, they will be written
	// to the HTTP response.
	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		eventlog.LogDatabaseEvent(db, persistence.SEVERITY_ERROR, fmt.Sprintf("Unable to read node object from database, error %v", err), persistence.EC_DATABASE_ERROR)
		return errorhandler(NewSystemError(fmt.Sprintf("Unable to read node object, error %v", err)))
	} else if pDevice == nil {
		LogDeviceEvent(db, persistence.SEVERITY_ERROR, fmt.Sprintf("Error unregistring the node. The node is not found from the database."), persistence.EC_ERROR_NODE_UNREG, nil)
		return errorhandler(NewNotFoundError("Exchange registration not recorded. Complete account and device registration with an exchange and then record device registration using this API.", "node"))
	} else if !pDevice.IsState(persistence.CONFIGSTATE_CONFIGURED) && !pDevice.IsState(persistence.CONFIGSTATE_CONFIGURING) {
		LogDeviceEvent(db, persistence.SEVERITY_ERROR, fmt.Sprintf("Error unregistring the node. The node must be in 'configured' or 'configuring' state in order to unconfigure it."), persistence.EC_ERROR_NODE_UNREG, pDevice)
		return errorhandler(NewBadRequestError(fmt.Sprintf("The node must be in configured or configuring state in order to unconfigure it.")))
	}

	// Verify optional input
	if removeNode != "" && removeNode != "true" && removeNode != "false" {
		LogDeviceEvent(db, persistence.SEVERITY_ERROR, fmt.Sprintf("Input error for node unregistration. %v is an incorrect value for removeNode", removeNode), persistence.EC_API_USER_INPUT_ERROR, pDevice)
		return errorhandler(NewAPIUserInputError("%v is an incorrect value for removeNode", "url.removeNode"))
	}

	if block != "" && block != "true" && block != "false" {
		LogDeviceEvent(db, persistence.SEVERITY_ERROR, fmt.Sprintf("Input error for node unregistration. %v is an incorrect value for block", block), persistence.EC_API_USER_INPUT_ERROR, pDevice)
		return errorhandler(NewAPIUserInputError("%v is an incorrect value for block", "url.block"))
	}

	// Establish defaults for optional inputs
	rNode := false
	if removeNode == "true" {
		rNode = true
	}
	blocking := true
	if block == "false" {
		blocking = false
	}

	// Mark the device as "unconfigure in progress"
	_, err = pDevice.SetConfigstate(db, pDevice.Id, persistence.CONFIGSTATE_UNCONFIGURING)
	if err != nil {
		eventlog.LogDatabaseEvent(db, persistence.SEVERITY_ERROR,
			fmt.Sprintf("Error saving new node config state (unconfiguring) in the database: %v", err),
			persistence.EC_DATABASE_ERROR)
		return errorhandler(NewSystemError(fmt.Sprintf("error persisting unconfiguring on node object: %v", err)))
	}

	// Remember that unconfiguration is in progress.
	Unconfiguring = true

	// Fire the NodeShutdown event to get the node to quiesce itself.
	ns := events.NewNodeShutdownMessage(events.START_UNCONFIGURE, blocking, rNode)
	msgQueue <- ns

	// Wait (if allowed) for the ShutdownComplete event
	if blocking {
		se := events.NewNodeShutdownCompleteMessage(events.UNCONFIGURE_COMPLETE, "")
		for {
			if em.ReceivedEvent(se, nil) {
				break
			}
			glog.V(5).Infof(apiLogString(fmt.Sprintf("Waiting for node shutdown to complete")))
			time.Sleep(5 * time.Second)
		}
	}

	LogDeviceEvent(db, persistence.SEVERITY_INFO, fmt.Sprintf("Node unregistration complete for node %v.", pDevice.Id), persistence.EC_NODE_UNREG_COMPLETE, pDevice)

	// now save this timestamp in db.
	if err := persistence.SaveLastUnregistrationTime(db, uint64(time.Now().Unix())); err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("error persisting the last unregistration timestamp: %v", err)))
	}

	return false

}
