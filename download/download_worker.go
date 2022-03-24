package download

import (
	"fmt"
	"github.com/boltdb/bolt"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	_ "github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/semanticversion"
	"github.com/open-horizon/anax/worker"
	"path"
	"sort"
)

const (
	CSSSOFTWAREUPGRADETYPE      = "agent_software_files"
	CSSCONFIGUPGRADETYPE        = "agent_config_files"
	CSSCERTUPGRADETYPE          = "agent_cert_files"
	CSSAGENTUPGRADEMANIFESTTYPE = "agent_upgrade_manifests"
	CSSSHAREDORG                = "IBM"

	LATESTVERSION = "current"

	DEBPACKAGETYPE  = "deb"
	RHELPACKAGETYPE = "rpm"
	MACPACKAGETYPE  = "pkg"

	HZN_CLUSTER_FILE   = "horizon-agent-edge-cluster-files.tar.gz"
	HZN_CLUSTER_IMAGE  = "amd64_anax_k8s.tar.gz"
	HZN_CONTAINER_FILE = "%v_anax.tar.gz"
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
			if err := w.DownloadAgentUpgradePackages(exchange.GetOrg(w.GetExchangeId()), cmd.Msg.Message.NMPStatus.AgentUpgrade.BaseWorkingDirectory, cmd.Msg.Message.NMPName, cmd.Msg.Message.NMPStatus.AgentUpgradeInternal.Manifest); err != nil {
				w.Messages() <- events.NewNMPDownloadCompleteMessage(events.NMP_DOWNLOAD_COMPLETE, false, cmd.Msg.Message.NMPName)
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

// Download the given object from css
func (w *DownloadWorker) DownloadCSSObject(org string, objType string, objId string, filePath string, nmpName string) error {
	filePath = path.Join(filePath, nmpName)
	glog.Infof(dwlog(fmt.Sprintf("Attempting to download css file %v/%v/%v to file %v", org, objType, objId, filePath)))
	objMeta, err := exchange.GetObject(w, org, objId, objType)
	if err != nil {
		return fmt.Errorf("Failed to get metadata for css object %v/%v/%v. Error was: %v", org, objType, objId, err)
	}

	err = exchange.GetObjectData(w, org, objType, objId, filePath, objId, objMeta, w.client)
	if err != nil {
		w.Messages() <- events.NewNMPDownloadCompleteMessage(events.NMP_DOWNLOAD_COMPLETE, false, nmpName)
		return fmt.Errorf("Failed to get data for object %v/%v/%v. Error was: %v", org, objType, objId, err)
	}

	return nil
}

// Download the manifest, then all packages required by it
func (w *DownloadWorker) DownloadAgentUpgradePackages(org string, filePath string, nmpName string, manifestId string) error {
	objIds, containerObjId, err := w.formAgentUpgradePackageNames()
	if err != nil {
		return err
	}

	// If org is specified in the manifest id, use that org. Otherwise use the user org
	manOrg, manId := cutil.SplitOrgSpecUrl(manifestId)
	if manOrg == "" {
		manOrg = org
		manId = manifestId
	}

	manifest, err := exchange.GetManifestData(w, manOrg, CSSAGENTUPGRADEMANIFESTTYPE, manId)
	if err != nil {
		return err
	}
	glog.Infof(dwlog(fmt.Sprintf("Found nmp %v manifest: %v", nmpName, manifest)))

	swType, configType, certType, err := w.FindAgentUpgradePackageTypes(manifest.Software.Version, manifest.Configuration.Version, manifest.Certificate.Version)
	if err != nil {
		return err
	}

	if swType != "" {
		if objIds != nil {
			for _, objId := range *objIds {
				if cutil.SliceContains(manifest.Software.FileList, objId) {
					if err = w.DownloadCSSObject(CSSSHAREDORG, swType, objId, filePath, nmpName); err != nil {
						return fmt.Errorf("Error downloading css object %v/%v/%v: %v", CSSSHAREDORG, swType, objId, err)
					}
				} else {
					glog.Infof("No software upgrade object found of expected type %v found in manifest list.", objId)
				}
			}
		}

		if containerObjId != "" && cutil.SliceContains(manifest.Software.FileList, containerObjId) {
			if err = w.DownloadCSSObject(CSSSHAREDORG, swType, containerObjId, "docker", nmpName); err != nil {
				return fmt.Errorf("Error downloading css object %v/%v/%v: %v", CSSSHAREDORG, swType, containerObjId, err)
			}
		} else if containerObjId != "" {
			glog.Infof("No software upgrade object found of expected type %v found in manifest list.", containerObjId)
		}
	}

	if configType != "" {
		if cutil.SliceContains(manifest.Configuration.FileList, HZN_CONFIG_FILE) {
			if err = w.DownloadCSSObject(CSSSHAREDORG, configType, HZN_CONFIG_FILE, filePath, nmpName); err != nil {
				return fmt.Errorf("Error downloading css object %v/%v/%v: %v", CSSSHAREDORG, configType, HZN_CONFIG_FILE, err)
			}
		} else {
			glog.Infof("No config upgrade object found of expected type %v found in manifest list.", HZN_CONFIG_FILE)
		}
	}

	if certType != "" {
		if cutil.SliceContains(manifest.Certificate.FileList, HZN_CERT_FILE) {
			if err = w.DownloadCSSObject(CSSSHAREDORG, certType, HZN_CERT_FILE, filePath, nmpName); err != nil {
				return fmt.Errorf("Error downloading css object %v/%v/%v: %v", CSSSHAREDORG, certType, HZN_CERT_FILE, err)
			}
		} else {
			glog.Infof("No cert upgrade object found of expected type %v found in manifest list.", HZN_CERT_FILE)
		}
	}

	w.Messages() <- events.NewNMPDownloadCompleteMessage(events.NMP_DOWNLOAD_COMPLETE, true, nmpName)

	return nil
}

// Find the best matching version availible to generate the css type
func (w *DownloadWorker) FindAgentUpgradePackageTypes(softwareManifestVers string, configManifestVers string, certManifestVers string) (string, string, string, error) {
	swType := ""
	configType := ""
	certType := ""
	versions, err := exchange.GetNodeUpgradeVersions(w)
	if err != nil {
		// 	return "","","",err
	}

	if softwareManifestVers != "" {
		if vers, err := findBestMatchingVersion(versions.SoftwareVersions, softwareManifestVers); err != nil {
			return swType, configType, certType, err
		} else {
			swType = fmt.Sprintf("%s-%s", CSSSOFTWAREUPGRADETYPE, vers)
		}
	}
	if configManifestVers != "" {
		if vers, err := findBestMatchingVersion(versions.ConfigVersions, configManifestVers); err != nil {
			return swType, configType, certType, err
		} else {
			configType = fmt.Sprintf("%s-%s", CSSCONFIGUPGRADETYPE, vers)
		}
	}
	if certManifestVers != "" {
		if vers, err := findBestMatchingVersion(versions.CertVersions, certManifestVers); err != nil {
			return swType, configType, certType, err
		} else {
			certType = fmt.Sprintf("%s-%s", CSSCERTUPGRADETYPE, vers)
		}
	}

	return swType, configType, certType, nil
}

// If the preferred version is current, return the highest version
// If the preferred version is a range, return the highest version currrently availible
func findBestMatchingVersion(availibleVers []string, preferredVers string) (string, error) {
	// Only works for single versions specified until the exchange file version api is ready
	return preferredVers, nil

	goodVers := make([]string, len(availibleVers))
	for _, vers := range availibleVers {
		if !semanticversion.IsVersionString(vers) {
			glog.Errorf(dwlog(fmt.Sprintf("Ignoring invalid software version %v in list of current agent upgrade files versions.", vers)))
		} else {
			goodVers = append(goodVers, vers)
		}
	}
	availibleVers = goodVers

	sort.Slice(availibleVers, func(i, j int) bool {
		comp, _ := semanticversion.CompareVersions(availibleVers[i], availibleVers[j])
		return comp > 0
	})

	if preferredVers == LATESTVERSION {
		return availibleVers[0], nil
	} else if semanticversion.IsVersionString(preferredVers) {
		for _, vers := range availibleVers {
			if res, err := semanticversion.CompareVersions(preferredVers, vers); res == 0 && err == nil {
				return vers, nil
			}
		}
		return "", fmt.Errorf("No version matching %v found in availible versions %v.", preferredVers, availibleVers)
	} else if prefVers, err := semanticversion.Version_Expression_Factory(preferredVers); err == nil {
		for _, vers := range availibleVers {
			if match, err := prefVers.Is_within_range(vers); err == nil && match {
				return vers, nil
			}
		}
	} else {
		return "", fmt.Errorf("Unrecognized version expression string %v.", preferredVers)
	}

	return "", fmt.Errorf("Failed to find matching version.")
}

// Create the package names from the system information
func (w *DownloadWorker) formAgentUpgradePackageNames() (*[]string, string, error) {
	pol, err := persistence.FindNodePolicy(w.db)
	if err != nil {
		return nil, "", fmt.Errorf("Failed to retrieve node policy from local db: %v", err)
	} else if pol == nil {
		return nil, "", fmt.Errorf("No node policy found in the local db.")
	}

	if dev, err := persistence.FindExchangeDevice(w.db); dev == nil || err != nil {
		return nil, "", fmt.Errorf("Failed to get device from the local db: %v", err)
	} else if dev.NodeType == persistence.DEVICE_TYPE_CLUSTER {
		return &[]string{HZN_CLUSTER_FILE, HZN_CLUSTER_IMAGE}, "", nil
	}

	installTypeProp, err := pol.Properties.GetProperty(externalpolicy.PROP_NODE_OS)
	if err != nil {
		return nil, "", fmt.Errorf("Failed to find node os property: %v", err)
	}

	archProp, err := pol.Properties.GetProperty(externalpolicy.PROP_NODE_ARCH)
	if err != nil {
		return nil, "", err
	}

	containerAgentFiles := ""
	containerizedProp, err := pol.Properties.GetProperty(externalpolicy.PROP_NODE_CONTAINERIZED)
	if err != nil {
		return nil, "", err
	}

	if containPropBool, ok := containerizedProp.Value.(bool); ok && containPropBool {
		containerAgentFiles = fmt.Sprintf(HZN_CONTAINER_FILE, archProp.Value)
	}

	osProp := fmt.Sprintf("%v", installTypeProp.Value)
	archPropVal := fmt.Sprintf("%v", archProp.Value)
	if osProp != "" {
		pkgType := getPkgTypeForInstallType(osProp)
		if pkgType == "" {
			return nil, containerAgentFiles, fmt.Errorf("Failed to find package type for install type %v", installTypeProp)
		}

		osType := "linux"

		if fmt.Sprintf("%v", installTypeProp.Value) == externalpolicy.OS_MAC {
			osType = externalpolicy.OS_MAC
		}

		pkgArch := getPkgArch(pkgType, archPropVal)

		return &[]string{fmt.Sprintf(HZN_EDGE_FILE, osType, pkgType, pkgArch)}, containerAgentFiles, nil
	}

	return nil, containerAgentFiles, nil
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
	} else if install == externalpolicy.OS_UBUNTU || install == externalpolicy.OS_DEBIAN || install == externalpolicy.OS_RASPBIAN {
		return DEBPACKAGETYPE
	} else if install == externalpolicy.OS_RHEL || install == externalpolicy.OS_CENTOS || install == externalpolicy.OS_FEDORA || install == externalpolicy.OS_SUSE {
		return RHELPACKAGETYPE
	}

	return ""
}

func getPkgArch(pkgType string, arch string) string {
	pkgArch := arch
	if arch == "arm" {
		pkgArch = "armhf"
	} else if arch == "amd64" && (pkgType == MACPACKAGETYPE || pkgType == RHELPACKAGETYPE) {
		pkgArch = "x86_64"
	}

	return pkgArch
}

func dwlog(input string) string {
	return fmt.Sprintf("Download worker: %v", input)
}
