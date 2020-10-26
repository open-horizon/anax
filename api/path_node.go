package api

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/eventlog"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/version"
	"os"
	"time"
)

// Global "static" field to remember that unconfig is in progress. We can't tell from the configstate in the node
// object because it eventually gets deleted at the end of unconfiguration.
var Unconfiguring bool

func LogDeviceEvent(db *bolt.DB, severity string, message *persistence.MessageMeta, event_code string, device interface{}) {
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
	getExchangeVersion exchange.ExchangeVersionHandler,
	patchDeviceHandler exchange.PatchDeviceHandler,
	getDeviceHandler exchange.DeviceHandler,
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

	LogDeviceEvent(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_API_START_NODE_REG, *device.Id), persistence.EC_START_NODE_CONFIG_REG, device)

	// There is no existing device registration in the database, so proceed to verifying the input device object.
	if device.Id == nil || *device.Id == "" {
		device_id := os.Getenv("HZN_DEVICE_ID")
		if device_id == "" {
			return errorhandler(NewAPIUserInputError("Either setup HZN_DEVICE_ID environmental variable or specify device.id.", "device.id")), nil, nil
		}

		glog.V(3).Infof(apiLogString(fmt.Sprintf("using HZN_DEVICE_ID=%v as node ID.", device_id)))
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

	// the default node type is 'device'
	if device.NodeType == nil || *device.NodeType == "" {
		np := persistence.DEVICE_TYPE_DEVICE
		device.NodeType = &np
	}

	// HA validation. Since the HA declaration is a boolean, there is nothing to validate for HA.

	// make sure current exchange version meet the requirement
	deviceId := fmt.Sprintf("%v/%v", *device.Org, *device.Id)
	if exchangeVersion, err := getExchangeVersion(deviceId, *device.Token); err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Error getting exchange version. error: %v", err))), nil, nil
	} else {
		if err := version.VerifyExchangeVersion1(exchangeVersion, false); err != nil {
			return errorhandler(NewSystemError(fmt.Sprintf("Error verifiying exchange version. error: %v", err))), nil, nil
		}
	}

	// Verify that the input organization exists in the exchange.
	if _, err := getOrg(*device.Org, deviceId, *device.Token); err != nil {
		return errorhandler(NewAPIUserInputError(fmt.Sprintf("organization %v not found in exchange, error: %v", *device.Org, err), "device.organization")), nil, nil
	}

	// Verify the pattern org if the patter is not in the same org as the device.

	// Check the node on the exchange to see if there is a pattern already defined for the node.
	// Check if the node type on the exchange is the same as the given node type
	exchDevice, err1 := getDeviceHandler(deviceId, *device.Token)
	if err1 != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Error getting device %v from the exchange. %v", deviceId, err1))), nil, nil
	} else {
		// the exchange should always return a non-empty node type. But just in case it does not, 'device' is default.
		if exchDevice.NodeType == "" {
			exchDevice.NodeType = persistence.DEVICE_TYPE_DEVICE
		}
		// the device should have the same node type as the exchange node
		if *device.NodeType != exchDevice.NodeType {
			return errorhandler(NewAPIUserInputError(fmt.Sprintf("the exchange node type '%v' is different from the given node type '%v'.", exchDevice.NodeType, *device.NodeType), "device.nodeType")), nil, nil
		}

		if exchDevice != nil && exchDevice.Pattern != "" {
			_, _, exchange_pattern := persistence.GetFormatedPatternString(exchDevice.Pattern, *device.Org)

			if device.Pattern != nil && *device.Pattern != "" {
				_, _, input_pattern := persistence.GetFormatedPatternString(*device.Pattern, *device.Org)

				if input_pattern != exchange_pattern {
					// error if the pattern from the input is different from the pattern on the exchange
					return errorhandler(NewAPIUserInputError(fmt.Sprintf("There is a conflict between the node pattern %v defined in the exchange and pattern %v. Please leave the pattern field empty if you want to use the pattern defined for the node in the exchange.", exchDevice.Pattern, *device.Pattern), "device.pattern")), nil, nil
				}
			} else {
				glog.Infof(apiLogString(fmt.Sprintf("No pattern specified with the device, will use the pattern %v defined for the node in the exchange.", exchDevice.Pattern)))
			}

			// use the pattern from the exchange if there is no pattern in the input device
			device.Pattern = &exchange_pattern
		}
	}

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

	pDev, err := persistence.SaveNewExchangeDevice(db, *device.Id, *device.Token, *device.Name, *device.NodeType, haDevice, *device.Org, *device.Pattern, persistence.CONFIGSTATE_CONFIGURING)
	if err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("error persisting new device registration: %v", err))), nil, nil
	}

	glog.V(5).Infof(apiLogString(fmt.Sprintf("Create node updated: %v", pDev)))

	exDev := ConvertFromPersistentHorizonDevice(pDev)

	// update the arch for the exchange node
	pdr := exchange.PatchDeviceRequest{}
	tmpArch := cutil.ArchString()
	pdr.Arch = &tmpArch
	if err := patchDeviceHandler(deviceId, *device.Token, &pdr); err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("error adding architecture for the exchange node. %v", err))), nil, nil
	}

	// Return 2 device objects, the first is the fully populated newly created device object. The second is a device
	// object suitable for output (external consumption). Specifically the token is omitted.
	return false, device, exDev
}

// Handles the PATCH verb on this resource. Only the exchange token is updateable.
func UpdateHorizonDevice(device *HorizonDevice,
	errorhandler ErrorHandler,
	getExchangeVersion exchange.ExchangeVersionHandler,
	db *bolt.DB) (bool, *HorizonDevice, *HorizonDevice) {

	LogDeviceEvent(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_API_START_NODE_UPDATE, *device.Id), persistence.EC_START_NODE_UPDATE, device)

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

	// make sure current exchange version meet the requirement
	deviceId := ""
	if device.Org != nil && *device.Org != "" {
		deviceId = fmt.Sprintf("%v/%v", *device.Org, *device.Id)
	} else {
		deviceId = fmt.Sprintf("%v/%v", pDevice.Org, *device.Id)
	}
	if exchangeVersion, err := getExchangeVersion(deviceId, *device.Token); err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Error getting exchange version. error: %v", err))), nil, nil
	} else {
		if err := version.VerifyExchangeVersion1(exchangeVersion, false); err != nil {
			return errorhandler(NewSystemError(fmt.Sprintf("Error verifiying exchange version. error: %v", err))), nil, nil
		}
	}

	updatedDev, err := pDevice.SetExchangeDeviceToken(db, *device.Id, *device.Token)
	if err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("error persisting token update on node object: %v", err))), nil, nil
	}

	// Return 2 device objects, the first is the fully populated newly updated device object. The second is a device
	// object suitable for output (external consumption). Specifically the token is omitted.
	exDev := ConvertFromPersistentHorizonDevice(updatedDev)

	LogDeviceEvent(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_API_COMPLETE_NODE_UPDATE, *device.Id), persistence.EC_NODE_UPDATE_COMPLETE, device)

	return false, device, exDev

}

// Handles the DELETE verb on this resource.
func DeleteHorizonDevice(removeNode string,
	deepClean string,
	block string,
	em *events.EventStateManager,
	msgQueue chan events.Message,
	errorhandler ErrorHandler,
	db *bolt.DB) bool {

	LogDeviceEvent(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_API_START_NODE_UNREG), persistence.EC_START_NODE_UNREG, nil)

	// Check for the device in the local database. If there are errors, they will be written
	// to the HTTP response.
	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		eventlog.LogDatabaseEvent(db, persistence.SEVERITY_ERROR, persistence.NewMessageMeta(EL_API_ERR_READ_NODE_FROM_DB, err.Error()), persistence.EC_DATABASE_ERROR)
		return errorhandler(NewSystemError(fmt.Sprintf("Unable to read node object, error %v", err)))
	} else if pDevice == nil {
		LogDeviceEvent(db, persistence.SEVERITY_ERROR, persistence.NewMessageMeta(EL_API_ERR_NODE_UNREG_NOT_FOUND), persistence.EC_ERROR_NODE_UNREG, nil)
		return errorhandler(NewNotFoundError("The node is not registered.", "node"))
	} else if !pDevice.IsState(persistence.CONFIGSTATE_CONFIGURED) && !pDevice.IsState(persistence.CONFIGSTATE_CONFIGURING) {
		LogDeviceEvent(db, persistence.SEVERITY_ERROR, persistence.NewMessageMeta(EL_API_ERR_NODE_UNREG_NOT_IN_STATE), persistence.EC_ERROR_NODE_UNREG, pDevice)
		return errorhandler(NewBadRequestError(fmt.Sprintf("INVALID_NODE_STATE. The node must be in configured or configuring state in order to unconfigure it.")))
	}

	// Verify optional input
	if removeNode != "" && removeNode != "true" && removeNode != "false" {
		LogDeviceEvent(db, persistence.SEVERITY_ERROR, persistence.NewMessageMeta(EL_API_ERR_NODE_UNREG_WRONG_VALUE_FOR_RN, removeNode), persistence.EC_API_USER_INPUT_ERROR, pDevice)
		return errorhandler(NewAPIUserInputError("%v is an incorrect value for removeNode", "url.removeNode"))
	}
	if deepClean != "" && deepClean != "true" && deepClean != "false" {
		LogDeviceEvent(db, persistence.SEVERITY_ERROR, persistence.NewMessageMeta(EL_API_ERR_NODE_UNREG_WRONG_VALUE_FOR_DC, deepClean), persistence.EC_API_USER_INPUT_ERROR, pDevice)
		return errorhandler(NewAPIUserInputError("%v is an incorrect value for deepClean", "url.deepClean"))
	}
	if block != "" && block != "true" && block != "false" {
		LogDeviceEvent(db, persistence.SEVERITY_ERROR, persistence.NewMessageMeta(EL_API_ERR_NODE_UNREG_WRONG_VALUE_FOR_BLOCK, block), persistence.EC_API_USER_INPUT_ERROR, pDevice)
		return errorhandler(NewAPIUserInputError("%v is an incorrect value for block", "url.block"))
	}

	// Establish defaults for optional inputs
	rNode := false
	if removeNode == "true" {
		rNode = true
	}
	bDeepClean := false
	if deepClean == "true" {
		bDeepClean = true
	}
	blocking := true
	if block == "false" {
		blocking = false
	}

	// Mark the device as "unconfigure in progress"
	_, err = pDevice.SetConfigstate(db, pDevice.Id, persistence.CONFIGSTATE_UNCONFIGURING)
	if err != nil {
		eventlog.LogDatabaseEvent(db, persistence.SEVERITY_ERROR,
			persistence.NewMessageMeta(EL_API_ERR_SAVE_NODE_CONF_TO_DB, err.Error()),
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

	LogDeviceEvent(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_API_COMPLETE_NODE_UNREG, pDevice.Id), persistence.EC_NODE_UNREG_COMPLETE, pDevice)

	// now save this timestamp in db.
	if err := persistence.SaveLastUnregistrationTime(db, uint64(time.Now().Unix())); err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("error persisting the last unregistration timestamp: %v", err)))
	}

	// save this so that the local db will get removed by main.go upon exiting.
	if bDeepClean {
		persistence.SetRemoveDatabaseOnExit(true)
	}

	return false

}
