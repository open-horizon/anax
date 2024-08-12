//go:build unit
// +build unit

package container

import (
	"encoding/json"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/open-horizon/anax/containermessage"
	"os"
	"testing"
)

func Test_UnmarshalNetworkIsolation(t *testing.T) {
	s := `
		{
			"outbound_permit_only": ["foo", "new", {"dd_key": "goo", "encoding": "JSON", "path": "too.zoo"}]
		}
	`

	var n containermessage.NetworkIsolation

	if err := json.Unmarshal([]byte(s), &n); err != nil {
		t.Error(err)
	} else {
		obj := n.OutboundPermitOnly[2].(containermessage.DynamicOutboundPermitValue)

		if obj.DdKey != "goo" || obj.Encoding != containermessage.JSON {
			t.Error("Ill-formed network isolation member")
		}
	}
}

func Test_generatePermittedStringDynamic(t *testing.T) {
	isolation := &containermessage.NetworkIsolation{
		OutboundPermitOnly: []containermessage.OutboundPermitValue{
			containermessage.StaticOutboundPermitValue("198.60.81.209/28"),
			containermessage.StaticOutboundPermitValue("4.2.2.2"),
			containermessage.DynamicOutboundPermitValue{
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

	configure := NewConfigure("", "", "", deploymentUserInfo)

	bytes, _ := json.Marshal(configure)

	permitted, err := generatePermittedString(isolation, containerNetwork, bytes)
	if err != nil {
		t.Error(err)
	} else if permitted != "198.60.81.209/28,4.2.2.2,8.8.8.8,10.55.24.100/24" {
		t.Errorf("Expected permitted string %v, but found %v", "gooo", permitted)
	}
}

func Test_isValidFor_API(t *testing.T) {

	serv1 := containermessage.Service{
		Image:            "an image",
		VariationLabel:   "label",
		Privileged:       true,
		Environment:      []string{"a=1", "b=2"},
		CapAdd:           []string{"a", "b"},
		Command:          []string{"start"},
		Devices:          []string{},
		EphemeralPorts:   []containermessage.Port{},
		NetworkIsolation: &containermessage.NetworkIsolation{},
		Binds:            []string{"/tmp/geth:/root"},
	}

	services := make(map[string]*containermessage.Service)
	services["geth"] = &serv1

	desc := containermessage.DeploymentDescription{
		Services:       services,
		ServicePattern: containermessage.Pattern{},
	}

	if valid := desc.IsValidFor("workload"); !valid {
		t.Errorf("Service1 is valid for a workload, %v", serv1)
	} else if valid := desc.IsValidFor("infrastructure"); !valid {
		t.Errorf("Service1 is valid for infrastructure, %v", serv1)
	}

	serv2 := containermessage.Service{
		Image:            "an image",
		VariationLabel:   "label",
		Privileged:       true,
		Environment:      []string{"a=1", "b=2"},
		CapAdd:           []string{"a", "b"},
		Command:          []string{"start"},
		Devices:          []string{},
		EphemeralPorts:   []containermessage.Port{},
		NetworkIsolation: &containermessage.NetworkIsolation{},
		Ports:            []docker.PortBinding{{HostIP: "0.0.0.0", HostPort: "8545"}},
	}

	services["geth"] = &serv2
	desc2 := containermessage.DeploymentDescription{
		Services:       services,
		ServicePattern: containermessage.Pattern{},
	}

	if valid := desc2.IsValidFor("workload"); !valid {
		t.Errorf("Service2 is valid for a workload, %v", serv2)
	} else if valid := desc2.IsValidFor("infrastructure"); !valid {
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

func Test_CheckPermissions_0(t *testing.T) {
	if err := createTmpDir(); err != nil {
		t.Errorf("Failed to create ./tmp directory with 0777 permission, err: %v", err)
	}

	binds := []string{"./tmp"}

	if err := hasValidBindPermissions(binds); err != nil {
		t.Errorf("Bind is supported, should not have returned an error")
	}

	binds = []string{"/root"}

	if err := hasValidBindPermissions(binds); err == nil {
		t.Errorf("Bind is not supported, should have returned an error.")
	}

	binds = []string{"/root:/hosttmp:ro"}

	if err := hasValidBindPermissions(binds); err == nil {
		t.Errorf("Bind is not supported, should have returned an error.")
	}

	binds = []string{"./tmp:/hosttmp:ro"}

	if err := hasValidBindPermissions(binds); err != nil {
		t.Errorf("Bind is supported, should not have returned an error.")
	}

	binds = []string{"/root:/hosttmp"}

	if err := hasValidBindPermissions(binds); err == nil {
		t.Errorf("Bind is not supported, should have returned an error.")
	}

	binds = []string{"./tmp:/hosttmp"}

	if err := hasValidBindPermissions(binds); err != nil {
		t.Errorf("Bind is supported, should not have returned an error.")
	}

	binds = []string{"/root:/hosttmp:rw"}

	if err := hasValidBindPermissions(binds); err == nil {
		t.Errorf("Bind is not supported, should have returned an error.")
	}

	binds = []string{"./tmp:/hosttmp:rw"}

	if err := hasValidBindPermissions(binds); err != nil {
		t.Errorf("Bind is supported, should not have returned an error.")
	}

	binds = []string{"/not-exist"}

	if err := hasValidBindPermissions(binds); err != nil {
		t.Errorf("Bind is supported, should not have returned an error.")
	}

	binds = []string{"tmpvol:/hosttmp"}

	if err := hasValidBindPermissions(binds); err != nil {
		t.Errorf("Bind is supported, should not have returned an error.")
	}

	os.RemoveAll("./tmp")

}

func createTmpDir() error {
	if err := os.MkdirAll("./tmp", 0777); err != nil {
		return err
	}

	if info, err := os.Stat("./tmp"); err != nil {
		return err
	} else if info.Mode()&4 == 0 || info.Mode()&2 == 0 {
		if err := os.Chmod("./tmp", 0777); err != nil {
			return err
		}
	}
	return nil
}
