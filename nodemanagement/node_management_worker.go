package nodemanagement

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
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
)

const STATUS_FILE_NAME = "status.json"
const NMP_MONITOR = "NMPMonitor"

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

	glog.Info(nmwlog(fmt.Sprintf("Starting Node Management Worker %v", worker.EC)))
	worker.Start(worker, 0)
	return worker
}

func (w *NodeManagementWorker) Initialize() bool {
	w.DispatchSubworker(NMP_MONITOR, w.checkNMPTimeToRun, 60, false)

	if dev, _ := persistence.FindExchangeDevice(w.db); dev != nil && dev.Config.State == persistence.CONFIGSTATE_CONFIGURED {
		// Node is registered. Check nmp's in exchange, statuses in db
		workingDir := w.Config.Edge.GetNodeMgmtDirectory()
		if err := w.ProcessAllNMPS(workingDir, exchange.GetAllExchangeNodeManagementPoliciesHandler(w), exchange.GetDeleteNodeManagementPolicyStatusHandler(w), exchange.GetPutNodeManagementPolicyStatusHandler(w)); err != nil {
			glog.Errorf(nmwlog(fmt.Sprintf("Error processing all exchange policies: %v", err)))
		}
		// Check if a nmp process completed
		if err := w.CheckNMPStatus(workingDir, STATUS_FILE_NAME); err != nil {
			glog.Errorf(nmwlog(fmt.Sprintf("Failed to collect status. error: %v", err)))
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
// if it finds multiple statuses that have passed, it will send start download messages in order of earliest to latest scheduled start time
// this is to ensure that a newly registered node will reach the same state as a node that has been registered as policies were added
func (w *NodeManagementWorker) checkNMPTimeToRun() int {
	glog.Infof(nmwlog("Starting run of node management policy monitoring subworker."))
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
			earliestNmpName, earliestNmpStatus = getEarliest(&waitingNMPs)
			if earliestNmpName != "" {
				glog.Infof(nmwlog(fmt.Sprintf("Time to start nmp %v", earliestNmpName)))
				earliestNmpStatus.AgentUpgrade.Status = exchangecommon.STATUS_DOWNLOAD_STARTED
				err = w.UpdateStatus(earliestNmpName, earliestNmpStatus, exchange.GetPutNodeManagementPolicyStatusHandler(w))
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
		if err := w.UpdateStatus(statusName, status, exchange.GetPutNodeManagementPolicyStatusHandler(w)); err != nil {
			return err
		}
	}

	return nil
}

// this returns the name and status struct of the status with the eariest scheduled start time and deletes that earliest status from the map passed in
func getEarliest(statusMap *map[string]*exchangecommon.NodeManagementPolicyStatus) (string, *exchangecommon.NodeManagementPolicyStatus) {
	earliestNmpName := ""
	earliestNmpStatus := &exchangecommon.NodeManagementPolicyStatus{}
	if statusMap == nil {
		return "", nil
	}

	for nmpName, nmpStatus := range *statusMap {
		if nmpStatus != nil && nmpStatus.TimeToStart() {
			if earliestNmpName == "" || nmpStatus.AgentUpgradeInternal.ScheduledUnixTime.Before(earliestNmpStatus.AgentUpgradeInternal.ScheduledUnixTime) {
				earliestNmpStatus = nmpStatus
				earliestNmpName = nmpName
			}
		}
	}
	delete(*statusMap, earliestNmpName)
	return earliestNmpName, earliestNmpStatus
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
	if err := n.ProcessAllNMPS(workingDir, exchange.GetAllExchangeNodeManagementPoliciesHandler(n), exchange.GetDeleteNodeManagementPolicyStatusHandler(n), exchange.GetPutNodeManagementPolicyStatusHandler(n)); err != nil {
		glog.Errorf(nmwlog(fmt.Sprintf("Error processing all exchange policies: %v", err)))
		// uncomment the return once nmp status is in the exchange-api
		// return
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
	pattern := ""
	configState := ""
	exchDev, err := persistence.FindExchangeDevice(n.db)
	if err != nil {
		glog.Errorf(nmwlog(fmt.Sprintf("Error getting device from database: %v", err)))
	} else if exchDev != nil {
		pattern = exchDev.Pattern
		configState = exchDev.Config.State
	}
	if cmd.Msg.Status == exchangecommon.STATUS_DOWNLOADED {
		glog.Infof(nmwlog(fmt.Sprintf("Sucessfully downloaded packages for nmp %v.", cmd.Msg.NMPName)))
		status.SetStatus(exchangecommon.STATUS_DOWNLOADED)
		eventlog.LogNodeEvent(n.db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_NMP_STATUS_CHANGED, cmd.Msg.NMPName, exchangecommon.STATUS_DOWNLOADED), persistence.EC_NMP_STATUS_UPDATE_NEW, exchange.GetId(n.GetExchangeId()), exchange.GetOrg(n.GetExchangeId()), pattern, configState)
	} else if cmd.Msg.Status == exchangecommon.STATUS_SUCCESSFUL {
		glog.Infof(nmwlog(fmt.Sprintf("Already in compliance with nmp %v. Download skipped.", cmd.Msg.NMPName)))
		status.SetStatus(exchangecommon.STATUS_SUCCESSFUL)
		eventlog.LogNodeEvent(n.db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_NMP_STATUS_CHANGED, cmd.Msg.NMPName, exchangecommon.STATUS_SUCCESSFUL), persistence.EC_NMP_STATUS_UPDATE_NEW, exchange.GetId(n.GetExchangeId()), exchange.GetOrg(n.GetExchangeId()), pattern, configState)
	} else {
		glog.Infof(nmwlog(fmt.Sprintf("Failed to download packages for nmp %v.", cmd.Msg.NMPName)))
		status.SetStatus(cmd.Msg.Status)
		eventlog.LogNodeEvent(n.db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_NMP_STATUS_CHANGED, cmd.Msg.NMPName, exchangecommon.STATUS_DOWNLOAD_FAILED), persistence.EC_NMP_STATUS_UPDATE_NEW, exchange.GetId(n.GetExchangeId()), exchange.GetOrg(n.GetExchangeId()), pattern, configState)
	}
	if cmd.Msg.Versions != nil {
		status.AgentUpgrade.UpgradedVersions = *cmd.Msg.Versions
	}
	if cmd.Msg.Latests != nil {
		status.AgentUpgradeInternal.LatestMap = *cmd.Msg.Latests
	}
	err = n.UpdateStatus(cmd.Msg.NMPName, status, exchange.GetPutNodeManagementPolicyStatusHandler(n))
	if err != nil {
		glog.Errorf(nmwlog(fmt.Sprintf("Failed to update nmp status %v: %v", cmd.Msg.NMPName, err)))
	}
}

func (n *NodeManagementWorker) CommandHandler(command worker.Command) bool {
	glog.Infof(nmwlog(fmt.Sprintf("Handling command %v", command)))
	switch command.(type) {
	case *NodeRegisteredCommand:
		n.HandleRegistration()
	case *NMPDownloadCompleteCommand:
		cmd := command.(*NMPDownloadCompleteCommand)
		n.DownloadComplete(cmd)
	case *NodeShutdownCommand:
		n.TerminateSubworkers()
		n.HandleUnregister()
	case *NMPChangeCommand:
		n.ProcessAllNMPS(n.Config.Edge.GetNodeMgmtDirectory(), exchange.GetAllExchangeNodeManagementPoliciesHandler(n), exchange.GetDeleteNodeManagementPolicyStatusHandler(n), exchange.GetPutNodeManagementPolicyStatusHandler(n))
	case *NodePolChangeCommand:
		n.ProcessAllNMPS(n.Config.Edge.GetNodeMgmtDirectory(), exchange.GetAllExchangeNodeManagementPoliciesHandler(n), exchange.GetDeleteNodeManagementPolicyStatusHandler(n), exchange.GetPutNodeManagementPolicyStatusHandler(n))
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
func (n *NodeManagementWorker) ProcessAllNMPS(baseWorkingFile string, getAllNMPS exchange.AllNodeManagementPoliciesHandler, deleteNMPStatus exchange.DeleteNodeManagementPolicyStatusHandler, putNMPStatus exchange.PutNodeManagementPolicyStatusHandler) error {
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
			if not in the exchange nmps
				delete status
	*/
	glog.Infof(nmwlog("Starting to process all nmps in the exchange and locally."))
	nodeOrg := exchange.GetOrg(n.GetExchangeId())
	allNMPs, err := getAllNMPS(nodeOrg)
	if err != nil {
		return fmt.Errorf("Error getting node management policies from the exchange: %v", err)
	}
	nodePol, err := persistence.FindNodePolicy(n.db)
	if err != nil {
		return fmt.Errorf("Error getting node's policy to check management policy compatibility: %v", err)
	}
	nodeMgmtPol := nodePol.GetManagementPolicy()

	exchDev, err := persistence.FindExchangeDevice(n.db)
	if err != nil {
		return fmt.Errorf("Error getting device from database: %v", err)
	}
	nodePattern := exchDev.Pattern
	matchingNMPs := map[string]exchangecommon.ExchangeNodeManagementPolicy{}

	for name, policy := range *allNMPs {
		if match, _ := VerifyCompatible(nodeMgmtPol, nodePattern, &policy); match {
			matchingNMPs[name] = policy
			org, nodeId := cutil.SplitOrgSpecUrl(n.GetExchangeId())
			glog.Infof(nmwlog(fmt.Sprintf("Found matching node management policy %v in the exchange.", name)))
			if !policy.Enabled {
				glog.Errorf("disabled policy %v", name)
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
				glog.Errorf("enabled policy %v", name)
				if err = persistence.SaveOrUpdateNodeManagementPolicy(n.db, name, policy); err != nil {
					return err
				}
				existingStatus, err := persistence.FindNMPStatus(n.db, name)
				if err != nil {
					return fmt.Errorf("Error getting status for policy %v from the database: %v", name, err)
				} else if existingStatus == nil {
					eventlog.LogNodeEvent(n.db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_NMP_STATUS_CREATED, name), persistence.EC_NMP_STATUS_UPDATE_NEW, exchange.GetId(n.GetExchangeId()), exchange.GetOrg(n.GetExchangeId()), nodePattern, exchDev.Config.State)
					glog.Infof(nmwlog(fmt.Sprintf("Saving node management policy status %v in the db.", name)))
					newStatus := exchangecommon.StatusFromNewPolicy(policy, baseWorkingFile)
					if err = n.UpdateStatus(name, &newStatus, putNMPStatus); err != nil {
						glog.Errorf(nmwlog(fmt.Sprintf("Failed to update status for %v: %v", name, err)))
					}
				}
			}
		}
	}
	if allStatuses, err := persistence.FindAllNMPStatus(n.db); err != nil {
		return err
	} else {
		for statusName, _ := range allStatuses {
			if _, ok := matchingNMPs[statusName]; !ok {
				// The nmp this status is for is no longer in the exchange. Delete it
				glog.Infof(nmwlog(fmt.Sprintf("Removing status %v from the local database as if no longer exists in the exchange.", statusName)))
				if _, err := persistence.DeleteNMPStatus(n.db, statusName); err != nil {
					return err
				}
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
		}
	}
}

func VerifyCompatible(nodePol *externalpolicy.ExternalPolicy, nodePattern string, nmPol *exchangecommon.ExchangeNodeManagementPolicy) (bool, error) {
	if nodePattern != "" {
		if cutil.SliceContains(nmPol.Patterns, nodePattern) {
			return true, nil
		}
		return cutil.SliceContains(nmPol.Patterns, strings.SplitN(nodePattern, "/", 2)[1]), nil
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
	if path, err := os.Open(filePath); err != nil {
		return fmt.Errorf("Failed to open status file %v for management job %v. Errorf was: %v", filePath, policyName, err)
	} else {
		contents := exchangecommon.NodeManagementPolicyStatus{}
		err = json.NewDecoder(path).Decode(&contents)
		if err != nil {
			return fmt.Errorf("Failed to decode status file %v for management job %v. Error was %v.", filePath, policyName, err)
		}

		dbStatus.SetActualStartTime(contents.AgentUpgrade.ActualStartTime)
		dbStatus.SetCompletionTime(contents.AgentUpgrade.CompletionTime)
		dbStatus.SetStatus(contents.AgentUpgrade.Status)
		dbStatus.SetErrorMessage(contents.AgentUpgrade.ErrorMessage)
		pattern := ""
		configState := ""
		exchDev, err := persistence.FindExchangeDevice(n.db)
		if err != nil {
			glog.Errorf(nmwlog(fmt.Sprintf("Error getting device from database: %v", err)))
		} else if exchDev != nil {
			pattern = exchDev.Pattern
			configState = exchDev.Config.State
		}
		if dbStatus.Status() == "" {
			dbStatus.SetStatus(exchangecommon.STATUS_UNKNOWN)
		}

		// Use the versions in the status to set the device versions
		if dbStatus.AgentUpgrade.UpgradedVersions.ConfigVersion != "" {
			exchDev.SetConfigVersion(n.db, exchDev.Id, dbStatus.AgentUpgrade.UpgradedVersions.ConfigVersion)
		}
		if dbStatus.AgentUpgrade.UpgradedVersions.CertVersion != "" {
			exchDev.SetCertVersion(n.db, exchDev.Id, dbStatus.AgentUpgrade.UpgradedVersions.CertVersion)
		}

		if err = n.UpdateStatus(policyName, dbStatus, exchange.GetPutNodeManagementPolicyStatusHandler(n)); err != nil {
			// return err
		}
		eventlog.LogNodeEvent(n.db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_NMP_STATUS_CHANGED, policyName, dbStatus), persistence.EC_NMP_STATUS_UPDATE_NEW, exchange.GetId(n.GetExchangeId()), exchange.GetOrg(n.GetExchangeId()), pattern, configState)

		// Status has been read-in and updated sucessfully. Can now remove the working dirctory for the job.
		err = os.RemoveAll(workingFolderPath)
		if err != nil {
			return fmt.Errorf("Failed to remove the working directory for management job %v. Error was: %v", policyName, err)
		}
	}
	return nil
}

// Update a given nmp status in the db and the exchange
func (n *NodeManagementWorker) UpdateStatus(policyName string, status *exchangecommon.NodeManagementPolicyStatus, putStatusHandler exchange.PutNodeManagementPolicyStatusHandler) error {
	org, nodeId := cutil.SplitOrgSpecUrl(n.GetExchangeId())
	if err := persistence.SaveOrUpdateNMPStatus(n.db, policyName, *status); err != nil {
		return err
	}
	if _, err := putStatusHandler(org, nodeId, policyName, status); err != nil {
		return fmt.Errorf("Failed to put node management policy status for policy %v to the exchange: %v", policyName, err)
	}
	return nil
}

// Change the statuses of all nmp statuses that specify "latest" for a version to waiting as there is a new version availible
// If there is no new version for whatever the status has "latest" for, it will be marked successful without executing
func (n *NodeManagementWorker) HandleAgentFilesVersionChange() {
	if latestStatuses, err := persistence.FindNMPWithLatestKeywordVersion(n.db); err != nil {
		glog.Errorf("Error getting nmp statuses from db to change to \"waiting\". Error was: %v", err)
		return
	} else {
		for statusName, status := range latestStatuses {
			status.AgentUpgrade.Status = exchangecommon.STATUS_NEW
			err = n.UpdateStatus(statusName, status, exchange.GetPutNodeManagementPolicyStatusHandler(n))
			if err != nil {
				glog.Errorf("Error changing nmp status for %v to \"waiting\". Error was %v.", statusName, err)
			}
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
