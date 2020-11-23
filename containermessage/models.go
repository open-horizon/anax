package containermessage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	docker "github.com/fsouza/go-dockerclient"
	"reflect"
	"strings"
)

/*
 *
 * The external representations of the service deployment string; once processed, the data is stored in a persistence.MicroserviceDefinition object for a service
 *
 * ex:
 * {
 *   "services": {
 *     "service_a": {
 *       "image": "...",
 *       "privileged": true,
 *       "environment": [
 *         "FOO=bar"
 *       ],
 *       "devices": [
 *         "/dev/bus/usb/001/001:/dev/bus/usb/001/001"
 *       ],
 *       "binds": [
 *         "/tmp/testdata:/tmp/mydata:ro",
 *         "myvolume1:/tmp/mydata2"
 *       ],
 *       "ports": [
 *         {
 *           "HostPort":"5200:6414/tcp",
 *           "HostIP": "0.0.0.0"
 *         }
 *       ]
 *     },
 *     "service_b": {
 *       "image": "...",
 *       "network_isolation": {
 *         "outbound_permit_only_ignore": "ETH_ACCT_SPECIFIED",
 *         "outbound_permit_only": [
 *           "4.2.2.2",
 *           "198.60.52.64/26",
 *           {
 *             "dd_key": "deployment_user_info",
 *             "encoding": "JSON",
 *             "path": "cloudMsgBrokerHost.foo.goo"
 *           }
 *         ]
 *       }
 *     }
 *   },
 *   "service_pattern": {
 *     "shared": {
 *       "singleton": [
 *         "service_a",
 *         "service_b"
 *       ]
 *     }
 *   }
 * }
 */

type DeploymentDescription struct {
	Services       map[string]*Service `json:"services"`
	ServicePattern Pattern             `json:"service_pattern"`
	Infrastructure bool                `json:"infrastructure"`
	Overrides      map[string]*Service `json:"overrides"`
}

var invalidDeploymentOptions = map[string][]string{
	"workload":       []string{},
	"infrastructure": []string{},
}

func (d DeploymentDescription) IsValidFor(context string) bool {
	for _, service := range d.Services {
		for _, invalidField := range invalidDeploymentOptions[context] {
			v := reflect.ValueOf(*service)
			fv := v.FieldByName(invalidField)
			switch fv.Type().String() {
			case "[]string":
				if fv.Len() != reflect.Zero(fv.Type()).Len() {
					return false
				}
			case "[]docker.PortBinding":
				if fv.Len() != reflect.Zero(fv.Type()).Len() {
					return false
				}
			case "bool":
				if fv.Bool() {
					return false
				}
			}
		}
	}
	return true
}

func (d DeploymentDescription) ServiceNames() []string {
	names := []string{}

	if d.Services != nil {
		for name, _ := range d.Services {
			names = append(names, name)
		}
	}

	return names
}

type Pattern struct {
	Shared map[string][]string `json:"shared"`
}

type Encoding string

const (
	JSON Encoding = "JSON"
)

func (p *Pattern) IsShared(tp string, serviceName string) bool {
	entries, defined := p.Shared[tp]
	if defined {
		for _, n := range entries {
			if n == serviceName {
				return true
			}
		}
	}

	return false
}

// Service Only those marked "omitempty" may be omitted
type Service struct {
	Image            string               `json:"image"`
	VariationLabel   string               `json:"variation_label,omitempty"`
	Privileged       bool                 `json:"privileged"`
	Network          string               `json:"network"`
	Environment      []string             `json:"environment,omitempty"`
	CapAdd           []string             `json:"cap_add,omitempty"`
	Command          []string             `json:"command,omitempty"`
	Devices          []string             `json:"devices,omitempty"`
	NetworkIsolation *NetworkIsolation    `json:"network_isolation,omitempty"` // Changed to pointer so that the hzn dev CLI doesnt generate this struct into the deployment config skeleton
	Binds            []string             `json:"binds,omitempty"`
	Tmpfs            map[string]string    `json:"tmpfs,omitempty"`
	Ports            []docker.PortBinding `json:"ports,omitempty"`
	EphemeralPorts   []Port               `json:"ephemeral_ports,omitempty"`
	SpecificPorts    []docker.PortBinding `json:"specific_ports,omitempty"` // obselete. for backward compatibility only, new way should use ports instead.
	Entrypoint       []string             `json:"entrypoint,omitempty"`
	MaxMemoryMb      int64                `json:"max_memory_mb,omitempty"`
	MaxCPUs          float32              `json:"max_cpus,omitempty"`
	LogDriver        string               `json:"log_driver,omitempty"`     // Docker's log-driver. Syslog will be used as default driver
}

func (s *Service) AddFilesystemBinding(bind string) {
	if s.Binds == nil {
		s.Binds = make([]string, 0, 10)
	}
	s.Binds = append(s.Binds, bind)
}

func (s *Service) HasSpecificPortBinding() bool {
	if s.SpecificPorts != nil && len(s.SpecificPorts) != 0 {
		return true
	}
	if s.Ports != nil && len(s.Ports) != 0 {
		return true
	}
	return false
}

func GetSpecificHostPort(hostPort string) string {
	p := strings.Split(hostPort, ":")
	if len(p) > 0 {
		port := strings.Split(p[0], "/")[0]
		return port
	}
	return ""
}

func (s *Service) GetSpecificHostPortBinding() string {
	p := ""
	if s.SpecificPorts != nil && len(s.SpecificPorts) != 0 {
		p = s.SpecificPorts[0].HostPort
	} else if s.Ports != nil && len(s.Ports) != 0 {
		p = s.Ports[0].HostPort
	}

	return GetSpecificHostPort(p)
}

func (s *Service) GetSpecificContainerPortBinding() string {
	p := []string{}
	if s.SpecificPorts != nil && len(s.SpecificPorts) != 0 {
		p = strings.Split(s.SpecificPorts[0].HostPort, ":")
	} else if s.Ports != nil && len(s.Ports) != 0 {
		p = strings.Split(s.Ports[0].HostPort, ":")
	}

	if len(p) > 0 {
		if len(p) < 2 {
			port := strings.Split(p[0], "/")[0]
			return port
		} else {
			port := strings.Split(p[1], "/")[0]
			return port
		}
	}

	return ""
}

func (s *Service) GetSpecificHostBinding() string {
	if s.SpecificPorts != nil && len(s.SpecificPorts) != 0 {
		return s.SpecificPorts[0].HostIP
	} else if s.Ports != nil && len(s.Ports) != 0 {
		return s.Ports[0].HostIP
	}
	return ""
}

func (s *Service) AddSpecificPortBinding(b docker.PortBinding) {
	if s.Ports == nil {
		s.Ports = make([]docker.PortBinding, 0, 5)
	}
	s.Ports = append(s.Ports, b)
}

type Port struct {
	LocalhostOnly   bool   `json:"localhost_only,omitempty"`
	PortAndProtocol string `json:"port_and_protocol"`
}

type DynamicOutboundPermitValue struct {
	DdKey    string   `json:"dd_key"`
	Encoding Encoding `json:"encoding"`
	Path     string   `json:"path"`
}

func (d *DynamicOutboundPermitValue) String() string {
	return fmt.Sprintf("ddKey: %v, path: %v", d.DdKey, d.Path)
}

type StaticOutboundPermitValue string

type OutboundPermitOnlyIgnore string

const (
	ETH_ACCT_SPECIFIED OutboundPermitOnlyIgnore = "ETH_ACCT_SPECIFIED"
)

type OutboundPermitValue interface{}

type NetworkIsolation struct {
	OutboundPermitOnlyIgnore OutboundPermitOnlyIgnore `json:"outbound_permit_only_ignore"`
	OutboundPermitOnly       []OutboundPermitValue    `json:"outbound_permit_only"`
}

func (n *NetworkIsolation) UnmarshalJSON(data []byte) error {
	type polyNType struct {
		OutboundPermitOnlyIgnore OutboundPermitOnlyIgnore `json:"outbound_permit_only_ignore,omitempty"`
		OutboundPermitOnly       []json.RawMessage        `json:"outbound_permit_only"`
	}

	var polyN polyNType

	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&polyN); err != nil {
		return err
	}

	n.OutboundPermitOnlyIgnore = polyN.OutboundPermitOnlyIgnore

	// dumb way you have to handle polymorphic types in golang
	for _, permit := range polyN.OutboundPermitOnly {

		var o OutboundPermitValue
		var d DynamicOutboundPermitValue

		dec := json.NewDecoder(bytes.NewReader(permit))
		if err := dec.Decode(&d); err != nil {
			var s StaticOutboundPermitValue
			if err := json.Unmarshal(permit, &s); err != nil {
				return err
			}
			o = s
		} else {
			o = d
		}

		n.OutboundPermitOnly = append(n.OutboundPermitOnly, o)
	}

	return nil
}

// Given a deployment string, unmarshal it as a native Horizon Deployment object. It might not be a native Deployment, so
// we have to verify what was just unmarshalled.
func GetNativeDeployment(depStr string) (*DeploymentDescription, error) {

	dd := new(DeploymentDescription)
	err := json.Unmarshal([]byte(depStr), dd)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("error unmarshalling deployment config as DeploymentDescription: %v", err))
	}

	if len(dd.Services) == 0 {
		return nil, errors.New(fmt.Sprintf("deployment config is not a DeploymentDescription"))
	}

	return dd, nil

}
