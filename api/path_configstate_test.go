// +build unit

package api

import (
	"flag"
	"fmt"
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

func Test_FindCSForOutput0(t *testing.T) {

	dir, db, err := setup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	if cfg, err := FindConfigstateForOutput(db); err != nil {
		t.Errorf("failed to find device in db, error %v", err)
	} else if *cfg.State != CONFIGSTATE_CONFIGURING {
		t.Errorf("incorrect configstate found: %v", *cfg)
	}

}

// Create output configstate object based on object in the DB
func Test_FindCSForOutput1(t *testing.T) {

	dir, db, err := setup()
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

	dir, db, err := setup()
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

	dir, db, err := setup()
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

	errHandled, cfg, _ := UpdateConfigstate(cs, errorhandler, getDummyGetOrg(), getDummyMicroserviceHandler(), getGoodPattern, getDummyWorkloadResolver(), db, getBasicConfig())

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

	dir, db, err := setup()
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

	errHandled, cfg, _ := UpdateConfigstate(cs, errorhandler, getDummyGetOrg(), getDummyMicroserviceHandler(), getGoodPattern, getDummyWorkloadResolver(), db, getBasicConfig())

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

	errHandled, cfg, _ = UpdateConfigstate(cs, errorhandler, getDummyGetOrg(), getDummyMicroserviceHandler(), getGoodPattern, getDummyWorkloadResolver(), db, getBasicConfig())

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

	dir, db, err := setup()
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

	errHandled, cfg, _ := UpdateConfigstate(cs, errorhandler, getDummyGetOrg(), getDummyMicroserviceHandler(), getGoodPattern, getDummyWorkloadResolver(), db, getBasicConfig())

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

	errHandled, cfg, _ = UpdateConfigstate(cs, errorhandler, getDummyGetOrg(), getDummyMicroserviceHandler(), getGoodPattern, getDummyWorkloadResolver(), db, getBasicConfig())

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

	dir, db, err := setup()
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

	dir, db, err := setup()
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

	errHandled, cfg, _ := UpdateConfigstate(cs, errorhandler, getSingleOrgHandler, getSingleMicroserviceHandler, getFullPattern, getSingleWorkloadResolver, db, getBasicConfig())

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

func getBasicConfigstate() *Configstate {
	state := CONFIGSTATE_CONFIGURING
	cs := &Configstate{
		State: &state,
	}
	return cs
}

func getGoodPattern(org string, pattern string, id string, token string) (map[string]exchange.Pattern, error) {
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

}

func getFullPattern(org string, pattern string, id string, token string) (map[string]exchange.Pattern, error) {
	patid := fmt.Sprintf("%v/%v", org, pattern)
	return map[string]exchange.Pattern{
		patid: exchange.Pattern{
			Label:       "label",
			Description: "desc",
			Public:      true,
			Workloads: []exchange.WorkloadReference{
				{
					WorkloadURL:  "http://mydomain.com/workload/test1",
					WorkloadOrg:  "testorg",
					WorkloadArch: "amd64",
					WorkloadVersions: []exchange.WorkloadChoice{
						{
							Version: "1.0.0",
						},
					},
				},
			},
			AgreementProtocols: []exchange.AgreementProtocol{},
		},
	}, nil

}

func getSingleWorkloadResolver(wUrl string, wOrg string, wVersion string, wArch string, id string, token string) (*policy.APISpecList, error) {
	sl := policy.APISpecList{
		policy.APISpecification{
			SpecRef:         wUrl,
			Org:             wOrg,
			Version:         wVersion,
			ExclusiveAccess: true,
			Arch:            wArch,
		},
	}
	return &sl, nil
}

func getSingleMicroserviceHandler(mUrl string, mOrg string, mVersion string, mArch string, id string, token string) (*exchange.MicroserviceDefinition, error) {
	md := exchange.MicroserviceDefinition{
		Owner:         "owner",
		Label:         "label",
		Description:   "desc",
		SpecRef:       mUrl,
		Version:       mVersion,
		Arch:          mArch,
		Sharable:      exchange.MS_SHARING_MODE_EXCLUSIVE,
		DownloadURL:   "",
		MatchHardware: exchange.HardwareMatch{},
		UserInputs:    []exchange.UserInput{},
		Workloads:     []exchange.WorkloadDeployment{},
		LastUpdated:   "today",
	}
	return &md, nil
}

func getSingleOrgHandler(org string, id string, token string) (*exchange.Organization, error) {
	o := &exchange.Organization{
		Label:       "label",
		Description: "desc",
		LastUpdated: "today",
	}
	return o, nil
}
