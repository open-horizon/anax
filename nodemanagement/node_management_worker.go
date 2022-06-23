package nodemanagement

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/common"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/eventlog"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/worker"
	"os"
	"path"
	"strings"
	"sync"
)

const STATUS_FILE_NAME = "status.json"
const NMP_MONITOR = "NMPMonitor"

var statusUpdateLock sync.Mutex

type NodeManagementWorker struct {
	worker.BaseWorker
	db *bolt.DB
}

func NewNodeManagementWorker(name string, config *config.HorizonConfig, db *bolt.DB) *NodeManagementWorker {
	ec := getEC(config, db)

	worker := &NodeManagementWorker{
		BaseWorker: worker.NewBaseWorker(name, config, ec),
		db:         db,
	}

	glog.Infof(nmwlog(fmt.Sprintf("Starting Node Management Worker %v", worker.EC)))
	worker.Start(worker, 0)
	return worker
}

func (w *NodeManagementWorker) Initialize() bool {
	w.DispatchSubworker(NMP_MONITOR, w.checkNMPTimeToRun, 60, false)

	if dev, _ := persistence.FindExchangeDevice(w.db); dev != nil && dev.Config.State == persistence.CONFIGSTATE_CONFIGURED {
		// Node is registered. Check nmp's in exchange, statuses in db
		workingDir := w.Config.Edge.GetNodeMgmtDirectory()
		if err := w.ProcessAllNMPS(workingDir, exchange.GetAllExchangeNodeManagementPoliciesHandler(w), exchange.GetDeleteNodeManagementPolicyStatusHandler(w), exchange.GetPutNodeManagementPolicyStatusHandler(w), exchange.GetAllNodeManagementPolicyStatusHandler(w)); err != nil {
			glog.Errorf(nmwlog(fmt.Sprintf("Error processing all exchange policies: %v", err)))
		}

		if dev.IsEdgeCluster() {
			// Check if a nmp process completed
			if err := w.CheckNMPStatus(workingDir, STATUS_FILE_NAME); err != nil {
				glog.Errorf(nmwlog(fmt.Sprintf("Failed to collect status. error: %v", err)))
			}
		}

		// Set any statuses in "download started" status back to waiting so the download will be retried
		if err := w.ResetDownloadStartedStatuses(); err != nil {
			glog.Errorf(nmwlog(fmt.Sprintf("Failed to reset nmp statuses in \"download started\" status back to \"waiting\".")))
		}
	}

	return true
}

// this  is the function for a subworker that monitors the waiting nmps
// when it finds an nmp status with a scheduled time that has passed it will send a start download message
// if it finds multiple statuses that have passed, it will send start download messages in order of latest to earliest scheduled start time
// this is to ensure that a newly registered node will reach the same state as a node that has been registered as policies were added
// the node will only run one upgrade per type unless the most recent nmp was a downgrade from a previous nmp
func (w *NodeManagementWorker) checkNMPTimeToRun() int {
	glog.Infof(nmwlog("Starting run of node management policy monitoring subworker."))
	statusUpdateLock.Lock()
	defer statusUpdateLock.Unlock()

	exchDev, err := persistence.FindExchangeDevice(w.db)
	if err != nil {
		glog.Errorf(nmwlog(fmt.Sprintf("Error getting device from database: %v", err)))
		return 60
	}

	if exchDev == nil || exchDev.Config.State != persistence.CONFIGSTATE_CONFIGURED {
		glog.Infof(nmwlog(fmt.Sprintf("Node is not configured.")))
		return 60
	}
	if downloadedInitiatedStatuses, err := persistence.FindNMPSWithStatuses(w.db, []string{exchangecommon.STATUS_DOWNLOADED, exchangecommon.STATUS_INITIATED, exchangecommon.STATUS_DOWNLOAD_STARTED}); err != nil {
		glog.Errorf(nmwlog(fmt.Sprintf("Failed to get nmp statuses from the database. Error was %v", err)))
	} else if len(downloadedInitiatedStatuses) > 0 {
		glog.Infof(nmwlog("There is an nmp currently being executed or downloaded. Exiting without looking for the next nmp to run."))
		return 60
	}
	if waitingNMPs, err := persistence.FindWaitingNMPStatuses(w.db); err != nil {
		glog.Errorf(nmwlog(fmt.Sprintf("Failed to get nmp statuses from the database. Error was %v", err)))
	} else {
		earliestNmpName := "initial"
		earliestNmpStatus := &exchangecommon.NodeManagementPolicyStatus{}
		for earliestNmpName != "" {
			earliestNmpName, earliestNmpStatus = getLatest(&waitingNMPs)
			if earliestNmpName != "" {
				glog.Infof(nmwlog(fmt.Sprintf("Time to start nmp %v", earliestNmpName)))
				earliestNmpStatus.AgentUpgrade.Status = exchangecommon.STATUS_DOWNLOAD_STARTED
				err = w.UpdateStatus(earliestNmpName, earliestNmpStatus, exchange.GetPutNodeManagementPolicyStatusHandler(w), persistence.NewMessageMeta(EL_NMP_STATUS_CHANGED, earliestNmpName, exchangecommon.STATUS_DOWNLOAD_STARTED), persistence.EC_NMP_STATUS_UPDATE_NEW)
				if err != nil {
					glog.Errorf(nmwlog(fmt.Sprintf("Failed to update nmp status %v: %v", earliestNmpName, err)))
				}
				w.Messages() <- events.NewNMPStartDownloadMessage(events.NMP_START_DOWNLOAD, events.StartDownloadMessage{NMPStatus: earliestNmpStatus, NMPName: earliestNmpName})
				return 60
			}
		}
	}
	return 60
}

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

func getEC(config *config.HorizonConfig, db *bolt.DB) *worker.BaseExchangeContext {
	var ec *worker.BaseExchangeContext
	if dev, _ := persistence.FindExchangeDevice(db); dev != nil {
		ec = worker.NewExchangeContext(fmt.Sprintf("%v/%v", dev.Org, dev.Id), dev.Token, config.Edge.ExchangeURL, config.GetCSSURL(), config.Collaborators.HTTPClientFactory)
	}

	return ec
}

func (w *NodeManagementWorker) Messages() chan events.Message {
	return w.BaseWorker.Manager.Messages
}

// When the node is started, need to remove the nmps from the db (incase registered in a new org), get and process all nmps from the exchange
// Then check for any status files left by update processes
func (n *NodeManagementWorker) HandleRegistration() {
	n.EC = getEC(n.Config, n.db)
	glog.Infof(nmwlog("Initializing"))
	workingDir := n.Config.Edge.GetNodeMgmtDirectory()
	if err := n.ProcessAllNMPS(workingDir, exchange.GetAllExchangeNodeManagementPoliciesHandler(n), exchange.GetDeleteNodeManagementPolicyStatusHandler(n), exchange.GetPutNodeManagementPolicyStatusHandler(n), exchange.GetAllNodeManagementPolicyStatusHandler(n)); err != nil {
		glog.Errorf(nmwlog(fmt.Sprintf("Error processing all exchange policies: %v", err)))

		return
	}
	return
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
	if cmd.Msg.Status == exchangecommon.STATUS_DOWNLOADED {
		glog.Infof(nmwlog(fmt.Sprintf("Sucessfully downloaded packages for nmp %v.", cmd.Msg.NMPName)))
		status.SetStatus(exchangecommon.STATUS_DOWNLOADED)
		msgMeta = persistence.NewMessageMeta(EL_NMP_STATUS_CHANGED, cmd.Msg.NMPName, exchangecommon.STATUS_DOWNLOADED)
		eventCode = persistence.EC_NMP_STATUS_DOWNLOAD_SUCCESSFUL
	} else if cmd.Msg.Status == exchangecommon.STATUS_NO_ACTION {
		glog.Infof(nmwlog(fmt.Sprintf("Already in compliance with nmp %v. Download skipped.", cmd.Msg.NMPName)))
		status.SetStatus(exchangecommon.STATUS_NO_ACTION)
		msgMeta = persistence.NewMessageMeta(EL_NMP_STATUS_CHANGED, cmd.Msg.NMPName, exchangecommon.STATUS_NO_ACTION)
		eventCode = persistence.EC_NMP_STATUS_DOWNLOAD_SUCCESSFUL
	} else {
		glog.Infof(nmwlog(fmt.Sprintf("Failed to download packages for nmp %v.", cmd.Msg.NMPName)))
		status.SetStatus(cmd.Msg.Status)
		msgMeta = persistence.NewMessageMeta(EL_NMP_STATUS_CHANGED, cmd.Msg.NMPName, exchangecommon.STATUS_DOWNLOAD_FAILED)
		eventCode = persistence.EC_NMP_STATUS_DOWNLOAD_FAILED
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

func (n *NodeManagementWorker) CommandHandler(command worker.Command) bool {
	glog.Infof(nmwlog(fmt.Sprintf("Handling command %v", command)))
	switch command.(type) {
	case *NodeRegisteredCommand:
		n.HandleRegistration()
	case *NodeConfiguredCommand:
		err := n.ProcessAllNMPS(n.Config.Edge.GetNodeMgmtDirectory(), exchange.GetAllExchangeNodeManagementPoliciesHandler(n), exchange.GetDeleteNodeManagementPolicyStatusHandler(n), exchange.GetPutNodeManagementPolicyStatusHandler(n), exchange.GetAllNodeManagementPolicyStatusHandler(n))
		if err != nil {
			glog.Errorf(nmwlog(fmt.Sprintf(err.Error())))
		}
	case *NMPDownloadCompleteCommand:
		cmd := command.(*NMPDownloadCompleteCommand)
		n.DownloadComplete(cmd)
	case *NodeShutdownCommand:
		n.TerminateSubworkers()
		n.HandleUnregister()
	case *NMPChangeCommand:
		err := n.ProcessAllNMPS(n.Config.Edge.GetNodeMgmtDirectory(), exchange.GetAllExchangeNodeManagementPoliciesHandler(n), exchange.GetDeleteNodeManagementPolicyStatusHandler(n), exchange.GetPutNodeManagementPolicyStatusHandler(n), exchange.GetAllNodeManagementPolicyStatusHandler(n))
		if err != nil {
			glog.Errorf(nmwlog(fmt.Sprintf(err.Error())))
		}
	case *NodePolChangeCommand:
		err := n.ProcessAllNMPS(n.Config.Edge.GetNodeMgmtDirectory(), exchange.GetAllExchangeNodeManagementPoliciesHandler(n), exchange.GetDeleteNodeManagementPolicyStatusHandler(n), exchange.GetPutNodeManagementPolicyStatusHandler(n), exchange.GetAllNodeManagementPolicyStatusHandler(n))
		if err != nil {
			glog.Errorf(nmwlog(fmt.Sprintf(err.Error())))
		}
	case *AgentFileVersionChangeCommand:
		cmd := command.(*AgentFileVersionChangeCommand)
		n.HandleAgentFilesVersionChange(cmd)
	default:
		return false
	}
	return true
}

func (n *NodeManagementWorker) NoWorkHandler() {
	if n.IsWorkerShuttingDown() {
		if n.AreAllSubworkersTerminated() {
			glog.Infof(nmwlog(fmt.Sprintf("NMPWorker initiating shutdown.")))

			n.SetWorkerShuttingDown(0, 0)
		}
	}
}

// This process runs after a changes to the exchange NMPS or the node's policy, when the node is registered or starts up if it is already registered
// The function will validate that there is a status for all nmp's the node matches and that an nmp exists in the exchange and matches this node for every status in the node's db
func (n *NodeManagementWorker) ProcessAllNMPS(baseWorkingFile string, getAllNMPS exchange.AllNodeManagementPoliciesHandler, deleteNMPStatus exchange.DeleteNodeManagementPolicyStatusHandler, putNMPStatus exchange.PutNodeManagementPolicyStatusHandler, getNMPStatus exchange.AllNodeManagementPolicyStatusHandler) error {
	/*
		Get all the policies  from  the exchange
		Check  compatibility
		if compatible
			check if status exists
			if exists
				done
			else
				create status
				update exchange status
		for each status
			if not in the matching exchange nmps
				delete status
	*/
	glog.Infof(nmwlog("Starting to process all nmps in the exchange and locally."))

	statusUpdateLock.Lock()
	defer statusUpdateLock.Unlock()

	nodeOrg := exchange.GetOrg(n.GetExchangeId())
	allNMPs, err := getAllNMPS(nodeOrg)
	if err != nil {
		return fmt.Errorf("Error getting node management policies from the exchange: %v", err)
	}
	nodePol, err := persistence.FindNodePolicy(n.db)
	if err != nil {
		return fmt.Errorf("Error getting node's policy to check management policy compatibility: %v", err)
	}
	nodeMgmtPol := &externalpolicy.ExternalPolicy{}
	if nodePol != nil {
		nodeMgmtPol = nodePol.GetManagementPolicy()
	}

	exchDev, err := persistence.FindExchangeDevice(n.db)
	if err != nil {
		return fmt.Errorf("Error getting device from database: %v", err)
	}

	if exchDev == nil || exchDev.Config.State != persistence.CONFIGSTATE_CONFIGURED {
		return fmt.Errorf("Node is not configured.")
	}

	nodePattern := exchDev.Pattern
	matchingNMPs := map[string]exchangecommon.ExchangeNodeManagementPolicy{}

	for name, policy := range *allNMPs {
		if match, _ := VerifyCompatible(nodeMgmtPol, nodePattern, &policy); match {
			matchingNMPs[name] = policy
			org, nodeId := cutil.SplitOrgSpecUrl(n.GetExchangeId())
			glog.Infof(nmwlog(fmt.Sprintf("Found matching node management policy %v in the exchange.", name)))
			if !policy.Enabled {
				existingStatus, err := persistence.DeleteNMPStatus(n.db, name)
				if err != nil {
					glog.Errorf(nmwlog(fmt.Sprintf("Failed to delete status for deactivated node policy %v from database. Error was %v", name, err)))
				}
				if existingStatus != nil {
					if err := deleteNMPStatus(org, nodeId, name); err != nil {
						glog.Errorf(nmwlog(fmt.Sprintf("Error removing status %v from exchange", name)))
					}
				}
			} else {
				if err = persistence.SaveOrUpdateNodeManagementPolicy(n.db, name, policy); err != nil {
					glog.Errorf(nmwlog(fmt.Sprintf("Error saving node management policy %v in the database: %v", name, err)))
				}
				existingStatus, err := persistence.FindNMPStatus(n.db, name)
				if err != nil {
					glog.Errorf(nmwlog(fmt.Sprintf("Error getting status for policy %v from the database: %v", name, err)))
				} else if existingStatus == nil {
					glog.Infof(nmwlog(fmt.Sprintf("Saving node management policy status %v in the db.", name)))
					newStatus := exchangecommon.StatusFromNewPolicy(policy, baseWorkingFile)
					if err = n.UpdateStatus(name, &newStatus, putNMPStatus, persistence.NewMessageMeta(EL_NMP_STATUS_CREATED, name), persistence.EC_NMP_STATUS_UPDATE_NEW); err != nil {
						glog.Errorf(nmwlog(fmt.Sprintf("Failed to update status for %v: %v", name, err)))
					}
				}
			}
		}
	}

	// get all the statuses for this node from the exchange in case they were not removed correctly at unregister
	org, nodeId := cutil.SplitOrgSpecUrl(n.GetExchangeId())
	allExStatusesResp, _ := getNMPStatus(org, nodeId)
	allExStatuses := allExStatusesResp.PolicyStatuses

	if allStatuses, err := persistence.FindAllNMPStatus(n.db); err != nil {
		return err
	} else {
		for statusName, _ := range allStatuses {
			if _, ok := matchingNMPs[statusName]; !ok {
				// The nmp this status is for is no longer in the exchange. Delete it
				glog.Infof(nmwlog(fmt.Sprintf("Removing status %v from the local database and exchange as the nmp no longer exists in the exchange or no longer matches this node.", statusName)))
				existingStatus, err := persistence.DeleteNMPStatus(n.db, statusName)
				if err != nil {
					glog.Errorf(nmwlog(fmt.Sprintf("Error removing status %v from database", statusName)))
				} else if existingStatus != nil {
					if err := deleteNMPStatus(org, nodeId, statusName); err != nil {
						glog.Errorf(nmwlog(fmt.Sprintf("Error removing status %v from exchange", statusName)))
					}
				}
			}
			delete(allExStatuses, statusName)
		}
		for statusName, _ := range allExStatuses {
			// these statuses are in the exchange but not the local db. remove them from the exchange
			if err := deleteNMPStatus(org, nodeId, statusName); err != nil {
				glog.Errorf(nmwlog(fmt.Sprintf("Error removing status %v from exchange", statusName)))
			}
		}
	}
	return nil
}

func (n *NodeManagementWorker) NewEvent(incoming events.Message) {
	glog.Infof(nmwlog(fmt.Sprintf("Handling event: %v", incoming)))
	switch incoming.(type) {
	case *events.EdgeRegisteredExchangeMessage:
		msg, _ := incoming.(*events.EdgeRegisteredExchangeMessage)

		switch msg.Event().Id {
		case events.NEW_DEVICE_REG:
			cmd := NewNodeRegisteredCommand(msg)
			n.Commands <- cmd
		}
	case *events.EdgeConfigCompleteMessage:
		msg, _ := incoming.(*events.EdgeConfigCompleteMessage)

		switch msg.Event().Id {
		case events.NEW_DEVICE_CONFIG_COMPLETE:
			cmd := NewNodeConfiguredCommand(msg)
			n.Commands <- cmd
		}
	case *events.NodeShutdownCompleteMessage:
		msg, _ := incoming.(*events.NodeShutdownCompleteMessage)
		switch msg.Event().Id {
		case events.UNCONFIGURE_COMPLETE:
			n.Commands <- worker.NewTerminateCommand("shutdown")
		}
	case *events.NodeShutdownMessage:
		msg, _ := incoming.(*events.NodeShutdownMessage)
		cmd := NewNodeShutdownCommand(msg)
		n.Commands <- cmd
	case *events.NMPDownloadCompleteMessage:
		msg, _ := incoming.(*events.NMPDownloadCompleteMessage)

		switch msg.Event().Id {
		case events.NMP_DOWNLOAD_COMPLETE:
			cmd := NewNMPDownloadCompleteCommand(msg)
			n.Commands <- cmd
		}
	case *events.ExchangeChangeMessage:
		msg, _ := incoming.(*events.ExchangeChangeMessage)
		switch msg.Event().Id {
		case events.CHANGE_NMP_TYPE:
			n.Commands <- NewNMPChangeCommand(msg)
		case events.CHANGE_NODE_POLICY_TYPE:
			n.Commands <- NewNodePolChangeCommand(msg)
		case events.CHANGE_AGENT_FILE_VERSION:
			n.Commands <- NewAgentFileVersionChangeCommand(msg)
		}
	}
}

func VerifyCompatible(nodePol *externalpolicy.ExternalPolicy, nodePattern string, nmPol *exchangecommon.ExchangeNodeManagementPolicy) (bool, error) {
	if nodePattern != "" || len(nmPol.Patterns) > 0 {
		if cutil.SliceContains(nmPol.Patterns, nodePattern) {
			return true, nil
		}
		patternPieces := strings.SplitN(nodePattern, "/", 2)
		if len(patternPieces) > 1 {
			return cutil.SliceContains(nmPol.Patterns, strings.SplitN(nodePattern, "/", 2)[1]), nil
		}
		return false, nil
	}
	if nodePol == nil || nmPol == nil {
		return false, nil
	}
	if err := nodePol.Constraints.IsSatisfiedBy(nmPol.Properties); err != nil {
		return false, err
	} else if err = nmPol.Constraints.IsSatisfiedBy(nodePol.Properties); err != nil {
		return false, err
	} else {
		return true, nil
	}
}

func (n *NodeManagementWorker) CheckNMPStatus(baseWorkingFile string, statusFileName string) error {
	/*
		Check working dir for folders
		for each folder
			collect status
			update exchange status
			if sucessfull
				remove folder
	*/
	if statuses, err := persistence.FindInitiatedNMPStatuses(n.db); err != nil {
		return fmt.Errorf("Failed to find nmp statuses in the local db: %v", err)
	} else {
		for name, status := range statuses {
			if err = n.CollectStatus(baseWorkingFile, name, status); err != nil {
				glog.Infof(nmwlog(fmt.Sprintf("Failed to collect status for nmp %v: %v", name, err)))
			}
		}
	}
	return nil
}

// Read and  persist the status out of the  file
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
				eventlog.LogNodeEvent(n.db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_NMP_STATUS_CHANGED, policyName, status_string), persistence.EC_NMP_STATUS_UPDATE_NEW, exchange.GetId(n.GetExchangeId()), exchange.GetOrg(n.GetExchangeId()), pattern, configState)
			}
		}
	}
	return nil
}

// Update a given nmp status in the db and the exchange
func (n *NodeManagementWorker) UpdateStatus(policyName string, status *exchangecommon.NodeManagementPolicyStatus, putStatusHandler exchange.PutNodeManagementPolicyStatusHandler, eventLogMessageMeta *persistence.MessageMeta, eventCode string) error {
	org, nodeId := cutil.SplitOrgSpecUrl(n.GetExchangeId())
	pattern := ""
	configState := ""
	exchDev, err := persistence.FindExchangeDevice(n.db)
	if err != nil {
		glog.Errorf(nmwlog(fmt.Sprintf("Error getting device from database: %v", err)))
	} else if exchDev != nil {
		pattern = exchDev.Pattern
		configState = exchDev.Config.State
	}
	eventlog.LogNodeEvent(n.db, persistence.SEVERITY_INFO, eventLogMessageMeta, eventCode, nodeId, org, pattern, configState)
	if err := persistence.SaveOrUpdateNMPStatus(n.db, policyName, *status); err != nil {
		return err
	}
	if _, err := putStatusHandler(org, nodeId, policyName, status); err != nil {
		return fmt.Errorf("Failed to put node management policy status for policy %v to the exchange: %v", policyName, err)
	}
	return nil
}

// Check all nmp statuses that specify "latest" for a version, if status is not "downloaded", "download started" or "initiated", then change to "waiting" as there is a new version availible
// If there is no new version for whatever the status has "latest" for, it will be marked successful without executing
func (n *NodeManagementWorker) HandleAgentFilesVersionChange(cmd *AgentFileVersionChangeCommand) {
	if latestStatuses, err := persistence.FindNMPWithLatestKeywordVersion(n.db); err != nil {
		glog.Errorf(nmwlog(fmt.Sprintf("Error getting nmp statuses from db to change to \"waiting\". Error was: %v", err)))
		return
	} else {
		needDeferCommand := false
		for statusName, status := range latestStatuses {
			if status.AgentUpgrade.Status == exchangecommon.STATUS_DOWNLOADED || status.AgentUpgrade.Status == exchangecommon.STATUS_DOWNLOAD_STARTED || status.AgentUpgrade.Status == exchangecommon.STATUS_INITIATED || status.AgentUpgrade.Status == exchangecommon.STATUS_ROLLBACK_STARTED {
				glog.Infof(nmwlog(fmt.Sprintf("The nmp %v with latest keyword is currently being executed or downloaded (status is %v). Exiting without changing status to \"waiting\", checking this nmp later", statusName, status.AgentUpgrade.Status)))
				needDeferCommand = true
			} else {
				glog.Infof(nmwlog(fmt.Sprintf("Change status to \"waiting\" for the nmp %v", statusName)))
				status.AgentUpgrade.Status = exchangecommon.STATUS_NEW
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
				err = n.UpdateStatus(statusName, status, exchange.GetPutNodeManagementPolicyStatusHandler(n), persistence.NewMessageMeta(EL_NMP_STATUS_CHANGED, statusName, exchangecommon.STATUS_NEW), persistence.EC_NMP_STATUS_UPDATE_NEW)
				if err != nil {
					glog.Errorf(nmwlog(fmt.Sprintf("Error changing nmp status for %v to \"waiting\". Error was %v.", statusName, err)))
				}
			}
		}

		if needDeferCommand {
			n.AddDeferredCommand(cmd)
		}
	}
}

func nmwlog(message string) string {
	return fmt.Sprintf("Node management worker: %v", message)
}

// This will remove nmps and statuses from the local db
func (n *NodeManagementWorker) HandleUnregister() {
	if err := persistence.DeleteAllNodeManagementPolicies(n.db); err != nil {
		glog.Errorf(nmwlog(fmt.Sprintf("Error removing node management policies from the local db: %v", err)))
	}
	if err := persistence.DeleteAllNMPStatuses(n.db); err != nil {
		glog.Errorf(nmwlog(fmt.Sprintf("Error removing node management policy statuses from the local db: %v", err)))
	}
}
