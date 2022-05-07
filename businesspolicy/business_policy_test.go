//go:build unit
// +build unit

package businesspolicy

import (
	"github.com/open-horizon/anax/externalpolicy"
	_ "github.com/open-horizon/anax/externalpolicy/text_language"
	"github.com/open-horizon/anax/policy"
	"strings"
	"testing"
)

// empty service def
func Test_Validate_Failed1(t *testing.T) {

	propList := new(externalpolicy.PropertyList)
	propList.Add_Property(externalpolicy.Property_Factory("prop1", "val1"), false)
	propList.Add_Property(externalpolicy.Property_Factory("prop2", "val2"), false)

	service := ServiceRef{}

	bPolicy := BusinessPolicy{
		Owner:       "me",
		Label:       "my business policy",
		Description: "blah",
		Service:     service,
		Properties:  *propList,
		Constraints: []string{"prop3 == val3"},
	}

	if err := bPolicy.Validate(); err == nil {
		t.Errorf("Validate should have returned error but not.")
	} else if !strings.Contains(err.Error(), "Name, or Org is empty string") {
		t.Errorf("Wrong error string: %v", err)
	}

	service = ServiceRef{
		Name:            "cpu",
		ServiceVersions: []WorkloadChoice{},
	}
	bPolicy.Service = service
	if err := bPolicy.Validate(); err == nil {
		t.Errorf("Validate should have returned error but not.")
	} else if !strings.Contains(err.Error(), "Name, or Org is empty string") {
		t.Errorf("Wrong error string: %v", err)
	}
}

// empty service versions
func Test_Validate_Failed2(t *testing.T) {

	propList := new(externalpolicy.PropertyList)
	propList.Add_Property(externalpolicy.Property_Factory("prop1", "val1"), false)
	propList.Add_Property(externalpolicy.Property_Factory("prop2", "val2"), false)

	service := ServiceRef{
		Name:            "cpu",
		Org:             "mycomp",
		Arch:            "amd64",
		ServiceVersions: []WorkloadChoice{},
	}

	bPolicy := BusinessPolicy{
		Owner:       "me",
		Label:       "my business policy",
		Description: "blah",
		Service:     service,
		Properties:  *propList,
		Constraints: []string{"prop3 == val3"},
	}

	if err := bPolicy.Validate(); err == nil {
		t.Errorf("Validate should have returned error but not.")
	} else if !strings.Contains(err.Error(), "The serviceVersions array is empty") {
		t.Errorf("Wrong error string: %v", err)
	}
}

// good one
func Test_Validate_Succeeded1(t *testing.T) {

	propList := new(externalpolicy.PropertyList)
	propList.Add_Property(externalpolicy.Property_Factory("prop1", "val1"), false)
	propList.Add_Property(externalpolicy.Property_Factory("prop2", "val2"), false)

	wlc := WorkloadChoice{
		Version: "1.00.%4",
		Priority: WorkloadPriority{
			PriorityValue:     50,
			Retries:           1,
			RetryDurationS:    3600,
			VerifiedDurationS: 52,
		},
		Upgrade: UpgradePolicy{
			Lifecycle: "immediate",
			Time:      "01.00AM",
		},
	}
	nh := NodeHealth{
		MissingHBInterval:    600,
		CheckAgreementStatus: 120,
	}
	service := ServiceRef{
		Name:            "cpu",
		Org:             "mycomp",
		Arch:            "amd64",
		ServiceVersions: []WorkloadChoice{wlc},
		NodeH:           nh,
	}

	bPolicy := BusinessPolicy{
		Owner:       "me",
		Label:       "my business policy",
		Description: "blah",
		Service:     service,
		Properties:  *propList,
		Constraints: []string{`prop3 == "val3"`},
	}

	if err := bPolicy.Validate(); err != nil {
		t.Errorf("Validate should have not have returned error but got: %v", err)
	}
}

// good one - missing Priority, Upgrade and NodeHealth
func Test_Validate_Succeeded2(t *testing.T) {

	propList := new(externalpolicy.PropertyList)
	propList.Add_Property(externalpolicy.Property_Factory("prop1", "val1"), false)
	propList.Add_Property(externalpolicy.Property_Factory("prop2", "val2"), false)

	wlc := WorkloadChoice{
		Version: "1.00.%4",
	}
	service := ServiceRef{
		Name:            "cpu",
		Org:             "mycomp",
		Arch:            "amd64",
		ServiceVersions: []WorkloadChoice{wlc},
	}

	bPolicy := BusinessPolicy{
		Owner:       "me",
		Label:       "my business policy",
		Description: "blah",
		Service:     service,
		Properties:  *propList,
		Constraints: []string{"prop3 == val3"},
	}

	if err := bPolicy.Validate(); err != nil {
		t.Errorf("Validate should have not have returned error but got: %v", err)
	}
}

// good one - missing properties
func Test_Validate_Succeeded3(t *testing.T) {
	wlc := WorkloadChoice{
		Version: "1.00.%4",
	}
	service := ServiceRef{
		Name:            "cpu",
		Org:             "mycomp",
		Arch:            "amd64",
		ServiceVersions: []WorkloadChoice{wlc},
	}

	bPolicy := BusinessPolicy{
		Owner:       "me",
		Label:       "my business policy",
		Description: "blah",
		Service:     service,
		Constraints: []string{"prop3 == val3"},
	}

	if err := bPolicy.Validate(); err != nil {
		t.Errorf("Validate should have not have returned error but got: %v", err)
	}
}

// good one - missing constraints
func Test_Validate_Succeeded4(t *testing.T) {

	propList := new(externalpolicy.PropertyList)
	propList.Add_Property(externalpolicy.Property_Factory("prop1", "val1"), false)
	propList.Add_Property(externalpolicy.Property_Factory("prop2", "val2"), false)

	wlc := WorkloadChoice{
		Version: "1.00.%4",
	}
	service := ServiceRef{
		Name:            "cpu",
		Org:             "mycomp",
		Arch:            "amd64",
		ServiceVersions: []WorkloadChoice{wlc},
	}

	bPolicy := BusinessPolicy{
		Owner:       "me",
		Label:       "my business policy",
		Description: "blah",
		Service:     service,
		Properties:  *propList,
	}

	if err := bPolicy.Validate(); err != nil {
		t.Errorf("Validate should have not have returned error but got: %v", err)
	}
}

func Test_GenPolicyFromBusinessPolicy_Simple(t *testing.T) {

	wlc := WorkloadChoice{
		Version: "1.00.%4",
	}
	service := ServiceRef{
		Name:            "cpu",
		Org:             "mycomp",
		Arch:            "amd64",
		ServiceVersions: []WorkloadChoice{wlc},
	}

	bPolicy := BusinessPolicy{
		Owner:       "me",
		Label:       "my business policy",
		Description: "blah",
		Service:     service,
	}

	if err := bPolicy.Validate(); err != nil {
		t.Errorf("Validate should have not have returned error but got: %v", err)
	}

	pPolicy, err := bPolicy.GenPolicyFromBusinessPolicy("mypolicy")
	if err != nil {
		t.Errorf("GenPolicyFromBusinessPolicy should have not have returned error but got: %v", err)
	} else if pPolicy == nil {
		t.Errorf("pPolicy should not be nil but it is nil")
	}
}

func Test_GenPolicyFromBusinessPolicy_Complicated(t *testing.T) {

	propList := new(externalpolicy.PropertyList)
	propList.Add_Property(externalpolicy.Property_Factory("prop1", "val1"), false)
	propList.Add_Property(externalpolicy.Property_Factory("prop2", "val2"), false)

	wlc := WorkloadChoice{
		Version: "1.00.%4",
		Priority: WorkloadPriority{
			PriorityValue:     50,
			Retries:           1,
			RetryDurationS:    3600,
			VerifiedDurationS: 52,
		},
		Upgrade: UpgradePolicy{
			Lifecycle: "immediate",
			Time:      "01.00AM",
		},
	}
	nh := NodeHealth{
		MissingHBInterval:    600,
		CheckAgreementStatus: 120,
	}
	service := ServiceRef{
		Name:            "cpu",
		Org:             "mycomp",
		Arch:            "amd64",
		ServiceVersions: []WorkloadChoice{wlc},
		NodeH:           nh,
	}

	svcUserInput1 := policy.UserInput{
		ServiceOrgid:        "mycomp",
		ServiceUrl:          "cpu",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
		Inputs:              []policy.Input{policy.Input{Name: "var1", Value: "val1"}, policy.Input{Name: "var2", Value: "val2"}},
	}

	bPolicy := BusinessPolicy{
		Owner:       "me",
		Label:       "my business policy",
		Description: "blah",
		Service:     service,
		Properties:  *propList,
		Constraints: []string{"prop3 == val3"},
		UserInput:   []policy.UserInput{svcUserInput1},
	}

	if err := bPolicy.Validate(); err != nil {
		t.Errorf("Validate should have not have returned error but got: %v", err)
	}

	pPolicy, err := bPolicy.GenPolicyFromBusinessPolicy("mypolicy")
	if err != nil {
		t.Errorf("GenPolicyFromBusinessPolicy should have not have returned error but got: %v", err)
	} else if pPolicy == nil {
		t.Errorf("pPolicy should not be nil but it is nil")
	} else if len(pPolicy.Properties) != 2 {
		t.Errorf("Policy properties should have 2 elements but got %v", len(pPolicy.Properties))
	} else if pPolicy.Workloads[0].WorkloadURL != bPolicy.Service.Name || pPolicy.Workloads[0].Org != bPolicy.Service.Org || pPolicy.Workloads[0].Arch != bPolicy.Service.Arch {
		t.Errorf("Workloads for policy is wrong: %v", pPolicy.Workloads)
	} else if pPolicy.NodeH.MissingHBInterval != bPolicy.Service.NodeH.MissingHBInterval || pPolicy.NodeH.CheckAgreementStatus != bPolicy.Service.NodeH.CheckAgreementStatus {
		t.Errorf("NodeHealth for policy is wrong: %v", pPolicy.NodeH)
	} else if pPolicy.Workloads[0].Version != bPolicy.Service.ServiceVersions[0].Version {
		t.Errorf("Service version for policy is wrong: %v", pPolicy.Workloads[0].Version)
	} else if pPolicy.Workloads[0].Priority.PriorityValue != bPolicy.Service.ServiceVersions[0].Priority.PriorityValue || pPolicy.Workloads[0].Priority.Retries != bPolicy.Service.ServiceVersions[0].Priority.Retries ||
		pPolicy.Workloads[0].Priority.RetryDurationS != bPolicy.Service.ServiceVersions[0].Priority.RetryDurationS || pPolicy.Workloads[0].Priority.VerifiedDurationS != bPolicy.Service.ServiceVersions[0].Priority.VerifiedDurationS {
		t.Errorf("Service priority for policy is wrong: %v", pPolicy.Workloads[0].Priority)
	} else if len(pPolicy.UserInput) != 1 {
		t.Errorf("UserInput should have 1 element but got %v.", len(pPolicy.UserInput))
	} else if len(pPolicy.UserInput[0].Inputs) != 2 {
		t.Errorf("UserInput for sevice cpu should have 2 variable but got %v.", len(pPolicy.UserInput[0].Inputs))
	} else if pPolicy.UserInput[0].ServiceUrl != "cpu" {
		t.Errorf("First UserInput should be for service cpu but got %v.", pPolicy.UserInput[0].ServiceUrl)
	} else if pPolicy.UserInput[0].Inputs[0].Name != "var1" {
		t.Errorf("First user input variable name for service cpu should be var1 but got %v.", pPolicy.UserInput[0].Inputs[0].Name)
	} else if pPolicy.UserInput[0].Inputs[1].Value != "val2" {
		t.Errorf("Second user input variable value for service cpu should be val2 but got %v.", pPolicy.UserInput[0].Inputs[1].Value)
	}
}
