// +build unit

package compcheck

import (
	"fmt"
	"github.com/open-horizon/anax/businesspolicy"
	"github.com/open-horizon/anax/common"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	_ "github.com/open-horizon/anax/externalpolicy/text_language"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/policy"
	"strings"
	"testing"
)

const (
	COMPATIBLE   = "Compatible"
	INCOMPATIBLE = "Incompatible"
)

// test starts here
func Test_processBusinessPolicy(t *testing.T) {
	svcUrl := "weather"
	svcOrg := "myorg"
	svcVersion := "1.0.1"
	svcArch := "amd64"
	service := businesspolicy.ServiceRef{
		Name:            svcUrl,
		Org:             svcOrg,
		Arch:            svcArch,
		ServiceVersions: []businesspolicy.WorkloadChoice{businesspolicy.WorkloadChoice{Version: svcVersion}},
	}

	svcUrl1 := "fake"
	svcOrg1 := "fake org"
	svcArch1 := "fake arch"
	service1 := businesspolicy.ServiceRef{
		Name:            svcUrl1,
		Org:             svcOrg1,
		Arch:            svcArch1,
		ServiceVersions: []businesspolicy.WorkloadChoice{businesspolicy.WorkloadChoice{Version: svcVersion}},
	}

	bHandler := getBusinessPolicyHandler(service1, map[string]string{}, []string{})
	bHandler1 := getBusinessPolicyHandler(service1, map[string]string{"prop11": "val11"}, []string{"prop31 == val31"})
	businessPolicy := createBusinessPolicy(service, map[string]string{"prop1": "val1", "prop2": "val2"}, []string{"prop3 == val3", "prop4 == \"some value\""})

	// test pol with no convertion
	if bPolicy, pPolicy, err := processBusinessPolicy(bHandler, "", businessPolicy, false, nil); err != nil {
		t.Errorf("processBusinessPolicy should not have returned error but got: %v", err)
	} else if pPolicy != nil {
		t.Errorf("processBusinessPolicy should not have converted the policy but did: %v", pPolicy)
	} else if bPolicy == nil {
		t.Errorf("processBusinessPolicy should have returned a business policy but did not.")
	} else if bPolicy.Service.Name != svcUrl || bPolicy.Service.Org != svcOrg || bPolicy.Service.Arch != svcArch {
		t.Errorf("processBusinessPolicy have converted a wrong policy: %v", bPolicy)
	}

	// test pol with convertion
	if bPolicy, pPolicy, err := processBusinessPolicy(bHandler, "", businessPolicy, true, nil); err != nil {
		t.Errorf("processBusinessPolicy should not have returned error but got: %v", err)
	} else if pPolicy == nil {
		t.Errorf("processBusinessPolicy should have converted the internal policy but did not")
	} else if bPolicy == nil {
		t.Errorf("processBusinessPolicy should have returned a business policy but did not.")
	} else if bPolicy.Service.Name != svcUrl || bPolicy.Service.Org != svcOrg || bPolicy.Service.Arch != svcArch {
		t.Errorf("processBusinessPolicy have converted a wrong policy: %v", bPolicy)
	} else if len(pPolicy.Workloads) != 1 {
		t.Errorf("Internal policy have 1 workload but got %v", len(pPolicy.Workloads))
	} else if pPolicy.Workloads[0].WorkloadURL != svcUrl || pPolicy.Workloads[0].Org != svcOrg || pPolicy.Workloads[0].Arch != svcArch || pPolicy.Workloads[0].Version != svcVersion {
		t.Errorf("processBusinessPolicy has returned a wrong internal policy: %v", pPolicy)
	}

	// test no id, no pol
	if bPolicy, pPolicy, err := processBusinessPolicy(bHandler, "", nil, true, nil); err == nil {
		t.Errorf("processBusinessPolicy should have returned error but not")
	} else if !strings.Contains(err.Error(), "Neither deployment policy nor deployment policy id is specified.") {
		t.Errorf("processBusinessPolicy returned wrong error message: %v", err)
	} else if bPolicy != nil {
		t.Errorf("processBusinessPolicy have returned nul business policy but got: %v", bPolicy)
	} else if pPolicy != nil {
		t.Errorf("processBusinessPolicy have returned nul internal policy but got: %v", pPolicy)
	}
	if bPolicy, pPolicy, err := processBusinessPolicy(bHandler, "", nil, false, nil); err == nil {
		t.Errorf("processBusinessPolicy should have returned error but not")
	} else if !strings.Contains(err.Error(), "Neither deployment policy nor deployment policy id is specified.") {
		t.Errorf("processBusinessPolicy returned wrong error message: %v", err)
	} else if bPolicy != nil {
		t.Errorf("processBusinessPolicy have returned nul business policy but got: %v", bPolicy)
	} else if pPolicy != nil {
		t.Errorf("processBusinessPolicy have returned nul internal policy but got: %v", pPolicy)
	}

	// test id with no convertion
	if bPolicy, pPolicy, err := processBusinessPolicy(bHandler1, "myorg/mybp", nil, false, nil); err != nil {
		t.Errorf("processBusinessPolicy should not have returned error but got: %v", err)
	} else if pPolicy != nil {
		t.Errorf("processBusinessPolicy should not have converted the policy but did: %v", pPolicy)
	} else if bPolicy == nil {
		t.Errorf("processBusinessPolicy should have returned a business policy but did not.")
	} else if bPolicy.Service.Name != svcUrl1 || bPolicy.Service.Org != svcOrg1 || bPolicy.Service.Arch != svcArch1 {
		t.Errorf("processBusinessPolicy have converted a wrong policy: %v", bPolicy)
	} else if len(bPolicy.Properties) != 1 {
		t.Errorf("There should be 1 property in business policy but got: %v", len(bPolicy.Properties))
	} else if len(bPolicy.Constraints) != 1 {
		t.Errorf("There should be 1 contraints in business policy but got: %v", len(bPolicy.Constraints))
	}

	// test id with convertion
	if bPolicy, pPolicy, err := processBusinessPolicy(bHandler1, "myorg/mybp", nil, true, nil); err != nil {
		t.Errorf("processBusinessPolicy should not have returned error but got: %v", err)
	} else if pPolicy == nil {
		t.Errorf("processBusinessPolicy should have converted the internal policy but did not")
	} else if bPolicy == nil {
		t.Errorf("processBusinessPolicy should have returned a business policy but did not.")
	} else if bPolicy.Service.Name != svcUrl1 || bPolicy.Service.Org != svcOrg1 || bPolicy.Service.Arch != svcArch1 {
		t.Errorf("processBusinessPolicy have converted a wrong policy: %v", bPolicy)
	} else if len(pPolicy.Workloads) != 1 {
		t.Errorf("Internal policy have 1 workload but got %v", len(pPolicy.Workloads))
	} else if pPolicy.Workloads[0].WorkloadURL != svcUrl1 || pPolicy.Workloads[0].Org != svcOrg1 || pPolicy.Workloads[0].Arch != svcArch1 || pPolicy.Workloads[0].Version != svcVersion {
		t.Errorf("processBusinessPolicy has returned a wrong internal policy: %v", pPolicy)
	} else if len(bPolicy.Properties) != 1 {
		t.Errorf("There should be 1 property in business policy but got: %v", len(bPolicy.Properties))
	} else if len(bPolicy.Constraints) != 1 {
		t.Errorf("There should be 1 contraints in business policy but got: %v", len(bPolicy.Constraints))
	} else if len(pPolicy.Properties) != 1 {
		t.Errorf("There should be 1 property in internal policy but got: %v", len(pPolicy.Properties))
	} else if len(pPolicy.Constraints) != 1 {
		t.Errorf("There should be 1 contraints in internal policy but got: %v", len(pPolicy.Constraints))
	}

	// test id with convertion, bad handler
	if bPolicy, pPolicy, err := processBusinessPolicy(getBusinessPolicyHandler_Error(), "myorg/mybp", nil, true, nil); err == nil {
		t.Errorf("processBusinessPolicy should have returned error but not")
	} else if !strings.Contains(err.Error(), "error getting business policy for myorg/mybp") {
		t.Errorf("processBusinessPolicy returned wrong error message: %v", err)
	} else if bPolicy != nil {
		t.Errorf("processBusinessPolicy have returned nul business policy but got: %v", bPolicy)
	} else if pPolicy != nil {
		t.Errorf("processBusinessPolicy have returned nul internal policy but got: %v", pPolicy)
	}
}

func Test_processNodePolicy(t *testing.T) {
	// test with id
	if nPolicy, pPolicy, err := processNodePolicy(getNodePolicyHandler(map[string]string{"prop3": "val3", "prop4": "some value"}, []string{"prop1 == val1", "prop5 == val5"}), "myorg/mynode", nil, nil); err != nil {
		t.Errorf("processNodePolicy should not have returned error but got: %v", err)
	} else if pPolicy == nil {
		t.Errorf("processNodePolicy should have converted the internal policy but did not")
	} else if nPolicy == nil {
		t.Errorf("processNodePolicy should have return a node policy but not.")
	} else if len(nPolicy.Properties) != 2 {
		t.Errorf("There should be 2 property in node policy but got: %v", len(nPolicy.Properties))
	} else if len(nPolicy.Constraints) != 2 {
		t.Errorf("There should be 2 contraints in node policy but got: %v", len(nPolicy.Constraints))
	} else if len(pPolicy.Properties) != 2 {
		t.Errorf("There should be 2 property in internal policy but got: %v", len(pPolicy.Properties))
	} else if len(pPolicy.Constraints) != 2 {
		t.Errorf("There should be 2 contraints in internal policy but got: %v", len(pPolicy.Constraints))
	}

	// test with pol
	nodePolicy := createExternalPolicy(map[string]string{"prop3": "val3", "prop5": "val5", "prop6": "val6"}, []string{"prop4 == \"some value\""})
	if nPolicy, pPolicy, err := processNodePolicy(getNodePolicyHandler(map[string]string{"prop3": "val3", "prop4": "some value"}, []string{"prop1 == val1", "prop5 == val5"}), "", nodePolicy, nil); err != nil {
		t.Errorf("processNodePolicy should not have returned error but got: %v", err)
	} else if pPolicy == nil {
		t.Errorf("processNodePolicy should have converted the internal policy but did not")
	} else if nPolicy == nil {
		t.Errorf("processNodePolicy should have return a node policy but not.")
	} else if len(nPolicy.Properties) != 3 {
		t.Errorf("There should be 3 property in node policy but got: %v", len(nPolicy.Properties))
	} else if len(nPolicy.Constraints) != 1 {
		t.Errorf("There should be 1 contraints in node policy but got: %v", len(nPolicy.Constraints))
	} else if len(pPolicy.Properties) != 3 {
		t.Errorf("There should be 3 property in internal policy but got: %v", len(pPolicy.Properties))
	} else if len(pPolicy.Constraints) != 1 {
		t.Errorf("There should be 1 contraints in internal policy but got: %v", len(pPolicy.Constraints))
	}

	// test no id, no pol.
	if nPolicy, pPolicy, err := processNodePolicy(getNodePolicyHandler(map[string]string{"prop3": "val3", "prop4": "some value"}, []string{"prop1 == val1", "prop5 == val5"}), "", nil, nil); err == nil {
		t.Errorf("processNodePolicy should have returned error but not")
	} else if !strings.Contains(err.Error(), "Neither node policy nor node id is specified.") {
		t.Errorf("processNodePolicy returned wrong error message: %v", err)
	} else if nPolicy != nil {
		t.Errorf("processNodePolicy have returned nul business policy but got: %v", nPolicy)
	} else if pPolicy != nil {
		t.Errorf("processNodePolicy have returned nul internal policy but got: %v", pPolicy)
	}

	// test with id and pol
	if nPolicy, pPolicy, err := processNodePolicy(getNodePolicyHandler(map[string]string{"prop3": "val3", "prop4": "some value"}, []string{"prop1 == val1", "prop5 == val5"}), "myorg/mynode", nodePolicy, nil); err != nil {
		t.Errorf("processNodePolicy should not have returned error but got: %v", err)
	} else if pPolicy == nil {
		t.Errorf("processNodePolicy should have converted the internal policy but did not")
	} else if nPolicy == nil {
		t.Errorf("processNodePolicy should have return a node policy but not.")
	} else if len(nPolicy.Properties) != 3 {
		t.Errorf("There should be 3 property in node policy but got: %v", len(nPolicy.Properties))
	} else if len(nPolicy.Constraints) != 1 {
		t.Errorf("There should be 1 contraints in node policy but got: %v", len(nPolicy.Constraints))
	} else if len(pPolicy.Properties) != 3 {
		t.Errorf("There should be 3 property in internal policy but got: %v", len(pPolicy.Properties))
	} else if len(pPolicy.Constraints) != 1 {
		t.Errorf("There should be 1 contraints in internal policy but got: %v", len(pPolicy.Constraints))
	} else if pPolicy.Header.Name != "Policy for myorg/mynode" {
		t.Errorf("The internal policy header name should be myorg/mynode but got: %v", pPolicy.Header.Name)
	}
}

func Test_policyCompatible_with_IDs(t *testing.T) {

	msgPrinter := i18n.GetMessagePrinter()

	input := PolicyCheck{
		NodeId:         "myorg/mynode",
		NodeArch:       "",
		NodePolicy:     nil,
		BusinessPolId:  "myorg/mybp",
		BusinessPolicy: nil,
		ServicePolicy:  nil,
	}

	svcUrl := "weather"
	svcOrg := "myorg"
	svcVersion1 := "1.0.1"
	svcVersion2 := "2.0.1"
	svcArch := "amd64"
	service := businesspolicy.ServiceRef{
		Name:            svcUrl,
		Org:             svcOrg,
		Arch:            svcArch,
		ServiceVersions: []businesspolicy.WorkloadChoice{businesspolicy.WorkloadChoice{Version: svcVersion1}, businesspolicy.WorkloadChoice{Version: svcVersion2}},
	}
	sId1 := cutil.FormExchangeIdForService(svcUrl, svcVersion1, svcArch)
	sId1 = fmt.Sprintf("%v/%v", svcOrg, sId1)
	sId2 := cutil.FormExchangeIdForService(svcUrl, svcVersion2, svcArch)
	sId2 = fmt.Sprintf("%v/%v", svcOrg, sId2)

	// if checkAll is true, it returns compaitble entry for each service versions defined in ap for the output reason map.
	if compOutput, err := policyCompatible(getDeviceHandler("amd64"),
		getNodePolicyHandler(map[string]string{"prop3": "val3", "prop4": "some value"}, []string{"prop1 == val1", "prop5 == val5"}),
		getBusinessPolicyHandler(service, map[string]string{"prop1": "val1", "prop2": "val2"}, []string{"prop3 == val3", "prop4 == \"some value\""}),
		getServicePolicyHandler(map[string]string{"prop5": "val5", "prop6": "val6"}, []string{"prop4 == \"some value\""}),
		getSelectedServicesHandler(nil), getServiceDefResolverHandler(),
		&input, true, msgPrinter); err != nil {
		t.Errorf("policyCompatible should have returned nil error but got: %v", err)
	} else if !compOutput.Compatible {
		t.Errorf("policyCompatible should have returned compatible but got: %v", compOutput)
	} else if len(compOutput.Reason) != 2 {
		t.Errorf("policyCompatible should have returned 2 reason but got : %v", len(compOutput.Reason))
	} else if compOutput.Reason[sId1] != COMPATIBLE {
		t.Errorf("The reason for service %v shoud be %v, but got: %v", sId1, COMPATIBLE, compOutput.Reason[sId1])
	} else if compOutput.Reason[sId2] != COMPATIBLE {
		t.Errorf("The reason for service %v shoud be %v, but got: %v", sId2, COMPATIBLE, compOutput.Reason[sId2])
	}

	// if checkAll is flase, it only returns one compaitble entry for the output reason map.
	if compOutput, err := policyCompatible(getDeviceHandler("amd64"),
		getNodePolicyHandler(map[string]string{"prop3": "val3", "prop4": "some value"}, []string{"prop1 == val1", "prop5 == val5"}),
		getBusinessPolicyHandler(service, map[string]string{"prop1": "val1", "prop2": "val2"}, []string{"prop3 == val3", "prop4 == \"some value\""}),
		getServicePolicyHandler(map[string]string{"prop5": "val5", "prop6": "val6"}, []string{"prop4 == \"some value\""}),
		getSelectedServicesHandler(nil), getServiceDefResolverHandler(),
		&input, false, msgPrinter); err != nil {
		t.Errorf("policyCompatible should have returned nil error but got: %v", err)
	} else if !compOutput.Compatible {
		t.Errorf("policyCompatible should have returned compatible but got: %v", compOutput)
	} else if len(compOutput.Reason) != 1 {
		t.Errorf("policyCompatible should have returned 1 reason but got : %v", len(compOutput.Reason))
	} else if compOutput.Reason[sId1] != COMPATIBLE {
		t.Errorf("The reason for service %v shoud be %v, but got: %v", sId1, COMPATIBLE, compOutput.Reason[sId1])
	}

	// node arch on the exchange does not agree with the input node arch
	input_wrong_arch := PolicyCheck{
		NodeId:         "myorg/mynode",
		NodeArch:       "arm64",
		NodePolicy:     nil,
		BusinessPolId:  "myorg/mybp",
		BusinessPolicy: nil,
		ServicePolicy:  nil,
	}
	if _, err := policyCompatible(getDeviceHandler("amd64"),
		getNodePolicyHandler(map[string]string{"prop3": "val3", "prop4": "some value"}, []string{"prop1 == val1", "prop5 == val5"}),
		getBusinessPolicyHandler(service, map[string]string{"prop1": "val1", "prop2": "val2"}, []string{"prop3 == val3", "prop4 == \"some value\""}),
		getServicePolicyHandler(map[string]string{"prop5": "val5", "prop6": "val6"}, []string{"prop4 == \"some value\""}),
		getSelectedServicesHandler(nil), getServiceDefResolverHandler(),
		&input_wrong_arch, true, msgPrinter); err == nil {
		t.Errorf("policyCompatible should not have returned nil error.")
	} else if !strings.Contains(err.Error(), "The input node architecture arm64 does not match") {
		t.Errorf("policyCompatible should have returned error that contains 'input node architecture arm64 does not match' but got: %v", err)
	}

	// node arch on the exchange does not agree with the service arch
	input2 := PolicyCheck{
		NodeId:         "myorg/mynode",
		NodeArch:       "",
		NodePolicy:     nil,
		BusinessPolId:  "myorg/mybp",
		BusinessPolicy: nil,
		ServicePolicy:  nil,
	}
	if compOutput, err := policyCompatible(getDeviceHandler("arm64"),
		getNodePolicyHandler(map[string]string{"prop3": "val3", "prop4": "some value"}, []string{"prop1 == val1", "prop5 == val5"}),
		getBusinessPolicyHandler(service, map[string]string{"prop1": "val1", "prop2": "val2"}, []string{"prop3 == val3", "prop4 == \"some value\""}),
		getServicePolicyHandler(map[string]string{"prop5": "val5", "prop6": "val6"}, []string{"prop4 == \"some value\""}),
		getSelectedServicesHandler(nil), getServiceDefResolverHandler(),
		&input2, true, msgPrinter); err != nil {
		t.Errorf("policyCompatible should have returned nil error but got: %v", err)
	} else if compOutput.Compatible {
		t.Errorf("policyCompatible should have returned incompatible but got: %v", compOutput)
	} else if len(compOutput.Reason) != 2 {
		t.Errorf("policyCompatible should have returned 2 reason but got : %v", len(compOutput.Reason))
	} else if !strings.Contains(compOutput.Reason[sId1], INCOMPATIBLE) {
		t.Errorf("The reason for service %v shoud contain %v, but got: %v", sId1, INCOMPATIBLE, compOutput.Reason[sId1])
	} else if !strings.Contains(compOutput.Reason[sId2], INCOMPATIBLE) {
		t.Errorf("The reason for service %v shoud contain %v, but got: %v", sId2, INCOMPATIBLE, compOutput.Reason[sId2])
	} else if !strings.Contains(compOutput.Reason[sId1], "Architecture does not match") {
		t.Errorf("The reason for service %v shoud contain '%v', but got: %v", sId1, "Architecture does not match", compOutput.Reason[sId1])
	} else if !strings.Contains(compOutput.Reason[sId2], "Architecture does not match") {
		t.Errorf("The reason for service %v shoud contain '%v', but got: %v", sId2, "Architecture does not match", compOutput.Reason[sId2])
	}

	// service arch is "*" in business policy
	service2 := businesspolicy.ServiceRef{
		Name:            svcUrl,
		Org:             svcOrg,
		Arch:            "*",
		ServiceVersions: []businesspolicy.WorkloadChoice{businesspolicy.WorkloadChoice{Version: svcVersion1}, businesspolicy.WorkloadChoice{Version: svcVersion2}},
	}
	input3 := PolicyCheck{
		NodeId:         "myorg/mynode",
		NodeArch:       "",
		NodePolicy:     nil,
		BusinessPolId:  "myorg/mybp",
		BusinessPolicy: nil,
		ServicePolicy:  nil,
	}
	services := map[string]exchange.ServiceDefinition{
		"myorg/weather_1.0.1_amd64": exchange.ServiceDefinition{URL: svcUrl, Version: "1.0.1", Arch: "amd64"},
		"myorg/weather_2.0.1_amd64": exchange.ServiceDefinition{URL: svcUrl, Version: "2.0.1", Arch: "amd64"},
		"myorg/weather_3.0.1_amd64": exchange.ServiceDefinition{URL: svcUrl, Version: "3.0.1", Arch: "amd64"},
		"myorg/weather_1.0.1_arm64": exchange.ServiceDefinition{URL: svcUrl, Version: "1.0.1", Arch: "arm64"},
		"myorg/weather_2.0.1_arm64": exchange.ServiceDefinition{URL: svcUrl, Version: "2.0.1", Arch: "arm64"},
		"myorg/weather_3.0.1_arm64": exchange.ServiceDefinition{URL: svcUrl, Version: "3.0.1", Arch: "arm64"},
	}

	sId4 := cutil.FormExchangeIdForService(svcUrl, "1.0.1", "arm64")
	sId4 = fmt.Sprintf("%v/%v", svcOrg, sId4)
	sId5 := cutil.FormExchangeIdForService(svcUrl, "2.0.1", "arm64")
	sId5 = fmt.Sprintf("%v/%v", svcOrg, sId5)
	if compOutput, err := policyCompatible(getDeviceHandler("amd64"),
		getNodePolicyHandler(map[string]string{"prop3": "val3", "prop4": "some value"}, []string{"prop1 == val1", "prop5 == val5"}),
		getBusinessPolicyHandler(service2, map[string]string{"prop1": "val1", "prop2": "val2"}, []string{"prop3 == val3", "prop4 == \"some value\""}),
		getServicePolicyHandler(map[string]string{"prop5": "val5", "prop6": "val6"}, []string{"prop4 == \"some value\""}),
		getSelectedServicesHandler(services), getServiceDefResolverHandler(),
		&input3, true, msgPrinter); err != nil {
		t.Errorf("policyCompatible should have returned nil error but got: %v", err)
	} else if !compOutput.Compatible {
		t.Errorf("policyCompatible should have returned compatible but got: %v", compOutput)
	} else if len(compOutput.Reason) != 2 {
		t.Errorf("policyCompatible should have returned 2 reason but got : %v", len(compOutput.Reason))
	} else if compOutput.Reason[sId1] != COMPATIBLE {
		t.Errorf("The reason for service %v shoud be %v, but got: %v", sId1, COMPATIBLE, compOutput.Reason[sId1])
	} else if compOutput.Reason[sId2] != COMPATIBLE {
		t.Errorf("The reason for service %v shoud be %v, but got: %v", sId2, COMPATIBLE, compOutput.Reason[sId2])
	}
}

func Test_policyCompatible_with_Pols(t *testing.T) {

	msgPrinter := i18n.GetMessagePrinter()

	svcUrl := "weather"
	svcOrg := "myorg"
	svcVersion1 := "1.0.1"
	svcVersion2 := "2.0.1"
	svcVersion3 := "3.0.1"
	svcArch1 := "amd64"
	svcArch2 := "arm64"
	service := businesspolicy.ServiceRef{
		Name:            svcUrl,
		Org:             svcOrg,
		Arch:            svcArch1,
		ServiceVersions: []businesspolicy.WorkloadChoice{businesspolicy.WorkloadChoice{Version: svcVersion1}, businesspolicy.WorkloadChoice{Version: svcVersion2}},
	}

	nodePolicy := createExternalPolicy(map[string]string{"prop3": "val3", "prop4": "some value"}, []string{"prop1 == val1", "prop5 == val5"})
	servicePolicy := createExternalPolicy(map[string]string{"prop5": "val5", "prop6": "val6"}, []string{"prop4 == \"some value\""})
	businessPolicy := createBusinessPolicy(service, map[string]string{"prop1": "val1", "prop2": "val2"}, []string{"prop3 == val3", "prop4 == \"some value\""})

	input0 := PolicyCheck{
		NodeId:         "",
		NodeArch:       "",
		NodePolicy:     nodePolicy,
		BusinessPolId:  "",
		BusinessPolicy: businessPolicy,
		ServicePolicy:  servicePolicy,
	}
	sId1 := cutil.FormExchangeIdForService(svcUrl, svcVersion1, svcArch1)
	sId1 = fmt.Sprintf("%v/%v", svcOrg, sId1)
	sId2 := cutil.FormExchangeIdForService(svcUrl, svcVersion2, svcArch1)
	sId2 = fmt.Sprintf("%v/%v", svcOrg, sId2)
	sId3 := cutil.FormExchangeIdForService(svcUrl, svcVersion3, svcArch1)
	sId3 = fmt.Sprintf("%v/%v", svcOrg, sId3)

	// compatible
	if compOutput, err := policyCompatible(getDeviceHandler(""),
		getNodePolicyHandler(map[string]string{}, []string{}),
		getBusinessPolicyHandler(service, map[string]string{}, []string{}),
		getServicePolicyHandler(map[string]string{}, []string{}),
		getSelectedServicesHandler(nil), getServiceDefResolverHandler(),
		&input0, true, msgPrinter); err != nil {
		t.Errorf("policyCompatible should have returned nil error but got: %v", err)
	} else if !compOutput.Compatible {
		t.Errorf("policyCompatible should have returned compatible but got: %v", compOutput)
	} else if len(compOutput.Reason) != 2 {
		t.Errorf("policyCompatible should have returned 2 reason but got : %v", len(compOutput.Reason))
	} else if compOutput.Reason[sId1] != COMPATIBLE {
		t.Errorf("The reason for service %v shoud be %v, but got: %v", sId1, COMPATIBLE, compOutput.Reason[sId1])
	} else if compOutput.Reason[sId2] != COMPATIBLE {
		t.Errorf("The reason for service %v shoud be %v, but got: %v", sId2, COMPATIBLE, compOutput.Reason[sId2])
	}

	// Incompatible, type mismatch
	input0.NodeType = "cluster"
	err_string := "Service does not have cluster deployment configuration for node type 'cluster'"
	// compatible
	if compOutput, err := policyCompatible(getDeviceHandler(""),
		getNodePolicyHandler(map[string]string{}, []string{}),
		getBusinessPolicyHandler(service, map[string]string{}, []string{}),
		getServicePolicyHandler(map[string]string{}, []string{}),
		getSelectedServicesHandler(nil), getServiceDefResolverHandler(),
		&input0, true, msgPrinter); err != nil {
		t.Errorf("policyCompatible should have returned nil error but got: %v", err)
	} else if compOutput.Compatible {
		t.Errorf("policyCompatible should returned compatible but got: %v", compOutput)
	} else if len(compOutput.Reason) != 2 {
		t.Errorf("policyCompatible should have returned 2 reason but got : %v", len(compOutput.Reason))
	} else if !strings.Contains(compOutput.Reason[sId1], err_string) {
		t.Errorf("The reason for service %v shoud be: %v, but got: %v", sId1, err_string, compOutput.Reason[sId1])
	} else if !strings.Contains(compOutput.Reason[sId2], err_string) {
		t.Errorf("The reason for service %v shoud be: %v, but got: %v", sId2, err_string, compOutput.Reason[sId2])
	}

	// compatible, with service input
	serviceDef1 := createService(svcUrl, svcOrg, svcVersion1, svcArch1, map[string]map[string]map[string]string{"services": {"cpu": {"image": "cpu:1.2.2"}}}, "")
	serviceDef2 := createService(svcUrl, svcOrg, svcVersion2, svcArch1, map[string]map[string]map[string]string{"services": {"cpu": {"image": "cpu:1.2.2"}}}, "")
	input1 := PolicyCheck{
		NodeId:         "",
		NodeArch:       "",
		NodePolicy:     nodePolicy,
		BusinessPolId:  "",
		BusinessPolicy: businessPolicy,
		ServicePolicy:  servicePolicy,
		Service:        []common.AbstractServiceFile{&serviceDef1, &serviceDef2},
	}
	if compOutput, err := policyCompatible(getDeviceHandler(""),
		getNodePolicyHandler(map[string]string{}, []string{}),
		getBusinessPolicyHandler(service, map[string]string{}, []string{}),
		getServicePolicyHandler(map[string]string{}, []string{}),
		getSelectedServicesHandler(nil), getServiceDefResolverHandler(),
		&input1, true, msgPrinter); err != nil {
		t.Errorf("policyCompatible should have returned nil error but got: %v", err)
	} else if !compOutput.Compatible {
		t.Errorf("policyCompatible should have returned compatible but got: %v", compOutput)
	} else if len(compOutput.Reason) != 2 {
		t.Errorf("policyCompatible should have returned 2 reason but got : %v.", len(compOutput.Reason))
	} else if compOutput.Reason[sId1] != COMPATIBLE {
		t.Errorf("The reason for service %v shoud be %v, but got: %v", sId1, COMPATIBLE, compOutput.Reason[sId1])
	} else if compOutput.Reason[sId2] != COMPATIBLE {
		t.Errorf("The reason for service %v shoud be %v, but got: %v", sId2, COMPATIBLE, compOutput.Reason[sId2])
	}

	// incompatible, type mismatch, with service input
	serviceDef3 := createService(svcUrl, svcOrg, svcVersion1, svcArch1, nil, "cluster deployment")
	serviceDef4 := createService(svcUrl, svcOrg, svcVersion2, svcArch1, nil, "cluster deployment")
	err_string2 := "Service does not have deployment configuration for node type 'device'"
	input1_1 := PolicyCheck{
		NodeId:         "",
		NodeArch:       "",
		NodePolicy:     nodePolicy,
		BusinessPolId:  "",
		BusinessPolicy: businessPolicy,
		ServicePolicy:  servicePolicy,
		Service:        []common.AbstractServiceFile{&serviceDef3, &serviceDef4},
		NodeType:       "device",
	}
	if compOutput, err := policyCompatible(getDeviceHandler(""),
		getNodePolicyHandler(map[string]string{}, []string{}),
		getBusinessPolicyHandler(service, map[string]string{}, []string{}),
		getServicePolicyHandler(map[string]string{}, []string{}),
		getSelectedServicesHandler(nil), getServiceDefResolverHandler(),
		&input1_1, true, msgPrinter); err != nil {
		t.Errorf("policyCompatible should have returned nil error but got: %v", err)
	} else if compOutput.Compatible {
		t.Errorf("policyCompatible should returned compatible but got: %v", compOutput)
	} else if len(compOutput.Reason) != 2 {
		t.Errorf("policyCompatible should have returned 2 reason but got : %v", len(compOutput.Reason))
	} else if !strings.Contains(compOutput.Reason[sId1], err_string2) {
		t.Errorf("The reason for service %v shoud be: %v, but got: %v", sId1, err_string2, compOutput.Reason[sId1])
	} else if !strings.Contains(compOutput.Reason[sId2], err_string2) {
		t.Errorf("The reason for service %v shoud be: %v, but got: %v", sId2, err_string2, compOutput.Reason[sId2])
	}

	// in compatible
	nodePolicy2 := createExternalPolicy(map[string]string{"prop3": "val3", "prop4": "some other value"}, []string{"prop1 == val1", "prop5 == val5"})
	input2 := PolicyCheck{
		NodeId:         "",
		NodeArch:       "",
		NodePolicy:     nodePolicy2,
		BusinessPolId:  "",
		BusinessPolicy: businessPolicy,
		ServicePolicy:  servicePolicy,
	}
	if compOutput, err := policyCompatible(getDeviceHandler(""),
		getNodePolicyHandler(map[string]string{}, []string{}),
		getBusinessPolicyHandler(service, map[string]string{}, []string{}),
		getServicePolicyHandler(map[string]string{}, []string{}),
		getSelectedServicesHandler(nil), getServiceDefResolverHandler(),
		&input2, true, msgPrinter); err != nil {
		t.Errorf("policyCompatible should have returned nil error but got: %v", err)
	} else if compOutput.Compatible {
		t.Errorf("policyCompatible should have returned incompatible but got: %v", compOutput)
	} else if len(compOutput.Reason) != 2 {
		t.Errorf("policyCompatible should have returned 2 reason but got : %v", len(compOutput.Reason))
	} else if !strings.Contains(compOutput.Reason[sId1], INCOMPATIBLE) {
		t.Errorf("The reason for service %v shoud contain %v, but got: %v", sId1, INCOMPATIBLE, compOutput.Reason[sId1])
	} else if !strings.Contains(compOutput.Reason[sId2], INCOMPATIBLE) {
		t.Errorf("The reason for service %v shoud contain %v, but got: %v", sId2, INCOMPATIBLE, compOutput.Reason[sId2])
	}

	// service arch is "*" in business policy
	service2 := businesspolicy.ServiceRef{
		Name:            svcUrl,
		Org:             svcOrg,
		Arch:            "*",
		ServiceVersions: []businesspolicy.WorkloadChoice{businesspolicy.WorkloadChoice{Version: svcVersion1}, businesspolicy.WorkloadChoice{Version: svcVersion2}},
	}
	businessPolicy2 := createBusinessPolicy(service2, map[string]string{"prop1": "val1", "prop2": "val2"}, []string{"prop3 == val3", "prop4 == \"some value\""})
	input3 := PolicyCheck{
		NodeId:         "",
		NodeArch:       "",
		NodePolicy:     nodePolicy,
		BusinessPolId:  "",
		BusinessPolicy: businessPolicy2,
		ServicePolicy:  nil,
	}
	services := map[string]exchange.ServiceDefinition{
		"myorg/weather_1.0.1_amd64": exchange.ServiceDefinition{URL: svcUrl, Version: "1.0.1", Arch: "amd64"},
		"myorg/weather_2.0.1_amd64": exchange.ServiceDefinition{URL: svcUrl, Version: "2.0.1", Arch: "amd64"},
		"myorg/weather_3.0.1_amd64": exchange.ServiceDefinition{URL: svcUrl, Version: "3.0.1", Arch: "amd64"},
		"myorg/weather_1.0.1_arm64": exchange.ServiceDefinition{URL: svcUrl, Version: "1.0.1", Arch: "arm64"},
		"myorg/weather_2.0.1_arm64": exchange.ServiceDefinition{URL: svcUrl, Version: "2.0.1", Arch: "arm64"},
		"myorg/weather_3.0.1_arm64": exchange.ServiceDefinition{URL: svcUrl, Version: "3.0.1", Arch: "arm64"},
	}

	sId4 := cutil.FormExchangeIdForService(svcUrl, svcVersion1, svcArch2)
	sId4 = fmt.Sprintf("%v/%v", svcOrg, sId4)
	sId5 := cutil.FormExchangeIdForService(svcUrl, svcVersion2, svcArch2)
	sId5 = fmt.Sprintf("%v/%v", svcOrg, sId5)
	sId6 := cutil.FormExchangeIdForService(svcUrl, svcVersion3, svcArch2)
	sId6 = fmt.Sprintf("%v/%v", svcOrg, sId6)
	if compOutput, err := policyCompatible(getDeviceHandler(""),
		getNodePolicyHandler(map[string]string{}, []string{}),
		getBusinessPolicyHandler(service2, map[string]string{}, []string{}),
		getServicePolicyHandler(map[string]string{"prop5": "val5", "prop6": "val6"}, []string{"prop4 == \"some value\""}),
		getSelectedServicesHandler(services), getServiceDefResolverHandler(),
		&input3, true, msgPrinter); err != nil {
		t.Errorf("policyCompatible should have returned nil error but got: %v", err)
	} else if !compOutput.Compatible {
		t.Errorf("policyCompatible should have returned compatible but got: %v", compOutput)
	} else if len(compOutput.Reason) != 4 {
		t.Errorf("policyCompatible should have returned 4 reason but got : %v", len(compOutput.Reason))
	} else if compOutput.Reason[sId1] != COMPATIBLE {
		t.Errorf("The reason for service %v shoud be %v, but got: %v", sId1, COMPATIBLE, compOutput.Reason[sId1])
	} else if compOutput.Reason[sId2] != COMPATIBLE {
		t.Errorf("The reason for service %v shoud be %v, but got: %v", sId2, COMPATIBLE, compOutput.Reason[sId2])
	} else if compOutput.Reason[sId4] != COMPATIBLE {
		t.Errorf("The reason for service %v shoud be %v, but got: %v", sId2, COMPATIBLE, compOutput.Reason[sId2])
	} else if compOutput.Reason[sId5] != COMPATIBLE {
		t.Errorf("The reason for service %v shoud be %v, but got: %v", sId2, COMPATIBLE, compOutput.Reason[sId2])
	}

	input4 := PolicyCheck{
		NodeId:         "",
		NodeArch:       "amd64",
		NodePolicy:     nodePolicy,
		BusinessPolId:  "",
		BusinessPolicy: businessPolicy2,
		ServicePolicy:  nil,
	}
	if compOutput, err := policyCompatible(getDeviceHandler(""),
		getNodePolicyHandler(map[string]string{}, []string{}),
		getBusinessPolicyHandler(service2, map[string]string{}, []string{}),
		getServicePolicyHandler(map[string]string{"prop5": "val5", "prop6": "val6"}, []string{"prop4 == \"some value\""}),
		getSelectedServicesHandler(services), getServiceDefResolverHandler(),
		&input4, true, msgPrinter); err != nil {
		t.Errorf("policyCompatible should have returned nil error but got: %v", err)
	} else if !compOutput.Compatible {
		t.Errorf("policyCompatible should have returned compatible but got: %v", compOutput)
	} else if len(compOutput.Reason) != 2 {
		t.Errorf("policyCompatible should have returned 2 reason but got : %v", len(compOutput.Reason))
	} else if compOutput.Reason[sId1] != COMPATIBLE {
		t.Errorf("The reason for service %v shoud be %v, but got: %v", sId1, COMPATIBLE, compOutput.Reason[sId1])
	} else if compOutput.Reason[sId2] != COMPATIBLE {
		t.Errorf("The reason for service %v shoud be %v, but got: %v", sId1, COMPATIBLE, compOutput.Reason[sId2])
	}
}

func Test_policyCompatible_Error(t *testing.T) {

	msgPrinter := i18n.GetMessagePrinter()

	input := PolicyCheck{
		NodeId:         "myorg/mynode",
		NodeArch:       "",
		NodePolicy:     nil,
		BusinessPolId:  "myorg/mybp",
		BusinessPolicy: nil,
		ServicePolicy:  nil,
	}

	svcUrl := "weather"
	svcOrg := "myorg"
	svcVersion1 := "1.0.1"
	svcVersion2 := "2.0.1"
	svcArch := "amd64"
	service := businesspolicy.ServiceRef{
		Name:            svcUrl,
		Org:             svcOrg,
		Arch:            svcArch,
		ServiceVersions: []businesspolicy.WorkloadChoice{businesspolicy.WorkloadChoice{Version: svcVersion1}, businesspolicy.WorkloadChoice{Version: svcVersion2}},
	}

	// error getting node policy from the exchange
	if _, err := policyCompatible(getDeviceHandler("amd64"),
		getNodePolicyHandler_Error(),
		getBusinessPolicyHandler(service, map[string]string{"prop1": "val1", "prop2": "val2"}, []string{"prop3 == val3", "prop4 == \"some value\""}),
		getServicePolicyHandler(map[string]string{"prop5": "val5", "prop6": "val6"}, []string{"prop4 == \"some value\""}),
		getSelectedServicesHandler(nil), getServiceDefResolverHandler(),
		&input, true, msgPrinter); err == nil {
		t.Errorf("policyCompatible should not have returned nil")
	} else if !strings.Contains(err.Error(), "Error trying to query node policy") {
		t.Errorf("policyCompatible should have returned 'Error trying to query node policy' error but got: %v", err)
	}

	// error getting business policy from the exchange
	if _, err := policyCompatible(getDeviceHandler("amd64"),
		getNodePolicyHandler(map[string]string{"prop3": "val3", "prop4": "some value"}, []string{"prop1 == val1", "prop5 == val5"}),
		getBusinessPolicyHandler_Error(),
		getServicePolicyHandler(map[string]string{"prop5": "val5", "prop6": "val6"}, []string{"prop4 == \"some value\""}),
		getSelectedServicesHandler(nil), getServiceDefResolverHandler(),
		&input, true, msgPrinter); err == nil {
		t.Errorf("policyCompatible should not have returned nil")
	} else if !strings.Contains(err.Error(), "Unable to get deployment policy") {
		t.Errorf("policyCompatible should have returned 'Unable to get deployment policy' error but got: %v", err)
	}

	// error getting service policy from the exchange
	if _, err := policyCompatible(getDeviceHandler("amd64"),
		getNodePolicyHandler(map[string]string{"prop3": "val3", "prop4": "some value"}, []string{"prop1 == val1", "prop5 == val5"}),
		getBusinessPolicyHandler(service, map[string]string{"prop1": "val1", "prop2": "val2"}, []string{"prop3 == val3", "prop4 == \"some value\""}),
		getServicePolicyHandler_Error(),
		getSelectedServicesHandler(nil), getServiceDefResolverHandler(),
		&input, true, msgPrinter); err == nil {
		t.Errorf("policyCompatible should not have returned nil")
	} else if !strings.Contains(err.Error(), "Error trying to query service policy") {
		t.Errorf("policyCompatible should have returned 'Error trying to query service policy' error but got: %v", err)
	}

	// error getting services from the exchange
	nodePolicy := createExternalPolicy(map[string]string{"prop3": "val3", "prop4": "some value"}, []string{"prop1 == val1", "prop5 == val5"})
	service2 := businesspolicy.ServiceRef{
		Name:            svcUrl,
		Org:             svcOrg,
		Arch:            "*",
		ServiceVersions: []businesspolicy.WorkloadChoice{businesspolicy.WorkloadChoice{Version: svcVersion1}, businesspolicy.WorkloadChoice{Version: svcVersion2}},
	}
	input2 := PolicyCheck{
		NodeId:         "",
		NodeArch:       "",
		NodePolicy:     nodePolicy,
		BusinessPolId:  "myorg/mybp",
		BusinessPolicy: nil,
		ServicePolicy:  nil,
	}
	if compOutput, err := policyCompatible(getDeviceHandler("amd64"),
		getNodePolicyHandler(map[string]string{"prop3": "val3", "prop4": "some value"}, []string{"prop1 == val1", "prop5 == val5"}),
		getBusinessPolicyHandler(service2, map[string]string{"prop1": "val1", "prop2": "val2"}, []string{"prop3 == val3", "prop4 == \"some value\""}),
		getServicePolicyHandler(map[string]string{"prop5": "val5", "prop6": "val6"}, []string{"prop4 == \"some value\""}),
		getSelectedServicesHandler_Error(), getServiceDefResolverHandler(),
		&input2, true, msgPrinter); err == nil {
		t.Errorf("policyCompatible should not have returned nil, %v", compOutput)
	} else if !strings.Contains(err.Error(), "Failed to get services for all archetctures for") {
		t.Errorf("policyCompatible should have returned 'Failed to get services for all archetctures for' error but got: %v", err)
	}

	// validation error
	input3 := PolicyCheck{
		NodeId:         "myorg/mynode",
		NodeArch:       "",
		NodePolicy:     nil,
		BusinessPolId:  "myorg/mybp",
		BusinessPolicy: nil,
		ServicePolicy:  nil,
	}
	if _, err := policyCompatible(getDeviceHandler("amd64"),
		getNodePolicyHandler(map[string]string{"prop3": "val3", "prop4": "some value"}, []string{"prop1 == val1", "prop5 !&&= val5"}),
		getBusinessPolicyHandler(service, map[string]string{"prop1": "val1", "prop2": "val2"}, []string{"prop3 == val3", "prop4 !&& \"some value\""}),
		getServicePolicyHandler(map[string]string{"prop5": "val5", "prop6": "val6"}, []string{"prop4 == \"some value\""}),
		getSelectedServicesHandler(nil), getServiceDefResolverHandler(),
		&input3, true, msgPrinter); err == nil {
		t.Errorf("policyCompatible should not have returned nil for error")
	} else if !strings.Contains(err.Error(), "Failed to validate the node policy") {
		t.Errorf("policyCompatible should have returned 'Failed to validate the node policy' error but got: %v", err)
	}

	input4 := PolicyCheck{
		NodeId:         "myorg/mynode",
		NodeArch:       "",
		NodePolicy:     nil,
		BusinessPolId:  "myorg/mybp",
		BusinessPolicy: nil,
		ServicePolicy:  nil,
	}
	if _, err := policyCompatible(getDeviceHandler("amd64"),
		getNodePolicyHandler(map[string]string{"prop3": "val3", "prop4": "some value"}, []string{"prop1 == val1", "prop5 == val5"}),
		getBusinessPolicyHandler(service, map[string]string{"prop1": "val1", "prop2": "val2"}, []string{"prop3 == val3", "prop4 !&& \"some value\""}),
		getServicePolicyHandler(map[string]string{"prop5": "val5", "prop6": "val6"}, []string{"prop4 == \"some value\""}),
		getSelectedServicesHandler(nil), getServiceDefResolverHandler(),
		&input4, true, msgPrinter); err == nil {
		t.Errorf("policyCompatible should not have returned nil for error")
	} else if !strings.Contains(err.Error(), "Failed to validate the business policy") {
		t.Errorf("policyCompatible should have returned 'Failed to validate the business policy' error but got: %v", err)
	}

	input5 := PolicyCheck{
		NodeId:         "myorg/mynode",
		NodeArch:       "",
		NodePolicy:     nil,
		BusinessPolId:  "myorg/mybp",
		BusinessPolicy: nil,
		ServicePolicy:  nil,
	}
	if _, err := policyCompatible(getDeviceHandler("amd64"),
		getNodePolicyHandler(map[string]string{"prop3": "val3", "prop4": "some value"}, []string{"prop1 == val1", "prop5 == val5"}),
		getBusinessPolicyHandler(service, map[string]string{"prop1": "val1", "prop2": "val2"}, []string{"prop3 == val3", "prop4 == \"some value\""}),
		getServicePolicyHandler(map[string]string{"prop5": "val5", "prop6": "val6"}, []string{"prop4 &&%% \"some value\""}),
		getSelectedServicesHandler(nil), getServiceDefResolverHandler(),
		&input5, true, msgPrinter); err == nil {
		t.Errorf("policyCompatible should not have returned nil for error")
	} else if !strings.Contains(err.Error(), "Failed to validate the service policy") {
		t.Errorf("policyCompatible should have returned 'Failed to validate the service policy' error but got: %v", err)
	}
}

func Test_CheckPolicyCompatiblility(t *testing.T) {

	msgPrinter := i18n.GetMessagePrinter()

	svcUrl := "weather"
	svcOrg := "myorg"
	svcVersion := "1.0.1"
	svcArch := "amd64"
	service := businesspolicy.ServiceRef{
		Name:            svcUrl,
		Org:             svcOrg,
		Arch:            svcArch,
		ServiceVersions: []businesspolicy.WorkloadChoice{businesspolicy.WorkloadChoice{Version: svcVersion}},
	}

	_, intBPol, err := GetBusinessPolicy(getBusinessPolicyHandler(service, map[string]string{"prop1": "val1", "prop2": "val2"}, []string{"prop3 == val3", "prop4 == \"some value\""}), "myorg/mybp", true, msgPrinter)
	if err != nil {
		t.Errorf("GetBusinessPolicy should have returned nil error but got: %v", err)
	}

	_, intNPol, err := GetNodePolicy(getNodePolicyHandler(map[string]string{"prop3": "val3", "prop4": "some value"}, []string{"prop1 == val1", "prop5 == val5"}), "myorg/mynode", msgPrinter)
	if err != nil {
		t.Errorf("GetNodePolicy should have returned nil error but got: %v", err)
	}

	mergedSPol, _, _, _, _, err := GetServicePolicyWithDefaultProperties(getServicePolicyHandler(map[string]string{"prop5": "val5", "prop6": "val6"}, []string{"prop4 == \"some value\""}), getServiceDefResolverHandler(), svcUrl, svcOrg, svcVersion, svcArch, msgPrinter)
	if err != nil {
		t.Errorf("GetServicePolicyWithDefaultProperties should have returned nil error but got: %v", err)
	}

	// compatible, node arch is "" -- this is the case when agbot calls it
	if compatible, reason, producerPolicy, consumerPolicy, err := CheckPolicyCompatiblility(intNPol, intBPol, mergedSPol, "", msgPrinter); err != nil {
		t.Errorf("CheckPolicyCompatiblility should have returned nil error but got: %v", err)
	} else if !compatible {
		t.Errorf("CheckPolicyCompatiblility should have returned compatible but got: %v", reason)
	} else if producerPolicy == nil {
		t.Errorf("producerPolicy should not be null")
	} else if consumerPolicy == nil {
		t.Errorf("consumerPolicy should not be null")
	} else if len(producerPolicy.Properties) != 2 {
		t.Errorf("The producerPolicy should not have 2 properties but got %v", len(producerPolicy.Properties))
	} else if len(producerPolicy.Constraints) != 2 {
		t.Errorf("The producerPolicy should not have 2 constraints but got %v", len(producerPolicy.Constraints))
	} else if len(consumerPolicy.Properties) != 10 {
		t.Errorf("The consumerPolicy should not have 9 properties but got %v", len(consumerPolicy.Properties))
	} else if len(consumerPolicy.Constraints) != 2 {
		t.Errorf("The consumerPolicy should not have 2 constraints but got %v", len(consumerPolicy.Constraints))
	}

	_, intNPol1, err := GetNodePolicy(getNodePolicyHandler(map[string]string{"prop3": "val3", "prop4": "some other value"}, []string{"prop1 == val1", "prop5 == val5"}), "myorg/mynode", msgPrinter)
	if err != nil {
		t.Errorf("GetNodePolicy should have returned nil error but got: %v", err)
	}
	// not compatible,
	if compatible, _, producerPolicy, consumerPolicy, err := CheckPolicyCompatiblility(intNPol1, intBPol, mergedSPol, "", msgPrinter); err != nil {
		t.Errorf("CheckPolicyCompatiblility should have returned nil error but got: %v", err)
	} else if compatible {
		t.Errorf("CheckPolicyCompatiblility should have returned not compatible but not")
	} else if producerPolicy == nil {
		t.Errorf("producerPolicy should not be null")
	} else if consumerPolicy == nil {
		t.Errorf("consumerPolicy should not be null")
	} else if len(producerPolicy.Properties) != 2 {
		t.Errorf("The producerPolicy should not have 2 properties but got %v", len(producerPolicy.Properties))
	} else if len(producerPolicy.Constraints) != 2 {
		t.Errorf("The producerPolicy should not have 2 constraints but got %v", len(producerPolicy.Constraints))
	} else if len(consumerPolicy.Properties) != 10 {
		t.Errorf("The consumerPolicy should not have 9 properties but got %v", len(consumerPolicy.Properties))
	} else if len(consumerPolicy.Constraints) != 2 {
		t.Errorf("The consumerPolicy should not have 2 constraints but got %v", len(consumerPolicy.Constraints))
	}

	// compatible, node arch is not empty -- this is the case when this function is called from policyCompatible_Pols
	if compatible, reason, producerPolicy, _, err := CheckPolicyCompatiblility(intNPol, intBPol, mergedSPol, "amd64", msgPrinter); err != nil {
		t.Errorf("CheckPolicyCompatiblility should have returned nil error but got: %v", err)
	} else if !compatible {
		t.Errorf("CheckPolicyCompatiblility should have returned compatible error but got: %v", reason)
	} else if producerPolicy == nil {
		t.Errorf("producerPolicy should not be null")
	} else if len(producerPolicy.Properties) != 3 {
		t.Errorf("The producerPolicy should not have 3 properties but got %v", len(producerPolicy.Properties))
	}

	// error cases
	if _, _, _, _, err := CheckPolicyCompatiblility(nil, intBPol, mergedSPol, "arm64", msgPrinter); err == nil {
		t.Errorf("CheckPolicyCompatiblility should not have returned nil error")
	} else if err.Error() != "Node policy cannot be null." {
		t.Errorf("CheckPolicyCompatiblility should return 'Node policy cannot be null.' error but got: %v", err)
	}
	if _, _, _, _, err := CheckPolicyCompatiblility(intNPol, nil, mergedSPol, "arm64", msgPrinter); err == nil {
		t.Errorf("CheckPolicyCompatiblility should not have returned nil error")
	} else if err.Error() != "Deployment policy cannot be null." {
		t.Errorf("CheckPolicyCompatiblility should return 'Deployment policy cannot be null.' error but got: %v", err)
	}
	if _, _, _, _, err := CheckPolicyCompatiblility(intNPol, intBPol, nil, "arm64", msgPrinter); err == nil {
		t.Errorf("CheckPolicyCompatiblility should not have returned nil error")
	} else if err.Error() != "Merged service policy cannot be null." {
		t.Errorf("CheckPolicyCompatiblility should return 'Merged service policy cannot be null.' error but got: %v", err)
	}
}

func Test_addNodeArchToPolicy(t *testing.T) {

	msgPrinter := i18n.GetMessagePrinter()

	service := businesspolicy.ServiceRef{
		Name:            "cpu",
		Org:             "mycomp",
		Arch:            "amd64",
		ServiceVersions: []businesspolicy.WorkloadChoice{businesspolicy.WorkloadChoice{Version: "1.0.1"}},
	}

	bPol, intPol, err := GetBusinessPolicy(getBusinessPolicyHandler(service, map[string]string{"prop1": "val1", "prop2": "val2"}, []string{"prop3 == val3", "prop4 == \"some value\""}), "myorg/mybp", true, msgPrinter)
	if err != nil {
		t.Errorf("GetBusinessPolicy should have returned nil error but got: %v", err)
	} else if bPol == nil {
		t.Errorf("The returned business policy should not be null")
	} else if len(bPol.Properties) != 2 {
		t.Errorf("The business policy should not have 2 properties but got %v", len(bPol.Properties))
	} else if len(bPol.Constraints) != 2 {
		t.Errorf("The business policy should not have 2 constraints but got %v", len(bPol.Constraints))
	} else if len(intPol.Properties) != 2 {
		t.Errorf("The internal policy should not have 2 properties but got %v", len(bPol.Properties))
	} else if len(intPol.Constraints) != 2 {
		t.Errorf("The internal policy should not have 2 constraints but got %v", len(bPol.Constraints))
	}

	if pol, err := addNodeArchToPolicy(intPol, "amd64", msgPrinter); err != nil {
		t.Errorf("addNodeArchToPolicy should have returned nil error but got: %v", err)
	} else if len(pol.Properties) != 3 {
		t.Errorf("The policy should not have 3 properties but got %v", len(pol.Properties))
	} else if len(bPol.Constraints) != 2 {
		t.Errorf("The policy should not have 2 constraints but got %v", len(pol.Constraints))
	} else {
		found := false
		for _, p := range pol.Properties {
			if p.Name == externalpolicy.PROP_NODE_ARCH && p.Value == "amd64" {
				found = true
			}
		}
		if !found {
			t.Errorf("%v=amd64 should have be in the properties but not.", externalpolicy.PROP_NODE_ARCH)
		}
	}

	if pol, err := addNodeArchToPolicy(nil, "amd64", msgPrinter); err != nil {
		t.Errorf("addNodeArchToPolicy should have returned nil error but got: %v", err)
	} else if pol != nil {
		t.Errorf("addNodeArchToPolicy should have returned nil policy but got: %v", pol)
	}

	if pol, err := addNodeArchToPolicy(intPol, "", msgPrinter); err != nil {
		t.Errorf("addNodeArchToPolicy should have returned nil error but got: %v", err)
	} else if pol != intPol {
		t.Errorf("addNodeArchToPolicy should have return the same policy when arch is empty, but not")
	}
}

func Test_GetServicePolicy(t *testing.T) {

	msgPrinter := i18n.GetMessagePrinter()

	svcUrl := "weather"
	svcOrg := "myorg"
	svcVersion := "1.0.1"
	svcArch := "amd64"

	sId1 := cutil.FormExchangeIdForService(svcUrl, svcVersion, svcArch)
	sId1 = fmt.Sprintf("%v/%v", svcOrg, sId1)

	if sPol, sId, err := GetServicePolicy(getServicePolicyHandler(map[string]string{"prop1": "val1", "prop2": "val2"}, []string{"prop3 == val3", "prop4 == \"some value\""}), svcUrl, svcOrg, svcVersion, svcArch, msgPrinter); err != nil {
		t.Errorf("GetServicePolicy should have returned nil error but got: %v", err)
	} else if sId != sId1 {
		t.Errorf("The servicd id should be %v but got: %v", sId1, sId)
	} else if sPol == nil {
		t.Errorf("The returned service policy should not be null")
	} else if len(sPol.Properties) != 2 {
		t.Errorf("The service policy hould not have 2 properties but got %v", len(sPol.Properties))
	} else if len(sPol.Constraints) != 2 {
		t.Errorf("The service policy hould not have 2 constraints but got %v", len(sPol.Constraints))
	}

	if _, _, err := GetServicePolicy(getServicePolicyHandler(map[string]string{"prop1": "val1", "prop2": "val2"}, []string{"prop4 == \"some value\""}), "", svcOrg, svcVersion, svcArch, msgPrinter); err == nil {
		t.Errorf("GetServicePolicy should not have returned nil error but it has.")
	} else if err.Error() != "Service name is empty." {
		t.Errorf("GetServicePolicy should have return error string 'Service name is empty.' but got: %v", err)
	}

	if _, _, err := GetServicePolicy(getServicePolicyHandler(map[string]string{"prop1": "val1", "prop2": "val2"}, []string{"prop4 == \"some value\""}), svcUrl, "", svcVersion, svcArch, msgPrinter); err == nil {
		t.Errorf("GetServicePolicy should not have returned nil error but it has.")
	} else if err.Error() != "Service organization is empty." {
		t.Errorf("GetServicePolicy should have return error string 'Service name is empty.' but got: %v", err)
	}

	if _, _, err := GetServicePolicy(getServicePolicyHandler_Error(), svcUrl, svcOrg, svcVersion, svcArch, msgPrinter); err == nil {
		t.Errorf("GetServicePolicy should have returned error but got nil")
	}
}

func Test_GetServicePolicyWithDefaultProperties(t *testing.T) {

	msgPrinter := i18n.GetMessagePrinter()

	svcUrl := "weather"
	svcOrg := "myorg"
	svcVersion := "1.0.1"
	svcArch := "amd64"

	sId1 := cutil.FormExchangeIdForService(svcUrl, svcVersion, svcArch)
	sId1 = fmt.Sprintf("%v/%v", svcOrg, sId1)

	if mergedPol, sPol, sDef, sId, _, err := GetServicePolicyWithDefaultProperties(getServicePolicyHandler(map[string]string{"prop1": "val1", "prop2": "val2"}, []string{"prop4 == \"some value\""}), getServiceDefResolverHandler(), svcUrl, svcOrg, svcVersion, svcArch, msgPrinter); err != nil {
		t.Errorf("GetServicePolicyWithDefaultProperties should have returned nil error but got: %v", err)
	} else if sId != sId1 {
		t.Errorf("The servicd id should be %v but got: %v", sId1, sId)
	} else if sDef == nil {
		t.Errorf("The returned top service definition should not be null")
	} else if sDef.GetURL() != svcUrl || sDef.GetVersion() != svcVersion || sDef.GetArch() != svcArch {
		t.Errorf("Wrong service definition returned. %v", sDef)
	} else if mergedPol == nil {
		t.Errorf("The returned merged service policy should not be null")
	} else if len(mergedPol.Properties) != 8 {
		t.Errorf("The merged service policy hould not have 7 properties but got %v", len(mergedPol.Properties))
	} else if len(mergedPol.Constraints) != 1 {
		t.Errorf("The merged service policy hould not have 1 constraints but got %v", len(mergedPol.Constraints))
	} else if sPol == nil {
		t.Errorf("The returned service policy should not be null")
	} else if len(sPol.Properties) != 2 {
		t.Errorf("The service policy hould not have 7 properties but got %v", len(sPol.Properties))
	} else if len(sPol.Constraints) != 1 {
		t.Errorf("The service policy hould not have 1 constraints but got %v", len(sPol.Constraints))
	}

	if _, _, _, _, _, err := GetServicePolicyWithDefaultProperties(getServicePolicyHandler_Error(), getServiceDefResolverHandler(), svcUrl, svcOrg, svcVersion, svcArch, msgPrinter); err == nil {
		t.Errorf("GetServicePolicyWithDefaultProperties should have returned error but got nil")
	}
}

func Test_AddDefaultPropertiesToServicePolicy(t *testing.T) {

	propList1 := new(externalpolicy.PropertyList)
	propList1.Add_Property(externalpolicy.Property_Factory("prop1", "val1"), false)
	propList1.Add_Property(externalpolicy.Property_Factory("prop2", "val2"), false)

	servicePol := &externalpolicy.ExternalPolicy{
		Properties:  *propList1,
		Constraints: []string{`prop4 == "some value"`},
	}

	propList2 := new(externalpolicy.PropertyList)
	propList2.Add_Property(externalpolicy.Property_Factory("openhorizon.service.url", "weather"), false)
	propList2.Add_Property(externalpolicy.Property_Factory("openhorizon.service.name", "weather"), false)
	propList2.Add_Property(externalpolicy.Property_Factory("openhorizon.service.org", "myorg"), false)
	propList2.Add_Property(externalpolicy.Property_Factory("openhorizon.service.version", "1.5.0"), false)
	propList2.Add_Property(externalpolicy.Property_Factory("openhorizon.service.arch", "amd64"), false)

	builtInSvcPol := &externalpolicy.ExternalPolicy{
		Properties: *propList2,
	}

	// if service policy is nil, it should return the default properties
	if mergedPol := AddDefaultPropertiesToServicePolicy(nil, nil, nil); len(mergedPol.Properties) != 0 {
		t.Errorf("The merged policy hould not have 0 properties but got %v", len(mergedPol.Properties))
	} else if len(mergedPol.Constraints) != 0 {
		t.Errorf("The merged policy hould not have 0 constraints but got %v", len(mergedPol.Constraints))
	}

	// if service policy is nil, it should return the default properties
	if mergedPol := AddDefaultPropertiesToServicePolicy(nil, builtInSvcPol, nil); len(mergedPol.Properties) != 5 {
		t.Errorf("The merged policy hould not have 5 properties but got %v", len(mergedPol.Properties))
	} else if len(mergedPol.Constraints) != 0 {
		t.Errorf("The merged policy hould not have 0 constraints but got %v", len(mergedPol.Constraints))
	}

	// if the default properties is nil, it should return the service policy
	if mergedPol := AddDefaultPropertiesToServicePolicy(servicePol, nil, nil); len(mergedPol.Properties) != 2 {
		t.Errorf("The merged policy hould not have 2 properties but got %v", len(mergedPol.Properties))
	} else if len(mergedPol.Constraints) != 1 {
		t.Errorf("The merged policy hould not have 1 constraints but got %v", len(mergedPol.Constraints))
	}

	// normal case
	if mergedPol := AddDefaultPropertiesToServicePolicy(servicePol, builtInSvcPol, nil); mergedPol == nil {
		t.Errorf("The merged policy should not be null.")
	} else if len(mergedPol.Properties) != 7 {
		t.Errorf("The merged policy hould not have 7 properties but got %v", len(mergedPol.Properties))
	} else if len(mergedPol.Constraints) != 1 {
		t.Errorf("The merged policy hould not have 1 constraints but got %v", len(mergedPol.Constraints))
	}
}

func Test_MergeFullServicePolicyToBusinessPolicy(t *testing.T) {

	msgPrinter := i18n.GetMessagePrinter()

	propList1 := new(externalpolicy.PropertyList)
	propList1.Add_Property(externalpolicy.Property_Factory("prop1", "val1"), false)
	propList1.Add_Property(externalpolicy.Property_Factory("prop2", "val2"), false)
	propList1.Add_Property(externalpolicy.Property_Factory("openhorizon.service.url", "weather"), false)
	propList1.Add_Property(externalpolicy.Property_Factory("openhorizon.service.name", "weather"), false)
	propList1.Add_Property(externalpolicy.Property_Factory("openhorizon.service.org", "myorg"), false)
	propList1.Add_Property(externalpolicy.Property_Factory("openhorizon.service.version", "1.5.0"), false)
	propList1.Add_Property(externalpolicy.Property_Factory("openhorizon.service.arch", "amd64"), false)

	servicePol := &externalpolicy.ExternalPolicy{
		Properties:  *propList1,
		Constraints: []string{`prop4 == "some value"`},
	}

	propList3 := new(externalpolicy.PropertyList)
	propList3.Add_Property(externalpolicy.Property_Factory("prop1", "val1"), false)
	propList3.Add_Property(externalpolicy.Property_Factory("prop3", "val3"), false)

	service := businesspolicy.ServiceRef{
		Name:            "weather",
		Org:             "myorg",
		Arch:            "amd64",
		ServiceVersions: []businesspolicy.WorkloadChoice{{Version: "1.5.0"}},
	}

	bPolicy := businesspolicy.BusinessPolicy{
		Owner:       "me",
		Label:       "my business policy",
		Description: "blah",
		Service:     service,
		Properties:  *propList3,
		Constraints: []string{"prop5 == val5", "prop4 == \"some value\""},
	}
	businessPol, err := bPolicy.GenPolicyFromBusinessPolicy("mypol")
	if err != nil {
		t.Errorf("GenPolicyFromBusinessPolicy should not have returned error but got: %v", err)
	}

	// if businessPol is nil, it should return error
	if _, err := MergeFullServicePolicyToBusinessPolicy(nil, servicePol, msgPrinter); err == nil {
		t.Errorf("MergeFullServicePolicyToBusinessPolicy should have returned error but got nil")
	}

	// if service policy is nil, it should return the business policy
	if outPol, err := MergeFullServicePolicyToBusinessPolicy(businessPol, nil, msgPrinter); err != nil {
		t.Errorf("MergeFullServicePolicyToBusinessPolicy should have returned error but got: %v", err)
	} else if outPol == nil {
		t.Errorf("The merged policy should not be null.")
	} else if outPol != businessPol {
		t.Errorf("The merged policy should be the same as the business policy but got: %v", outPol)
	}

	// normal case
	if outPol, err := MergeFullServicePolicyToBusinessPolicy(businessPol, servicePol, msgPrinter); err != nil {
		t.Errorf("MergeFullServicePolicyToBusinessPolicy should not have returned error but got: %v", err)
	} else if outPol == nil {
		t.Errorf("The merged policy should not be null.")
	} else if len(outPol.Properties) != 8 {
		t.Errorf("The merged policy hould not have 8 properties but got %v", len(outPol.Properties))
	} else if len(outPol.Constraints) != 2 {
		t.Errorf("The merged policy hould not have 2 constraints but got %v", len(outPol.Constraints))
	}
}

// Helper functions
func createExternalPolicy(p map[string]string, c []string) *externalpolicy.ExternalPolicy {
	propList := new(externalpolicy.PropertyList)
	for k, v := range p {
		propList.Add_Property(externalpolicy.Property_Factory(k, v), false)
	}

	pol := externalpolicy.ExternalPolicy{
		Properties:  *propList,
		Constraints: c,
	}
	return &pol
}

func createBusinessPolicy(service businesspolicy.ServiceRef, p map[string]string, c []string) *businesspolicy.BusinessPolicy {
	propList := new(externalpolicy.PropertyList)
	for k, v := range p {
		propList.Add_Property(externalpolicy.Property_Factory(k, v), false)
	}

	bPolicy := businesspolicy.BusinessPolicy{
		Owner:       "me",
		Label:       "my business policy",
		Description: "blah",
		Service:     service,
		Properties:  *propList,
		Constraints: c,
	}
	return &bPolicy
}

func createService(sUrl string, sOrg string, sVersion string, sArch string, deployment interface{}, clusterDeployment interface{}) common.ServiceFile {
	service := common.ServiceFile{URL: sUrl, Version: sVersion, Arch: sArch, Deployment: deployment, ClusterDeployment: clusterDeployment}
	return service
}

// exchange handlers
func getSelectedServicesHandler(services map[string]exchange.ServiceDefinition) exchange.SelectedServicesHandler {
	return func(wUrl string, wOrg string, wVersion string, wArch string) (map[string]exchange.ServiceDefinition, error) {
		services_to_ret := map[string]exchange.ServiceDefinition{}
		for k, s := range services {
			if s.URL == wUrl && s.Version == wVersion {
				services_to_ret[k] = s
			}
		}

		return services_to_ret, nil
	}
}

func getSelectedServicesHandler_Error() exchange.SelectedServicesHandler {
	return func(wUrl string, wOrg string, wVersion string, wArch string) (map[string]exchange.ServiceDefinition, error) {
		return nil, fmt.Errorf("error getting services")
	}
}

func getDeviceHandler(arch string) exchange.DeviceHandler {
	return func(id string, token string) (*exchange.Device, error) {
		dev := exchange.Device{
			Token:    "xxxx",
			Name:     id,
			Owner:    "me",
			Arch:     arch,
			NodeType: "device",
		}
		return &dev, nil
	}
}

func getDeviceHandler_Error() exchange.DeviceHandler {
	return func(id string, token string) (*exchange.Device, error) {
		return nil, fmt.Errorf("error getting node for %v", id)
	}
}

func getNodePolicyHandler(p map[string]string, c []string) exchange.NodePolicyHandler {
	return func(deviceId string) (*exchange.ExchangePolicy, error) {
		propList := new(externalpolicy.PropertyList)
		for k, v := range p {
			propList.Add_Property(externalpolicy.Property_Factory(k, v), false)
		}

		nodePol := createExternalPolicy(p, c)
		return &exchange.ExchangePolicy{*nodePol, "11-14-2019:03:45"}, nil
	}
}

func getNodePolicyHandler_Error() exchange.NodePolicyHandler {
	return func(deviceId string) (*exchange.ExchangePolicy, error) {
		return nil, fmt.Errorf("error getting node policy for %v", deviceId)
	}
}

func getBusinessPolicyHandler(service businesspolicy.ServiceRef, p map[string]string, c []string) exchange.BusinessPoliciesHandler {
	return func(org string, id string) (map[string]exchange.ExchangeBusinessPolicy, error) {

		bPolicy := createBusinessPolicy(service, p, c)

		exchBP := exchange.ExchangeBusinessPolicy{*bPolicy, "11-14-2019:03:45", "11-14-2019:03:45"}

		return map[string]exchange.ExchangeBusinessPolicy{fmt.Sprintf("%v/%v", org, id): exchBP}, nil
	}
}

func getBusinessPolicyHandler_Error() exchange.BusinessPoliciesHandler {
	return func(org string, id string) (map[string]exchange.ExchangeBusinessPolicy, error) {
		return nil, fmt.Errorf("error getting business policy for %v/%v", org, id)
	}
}

func getServicePolicyHandler(p map[string]string, c []string) exchange.ServicePolicyHandler {
	return func(sUrl string, sOrg string, sVersion string, sArch string) (*exchange.ExchangePolicy, string, error) {
		servicePol := createExternalPolicy(p, c)

		sId := cutil.FormExchangeIdForService(sUrl, sVersion, sArch)
		sId = fmt.Sprintf("%v/%v", sOrg, sId)
		return &exchange.ExchangePolicy{*servicePol, "11-14-2019:03:45"}, sId, nil
	}
}

func getServicePolicyHandler_Error() exchange.ServicePolicyHandler {
	return func(sUrl string, sOrg string, sVersion string, sArch string) (*exchange.ExchangePolicy, string, error) {
		return nil, "", fmt.Errorf("error getting service for %v/%v", sOrg, sUrl)
	}
}

func getServiceDefResolverHandler() exchange.ServiceDefResolverHandler {
	return func(sUrl string, sOrg string, sVersion string, sArch string) (*policy.APISpecList, map[string]exchange.ServiceDefinition, *exchange.ServiceDefinition, string, error) {
		sId := cutil.FormExchangeIdForService(sUrl, sVersion, sArch)
		sId = fmt.Sprintf("%v/%v", sOrg, sId)
		service := &exchange.ServiceDefinition{URL: sUrl, Version: sVersion, Arch: sArch, Deployment: "{\"services\":{\"cpu\":{\"image\":\"cpu:1.2.2\"}}}"}
		return nil, map[string]exchange.ServiceDefinition{}, service, sId, nil
	}
}
