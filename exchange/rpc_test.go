// +build unit

package exchange

import (
	"encoding/json"
	"fmt"
	"testing"
)

func Test_Blockchain_Demarshal(t *testing.T) {
	// Simulate the detail string that we get from the exchange on a call to get details for a given blockchain type and name
	details := `{"chains":[{"arch":"amd64","deployment_description":{"deployment":"{\"services\":{\"geth\":{\"environment\":[\"CHAIN=bluehorizon\"],\"image\":\"summit.hovitos.engineering/x86_64/geth:1.5.7\",\"command\":[\"start.sh\"]}}}","deployment_signature":"abcdefg","deployment_user_info":"","torrent":{"url":"https://images.bluehorizon.network/f27f762cef632af1a19cd8a761ac4c3da4f9ef7d.torrent","images":[{"file":"f27f762cef632af1a19cd8a761ac4c3da4f9ef7d.tar.gz","signature":"123456"}]}}}]}`

	detailsObj := new(BlockchainDetails)
	if err := json.Unmarshal([]byte(details), detailsObj); err != nil {
		t.Errorf("Could not unmarshal details, error %v\n", err)
	} else {
		for _, chain := range detailsObj.Chains {
			if chain.Arch != "amd64" {
				t.Errorf("Could not find amd64 arch in the array.\n")
			} else if chain.DeploymentDesc.Deployment[:6] != `{"serv` {
				t.Errorf("Could not find deployment string %v in %v.\n", `{"serv`, chain.DeploymentDesc.Deployment[:6])
			}
		}
	}

}

func Test_ConvertPattern0(t *testing.T) {

	org := "testorg"
	name := "testpattern"

	pa := `{"label":"Weather","description":"a weather pattern","public":true,` +
		`"workloads":[],` +
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

	pn := makePolicyName(name, "https://bluehorizon.network/workloads/weather", org, "amd64")

	pa := `{"label":"Weather","description":"a weather pattern","public":true,` +
		`"workloads":[` +
		`{"workloadUrl":"https://bluehorizon.network/workloads/weather","workloadOrgid":"testorg","workloadArch":"amd64","workloadVersions":` +
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

	pn := makePolicyName(name, "https://bluehorizon.network/workloads/weather", org, "amd64")

	pa := `{"label":"Weather","description":"a weather pattern","public":true,` +
		`"workloads":[` +
		`{"workloadUrl":"https://bluehorizon.network/workloads/weather","workloadOrgid":"testorg","workloadArch":"amd64","workloadVersions":` +
		`[{"version":"1.5.0",` +
		`"priority":{"priority_value":3,"retries":1,"retry_durations":3600,"verified_durations":52},` +
		`"upgradePolicy":{}},` +
		`{"version":"1.5.0",` +
		`"priority":{"priority_value":2,"retries":1,"retry_durations":3600,"verified_durations": 52},` +
		`"upgradePolicy":{}}],` +
		`"dataVerification":{"enabled":true,"user":"","password":"","URL":"myURL","interval":240,"metering":{"tokens":1,"per_time_unit":"min","notification_interval":30}}},` +
		`{"workloadUrl":"https://bluehorizon.network/workloads/netspeed","workloadOrgid":"testorg","workloadArch":"amd64","workloadVersions":` +
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

	if !isTransportError(error1) {
		t.Errorf("Error: expection isTransportError return true for %v but got false", error1)
	}

	if !isTransportError(error2) {
		t.Errorf("Error: expection isTransportError return true for %v but got false", error2)
	}

	if !isTransportError(error3) {
		t.Errorf("Error: expection isTransportError return true for %v but got false", error3)
	}

	if isTransportError(error4) {
		t.Errorf("Error: expection isTransportError return false for %v but got true", error4)
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
