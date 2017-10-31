package governance

import (
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/producer"
	"runtime"
	"time"
)

// This function will quiesce the anax system, getting rid of agreements, containers, networks, etc so that the node can be
// restarted and then reconfigured. It runs as its own go routine so that it can wait for asynchronous things to happen. It
// will return to caller but it must put a shutdown complete message on the internal message bus before returning. If this
// function ends in an error, the error will be in the shutdown complete message.
//
// There are other workers responsible for other functions, which will also so some cleanup when the Node Shutdown Message
// arrives. For example, the node heartbeat function is stopped by the Agreement worker.
func (w *GovernanceWorker) nodeShutdown(cmd *NodeShutdownCommand) {
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

	// Clear the Pattern and RegisteredMicroservices array in the node’s exchange resource. We have to leave the public key
	// so that the node can send messages to an agbot. Removing the pattern and RegisteredMicroservices will prevent the exchange
	// from finding the node and thereby prevent agobts from trying to make new agreements.
	if err := w.clearNodePatternAndMS(); err != nil {
		w.completedWithError(logString(err.Error()))
		return
	}

	// Cancel all agreements, all workload containers and networks will automatically terminate.
	if err := w.terminateAllAgreements(); err != nil {
		w.completedWithError(logString(err.Error()))
		return
	}

	// Remove the node’s messaging public key from the node’s exchange resource and delete the node’s message key pair from the filesystem.
	if err := w.patchNodeKey(); err != nil {
		w.completedWithError(logString(err.Error()))
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

	// Remove workload config from the database
	if err := w.deleteWorkloadconfig(); err != nil {
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
		if err := w.deleteNode(); err != nil {
			w.completedWithError(logString(err.Error()))
			return
		}
	}

	// Delete the HorizonDevice object from the local database.
	if err := w.deleteHorizonDevice(); err != nil {
		w.completedWithError(logString(err.Error()))
		return
	}

	// Tell the system that node quiesce is complete without error. The API worker might be waiting for this message.
	// All the workers in the system will start quiescing as a result of this message.
	w.Messages() <- events.NewNodeShutdownCompleteMessage(events.UNCONFIGURE_COMPLETE, "")

	glog.V(3).Infof(logString(fmt.Sprintf("node shutdown process complete.")))
}

// Clear out the registered microservices and the configured pattern for the node.
func (w *GovernanceWorker) clearNodePatternAndMS() error {
	exDev, err := exchange.GetExchangeDevice(w.Config.Collaborators.HTTPClientFactory, w.deviceId, w.deviceToken, w.Config.Edge.ExchangeURL)
	if err != nil {
		return errors.New(fmt.Sprintf("error reading node from exchange: %v", err))
	}

	// CreateDevicePut will include the existng message key in the returned object, and the Pattern field will be an empty string.
	// Preserve the rest of the existing fields on the PUT.
	pdr := exchange.CreateDevicePut(w.deviceToken, exDev.Name)
	pdr.RegisteredMicroservices = []exchange.Microservice{}
	pdr.SoftwareVersions = exDev.SoftwareVersions
	pdr.MsgEndPoint = exDev.MsgEndPoint

	var resp interface{}
	resp = new(exchange.PutDeviceResponse)
	targetURL := w.Manager.Config.Edge.ExchangeURL + "orgs/" + exchange.GetOrg(w.deviceId) + "/nodes/" + exchange.GetId(w.deviceId)

	glog.V(3).Infof(logString(fmt.Sprintf("clearing node entry in exchange: %v", pdr.ShortString())))

	for {
		if err, tpErr := exchange.InvokeExchange(w.Config.Collaborators.HTTPClientFactory.NewHTTPClient(nil), "PUT", targetURL, w.deviceId, w.deviceToken, pdr, &resp); err != nil {
			return err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(3).Infof(logString(fmt.Sprintf("cleared node entry in exchange: %v", resp)))
			return nil
		}
	}
}

// Terminate all active agreements and wait until they are all archived.
func (w *GovernanceWorker) terminateAllAgreements() error {
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
		reasonCode := pph.GetTerminationCode(producer.TERM_REASON_NODE_SHUTDOWN)
		w.cancelAgreement(ag.CurrentAgreementId, ag.AgreementProtocol, reasonCode, pph.GetTerminationReason(reasonCode))

		// send the event to the container worker in case it has started workload containers.
		w.Messages() <- events.NewGovernanceWorkloadCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, ag.AgreementProtocol, ag.CurrentAgreementId, ag.CurrentDeployment)
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

// Terminate any remaining microservice containers. All ms(es) associated with an agreement should be gone. The
// remaining containers are the shared singleton containers.
func (w *GovernanceWorker) terminateMicroservices() error {
	// Get all unarchived microservice instances and ask them to terminate. Microservices that have containers will be
	// cleaned up asynchronously so we have to wait to make sure they are all gone.
	ms_instances, err := persistence.FindMicroserviceInstances(w.db, []persistence.MIFilter{persistence.NotCleanedUpMIFilter(), persistence.UnarchivedMIFilter()})
	if err != nil {
		return errors.New(fmt.Sprintf("unable to retrieve microservice instances from database, error: %v", err))
	} else if ms_instances != nil {
		for _, msi := range ms_instances {
			glog.V(3).Infof(logString(fmt.Sprintf("terminating microservice instance %v.", msi.GetKey())))
			if err := w.CleanupMicroservice(msi.SpecRef, msi.Version, msi.GetKey(), 0); err != nil {
				return errors.New(fmt.Sprintf("unable to terminate microservice instances %v, error: %v", msi, err))
			}
		}
	}

	// Make sure they are all gone
	runtime.Gosched()
	for {
		remainingInstances, err := persistence.FindMicroserviceInstances(w.db, []persistence.MIFilter{persistence.UnarchivedMIFilter()})
		if err != nil {
			return errors.New(fmt.Sprintf("unable to retrieve microservice instances from database, error: %v", err))
		} else if remainingInstances != nil && len(remainingInstances) != 0 {
			glog.V(3).Infof(logString(fmt.Sprintf("waiting for microservices to terminate, have %v", len(remainingInstances))))
			time.Sleep(15 * time.Second)
		} else {
			glog.V(3).Infof(logString(fmt.Sprintf("microservice instance termination complete")))
			break
		}
	}

	// Clean up all microservice definitions too
	if msDefs, err := persistence.FindMicroserviceDefs(w.db, []persistence.MSFilter{persistence.UnarchivedMSFilter()}); err != nil {
		return errors.New(fmt.Sprintf("unable to retrieve microservice definitions from database, error: %v", err))
	} else {
		for _, mdi := range msDefs {
			if _, err := persistence.MsDefArchived(w.db, mdi.Id); err != nil {
				return errors.New(fmt.Sprintf("unable to archive microservice definition %v, error: %v", mdi, err))
			}
		}
	}
	glog.V(3).Infof(logString(fmt.Sprintf("microservice definition cleanup complete")))

	return nil
}

// Remove the messaging key so that no one tries to communicate with the node.
func (w *GovernanceWorker) patchNodeKey() error {

	pdr := exchange.CreatePatchDeviceKey()
	pdr.PublicKey = []byte("")

	var resp interface{}
	resp = new(exchange.PutDeviceResponse)
	targetURL := w.Manager.Config.Edge.ExchangeURL + "orgs/" + exchange.GetOrg(w.deviceId) + "/nodes/" + exchange.GetId(w.deviceId)

	glog.V(3).Infof(logString(fmt.Sprintf("clearing messaging key in node entry: %v at %v", pdr, targetURL)))

	for {
		if err, tpErr := exchange.InvokeExchange(w.Config.Collaborators.HTTPClientFactory.NewHTTPClient(nil), "PATCH", targetURL, w.deviceId, w.deviceToken, pdr, &resp); err != nil {
			return err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(3).Infof(logString(fmt.Sprintf("cleared messaging key for device %v in exchange: %v", w.deviceId, resp)))
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

// Remove all workload config records from the DB.
func (w *GovernanceWorker) deleteWorkloadconfig() error {
	wcs, err := persistence.FindWorkloadConfigs(w.db, []persistence.WCFilter{})
	if err != nil {
		return errors.New(fmt.Sprintf("unable to retrieve workload config objects from database, error: %v", err))
	} else if wcs == nil {
		return nil
	}

	for _, wc := range wcs {
		if err := persistence.DeleteWorkloadConfig(w.db, wc.WorkloadURL, wc.VersionExpression); err != nil {
			glog.Errorf(logString(fmt.Sprintf("error deleting workload config object %v, error: %v", wc, err)))
		}
	}

	glog.V(3).Infof(logString(fmt.Sprintf("deleted all workloadconfig objects")))

	return nil
}

// Remove all attributes from the DB.
func (w *GovernanceWorker) deleteAttributes() error {
	// Retrieve all attributes in the DB.
	attrs, err := persistence.FindApplicableAttributes(w.db, "")
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
	if err := policy.DeleteAllPolicyFiles(w.Config.Edge.PolicyPath); err != nil {
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

// Delete the node from the exchange.
func (w *GovernanceWorker) deleteNode() error {

	var resp interface{}
	resp = new(exchange.PutDeviceResponse)
	targetURL := w.Manager.Config.Edge.ExchangeURL + "orgs/" + exchange.GetOrg(w.deviceId) + "/nodes/" + exchange.GetId(w.deviceId)

	glog.V(3).Infof(logString(fmt.Sprintf("deleting node %v from exchange", w.deviceId)))

	for {
		if err, tpErr := exchange.InvokeExchange(w.Config.Collaborators.HTTPClientFactory.NewHTTPClient(nil), "DELETE", targetURL, w.deviceId, w.deviceToken, nil, &resp); err != nil {
			return err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			break
		}
	}

	glog.V(3).Infof(logString(fmt.Sprintf("deleted node from exchange")))
	return nil
}

// Send the shutdown completed message on the internal message bus.
func (w *GovernanceWorker) completedWithError(e string) {
	if e != "" {
		glog.Errorf(logString(fmt.Sprintf("node shutdown terminating with error: %v", e)))
	}
	w.Messages() <- events.NewNodeShutdownCompleteMessage(events.UNCONFIGURE_COMPLETE, e)
}
