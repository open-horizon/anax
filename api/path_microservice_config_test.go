// +build unit

package api

import (
	"flag"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"testing"
)

func init() {
	flag.Set("alsologtostderr", "true")
	flag.Set("v", "7")
	// no need to parse flags, that's done by test framework
}

func Test_CreateService0(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	surl := "http://dummy.com"
	myOrg := "myorg"
	name := "name"
	vers := "1.0.0"
	attrs := []Attribute{}
	autoU := true
	activeU := true

	service := &Service{
		SensorUrl:     &surl,
		SensorOrg:     &myOrg,
		SensorName:    &name,
		SensorVersion: &vers,
		AutoUpgrade:   &autoU,
		ActiveUpgrade: &activeU,
		Attributes:    &attrs,
	}

	_, err = persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", false, myOrg, "apattern", CONFIGSTATE_CONFIGURING)
	if err != nil {
		t.Errorf("failed to create persisted device, error %v", err)
	}

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)
	msHandler := getVariableMicroserviceHandler(exchange.UserInput{})
	errHandled, newService, msg := CreateService(service, errorhandler, getDummyGetPatterns(), getDummyWorkloadResolver(), msHandler, db, getBasicConfig(), false)
	if errHandled {
		t.Errorf("unexpected error (%T) %v", myError, myError)
	} else if newService == nil {
		t.Errorf("returned service should not be nil")
	} else if msg == nil {
		t.Errorf("returned msg should not be nil")
	}

}
