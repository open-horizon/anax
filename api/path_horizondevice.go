package api

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/persistence"
	"os"
	"time"
)

func FindHorizonDeviceForOutput(db *bolt.DB) (*HorizonDevice, error) {

	var device *HorizonDevice

	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to read horizondevice object, error %v", err))
	} else if pDevice == nil {
		device_id := os.Getenv("CMTN_DEVICE_ID")
		state := CONFIGSTATE_CONFIGURING
		cfg := &Configstate{
			State: &state,
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
	getOrg OrgHandler,
	getPatterns PatternHandler,
	db *bolt.DB) (bool, *HorizonDevice, *HorizonDevice) {

	// Check for the device in the local database. If there are errors, they will be written
	// to the HTTP response.

	if pDevice, err := persistence.FindExchangeDevice(db); err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("unable to read horizondevice object, error %v", err))), nil, nil
	} else if pDevice != nil {
		return errorhandler(NewConflictError("device is already registered")), nil, nil
	}

	glog.V(5).Infof(apiLogString(fmt.Sprintf("Create horizondevice payload: %v", device)))

	// There is no existing device registration in the database, so proceed to verifying the input device object.
	if device.Id == nil || *device.Id == "" {
		device_id := os.Getenv("CMTN_DEVICE_ID")
		if device_id == "" {
			return errorhandler(NewAPIUserInputError("Either setup CMTN_DEVICE_ID environmental variable or specify device.id.", "device.id")), nil, nil
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

	// Verify that the input pattern is defined in the exchange. A device (or node) canonly use patterns that are defined within its own org.
	if device.Pattern != nil && *device.Pattern != "" {
		if patternDefs, err := getPatterns(*device.Org, *device.Pattern, deviceId, *device.Token); err != nil {
			return errorhandler(NewAPIUserInputError(fmt.Sprintf("error searching for pattern %v in exchange, error: %v", *device.Pattern, err), "device.pattern")), nil, nil
		} else if _, ok := patternDefs[fmt.Sprintf("%v/%v", *device.Org, *device.Pattern)]; !ok {
			return errorhandler(NewAPIUserInputError(fmt.Sprintf("pattern %v not found in exchange, error: %v", *device.Pattern, err), "device.pattern")), nil, nil
		}
	}

	// So far everything checks out and verifies, so save the registration to the local database.
	haDevice := false
	if device.HADevice != nil && *device.HADevice == true {
		haDevice = true
	}

	pDev, err := persistence.SaveNewExchangeDevice(db, *device.Id, *device.Token, *device.Name, haDevice, *device.Org, *device.Pattern, CONFIGSTATE_CONFIGURING)
	if err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("error persisting new device registration: %v", err))), nil, nil
	}

	glog.V(5).Infof(apiLogString(fmt.Sprintf("Create horizondevice updated: %v", pDev)))

	// Return 2 device objects, the first is the fully populated newly created device object. The second is a device
	// object suitable for output (external consumption). Specifically the token is omitted.
	exDev := ConvertFromPersistentHorizonDevice(pDev)
	return false, device, exDev

}

// Handles the PATCH verb on this resource. Only the exchange token is updateable.
func UpdateHorizonDevice(device *HorizonDevice,
	errorhandler ErrorHandler,
	db *bolt.DB) (bool, *HorizonDevice, *HorizonDevice) {

	// Check for the device in the local database. If there are errors, they will be written
	// to the HTTP response.
	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Unable to read horizondevice object, error %v", err))), nil, nil
	} else if pDevice == nil {
		return errorhandler(NewNotFoundError("Exchange registration not recorded. Complete account and device registration with an exchange and then record device registration using this API.", "horizondevice")), nil, nil
	} else if pDevice.IsState(CONFIGSTATE_UNCONFIGURING) {
		return errorhandler(NewBadRequestError(fmt.Sprintf("The node is already unconfiguring. The GET API will return HTTP status 404 when unconfiguration is complete."))), nil, nil
	}

	// Verify that the input id is ok.
	if bail := checkInputString(errorhandler, "device.id", device.Id); bail {
		return true, nil, nil
	}

	// We dont compute the token so there is no need to try to check it.

	// If there is no token, that's an errir
	if device.Token == nil {
		return errorhandler(NewAPIUserInputError("null and must not be", "device.token")), nil, nil
	}

	updatedDev, err := pDevice.SetExchangeDeviceToken(db, *device.Id, *device.Token)
	if err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("error persisting token update on horizondevice object: %v", err))), nil, nil
	}

	// Return 2 device objects, the first is the fully populated newly updated device object. The second is a device
	// object suitable for output (external consumption). Specifically the token is omitted.
	exDev := ConvertFromPersistentHorizonDevice(updatedDev)
	return false, device, exDev

}

// Handles the DELETE verb on this resource.
func DeleteHorizonDevice(removeNode string,
	block string,
	em *events.EventStateManager,
	msgQueue chan events.Message,
	errorhandler ErrorHandler,
	db *bolt.DB) bool {

	// Check for the device in the local database. If there are errors, they will be written
	// to the HTTP response.
	pDevice, err := persistence.FindExchangeDevice(db)
	if err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("Unable to read horizondevice object, error %v", err)))
	} else if pDevice == nil {
		return errorhandler(NewNotFoundError("Exchange registration not recorded. Complete account and device registration with an exchange and then record device registration using this API.", "horizondevice"))
	} else if pDevice.IsState(CONFIGSTATE_UNCONFIGURING) {
		return errorhandler(NewBadRequestError(fmt.Sprintf("The node is already unconfiguring. The GET API will return HTTP status 404 when unconfiguration is complete.")))
	}

	// Verify optional input
	if removeNode != "" && removeNode != "true" && removeNode != "false" {
		return errorhandler(NewAPIUserInputError("%v is an incorrect value for removeNode", "url.removeNode"))
	}

	if block != "" && block != "true" && block != "false" {
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
	_, err = pDevice.SetDeviceState(db, CONFIGSTATE_UNCONFIGURING)
	if err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("error persisting unconfiguring on horizondevice object: %v", err)))
	}

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

	return false

}
