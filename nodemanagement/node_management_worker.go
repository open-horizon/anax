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
		if err := w.ProcessAllNMPS(workingDir); err != nil {
			glog.Errorf(nmwlog(fmt.Sprintf("Error processing all exchange policies: %v", err)))
			// return true
		}
		// Check if a nmp process completed
		if err := w.CheckNMPStatus(workingDir, STATUS_FILE_NAME); err != nil {
			glog.Errorf(nmwlog(fmt.Sprintf("Failed to collect status. error: %v", err)))
			return true
		}
	}

	return true
}

func (w *NodeManagementWorker) checkNMPTimeToRun() int {
	glog.Infof(nmwlog("Starting run of node management policy monitoring subworker."))
	if waitingNMPs, err := persistence.FindWaitingNMPStatuses(w.db); err != nil {
		glog.Errorf(nmwlog(fmt.Sprintf("Failed to get nmp statuses from the database. Error was %v", err)))
	} else {
		for nmpName, nmpStatus := range waitingNMPs {
			if nmpStatus.TimeToStart() {
				glog.Infof(nmwlog(fmt.Sprintf("Time to start nmp %v", nmpName)))
				w.Messages() <- events.NewNMPStartDownloadMessage(events.NMP_START_DOWNLOAD, events.StartDownloadMessage{NMPStatus: nmpStatus, NMPName: nmpName})
			}
		}
	}

	return 60
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
	if err := n.ProcessAllNMPS(workingDir); err != nil {
		glog.Errorf(nmwlog(fmt.Sprintf("Error processing all exchange policies: %v", err)))
		// uncomment the return once nmp status is in the exchange-api
		// return
	}
	return
}

func (n *NodeManagementWorker) DownloadComplete(cmd *NMPDownloadCompleteCommand) {
	status, err := persistence.FindNMPStatus(n.db, cmd.Msg.NMPName)
	if err != nil {
		glog.Errorf(nmwlog(fmt.Sprintf("Failed to get nmp status %v from the database: %v", cmd.Msg.NMPName, err)))
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
	if cmd.Msg.Success {
		status.SetStatus(exchangecommon.STATUS_DOWNLOADED)
		eventlog.LogNodeEvent(n.db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_NMP_STATUS_CHANGED, cmd.Msg.NMPName, exchangecommon.STATUS_DOWNLOADED), persistence.EC_NMP_STATUS_UPDATE_NEW, exchange.GetId(n.GetExchangeId()), exchange.GetOrg(n.GetExchangeId()), pattern, configState)
	} else {
		status.SetStatus(exchangecommon.STATUS_DOWNLOAD_FAILED)
		eventlog.LogNodeEvent(n.db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_NMP_STATUS_CHANGED, cmd.Msg.NMPName, exchangecommon.STATUS_DOWNLOAD_FAILED), persistence.EC_NMP_STATUS_UPDATE_NEW, exchange.GetId(n.GetExchangeId()), exchange.GetOrg(n.GetExchangeId()), pattern, configState)
	}
	err = persistence.SaveOrUpdateNMPStatus(n.db, cmd.Msg.NMPName, *status)
	if err != nil {
		glog.Errorf(nmwlog(fmt.Sprintf("Failed to update nmp status %v in the database: %v", cmd.Msg.NMPName, err)))
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
		n.ProcessAllNMPS(n.Config.Edge.GetNodeMgmtDirectory())
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

func (n *NodeManagementWorker) ProcessAllNMPS(baseWorkingFile string) error {
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
	allNMPs, err := exchange.GetAllExchangeNodeManagementPolicy(n, nodeOrg)
	if err != nil {
		return fmt.Errorf("Error getting node management policies from the exchange: %v", err)
	}
	nodePol, err := persistence.FindNodePolicy(n.db)
	if err != nil {
		return fmt.Errorf("Error getting node's policy to check management policy compatibility: %v", err)
	}
	nodePattern := ""
	exchDev, err := persistence.FindExchangeDevice(n.db)
	if err != nil {
		return fmt.Errorf("Error getting device from database: %v", err)
	}
	nodePattern = exchDev.Pattern

	for name, policy := range *allNMPs {
		if match, _ := VerifyCompatible(&nodePol.Management, nodePattern, &policy); match {
			deleted := false
			glog.Infof(nmwlog(fmt.Sprintf("Found matching node management policy %v in the exchange.", name)))
			if !policy.Enabled {
				if _, err = persistence.DeleteNMPStatus(n.db, name); err != nil {
					return fmt.Errorf("Failed to delete status for deactivated node policy %v from database. Error was %v", name, err)
				}
				deleted = true
			}
			if err = persistence.SaveOrUpdateNodeManagementPolicy(n.db, name, policy); err != nil {
				return err
			}
			existingStatus, err := persistence.FindNMPStatus(n.db, name)
			if err != nil {
				return fmt.Errorf("Error getting status for policy %v from the database: %v", name, err)
			} else if existingStatus == nil && !deleted {
				eventlog.LogNodeEvent(n.db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(EL_NMP_STATUS_CREATED, name), persistence.EC_NMP_STATUS_UPDATE_NEW, exchange.GetId(n.GetExchangeId()), exchange.GetOrg(n.GetExchangeId()), nodePattern, exchDev.Config.State)
				glog.Infof(nmwlog(fmt.Sprintf("Saving node management policy status %v in the db.", name)))
				newStatus := exchangecommon.StatusFromNewPolicy(policy, baseWorkingFile)
				org, nodeId := cutil.SplitOrgSpecUrl(n.GetExchangeId())
				if err = persistence.SaveOrUpdateNMPStatus(n.db, name, newStatus); err != nil {
					return err
				} else if _, err = exchange.PutNodeManagementPolicyStatus(n, org, nodeId, name, &newStatus); err != nil {
					return err
				}
			}
		}
	}
	if allStatuses, err := persistence.FindAllNMPStatus(n.db); err != nil {
		return err
	} else {
		for statusName, _ := range allStatuses {
			if _, ok := (*allNMPs)[statusName]; !ok {
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
				glog.Infof(nmwlog(fmt.Sprintf("No status file found for nmp %v: %v", name, err)))
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
		if err = n.UpdateStatus(policyName, dbStatus); err != nil {
			return err
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
func (n *NodeManagementWorker) UpdateStatus(policyName string, status *exchangecommon.NodeManagementPolicyStatus) error {
	org, nodeId := cutil.SplitOrgSpecUrl(n.GetExchangeId())
	if err := persistence.SaveOrUpdateNMPStatus(n.db, policyName, *status); err != nil {
		return err
	}
	if _, err := exchange.PutNodeManagementPolicyStatus(n, org, nodeId, policyName, status); err != nil {
		return fmt.Errorf("Failed to put node management policy status for policy %v to the exchange: %v", policyName, err)
	}
	return nil
}

func nmwlog(message string) string {
	return fmt.Sprintf("Node management worker: %v", message)
}

// This will remove nmps and statuses from the local db and the exchange
func (n *NodeManagementWorker) HandleUnregister() {
	if err := persistence.DeleteAllNodeManagementPolicies(n.db); err != nil {
		glog.Errorf(nmwlog(fmt.Sprintf("Error removing node management policies from the local db: %v", err)))
	}
	if err := persistence.DeleteAllNMPStatuses(n.db); err != nil {
		glog.Errorf(nmwlog(fmt.Sprintf("Error removing node management policy statuses from the local db: %v", err)))
	}
}
