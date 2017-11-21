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
	} else if *cfg.State != CONFIGSTATE_UNCONFIGURED {
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

	_, err = persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", false, theOrg, "apattern", CONFIGSTATE_CONFIGURING)
	if err != nil {
		t.Errorf("failed to create persisted device, error %v", err)
	}

	if cfg, err := FindConfigstateForOutput(db); err != nil {
		t.Errorf("failed to find device in db, error %v", err)
	} else if *cfg.State != CONFIGSTATE_CONFIGURING {
		t.Errorf("incorrect configstate, found: %v", *cfg)
	} else if *cfg.LastUpdateTime == uint64(0) {
		t.Errorf("incorrect last update time, found: %v", *cfg)
	}

}

// no change in state - confguring to configuring
func Test_UpdateConfigstate1(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	cs := getBasicConfigstate()

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)

	theOrg := "myorg"

	_, err = persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", false, theOrg, "apattern", CONFIGSTATE_CONFIGURING)
	if err != nil {
		t.Errorf("failed to create persisted device, error %v", err)
	}

	errHandled, cfg, _ := UpdateConfigstate(cs, errorhandler, getDummyGetOrg(), getDummyMicroserviceHandler(), getDummyGetPatterns(), getDummyWorkloadResolver(), db, getBasicConfig())

	if errHandled {
		t.Errorf("unexpected error %v", myError)
	} else if myError != nil && len(myError.Error()) != 0 {
		t.Errorf("myError set unexpectedly (%T) %v", myError, myError)
	} else if cfg == nil {
		t.Errorf("no configstate returned")
	} else if *cfg.State != CONFIGSTATE_CONFIGURING {
		t.Errorf("wrong state field %v", *cfg)
	} else if *cfg.LastUpdateTime == uint64(0) {
		t.Errorf("last update time should be set, is %v", *cfg)
	}

}

// change state to configured
func Test_UpdateConfigstate2(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	cs := getBasicConfigstate()
	state := CONFIGSTATE_CONFIGURED
	cs.State = &state

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)

	myOrg := "myorg"
	myPattern := "mypattern"

	_, err = persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", false, myOrg, myPattern, CONFIGSTATE_CONFIGURING)
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

	wref := exchange.WorkloadReference{
		WorkloadURL:      "wurl",
		WorkloadOrg:      myOrg,
		WorkloadArch:     cutil.ArchString(),
		WorkloadVersions: []exchange.WorkloadChoice{wc},
		DataVerify:       exchange.DataVerification{},
		NodeH:            exchange.NodeHealth{},
	}

	mURL := "http://utest.com/mservice"
	mVersion := "1.0.0"
	mArch := cutil.ArchString()
	wlResolver := getVariableWorkloadResolver(mURL, myOrg, mVersion, mArch, nil)

	patternHandler := getVariablePatternHandler(wref)
	errHandled, cfg, _ := UpdateConfigstate(cs, errorhandler, getDummyGetOrg(), getVariableMicroserviceHandler(exchange.UserInput{}), patternHandler, wlResolver, db, getBasicConfig())

	if errHandled {
		t.Errorf("unexpected error %v", myError)
	} else if myError != nil && len(myError.Error()) != 0 {
		t.Errorf("myError set unexpectedly (%T) %v", myError, myError)
	} else if cfg == nil {
		t.Errorf("no configstate returned")
	} else if *cfg.State != CONFIGSTATE_CONFIGURED {
		t.Errorf("wrong state field %v", *cfg)
	} else if *cfg.LastUpdateTime == uint64(0) {
		t.Errorf("last update time should be set, is %v", *cfg)
	}

}

// change state to from configured to configuring
func Test_UpdateConfigstate_Illegal_state_change(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	cs := getBasicConfigstate()
	state := CONFIGSTATE_CONFIGURED
	cs.State = &state

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)

	myOrg := "myorg"
	myPattern := "mypattern"

	_, err = persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", false, myOrg, myPattern, CONFIGSTATE_CONFIGURING)
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

	wref := exchange.WorkloadReference{
		WorkloadURL:      "wurl",
		WorkloadOrg:      myOrg,
		WorkloadArch:     cutil.ArchString(),
		WorkloadVersions: []exchange.WorkloadChoice{wc},
		DataVerify:       exchange.DataVerification{},
		NodeH:            exchange.NodeHealth{},
	}

	mURL := "http://utest.com/mservice"
	mVersion := "1.0.0"
	mArch := cutil.ArchString()
	wlResolver := getVariableWorkloadResolver(mURL, myOrg, mVersion, mArch, nil)

	patternHandler := getVariablePatternHandler(wref)
	errHandled, cfg, _ := UpdateConfigstate(cs, errorhandler, getDummyGetOrg(), getVariableMicroserviceHandler(exchange.UserInput{}), patternHandler, wlResolver, db, getBasicConfig())

	if errHandled {
		t.Errorf("unexpected error %v", myError)
	} else if myError != nil && len(myError.Error()) != 0 {
		t.Errorf("myError set unexpectedly (%T) %v", myError, myError)
	} else if cfg == nil {
		t.Errorf("no configstate returned")
	} else if *cfg.State != CONFIGSTATE_CONFIGURED {
		t.Errorf("wrong state field %v", *cfg)
	} else if *cfg.LastUpdateTime == uint64(0) {
		t.Errorf("last update time should be set, is %v", *cfg)
	}

	state = CONFIGSTATE_CONFIGURING
	cs.State = &state

	errHandled, cfg, _ = UpdateConfigstate(cs, errorhandler, getDummyGetOrg(), getDummyMicroserviceHandler(), patternHandler, getDummyWorkloadResolver(), db, getBasicConfig())

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
func Test_UpdateConfigstate_no_state_change(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	cs := getBasicConfigstate()
	state := CONFIGSTATE_CONFIGURED
	cs.State = &state

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)

	myOrg := "myorg"
	myPattern := "mypattern"

	_, err = persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", false, myOrg, myPattern, CONFIGSTATE_CONFIGURING)
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

	wref := exchange.WorkloadReference{
		WorkloadURL:      "wurl",
		WorkloadOrg:      myOrg,
		WorkloadArch:     cutil.ArchString(),
		WorkloadVersions: []exchange.WorkloadChoice{wc},
		DataVerify:       exchange.DataVerification{},
		NodeH:            exchange.NodeHealth{},
	}

	mURL := "http://utest.com/mservice"
	mVersion := "1.0.0"
	mArch := cutil.ArchString()
	wlResolver := getVariableWorkloadResolver(mURL, myOrg, mVersion, mArch, nil)

	patternHandler := getVariablePatternHandler(wref)
	errHandled, cfg, _ := UpdateConfigstate(cs, errorhandler, getDummyGetOrg(), getVariableMicroserviceHandler(exchange.UserInput{}), patternHandler, wlResolver, db, getBasicConfig())

	if errHandled {
		t.Errorf("unexpected error %v", myError)
	} else if myError != nil && len(myError.Error()) != 0 {
		t.Errorf("myError set unexpectedly (%T) %v", myError, myError)
	} else if cfg == nil {
		t.Errorf("no configstate returned")
	} else if *cfg.State != CONFIGSTATE_CONFIGURED {
		t.Errorf("wrong state field %v", *cfg)
	} else if *cfg.LastUpdateTime == uint64(0) {
		t.Errorf("last update time should be set, is %v", *cfg)
	}

	errHandled, cfg, _ = UpdateConfigstate(cs, errorhandler, getDummyGetOrg(), getDummyMicroserviceHandler(), patternHandler, getDummyWorkloadResolver(), db, getBasicConfig())

	if errHandled {
		t.Errorf("unexpected error %v", myError)
	} else if myError != nil && len(myError.Error()) != 0 {
		t.Errorf("myError set unexpectedly (%T) %v", myError, myError)
	} else if cfg == nil {
		t.Errorf("no configstate returned")
	} else if *cfg.State != CONFIGSTATE_CONFIGURED {
		t.Errorf("wrong state field %v", *cfg)
	} else if *cfg.LastUpdateTime == uint64(0) {
		t.Errorf("last update time should be set, is %v", *cfg)
	}

}

// change state to unsupported state
func Test_UpdateConfigstate_unknown_state_change(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	cs := getBasicConfigstate()
	state := "blah"
	cs.State = &state

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)

	theOrg := "myorg"

	_, err = persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", false, theOrg, "apattern", CONFIGSTATE_CONFIGURING)
	if err != nil {
		t.Errorf("failed to create persisted device, error %v", err)
	}

	errHandled, cfg, _ := UpdateConfigstate(cs, errorhandler, getDummyGetOrg(), getDummyMicroserviceHandler(), getDummyGetPatterns(), getDummyWorkloadResolver(), db, getBasicConfig())

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

// change state to configured with microservices
func Test_UpdateConfigstateWithMS(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	cs := getBasicConfigstate()
	state := CONFIGSTATE_CONFIGURED
	cs.State = &state

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)

	myOrg := "myorg"
	myPattern := "mypattern"

	_, err = persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", false, myOrg, myPattern, CONFIGSTATE_CONFIGURING)
	if err != nil {
		t.Errorf("failed to create persisted device, error %v", err)
	}

	msHandler := getVariableMicroserviceHandler(exchange.UserInput{})
	wr := exchange.WorkloadReference{
		WorkloadURL:  "http://mydomain.com/workload/test1",
		WorkloadOrg:  "testorg",
		WorkloadArch: "amd64",
		WorkloadVersions: []exchange.WorkloadChoice{
			{
				Version: "1.0.0",
			},
		},
	}
	patternHandler := getVariablePatternHandler(wr)

	mURL := "http://utest.com/mservice"
	mVersion := "1.0.0"
	mArch := "amd64"
	wlResolver := getVariableWorkloadResolver(mURL, myOrg, mVersion, mArch, nil)

	errHandled, cfg, _ := UpdateConfigstate(cs, errorhandler, getSingleOrgHandler, msHandler, patternHandler, wlResolver, db, getBasicConfig())

	if errHandled {
		t.Errorf("unexpected error %v", myError)
	} else if myError != nil && len(myError.Error()) != 0 {
		t.Errorf("myError set unexpectedly (%T) %v", myError, myError)
	} else if cfg == nil {
		t.Errorf("no configstate returned")
	} else if *cfg.State != CONFIGSTATE_CONFIGURED {
		t.Errorf("wrong state field %v", *cfg)
	} else if *cfg.LastUpdateTime == uint64(0) {
		t.Errorf("last update time should be set, is %v", *cfg)
	}

	cleanTestDir(getBasicConfig().Edge.PolicyPath + "/" + myOrg)

}

// change state with a pattern that has an MS which requires config, error results.
func Test_UpdateConfigstate_unconfig_ms(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	cs := getBasicConfigstate()
	state := CONFIGSTATE_CONFIGURED
	cs.State = &state

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)

	theOrg := "myorg"
	thePattern := "apattern"

	_, err = persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", false, theOrg, thePattern, CONFIGSTATE_CONFIGURING)
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
	msHandler := getVariableMicroserviceHandler(ui)
	wr := exchange.WorkloadReference{
		WorkloadURL:  "http://mydomain.com/workload/test1",
		WorkloadOrg:  "testorg",
		WorkloadArch: "amd64",
		WorkloadVersions: []exchange.WorkloadChoice{
			{
				Version: "1.0.0",
			},
		},
	}
	patternHandler := getVariablePatternHandler(wr)
	wlResolver := getVariableWorkloadResolver(mURL, theOrg, mVersion, mArch, nil)

	errHandled, cfg, _ := UpdateConfigstate(cs, errorhandler, getSingleOrgHandler, msHandler, patternHandler, wlResolver, db, getBasicConfig())

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

// change state with a pattern that has a workload which requires config, error results.
func Test_UpdateConfigstate_unconfig_workload(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	cs := getBasicConfigstate()
	state := CONFIGSTATE_CONFIGURED
	cs.State = &state

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)

	theOrg := "myorg"
	thePattern := "apattern"

	_, err = persistence.SaveNewExchangeDevice(db, "testid", "testtoken", "testname", false, theOrg, thePattern, CONFIGSTATE_CONFIGURING)
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
	msHandler := getVariableMicroserviceHandler(exchange.UserInput{})
	wr := exchange.WorkloadReference{
		WorkloadURL:  "http://mydomain.com/workload/test1",
		WorkloadOrg:  "testorg",
		WorkloadArch: "amd64",
		WorkloadVersions: []exchange.WorkloadChoice{
			{
				Version: "1.0.0",
			},
		},
	}
	patternHandler := getVariablePatternHandler(wr)
	wlResolver := getVariableWorkloadResolver(mURL, theOrg, mVersion, mArch, &ui)

	errHandled, cfg, _ := UpdateConfigstate(cs, errorhandler, getSingleOrgHandler, msHandler, patternHandler, wlResolver, db, getBasicConfig())

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
	state := CONFIGSTATE_CONFIGURING
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
