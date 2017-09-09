package container

import (
	"encoding/json"
	docker "github.com/fsouza/go-dockerclient"
	"net/url"
	"testing"
)

func Test_UnmarshalNetworkIsolation(t *testing.T) {
	s := `
		{
			"outbound_permit_only": ["foo", "new", {"dd_key": "goo", "encoding": "JSON", "path": "too.zoo"}]
		}
	`

	var n NetworkIsolation

	if err := json.Unmarshal([]byte(s), &n); err != nil {
		t.Error(err)
	} else {
		obj := n.OutboundPermitOnly[2].(DynamicOutboundPermitValue)

		if obj.DdKey != "goo" || obj.Encoding != JSON {
			t.Error("Ill-formed network isolation member")
		}
	}
}

func Test_generatePermittedStringDynamic(t *testing.T) {
	isolation := &NetworkIsolation{
		OutboundPermitOnly: []OutboundPermitValue{
			StaticOutboundPermitValue("198.60.81.209/28"),
			StaticOutboundPermitValue("4.2.2.2"),
			DynamicOutboundPermitValue{
				DdKey:    "deployment_user_info",
				Encoding: "JSON",
				Path:     "quarks.externalBrokerHost",
			},
		},
	}

	containerNetwork := docker.ContainerNetwork{
		IPAddress:   "10.55.24.100",
		IPPrefixLen: 24,
	}

	deploymentUserInfo := `
		{
			"quarks": {
				"externalBrokerHost": "8.8.8.8"
			}
		}
	`

	url, _ := url.Parse("http://goo.foo")

	configure := NewConfigure("", *url, map[string]string{}, map[string]string{}, "", "", deploymentUserInfo)

	bytes, _ := json.Marshal(configure)

	permitted, err := generatePermittedString(isolation, containerNetwork, bytes)
	if err != nil {
		t.Error(err)
	} else if permitted != "198.60.81.209/28,4.2.2.2,8.8.8.8,10.55.24.100/24" {
		t.Errorf("Expected permitted string %v, but found %v", "gooo", permitted)
	}
}

func Test_isValidFor_API(t *testing.T) {

	serv1 := Service{
		Image:            "an image",
		VariationLabel:   "label",
		Privileged:       true,
		Environment:      []string{"a=1", "b=2"},
		CapAdd:           []string{"a", "b"},
		Command:          []string{"start"},
		Devices:          []string{},
		Ports:            []Port{},
		NetworkIsolation: NetworkIsolation{},
		Binds:            []string{"/tmp/geth:/root"},
	}

	services := make(map[string]*Service)
	services["geth"] = &serv1

	desc := DeploymentDescription{
		Services:       services,
		ServicePattern: Pattern{},
	}

	if valid := desc.isValidFor("workload"); valid {
		t.Errorf("Service1 is not valid for a workload, %v", serv1)
	} else if valid := desc.isValidFor("infrastructure"); !valid {
		t.Errorf("Service1 is valid for infrastructure, %v", serv1)
	}

	serv2 := Service{
		Image:            "an image",
		VariationLabel:   "label",
		Privileged:       true,
		Environment:      []string{"a=1", "b=2"},
		CapAdd:           []string{"a", "b"},
		Command:          []string{"start"},
		Devices:          []string{},
		Ports:            []Port{},
		NetworkIsolation: NetworkIsolation{},
		SpecificPorts:    []docker.PortBinding{{HostIP: "0.0.0.0", HostPort: "8545"}},
	}

	services["geth"] = &serv2
	desc2 := DeploymentDescription{
		Services:       services,
		ServicePattern: Pattern{},
	}

	if valid := desc2.isValidFor("workload"); valid {
		t.Errorf("Service2 is not valid for a workload, %v", serv2)
	} else if valid := desc2.isValidFor("infrastructure"); !valid {
		t.Errorf("Service2 is valid for infrastructure, %v", serv2)
	}
}

func Test_RemoveEnvVar_success1(t *testing.T) {

	e1 := "a=b"
	e2 := "c=d"
	e3 := "e=f"

	eList := []string{e1, e2, e3}

	removeDuplicateVariable(&eList, "c=5")
	if eList[0] != e1 && eList[1] != e3 {
		t.Errorf("Env var list should contain %v %v, but is %v\n", e1, e3, eList)
	}

}

func Test_RemoveEnvVar_success2(t *testing.T) {

	e1 := "a=b"
	e2 := "c=d"
	e3 := "e=f"

	eList := []string{e1, e2, e3}

	removeDuplicateVariable(&eList, "a=2")
	if eList[0] != e2 && eList[1] != e3 {
		t.Errorf("Env var list should contain %v %v, but is %v\n", e2, e3, eList)
	}

}

func Test_RemoveEnvVar_success3(t *testing.T) {

	e1 := "a=b"
	e2 := "c=d"
	e3 := "e=f"

	eList := []string{e1, e2, e3}

	removeDuplicateVariable(&eList, "e=11")
	if eList[0] != e1 && eList[1] != e2 {
		t.Errorf("Env var list should contain %v %v, but is %v\n", e1, e2, eList)
	}

}

func Test_RemoveEnvVar_nothing1(t *testing.T) {

	e1 := "a=b"
	e2 := "c=d"
	e3 := "e=f"

	eList := []string{e1, e2, e3}

	removeDuplicateVariable(&eList, "b=3")
	if eList[0] != e1 && eList[1] != e2 && eList[2] != e3 {
		t.Errorf("Env var list should contain %v %v %v, but is %v\n", e1, e2, e3, eList)
	}

}

func Test_RemoveEnvVar_nothing2(t *testing.T) {

	eList := []string{}

	removeDuplicateVariable(&eList, "b=3")
	if len(eList) != 0 {
		t.Errorf("Env var list should be empty, but is %v\n", eList)
	}

}

func Test_RemoveEnvVar_nothing3(t *testing.T) {

	e1 := "a=b"
	e2 := "c=d"
	e3 := "e=f"

	eList := []string{e1, e2, e3}

	removeDuplicateVariable(&eList, "ab=3")
	if eList[0] != e1 && eList[1] != e2 && eList[2] != e3 {
		t.Errorf("Env var list should contain %v %v %v, but is %v\n", e1, e2, e3, eList)
	}

}
