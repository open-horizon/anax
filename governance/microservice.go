package governance

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/eventlog"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/microservice"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/producer"
	"math"
	"strconv"
	"strings"
	"time"
)

// This function runs periodically in a separate process. It checks if the service containers are up and running.
func (w *GovernanceWorker) governMicroservices() int {

	// check if service instance containers are down
	glog.V(4).Infof(logString(fmt.Sprintf("governing service containers")))
	if ms_instances, err := persistence.FindMicroserviceInstances(w.db, []persistence.MIFilter{persistence.UnarchivedMIFilter()}); err != nil {
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

// Since we changed to saving the signing key with the agreement id, we need to make sure we delete the key when done with it
// to avoid filling up the filesystem
func (w *GovernanceWorker) cleanupSigningKeys(keys []string) {

	errHandler := func(keyname string) api.ErrorHandler {
		return func(err error) bool {
			glog.Errorf(logString(fmt.Sprintf("received error when deleting the signing key file %v to anax. %v", keyname, err)))
			return true
		}
	}

	for _, key := range keys {
		glog.V(3).Info(fmt.Sprintf("About to delete signing key %s", key))
		api.DeletePublicKey(key, w.Config, errHandler(key))
	}
}

// This function is called when there is a change to a service in the exchange. That might signal a service upgrade.
func (w *GovernanceWorker) governMicroserviceVersions() {

	// handle service upgrade. The upgrade includes inactive upgrades if the associated agreements happen to be 0.
	glog.V(3).Infof(logString(fmt.Sprintf("governing service upgrades")))
	if ms_defs, err := persistence.FindMicroserviceDefs(w.db, []persistence.MSFilter{persistence.UnarchivedMSFilter()}); err != nil {
		glog.Errorf(logString(fmt.Sprintf("Error getting service definitions from db. %v", err)))
	} else if ms_defs != nil && len(ms_defs) > 0 {
		for _, ms := range ms_defs {
			glog.V(5).Infof(logString(fmt.Sprintf("MS:%v", ms)))
			// upgrade the service if needed
			cmd := w.NewUpgradeMicroserviceCommand(ms.Id)
			w.Commands <- cmd
		}
	}
}

// It creates microservice instance and loads the containers for the given microservice def.
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
				mi, err1 = persistence.NewMicroserviceInstance(w.db, msdef.SpecRef, msdef.Org, msdef.Version, ms_key, dependencyPath, false)
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
			deployment, deploymentSig := msdef.GetDeployment()

			// keep track of keys used for validating signatures so we can clean them up when signatures verified
			signingKeys := make([]string, 0, 1)
			num_signing_keys := 0

			// convert workload to policy workload structure
			var ms_workload policy.Workload
			ms_workload.Deployment = deployment
			ms_workload.DeploymentSignature = deploymentSig
			ms_workload.WorkloadPassword = ""
			ms_workload.DeploymentUserInfo = ""

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

					var prepend_key_string string
					if len(agreementId) > 0 {
						prepend_key_string = agreementId
					} else {
						prepend_key_string = ms_key
					}
					for key, content := range key_map {
						//add .pem the end of the keyname if it does not have none.
						fn := key
						if !strings.HasSuffix(key, ".pem") {
							fn = fmt.Sprintf("%v.pem", key)
						}

						// Keys for different services might have the same key name like service.public.pem so prepend something unique like the agreement id
						// but then we have to make sure we delete the key when done with it
						prepend_string := prepend_key_string + "_" + strconv.Itoa(num_signing_keys) + "_"
						num_signing_keys += 1
						key_name := prepend_string + fn

						api.UploadPublicKey(key_name, []byte(content), w.Config, errHandler(fn))
						signingKeys = append(signingKeys, key_name)
					}
				}
			}

			// Verify the deployment signature
			if pemFiles, err := w.Config.Collaborators.KeyFileNamesFetcher.GetKeyFileNames(w.Config.Edge.PublicKeyPath, w.Config.UserPublicKeyPath()); err != nil {
				w.cleanupSigningKeys(signingKeys)
				return nil, fmt.Errorf(logString(fmt.Sprintf("received error getting pem key files: %v", err)))
			} else if err := ms_workload.HasValidSignature(pemFiles); err != nil {
				w.cleanupSigningKeys(signingKeys)
				return nil, fmt.Errorf(logString(fmt.Sprintf("service container has invalid deployment signature %v for %v", ms_workload.DeploymentSignature, ms_workload.Deployment)))
			}

			w.cleanupSigningKeys(signingKeys)

			// Gather up the service dependencies, if there are any. Microservices in the workload/microservice model never have dependencies,
			// but services can. It is important to use the correct version for the service dependency, which is the version we have
			// in the local database, not necessarily the version in the dependency. The version we have in the local database should always
			// be greater than the dependency version.
			ms_specs := []events.MicroserviceSpec{}
			for _, rs := range msdef.RequiredServices {
				msdef_dep, err := microservice.FindOrCreateMicroserviceDef(w.db, rs.URL, rs.Org, rs.Version, rs.Arch, false, w.devicePattern != "", exchange.GetHTTPServiceHandler(w))
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
				ms_instance, err1 = persistence.NewMicroserviceInstance(w.db, msdef.SpecRef, msdef.Org, msdef.Version, ms_key, dependencyPath, false)
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
			cc := events.NewContainerConfig(ms_workload.Deployment, ms_workload.DeploymentSignature, ms_workload.DeploymentUserInfo, "", "", "", img_auths)

			// convert the user input from the service attributes, user input from policy and node to env variables
			envAdds, err := w.GetEnvVarsForServiceDepolyment(msdef, ms_instance, agreementId)
			if err != nil {
				return nil, err
			}

			agIds := make([]string, 0)
			if agreementId != "" {
				// service originally start up
				agIds = append(agIds, agreementId)
			} else {
				// retry case or agreementless case
				agIds = ms_instance.AssociatedAgreements
			}

			lc := events.NewContainerLaunchContext(cc, &envAdds, events.BlockchainConfig{}, ms_instance.GetKey(), agIds, ms_specs, dependencyPath, isRetry)
			w.Messages() <- events.NewLoadContainerMessage(events.LOAD_CONTAINER, lc)

			return ms_instance, nil // assume there is only one workload for a microservice
		}
	}
}

// Collect the user inputs from node, policy and service. Convert them to a map of strings that can be used as environmental variables for a dependent service container.
func (w *GovernanceWorker) GetEnvVarsForServiceDepolyment(msdef *persistence.MicroserviceDefinition, msInst *persistence.MicroserviceInstance, agreementId string) (map[string]string, error) {

	var envAdds map[string]string

	var tcPolicy *policy.Policy

	if agreementId != "" {
		// this the first time this dependent service is brought up
		ags, err := persistence.FindEstablishedAgreementsAllProtocols(w.db, policy.AllAgreementProtocols(), []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(agreementId)})
		if err != nil {
			return nil, fmt.Errorf(logString(fmt.Sprintf("failed to retrieve agreement %v from database, error %v", agreementId, err)))
		} else if len(ags) == 0 {
			return nil, fmt.Errorf(logString(fmt.Sprintf("unable to find agreement %v from database.", agreementId)))
		}

		if proposal, err := w.producerPH[ags[0].AgreementProtocol].AgreementProtocolHandler("", "", "").DemarshalProposal(ags[0].Proposal); err != nil {
			return nil, fmt.Errorf(logString(fmt.Sprintf("Error demarshalling proposal from agreement %v, %v", agreementId, err)))
		} else if pol, err := policy.DemarshalPolicy(proposal.TsAndCs()); err != nil {
			return nil, fmt.Errorf(logString(fmt.Sprintf("Error demarshalling policy from proposal for agreement %v, %v", agreementId, err)))
		} else {
			tcPolicy = pol
		}

	} else {
		tcPolicy = nil
	}

	envAdds, err := w.GetServicePreference(msdef.SpecRef, msdef.Org, tcPolicy)
	if err != nil {
		return nil, fmt.Errorf(logString(fmt.Sprintf("Error getting environment variables from node settings for %v %v: %v", msdef.SpecRef, msdef.Org, err)))
	}

	// for the retry case, get the variables from the old tcPolicy back
	if msInst != nil && msInst.EnvVars != nil && len(msInst.EnvVars) != 0 {
		for k, v := range msInst.EnvVars {
			if _, ok := envAdds[k]; !ok {
				envAdds[k] = v
			}
		}
	}

	cutil.SetPlatformEnvvars(envAdds,
		config.ENVVAR_PREFIX,
		"",
		exchange.GetId(w.GetExchangeId()),
		exchange.GetOrg(w.GetExchangeId()),
		w.Config.Edge.ExchangeURL,
		w.devicePattern,
		w.BaseWorker.Manager.Config.GetFileSyncServiceProtocol(),
		w.BaseWorker.Manager.Config.GetFileSyncServiceAPIListen(),
		strconv.Itoa(int(w.BaseWorker.Manager.Config.GetFileSyncServiceAPIPort())))

	// Add in any default variables from the microservice userInputs that havent been overridden
	for _, ui := range msdef.UserInputs {
		if ui.DefaultValue != "" {
			if _, ok := envAdds[ui.Name]; !ok {
				envAdds[ui.Name] = ui.DefaultValue
			}
		}
	}

	// save the envvars for retry case
	if _, err := persistence.UpdateMSInstanceEnvVars(w.db, msInst.GetKey(), envAdds); err != nil {
		return nil, fmt.Errorf(logString(fmt.Sprintf("Error saving environmental variable settings to ms instance %v: %v", msInst.GetKey(), err)))
	}

	return envAdds, nil
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
	if err := persistence.ArchiveMicroserviceInstAndDef(w.db, inst_key, w.devicePattern == ""); err != nil {
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
	} else if unregError = microservice.UnregisterMicroserviceExchange(exchange.GetHTTPDeviceHandler(w), exchange.GetHTTPPatchDeviceHandler(w.limitedRetryEC), msdef.SpecRef, msdef.Org, msdef.Version, w.GetExchangeId(), w.GetExchangeToken(), w.db); unregError != nil {
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

	// create a new policy file and register the new microservice in exchange only for the pattern case.
	// the business policy case does not need policy files.
	var genPolErr error
	if w.devicePattern != "" {
		genPolErr = microservice.GenMicroservicePolicy(new_msdef, w.Config.Edge.PolicyPath, w.db, w.Messages(), exchange.GetOrg(w.GetExchangeId()), w.devicePattern)
		if genPolErr != nil {
			if _, err := persistence.MSDefUpgradeFailed(w.db, new_msdef.Id, microservice.MS_REREG_EXCH_FAILED, microservice.DecodeReasonCode(microservice.MS_REREG_EXCH_FAILED)); err != nil {
				return fmt.Errorf(logString(fmt.Sprintf("Failed to update service upgrading failure reason for service def %v/%v version %v id %v. %v", new_msdef.Org, new_msdef.SpecRef, new_msdef.Version, new_msdef.Id, err)))
			}
		}
	}

	if genPolErr == nil {
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
				persistence.NewMessageMeta(EL_GOV_ERR_FIND_SDEF_FOR_DOWNGRADE, msdef.Org, msdef.SpecRef, msdef.Version, msdef.Id, err.Error()),
				persistence.EC_DATABASE_ERROR)
			return fmt.Errorf(logString(fmt.Sprintf("Error finding the new service definition to downgrade to for %v/%v version %v key %v. error: %v", msdef.Org, msdef.SpecRef, msdef.Version, msdef.Id, err)))
		} else if new_msdef == nil { //no more to try, exit out
			eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_INFO,
				persistence.NewMessageMeta(EL_GOV_ERR_NO_VERSION_TO_DOWNGRADE, msdef.Org, msdef.SpecRef, msdef.Version),
				persistence.EC_NO_VERSION_TO_DOWNGRADE,
				"", msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch, []string{})
			glog.Warningf(logString(fmt.Sprintf("Unable to find the service definition to downgrade to for %v/%v version %v key %v.", msdef.Org, msdef.SpecRef, msdef.Version, msdef.Id)))
			return fmt.Errorf(logString(fmt.Sprintf("Unable to find the service definition to downgrade to for %v/%v version %v key %v.", msdef.Org, msdef.SpecRef, msdef.Version, msdef.Id)))
		} else {
			if err := w.UpgradeMicroservice(msdef, new_msdef, false); err != nil {
				eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_ERROR,
					persistence.NewMessageMeta(EL_GOV_ERR_DOWNGRADE_FROM, msdef.Org, msdef.SpecRef, msdef.Version, new_msdef.Version, err.Error()),
					persistence.EC_ERROR_DOWNGRADE_SERVICE,
					"", msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch, []string{})
				glog.Errorf(logString(fmt.Sprintf("Failed to downgrade %v/%v from version %v key %v to version %v key %v. %v", msdef.Org, msdef.SpecRef, msdef.Version, msdef.Id, new_msdef.Version, new_msdef.Id, err)))
				msdef = new_msdef
			} else {
				eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_INFO,
					persistence.NewMessageMeta(EL_GOV_COMPLETE_DOWNGRADE, msdef.Org, msdef.SpecRef, msdef.Version, new_msdef.Version),
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
	if msdef.Sharable == exchangecommon.SERVICE_SHARING_MODE_MULTIPLE {
		needs_new_ms = true
		// For other sharing modes, start a new instance only if there is no existing one.
		// The "exclusive" sharing mode is handled by maxAgreements=1 in the node side policy file. This ensures that agbots and nodes will
		// only support one agreement at any time.
	} else if ms_insts, err := persistence.FindMicroserviceInstances(w.db, []persistence.MIFilter{persistence.UnarchivedMIFilter(), persistence.NotCleanedUpMIFilter(), persistence.AllInstancesMIFilter(msdef.SpecRef, msdef.Org, msdef.Version)}); err != nil {
		eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
			persistence.NewMessageMeta(EL_GOV_ERR_RETRIEVE_SINSTS_VER_FROM_DB, msdef.Org, msdef.SpecRef, msdef.Version, msdef.Id, err.Error()),
			persistence.EC_DATABASE_ERROR)
		return fmt.Errorf(logString(fmt.Sprintf("Error retrieving all the service instances from db for %v/%v version %v key %v. %v", msdef.Org, msdef.SpecRef, msdef.Version, msdef.Id, err)))
	} else if ms_insts == nil || len(ms_insts) == 0 {
		needs_new_ms = true
	} else {
		msi = &ms_insts[0]
		glog.V(3).Infof(logString(fmt.Sprintf("For agreement %v, microservice %v/%v %v %v is already started as dependency %v, was requested as dependency %v.", agreementId, msi.Org, msi.SpecRef, msi.Version, msi.InstanceId, msi.ParentPath, dependencyPath)))
	}

	if needs_new_ms {
		var inst_err error
		if msi, inst_err = w.StartMicroservice(msdef.Id, agreementId, dependencyPath, ""); inst_err != nil {

			eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_ERROR,
				persistence.NewMessageMeta(EL_GOV_ERR_START_SVC, msdef.Org, msdef.SpecRef, msdef.Version, inst_err.Error()),
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
				persistence.NewMessageMeta(EL_GOV_START_DOWNGRADE_FOR_AG, msdef.Org, msdef.SpecRef, msdef.Version),
				persistence.EC_START_DOWNGRADE_SERVICE,
				"", msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch, []string{agreementId})

			if err := w.RollbackMicroservice(msdef); err != nil {
				eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_ERROR,
					persistence.NewMessageMeta(EL_GOV_ERR_DOWNGRADE, msdef.Org, msdef.SpecRef, msdef.Version, err.Error()),
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
			persistence.NewMessageMeta(EL_GOV_ERR_RETRIEVE_SINSTS_VER_FROM_DB, err.Error()),
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
								persistence.NewMessageMeta(EL_GOV_START_CLEANUP_SVC, cutil.FormOrgSpecUrl(msi.SpecRef, msi.Org), agreementId),
								persistence.EC_START_CLEANUP_SERVICE,
								msi)

							// If this microservice is only associated with 1 agreement, then it can be stopped. The only exception
							// is for microservices that are agreementless, which are never stopped.
							if (msd.Sharable == exchangecommon.SERVICE_SHARING_MODE_MULTIPLE || len(msi.AssociatedAgreements) < 2) && !msi.AgreementLess {
								// mark the ms clean up started and remove all the microservice containers if any
								if _, err := persistence.MicroserviceInstanceCleanupStarted(w.db, msi.GetKey()); err != nil {
									glog.Errorf(logString(fmt.Sprintf("Error setting cleanup start time for service instance %v. %v", msi.GetKey(), err)))
								} else if has_wl, err := msi.HasWorkload(w.db); err != nil {
									glog.Errorf(logString(fmt.Sprintf("Error checking if the service %v has workload. %v", msi.GetKey(), err)))
								} else if has_wl {
									// the ms instance will be archived after the microservice containers are destroyed.
									glog.V(5).Infof(logString(fmt.Sprintf("Removing all the containers for %v", msi.GetKey())))
									w.Messages() <- events.NewMicroserviceCancellationMessage(events.CANCEL_MICROSERVICE, msi.GetKey())
								}
								if err := persistence.ArchiveMicroserviceInstAndDef(w.db, msi.GetKey(), w.devicePattern == ""); err != nil {
									glog.Errorf(logString(fmt.Sprintf("Error archiving service instance %v. %v", msi.GetKey(), err)))
								}
							} else {
								if _, err := persistence.UpdateMSInstanceAssociatedAgreements(w.db, msi.GetKey(), false, agreementId); err != nil {
									glog.Errorf(logString(fmt.Sprintf("error removing agreement id %v from the service db: %v", agreementId, err)))
								} else if ags, err := persistence.FindEstablishedAgreementsAllProtocols(w.db, policy.AllAgreementProtocols(), []persistence.EAFilter{persistence.IdEAFilter(agreementId)}); err != nil {
									glog.Errorf(logString(fmt.Sprintf("unable to retrieve agreement %v from database, error %v", agreementId, err)))
								} else if len(ags) != 1 {
									glog.Errorf(logString(fmt.Sprintf("Should have one agreement from the db but found %v.", len(ags))))
								} else if ags[0].RunningWorkload.URL != "" {
									// remove the related parent path from the service instance
									tpe := persistence.NewServiceInstancePathElement(ags[0].RunningWorkload.URL, ags[0].RunningWorkload.Org, ags[0].RunningWorkload.Version)
									if _, err := persistence.UpdateMSInstanceRemoveDependencyPath2(w.db, msi.GetKey(), tpe); err != nil {
										glog.Errorf(logString(fmt.Sprintf("error removing parent path from the db for service instance %v fro agreement %v: %v", msi.GetKey(), agreementId, err)))
									}
								}
								// Singleton services that are dependencies will have extra networks, which might not be needed any more since
								// at least one of the parents is going away when the current agreement terminates.
								if msd.Sharable == exchangecommon.SERVICE_SHARING_MODE_SINGLE || msd.Sharable == exchangecommon.SERVICE_SHARING_MODE_SINGLETON {
									glog.V(5).Infof(logString(fmt.Sprintf("Remove extra networks for %v all context msi: %v", msi.GetKey(), msi)))
									w.Messages() <- events.NewMicroserviceCancellationMessage(events.CANCEL_MICROSERVICE_NETWORK, msi.GetKey())
								}
							}

							// handle inactive microservice upgrade, upgrade the microservice if needed
							if !skipUpgrade && w.devicePattern != "" {
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
func (w *GovernanceWorker) getMicroserviceRetryCount(msi *persistence.MicroserviceInstance) (uint, uint, error) {
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
			persistence.NewMessageMeta(EL_GOV_ERR_RETRIEVE_SINST_FROM_DB, msinst_key, err.Error()),
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

	// new retry cycle. getting the retry count again because
	// there may be new agreements associated with this service instance after last retry cycle
	if msi.RetryStartTime == 0 || timeNow-msi.RetryStartTime > uint64(msi.MaxRetryDuration) {
		need_retry = true
		retries, retry_duration, err := w.getMicroserviceRetryCount(msi)
		if err != nil {
			eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_ERROR,
				persistence.NewMessageMeta(EL_GOV_ERR_GET_SVC_RETRY_CNT, msdef.SpecRef, msdef.Version, err.Error()),
				persistence.EC_START_DOWNGRADE_SERVICE,
				msinst_key, msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch, []string{})

			glog.Errorf(logString(fmt.Sprintf("Failed to get the retry counts for failed dependent service instance %v. %v", msinst_key, err)))
			return
		}

		var err1 error
		msi, err1 = persistence.UpdateMSInstanceRetryState(w.db, msinst_key, true, retries, retry_duration)
		if err1 != nil {
			eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
				persistence.NewMessageMeta(EL_GOV_ERR_UPDATE_SVC_RETRY_STATE, msinst_key, err1.Error()),
				persistence.EC_DATABASE_ERROR)
			glog.Errorf(logString(fmt.Sprintf("error updating retry start state for service instance %v in db. %v", msinst_key, err1)))
			return
		}
	}

	if need_retry {
		current_retry := msi.CurrentRetryCount + 1
		// start the retry
		eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_INFO,
			persistence.NewMessageMeta(EL_GOV_START_SVC_RETRY, strconv.Itoa(int(current_retry)), msdef.SpecRef, msdef.Version),
			persistence.EC_START_RETRY_DEPENDENT_SERVICE,
			msinst_key, msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch, []string{})

		if err := w.RetryMicroservice(msi); err != nil {
			eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_ERROR,
				persistence.NewMessageMeta(EL_GOV_FAILED_SVC_RETRY, strconv.Itoa(int(current_retry)), msdef.SpecRef, msdef.Version),
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
			persistence.NewMessageMeta(EL_GOV_START_DOWNGRADE, msdef.Org, msdef.SpecRef, msdef.Version),
			persistence.EC_START_DOWNGRADE_SERVICE,
			msinst_key, msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch, []string{})

		if err := w.RollbackMicroservice(msdef); err != nil {
			eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_ERROR,
				persistence.NewMessageMeta(EL_GOV_FAILED_DOWNGRADE, msdef.Org, msdef.SpecRef, msdef.Version, err.Error()),
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
			persistence.NewMessageMeta(EL_GOV_ERR_RETRIEVE_SDEFS_FROM_DB, msdef_id, err.Error()),
			persistence.EC_DATABASE_ERROR)
		glog.Errorf(logString(fmt.Sprintf("error getting service definitions %v from db. %v", msdef_id, err)))
	} else if microservice.MicroserviceReadyForUpgrade(msdef, w.db) {
		// find the new ms def to upgrade to
		if new_msdef, err := microservice.GetUpgradeMicroserviceDef(exchange.GetHTTPServiceResolverHandler(w.limitedRetryEC), msdef, w.db); err != nil {
			glog.Errorf(logString(fmt.Sprintf("Error finding the new service definition to upgrade to for %v/%v version %v. %v", msdef.Org, msdef.SpecRef, msdef.Version, err)))
		} else if new_msdef == nil {
			glog.V(5).Infof(logString(fmt.Sprintf("No changes for service definition %v/%v, no need to upgrade.", msdef.Org, msdef.SpecRef)))
		} else {
			eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_INFO,
				persistence.NewMessageMeta(EL_GOV_START_UPGRADE, msdef.Org, msdef.SpecRef, msdef.Version, new_msdef.Version),
				persistence.EC_START_UPGRADE_SERVICE,
				"", msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch, []string{})

			if err := w.UpgradeMicroservice(msdef, new_msdef, true); err != nil {
				eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_ERROR,
					persistence.NewMessageMeta(EL_GOV_FAILED_UPGRADE, msdef.Org, msdef.SpecRef, msdef.Version, new_msdef.Version, err.Error()),
					persistence.EC_ERROR_UPGRADE_SERVICE,
					"", msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch, []string{})
				glog.Errorf(logString(fmt.Sprintf("Error upgrading service %v/%v version %v key %v. %v", msdef.Org, msdef.SpecRef, msdef.Version, msdef.Id, err)))

				// rollback the microservice to lower version
				eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_INFO,
					persistence.NewMessageMeta(EL_GOV_START_DOWNGRADE_BECAUSE_UPGRADE_FAILED, new_msdef.Org, new_msdef.SpecRef, new_msdef.Version),
					persistence.EC_START_DOWNGRADE_SERVICE,
					"", new_msdef.SpecRef, new_msdef.Org, new_msdef.Version, new_msdef.Arch, []string{})
				if err := w.RollbackMicroservice(new_msdef); err != nil {
					eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_ERROR,
						persistence.NewMessageMeta(EL_GOV_FAILED_DOWNGRADE, new_msdef.Org, new_msdef.SpecRef, new_msdef.Version, err.Error()),
						persistence.EC_ERROR_DOWNGRADE_SERVICE,
						"", new_msdef.SpecRef, new_msdef.Org, new_msdef.Version, new_msdef.Arch, []string{})
					glog.Errorf(logString(fmt.Sprintf("Error downgrading service %v/%v version %v key %v. %v", new_msdef.Org, new_msdef.SpecRef, new_msdef.Version, new_msdef.Id, err)))
				}
			} else {
				eventlog.LogServiceEvent2(w.db, persistence.SEVERITY_INFO,
					persistence.NewMessageMeta(EL_GOV_COMPLETE_UPGRADE, msdef.Org, msdef.SpecRef, msdef.Version, new_msdef.Version),
					persistence.EC_COMPLETE_UPGRADE_SERVICE,
					"", msdef.SpecRef, msdef.Org, msdef.Version, msdef.Arch, []string{})
			}
		}
	}
}

// For the given suspended services, cancel all the related agreements and hence remove all the related containers.
func (w *GovernanceWorker) handleServiceSuspended(service_cs []events.ServiceConfigState) error {
	if service_cs == nil || len(service_cs) == 0 {
		// nothing to handle
		return nil
	}

	svcsToSuspend := make([]events.ServiceConfigState, 0)
	for _, svc := range service_cs {
		if svc.ConfigState == exchange.SERVICE_CONFIGSTATE_SUSPENDED {
			svcsToSuspend = append(svcsToSuspend, svc)
		}
	}
	if len(svcsToSuspend) == 0 {
		return nil
	}
	service_cs = svcsToSuspend

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
			persistence.NewMessageMeta(EL_GOV_ERR_RETRIEVE_MATCH_AGS_FROM_DB, fmt.Sprintf("%v", service_cs), err.Error()),
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
				persistence.NewMessageMeta(EL_GOV_ERR_RETRIEVE_SINSTS_FOR_FROM_DB, fmt.Sprintf("%v", service_cs), err.Error()),
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
			persistence.NewMessageMeta(EL_GOV_START_TERM_AG_WITH_REASON, cutil.FormOrgSpecUrl(ag.RunningWorkload.URL, ag.RunningWorkload.Org), w.producerPH[ag.AgreementProtocol].GetTerminationReason(reason)),
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

// For the policy case, the registeredServices is not used except for service suspension and resumption
// We need to update the registeredServices when an agreement created and canceled.
func (w *GovernanceWorker) UpdateRegisteredServicesWithAgreement() {
	// only do it for
	if w.devicePattern != "" {
		return
	}

	glog.V(3).Infof(logString(fmt.Sprintf("Start updating the registeredServices %v in the exchange for policy case.", w.GetExchangeId())))

	activeServices := []exchange.Microservice{}

	msdefs, err := persistence.FindMicroserviceDefs(w.db, []persistence.MSFilter{persistence.UnarchivedMSFilter()})
	if err != nil {
		eventlog.LogDatabaseEvent(w.db, persistence.SEVERITY_ERROR,
			persistence.NewMessageMeta(EL_GOV_ERR_RETRIEVE_ALL_SDEFS_FROM_DB, err.Error()),
			persistence.EC_DATABASE_ERROR)
		glog.Errorf(logString(fmt.Sprintf("Error retrieving all service definitions from database. %v", err)))
		return
	} else if msdefs != nil {
		// create a registeredServices object from the services. assume all are active for now
		for _, msdef := range msdefs {
			new_s := exchange.Microservice{
				Url:         cutil.FormOrgSpecUrl(msdef.SpecRef, msdef.Org),
				Version:     msdef.Version,
				ConfigState: exchange.SERVICE_CONFIGSTATE_ACTIVE,
			}
			activeServices = append(activeServices, new_s)
		}
	}

	// get current registeredServices
	pDevice, err := exchange.GetHTTPDeviceHandler(w)(w.GetExchangeId(), w.GetExchangeToken())
	if err != nil {
		eventlog.LogExchangeEvent(w.db, persistence.SEVERITY_ERROR,
			persistence.NewMessageMeta(EL_GOV_ERR_RETRIEVE_NODE_FROM_EXCH, w.GetExchangeId(), err.Error()),
			persistence.EC_EXCHANGE_ERROR, w.GetExchangeURL())
		glog.Errorf(logString(fmt.Sprintf("Error retrieving node %v from the exchange: %v", w.GetExchangeId(), err)))
		return
	} else if pDevice == nil {
		glog.Errorf(logString(fmt.Sprintf("Cannot get node %v from the exchange.", w.GetExchangeId())))
		return
	}

	// go through existing registeredServices and move the suspended services into the new registsteredServices
	newRegisteredServices, isSame := composeNewRegisteredServices(activeServices, pDevice.RegisteredServices)

	// no changes, no need to update the node on the exchange.
	if isSame {
		glog.V(3).Infof(logString(fmt.Sprintf("No change for the node's registeredServices in the exchange.")))
		return
	}

	// update the exchange with the new registeredServices
	pdr := exchange.PatchDeviceRequest{}
	pdr.RegisteredServices = &newRegisteredServices
	patchDevice := exchange.GetHTTPPatchDeviceHandler(w)
	if err := patchDevice(w.GetExchangeId(), w.GetExchangeToken(), &pdr); err != nil {
		eventlog.LogExchangeEvent(w.db, persistence.SEVERITY_ERROR,
			persistence.NewMessageMeta(EL_GOV_ERR_UPDATE_REGSVCS_IN_EXCH, w.GetExchangeId(), err.Error()),
			persistence.EC_EXCHANGE_ERROR, w.GetExchangeURL())
		glog.Errorf(logString(fmt.Sprintf("Error patching node %v with new registeredServices %v. %v", w.GetExchangeId(), newRegisteredServices, err)))
		return
	} else {
		glog.V(3).Infof(logString(fmt.Sprintf("Complete updating the node %v with the new registeredServices %v in the exchange.", w.GetExchangeId(), newRegisteredServices)))
	}
}

// For the policy case get the new registeredServices for node from the active services and existing registeredServices.
// Some services in the existing registeredServices are suspended, we need to move them over.
func composeNewRegisteredServices(activeServices []exchange.Microservice, oldRegisteredServices []exchange.Microservice) ([]exchange.Microservice, bool) {
	// go through existing registeredServices and move the suspended services into the new registsteredServices
	isSame := true
	if activeServices == nil {
		activeServices = []exchange.Microservice{}
	}

	if oldRegisteredServices == nil {
		return activeServices, len(activeServices) == 0
	}

	newRegisteredServices := make([]exchange.Microservice, len(activeServices))
	copy(newRegisteredServices, activeServices)

	for _, rs := range oldRegisteredServices {
		found := false
		for i, nrs := range activeServices {
			if rs.Url == nrs.Url && (rs.Version == nrs.Version || rs.Version == "" || nrs.Version == "") {
				found = true
				if rs.ConfigState != nrs.ConfigState {
					isSame = false
					newRegisteredServices[i].ConfigState = exchange.SERVICE_CONFIGSTATE_SUSPENDED
				}
				break
			}
		}

		if !found {
			if rs.ConfigState == exchange.SERVICE_CONFIGSTATE_SUSPENDED {
				newRegisteredServices = append(newRegisteredServices, exchange.Microservice(rs))
			}
		}
	}

	// no changes, no need to update the node on the exchange.
	if isSame && len(oldRegisteredServices) == len(newRegisteredServices) {
		return newRegisteredServices, true
	} else {
		return newRegisteredServices, false
	}
}
