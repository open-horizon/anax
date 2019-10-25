package agreement

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/eventlog"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangesync"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/version"
	"sort"
	"strings"
	"time"
)

// Heartbeat to the exchange. This function is called by the heartbeat subworker.
func (w *AgreementWorker) heartBeat() int {

	if w.Config.Edge.ExchangeVersionCheckIntervalM > 0 {
		// get the exchange version check interval and change to seconds
		check_interval := w.Config.Edge.ExchangeVersionCheckIntervalM * 60

		// check the exchange version when time is right
		time_now := time.Now().Unix()
		if w.lastExchVerCheck == 0 || time_now-w.lastExchVerCheck >= int64(check_interval) {
			w.lastExchVerCheck = time_now

			// log error if the current exchange version does not meet the requirement
			if err := version.VerifyExchangeVersion(w.GetHTTPFactory(), w.GetExchangeURL(), w.GetExchangeId(), w.GetExchangeToken(), false); err != nil {
				glog.Errorf(logString(fmt.Sprintf("Error verifiying exchange version. error: %v", err)))
			}
		}
	}

	// now do the hearbeat
	nodeOrg := exchange.GetOrg(w.GetExchangeId())
	nodeId := exchange.GetId(w.GetExchangeId())

	targetURL := w.GetExchangeURL() + "orgs/" + nodeOrg + "/nodes/" + nodeId + "/heartbeat"
	err := exchange.Heartbeat(w.GetHTTPFactory().NewHTTPClient(nil), targetURL, w.GetExchangeId(), w.GetExchangeToken())

	if err != nil {
		if strings.Contains(err.Error(), "status: 401") {
			// If the heartbeat fails because the node entry is gone then initiate a full node quiesce
			w.Messages() <- events.NewNodeShutdownMessage(events.START_UNCONFIGURE, false, false)
		} else {
			// let other workers know that the heartbeat failed.
			// the message is sent out only when the heartbeat state changes from success to failed.
			if !w.heartBeatFailed {
				w.heartBeatFailed = true

				glog.Errorf(logString(fmt.Sprintf("node heartbeat failed for node %v/%v. Error: %v", nodeOrg, nodeId, err)))
				eventlog.LogNodeEvent(w.db, persistence.SEVERITY_ERROR,
					persistence.NewMessageMeta(EL_AG_NODE_HB_FAILED, nodeOrg, nodeId, err.Error()),
					persistence.EC_NODE_HEARTBEAT_FAILED, nodeId, nodeOrg, "", "")

				w.Messages() <- events.NewNodeHeartbeatStateChangeMessage(events.NODE_HEARTBEAT_FAILED, nodeOrg, nodeId)
			}
		}
	} else {
		if w.heartBeatFailed {
			// let other workers know that the heartbeat restored
			// the message is sent out only when the heartbeat state changes from faild to success.
			w.heartBeatFailed = false

			glog.Infof(logString(fmt.Sprintf("node heartbeat restored for node %v/%v.", nodeOrg, nodeId)))
			eventlog.LogNodeEvent(w.db, persistence.SEVERITY_INFO,
				persistence.NewMessageMeta(EL_AG_NODE_HB_RESTORED, nodeOrg, nodeId),
				persistence.EC_NODE_HEARTBEAT_RESTORED, nodeId, nodeOrg, "", "")

			w.Messages() <- events.NewNodeHeartbeatStateChangeMessage(events.NODE_HEARTBEAT_RESTORED, nodeOrg, nodeId)
		}
	}

	return 0
}

// handles the node policy UPDATE_POLICY event
func (w *AgreementWorker) NodePolicyUpdated() {
	glog.V(5).Infof(logString("handling node policy updated."))
	// get the node policy
	nodePolicy, err := persistence.FindNodePolicy(w.db)
	if err != nil {
		glog.Errorf(logString(fmt.Sprintf("unable to read node policy from the local database. %v", err)))
		eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
			persistence.NewMessageMeta(EL_AG_UNABLE_READ_NODE_POL_FROM_DB, err.Error()),
			persistence.EC_DATABASE_ERROR)
		return
	}

	// add the node policy to the policy manager
	newPol, err := policy.GenPolicyFromExternalPolicy(nodePolicy, policy.MakeExternalPolicyHeaderName(w.GetExchangeId()))
	if err != nil {
		glog.Errorf(logString(fmt.Sprintf("Failed to convert node policy to policy file format: %v", err)))
		return
	}
	w.pm.UpdatePolicy(exchange.GetOrg(w.GetExchangeId()), newPol)
}

// handles the node policy DELETE_POLICY event
func (w *AgreementWorker) NodePolicyDeleted() {
	glog.V(5).Infof(logString("handling node policy deleted."))
	w.pm.DeletePolicyByName(exchange.GetOrg(w.GetExchangeId()), policy.MakeExternalPolicyHeaderName(w.GetExchangeId()))
}

// Check node changes on the exchange and save it on local node
func (w *AgreementWorker) checkNodeChanges() int {
	glog.V(5).Infof(logString(fmt.Sprintf("checking the exchange node changes.")))

	// get the node
	pDevice, err := persistence.FindExchangeDevice(w.db)
	if err != nil {
		glog.Errorf(logString(fmt.Sprintf("Unable to read node object from the local database. %v", err)))
		eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
			persistence.NewMessageMeta(EL_AG_UNABLE_READ_NODE_FROM_DB, err.Error()),
			persistence.EC_DATABASE_ERROR)
		return 0
	} else if pDevice == nil {
		glog.Errorf(logString(fmt.Sprintf("No device is found from the local database.")))
		return 0
	}

	// save a local copy of the exchange node
	exchNode, err := exchangesync.SyncNodeWithExchange(w.db, pDevice, exchange.GetHTTPDeviceHandler(w))
	if err != nil {
		if !w.hznOffline {
			glog.Errorf(logString(fmt.Sprintf("Unable to sync the node with the exchange copy. Error: %v", err)))
			eventlog.LogNodeEvent(w.db, persistence.SEVERITY_ERROR,
				persistence.NewMessageMeta(EL_AG_UNABLE_SYNC_NODE_WITH_EXCH, err.Error()),
				persistence.EC_ERROR_NODE_SYNC,
				exchange.GetOrg(w.GetExchangeId()),
				exchange.GetId(w.GetExchangeId()),
				w.devicePattern, "")
			w.isOffline()
			return 0
		}
	} else {
		w.hznOffline = false
	}

	glog.V(5).Infof(logString(fmt.Sprintf("Done checking exchange node changes.")))

	// now check the user input changes.
	w.checkNodeUserInputChanges(pDevice)

	// check the pattern changes
	w.checkNodePatternChanges(exchNode)

	// check the service configstate changes
	w.checkServiceConfigStateChanges(exchNode)

	return 0
}

// Check the node user input changes on the exchange and sync up with
// the local copy. THe exchange is the master
func (w *AgreementWorker) checkNodePatternChanges(exchDevice *exchange.Device) {
	glog.V(5).Infof(logString(fmt.Sprintf("checking the node pattern changes.")))

	if exchDevice == nil {
		return
	}

	glog.V(5).Infof(logString(fmt.Sprintf("checking the node pattern devp=%v, exchp=%v", w.devicePattern, exchDevice.Pattern)))

	if w.devicePattern != "" && exchDevice.Pattern != "" && w.devicePattern != exchDevice.Pattern {
		saved_pattern, err := persistence.FindSavedNodeExchPattern(w.db)
		if err != nil {
			glog.Errorf(logString(fmt.Sprintf("Unable to retrieve the saved node exchange pattern from the local database. %v", err)))
			eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
				persistence.NewMessageMeta(EL_AG_UNABLE_READ_NODE_EXCH_PATTERN_FROM_DB, err.Error()),
				persistence.EC_DATABASE_ERROR)
			return
		} else if saved_pattern != "" {
			// will not handle it because the pattern may be changed by the shutdown process.
			glog.Infof(logString(fmt.Sprintf("Node pattern changed to %v on the exchange, but will not handle it because there is already a saved exchange pattern %v in local database that needs to be handled.", exchDevice.Pattern, saved_pattern)))
			return
		}

		// save the node change pattern into the local db
		if err := persistence.SaveNodeExchPattern(w.db, exchDevice.Pattern); err != nil {
			glog.Errorf(logString(fmt.Sprintf("Unable to save the new node exchange pattern %v to the local database. %v", exchDevice.Pattern, err)))
			eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
				persistence.NewMessageMeta(EL_AG_UNABLE_WRITE_NODE_EXCH_PATTERN_TO_DB, exchDevice.Pattern, err.Error()),
				persistence.EC_DATABASE_ERROR)
			return
		} else {
			glog.Infof(logString(fmt.Sprintf("Node pattern changed on the exchange from %v to %v. Will re-register the node.", w.devicePattern, exchDevice.Pattern)))
			w.Messages() <- events.NewNodePatternMessage(events.NODE_PATTERN_CHANGE_SHUTDOWN, exchDevice.Pattern)
		}
	}

	glog.V(5).Infof(logString(fmt.Sprintf("Done checking the node pattern changes.")))
}

// Check the node user input changes on the exchange and sync up with
// the local copy. THe exchange is the master
func (w *AgreementWorker) checkNodeUserInputChanges(pDevice *persistence.ExchangeDevice) {
	glog.V(5).Infof(logString(fmt.Sprintf("checking the node user input changes.")))

	// exchange is the master
	updated, changedSvcSpecs, err := exchangesync.SyncLocalUserInputWithExchange(w.db, pDevice, nil)
	if err != nil {
		glog.Errorf(logString(fmt.Sprintf("Unable to sync the local node user input with the exchange copy. Error: %v", err)))
		if !w.hznOffline {
			eventlog.LogNodeEvent(w.db, persistence.SEVERITY_ERROR,
				persistence.NewMessageMeta(EL_AG_UNABLE_SYNC_NODE_UI_WITH_EXCH, err.Error()),
				persistence.EC_ERROR_NODE_USERINPUT_UPDATE,
				exchange.GetOrg(w.GetExchangeId()),
				exchange.GetId(w.GetExchangeId()),
				w.devicePattern, "")
			w.isOffline()
		}
	} else if updated {
		w.hznOffline = false
		glog.V(3).Infof(logString(fmt.Sprintf("Node user input updated with the exchange copy. The changed user inputs are: %v", changedSvcSpecs)))
		eventlog.LogNodeEvent(w.db, persistence.SEVERITY_INFO,
			persistence.NewMessageMeta(EL_AG_NODE_UI_SYNCED_WITH_EXCH, changedSvcSpecs),
			persistence.EC_NODE_POLICY_UPDATED,
			exchange.GetOrg(w.GetExchangeId()),
			exchange.GetId(w.GetExchangeId()),
			w.devicePattern, "")

		// only inform the user input changes when the node is configured.
		if pDevice.Config.State == persistence.CONFIGSTATE_CONFIGURED {
			w.Messages() <- events.NewNodeUserInputMessage(events.UPDATE_NODE_USERINPUT, changedSvcSpecs)
		}
	} else {
		w.hznOffline = false
	}

	glog.V(5).Infof(logString(fmt.Sprintf("Done checking the user input changes.")))
}

// get the service configuration state from the exchange, check if any of them are suspended.
// if a service is suspended, cancel the agreements and remove the containers associated with it.
func (w *AgreementWorker) checkServiceConfigStateChanges(exchDevice *exchange.Device) {
	glog.V(4).Infof(logString(fmt.Sprintf("Check the service configuration state")))

	if exchDevice == nil {
		return
	}

	// get the service configuration states from the node
	service_cs := []exchange.ServiceConfigState{}
	for _, service := range exchDevice.RegisteredServices {
		// service.Url is in the form of org/url
		org, url := cutil.SplitOrgSpecUrl(service.Url)

		// set to default if empty
		config_state := service.ConfigState
		if config_state == "" {
			config_state = exchange.SERVICE_CONFIGSTATE_ACTIVE
		}

		mcs := exchange.NewServiceConfigState(url, org, config_state)
		service_cs = append(service_cs, *mcs)
	}

	// get the services that has been changed to suspended state
	suspended_services := []events.ServiceConfigState{}

	if service_cs != nil {
		for _, scs_exchange := range service_cs {
			// all suspended services will be handled even the ones that was in the suspended state from last check.
			// this will make sure there is no leak between the checking intervals, for example the un-arrived agreements
			// from last check.
			if scs_exchange.ConfigState == exchange.SERVICE_CONFIGSTATE_SUSPENDED {
				suspended_services = append(suspended_services, *(events.NewServiceConfigState(scs_exchange.Url, scs_exchange.Org, scs_exchange.ConfigState)))
			}
		}
	}

	glog.V(5).Infof(logString(fmt.Sprintf("Suspended services to handle are %v", suspended_services)))

	// fire event to handle the suspended services if any
	if len(suspended_services) != 0 {
		// we only handle the suspended services for the configstate change now
		w.Messages() <- events.NewServiceConfigStateChangeMessage(events.SERVICE_SUSPENDED, suspended_services)
	}
}

// Check the node policy changes on the exchange and sync up with
// the local copy. THe exchange is the master
func (w *AgreementWorker) checkNodePolicyChanges() int {
	glog.V(5).Infof(logString(fmt.Sprintf("checking the node policy changes.")))

	// get the node
	pDevice, err := persistence.FindExchangeDevice(w.db)
	if err != nil {
		glog.Errorf(logString(fmt.Sprintf("Unable to read node object from the local database. %v", err)))
		eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
			persistence.NewMessageMeta(EL_AG_UNABLE_READ_NODE_FROM_DB, err.Error()),
			persistence.EC_DATABASE_ERROR)
		return 0
	} else if pDevice == nil {
		glog.Errorf(logString(fmt.Sprintf("No device is found from the local database.")))
		return 0
	}

	// exchange is the master
	updated, newNodePolicy, err := exchangesync.SyncNodePolicyWithExchange(w.db, pDevice, exchange.GetHTTPNodePolicyHandler(w), exchange.GetHTTPPutNodePolicyHandler(w))
	if err != nil {
		glog.Errorf(logString(fmt.Sprintf("Unable to sync the local node policy with the exchange copy. Error: %v", err)))
		if !w.hznOffline {
			eventlog.LogNodeEvent(w.db, persistence.SEVERITY_ERROR,
				persistence.NewMessageMeta(EL_AG_UNABLE_SYNC_NODE_POL_WITH_EXCH, err.Error()),
				persistence.EC_ERROR_NODE_POLICY_UPDATE,
				exchange.GetOrg(w.GetExchangeId()),
				exchange.GetId(w.GetExchangeId()),
				w.devicePattern, "")
			w.isOffline()
		}
	} else if updated {
		w.hznOffline = false
		glog.V(3).Infof(logString(fmt.Sprintf("Node policy updated with the exchange copy: %v", newNodePolicy)))
		eventlog.LogNodeEvent(w.db, persistence.SEVERITY_INFO,
			persistence.NewMessageMeta(EL_AG_NODE_POL_SYNCED_WITH_EXCH, newNodePolicy),
			persistence.EC_NODE_POLICY_UPDATED,
			exchange.GetOrg(w.GetExchangeId()),
			exchange.GetId(w.GetExchangeId()),
			w.devicePattern, "")

		if pDevice.Pattern == "" {
			w.Messages() <- events.NewNodePolicyMessage(events.UPDATE_POLICY)
		}
	} else {
		w.hznOffline = false
	}
	glog.V(5).Infof(logString(fmt.Sprint("Done checking the node policy changes.")))
	return 0
}

func (w *AgreementWorker) isOffline() {
	msgPrinter := i18n.GetMessagePrinterWithLocale("en")
	eventLogs, err := eventlog.GetEventLogs(w.db, false, nil, msgPrinter)
	if err != nil {
		glog.V(2).Infof("Error getting event logs: %v", err)
		return
	}
	sort.Sort(eventlog.EventLogByTimestamp(eventLogs))
	var lastThree []persistence.EventLog
	lastThree = eventLogs
	if len(eventLogs) > 4 {
		lastThree = eventLogs[len(eventLogs)-4:]
	}
	for _, log := range lastThree {
		if !(strings.Contains(log.Message, "network") || strings.Contains(log.Message, "connection") || strings.Contains(log.Message, "time out") || strings.Contains(log.Message, "server")) {
			w.hznOffline = false
			return
		}
	}
	eventlog.LogNodeEvent(w.db, persistence.SEVERITY_ERROR,
		persistence.NewMessageMeta(EL_AG_NODE_IS_OFFLINE),
		persistence.EC_ERROR_NODE_IS_OFFLINE,
		exchange.GetOrg(w.GetExchangeId()),
		exchange.GetId(w.GetExchangeId()),
		w.devicePattern, "")
	w.hznOffline = true
}
