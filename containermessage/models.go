package containermessage

import (
	"bytes"
	"encoding/json"
	"fmt"
	docker "github.com/fsouza/go-dockerclient"
	"reflect"
	"strings"
)

/*
 *
 * The external representations of the config; once processed, the data about the pattern is stored in a persistence.ServiceConfig object
 *
 * ex:
 * {
 *   "services": {
 *     "service_a": {
 *       "image": "..."
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
	Environment      []string             `json:"environment,omitempty"`
	CapAdd           []string             `json:"cap_add,omitempty"`
	Command          []string             `json:"command,omitempty"`
	Devices          []string             `json:"devices,omitempty"`
	Ports            []Port               `json:"ports,omitempty"`
	NetworkIsolation *NetworkIsolation    `json:"network_isolation,omitempty"` // Changed to pointer so that the hzn dev CLI doesnt generate this struct into the deployment config skeleton
	Binds            []string             `json:"binds,omitempty"`             // Only used by infrastructure containers
	SpecificPorts    []docker.PortBinding `json:"specific_ports,omitempty"`    // Only used by infrastructure containers
}

func (s *Service) AddFilesystemBinding(bind string) {
	if s.Binds == nil {
		s.Binds = make([]string, 0, 10)
	}
	s.Binds = append(s.Binds, bind)
}

func (s *Service) HasSpecificPortBinding() bool {
	if s.SpecificPorts == nil {
		return false
	}
	if len(s.SpecificPorts) != 0 {
		return true
	}
	return false
}

func (s *Service) GetSpecificHostPortBinding() string {
	if s.SpecificPorts == nil {
		return ""
	} else if len(s.SpecificPorts) == 0 {
		return ""
	} else {
		p := strings.Split(s.SpecificPorts[0].HostPort, ":")
		port := strings.Split(p[0], "/")[0]
		return port
	}
}

func (s *Service) GetSpecificContainerPortBinding() string {
	if s.SpecificPorts == nil {
		return ""
	} else if len(s.SpecificPorts) == 0 {
		return ""
	} else {
		p := strings.Split(s.SpecificPorts[0].HostPort, ":")
		if len(p) < 2 {
			port := strings.Split(p[0], "/")[0]
			return port
		} else {
			port := strings.Split(p[1], "/")[0]
			return port
		}
	}
}

func (s *Service) GetSpecificHostBinding() string {
	if s.SpecificPorts == nil {
		return ""
	} else if len(s.SpecificPorts) == 0 {
		return ""
	} else {
		return s.SpecificPorts[0].HostIP
	}
}

func (s *Service) AddSpecificPortBinding(b docker.PortBinding) {
	if s.SpecificPorts == nil {
		s.SpecificPorts = make([]docker.PortBinding, 0, 5)
	}
	s.SpecificPorts = append(s.SpecificPorts, b)
}

type Port struct {
	LocalhostOnly   bool   `json:"localhost_only"`
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
