// +build unit

package api

import (
	"flag"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"strings"
	"testing"
)

func init() {
	flag.Set("alsologtostderr", "true")
	flag.Set("v", "7")
	// no need to parse flags, that's done by test framework
}

func Test_ChangeServiceConfigState(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	_, err = persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", "", false, "myOrg", "apattern", persistence.CONFIGSTATE_CONFIGURING)
	if err != nil {
		t.Errorf("failed to create persisted device, error %v", err)
	}

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)
	deviceHandler := getTestDeviceHandler()
	postSCS := getTestPostDeviceSCSHandler()

	// turn 1 to suspended
	service_cs := exchange.ServiceConfigState{
		Url:         "netspeed",
		Org:         "myorg1",
		ConfigState: "suspended",
	}
	errHandled, changed_services := ChangeServiceConfigState(&service_cs, errorhandler, deviceHandler, postSCS, db)
	if errHandled {
		t.Errorf("ChangeServiceConfigState should have returned false for error_handled but got true.")
	} else if changed_services == nil {
		t.Errorf("returned suspended services should not be nil")
	} else if len(changed_services) != 1 {
		t.Errorf("returned suspended services should have 1 element, but got %v", len(changed_services))
	} else if changed_services[0].Url != "netspeed" || changed_services[0].Org != "myorg1" {
		t.Errorf("ChangeServiceConfigState returned wrong suspended service: %v", changed_services[0])
	}

	// turn all to suspended
	service_cs = exchange.ServiceConfigState{
		Url:         "",
		Org:         "",
		ConfigState: "suspended",
	}
	errHandled, changed_services = ChangeServiceConfigState(&service_cs, errorhandler, deviceHandler, postSCS, db)
	if errHandled {
		t.Errorf("ChangeServiceConfigState should have returned false for error_handled but got true.")
	} else if changed_services == nil {
		t.Errorf("returned suspended services should not be nil")
	} else if len(changed_services) != 4 {
		t.Errorf("returned suspended services should have 4 element, but got %v", len(changed_services))
	}

	// turn all in myorg1 to suspended
	service_cs = exchange.ServiceConfigState{
		Url:         "",
		Org:         "myorg1",
		ConfigState: "suspended",
	}
	errHandled, changed_services = ChangeServiceConfigState(&service_cs, errorhandler, deviceHandler, postSCS, db)
	if errHandled {
		t.Errorf("ChangeServiceConfigState should have returned false for error_handled but got true.")
	} else if changed_services == nil {
		t.Errorf("returned suspended services should not be nil")
	} else if len(changed_services) != 2 {
		t.Errorf("returned suspended services should have 2 element, but got %v", len(changed_services))
	}

	// do nothing for already suspended
	service_cs = exchange.ServiceConfigState{
		Url:         "gps",
		Org:         "myorg2",
		ConfigState: "suspended",
	}
	errHandled, changed_services = ChangeServiceConfigState(&service_cs, errorhandler, deviceHandler, postSCS, db)
	if errHandled {
		t.Errorf("ChangeServiceConfigState should have returned false for error_handled but got true.")
	} else if changed_services == nil {
		t.Errorf("returned suspended services should not be nil")
	} else if len(changed_services) != 0 {
		t.Errorf("returned suspended services should have 0 element, but got %v", len(changed_services))
	}

	// turn all to active
	service_cs = exchange.ServiceConfigState{
		Url:         "",
		Org:         "",
		ConfigState: "active",
	}
	errHandled, changed_services = ChangeServiceConfigState(&service_cs, errorhandler, deviceHandler, postSCS, db)
	if errHandled {
		t.Errorf("ChangeServiceConfigState should have returned false for error_handled but got true.")
	} else if changed_services == nil {
		t.Errorf("returned suspended services should not be nil")
	} else if len(changed_services) != 1 {
		t.Errorf("returned services should have 1 element, but got %v", len(changed_services))
	}

}

func Test_ChangeServiceConfigState_Wrong_Url(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	_, err = persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", "", false, "myOrg", "apattern", persistence.CONFIGSTATE_CONFIGURING)
	if err != nil {
		t.Errorf("failed to create persisted device, error %v", err)
	}

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)
	deviceHandler := getTestDeviceHandler()
	postSCS := getTestPostDeviceSCSHandler()

	// turn service does not exist
	service_cs := exchange.ServiceConfigState{
		Url:         "gps",
		Org:         "myorg1",
		ConfigState: "suspended",
	}
	errHandled, _ := ChangeServiceConfigState(&service_cs, errorhandler, deviceHandler, postSCS, db)
	if !errHandled {
		t.Errorf("ChangeServiceConfigState should have returned true for error_handled but got false.")
	} else if !strings.Contains(myError.Error(), "does not exist") {
		t.Errorf("ChangeServiceConfigState returned wrong error message: %v.", myError.Error())
	}
}

func Test_ChangeServiceConfigState_Device_not_registered(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)
	deviceHandler := getTestDeviceHandler()
	postSCS := getTestPostDeviceSCSHandler()

	// turn service does not exist
	service_cs := exchange.ServiceConfigState{
		Url:         "gps",
		Org:         "myorg1",
		ConfigState: "suspended",
	}
	errHandled, _ := ChangeServiceConfigState(&service_cs, errorhandler, deviceHandler, postSCS, db)
	if !errHandled {
		t.Errorf("ChangeServiceConfigState should have returned true for error_handled but got false.")
	} else if !strings.Contains(myError.Error(), "Exchange registration not recorded") {
		t.Errorf("ChangeServiceConfigState returned wrong error message: %v.", myError)
	}
}

func getTestDeviceHandler() exchange.DeviceHandler {
	return func(id string, token string) (*exchange.Device, error) {
		m1 := exchange.Microservice{
			Url:         "myorg1/netspeed",
			ConfigState: "active",
		}
		m2 := exchange.Microservice{
			Url:         "myorg1/weather",
			ConfigState: "active",
		}
		m3 := exchange.Microservice{
			Url:         "myorg2/cpu",
			ConfigState: "active",
		}
		m4 := exchange.Microservice{
			Url:         "myorg2/netspeed",
			ConfigState: "active",
		}
		m5 := exchange.Microservice{
			Url:         "myorg2/gps",
			ConfigState: "suspended",
		}

		dev := exchange.Device{
			Token:              "testtoken",
			Name:               "testname",
			Pattern:            "apattern",
			RegisteredServices: []exchange.Microservice{m1, m2, m3, m4, m5},
		}
		return &dev, nil
	}
}

func getTestPostDeviceSCSHandler() exchange.PostDeviceServicesConfigStateHandler {
	return func(id string, token string, svcsConfigState *exchange.ServiceConfigState) error {
		return nil
	}
}
