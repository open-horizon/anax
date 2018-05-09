package governance

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/microservice"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/producer"
	"net/url"
	"strings"
	"time"
)

// This function runs periodically in a separate process. It checks if the service/microservice containers are up and running and
// if new service/microservice versions are available for upgrade.
func (w *GovernanceWorker) governMicroservices() int {

	if w.Config.Edge.ServiceUpgradeCheckIntervalS > 0 {
		// get the microservice upgrade check interval
		check_interval := w.Config.Edge.ServiceUpgradeCheckIntervalS

		// check for the new microservice version when time is right
		time_now := time.Now().Unix()
		if time_now-w.lastSvcUpgradeCheck >= int64(check_interval) {
			w.lastSvcUpgradeCheck = time_now

			// handle microservice upgrade. The upgrade includes inactive upgrades if the associated agreements happen to be 0.
			glog.V(4).Infof(logString(fmt.Sprintf("governing service upgrades")))
			if ms_defs, err := persistence.FindMicroserviceDefs(w.db, []persistence.MSFilter{persistence.UnarchivedMSFilter()}); err != nil {
				glog.Errorf(logString(fmt.Sprintf("Error getting service definitions from db. %v", err)))
			} else if ms_defs != nil && len(ms_defs) > 0 {
				for _, ms := range ms_defs {
					// upgrade the microserice if needed
					cmd := w.NewUpgradeMicroserviceCommand(ms.Id)
					w.Commands <- cmd
				}
			}
		}
	}

	// check if microservice instance containers are down
	glog.V(4).Infof(logString(fmt.Sprintf("governing service containers")))
	if ms_instances, err := persistence.FindMicroserviceInstances(w.db, []persistence.MIFilter{persistence.AllMIFilter(), persistence.UnarchivedMIFilter()}); err != nil {
		glog.Errorf(logString(fmt.Sprintf("Error retrieving all service instances from database, error: %v", err)))
	} else if ms_instances != nil {
		for _, msi := range ms_instances {
			// only check the ones that has containers started already and not in the middle of cleanup
			if hasWL, _ := msi.HasWorkload(w.db); hasWL && msi.ExecutionStartTime != 0 && msi.CleanupStartTime == 0 {
				glog.V(3).Infof(logString(fmt.Sprintf("fire event to ensure service containers are still up for service instance %v.", msi.GetKey())))

				// ensure containers are still running
				w.Messages() <- events.NewMicroserviceMaintenanceMessage(events.CONTAINER_MAINTAIN, msi.GetKey())
			}
		}
	}
	return 0
}

// It creates microservice instance and loads the containers for the given microservice def
func (w *GovernanceWorker) StartMicroservice(ms_key string, agreementId string, dependencyPath []persistence.ServiceInstancePathElement) (*persistence.MicroserviceInstance, error) {
	glog.V(5).Infof(logString(fmt.Sprintf("Starting service instance for %v", ms_key)))
	if msdef, err := persistence.FindMicroserviceDefWithKey(w.db, ms_key); err != nil {
		return nil, fmt.Errorf(logString(fmt.Sprintf("Error finding service definition from db with key %v. %v", ms_key, err)))
	} else if msdef == nil {
		return nil, fmt.Errorf(logString(fmt.Sprintf("No service definition available for key %v.", ms_key)))
	} else {
		if !msdef.HasDeployment() {
			glog.Infof(logString(fmt.Sprintf("No workload needed for service %v.", msdef.SpecRef)))
			if mi, err := persistence.NewMicroserviceInstance(w.db, msdef.SpecRef, msdef.Version, ms_key, dependencyPath); err != nil {
				return nil, fmt.Errorf(logString(fmt.Sprintf("Error persisting service instance for %v %v %v.", msdef.SpecRef, msdef.Version, ms_key)))
				// if the new microservice does not have containers, just mark it done.
			} else if mi, err := persistence.UpdateMSInstanceExecutionState(w.db, mi.GetKey(), true, 0, ""); err != nil {
				return nil, fmt.Errorf(logString(fmt.Sprintf("Failed to update the ExecutionStartTime for service instance %v. %v", mi.GetKey(), err)))
			} else {
				return mi, nil
			}
		}

		deployment, deploymentSig, torr := msdef.GetDeployment()

		// convert the torrent string to a structure
		var torrent policy.Torrent

		// convert to torrent structure only if the torrent string exists on the exchange
		if torr != "" {
			if err := json.Unmarshal([]byte(torr), &torrent); err != nil {
				return nil, fmt.Errorf(logString(fmt.Sprintf("The torrent definition for service %v has error: %v", msdef.SpecRef, err)))
			}
		} else {
			// this is the service case where the image server is defined
			if image_store := msdef.GetImageStore(); len(image_store) > 0 {
				if st, ok := image_store[exchange.IMPL_PACKAGE_DISCRIMINATOR]; ok && st == exchange.IMPL_PACKAGE_IMAGESERVER {
					if url, ok1 := image_store["url"]; ok1 {
						torrent.Url = url.(string)
					}
					if sig, ok2 := image_store["signature"]; ok2 {
						torrent.Signature = sig.(string)
					}
				}
			}
		}

		// convert workload to policy workload structure
		var ms_workload policy.Workload
		ms_workload.Deployment = deployment
		ms_workload.DeploymentSignature = deploymentSig
		ms_workload.Torrent = torrent
		ms_workload.WorkloadPassword = ""
		ms_workload.DeploymentUserInfo = ""

		// verify torrent url
		if url, err := url.Parse(torrent.Url); err != nil {
			return nil, fmt.Errorf("ill-formed URL: %v, error %v", torrent.Url, err)
		} else {
			// get microservice/service keys and save it to the user keys.
			if w.Config.Edge.TrustCertUpdatesFromOrg {
				key_map, err := exchange.GetHTTPObjectSigningKeysHandler(w)(exchange.MICROSERVICE, msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch)
				if err == nil {
					// No error means that we are working with a microservice.
				} else if key_map, err = exchange.GetHTTPObjectSigningKeysHandler(w)(exchange.SERVICE, msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch); err != nil {
					return nil, fmt.Errorf(logString(fmt.Sprintf("received error getting signing keys from the exchange: %v %v %v %v. %v", msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch, err)))
				}

				if key_map != nil {
					errHandler := func(keyname string) api.ErrorHandler {
						return func(err error) bool {
							glog.Errorf(logString(fmt.Sprintf("received error when saving the signing key file %v to anax. %v", keyname, err)))
							return true
						}
					}

					for key, content := range key_map {
						//add .pem the end of the keyname if it does not have none.
						fn := key
						if !strings.HasSuffix(key, ".pem") {
							fn = fmt.Sprintf("%v.pem", key)
						}

						api.UploadPublicKey(fn, []byte(content), w.Config, errHandler(fn))
					}
				}
			}

			// Verify the deployment signature
			if pemFiles, err := w.Config.Collaborators.KeyFileNamesFetcher.GetKeyFileNames(w.Config.Edge.PublicKeyPath, w.Config.UserPublicKeyPath()); err != nil {
				return nil, fmt.Errorf(logString(fmt.Sprintf("received error getting pem key files: %v", err)))
			} else if err := ms_workload.HasValidSignature(pemFiles); err != nil {
				return nil, fmt.Errorf(logString(fmt.Sprintf("service container has invalid deployment signature %v for %v", ms_workload.DeploymentSignature, ms_workload.Deployment)))
			}

			// Gather up the service dependencies, if there are any. Microservices in the workload/microservice model never have dependencies,
			// but services can. It is important to use the correct version for the service dependency, which is the version we have
			// in the local database, not necessarily the version in the dependency. The version we have in the local database should always
			// be greater than the dependency version.
			ms_specs := []events.MicroserviceSpec{}
			for _, rs := range msdef.RequiredServices {
				if msdefs, err := persistence.FindUnarchivedMicroserviceDefs(w.db, rs.URL); err != nil {
					return nil, fmt.Errorf(logString(fmt.Sprintf("received error reading service definition for %v: %v", rs.URL, err)))
				} else {
					// Assume the first msdef is the one we want.
					msspec := events.MicroserviceSpec{SpecRef: rs.URL, Version: msdefs[0].Version, MsdefId: msdefs[0].Id}
					ms_specs = append(ms_specs, msspec)
				}
			}

			// save the instance
			if ms_instance, err := persistence.NewMicroserviceInstance(w.db, msdef.SpecRef, msdef.Version, ms_key, dependencyPath); err != nil {
				return nil, fmt.Errorf(logString(fmt.Sprintf("Error persisting service instance for %v %v %v.", msdef.SpecRef, msdef.Version, ms_key)))
			} else {
				// get the image auth for service (we have to try even for microservice because we do not know if this is ms or svc.)
				img_auths := make([]events.ImageDockerAuth, 0)
				if w.Config.Edge.TrustDockerAuthFromOrg {
					if ias, err := exchange.GetHTTPServiceDockerAuthsHandler(w)(msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch); err != nil {
						glog.V(5).Infof(logString(fmt.Sprintf("received error querying exchange for service image auths: %v version %v, error %v", msdef.SpecRef, msdef.Version, err)))
					} else {
						if ias != nil {
							for _, iau_temp := range ias {
								img_auths = append(img_auths, events.ImageDockerAuth{Registry: iau_temp.Registry, UserName: "token", Password: iau_temp.Token})
							}
						}
					}
				}

				// Fire an event to the torrent worker so that it will download the container
				cc := events.NewContainerConfig(*url, ms_workload.Torrent.Signature, ms_workload.Deployment, ms_workload.DeploymentSignature, ms_workload.DeploymentUserInfo, "", img_auths)

				// convert the user input from the service attributes to env variables
				if attrs, err := persistence.FindApplicableAttributes(w.db, msdef.SpecRef); err != nil {
					return nil, fmt.Errorf(logString(fmt.Sprintf("Unable to fetch service preferences for %v. Err: %v", msdef.SpecRef, err)))
				} else if envAdds, err := persistence.AttributesToEnvvarMap(attrs, make(map[string]string), config.ENVVAR_PREFIX, w.Config.Edge.DefaultServiceRegistrationRAM); err != nil {
					return nil, fmt.Errorf(logString(fmt.Sprintf("Failed to convert service preferences to environmental variables for %v. Err: %v", msdef.SpecRef, err)))
				} else {
					envAdds[config.ENVVAR_PREFIX+"DEVICE_ID"] = exchange.GetId(w.GetExchangeId())
					envAdds[config.ENVVAR_PREFIX+"ORGANIZATION"] = exchange.GetOrg(w.GetExchangeId())
					envAdds[config.ENVVAR_PREFIX+"EXCHANGE_URL"] = w.Config.Edge.ExchangeURL
					// Add in any default variables from the microservice userInputs that havent been overridden
					for _, ui := range msdef.UserInputs {
						if ui.DefaultValue != "" {
							if _, ok := envAdds[ui.Name]; !ok {
								envAdds[ui.Name] = ui.DefaultValue
							}
						}
					}
					lc := events.NewContainerLaunchContext(cc, &envAdds, events.BlockchainConfig{}, ms_instance.GetKey(), agreementId, ms_specs, persistence.NewServiceInstancePathElement(msdef.SpecRef, msdef.Version))
					w.Messages() <- events.NewLoadContainerMessage(events.LOAD_CONTAINER, lc)
				}

				return ms_instance, nil // assume there is only one workload for a microservice
			}
		}
	}

}

// It cleans the microservice instance and its associated agreements
func (w *GovernanceWorker) CleanupMicroservice(spec_ref string, version string, inst_key string, ms_reason_code uint) error {
	glog.V(5).Infof(logString(fmt.Sprintf("Deleting service instance %v", inst_key)))

	// archive this microservice instance in the db
	if ms_inst, err := persistence.MicroserviceInstanceCleanupStarted(w.db, inst_key); err != nil {
		glog.Errorf(logString(fmt.Sprintf("Error setting cleanup start time for service instance %v. %v", inst_key, err)))
		return fmt.Errorf(logString(fmt.Sprintf("Error setting cleanup start time for service instance %v. %v", inst_key, err)))
	} else if ms_inst == nil {
		glog.Errorf(logString(fmt.Sprintf("Unable to find service instance %v.", inst_key)))
		return fmt.Errorf(logString(fmt.Sprintf("Unable to find service instance %v.", inst_key)))
		// remove all the containers for agreements associated with it so that new agreements can be created over the new microservice
	} else if agreements, err := w.FindEstablishedAgreementsWithIds(ms_inst.AssociatedAgreements); err != nil {
		glog.Errorf(logString(fmt.Sprintf("Error finding agreements %v from the db. %v", ms_inst.AssociatedAgreements, err)))
		return fmt.Errorf(logString(fmt.Sprintf("Error finding agreements %v from the db. %v", ms_inst.AssociatedAgreements, err)))
	} else if agreements != nil {
		// If this function is called by the only clean up the workload containers for the agreement
		glog.V(5).Infof(logString(fmt.Sprintf("Removing all the containers for associated agreements %v", ms_inst.AssociatedAgreements)))
		for _, ag := range agreements {
			// send the event to the container so that the workloads can be deleted
			w.Messages() <- events.NewGovernanceWorkloadCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, ag.AgreementProtocol, ag.CurrentAgreementId, ag.CurrentDeployment)

			var ag_reason_code uint
			switch ms_reason_code {
			case microservice.MS_IMAGE_LOAD_FAILED:
				ag_reason_code = w.producerPH[ag.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_MS_IMAGE_LOAD_FAILURE)
			case microservice.MS_DELETED_BY_UPGRADE_PROCESS:
				ag_reason_code = w.producerPH[ag.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_MS_UPGRADE_REQUIRED)
			case microservice.MS_DELETED_BY_DOWNGRADE_PROCESS:
				ag_reason_code = w.producerPH[ag.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_MS_DOWNGRADE_REQUIRED)
			case microservice.MS_IMAGE_FETCH_FAILED:
				ag_reason_code = w.producerPH[ag.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_IMAGE_FETCH_FAILURE)
			default:
				ag_reason_code = w.producerPH[ag.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_MICROSERVICE_FAILURE)
			}
			ag_reason_text := w.producerPH[ag.AgreementProtocol].GetTerminationReason(ag_reason_code)

			// end the agreements
			glog.V(3).Infof(logString(fmt.Sprintf("Ending the agreement: %v because service %v is deleted", ag.CurrentAgreementId, inst_key)))
			w.cancelAgreement(ag.CurrentAgreementId, ag.AgreementProtocol, ag_reason_code, ag_reason_text)
		}

		// remove all the microservice containers if any
		if has_wl, err := ms_inst.HasWorkload(w.db); err != nil {
			return fmt.Errorf(logString(fmt.Sprintf("Error checking if the service %v has workload. %v", ms_inst.GetKey(), err)))
		} else if has_wl {
			glog.V(5).Infof(logString(fmt.Sprintf("Removing all the containers for %v", inst_key)))
			w.Messages() <- events.NewMicroserviceCancellationMessage(events.CANCEL_MICROSERVICE, inst_key)
		}
	}

	// archive this microservice instance
	if _, err := persistence.ArchiveMicroserviceInstance(w.db, inst_key); err != nil {
		glog.Errorf(logString(fmt.Sprintf("Error archiving service instance %v. %v", inst_key, err)))
		return fmt.Errorf(logString(fmt.Sprintf("Error archiving service instance %v. %v", inst_key, err)))
	}

	return nil
}

// It changes the current running microservice from the old to new, assuming the given microservice is ready for a change.
// One can check it by calling microservice.MicroserviceReadyForUpgrade to find out.
func (w *GovernanceWorker) UpgradeMicroservice(msdef *persistence.MicroserviceDefinition, new_msdef *persistence.MicroserviceDefinition, upgrade bool) error {
	glog.V(3).Infof(logString(fmt.Sprintf("Start changing service %v from version %v to version %v", msdef.SpecRef, msdef.Version, new_msdef.Version)))

	// archive the old ms def and save the new one to db
	if _, err := persistence.MsDefArchived(w.db, msdef.Id); err != nil {
		return fmt.Errorf(logString(fmt.Sprintf("Failed to archived service definition %v. %v", msdef, err)))
	} else if err := persistence.SaveOrUpdateMicroserviceDef(w.db, new_msdef); err != nil {
		return fmt.Errorf(logString(fmt.Sprintf("Failed to save service definition to db. %v. %v", new_msdef, err)))
	} else if _, err := persistence.MSDefUpgradeNewMsId(w.db, msdef.Id, new_msdef.Id); err != nil {
		return fmt.Errorf(logString(fmt.Sprintf("Failed to update the UpgradeNewMsId to %v for service def %v version %v id %v. %v", new_msdef.Id, msdef.SpecRef, msdef.Version, msdef.Id, err)))
	} else if _, err := persistence.MSDefUpgradeStarted(w.db, new_msdef.Id); err != nil {
		return fmt.Errorf(logString(fmt.Sprintf("Failed to update the UpgradeStartTime for service def %v version %v id %v. %v", new_msdef.SpecRef, new_msdef.Version, new_msdef.Id, err)))
	}

	// clean up old microservice
	var eClearError error
	var ms_insts []persistence.MicroserviceInstance
	eClearError = nil
	if ms_insts, eClearError = persistence.FindMicroserviceInstances(w.db, []persistence.MIFilter{persistence.AllInstancesMIFilter(msdef.SpecRef, msdef.Version), persistence.UnarchivedMIFilter()}); eClearError != nil {
		glog.Errorf(logString(fmt.Sprintf("Error retrieving all the service instances from db for %v version %v key %v. %v", msdef.SpecRef, msdef.Version, msdef.Id, eClearError)))
	} else if ms_insts != nil && len(ms_insts) > 0 {
		for _, msi := range ms_insts {
			if msi.MicroserviceDefId == msdef.Id {
				cleanup_reason := microservice.MS_DELETED_BY_UPGRADE_PROCESS
				if !upgrade {
					cleanup_reason = microservice.MS_DELETED_BY_DOWNGRADE_PROCESS
				}
				if eClearError = w.CleanupMicroservice(msdef.SpecRef, msdef.Version, msi.GetKey(), uint(cleanup_reason)); eClearError != nil {
					glog.Errorf(logString(fmt.Sprintf("Error cleanup service instances %v. %v", msi.GetKey(), eClearError)))
				}
			}
		}
	}
	// update msdef UpgradeAgreementsClearedTime
	if eClearError != nil {
		if _, err := persistence.MSDefUpgradeFailed(w.db, new_msdef.Id, microservice.MS_CLEAR_OLD_AGS_FAILED, microservice.DecodeReasonCode(microservice.MS_CLEAR_OLD_AGS_FAILED)); err != nil {
			return fmt.Errorf(logString(fmt.Sprintf("Failed to update the UpgradeAgreementsClearedTime for service def %v version %v id %v. %v", new_msdef.SpecRef, new_msdef.Version, new_msdef.Id, err)))
		}
	} else {
		if _, err := persistence.MsDefUpgradeAgreementsCleared(w.db, new_msdef.Id); err != nil {
			return fmt.Errorf(logString(fmt.Sprintf("Failed to update the UpgradeAgreementsClearedTime for service def %v version %v id %v. %v", new_msdef.SpecRef, new_msdef.Version, new_msdef.Id, err)))
		}
	}

	// unregister the old ms from exchange
	var unregError error
	unregError = nil
	unregError = microservice.RemoveMicroservicePolicy(msdef.SpecRef, msdef.Org, msdef.Version, msdef.Id, w.Config.Edge.PolicyPath, w.pm)
	if unregError != nil {
		glog.Errorf(logString(fmt.Sprintf("Failed to remove service policy for service def %v version %v. %v", msdef.SpecRef, msdef.Version, unregError)))
	} else if unregError = microservice.UnregisterMicroserviceExchange(exchange.GetHTTPDeviceHandler(w), exchange.GetHTTPPutDeviceHandler(w), msdef.SpecRef, w.GetServiceBased(), w.GetExchangeId(), w.GetExchangeToken(), w.db); unregError != nil {
		glog.Errorf(logString(fmt.Sprintf("Failed to unregister service from the exchange for service def %v. %v", msdef.SpecRef, unregError)))
	}

	// update msdef UpgradeMsUnregisteredTime
	if unregError != nil {
		if _, err := persistence.MSDefUpgradeFailed(w.db, new_msdef.Id, microservice.MS_UNREG_EXCH_FAILED, microservice.DecodeReasonCode(microservice.MS_UNREG_EXCH_FAILED)); err != nil {
			return fmt.Errorf(logString(fmt.Sprintf("Failed to update service upgrading failure reason for service def %v version %v id %v. %v", new_msdef.SpecRef, new_msdef.Version, new_msdef.Id, err)))
		}
	} else {
		if _, err := persistence.MSDefUpgradeMsUnregistered(w.db, new_msdef.Id); err != nil {
			return fmt.Errorf(logString(fmt.Sprintf("Failed to update the UpgradeMsUnregisteredTime for service def %v version %v id %v. %v", new_msdef.SpecRef, new_msdef.Version, new_msdef.Id, err)))
		}
	}

	// create a new policy file and register the new microservice in exchange
	if err := microservice.GenMicroservicePolicy(new_msdef, w.Config.Edge.PolicyPath, w.db, w.Messages(), exchange.GetOrg(w.GetExchangeId())); err != nil {
		if _, err := persistence.MSDefUpgradeFailed(w.db, new_msdef.Id, microservice.MS_REREG_EXCH_FAILED, microservice.DecodeReasonCode(microservice.MS_REREG_EXCH_FAILED)); err != nil {
			return fmt.Errorf(logString(fmt.Sprintf("Failed to update service upgrading failure reason for service def %v version %v id %v. %v", new_msdef.SpecRef, new_msdef.Version, new_msdef.Id, err)))
		}
	} else {
		if _, err := persistence.MSDefUpgradeMsReregistered(w.db, new_msdef.Id); err != nil {
			return fmt.Errorf(logString(fmt.Sprintf("Failed to update the UpgradeMsReregisteredTime for service def %v version %v id %v. %v", new_msdef.SpecRef, new_msdef.Version, new_msdef.Id, err)))
		}
	}

	// done for the microservices without containers.
	glog.V(3).Infof(logString(fmt.Sprintf("End changing service %v version %v key %v", msdef.SpecRef, msdef.Version, msdef.Id)))

	return nil
}

// Get the next highest microservice version and rollback to it. Tryer even lower version if it fails
func (w *GovernanceWorker) RollbackMicroservice(msdef *persistence.MicroserviceDefinition) error {
	for true {
		// get next lower version
		if new_msdef, err := microservice.GetRollbackMicroserviceDef(exchange.GetHTTPMicroserviceHandler(w), msdef, w.db); err != nil {
			return fmt.Errorf(logString(fmt.Sprintf("Error finding the new service definition to downgrade to for %v %v version key %v. error: %v", msdef.SpecRef, msdef.Version, msdef.Id, err)))
		} else if new_msdef == nil { //no more to try, exit out
			glog.Warningf(logString(fmt.Sprintf("Unable to find the service definition to downgrade to for %v %v version key %v. error: %v", msdef.SpecRef, msdef.Version, msdef.Id, err)))
			return nil
		} else if err := w.UpgradeMicroservice(msdef, new_msdef, false); err != nil {
			glog.Errorf(logString(fmt.Sprintf("Failed to downgrade %v from version %v key %v to version %v key %v. %v", msdef.SpecRef, msdef.Version, msdef.Id, new_msdef.Version, new_msdef.Id, err)))
			msdef = new_msdef
		} else {
			return nil
		}
	}

	return nil
}

// Start a servic/microservice instance for the given agreement according to the sharing mode.
func (w *GovernanceWorker) startMicroserviceInstForAgreement(msdef *persistence.MicroserviceDefinition, agreementId string, dependencyPath []persistence.ServiceInstancePathElement, protocol string) error {
	glog.V(3).Infof(logString(fmt.Sprintf("start service instance %v for agreement %v", msdef.SpecRef, agreementId)))

	var msi *persistence.MicroserviceInstance
	needs_new_ms := false

	// Always start a new instance if the sharing mode is multiple.
	if msdef.Sharable == exchange.MS_SHARING_MODE_MULTIPLE {
		needs_new_ms = true
		// For other sharing modes, start a new instance only if there is no existing one.
		// The "exclusive" sharing mode is handled by maxAgreements=1 in the node side policy file. This ensures that agbots and nodes will
		// only support one agreement at any time.
	} else if ms_insts, err := persistence.FindMicroserviceInstances(w.db, []persistence.MIFilter{persistence.AllInstancesMIFilter(msdef.SpecRef, msdef.Version), persistence.UnarchivedMIFilter()}); err != nil {
		return fmt.Errorf(logString(fmt.Sprintf("Error retrieving all the service instances from db for %v version %v key %v. %v", msdef.SpecRef, msdef.Version, msdef.Id, err)))
	} else if ms_insts == nil || len(ms_insts) == 0 {
		needs_new_ms = true
	} else {
		msi = &ms_insts[0]
	}

	if needs_new_ms {
		var inst_err error
		if msi, inst_err = w.StartMicroservice(msdef.Id, agreementId, dependencyPath); inst_err != nil {

			// Try to downgrade the service/microservice to a lower version.
			glog.V(3).Infof(logString(fmt.Sprintf("Ending the agreement: %v because service %v failed to start", agreementId, msdef.SpecRef)))
			ag_reason_code := w.producerPH[protocol].GetTerminationCode(producer.TERM_REASON_MS_DOWNGRADE_REQUIRED)
			ag_reason_text := w.producerPH[protocol].GetTerminationReason(ag_reason_code)
			if agreementId != "" {
				w.cancelAgreement(agreementId, protocol, ag_reason_code, ag_reason_text)
			}

			glog.V(3).Infof(logString(fmt.Sprintf("Downgrading service %v because version %v key %v failed to start. Error: %v", msdef.SpecRef, msdef.Version, msdef.Id, inst_err)))
			if err := w.RollbackMicroservice(msdef); err != nil {
				glog.Errorf(logString(fmt.Sprintf("Error downgrading service %v version %v key %v. %v", msdef.SpecRef, msdef.Version, msdef.Id, err)))
			}

			return fmt.Errorf(logString(fmt.Sprintf("Failed to start service instance for %v version %v key %v. %v", msdef.SpecRef, msdef.Version, msdef.Id, inst_err)))
		}
	} else if _, err := persistence.UpdateMSInstanceAddDependencyPath(w.db, msi.GetKey(), &dependencyPath); err != nil {
		return fmt.Errorf(logString(fmt.Sprintf("error adding dependency path %v to the service %v: %v", dependencyPath, msi.GetKey(), err)))
	}

	// Add the agreement id into the instance so that the workload containers know which instance to associate with.
	if agreementId != "" {
		if _, err := persistence.UpdateMSInstanceAssociatedAgreements(w.db, msi.GetKey(), true, agreementId); err != nil {
			return fmt.Errorf(logString(fmt.Sprintf("error adding agreement id %v to the service %v: %v", agreementId, msi.GetKey(), err)))
		}
	} else {
		if _, err := persistence.UpdateMSInstanceAgreementLess(w.db, msi.GetKey()); err != nil {
			return fmt.Errorf(logString(fmt.Sprintf("error setting agreement-less on the service %v: %v", msi.GetKey(), err)))
		}
	}

	return nil
}

// process microservice instance after an agreement is ended.
func (w *GovernanceWorker) handleMicroserviceInstForAgEnded(agreementId string, skipUpgrade bool) {
	glog.V(3).Infof(logString(fmt.Sprintf("handle service instance for agreement %v ended.", agreementId)))

	// delete the agreement from the microservice instance and upgrade the microservice if needed
	if ms_instances, err := persistence.FindMicroserviceInstances(w.db, []persistence.MIFilter{persistence.UnarchivedMIFilter()}); err != nil {
		glog.Errorf(logString(fmt.Sprintf("error retrieving all service instances from database, error: %v", err)))
	} else if ms_instances != nil {
		for _, msi := range ms_instances {
			if msi.AssociatedAgreements != nil && len(msi.AssociatedAgreements) > 0 {
				for _, id := range msi.AssociatedAgreements {
					if id == agreementId {
						msd, err := persistence.FindMicroserviceDefWithKey(w.db, msi.MicroserviceDefId)
						if err != nil {
							glog.Errorf(logString(fmt.Sprintf("Error retrieving service definition %v version %v key %v from database, error: %v", msi.SpecRef, msi.Version, msi.MicroserviceDefId, err)))
							// delete the microservice instance if the sharing mode is "multiple"
						} else {
							if msd.Sharable == exchange.MS_SHARING_MODE_MULTIPLE {
								// mark the ms clean up started and remove all the microservice containers if any
								if _, err := persistence.MicroserviceInstanceCleanupStarted(w.db, msi.GetKey()); err != nil {
									glog.Errorf(logString(fmt.Sprintf("Error setting cleanup start time for service instance %v. %v", msi.GetKey(), err)))
								} else if has_wl, err := msi.HasWorkload(w.db); err != nil {
									glog.Errorf(logString(fmt.Sprintf("Error checking if the service %v has workload. %v", msi.GetKey(), err)))
								} else if has_wl {
									// the ms instance will be archived after the microservice containers are destroyed.
									glog.V(5).Infof(logString(fmt.Sprintf("Removing all the containers for %v", msi.GetKey())))
									w.Messages() <- events.NewMicroserviceCancellationMessage(events.CANCEL_MICROSERVICE, msi.GetKey())
								} else if _, err := persistence.ArchiveMicroserviceInstance(w.db, msi.GetKey()); err != nil {
									glog.Errorf(logString(fmt.Sprintf("Error archiving service instance %v. %v", msi.GetKey(), err)))
								}
								//remove the agreement from the microservice instance
							} else if _, err := persistence.UpdateMSInstanceAssociatedAgreements(w.db, msi.GetKey(), false, agreementId); err != nil {
								glog.Errorf(logString(fmt.Sprintf("error removing agreement id %v from the service db: %v", agreementId, err)))
							}

							// handle inactive microservice upgrade, upgrade the microservice if needed
							if !skipUpgrade {
								if msd.AutoUpgrade && !msd.ActiveUpgrade {
									cmd := w.NewUpgradeMicroserviceCommand(msd.Id)
									w.Commands <- cmd
								}
							}
						}
						break
					}
				}
			}
		}
	}
}

// This is the case where the agreement is made but the microservices containers fail.
// This function will try a new microservice with lower version.
func (w *GovernanceWorker) handleMicroserviceExecFailure(msdef *persistence.MicroserviceDefinition, msinst_key string) {
	glog.V(3).Infof(logString(fmt.Sprintf("handle service execution failure for %v", msinst_key)))

	// rollback the microservice to lower version
	if err := w.RollbackMicroservice(msdef); err != nil {
		glog.Errorf(logString(fmt.Sprintf("Error downgrading service %v version %v key %v. %v", msdef.SpecRef, msdef.Version, msdef.Id, err)))
	}
}

// Given a microservice id and check if it is set for upgrade, if yes do the upgrade
func (w *GovernanceWorker) handleMicroserviceUpgrade(msdef_id string) {
	glog.V(3).Infof(logString(fmt.Sprintf("handling service upgrade for service id %v", msdef_id)))
	if msdef, err := persistence.FindMicroserviceDefWithKey(w.db, msdef_id); err != nil {
		glog.Errorf(logString(fmt.Sprintf("error getting service definitions %v from db. %v", msdef_id, err)))
	} else if microservice.MicroserviceReadyForUpgrade(msdef, w.db) {
		// find the new ms def to upgrade to
		if new_msdef, err := microservice.GetUpgradeMicroserviceDef(exchange.GetHTTPMicroserviceOrServiceResolverHandler(w), msdef, w.db); err != nil {
			glog.Errorf(logString(fmt.Sprintf("Error finding the new service definition to upgrade to for %v version %v. %v", msdef.SpecRef, msdef.Version, err)))
		} else if new_msdef == nil {
			glog.V(5).Infof(logString(fmt.Sprintf("No changes for service definition %v, no need to upgrade.", msdef.SpecRef)))
		} else if err := w.UpgradeMicroservice(msdef, new_msdef, true); err != nil {
			glog.Errorf(logString(fmt.Sprintf("Error upgrading service %v version %v key %v. %v", msdef.SpecRef, msdef.Version, msdef.Id, err)))

			// rollback the microservice to lower version
			if err := w.RollbackMicroservice(new_msdef); err != nil {
				glog.Errorf(logString(fmt.Sprintf("Error downgrading service %v version %v key %v. %v", new_msdef.SpecRef, new_msdef.Version, new_msdef.Id, err)))
			}
		}
	}
}
