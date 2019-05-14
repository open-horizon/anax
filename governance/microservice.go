package governance

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/eventlog"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/microservice"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/producer"
	"math"
	"net/url"
	"strings"
	"time"
)

// This function runs periodically in a separate process. It checks if the service containers are up and running and
// if new service versions are available for upgrade.
func (w *GovernanceWorker) governMicroservices() int {

	if w.Config.Edge.ServiceUpgradeCheckIntervalS > 0 {
		// get the microservice upgrade check interval
		check_interval := w.Config.Edge.ServiceUpgradeCheckIntervalS

		// check for the new service version when time is right
		time_now := time.Now().Unix()
		if time_now-w.lastSvcUpgradeCheck >= int64(check_interval) {
			w.lastSvcUpgradeCheck = time_now

			// handle service upgrade. The upgrade includes inactive upgrades if the associated agreements happen to be 0.
			glog.V(4).Infof(logString(fmt.Sprintf("governing service upgrades")))
			if ms_defs, err := persistence.FindMicroserviceDefs(w.db, []persistence.MSFilter{persistence.UnarchivedMSFilter()}); err != nil {
				glog.Errorf(logString(fmt.Sprintf("Error getting service definitions from db. %v", err)))
			} else if ms_defs != nil && len(ms_defs) > 0 {
				for _, ms := range ms_defs {
					// upgrade the service if needed
					cmd := w.NewUpgradeMicroserviceCommand(ms.Id)
					w.Commands <- cmd
				}
			}
		}
	}

	// check if service instance containers are down
	glog.V(4).Infof(logString(fmt.Sprintf("governing service containers")))
	if ms_instances, err := persistence.FindMicroserviceInstances(w.db, []persistence.MIFilter{persistence.AllMIFilter(), persistence.UnarchivedMIFilter()}); err != nil {
		glog.Errorf(logString(fmt.Sprintf("Error retrieving all service instances from database, error: %v", err)))
	} else if ms_instances != nil {
		for _, msi := range ms_instances {
			// only check the ones that have containers started already and not in the middle of cleanup
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
// If the msinst_key is not empty, the function is called to restart a failed dependent service.
func (w *GovernanceWorker) StartMicroservice(ms_key string, agreementId string, dependencyPath []persistence.ServiceInstancePathElement, msinst_key string) (*persistence.MicroserviceInstance, error) {
	glog.V(5).Infof(logString(fmt.Sprintf("Starting service instance for %v", ms_key)))

	// get the service instance if the key is given
	var msinst_given *persistence.MicroserviceInstance
	var err1 error
	isRetry := false
	if msinst_key != "" {
		isRetry = true
		msinst_given, err1 = persistence.FindMicroserviceInstanceWithKey(w.db, msinst_key)
		if err1 != nil {
			return nil, fmt.Errorf(logString(fmt.Sprintf("Error finding service instance %v from db. %v", msinst_key, err1)))
		}
	}

	if msdef, err := persistence.FindMicroserviceDefWithKey(w.db, ms_key); err != nil {
		return nil, fmt.Errorf(logString(fmt.Sprintf("Error finding service definition from db with key %v. %v", ms_key, err)))
	} else if msdef == nil {
		return nil, fmt.Errorf(logString(fmt.Sprintf("No service definition available for key %v.", ms_key)))
	} else {
		if !msdef.HasDeployment() {
			glog.Infof(logString(fmt.Sprintf("No workload needed for service %v/%v.", msdef.Org, msdef.SpecRef)))
			var mi *persistence.MicroserviceInstance

			if isRetry {
				mi = msinst_given
			} else {
				mi, err1 = persistence.NewMicroserviceInstance(w.db, msdef.SpecRef, msdef.Org, msdef.Version, ms_key, dependencyPath)
				if err1 != nil {
					return nil, fmt.Errorf(logString(fmt.Sprintf("Error persisting service instance for %v/%v %v %v.", msdef.Org, msdef.SpecRef, msdef.Version, ms_key)))
				}
			}

			// if the new microservice does not have containers, just mark it done.
			if mi_new, err := persistence.UpdateMSInstanceExecutionState(w.db, mi.GetKey(), true, 0, ""); err != nil {
				return nil, fmt.Errorf(logString(fmt.Sprintf("Failed to update the ExecutionStartTime for service instance %v. %v", mi.GetKey(), err)))
			} else {
				return mi_new, nil
			}
		} else {

			// now handle the case where there are containers
			deployment, deploymentSig, torr := msdef.GetDeployment()

			// convert the torrent string to a structure
			var torrent policy.Torrent

			// convert to torrent structure only if the torrent string exists on the exchange
			if torr != "" {
				if err := json.Unmarshal([]byte(torr), &torrent); err != nil {
					return nil, fmt.Errorf(logString(fmt.Sprintf("The torrent definition for service %v/%v has error: %v", msdef.Org, msdef.SpecRef, err)))
				}
			} else {
				// this is the service case where the image server is defined
				ptorrent := msdef.GetImageStore().ConvertToTorrent()

				// convert the persistence.Torrent to policy.Torrent.
				torrent.Url = ptorrent.Url
				torrent.Signature = ptorrent.Signature
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
					key_map, err := exchange.GetHTTPObjectSigningKeysHandler(w)(exchange.SERVICE, msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch)
					if err != nil {
						return nil, fmt.Errorf(logString(fmt.Sprintf("received error getting signing keys from the exchange: %v/%v %v %v. %v", msdef.Org, msdef.SpecRef, msdef.Version, msdef.Arch, err)))
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
					msdef_dep, err := microservice.FindOrCreateMicroserviceDef(w.db, rs.URL, rs.Org, rs.Version, rs.Arch, exchange.GetHTTPServiceHandler(w))
					if err != nil {
						return nil, fmt.Errorf(logString(fmt.Sprintf("failed to get or create service definition for for %v/%v: %v", rs.Org, rs.URL, err)))
					} else {
						// Assume the first msdef is the one we want.
						msspec := events.MicroserviceSpec{SpecRef: rs.URL, Org: rs.Org, Version: msdef_dep.Version, MsdefId: msdef_dep.Id}
						ms_specs = append(ms_specs, msspec)
					}
				}

				// save the instance
				var ms_instance *persistence.MicroserviceInstance
				if isRetry {
					ms_instance = msinst_given
				} else {
					ms_instance, err1 = persistence.NewMicroserviceInstance(w.db, msdef.SpecRef, msdef.Org, msdef.Version, ms_key, dependencyPath)
					if err1 != nil {
						return nil, fmt.Errorf(logString(fmt.Sprintf("Error persisting service instance for %v/%v %v %v.", msdef.Org, msdef.SpecRef, msdef.Version, ms_key)))
					}
				}

				// get the image auth for service (we have to try even for microservice because we do not know if this is ms or svc.)
				img_auths := make([]events.ImageDockerAuth, 0)
				if w.Config.Edge.TrustDockerAuthFromOrg {
					if ias, err := exchange.GetHTTPServiceDockerAuthsHandler(w)(msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch); err != nil {
						glog.V(5).Infof(logString(fmt.Sprintf("received error querying exchange for service image auths: %v/%v version %v, error %v", msdef.Org, msdef.SpecRef, msdef.Version, err)))
					} else {
						if ias != nil {
							for _, iau_temp := range ias {
								username := iau_temp.UserName
								if username == "" {
									username = "token"
								}
								img_auths = append(img_auths, events.ImageDockerAuth{Registry: iau_temp.Registry, UserName: username, Password: iau_temp.Token})
							}
						}
					}
				}

				// Fire an event to the torrent worker so that it will download the container
				cc := events.NewContainerConfig(*url, ms_workload.Torrent.Signature, ms_workload.Deployment, ms_workload.DeploymentSignature, ms_workload.DeploymentUserInfo, "", img_auths)

				// convert the user input from the service attributes to env variables
				if attrs, err := persistence.FindApplicableAttributes(w.db, msdef.SpecRef, msdef.Org); err != nil {
					return nil, fmt.Errorf(logString(fmt.Sprintf("Unable to fetch service preferences for %v/%v. Err: %v", msdef.Org, msdef.SpecRef, err)))
				} else if envAdds, err := persistence.AttributesToEnvvarMap(attrs, make(map[string]string), config.ENVVAR_PREFIX, w.Config.Edge.DefaultServiceRegistrationRAM); err != nil {
					return nil, fmt.Errorf(logString(fmt.Sprintf("Failed to convert service preferences to environmental variables for %v/%v. Err: %v", msdef.Org, msdef.SpecRef, err)))
				} else {
					envAdds[config.ENVVAR_PREFIX+"DEVICE_ID"] = exchange.GetId(w.GetExchangeId())
					envAdds[config.ENVVAR_PREFIX+"ORGANIZATION"] = exchange.GetOrg(w.GetExchangeId())
					envAdds[config.ENVVAR_PREFIX+"PATTERN"] = w.devicePattern
					envAdds[config.ENVVAR_PREFIX+"EXCHANGE_URL"] = w.Config.Edge.ExchangeURL
					// Add in any default variables from the microservice userInputs that havent been overridden
					for _, ui := range msdef.UserInputs {
						if ui.DefaultValue != "" {
							if _, ok := envAdds[ui.Name]; !ok {
								envAdds[ui.Name] = ui.DefaultValue
							}
						}
					}

					agIds := make([]string, 0)

					if agreementId != "" {
						// service originally start up
						agIds = append(agIds, agreementId)
					} else {
						// retry case or agreementless case
						agIds = ms_instance.AssociatedAgreements
					}
					lc := events.NewContainerLaunchContext(cc, &envAdds, events.BlockchainConfig{}, ms_instance.GetKey(), agIds, ms_specs, persistence.NewServiceInstancePathElement(msdef.SpecRef, msdef.Org, msdef.Version), isRetry)
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
			w.Messages() <- events.NewGovernanceWorkloadCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, ag.AgreementProtocol, ag.CurrentAgreementId, ag.GetDeploymentConfig())

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

			// cleanup all the related dependent services for this agreement
			w.handleMicroserviceInstForAgEnded(ag.CurrentAgreementId, true)
		}

		// remove all the containers if any for this specific service instance
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
	glog.V(3).Infof(logString(fmt.Sprintf("Start changing service %v/%v from version %v to version %v", msdef.Org, msdef.SpecRef, msdef.Version, new_msdef.Version)))

	// archive the old ms def and save the new one to db
	if _, err := persistence.MsDefArchived(w.db, msdef.Id); err != nil {
		return fmt.Errorf(logString(fmt.Sprintf("Failed to archived service definition %v. %v", msdef, err)))
	} else if err := persistence.SaveOrUpdateMicroserviceDef(w.db, new_msdef); err != nil {
		return fmt.Errorf(logString(fmt.Sprintf("Failed to save service definition to db. %v. %v", new_msdef, err)))
	} else if _, err := persistence.MSDefUpgradeNewMsId(w.db, msdef.Id, new_msdef.Id); err != nil {
		return fmt.Errorf(logString(fmt.Sprintf("Failed to update the UpgradeNewMsId to %v for service def %v/%v version %v id %v. %v", new_msdef.Id, new_msdef.Org, msdef.SpecRef, msdef.Version, msdef.Id, err)))
	} else if _, err := persistence.MSDefUpgradeStarted(w.db, new_msdef.Id); err != nil {
		return fmt.Errorf(logString(fmt.Sprintf("Failed to update the UpgradeStartTime for service def %v/%v version %v id %v. %v", new_msdef.Org, new_msdef.SpecRef, new_msdef.Version, new_msdef.Id, err)))
	}

	// clean up old microservice
	var eClearError error
	var ms_insts []persistence.MicroserviceInstance
	eClearError = nil
	if ms_insts, eClearError = persistence.FindMicroserviceInstances(w.db, []persistence.MIFilter{persistence.AllInstancesMIFilter(msdef.SpecRef, msdef.Org, msdef.Version), persistence.UnarchivedMIFilter()}); eClearError != nil {
		glog.Errorf(logString(fmt.Sprintf("Error retrieving all the service instances from db for %v/%v version %v key %v. %v", msdef.Org, msdef.SpecRef, msdef.Version, msdef.Id, eClearError)))
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
			return fmt.Errorf(logString(fmt.Sprintf("Failed to update the UpgradeAgreementsClearedTime for service def %v/%v version %v id %v. %v", new_msdef.Org, new_msdef.SpecRef, new_msdef.Version, new_msdef.Id, err)))
		}
	} else {
		if _, err := persistence.MsDefUpgradeAgreementsCleared(w.db, new_msdef.Id); err != nil {
			return fmt.Errorf(logString(fmt.Sprintf("Failed to update the UpgradeAgreementsClearedTime for service def %v/%v version %v id %v. %v", new_msdef.Org, new_msdef.SpecRef, new_msdef.Version, new_msdef.Id, err)))
		}
	}

	// unregister the old ms from exchange
	var unregError error
	unregError = nil
	unregError = microservice.RemoveMicroservicePolicy(msdef.SpecRef, msdef.Org, msdef.Version, msdef.Id, w.Config.Edge.PolicyPath, w.pm)
	if unregError != nil {
		glog.Errorf(logString(fmt.Sprintf("Failed to remove service policy for service def %v/%v version %v. %v", msdef.Org, msdef.SpecRef, msdef.Version, unregError)))
	} else if unregError = microservice.UnregisterMicroserviceExchange(exchange.GetHTTPDeviceHandler(w), exchange.GetHTTPPutDeviceHandler(w), msdef.SpecRef, msdef.Org, w.GetExchangeId(), w.GetExchangeToken(), w.db); unregError != nil {
		glog.Errorf(logString(fmt.Sprintf("Failed to unregister service from the exchange for service def %v/%v. %v", msdef.Org, msdef.SpecRef, unregError)))
	}

	// update msdef UpgradeMsUnregisteredTime
	if unregError != nil {
		if _, err := persistence.MSDefUpgradeFailed(w.db, new_msdef.Id, microservice.MS_UNREG_EXCH_FAILED, microservice.DecodeReasonCode(microservice.MS_UNREG_EXCH_FAILED)); err != nil {
			return fmt.Errorf(logString(fmt.Sprintf("Failed to update service upgrading failure reason for service def %v/%v version %v id %v. %v", new_msdef.Org, new_msdef.SpecRef, new_msdef.Version, new_msdef.Id, err)))
		}
	} else {
		if _, err := persistence.MSDefUpgradeMsUnregistered(w.db, new_msdef.Id); err != nil {
			return fmt.Errorf(logString(fmt.Sprintf("Failed to update the UpgradeMsUnregisteredTime for service def %v/%v version %v id %v. %v", new_msdef.Org, new_msdef.SpecRef, new_msdef.Version, new_msdef.Id, err)))
		}
	}

	// create a new policy file and register the new microservice in exchange
	if err := microservice.GenMicroservicePolicy(new_msdef, w.Config.Edge.PolicyPath, w.db, w.Messages(), exchange.GetOrg(w.GetExchangeId()), w.devicePattern); err != nil {
		if _, err := persistence.MSDefUpgradeFailed(w.db, new_msdef.Id, microservice.MS_REREG_EXCH_FAILED, microservice.DecodeReasonCode(microservice.MS_REREG_EXCH_FAILED)); err != nil {
			return fmt.Errorf(logString(fmt.Sprintf("Failed to update service upgrading failure reason for service def %v/%v version %v id %v. %v", new_msdef.Org, new_msdef.SpecRef, new_msdef.Version, new_msdef.Id, err)))
		}
	} else {
		if _, err := persistence.MSDefUpgradeMsReregistered(w.db, new_msdef.Id); err != nil {
			return fmt.Errorf(logString(fmt.Sprintf("Failed to update the UpgradeMsReregisteredTime for service def %v/%v version %v id %v. %v", new_msdef.Org, new_msdef.SpecRef, new_msdef.Version, new_msdef.Id, err)))
		}
	}

	// done for the microservices without containers.
	glog.V(3).Infof(logString(fmt.Sprintf("End changing service %v/%v version %v key %v", msdef.Org, msdef.SpecRef, msdef.Version, msdef.Id)))

	return nil
}

// This function will call StartMicroservice to restart all the containers for the given service instance.
// The process will eventually trigger image loading (just in case the imgges are gone on the node), old container cleaning and
// new container brought up.
func (w *GovernanceWorker) RetryMicroservice(msi *persistence.MicroserviceInstance) error {
	inst_key := msi.GetKey()
	glog.V(5).Infof(logString(fmt.Sprintf("RetryMicroservice will restart all the containers for %v. Retry count: %v.", inst_key, msi.CurrentRetryCount+1)))

	// increment the retry count
	if _, err := persistence.UpdateMSInstanceCurrentRetryCount(w.db, inst_key, msi.CurrentRetryCount+1); err != nil {
		return fmt.Errorf(logString(fmt.Sprintf("error setting the current retry count to %v for service instance %v in db. %v", msi.CurrentRetryCount+1, inst_key, err)))
		// clear the execution state
	} else if _, err := persistence.ResetMsInstanceExecutionStatus(w.db, inst_key); err != nil {
		return fmt.Errorf(logString(fmt.Sprintf("error resetting the execution status for service instance %v in db. %v", inst_key, err)))
		// start retry
	} else if _, err := w.StartMicroservice(msi.MicroserviceDefId, "", []persistence.ServiceInstancePathElement{}, inst_key); err != nil {
		return err
	} else {
		return nil
	}
}

// Get the next highest microservice version and rollback to it. Tryer even lower version if it fails
func (w *GovernanceWorker) RollbackMicroservice(msdef *persistence.MicroserviceDefinition) error {
	for true {
		// get next lower version
		if new_msdef, err := microservice.GetRollbackMicroserviceDef(exchange.GetHTTPServiceResolverHandler(w), msdef, w.db); err != nil {
			eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
				fmt.Sprintf("Error finding the new service definition to downgrade to for %v/%v version %v key %v. error: %v", msdef.Org, msdef.SpecRef, msdef.Version, msdef.Id, err),
				persistence.EC_DATABASE_ERROR)
			return fmt.Errorf(logString(fmt.Sprintf("Error finding the new service definition to downgrade to for %v/%v version %v key %v. error: %v", msdef.Org, msdef.SpecRef, msdef.Version, msdef.Id, err)))
		} else if new_msdef == nil { //no more to try, exit out
			eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_INFO,
				fmt.Sprintf("Could not find lower version to downgrade for %v/%v version %v.", msdef.Org, msdef.SpecRef, msdef.Version),
				persistence.EC_NO_VERSION_TO_DOWNGRADE,
				"", msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch, []string{})
			glog.Warningf(logString(fmt.Sprintf("Unable to find the service definition to downgrade to for %v/%v version %v key %v.", msdef.Org, msdef.SpecRef, msdef.Version, msdef.Id)))
			return fmt.Errorf(logString(fmt.Sprintf("Unable to find the service definition to downgrade to for %v/%v version %v key %v.", msdef.Org, msdef.SpecRef, msdef.Version, msdef.Id)))
		} else {
			if err := w.UpgradeMicroservice(msdef, new_msdef, false); err != nil {
				eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_ERROR,
					fmt.Sprintf("Error downgrading service %v/%v frpm version %v to version %v. Eror: %v", msdef.Org, msdef.SpecRef, msdef.Version, new_msdef.Version, err),
					persistence.EC_ERROR_DOWNGRADE_SERVICE,
					"", msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch, []string{})
				glog.Errorf(logString(fmt.Sprintf("Failed to downgrade %v/%v from version %v key %v to version %v key %v. %v", msdef.Org, msdef.SpecRef, msdef.Version, msdef.Id, new_msdef.Version, new_msdef.Id, err)))
				msdef = new_msdef
			} else {
				eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_INFO,
					fmt.Sprintf("Complete downgrading service %v/%v from version %v to version %v.", msdef.Org, msdef.SpecRef, msdef.Version, new_msdef.Version),
					persistence.EC_COMPLETE_DOWNGRADE_SERVICE,
					"", msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch, []string{})
				return nil
			}
		}
	}

	return nil
}

// Start a service instance for the given agreement according to the sharing mode.
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
	} else if ms_insts, err := persistence.FindMicroserviceInstances(w.db, []persistence.MIFilter{persistence.UnarchivedMIFilter(), persistence.NotCleanedUpMIFilter(), persistence.AllInstancesMIFilter(msdef.SpecRef, msdef.Org, msdef.Version)}); err != nil {
		eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
			fmt.Sprintf("Error retrieving all the service instances from db for %v/%v version %v key %v. %v", msdef.Org, msdef.SpecRef, msdef.Version, msdef.Id, err),
			persistence.EC_DATABASE_ERROR)
		return fmt.Errorf(logString(fmt.Sprintf("Error retrieving all the service instances from db for %v/%v version %v key %v. %v", msdef.Org, msdef.SpecRef, msdef.Version, msdef.Id, err)))
	} else if ms_insts == nil || len(ms_insts) == 0 {
		needs_new_ms = true
	} else {
		msi = &ms_insts[0]
	}

	if needs_new_ms {
		var inst_err error
		if msi, inst_err = w.StartMicroservice(msdef.Id, agreementId, dependencyPath, ""); inst_err != nil {

			eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_ERROR,
				fmt.Sprintf("Service starting failed for %v/%v version %v, error: %v", msdef.Org, msdef.SpecRef, msdef.Version, inst_err),
				persistence.EC_ERROR_START_SERVICE,
				"", msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch, []string{agreementId})

			// Try to downgrade the service/microservice to a lower version.
			glog.V(3).Infof(logString(fmt.Sprintf("Ending the agreement: %v because service %v/%v failed to start", agreementId, msdef.Org, msdef.SpecRef)))
			ag_reason_code := w.producerPH[protocol].GetTerminationCode(producer.TERM_REASON_MS_DOWNGRADE_REQUIRED)
			ag_reason_text := w.producerPH[protocol].GetTerminationReason(ag_reason_code)
			if agreementId != "" {
				w.cancelAgreement(agreementId, protocol, ag_reason_code, ag_reason_text)
			}

			glog.V(3).Infof(logString(fmt.Sprintf("Downgrading service %v/%v because version %v key %v failed to start. Error: %v", msdef.Org, msdef.SpecRef, msdef.Version, msdef.Id, inst_err)))
			eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_INFO,
				fmt.Sprintf("Start downgrading service %v/%v version %v because service for aagreement failed to start.", msdef.Org, msdef.SpecRef, msdef.Version),
				persistence.EC_START_DOWNGRADE_SERVICE,
				"", msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch, []string{agreementId})

			if err := w.RollbackMicroservice(msdef); err != nil {
				eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_INFO,
					fmt.Sprintf("Error downgrading service %v/%v version %v. %v", msdef.Org, msdef.SpecRef, msdef.Version, err),
					persistence.EC_ERROR_DOWNGRADE_SERVICE,
					"", msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch, []string{agreementId})
				glog.Errorf(logString(fmt.Sprintf("Error downgrading service %v/%v version %v key %v. %v", msdef.Org, msdef.SpecRef, msdef.Version, msdef.Id, err)))
			}

			return fmt.Errorf(logString(fmt.Sprintf("Failed to start service instance for %v/%v version %v key %v. %v", msdef.Org, msdef.SpecRef, msdef.Version, msdef.Id, inst_err)))
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
		eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
			fmt.Sprintf("Error retrieving all service instances from database, error: %v", err),
			persistence.EC_DATABASE_ERROR)
		glog.Errorf(logString(fmt.Sprintf("error retrieving all service instances from database, error: %v", err)))
	} else if ms_instances != nil {
		for _, msi := range ms_instances {
			if msi.AssociatedAgreements != nil && len(msi.AssociatedAgreements) > 0 {
				for _, id := range msi.AssociatedAgreements {
					if id == agreementId {
						msd, err := persistence.FindMicroserviceDefWithKey(w.db, msi.MicroserviceDefId)
						if err != nil {
							glog.Errorf(logString(fmt.Sprintf("Error retrieving service definition %v version %v key %v from database, error: %v", cutil.FormOrgSpecUrl(msi.SpecRef, msi.Org), msi.Version, msi.MicroserviceDefId, err)))
							// delete the microservice instance if the sharing mode is "multiple"
						} else {
							eventlog.LogServiceEvent(w.db, persistence.SEVERITY_INFO,
								fmt.Sprintf("Start cleaning up service %v because agreement %v ended.", cutil.FormOrgSpecUrl(msi.SpecRef, msi.Org), agreementId),
								persistence.EC_START_CLEANUP_SERVICE,
								msi)

							if msd.Sharable == exchange.MS_SHARING_MODE_MULTIPLE || len(msi.AssociatedAgreements) == 1 {
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
							} else if ags, err := persistence.FindEstablishedAgreementsAllProtocols(w.db, policy.AllAgreementProtocols(), []persistence.EAFilter{persistence.IdEAFilter(agreementId)}); err != nil {
								glog.Errorf(logString(fmt.Sprintf("unable to retrieve agreement %v from database, error %v", agreementId, err)))
							} else if len(ags) != 1 {
								glog.Errorf(logString(fmt.Sprintf("Should have one agreement from the db but found %v.", len(ags))))
							} else if ags[0].RunningWorkload.URL != "" {
								// remove the related partent path from the service instance
								tpe := persistence.NewServiceInstancePathElement(ags[0].RunningWorkload.URL, ags[0].RunningWorkload.Org, ags[0].RunningWorkload.Version)
								if _, err := persistence.UpdateMSInstanceRemoveDependencyPath2(w.db, msi.GetKey(), tpe); err != nil {
									glog.Errorf(logString(fmt.Sprintf("error removing parent path from the db for service instance %v fro agreement %v: %v", msi.GetKey(), agreementId, err)))
								}
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

// This function goes through all associated agreements for a dependent servie instance and get the retry count
// and the duration for the top level service. The retry duration means that all the retires must happen within the duration,
// other wise retry count will be reset.
// If there are mutiple agreements associated with the depenent service, the retry count is the average of all the
// non-zero retry counts. The default retry count is 1 if all the areements have 0 retry counts.
func (w *GovernanceWorker) getMiceroserviceRetryCount(msi *persistence.MicroserviceInstance) (uint, uint, error) {
	retry_count := w.Config.Edge.DefaultServiceRetryCount
	retry_duration := uint(w.Config.Edge.DefaultServiceRetryDuration)

	if ags, err := w.FindEstablishedAgreementsWithIds(msi.AssociatedAgreements); err != nil {
		return 0, 0, fmt.Errorf(logString(fmt.Sprintf("unable to retrieve agreements %v from database, error %v", msi.AssociatedAgreements, err)))
	} else if len(ags) != 0 {
		total_retries := 0
		total_retry_duration := 0
		total_nums := 0
		total_duration_nums := 0

		for _, ag := range ags {
			protocolHandler := w.producerPH[ag.AgreementProtocol].AgreementProtocolHandler("", "", "")
			if proposal, err := protocolHandler.DemarshalProposal(ag.Proposal); err != nil {
				return 0, 0, fmt.Errorf(logString(fmt.Sprintf("could not hydrate proposal, error: %v", err)))
			} else if tcPolicy, err := policy.DemarshalPolicy(proposal.TsAndCs()); err != nil {
				return 0, 0, fmt.Errorf(logString(fmt.Sprintf("error demarshalling TsAndCs policy for agreement %v, error %v", ag.CurrentAgreementId, err)))
			} else {
				total_retries += tcPolicy.Workloads[0].Priority.Retries
				total_retry_duration += tcPolicy.Workloads[0].Priority.RetryDurationS
				if tcPolicy.Workloads[0].Priority.Retries != 0 {
					total_nums += 1
				}
				if tcPolicy.Workloads[0].Priority.Retries != 0 {
					total_duration_nums += 1
				}
			}
		}

		if total_nums > 0 {
			retry_count = int(math.Ceil(float64(total_retries) / float64(total_nums)))
		}
		if total_duration_nums > 0 {
			retry_duration = uint(math.Ceil(float64(total_retry_duration) / float64(total_duration_nums)))
		}
	}

	return uint(retry_count), retry_duration, nil
}

// This is the case where the agreement is made but the dependent service containers fail.
// This function will retry the dependent service containers. If the retry fails it will try with a lower version.
func (w *GovernanceWorker) handleMicroserviceExecFailure(msdef *persistence.MicroserviceDefinition, msinst_key string) {
	glog.V(3).Infof(logString(fmt.Sprintf("handle dependent service execution failure for %v", msinst_key)))

	need_retry := false
	// check if we need to retry.
	msi, err := persistence.FindMicroserviceInstanceWithKey(w.db, msinst_key)
	if err != nil {
		eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
			fmt.Sprintf("Error getting service instance %v from db. %v", msinst_key, err),
			persistence.EC_DATABASE_ERROR)
		glog.Errorf(logString(fmt.Sprintf("error getting service instance %v from db. %v", msinst_key, err)))
		return
	}

	timeNow := uint64(time.Now().Unix())
	if msi.RetryStartTime != 0 {
		if timeNow-msi.RetryStartTime <= uint64(msi.MaxRetryDuration) && msi.CurrentRetryCount < msi.MaxRetries {
			need_retry = true
		}
	}

	// new ewtry cycle. getting the retry count again because
	// there may be new agreements associated with this service instance after last retry cycle
	if msi.RetryStartTime == 0 || timeNow-msi.RetryStartTime > uint64(msi.MaxRetryDuration) {
		need_retry = true
		retries, retry_duration, err := w.getMiceroserviceRetryCount(msi)
		if err != nil {
			eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_ERROR,
				fmt.Sprintf("Failed to get the service retry count for %v version %v. %v", msdef.SpecRef, msdef.Version, err),
				persistence.EC_START_DOWNGRADE_SERVICE,
				msinst_key, msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch, []string{})

			glog.Errorf(logString(fmt.Sprintf("Failed to get the retry counts for failed dependent service instance %v. %v", msinst_key, err)))
			return
		}

		var err1 error
		msi, err1 = persistence.UpdateMSInstanceRetryState(w.db, msinst_key, true, retries, retry_duration)
		if err1 != nil {
			eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
				fmt.Sprintf("Error updating retry start state for service instance %v in db. %v", msinst_key, err1),
				persistence.EC_DATABASE_ERROR)
			glog.Errorf(logString(fmt.Sprintf("error updating retry start state for service instance %v in db. %v", msinst_key, err1)))
			return
		}
	}

	if need_retry {
		current_retry := msi.CurrentRetryCount + 1
		// start the retry
		eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_INFO,
			fmt.Sprintf("Start retrying number %v for dependent service %v version %v because service failed.", current_retry, msdef.SpecRef, msdef.Version),
			persistence.EC_START_RETRY_DEPENDENT_SERVICE,
			msinst_key, msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch, []string{})

		if err := w.RetryMicroservice(msi); err != nil {
			eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_ERROR,
				fmt.Sprintf("Failed retrying number %v for dependent service %v version %v.", current_retry, msdef.SpecRef, msdef.Version),
				persistence.EC_ERROR_START_RETRY_DEPENDENT_SERVICE,
				msinst_key, msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch, []string{})
			glog.Errorf(logString(fmt.Sprintf("error retrying number %v for failed dependent service %v.", msinst_key, err)))
			// recursive call to do next retry
			w.handleMicroserviceExecFailure(msdef, msinst_key)
		}
	} else {
		// rollback the microservice to lower version
		// a new ms instance will be created if successful
		eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_INFO,
			fmt.Sprintf("Start downgrading service %v/%v version %v because service failed to start.", msdef.Org, msdef.SpecRef, msdef.Version),
			persistence.EC_START_DOWNGRADE_SERVICE,
			msinst_key, msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch, []string{})

		if err := w.RollbackMicroservice(msdef); err != nil {
			eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_ERROR,
				fmt.Sprintf("Failed to downgrade service %v/%v version %v, error: %v", msdef.Org, msdef.SpecRef, msdef.Version, err),
				persistence.EC_ERROR_DOWNGRADE_SERVICE,
				msinst_key, msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch, []string{})

			glog.Errorf(logString(fmt.Sprintf("Error downgrading service %v/%v version %v key %v. %v", msdef.Org, msdef.SpecRef, msdef.Version, msdef.Id, err)))

			// this service just could not be started. we have to cancel all the associated agreements
			// a new instance will be created.
			cleanup_reason := microservice.MS_EXEC_FAILED
			if err = w.CleanupMicroservice(msdef.SpecRef, msdef.Version, msinst_key, uint(cleanup_reason)); err != nil {
				glog.Errorf(logString(fmt.Sprintf("Error cleanup service instances %v. %v", msinst_key, err)))
			}
		}
	}
}

// Given a microservice id and check if it is set for upgrade, if yes do the upgrade
func (w *GovernanceWorker) handleMicroserviceUpgrade(msdef_id string) {
	glog.V(3).Infof(logString(fmt.Sprintf("handling service upgrade for service id %v", msdef_id)))
	if msdef, err := persistence.FindMicroserviceDefWithKey(w.db, msdef_id); err != nil {
		eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
			fmt.Sprintf("Error getting service definitions %v from db. %v", msdef_id, err),
			persistence.EC_DATABASE_ERROR)
		glog.Errorf(logString(fmt.Sprintf("error getting service definitions %v from db. %v", msdef_id, err)))
	} else if microservice.MicroserviceReadyForUpgrade(msdef, w.db) {
		// find the new ms def to upgrade to
		if new_msdef, err := microservice.GetUpgradeMicroserviceDef(exchange.GetHTTPServiceResolverHandler(w), msdef, w.db); err != nil {
			glog.Errorf(logString(fmt.Sprintf("Error finding the new service definition to upgrade to for %v/%v version %v. %v", msdef.Org, msdef.SpecRef, msdef.Version, err)))
		} else if new_msdef == nil {
			glog.V(5).Infof(logString(fmt.Sprintf("No changes for service definition %v/%v, no need to upgrade.", msdef.Org, msdef.SpecRef)))
		} else {
			eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_INFO,
				fmt.Sprintf("Start upgrading service %v/%v from version %v to version %v.", msdef.Org, msdef.SpecRef, msdef.Version, new_msdef.Version),
				persistence.EC_START_UPGRADE_SERVICE,
				"", msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch, []string{})

			if err := w.UpgradeMicroservice(msdef, new_msdef, true); err != nil {
				eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_ERROR,
					fmt.Sprintf("Failed to upgrade service %v/%v from version %v to version %v, error: %v", msdef.Org, msdef.SpecRef, msdef.Version, new_msdef.Version, err),
					persistence.EC_ERROR_UPGRADE_SERVICE,
					"", msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch, []string{})
				glog.Errorf(logString(fmt.Sprintf("Error upgrading service %v/%v version %v key %v. %v", msdef.Org, msdef.SpecRef, msdef.Version, msdef.Id, err)))

				// rollback the microservice to lower version
				eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_INFO,
					fmt.Sprintf("Start downgrading service %v/%v version %v because upgrade failed.", new_msdef.Org, new_msdef.SpecRef, new_msdef.Version),
					persistence.EC_START_DOWNGRADE_SERVICE,
					"", new_msdef.SpecRef, new_msdef.Org, new_msdef.Version, new_msdef.Arch, []string{})
				if err := w.RollbackMicroservice(new_msdef); err != nil {
					eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_ERROR,
						fmt.Sprintf("Failed to downgrade service %v/%v version %v, error: %v", new_msdef.Org, new_msdef.SpecRef, new_msdef.Version, err),
						persistence.EC_ERROR_DOWNGRADE_SERVICE,
						"", new_msdef.SpecRef, new_msdef.Org, new_msdef.Version, new_msdef.Arch, []string{})
					glog.Errorf(logString(fmt.Sprintf("Error downgrading service %v/%v version %v key %v. %v", new_msdef.Org, new_msdef.SpecRef, new_msdef.Version, new_msdef.Id, err)))
				}
			} else {
				eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_INFO,
					fmt.Sprintf("Complete upgrading service %v/%v from version %v to version %v.", msdef.Org, msdef.SpecRef, msdef.Version, new_msdef.Version),
					persistence.EC_COMPLETE_UPGRADE_SERVICE,
					"", msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch, []string{})
			}
		}
	}
}

// get the service configuration state from the exchange, check if any of them are suspended.
// if a service is suspended, cancel the agreements and remove the containers associated with it.
func (w *GovernanceWorker) governServiceConfigState() int {
	// go govern
	glog.V(4).Infof(logString(fmt.Sprintf("governing the service configuration state")))

	service_cs, err := exchange.GetServicesConfigState(w.GetHTTPFactory(), w.GetExchangeId(), w.GetExchangeToken(), w.GetExchangeURL())
	if err != nil {
		glog.Errorf(logString(fmt.Sprintf("Unable to retrieve servcie configuration state from the exchange, error %v", err)))
		eventlog.LogExchangeEvent(w.db, persistence.SEVERITY_ERROR,
			fmt.Sprintf("Unable to retrieve the service configuration state for node resource %v from the exchange, error %v", w.GetExchangeId(), err),
			persistence.EC_EXCHANGE_ERROR, w.GetExchangeURL())
	} else {
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
	return 0
}

// For the given suspended services, cancel all the related agreements and hence remove all the related containers.
func (w *GovernanceWorker) handleServiceSuspended(service_cs []events.ServiceConfigState) error {
	if service_cs == nil || len(service_cs) == 0 {
		// nothing to handle
		return nil
	}

	glog.V(3).Infof(logString(fmt.Sprintf("handle service suspension for %v", service_cs)))

	orgUrlMIFilter := func() persistence.MIFilter {
		return func(e persistence.MicroserviceInstance) bool {
			for _, s := range service_cs {
				if e.SpecRef == s.Url && e.Org == s.Org {
					return true
				}
			}
			return false
		}
	}

	// get the all the agreements the suspended services are associated with.
	// The agreement that a top level service is associated with is from the EstablishedAgreement.RunningWorkload.
	// The agreement that a dependent level service is associated with is from the MicroserviceInstance.AssociatedAgreements.
	// We need to go through both to get all the agreements that the user want to stop because we do not know from the input, service_cs, if the given
	// service is top level to dependent.
	agreements_to_cancel := make(map[string]persistence.EstablishedAgreement, 10)
	establishedAgreements, err := persistence.FindEstablishedAgreementsAllProtocols(w.db, policy.AllAgreementProtocols(), []persistence.EAFilter{persistence.UnarchivedEAFilter()})
	if err != nil {
		eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
			fmt.Sprintf("Error retrieving matching agreements from database for workloads %v. Error: %v", service_cs, err),
			persistence.EC_DATABASE_ERROR)
		glog.Errorf(logString(fmt.Sprintf("Error retrieving matching agreements from database for workloads %v. Error: %v", service_cs, err)))
		return fmt.Errorf("Error retrieving matching agreements from database for workloads %v. Error: %v", service_cs, err)
	} else if establishedAgreements != nil && len(establishedAgreements) > 0 {
		for _, ag := range establishedAgreements {
			for _, s := range service_cs {
				if ag.RunningWorkload.URL == s.Url && ag.RunningWorkload.Org == s.Org {
					agreements_to_cancel[ag.CurrentAgreementId] = ag
					break
				}
			}
		}

		ms_insts, err := persistence.FindMicroserviceInstances(w.db, []persistence.MIFilter{persistence.UnarchivedMIFilter(), orgUrlMIFilter()})
		if err != nil {
			eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
				fmt.Sprintf("Error retrieving all the service instances from db for %v. %v", service_cs, err),
				persistence.EC_DATABASE_ERROR)
			glog.Errorf(logString(fmt.Sprintf("Error retrieving the service instances from db for %v. %v", service_cs, err)))
			return fmt.Errorf("Error retrieving all the service instances from db for %v. %v", service_cs, err)
		} else if ms_insts != nil && len(ms_insts) > 0 {
			for _, msi := range ms_insts {
				ag_ids := msi.AssociatedAgreements
				if ag_ids != nil && len(ag_ids) > 0 {
					for _, ag_id := range ag_ids {
						for _, ag := range establishedAgreements {
							if ag.CurrentAgreementId == ag_id {
								agreements_to_cancel[ag_id] = ag
								break
							}
						}
					}
				}
			}
		}
	}

	// now cancel the agreements
	for _, ag := range agreements_to_cancel {
		glog.V(3).Infof(logString(fmt.Sprintf("Start terminating agreement %v because service suspened.", ag.CurrentAgreementId)))

		reason := w.producerPH[ag.AgreementProtocol].GetTerminationCode(producer.TERM_REASON_SERVICE_SUSPENDED)

		eventlog.LogAgreementEvent(w.db, persistence.SEVERITY_INFO,
			fmt.Sprintf("Start terminating agreement for %v/%v. Reason: %v", ag.RunningWorkload.Org, ag.RunningWorkload.URL, w.producerPH[ag.AgreementProtocol].GetTerminationReason(reason)),
			persistence.EC_CANCEL_AGREEMENT_SERVICE_SUSPENDED,
			ag)

		w.cancelAgreement(ag.CurrentAgreementId, ag.AgreementProtocol, reason, w.producerPH[ag.AgreementProtocol].GetTerminationReason(reason))

		// cleanup workloads
		w.Messages() <- events.NewGovernanceWorkloadCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, ag.AgreementProtocol, ag.CurrentAgreementId, ag.GetDeploymentConfig())

		// clean up microservice instances
		w.handleMicroserviceInstForAgEnded(ag.CurrentAgreementId, true)
	}

	return nil
}
