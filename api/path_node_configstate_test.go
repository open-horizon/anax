// +build unit

package api

import (
	"flag"
	"github.com/open-horizon/anax/cutil"
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

func Test_FindCSForOutput0(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	if cfg, err := FindConfigstateForOutput(db); err != nil {
		t.Errorf("failed to find device in db, error %v", err)
	} else if *cfg.State != persistence.CONFIGSTATE_UNCONFIGURED {
		t.Errorf("incorrect configstate found: %v", *cfg)
	}

}

// Create output configstate object based on object in the DB
func Test_FindCSForOutput1(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	theOrg := "myorg"

	_, err = persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", "device", false, theOrg, "apattern", persistence.CONFIGSTATE_CONFIGURING)
	if err != nil {
		t.Errorf("failed to create persisted device, error %v", err)
	}

	if cfg, err := FindConfigstateForOutput(db); err != nil {
		t.Errorf("failed to find device in db, error %v", err)
	} else if *cfg.State != persistence.CONFIGSTATE_CONFIGURING {
		t.Errorf("incorrect configstate, found: %v", *cfg)
	} else if *cfg.LastUpdateTime == uint64(0) {
		t.Errorf("incorrect last update time, found: %v", *cfg)
	}

}

// =================================== tests based on the services ===============================
// change state to configured - top level service and dependent service
func Test_UpdateConfigstate2services(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	cs := getBasicConfigstate()
	state := persistence.CONFIGSTATE_CONFIGURED
	cs.State = &state

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)

	myOrg := "myorg"
	myPattern := "mypattern"

	_, err = persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", "", false, myOrg, myPattern, persistence.CONFIGSTATE_CONFIGURING)
	if err != nil {
		t.Errorf("failed to create persisted device, error %v", err)
	}

	wc := exchange.WorkloadChoice{
		Version:                      "1.0.0",
		Priority:                     exchange.WorkloadPriority{},
		Upgrade:                      exchange.UpgradePolicy{},
		DeploymentOverrides:          "",
		DeploymentOverridesSignature: "",
	}

	sref := exchange.ServiceReference{
		ServiceURL:      "wurl",
		ServiceOrg:      myOrg,
		ServiceArch:     cutil.ArchString(),
		ServiceVersions: []exchange.WorkloadChoice{wc},
		DataVerify:      exchange.DataVerification{},
		NodeH:           exchange.NodeHealth{},
	}

	mURL := "http://utest.com/mservice"
	mVersion := "1.0.0"
	mArch := cutil.ArchString()
	sResolver := getVariableServiceDefResolver(mURL, myOrg, mVersion, mArch, nil)
	sHandler := getVariableServiceHandler(exchange.UserInput{})

	patternHandler := getVariablePatternHandler(sref)
	errHandled, cfg, msgs := UpdateConfigstate(cs, errorhandler, patternHandler, sResolver, sHandler, getDummyDeviceHandler(), getDummyPatchDeviceHandler(), db, getBasicConfig())

	if errHandled {
		t.Errorf("%v", myError)
	} else if myError != nil && len(myError.Error()) != 0 {
		t.Errorf("myError set unexpectedly (%T) %v", myError, myError)
	} else if cfg == nil {
		t.Errorf("no configstate returned")
	} else if *cfg.State != persistence.CONFIGSTATE_CONFIGURED {
		t.Errorf("wrong state field %v", *cfg)
	} else if *cfg.LastUpdateTime == uint64(0) {
		t.Errorf("last update time should be set, is %v", *cfg)
	} else if len(msgs) != 2 {
		t.Errorf("there should be 2 messages, received %v", len(msgs))
	}

}

// change state to configured - top level service only
func Test_UpdateConfigstate2service_only(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	cs := getBasicConfigstate()
	state := persistence.CONFIGSTATE_CONFIGURED
	cs.State = &state

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)

	myOrg := "myorg"
	myPattern := "mypattern"

	_, err = persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", "device", false, myOrg, myPattern, persistence.CONFIGSTATE_CONFIGURING)
	if err != nil {
		t.Errorf("failed to create persisted device, error %v", err)
	}

	wc := exchange.WorkloadChoice{
		Version:                      "1.0.0",
		Priority:                     exchange.WorkloadPriority{},
		Upgrade:                      exchange.UpgradePolicy{},
		DeploymentOverrides:          "",
		DeploymentOverridesSignature: "",
	}

	sref := exchange.ServiceReference{
		ServiceURL:      "wurl",
		ServiceOrg:      myOrg,
		ServiceArch:     cutil.ArchString(),
		ServiceVersions: []exchange.WorkloadChoice{wc},
		DataVerify:      exchange.DataVerification{},
		NodeH:           exchange.NodeHealth{},
	}

	sResolver := getVariableServiceDefResolver("", "", "", "", nil)
	sHandler := getVariableServiceHandler(exchange.UserInput{})

	patternHandler := getVariablePatternHandler(sref)
	errHandled, cfg, msgs := UpdateConfigstate(cs, errorhandler, patternHandler, sResolver, sHandler, getDummyDeviceHandler(), getDummyPatchDeviceHandler(), db, getBasicConfig())

	if errHandled {
		t.Errorf("%v", myError)
	} else if myError != nil && len(myError.Error()) != 0 {
		t.Errorf("myError set unexpectedly (%T) %v", myError, myError)
	} else if cfg == nil {
		t.Errorf("no configstate returned")
	} else if *cfg.State != persistence.CONFIGSTATE_CONFIGURED {
		t.Errorf("wrong state field %v", *cfg)
	} else if *cfg.LastUpdateTime == uint64(0) {
		t.Errorf("last update time should be set, is %v", *cfg)
	} else if len(msgs) != 1 {
		t.Errorf("there should be 1 message, received %v", len(msgs))
	}

}

// change state from configured to configuring
func Test_UpdateConfigstate_Illegal_state_change_services(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	cs := getBasicConfigstate()
	state := persistence.CONFIGSTATE_CONFIGURED
	cs.State = &state

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)

	myOrg := "myorg"
	myPattern := "mypattern"

	_, err = persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", "device", false, myOrg, myPattern, persistence.CONFIGSTATE_CONFIGURING)
	if err != nil {
		t.Errorf("failed to create persisted device, error %v", err)
	}

	wc := exchange.WorkloadChoice{
		Version:                      "1.0.0",
		Priority:                     exchange.WorkloadPriority{},
		Upgrade:                      exchange.UpgradePolicy{},
		DeploymentOverrides:          "",
		DeploymentOverridesSignature: "",
	}

	sref := exchange.ServiceReference{
		ServiceURL:      "wurl",
		ServiceOrg:      myOrg,
		ServiceArch:     cutil.ArchString(),
		ServiceVersions: []exchange.WorkloadChoice{wc},
		DataVerify:      exchange.DataVerification{},
		NodeH:           exchange.NodeHealth{},
	}

	mURL := "http://utest.com/mservice"
	mVersion := "1.0.0"
	mArch := cutil.ArchString()
	sResolver := getVariableServiceDefResolver(mURL, myOrg, mVersion, mArch, nil)
	sHandler := getVariableServiceHandler(exchange.UserInput{})

	patternHandler := getVariablePatternHandler(sref)
	errHandled, cfg, msgs := UpdateConfigstate(cs, errorhandler, patternHandler, sResolver, sHandler, getDummyDeviceHandler(), getDummyPatchDeviceHandler(), db, getBasicConfig())

	if errHandled {
		t.Errorf("unexpected error %v", myError)
	} else if myError != nil && len(myError.Error()) != 0 {
		t.Errorf("myError set unexpectedly (%T) %v", myError, myError)
	} else if cfg == nil {
		t.Errorf("no configstate returned")
	} else if *cfg.State != persistence.CONFIGSTATE_CONFIGURED {
		t.Errorf("wrong state field %v", *cfg)
	} else if *cfg.LastUpdateTime == uint64(0) {
		t.Errorf("last update time should be set, is %v", *cfg)
	} else if len(msgs) != 2 {
		t.Errorf("there should be 2 message, received %v", len(msgs))
	}

	state = persistence.CONFIGSTATE_CONFIGURING
	cs.State = &state

	errHandled, cfg, _ = UpdateConfigstate(cs, errorhandler, patternHandler, sResolver, sHandler, getDummyDeviceHandler(), getDummyPatchDeviceHandler(), db, getBasicConfig())

	if !errHandled {
		t.Errorf("expected error")
	} else if apiErr, ok := myError.(*APIUserInputError); !ok {
		t.Errorf("myError has the wrong type (%T)", myError)
	} else if apiErr.Input != "configstate.state" {
		t.Errorf("wrong error input field %v", *apiErr)
	} else if cfg != nil {
		t.Errorf("configstate should not be returned")
	}

}

// no change in state - configured to configured
func Test_UpdateConfigstate_no_state_change_services(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	cs := getBasicConfigstate()
	state := persistence.CONFIGSTATE_CONFIGURED
	cs.State = &state

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)

	myOrg := "myorg"
	myPattern := "mypattern"

	_, err = persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", "", false, myOrg, myPattern, persistence.CONFIGSTATE_CONFIGURING)
	if err != nil {
		t.Errorf("failed to create persisted device, error %v", err)
	}

	wc := exchange.WorkloadChoice{
		Version:                      "1.0.0",
		Priority:                     exchange.WorkloadPriority{},
		Upgrade:                      exchange.UpgradePolicy{},
		DeploymentOverrides:          "",
		DeploymentOverridesSignature: "",
	}

	sref := exchange.ServiceReference{
		ServiceURL:      "wurl",
		ServiceOrg:      myOrg,
		ServiceArch:     cutil.ArchString(),
		ServiceVersions: []exchange.WorkloadChoice{wc},
		DataVerify:      exchange.DataVerification{},
		NodeH:           exchange.NodeHealth{},
	}

	mURL := "http://utest.com/mservice"
	mVersion := "1.0.0"
	mArch := cutil.ArchString()
	sResolver := getVariableServiceDefResolver(mURL, myOrg, mVersion, mArch, nil)
	sHandler := getVariableServiceHandler(exchange.UserInput{})

	patternHandler := getVariablePatternHandler(sref)
	errHandled, cfg, msgs := UpdateConfigstate(cs, errorhandler, patternHandler, sResolver, sHandler, getDummyDeviceHandler(), getDummyPatchDeviceHandler(), db, getBasicConfig())

	if errHandled {
		t.Errorf("unexpected error %v", myError)
	} else if myError != nil && len(myError.Error()) != 0 {
		t.Errorf("myError set unexpectedly (%T) %v", myError, myError)
	} else if cfg == nil {
		t.Errorf("no configstate returned")
	} else if *cfg.State != persistence.CONFIGSTATE_CONFIGURED {
		t.Errorf("wrong state field %v", *cfg)
	} else if *cfg.LastUpdateTime == uint64(0) {
		t.Errorf("last update time should be set, is %v", *cfg)
	} else if len(msgs) != 2 {
		t.Errorf("there should be 2 messages, received %v", len(msgs))
	}

	errHandled, cfg, msgs = UpdateConfigstate(cs, errorhandler, patternHandler, getDummyServiceDefResolver(), getDummyServiceHandler(), getDummyDeviceHandler(), getDummyPatchDeviceHandler(), db, getBasicConfig())

	if errHandled {
		t.Errorf("unexpected error %v", myError)
	} else if myError != nil && len(myError.Error()) != 0 {
		t.Errorf("myError set unexpectedly (%T) %v", myError, myError)
	} else if cfg == nil {
		t.Errorf("no configstate returned")
	} else if *cfg.State != persistence.CONFIGSTATE_CONFIGURED {
		t.Errorf("wrong state field %v", *cfg)
	} else if *cfg.LastUpdateTime == uint64(0) {
		t.Errorf("last update time should be set, is %v", *cfg)
	} else if len(msgs) != 0 {
		t.Errorf("there should be 0 messages, received %v", len(msgs))
	}

}

// change state to configured with services
func Test_UpdateConfigstateWith_services(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	cs := getBasicConfigstate()
	state := persistence.CONFIGSTATE_CONFIGURED
	cs.State = &state

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)

	myOrg := "myorg"
	myPattern := "mypattern"

	_, err = persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", "device", false, myOrg, myPattern, persistence.CONFIGSTATE_CONFIGURING)
	if err != nil {
		t.Errorf("failed to create persisted device, error %v", err)
	}

	sHandler := getVariableServiceHandler(exchange.UserInput{})
	sr := exchange.ServiceReference{
		ServiceURL:  "http://mydomain.com/workload/test1",
		ServiceOrg:  "testorg",
		ServiceArch: "amd64",
		ServiceVersions: []exchange.WorkloadChoice{
			{
				Version: "1.0.0",
			},
		},
	}
	patternHandler := getVariablePatternHandler(sr)

	mURL := "http://utest.com/mservice"
	mVersion := "1.0.0"
	mArch := "amd64"
	sResolver := getVariableServiceDefResolver(mURL, myOrg, mVersion, mArch, nil)

	errHandled, cfg, msgs := UpdateConfigstate(cs, errorhandler, patternHandler, sResolver, sHandler, getDummyDeviceHandler(), getDummyPatchDeviceHandler(), db, getBasicConfig())

	if errHandled {
		t.Errorf("unexpected error %v", myError)
	} else if myError != nil && len(myError.Error()) != 0 {
		t.Errorf("myError set unexpectedly (%T) %v", myError, myError)
	} else if cfg == nil {
		t.Errorf("no configstate returned")
	} else if *cfg.State != persistence.CONFIGSTATE_CONFIGURED {
		t.Errorf("wrong state field %v", *cfg)
	} else if *cfg.LastUpdateTime == uint64(0) {
		t.Errorf("last update time should be set, is %v", *cfg)
	} else if len(msgs) != 2 {
		t.Errorf("there should be 2 messages, received %v", len(msgs))
	}

	cleanTestDir(getBasicConfig().Edge.PolicyPath + "/" + myOrg)

}

// change state with a pattern that has a service which requires config, error results.
func Test_UpdateConfigstate_unconfig_services(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	cs := getBasicConfigstate()
	state := persistence.CONFIGSTATE_CONFIGURED
	cs.State = &state

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)

	theOrg := "myorg"
	thePattern := "apattern"

	_, err = persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", "", false, theOrg, thePattern, persistence.CONFIGSTATE_CONFIGURING)
	if err != nil {
		t.Errorf("failed to create persisted device, error %v", err)
	}

	missingVarName := "missingVar"
	ui := exchange.UserInput{
		Name:         missingVarName,
		Label:        "label",
		Type:         "string",
		DefaultValue: "",
	}
	mURL := "http://utest.com/mservice"
	mVersion := "1.0.0"
	mArch := "amd64"
	sHandler := getVariableServiceHandler(ui)
	sr := exchange.ServiceReference{
		ServiceURL:  "http://mydomain.com/workload/test1",
		ServiceOrg:  "testorg",
		ServiceArch: "amd64",
		ServiceVersions: []exchange.WorkloadChoice{
			{
				Version: "1.0.0",
			},
		},
	}
	patternHandler := getVariablePatternHandler(sr)
	sResolver := getVariableServiceDefResolver(mURL, theOrg, mVersion, mArch, nil)

	errHandled, cfg, _ := UpdateConfigstate(cs, errorhandler, patternHandler, sResolver, sHandler, getDummyDeviceHandler(), getDummyPatchDeviceHandler(), db, getBasicConfig())

	if !errHandled {
		t.Errorf("expected error")
	} else if apiErr, ok := myError.(*APIUserInputError); !ok {
		t.Errorf("myError has the wrong type (%T)", myError)
	} else if apiErr.Input != "configstate.state" {
		t.Errorf("wrong error input field %v", *apiErr)
	} else if !strings.Contains(apiErr.Err, missingVarName) {
		t.Errorf("wrong error reason, is %v", apiErr.Err)
	} else if cfg != nil {
		t.Errorf("configstate should not be returned")
	}

}

// change state with a pattern that has a top-level service which requires config, error results.
func Test_UpdateConfigstate_unconfig_top_level_services(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	cs := getBasicConfigstate()
	state := persistence.CONFIGSTATE_CONFIGURED
	cs.State = &state

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)

	theOrg := "myorg"
	thePattern := "apattern"

	_, err = persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", "device", false, theOrg, thePattern, persistence.CONFIGSTATE_CONFIGURING)
	if err != nil {
		t.Errorf("failed to create persisted device, error %v", err)
	}

	missingVarName := "missingVar"
	ui := exchange.UserInput{
		Name:         missingVarName,
		Label:        "label",
		Type:         "string",
		DefaultValue: "",
	}
	mURL := "http://utest.com/mservice"
	mVersion := "1.0.0"
	mArch := "amd64"
	sHandler := getVariableServiceHandler(exchange.UserInput{})
	sr := exchange.ServiceReference{
		ServiceURL:  "http://mydomain.com/workload/test1",
		ServiceOrg:  "testorg",
		ServiceArch: "amd64",
		ServiceVersions: []exchange.WorkloadChoice{
			{
				Version: "1.0.0",
			},
		},
	}
	patternHandler := getVariablePatternHandler(sr)
	sResolver := getVariableServiceDefResolver(mURL, theOrg, mVersion, mArch, &ui)

	errHandled, cfg, _ := UpdateConfigstate(cs, errorhandler, patternHandler, sResolver, sHandler, getDummyDeviceHandler(), getDummyPatchDeviceHandler(), db, getBasicConfig())

	if !errHandled {
		t.Errorf("expected error")
	} else if apiErr, ok := myError.(*MSMissingVariableConfigError); !ok {
		t.Errorf("myError has the wrong type (%T)", myError)
	} else if apiErr.Input != "configstate.state" {
		t.Errorf("wrong error input field %v", *apiErr)
	} else if !strings.Contains(apiErr.Err, "missing") {
		t.Errorf("wrong error reason, is %v", apiErr.Err)
	} else if cfg != nil {
		t.Errorf("configstate should not be returned")
	}

}

func getBasicConfigstate() *Configstate {
	state := persistence.CONFIGSTATE_CONFIGURING
	cs := &Configstate{
		State: &state,
	}
	return cs
}

func getSingleOrgHandler(org string, id string, token string) (*exchange.Organization, error) {
	o := &exchange.Organization{
		Label:       "label",
		Description: "desc",
		LastUpdated: "today",
	}
	return o, nil
}
