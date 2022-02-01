package download

import (
	"fmt"
	"github.com/boltdb/bolt"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/worker"
	"path"
)

const (
	CSSAGENTUPGRADETYPE = "agent_files"

	DEBPACKAGETYPE  = "deb"
	RHELPACKAGETYPE = "rpm"
	MACPACKAGETYPE  = "pkg"

	HZN_CLUSTER_FILE   = "horizon-agent-edge-cluster-files.tar.gz"
	HZN_CONTAINER_FILE = "horizon-agent-container-%v.tar.gz"
	HZN_EDGE_FILE      = "horizon-agent-%v-%v-%v.tar.gz"
	HZN_CONFIG_FILE    = "agent-install.cfg"
	HZN_CERT_FILE      = "agent-install.crt"
)

type DownloadWorker struct {
	worker.BaseWorker
	db     *bolt.DB
	client *docker.Client
}

func NewDownloadWorker(name string, config *config.HorizonConfig, db *bolt.DB) *DownloadWorker {
	ec := getEC(config, db)

	var dockerClient *docker.Client
	var err error
	if config.Edge.DockerEndpoint != "" {
		dockerClient, err = docker.NewClient(config.Edge.DockerEndpoint)
		if err != nil {
			glog.Errorf("Failed to instantiate docker Client: %v", err)
			panic("Unable to instantiate docker Client")
		}
	}

	worker := &DownloadWorker{
		BaseWorker: worker.NewBaseWorker(name, config, ec),
		db:         db,
		client:     dockerClient,
	}

	glog.Info(dwlog(fmt.Sprintf("Starting Download Worker %v", worker.EC)))
	worker.Start(worker, 0)
	return worker
}

func (w *DownloadWorker) Messages() chan events.Message {
	return w.BaseWorker.Manager.Messages
}

func (w *DownloadWorker) CommandHandler(command worker.Command) bool {
	glog.Infof(dwlog(fmt.Sprintf("Handling command %v", command)))
	switch command.(type) {
	case *StartDownloadCommand:
		cmd := command.(*StartDownloadCommand)
		if cmd.Msg.Message.NMPStatus.IsAgentUpgradePolicy() {
			if err := w.DownloadAgentUpgradePackages(exchange.GetOrg(w.GetExchangeId()), CSSAGENTUPGRADETYPE, cmd.Msg.Message.NMPStatus.AgentUpgrade.BaseWorkingDirectory, cmd.Msg.Message.NMPName); err != nil {
				glog.Errorf(dwlog(fmt.Sprintf("Error downloading agent packages for upgrade: %v", err)))
			}
		}
	case *NodeRegisteredCommand:
		w.EC = getEC(w.Config, w.db)
	default:
		return false
	}
	return true
}

func (w *DownloadWorker) NewEvent(incoming events.Message) {
	glog.Infof(dwlog(fmt.Sprintf("Handling event: %v", incoming)))
	switch incoming.(type) {
	case *events.NMPStartDownloadMessage:
		msg, _ := incoming.(*events.NMPStartDownloadMessage)

		switch msg.Event().Id {
		case events.NMP_START_DOWNLOAD:
			cmd := NewStartDownloadCommand(msg)
			w.Commands <- cmd
		}

	case *events.NodeShutdownCompleteMessage:
		msg, _ := incoming.(*events.NodeShutdownCompleteMessage)
		switch msg.Event().Id {
		case events.UNCONFIGURE_COMPLETE:
			w.Commands <- worker.NewTerminateCommand("shutdown")
		}
	case *events.EdgeRegisteredExchangeMessage:
		msg, _ := incoming.(*events.EdgeRegisteredExchangeMessage)

		switch msg.Event().Id {
		case events.NEW_DEVICE_REG:
			cmd := NewNodeRegisteredCommand(msg)
			w.Commands <- cmd
		}
	}
}

func (w *DownloadWorker) DownloadAgentUpgradePackages(org string, objType string, filePath string, nmpName string) error {
	objId, containerObjId, err := w.formAgentUpgradePackageNames()
	if err != nil {
		return err
	}

	glog.Infof(dwlog(fmt.Sprintf("Attempting to download agent config file %v from css.", HZN_CONFIG_FILE)))
	configObjectMeta, err := exchange.GetObject(w, "IBM", HZN_CONFIG_FILE, objType)
	if err != nil {
		return fmt.Errorf("Failed to get object metadata: %v", err)
	}

	err = exchange.GetObjectData(w, "IBM", objType, HZN_CONFIG_FILE, path.Join(filePath, nmpName), HZN_CONFIG_FILE, configObjectMeta, w.client)
	if err != nil {
		w.Messages() <- events.NewNMPDownloadCompleteMessage(events.NMP_DOWNLOAD_COMPLETE, false, nmpName)
		return fmt.Errorf("Failed to get object data: %v", err)
	}

	glog.Infof(dwlog(fmt.Sprintf("Attempting to download agent cert file %v from css.", HZN_CERT_FILE)))
	certObjectMeta, err := exchange.GetObject(w, "IBM", HZN_CERT_FILE, objType)
	if err != nil {
		return fmt.Errorf("Failed to get object metadata: %v", err)
	}

	err = exchange.GetObjectData(w, "IBM", objType, HZN_CERT_FILE, path.Join(filePath, nmpName), HZN_CERT_FILE, certObjectMeta, w.client)
	if err != nil {
		w.Messages() <- events.NewNMPDownloadCompleteMessage(events.NMP_DOWNLOAD_COMPLETE, false, nmpName)
		return fmt.Errorf("Failed to get object data: %v", err)
	}

	if objId != "" {
		glog.Infof(dwlog(fmt.Sprintf("Attempting to download agent upgrade package %v from css.", objId)))
		objectMeta, err := exchange.GetObject(w, "IBM", objId, objType)
		if err != nil {
			return fmt.Errorf("Failed to get object metadata: %v", err)
		}

		err = exchange.GetObjectData(w, "IBM", objType, objId, path.Join(filePath, nmpName), objId, objectMeta, w.client)
		if err != nil {
			w.Messages() <- events.NewNMPDownloadCompleteMessage(events.NMP_DOWNLOAD_COMPLETE, false, nmpName)
			return fmt.Errorf("Failed to get object data: %v", err)
		}
	}

	if containerObjId != "" {
		glog.Infof(dwlog(fmt.Sprintf("Attempting to download agent image file %v from css.", containerObjId)))
		containerObjectMeta, err := exchange.GetObject(w, "IBM", containerObjId, objType)
		if err != nil {
			return fmt.Errorf("Failed to get object metadata: %v", err)
		}

		err = exchange.GetObjectData(w, "IBM", objType, containerObjId, "docker", containerObjId, containerObjectMeta, w.client)
		if err != nil {
			w.Messages() <- events.NewNMPDownloadCompleteMessage(events.NMP_DOWNLOAD_COMPLETE, false, nmpName)
			return fmt.Errorf("Failed to get object data: %v", err)
		}
	}

	w.Messages() <- events.NewNMPDownloadCompleteMessage(events.NMP_DOWNLOAD_COMPLETE, true, nmpName)

	return nil
}

func (w *DownloadWorker) formAgentUpgradePackageNames() (string, string, error) {
	pol, err := persistence.FindNodePolicy(w.db)
	if err != nil {
		return "", "", fmt.Errorf("Failed to retrieve node policy from local db: %v", err)
	}

	installTypeProp, err := pol.Properties.GetProperty(externalpolicy.PROP_NODE_OS)
	if err != nil {
		return "", "", fmt.Errorf("Failed to find node arch property: err")
	}

	if fmt.Sprintf("%v", installTypeProp.Value) == externalpolicy.OS_CLUSTER {
		return HZN_CLUSTER_FILE, "", nil
	}

	archProp, err := pol.Properties.GetProperty(externalpolicy.PROP_NODE_ARCH)
	if err != nil {
		return "", "", err
	}

	containerAgentFiles := ""
	containerizedProp, err := pol.Properties.GetProperty(externalpolicy.PROP_NODE_CONTAINERIZED)
	if err != nil {
		return "", "", err
	}

	if containPropBool, ok := containerizedProp.Value.(bool); ok && containPropBool {
		containerAgentFiles = fmt.Sprintf(HZN_CONTAINER_FILE, archProp.Value)
	}

	osProp := fmt.Sprintf("%v", installTypeProp.Value)
	if osProp != "" {
		pkgType := getPkgTypeForInstallType(osProp)
		if pkgType == "" {
			return "", containerAgentFiles, fmt.Errorf("Failed to find package type for install type %v", installTypeProp)
		}

		osType := "linux"

		if fmt.Sprintf("%v", installTypeProp.Value) == "mac" {
			osType = "macos"
		}

		return fmt.Sprintf(HZN_EDGE_FILE, osType, pkgType, archProp.Value), containerAgentFiles, nil
	}

	return "", containerAgentFiles, nil
}

func getEC(config *config.HorizonConfig, db *bolt.DB) *worker.BaseExchangeContext {
	var ec *worker.BaseExchangeContext
	if dev, _ := persistence.FindExchangeDevice(db); dev != nil {
		ec = worker.NewExchangeContext(fmt.Sprintf("%v/%v", dev.Org, dev.Id), dev.Token, config.Edge.ExchangeURL, config.GetCSSURL(), config.Collaborators.HTTPClientFactory)
	}

	return ec
}

func getPkgTypeForInstallType(install string) string {
	if install == externalpolicy.OS_MAC {
		return MACPACKAGETYPE
	} else if install == externalpolicy.OS_UBUNTU || install == externalpolicy.OS_DEBIAN {
		return DEBPACKAGETYPE
	} else if install == externalpolicy.OS_RHEL {
		return RHELPACKAGETYPE
	}

	return ""
}

func dwlog(input string) string {
	return fmt.Sprintf("Download worker: %v", input)
}
