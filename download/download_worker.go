package download

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/semanticversion"
	"github.com/open-horizon/anax/worker"
	"path"
	"sort"
)

const (
	CSSSHAREDORG = "IBM"

	LATESTVERSION = "latest"

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
	db *bolt.DB
}

func NewDownloadWorker(name string, config *config.HorizonConfig, db *bolt.DB) *DownloadWorker {
	ec := getEC(config, db)

	worker := &DownloadWorker{
		BaseWorker: worker.NewBaseWorker(name, config, ec),
		db:         db,
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
			if err := w.DownloadAgentUpgradePackages(exchange.GetOrg(w.GetExchangeId()), cmd.Msg.Message.NMPStatus.AgentUpgrade.BaseWorkingDirectory, cmd.Msg.Message.NMPName, cmd.Msg.Message.NMPStatus); err != nil {
				w.Messages() <- events.NewNMPDownloadCompleteMessage(events.NMP_DOWNLOAD_COMPLETE, exchangecommon.STATUS_DOWNLOAD_FAILED, cmd.Msg.Message.NMPName, nil, nil)
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
	glog.Infof(dwlog(fmt.Sprintf("Attempting to download css file %v/%v/%v to file %v", org, objType, objId, filePath)))
	objMeta, err := exchange.GetObject(w, org, objId, objType)
	if err != nil {
		return fmt.Errorf("Failed to get metadata for css object %v/%v/%v. Error was: %v", org, objType, objId, err)
	}

	filePath = path.Join(filePath, nmpName)

	if w.Config.IsDataChunkEnabled() && int(objMeta.ObjectSize) > w.Config.GetFileSyncServiceMaxDataChunkSize() {
		offsetStep := w.Config.GetFileSyncServiceMaxDataChunkSize()
		startOffest := 0
		endOffset := offsetStep
		lastChunk := false
		for !lastChunk {
			if endOffset > int(objMeta.ObjectSize) {
				lastChunk = true
				endOffset = int(objMeta.ObjectSize)
			}
			_, err = exchange.GetObjectDataByChunk(w, org, objType, objId, int64(startOffest), int64(endOffset), lastChunk, filePath, objId)
			if err != nil {
				return fmt.Errorf("Failed to get object %v/%v/%v data chunk. Error was %v.", org, objType, objId, err)
			}
			startOffest = endOffset
			endOffset = endOffset + offsetStep
		}
	} else {
		err = exchange.GetObjectData(w, org, objType, objId, filePath, objId, objMeta)
		if err != nil {
			w.Messages() <- events.NewNMPDownloadCompleteMessage(events.NMP_DOWNLOAD_COMPLETE, exchangecommon.STATUS_DOWNLOAD_FAILED, nmpName, nil, nil)
			return fmt.Errorf("Failed to get data for object %v/%v/%v. Error was: %v", org, objType, objId, err)
		}
	}

	return nil
}

// Download the manifest, then all packages required by it
func (w *DownloadWorker) DownloadAgentUpgradePackages(org string, filePath string, nmpName string, nmpStatus *exchangecommon.NodeManagementPolicyStatus) error {
	objIds, err := w.formAgentUpgradePackageNames()
	if err != nil {
		return err
	}

	// If org is specified in the manifest id, use that org. Otherwise use the user org
	manOrg, manId := cutil.SplitOrgSpecUrl(nmpStatus.AgentUpgradeInternal.Manifest)
	if manOrg == "" {
		manOrg = org
		manId = nmpStatus.AgentUpgradeInternal.Manifest
	}

	manifest, err := exchange.GetManifestData(w, manOrg, exchangecommon.AU_MANIFEST_TYPE, manId)
	if err != nil {
		return err
	}
	glog.Infof(dwlog(fmt.Sprintf("Found nmp %v manifest: %v", nmpName, manifest)))

	manifestUpgradeVersions, err := findAgentUpgradePackageVersions(manifest.Software.Version, manifest.Configuration.Version, manifest.Certificate.Version, exchange.GetNodeUpgradeVersionsHandler(w))
	if err != nil {
		return err
	}
	upgradeVersions, err := w.ResolveUpgradeVersions(manifestUpgradeVersions, nmpName, nmpStatus)
	if err != nil {
		return err
	}

	swType, configType, certType := getUpgradeCSSType(upgradeVersions)

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

	latestVersions := checkForLatestKeywords(manifest)

	// Return the software version regardless of whether or not it was upgraded as this version is set in the software
	// The config and cert versions should be the actual version downloaded so after the upgrade is executed, these versions can be used to set the device versions
	versionsToSave := exchangecommon.AgentUpgradeVersions{SoftwareVersion: manifestUpgradeVersions.SoftwareVersion, ConfigVersion: upgradeVersions.ConfigVersion, CertVersion: upgradeVersions.CertVersion}

	if swType != "" || configType != "" || certType != "" {
		w.Messages() <- events.NewNMPDownloadCompleteMessage(events.NMP_DOWNLOAD_COMPLETE, exchangecommon.STATUS_DOWNLOADED, nmpName, &versionsToSave, latestVersions)
	} else {
		w.Messages() <- events.NewNMPDownloadCompleteMessage(events.NMP_DOWNLOAD_COMPLETE, exchangecommon.STATUS_SUCCESSFUL, nmpName, &versionsToSave, latestVersions)
	}

	return nil
}

// This  function takes as input upgrade versions from the nmp and returns an upgradee versions struct with only the upgrades that should execute present
// If the nmp version is higher than the node's current version, it should execute
// If the nmp version is the same or lower than the node's current version:
// Allow only if allowDowngrade is true and there is no nmp status with a more recent start time that updated the same resource
func (w *DownloadWorker) ResolveUpgradeVersions(upgradeVersions *exchangecommon.AgentUpgradeVersions, nmpName string, nmpStatus *exchangecommon.NodeManagementPolicyStatus) (*exchangecommon.AgentUpgradeVersions, error) {
	dev, err := persistence.FindExchangeDevice(w.db)
	if err != nil || dev == nil {
		return nil, fmt.Errorf("Failed to get device from the local db: %v", err)
	}
	versToDownload := exchangecommon.AgentUpgradeVersions{}

	currentVers := dev.SoftwareVersions[persistence.AGENT_VERSION]
	if currentVers == "local build" {
		currentVers = "0.0.0"
	}
	if upgradeVersions.SoftwareVersion != "" {
		if comp, err := semanticversion.CompareVersions(currentVers, upgradeVersions.SoftwareVersion); err != nil && currentVers != "" {
			return nil, fmt.Errorf("Error checking upgrade version against current node version: %v", err)
		} else if err == nil && comp >= 0 {
			// The software version from the nmp found is a downgrade or same as current level
			if nmpStatus.AgentUpgradeInternal.AllowDowngrade {
				if statuses, err := persistence.FindNodeUpgradeStatusesWithTypeAfterTime(w.db, nmpStatus.AgentUpgradeInternal.ScheduledUnixTime, "software"); err != nil {
					glog.Errorf("Error finding node statuses in db: %v", err)
				} else if len(statuses) == 0 {
					// No more recent node management policies that have a software upgrade. This downgrade should be executed
					versToDownload.SoftwareVersion = upgradeVersions.SoftwareVersion
				}
			} else {
				glog.Infof("Current node version %v is higher than or same as version %v from nmp %v. No need to download packages.", currentVers, upgradeVersions.SoftwareVersion, nmpName)
			}
		} else {
			// The software version is an upgrade. Allow it
			versToDownload.SoftwareVersion = upgradeVersions.SoftwareVersion
		}
	}

	currentVers = dev.SoftwareVersions[persistence.CONFIG_VERSION]
	if upgradeVersions.ConfigVersion != "" {
		if comp, err := semanticversion.CompareVersions(currentVers, upgradeVersions.ConfigVersion); err != nil && currentVers != "" {
			return nil, fmt.Errorf("Error checking upgrade version against current node version: %v", err)
		} else if err == nil && comp >= 0 {
			// The config version from the nmp found is a downgrade or same as current level
			if nmpStatus.AgentUpgradeInternal.AllowDowngrade {
				if statuses, err := persistence.FindNodeUpgradeStatusesWithTypeAfterTime(w.db, nmpStatus.AgentUpgradeInternal.ScheduledUnixTime, "config"); err != nil {
					glog.Errorf("Error finding node statuses in db: %v", err)
				} else if len(statuses) == 0 {
					// No more recent node management policies that have a config upgrade. This downgrade should be executed
					versToDownload.ConfigVersion = upgradeVersions.ConfigVersion
				}
			} else {
				glog.Infof("Current config version %v is higher than or same as version %v from nmp %v. No need to download packages.", currentVers, upgradeVersions.ConfigVersion, nmpName)
			}
		} else {
			// The config version is an upgrade. Allow it
			versToDownload.ConfigVersion = upgradeVersions.ConfigVersion
		}
	}

	currentVers = dev.SoftwareVersions[persistence.CERT_VERSION]
	if upgradeVersions.CertVersion != "" {
		if comp, err := semanticversion.CompareVersions(currentVers, upgradeVersions.CertVersion); err != nil && currentVers != "" {
			return nil, fmt.Errorf("Error checking upgrade version against current node version: %v", err)
		} else if err == nil && comp >= 0 {
			// The cert version from the nmp found is a downgrade or same as current level
			if nmpStatus.AgentUpgradeInternal.AllowDowngrade {
				if statuses, err := persistence.FindNodeUpgradeStatusesWithTypeAfterTime(w.db, nmpStatus.AgentUpgradeInternal.ScheduledUnixTime, "cert"); err != nil {
					glog.Errorf("Error finding node statuses in db: %v", err)
				} else if len(statuses) == 0 {
					// No more recent node management policies that have a cert upgrade. This downgrade should be executed
					versToDownload.CertVersion = upgradeVersions.CertVersion
				}
			} else {
				glog.Infof("Current cert version %v is higher than or same as version %v from nmp %v. No need to download packages.", currentVers, upgradeVersions.CertVersion, nmpName)
			}
		} else {
			// The cert version is an upgrade. Allow it
			versToDownload.CertVersion = upgradeVersions.CertVersion
		}
	}

	return &versToDownload, nil
}

// Use the upgrade type to create the object css type
func getUpgradeCSSType(vers *exchangecommon.AgentUpgradeVersions) (swType string, configType string, certType string) {
	swType = ""
	configType = ""
	certType = ""
	if vers.SoftwareVersion != "" {
		swType = fmt.Sprintf("%s-%s", exchangecommon.AU_AGENTFILE_TYPE_SOFTWARE, vers.SoftwareVersion)
	}
	if vers.ConfigVersion != "" {
		configType = fmt.Sprintf("%s-%s", exchangecommon.AU_AGENTFILE_TYPE_CONFIG, vers.ConfigVersion)
	}
	if vers.CertVersion != "" {
		certType = fmt.Sprintf("%s-%s", exchangecommon.AU_AGENTFILE_TYPE_CERT, vers.CertVersion)
	}
	return
}

// Find the best matching version availible to generate the css type
func findAgentUpgradePackageVersions(softwareManifestVers string, configManifestVers string, certManifestVers string, getUpgradeVers exchange.NodeUpgradeVersionsHandler) (*exchangecommon.AgentUpgradeVersions, error) {
	versions, err := getUpgradeVers()
	upgradeVersions := exchangecommon.AgentUpgradeVersions{}
	if err != nil {
		return nil, err
	}

	if softwareManifestVers != "" {
		if vers, err := findBestMatchingVersion(versions.SoftwareVersions, softwareManifestVers); err != nil {
			return nil, err
		} else {
			upgradeVersions.SoftwareVersion = vers
		}

	}
	if configManifestVers != "" {
		if vers, err := findBestMatchingVersion(versions.ConfigVersions, configManifestVers); err != nil {
			return nil, err
		} else {
			upgradeVersions.ConfigVersion = vers
		}
	}
	if certManifestVers != "" {
		if vers, err := findBestMatchingVersion(versions.CertVersions, certManifestVers); err != nil {
			return nil, err
		} else {
			upgradeVersions.CertVersion = vers
		}
	}

	return &upgradeVersions, nil
}

// If the preferred version is current, return the highest version
// If the preferred version is a range, return the highest version currrently availible
func findBestMatchingVersion(availibleVers []string, preferredVers string) (string, error) {
	goodVers := []string{}
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
func (w *DownloadWorker) formAgentUpgradePackageNames() (*[]string, error) {
	pol, err := persistence.FindNodePolicy(w.db)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve node policy from local db: %v", err)
	} else if pol == nil {
		return nil, fmt.Errorf("No node policy found in the local db.")
	}

	if dev, err := persistence.FindExchangeDevice(w.db); dev == nil || err != nil {
		return nil, fmt.Errorf("Failed to get device from the local db: %v", err)
	} else if dev.NodeType == persistence.DEVICE_TYPE_CLUSTER {
		return &[]string{HZN_CLUSTER_FILE, HZN_CLUSTER_IMAGE}, nil
	}

	installTypeProp, err := pol.Properties.GetProperty(externalpolicy.PROP_NODE_OS)
	if err != nil {
		return nil, fmt.Errorf("Failed to find node os property: %v", err)
	}

	archProp, err := pol.Properties.GetProperty(externalpolicy.PROP_NODE_ARCH)
	if err != nil {
		return nil, err
	}

	allFiles := []string{}

	containerizedProp, err := pol.Properties.GetProperty(externalpolicy.PROP_NODE_CONTAINERIZED)
	if err != nil {
		return nil, err
	}

	if containPropBool, ok := containerizedProp.Value.(bool); ok && containPropBool {
		allFiles = append(allFiles, fmt.Sprintf(HZN_CONTAINER_FILE, archProp.Value))
	}

	osProp := fmt.Sprintf("%v", installTypeProp.Value)
	archPropVal := fmt.Sprintf("%v", archProp.Value)
	if osProp != "" {
		pkgType := getPkgTypeForInstallType(osProp)
		if pkgType == "" {
			return &allFiles, fmt.Errorf("Failed to find package type for install type %v", installTypeProp)
		}

		osType := "linux"

		if fmt.Sprintf("%v", installTypeProp.Value) == externalpolicy.OS_MAC {
			osType = externalpolicy.OS_MAC
		}

		pkgArch := getPkgArch(pkgType, archPropVal)
		allFiles = append(allFiles, fmt.Sprintf(HZN_EDGE_FILE, osType, pkgType, pkgArch))
		return &allFiles, nil
	}

	return &allFiles, nil
}

func checkForLatestKeywords(manifest *exchangecommon.UpgradeManifest) *exchangecommon.AgentUpgradeLatest {
	if manifest == nil {
		return nil
	}

	latestVers := exchangecommon.AgentUpgradeLatest{}

	if manifest.Software.Version == LATESTVERSION {
		latestVers.SoftwareLatest = true
	}
	if manifest.Certificate.Version == LATESTVERSION {
		latestVers.CertLatest = true
	}
	if manifest.Configuration.Version == LATESTVERSION {
		latestVers.ConfigLatest = true
	}

	return &latestVers
}

func getEC(config *config.HorizonConfig, db *bolt.DB) *worker.BaseExchangeContext {
	var ec *worker.BaseExchangeContext
	if dev, _ := persistence.FindExchangeDevice(db); dev != nil {
		ec = worker.NewExchangeContext(fmt.Sprintf("%v/%v", dev.Org, dev.Id), dev.Token, config.Edge.ExchangeURL, config.GetCSSURL(), config.Collaborators.HTTPClientFactory)
	}

	return ec
}

// match the operating system with the corresponding install package type
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

// match the GOARCH with the arch name used for install packages
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
