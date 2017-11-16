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

func Test_FindWorkloadConfigForOutput0(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	// No workloadconfigs in the DB yet.

	// Now test the GET /agreements function.
	if wcsout, err := FindWorkloadConfigForOutput(db); err != nil {
		t.Errorf("error finding workloadconfigs: %v", err)
	} else if len(wcsout["config"]) != 0 {
		t.Errorf("expecting 0 active workloadconfigs have %v", wcsout["config"])
	} else if len(wcsout) != 1 {
		t.Errorf("should always be 1 key in the output, %v", wcsout)
	} else if _, ok := wcsout["config"]; !ok {
		t.Errorf("should always have config key in the output, %v", wcsout)
	}
}

func Test_CreateWorkloadConfig_success(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	myorg := "myorg"
	mypattern := "mypattern"
	myurl := "myurl"
	myversion := ""
	myarch := "amd64"
	myvar := "varName"

	vars := map[string]interface{}{
		myvar: "ethel",
	}

	attr := NewAttribute("UserInputAttributes", []string{}, "label", false, false, vars)

	cfg := WorkloadConfig{
		WorkloadURL: myurl,
		Org:         myorg,
		Version:     myversion,
		Attributes:  []Attribute{*attr},
	}

	existingDevice := persistence.ExchangeDevice{
		Id:                 "12345",
		Org:                myorg,
		Pattern:            mypattern,
		Name:               "fred",
		Token:              "abc",
		TokenLastValidTime: 0,
		TokenValid:         true,
		HA:                 false,
		Config: persistence.Configstate{
			State:          CONFIGSTATE_CONFIGURING,
			LastUpdateTime: 0,
		},
	}

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)

	ui := exchange.UserInput{
		Name:         myvar,
		Label:        "label",
		Type:         "string",
		DefaultValue: "",
	}

	getWorkload := getVariableWorkload(myurl, myorg, myversion, myarch, &ui)

	errHandled, newWC := CreateWorkloadconfig(&cfg, &existingDevice, errorhandler, getWorkload, db)

	if errHandled {
		t.Errorf("unexpected error %v", myError)
	} else if newWC == nil {
		t.Errorf("expected non-nil workloadconfig object")
	} else if newWC.Attributes[0].GetGenericMappings()[myvar] != "ethel" {
		t.Errorf("expected %v to be in workloadconfig variables: %v", myvar, newWC)
	} else if newWC.VersionExpression != "[0.0.0,INFINITY)" {
		t.Errorf("version not defaulted correctly, expected %v, but is %v", "[0.0.0,INFINITY)", newWC)
	}

}

func Test_CreateWorkloadConfig_fail(t *testing.T) {
	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	myorg := "myorg"
	mypattern := "mypattern"
	myurl := "myurl"
	myversion := ""
	myarch := "amd64"

	attr := NewAttribute("UserInputAttributes", []string{}, "label", false, false, map[string]interface{}{})

	cfg := WorkloadConfig{
		WorkloadURL: myurl,
		Org:         myorg,
		Version:     myversion,
		Attributes:  []Attribute{*attr},
	}

	existingDevice := persistence.ExchangeDevice{
		Id:                 "12345",
		Org:                myorg,
		Pattern:            mypattern,
		Name:               "fred",
		Token:              "abc",
		TokenLastValidTime: 0,
		TokenValid:         true,
		HA:                 false,
		Config: persistence.Configstate{
			State:          CONFIGSTATE_CONFIGURING,
			LastUpdateTime: 0,
		},
	}

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)

	ui := exchange.UserInput{
		Name:         "varName",
		Label:        "label",
		Type:         "string",
		DefaultValue: "",
	}

	getWorkload := getVariableWorkload(myurl, myorg, myversion, myarch, &ui)

	errHandled, newWC := CreateWorkloadconfig(&cfg, &existingDevice, errorhandler, getWorkload, db)

	if !errHandled {
		t.Errorf("expected error")
	} else if apiErr, ok := myError.(*APIUserInputError); !ok {
		t.Errorf("myError has the wrong type (%T)", myError)
	} else if apiErr.Input != "variables" {
		t.Errorf("wrong error input field %v", *apiErr)
	} else if newWC != nil {
		t.Errorf("expected nil workloadconfig object")
	}

}
