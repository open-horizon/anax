package container

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/coreos/go-iptables/iptables"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/worker"
	gwhisper "github.com/open-horizon/go-whisper"
	"golang.org/x/sys/unix"
	"io"
	"io/ioutil"
	"os"
	"path"
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
	Services       map[string]Service `json:"services"`
	ServicePattern Pattern            `json:"service_pattern"`
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
	Image            string           `json:"image"`
	VariationLabel   string           `json:"variation_label,omitempty"`
	Privileged       bool             `json:"privileged"`
	Environment      []string         `json:"environment,omitempty"`
	CapAdd           []string         `json:"cap_add,omitempty"`
	Command          []string         `json:"command,omitempty"`
	Devices          []string         `json:"devices,omitempty"`
	Ports            []Port           `json:"ports,omitempty"`
	NetworkIsolation NetworkIsolation `json:"network_isolation,omitempty"`
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
		deploymentHash, err := hashService(&service)
		if err != nil {
			return nil, err
		}

		serviceConfig := &persistence.ServiceConfig{
			Config: docker.Config{
				Image:  service.Image,
				Env:    []string{},
				Cmd:    service.Command,
				CPUSet: cpuSet,
				Labels: map[string]string{
					LABEL_PREFIX + ".agreement_id":                agreementId,
					LABEL_PREFIX + ".service_name":                serviceName,
					LABEL_PREFIX + ".variation":                   service.VariationLabel,
					LABEL_PREFIX + ".deployment_description_hash": deploymentHash,
				},
				Volumes: map[string]struct{}{
					workloadROStorageDir: {},
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
				LogConfig: docker.LogConfig{
					Type: "syslog",
					Config: map[string]string{
						"tag": fmt.Sprintf("workload-%v_%v", strings.ToLower(agreementId), serviceName),
					},
				},
				Binds: []string{fmt.Sprintf("%v:%v:ro", workloadROStorageDir, "/workload_config")},
			},
		}

		// add environment additions to each service
		for k, v := range environmentAdditions {
			serviceConfig.Config.Env = append(serviceConfig.Config.Env, fmt.Sprintf("%s=%v", k, v))
		}

		for _, v := range service.Environment {
			// skip this one b/c it's dangerous
			if !strings.HasPrefix(config.ENVVAR_PREFIX+"ETHEREUM_ACCOUNT", v) && !strings.HasPrefix(config.COMPAT_ENVVAR_PREFIX+"ETHEREUM_ACCOUNT", v){
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
			service:       &service,
		}
	}

	return services, nil
}

type ContainerWorker struct {
	worker.Worker // embedded field
	client        *docker.Client
	iptables      *iptables.IPTables
}

func NewContainerWorker(config *config.HorizonConfig) *ContainerWorker {
	messages := make(chan events.Message)
	commands := make(chan worker.Command, 200)

	if err := unix.Access(config.Edge.WorkloadROStorage, unix.W_OK); err != nil {
		glog.Errorf("Unable to access workload RO storage dir: %v. Error: %v", config.Edge.WorkloadROStorage, err)
		panic("Unable to access workload RO storage dir specified in config")
	} else if ipt, err := iptables.New(); err != nil {
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
			client:   client,
			iptables: ipt,
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
			cCmd := w.NewContainerConfigureCommand(msg.ImageFiles, msg.AgreementLaunchContext)
			w.Commands <- cCmd
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
			glog.V(3).Infof("Found shared network: %v", net.ID)
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
	containers, err := client.ListContainers(sharedOnly)
	if err != nil {
		return nil, nil, err
	}

	if len(containers) > 1 {
		return nil, nil, fmt.Errorf("Odd to find more than one shared service matching share criteria: %v. Containers: %v", sharedOnly, containers)
	}

	if sBridge.ID != "" {
		if len(containers) == 0 {
			glog.V(4).Infof("Couldn't find shared service %v with hash %v, but found existing bridge, %v, using that", serviceName, servicePair.service.VariationLabel, sBridge.ID)
			return &sBridge, nil, nil
		}

		if len(containers) == 1 {
			// success finding existing
			glog.V(4).Infof("Found shared service %v and matching existing net: %v", containers[0].ID, sBridge.ID)
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

func (b *ContainerWorker) resourcesCreate(agreementId string, configure *gwhisper.Configure, configureRaw []byte, environmentAdditions map[string]string) (*map[string]persistence.ServiceConfig, error) {

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

	// incoming def
	var deployment DeploymentDescription
	if err := json.Unmarshal([]byte(configure.Deployment), &deployment); err != nil {
		return nil, err
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

	servicePairs, err := finalizeDeployment(agreementId, &deployment, environmentAdditions, workloadROStorageDir, b.Config.Edge.DefaultCPUSet)
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
			return nil, err
		} else if image == nil {
			return nil, fmt.Errorf("Unable to find Docker image: %v", servicePair.serviceConfig.Config.Image)
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

		// append variation label if it exists
		if servicePair.service.VariationLabel != "" {
			bridgeName = fmt.Sprintf("%v-%v", bridgeName, servicePair.service.VariationLabel)
		}

		var existingNetwork *docker.Network
		var existingContainer *docker.APIContainers

		existingNetwork, existingContainer, err = existingShared(b.client, serviceName, &servicePair, bridgeName, shareLabel)
		if err != nil {
			return nil, fail(nil, serviceName, fmt.Errorf("Failed to discover and use existing shared containers. Original error: %v", err))
		}

		if existingNetwork == nil {
			existingNetwork, err = mkBridge(bridgeName, b.client)
			glog.V(2).Infof("Created new network for shared container: %v. Network: %v", serviceName, existingNetwork)
			if err != nil {
				return nil, fail(nil, serviceName, fmt.Errorf("Unable to create bridge for shared container. Original error: %v", err))
			}
		}

		glog.V(4).Infof("Using network for shared service: %v. Network: %v", serviceName, existingNetwork.ID)

		// retain reference so we can wire "private" containers from this agreement to this bridge later; need to do this even if we already saw a net
		eps := mkEndpoints(existingNetwork, serviceName)
		recordEndpoints(sharedEndpoints, eps)

		if existingContainer == nil {
			// only create container if there wasn't one
			servicePair.serviceConfig.HostConfig.NetworkMode = bridgeName
			if err := serviceStart(b.client, agreementId, serviceName, shareLabel, servicePair.serviceConfig, eps, nil, &postCreateContainers, fail); err != nil {
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

	if err := processPostCreate(b.iptables, b.client, agreementId, deployment, configureRaw, hasSpecifiedEthAccount, postCreateContainers, fail); err != nil {
		return nil, err
	}

	for name, _ := range ret {
		glog.Infof("Created service %v in agreement %v", name, agreementId)
	}
	return &ret, nil
}

func (b *ContainerWorker) start() {
	go func() {

		for {
			glog.V(4).Infof("ContainerWorker command processor blocking waiting to receive incoming commands")

			command := <-b.Commands

			switch command.(type) {
			case *ContainerConfigureCommand:
				cmd := command.(*ContainerConfigureCommand)
				glog.V(3).Infof("ContainerWorker received configure command: %v", cmd)
				if len(cmd.ImageFiles) == 0 {
					glog.Infof("Command specified no new Docker images to load, expecting that the caller knows they're preloaded and this is not a bug. Skipping load operation")

				} else if err := loadImages(b.client, b.Config.Edge.TorrentDir, cmd.ImageFiles); err != nil {
					glog.Errorf("Error loading image files: %v", err)

					b.Messages() <- events.NewContainerMessage(events.EXECUTION_FAILED, cmd.AgreementLaunchContext.AgreementProtocol, cmd.AgreementLaunchContext.AgreementId, nil)

					continue
				}

				if deployment, err := b.resourcesCreate(cmd.AgreementLaunchContext.AgreementId, cmd.AgreementLaunchContext.Configure, cmd.AgreementLaunchContext.ConfigureRaw, *cmd.AgreementLaunchContext.EnvironmentAdditions); err != nil {
					glog.Errorf("Error starting containers: %v", err)
					b.Messages() <- events.NewContainerMessage(events.EXECUTION_FAILED, cmd.AgreementLaunchContext.AgreementProtocol, cmd.AgreementLaunchContext.AgreementId, *deployment) // still using deployment here, need it to shutdown containers

				} else {
					glog.Infof("Success starting pattern for agreement: %v, protocol: %v, serviceNames: %v", cmd.AgreementLaunchContext.AgreementId, cmd.AgreementLaunchContext.AgreementProtocol, persistence.ServiceConfigNames(deployment))

					// perhaps add the tc info to the container message so it can be enforced
					b.Messages() <- events.NewContainerMessage(events.EXECUTION_BEGUN, cmd.AgreementLaunchContext.AgreementProtocol, cmd.AgreementLaunchContext.AgreementId, *deployment)
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
					b.Messages() <- events.NewContainerMessage(events.EXECUTION_FAILED, cmd.AgreementProtocol, cmd.AgreementId, cmd.Deployment)
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

			default:
				glog.Errorf("Unsupported command: %v", command)

			}
			runtime.Gosched()
		}
	}()
}

func (b *ContainerWorker) resourcesRemove(agreements []string) error {
	glog.V(5).Infof("Killing and removing resources in agreements: %v", agreements)

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

		// remove old workspaceROStorage dir
		workloadROStorageDir := b.workloadStorageDir(agreementId)
		if err := os.RemoveAll(workloadROStorageDir); err != nil {
			return fmt.Errorf("Failed to remove workloadROStorageDir: %v. Error: %v", workloadROStorageDir, err)
		}

		return nil
	}

	b.ContainersMatchingAgreement(agreements, true, destroy)

	// gather agreement networks to free
	for _, net := range networks {
		for _, agreementId := range agreements {
			if net.Name == agreementId {
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

type ContainerConfigureCommand struct {
	ImageFiles             []string
	AgreementLaunchContext *events.AgreementLaunchContext
}

func (c ContainerConfigureCommand) String() string {
	return fmt.Sprintf("ImageFiles: %v, AgreementLaunchContext: %v", c.ImageFiles, c.AgreementLaunchContext)
}

func (b *ContainerWorker) NewContainerConfigureCommand(imageFiles []string, agreementLaunchContext *events.AgreementLaunchContext) *ContainerConfigureCommand {
	return &ContainerConfigureCommand{
		ImageFiles:             imageFiles,
		AgreementLaunchContext: agreementLaunchContext,
	}
}

type ContainerMaintenanceCommand struct {
	AgreementProtocol string
	AgreementId       string
	Deployment        map[string]persistence.ServiceConfig
}

func (c ContainerMaintenanceCommand) String() string {
	return fmt.Sprintf("AgreementProtocol: %v, AgreementId: %v, Deployment: %v", c.AgreementProtocol, c.AgreementId, persistence.ServiceConfigNames(&c.Deployment))
}

func (b *ContainerWorker) NewContainerMaintenanceCommand(protocol string, agreementId string, deployment map[string]persistence.ServiceConfig) *ContainerMaintenanceCommand {
	return &ContainerMaintenanceCommand{
		AgreementProtocol: protocol,
		AgreementId:       agreementId,
		Deployment:        deployment,
	}
}

type ContainerShutdownCommand struct {
	AgreementProtocol  string
	CurrentAgreementId string
	Deployment         map[string]persistence.ServiceConfig
	Agreements         []string
}

func (c ContainerShutdownCommand) String() string {
	return fmt.Sprintf("AgreementProtocol: %v, CurrentAgreementId: %v, Deployment: %v, Agreements (sample): %v", c.AgreementProtocol, c.CurrentAgreementId, persistence.ServiceConfigNames(&c.Deployment), cutil.FirstN(10, c.Agreements))
}

func (b *ContainerWorker) NewContainerShutdownCommand(protocol string, currentAgreementId string, deployment map[string]persistence.ServiceConfig, agreements []string) *ContainerShutdownCommand {
	return &ContainerShutdownCommand{
		AgreementProtocol:  protocol,
		CurrentAgreementId: currentAgreementId,
		Deployment:         deployment,
		Agreements:         agreements,
	}
}
