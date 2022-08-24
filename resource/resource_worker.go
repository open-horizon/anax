package resource

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/worker"
)

type ResourceWorker struct {
	worker.BaseWorker // embedded field
	db                *bolt.DB
	rm                *ResourceManager
	am                *AuthenticationManager
}

func NewResourceWorker(name string, config *config.HorizonConfig, db *bolt.DB, am *AuthenticationManager) *ResourceWorker {

	var ec *worker.BaseExchangeContext
	var rm *ResourceManager
	dev, _ := persistence.FindExchangeDevice(db)
	if dev != nil {
		ec = worker.NewExchangeContext(fmt.Sprintf("%v/%v", dev.Org, dev.Id), dev.Token, config.Edge.ExchangeURL, config.GetCSSURL(), config.Edge.AgbotURL, config.Collaborators.HTTPClientFactory)
		if !dev.IsEdgeCluster() {
			if config == nil || config.GetCSSURL() == "" {
				term_string := "Terminating, unable to start model management resource manager. Please set either CSSURL in the anax configuration file or HZN_FSS_CSSURL in /etc/default/horizon file."
				glog.Errorf(term_string)
				panic(term_string)
			}

			rm = NewResourceManager(config, dev.Org, dev.Pattern, dev.Id, dev.Token)
		}
	}

	if rm == nil {
		rm = NewResourceManager(config, "", "", "", "")
	}

	worker := &ResourceWorker{
		BaseWorker: worker.NewBaseWorker(name, config, ec),
		db:         db,
		rm:         rm,
		am:         am,
	}

	glog.Info(reslog(fmt.Sprintf("Starting Resource worker")))
	// Establish the no work interval at 1 hour for garbage collection of resources.
	worker.Start(worker, 3600)
	return worker
}

func (w *ResourceWorker) Messages() chan events.Message {
	return w.BaseWorker.Manager.Messages
}

func (w *ResourceWorker) Initialize() bool {
	if w.rm.Configured() {
		if err := w.rm.StartFileSyncServiceAndSecretsAPI(w.am, w.db); err != nil {
			glog.Errorf(reslog(fmt.Sprintf("Error starting ESS and Secrets API: %v", err)))
			return false
		}
	}
	return true
}

// Handle events that are propogated to this worker from the internal event bus.
func (w *ResourceWorker) NewEvent(incoming events.Message) {

	switch incoming.(type) {

	case *events.EdgeRegisteredExchangeMessage:
		msg, _ := incoming.(*events.EdgeRegisteredExchangeMessage)
		w.EC = worker.NewExchangeContext(fmt.Sprintf("%v/%v", msg.Org(), msg.DeviceId()), msg.Token(), w.Config.Edge.ExchangeURL, w.Config.Edge.AgbotURL, w.Config.GetCSSURL(), w.Config.Collaborators.HTTPClientFactory)
		w.Commands <- NewNodeConfigCommand(msg)

	case *events.NodeShutdownMessage:
		msg, _ := incoming.(*events.NodeShutdownMessage)
		switch msg.Event().Id {
		case events.START_UNCONFIGURE:
			w.Commands <- NewNodeUnconfigCommand(msg)
		}

	default: //nothing

	}

	return
}

// Handle commands that are placed on the command queue.
func (w *ResourceWorker) CommandHandler(command worker.Command) bool {

	switch command.(type) {
	case *NodeConfigCommand:
		cmd, _ := command.(*NodeConfigCommand)
		if err := w.handleNodeConfigCommand(cmd); err != nil {
			glog.Errorf(reslog(fmt.Sprintf("Error handling node config command: %v", err)))
		}

	case *NodeUnconfigCommand:
		cmd, _ := command.(*NodeUnconfigCommand)
		err := w.handleNodeUnconfigCommand(cmd)
		errMsg := ""
		if err != nil {
			glog.Errorf(reslog(fmt.Sprintf("Error handling node unconfig command: %v", err)))
			errMsg = err.Error()
		}
		w.Messages() <- events.NewSyncServiceCleanedUpMessage(events.ESS_UNCONFIG, errMsg)

	default:
		return false
	}
	return true

}

// This function gets called when the worker framework has found nothing to do for the "no work interval"
// that was set when the worker was started.
func (w *ResourceWorker) NoWorkHandler() {
	glog.V(5).Infof(reslog(fmt.Sprintf("beginning garbage collection.")))

	glog.V(5).Infof(reslog(fmt.Sprintf("ending garbage collection.")))

}

// The node has just been configured so we can start functions that need node credentials to login. If the node is
// not using a pattern, then we will hard code the destination type of the node. The destination type is not important
// when services and models are being placed on nodes by policy, thus we can hard code it.
func (w *ResourceWorker) handleNodeConfigCommand(cmd *NodeConfigCommand) error {

	// For now, the model management system and therefore the embedded ESS is disabled when the agent is running
	// on an edge cluster.
	if dev, err := persistence.FindExchangeDevice(w.db); err != nil {
		glog.Errorf(reslog(fmt.Sprintf("Error reading device from local DB: %v", err)))
		return err
	} else if dev == nil {
		return errors.New("no device object in local DB")
	} else if dev.IsEdgeCluster() {
		return nil
	} else {
		if w.Config == nil || w.Config.GetCSSURL() == "" {
			term_string := "Terminating, unable to start model management resource manager. Please set either CSSURL in the anax configuration file or HZN_FSS_CSSURL in /etc/default/horizon file."
			glog.Errorf(term_string)
			panic(term_string)
		}
	}

	// Start the embedded ESS this edge device.
	destinationType := cmd.msg.Pattern()
	if destinationType == "" {
		destinationType = "openhorizon/openhorizon.edgenode"
	}
	w.rm.NodeConfigUpdate(cmd.msg.Org(), destinationType, cmd.msg.DeviceId(), cmd.msg.Token())
	return w.rm.StartFileSyncServiceAndSecretsAPI(w.am, w.db)
}

// The node has just been unconfigured so we can stop the file sync service.
func (w *ResourceWorker) handleNodeUnconfigCommand(cmd *NodeUnconfigCommand) error {
	w.rm.StopFileSyncService()
	w.Commands <- worker.NewTerminateCommand("shutdown")
	return nil
}

// Utility logging function
var reslog = func(v interface{}) string {
	return fmt.Sprintf("Resource Worker: %v", v)
}
