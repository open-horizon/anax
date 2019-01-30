// +build unit

package containermessage

import (
	docker "github.com/fsouza/go-dockerclient"
	"testing"
)

func Test_HasSpecificPortBinding(t *testing.T) {
	serv := Service{
		Image:            "an image",
		VariationLabel:   "label",
		Privileged:       true,
		Environment:      []string{"a=1", "b=2"},
		CapAdd:           []string{"a", "b"},
		Command:          []string{"start"},
		Devices:          []string{},
		EphemeralPorts:   []Port{{LocalhostOnly: true, PortAndProtocol: "12345/tcp"}},
		NetworkIsolation: &NetworkIsolation{},
		Ports:            []docker.PortBinding{},
	}

	if serv.HasSpecificPortBinding() {
		t.Errorf("HasSpecificPortBinding for service %v should not have returned true.", serv)
	}

	serv.Ports = []docker.PortBinding{{HostIP: "0.0.0.0", HostPort: "8545"}}

	if !serv.HasSpecificPortBinding() {
		t.Errorf("HasSpecificPortBinding for service %v should not have returned false.", serv)
	}

	serv.Ports = []docker.PortBinding{}
	serv.SpecificPorts = []docker.PortBinding{{HostIP: "0.0.0.0", HostPort: "8545"}}
	if !serv.HasSpecificPortBinding() {
		t.Errorf("HasSpecificPortBinding for service %v should not have returned false.", serv)
	}
}

func Test_GetSpecificHostPortBinding(t *testing.T) {
	serv := Service{
		Image:            "an image",
		VariationLabel:   "label",
		Privileged:       true,
		Environment:      []string{"a=1", "b=2"},
		CapAdd:           []string{"a", "b"},
		Command:          []string{"start"},
		Devices:          []string{},
		EphemeralPorts:   []Port{{LocalhostOnly: true, PortAndProtocol: "12345/tcp"}},
		NetworkIsolation: &NetworkIsolation{},
		Ports:            []docker.PortBinding{},
	}

	hostp := serv.GetSpecificHostPortBinding()
	if hostp != "" {
		t.Errorf("GetSpecificHostPortBinding for service should return an empty string but got %v.", hostp)
	}

	serv.Ports = []docker.PortBinding{{HostIP: "0.0.0.0", HostPort: "1111:2222/tcp"}, {HostIP: "127.0.0.1", HostPort: "3333"}}

	hostp = serv.GetSpecificHostPortBinding()
	if hostp != "1111" {
		t.Errorf("GetSpecificHostPortBinding for service should return 1111 but got %v.", hostp)
	}

	serv.Ports = []docker.PortBinding{}
	serv.SpecificPorts = []docker.PortBinding{{HostIP: "0.0.0.0", HostPort: "8545"}}
	hostp = serv.GetSpecificHostPortBinding()
	if hostp != "8545" {
		t.Errorf("GetSpecificHostPortBinding for service should return 8545 but got %v.", hostp)
	}
}

func Test_GetSpecificContainerPortBinding(t *testing.T) {
	serv := Service{
		Image:            "an image",
		VariationLabel:   "label",
		Privileged:       true,
		Environment:      []string{"a=1", "b=2"},
		CapAdd:           []string{"a", "b"},
		Command:          []string{"start"},
		Devices:          []string{},
		EphemeralPorts:   []Port{{LocalhostOnly: true, PortAndProtocol: "12345/tcp"}},
		NetworkIsolation: &NetworkIsolation{},
		Ports:            []docker.PortBinding{},
	}

	containerp := serv.GetSpecificContainerPortBinding()
	if containerp != "" {
		t.Errorf("GetSpecificContainerPortBinding for service should return an empty string but got %v.", containerp)
	}

	serv.Ports = []docker.PortBinding{{HostIP: "0.0.0.0", HostPort: "1111:2222/tcp"}, {HostIP: "127.0.0.1", HostPort: "3333"}}

	containerp = serv.GetSpecificContainerPortBinding()
	if containerp != "2222" {
		t.Errorf("GetSpecificContainerPortBinding for service should return 2222 but got %v.", containerp)
	}

	serv.Ports = []docker.PortBinding{}
	serv.SpecificPorts = []docker.PortBinding{{HostIP: "0.0.0.0", HostPort: "8545"}}
	containerp = serv.GetSpecificContainerPortBinding()
	if containerp != "8545" {
		t.Errorf("GetSpecificContainerPortBinding for service should return 8545 but got %v.", containerp)
	}
}

func Test_GetSpecificHostBinding(t *testing.T) {
	serv := Service{
		Image:            "an image",
		VariationLabel:   "label",
		Privileged:       true,
		Environment:      []string{"a=1", "b=2"},
		CapAdd:           []string{"a", "b"},
		Command:          []string{"start"},
		Devices:          []string{},
		EphemeralPorts:   []Port{{LocalhostOnly: true, PortAndProtocol: "12345/tcp"}},
		NetworkIsolation: &NetworkIsolation{},
		Ports:            []docker.PortBinding{},
	}

	host_ip := serv.GetSpecificHostBinding()
	if host_ip != "" {
		t.Errorf("GetSpecificHostBinding for service should return an empty string but got %v.", host_ip)
	}

	serv.Ports = []docker.PortBinding{{HostIP: "0.0.0.0", HostPort: "1111:2222/tcp"}, {HostIP: "127.0.0.1", HostPort: "3333"}}

	host_ip = serv.GetSpecificHostBinding()
	if host_ip != "0.0.0.0" {
		t.Errorf("GetSpecificHostBinding for service should return 0.0.0.0 but got %v.", host_ip)
	}

	serv.Ports = []docker.PortBinding{}
	serv.SpecificPorts = []docker.PortBinding{{HostIP: "1.0.2.0", HostPort: "8545"}}
	host_ip = serv.GetSpecificHostBinding()
	if host_ip != "1.0.2.0" {
		t.Errorf("GetSpecificHostBinding for service should return 1.0.2.0 but got %v.", host_ip)
	}
}

func Test_AddSpecificPortBinding(t *testing.T) {
	serv := Service{
		Image:            "an image",
		VariationLabel:   "label",
		Privileged:       true,
		Environment:      []string{"a=1", "b=2"},
		CapAdd:           []string{"a", "b"},
		Command:          []string{"start"},
		Devices:          []string{},
		EphemeralPorts:   []Port{{LocalhostOnly: true, PortAndProtocol: "12345/tcp"}},
		NetworkIsolation: &NetworkIsolation{},
		Ports:            []docker.PortBinding{},
	}

	if serv.HasSpecificPortBinding() {
		t.Errorf("HasSpecificPortBinding for service %v should not have returned true.", serv)
	}

	serv.AddSpecificPortBinding(docker.PortBinding{HostIP: "0.0.0.0", HostPort: "1111:2222/tcp"})
	serv.AddSpecificPortBinding(docker.PortBinding{HostIP: "127.0.0.1", HostPort: "3333"})

	hostp := serv.GetSpecificHostPortBinding()
	if hostp != "1111" {
		t.Errorf("GetSpecificHostPortBinding for service should return 1111 but got %v.", hostp)
	}
	host_ip := serv.GetSpecificHostBinding()
	if host_ip != "0.0.0.0" {
		t.Errorf("GetSpecificHostBinding for service should return 0.0.0.0 but got %v.", host_ip)
	}
	containerp := serv.GetSpecificContainerPortBinding()
	if containerp != "2222" {
		t.Errorf("GetSpecificContainerPortBinding for service should return 2222 but got %v.", containerp)
	}

	if len(serv.Ports) != 2 {
		t.Errorf("Service should have 2 specific port bindings but not.")
	}
}
