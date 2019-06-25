// +build unit

package exchange

import (
	"encoding/json"
	"fmt"
	"testing"
)

func Test_ConvertPattern0(t *testing.T) {

	org := "testorg"
	name := "testpattern"

	pa := `{"label":"Weather","description":"a weather pattern","public":true,` +
		`"services":[],` +
		`"agreementProtocols":[{"name":"Basic"}]}`

	if p1 := create_Pattern(pa, t); p1 == nil {
		t.Errorf("Pattern not created from %v\n", pa)
	} else if pols, err := ConvertToPolicies(fmt.Sprintf("%v/%v", org, name), p1); err != nil {
		t.Errorf("Error: %v converting %v to a policy\n", err, pa)
	} else if len(pols) != 0 {
		t.Errorf("Error: should be 0 policies in the pattern, there are %v\n", len(pols))
	}

}

func Test_ConvertPattern1(t *testing.T) {

	org := "testorg"
	name := "testpattern"

	pn := makePolicyName(name, "https://bluehorizon.network/services/weather", org, "amd64")

	pa := `{"label":"Weather","description":"a weather pattern","public":true,` +
		`"services":[` +
		`{"serviceUrl":"https://bluehorizon.network/services/weather","serviceOrgid":"testorg","serviceArch":"amd64","serviceVersions":` +
		`[{"version":"1.5.0",` +
		`"priority":{"priority_value":3,"retries":1,"retry_durations":3600,"verified_durations":52},` +
		`"upgradePolicy":{}},` +
		`{"version":"1.5.0",` +
		`"priority":{"priority_value":2,"retries":1,"retry_durations":3600,"verified_durations": 52},` +
		`"upgradePolicy":{}}],` +
		`"dataVerification":{"enabled":true,"user":"","password":"","URL":"myURL","interval":240,"metering":{"tokens":1,"per_time_unit":"min","notification_interval":30}},` +
		`"nodeHealth":{"missing_heartbeat_interval":480}}` +
		`],` +
		`"agreementProtocols":[{"name":"Basic"}]}`

	if p1 := create_Pattern(pa, t); p1 == nil {
		t.Errorf("Pattern not created from %v\n", pa)
	} else if pols, err := ConvertToPolicies(fmt.Sprintf("%v/%v", org, name), p1); err != nil {
		t.Errorf("Error: %v converting %v to a policy\n", err, pa)
	} else if pols[0].Header.Name != pn {
		t.Errorf("Error: wrong header name generated, was %v\n", pols[0].Header.Name)
	} else if len(pols) != 1 {
		t.Errorf("Error: should be 1 policies in the pattern, there are %v\n", len(pols))
	} else if len(pols[0].AgreementProtocols) != 1 {
		t.Errorf("Error: should be 1 agreement protocol, but there are %v\n", len(pols[0].AgreementProtocols))
	} else if pols[0].DataVerify.URL != "myURL" {
		t.Errorf("Error: Data verification didnt get setup correctly, is %v\n", pols[0].DataVerify)
	} else if pols[0].NodeH.MissingHBInterval != 480 {
		t.Errorf("Error: Node health policy not converted correctly, is %v", pols[0].NodeH)
	}

}

func Test_ConvertPattern2(t *testing.T) {

	org := "testorg"
	name := "testpattern"

	pn := makePolicyName(name, "https://bluehorizon.network/services/weather", org, "amd64")

	pa := `{"label":"Weather","description":"a weather pattern","public":true,` +
		`"services":[` +
		`{"serviceUrl":"https://bluehorizon.network/services/weather","serviceOrgid":"testorg","serviceArch":"amd64","serviceVersions":` +
		`[{"version":"1.5.0",` +
		`"priority":{"priority_value":3,"retries":1,"retry_durations":3600,"verified_durations":52},` +
		`"upgradePolicy":{}},` +
		`{"version":"1.5.0",` +
		`"priority":{"priority_value":2,"retries":1,"retry_durations":3600,"verified_durations": 52},` +
		`"upgradePolicy":{}}],` +
		`"dataVerification":{"enabled":true,"user":"","password":"","URL":"myURL","interval":240,"metering":{"tokens":1,"per_time_unit":"min","notification_interval":30}}},` +
		`{"serviceUrl":"https://bluehorizon.network/services/netspeed","serviceOrgid":"testorg","serviceArch":"amd64","serviceVersions":` +
		`[{"version":"1.5.0",` +
		`"priority":{"priority_value":3,"retries":1,"retry_durations":3600,"verified_durations":52},` +
		`"upgradePolicy":{}},` +
		`{"version":"1.5.0",` +
		`"priority":{"priority_value":2,"retries":1,"retry_durations":3600,"verified_durations": 52},` +
		`"upgradePolicy":{}}],` +
		`"dataVerification":{"enabled":true,"user":"","password":"","URL":"myURL","interval":240,"metering":{"tokens":1,"per_time_unit":"min","notification_interval":30}}}` +
		`],` +
		`"agreementProtocols":[{"name":"Basic"}]}`

	if p1 := create_Pattern(pa, t); p1 == nil {
		t.Errorf("Pattern not created from %v\n", pa)
	} else if pols, err := ConvertToPolicies(fmt.Sprintf("%v/%v", org, name), p1); err != nil {
		t.Errorf("Error: %v converting %v to a policy\n", err, pa)
	} else if pols[0].Header.Name != pn {
		t.Errorf("Error: wrong header name generated, was %v\n", pols[0].Header.Name)
	} else if len(pols) != 2 {
		t.Errorf("Error: should be 2 policies in the pattern, there are %v\n", len(pols))
	} else if len(pols[0].AgreementProtocols) != 1 {
		t.Errorf("Error: should be 1 agreement protocol, but there are %v\n", len(pols[0].AgreementProtocols))
	} else if pols[0].DataVerify.URL != "myURL" {
		t.Errorf("Error: Data verification didnt get setup correctly, is %v\n", pols[0].DataVerify)
	}

}

func Test_ConvertPattern3(t *testing.T) {

	org := "testorg"
	name := "testpattern"

	pa_no_wlurl := `{"label":"Weather","description":"a weather pattern","public":true,` +
		`"services":[` +
		`{"serviceUrl":"","serviceOrgid":"testorg","serviceArch":"amd64","serviceVersions":` +
		`[{"version":"1.5.0",` +
		`"priority":{"priority_value":3,"retries":1,"retry_durations":3600,"verified_durations":52},` +
		`"upgradePolicy":{}},` +
		`{"version":"1.5.0",` +
		`"priority":{"priority_value":2,"retries":1,"retry_durations":3600,"verified_durations": 52},` +
		`"upgradePolicy":{}}],` +
		`"dataVerification":{"enabled":true,"user":"","password":"","URL":"myURL","interval":240,"metering":{"tokens":1,"per_time_unit":"min","notification_interval":30}},` +
		`"nodeHealth":{"missing_heartbeat_interval":480}}` +
		`],` +
		`"agreementProtocols":[{"name":"Basic"}]}`

	pa_no_wlarch := `{"label":"Weather","description":"a weather pattern","public":true,` +
		`"services":[` +
		`{"serviceUrl":"https://bluehorizon.network/services/weather","serviceOrgid":"testorg","serviceArch":"","serviceVersions":` +
		`[{"version":"1.5.0",` +
		`"priority":{"priority_value":3,"retries":1,"retry_durations":3600,"verified_durations":52},` +
		`"upgradePolicy":{}},` +
		`{"version":"1.5.0",` +
		`"priority":{"priority_value":2,"retries":1,"retry_durations":3600,"verified_durations": 52},` +
		`"upgradePolicy":{}}],` +
		`"dataVerification":{"enabled":true,"user":"","password":"","URL":"myURL","interval":240,"metering":{"tokens":1,"per_time_unit":"min","notification_interval":30}},` +
		`"nodeHealth":{"missing_heartbeat_interval":480}}` +
		`],` +
		`"agreementProtocols":[{"name":"Basic"}]}`

	pa_no_wlorg := `{"label":"Weather","description":"a weather pattern","public":true,` +
		`"services":[` +
		`{"serviceUrl":"https://bluehorizon.network/services/weather","serviceOrgid":"","serviceArch":"amd64","serviceVersions":` +
		`[{"version":"1.5.0",` +
		`"priority":{"priority_value":3,"retries":1,"retry_durations":3600,"verified_durations":52},` +
		`"upgradePolicy":{}},` +
		`{"version":"1.5.0",` +
		`"priority":{"priority_value":2,"retries":1,"retry_durations":3600,"verified_durations": 52},` +
		`"upgradePolicy":{}}],` +
		`"dataVerification":{"enabled":true,"user":"","password":"","URL":"myURL","interval":240,"metering":{"tokens":1,"per_time_unit":"min","notification_interval":30}},` +
		`"nodeHealth":{"missing_heartbeat_interval":480}}` +
		`],` +
		`"agreementProtocols":[{"name":"Basic"}]}`

	pa_empty_wl_versions := `{"label":"Weather","description":"a weather pattern","public":true,` +
		`"services":[` +
		`{"serviceUrl":"https://bluehorizon.network/services/weather","serviceOrgid":"testorg","serviceArch":"amd64","serviceVersions":` +
		`[],` +
		`"dataVerification":{"enabled":true,"user":"","password":"","URL":"myURL","interval":240,"metering":{"tokens":1,"per_time_unit":"min","notification_interval":30}},` +
		`"nodeHealth":{"missing_heartbeat_interval":480}}` +
		`],` +
		`"agreementProtocols":[{"name":"Basic"}]}`

	pa_no_wlversion := `{"label":"Weather","description":"a weather pattern","public":true,` +
		`"services":[` +
		`{"serviceUrl":"https://bluehorizon.network/services/weather","serviceOrgid":"testorg","serviceArch":"amd64","serviceVersions":` +
		`[{"version":"",` +
		`"priority":{"priority_value":3,"retries":1,"retry_durations":3600,"verified_durations":52},` +
		`"upgradePolicy":{}},` +
		`{"version":"1.5.0",` +
		`"priority":{"priority_value":2,"retries":1,"retry_durations":3600,"verified_durations": 52},` +
		`"upgradePolicy":{}}],` +
		`"dataVerification":{"enabled":true,"user":"","password":"","URL":"myURL","interval":240,"metering":{"tokens":1,"per_time_unit":"min","notification_interval":30}},` +
		`"nodeHealth":{"missing_heartbeat_interval":480}}` +
		`],` +
		`"agreementProtocols":[{"name":"Basic"}]}`

	if p_no_wlurl := create_Pattern(pa_no_wlurl, t); p_no_wlurl == nil {
		t.Errorf("Pattern not created from %v\n", pa_no_wlurl)
	} else if _, err := ConvertToPolicies(fmt.Sprintf("%v/%v", org, name), p_no_wlurl); err == nil {
		t.Errorf("Error: Should get error for converting %v to a policy, but did not\n", pa_no_wlurl)
	}

	if p_no_wlarch := create_Pattern(pa_no_wlarch, t); p_no_wlarch == nil {
		t.Errorf("Pattern not created from %v\n", pa_no_wlarch)
	} else if _, err := ConvertToPolicies(fmt.Sprintf("%v/%v", org, name), p_no_wlarch); err == nil {
		t.Errorf("Error: Should get error for converting %v to a policy, but did not\n", pa_no_wlarch)
	}

	if p_no_wlorg := create_Pattern(pa_no_wlorg, t); p_no_wlorg == nil {
		t.Errorf("Pattern not created from %v\n", pa_no_wlarch)
	} else if _, err := ConvertToPolicies(fmt.Sprintf("%v/%v", org, name), p_no_wlorg); err == nil {
		t.Errorf("Error: Should get error for converting %v to a policy, but did not\n", pa_no_wlorg)
	}

	if p_empty_wl_versions := create_Pattern(pa_empty_wl_versions, t); p_empty_wl_versions == nil {
		t.Errorf("Pattern not created from %v\n", pa_empty_wl_versions)
	} else if _, err := ConvertToPolicies(fmt.Sprintf("%v/%v", org, name), p_empty_wl_versions); err == nil {
		t.Errorf("Error: Should get error for converting %v to a policy, but did not\n", pa_empty_wl_versions)
	}

	if p_no_wlversion := create_Pattern(pa_no_wlversion, t); p_no_wlversion == nil {
		t.Errorf("Pattern not created from %v\n", pa_no_wlarch)
	} else if _, err := ConvertToPolicies(fmt.Sprintf("%v/%v", org, name), p_no_wlversion); err == nil {
		t.Errorf("Error: Should get error for converting %v to a policy, but did not\n", pa_no_wlversion)
	}
}

func Test_makePolicyName1(t *testing.T) {

	pat := "pat1"
	url := "http://mydomain.com"
	org := "myorg"
	arch := "amd64"

	expected := fmt.Sprintf("%v_%v_%v_%v", pat, "mydomain.com", org, arch)

	if res := makePolicyName(pat, url, org, arch); res != expected {
		t.Errorf("Error: expecting %v got %v\n", expected, res)
	}

}

func Test_makePolicyName2(t *testing.T) {

	pat := "pat1"
	url := ""
	org := "myorg"
	arch := "amd64"

	expected := fmt.Sprintf("%v_%v_%v_%v", pat, "", org, arch)

	if res := makePolicyName(pat, url, org, arch); res != expected {
		t.Errorf("Error: expecting %v got %v\n", expected, res)
	}

}

func Test_IsTraportError(t *testing.T) {
	error1 := fmt.Errorf("Time is out")
	error2 := fmt.Errorf("connection is refused")
	error3 := fmt.Errorf("connection reset by the peer")
	error4 := fmt.Errorf("something is wrong")

	if !IsTransportError(error1) {
		t.Errorf("Error: expection IsTransportError return true for %v but got false", error1)
	}

	if !IsTransportError(error2) {
		t.Errorf("Error: expection IsTransportError return true for %v but got false", error2)
	}

	if !IsTransportError(error3) {
		t.Errorf("Error: expection IsTransportError return true for %v but got false", error3)
	}

	if IsTransportError(error4) {
		t.Errorf("Error: expection IsTransportError return false for %v but got true", error4)
	}

}

// Create a Pattern object from a JSON serialization. The JSON serialization
// does not have to be a valid pattern serialization, just has to be a valid
// JSON serialization.
func create_Pattern(jsonString string, t *testing.T) *Pattern {
	wl := new(Pattern)

	if err := json.Unmarshal([]byte(jsonString), &wl); err != nil {
		t.Errorf("Error unmarshalling pattern json string: %v error:%v\n", jsonString, err)
		return nil
	} else {
		return wl
	}
}
