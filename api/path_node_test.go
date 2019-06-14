// +build unit

package api

import (
	"errors"
	"flag"
	"fmt"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"os"
	"strings"
	"testing"
)

func init() {
	flag.Set("alsologtostderr", "true")
	flag.Set("v", "7")
	// no need to parse flags, that's done by test framework
}

// Create output device object based on env var setting
func Test_FindHDForOutput0(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}

	defer cleanTestDir(dir)

	myDevice := "myid"
	os.Setenv("HZN_DEVICE_ID", myDevice)

	if dev, err := FindHorizonDeviceForOutput(db); err != nil {
		t.Errorf("failed to find device in db, error %v", err)
	} else if dev.Org != nil && *dev.Org != "" {
		t.Errorf("incorrect device found: %v", *dev)
	} else if dev.Id == nil || *dev.Id == "" {
		t.Errorf("id should be filled in, is %v", *dev)
	} else if *dev.Id != myDevice {
		t.Errorf("id is not set correctly, is %v", *dev)
	} else if dev.Config == nil {
		t.Errorf("config state should be initialized, is %v", *dev)
	} else if *dev.Config.State != persistence.CONFIGSTATE_UNCONFIGURED {
		t.Errorf("config state has wrong state %v", *dev)
	}

	os.Unsetenv("HZN_DEVICE_ID")

}

// Create output device object based on object in the DB
func Test_FindHDForOutput1(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}

	defer cleanTestDir(dir)

	theOrg := "myorg"

	_, err = persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", false, theOrg, "apattern", persistence.CONFIGSTATE_CONFIGURING)
	if err != nil {
		t.Errorf("failed to create persisted device, error %v", err)
	}

	if dev, err := FindHorizonDeviceForOutput(db); err != nil {
		t.Errorf("failed to find device in db, error %v", err)
	} else if *dev.Org != theOrg {
		t.Errorf("incorrect device found: %v", *dev)
	} else if dev.Token != nil && *dev.Token != "" {
		t.Errorf("token should not be returned, but is %v", *dev)
	} else if dev.Config == nil {
		t.Errorf("config state should be initialized, is %v", *dev)
	} else if *dev.Config.State != persistence.CONFIGSTATE_CONFIGURING {
		t.Errorf("config state has wrong state %v", *dev)
	}

}

// Create output device object based on object in the DB. THe pattern org is different from the device org
func Test_FindHDForOutput2(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}

	defer cleanTestDir(dir)

	theOrg := "myorg"

	_, err = persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", false, theOrg, "otherorg/apattern", persistence.CONFIGSTATE_CONFIGURING)
	if err != nil {
		t.Errorf("failed to create persisted device, error %v", err)
	}

	if dev, err := FindHorizonDeviceForOutput(db); err != nil {
		t.Errorf("failed to find device in db, error %v", err)
	} else if *dev.Org != theOrg {
		t.Errorf("incorrect device found: %v", *dev)
	} else if *dev.Pattern != "otherorg/apattern" {
		t.Errorf("incorrect pattern found: %v", *dev)
	} else if dev.Token != nil && *dev.Token != "" {
		t.Errorf("token should not be returned, but is %v", *dev)
	} else if dev.Config == nil {
		t.Errorf("config state should be initialized, is %v", *dev)
	} else if *dev.Config.State != persistence.CONFIGSTATE_CONFIGURING {
		t.Errorf("config state has wrong state %v", *dev)
	}
}

// no device id
func Test_CreateHorizonDevice_NoDeviceid(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	badId := ""
	hd := getBasicDevice("myOrg", "myPattern")
	hd.Id = &badId

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)

	errHandled, device, exDevice := CreateHorizonDevice(hd, errorhandler, getDummyGetOrg(), getDummyGetPatternsWithContext(), getDummyGetExchangeVersion(), getDummyPatchDeviceHandler(), events.NewEventStateManager(), db)

	if !errHandled {
		t.Errorf("expected error")
	} else if apiErr, ok := myError.(*APIUserInputError); !ok {
		t.Errorf("myError has the wrong type (%T)", myError)
	} else if apiErr.Input != "device.id" {
		t.Errorf("wrong error input field %v", *apiErr)
	} else if device != nil {
		t.Errorf("device should not be returned")
	} else if exDevice != nil {
		t.Errorf("output device should not be returned")
	}

}

// Invalid characters in id
func Test_CreateHorizonDevice_IllegalId(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	badId := "<0"
	hd := getBasicDevice("myOrg", "myPattern")
	hd.Id = &badId

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)

	errHandled, device, exDevice := CreateHorizonDevice(hd, errorhandler, getDummyGetOrg(), getDummyGetPatternsWithContext(), getDummyGetExchangeVersion(), getDummyPatchDeviceHandler(), events.NewEventStateManager(), db)

	if !errHandled {
		t.Errorf("expected error")
	} else if apiErr, ok := myError.(*APIUserInputError); !ok {
		t.Errorf("myError has the wrong type (%T)", myError)
	} else if apiErr.Input != "device.id" {
		t.Errorf("wrong error input field %v", *apiErr)
	} else if device != nil {
		t.Errorf("device should not be returned")
	} else if exDevice != nil {
		t.Errorf("output device should not be returned")
	}

}

// Invalid characters in org
func Test_CreateHorizonDevice_IllegalOrg(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	badOrg := "<0"
	hd := getBasicDevice(badOrg, "myPattern")

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)

	errHandled, device, exDevice := CreateHorizonDevice(hd, errorhandler, getDummyGetOrg(), getDummyGetPatternsWithContext(), getDummyGetExchangeVersion(), getDummyPatchDeviceHandler(), events.NewEventStateManager(), db)

	if !errHandled {
		t.Errorf("expected error")
	} else if apiErr, ok := myError.(*APIUserInputError); !ok {
		t.Errorf("myError has the wrong type (%T)", myError)
	} else if apiErr.Input != "device.organization" {
		t.Errorf("wrong error input field %v", *apiErr)
	} else if device != nil {
		t.Errorf("device should not be returned")
	} else if exDevice != nil {
		t.Errorf("output device should not be returned")
	}

}

// Invalid characters in pattern
func Test_CreateHorizonDevice_IllegalPattern(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	badPattern := "<0"
	hd := getBasicDevice("myOrg", badPattern)

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)

	errHandled, device, exDevice := CreateHorizonDevice(hd, errorhandler, getDummyGetOrg(), getDummyGetPatternsWithContext(), getDummyGetExchangeVersion(), getDummyPatchDeviceHandler(), events.NewEventStateManager(), db)

	if !errHandled {
		t.Errorf("expected error")
	} else if apiErr, ok := myError.(*APIUserInputError); !ok {
		t.Errorf("myError has the wrong type (%T)", myError)
	} else if apiErr.Input != "device.pattern" {
		t.Errorf("wrong error input field %v", *apiErr)
	} else if device != nil {
		t.Errorf("device should not be returned")
	} else if exDevice != nil {
		t.Errorf("output device should not be returned")
	}

}

// Invalid characters in name
func Test_CreateHorizonDevice_IllegalName(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	badname := "<0"
	hd := getBasicDevice("myOrg", "myPattern")
	hd.Name = &badname

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)

	errHandled, device, exDevice := CreateHorizonDevice(hd, errorhandler, getDummyGetOrg(), getDummyGetPatternsWithContext(), getDummyGetExchangeVersion(), getDummyPatchDeviceHandler(), events.NewEventStateManager(), db)

	if !errHandled {
		t.Errorf("expected error")
	} else if apiErr, ok := myError.(*APIUserInputError); !ok {
		t.Errorf("myError has the wrong type (%T)", myError)
	} else if apiErr.Input != "device.name" {
		t.Errorf("wrong error input field %v", *apiErr)
	} else if device != nil {
		t.Errorf("device should not be returned")
	} else if exDevice != nil {
		t.Errorf("output device should not be returned")
	}

}

// empty token field
func Test_CreateHorizonDevice_IllegalToken(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	hd := getBasicDevice("myOrg", "myPattern")
	hd.Token = nil

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)

	errHandled, device, exDevice := CreateHorizonDevice(hd, errorhandler, getDummyGetOrg(), getDummyGetPatternsWithContext(), getDummyGetExchangeVersion(), getDummyPatchDeviceHandler(), events.NewEventStateManager(), db)

	if !errHandled {
		t.Errorf("expected error")
	} else if apiErr, ok := myError.(*APIUserInputError); !ok {
		t.Errorf("myError has the wrong type (%T)", myError)
	} else if apiErr.Input != "device.token" {
		t.Errorf("wrong error input field %v", *apiErr)
	} else if device != nil {
		t.Errorf("device should not be returned")
	} else if exDevice != nil {
		t.Errorf("output device should not be returned")
	}
}

// exchange version not meet the requirement
func Test_CreateHorizonDevice_WrongExchangeVersion(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	hd := getBasicDevice("myOrg", "myPattern")

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)

	getExchangeVersion := func(id string, token string) (string, error) {
		return "0.1.1", nil
	}

	errHandled, _, _ := CreateHorizonDevice(hd, errorhandler, getDummyGetOrg(), getDummyGetPatternsWithContext(), getExchangeVersion, getDummyPatchDeviceHandler(), events.NewEventStateManager(), db)

	if !errHandled {
		t.Errorf("expected error")
	} else if systemErr, ok := myError.(*SystemError); !ok {
		t.Errorf("myError has the wrong type (%T)", myError)
	} else if !strings.Contains(systemErr.Error(), "Error verifiying exchange version") {
		t.Errorf("wrong message: %v", systemErr.Error())
	}
}

// Successful create, everything works
func Test_CreateHorizonDevice0(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	myOrg := "testOrg"
	myPattern := "testPattern"
	hd := getBasicDevice(myOrg, myPattern)

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)
	getOrg := func(org string, id string, token string) (*exchange.Organization, error) {
		if org == myOrg {
			return &exchange.Organization{
				Label:       "test label",
				Description: "test description",
				LastUpdated: "some time ago",
			}, nil
		} else {
			return nil, errors.New("org not found")
		}
	}

	getPatterns := func(org string, pattern string, id string, token string) (map[string]exchange.Pattern, error) {
		if pattern == myPattern && org == myOrg {
			patid := fmt.Sprintf("%v/%v", org, pattern)
			return map[string]exchange.Pattern{
				patid: exchange.Pattern{
					Label:              "label",
					Description:        "desc",
					Public:             true,
					Services:           []exchange.ServiceReference{},
					AgreementProtocols: []exchange.AgreementProtocol{},
				},
			}, nil
		} else {
			return nil, errors.New("pattern not found")
		}
	}

	errHandled, device, exDevice := CreateHorizonDevice(hd, errorhandler, getOrg, getPatterns, getDummyGetExchangeVersion(), getDummyPatchDeviceHandler(), events.NewEventStateManager(), db)

	if errHandled {
		t.Errorf("unexpected error %v", myError)
	} else if myError != nil && len(myError.Error()) != 0 {
		t.Errorf("myError set unexpectedly (%T) %v", myError, myError)
	} else if *device.Org != myOrg {
		t.Errorf("org did not get set correctly, expecting %v is %v", myOrg, *device)
	} else if exDevice.Token != nil {
		t.Errorf("output device should not have token, but does %v", *exDevice)
	}

}

// device id from env var
func Test_CreateHorizonDevice_EnvVarDeviceid(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	myOrg := "testOrg"
	myPattern := "testPattern"
	hd := getBasicDevice(myOrg, myPattern)
	badId := ""
	hd.Id = &badId
	os.Setenv("HZN_DEVICE_ID", "myDevice")

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)
	getOrg := func(org string, id string, token string) (*exchange.Organization, error) {
		if org == myOrg {
			return &exchange.Organization{
				Label:       "test label",
				Description: "test description",
				LastUpdated: "some time ago",
			}, nil
		} else {
			return nil, errors.New("org not found")
		}
	}

	getPatterns := func(org string, pattern string, id string, token string) (map[string]exchange.Pattern, error) {
		if pattern == myPattern && org == myOrg {
			patid := fmt.Sprintf("%v/%v", org, pattern)
			return map[string]exchange.Pattern{
				patid: exchange.Pattern{
					Label:              "label",
					Description:        "desc",
					Public:             true,
					Services:           []exchange.ServiceReference{},
					AgreementProtocols: []exchange.AgreementProtocol{},
				},
			}, nil
		} else {
			return nil, errors.New("pattern not found")
		}
	}

	errHandled, device, exDevice := CreateHorizonDevice(hd, errorhandler, getOrg, getPatterns, getDummyGetExchangeVersion(), getDummyPatchDeviceHandler(), events.NewEventStateManager(), db)

	if errHandled {
		t.Errorf("unexpected error %v", myError)
	} else if device == nil {
		t.Errorf("device should be returned")
	} else if exDevice == nil {
		t.Errorf("output device should be returned")
	} else if *device.Id != "myDevice" {
		t.Errorf("wrong device id, expected %v but is %v", "myDevice", *device)
	}

	os.Unsetenv("HZN_DEVICE_ID")

}

// Failed create, device already registered
func Test_CreateHorizonDevice_alreadythere(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	myOrg := "testOrg"
	myPattern := "testPattern"
	hd := getBasicDevice(myOrg, myPattern)

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)
	getOrg := func(org string, id string, token string) (*exchange.Organization, error) {
		if org == myOrg {
			return &exchange.Organization{
				Label:       "test label",
				Description: "test description",
				LastUpdated: "some time ago",
			}, nil
		} else {
			return nil, errors.New("org not found")
		}
	}

	getPatterns := func(org string, pattern string, id string, token string) (map[string]exchange.Pattern, error) {
		if pattern == myPattern && org == myOrg {
			patid := fmt.Sprintf("%v/%v", org, pattern)
			return map[string]exchange.Pattern{
				patid: exchange.Pattern{
					Label:              "label",
					Description:        "desc",
					Public:             true,
					Services:           []exchange.ServiceReference{},
					AgreementProtocols: []exchange.AgreementProtocol{},
				},
			}, nil
		} else {
			return nil, errors.New("pattern not found")
		}
	}

	errHandled, device, exDevice := CreateHorizonDevice(hd, errorhandler, getOrg, getPatterns, getDummyGetExchangeVersion(), getDummyPatchDeviceHandler(), events.NewEventStateManager(), db)
	if errHandled {
		t.Errorf("unexpected error %v", myError)
	}

	errHandled, device, exDevice = CreateHorizonDevice(hd, errorhandler, getOrg, getPatterns, getDummyGetExchangeVersion(), getDummyPatchDeviceHandler(), events.NewEventStateManager(), db)

	if !errHandled {
		t.Errorf("expected error")
	} else if conErr, ok := myError.(*ConflictError); !ok {
		t.Errorf("myError has the wrong type (%T)", myError)
	} else if conErr.msg != "device is already registered" {
		t.Errorf("wrong error input field")
	} else if device != nil {
		t.Errorf("device should not be returned")
	} else if exDevice != nil {
		t.Errorf("output device should not be returned")
	}

}

// Org doesnt exist
func Test_CreateHorizonDevice_badorg(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	myOrg := "testOrg"
	myPattern := "testPattern"
	hd := getBasicDevice("otherOrg", myPattern)

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)
	getOrg := func(org string, id string, token string) (*exchange.Organization, error) {
		if org == myOrg {
			return &exchange.Organization{
				Label:       "test label",
				Description: "test description",
				LastUpdated: "some time ago",
			}, nil
		} else {
			return nil, errors.New("org not found")
		}
	}

	getPatterns := func(org string, pattern string, id string, token string) (map[string]exchange.Pattern, error) {
		if pattern == myPattern && org == myOrg {
			patid := fmt.Sprintf("%v/%v", org, pattern)
			return map[string]exchange.Pattern{
				patid: exchange.Pattern{
					Label:              "label",
					Description:        "desc",
					Public:             true,
					Services:           []exchange.ServiceReference{},
					AgreementProtocols: []exchange.AgreementProtocol{},
				},
			}, nil
		} else {
			return nil, errors.New("pattern not found")
		}
	}

	errHandled, device, exDevice := CreateHorizonDevice(hd, errorhandler, getOrg, getPatterns, getDummyGetExchangeVersion(), getDummyPatchDeviceHandler(), events.NewEventStateManager(), db)

	if !errHandled {
		t.Errorf("expected error")
	} else if apiErr, ok := myError.(*APIUserInputError); !ok {
		t.Errorf("myError has the wrong type (%T)", myError)
	} else if apiErr.Input != "device.organization" {
		t.Errorf("wrong error input field %v", *apiErr)
	} else if device != nil {
		t.Errorf("device should not be returned")
	} else if exDevice != nil {
		t.Errorf("output device should not be returned")
	}

}

// Org doesnt exist
func Test_CreateHorizonDevice_badpattern(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	myOrg := "testOrg"
	myPattern := "testPattern"
	hd := getBasicDevice(myOrg, "otherPattern")

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)
	getOrg := func(org string, id string, token string) (*exchange.Organization, error) {
		if org == myOrg {
			return &exchange.Organization{
				Label:       "test label",
				Description: "test description",
				LastUpdated: "some time ago",
			}, nil
		} else {
			return nil, errors.New("org not found")
		}
	}

	getPatterns := func(org string, pattern string, id string, token string) (map[string]exchange.Pattern, error) {
		if pattern == myPattern && org == myOrg {
			patid := fmt.Sprintf("%v/%v", org, pattern)
			return map[string]exchange.Pattern{
				patid: exchange.Pattern{
					Label:              "label",
					Description:        "desc",
					Public:             true,
					Services:           []exchange.ServiceReference{},
					AgreementProtocols: []exchange.AgreementProtocol{},
				},
			}, nil
		} else {
			return nil, errors.New("pattern not found")
		}
	}

	errHandled, device, exDevice := CreateHorizonDevice(hd, errorhandler, getOrg, getPatterns, getDummyGetExchangeVersion(), getDummyPatchDeviceHandler(), events.NewEventStateManager(), db)

	if !errHandled {
		t.Errorf("expected error")
	} else if apiErr, ok := myError.(*APIUserInputError); !ok {
		t.Errorf("myError has the wrong type (%T)", myError)
	} else if apiErr.Input != "device.pattern" {
		t.Errorf("wrong error input field %v", *apiErr)
	} else if device != nil {
		t.Errorf("device should not be returned")
	} else if exDevice != nil {
		t.Errorf("output device should not be returned")
	}

}

// Non-blocking delete of horizondevice
func Test_DeleteHorizonDevice_success(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	myOrg := "testOrg"
	myPattern := "testPattern"
	device := getBasicDevice(myOrg, myPattern)

	_, err = persistence.SaveNewExchangeDevice(db, *device.Id, *device.Token, *device.Name, false, *device.Org, *device.Pattern, persistence.CONFIGSTATE_CONFIGURED)
	if err != nil {
		t.Errorf("unexpected error creating device %v", err)
	}

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)

	removeNode := "false"
	blocking := "false"
	deepClean := "false"
	msgQueue := make(chan events.Message, 10)
	errHandled := DeleteHorizonDevice(removeNode, deepClean, blocking, events.NewEventStateManager(), msgQueue, errorhandler, db)

	if errHandled {
		t.Errorf("unexpected error %v", myError)
	} else if len(msgQueue) != 1 {
		t.Errorf("there should be a message on the queue")
	} else if dev, err := FindHorizonDeviceForOutput(db); err != nil {
		t.Errorf("failed to find device in db, error %v", err)
	} else if *dev.Config.State != persistence.CONFIGSTATE_UNCONFIGURING {
		t.Errorf("config state is incorrect: %v, should be unconfiguring", *dev.Config.State)
	}

}

// Delete of horizondevice fails because its in the wrong state
func Test_DeleteHorizonDevice_fail1(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	myOrg := "testOrg"
	myPattern := "testPattern"
	device := getBasicDevice(myOrg, myPattern)

	_, err = persistence.SaveNewExchangeDevice(db, *device.Id, *device.Token, *device.Name, false, *device.Org, *device.Pattern, persistence.CONFIGSTATE_UNCONFIGURED)
	if err != nil {
		t.Errorf("unexpected error creating device %v", err)
	}

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)

	removeNode := "false"
	blocking := "false"
	deepClean := "false"
	msgQueue := make(chan events.Message, 10)
	errHandled := DeleteHorizonDevice(removeNode, deepClean, blocking, events.NewEventStateManager(), msgQueue, errorhandler, db)

	if !errHandled {
		t.Errorf("expected error")
	} else if _, ok := myError.(*BadRequestError); !ok {
		t.Errorf("myError has the wrong type (%T)", myError)
	} else if len(msgQueue) != 0 {
		t.Errorf("there should not be a message on the queue")
	} else if dev, err := FindHorizonDeviceForOutput(db); err != nil {
		t.Errorf("failed to find device in db, error %v", err)
	} else if *dev.Config.State != persistence.CONFIGSTATE_UNCONFIGURED {
		t.Errorf("config state is incorrect: %v, should be configuring", *dev.Config.State)
	}

}

// Patch of horizondevice fails because its in the wrong state
func Test_PatchHorizonDevice_fail1(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	myOrg := "testOrg"
	myPattern := "testPattern"
	device := getBasicDevice(myOrg, myPattern)

	_, err = persistence.SaveNewExchangeDevice(db, *device.Id, *device.Token, *device.Name, false, *device.Org, *device.Pattern, persistence.CONFIGSTATE_CONFIGURED)
	if err != nil {
		t.Errorf("unexpected error creating device %v", err)
	}

	myId := "testid"
	myToken := "testToken"
	hd := &HorizonDevice{
		Id:    &myId,
		Token: &myToken,
	}

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)

	errHandled, dev1, dev2 := UpdateHorizonDevice(hd, errorhandler, getDummyGetExchangeVersion(), db)

	if !errHandled {
		t.Errorf("expected error")
	} else if _, ok := myError.(*BadRequestError); !ok {
		t.Errorf("myError has the wrong type (%T)", myError)
	} else if dev, err := FindHorizonDeviceForOutput(db); err != nil {
		t.Errorf("failed to find device in db, error %v", err)
	} else if *dev.Config.State != persistence.CONFIGSTATE_CONFIGURED {
		t.Errorf("config state is incorrect: %v, should be configuring", *dev.Config.State)
	} else if dev1 != nil || dev2 != nil {
		t.Errorf("returned non-nil response devices objects: %v %v", *dev1, *dev2)
	}
}

func getBasicDevice(org string, pattern string) *HorizonDevice {
	myId := "testid"
	myName := "testName"
	myToken := "testToken"
	myHA := false

	hd := &HorizonDevice{
		Id:      &myId,
		Org:     &org,
		Pattern: &pattern,
		Name:    &myName,
		Token:   &myToken,
		HA:      &myHA,
	}
	return hd
}
