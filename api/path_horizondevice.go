package api

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/persistence"
	"os"
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
