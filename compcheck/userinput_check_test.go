// +build unit

package compcheck

import (
	"github.com/open-horizon/anax/common"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/policy"
	"strings"
	"testing"
)

// test starts here
func Test_CheckRedundantUserinput(t *testing.T) {
	s1 := ServiceDefinition{
		"mycomp1",
		exchange.ServiceDefinition{
			URL:     "cpu1",
			Version: "1.0.0",
			Arch:    "amd64",
			UserInputs: []exchangecommon.UserInput{
				exchangecommon.UserInput{
					Name:         "var1",
					Type:         "string",
					DefaultValue: "",
				},
				exchangecommon.UserInput{
					Name:         "var2",
					Type:         "string",
					DefaultValue: "",
				},
				exchangecommon.UserInput{
					Name:         "var3",
					Type:         "string",
					DefaultValue: "",
				},
			},
		},
	}
	s2 := ServiceDefinition{
		"mycomp2",
		exchange.ServiceDefinition{
			URL:     "cpu2",
			Version: "2.0.0",
			Arch:    "amd64",
			UserInputs: []exchangecommon.UserInput{
				exchangecommon.UserInput{
					Name:         "var21",
					Type:         "string",
					DefaultValue: "",
				},
				exchangecommon.UserInput{
					Name:         "var22",
					Type:         "string",
					DefaultValue: "",
				},
			},
		},
	}

	svcUserInput1 := policy.UserInput{
		ServiceOrgid:        "mycomp1",
		ServiceUrl:          "cpu1",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
		Inputs:              []policy.Input{policy.Input{Name: "var1", Value: "val1"}, policy.Input{Name: "var2", Value: "val2"}},
	}

	svcUserInput2 := policy.UserInput{
		ServiceOrgid:        "mycomp2",
		ServiceUrl:          "cpu2",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
		Inputs:              []policy.Input{policy.Input{Name: "var21", Value: "val21"}, policy.Input{Name: "var22", Value: "val22"}},
	}
	services := []common.AbstractServiceFile{&s1, &s2}

	// good case
	ui1 := []policy.UserInput{svcUserInput1, svcUserInput2}
	if err := CheckRedundantUserinput(services, nil, ui1, nil); err != nil {
		t.Errorf("CheckRedundantUserinput should have returned nil but got %v", err)
	}

	// redundant variable name
	svcUserInput2.Inputs = []policy.Input{policy.Input{Name: "var21", Value: "val21"}, policy.Input{Name: "var22", Value: "val22"}, policy.Input{Name: "var23", Value: "val23"}}
	ui2 := []policy.UserInput{svcUserInput1, svcUserInput2}
	if err := CheckRedundantUserinput(services, nil, ui2, nil); err == nil {
		t.Errorf("CheckRedundantUserinput should not have returned nil")
	} else if !strings.Contains(err.Error(), "Variable var23") {
		t.Errorf("CheckRedundantUserinput returned wrong error: %v", err)
	}

	// redundant service
	svcUserInput3 := policy.UserInput{
		ServiceOrgid:        "mycomp2",
		ServiceUrl:          "cpu3",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
		Inputs:              []policy.Input{policy.Input{Name: "var21", Value: "val21"}, policy.Input{Name: "var22", Value: "val22"}},
	}
	ui3 := []policy.UserInput{svcUserInput1, svcUserInput3}
	if err := CheckRedundantUserinput(services, nil, ui3, nil); err == nil {
		t.Errorf("CheckRedundantUserinput should not have returned nil")
	} else if !strings.Contains(err.Error(), "is not referenced by the pattern or deployment policy") {
		t.Errorf("CheckRedundantUserinput returned wrong error: %v", err)
	}

	// no user input for a service, but the service is specified in the ui
	svcUserInput4 := policy.UserInput{
		ServiceOrgid:        "mycomp2",
		ServiceUrl:          "cpu2",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
	}
	ui4 := []policy.UserInput{svcUserInput1, svcUserInput4}
	if err := CheckRedundantUserinput(services, nil, ui4, nil); err != nil {
		t.Errorf("CheckRedundantUserinput should have returned nil but got %v", err)
	}

	// no user input at all for a service
	ui5 := []policy.UserInput{svcUserInput4}
	if err := CheckRedundantUserinput(services, nil, ui5, nil); err != nil {
		t.Errorf("CheckRedundantUserinput should have returned nil but got %v", err)
	}
}
