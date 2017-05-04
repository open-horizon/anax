package container

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/coreos/go-iptables/iptables"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/citizenscientist"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/worker"
	"golang.org/x/sys/unix"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"path"
	"reflect"
	"runtime"
	"strconv"
	"strings"
)

const LABEL_PREFIX = "network.bluehorizon.colonus"
const IPT_COLONUS_ISOLATED_CHAIN = "COLONUS-ISOLATION"

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
}

var invalidDeploymentOptions = map[string][]string {
	"workload": []string{"Binds","SpecificPorts"},
	"infrastructure": []string{},
}

func (d DeploymentDescription) isValidFor(context string) bool {
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

func (d DeploymentDescription) serviceNames() []string {
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

func (p *Pattern) isShared(tp string, serviceName string) bool {
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
	NetworkIsolation NetworkIsolation     `json:"network_isolation,omitempty"`
	Binds            []string             `json:"binds,omitempty"`            // Only used by infrastructure containers
	SpecificPorts    []docker.PortBinding `json:"specific_ports,omitempty"`   // Only used by infrastructure containers
}

func (s *Service) addFilesystemBinding(bind string) {
	if s.Binds == nil {
		s.Binds = make([]string, 0, 10)
	}
	s.Binds = append(s.Binds, bind)
}

func(s *Service) addSpecificPortBinding(b docker.PortBinding) {
	if s.SpecificPorts == nil {
		s.SpecificPorts = make([]docker.PortBinding, 0, 5)
	}
	s.SpecificPorts = append(s.SpecificPorts, b)
}

type Port struct {
	LocalhostOnly   bool   `json:"localhost_only"`
	PortAndProtocol string `json:"port_and_protocol"`
}

// an internal convenience type
type servicePair struct {
	service       *Service                   // the external type
	serviceConfig *persistence.ServiceConfig // the internal type
}

func hashService(service *Service) (string, error) {
	if service == nil {
		return "", errors.New("required service ref not provided")
	}

	b, err := json.Marshal(service)
	if err != nil {
		return "", err
	}

	glog.V(5).Infof("Hashing Service: %v", string(b))

	h := sha1.New()
	if _, err := io.Copy(h, bytes.NewBuffer(b)); err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(h.Sum(nil)), nil
}

func finalizeDeployment(agreementId string, deployment *DeploymentDescription, environmentAdditions map[string]string, workloadROStorageDir string, cpuSet string) (map[string]servicePair, error) {

	// final structure
	services := make(map[string]servicePair, 0)

	var ramBytes int64

	// we know that RAM is in MB
	if ram, exists := (environmentAdditions)[config.ENVVAR_PREFIX+"RAM"]; !exists {
		return nil, fmt.Errorf("Missing required environment var *RAM for agreement: %v", agreementId)
	} else {
		ramMB, err := strconv.ParseInt(ram, 10, 64)
		if err != nil {
			return nil, err
		}
		ramBytes = ramMB * 1024 * 1024
	}

	if len(deployment.Services) == 0 {
		return nil, fmt.Errorf("No services specified in pattern: %v", deployment)
	}

	for serviceName, service := range deployment.Services {
		deploymentHash, err := hashService(service)
		if err != nil {
			return nil, err
		}

		// Assume only 1 filesystem binding
		hostVol := ""
		hostVolIndex := strings.Index(service.Binds[0], ":")
		if hostVolIndex != -1 {
			hostVol = service.Binds[0][:hostVolIndex]
		}

		// setup labels and log config for the new container
		labels := make(map[string]string)
		labels[LABEL_PREFIX + ".service_name"] = serviceName
		labels[LABEL_PREFIX + ".variation"] = service.VariationLabel
		labels[LABEL_PREFIX + ".deployment_description_hash"] = deploymentHash

		var logConfig docker.LogConfig

		if !deployment.ServicePattern.isShared("singleton", serviceName) {
			labels[LABEL_PREFIX + ".agreement_id"] = agreementId
			logConfig = docker.LogConfig{
						Type: "syslog",
						Config: map[string]string{
							"tag": fmt.Sprintf("workload-%v_%v", strings.ToLower(agreementId), serviceName),
						},
					}
		} else {
			logName := serviceName
			if service.VariationLabel != "" {
				logName = fmt.Sprintf("%v-%v", serviceName, service.VariationLabel)
			}

			logConfig = docker.LogConfig{
						Type: "syslog",
						Config: map[string]string{
							"tag": fmt.Sprintf("workload-%v_%v", "singleton", logName),
						},
					}
		}

		serviceConfig := &persistence.ServiceConfig{
			Config: docker.Config{
				Image:  service.Image,
				Env:    []string{},
				Cmd:    service.Command,
				CPUSet: cpuSet,
				Labels: labels,
				Volumes: map[string]struct{}{
					hostVol: {},
				},
				ExposedPorts: map[docker.Port]struct{}{},
			},
			HostConfig: docker.HostConfig{
				Privileged:      service.Privileged,
				CapAdd:          service.CapAdd,
				PublishAllPorts: false,
				PortBindings:    map[docker.Port][]docker.PortBinding{},
				Links:           nil, // do not allow any
				RestartPolicy:   docker.AlwaysRestart(),
				Memory:          ramBytes,
				MemorySwap:      0,
				Devices:         []docker.Device{},
				LogConfig:       logConfig,
				Binds: service.Binds,
			},
		}

		// Mark each container as infrastructure if the deployment description indicates infrastructure
		if deployment.Infrastructure {
			serviceConfig.Config.Labels[LABEL_PREFIX + ".infrastructure"] = ""
		}

		// add environment additions to each service
		for k, v := range environmentAdditions {
			serviceConfig.Config.Env = append(serviceConfig.Config.Env, fmt.Sprintf("%s=%v", k, v))
		}

		for _, v := range service.Environment {
			// skip this one b/c it's dangerous
			if !strings.HasPrefix(config.ENVVAR_PREFIX+"ETHEREUM_ACCOUNT", v) && !strings.HasPrefix(config.COMPAT_ENVVAR_PREFIX+"ETHEREUM_ACCOUNT", v) {
				serviceConfig.Config.Env = append(serviceConfig.Config.Env, v)
			}
		}

		for _, port := range service.Ports {
			var hostIP string

			if port.LocalhostOnly {
				hostIP = "127.0.0.1"
			} else {
				hostIP = "0.0.0.0"
			}

			if port.PortAndProtocol == "" {
				return nil, fmt.Errorf("Failed to locate necessary port setup param, PortAndProtocol in %v", port)
			}

			dPort := docker.Port(port.PortAndProtocol)
			var emptyS struct{}
			serviceConfig.Config.ExposedPorts[dPort] = emptyS

			serviceConfig.HostConfig.PortBindings[dPort] = []docker.PortBinding{
				docker.PortBinding{
					HostIP:   hostIP,
					HostPort: "", // empty so it'll be randomly-chosen on host
				},
			}
		}

		for _, specificPort := range service.SpecificPorts {
			var emptyS struct{}
			dPort := docker.Port(specificPort.HostPort + "/tcp")
			serviceConfig.Config.ExposedPorts[dPort] = emptyS

			serviceConfig.HostConfig.PortBindings[dPort] = []docker.PortBinding{
				specificPort,
			}
		}

		for _, givenDevice := range service.Devices {
			sp := strings.Split(givenDevice, ":")
			if len(sp) != 2 {
				return nil, fmt.Errorf("Illegal device specified in deployment description: %v", givenDevice)
			}

			serviceConfig.HostConfig.Devices = append(serviceConfig.HostConfig.Devices, docker.Device{
				PathOnHost:      sp[0],
				PathInContainer: sp[1],
			})
		}

		services[serviceName] = servicePair{
			serviceConfig: serviceConfig,
			service:       service,
		}
	}

	return services, nil
}

type ContainerWorker struct {
	worker.Worker // embedded field
	db            *bolt.DB
	client        *docker.Client
	iptables      *iptables.IPTables
	inAgbot       bool
}

func NewContainerWorker(config *config.HorizonConfig, db *bolt.DB) *ContainerWorker {
	messages := make(chan events.Message)
	commands := make(chan worker.Command, 200)

	inAgbot := false
	if config.Edge.WorkloadROStorage == "" && config.Edge.DBPath == "" {
		// We are running in an agbot, dont need the workload RO storage config.
		inAgbot = true

	} else if err := unix.Access(config.Edge.WorkloadROStorage, unix.W_OK); err != nil {
		glog.Errorf("Unable to access workload RO storage dir: %v. Error: %v", config.Edge.WorkloadROStorage, err)
		panic("Unable to access workload RO storage dir specified in config")
	}

	if ipt, err := iptables.New(); err != nil {
		glog.Errorf("Failed to instantiate iptables Client: %v", err)
		panic("Unable to instantiate iptables Client")
	} else if client, err := docker.NewClient(config.Edge.DockerEndpoint); err != nil {
		glog.Errorf("Failed to instantiate docker Client: %v", err)
		panic("Unable to instantiate docker Client")
	} else {
		worker := &ContainerWorker{
			Worker: worker.Worker{
				Manager: worker.Manager{
					Config:   config,
					Messages: messages,
				},
				Commands: commands,
			},
			db:       db,
			client:   client,
			iptables: ipt,
			inAgbot:  inAgbot,
		}

		worker.start()
		return worker
	}
}

func (w *ContainerWorker) Messages() chan events.Message {
	return w.Worker.Manager.Messages
}

func (w *ContainerWorker) NewEvent(incoming events.Message) {

	switch incoming.(type) {
	case *events.TorrentMessage:
		msg, _ := incoming.(*events.TorrentMessage)
		switch msg.Event().Id {
		case events.TORRENT_FETCHED:
			glog.Infof("Fetched image files from torrent: %v", msg.ImageFiles)
			switch msg.LaunchContext.(type) {
			case *events.AgreementLaunchContext:
				lc := msg.LaunchContext.(*events.AgreementLaunchContext)
				cCmd := w.NewWorkloadConfigureCommand(msg.ImageFiles, lc)
				w.Commands <- cCmd

			case *events.ContainerLaunchContext:
				lc := msg.LaunchContext.(*events.ContainerLaunchContext)
				cCmd := w.NewContainerConfigureCommand(msg.ImageFiles, lc)
				w.Commands <- cCmd
			}
		}

	case *events.GovernanceMaintenanceMessage:
		msg, _ := incoming.(*events.GovernanceMaintenanceMessage)

		switch msg.Event().Id {
		case events.CONTAINER_MAINTAIN:
			containerCmd := w.NewContainerMaintenanceCommand(msg.AgreementProtocol, msg.AgreementId, msg.Deployment)
			w.Commands <- containerCmd
		}

	case *events.GovernanceCancelationMessage:
		msg, _ := incoming.(*events.GovernanceCancelationMessage)

		switch msg.Event().Id {
		case events.AGREEMENT_ENDED:
			containerCmd := w.NewContainerShutdownCommand(msg.AgreementProtocol, msg.AgreementId, msg.Deployment, []string{})
			w.Commands <- containerCmd
		}

	default: // nothing

	}

	return
}

func mkBridge(name string, client *docker.Client) (*docker.Network, error) {
	bridgeOpts := docker.CreateNetworkOptions{
		Name:           name,
		EnableIPv6:     false,
		Internal:       false,
		Driver:         "bridge",
		CheckDuplicate: true,
		Options: map[string]interface{}{
			"com.docker.network.bridge.enable_icc":           "true",
			"com.docker.network.bridge.enable_ip_masquerade": "true",
			"com.docker.network.bridge.default_bridge":       "false",
		},
	}

	bridge, err := client.CreateNetwork(bridgeOpts)
	if err != nil {
		return nil, err
	}
	return bridge, nil
}

func serviceStart(client *docker.Client,
	agreementId string,
	serviceName string,
	shareLabel string,
	serviceConfig *persistence.ServiceConfig,
	endpointsConfig map[string]*docker.EndpointConfig,
	sharedEndpoints map[string]*docker.EndpointConfig,
	postCreateContainers *[]interface{},
	fail func(container *docker.Container, name string, err error) error) error {

	var namePrefix string
	if shareLabel != "" {
		namePrefix = shareLabel
	} else {
		namePrefix = agreementId
	}

	containerOpts := docker.CreateContainerOptions{
		Name:       fmt.Sprintf("%v-%v", namePrefix, serviceName),
		Config:     &serviceConfig.Config,
		HostConfig: &serviceConfig.HostConfig,
		NetworkingConfig: &docker.NetworkingConfig{
			EndpointsConfig: endpointsConfig,
		},
	}

	glog.V(5).Infof("CreateContainer options: Config: %v, HostConfig: %v, EndpointsConfig: %v", serviceConfig.Config, serviceConfig.HostConfig, endpointsConfig)

	container, cErr := client.CreateContainer(containerOpts)
	if cErr != nil {
		return fail(container, serviceName, cErr)
	}

	// second arg just a backwards compat feature, will go away someday
	err := client.StartContainer(container.ID, nil)
	if err != nil {
		return fail(container, serviceName, err)
	}
	for _, cfg := range sharedEndpoints {
		err := client.ConnectNetwork(cfg.NetworkID, docker.NetworkConnectionOptions{
			Container:      container.ID,
			EndpointConfig: cfg,
			Force:          true,
		})
		if err != nil {
			return fail(container, serviceName, err)
		}
	}

	*postCreateContainers = append(*postCreateContainers, container)

	glog.V(3).Infof("In agreement %v, successfully created container %v", agreementId, container)
	return nil
}

func serviceDestroy(client *docker.Client, agreementId string, containerId string) (bool, error) {
	glog.V(3).Infof("Attempting to stop container %v from agreement: %v.", containerId, agreementId)
	err := client.KillContainer(docker.KillContainerOptions{ID: containerId})

	if err != nil {
		if _, ok := err.(*docker.NoSuchContainer); ok {
			return false, nil
		} else {
			return false, fmt.Errorf("Unable to kill container in agreement: %v. Error: %v", agreementId, err)
		}
	}

	glog.V(3).Infof("Attempting to remove container %v from agreement: %v.", containerId, agreementId)
	return true, client.RemoveContainer(docker.RemoveContainerOptions{ID: containerId, RemoveVolumes: true, Force: true})
}

func existingShared(client *docker.Client, serviceName string, servicePair *servicePair, bridgeName string, shareLabel string) (*docker.Network, *docker.APIContainers, error) {

	var sBridge docker.Network
	networks, err := client.ListNetworks()
	if err != nil {
		return nil, nil, err
	}

	for _, net := range networks {
		if net.Name == bridgeName {
			glog.V(3).Infof("Found shared network: %v with name %v", net.ID, net.Name)
			sBridge = net
		}
	}

	// some of the facts in the labels that are compared will also be in the hash
	sharedOnly := docker.ListContainersOptions{
		All: false,
		Filters: map[string][]string{
			"label": []string{
				fmt.Sprintf("%v.service_name=%v", LABEL_PREFIX, serviceName),
				fmt.Sprintf("%v.variation=%v", LABEL_PREFIX, servicePair.service.VariationLabel),
				fmt.Sprintf("%v.deployment_description_hash=%v", LABEL_PREFIX, servicePair.serviceConfig.Config.Labels[fmt.Sprintf("%v.deployment_description_hash", LABEL_PREFIX)]),
				fmt.Sprintf("%v.service_pattern.shared=%v", LABEL_PREFIX, shareLabel),
			},
		},
	}
	glog.V(5).Infof("Searching for containers with filter %v", sharedOnly)
	containers, err := client.ListContainers(sharedOnly)
	if err != nil {
		return nil, nil, err
	}

	if len(containers) > 1 {
		return nil, nil, fmt.Errorf("Odd to find more than one shared service matching share criteria: %v. Containers: %v", sharedOnly, containers)
	}

	if sBridge.ID != "" {
		if len(containers) == 0 {
			glog.V(4).Infof("Couldn't find shared service %v with hash %v, but found existing bridge, ID: %v and Name: %v", serviceName, servicePair.serviceConfig.Config.Labels[fmt.Sprintf("%v.deployment_description_hash", LABEL_PREFIX)], sBridge.ID, sBridge.Name)
			return &sBridge, nil, nil
		}

		if len(containers) == 1 {
			// success finding existing
			glog.V(4).Infof("Found shared service ID: %v Name: %v and matching existing net, ID: %v Name: %v", containers[0].ID, containers[0].Names, sBridge.ID, sBridge.Name)
			return &sBridge, &containers[0], nil
		} else {
			return nil, nil, fmt.Errorf("Unknown state encountered finding shared container and bridge. Bridge: %v. Containers inspected: %v", sBridge, containers)
		}
	}

	return nil, nil, nil
}

func generatePermittedString(isolation *NetworkIsolation, network docker.ContainerNetwork, configureRaw []byte) (string, error) {

	permittedString := ""

	for _, permitted := range isolation.OutboundPermitOnly {
		var permittedValue OutboundPermitValue

		switch permitted.(type) {
		case StaticOutboundPermitValue:
			permittedValue = permitted.(StaticOutboundPermitValue)

		case DynamicOutboundPermitValue:
			p := permitted.(DynamicOutboundPermitValue)
			// do specialized ad-hoc deserialization of the configure whisper message in order to read the dynamic permit value
			var configureUnstruct map[string]interface{}
			if p.Encoding != JSON {
				return "", fmt.Errorf("Unsupported encoding: %v", p.Encoding)
			} else if err := json.Unmarshal(configureRaw, &configureUnstruct); err != nil {
				return "", err
			} else {

				val, exists := configureUnstruct[p.DdKey]
				if !exists {
					return "", fmt.Errorf("Required key %v not present in struct %v", p.DdKey, configureUnstruct)
				}

				var dyn interface{}
				if err := json.Unmarshal([]byte(val.(string)), &dyn); err != nil {
					return "", err
				}

				spl := strings.Split(p.Path, ".")
				for _, part := range spl {
					dyn = dyn.(map[string]interface{})[part]
				}
				permittedValue = dyn
			}

		default:
			return "", fmt.Errorf("Unknown OutboundPermitValue type: %T. Value: %v", permitted, permitted)

		}

		permittedString += fmt.Sprintf("%v,", permittedValue)
	}

	return fmt.Sprintf("%v%v/%v", permittedString, network.IPAddress, network.IPPrefixLen), nil
}

func processPostCreate(ipt *iptables.IPTables, client *docker.Client, agreementId string, deployment DeploymentDescription, configureRaw []byte, hasSpecifiedEthAccount bool, containers []interface{}, fail func(container *docker.Container, name string, err error) error) error {

	rules, err := ipt.List("filter", IPT_COLONUS_ISOLATED_CHAIN)
	if err != nil {
		// could be that it just isn't created, try that

		err := ipt.NewChain("filter", IPT_COLONUS_ISOLATED_CHAIN)
		if err != nil {
			return fail(nil, "<unknown>", fmt.Errorf("Unable to manipulate IPTables rules in container post-creation step: Error: %v", err))
		}

		rules, err = ipt.List("filter", IPT_COLONUS_ISOLATED_CHAIN)
		if err != nil {
			return fail(nil, "<unknown>", fmt.Errorf("Unable to manipulate IPTables rules in container post-creation step: Error: %v", err))
		}
	}

	foundReturn := false
	for _, rule := range rules {
		if rule == fmt.Sprintf("-A %v -j RETURN", IPT_COLONUS_ISOLATED_CHAIN) {
			foundReturn = true
		}
	}

	if !foundReturn {
		err = ipt.Insert("filter", IPT_COLONUS_ISOLATED_CHAIN, 1, "-j", "RETURN")
		if err != nil {
			return fail(nil, "<unknown>", fmt.Errorf("Unable to manipulate IPTables rules in container post-creation step: Error: %v", err))
		}
	}

	rules, err = ipt.List("filter", "FORWARD")

	if err != nil {
		return fail(nil, "<unknown>", fmt.Errorf("Unable to manipulate IPTables rules in container post-creation step: Error: %v", err))
	}
	for _, rule := range rules {
		if rule == fmt.Sprintf("-A FORWARD -j %v", IPT_COLONUS_ISOLATED_CHAIN) {
			glog.Infof("rule: %v", rule)
			err := ipt.Delete("filter", "FORWARD", "-j", IPT_COLONUS_ISOLATED_CHAIN)
			if err != nil {
				return fail(nil, "<unknown>", fmt.Errorf("Unable to manipulate IPTables rules in container post-creation step: Error: %v", err))
			}
		}
	}

	// need to always insert this at the head of the chain; if this fails, there will be no isolation security but normal container traffic will be allowed
	err = ipt.Insert("filter", "FORWARD", 1, "-j", IPT_COLONUS_ISOLATED_CHAIN)
	if err != nil {
		return fail(nil, "<unknown>", fmt.Errorf("Unable to manipulate IPTables rules in container post-creation step: Error: %v", err))
	}

	comment := fmt.Sprintf("agreement_id=%v", agreementId)

	for _, con := range containers {

		newContainerIPs := make([]string, 0)

		switch con.(type) {
		case *docker.Container:
			container := con.(*docker.Container)

			// incoming "container" type does not have Config member
			conDetail, err := client.InspectContainer(container.ID)
			if err != nil {
				return fail(nil, container.Name, fmt.Errorf("Unable to find container detail for container during post-creation step: Error: %v", err))
			}

			if serviceName, exists := conDetail.Config.Labels[LABEL_PREFIX+".service_name"]; exists {
				glog.V(3).Infof("Examining service: %v", serviceName)
				glog.V(3).Infof("Detail from container obj: %v", conDetail.Config.Labels)

				isolation := deployment.Services[serviceName].NetworkIsolation
				if conDetail.Config.Labels[LABEL_PREFIX+".service_pattern.shared"] == "singleton" {
					comment = comment + ",service_pattern.shared=singleton"
				}

				if isolation.OutboundPermitOnly != nil {
					if isolation.OutboundPermitOnlyIgnore == ETH_ACCT_SPECIFIED && hasSpecifiedEthAccount {
						glog.Infof("Skipping application of network isolation rules b/c OutboundPermitOnlyIgnore specified and conditions met")
					} else {
						// reject for this address if rules below don't specifically allow
						for name, network := range conDetail.NetworkSettings.Networks {

							glog.Infof("Creating general isolation rule for network: %v, %v on service %v", name, network, serviceName)
							err = ipt.Insert("filter", IPT_COLONUS_ISOLATED_CHAIN, 1, "-s", network.IPAddress, "-j", "REJECT", "-m", "comment", "--comment", comment)
							if err != nil {
								return fail(nil, serviceName, fmt.Errorf("Unable to create new rules for service. Error: %v", err))
							}

							newContainerIPs = append(newContainerIPs, network.IPAddress)

							permittedString, err := generatePermittedString(&isolation, network, configureRaw)
							if err != nil {
								return fail(nil, serviceName, fmt.Errorf("Unable to determine network permit string for service. Error: %v", err))
							}

							glog.Infof("Creating permission rule for network: %v, %v on service %v. Permitted: %v", name, network, serviceName, permittedString)
							err = ipt.Insert("filter", IPT_COLONUS_ISOLATED_CHAIN, 1, "-s", network.IPAddress, "-d", permittedString, "-j", "ACCEPT", "-m", "comment", "--comment", comment)
							if err != nil {
								return fail(nil, serviceName, fmt.Errorf("Unable to create new rules for service. Error: %v", err))
							}
						}
					}
				}
			}

			// no need to handle case of new shared container needing rules to permit traffic from existing agreements, that case is unsupported

		case *docker.APIContainers:
			glog.Infof("Doing post-create on existing (shared) container: %v", con)
			container := con.(*docker.APIContainers)

			if serviceName, exists := container.Labels[LABEL_PREFIX+".service_name"]; exists {

				if container.Labels[LABEL_PREFIX+".service_pattern.shared"] != "singleton" {
					glog.Infof("Warning: existing container passed to post-processing procedure isn't shared; this is unexpected")
				}

				// doesn't need rules for any agreements except this one
				// rules here permit access *to* existing shared container from those just configured from this agreement (they are on new networks and need access plumbed to this shared container); this is really necessary only if the shared container has network isolation enabled, but won't hurt in any case
				for _, ip := range newContainerIPs {
					for name, network := range container.Networks.Networks {
						glog.Infof("Creating permission rule for network: %v, %v on service %v", name, network, serviceName)

						err := ipt.Insert("filter", IPT_COLONUS_ISOLATED_CHAIN, 1, "-s", ip, "-d", network.IPAddress, "-j", "ACCEPT", "-m", "comment", "--comment", comment)
						if err != nil {
							return fail(nil, serviceName, fmt.Errorf("Unable to create new rules for service. Error: %v", err))
						}
					}
				}
			}

		default:
			return fail(nil, "<unknown>", fmt.Errorf("Unknown container type (%T) from argument %v", con, con))
		}
	}

	return nil
}

func (b *ContainerWorker) workloadStorageDir(agreementId string) string {
	return path.Join(b.Config.Edge.WorkloadROStorage, agreementId)
}

func (b *ContainerWorker) resourcesCreate(agreementId string, configure *events.ContainerConfig, deployment *DeploymentDescription, configureRaw []byte, environmentAdditions map[string]string) (*map[string]persistence.ServiceConfig, error) {

	// local helpers
	fail := func(container *docker.Container, name string, err error) error {
		if container != nil {
			glog.Errorf("Error processing container setup: %v", container)
		}

		glog.Errorf("Failed to set up %v. Attempting to remove other resources in agreement (%v) before returning control to caller. Error: %v", name, agreementId, err)

		rErr := b.resourcesRemove([]string{agreementId})
		if rErr != nil {
			glog.Errorf("Following error setting up patterned deployment, failed to clean up other resources for agreement: %v. Error: %v", agreementId, rErr)
		}

		return err
	}

	mkEndpoints := func(bridge *docker.Network, containerName string) map[string]*docker.EndpointConfig {

		return map[string]*docker.EndpointConfig{
			bridge.Name: &docker.EndpointConfig{
				Aliases:   []string{containerName},
				Links:     nil,
				NetworkID: bridge.ID,
			},
		}
	}

	recordEndpoints := func(endpoints map[string]*docker.EndpointConfig, incoming map[string]*docker.EndpointConfig) map[string]*docker.EndpointConfig {

		for name, cfg := range incoming {
			// don't fail if it already exists; last one wins
			if _, exists := endpoints[name]; exists {
				glog.V(5).Infof("Endpoint for bridge %v is already defined in endpointsConfig. This is ok, overwriting", name)
			}

			woutAliases := &docker.EndpointConfig{
				Aliases:   nil,
				Links:     nil,
				NetworkID: cfg.NetworkID,
			}

			endpoints[name] = woutAliases
		}

		return endpoints
	}

	workloadROStorageDir := b.workloadStorageDir(agreementId)

	// create RO workload storage dir
	if err := os.Mkdir(workloadROStorageDir, 0700); err != nil {
		return nil, err
	}

	glog.V(5).Infof("Writing raw config to file in %v. Config data: %v", workloadROStorageDir, string(configureRaw))
	// write raw to workloadROStorageDir
	if err := ioutil.WriteFile(path.Join(workloadROStorageDir, "Configure"), configureRaw, 0644); err != nil {
		return nil, err
	}

	servicePairs, err := finalizeDeployment(agreementId, deployment, environmentAdditions, workloadROStorageDir, b.Config.Edge.DefaultCPUSet)
	if err != nil {
		return nil, err
	}

	// process services that are "shared" first, then others
	shared := make(map[string]servicePair, 0)
	private := make(map[string]servicePair, 0)

	// trimmed structure to return to caller
	ret := make(map[string]persistence.ServiceConfig, 0)

	for serviceName, servicePair := range servicePairs {
		if image, err := b.client.InspectImage(servicePair.serviceConfig.Config.Image); err != nil {
			return nil, fail(nil, serviceName, fmt.Errorf("Failed to inspect image. Original error: %v", err))
		} else if image == nil {
			return nil, fail(nil, serviceName, fmt.Errorf("Unable to find Docker image: %v", servicePair.serviceConfig.Config.Image))
		}

		// need to examine original deploymentDescription to determine which containers are "shared" or in other special patterns
		if deployment.ServicePattern.isShared("singleton", serviceName) {
			shared[serviceName] = servicePair
		} else {
			private[serviceName] = servicePair
		}

		ret[serviceName] = *servicePair.serviceConfig
	}

	// finished pre-processing

	// process shared by finding existing or creating new then hooking up "private" in pattern to the shared by adding two endpoints. Note! a shared container is not in the agreement bridge it came from

	// could be a *docker.APIContainers or *docker.Container
	postCreateContainers := make([]interface{}, 0)

	sharedEndpoints := make(map[string]*docker.EndpointConfig, 0)
	for serviceName, servicePair := range shared {
		shareLabel := "singleton"

		servicePair.serviceConfig.Config.Labels[LABEL_PREFIX+".service_pattern.shared"] = shareLabel
		bridgeName := fmt.Sprintf("%v-%v", shareLabel, serviceName)
		containerName := serviceName

		// append variation label if it exists
		if servicePair.service.VariationLabel != "" {
			bridgeName = fmt.Sprintf("%v-%v", bridgeName, servicePair.service.VariationLabel)
			containerName = fmt.Sprintf("%v-%v", serviceName, servicePair.service.VariationLabel)
		}

		var existingNetwork *docker.Network
		var existingContainer *docker.APIContainers

		existingNetwork, existingContainer, err = existingShared(b.client, serviceName, &servicePair, bridgeName, shareLabel)
		if err != nil {
			return nil, fail(nil, containerName, fmt.Errorf("Failed to discover and use existing shared containers. Original error: %v", err))
		}

		if existingNetwork == nil {
			existingNetwork, err = mkBridge(bridgeName, b.client)
			glog.V(2).Infof("Created new network for shared container: %v. Network: %v", containerName, existingNetwork)
			if err != nil {
				return nil, fail(nil, containerName, fmt.Errorf("Unable to create bridge for shared container. Original error: %v", err))
			}
		}

		glog.V(4).Infof("Using network for shared service: %v. Network ID: %v", containerName, existingNetwork.ID)

		// retain reference so we can wire "private" containers from this agreement to this bridge later; need to do this even if we already saw a net
		eps := mkEndpoints(existingNetwork, serviceName)
		recordEndpoints(sharedEndpoints, eps)

		if existingContainer == nil {
			// only create container if there wasn't one
			servicePair.serviceConfig.HostConfig.NetworkMode = bridgeName
			if err := serviceStart(b.client, agreementId, containerName, shareLabel, servicePair.serviceConfig, eps, nil, &postCreateContainers, fail); err != nil {
				return nil, err
			}
		} else {
			// will add a *docker.APIContainers type
			postCreateContainers = append(postCreateContainers, existingContainer)
		}
	}

	// from here on out, need to clean up bridge(s) if there is a problem
	agBridge, err := mkBridge(agreementId, b.client)
	if err != nil {
		return nil, err
	}

	// every one of these gets wired to both the agBridge and every shared bridge from this agreement
	for serviceName, servicePair := range private {
		servicePair.serviceConfig.HostConfig.NetworkMode = agreementId // custom bridge has agreementId as name, same as endpoint key
		if err := serviceStart(b.client, agreementId, serviceName, "", servicePair.serviceConfig, mkEndpoints(agBridge, serviceName), sharedEndpoints, &postCreateContainers, fail); err != nil {
			return nil, err
		}
	}

	// check environmentAdditions for MTN_ETHEREUM_ACCOUNT
	_, hasSpecifiedEthAccount := environmentAdditions[config.ENVVAR_PREFIX+"ETHEREUM_ACCOUNT"]

	if err := processPostCreate(b.iptables, b.client, agreementId, *deployment, configureRaw, hasSpecifiedEthAccount, postCreateContainers, fail); err != nil {
		return nil, err
	}

	for name, _ := range ret {
		glog.Infof("Created service %v in agreement %v", name, agreementId)
	}
	return &ret, nil
}

func (b *ContainerWorker) start() {

	go func() {

		b.syncupResources()

		// Now we can drop into the main command processing loop
		for {
			glog.V(4).Infof("ContainerWorker command processor blocking waiting to receive incoming commands")

			command := <-b.Commands

			switch command.(type) {
			case *WorkloadConfigureCommand:
				cmd := command.(*WorkloadConfigureCommand)

				glog.V(3).Infof("ContainerWorker received workoad configure command: %v", cmd)

				agreementId := cmd.AgreementLaunchContext.AgreementId

				if ags, err := persistence.FindEstablishedAgreements(b.db, citizenscientist.PROTOCOL_NAME, []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(agreementId)}); err != nil {
					glog.Errorf("Unable to retrieve agreement %v from database, error %v", agreementId, err)
				} else if len(ags) != 1 {
					glog.Infof("Ignoring the configure event for agreement %v, the agreement is archived.", agreementId)
				} else if ags[0].AgreementTerminatedTime != 0 {
					glog.Infof("Receved configure command for agreement %v. Ignoring it because this agreement has been terminated.", agreementId)
				} else if ags[0].AgreementExecutionStartTime != 0 {
					glog.Infof("Receved configure command for agreement %v. Ignoring it because the containers for this agreement has been configured.", agreementId)
				} else {
					if len(cmd.ImageFiles) == 0 {
						glog.Infof("Command specified no new Docker images to load, expecting that the caller knows they're preloaded and this is not a bug. Skipping load operation")

					} else if err := loadImages(b.client, b.Config.Edge.TorrentDir, cmd.ImageFiles); err != nil {
						glog.Errorf("Error loading image files: %v", err)

						b.Messages() <- events.NewWorkloadMessage(events.EXECUTION_FAILED, cmd.AgreementLaunchContext.AgreementProtocol, agreementId, nil)

						continue
					}

					// We support capabilities in the deployment string that not all container deployments should be able
					// to exploit, e.g. file system mapping from host to container. This check ensures that workloads dont try
					// to do something dangerous.
					deploymentDesc := new(DeploymentDescription)
					if err := json.Unmarshal([]byte(cmd.AgreementLaunchContext.Configure.Deployment), &deploymentDesc); err != nil {
						glog.Errorf("Error Unmarshalling deployment string %v for agreement %v, error: %v", cmd.AgreementLaunchContext.Configure.Deployment, agreementId, err)
						continue
					} else if valid := deploymentDesc.isValidFor("workload"); !valid {
						glog.Errorf("Deployment config %v contains unsupported capability for a workload", cmd.AgreementLaunchContext.Configure.Deployment)
						b.Messages() <- events.NewWorkloadMessage(events.EXECUTION_FAILED, cmd.AgreementLaunchContext.AgreementProtocol, agreementId, nil)
					}

					// Dynamically add in a filesystem mapping so that the workload container has a RO filesystem.
					for serviceName, service := range deploymentDesc.Services {
						dir := ""
						if deploymentDesc.ServicePattern.isShared("singleton", serviceName) {
							dir = b.workloadStorageDir(fmt.Sprintf("%v-%v-%v", "singleton", serviceName, service.VariationLabel))
						} else {
							dir = b.workloadStorageDir(agreementId)
						}
						deploymentDesc.Services[serviceName].addFilesystemBinding(fmt.Sprintf("%v:%v:ro", dir, "/workload_config"))
					}

					// Create the docker configuration and launch the containers.
					if deployment, err := b.resourcesCreate(agreementId, &cmd.AgreementLaunchContext.Configure, deploymentDesc, cmd.AgreementLaunchContext.ConfigureRaw, *cmd.AgreementLaunchContext.EnvironmentAdditions); err != nil {
						glog.Errorf("Error starting containers: %v", err)
						var dep map[string]persistence.ServiceConfig
						if deployment != nil {
							dep = *deployment
						}
						b.Messages() <- events.NewWorkloadMessage(events.EXECUTION_FAILED, cmd.AgreementLaunchContext.AgreementProtocol, agreementId, dep) // still using deployment here, need it to shutdown containers

					} else {
						glog.Infof("Success starting pattern for agreement: %v, protocol: %v, serviceNames: %v", agreementId, cmd.AgreementLaunchContext.AgreementProtocol, persistence.ServiceConfigNames(deployment))

						// perhaps add the tc info to the container message so it can be enforced
						b.Messages() <- events.NewWorkloadMessage(events.EXECUTION_BEGUN, cmd.AgreementLaunchContext.AgreementProtocol, agreementId, *deployment)
					}
				}

			case *ContainerConfigureCommand:
				cmd := command.(*ContainerConfigureCommand)

				glog.V(3).Infof("ContainerWorker received container configure command: %v", cmd)

				// We support capabilities in the deployment string that not all container deployments should be able
				// to exploit, e.g. file system mapping from host to container. This check ensures that infrastructure
				// containers dont try to do something unsupported.
				deploymentDesc := new(DeploymentDescription)
				if err := json.Unmarshal([]byte(cmd.ContainerLaunchContext.Configure.Deployment), &deploymentDesc); err != nil {
					glog.Errorf("Error Unmarshalling deployment string %v, error: %v", cmd.ContainerLaunchContext.Configure.Deployment, err)
					b.Messages() <- events.NewContainerMessage(events.EXECUTION_FAILED, *cmd.ContainerLaunchContext)
					continue
				} else if valid := deploymentDesc.isValidFor("infrastructure"); !valid {
					glog.Errorf("Deployment config %v contains unsupported capability for infrastructure container", cmd.ContainerLaunchContext.Configure.Deployment)
					b.Messages() <- events.NewContainerMessage(events.EXECUTION_FAILED, *cmd.ContainerLaunchContext)
					continue
				}

				// Check to see if the container is already running. No need to start it if so.
				cMatches := make([]docker.APIContainers, 0)
				serviceNames := deploymentDesc.serviceNames()
				report := func(container *docker.APIContainers, contName string) error {

					for _, name := range serviceNames {
						if container.Labels[LABEL_PREFIX+".service_name"] == name {
							cMatches = append(cMatches, *container)
							glog.V(4).Infof("Matching container instance for name %v: %v", contName, container)
						}
					}
					return nil
				}

				b.ContainersMatchingAgreement([]string{cmd.ContainerLaunchContext.Blockchain.Name}, false, report)

				if len(serviceNames) <= len(cMatches) {
					glog.V(4).Infof("Found container already running for %v: %v", cmd.ContainerLaunchContext.Blockchain.Name, len(cMatches))
					continue
				}

				// The container is not already running, so let's make sure we do some clean up to ensure that the container will start.
				workloadROStorageDir := b.workloadStorageDir(cmd.ContainerLaunchContext.Blockchain.Name)
				if err := os.RemoveAll(workloadROStorageDir); err != nil {
					glog.Infof("workloadROStorageDir already cleaned up: %v. Error: %v", workloadROStorageDir, err)
				}

				// Remove network if it is left hanging around
				if networks, err := b.client.ListNetworks(); err != nil {
					glog.Infof("Unable to list networks: %v", err)
				} else {
					for _, net := range networks {
						if net.Name == cmd.ContainerLaunchContext.Blockchain.Name {
							glog.V(5).Infof("Freeing blockchain net: %v", net.Name)
							if err := b.client.RemoveNetwork(net.ID); err != nil {
								glog.Errorf("Failure removing network: %v. Error: %v", net.ID, err)
							} else {
								glog.V(5).Infof("Freed blockchain net: %v", net.Name)
							}
							break
						}
					}
				}

				// Proceed to load the docker image.
				if len(cmd.ImageFiles) == 0 {
					glog.Errorf("Torrent configuration in deployment specified no new Docker images to load: %v, unable to load container", deploymentDesc)
					b.Messages() <- events.NewContainerMessage(events.EXECUTION_FAILED, *cmd.ContainerLaunchContext)
					continue
				} else if err := loadImages(b.client, b.Config.Edge.TorrentDir, cmd.ImageFiles); err != nil {
					glog.Errorf("Error loading image files: %v", err)
					b.Messages() <- events.NewContainerMessage(events.EXECUTION_FAILED, *cmd.ContainerLaunchContext)
					continue
				}

				// Dynamically add in a filesystem mapping so that the infrastructure container can write files that will
				// be saveable or observable to the host system. Also turn on the privileged flag for this container.
				for serviceName, _ := range deploymentDesc.Services {
					dir := ""
					// NON_SNAP_COMMON is used for testing purposes only
					if altDir := os.Getenv("NON_SNAP_COMMON"); len(altDir) != 0 {
						dir = altDir + ":/root"
					} else {
						dir = path.Join(os.Getenv("SNAP_COMMON")) + ":/root"
					}
					deploymentDesc.Services[serviceName].addFilesystemBinding(dir)
					deploymentDesc.Services[serviceName].addSpecificPortBinding(docker.PortBinding{HostIP:"0.0.0.0", HostPort:"8545"})
					deploymentDesc.Services[serviceName].Privileged = true
				}

				// Indicate that this deployment description is part of the infrastructure
				deploymentDesc.Infrastructure = true

				// Get the container started.
				if deployment, err := b.resourcesCreate(cmd.ContainerLaunchContext.Blockchain.Name, &cmd.ContainerLaunchContext.Configure, deploymentDesc, []byte(""), *cmd.ContainerLaunchContext.EnvironmentAdditions); err != nil {
					glog.Errorf("Error starting containers: %v", err)
					b.Messages() <- events.NewContainerMessage(events.EXECUTION_FAILED, *cmd.ContainerLaunchContext)

				} else {
					glog.Infof("Success starting pattern for serviceNames: %v", persistence.ServiceConfigNames(deployment))

					// perhaps add the tc info to the container message so it can be enforced
					b.Messages() <- events.NewContainerMessage(events.EXECUTION_BEGUN, *cmd.ContainerLaunchContext)
				}

			case *ContainerMaintenanceCommand:
				cmd := command.(*ContainerMaintenanceCommand)
				glog.V(3).Infof("ContainerWorker received maintenance command: %v", cmd)

				cMatches := make([]docker.APIContainers, 0)

				serviceNames := persistence.ServiceConfigNames(&cmd.Deployment)

				report := func(container *docker.APIContainers, agreementId string) error {

					for _, name := range serviceNames {
						if container.Labels[LABEL_PREFIX+".service_name"] == name {
							cMatches = append(cMatches, *container)
							glog.V(4).Infof("Matching container instance for agreement %v: %v", agreementId, container)
						}
					}
					return nil
				}

				b.ContainersMatchingAgreement([]string{cmd.AgreementId}, true, report)

				if len(serviceNames) == len(cMatches) {
					glog.V(4).Infof("Found expected count of running containers for agreement %v: %v", cmd.AgreementId, len(cMatches))
				} else {
					glog.Errorf("Insufficient running containers found for agreement %v. Found: %v", cmd.AgreementId, cMatches)

					// ask governer to cancel the agreement
					b.Messages() <- events.NewWorkloadMessage(events.EXECUTION_FAILED, cmd.AgreementProtocol, cmd.AgreementId, cmd.Deployment)
				}

			case *ContainerShutdownCommand:
				cmd := command.(*ContainerShutdownCommand)

				agreements := cmd.Agreements
				if cmd.CurrentAgreementId != "" {
					glog.Infof("ContainerWorker received shutdown command w/ current agreement id: %v. Shutting down resources", cmd.CurrentAgreementId)
					glog.V(5).Infof("Shutdown command for agreement id %v: %v", cmd.CurrentAgreementId, cmd)
					agreements = append(agreements, cmd.CurrentAgreementId)
				}

				if err := b.resourcesRemove(agreements); err != nil {
					glog.Errorf("Error removing resources: %v", err)
				}

				// send the event to let others know that the workload clean up has been processed
				b.Messages() <- events.NewWorkloadMessage(events.WORKLOAD_DESTROYED, cmd.AgreementProtocol, cmd.CurrentAgreementId, nil)

			default:
				glog.Errorf("Unsupported command: %v", command)

			}
			runtime.Gosched()
		}
	}()
}

// Before we let the worker do anything, we need to sync up the running containers, networks, etc with the
// agreements in the local DB. If we find any containers or networks that shouldnt be there, we will get
// rid of them. This can occur if anax terminates abruptly while in the middle of starting or cancelling an
// agreement. Shared networks and containers will be implicitly cleaned up by cleaning up any leftovers from
// agreements. If all the usages of a shared container or network are from leftover agreements, when we cleanup
// the leftovers, the shared resources will be cleaned up too when the last usage is removed.
// To ensure that all the containers for all known agreements are running, we will depend on the governance
// function which periodically checks to ensure that all containers are running.
func (b *ContainerWorker) syncupResources() {

	if b.inAgbot {
		glog.V(3).Infof("ContainerWorker skipping resource sync up on agreement bot.")
		return
	}

	outcome := true
	leftoverAgreements := make(map[string]bool)

	fail := func(msg string) {
		glog.Errorf(msg)
		outcome = false
	}

	IsAgreementId := func(id string) bool {
		if len(id) < 64 {
			return false
		}

		idInt := big.NewInt(0)
		if _, ok := idInt.SetString(id, 16); !ok {
			return false
		}
		return true
	}

	glog.V(3).Infof("ContainerWorker beginning sync up of docker resources.")

	// First get all the agreements from the DB.
	if agreements, err := persistence.FindEstablishedAgreements(b.db, citizenscientist.PROTOCOL_NAME, []persistence.EAFilter{persistence.UnarchivedEAFilter()}); err != nil {
		fail(fmt.Sprintf("ContainerWorker unable to retrieve agreements from database, error %v", err))
	} else {

		// Create quick access map of agreement ids. This will allow us to avoid nested loops.
		agMap := make(map[string]bool)
		for _, agreement := range agreements {
			agMap[agreement.CurrentAgreementId] = true
		}

		// Second, run through each container looking for containers that are leftover from old agreements. Be aware that there
		// could be other non-Horizon containers on this host, so we have to be careful to NOT terminate them.
		if containers, err := b.client.ListContainers(docker.ListContainersOptions{}); err != nil {
			fail(fmt.Sprintf("ContainerWorker unable to get list of running containers: %v", err))
		} else {

			// Look for orphaned containers.
			for _, container := range containers {
				glog.V(5).Infof("ContainerWorker working on container %v", container)

				// Containers that are part of our horizon infrastructure or are shared or without an agreement id label will be ignored.
				if _, infraLabel := container.Labels[LABEL_PREFIX + ".infrastructure"]; infraLabel {
					continue
				} else if _, sharedThere := container.Labels[LABEL_PREFIX + ".service_pattern.shared"]; sharedThere {
					continue
				} else if _, labelThere := container.Labels[LABEL_PREFIX + ".agreement_id"]; !labelThere {
					continue
				} else if !IsAgreementId(container.Labels[LABEL_PREFIX + ".agreement_id"]) {
					// Not a valid number so it must be old infrastructure before the infrastructure label was added, ignore it.
					continue
				} else if _, there := agMap[container.Labels[LABEL_PREFIX + ".agreement_id"]]; !there {
					// The container has the horizon agreement id label, but the agreement id is not in our local DB.
					glog.V(3).Infof("ContainerWorker found leftover container %v", container)
					leftoverAgreements[container.Labels[LABEL_PREFIX+".agreement_id"]] = true
				}
			}
		}

		// Third, run through each network looking for networks that are leftover from old agreements. Be aware that there
		// could be other non-Horizon networks on this host, so we have to be careful to NOT terminate them.
		if networks, err := b.client.ListNetworks(); err != nil {
			fail(fmt.Sprintf("ContainerWorker unable to get list of networks: %v", err))
		} else {
			for _, net := range networks {
				glog.V(5).Infof("ContainerWorker working on network %v", net)
				if !IsAgreementId(net.Name) {
					continue
				} else if _, there := agMap[net.Name]; !there {
					glog.V(3).Infof("ContainerWorker found leftover network %v", net)
					leftoverAgreements[net.Name] = true
				}
			}
		}

		// Fourth, run through IP routing table rules, looking for rules that are leftover from old agreements. Be aware that there
		// could be other non-Horizon rules on this host, so we have to be careful to NOT terminate them.
		if exists, err := b.iptables.Exists("filter", IPT_COLONUS_ISOLATED_CHAIN, "-j", "RETURN"); err != nil {
			fail(fmt.Sprintf("ContainerWorker unable to interrogate iptables on host. Error: %v", err))
		} else if !exists {
			glog.V(3).Infof(fmt.Sprintf("ContainerWorker primary redirect rule missing from %v chain.", IPT_COLONUS_ISOLATED_CHAIN))
		} else if rules, err := b.iptables.List("filter", IPT_COLONUS_ISOLATED_CHAIN); err != nil {
			fail(fmt.Sprintf("ContainerWorker unable to list rules in %v. Error: %v", IPT_COLONUS_ISOLATED_CHAIN, err))
		} else {
			for ix := len(rules) - 1; ix >= 0; ix-- {
				glog.V(5).Infof("ContainerWorker found isolation rule: %v", rules[ix])
			}
		}

		// If there are leftover resources, get rid of them.
		if len(leftoverAgreements) != 0 {
			// convert to an array of agreement ids to be removed
			agreementList := make([]string, 0, 10)
			for key, _ := range leftoverAgreements {
				agreementList = append(agreementList, key)
			}

			// remove the leftovers
			glog.V(5).Infof("ContainerWorker found leftover pieces of agreements: %v", agreementList)
			if err := b.resourcesRemove(agreementList); err != nil {
				fail(fmt.Sprintf("ContainerWorker unable to get rid of left over resources, error: %v", err))
			}
		}

	}

	glog.V(3).Infof("ContainerWorker done syncing docker resources, successful: %v.", outcome)
	// Finally issue an event to tell everyone else that we are done with the sync up, and the final status of it.
	b.Messages() <- events.NewDeviceContainersSyncedMessage(events.DEVICE_CONTAINERS_SYNCED, outcome)
}

func (b *ContainerWorker) resourcesRemove(agreements []string) error {
	glog.V(5).Infof("Killing and removing resources in agreements: %v", agreements)

	// remove old workspaceROStorage dir
	for _, agreementId := range agreements {
		workloadROStorageDir := b.workloadStorageDir(agreementId)
		if err := os.RemoveAll(workloadROStorageDir); err != nil {
			glog.Errorf("Failed to remove workloadROStorageDir: %v. Error: %v", workloadROStorageDir, err)
		}
	}

	// Remove networks
	networks, err := b.client.ListNetworks()
	if err != nil {
		return fmt.Errorf("Unable to list networks: %v", err)
	}

	freeNets := make([]docker.Network, 0)
	destroy := func(container *docker.APIContainers, agreementId string) error {
		if val, exists := container.Labels[LABEL_PREFIX+".service_pattern.shared"]; exists && val == "singleton" {
			// must investigate bridge to see if other containers are still using this shared service

			glog.V(4).Infof("Found shared container with names: %v", container.Names)

			var sharedNet docker.Network
			for netName, _ := range container.Networks.Networks {
				for _, conName := range container.Names {
					if strings.TrimLeft(conName, "/") == netName {
						// this shared container should only have one network, its own and it should be the same as the name of the container (minus the prefix, of course)
						for _, network := range networks {
							// want a handle to a real network from our earlier-fetched list, not just the Container stub
							if netName == network.Name {
								sharedNet = network
							}
						}
					}
				}
			}

			if sharedNet.ID == "" {
				glog.V(3).Infof("Did not find existing network for shared container: %v", container)
			}

			for conId, _ := range sharedNet.Containers {
				// do container lookup

				allContainers, err := b.client.ListContainers(docker.ListContainersOptions{})
				if err != nil {
					return err
				}

				for _, con := range allContainers {
					if conId == con.ID &&
						con.Labels[LABEL_PREFIX+".agreement_id"] != container.Labels[LABEL_PREFIX+".agreement_id"] {

						glog.V(2).Infof("Will not free resources for shared container: %v, it's in use by %v", container.Names, con)

						return nil
					}
				}
			}

			// if we made it this far, we can free this network
			freeNets = append(freeNets, sharedNet)
		}

		serviceName := container.Labels[LABEL_PREFIX+".service_name"]
		// if we made it this far, we're hosing the container
		if destroyed, err := serviceDestroy(b.client, agreementId, container.ID); err != nil {
			glog.Errorf("Service %v in agreement %v could not be removed. Error: %v", serviceName, agreementId, err)
		} else if destroyed {
			glog.Infof("Service %v in agreement %v stopped and removed", serviceName, agreementId)
		} else {
			glog.V(5).Infof("Service %v in agreement %v already removed", serviceName, agreementId)
		}

		return nil
	}

	b.ContainersMatchingAgreement(agreements, true, destroy)

	// gather agreement networks to free
	for _, net := range networks {
		for _, agreementId := range agreements {
			if net.Name == agreementId {
				// disconnect the network from the containers if they are still connected to it.
				if netInfo, err := b.client.NetworkInfo(net.ID); err != nil {
					glog.Errorf("Failure getting network info for %v. Error: %v", net.Name, err)
				} else {
					for conID, container := range netInfo.Containers {
						glog.V(5).Infof("Disconnecting network %v from container %v.", netInfo.Name, container.Name)
						err := b.client.DisconnectNetwork(netInfo.ID, docker.NetworkConnectionOptions{
							Container:      conID,
							EndpointConfig: nil,
							Force:          true,
						})
						if err != nil {
							glog.Errorf("Failure disconnecting network: %v from container %v. Error: %v", netInfo.Name, container.Name, err)
						} else {
							glog.Infof("Succeeded disconnecting network: %v from container %v", netInfo.Name, container.Name)
						}
					}
				}

				// save the net for removing later
				glog.V(5).Infof("Freeing agreement net: %v", net.Name)
				freeNets = append(freeNets, net)
			}
		}
	}

	// free networks
	for _, net := range freeNets {
		if err := b.client.RemoveNetwork(net.ID); err != nil {
			glog.Errorf("Failure removing network: %v. Error: %v", net.ID, err)
		} else {
			glog.Infof("Succeeded removing unused network: %v", net.ID)
		}
	}

	// the primary rule
	if exists, err := b.iptables.Exists("filter", IPT_COLONUS_ISOLATED_CHAIN, "-j", "RETURN"); err != nil {
		return fmt.Errorf("Unable to interrogate iptables on host. Error: %v", err)
	} else if !exists {
		glog.V(3).Infof("Primary redirect rule missing from %v chain. Skipping agreement rule deletion", IPT_COLONUS_ISOLATED_CHAIN)
	} else {
		// free iptables rules for this agreement (will hose access to shared too)
		rules, err := b.iptables.List("filter", IPT_COLONUS_ISOLATED_CHAIN)
		if err != nil {
			return fmt.Errorf("Unable to list rules in %v. Error: %v", IPT_COLONUS_ISOLATED_CHAIN, err)
		}

		for _, agreementId := range agreements {
			glog.V(4).Infof("Removing iptables isolation rules for agreement %v", agreementId)

			// count backwards so we don't have to adjust the indices b/c they change w/ each ipt delete
			for ix := len(rules) - 1; ix >= 0; ix-- {
				if strings.Contains(rules[ix], fmt.Sprintf("agreement_id=%v", agreementId)) {

					glog.V(3).Infof("Deleting isolation rule: %v", rules[ix])
					if err := b.iptables.Delete("filter", IPT_COLONUS_ISOLATED_CHAIN, strconv.Itoa(ix)); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func (b *ContainerWorker) ContainersMatchingAgreement(agreements []string, includeShared bool, fn func(*docker.APIContainers, string) error) error {
	var processingErr error

	containers, err := b.client.ListContainers(docker.ListContainersOptions{})
	if err != nil {
		glog.Errorf("Unable to get list of running containers: %v", err)
	} else {

		for _, container := range containers {
			for _, agreementId := range agreements { // important to allow shortcutting this b/c the list can be really long
				glog.V(5).Infof("Checking %v (names: %v) in agreement: %v", container.ID, container.Names, agreementId)
				if agreementId == container.Labels[LABEL_PREFIX+".agreement_id"] ||
					(includeShared && container.Labels[LABEL_PREFIX+".service_pattern.shared"] == "singleton") {
					processingErr = fn(&container, agreementId)
					if processingErr != nil {
						glog.Errorf("Error executing function on agreement-matching containers: %v. Continuing processing", processingErr)
					}
				} else {
					glog.V(4).Infof("Skipping container: %v", container.ID)
				}
			}
		}
	}

	return processingErr
}
