// +build unit

package api

import (
	"flag"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
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
	vers := "[1.0.0,INFINITY)"
	attrs := []Attribute{}
	autoU := true
	activeU := true

	service := &Service{
		Url:           &surl,
		Org:           &myOrg,
		VersionRange:  &vers,
		AutoUpgrade:   &autoU,
		ActiveUpgrade: &activeU,
		Attributes:    &attrs,
	}

	_, err = persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", "device", false, myOrg, "apattern", persistence.CONFIGSTATE_CONFIGURING)
	if err != nil {
		t.Errorf("failed to create persisted device, error %v", err)
	}

	sref := exchange.ServiceReference{
		ServiceURL:      surl,
		ServiceOrg:      myOrg,
		ServiceArch:     cutil.ArchString(),
		ServiceVersions: []exchange.WorkloadChoice{},
		DataVerify:      exchange.DataVerification{},
		NodeH:           exchange.NodeHealth{},
	}
	patternHandler := getVariablePatternHandler(sref)

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)
	sHandler := getVariableServiceHandler(exchange.UserInput{})
	errHandled, newService, msg := CreateService(service, errorhandler, patternHandler, getDummyServiceDefResolver(), sHandler, getDummyDeviceHandler(), getDummyPatchDeviceHandler(), nil, db, getBasicConfig(), false)
	if errHandled {
		t.Errorf("unexpected error (%T) %v", myError, myError)
	} else if newService == nil {
		t.Errorf("returned service should not be nil")
	} else if msg == nil {
		t.Errorf("returned msg should not be nil")
	}

}

func Test_validateUserInput(t *testing.T) {
	ui := []exchange.UserInput{
		exchange.UserInput{Name: "var1", Type: "string", DefaultValue: "val1"},
		exchange.UserInput{Name: "var2", Type: "int"},
		exchange.UserInput{Name: "var3", Type: "float", DefaultValue: ""},
		exchange.UserInput{Name: "var4", Type: "bool"},
		exchange.UserInput{Name: "var5", Type: "list of strings", DefaultValue: "[123, 456]"},
	}

	sdef := exchange.ServiceDefinition{UserInputs: ui}

	userInput := policy.UserInput{
		ServiceOrgid:        "mycomp",
		ServiceUrl:          "cpu",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
		Inputs: []policy.Input{policy.Input{Name: "var1", Value: "val11"},
			policy.Input{Name: "var2", Value: 21},
			policy.Input{Name: "var3", Value: 16.5},
			policy.Input{Name: "var4", Value: false},
			policy.Input{Name: "var5", Value: []string{"abcd", "1234"}},
		},
	}

	ok, missedName := validateUserInput(&sdef, &userInput)
	if !ok {
		t.Errorf("validateUserInput should return true, but not.")
	} else if missedName != "" {
		t.Errorf("missedName should be empty but got: %v.", missedName)
	}

	ip := []policy.Input{policy.Input{Name: "var1", Value: "val11"},
		policy.Input{Name: "var4", Value: false},
		policy.Input{Name: "var5", Value: []string{"abcd", "1234"}},
	}
	userInput.Inputs = ip
	ok, missedName = validateUserInput(&sdef, &userInput)
	if ok {
		t.Errorf("validateUserInput should return false, but not.")
	} else if missedName != "var2" {
		t.Errorf("missedName should be var2 but got: %v.", missedName)
	}

	userInput.Inputs = nil
	ok, missedName = validateUserInput(&sdef, &userInput)
	if ok {
		t.Errorf("validateUserInput should return false, but not.")
	} else if missedName != "var2" {
		t.Errorf("missedName should be var2 but got: %v.", missedName)
	}

	ip = []policy.Input{policy.Input{Name: "var2", Value: 21},
		policy.Input{Name: "var3", Value: 16.5},
	}
	userInput.Inputs = ip

	ok, missedName = validateUserInput(&sdef, &userInput)
	if ok {
		t.Errorf("validateUserInput should return false, but not.")
	} else if missedName != "var4" {
		t.Errorf("missedName should be var4 but got: %v.", missedName)
	}
}
