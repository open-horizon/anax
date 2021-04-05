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
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/containermessage"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/eventlog"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/resource"
	"github.com/open-horizon/anax/worker"
	"golang.org/x/sys/unix"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"os/user"
	"path"
	"strconv"
	"strings"
)

const LABEL_PREFIX = "openhorizon.anax"
const IPT_COLONUS_ISOLATED_CHAIN = "OPENHORIZON-ANAX-ISOLATION"

const (
	API_SERVER_TYPE_DOCKER = "docker"
	API_SERVER_TYPE_PODMAN = "podman"
	LOG_DRIVER_SYSLOG      = "syslog"
	LOG_DRIVER_JOURNALD    = "journald"
)

// messages for event logs
const (
	EL_CONT_DEPLOYCONF_UNSUPPORT_CAP_FOR_WL   = "Deployment config %v contains unsupported capability for a workload"
	EL_CONT_DEPLOYCONF_UNSUPPORT_CAP_FOR_CONT = "Deployment config %v contains unsupported capability for infrastructure container."
	EL_CONT_DEPLOYCONF_UNSUPPORT_BIND         = "Deployment config %v contains unsupported bind for a workload, %v"
	EL_CONT_DEPLOYCONF_UNSUPPORT_BIND_FOR     = "Deployment config %v contains unsupported bind for %v, %v"
	EL_CONT_ERROR_UNMARSHAL_DEPLOY            = "Error Unmarshalling deployment string %v, error: %v"
	EL_CONT_ERROR_UNMARSHAL_DEPLOY_OVERRIDE   = "Error Unmarshalling deployment override string %v for agreement %v, error: %v"
	EL_CONT_START_CONTAINER_ERROR             = "Error starting containers: %v"
	EL_CONT_START_CONTAINER_ERROR_FOR_AG      = "Error starting containers for agreement %v: %v"
	EL_CONT_RESTART_CONTAINER_ERROR_FOR_AG    = "Error restarting containers for agreements %v: %v"
	EL_CONT_CLEAN_OLD_CONTAINER_ERROR         = "Error cleaning up old containers before starting up new containers for %v. Error: %v"
	EL_CONT_FAIL_GET_PAENT_CONT_FOR_SVC       = "Failed to get a list of parent containers for service retry for %v. %v"
	EL_CONT_FAIL_RESTORE_NW_WITH_PARENT       = "Failed to restoring the network connection with the parents for service %v. %v"
	EL_CONT_TERM_UNABLE_ACCESS_STORAGE_DIR    = "anax terminating. Unable to access service storage direcotry specified in config: %v. %v"
	EL_CONT_TERM_UNABLE_INIT_IPTABLE_CLIENT   = "anax terminating. Failed to instantiate iptables client. %v"
	EL_CONT_TERM_UNABLE_INIT_DOCKER_CLIENT    = "anax terminating. Failed to instantiate docker client. %v"
)

// This is does nothing useful at run time.
// This code is only used in compileing time to make the eventlog messages gets into the catalog so that
// they can be translated.
// The event log messages will be saved in English. But the CLI can request them in different languages.
func MarkI18nMessages() {
	// get message printer. anax default language is English
	msgPrinter := i18n.GetMessagePrinter()

	msgPrinter.Sprintf(EL_CONT_DEPLOYCONF_UNSUPPORT_CAP_FOR_WL)
	msgPrinter.Sprintf(EL_CONT_DEPLOYCONF_UNSUPPORT_CAP_FOR_CONT)
	msgPrinter.Sprintf(EL_CONT_DEPLOYCONF_UNSUPPORT_BIND)
	msgPrinter.Sprintf(EL_CONT_DEPLOYCONF_UNSUPPORT_BIND_FOR)
	msgPrinter.Sprintf(EL_CONT_ERROR_UNMARSHAL_DEPLOY)
	msgPrinter.Sprintf(EL_CONT_ERROR_UNMARSHAL_DEPLOY_OVERRIDE)
	msgPrinter.Sprintf(EL_CONT_START_CONTAINER_ERROR)
	msgPrinter.Sprintf(EL_CONT_START_CONTAINER_ERROR_FOR_AG)
	msgPrinter.Sprintf(EL_CONT_RESTART_CONTAINER_ERROR_FOR_AG)
	msgPrinter.Sprintf(EL_CONT_CLEAN_OLD_CONTAINER_ERROR)
	msgPrinter.Sprintf(EL_CONT_FAIL_GET_PAENT_CONT_FOR_SVC)
	msgPrinter.Sprintf(EL_CONT_FAIL_RESTORE_NW_WITH_PARENT)
	msgPrinter.Sprintf(EL_CONT_TERM_UNABLE_ACCESS_STORAGE_DIR)
	msgPrinter.Sprintf(EL_CONT_TERM_UNABLE_INIT_IPTABLE_CLIENT)
	msgPrinter.Sprintf(EL_CONT_TERM_UNABLE_INIT_DOCKER_CLIENT)
}

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

const T_CONFIGURE = "CONFIGURE"

type WhisperProviderMsg struct {
	Type string `json:"type"`
}

// message sent to contract owner from provider
type Configure struct {
	// embedded
	WhisperProviderMsg
	ConfigureNonce      string `json:"configure_nonce"`
	Deployment          string `json:"deployment"` // JSON docker-compose like
	DeploymentSignature string `json:"deployment_signature"`
	DeploymentUserInfo  string `json:"deployment_user_info"`
}

func (c Configure) String() string {
	return fmt.Sprintf("Type: %v, ConfigureNonce: %v, Deployment: %v, DeploymentSignature: %v, DeploymentUserInfo: %v", c.Type, c.ConfigureNonce, c.Deployment, c.DeploymentSignature, c.DeploymentUserInfo)
}

func NewConfigure(configureNonce string, deployment string, deploymentSignature string, deploymentUserInfo string) *Configure {
	return &Configure{
		WhisperProviderMsg:  WhisperProviderMsg{Type: T_CONFIGURE},
		ConfigureNonce:      configureNonce,
		Deployment:          deployment,
		DeploymentSignature: deploymentSignature,
		DeploymentUserInfo:  deploymentUserInfo,
	}

}

// an internal convenience type
type servicePair struct {
	service       *containermessage.Service  // the external type
	serviceConfig *persistence.ServiceConfig // the internal type
}

func hashService(service *containermessage.Service) (string, error) {
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

// This function will remove an env var that is already in the array. This function
// modifies the input array.
func removeDuplicateVariable(existingArray *[]string, newVar string) {

	// Match the variable name and remove it from the array. We dont care about the value of the variable.
	for ix, v := range *existingArray {
		envvar := v[:strings.Index(v, "=")]
		if envvar == newVar[:strings.Index(newVar, "=")] {
			(*existingArray) = append((*existingArray)[:ix], (*existingArray)[ix+1:]...)
			return
		}
	}

}

func (w *ContainerWorker) finalizeDeployment(agreementId string, deployment *containermessage.DeploymentDescription, environmentAdditions map[string]string, workloadRWStorageDir string, cpuSet string, uds string) (map[string]servicePair, error) {

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

		// If the FSS is using a unix domain socket listener, add a filesystem binding for it.
		if uds != "" {
			service.Binds = append(service.Binds, fmt.Sprintf("%v:%v", uds, uds))
		}

		// Add a filesystem binding for the FSS (ESS) API authentication credentials.
		service.Binds = append(service.Binds, fmt.Sprintf("%v:%v:ro", w.GetAuthenticationManager().GetCredentialPath(agreementId), config.HZN_FSS_AUTH_MOUNT))

		// Add a filesystem binding for the FSS (ESS) API SSL client certificate.
		service.Binds = append(service.Binds, fmt.Sprintf("%v:%v:ro", w.Config.GetESSSSLClientCertPath(), config.HZN_FSS_CERT_MOUNT))

		// Get the group id that owns the service ess auth folder/file. Add this group id in the GroupAdd fields in docker.HostConfig. So that service account in service container can read ess auth folder/file (750)
		groupName := cutil.GetHashFromString(agreementId)
		group, err := user.LookupGroup(groupName)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("unable to find group %v created for ess auth file %v", groupName, agreementId))
		}

		groupAdds := make([]string, 0)
		groupAdds = append(groupAdds, group.Gid)

		// Create the volume map based on the container paths being bound to the host.
		// The bind string looks like this: <host-path>:<container-path>:<ro> where ro means readonly and is optional.
		vols := make(map[string]struct{})
		for _, bind := range service.Binds {
			containerVol := strings.Split(bind, ":")
			if len(containerVol) > 1 && containerVol[1] != "" {
				vols[containerVol[1]] = struct{}{}
			}
		}

		// setup labels and log config for the new container
		labels := make(map[string]string)
		labels[LABEL_PREFIX+".service_name"] = serviceName
		labels[LABEL_PREFIX+".variation"] = service.VariationLabel
		labels[LABEL_PREFIX+".deployment_description_hash"] = deploymentHash
		if w.IsDevInstance() {
			labels[LABEL_PREFIX+".dev_service"] = "true"
		}

		var logConfig docker.LogConfig

		// Use -log-driver defined in the deployment string of the service.
		// If -log-driver is not defined
		// Use syslog log driver by default.
		// Use journald log driver by default if podman is running
		logDriver := LOG_DRIVER_SYSLOG
		if w.apiServerType == API_SERVER_TYPE_PODMAN {
			logDriver = LOG_DRIVER_JOURNALD
		}
		if service.LogDriver != "" {
			logDriver = service.LogDriver
		}

		if !deployment.ServicePattern.IsShared("singleton", serviceName) {
			labels[LABEL_PREFIX+".agreement_id"] = agreementId
			logConfig = docker.LogConfig{
				Type: logDriver,
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
				Type: logDriver,
				Config: map[string]string{
					"tag": fmt.Sprintf("workload-%v_%v", "singleton", logName),
				},
			}
		}

		// Some log drivers don't support tagging, the "tag" config should be removed for them
		if !cliutils.LoggingDriverSupportsTagging(logDriver) {
			delete(logConfig.Config, "tag")
		}

		serviceConfig := &persistence.ServiceConfig{
			Config: docker.Config{
				Image:        service.Image,
				Env:          []string{},
				Cmd:          service.Command,
				CPUSet:       cpuSet,
				Labels:       labels,
				Volumes:      vols,
				ExposedPorts: map[docker.Port]struct{}{},
			},
			HostConfig: docker.HostConfig{
				Privileged:      service.Privileged,
				NetworkMode:     service.Network,
				CapAdd:          service.CapAdd,
				PublishAllPorts: false,
				PortBindings:    map[docker.Port][]docker.PortBinding{},
				Links:           nil, // do not allow any
				RestartPolicy:   docker.AlwaysRestart(),
				Memory:          ramBytes,
				MemorySwap:      0,
				Devices:         []docker.Device{},
				LogConfig:       logConfig,
				Binds:           service.Binds,
				GroupAdd:        groupAdds,
				Tmpfs:           service.Tmpfs,
			},
		}

		// Set CPU and memory limits if they are defined in the service config
		if service.MaxMemoryMb != 0 {
			serviceConfig.HostConfig.Memory = service.MaxMemoryMb * 1024 * 1024
		}
		if service.MaxCPUs != 0 {
			serviceConfig.HostConfig.NanoCPUs = int64(service.MaxCPUs * 1000000000)
		}

		// Mark each container as infrastructure if the deployment description indicates infrastructure
		if deployment.Infrastructure {
			serviceConfig.Config.Labels[LABEL_PREFIX+".infrastructure"] = ""
		}

		// add environment additions to each service
		for k, v := range environmentAdditions {
			serviceConfig.Config.Env = append(serviceConfig.Config.Env, fmt.Sprintf("%s=%v", k, v))
		}

		// add the environment variables from the deployment definition
		for _, v := range service.Environment {
			// skip this one b/c it's dangerous
			if !strings.HasPrefix(config.ENVVAR_PREFIX+"ETHEREUM_ACCOUNT", v) {
				serviceConfig.Config.Env = append(serviceConfig.Config.Env, v)
			}
		}

		// overwrite container's entrypoint if it's set in deployment
		if len(service.Entrypoint) != 0 {
			serviceConfig.Config.Entrypoint = service.Entrypoint
		}

		// add the environment variable overrides
		if len(deployment.Overrides) == 0 {
			// nothing
		} else if _, ok := deployment.Overrides[serviceName]; !ok {
			// nothing
		} else {
			for _, v := range deployment.Overrides[serviceName].Environment {
				// If the env var array already has the variable then we need to remove it before
				// we add the new one.
				removeDuplicateVariable(&serviceConfig.Config.Env, v)
				serviceConfig.Config.Env = append(serviceConfig.Config.Env, v)
			}
		}

		for _, port := range service.EphemeralPorts {
			var hostIP string

			if port.LocalhostOnly {
				hostIP = "127.0.0.1"
			} else {
				hostIP = "0.0.0.0"
			}

			if port.PortAndProtocol == "" {
				return nil, fmt.Errorf("Failed to locate necessary port setup param, PortAndProtocol in %v", port)
			}

			// default is tcp protocal
			if !strings.Contains(port.PortAndProtocol, "/") {
				port.PortAndProtocol = port.PortAndProtocol + "/tcp"
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

		// HostPort schema: <host_port>:<container_port>/<protocol>
		// If <host_port> is absent, <container_port> is used instead.
		// If <protocol> is absent, "/tcp" is used.
		// service.SpecificPorts is for backward compatibility, the new way is using service.Ports.
		for _, specificPort := range append(service.SpecificPorts, service.Ports...) {
			var emptyS struct{}
			cPort := ""
			hPort := ""
			pieces := strings.Split(specificPort.HostPort, ":")
			hPort = pieces[0]
			if len(pieces) < 2 {
				cPort = pieces[0]
			} else {
				cPort = pieces[1]
			}
			if !strings.Contains(cPort, "/") {
				cPort = cPort + "/tcp"
			}

			// trim the protocol part for hPort
			port_pieces := strings.Split(hPort, "/")
			hPort = port_pieces[0]

			dPort := docker.Port(cPort)
			serviceConfig.Config.ExposedPorts[dPort] = emptyS

			hMapping := docker.PortBinding{
				HostIP:   specificPort.HostIP,
				HostPort: hPort,
			}
			serviceConfig.HostConfig.PortBindings[dPort] = append(serviceConfig.HostConfig.PortBindings[dPort], hMapping)
		}

		// The format of device mapping is: <host device name>:<contianer device name>:<cgroup permission>
		// the cgoup permission can be omitted. It defaults to "rwm" when omitted.
		for _, givenDevice := range service.Devices {
			cgp := "rwm"
			pic := ""
			sp := strings.Split(givenDevice, ":")
			if len(sp) == 3 {
				// the cgroup permission
				cgp = sp[2]
				pic = sp[1]
			} else if len(sp) == 2 {
				pic = sp[1]
			} else if len(sp) == 1 {
				pic = sp[0]
			} else if len(sp) <= 0 || len(sp) > 3 {
				return nil, fmt.Errorf("Illegal device specified in deployment description: %v", givenDevice)
			}

			serviceConfig.HostConfig.Devices = append(serviceConfig.HostConfig.Devices, docker.Device{
				PathOnHost:        sp[0],
				PathInContainer:   pic,
				CgroupPermissions: cgp,
			})
		}

		services[serviceName] = servicePair{
			serviceConfig: serviceConfig,
			service:       service,
		}
	}

	return services, nil
}

// Check if the client is talking with docker or podman
// The /version api returns something like:
// {
//   "Components": [{"Name": "Podman Engine","Version": "3.1.0-dev",...]
//   ...
// }
func GetServerEnginType(client *docker.Client) (string, error) {
	svType := API_SERVER_TYPE_DOCKER

	if client == nil {
		return svType, fmt.Errorf("Invalid client pointer: nil.")
	}

	versionInfo, err := client.Version()
	if err != nil {
		return svType, fmt.Errorf("Failed to get the container HTTP server version info. %v", err)
	}
	glog.V(5).Infof("API version info: %v", versionInfo)

	if versionInfo != nil {
		for _, info := range *versionInfo {
			if strings.Contains(strings.ToLower(info), "podman") {
				glog.V(3).Infof("podman endpoint is detected.")
				svType = API_SERVER_TYPE_PODMAN
				break
			}
		}
	}

	return svType, nil
}

type ContainerWorker struct {
	worker.BaseWorker // embedded field
	db                *bolt.DB
	client            *docker.Client
	iptables          *iptables.IPTables
	authMgr           *resource.AuthenticationManager
	pattern           string
	isDevInstance     bool
	apiServerType     string
}

func (cw *ContainerWorker) GetClient() *docker.Client {
	return cw.client
}

func (cw *ContainerWorker) IsDevInstance() bool {
	return cw.isDevInstance
}

func (cw *ContainerWorker) GetAuthenticationManager() *resource.AuthenticationManager {
	return cw.authMgr
}

func CreateCLIContainerWorker(config *config.HorizonConfig) (*ContainerWorker, error) {
	dockerEP := "unix:///var/run/docker.sock"
	client, derr := docker.NewClient(dockerEP)
	if derr != nil {
		return nil, derr
	}

	svType, err := GetServerEnginType(client)
	if err != nil {
		return nil, err
	}

	return &ContainerWorker{
		BaseWorker:    worker.NewBaseWorker("mock", config, nil),
		db:            nil,
		client:        client,
		iptables:      nil,
		authMgr:       resource.NewAuthenticationManager(config.GetFileSyncServiceAuthPath()),
		pattern:       "",
		isDevInstance: true,
		apiServerType: svType,
	}, nil
}

func NewContainerWorker(name string, config *config.HorizonConfig, db *bolt.DB, am *resource.AuthenticationManager) *ContainerWorker {

	// do not start this container if the the node is registered and the type is cluster
	dev, _ := persistence.FindExchangeDevice(db)
	if dev != nil && dev.GetNodeType() == persistence.DEVICE_TYPE_CLUSTER {
		return nil
	}

	// If config.Edge.ServiceStorage is not empty, then we assume that the local file system directory will
	// be used for the storage of the service container.
	// If config.Edge.ServiceStorage is empty, docker volume will be used instead for the storage of the service container.
	if config.Edge.ServiceStorage != "" {
		if err := unix.Access(config.Edge.ServiceStorage, unix.W_OK); err != nil {
			glog.Errorf("Unable to access service storage dir: %v. Error: %v", config.Edge.ServiceStorage, err)
			eventlog.LogNodeEvent(db, persistence.SEVERITY_FATAL,
				persistence.NewMessageMeta(EL_CONT_TERM_UNABLE_ACCESS_STORAGE_DIR, config.Edge.ServiceStorage, err.Error()),
				persistence.EC_ERROR_ACCESS_STORAGE_DIR,
				"", "", "", "")
			panic(fmt.Sprintf("Terminating, unable to access service storage dir specified in config: %v. %v", config.Edge.ServiceStorage, err))
		}
	}

	var err error
	var ipt *iptables.IPTables
	var client *docker.Client

	ipt, err = iptables.New()
	if err != nil {
		glog.Errorf("Failed to instantiate iptables Client: %v", err)
		eventlog.LogNodeEvent(db, persistence.SEVERITY_FATAL,
			persistence.NewMessageMeta(EL_CONT_TERM_UNABLE_INIT_IPTABLE_CLIENT, err.Error()),
			persistence.EC_ERROR_CREATE_IPTABLE_CLIENT,
			"", "", "", "")
		panic(fmt.Sprintf("Terminating, unable to instantiate iptables Client. %v", err))
	}

	if config.Edge.DockerEndpoint != "" {
		client, err = docker.NewClient(config.Edge.DockerEndpoint)
		if err != nil {
			glog.Errorf("Failed to instantiate docker Client: %v", err)
			eventlog.LogNodeEvent(db, persistence.SEVERITY_FATAL,
				persistence.NewMessageMeta(EL_CONT_TERM_UNABLE_INIT_DOCKER_CLIENT, err.Error()),
				persistence.EC_ERROR_CREATE_DOCKER_CLIENT,
				"", "", "", "")
			panic(fmt.Sprintf("Terminating, unable to instantiate docker Client. %v", err))
		}
	}

	svType, err := GetServerEnginType(client)
	if err != nil {
		glog.Errorf(fmt.Sprintf("Failed to get the docker API server engine type. %v", err))
	}

	pattern := ""
	if dev != nil {
		pattern = dev.Pattern
	}

	worker := &ContainerWorker{
		BaseWorker:    worker.NewBaseWorker(name, config, nil),
		db:            db,
		client:        client,
		iptables:      ipt,
		authMgr:       am,
		pattern:       pattern,
		apiServerType: svType,
	}
	worker.SetDeferredDelay(15)

	worker.Start(worker, 0)
	return worker
}

func (w *ContainerWorker) Messages() chan events.Message {
	return w.BaseWorker.Manager.Messages
}

func (w *ContainerWorker) NewEvent(incoming events.Message) {

	switch incoming.(type) {
	case *events.EdgeRegisteredExchangeMessage:
		msg, _ := incoming.(*events.EdgeRegisteredExchangeMessage)
		w.pattern = msg.Pattern()

		// stop the container worker for the cluster device type
		if msg.DeviceType() == persistence.DEVICE_TYPE_CLUSTER {
			w.Commands <- worker.NewTerminateCommand("cluster node")
		}

	case *events.ImageFetchMessage:
		msg, _ := incoming.(*events.ImageFetchMessage)
		switch msg.Event().Id {
		case events.IMAGE_FETCHED:
			glog.Infof("Fetched image files in deployment description for services: %v", msg.DeploymentDescription.ServiceNames())
			switch msg.LaunchContext.(type) {
			case *events.AgreementLaunchContext:
				lc := msg.LaunchContext.(*events.AgreementLaunchContext)
				cCmd := w.NewWorkloadConfigureCommand(msg.DeploymentDescription, lc)
				w.Commands <- cCmd

			case *events.ContainerLaunchContext:
				lc := msg.LaunchContext.(*events.ContainerLaunchContext)
				cCmd := w.NewContainerConfigureCommand(msg.DeploymentDescription, lc)
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

	case *events.GovernanceWorkloadCancelationMessage:
		msg, _ := incoming.(*events.GovernanceWorkloadCancelationMessage)

		switch msg.Event().Id {
		case events.AGREEMENT_ENDED:
			containerCmd := w.NewWorkloadShutdownCommand(msg.AgreementProtocol, msg.AgreementId, msg.Deployment, []string{})
			w.Commands <- containerCmd
		}

	case *events.ContainerStopMessage:
		msg, _ := incoming.(*events.ContainerStopMessage)

		switch msg.Event().Id {
		case events.CONTAINER_STOPPING:
			containerCmd := w.NewContainerStopCommand(msg)
			w.Commands <- containerCmd
		}

	case *events.MicroserviceMaintenanceMessage:
		msg, _ := incoming.(*events.MicroserviceMaintenanceMessage)

		switch msg.Event().Id {
		case events.CONTAINER_MAINTAIN:
			containerCmd := w.NewMaintainMicroserviceCommand(msg.MsInstKey)
			w.Commands <- containerCmd
		}

	case *events.MicroserviceCancellationMessage:
		msg, _ := incoming.(*events.MicroserviceCancellationMessage)

		switch msg.Event().Id {
		case events.CANCEL_MICROSERVICE:
			containerCmd := w.NewShutdownMicroserviceCommand(msg.MsInstKey)
			w.Commands <- containerCmd
		case events.CANCEL_MICROSERVICE_NETWORK:
			containerCmd := w.NewCancelMicroserviceNetworkCommand(msg.MsInstKey)
			w.Commands <- containerCmd
		}

	case *events.NodeShutdownCompleteMessage:
		msg, _ := incoming.(*events.NodeShutdownCompleteMessage)
		switch msg.Event().Id {
		case events.UNCONFIGURE_COMPLETE:
			w.Commands <- NewNodeUnconfigCommand(msg)
		}

	default: // nothing

	}

	return
}

func MakeBridge(client *docker.Client, name string, infrastructure, sharedPattern, isDev bool) (*docker.Network, error) {

	// Labels on the docker network indicate attributes about the network.
	labels := make(map[string]string)

	if isDev {
		labels[LABEL_PREFIX+".dev_network"] = "true"
	} else {
		labels[LABEL_PREFIX+".network"] = ""
	}
	if infrastructure {
		labels[LABEL_PREFIX+".infrastructure"] = ""
	}
	if sharedPattern {
		labels[LABEL_PREFIX+".service_pattern.shared"] = "singleton"
	}

	bridgeOpts := docker.CreateNetworkOptions{
		Name:           name,
		EnableIPv6:     false,
		Internal:       false,
		Driver:         "bridge",
		CheckDuplicate: true,
		IPAM: &docker.IPAMOptions{
			Driver: "default",
			Config: []docker.IPAMConfig{},
		},
		Options: map[string]interface{}{
			"com.docker.network.bridge.enable_icc":           "true",
			"com.docker.network.bridge.enable_ip_masquerade": "true",
			"com.docker.network.bridge.default_bridge":       "false",
		},
		Labels: labels,
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
	fail func(container *docker.Container, name string, err error) error,
	isFirstTry bool) error {

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

	// this for the retry after log driver using syslog failed.
	if !isFirstTry {
		containerOpts.HostConfig.LogConfig = docker.LogConfig{}
	}

	glog.V(5).Infof("CreateContainer options: Config: %v, HostConfig: %v, EndpointsConfig: %v", serviceConfig.Config, serviceConfig.HostConfig, endpointsConfig)

	container, cErr := client.CreateContainer(containerOpts)
	if cErr != nil {
		if cErr == docker.ErrContainerAlreadyExists {
			return cErr
		} else {
			return fail(container, serviceName, cErr)
		}
	}

	// second arg just a backwards compat feature, will go away someday
	logDriverName := serviceConfig.HostConfig.LogConfig.Type
	err := client.StartContainer(container.ID, nil)
	if err != nil {
		if strings.Contains(err.Error(), "logging driver") && (strings.Contains(err.Error(), LOG_DRIVER_SYSLOG) || strings.Contains(err.Error(), LOG_DRIVER_JOURNALD)) {
			// prevent infinit loop, just in case
			if !isFirstTry {
				return fail(container, serviceName, err)
			}

			// if the error is related to syslog or journald, use the default for logconfig and retry
			glog.V(3).Infof("StartContainer logconfig cannot use %v: %v. Switching to default. You can use 'docker logs -f <container_name> to view the logs.", logDriverName, err)

			if err_r := client.RemoveContainer(docker.RemoveContainerOptions{ID: container.ID, RemoveVolumes: false, Force: true}); err_r != nil {
				return fail(container, serviceName, err_r)
			} else {
				return serviceStart(client, agreementId, serviceName, shareLabel, serviceConfig, endpointsConfig,
					sharedEndpoints, postCreateContainers, fail, false)
			}
		} else {
			return fail(container, serviceName, err)
		}
	}
	if serviceConfig.HostConfig.NetworkMode != "host" {
		for _, cfg := range sharedEndpoints {
			glog.V(5).Infof("Connecting network: %v to container id: %v as endpoint: %v", cfg.NetworkID, container.ID, cfg.Aliases)
			err := client.ConnectNetwork(cfg.NetworkID, docker.NetworkConnectionOptions{
				Container:      container.ID,
				EndpointConfig: cfg,
				Force:          true,
			})
			if err != nil {
				return fail(container, serviceName, err)
			}
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
		} else if _, ok := err.(*docker.ContainerNotRunning); !ok {
			glog.Warningf("Unable to kill container in agreement: %v. Error: %v. Will try to forcefully remove it.", agreementId, err)
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
		if isAnaxNetwork(&net, bridgeName) {
			glog.V(3).Infof("Found shared network: %v with name %v", net.ID, net.Name)
			sBridge = net
		}
	}

	// some of the facts in the labels that are compared will also be in the hash
	sharedOnly := docker.ListContainersOptions{
		All: true,
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
	glog.V(5).Infof("Found containers %v", containers)

	// The container we're looking for might exist, but might not be running. If it exists but is not running
	// docker will prevent us from starting a new container so we have to get rid of it.
	for _, con := range containers {
		if con.State != "running" {
			for _, name := range con.Names {
				if strings.TrimLeft(name, "/") == bridgeName {
					// We found the shared container, but it is not running
					if err := client.RemoveContainer(docker.RemoveContainerOptions{ID: con.ID, RemoveVolumes: true, Force: true}); err != nil {
						glog.Errorf("Error removing stopped shared container %v %v, error %v", con.Names, con.ID, err)
						return nil, nil, err
					}
					break
				}
			}
			// Do the container search again so that we are working with an updated list
			containers, err = client.ListContainers(sharedOnly)
			if err != nil {
				return nil, nil, err
			}
			glog.V(5).Infof("Found containers again %v", containers)
		}
	}

	// Return the shared resources that we found
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

func generatePermittedString(isolation *containermessage.NetworkIsolation, network docker.ContainerNetwork, configureRaw []byte) (string, error) {

	permittedString := ""

	for _, permitted := range isolation.OutboundPermitOnly {
		var permittedValue containermessage.OutboundPermitValue

		switch permitted.(type) {
		case containermessage.StaticOutboundPermitValue:
			permittedValue = permitted.(containermessage.StaticOutboundPermitValue)

		case containermessage.DynamicOutboundPermitValue:
			p := permitted.(containermessage.DynamicOutboundPermitValue)
			// do specialized ad-hoc deserialization of the configure whisper message in order to read the dynamic permit value
			var configureUnstruct map[string]interface{}
			if p.Encoding != containermessage.JSON {
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

func processPostCreate(ipt *iptables.IPTables, client *docker.Client, agreementId string, deployment containermessage.DeploymentDescription, configureRaw []byte, hasSpecifiedEthAccount bool, containers []interface{}, fail func(container *docker.Container, name string, err error) error) error {
	// check if any of the service containers require iptables manipulation to limit outbound traffic. If not, skip this step
	requiresProcessPostCreate := false
	for _, con := range containers {
		switch con.(type) {
		case *docker.Container:
			container := con.(*docker.Container)

			// incoming "container" type does not have Config member
			conDetail, err := client.InspectContainer(container.ID)
			if err != nil {
				return fail(nil, container.Name, fmt.Errorf("Unable to find container detail for container during post-creation step: Error: %v", err))
			}
			if serviceName, exists := conDetail.Config.Labels[LABEL_PREFIX+".service_name"]; exists {
				if deployment.Services[serviceName].NetworkIsolation != nil {
					requiresProcessPostCreate = true
				}
			}
		}
	}

	if !requiresProcessPostCreate {
		return nil
	}

	if ipt != nil {
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

				if isolation != nil && isolation.OutboundPermitOnly != nil {
					if isolation.OutboundPermitOnlyIgnore == containermessage.ETH_ACCT_SPECIFIED && hasSpecifiedEthAccount {
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

							permittedString, err := generatePermittedString(isolation, network, configureRaw)
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

// Return the base workload rw storage directory.
// If Config.Edge.ServiceStorage is empty then the docker volume is used, it returns the new volume name.
func (b *ContainerWorker) workloadStorageDir(agreementId string) (string, bool) {
	if b.Config.Edge.ServiceStorage != "" {
		return path.Join(b.Config.Edge.ServiceStorage, agreementId), false
	} else {
		return agreementId, true
	}
}

// This function creates the containers, volumes, networks for the given agreement or service.
func (b *ContainerWorker) ResourcesCreate(agreementId string, agreementProtocol string, deployment *containermessage.DeploymentDescription, configureRaw []byte, environmentAdditions map[string]string, ms_networks map[string]string, serviceURL string, sVer string) (persistence.DeploymentConfig, error) {

	// local helpers
	fail := func(container *docker.Container, name string, err error) error {
		if container != nil {
			glog.Errorf("Error processing container setup: %v", container)
		}

		glog.Errorf("Failed to set up %v. Attempting to remove other resources in agreement (%v) before returning control to caller. Error: %v", name, agreementId, err)

		rErr := b.ResourcesRemove([]string{agreementId})
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

			endpoints[name] = &docker.EndpointConfig{
				Aliases:   cfg.Aliases,
				Links:     nil,
				NetworkID: cfg.NetworkID,
			}
		}

		return endpoints
	}

	workloadRWStorageDir, useVolume := b.workloadStorageDir(agreementId)

	if !useVolume {
		// create RO workload storage dir if it doesnt already exist
		if err := os.Mkdir(workloadRWStorageDir, 0700); err != nil {
			if pErr, ok := err.(*os.PathError); ok {
				if pErr.Err.Error() != "file exists" {
					return nil, err
				}
			} else {
				return nil, err
			}
		}

		glog.V(5).Infof("Writing raw config to file in %v. Config data: %v", workloadRWStorageDir, string(configureRaw))
		// write raw to workloadRWStorageDir
		if err := ioutil.WriteFile(path.Join(workloadRWStorageDir, "Configure"), configureRaw, 0644); err != nil {
			return nil, err
		}
	} else {
		// The volume has been specified in the binds section of the deployment config in the WorkloadConfigureCommand and
		// ContainerConfigureCommand command handler section.
		// The volume will be created later if it does not exist.
	}

	// Create the MMS authentication credentials for this container. The only time we need the service version to be
	// part of authentication is when policy is in use.
	serviceVersion := ""
	if b.pattern == "" {
		serviceVersion = sVer
	}
	if err := b.GetAuthenticationManager().CreateCredential(agreementId, serviceURL, serviceVersion); err != nil {
		glog.Errorf("Failed to create MMS Authentication credential file for %v, error %v", agreementId, err)
	}

	servicePairs, err := b.finalizeDeployment(agreementId, deployment, environmentAdditions, workloadRWStorageDir, b.Config.Edge.DefaultCPUSet, b.Config.GetFileSyncServiceAPIUnixDomainSocketPath())
	if err != nil {
		return nil, err
	}

	// process services that are "shared" first, then others
	shared := make(map[string]servicePair, 0)
	private := make(map[string]servicePair, 0)

	// trimmed structure to return to caller
	ret := persistence.NativeDeploymentConfig{
		Services: make(map[string]persistence.ServiceConfig, 0),
	}

	// New network will be created if there is at least one service without 'network:host' mode
	newNetworkNeeded := false
	for serviceName, servicePair := range servicePairs {
		if image, err := b.client.InspectImage(servicePair.serviceConfig.Config.Image); err != nil {
			return nil, fail(nil, serviceName, fmt.Errorf("Failed to locally inspect image: %v. Please build and tag image locally or pull the image from your docker repository before running this command. Original error: %v", servicePair.serviceConfig.Config.Image, err))
		} else if image == nil {
			return nil, fail(nil, serviceName, fmt.Errorf("Unable to find Docker image: %v", servicePair.serviceConfig.Config.Image))
		}

		// need to examine original deploymentDescription to determine which containers are "shared" or in other special patterns
		if deployment.ServicePattern.IsShared("singleton", serviceName) {
			shared[serviceName] = servicePair
		} else {
			private[serviceName] = servicePair

			if servicePair.serviceConfig.HostConfig.NetworkMode != "host" {
				newNetworkNeeded = true
			}
		}

		ret.Services[serviceName] = *servicePair.serviceConfig
	}

	// Now that we know we are going to process this deployment, save the deployment config before we create any docker resources.
	if agreementProtocol != "" {
		if _, err := persistence.AgreementDeploymentStarted(b.db, agreementId, agreementProtocol, &ret); err != nil {
			return nil, err
		}
	}

	// create a list of ms shared endpoints for all the workload containers to connect
	ms_sharedendpoints := make(map[string]*docker.EndpointConfig)
	if ms_networks != nil {
		for msnw_name, ms_nw := range ms_networks {
			ms_ep := new(docker.EndpointConfig)
			ms_ep.Aliases = deployment.ServiceNames()
			ms_ep.NetworkID = ms_nw

			ms_sharedendpoints[msnw_name] = ms_ep
		}
	}

	// create the volumes that do not exist yet, The user specified volumes are owned by anax and will be
	// removed during the unregistration process.
	// The 'workloadRWStorageDir' volume will be removed when the agreement is canceled.
	for serviceName, servicePair := range servicePairs {
		if err := b.createDockerVolumesForContainer(serviceName, agreementId, &servicePair); err != nil {
			return nil, err
		}
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
			existingNetwork, err = MakeBridge(b.client, bridgeName, deployment.Infrastructure, true, b.isDevInstance)
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
			if err := serviceStart(b.client, agreementId, containerName, shareLabel, servicePair.serviceConfig, eps, ms_sharedendpoints, &postCreateContainers, fail, true); err != nil {
				return nil, err
			}
		} else {
			// will add a *docker.APIContainers type
			postCreateContainers = append(postCreateContainers, existingContainer)
		}
	}

	// from here on out, need to clean up bridge(s) if there is a problem

	var agBridge *docker.Network
	if newNetworkNeeded {
		// If the network we want already exists, just use it.
		if networks, err := b.client.ListNetworks(); err != nil {
			glog.Errorf("Unable to list networks: %v", err)
			return nil, err
		} else {
			for _, net := range networks {
				if isAnaxNetwork(&net, agreementId) {
					glog.V(5).Infof("Found network %v already present", net.Name)
					agBridge = &net
					break
				}
			}
			if agBridge == nil {
				glog.V(5).Infof("Making network %v", agreementId)
				newBridge, err := MakeBridge(b.client, agreementId, deployment.Infrastructure, false, b.isDevInstance)
				if err != nil {
					return nil, err
				}
				agBridge = newBridge
			}
		}
	}

	// add ms endpoints to the sharedEndpoints
	if ms_sharedendpoints != nil {
		recordEndpoints(sharedEndpoints, ms_sharedendpoints)
	}

	// every one of these gets wired to both the agBridge and every shared bridge from this agreement
	for serviceName, servicePair := range private {
		if servicePair.serviceConfig.HostConfig.NetworkMode == "" {
			servicePair.serviceConfig.HostConfig.NetworkMode = agreementId // custom bridge has agreementId as name, same as endpoint key
		}
		var endpoints map[string]*docker.EndpointConfig
		if servicePair.serviceConfig.HostConfig.NetworkMode != "host" {
			endpoints = mkEndpoints(agBridge, serviceName)
		}
		if err := serviceStart(b.client, agreementId, serviceName, "", servicePair.serviceConfig, endpoints, sharedEndpoints, &postCreateContainers, fail, true); err != nil {
			if err != docker.ErrContainerAlreadyExists {
				return nil, err
			}
		}
	}

	// check environmentAdditions for MTN_ETHEREUM_ACCOUNT
	_, hasSpecifiedEthAccount := environmentAdditions[config.ENVVAR_PREFIX+"ETHEREUM_ACCOUNT"]

	if err := processPostCreate(b.iptables, b.client, agreementId, *deployment, configureRaw, hasSpecifiedEthAccount, postCreateContainers, fail); err != nil {
		return nil, err
	}

	for name, _ := range ret.Services {
		glog.V(1).Infof("Created service %v in agreement %v", name, agreementId)
	}
	return &ret, nil
}

func (b *ContainerWorker) Initialize() bool {
	b.syncupResources()
	return true
}

func (b *ContainerWorker) CommandHandler(command worker.Command) bool {

	switch command.(type) {
	case *WorkloadConfigureCommand:
		cmd := command.(*WorkloadConfigureCommand)

		glog.V(3).Infof("ContainerWorker received workload configure command: %v", cmd.ShortString())

		agreementId := cmd.AgreementLaunchContext.AgreementId

		if ags, err := persistence.FindEstablishedAgreements(b.db, cmd.AgreementLaunchContext.AgreementProtocol, []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(agreementId)}); err != nil {
			glog.Errorf("Unable to retrieve agreement %v from database, error %v", agreementId, err)
		} else if len(ags) != 1 {
			glog.Infof("Ignoring the configure event for agreement %v, the agreement is archived.", agreementId)
		} else if ags[0].AgreementTerminatedTime != 0 {
			glog.Infof("Received configure command for agreement %v. Ignoring it because this agreement has been terminated.", agreementId)
		} else if ags[0].AgreementExecutionStartTime != 0 {
			glog.Infof("Received configure command for agreement %v. Ignoring it because the containers for this agreement has been configured.", agreementId)
		} else if ms_containers, err := b.findDependencyContainersForService(persistence.NewServiceInstancePathElement(ags[0].RunningWorkload.URL, ags[0].RunningWorkload.Org, ags[0].RunningWorkload.Version), []string{agreementId}, cmd.AgreementLaunchContext.Microservices); err != nil {
			glog.Errorf("Error checking service containers: %v", err)

			// requeue the command
			b.AddDeferredCommand(cmd)
			return true
		} else {

			// Now that we have a list of containers on which this workload is dependent, we need to get a list of service
			// network ids to be added to all the workload containers.
			ms_children_networks := b.GatherAndCreateDependencyNetworks(ms_containers, agreementId)

			// We support capabilities in the deployment string that not all container deployments should be able
			// to exploit, e.g. file system mapping from host to container. This check ensures that workloads dont try
			// to do something dangerous.
			deploymentDesc := cmd.DeploymentDescription
			if valid := deploymentDesc.IsValidFor("workload"); !valid {
				eventlog.LogAgreementEvent(b.db, persistence.SEVERITY_ERROR,
					persistence.NewMessageMeta(EL_CONT_DEPLOYCONF_UNSUPPORT_CAP_FOR_WL, cmd.AgreementLaunchContext.Configure.Deployment),
					persistence.EC_ERROR_IN_DEPLOYMENT_CONFIG, ags[0])
				glog.Errorf("Deployment config %v contains unsupported capability for a workload", cmd.AgreementLaunchContext.Configure.Deployment)
				b.Messages() <- events.NewWorkloadMessage(events.EXECUTION_FAILED, cmd.AgreementLaunchContext.AgreementProtocol, agreementId, nil)
			}

			// Add the deployment overrides to the deployment description, if there are any
			if len(cmd.AgreementLaunchContext.Configure.Overrides) != 0 {
				overrideDD := new(containermessage.DeploymentDescription)
				if err := json.Unmarshal([]byte(cmd.AgreementLaunchContext.Configure.Overrides), &overrideDD); err != nil {
					eventlog.LogAgreementEvent(b.db, persistence.SEVERITY_ERROR,
						persistence.NewMessageMeta(EL_CONT_ERROR_UNMARSHAL_DEPLOY_OVERRIDE, cmd.AgreementLaunchContext.Configure.Overrides, agreementId, err.Error()),
						persistence.EC_ERROR_IN_DEPLOYMENT_CONFIG, ags[0])
					glog.Errorf("Error Unmarshalling deployment override string %v for agreement %v, error: %v", cmd.AgreementLaunchContext.Configure.Overrides, agreementId, err)
					return true
				} else {
					deploymentDesc.Overrides = overrideDD.Services
				}
			}

			// Dynamically add in a filesystem mapping so that the workload container has a RO filesystem.
			for serviceName, service := range deploymentDesc.Services {

				if !service.Privileged {
					glog.V(5).Infof("Checking bind permissions for service %v", serviceName)
					if err := hasValidBindPermissions(service.Binds); err != nil {
						eventlog.LogAgreementEvent(b.db, persistence.SEVERITY_ERROR,
							persistence.NewMessageMeta(EL_CONT_DEPLOYCONF_UNSUPPORT_BIND, cmd.AgreementLaunchContext.Configure.Deployment, err.Error()),
							persistence.EC_ERROR_IN_DEPLOYMENT_CONFIG, ags[0])
						glog.Errorf("Deployment config for service %v contains unsupported bind, %v", serviceName, err)
						b.Messages() <- events.NewWorkloadMessage(events.EXECUTION_FAILED, cmd.AgreementLaunchContext.AgreementProtocol, agreementId, nil)
						return true
					}
				}

				dir := ""
				if deploymentDesc.ServicePattern.IsShared("singleton", serviceName) {
					dir, _ = b.workloadStorageDir(fmt.Sprintf("%v-%v-%v", "singleton", serviceName, service.VariationLabel))
				} else {
					dir, _ = b.workloadStorageDir(agreementId)
				}
				deploymentDesc.Services[serviceName].AddFilesystemBinding(fmt.Sprintf("%v:%v:rw", dir, "/service_config"))
			}

			// Each service has an identity that is based on its service defintion URL and Org. This identity is what we can use to
			// authenticate a service to an API that is hosted by Anax.
			serviceIdentity := cutil.FormOrgSpecUrl(cutil.NormalizeURL(ags[0].RunningWorkload.URL), ags[0].RunningWorkload.Org)
			sVer := ags[0].RunningWorkload.Version

			// Create the docker configuration and launch the containers.
			if deploymentConfig, err := b.ResourcesCreate(agreementId, cmd.AgreementLaunchContext.AgreementProtocol, deploymentDesc, cmd.AgreementLaunchContext.ConfigureRaw, *cmd.AgreementLaunchContext.EnvironmentAdditions, ms_children_networks, serviceIdentity, sVer); err != nil {
				eventlog.LogAgreementEvent(b.db, persistence.SEVERITY_ERROR,
					persistence.NewMessageMeta(EL_CONT_START_CONTAINER_ERROR, err.Error()),
					persistence.EC_ERROR_START_CONTAINER,
					ags[0])
				glog.Errorf("Error starting containers: %v", err)
				b.Messages() <- events.NewWorkloadMessage(events.EXECUTION_FAILED, cmd.AgreementLaunchContext.AgreementProtocol, agreementId, deploymentConfig) // still using deployment here, need it to shutdown containers

			} else {
				glog.Infof("Success starting pattern for agreement: %v, protocol: %v, serviceNames: %v", agreementId, cmd.AgreementLaunchContext.AgreementProtocol, deploymentConfig.ToString())

				// perhaps add the tc info to the container message so it can be enforced
				b.Messages() <- events.NewWorkloadMessage(events.EXECUTION_BEGUN, cmd.AgreementLaunchContext.AgreementProtocol, agreementId, deploymentConfig)
			}
		}

	case *ContainerConfigureCommand:
		cmd := command.(*ContainerConfigureCommand)

		glog.V(3).Infof("ContainerWorker received container configure command: %v", cmd.ShortString())

		lc := cmd.ContainerLaunchContext

		// get the service info
		serviceInfo := lc.GetServicePathElement()

		// Verify that the agreements related to this container are still active.
		// Create a new filter for agreements.
		notTerminatedFilter := func() persistence.EAFilter {
			return func(a persistence.EstablishedAgreement) bool {
				return a.AgreementCreationTime != 0 && a.AgreementTerminatedTime == 0
			}
		}

		for _, ag := range lc.GetAgreementIds() {
			glog.V(5).Infof("ContainerWorker checking agreement %v", ag)

			if ags, err := persistence.FindEstablishedAgreementsAllProtocols(b.db, policy.AllAgreementProtocols(), []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(ag), notTerminatedFilter()}); err != nil {
				glog.Errorf("Unable to retrieve agreement %v from database, error %v", ag, err)
			} else if len(ags) != 1 {
				glog.Infof("Ignoring the configure event for agreement %v, the agreement is no longer active.", ag)
				return true
			}
		}

		// For the dependent service retry case, remove the old containers and networks before creating anything new.
		if lc.IsRetry {
			err := b.ResourcesRemove([]string{lc.Name})
			if err != nil {
				eventlog.LogServiceEvent2(b.db, persistence.SEVERITY_WARN,
					persistence.NewMessageMeta(EL_CONT_CLEAN_OLD_CONTAINER_ERROR, lc.Name, err.Error()),
					persistence.EC_REMOVE_OLD_DEPENDENT_SERVICE_FAILED,
					lc.Name, lc.Name, "", "", "", []string{})
				glog.Errorf("Error cleaning up old containers before starting up new containers for %v. Error: %v", lc.Name, err)
			}
		}

		// Locate dependency containers (if there are any) so that this new container will be added to their docker network.
		var ms_children_networks map[string]string

		if len(lc.Microservices) != 0 {
			if ms_containers, err := b.findDependencyContainersForService(lc.GetServicePathElement(), lc.AgreementIds, lc.Microservices); err != nil {
				glog.Errorf("Error checking service containers: %v", err)

				// Requeue the command
				b.AddDeferredCommand(cmd)
				return true
			} else {

				// Now that we have a list of containers on which this service is dependent, we need to get a list of network ids
				// for the dependencies so that all of this service's containers can be added to the dependency networks.
				ms_children_networks = b.GatherAndCreateDependencyNetworks(ms_containers, lc.Name)
			}
		}

		// We support capabilities in the deployment string that not all container deployments should be able
		// to exploit, e.g. file system mapping from host to container. This check ensures that infrastructure
		// containers dont try to do something unsupported.
		deploymentDesc := new(containermessage.DeploymentDescription)
		if err := json.Unmarshal([]byte(lc.Configure.Deployment), &deploymentDesc); err != nil {
			eventlog.LogServiceEvent2(b.db, persistence.SEVERITY_ERROR,
				persistence.NewMessageMeta(EL_CONT_ERROR_UNMARSHAL_DEPLOY, lc.Configure.Deployment, err.Error()),
				persistence.EC_ERROR_IN_DEPLOYMENT_CONFIG,
				"", serviceInfo.URL, serviceInfo.Org, serviceInfo.Version, "", lc.AgreementIds)
			glog.Errorf("Error Unmarshalling deployment string %v, error: %v", lc.Configure.Deployment, err)
			b.Messages() <- events.NewContainerMessage(events.EXECUTION_FAILED, *cmd.ContainerLaunchContext, "", "")
			return true
		} else if valid := deploymentDesc.IsValidFor("infrastructure"); !valid {
			eventlog.LogServiceEvent2(b.db, persistence.SEVERITY_ERROR,
				persistence.NewMessageMeta(EL_CONT_DEPLOYCONF_UNSUPPORT_CAP_FOR_CONT, lc.Configure.Deployment),
				persistence.EC_ERROR_IN_DEPLOYMENT_CONFIG,
				"", serviceInfo.URL, serviceInfo.Org, serviceInfo.Version, "", lc.AgreementIds)
			glog.Errorf("Deployment config %v contains unsupported capability for infrastructure container.", lc.Configure.Deployment)
			b.Messages() <- events.NewContainerMessage(events.EXECUTION_FAILED, *cmd.ContainerLaunchContext, "", "")
			return true
		}

		serviceNames := deploymentDesc.ServiceNames()

		for serviceName, service := range deploymentDesc.Services {

			if !service.Privileged {
				glog.V(5).Infof("Checking bind permissions for service %v", serviceName)
				if err := hasValidBindPermissions(service.Binds); err != nil {
					eventlog.LogServiceEvent2(b.db, persistence.SEVERITY_ERROR,
						persistence.NewMessageMeta(EL_CONT_DEPLOYCONF_UNSUPPORT_BIND_FOR, lc.Configure.Deployment, serviceName, err.Error()),
						persistence.EC_ERROR_IN_DEPLOYMENT_CONFIG,
						"", serviceInfo.URL, serviceInfo.Org, serviceInfo.Version, "", lc.AgreementIds)
					glog.Errorf("Deployment config for service %v contains unsupported bind, %v", serviceName, err)
					b.Messages() <- events.NewContainerMessage(events.EXECUTION_FAILED, *cmd.ContainerLaunchContext, "", "")
					return true
				}
			}

			if lc.Blockchain.Name != "" {
				// Dynamically add in a filesystem mapping so that the infrastructure container can write files that will
				// be saveable or observable to the host system. Also turn on the privileged flag for this container.
				dir := ""
				// NON_HZN_VAR_BASE is used for testing purposes only
				if altDir := os.Getenv("NON_HZN_VAR_BASE"); len(altDir) != 0 {
					dir = altDir + ":/root"
				} else {
					snap_common := os.Getenv("HZN_VAR_BASE")
					if len(snap_common) == 0 {
						snap_common = config.HZN_VAR_BASE_DEFAULT
					}
					dir = path.Join(snap_common) + ":/root"
				}
				deploymentDesc.Services[serviceName].AddFilesystemBinding(dir)
				if !deploymentDesc.Services[serviceName].HasSpecificPortBinding() { // Add compatibility config - assume eth container
					deploymentDesc.Services[serviceName].AddSpecificPortBinding(docker.PortBinding{HostIP: "127.0.0.1", HostPort: "8545"})
				}
				deploymentDesc.Services[serviceName].Privileged = true
			} else { // microservice case
				// Dynamically add in a filesystem mapping so that the workload container has a RO filesystem.
				dir := ""
				if deploymentDesc.ServicePattern.IsShared("singleton", serviceName) {
					dir, _ = b.workloadStorageDir(fmt.Sprintf("%v-%v-%v", "singleton", serviceName, service.VariationLabel))
				} else {
					dir, _ = b.workloadStorageDir(lc.Name)
				}
				deploymentDesc.Services[serviceName].AddFilesystemBinding(fmt.Sprintf("%v:%v:rw", dir, "/service_config"))
			}
		}

		// Indicate that this deployment description is part of the infrastructure
		deploymentDesc.Infrastructure = true

		// Each service has an identity that is based on its service defintion URL and Org. This identity is what we can use to
		// authenticate a service to an API that is hosted by Anax.
		serviceIdentity := cutil.FormOrgSpecUrl(cutil.NormalizeURL(serviceInfo.URL), serviceInfo.Org)
		sVer := serviceInfo.Version

		// Get the container started
		if deployment, err := b.ResourcesCreate(lc.Name, "", deploymentDesc, []byte(""), *lc.EnvironmentAdditions, ms_children_networks, serviceIdentity, sVer); err != nil {
			log_str := EL_CONT_START_CONTAINER_ERROR_FOR_AG
			if lc.IsRetry {
				log_str = EL_CONT_RESTART_CONTAINER_ERROR_FOR_AG
			}
			eventlog.LogServiceEvent2(b.db, persistence.SEVERITY_ERROR,
				persistence.NewMessageMeta(log_str, fmt.Sprintf("%v", lc.AgreementIds), err.Error()),
				persistence.EC_ERROR_START_CONTAINER, "",
				serviceInfo.URL, serviceInfo.Org, serviceInfo.Version, "", lc.AgreementIds)
			glog.Errorf("Error starting containers: %v", err)
			b.Messages() <- events.NewContainerMessage(events.EXECUTION_FAILED, *cmd.ContainerLaunchContext, "", "")

		} else {

			// Restarting a failed dependency service. Restore the network connection with the parents of this service.
			if lc.IsRetry {
				glog.V(5).Infof("Retrying process restoring the network connection with the parents for service %v.", lc.Name)
				if ms_parents_containers, err := b.findParentContainersForService(lc.Name); err != nil {
					eventlog.LogServiceEvent2(b.db, persistence.SEVERITY_ERROR,
						persistence.NewMessageMeta(EL_CONT_FAIL_GET_PAENT_CONT_FOR_SVC, lc.Name, err.Error()),
						persistence.EC_DEPENDENT_SERVICE_RETRY_FAILED, "",
						serviceInfo.URL, "", serviceInfo.Version, "", lc.AgreementIds)
					glog.Errorf("Failed to get a list of parent containers for service retry for %v. %v", lc.Name, err)

				} else if err := b.restoreDependencyServiceNetworks(lc.Name, &ms_parents_containers); err != nil {
					eventlog.LogServiceEvent2(b.db, persistence.SEVERITY_ERROR,
						persistence.NewMessageMeta(EL_CONT_FAIL_RESTORE_NW_WITH_PARENT, lc.Name, err.Error()),
						persistence.EC_DEPENDENT_SERVICE_RETRY_FAILED, "",
						serviceInfo.URL, "", serviceInfo.Version, "", lc.AgreementIds)
					glog.Errorf("Failed to restore the network connection with the parents for service %v. %v", lc.Name, err)

				}
			}

			glog.V(1).Infof("Success starting container pattern for serviceNames: %v", deployment.ToString())

			// perhaps add the tc info to the container message so it can be enforced
			if ov := os.Getenv("CMTN_SERVICEOVERRIDE"); ov != "" {
				b.Messages() <- events.NewContainerMessage(events.EXECUTION_BEGUN, *cmd.ContainerLaunchContext, serviceNames[0], deploymentDesc.Services[serviceNames[0]].GetSpecificContainerPortBinding())
			} else {
				b.Messages() <- events.NewContainerMessage(events.EXECUTION_BEGUN, *cmd.ContainerLaunchContext, deploymentDesc.Services[serviceNames[0]].GetSpecificHostBinding(), deploymentDesc.Services[serviceNames[0]].GetSpecificHostPortBinding())
			}
		}

	case *ContainerMaintenanceCommand:
		cmd := command.(*ContainerMaintenanceCommand)
		glog.V(3).Infof("ContainerWorker received maintenance command: %v", cmd.ShortString())

		cMatches := make([]docker.APIContainers, 0)

		if cmd.Deployment.IsNative() {

			nd := cmd.Deployment.(*persistence.NativeDeploymentConfig)
			serviceNames := persistence.ServiceConfigNames(&nd.Services)

			report := func(container *docker.APIContainers, agreementId string) error {

				for _, name := range serviceNames {
					if container.Labels[LABEL_PREFIX+".service_name"] == name && container.State == "running" {
						cMatches = append(cMatches, *container)
						glog.V(4).Infof("Matching container instance for agreement %v: %v", agreementId, container)
					}
				}
				return nil
			}

			b.ContainersMatchingAgreement([]string{cmd.AgreementId}, true, report)

			if len(serviceNames) == len(cMatches) {
				glog.V(3).Infof("Found expected count of running containers for agreement %v: %v", cmd.AgreementId, len(cMatches))
			} else {
				glog.Errorf("Insufficient running containers found for agreement %v. Found: %v", cmd.AgreementId, cMatches)

				// ask governer to cancel the agreement
				b.Messages() <- events.NewWorkloadMessage(events.EXECUTION_FAILED, cmd.AgreementProtocol, cmd.AgreementId, cmd.Deployment)
			}
		}

	case *WorkloadShutdownCommand:
		cmd := command.(*WorkloadShutdownCommand)

		// The container worker might not be the right handler for this event, if the deployment is handled by some other worker.
		if cmd.Deployment != nil && !cmd.Deployment.IsNative() {
			glog.V(5).Infof("ContainerWorker ignoring shutdown command for agreement id %v: %v", cmd.CurrentAgreementId, cmd)
			return true
		}

		// This agreement should be handled by the container worker.
		agreements := cmd.Agreements
		if cmd.CurrentAgreementId != "" {
			glog.Infof("ContainerWorker received shutdown command w/ current agreement id: %v. Shutting down resources", cmd.CurrentAgreementId)
			glog.V(5).Infof("Shutdown command for agreement id %v: %v", cmd.CurrentAgreementId, cmd)
			agreements = append(agreements, cmd.CurrentAgreementId)
		}

		if err := b.ResourcesRemove(agreements); err != nil {
			glog.Errorf("Error removing resources: %v", err)
		}

		// send the event to let others know that the workload clean up has been processed
		b.Messages() <- events.NewWorkloadMessage(events.WORKLOAD_DESTROYED, cmd.AgreementProtocol, cmd.CurrentAgreementId, nil)

	case *ContainerStopCommand:
		cmd := command.(*ContainerStopCommand)

		glog.V(3).Infof("ContainerWorker received infrastructure container stop command: %v", cmd.ShortString())
		if err := b.ResourcesRemove([]string{cmd.Msg.ContainerName}); err != nil {
			glog.Errorf("Error removing resources: %v", err)
		}

		// send the event to let others know that the workload clean up has been processed
		b.Messages() <- events.NewContainerShutdownMessage(events.CONTAINER_DESTROYED, cmd.Msg.ContainerName, cmd.Msg.Org)

	case *MaintainMicroserviceCommand:
		cmd := command.(*MaintainMicroserviceCommand)
		glog.V(3).Infof("ContainerWorker received service maintenance command: %v", cmd.ShortString())

		cMatches := make([]docker.APIContainers, 0)

		if msinst, err := persistence.FindMicroserviceInstanceWithKey(b.db, cmd.MsInstKey); err != nil {
			glog.Errorf("Error retrieving service instance from database for %v, error: %v", cmd.MsInstKey, err)
		} else if msinst == nil {
			glog.Errorf("Cannot find service instance record from database for %v.", cmd.MsInstKey)
		} else if serviceNames, err := b.findMicroserviceDefContainerNames(msinst.SpecRef, msinst.Org, msinst.Version, msinst.MicroserviceDefId); err != nil {
			glog.Errorf("Error retrieving service contianers for %v, error: %v", cmd.MsInstKey, err)
		} else if serviceNames != nil && len(serviceNames) > 0 {

			report := func(container *docker.APIContainers, instance_key string) error {

				for _, name := range serviceNames {
					if container.Labels[LABEL_PREFIX+".service_name"] == name {
						if container.State != "running" {
							glog.Errorf("Service container for %v is not in the running state.", instance_key)
						} else {
							cMatches = append(cMatches, *container)
							glog.V(4).Infof("Matching container instance for service instance %v: %v", instance_key, container)
						}
					}
				}
				return nil
			}

			b.ContainersMatchingAgreement([]string{cmd.MsInstKey}, true, report)

			if len(serviceNames) == len(cMatches) {
				glog.V(3).Infof("Found expected count of running containers for service instance %v: %v", cmd.MsInstKey, len(cMatches))
			} else {
				glog.Errorf("Insufficient running containers found for service instance %v. Found: %v", cmd.MsInstKey, cMatches)

				// ask governer to record it into the db
				cc := events.NewContainerConfig("", "", "", "", "", "", nil)
				ll := events.NewContainerLaunchContext(cc, nil, events.BlockchainConfig{}, cmd.MsInstKey, []string{}, []events.MicroserviceSpec{}, []persistence.ServiceInstancePathElement{}, false)
				b.Messages() <- events.NewContainerMessage(events.EXECUTION_FAILED, *ll, "", "")
			}
		}
	case *ShutdownMicroserviceCommand:
		cmd := command.(*ShutdownMicroserviceCommand)

		agreements := make([]string, 0)
		if cmd.MsInstKey != "" {
			glog.Infof("ContainerWorker received shutdown command for service %v. Shutting down resources", cmd.MsInstKey)
			agreements = append(agreements, cmd.MsInstKey)
		}

		if err := b.ResourcesRemove(agreements); err != nil {
			glog.Errorf("Error removing resources: %v", err)
		}

		// send the event to let others know that the microservice clean up has been processed
		b.Messages() <- events.NewMicroserviceContainersDestroyedMessage(events.CONTAINER_DESTROYED, cmd.MsInstKey)

	case *CancelMicroserviceNetworkCommand:
		cmd := command.(*CancelMicroserviceNetworkCommand)

		if cmd.MsInstKey != "" {
			glog.Infof("ContainerWorker received cancel network command for service %v. Cancelling extra networks.", cmd.MsInstKey)
		}

		// Get a list of all the networks that currently exist.
		networks, err := b.client.ListNetworks()
		if err != nil {
			glog.Errorf("Unable to list networks: %v", err)
			return true
		}

		// Look for networks related to the microservice that is no longer being used, so that we can figure out if they need to be terminated.
		for _, net := range networks {
			glog.V(3).Infof("ContainerWorker working on network %v", net.Name)
			if _, anaxNet := net.Labels[LABEL_PREFIX+".network"]; !anaxNet {
				continue
			} else if strings.HasPrefix(net.Name, cmd.MsInstKey) {
				if netInfo, err := b.client.NetworkInfo(net.ID); err != nil {
					glog.Errorf("Failure getting network info for %v. Error: %v", net.Name, err)
				} else {
					glog.V(5).Infof("ContainerWorker network %v has %v containers: %v", netInfo.Name, len(netInfo.Containers), netInfo.Containers)

					// Verify that this network contains only the containers from the dependency service. All container names will be in this form:
					// <dependency-microservice-instance-key>_<service-name-from-deployment-config>
					// The event being handled here contains the microservice instance key for a dependency service that is not needed by at least one
					// of it's parents. Therefore, if there are any containers connected to this network that don't have the microservice instance key
					// prefix then this network is still in use for a parent service that has not yet terminated, so the network cannot be removed.
					// This situation only occurs for singleton dependency services, where one of the parent agreements is terminating but at least
					// one parent is not terminating.

					networkStillInUse := false
					for _, container := range netInfo.Containers {
						if !strings.HasPrefix(container.Name, cmd.MsInstKey) {
							networkStillInUse = true
							glog.V(5).Infof("ContainerWorker network %v is still in use by: %v", netInfo.Name, container.Name)
							break
						}
					}

					if networkStillInUse {
						continue
					}

					// Disconnect every container from the network before removing the network.
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
							glog.V(3).Infof("Succeeded disconnecting network: %v from container %v", netInfo.Name, container.Name)
						}
					}

					// Remove the network because it's not needed any more.
					err := b.client.RemoveNetwork(net.ID)
					if err != nil {
						glog.Errorf("Unable to remove network %v for %v, error %v", net.Name, cmd.MsInstKey, err)
					} else {
						glog.V(3).Infof("ContainerWorker removed network %v for %v", net.Name, cmd.MsInstKey)
					}

				}
			}
		}

	case *NodeUnconfigCommand:
		if err := b.GetAuthenticationManager().RemoveAll(); err != nil {
			glog.Errorf("Error handling node unconfig command: %v", err)
		}
		b.Commands <- worker.NewTerminateCommand("shutdown")

	default:
		return false
	}
	return true

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

	if b.Config.Edge.DockerEndpoint == "" {
		glog.V(3).Infof("ContainerWorker: skip syncupResources. Docker client could be be initialized because DockerEndpoint is not set in the configuration.")
		return
	}

	// For multiple anax instances case, we do not want to remove containers that
	// belong to other anax instances. But we need to remove the docker volumes
	// that were created by current instance and no longer used by itself and other
	// instances.
	if b.Config.Edge.MultipleAnaxInstances {
		// remove the leftover docker volumes created by this instance of anax
		// when the node is not registered.
		if b.GetExchangeToken() == "" {
			if err := DeleteLeftoverDockerVolumes(b.db, b.Config); err != nil {
				glog.Errorf("Container worker sync resources. %v", err)
			}
		}
		glog.V(3).Infof("ContainerWorker: multiple anax instances enabled. will not cleanup left over containers.")
		b.Messages() <- events.NewDeviceContainersSyncedMessage(events.DEVICE_CONTAINERS_SYNCED, true)
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
	if agreements, err := persistence.FindEstablishedAgreementsAllProtocols(b.db, policy.AllAgreementProtocols(), []persistence.EAFilter{persistence.UnarchivedEAFilter()}); err != nil {
		fail(fmt.Sprintf("ContainerWorker unable to retrieve agreements from database, error %v", err))
	} else {

		// Create quick access map of agreement ids. This will allow us to avoid nested loops.
		agMap := make(map[string]bool)
		for _, agreement := range agreements {
			agMap[agreement.CurrentAgreementId] = true
		}

		glog.V(5).Infof("Container worker found active agreements: %v", agMap)

		// Second, run through each container (active or inactive) looking for containers that are leftover from old agreements. Be aware that there
		// could be other non-Horizon containers on this host, so we have to be careful to NOT terminate them.
		if containers, err := b.client.ListContainers(docker.ListContainersOptions{All: true}); err != nil {
			fail(fmt.Sprintf("ContainerWorker unable to get list of containers: %v", err))
		} else {

			// Look for orphaned containers.
			for _, container := range containers {
				glog.V(5).Infof("ContainerWorker working on container %v", container)
				if val, exists := container.Labels[LABEL_PREFIX+".dev_service"]; exists && val == "true" {
					//skip dev containers
					glog.V(4).Infof("Skipping non-leftover dev container: %v", container.ID)
					continue
				}
				// Containers that are part of our horizon infrastructure or are shared or without an agreement id label will be ignored.
				if _, infraLabel := container.Labels[LABEL_PREFIX+".infrastructure"]; infraLabel {
					continue
				} else if _, sharedThere := container.Labels[LABEL_PREFIX+".service_pattern.shared"]; sharedThere {
					continue
				} else if _, labelThere := container.Labels[LABEL_PREFIX+".agreement_id"]; !labelThere {
					continue
				} else if _, there := agMap[container.Labels[LABEL_PREFIX+".agreement_id"]]; !there {
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
				if _, anaxNet := net.Labels[LABEL_PREFIX+".network"]; !anaxNet {
					continue
				} else if val, exists := net.Labels[LABEL_PREFIX+".dev_network"]; exists && val == "true" {
					//skip dev networks
					continue
				} else if val, exists := net.Labels[LABEL_PREFIX+".service_pattern.shared"]; exists && val == "singleton" {

					if netInfo, err := b.client.NetworkInfo(net.ID); err != nil {
						glog.Errorf("Failure getting network info for %v. Error: %v", net.Name, err)
					} else if len(netInfo.Containers) != 0 {
						glog.V(3).Infof("Shared network %v has containers %v, so leave it alone", net.Name, netInfo.Containers)
					} else if err := b.client.RemoveNetwork(net.ID); err != nil {
						glog.Errorf("Failure removing network: %v. Error: %v", net, err)
					} else {
						glog.Infof("Succeeded removing unused shared network: %v", net)
					}
				} else if !IsAgreementId(net.Name) {
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
			if err := b.ResourcesRemove(agreementList); err != nil {
				fail(fmt.Sprintf("ContainerWorker unable to get rid of left over resources, error: %v", err))
			}
		}

	}
	// remove the leftover docker volumes created by anax
	if b.GetExchangeToken() == "" {
		if err := DeleteLeftoverDockerVolumes(b.db, b.Config); err != nil {
			glog.Errorf("Container worker sync resources. %v", err)
		}
	}

	glog.V(3).Infof("ContainerWorker done syncing docker resources, successful: %v.", outcome)
	// Finally issue an event to tell everyone else that we are done with the sync up, and the final status of it.
	b.Messages() <- events.NewDeviceContainersSyncedMessage(events.DEVICE_CONTAINERS_SYNCED, outcome)
}

// Given a list of containers on which a parent service is dependent, we need to get a list of dependency service network ids
// so that they can be added to all of this (parent) service's containers. The dependency containers can be in more than 1 network so
// we have to carefully choose the networks that the parent container should connect to. Only choose the network
// on which the dependent service is providing the service. This will be the network that has a prefix with the same name as
// the agreement_id label on the service container. For dependency service's that have more than 1 parent, also make sure that
// any missing networks are created and connected to the dependency service's containers. Dependency containers dont know their specific
// parent containers because parents are started after the dependencies.
func (b *ContainerWorker) GatherAndCreateDependencyNetworks(dependencyContainers []docker.APIContainers, parentName string) map[string]string {

	glog.V(5).Infof("Dependency containers for %v are: %v", parentName, dependencyContainers)
	ms_children_networks := make(map[string]string)

	if dependencyContainers == nil || len(dependencyContainers) == 0 {
		return ms_children_networks
	}

	var dependencyBaseNetworkName string
	var nw docker.ContainerNetwork
	var ok bool

	for _, msc := range dependencyContainers {

		dependencyBaseNetworkName, ok = msc.Labels[LABEL_PREFIX+".agreement_id"]
		if !ok {
			continue
		}

		// Search for the network to which the parent service container should be connected. Parent containers are connected
		// to a dependency specific network, used solely by the parent and a dependency. Therefore, the name of the
		// dependency's network includes both the parent and the dependency.
		nwForParentSvc := dependencyBaseNetworkName + "_" + parentName

		// If this dependency has a network that is specific to this parent, then save the network ID so that the parent service will
		// be connected to the dependency's network when the parent containers are started.
		nw, ok = msc.Networks.Networks[nwForParentSvc]
		if ok {
			ms_children_networks[nwForParentSvc] = nw.NetworkID
			glog.V(3).Infof("Found network %v for dependency %v of service %v, network: %v", nwForParentSvc, dependencyBaseNetworkName, parentName, nw)
			continue
		}

		glog.V(3).Infof("Dependency service's parent specific network (%s) has not been found for the service, creating and connecting it.", nwForParentSvc)

		// Create workload specific network for this dependency and connect it to the dependency. The workload container will be
		// connected to this network when the workload containers are created.

		var parentSpecificNetwork *docker.Network
		if nws, err := b.client.FilteredListNetworks(docker.NetworkFilterOpts{"name": {nwForParentSvc: true}}); err != nil {
			glog.Errorf("failure listing network %v, error %v", nwForParentSvc, err)
			continue
		} else if len(nws) == 0 {
			if newNetwork, err := MakeBridge(b.client, nwForParentSvc, true, false, b.isDevInstance); err != nil {
				glog.Errorf("Could not create parent specific network %v for service: %v", nwForParentSvc, err)
				continue
			} else {
				parentSpecificNetwork = newNetwork
			}
		} else {
			parentSpecificNetwork = &nws[0]
		}

		childSvcName := ""
		childSvcName, ok = msc.Labels[LABEL_PREFIX+".service_name"]
		if !ok {
			glog.Errorf("Could not find service name of dependency's container %v", msc.Names)
			continue
		}

		if err := b.ConnectContainerToNetwork(parentSpecificNetwork, msc.ID, childSvcName); err != nil {
			glog.Errorf("Could not connect dependency service %v to the new network, error %v", msc.Names, err)
		} else {
			glog.V(3).Infof("Created dependent service's network %v and connected container %v to the network.", nwForParentSvc, msc.ID)
			ms_children_networks[nwForParentSvc] = parentSpecificNetwork.ID

			// When a dependency is created, a network is created for it. The network name matches the dependency's microservice instance name,
			// which is the same as the agreement_id label on each of the dependency's containers. If this original network still exists, then
			// disconnect the container from it so that the original network can be deleted.
			originalBaseNetwork, ok := msc.Networks.Networks[dependencyBaseNetworkName]
			if !ok {
				continue
			}

			glog.V(5).Infof("Disconnecting network %v from container %v.", dependencyBaseNetworkName, msc.Names)
			err := b.client.DisconnectNetwork(originalBaseNetwork.NetworkID, docker.NetworkConnectionOptions{
				Container:      msc.ID,
				EndpointConfig: nil,
				Force:          true,
			})
			if err != nil {
				glog.Errorf("Failure disconnecting network: %v from container %v. Error: %v", dependencyBaseNetworkName, msc.Names, err)
			} else {
				glog.V(3).Infof("Succeeded disconnecting network: %v from container %v", dependencyBaseNetworkName, msc.Names)
			}

			// Check the dependency's original base network. If it has no containers connected to it any more, remove it.
			if netInfo, err := b.client.NetworkInfo(originalBaseNetwork.NetworkID); err != nil {
				glog.Errorf("Failure getting network info for %v. Error: %v", dependencyBaseNetworkName, err)
			} else if len(netInfo.Containers) == 0 {

				// Remove the network because it's not needed any more.
				if err := b.client.RemoveNetwork(netInfo.ID); err != nil {
					glog.Errorf("Unable to remove network %v, error %v", netInfo.Name, err)
				} else {
					glog.V(3).Infof("ContainerWorker removed network %v", netInfo.Name)
				}

			}

		}
	}

	return ms_children_networks
}

// When a service fails and is restarted, the networks that connect it to its parents and dependencies have to be recreated, and
// all the containers have to be re-connected.
func (b *ContainerWorker) restoreDependencyServiceNetworks(networkName string, parentContainers *[]docker.APIContainers) error {

	// Get the list of containers in the service's default network, so that we can move them to parent specific networks.
	var originalNetwork *docker.Network
	var serviceContainers []docker.APIContainers
	filter := docker.NetworkFilterOpts{"name": {networkName: true}}
	if nws, err := b.client.FilteredListNetworks(filter); err != nil {
		return fmt.Errorf("failure getting original network %v, error %v", networkName, err)
	} else if len(nws) == 0 {
		return fmt.Errorf("original network not found")
	} else if len(nws) != 1 {
		return fmt.Errorf("expected 1 network, received: %v", nws)
	} else if netInfo, err := b.client.NetworkInfo(nws[0].ID); err != nil {
		return fmt.Errorf("failure getting network info for %v, error %v", networkName, err)
	} else {
		originalNetwork = netInfo

		var err error
		serviceContainers, err = b.client.ListContainers(docker.ListContainersOptions{Filters: map[string][]string{"network": []string{networkName}}})
		if err != nil {
			return fmt.Errorf("unable to get list of containers in network %v, error %v", networkName, err)
		}
	}

	// For each parent container, make sure there is a network for it and the restarted dependency, and connect them both to it.
	for _, parentContainer := range *parentContainers {

		// Grab the agreement id label and make a network for this container and it's parent.
		parentName, ok := parentContainer.Labels[LABEL_PREFIX+".agreement_id"]
		if !ok {
			return fmt.Errorf("could not find agreement_id on parent container %v", parentContainer.Names)
		}

		// Parent containers are connected to a dependency specific network, used solely by the parent and a dependency. Therefore,
		// the name of the dependency's network includes both the parent and the dependency.
		nwForParentSvc := networkName + "_" + parentName

		// Create parent specific network for this service and connect it to the dependency.
		var parentSpecificNetwork *docker.Network
		if nws, err := b.client.FilteredListNetworks(docker.NetworkFilterOpts{"name": {nwForParentSvc: true}}); err != nil {
			return fmt.Errorf("failure listing network %v, error %v", nwForParentSvc, err)
		} else if len(nws) == 0 {
			if newNetwork, err := MakeBridge(b.client, nwForParentSvc, true, false, b.isDevInstance); err != nil {
				return fmt.Errorf("Could not create parent specific network %v for service: %v", nwForParentSvc, err)
			} else {
				parentSpecificNetwork = newNetwork
			}
		} else {
			parentSpecificNetwork = &nws[0]
		}

		// Create parent specific network for this dependency and connect it to the dependency and the parent.
		if parentSvcName, ok := parentContainer.Labels[LABEL_PREFIX+".service_name"]; !ok {
			return fmt.Errorf("Could not find service name of dependency's container %v", parentContainer.Names)
		} else if err := b.ConnectContainerToNetwork(parentSpecificNetwork, parentContainer.ID, parentSvcName); err != nil {
			return fmt.Errorf("Could not connect parent service %v to the new network, error %v", parentContainer.Names, err)
		} else if err := b.connectContainers(parentSpecificNetwork, serviceContainers); err != nil {
			return fmt.Errorf("Could not connect dependency service %v containers to the new network, error %v", networkName, err)
		} else {
			glog.V(3).Infof("ContainerWorker Created restarted service's network %v and connected parent container %v and dependency containers %v to new network %v.", nwForParentSvc, parentContainer.ID, serviceContainers, parentSpecificNetwork.Name)
		}
	}

	// The parent containers are re-connected to the restarted dependency, now we can remove the dependency's default network.
	for _, container := range serviceContainers {
		glog.V(5).Infof("ContainerWorker Disconnecting network (%v) %v from container (%v) %v.", originalNetwork.ID, networkName, container.ID, container.Names)
		err := b.client.DisconnectNetwork(originalNetwork.ID, docker.NetworkConnectionOptions{
			Container:      container.ID,
			EndpointConfig: nil,
			Force:          true,
		})
		if err != nil {
			return fmt.Errorf("failure disconnecting network: (%v) %v from container (%v) %v, error %v", originalNetwork.ID, networkName, container.ID, container.Names, err)
		} else {
			glog.V(3).Infof("ContainerWorker Succeeded disconnecting network: %v from container %v", networkName, container.Names)
		}
	}

	// Remove the network because it's not needed any more.
	if err := b.client.RemoveNetwork(originalNetwork.ID); err != nil {
		return fmt.Errorf("unable to remove network %v, error %v", networkName, err)
	}

	glog.V(3).Infof("ContainerWorker removed network %v, no longer needed.", networkName)

	return nil
}

func (b *ContainerWorker) ResourcesRemove(agreements []string) error {
	glog.V(5).Infof("Killing and removing resources in agreements: %v", agreements)

	// Remove networks
	networks, err := b.client.ListNetworks()
	if err != nil {
		return fmt.Errorf("Unable to list networks: %v", err)
	}
	glog.V(5).Infof("Existing networks: %v", networks)

	freeNets := make([]docker.Network, 0)
	destroy := func(container *docker.APIContainers, agreementId string) error {
		if !serviceAndWorkerTypeMatches(b.isDevInstance, container) {
			// skip dev containers is it's non-dev instance and vice versa
			glog.V(4).Infof("Skipping dev container: %v", container.ID)
			return nil
		}
		if val, exists := container.Labels[LABEL_PREFIX+".service_pattern.shared"]; exists && val == "singleton" {
			// must investigate bridge to see if other containers are still using this shared service

			glog.V(4).Infof("Found shared container with names: %v and networks: %v", container.Names, container.Networks.Networks)

			var sharedNet docker.Network
			for netName, _ := range container.Networks.Networks {
				for _, conName := range container.Names {
					if strings.TrimLeft(conName, "/") == netName {
						// this shared container should only have one network, its own and it should be the same as the name of the container (minus the prefix, of course)
						for _, network := range networks {
							// want a handle to a real network from our earlier-fetched list, not just the Container stub
							if isAnaxNetwork(&network, netName) {
								sharedNet = network
							}
						}
					}
				}
			}

			if sharedNet.ID == "" {
				glog.Warningf("Did not find existing network for shared container: %v", container)
			} else if netInfo, err := b.client.NetworkInfo(sharedNet.ID); err != nil {
				glog.Errorf("Failure getting network info for %v. Error: %v", sharedNet, err)
			} else {

				glog.V(3).Infof("Shared container network %v has container ids: %v", netInfo.Name, netInfo.Containers)

				for conId, _ := range netInfo.Containers {
					// do container lookup

					allContainers, err := b.client.ListContainers(docker.ListContainersOptions{})
					if err != nil {
						return err
					}

					glog.V(5).Infof("All containers: %v", allContainers)

					// Look through each container in the system to find one that is on the shared container's network and
					// that has a different agreement id than the agreement we're terminating. If we find one, then we cant
					// get rid of the shared container.
					for _, con := range allContainers {
						// The shared container we are trying to get rid of is one of the containers in the system, so we can just
						// skip over it.
						if val, exists := con.Labels[LABEL_PREFIX+".service_pattern.shared"]; exists && val == "singleton" {
							continue
						}

						// If the current container is in the list of containers on the shared container's network, then we should check
						// the agreement id. If it's different, the shared container will remain in use after we terminate all the containers
						// in this agreement, so we need to leave the shared container up.
						if conId == con.ID &&
							con.Labels[LABEL_PREFIX+".agreement_id"] != agreementId {

							glog.V(2).Infof("Will not free resources for shared container: %v, it's in use by %v", container.Names, con)

							return nil
						}
					}
				}

				// if we made it this far, we can free this network
				freeNets = append(freeNets, sharedNet)
			}
		}

		serviceName := container.Labels[LABEL_PREFIX+".service_name"]
		// if we made it this far, we're hosing the container
		if destroyed, err := serviceDestroy(b.client, agreementId, container.ID); err != nil {
			glog.Errorf("Service %v in agreement %v could not be removed. Error: %v", serviceName, agreementId, err)
		} else if destroyed {
			glog.V(1).Infof("Service %v in agreement %v stopped and removed", serviceName, agreementId)
		} else {
			glog.V(5).Infof("Service %v in agreement %v already removed", serviceName, agreementId)
		}

		return nil
	}

	err = b.ContainersMatchingAgreement(agreements, true, destroy)
	if err != nil {
		glog.Errorf("Error removing containers for %v. Error: %v", agreements, err)
	}

	// Remove the pieces of the host file system that are no longer needed.
	for _, agreementId := range agreements {
		// Remove workspaceROStorage directory and docker volume.
		workloadRWStorageDir, useVolume := b.workloadStorageDir(agreementId)
		if !useVolume {
			if err := os.RemoveAll(workloadRWStorageDir); err != nil {
				glog.Errorf("Failed to remove workloadStorageDir: %v. Error: %v", workloadRWStorageDir, err)
			}
		} else {
			// remove the docker volume
			if err := b.client.RemoveVolume(workloadRWStorageDir); err != nil {
				if err != docker.ErrNoSuchVolume {
					glog.Errorf("Failed to remove workloadStorageDir docker volume: %v. Error: %v", workloadRWStorageDir, err)
				}
			}
		}

		// Remove the File Sync Service API authentication credential file.
		if err := b.GetAuthenticationManager().RemoveCredential(agreementId); err != nil {
			glog.Errorf("Failed to remove FSS Authentication credential file for %v, error %v", agreementId, err)
		}

	}

	// gather agreement networks to free
	for _, net := range networks {
		for _, agreementId := range agreements {

			if strings.HasPrefix(net.Name, agreementId) || strings.Contains(net.Name, agreementId) {
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
							glog.V(3).Infof("Succeeded disconnecting network: %v from container %v", netInfo.Name, container.Name)
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
			glog.V(1).Infof("Succeeded removing unused network: %v", net.ID)
		}
	}

	// the primary rule
	if b.iptables != nil {
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
	}

	return nil
}

func (b *ContainerWorker) ContainersMatchingAgreement(agreements []string, includeShared bool, fn func(*docker.APIContainers, string) error) error {
	var processingErr error

	// get all containers including the inactive ones.
	containers, err := b.client.ListContainers(docker.ListContainersOptions{All: true})
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

// find the microservice definition from the db
func (b *ContainerWorker) findMicroserviceDefContainerNames(api_spec string, org string, version string, msdef_key string) ([]string, error) {

	container_names := make([]string, 0)
	var msdef *persistence.MicroserviceDefinition

	if msdef_key != "" {
		if msd, err := persistence.FindMicroserviceDefWithKey(b.db, msdef_key); err != nil {
			return nil, fmt.Errorf("Error finding service definition from the local db for %v version %v key %v. %v", api_spec, version, msdef_key, err)
		} else {
			msdef = msd
		}
	} else {
		if ms_defs, err := persistence.FindMicroserviceDefs(b.db, []persistence.MSFilter{persistence.UnarchivedMSFilter(), persistence.UrlOrgVersionMSFilter(api_spec, org, version)}); err != nil {
			return nil, fmt.Errorf("Error finding service definition from the local db for %v version %v key %v. %v", cutil.FormOrgSpecUrl(api_spec, org), version, msdef_key, err)
		} else if ms_defs != nil && len(ms_defs) > 0 {
			// assume only one microservicedef exists for each service
			msdef = &ms_defs[0]
		}
	}

	if msdef != nil && msdef.HasDeployment() {
		// get the service name from the ms def
		deployment, _ := msdef.GetDeployment()
		deploymentDesc := new(containermessage.DeploymentDescription)
		if err := json.Unmarshal([]byte(deployment), &deploymentDesc); err != nil {
			return nil, fmt.Errorf("Error Unmarshalling deployment string %v for service %v version %v. %v", deployment, cutil.FormOrgSpecUrl(api_spec, org), version, err)
		} else {
			for serviceName, _ := range deploymentDesc.Services {
				container_names = append(container_names, serviceName)
			}
		}
	}
	glog.V(5).Infof("The container names for service %v/%v version %v are: %v", org, api_spec, version, container_names)
	return container_names, nil
}

// This function finds the all the containers of the direct children of the given service.
// It only includes the containers with "running" state.
func (b *ContainerWorker) findDependencyContainersForService(parent *persistence.ServiceInstancePathElement, agreementIds []string, microservices []events.MicroserviceSpec) ([]docker.APIContainers, error) {
	ms_containers := make([]docker.APIContainers, 0)
	if containers, err := b.client.ListContainers(docker.ListContainersOptions{}); err != nil {
		return nil, fmt.Errorf("Unable to get list of running containers: %v", err)
	} else {
		for _, api_spec := range microservices {
			// find the ms from the local db,
			if msc_names, err := b.findMicroserviceDefContainerNames(api_spec.SpecRef, api_spec.Org, api_spec.Version, api_spec.MsdefId); err != nil {
				return nil, fmt.Errorf("Error finding service definition from the local db for %v. %v", api_spec, err)
			} else if msinsts, err := persistence.FindMicroserviceInstances(b.db, []persistence.MIFilter{persistence.AllInstancesMIFilter(api_spec.SpecRef, api_spec.Org, api_spec.Version), persistence.UnarchivedMIFilter()}); err != nil {
				return nil, fmt.Errorf("Error retrieving service instances for %v/%v version %v from database, error: %v", api_spec.Org, api_spec.SpecRef, api_spec.Version, err)
			} else if msinsts == nil || len(msinsts) == 0 {
				return nil, fmt.Errorf("No service instance for service %v yet.", api_spec)
			} else {
				// find the ms instance that has the agreement id in it
				var ms_instance *persistence.MicroserviceInstance
				ms_instance = nil
				for _, msi := range msinsts {
					// If we're starting an agreement-less service then the agreement id will be empty. Dependent services will also
					// be marked as agreement-less so we should try these. The HasDirectParent check further down will prevent us from
					// picking the wrong agreement-less dependent.
					if (agreementIds == nil || len(agreementIds) == 0) && msi.AgreementLess {
						ms_instance = &msi
					} else if msi.AssociatedAgreements != nil && len(msi.AssociatedAgreements) > 0 {
						// Make sure the dependent service is in our agreement, ignore it if not.
						for _, id := range msi.AssociatedAgreements {
							if cutil.SliceContains(agreementIds, id) {
								ms_instance = &msi
								break
							}
						}
					}
					// We found a service instance that is in the current agreement. Now check to make sure
					// that this instance is a dependent of the container we're trying to start.
					if ms_instance != nil && ms_instance.HasDirectParent(parent) {
						break
					} else {
						ms_instance = nil
					}
				}

				if ms_instance == nil {
					return nil, fmt.Errorf("Service instance has not been initiated for service %v yet.", api_spec)
				}

				if msc_names == nil || len(msc_names) == 0 {
					continue
				}

				// get the service name from the ms def
				for _, serviceName := range msc_names {
					// compare with the container name. assume the container name = <msinstkey>-<service name>
					for _, container := range containers {
						if _, ok := container.Labels[LABEL_PREFIX+".infrastructure"]; ok {
							cname := container.Names[0]
							if cname == "/"+ms_instance.GetKey()+"-"+serviceName {
								// check if the container is up and running
								if container.State != "running" {
									return nil, fmt.Errorf("The service container %v is not up and running. %v", serviceName, err)
								} else {
									glog.V(5).Infof("Found running service container %v for service %v", container, api_spec)
									ms_containers = append(ms_containers, container)
								}
							}
						}
					}

				}
			}
		}
	}
	return ms_containers, nil
}

// This function finds the all the containers of the parent services for the given dependency service. It returns only the containers
// in the "running" state. It matches parents with the input dependency by finding microservices with parent service URLs that are
// in the same agreement as the input service.
func (b *ContainerWorker) findParentContainersForService(msinst_key string) ([]docker.APIContainers, error) {
	glog.V(3).Infof("ContainerWorker Get direct parent containers for %v", msinst_key)

	ms_containers := make([]docker.APIContainers, 0)

	msi_child, err := persistence.FindMicroserviceInstanceWithKey(b.db, msinst_key)
	if err != nil {
		return nil, fmt.Errorf("Error finding service instance from db for %v. %v", msinst_key, err)
	} else if msi_child == nil {
		glog.V(3).Infof("ContainerWorker Service %v has no microservice instance.", msinst_key)
		return nil, nil
	}

	parents := msi_child.GetDirectParents()
	if parents == nil || len(parents) == 0 {
		glog.V(3).Infof("No direct parents found for service %v.", msinst_key)
		return nil, nil
	}
	glog.V(3).Infof("ContainerWorker The direct parents for %v are %v", msinst_key, parents)

	// The instances of top level services are stored in the "established_agreements" table, not in the microservice instances database.
	// Here we convert them to MicroserviceInstances for easier processing.
	top_level_msinsts := make([]persistence.MicroserviceInstance, 0)
	if ags, err := persistence.FindEstablishedAgreementsAllProtocols(b.db, policy.AllAgreementProtocols(), []persistence.EAFilter{persistence.UnarchivedEAFilter()}); err != nil {
		return nil, errors.New(fmt.Sprintf("unable to read active agreements from db, error %v", err))
	} else {
		for _, ag := range ags {
			top_level_msinsts = append(top_level_msinsts, *persistence.AgreementToMicroserviceInstance(ag, ""))
		}
	}

	glog.V(5).Infof("ContainerWorker Top level services are converted to microservices: %v", top_level_msinsts)

	// Get all the containers on the system and examine them for matches.
	containers, err := b.client.ListContainers(docker.ListContainersOptions{})
	if err != nil {
		return nil, fmt.Errorf("Unable to get list of running containers: %v", err)
	}

	// Find the corresponding microservice instances for a given parent service by org, url and version.
	for _, api_spec := range parents {

		msinsts, err := persistence.FindMicroserviceInstances(b.db, []persistence.MIFilter{persistence.UnarchivedMIFilter(), persistence.AllInstancesMIFilter(api_spec.URL, api_spec.Org, api_spec.Version)})
		if err != nil {
			return nil, fmt.Errorf("Error retrieving service instances for %v version %v from database, error: %v", api_spec.URL, api_spec.Version, err)
		} else if msinsts == nil {
			msinsts = make([]persistence.MicroserviceInstance, 0)
		}

		// Add top level services to the list of potential parent microservice instances. A container can be belong to a dependent service
		// and top level service at the same time.
		for _, tmsi := range top_level_msinsts {
			if tmsi.SpecRef == api_spec.URL && tmsi.Org == api_spec.Org && tmsi.Version == api_spec.Version {
				msinsts = append(msinsts, tmsi)
			}
		}

		glog.V(5).Infof("ContainerWorker found microservice instances for %v, %v", api_spec, msinsts)

		// Find the parent ms instances that have the same agreement id as the input dependency service. This is done by intersecting the
		// agreement ids for each potential parent microservice with the agreement ids of the input dependency service.
		var ms_instances_to_use = []persistence.MicroserviceInstance{}
		for _, msi := range msinsts {
			if msi.AgreementLess {
				ms_instances_to_use = append(ms_instances_to_use, msi)
			} else if msi_child.AssociatedAgreements != nil && len(msi_child.AssociatedAgreements) > 0 {
				// Make sure the dependent service is in our list of agreements, otherwise ignore it.
				for _, id := range msi_child.AssociatedAgreements {
					if cutil.SliceContains(msi.AssociatedAgreements, id) && msi.CleanupStartTime == 0 {
						ms_instances_to_use = append(ms_instances_to_use, msi)
					}
				}
			}
		}

		// If there are no microservices for this parent URL, then skip it.
		if len(ms_instances_to_use) == 0 {
			glog.V(3).Infof("ContainerWorker Service instance has not been initiated for service %v yet, so there are no containers.", api_spec)
			continue
		}

		glog.V(5).Infof("ContainerWorker Found running service instances %v.", ms_instances_to_use)

		// Now find the containers for the parent microservices that we just found.
		for _, container := range containers {

			if container.State != "running" {
				glog.V(5).Infof("ContainerWorker The service container %v is not up and running, skipping it.", container.Names)
				continue
			}

			for _, msi_to_use := range ms_instances_to_use {
				if strings.Contains(container.Names[0], msi_to_use.InstanceId) {
					ms_containers = append(ms_containers, container)
					glog.V(5).Infof("ContainerWorker Added container (%v) %v to parent container list.", container.ID, container.Names)
					break
				}
			}
		}

	}

	glog.V(3).Infof("Parent containers for %v are %v.", msinst_key, ms_containers)

	return ms_containers, nil
}

// connects the given containers to the given network
func (b *ContainerWorker) connectContainers(network *docker.Network, containers []docker.APIContainers) error {

	if network == nil {
		return fmt.Errorf("input network is nil")
	} else if len(containers) == 0 {
		return fmt.Errorf("input container list is empty")
	}

	for _, container := range containers {
		containerServiceName, ok := container.Labels[LABEL_PREFIX+".service_name"]
		if !ok {
			return fmt.Errorf("container is missing service_name label: %v", container)
		}

		if err := b.ConnectContainerToNetwork(network, container.ID, containerServiceName); err != nil {
			return err
		}
	}
	return nil
}

func isAnaxNetwork(net *docker.Network, bridgeName string) bool {
	if _, anaxNet := net.Labels[LABEL_PREFIX+".network"]; anaxNet && net.Name == bridgeName {
		return true
	}
	return false
}

// Verify that the permission bits for the host side of the binding allow anyone/other
// to access that file or directory.
func hasValidBindPermissions(binds []string) error {
	for _, bind := range binds {
		containerVol := strings.Split(bind, ":")

		info, err := os.Stat(containerVol[0])
		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}

		// Grab the permission bits for the host file/directory. The bits are in an int type so it's
		// possible to do bitwise calculations on it.
		m := info.Mode()
		if len(containerVol) > 2 && containerVol[2] == "ro" {
			// Check the 'other' read permission bit on the host file/directory. If read permission
			// is not enabled then the bind will not be allowed (because the bind is asking for ro permission).
			if m&4 == 0 {
				return errors.New(fmt.Sprintf("Read permission for bind to %v is denied", containerVol[0]))
			}

		} else {
			// Check the 'other' write permission bit on the host file/directory. If write permission
			// is not enabled then the bind will not be allowed (because the bind is asking for rw permission).
			if m&2 == 0 {
				return errors.New(fmt.Sprintf("Write permission for bind to %v is denied", containerVol[0]))
			}
		}
	}
	return nil
}

// get the volumes specified in the service config and create the volumes if they do not exist.
func (b *ContainerWorker) createDockerVolumesForContainer(serviceName string, agreementId string, servicePair *servicePair) error {
	if servicePair == nil || servicePair.serviceConfig == nil {
		return nil
	}

	binds := servicePair.serviceConfig.HostConfig.Binds

	// get all the existing docker volumes
	volumes_docker, err := b.client.ListVolumes(docker.ListVolumesOptions{})
	if err != nil {
		return fmt.Errorf("Failed to get the docker volumes. %v", err)
	}

	workloadRWStorageDir, _ := b.workloadStorageDir(agreementId)

	// The bind string looks like this: <host-path>:<container-path>:<ro> where ro means readonly and is optional.
	for _, bind := range binds {
		containerVol := strings.Split(bind, ":")
		// The volume name does not contain slashes
		if len(containerVol) != 0 && !strings.Contains(containerVol[0], "/") {
			// check if the volume exists already
			vol_name := containerVol[0]

			// check if the volume is the workload storage, then skip it because it will be created automatically by docker
			// and it will be removed by other parts of the code
			if vol_name == workloadRWStorageDir {
				continue
			}

			bExists := false
			if volumes_docker != nil {
				for _, v_docker := range volumes_docker {
					if v_docker.Name == vol_name {
						bExists = true
						break
					}
				}
			}

			if !bExists {
				// create the volume if it does not exist
				vOption := docker.CreateVolumeOptions{
					Name:   vol_name,
					Driver: "local",
					Labels: map[string]string{
						LABEL_PREFIX + ".service_name": serviceName,
						LABEL_PREFIX + ".agreement_id": agreementId,
						LABEL_PREFIX + ".owner":        "openhorizon"},
				}

				if _, err := b.client.CreateVolume(vOption); err != nil {
					return fmt.Errorf("Failed to create the docker volume %v for service %v. %v", vol_name, serviceName, err)
				} else {
					glog.V(3).Infof("Volume %v created for service %v.", vol_name, serviceName)

					// same the volume in local db so that it can be cleaned up at unregistration time
					// Ling todo - only save the ones that are specified by the user in binds.
					if b.db != nil {
						if persistence.SaveContainerVolumeByName(b.db, vol_name); err != nil {
							return fmt.Errorf("Failed to get save the docker volume name %v into the local db. %v", vol_name, err)
						}
					}
				}
			}
		}
	}

	return nil
}

func (b *ContainerWorker) ConnectContainerToNetwork(network *docker.Network, containerId, serviceName string) error {

	epc := docker.EndpointConfig{
		Aliases:   []string{serviceName},
		Links:     nil,
		NetworkID: network.ID,
	}
	err := b.GetClient().ConnectNetwork(network.ID, docker.NetworkConnectionOptions{
		Container:      containerId,
		EndpointConfig: &epc,
		Force:          true,
	})
	if err != nil {
		return fmt.Errorf("failed to connect container %v to network %v, error %v", containerId, network.Name, err)
	}

	return nil
}

// Delete the docker volumes that are created by anax
func DeleteLeftoverDockerVolumes(db *bolt.DB, config *config.HorizonConfig) error {
	glog.V(3).Infof("Cleaning up leftover docker volumes created by anax.")

	if config.Edge.DockerEndpoint == "" {
		return fmt.Errorf("Docker client cannot be initialized. Please make sure DockerEndpoint is set in the configuration file.")
	}

	// get leftover volume names from local db
	cvs, err := persistence.FindAllUndeletedContainerVolumes(db)
	if err != nil {
		return fmt.Errorf("Error retrieving undeleted container volumes from local db. %v", err)
	} else if cvs == nil || len(cvs) == 0 {
		// nothing to remove
		glog.V(3).Infof("The cleanup process found no leftover docker volumes in the local db.")
		return nil
	}

	if client, err := docker.NewClient(config.Edge.DockerEndpoint); err != nil {
		return fmt.Errorf("Failed to instantiate docker Client: %v", err)
	} else {
		// check existing docker volumes
		volumes_docker, err := client.ListVolumes(docker.ListVolumesOptions{})
		if err != nil {
			return fmt.Errorf("Failed to get the docker volumes. %v", err)
		} else if volumes_docker == nil || len(volumes_docker) == 0 {
			// no docker volumes
			glog.V(3).Infof("The cleanup process found no docker volumes.")
			return nil
		}

		for _, cv := range cvs {

			// make sure the volume still exists and it has openhorizon as the owner
			found := false
			for _, dv := range volumes_docker {
				if cv.Name == dv.Name {
					if dv.Labels != nil && dv.Labels[LABEL_PREFIX+".owner"] == "openhorizon" {
						found = true
						break
					}
				}
			}

			// remove the volume and remove it from the local db
			if found {
				if err := client.RemoveVolume(cv.Name); err != nil {
					// failure to delete the volume should not prevent the process from going on
					glog.Errorf("Container sync resources. Failed to delete docker volume %v. %v", cv.Name, err)
				} else if err := persistence.ArchiveContainerVolumes(db, &cv); err != nil {
					return err
				} else {
					glog.V(3).Infof("Docker volume %v is removed in cleanup process.", cv.Name)
				}
			} else {
				glog.V(3).Infof("The leftover docker volume name %v is found in the local db but the volume is not in docker or it is not created by anax. It will be archived in loval db.", cv.Name)
				if err := persistence.ArchiveContainerVolumes(db, &cv); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// serviceAndWorkerTypeMatches returns true if the container type matches the ContainerWorker instance type
// (for dev and non-dev containers)
func serviceAndWorkerTypeMatches(isDevInstance bool, container *docker.APIContainers) bool {
	isDevContainer := false
	if val, exists := container.Labels[LABEL_PREFIX+".dev_service"]; exists && val == "true" {
		isDevContainer = true
	}

	if isDevInstance == isDevContainer {
		return true
	}

	return false
}
