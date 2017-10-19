// +build unit

package api

import (
	"errors"
	"flag"
	"fmt"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"os"
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
	os.Setenv("CMTN_DEVICE_ID", myDevice)

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
	} else if *dev.Config.State != CONFIGSTATE_CONFIGURING {
		t.Errorf("config state has wrong state %v", *dev)
	}

	os.Unsetenv("CMTN_DEVICE_ID")

}

// Create output device object based on object in the DB
func Test_FindHDForOutput1(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}

	defer cleanTestDir(dir)

	theOrg := "myorg"

	_, err = persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", false, theOrg, "apattern", CONFIGSTATE_CONFIGURING)
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
	} else if *dev.Config.State != CONFIGSTATE_CONFIGURING {
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

	errHandled, device, exDevice := CreateHorizonDevice(hd, errorhandler, getDummyGetOrg(), getDummyGetPatterns(), db)

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

	errHandled, device, exDevice := CreateHorizonDevice(hd, errorhandler, getDummyGetOrg(), getDummyGetPatterns(), db)

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

	errHandled, device, exDevice := CreateHorizonDevice(hd, errorhandler, getDummyGetOrg(), getDummyGetPatterns(), db)

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

	errHandled, device, exDevice := CreateHorizonDevice(hd, errorhandler, getDummyGetOrg(), getDummyGetPatterns(), db)

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

	errHandled, device, exDevice := CreateHorizonDevice(hd, errorhandler, getDummyGetOrg(), getDummyGetPatterns(), db)

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

	errHandled, device, exDevice := CreateHorizonDevice(hd, errorhandler, getDummyGetOrg(), getDummyGetPatterns(), db)

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
					Workloads:          []exchange.WorkloadReference{},
					AgreementProtocols: []exchange.AgreementProtocol{},
				},
			}, nil
		} else {
			return nil, errors.New("pattern not found")
		}
	}

	errHandled, device, exDevice := CreateHorizonDevice(hd, errorhandler, getOrg, getPatterns, db)

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
	os.Setenv("CMTN_DEVICE_ID", "myDevice")

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
					Workloads:          []exchange.WorkloadReference{},
					AgreementProtocols: []exchange.AgreementProtocol{},
				},
			}, nil
		} else {
			return nil, errors.New("pattern not found")
		}
	}

	errHandled, device, exDevice := CreateHorizonDevice(hd, errorhandler, getOrg, getPatterns, db)

	if errHandled {
		t.Errorf("unexpected error %v", myError)
	} else if device == nil {
		t.Errorf("device should be returned")
	} else if exDevice == nil {
		t.Errorf("output device should be returned")
	} else if *device.Id != "myDevice" {
		t.Errorf("wrong device id, expected %v but is %v", "myDevice", *device)
	}

	os.Unsetenv("CMTN_DEVICE_ID")

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
					Workloads:          []exchange.WorkloadReference{},
					AgreementProtocols: []exchange.AgreementProtocol{},
				},
			}, nil
		} else {
			return nil, errors.New("pattern not found")
		}
	}

	errHandled, device, exDevice := CreateHorizonDevice(hd, errorhandler, getOrg, getPatterns, db)
	if errHandled {
		t.Errorf("unexpected error %v", myError)
	}

	errHandled, device, exDevice = CreateHorizonDevice(hd, errorhandler, getOrg, getPatterns, db)

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
					Workloads:          []exchange.WorkloadReference{},
					AgreementProtocols: []exchange.AgreementProtocol{},
				},
			}, nil
		} else {
			return nil, errors.New("pattern not found")
		}
	}

	errHandled, device, exDevice := CreateHorizonDevice(hd, errorhandler, getOrg, getPatterns, db)

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
					Workloads:          []exchange.WorkloadReference{},
					AgreementProtocols: []exchange.AgreementProtocol{},
				},
			}, nil
		} else {
			return nil, errors.New("pattern not found")
		}
	}

	errHandled, device, exDevice := CreateHorizonDevice(hd, errorhandler, getOrg, getPatterns, db)

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

func getBasicDevice(org string, pattern string) *HorizonDevice {
	myId := "testid"
	myName := "testName"
	myToken := "testToken"
	myHA := false

	hd := &HorizonDevice{
		Id:       &myId,
		Org:      &org,
		Pattern:  &pattern,
		Name:     &myName,
		Token:    &myToken,
		HADevice: &myHA,
	}
	return hd
}
