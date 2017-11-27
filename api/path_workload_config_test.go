// +build unit

package api

import (
	"bytes"
	"encoding/json"
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

	// Now test the GET /workload/config function.
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

func Test_CreateDeleteWorkloadConfig_success(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	myorg := "myorg"
	mypattern := "mypattern"
	myurl := "myurl"
	myversion := "1.0.0"
	myarch := "amd64"
	mystr := "varName"
	mybool := "varBool"
	myint := "varInt"
	myfloat := "varFloat"
	myastr := "varAString"

	varss := `{` +
		`"` + mystr + `":"ethel",` +
		`"` + mybool + `":true,` +
		`"` + myint + `":6,` +
		`"` + myfloat + `":11.3,` +
		`"` + myastr + `":["abc","123"]` +
		`}`

	decoder := json.NewDecoder(bytes.NewReader([]byte(varss)))
	decoder.UseNumber()

	vars := make(map[string]interface{}, 0)

	if err := decoder.Decode(&vars); err != nil {
		t.Errorf("error decoding variable attribute %v", err)
	}

	// if err := json.Unmarshal([]byte(varss), &vars); err != nil {
	// 	t.Errorf("Error unmarshalling variables json string: %v error:%v\n", varss, err)
	// }

	// vars := map[string]interface{}{
	// 	mystr: "ethel",
	// 	mybool: true,
	// 	myint: 5,
	// 	myfloat: 11.3,
	// 	myastr: []string{"abc","123"},
	// }

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

	ui := []exchange.UserInput{
		exchange.UserInput{
			Name:         mystr,
			Label:        "str",
			Type:         "string",
			DefaultValue: "",
		},
		exchange.UserInput{
			Name:         mybool,
			Label:        "bool",
			Type:         "boolean",
			DefaultValue: "",
		},
		exchange.UserInput{
			Name:         myint,
			Label:        "int",
			Type:         "int",
			DefaultValue: "",
		},
		exchange.UserInput{
			Name:         myfloat,
			Label:        "float",
			Type:         "float",
			DefaultValue: "",
		},
		exchange.UserInput{
			Name:         myastr,
			Label:        "array string",
			Type:         "list of strings",
			DefaultValue: "",
		},
	}

	getWorkload := getVariableWorkload(myurl, myorg, myversion, myarch, ui)

	errHandled, newWC := CreateWorkloadconfig(&cfg, &existingDevice, errorhandler, getWorkload, db)

	if errHandled {
		t.Errorf("unexpected error %v", myError)
	} else if newWC == nil {
		t.Errorf("expected non-nil workloadconfig object")
	} else if newWC.Attributes[0].GetGenericMappings()[mystr] != "ethel" {
		t.Errorf("expected %v to be in workloadconfig variables: %v", mystr, newWC)
	} else if newWC.VersionExpression != "[1.0.0,INFINITY)" {
		t.Errorf("version not defaulted correctly, expected %v, but is %v", "[1.0.0,INFINITY)", newWC)
	}

	// Delete the wc just added
	errHandled = DeleteWorkloadconfig(&cfg, errorhandler, db)
	if errHandled {
		t.Errorf("unexpected error %v", myError)
	}

	// Make sure te deleted wc is gone
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

	ui := []exchange.UserInput{
		exchange.UserInput{
			Name:         "varName",
			Label:        "label",
			Type:         "string",
			DefaultValue: "",
		},
	}

	getWorkload := getVariableWorkload(myurl, myorg, myversion, myarch, ui)

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
