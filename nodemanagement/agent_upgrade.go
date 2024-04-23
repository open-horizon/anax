package nodemanagement

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/common"
	"github.com/open-horizon/anax/eventlog"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/semanticversion"
	"github.com/open-horizon/anax/version"
	"os"
	"path"
	"sort"
)

const STATUS_FILE_NAME = "status.json"
const NMP_MONITOR = "NMPMonitor"

// this function will set the status of any nmp in "download started" to "waiting"
// run this when the node starts or is registered so a partial download that ended unexpectedly  will be restarted
func (w *NodeManagementWorker) ResetDownloadStartedStatuses() error {
	downloadStartedStatuses, err := persistence.FindDownloadStartedNMPStatuses(w.db)
	if err != nil {
		return err
	}
	for statusName, status := range downloadStartedStatuses {
		status.SetStatus(exchangecommon.STATUS_NEW)
		if err := w.UpdateStatus(statusName, status, exchange.GetPutNodeManagementPolicyStatusHandler(w), persistence.NewMessageMeta(EL_NMP_STATUS_CHANGED, statusName, exchangecommon.STATUS_NEW), persistence.EC_NMP_STATUS_UPDATE_NEW); err != nil {
			return err
		}
	}

	return nil
}

// this returns the name and status struct of the status with the eariest scheduled start time and deletes that earliest status from the map passed in
func getLatest(statusMap *map[string]*exchangecommon.NodeManagementPolicyStatus) (string, *exchangecommon.NodeManagementPolicyStatus) {
	latestNmpName := ""
	latestNmpStatus := &exchangecommon.NodeManagementPolicyStatus{}
	if statusMap == nil {
		return "", nil
	}

	for nmpName, nmpStatus := range *statusMap {
		if nmpStatus != nil && nmpStatus.TimeToStart() {
			if latestNmpName == "" || !nmpStatus.AgentUpgradeInternal.ScheduledUnixTime.Before(latestNmpStatus.AgentUpgradeInternal.ScheduledUnixTime) {
				latestNmpStatus = nmpStatus
				latestNmpName = nmpName
			}
		}
	}
	delete(*statusMap, latestNmpName)
	return latestNmpName, latestNmpStatus
}

// After a successful download,  update the node status in the db and the exchange and create an eventlog event for the change
func (n *NodeManagementWorker) DownloadComplete(cmd *NMPDownloadCompleteCommand) {
	status, err := persistence.FindNMPStatus(n.db, cmd.Msg.NMPName)
	if err != nil {
		glog.Errorf(nmwlog(fmt.Sprintf("Failed to get nmp status %v from the database: %v", cmd.Msg.NMPName, err)))
		return
	} else if status == nil {
		glog.Errorf(nmwlog(fmt.Sprintf("Failed to find status for nmp %v in the database.", cmd.Msg.NMPName)))
		return
	}
	var msgMeta *persistence.MessageMeta
	eventCode := ""

	if cmd.Msg.Status == exchangecommon.STATUS_NO_ACTION {
		glog.Infof(nmwlog(fmt.Sprintf("Already in compliance with nmp %v. Download skipped.", cmd.Msg.NMPName)))
		status.SetStatus(exchangecommon.STATUS_NO_ACTION)
		msgMeta = persistence.NewMessageMeta(EL_NMP_STATUS_CHANGED, cmd.Msg.NMPName, exchangecommon.STATUS_NO_ACTION)
		eventCode = persistence.EC_NMP_STATUS_DOWNLOAD_SUCCESSFUL
	} else if cmd.Msg.Status == exchangecommon.STATUS_DOWNLOADED {
		glog.Infof(nmwlog(fmt.Sprintf("Sucessfully downloaded packages for nmp %v.", cmd.Msg.NMPName)))
		if dev, err := exchange.GetExchangeDevice(n.GetHTTPFactory(), n.GetExchangeId(), n.GetExchangeId(), n.GetExchangeToken(), n.GetExchangeURL()); err != nil {
			glog.Errorf(nmwlog(fmt.Sprintf("Failed to get device from the db: %v", err)))
			return
		} else if dev.HAGroup != "" {
			status.SetStatus(exchangecommon.STATUS_HA_WAITING)
			msgMeta = persistence.NewMessageMeta(EL_NMP_STATUS_CHANGED, cmd.Msg.NMPName, exchangecommon.STATUS_HA_WAITING)
			eventCode = persistence.EC_NMP_STATUS_CHANGED
		} else {
			status.SetStatus(exchangecommon.STATUS_DOWNLOADED)
			msgMeta = persistence.NewMessageMeta(EL_NMP_STATUS_CHANGED, cmd.Msg.NMPName, exchangecommon.STATUS_DOWNLOADED)
			eventCode = persistence.EC_NMP_STATUS_DOWNLOAD_SUCCESSFUL
		}
	} else if cmd.Msg.Status == exchangecommon.STATUS_PRECHECK_FAILED {
		glog.Infof(nmwlog(fmt.Sprintf("Node management policy %v failed precheck conditions. %v", cmd.Msg.NMPName, cmd.Msg.ErrorMessage)))
		status.SetStatus(exchangecommon.STATUS_PRECHECK_FAILED)
		status.SetErrorMessage(cmd.Msg.ErrorMessage)
		msgMeta = persistence.NewMessageMeta(EL_NMP_STATUS_CHANGED_WITH_ERROR, cmd.Msg.NMPName, exchangecommon.STATUS_PRECHECK_FAILED, cmd.Msg.ErrorMessage)
		eventCode = persistence.EC_NMP_STATUS_CHANGED
	} else {
		if status.AgentUpgradeInternal.DownloadAttempts < 4 {
			glog.Infof(nmwlog(fmt.Sprintf("Resetting status for %v to waiting to retry failed download.", cmd.Msg.NMPName)))
			status.AgentUpgradeInternal.DownloadAttempts = status.AgentUpgradeInternal.DownloadAttempts + 1
			status.SetStatus(exchangecommon.STATUS_NEW)
			msgMeta = persistence.NewMessageMeta(EL_NMP_STATUS_CHANGED, cmd.Msg.NMPName, exchangecommon.STATUS_NEW)
			eventCode = persistence.EC_NMP_STATUS_CHANGED
		} else {
			glog.Infof(nmwlog(fmt.Sprintf("Download attempted 3 times already for %v. Download will not be tried again.", cmd.Msg.NMPName)))
			glog.Errorf(nmwlog(fmt.Sprintf("Failed to download packages for nmp %v. %v", cmd.Msg.NMPName, cmd.Msg.ErrorMessage)))
			status.SetStatus(cmd.Msg.Status)
			status.SetErrorMessage(cmd.Msg.ErrorMessage)
			msgMeta = persistence.NewMessageMeta(EL_NMP_STATUS_CHANGED_WITH_ERROR, cmd.Msg.NMPName, cmd.Msg.Status, cmd.Msg.ErrorMessage)
			eventCode = persistence.EC_NMP_STATUS_CHANGED
		}
	}
	if cmd.Msg.Versions != nil {
		status.AgentUpgrade.UpgradedVersions = *cmd.Msg.Versions
	}
	if cmd.Msg.Latests != nil {
		status.AgentUpgradeInternal.LatestMap = *cmd.Msg.Latests
	}
	err = n.UpdateStatus(cmd.Msg.NMPName, status, exchange.GetPutNodeManagementPolicyStatusHandler(n), msgMeta, eventCode)
	if err != nil {
		glog.Errorf(nmwlog(fmt.Sprintf("Failed to update nmp status %v: %v", cmd.Msg.NMPName, err)))
	}

	if cmd.Msg.Status == exchangecommon.STATUS_DOWNLOADED {
		n.Messages() <- events.NewAgentPackageDownloadedMessage(events.AGENT_PACKAGE_DOWNLOADED, events.StartDownloadMessage{NMPStatus: status, NMPName: cmd.Msg.NMPName})
	}
}

// Read and persist the status out of the file
// Update status in the exchange
// If everything is successful, delete the job working dir
func (n *NodeManagementWorker) CollectStatus(workingFolderPath string, policyName string, dbStatus *exchangecommon.NodeManagementPolicyStatus) error {
	filePath := path.Join(workingFolderPath, policyName, STATUS_FILE_NAME)
	// Read in the status file
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("Failed to open status file %v for management job %v. Error was: %v", filePath, policyName, err)
	}
	if openPath, err := os.Open(filePath); err != nil {
		return fmt.Errorf("Failed to open status file %v for management job %v. Errorf was: %v", filePath, policyName, err)
	} else {
		contents := exchangecommon.NodeManagementPolicyStatus{}
		err = json.NewDecoder(openPath).Decode(&contents)
		if err != nil {
			return fmt.Errorf("Failed to decode status file %v for management job %v. Error was %v.", filePath, policyName, err)
		}

		exchDev, err := persistence.FindExchangeDevice(n.db)
		if err != nil {
			glog.Errorf(nmwlog(fmt.Sprintf("Error getting device from database: %v", err)))
			exchDev = nil
		}

		status_changed, err := common.SetNodeManagementPolicyStatus(n.db, exchDev, policyName, &contents, dbStatus,
			exchange.GetPutNodeManagementPolicyStatusHandler(n),
			exchange.GetHTTPDeviceHandler(n),
			exchange.GetHTTPPatchDeviceHandler(n))
		if err != nil {
			glog.Errorf(nmwlog(fmt.Sprintf("Error saving nmp status for %v: %v", policyName, err)))
			return err
		} else {
			// log the event
			if status_changed {
				pattern := ""
				configState := ""
				if exchDev != nil {
					pattern = exchDev.Pattern
					configState = exchDev.Config.State
				}
				status_string := contents.AgentUpgrade.Status
				if status_string == "" {
					status_string = exchangecommon.STATUS_UNKNOWN
				}
				if contents.AgentUpgrade.ErrorMessage != "" {
					status_string += fmt.Sprintf(", ErrorMessage: %v", contents.AgentUpgrade.ErrorMessage)
				}
				eventlog.LogNodeEvent(n.db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_NMP_STATUS_CHANGED, policyName, status_string), persistence.EC_NMP_STATUS_CHANGED, exchange.GetId(n.GetExchangeId()), exchange.GetOrg(n.GetExchangeId()), pattern, configState)
			}
		}
	}
	return nil
}

// Check if the current agent versions are up to date for software, cert and config according to
// the specification of the nmp. The NMP must have at least one 'latest' as the version string.
func IsAgentUpToDate(status *exchangecommon.NodeManagementPolicyStatus, exchAFVs *exchangecommon.AgentFileVersions, db *bolt.DB) (bool, error) {
	// get local device info
	dev, err := persistence.FindExchangeDevice(db)
	if err != nil || dev == nil {
		return false, fmt.Errorf("Failed to get device from the local db: %v", err)
	}

	if exchAFVs != nil {
		// check software version
		if status.AgentUpgradeInternal.LatestMap.SoftwareLatest {
			versions := exchAFVs.SoftwareVersions
			if !IsVersionLatest(versions, version.HORIZON_VERSION) {
				return false, nil
			}
		}
		// check config version
		if status.AgentUpgradeInternal.LatestMap.ConfigLatest {
			versions := exchAFVs.ConfigVersions

			devConfigVer := ""
			if dev.SoftwareVersions != nil {
				if ver, ok := dev.SoftwareVersions[persistence.CONFIG_VERSION]; ok {
					devConfigVer = ver
				}
			}

			if !IsVersionLatest(versions, devConfigVer) {
				return false, nil
			}
		}
		// check certificate version
		if status.AgentUpgradeInternal.LatestMap.CertLatest {
			versions := exchAFVs.CertVersions

			devCertVer := ""
			if dev.SoftwareVersions != nil {
				if ver, ok := dev.SoftwareVersions[persistence.CERT_VERSION]; ok {
					devCertVer = ver
				}
			}

			if !IsVersionLatest(versions, devCertVer) {
				return false, nil
			}
		}
	}
	return true, nil
}

// Compare status.UpgradedVersions with the AgentFileVersions.
// It returns true if all the versions are up to date. This means
// that the nmp has been processed before with the latest versions.
func IsLatestVersionHandled(status *exchangecommon.NodeManagementPolicyStatus, exchAFVs *exchangecommon.AgentFileVersions) (bool, error) {

	// not handled
	if status.AgentUpgrade == nil {
		return false, nil
	}

	upgradedVersions := status.AgentUpgrade.UpgradedVersions

	if exchAFVs != nil {
		// check software version
		if status.AgentUpgradeInternal.LatestMap.SoftwareLatest {
			versions := exchAFVs.SoftwareVersions
			if !IsVersionLatest(versions, upgradedVersions.SoftwareVersion) {
				return false, nil
			}
		}
		// check config version
		if status.AgentUpgradeInternal.LatestMap.ConfigLatest {
			versions := exchAFVs.ConfigVersions
			if !IsVersionLatest(versions, upgradedVersions.ConfigVersion) {
				return false, nil
			}
		}
		// check certificate version
		if status.AgentUpgradeInternal.LatestMap.CertLatest {
			versions := exchAFVs.CertVersions
			if !IsVersionLatest(versions, upgradedVersions.CertVersion) {
				return false, nil
			}
		}
	}
	return true, nil
}

// check if current version is the latest available version. If the number of
// available versions is zero, the current version is considered the latest.
func IsVersionLatest(availibleVers []string, currentVersion string) bool {
	if availibleVers != nil && len(availibleVers) != 0 {
		sort.Slice(availibleVers, func(i, j int) bool {
			comp, _ := semanticversion.CompareVersions(availibleVers[i], availibleVers[j])
			return comp > 0
		})

		return currentVersion == availibleVers[0]
	}
	return true
}

// Check all nmp statuses that specify "latest" for a version, if status is not "downloaded", "download started" or "initiated", then change to "waiting" as there is a new version availible
// If there is no new version for whatever the status has "latest" for, it will be marked successful without executing
func (n *NodeManagementWorker) HandleAgentFilesVersionChange(cmd *AgentFileVersionChangeCommand) {
	glog.V(3).Infof(nmwlog(fmt.Sprintf("HandleAgentFilesVersionChange re-evaluating NMPs that request the 'latest' versions.")))
	if latestStatuses, err := persistence.FindNMPWithLatestKeywordVersion(n.db); err != nil {
		glog.Errorf(nmwlog(fmt.Sprintf("Error getting nmp statuses from db to change to \"waiting\". Error was: %v", err)))
		return
	} else {
		// get agent file versions
		exchAFVs, err := exchange.GetNodeUpgradeVersionsHandler(n)()
		if err != nil {
			glog.Errorf("Failed to get the AgentFileVersion from the exchange. %v", err)
			return
		}

		needDeferCommand := false
		for statusName, status := range latestStatuses {
			setStatusToWaiting := false
			nmpStatus := status.AgentUpgrade.Status
			if nmpStatus == exchangecommon.STATUS_NEW {
				glog.V(3).Infof(nmwlog(fmt.Sprintf("The nmp %v is already in 'waiting' status. do nothing.", statusName)))
				continue
			} else if nmpStatus == exchangecommon.STATUS_DOWNLOADED || nmpStatus == exchangecommon.STATUS_DOWNLOAD_STARTED || nmpStatus == exchangecommon.STATUS_INITIATED || nmpStatus == exchangecommon.STATUS_ROLLBACK_STARTED {
				glog.V(3).Infof(nmwlog(fmt.Sprintf("The nmp %v with latest keyword is currently being executed or downloaded (status is %v). Exiting without changing status to \"waiting\", checking this nmp later", statusName, nmpStatus)))
				needDeferCommand = true
			} else if nmpStatus == exchangecommon.STATUS_DOWNLOAD_FAILED || nmpStatus == exchangecommon.STATUS_FAILED_JOB || nmpStatus == exchangecommon.STATUS_PRECHECK_FAILED || nmpStatus == exchangecommon.STATUS_ROLLBACK_FAILED || nmpStatus == exchangecommon.STATUS_ROLLBACK_SUCCESSFUL {
				if isHandled, err := IsLatestVersionHandled(status, exchAFVs); err != nil {
					glog.Errorf(nmwlog(fmt.Sprintf("Error checking if the latest versions are previously handled for nmp %v. %v", statusName, err)))
				} else if isHandled {
					glog.V(3).Infof(nmwlog(fmt.Sprintf("The latest agent versions are previously handled for nmp %v. The status was %v. Exiting without changing status to \"waiting\".", statusName, nmpStatus)))
				} else {
					setStatusToWaiting = true
				}
			} else {
				if isUpToDate, err := IsAgentUpToDate(status, exchAFVs, n.db); err != nil {
					glog.Errorf(nmwlog(fmt.Sprintf("Error checking if the agent versions are up to date for nmp %v. %v", statusName, err)))
				} else if isUpToDate {
					glog.V(3).Infof(nmwlog(fmt.Sprintf("The agent versions are up to date for nmp %v. Exiting without changing status to \"waiting\".", statusName)))
				} else {
					setStatusToWaiting = true
				}
			}

			// set the status to waiting for this nmp
			if setStatusToWaiting {
				glog.V(3).Infof(nmwlog(fmt.Sprintf("Change status to \"waiting\" for the nmp %v", statusName)))

				// Add startWindow to current time to randomize upgrade start times just like what occurs when an NMP first executes
				if status.TimeToStart() {
					nmp, err := persistence.FindNodeManagementPolicy(n.db, statusName)
					if err != nil {
						glog.Errorf(nmwlog(fmt.Sprintf("Error getting nmp from db to check the startWindow value. Error was: %v", err)))
					}
					if nmp != nil {
						status.SetScheduledStartTime(exchangecommon.TIME_NOW_KEYWORD, nmp.LastUpdated, nmp.UpgradeWindowDuration)
					}
				}

				status.AgentUpgrade.Status = exchangecommon.STATUS_NEW
				err = n.UpdateStatus(statusName, status, exchange.GetPutNodeManagementPolicyStatusHandler(n), persistence.NewMessageMeta(EL_NMP_STATUS_CHANGED, statusName, exchangecommon.STATUS_NEW), persistence.EC_NMP_STATUS_UPDATE_NEW)
				if err != nil {
					glog.Errorf(nmwlog(fmt.Sprintf("Error changing nmp status for %v to \"waiting\". Error was %v.", statusName, err)))
				}
			}

		} // end for

		if needDeferCommand && cmd != nil {
			n.AddDeferredCommand(cmd)
		}
	}
}

// This function gets all the 'reset' nmp status from the exchange and set them to
// 'waiting' so that the agent can start re-evaluating them.
func (w *NodeManagementWorker) HandleNmpStatusReset() {
	glog.V(3).Infof(nmwlog(fmt.Sprintf("HandleNmpStatusReset re-evaluating NMPs that has the status 'reset'.")))

	// get all the nmps that applies to this node from the exchange
	allNmpStatus, err := exchange.GetNodeManagementAllStatuses(w, exchange.GetOrg(w.GetExchangeId()), exchange.GetId(w.GetExchangeId()))
	if err != nil {
		glog.Errorf(nmwlog(fmt.Sprintf("Error getting all nmp statuses for node %v from the exchange. %v", w.GetExchangeId(), err)))
	} else {
		glog.V(5).Infof(nmwlog(fmt.Sprintf("GetNodeManagementAllStatuses returns: %v", allNmpStatus)))
	}

	// find all nmp status from local db
	allLocalStatuses, err := persistence.FindAllNMPStatus(w.db)
	if err != nil {
		glog.Errorf(nmwlog(fmt.Sprintf("Error getting all nmp statuses from the local database. %v", err)))
	}

	// change the status to 'waiting'
	if allNmpStatus != nil {
		for nmp_name, nmp_status := range allNmpStatus.PolicyStatuses {
			if nmp_status.Status() == exchangecommon.STATUS_RESET {
				if local_status, ok := allLocalStatuses[nmp_name]; ok {
					glog.V(3).Infof(nmwlog(fmt.Sprintf("Change status from \"reset\" to \"waiting\" for the nmp %v", nmp_name)))

					local_status.AgentUpgrade.Status = exchangecommon.STATUS_NEW
					if local_status.AgentUpgradeInternal != nil {
						local_status.AgentUpgradeInternal.DownloadAttempts = 0
					}

					err = w.UpdateStatus(nmp_name, local_status, exchange.GetPutNodeManagementPolicyStatusHandler(w), persistence.NewMessageMeta(EL_NMP_STATUS_CHANGED, nmp_name, exchangecommon.STATUS_NEW), persistence.EC_NMP_STATUS_UPDATE_NEW)
					if err != nil {
						glog.Errorf(nmwlog(fmt.Sprintf("Error changing nmp status for %v from \"reset\" to \"waiting\". Error was %v.", nmp_name, err)))
					}
				} else {
					glog.V(3).Infof(nmwlog(fmt.Sprintf("node management status for nmp %v for node %v is set to \"reset\" but the status cannot be found from the local db. Skiping it.", nmp_name, w.GetExchangeId())))
				}
			}
		}
	}
}
