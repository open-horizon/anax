package agreement

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/eventlog"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangesync"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"sort"
	"strings"
)

// Check node changes on the exchange and save it on local node
func (w *AgreementWorker) checkNodeChanges() {
	glog.V(3).Infof(logString(fmt.Sprintf("checking the exchange node changes.")))

	// get the node
	pDevice, err := persistence.FindExchangeDevice(w.db)
	if err != nil {
		glog.Errorf(logString(fmt.Sprintf("Unable to read node object from the local database. %v", err)))
		eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
			persistence.NewMessageMeta(EL_AG_UNABLE_READ_NODE_FROM_DB, err.Error()),
			persistence.EC_DATABASE_ERROR)
		return
	} else if pDevice == nil {
		glog.Errorf(logString(fmt.Sprintf("No device is found from the local database.")))
		return
	}

	// save a local copy of the exchange node
	exchNode, err := exchangesync.SyncNodeWithExchange(w.db, pDevice, exchange.GetHTTPDeviceHandler(w.limitedRetryEC))
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
			return
		}
	} else {
		w.hznOffline = false
	}

	glog.V(3).Infof(logString(fmt.Sprintf("Done checking exchange node changes.")))

	// now check the user input changes.
	w.checkNodeUserInputChanges(pDevice)

	// check the pattern changes
	w.checkNodePatternChanges(exchNode)

	return
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
// the local copy. The exchange is the master
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
	} else if updated && len(changedSvcSpecs) != 0{
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
func (w *AgreementWorker) checkServiceConfigStateChanges() {
	glog.V(3).Infof(logString(fmt.Sprintf("Check the service configuration state")))

	// get the node
	pDevice, err := persistence.FindExchangeDevice(w.db)
	if err != nil {
		glog.Errorf(logString(fmt.Sprintf("Unable to read node object from the local database. %v", err)))
		eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
			persistence.NewMessageMeta(EL_AG_UNABLE_READ_NODE_FROM_DB, err.Error()),
			persistence.EC_DATABASE_ERROR)
		return
	} else if pDevice == nil {
		glog.Errorf(logString(fmt.Sprintf("No device is found from the local database.")))
		return
	}

	// save a local copy of the exchange node
	exchDevice, err := exchangesync.SyncNodeWithExchange(w.db, pDevice, exchange.GetHTTPDeviceHandler(w.limitedRetryEC))
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
			return
		}
	} else {
		w.hznOffline = false
	}

	if exchDevice == nil {
		return
	}

	// get the services that has been changed
	var changed_services []events.ServiceConfigState

	for _, service := range exchDevice.RegisteredServices {
		// service.Url is in the form of org/url
		org, url := cutil.SplitOrgSpecUrl(service.Url)

		// set to default if empty
		config_state := service.ConfigState
		if config_state == "" {
			config_state = exchange.SERVICE_CONFIGSTATE_ACTIVE
		}

		var version, arch string
		pol, err := policy.DemarshalPolicy(service.Policy)
		if err != nil {
			glog.Errorf(logString(fmt.Sprintf("Error unmarshaling service policy: %v", err)))
		} else {
			for _, spec := range pol.APISpecs {
				if spec.SpecRef == url && spec.Org == org {
					version = spec.Version
					arch = spec.Arch
				}
			}
		}

		// all services will be handled even the ones that wasn't changed from last check.
		// this will make sure there is no leak between the checking intervals, for example the un-arrived agreements
		// from last check.
		changed_services = append(changed_services,
			*(events.NewServiceConfigState(url, org, version, arch, config_state)))
	}

	glog.V(5).Infof(logString(fmt.Sprintf("Suspended services to handle are %v", changed_services)))

	// fire event to handle the suspended services if any
	if len(changed_services) != 0 {
		w.Messages() <- events.NewServiceConfigStateChangeMessage(events.SERVICE_CONFIG_STATE_CHANGED, changed_services)
	}
}

// Check the node policy changes on the exchange and sync up with
// the local copy. The exchange is the master.
func (w *AgreementWorker) checkNodePolicyChanges() {
	glog.V(3).Infof(logString(fmt.Sprintf("checking the node policy changes.")))

	// get the node
	pDevice, err := persistence.FindExchangeDevice(w.db)
	if err != nil {
		glog.Errorf(logString(fmt.Sprintf("Unable to read node object from the local database. %v", err)))
		eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
			persistence.NewMessageMeta(EL_AG_UNABLE_READ_NODE_FROM_DB, err.Error()),
			persistence.EC_DATABASE_ERROR)
		return
	} else if pDevice == nil {
		glog.Errorf(logString(fmt.Sprintf("No device is found from the local database.")))
		return
	}

	// exchange is the master
	uc_deployment, uc_management, newNodePolicy, err := exchangesync.SyncNodePolicyWithExchange(w.db, pDevice, exchange.GetHTTPNodePolicyHandler(w.limitedRetryEC), exchange.GetHTTPPutNodePolicyHandler(w.limitedRetryEC))
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
	} else if uc_deployment != externalpolicy.EP_COMPARE_NOCHANGE || uc_management != externalpolicy.EP_COMPARE_NOCHANGE {
		w.hznOffline = false
		glog.V(3).Infof(logString(fmt.Sprintf("Node policy updated with the exchange copy: %v", newNodePolicy)))
		eventlog.LogNodeEvent(w.db, persistence.SEVERITY_INFO,
			persistence.NewMessageMeta(EL_AG_NODE_POL_SYNCED_WITH_EXCH, newNodePolicy),
			persistence.EC_NODE_POLICY_UPDATED,
			exchange.GetOrg(w.GetExchangeId()),
			exchange.GetId(w.GetExchangeId()),
			w.devicePattern, "")

		w.Messages() <- events.NewNodePolicyMessage(events.UPDATE_POLICY, uc_deployment, uc_management)
	} else {
		w.hznOffline = false
	}
	glog.V(3).Infof(logString(fmt.Sprint("Done checking the node policy changes.")))
	return
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
