// +build unit

package exchange

import (
	"errors"
	"flag"
	"github.com/open-horizon/anax/exchangecommon"
	"testing"
)

func TestServiceString1(t *testing.T) {
	s := ServiceDefinition{
		Owner:                      "testOwner",
		Label:                      "service def",
		Description:                "a test",
		Public:                     false,
		URL:                        "http://test.company.com/service1",
		Version:                    "1.0.0",
		Arch:                       "amd64",
		Sharable:                   exchangecommon.SERVICE_SHARING_MODE_SINGLETON,
		MatchHardware:              HardwareRequirement{},
		RequiredServices:           []exchangecommon.ServiceDependency{},
		UserInputs:                 []exchangecommon.UserInput{},
		Deployment:                 `{"services":{}}`,
		DeploymentSignature:        "xyzpdq=",
		ClusterDeployment:          `{}`,
		ClusterDeploymentSignature: "abcdef=",
		LastUpdated:                "today",
	}
	str := s.String()
	t.Log(str)

	expected := `Owner: testOwner, Label: service def, Description: a test, Public: false, URL: http://test.company.com/service1, Version: 1.0.0, Arch: amd64, Sharable: singleton, MatchHardware: none, RequiredServices: [], UserInputs: [], Deployment: {"services":{}}, DeploymentSignature: xyzpdq=, ClusterDeployment: {}, ClusterDeploymentSignature: abcdef=, LastUpdated: today`
	if str != expected {
		t.Errorf("String() output expected: %v", expected)
	}
}

func TestServiceString2(t *testing.T) {
	s := ServiceDefinition{
		Owner:       "testOwner",
		Label:       "service def",
		Description: "a test",
		Public:      false,
		URL:         "http://test.company.com/service1",
		Version:     "1.0.0",
		Arch:        "amd64",
		Sharable:    exchangecommon.SERVICE_SHARING_MODE_SINGLETON,
		MatchHardware: HardwareRequirement{
			"dev": "/dev/dev1",
		},
		RequiredServices: []exchangecommon.ServiceDependency{
			exchangecommon.ServiceDependency{
				URL:     "http://my.com/ms/ms1",
				Org:     "otherOrg",
				Version: "1.5.0",
				Arch:    "amd64",
			},
			exchangecommon.ServiceDependency{
				URL:     "http://my.com/ms/ms2",
				Org:     "otherOrg",
				Version: "2.7",
				Arch:    "amd64",
			},
		},
		UserInputs: []exchangecommon.UserInput{
			exchangecommon.UserInput{
				Name:         "name",
				Label:        "a ui",
				Type:         "string",
				DefaultValue: "",
			},
			exchangecommon.UserInput{
				Name:         "name2",
				Label:        "another ui",
				Type:         "string",
				DefaultValue: "three",
			},
		},
		Deployment:                 `{"services":{}}`,
		DeploymentSignature:        "xyzpdq=",
		ClusterDeployment:          `{}`,
		ClusterDeploymentSignature: "abcdef=",
		LastUpdated:                "today",
	}
	str := s.String()
	t.Log(str)

	expected := `Owner: testOwner, Label: service def, Description: a test, Public: false, URL: http://test.company.com/service1, Version: 1.0.0, Arch: amd64, Sharable: singleton, MatchHardware: {dev:/dev/dev1}, RequiredServices: [{URL: http://my.com/ms/ms1, Org: otherOrg, Version: 1.5.0, VersionRange: , Arch: amd64} {URL: http://my.com/ms/ms2, Org: otherOrg, Version: 2.7, VersionRange: , Arch: amd64}], UserInputs: [{Name: name, :Label: a ui, Type: string, DefaultValue: } {Name: name2, :Label: another ui, Type: string, DefaultValue: three}], Deployment: {"services":{}}, DeploymentSignature: xyzpdq=, ClusterDeployment: {}, ClusterDeploymentSignature: abcdef=, LastUpdated: today`
	if str != expected {
		t.Errorf("String() output expected: %v", expected)
	}

}

func TestServiceString3(t *testing.T) {
	s := ServiceDefinition{
		Owner:       "testOwner",
		Label:       "service def",
		Description: "a test",
		Public:      false,
		URL:         "http://test.company.com/service1",
		Version:     "1.0.0",
		Arch:        "amd64",
		Sharable:    exchangecommon.SERVICE_SHARING_MODE_SINGLETON,
		MatchHardware: HardwareRequirement{
			"dev": "/dev/dev1",
		},
		RequiredServices: []exchangecommon.ServiceDependency{
			exchangecommon.ServiceDependency{
				URL:     "http://my.com/ms/ms1",
				Org:     "otherOrg",
				Version: "1.5.0",
				Arch:    "amd64",
			},
			exchangecommon.ServiceDependency{
				URL:     "http://my.com/ms/ms2",
				Org:     "otherOrg",
				Version: "2.7",
				Arch:    "amd64",
			},
		},
		UserInputs: []exchangecommon.UserInput{
			exchangecommon.UserInput{
				Name:         "name",
				Label:        "a ui",
				Type:         "string",
				DefaultValue: "",
			},
			exchangecommon.UserInput{
				Name:         "name2",
				Label:        "another ui",
				Type:         "string",
				DefaultValue: "three",
			},
		},
		Deployment:                 `{"services":{}}`,
		DeploymentSignature:        "xyzpdq=",
		ClusterDeployment:          `{}`,
		ClusterDeploymentSignature: "abcdef=",
		LastUpdated:                "today",
	}
	str := s.ShortString()
	t.Log(str)

	expected := `URL: http://test.company.com/service1, Version: 1.0.0, Arch: amd64, RequiredServices: [{URL: http://my.com/ms/ms1, Org: otherOrg, Version: 1.5.0, VersionRange: , Arch: amd64} {URL: http://my.com/ms/ms2, Org: otherOrg, Version: 2.7, VersionRange: , Arch: amd64}]`
	if str != expected {
		t.Errorf("ShortString() output expected: %v", expected)
	}

}

// Test when user input is found and not found.
func TestService_GetUserInputByName(t *testing.T) {

	targetName := "name"
	targetLabel := "a ui"

	s := ServiceDefinition{
		Owner:       "testOwner",
		Label:       "service def",
		Description: "a test",
		Public:      false,
		URL:         "http://test.company.com/service1",
		Version:     "1.0.0",
		Arch:        "amd64",
		Sharable:    exchangecommon.SERVICE_SHARING_MODE_SINGLETON,
		MatchHardware: HardwareRequirement{
			"dev": "/dev/dev1",
		},
		RequiredServices: []exchangecommon.ServiceDependency{
			exchangecommon.ServiceDependency{
				URL:     "http://my.com/ms/ms1",
				Org:     "otherOrg",
				Version: "1.5.0",
				Arch:    "amd64",
			},
			exchangecommon.ServiceDependency{
				URL:     "http://my.com/ms/ms2",
				Org:     "otherOrg",
				Version: "2.7",
				Arch:    "amd64",
			},
		},
		UserInputs: []exchangecommon.UserInput{
			exchangecommon.UserInput{
				Name:         targetName,
				Label:        targetLabel,
				Type:         "string",
				DefaultValue: "",
			},
			exchangecommon.UserInput{
				Name:         "name2",
				Label:        "another ui",
				Type:         "string",
				DefaultValue: "three",
			},
		},
		Deployment:          `{"services":{}}`,
		DeploymentSignature: "xyzpdq=",
		LastUpdated:         "today",
	}

	// Test when name is found
	ui := s.GetUserInputName(targetName)

	if ui == nil {
		t.Errorf("A userinput should have been returned")
	} else if ui.Name != targetName {
		t.Errorf("The userinput with name %v should have been returned, was %v", targetName, *ui)
	} else if ui.Label != targetLabel {
		t.Errorf("The userinput with label %v should have been returned, was %v", targetLabel, *ui)
	}

	// Test when name is not found
	ui = s.GetUserInputName("foobar")

	if ui != nil {
		t.Errorf("A userinput should NOT have been returned.")
	}
}

// Needs user input
func TestService_NeedsUserInput1(t *testing.T) {

	s := ServiceDefinition{
		Owner:       "testOwner",
		Label:       "service def",
		Description: "a test",
		Public:      false,
		URL:         "http://test.company.com/service1",
		Version:     "1.0.0",
		Arch:        "amd64",
		Sharable:    exchangecommon.SERVICE_SHARING_MODE_SINGLETON,
		MatchHardware: HardwareRequirement{
			"dev": "/dev/dev1",
		},
		RequiredServices: []exchangecommon.ServiceDependency{
			exchangecommon.ServiceDependency{
				URL:     "http://my.com/ms/ms1",
				Org:     "otherOrg",
				Version: "1.5.0",
				Arch:    "amd64",
			},
			exchangecommon.ServiceDependency{
				URL:     "http://my.com/ms/ms2",
				Org:     "otherOrg",
				Version: "2.7",
				Arch:    "amd64",
			},
		},
		UserInputs: []exchangecommon.UserInput{
			exchangecommon.UserInput{
				Name:         "name",
				Label:        "a ui",
				Type:         "string",
				DefaultValue: "",
			},
			exchangecommon.UserInput{
				Name:         "name2",
				Label:        "another ui",
				Type:         "string",
				DefaultValue: "three",
			},
		},
		Deployment:          `{"services":{}}`,
		DeploymentSignature: "xyzpdq=",
		LastUpdated:         "today",
	}

	// Test when user input is needed
	need := s.NeedsUserInput()

	if need == false {
		t.Errorf("There is a non-default user input field, so input is needed.")
	}

}

// Does not need user input
func TestService_NeedsUserInput2(t *testing.T) {

	s := ServiceDefinition{
		Owner:       "testOwner",
		Label:       "service def",
		Description: "a test",
		Public:      false,
		URL:         "http://test.company.com/service1",
		Version:     "1.0.0",
		Arch:        "amd64",
		Sharable:    exchangecommon.SERVICE_SHARING_MODE_SINGLETON,
		MatchHardware: HardwareRequirement{
			"dev": "/dev/dev1",
		},
		RequiredServices: []exchangecommon.ServiceDependency{
			exchangecommon.ServiceDependency{
				URL:     "http://my.com/ms/ms1",
				Org:     "otherOrg",
				Version: "1.5.0",
				Arch:    "amd64",
			},
			exchangecommon.ServiceDependency{
				URL:     "http://my.com/ms/ms2",
				Org:     "otherOrg",
				Version: "2.7",
				Arch:    "amd64",
			},
		},
		UserInputs: []exchangecommon.UserInput{
			exchangecommon.UserInput{
				Name:         "name",
				Label:        "a ui",
				Type:         "string",
				DefaultValue: "four",
			},
			exchangecommon.UserInput{
				Name:         "name2",
				Label:        "another ui",
				Type:         "string",
				DefaultValue: "three",
			},
		},
		Deployment:          `{"services":{}}`,
		DeploymentSignature: "xyzpdq=",
		LastUpdated:         "today",
	}

	// Test when user input is needed
	need := s.NeedsUserInput()

	if need == true {
		t.Errorf("All user input fields have a default, so no input is needed.")
	}

}

func TestService_PopulateDefaultUserInput(t *testing.T) {

	targetValue := "four"
	targetName := "name"

	s := ServiceDefinition{
		Owner:       "testOwner",
		Label:       "service def",
		Description: "a test",
		Public:      false,
		URL:         "http://test.company.com/service1",
		Version:     "1.0.0",
		Arch:        "amd64",
		Sharable:    exchangecommon.SERVICE_SHARING_MODE_SINGLETON,
		MatchHardware: HardwareRequirement{
			"dev": "/dev/dev1",
		},
		RequiredServices: []exchangecommon.ServiceDependency{
			exchangecommon.ServiceDependency{
				URL:     "http://my.com/ms/ms1",
				Org:     "otherOrg",
				Version: "1.5.0",
				Arch:    "amd64",
			},
			exchangecommon.ServiceDependency{
				URL:     "http://my.com/ms/ms2",
				Org:     "otherOrg",
				Version: "2.7",
				Arch:    "amd64",
			},
		},
		UserInputs: []exchangecommon.UserInput{
			exchangecommon.UserInput{
				Name:         targetName,
				Label:        "a ui",
				Type:         "string",
				DefaultValue: targetValue,
			},
			exchangecommon.UserInput{
				Name:         "name2",
				Label:        "another ui",
				Type:         "string",
				DefaultValue: "",
			},
		},
		Deployment:          `{"services":{}}`,
		DeploymentSignature: "xyzpdq=",
		LastUpdated:         "today",
	}

	envAdds := make(map[string]string)
	s.PopulateDefaultUserInput(envAdds)

	if len(envAdds) != 1 {
		t.Errorf("Should have populated 1 entry in the map. Map is %v", envAdds)
	} else if val, ok := envAdds[targetName]; !ok {
		t.Errorf("Should have entry for %v, have %v", targetName, envAdds)
	} else if val != targetValue {
		t.Errorf("Should have value of %v, have %v", targetValue, envAdds)
	}

}

func TestService_GetDeployment(t *testing.T) {

	targetD := `{"services":{}}`
	targetDS := "xyzpdq="

	s := ServiceDefinition{
		Owner:       "testOwner",
		Label:       "service def",
		Description: "a test",
		Public:      false,
		URL:         "http://test.company.com/service1",
		Version:     "1.0.0",
		Arch:        "amd64",
		Sharable:    exchangecommon.SERVICE_SHARING_MODE_SINGLETON,
		MatchHardware: HardwareRequirement{
			"dev": "/dev/dev1",
		},
		RequiredServices: []exchangecommon.ServiceDependency{
			exchangecommon.ServiceDependency{
				URL:     "http://my.com/ms/ms1",
				Org:     "otherOrg",
				Version: "1.5.0",
				Arch:    "amd64",
			},
			exchangecommon.ServiceDependency{
				URL:     "http://my.com/ms/ms2",
				Org:     "otherOrg",
				Version: "2.7",
				Arch:    "amd64",
			},
		},
		UserInputs: []exchangecommon.UserInput{
			exchangecommon.UserInput{
				Name:         "name",
				Label:        "a ui",
				Type:         "string",
				DefaultValue: "four",
			},
			exchangecommon.UserInput{
				Name:         "name2",
				Label:        "another ui",
				Type:         "string",
				DefaultValue: "",
			},
		},
		Deployment:          targetD,
		DeploymentSignature: targetDS,
		LastUpdated:         "today",
	}

	d := s.GetDeploymentString()
	ds := s.GetDeploymentSignature()

	if d != targetD {
		t.Errorf("Returned Deployment should be %v, was %v", targetD, d)
	} else if ds != targetDS {
		t.Errorf("Returned DeploymentSig should be %v, was %v", targetDS, ds)
	}
}

func Test_GetSearchVersion(t *testing.T) {

	searchAllVersions := ""

	// Specific version
	vers := "1.2.3"
	sv, err := getSearchVersion(vers)

	if err != nil {
		t.Errorf("Returned error: %v", err)
	} else if sv != vers {
		t.Errorf("Returned %v, should have returned %v", sv, vers)
	}

	// Version Expression
	vers = "[1.2.3,4.5.6)"
	sv, err = getSearchVersion(vers)

	if err != nil {
		t.Errorf("Returned error: %v", err)
	} else if sv != searchAllVersions {
		t.Errorf("Returned %v, should have returned empty string", sv)
	}

	// No Version
	vers = ""
	sv, err = getSearchVersion(vers)

	if err != nil {
		t.Errorf("Returned error: %v", err)
	} else if sv != searchAllVersions {
		t.Errorf("Returned %v, should have returned empty string", sv)
	}

	// Invalid Version Expression
	vers = "[1.2.3)"
	_, err = getSearchVersion(vers)

	if err == nil {
		t.Errorf("Should have returned error")
	}

}

// Test the response handling for a GetService call to the exchange.

// Don't find the version being asked for.
func Test_GetServiceResponse_0specific(t *testing.T) {

	myURL := "http://service1"
	myOrg := "test"
	myVersion := "2.0.0"
	myArch := "amd64"

	// Specific version not found
	resp := &GetServicesResponse{
		Services:  map[string]ServiceDefinition{},
		LastIndex: 0,
	}
	searchVersion := myVersion
	if sDef, _, err := processGetServiceResponse(myURL, myOrg, myVersion, myArch, searchVersion, resp); err == nil {
		t.Errorf("Should have returned error: %v", err)
	} else if sDef != nil {
		t.Errorf("Should not have returned a service def: %v", sDef)
	}

}

// Return the single specific version being asked for.
func Test_GetServiceResponse_1specific(t *testing.T) {

	myURL := "http://service1"
	myOrg := "test"
	myVersion := "2.0.0"
	myArch := "amd64"

	sh := getVariableServiceHandler([]exchangecommon.UserInput{}, []exchangecommon.ServiceDependency{})
	sd, id, _ := sh(myURL, myOrg, myVersion, myArch)

	resp := &GetServicesResponse{
		Services: map[string]ServiceDefinition{
			id: *sd,
		},
		LastIndex: 0,
	}

	// Return a specific version
	searchVersion := myVersion
	if sDef, _, err := processGetServiceResponse(myURL, myOrg, myVersion, myArch, searchVersion, resp); err != nil {
		t.Errorf("Should not have returned error: %v", err)
	} else if sDef == nil {
		t.Errorf("Should not have returned nil service def")
	}

}

// Don't find the version range being asked for.
func Test_GetServiceResponse_0range(t *testing.T) {

	myURL := "http://service1"
	myOrg := "test"
	myVersion := "2.0.0"
	myArch := "amd64"

	// Version range not found
	resp := &GetServicesResponse{
		Services:  map[string]ServiceDefinition{},
		LastIndex: 0,
	}
	searchVersion := ""
	if sDef, _, err := processGetServiceResponse(myURL, myOrg, myVersion, myArch, searchVersion, resp); err == nil {
		t.Errorf("Should have returned error: %v", err)
	} else if sDef != nil {
		t.Errorf("Should not have returned a service def: %v", sDef)
	}

}

// There is 1 response that matches the open range being asked for
func Test_GetServiceResponse_1range_open(t *testing.T) {

	myURL := "http://service1"
	myOrg := "test"
	myVersion := "2.0.0"
	myArch := "amd64"

	// 1 in the version range
	sh := getVariableServiceHandler([]exchangecommon.UserInput{}, []exchangecommon.ServiceDependency{})
	sd, id, _ := sh(myURL, myOrg, myVersion, myArch)

	resp := &GetServicesResponse{
		Services: map[string]ServiceDefinition{
			id: *sd,
		},
		LastIndex: 0,
	}

	searchVersion := ""
	if sDef, _, err := processGetServiceResponse(myURL, myOrg, myVersion, myArch, searchVersion, resp); err != nil {
		t.Errorf("Should not have returned error: %v", err)
	} else if sDef == nil {
		t.Errorf("Should have returned a service def")
	} else if sDef.URL != myURL {
		t.Errorf("Should have returned %v, but was %v", *sd, *sDef)
	}

}

// There is 1 response but not in the specific range being asked for
func Test_GetServiceResponse_1range_specific_none(t *testing.T) {

	myURL := "http://service1"
	myOrg := "test"
	myVersion := "2.0.0"
	myArch := "amd64"

	callerMSRange := "[1.0.0,2.0.0)"

	// 1 not in the version range
	sh := getVariableServiceHandler([]exchangecommon.UserInput{}, []exchangecommon.ServiceDependency{})
	sd, id, _ := sh(myURL, myOrg, myVersion, myArch)

	resp := &GetServicesResponse{
		Services: map[string]ServiceDefinition{
			id: *sd,
		},
		LastIndex: 0,
	}

	searchVersion := ""
	if sDef, _, err := processGetServiceResponse(myURL, myOrg, callerMSRange, myArch, searchVersion, resp); err != nil {
		t.Errorf("Should not have returned error")
	} else if sDef != nil {
		t.Errorf("Should not have returned a service def: %v", sDef)
	}

}

// There is 1 response inside the specific range being asked for
func Test_GetServiceResponse_1range_specific_success(t *testing.T) {

	myURL := "http://service1"
	myOrg := "test"
	myVersion := "2.0.0"
	myArch := "amd64"

	callerMSRange := "[1.0.0,2.0.0]"

	// 1 in the version range
	sh := getVariableServiceHandler([]exchangecommon.UserInput{}, []exchangecommon.ServiceDependency{})
	sd, id, _ := sh(myURL, myOrg, myVersion, myArch)

	resp := &GetServicesResponse{
		Services: map[string]ServiceDefinition{
			id: *sd,
		},
		LastIndex: 0,
	}

	searchVersion := ""
	if sDef, _, err := processGetServiceResponse(myURL, myOrg, callerMSRange, myArch, searchVersion, resp); err != nil {
		t.Errorf("Should not have returned error: %v", err)
	} else if sDef == nil {
		t.Errorf("Should have returned a service def")
	} else if sDef.URL != myURL {
		t.Errorf("Should have returned %v, but was %v", *sd, *sDef)
	}

}

// There is 1 response but the specific range being asked for is invalid.
func Test_GetServiceResponse_1range_specific_error(t *testing.T) {

	myURL := "http://service1"
	myOrg := "test"
	myVersion := "2.0.0"
	myArch := "amd64"

	callerMSRange := "[1.0.0,a]"

	// 1 in the version range
	sh := getVariableServiceHandler([]exchangecommon.UserInput{}, []exchangecommon.ServiceDependency{})
	sd, id, _ := sh(myURL, myOrg, myVersion, myArch)

	resp := &GetServicesResponse{
		Services: map[string]ServiceDefinition{
			id: *sd,
		},
		LastIndex: 0,
	}

	searchVersion := ""
	if sDef, _, err := processGetServiceResponse(myURL, myOrg, callerMSRange, myArch, searchVersion, resp); err == nil {
		t.Errorf("Should have returned error")
	} else if sDef != nil {
		t.Errorf("Should not have returned a service def: %v", sDef)
	}

}

// There are 2 responses inside the specific range being asked for
func Test_GetServiceResponse_2range_specific_success(t *testing.T) {

	myURL := "http://service1"
	myOrg := "test"
	myVersion := "2.0.0"
	myArch := "amd64"

	callerMSRange := "[1.0.0,2.0.0]"

	// 1 in the version range
	sh := getVariableServiceHandler([]exchangecommon.UserInput{}, []exchangecommon.ServiceDependency{})
	sd1, _, _ := sh(myURL, myOrg, myVersion, myArch)
	sd2, _, _ := sh("http://service2", myOrg, "1.5.0", myArch)

	resp := &GetServicesResponse{
		Services: map[string]ServiceDefinition{
			"id1": *sd1,
			"id2": *sd2,
		},
		LastIndex: 0,
	}

	searchVersion := ""
	if sDef, _, err := processGetServiceResponse(myURL, myOrg, callerMSRange, myArch, searchVersion, resp); err != nil {
		t.Errorf("Should not have returned error: %v", err)
	} else if sDef == nil {
		t.Errorf("Should have returned a service def")
	} else if sDef.URL != myURL {
		t.Errorf("Should have returned %v, but was %v", *sd1, *sDef)
	}

}

// Resolve a service with no dependencies
func TestServiceResolver1(t *testing.T) {

	myURL := "http://service1"
	myOrg := "test"
	myVersion := "2.0.0"
	myArch := "amd64"

	sh := getVariableServiceHandler([]exchangecommon.UserInput{}, []exchangecommon.ServiceDependency{})
	apiSpecList, sd, _, err := ServiceResolver(myURL, myOrg, myVersion, myArch, sh)

	if err != nil {
		t.Errorf("received unexpected error: %v", err)
	} else if sd == nil {
		t.Errorf("received no service definition")
	} else if len(*apiSpecList) != 0 {
		t.Errorf("should have received empty api spec list: %v", apiSpecList)
	} else if sd.HasDependencies() {
		t.Errorf("should not have dependencies: %v", sd.RequiredServices)
	}

}

func TestServiceDefResolver1(t *testing.T) {

	myURL := "http://service1"
	myOrg := "test"
	myVersion := "2.0.0"
	myArch := "amd64"

	sh := getVariableServiceHandler([]exchangecommon.UserInput{}, []exchangecommon.ServiceDependency{})
	apiSpecs, service_map, sd, _, err := ServiceDefResolver(myURL, myOrg, myVersion, myArch, sh)

	if err != nil {
		t.Errorf("received unexpected error: %v", err)
	} else if sd == nil {
		t.Errorf("received no service definition")
	} else if len(service_map) != 0 {
		t.Errorf("should have received empty service map: %v", service_map)
	} else if sd.HasDependencies() {
		t.Errorf("should not have dependencies: %v", sd.RequiredServices)
	} else if len(*apiSpecs) != 0 {
		t.Errorf("should not have api spec list but got: %v", apiSpecs)
	}
}

// Resolve a service with 1 dependency
func TestServiceResolver2(t *testing.T) {

	myURL := "http://service1"
	myOrg := "test"
	myVersion := "2.0.0"
	myArch := "amd64"

	sDep := []exchangecommon.ServiceDependency{
		exchangecommon.ServiceDependency{
			URL:     "http://my.com/ms/ms1",
			Org:     "otherOrg",
			Version: "1.5.0",
			Arch:    "amd64",
		},
	}

	// Establish service dependencies that the mock service handler will provide.
	sdMap := make(map[string][]exchangecommon.ServiceDependency)
	sdMap[myURL] = sDep

	sh := getRecursiveVariableServiceHandler([]exchangecommon.UserInput{}, sdMap)
	apiSpecList, sd, _, err := ServiceResolver(myURL, myOrg, myVersion, myArch, sh)

	if err != nil {
		t.Errorf("received unexpected error: %v", err)
	} else if sd == nil {
		t.Errorf("received no service definition")
	} else if len(*apiSpecList) == 0 {
		t.Errorf("should not have received empty api spec list")
	} else if !sd.HasDependencies() {
		t.Errorf("should have dependencies")
	}

}

func TestServiceDefResolver2(t *testing.T) {

	myURL := "http://service1"
	myOrg := "test"
	myVersion := "2.0.0"
	myArch := "amd64"

	sDep := []exchangecommon.ServiceDependency{
		exchangecommon.ServiceDependency{
			URL:     "http://my.com/ms/ms1",
			Org:     "otherOrg",
			Version: "1.5.0",
			Arch:    "amd64",
		},
	}

	// Establish service dependencies that the mock service handler will provide.
	sdMap := make(map[string][]exchangecommon.ServiceDependency)
	sdMap[myURL] = sDep

	sh := getRecursiveVariableServiceHandler([]exchangecommon.UserInput{}, sdMap)
	apiSpecs, service_map, sd, _, err := ServiceDefResolver(myURL, myOrg, myVersion, myArch, sh)

	if err != nil {
		t.Errorf("received unexpected error: %v", err)
	} else if sd == nil {
		t.Errorf("received no service definition")
	} else if len(service_map) == 0 {
		t.Errorf("should not have received empty service map.")
	} else if !sd.HasDependencies() {
		t.Errorf("should have dependencies")
	} else if apiSpecs == nil {
		t.Errorf("apiSpecs should not be nil")
	} else if len(*apiSpecs) != 1 {
		t.Errorf("Should have 1 api spec but got %v.", len(*apiSpecs))
	} else {
		for _, a := range *apiSpecs {
			if a.SpecRef != "http://my.com/ms/ms1" || a.Org != "otherOrg" || a.Version != "1.5.0" || a.ExclusiveAccess != true {
				t.Errorf("Wrong apiSpecs returned: %v", a)
			}
		}
	}
}

func Test_RecursiveServiceResolver_1level(t *testing.T) {

	flag.Set("alsologtostderr", "true")
	flag.Set("v", "7")

	s_contains := func(s_array []string, elem string) bool {
		for _, e := range s_array {
			if e == elem {
				return true
			}
		}
		return false
	}

	myURL := "http://service1"
	myOrg := "test"
	myVersion := "1.0.0"
	myArch := "amd64"

	sDep := []exchangecommon.ServiceDependency{
		exchangecommon.ServiceDependency{
			URL:     "http://my.com/ms/ms1",
			Org:     "otherOrg",
			Version: "1.5.0",
			Arch:    "amd64",
		},
		exchangecommon.ServiceDependency{
			URL:     "http://my.com/ms/ms2",
			Org:     "thirdOrg",
			Version: "1.5.0",
			Arch:    "amd64",
		},
	}

	// Establish service dependencies that the mock service handler will provide.
	sdMap := make(map[string][]exchangecommon.ServiceDependency)
	sdMap[myURL] = sDep

	sh := getRecursiveVariableServiceHandler([]exchangecommon.UserInput{}, sdMap)

	// Test the resolver API
	apiSpecs, sd, sIds, err := ServiceResolver(myURL, myOrg, myVersion, myArch, sh)

	if err != nil {
		t.Errorf("should not have returned err: %v", err)
	} else if len(*apiSpecs) != len(sDep) {
		t.Errorf("there should be api specs returned")
	} else if sd == nil {
		t.Errorf("should have returned a service def")
	} else if sIds == nil || len(sIds) != 3 {
		t.Errorf("Wrong service ids returns: %v", sIds)
	} else if sIds[0] != myURL {
		t.Errorf("The first element of the service ids should be %v but got %v", myURL, sIds[0])
	} else if !s_contains(sIds, "http://my.com/ms/ms1") {
		t.Errorf("http://my.com/ms/ms1 should be in the service id array but not")
	} else if !s_contains(sIds, "http://my.com/ms/ms2") {
		t.Errorf("http://my.com/ms/ms2 should be in the service id array but not")
	}
}

func Test_RecursiveServiceDefResolver_1level(t *testing.T) {

	flag.Set("alsologtostderr", "true")
	flag.Set("v", "7")

	myURL := "http://service1"
	myOrg := "test"
	myVersion := "1.0.0"
	myArch := "amd64"

	sDep := []exchangecommon.ServiceDependency{
		exchangecommon.ServiceDependency{
			URL:     "http://my.com/ms/ms1",
			Org:     "otherOrg",
			Version: "1.5.0",
			Arch:    "amd64",
		},
		exchangecommon.ServiceDependency{
			URL:     "http://my.com/ms/ms2",
			Org:     "thirdOrg",
			Version: "1.5.0",
			Arch:    "amd64",
		},
	}

	// Establish service dependencies that the mock service handler will provide.
	sdMap := make(map[string][]exchangecommon.ServiceDependency)
	sdMap[myURL] = sDep

	sh := getRecursiveVariableServiceHandler([]exchangecommon.UserInput{}, sdMap)

	// Test the resolver API
	apiSpecs, service_map, sd, sId, err := ServiceDefResolver(myURL, myOrg, myVersion, myArch, sh)

	if err != nil {
		t.Errorf("should not have returned err: %v", err)
	} else if len(service_map) != len(sDep) {
		t.Errorf("there should be api specs returned")
	} else if sd == nil {
		t.Errorf("should have returned a service def")
	} else if sId != myURL {
		t.Errorf("The first element of the service ids should be %v but got %v", myURL, sId)
	} else if _, ok := service_map["http://my.com/ms/ms1"]; !ok {
		t.Errorf("http://my.com/ms/ms1 should be in the service map but not")
	} else if _, ok := service_map["http://my.com/ms/ms2"]; !ok {
		t.Errorf("http://my.com/ms/ms2 should be in the service map but not")
	} else if apiSpecs == nil {
		t.Errorf("apiSpecs should not be nil")
	} else if len(*apiSpecs) != 2 {
		t.Errorf("Should have 2 api spec but got %v.", len(*apiSpecs))
	} else {
		for _, a := range *apiSpecs {
			if a.SpecRef != "http://my.com/ms/ms1" && a.SpecRef != "http://my.com/ms/ms2" || a.ExclusiveAccess != true {
				t.Errorf("Wrong apiSpecs returned: %v", a)
			}
		}
	}
}

func Test_RecursiveServiceResolver_2level(t *testing.T) {

	flag.Set("alsologtostderr", "true")
	flag.Set("v", "7")

	myURL := "http://service1"
	myOrg := "test"
	myVersion := "1.0.0"
	myArch := "amd64"

	// Dependencies of top level service
	sDep1 := []exchangecommon.ServiceDependency{
		exchangecommon.ServiceDependency{
			URL:     "http://my.com/ms/ms1",
			Org:     "otherOrg",
			Version: "1.5.0",
			Arch:    "amd64",
		},
		exchangecommon.ServiceDependency{
			URL:     "http://my.com/ms/ms2",
			Org:     "thirdOrg",
			Version: "1.5.0",
			Arch:    "amd64",
		},
	}

	// Dependencies of top level dependency: ms1
	sDep21 := []exchangecommon.ServiceDependency{
		exchangecommon.ServiceDependency{
			URL:     "http://my.com/ms/msa",
			Org:     "otherOrg",
			Version: "2.7.0",
			Arch:    "amd64",
		},
		exchangecommon.ServiceDependency{
			URL:     "http://my.com/ms/msb",
			Org:     "otherOrg",
			Version: "1.0.0",
			Arch:    "amd64",
		},
	}

	// Dependencies of top level dependency: ms2
	sDep22 := []exchangecommon.ServiceDependency{
		exchangecommon.ServiceDependency{
			URL:     "http://my.com/ms/msx",
			Org:     "thirdOrg",
			Version: "2.7.0",
			Arch:    "amd64",
		},
		exchangecommon.ServiceDependency{
			URL:     "http://my.com/ms/msa",
			Org:     "otherOrg",
			Version: "2.0.0",
			Arch:    "amd64",
		},
	}

	// Establish service dependencies that the mock service handler will provide.
	sdMap := make(map[string][]exchangecommon.ServiceDependency)
	sdMap[myURL] = sDep1
	sdMap[sDep1[0].URL] = sDep21
	sdMap[sDep1[1].URL] = sDep22

	sh := getRecursiveVariableServiceHandler([]exchangecommon.UserInput{}, sdMap)

	// Test the resolver API
	apiSpecs, sd, _, err := ServiceResolver(myURL, myOrg, myVersion, myArch, sh)

	//number of unique API specs returned. -1 is applied because there is a dup ms1->msa and ms2->msa.
	num := len(sDep1) + len(sDep21) + len(sDep22) - 1

	if err != nil {
		t.Errorf("should not have returned err: %v", err)
	} else if len(*apiSpecs) != num {
		t.Errorf("there should %v api specs returned", num)
	} else if sd == nil {
		t.Errorf("should have returned a service def")
	}

}

func Test_RecursiveServiceDefResolver_2level(t *testing.T) {

	flag.Set("alsologtostderr", "true")
	flag.Set("v", "7")

	myURL := "http://service1"
	myOrg := "test"
	myVersion := "1.0.0"
	myArch := "amd64"

	// Dependencies of top level service
	sDep1 := []exchangecommon.ServiceDependency{
		exchangecommon.ServiceDependency{
			URL:     "http://my.com/ms/ms1",
			Org:     "otherOrg",
			Version: "1.5.0",
			Arch:    "amd64",
		},
		exchangecommon.ServiceDependency{
			URL:     "http://my.com/ms/ms2",
			Org:     "thirdOrg",
			Version: "1.5.0",
			Arch:    "amd64",
		},
	}

	// Dependencies of top level dependency: ms1
	sDep21 := []exchangecommon.ServiceDependency{
		exchangecommon.ServiceDependency{
			URL:     "http://my.com/ms/msa",
			Org:     "otherOrg",
			Version: "2.7.0",
			Arch:    "amd64",
		},
		exchangecommon.ServiceDependency{
			URL:     "http://my.com/ms/msb",
			Org:     "otherOrg",
			Version: "1.0.0",
			Arch:    "amd64",
		},
	}

	// Dependencies of top level dependency: ms2
	sDep22 := []exchangecommon.ServiceDependency{
		exchangecommon.ServiceDependency{
			URL:     "http://my.com/ms/msx",
			Org:     "thirdOrg",
			Version: "2.7.0",
			Arch:    "amd64",
		},
		exchangecommon.ServiceDependency{
			URL:     "http://my.com/ms/msa",
			Org:     "otherOrg",
			Version: "2.0.0",
			Arch:    "amd64",
		},
	}

	// Establish service dependencies that the mock service handler will provide.
	sdMap := make(map[string][]exchangecommon.ServiceDependency)
	sdMap[myURL] = sDep1
	sdMap[sDep1[0].URL] = sDep21
	sdMap[sDep1[1].URL] = sDep22

	sh := getRecursiveVariableServiceHandler([]exchangecommon.UserInput{}, sdMap)

	// Test the resolver API
	apiSpecs, service_map, sd, _, err := ServiceDefResolver(myURL, myOrg, myVersion, myArch, sh)

	//number of unique API specs returned. -1 is applied because there is a dup ms1->msa and ms2->msa.
	num := len(sDep1) + len(sDep21) + len(sDep22) - 1

	if err != nil {
		t.Errorf("should not have returned err: %v", err)
	} else if len(service_map) != num {
		t.Errorf("there should %v api specs returned", num)
	} else if sd == nil {
		t.Errorf("should have returned a service def")
	}

	if len(*apiSpecs) != num {
		t.Errorf("there should %v api specs returned", num)
	} else if sd == nil {
		t.Errorf("should have returned a service def")
	}
}

func getRecursiveVariableServiceHandler(mUserInput []exchangecommon.UserInput, mRequiredServices map[string][]exchangecommon.ServiceDependency) ServiceHandler {
	return func(mUrl string, mOrg string, mVersion string, mArch string) (*ServiceDefinition, string, error) {

		reqServ, ok := mRequiredServices[mUrl]
		if !ok {
			reqServ = []exchangecommon.ServiceDependency{}
		}

		md := ServiceDefinition{
			Owner:               "testOwner",
			Label:               "service def",
			Description:         "a test",
			Public:              false,
			URL:                 mUrl,
			Version:             mVersion,
			Arch:                mArch,
			Sharable:            exchangecommon.SERVICE_SHARING_MODE_EXCLUSIVE,
			MatchHardware:       HardwareRequirement{},
			RequiredServices:    reqServ,
			UserInputs:          mUserInput,
			Deployment:          `{"services":{}}`,
			DeploymentSignature: "xyzpdq=",
			LastUpdated:         "today",
		}
		return &md, mUrl, nil
	}
}

func getVariableServiceHandler(mUserInput []exchangecommon.UserInput, mRequiredServices []exchangecommon.ServiceDependency) ServiceHandler {
	return func(mUrl string, mOrg string, mVersion string, mArch string) (*ServiceDefinition, string, error) {
		md := ServiceDefinition{
			Owner:               "testOwner",
			Label:               "service def",
			Description:         "a test",
			Public:              false,
			URL:                 mUrl,
			Version:             mVersion,
			Arch:                mArch,
			Sharable:            exchangecommon.SERVICE_SHARING_MODE_EXCLUSIVE,
			MatchHardware:       HardwareRequirement{},
			RequiredServices:    mRequiredServices,
			UserInputs:          mUserInput,
			Deployment:          `{"services":{}}`,
			DeploymentSignature: "xyzpdq=",
			LastUpdated:         "today",
		}
		return &md, mUrl, nil
	}
}

func getErrorServiceHandler() ServiceHandler {
	return func(wUrl string, wOrg string, wVersion string, wArch string) (*ServiceDefinition, string, error) {
		return nil, "", errors.New("service error")
	}
}

func Test_ServiceSuspended(t *testing.T) {
	m1 := Microservice{
		Url:         "myorg1/http://servicename1",
		Version:     "1.0",
		ConfigState: "suspended",
	}
	m2 := Microservice{
		Url:         "myorg2/http://servicename2",
		Version:     "2.0",
		ConfigState: "active",
	}
	m3 := Microservice{
		Url:         "myname@myorg.com/http://servicename3",
		Version:     "3.0",
		ConfigState: "suspended",
	}

	found, suspended := ServiceSuspended([]Microservice{m1, m2, m3}, "http://servicename1", "myorg1", "")
	if !found || !suspended {
		t.Errorf("ServiceSuspended should have returned (true, true) but got (%v, %v)", found, suspended)
	}
	found, suspended = ServiceSuspended([]Microservice{m1, m2, m3}, "http://servicename2", "myorg2", "2.0")
	if !found || suspended {
		t.Errorf("ServiceSuspended should have returned (true, false) but got (%v, %v)", found, suspended)
	}
	found, suspended = ServiceSuspended([]Microservice{m1, m2, m3}, "http://servicename3", "myname@myorg.com", "3.0")
	if !found || !suspended {
		t.Errorf("ServiceSuspended should have returned (true, true) but got (%v, %v)", found, suspended)
	}
	found, suspended = ServiceSuspended([]Microservice{m1, m2, m3}, "http://servicename2", "myorg1", "")
	if found || suspended {
		t.Errorf("ServiceSuspended should have returned (true, true) but got (%v, %v)", found, suspended)
	}
}

func Test_Support_versionrange_0(t *testing.T) {

	sd1 := exchangecommon.ServiceDependency{
		URL:          "other",
		Org:          "test",
		Version:      "",
		VersionRange: "1.0.0",
		Arch:         "amd64",
	}

	sd2 := exchangecommon.ServiceDependency{
		URL:          "other",
		Org:          "test",
		Version:      "1.0.0",
		VersionRange: "",
		Arch:         "amd64",
	}

	s1 := ServiceDefinition{
		Owner:               "testOwner",
		Label:               "service def",
		Description:         "a test",
		Public:              false,
		URL:                 "test name",
		Version:             "1.0.0",
		Arch:                "amd64",
		Sharable:            exchangecommon.SERVICE_SHARING_MODE_EXCLUSIVE,
		MatchHardware:       HardwareRequirement{},
		RequiredServices:    []exchangecommon.ServiceDependency{sd1, sd2},
		UserInputs:          []exchangecommon.UserInput{},
		Deployment:          `{"services":{}}`,
		DeploymentSignature: "xyzpdq=",
		LastUpdated:         "today",
	}

	gsr := new(GetServicesResponse)
	gsr.Services = map[string]ServiceDefinition{
		"s1": s1,
	}

	gsr.SupportVersionRange()
	if gsr.Services["s1"].RequiredServices[0].Version != sd1.VersionRange {
		t.Errorf("Error, version range was not copied into version field: %v, should be %v", gsr.Services["s1"].RequiredServices[0].Version, sd1.VersionRange)
	} else if gsr.Services["s1"].RequiredServices[1].Version != sd2.Version {
		t.Errorf("Error, version range was copied into version field: %v, should be %v", gsr.Services["s1"].RequiredServices[1].Version, sd2.Version)
	}
}
