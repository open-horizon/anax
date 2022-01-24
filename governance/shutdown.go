package governance

import (
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/container"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/producer"
	"runtime"
	"strings"
	"time"
)

// This function will quiesce the anax system, getting rid of agreements, containers, networks, etc so that the node can be
// restarted and then reconfigured. It runs as its own go routine so that it can wait for asynchronous things to happen. It
// will return to caller but it must put a shutdown complete message on the internal message bus before returning. If this
// function ends in an error, the error will be in the shutdown complete message.
//
// There are other workers responsible for other functions, which will also so some cleanup when the Node Shutdown Message
// arrives.
func (w *GovernanceWorker) nodeShutdown(cmd *NodeShutdownCommand) {

	errorMessage := ""

	glog.V(3).Infof(logString(fmt.Sprintf("begin node shutdown process.")))

	// Get the node's registration info from the local DB.
	dev, err := persistence.FindExchangeDevice(w.db)
	if err != nil {
		w.completedWithError(logString(fmt.Sprintf("received error reading node: %v", err)))
		return
	} else if dev == nil {
		w.completedWithError(logString(fmt.Sprintf("could not get node name because node was not registered yet")))
		return
	}

	new_pattern, err := persistence.FindSavedNodeExchPattern(w.db)
	if err != nil {
		w.completedWithError(logString(fmt.Sprintf("received error reading node exchange pattern from local database: %v", err)))
		return
	} else if new_pattern != "" {
		w.nodeShutDownForPattenChanged(dev, new_pattern)
		return
	}

	// Clear the Pattern and RegisteredServices array in the node’s exchange resource. We have to leave the
	// public key so that the node can send messages to an agbot. Removing the pattern and RegisteredServices
	// will prevent the exchange from finding the node and thereby prevent agobts from trying to make new agreements.
	if err := w.clearNodePatternAndMS(false); err != nil {
		w.continueWithError(logString(err.Error()))
	}

	// Cancel all agreements, all workload containers and networks will automatically terminate.
	if err := w.terminateAllAgreements(producer.TERM_REASON_NODE_SHUTDOWN); err != nil {
		w.completedWithError(logString(err.Error()))
		return
	}

	// Remove the node’s messaging public key from the node’s exchange resource and delete the node’s message key pair from the filesystem.
	if err := w.patchNodeKey(w.limitedRetryEC.GetHTTPFactory()); err != nil {
		w.continueWithError(logString(err.Error()))
		errorMessage = fmt.Sprintf("Unable to reset the node in the Exchange. Please use 'hzn exchange node remove %v' to remove it. The error was: %v", w.GetExchangeId(), err)
	}

	// Clear out any node errors.
	if err := w.deleteNodeError(); err != nil {
		w.continueWithError(logString(err.Error()))
	}

	// Delete the node's management policy statuses from the exchange
	if err := exchange.DeleteNodeManagementAllStatuses(w, exchange.GetOrg(w.GetExchangeId()), exchange.GetId(w.GetExchangeId())); err != nil {
		w.continueWithError(logString(err.Error()))
	}

	// Tell the exchange changes worker to stop retrieving and recording changes. We dont need it any longer now that the agreements
	// are all gone.
	w.Messages() <- events.NewExchangeChangesShutdownMessage(events.MESSAGE_STOP)

	// Tell the blockchain workers to terminate blockchain containers. We will do this by telling the producer protocol handlers to shutdown.
	// Any protocol handlers that are using a blockchain will tell the blockchain worker to terminate.
	w.Messages() <- events.NewAllBlockchainShutdownMessage(events.ALL_STOP)

	// Tell running microservices to terminate.
	if err := w.terminateMicroservices(); err != nil {
		w.completedWithError(logString(err.Error()))
		return
	}

	// Remove attributes from the database
	if err := w.deleteAttributes(); err != nil {
		w.completedWithError(logString(err.Error()))
		return
	}

	// Remove any policy files from the filesystem.
	if err := w.deletePolicyFiles(); err != nil {
		w.completedWithError(logString(err.Error()))
		return
	}

	// Remove the node's exchange resource.
	if cmd.Msg.RemoveNode() {
		if err := w.deleteNode(w.limitedRetryEC.GetHTTPFactory()); err != nil {
			w.continueWithError(logString(err.Error()))
			errorMessage = fmt.Sprintf("Unable to delete the node from the Exchange. Please use 'hzn exchange node remove %v' to remove it. The error was: %v", w.GetExchangeId(), err)
		}
	} else {
		// Remove any left over node status.
		ds := NewDeviceStatus()
		ds.Services = make([]WorkloadStatus, 0)
		if err := w.writeStatusToExchange(ds); err != nil {
			w.continueWithError(logString(err.Error()))
			errorMessage = fmt.Sprintf("Unable to delete node status from the Exchange. Please use 'hzn exchange node remove %v' to remove it. The error was: %v", w.GetExchangeId(), err)
		}
	}

	// Delete the HorizonDevice object from the local database.
	if err := w.deleteHorizonDevice(); err != nil {
		w.completedWithError(logString(err.Error()))
		return
	}

	// Delete node policy from local db
	if err := persistence.DeleteNodePolicy(w.db); err != nil {
		w.completedWithError(logString(err.Error()))
		return
	}

	// Reset the node policy last update time
	if err := persistence.DeleteNodePolicyLastUpdated_Exch(w.db); err != nil {
		w.completedWithError(logString(err.Error()))
		return
	}

	// Delete node user input from local db
	if err := persistence.DeleteNodeUserInput(w.db); err != nil {
		w.completedWithError(logString(err.Error()))
		return
	}

	// Reset the node user input hash
	if err := persistence.DeleteNodeUserInputHash_Exch(w.db); err != nil {
		w.completedWithError(logString(err.Error()))
		return
	}

	// Delete exchange change state from local db
	if err := persistence.DeleteExchangeChangeState(w.db); err != nil {
		w.completedWithError(logString(err.Error()))
		return
	}

	// remove the docker volumes that are created by anax if device type is "device"
	if w.deviceType == persistence.DEVICE_TYPE_DEVICE {
		if err := container.DeleteLeftoverDockerVolumes(w.db, w.Config); err != nil {
			w.completedWithError(logString(err.Error()))
			return
		}
	}

	// Tell the system that node quiesce is complete without error. The API worker might be waiting for this message.
	// All the workers in the system will start quiescing as a result of this message.
	w.Messages() <- events.NewNodeShutdownCompleteMessage(events.UNCONFIGURE_COMPLETE, errorMessage)

	glog.V(3).Infof(logString(fmt.Sprintf("node shutdown process complete.")))
}

// This function is called due to the node pattern change on the exchange while
// the device is registered with a different pattern.
// It will remove the agreements, workloads etc, cleanup the exchange node and
// local device so that the node can be registered again.
func (w *GovernanceWorker) nodeShutDownForPattenChanged(dev *persistence.ExchangeDevice, new_pattern string) {

	glog.V(3).Infof(logString(fmt.Sprintf("begin node shutdown process for pattern change.")))

	// Clear the Pattern and RegisteredServices array in the node’s exchange resource. We have to leave the
	// public key so that the node can send messages to an agbot. Removing the pattern and RegisteredServices
	// will prevent the exchange from finding the node and thereby prevent agbots from trying to make new agreements.
	// It will keep the user input.
	if err := w.clearNodePatternAndMS(true); err != nil {
		w.continueWithError(logString(err.Error()))
	}

	// Cancel all agreements, all workload containers and networks will automatically terminate.
	if err := w.terminateAllAgreements(producer.TERM_REASON_NODE_PATTERN_CHANGED); err != nil {
		w.completedWithError(logString(err.Error()))
		return
	}

	// Remove the node’s messaging public key from the node’s exchange resource and delete the node’s message key pair from the filesystem.
	if err := w.patchNodeKey(w.GetHTTPFactory()); err != nil {
		w.continueWithError(logString(err.Error()))
	}

	// Add the new pattern name back to the node for the pattern change case, It is safe to add it back because the public keys are cleared
	// from the node
	if err := w.patchNodePattern(new_pattern); err != nil {
		w.continueWithError(logString(err.Error()))
		return
	}

	// Tell the blockchain workers to terminate blockchain containers. We will do this by telling the producer protocol handlers to shutdown.
	// Any protocol handlers that are using a blockchain will tell the blockchain worker to terminate.
	w.Messages() <- events.NewAllBlockchainShutdownMessage(events.ALL_STOP)

	// Tell running microservices to terminate.
	if err := w.terminateMicroservices(); err != nil {
		w.completedWithError(logString(err.Error()))
		return
	}

	// Remove any policy files from the filesystem.
	if err := w.deletePolicyFiles(); err != nil {
		w.completedWithError(logString(err.Error()))
		return
	}

	// change the device node pattern
	if err := w.updateHorizonDevice(dev, new_pattern); err != nil {
		w.completedWithError(logString(err.Error()))
		return
	}

	// remove the docker volumes that are created by anax if device type is "device"
	if w.deviceType == persistence.DEVICE_TYPE_DEVICE {
		if err := container.DeleteLeftoverDockerVolumes(w.db, w.Config); err != nil {
			w.completedWithError(logString(err.Error()))
			return
		}
	}

	// Tell the system that node quiesce is complete without error. The API worker might be waiting for this message.
	// All the workers in the system will start quiescing as a result of this message.
	w.Messages() <- events.NewNodeShutdownCompleteMessage(events.UNCONFIGURE_COMPLETE, "")

	glog.V(3).Infof(logString(fmt.Sprintf("node shutdown process for pattern change complete.")))
}

// Clear out the registered microservices/services and the configured pattern for the node.
// It also clears the userinput if keepUI is false.
func (w *GovernanceWorker) clearNodePatternAndMS(keepUI bool) error {

	// If the node entry has already been removed form the exchange, skip this step.
	_, err := exchange.GetExchangeDevice(w.limitedRetryEC.GetHTTPFactory(), w.GetExchangeId(), w.GetExchangeId(), w.GetExchangeToken(), w.GetExchangeURL())
	if err != nil && strings.Contains(err.Error(), "status: 401") {
		return nil
	} else if err != nil {
		return errors.New(fmt.Sprintf("error reading node from exchange: %v", err))
	}

	patchDevice := exchange.GetHTTPPatchDeviceHandler(w)

	// clear the registeredServices
	glog.V(3).Infof(logString(fmt.Sprintf("clearing node registerServices entry in exchange.")))
	pdrRS := exchange.PatchDeviceRequest{}
	tmpRS := make([]exchange.Microservice, 0)
	pdrRS.RegisteredServices = &tmpRS
	if err := patchDevice(w.GetExchangeId(), w.GetExchangeToken(), &pdrRS); err != nil {
		return errors.New(fmt.Sprintf("error clearing node registerServices entry in exchange. %v", err))
	} else {
		glog.V(3).Infof(logString(fmt.Sprintf("cleared node registeredServices entry in exchange.")))
	}

	// clear the pattern
	glog.V(3).Infof(logString(fmt.Sprintf("clearing node pattern entry in exchange.")))
	pdrPattern := exchange.PatchDeviceRequest{}
	pattern := ""
	pdrPattern.Pattern = &pattern
	if err := patchDevice(w.GetExchangeId(), w.GetExchangeToken(), &pdrPattern); err != nil {
		return errors.New(fmt.Sprintf("error clearing node pattern entry in exchange. %v", err))
	} else {
		glog.V(3).Infof(logString(fmt.Sprintf("cleared node pattern entry in exchange.")))
	}

	// clear the userInput if needed
	if !keepUI {
		// clear the user input
		glog.V(3).Infof(logString(fmt.Sprintf("clearing node userInput entry in exchange.")))
		pdrUI := exchange.PatchDeviceRequest{}
		tmpUI := make([]policy.UserInput, 0)
		pdrUI.UserInput = &tmpUI
		if err := patchDevice(w.GetExchangeId(), w.GetExchangeToken(), &pdrUI); err != nil {
			return errors.New(fmt.Sprintf("error clearing node userInput entry in exchange. %v", err))
		} else {
			glog.V(3).Infof(logString(fmt.Sprintf("cleared node userInput entry in exchange.")))
		}
	}

	return nil
}

// Terminate all active agreements and wait until they are all archived.
func (w *GovernanceWorker) terminateAllAgreements(reason string) error {
	// Create a new filter for active, unterminated agreements
	notYetFinalFilter := func() persistence.EAFilter {
		return func(a persistence.EstablishedAgreement) bool {
			return a.AgreementCreationTime != 0 && a.AgreementTerminatedTime == 0
		}
	}

	establishedAgreements, err := persistence.FindEstablishedAgreementsAllProtocols(w.db, policy.AllAgreementProtocols(), []persistence.EAFilter{persistence.UnarchivedEAFilter(), notYetFinalFilter()})
	if err != nil {
		return errors.New(fmt.Sprintf("unable to retrieve agreements from database, error: %v", err))
	}

	// Cancel all the agreements.
	for _, ag := range establishedAgreements {

		glog.V(3).Infof(logString(fmt.Sprintf("ending agreement: %v", ag.CurrentAgreementId)))
		pph := w.producerPH[ag.AgreementProtocol]
		reasonCode := pph.GetTerminationCode(reason)
		w.cancelAgreement(ag.CurrentAgreementId, ag.AgreementProtocol, reasonCode, pph.GetTerminationReason(reasonCode))

		// send the event to the container worker in case it has started workload containers.
		w.Messages() <- events.NewGovernanceWorkloadCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, ag.AgreementProtocol, ag.CurrentAgreementId, ag.GetDeploymentConfig())
		// clean up microservice instances, but make sure we dont upgrade any microservices as a result of agreement cancellation.
		skipUpgrade := true
		w.handleMicroserviceInstForAgEnded(ag.CurrentAgreementId, skipUpgrade)
	}

	// Wait until there are no active agreements in the local DB. Agreements dont get archived until the workload containers have stopped.
	runtime.Gosched()
	for {
		remainingAgreements, err := persistence.FindEstablishedAgreementsAllProtocols(w.db, policy.AllAgreementProtocols(), []persistence.EAFilter{persistence.UnarchivedEAFilter()})
		if err != nil {
			return errors.New(fmt.Sprintf("unable to retrieve agreements from database, error: %v", err))
		} else if len(remainingAgreements) != 0 {
			glog.V(3).Infof(logString(fmt.Sprintf("waiting for agreements to terminate, have %v", len(remainingAgreements))))
			time.Sleep(15 * time.Second)
		} else {
			glog.V(3).Infof(logString(fmt.Sprintf("all agreements terminated")))
			break
		}
	}
	return nil
}

// Terminate any remaining service/microservice containers. All ms(es) associated with an agreement should be gone. The
// remaining containers are the shared singleton containers.
func (w *GovernanceWorker) terminateMicroservices() error {
	// Get all unarchived service/microservice instances and ask them to terminate. Services/Microservices that have containers will be
	// cleaned up asynchronously so we have to wait to make sure they are all gone.
	ms_instances, err := persistence.FindMicroserviceInstances(w.db, []persistence.MIFilter{persistence.NotCleanedUpMIFilter(), persistence.UnarchivedMIFilter()})
	if err != nil {
		return errors.New(fmt.Sprintf("unable to retrieve service instances from database, error: %v", err))
	} else if ms_instances != nil {
		for _, msi := range ms_instances {
			glog.V(3).Infof(logString(fmt.Sprintf("terminating service instance %v.", msi.GetKey())))
			if err := w.CleanupMicroservice(msi.SpecRef, msi.Version, msi.GetKey(), 0); err != nil {
				return errors.New(fmt.Sprintf("unable to terminate service instances %v, error: %v", msi, err))
			}
		}
	}

	// Make sure they are all gone.
	runtime.Gosched()
	for {
		remainingInstances, err := persistence.FindMicroserviceInstances(w.db, []persistence.MIFilter{persistence.UnarchivedMIFilter()})
		if err != nil {
			return errors.New(fmt.Sprintf("unable to retrieve service instances from database, error: %v", err))
		} else if remainingInstances != nil && len(remainingInstances) != 0 {
			glog.V(3).Infof(logString(fmt.Sprintf("waiting for services to terminate, have %v, %v", len(remainingInstances), remainingInstances)))
			time.Sleep(15 * time.Second)
		} else {
			glog.V(3).Infof(logString(fmt.Sprintf("service instance termination complete")))
			break
		}
	}

	// Clean up all service/microservice definitions too.
	if msDefs, err := persistence.FindMicroserviceDefs(w.db, []persistence.MSFilter{persistence.UnarchivedMSFilter()}); err != nil {
		return errors.New(fmt.Sprintf("unable to retrieve service definitions from database, error: %v", err))
	} else {
		for _, mdi := range msDefs {
			if _, err := persistence.MsDefArchived(w.db, mdi.Id); err != nil {
				return errors.New(fmt.Sprintf("unable to archive service definition %v, error: %v", mdi, err))
			}
		}
	}
	glog.V(3).Infof(logString(fmt.Sprintf("service definition cleanup complete")))

	return nil
}

// Remove the messaging key so that no one tries to communicate with the node. If the node is already gone from the exchange, ignore the error.
func (w *GovernanceWorker) patchNodeKey(httpClientFactory *config.HTTPClientFactory) error {

	pdr := exchange.CreatePatchDeviceKey()
	pdr.PublicKey = []byte("")

	var resp interface{}
	resp = new(exchange.PutDeviceResponse)
	targetURL := w.GetExchangeURL() + "orgs/" + exchange.GetOrg(w.GetExchangeId()) + "/nodes/" + exchange.GetId(w.GetExchangeId())

	glog.V(3).Infof(logString(fmt.Sprintf("clearing messaging key in node entry: %v at %v", pdr, targetURL)))

	retryCount := httpClientFactory.RetryCount
	retryInterval := httpClientFactory.GetRetryInterval()

	for {
		if err, tpErr := exchange.InvokeExchange(httpClientFactory.NewHTTPClient(nil), "PATCH", targetURL, w.GetExchangeId(), w.GetExchangeToken(), pdr, &resp); err != nil {
			if strings.Contains(err.Error(), "status: 401") {
				break
			} else {
				return err
			}
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			if httpClientFactory.RetryCount == 0 {
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else if retryCount == 0 {
				// Break so that the rest of the function can do its cleanup.
				glog.Errorf(logString(fmt.Sprintf("exceeded %v retries trying to clear node messaging key for %v", httpClientFactory.RetryCount, tpErr)))
				break
			} else {
				retryCount--
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			}
		} else {
			glog.V(3).Infof(logString(fmt.Sprintf("cleared messaging key for device %v in exchange: %v", w.GetExchangeId(), resp)))
			break
		}
	}

	// Get rid of the keys on disk
	if err := exchange.DeleteKeys(""); err != nil {
		return err
	}

	glog.V(3).Infof(logString(fmt.Sprintf("deleted messaging keys from the node")))

	return nil

}

// Update the node pattern.
func (w *GovernanceWorker) patchNodePattern(pattern string) error {

	pdr := exchange.PatchDeviceRequest{}
	tmpPattern := pattern
	pdr.Pattern = &tmpPattern

	var resp interface{}
	resp = new(exchange.PutDeviceResponse)
	targetURL := w.GetExchangeURL() + "orgs/" + exchange.GetOrg(w.GetExchangeId()) + "/nodes/" + exchange.GetId(w.GetExchangeId())

	glog.V(3).Infof(logString(fmt.Sprintf("patch pattern in node entry: %v at %v", pdr, targetURL)))

	for {
		if err, tpErr := exchange.InvokeExchange(w.Config.Collaborators.HTTPClientFactory.NewHTTPClient(nil), "PATCH", targetURL, w.GetExchangeId(), w.GetExchangeToken(), pdr, &resp); err != nil {
			if strings.Contains(err.Error(), "status: 401") {
				break
			} else {
				return err
			}
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(3).Infof(logString(fmt.Sprintf("parched pattern for device %v in exchange: %v", w.GetExchangeId(), resp)))
			break
		}
	}

	return nil
}

// Remove all attributes from the DB.
func (w *GovernanceWorker) deleteAttributes() error {
	// Retrieve all attributes in the DB.
	attrs, err := persistence.FindApplicableAttributes(w.db, "", "")
	if err != nil {
		return errors.New(fmt.Sprintf("unable to retrieve attribute objects from database, error: %v", err))
	} else if attrs == nil || len(attrs) == 0 {
		return nil
	}

	// Delete them all
	for _, attr := range attrs {
		if _, err := persistence.DeleteAttribute(w.db, attr.GetMeta().Id); err != nil {
			glog.Errorf(logString(fmt.Sprintf("error deleting attribute object %v, error: %v", attr, err)))
		}
	}

	glog.V(3).Infof(logString(fmt.Sprintf("deleted all attributes from the node")))
	return nil
}

// Delete all policy files from the filesystem.
func (w *GovernanceWorker) deletePolicyFiles() error {

	if err := persistence.DeleteNodePolicy(w.db); err != nil {
		return errors.New(fmt.Sprintf("unable to delete node policy object from local database, error: %v", err))
	} else if err := policy.DeleteAllPolicyFiles(w.Config.Edge.PolicyPath, false); err != nil {
		return errors.New(fmt.Sprintf("unable to delete policy files from disk, error: %v", err))
	}
	glog.V(3).Infof(logString(fmt.Sprintf("deleted all policy files from the node")))
	return nil
}

// Delete the horizon device object.
func (w *GovernanceWorker) deleteHorizonDevice() error {
	if err := persistence.DeleteExchangeDevice(w.db); err != nil {
		return errors.New(fmt.Sprintf("unable to delete horizon device, error: %v", err))
	}
	glog.V(3).Infof(logString(fmt.Sprintf("deleted horizon device object")))
	return nil
}

// Update the config state and the pattern for the horizon device
func (w *GovernanceWorker) updateHorizonDevice(device *persistence.ExchangeDevice, pattern string) error {
	if _, err := device.SetConfigstate(w.db, device.Id, persistence.CONFIGSTATE_UNCONFIGURED); err != nil {
		return errors.New(fmt.Sprintf("unable to update the config state for horizon device, error: %v", err))
	} else {
		device.Config.State = persistence.CONFIGSTATE_UNCONFIGURED
	}
	if _, err := device.SetPattern(w.db, device.Id, pattern); err != nil {
		return errors.New(fmt.Sprintf("unable to update the pattern to %v for horizon device, error: %v", pattern, err))
	} else {
		device.Pattern = pattern
	}
	glog.V(3).Infof(logString(fmt.Sprintf("updated horizon device object")))
	return nil
}

// Delete the node from the exchange.
func (w *GovernanceWorker) deleteNode(httpClientFactory *config.HTTPClientFactory) error {

	var resp interface{}
	resp = new(exchange.PutDeviceResponse)
	targetURL := w.GetExchangeURL() + "orgs/" + exchange.GetOrg(w.GetExchangeId()) + "/nodes/" + exchange.GetId(w.GetExchangeId())

	glog.V(3).Infof(logString(fmt.Sprintf("deleting node %v from exchange", w.GetExchangeId())))

	retryCount := httpClientFactory.RetryCount
	retryInterval := httpClientFactory.GetRetryInterval()

	for {
		if err, tpErr := exchange.InvokeExchange(httpClientFactory.NewHTTPClient(nil), "DELETE", targetURL, w.GetExchangeId(), w.GetExchangeToken(), nil, &resp); err != nil {
			return err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			if httpClientFactory.RetryCount == 0 {
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else if retryCount == 0 {
				return errors.New(fmt.Sprintf("exceeded %v retries trying to delete node for %v", httpClientFactory.RetryCount, tpErr))
			} else {
				retryCount--
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			}
		} else {
			break
		}
	}

	glog.V(3).Infof(logString(fmt.Sprintf("deleted node from exchange")))
	return nil
}

// Delete the node error from the exchange and local db
func (w *GovernanceWorker) deleteNodeError() error {
	glog.V(3).Infof(logString(fmt.Sprintf("deleting node surface error for node %v from exchange", w.GetExchangeId())))
	err := exchange.DeleteSurfaceErrors(w.limitedRetryEC, w.GetExchangeId())
	if err != nil {
		return errors.New(fmt.Sprintf("error deleting node error from exchange: %v", err))
	}

	glog.V(3).Infof(logString(fmt.Sprintf("deleted node error from exchange")))

	glog.V(3).Infof(logString(fmt.Sprintf("deleting node surface error from local db")))
	if err := persistence.DeleteSurfaceErrors(w.db); err != nil {
		return err
	}

	glog.V(3).Infof(logString(fmt.Sprintf("deleted node surface error from local db")))
	return nil

}

// Send the shutdown completed message on the internal message bus.
func (w *GovernanceWorker) completedWithError(e string) {
	if e != "" {
		glog.Errorf(logString(fmt.Sprintf("node shutdown terminating with error: %v", e)))
	}
	w.Messages() <- events.NewNodeShutdownCompleteMessage(events.UNCONFIGURE_COMPLETE, e)
}

// Log the error but continue shutdown process
func (w *GovernanceWorker) continueWithError(e string) {
	if e != "" {
		glog.Errorf(logString(fmt.Sprintf("node shutdown continuing after encountering error: %v", e)))
	}
}
