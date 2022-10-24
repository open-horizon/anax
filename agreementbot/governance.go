package agreementbot

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/agreementbot/persistence"
	"github.com/open-horizon/anax/basicprotocol"
	"github.com/open-horizon/anax/compcheck"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/policy"
	"math"
	"net/http"
	"strings"
	"time"
)

const (
	WORKLOAD_STATUS_UPGRADING = 1
	WORKLOAD_STATUS_UPGRADED  = 2
)

func (w *AgreementBotWorker) GovernAgreements() int {

	// This is the amount of time for the routine to wait as discovered through scanning active agreements. Node health
	// checks might be skipped if they dont have to occur every time this function wakes up. The idea is to do one scan
	// of all agreements and do as much checking as necessary, but not more.
	discoveredNHWaitTime := uint64(0) // Shortest node health check rate value across all agreements.

	// A filter for limiting the returned set of agreements just to those that are in progress and not yet timed out.
	notYetFinalFilter := func() persistence.AFilter {
		return func(a persistence.Agreement) bool { return a.AgreementCreationTime != 0 && a.AgreementTimedout == 0 }
	}

	// Reset the updated status of the Node Health manager. This ensures that the agbot will attempt to get updated status
	// info from the exchange. The exchange might return no updates, but at least the agbot asked for updates.
	if w.GovTiming.nhSkip == 0 {
		w.NHManager.ResetUpdateStatus()
	}

	// Grab the next set of secret updates to process.
	secretUpdates := w.secretUpdateManager.GetNextUpdateEvent()

	// Look at all agreements across all protocols
	for _, agp := range policy.AllAgreementProtocols() {

		protocolHandler := w.consumerPH.Get(agp)

		// Find all agreements that are in progress. They might be waiting for a reply or not yet finalized on blockchain.
		if agreements, err := w.db.FindAgreements([]persistence.AFilter{notYetFinalFilter(), persistence.UnarchivedAFilter()}, agp); err == nil {

			// set the node orgs for the given agreement protocol
			if w.GovTiming.nhSkip == 0 {
				glog.V(5).Infof("AgreementBot Governance saving the node orgs to the node health manager for all active agreements under %v protocol.", agp)
				w.NHManager.SetNodeOrgs(agreements, agp)
			}

			// map of updated secrets with key agreementOrg_secretUser_secretName
			updatedSecretsMap := make(map[string]string)

			for _, ag := range agreements {

				// Govern agreements that have seen a reply from the device
				if protocolHandler.AlreadyReceivedReply(&ag) {

					// For agreements that havent seen a blockchain write yet, check timeout
					if ag.AgreementFinalizedTime == 0 {

						glog.V(5).Infof("AgreementBot Governance detected agreement %v not yet final.", ag.CurrentAgreementId)
						timeout := ag.AgreementTimeoutS
						if timeout == 0 {
							timeout, _ = w.SetAgreementTimeouts(ag, agp)
						}
						now := uint64(time.Now().Unix())
						if ag.AgreementCreationTime+timeout < now {
							// Start timing out the agreement
							w.TerminateAgreement(&ag, protocolHandler.GetTerminationCode(TERM_REASON_NOT_FINALIZED_TIMEOUT))
						}
					}

					// Do node health check only if not skipping it this time.
					if w.GovTiming.nhSkip == 0 {
						// Check for agreement termination based on node health issues. Checking node health might require an expensive
						// call to the exchange for batch node status, so only do the health checks if we have to.
						if checkrate, err := w.VerifyNodeHealth(&ag, protocolHandler); err != nil {
							glog.Errorf(logString(fmt.Sprintf("unable to verify node health for %v, error: %v", ag.CurrentAgreementId, err)))
						} else if checkrate != 0 && (discoveredNHWaitTime == 0 || (discoveredNHWaitTime != 0 && uint64(checkrate) < discoveredNHWaitTime)) {
							discoveredNHWaitTime = uint64(checkrate)
						}
					}

					// Govern agreements that havent seen a proposal reply yet
				} else {
					// We are waiting for a reply
					glog.V(5).Infof("AgreementBot Governance waiting for reply to %v.", ag.CurrentAgreementId)
					timeout := ag.ProtocolTimeoutS
					if timeout == 0 {
						_, timeout = w.SetAgreementTimeouts(ag, agp)
					}
					now := uint64(time.Now().Unix())
					if ag.AgreementCreationTime+timeout < now {
						w.nodeSearch.AddRetry(ag.PolicyName, ag.AgreementCreationTime-w.BaseWorker.Manager.Config.GetAgbotRetryLookBackWindow())
						w.TerminateAgreement(&ag, protocolHandler.GetTerminationCode(TERM_REASON_NO_REPLY))
					}
				}

				// If any secrets have changed, existing agreements will need to be updated with the new secrets. Check this agreement
				// to see if it needs to be updated.
				if secretUpdates != nil {

					// Is the current agreement affected by secrets that have changed? If so, return the secrets that have changed.
					var updatedSecrets []string
					var newestUpdateTime uint64
					if ag.Pattern != "" {
						newestUpdateTime, updatedSecrets = secretUpdates.GetUpdatedSecretsForPattern(ag.Pattern, ag.LastSecretUpdateTime)
					} else {
						newestUpdateTime, updatedSecrets = secretUpdates.GetUpdatedSecretsForPolicy(ag.PolicyName, ag.LastSecretUpdateTime)
					}

					// If there are secret updates for this agreement AND the agreement has not seen these updates yet, then process them for this agreement.
					if len(updatedSecrets) != 0 && ag.LastSecretUpdateTime < newestUpdateTime {

						// Extract the consumer policy from agreement.
						pol, err := policy.DemarshalPolicy(ag.Policy)
						if err != nil {
							glog.Errorf(logString(fmt.Sprintf("unable to demarshal consumer policy for agreement %s, error: %v", ag.CurrentAgreementId, err)))
						}

						// Collect the updated secrets into a list of new secret bindings to send to the agent.
						updatedBindings := make([]exchangecommon.SecretBinding, 0)

						glog.V(3).Infof(logString(fmt.Sprintf("handling %s with %v for updated secrets %v, newest update time %v", ag.CurrentAgreementId, pol.SecretBinding, updatedSecrets, newestUpdateTime)))

						for _, binding := range pol.SecretBinding {

							// The secret details are transported within the proposal in the SecretBinding section where the secret provider secret name is
							// replaced with the secret details.
							sb := binding.MakeCopy()
							sb.Secrets = make([]exchangecommon.BoundSecret, 0)
							bindingUpdate := false

							for _, bs := range binding.Secrets {

								serviceSecretName, smSecretName := bs.GetBinding()
								for _, updatedSecretName := range updatedSecrets {
									if glog.V(5) {
										glog.Infof(logString(fmt.Sprintf("checking secret %v against %v", updatedSecretName, bs)))
									}
									if smSecretName == exchange.GetId(updatedSecretName) {

										// Call the secret manager plugin to get the secret details.
										secretUser, secretName, err := compcheck.ParseVaultSecretName(exchange.GetId(updatedSecretName), nil)
										if err != nil {
											glog.Errorf(logString(fmt.Sprintf("error parsing secret %s, error: %v", updatedSecretName, err)))
											continue
										}

										newBS := make(exchangecommon.BoundSecret)
										secretLookupKey := fmt.Sprintf("%v_%v_%v", ag.Org, secretUser, secretName)
										//# check if new secret value already retrieved
										if val, ok := updatedSecretsMap[secretLookupKey]; ok {
											newBS[serviceSecretName] = val
										} else {
											details, err := w.secretProvider.GetSecretDetails(w.GetExchangeId(), w.GetExchangeToken(), exchange.GetOrg(updatedSecretName), secretUser, secretName)
											if err != nil {
												glog.Errorf(logString(fmt.Sprintf("error retrieving secret %v for policy %v, error: %v", updatedSecretName, ag.PolicyName, err)))
												continue
											}
											detailBytes, err := json.Marshal(details)
											if err != nil {
												glog.Errorf(logString(fmt.Sprintf("error marshalling secret details of %v for policy %v, error: %v", updatedSecretName, ag.PolicyName, err)))
												continue
											} else {
												encodedDetails := base64.StdEncoding.EncodeToString(detailBytes)
												newBS[serviceSecretName] = encodedDetails
												updatedSecretsMap[secretLookupKey] = encodedDetails
											}
										}

										sb.Secrets = append(sb.Secrets, newBS)
										bindingUpdate = true

										break
									}
								}
							}

							if bindingUpdate {
								updatedBindings = append(updatedBindings, sb)
							}
						}

						if glog.V(5) {
							glog.Infof(logString(fmt.Sprintf("sending secret updates %v to the agent for %s", updatedBindings, ag.CurrentAgreementId)))
						}

						// Send the Update Agreement protocol message
						protocolHandler.UpdateAgreement(&ag, basicprotocol.MsgUpdateTypeSecret, updatedBindings, protocolHandler)

						if _, err := w.db.AgreementSecretUpdateTime(ag.CurrentAgreementId, agp, newestUpdateTime); err != nil {
							glog.Errorf(logString(fmt.Sprintf("unable to save secret update time for %s, error: %v", ag.CurrentAgreementId, err)))
						}

					}
				}
			}

		} else {
			msg := logString(fmt.Sprintf("unable to read agreements from database, error: %v", err))
			glog.Errorf(msg)

			// This is the case where Postgresql database certificate got upgraded. The error is: "x509: certificate signed by unknown authority"
			// Only checks for "x509" here to support globalization.
			if strings.Contains(err.Error(), "x509") || strings.Contains(err.Error(), "X509") {
				glog.Warningf(logString(fmt.Sprintf("The agbot will panic due to the database certificate error.")))
				panic(msg)
			}
		}
	}

	// After processing all agreements, update the agbot's secret manager DB to indicate that all secrets have been processed.
	if secretUpdates != nil {
		for _, su := range secretUpdates.Updates {
			glog.V(5).Infof(logString(fmt.Sprintf("updating secret DB %s/%s with time %v", su.SecretOrg, su.SecretFullName, su.SecretUpdateTime)))

			err := w.db.SetSecretUpdate(su.SecretOrg, su.SecretFullName, su.SecretUpdateTime)
			if err != nil {
				glog.Errorf(logString(fmt.Sprintf("unable to save secret update time for %s/%s, error: %v", su.SecretOrg, su.SecretFullName, err)))
			}
		}
	}

	// Govern the HA partners by examining workload usage records.
	w.governHAPartners()

	// Dynamically adjust skips to account for long NH check rates.
	if w.GovTiming.nhSkip == 0 {
		w.GovTiming.nhSkip = calculateSkipTime(discoveredNHWaitTime, w.BaseWorker.Manager.Config.AgreementBot.ProcessGovernanceIntervalS)
	} else {
		// Decrement skip count here to prepare for next iteration
		if w.GovTiming.nhSkip > 0 {
			w.GovTiming.nhSkip = w.GovTiming.nhSkip - 1
		}
	}
	glog.V(5).Infof(logString(fmt.Sprintf("sleeping for %v seconds, skipping node health %v time(s).", w.BaseWorker.Manager.Config.AgreementBot.ProcessGovernanceIntervalS, w.GovTiming.nhSkip)))
	return int(w.BaseWorker.Manager.Config.AgreementBot.ProcessGovernanceIntervalS)

}

// Calculate wait time intervals for node health checks before we run the next agreement iteration(s). When the skip count is zero, this function
// will get called again to recalculate the skips.
func calculateSkipTime(nhCheckrate uint64, pgi uint64) uint64 {

	var nhSkip uint64

	if nhCheckrate > pgi {
		nhSkip = uint64(math.Abs((float64(nhCheckrate) / float64(pgi)) - 1.0))
		if nhCheckrate > 60 {
			nhSkip = nhSkip / 2
		}
	}

	return nhSkip
}

// Proactively check the state of pending workload upgrades for HA devices. When the need for an upgrade is detected, one of the
// devices in the HA group is chosen for upgrade and the others are marked for a pending upgrade (in their workload usage record).
// The goal of this routine is to detect when 1 member of the group is upgraded and it's safe to start to upgrade another member.
//
// Workload usage records survive agreement cancellations. They track the current workload being run on the device. We can be certain of
// this because proposals from agbots to devices only contain a single workload choice.

// First, make a more optimized quick check to see if there is anything we need to do by looking for any workload
// usage records that need to be upgraded. If there are none, then there is no need to do a more exhaustive analysis of
// the state of the HA group. Non-HA workload usages dont have the concern about incremental workload upgrades, so they are ignored
// by this routine.
// Note: Multiple agbot could call this function at the same time (for different agreement).
//
//	Table workloadusage is partitioned. So one agbot could only see the workloadusage in
//	its own partition. Table ha_workload_upgrade is not partitioned.
func (w *AgreementBotWorker) governHAPartners() {
	// Part A: remove all entries from the ha_workload_upgrade table if the upgrade is done.
	// Part B: handle workloaduages that has pendingUpdateTime != 0
	// 1. get all the workload with pendingUpdateTime != 0
	// 2. for each workload get from step 1:
	//    - get device of that workload (device id, org)
	//    - device != nil then get hagroup name of that device
	//         - hagroup != "":
	//               - check ha workload upgrade table with (org, hagroupName, workload.policyName)
	//                     - doesn't have entry
	//                           => no one is upgrading, can update this workload: insert this workload in table, delete workload from db, cancel agreement if there is one
	//         - hagroup == ""
	//              - upgrade this workload: delete workload from db, cancel agreement if there is one

	glog.V(5).Infof(logString("checking for HA partners needing a workload upgrade."))

	// check if current workload is upgraded.
	// remove it from the ha_workload_upgrade table if upgraded.
	haWorkloads, err := w.db.ListAllHAUpgradingWorkloads()
	if err != nil {
		glog.Errorf(logString(fmt.Sprintf("Failed to get all entries from HA_workload_upgrade table. %v", err)))
		return
	}
	if haWorkloads == nil || len(haWorkloads) == 0 {
		glog.V(5).Infof(AWlogString(fmt.Sprintf("No rentires in the HA_workload_upgrade table.")))
	} else {
		glog.V(5).Infof(AWlogString(fmt.Sprintf("There are %v entries in the HA_workload_upgrade table.", len(haWorkloads))))

		for _, ha_wlu := range haWorkloads {
			statusCode := w.checkWorkloadStatus(ha_wlu.NodeId, ha_wlu.PolicyName)
			glog.V(5).Infof(AWlogString(fmt.Sprintf("Upgrade status code is %v for %v", statusCode, ha_wlu)))

			if statusCode == WORKLOAD_STATUS_UPGRADED {
				glog.V(5).Infof(AWlogString(fmt.Sprintf("Upgrade completed. Removing HA upgrading record %v", ha_wlu)))
				// remove the entry from the ha_workload_upgrade table for the ones that are done
				if err := w.db.DeleteHAUpgradingWorkload(ha_wlu); err != nil {
					// might not be an error if the entry is deleted by another agbot
					glog.Warningf(logString(fmt.Sprintf("unable to delete the HA upgrading workload record %v. %v", ha_wlu, err)))
				}
			}
		}
	}

	// find the workloads that are waiting to upgrade
	HAPendingUpgradeWUFilter := func() persistence.WUFilter {
		return func(a persistence.WorkloadUsage) bool { return a.PendingUpgradeTime != 0 }
	}
	if upgrades, err := w.db.FindWorkloadUsages([]persistence.WUFilter{HAPendingUpgradeWUFilter()}); err != nil {
		glog.Errorf(logString(fmt.Sprintf("error searching for workload usage that are waiting for upgrade, error: %v", err)))
		return
	} else if len(upgrades) != 0 {
		for _, wlu := range upgrades {
			glog.V(5).Infof(logString(fmt.Sprintf("checking for workload usage %v that are waiting for upgrading", wlu.String())))
			// Setup variables to track the state of the HA group that the current workload usage record belongs to.
			device, err := GetDevice(w.GetHTTPFactory().NewHTTPClient(nil), wlu.DeviceId, w.GetExchangeURL(), w.GetExchangeId(), w.GetExchangeToken())
			if err != nil {
				glog.Errorf(logString(fmt.Sprintf("error getting device %v, error: %v", wlu.DeviceId, err)))
				return
			} else if device == nil {
				// ignore it? continue to next waiting workload
				continue
			} else if device.HAGroup == "" {
				// update this workload:
				glog.V(5).Infof(logString(fmt.Sprintf("device %v does not belong to hagroup, update workload %v.", device, wlu.String())))
				w.UpgradeWorkload(wlu)
			} else { // device != nil && device.HAGroup != ""
				glog.V(5).Infof(logString(fmt.Sprintf("device %v belongs to hagroup %v.", device, device.HAGroup)))
				haGroupName := device.HAGroup
				org := exchange.GetOrg(wlu.DeviceId)
				currentUpgradingWorkloadForGroup, err := w.db.GetHAUpgradingWorkload(org, haGroupName, wlu.PolicyName)
				if err != nil {
					glog.Errorf(logString(fmt.Sprintf("error getting ha upgrading workload for %v/%v/%v, error: %v", org, haGroupName, wlu.PolicyName, err)))
					return
				} else if currentUpgradingWorkloadForGroup == nil {
					glog.V(5).Infof(logString(fmt.Sprintf("no workload is upgrading for hagroup %v, now upgrade the workload: %v.", device.HAGroup, wlu.String())))
					// insert this workload into ha workload upgrade table, then upgrade this workload
					if err = w.db.InsertHAUpgradingWorkloadForGroupAndPolicy(org, haGroupName, wlu.PolicyName, wlu.DeviceId); err != nil {
						// might not be an error if the insertion was done by another agbot before this one
						glog.Warningf(logString(fmt.Sprintf("unable to insert HA upgrading workloads with hagroup %v, org: %v, policyName: %v deviceId: %v. %v", haGroupName, org, wlu.PolicyName, wlu.DeviceId, err)))
					} else {
						w.UpgradeWorkload(wlu)
					}
				}
			}
		}
	}
}

// check if a workload is upgrading (1), upgrading(2), other(0)
func (w *AgreementBotWorker) checkWorkloadStatus(nodeId string, policyName string) int {
	wlu, err := w.db.FindSingleWorkloadUsageByDeviceAndPolicyName(nodeId, policyName)
	if err != nil {
		// might not be error if the wlu in not in the same partition as this agbot
		glog.Warningf(logString(fmt.Sprintf("could not get workload usage for node %v, policy %v. %v", nodeId, policyName, err)))
		return 0
	} else if wlu == nil || wlu.PendingUpgradeTime != 0 {
		// If it doesnt have a workload usage record, then it is because that it is upgrading.
		// Workload usage records are deleted when we want to upgrade a device. We also cancel the previous agreement.
		return WORKLOAD_STATUS_UPGRADING
	} else { //wlu.PendingUpgradeTime == 0
		upgrading, upgraded := w.checkWorkloadUsageAgreement(wlu)
		if upgraded != "" {
			return WORKLOAD_STATUS_UPGRADED
		}
		if upgrading != "" {
			return WORKLOAD_STATUS_UPGRADING
		}
	}
	return 0
}

// This function is used to determine if a device is actively trying to make an agreement. This is important to know because
// a device in an HA group that is in the midst of making an agreement will prevent the agbot from upgrading other HA
// partners. This function also considers the possibility that an HA partner has stopped heart beating (because it died), and
// therefore wont be making any agreements right now. In that case, we skip that device and look for others to start upgrading.
func (w *AgreementBotWorker) checkWorkloadUsageAgreement(partnerWLU *persistence.WorkloadUsage) (string, string) {

	// partnerWLU.PendingUpgradeTime == 0 && this wlu.PendingUpgradeTime != 0
	partnerUpgrading := ""
	upgradedPartnerFound := ""

	if ag, err := w.db.FindSingleAgreementByAgreementIdAllProtocols(partnerWLU.CurrentAgreementId, policy.AllAgreementProtocols(), []persistence.AFilter{persistence.UnarchivedAFilter()}); err != nil {
		glog.Warningf(logString(fmt.Sprintf("unable to read agreement %v from database. %v", partnerWLU.CurrentAgreementId, err)))
	} else if ag == nil {
		// If we dont find an agreement for a partner, then it is because a previous agreement with that partner has failed and we
		// managed to catch the workload usage record in a transition state between agreement attempts.
		// Check to make sure the partner is heart-beating to the exchange. This should tell us if we can expect this device to
		// complete an agreement at some time, or not.

		if dev, err := GetDevice(w.Config.Collaborators.HTTPClientFactory.NewHTTPClient(nil), partnerWLU.DeviceId, w.GetExchangeURL(), w.GetExchangeId(), w.GetExchangeToken()); err != nil {
			glog.Errorf(logString(fmt.Sprintf("error obtaining device %v heartbeat state: %v", partnerWLU.DeviceId, err)))
		} else if len(dev.LastHeartbeat) != 0 && (uint64(cutil.TimeInSeconds(dev.LastHeartbeat, cutil.ExchangeTimeFormat)+300) > uint64(time.Now().Unix())) {
			// If the device is still alive (heart beat received in the last 5 mins), then assume this partner is trying to make an
			// agreement. Exit the partner loop because no one else can safely upgrade right now. The upgrade might be bad.
			glog.V(5).Infof(logString(fmt.Sprintf("HA group member %v is upgrading.", partnerWLU.DeviceId)))
			partnerUpgrading = partnerWLU.DeviceId
		} else {
			// If the device is not alive then ignore it. We dont want this failed device to hold up the workload
			// upgrade of other devices.
			glog.V(5).Infof(logString(fmt.Sprintf("HA group member %v is not heartbeating.", partnerWLU.DeviceId)))
		}
	} else if ag.DataVerifiedTime != ag.AgreementCreationTime && ag.AgreementTimedout == 0 {
		// If we find a partner with an agreement where data has been verified and that is also not being cancelled,
		// then we have found a partner who is upgraded. Now we just need to make sure this partner is running the highest
		// priority workload. If not, then it is not considered to be upgraded.

		if pol, err := policy.DemarshalPolicy(partnerWLU.Policy); err != nil {
			glog.Errorf(logString(fmt.Sprintf("unable to demarshal policy for workload usage %v, error %v", partnerWLU, err)))
		} else {
			workload := pol.NextHighestPriorityWorkload(0, 0, 0)
			if partnerWLU.Priority == workload.Priority.PriorityValue {
				glog.V(5).Infof(logString(fmt.Sprintf("HA group member %v has upgraded.", partnerWLU.DeviceId)))
				upgradedPartnerFound = partnerWLU.DeviceId
			}
		}
	} else {
		// All other states that the agreement might be in are considered to be making an agreement and therefore the
		// partner is considered to be upgrading.
		partnerUpgrading = partnerWLU.DeviceId
	}

	return partnerUpgrading, upgradedPartnerFound
}

func (w *AgreementBotWorker) UpgradeWorkload(wlu persistence.WorkloadUsage) {
	unarchived := []persistence.AFilter{persistence.UnarchivedAFilter()}
	if ag, err := w.db.FindSingleAgreementByAgreementIdAllProtocols(wlu.CurrentAgreementId, policy.AllAgreementProtocols(), unarchived); err != nil {
		glog.Errorf(logString(fmt.Sprintf("unable to read agreement %v from database, error: %v", wlu.CurrentAgreementId, err)))
	} else {
		// Make sure the workload usage record is gone,this will allow the device to pick up the newest workload.
		if err := w.db.DeleteWorkloadUsage(wlu.DeviceId, wlu.PolicyName); err != nil {
			glog.Errorf(logString(fmt.Sprintf("error deleting workload usage for %v using policy %v, error: %v", wlu.DeviceId, wlu.PolicyName, err)))
		}

		// Cancel the agreement if there is one
		if ag == nil {
			glog.V(5).Infof(logString(fmt.Sprintf("agreement for %v already terminated.", wlu.DeviceId)))

		} else {
			w.TerminateAgreement(ag, w.consumerPH.Get(ag.AgreementProtocol).GetTerminationCode(TERM_REASON_POLICY_CHANGED))
		}
	}
}

// This function is used to verify that a node is still functioning correctly
func (w *AgreementBotWorker) VerifyNodeHealth(ag *persistence.Agreement, cph ConsumerProtocolHandler) (int, error) {

	// If there is no node health policy configured, or the agreement is not yet ready to be checked, return quickly.
	if !ag.NodeHealthInUse() || ag.AgreementFinalizedTime == 0 {
		return 0, nil
	}

	nodeHealthHandler := func(pattern string, org string, nodeOrgs []string, lastCallTime string) (*exchange.NodeHealthStatus, error) {
		return exchange.GetNodeHealthStatus(w.Config.Collaborators.HTTPClientFactory, pattern, org, nodeOrgs, lastCallTime, w.GetExchangeURL(), w.GetExchangeId(), w.GetExchangeToken())
	}

	glog.V(5).Infof("AgreementBot Governance checking node health for %v.", ag.CurrentAgreementId)

	// Make sure the Node Health Manager has updated info for this agreement's pattern.
	if err := w.NHManager.SetUpdatedStatus(ag.Pattern, ag.Org, nodeHealthHandler); err != nil {
		return ag.NHCheckAgreementStatus, errors.New(fmt.Sprintf("unable to update node health for %v, error %v", ag.Pattern, err))
	}

	// If this agreement's node is out of policy, cancel the agreement and remove the node from the cache.
	// If the agreement is missing, cancel it.
	if w.NHManager.NodeOutOfPolicy(ag.Pattern, ag.Org, ag.DeviceId, ag.NHMissingHBInterval) {
		w.TerminateAgreement(ag, cph.GetTerminationCode(TERM_REASON_NODE_HEARTBEAT))
	} else if w.NHManager.AgreementOutOfPolicy(ag.Pattern, ag.Org, ag.DeviceId, ag.CurrentAgreementId, ag.AgreementFinalizedTime, ag.NHCheckAgreementStatus) {
		w.TerminateAgreement(ag, cph.GetTerminationCode(TERM_REASON_AG_MISSING))
	}

	return ag.NHCheckAgreementStatus, nil
}

func (w *AgreementBotWorker) TerminateAgreement(ag *persistence.Agreement, reason uint) {
	// Start timing out the agreement
	glog.V(3).Infof(logString(fmt.Sprintf("detected agreement %v needs to terminate.", ag.CurrentAgreementId)))

	// Update the database
	if _, err := w.db.AgreementTimedout(ag.CurrentAgreementId, ag.AgreementProtocol); err != nil {
		glog.Errorf(logString(fmt.Sprintf("error marking agreement %v terminate: %v", ag.CurrentAgreementId, err)))
	}

	// Queue up a command for an agreement worker to do the blockchain work
	w.consumerPH.Get(ag.AgreementProtocol).HandleAgreementTimeout(NewAgreementTimeoutCommand(ag.CurrentAgreementId, ag.AgreementProtocol, reason), w.consumerPH.Get(ag.AgreementProtocol))
}

func GetDevice(httpClient *http.Client, deviceId string, url string, agbotId string, token string) (*exchange.Device, error) {

	glog.V(5).Infof(logString(fmt.Sprintf("retrieving device %v from exchange", deviceId)))

	cachedDevice := exchange.GetNodeFromCache(exchange.GetOrg(deviceId), exchange.GetId(deviceId))
	if cachedDevice != nil {
		return cachedDevice, nil
	}

	var resp interface{}
	resp = new(exchange.GetDevicesResponse)
	targetURL := url + "orgs/" + exchange.GetOrg(deviceId) + "/nodes/" + exchange.GetId(deviceId)
	for {
		if err, tpErr := exchange.InvokeExchange(httpClient, "GET", targetURL, agbotId, token, nil, &resp); err != nil {
			glog.Errorf(logString(err.Error()))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(logString(tpErr.Error()))
			time.Sleep(10 * time.Second)
			continue
		} else {
			devs := resp.(*exchange.GetDevicesResponse).Devices
			if dev, there := devs[deviceId]; !there {
				return nil, errors.New(fmt.Sprintf("device %v not in GET response %v as expected", deviceId, devs))
			} else {
				glog.V(5).Infof(logString(fmt.Sprintf("retrieved device %v from exchange %v", deviceId, dev)))
				exchange.UpdateCache(exchange.NodeCacheMapKey(exchange.GetOrg(deviceId), exchange.GetId(deviceId)), exchange.NODE_DEF_TYPE_CACHE, dev)
				return &dev, nil
			}
		}
	}
}

// Govern the archived agreements, periodically deleting them from the database if they are old enough. The
// age limit is defined by the agbot configuration, PurgeArchivedAgreementHours.
func (w *AgreementBotWorker) GovernArchivedAgreements() int {

	// Default to purging archived agreements an hour after they are terminated.
	ageLimit := 1
	if w.Config.AgreementBot.PurgeArchivedAgreementHours != 0 {
		ageLimit = w.Config.AgreementBot.PurgeArchivedAgreementHours
	} else {
		glog.Info(logString(fmt.Sprintf("archive purge using default age limit of %v hour.", ageLimit)))
	}

	glog.V(5).Infof(logString(fmt.Sprintf("archive purge scanning for agreements archived more than %v hour(s) ago.", ageLimit)))

	// A filter for limiting the returned set of agreements to just those that are too old.
	agedOutFilter := func(now int64, limitH int) persistence.AFilter {
		return func(a persistence.Agreement) bool {
			return a.AgreementTimedout != 0 && (a.AgreementTimedout+uint64(limitH*3600) <= uint64(now))
		}
	}

	// Find all archived agreements that are old enough and delete them.
	for _, agp := range policy.AllAgreementProtocols() {
		now := time.Now().Unix()
		if agreements, err := w.db.FindAgreements([]persistence.AFilter{persistence.ArchivedAFilter(), agedOutFilter(now, ageLimit)}, agp); err == nil {
			for _, ag := range agreements {
				if err := w.db.DeleteAgreement(ag.CurrentAgreementId, agp); err != nil {
					glog.Error(logString(fmt.Sprintf("error deleting archived agreement %v, error: %v", ag.CurrentAgreementId, err)))
				} else {
					glog.V(3).Infof(logString(fmt.Sprintf("archive purge deleted %v", ag.CurrentAgreementId)))
				}
			}

		} else {
			glog.Errorf(logString(fmt.Sprintf("unable to read archived agreements from database for protocol %v, error: %v", agp, err)))
		}
	}
	return 0
}

// Govern the active agreements, reporting which ones need a blockchain running so that the blockchain workers
// can keep them running.
func (w *AgreementBotWorker) GovernBlockchainNeeds() int {

	// Find all agreements that need a blockchain by searching through all the agreement protocol DB buckets
	for _, agp := range policy.AllAgreementProtocols() {

		// If the agreement protocol doesnt require a blockchain then we can skip it.
		if bcType := policy.RequiresBlockchainType(agp); bcType == "" {
			continue
		} else {

			// Make a map of all blockchain names that we need to have running
			neededBCs := make(map[string]map[string]bool)
			if agreements, err := w.db.FindAgreements([]persistence.AFilter{persistence.UnarchivedAFilter()}, agp); err == nil {
				for _, ag := range agreements {
					_, bcName, bcOrg := w.consumerPH.Get(agp).GetKnownBlockchain(&ag)
					if bcName != "" {
						if _, ok := neededBCs[bcOrg]; !ok {
							neededBCs[bcOrg] = make(map[string]bool)
						}
						neededBCs[bcOrg][bcName] = true
					}
				}

				// If we captured any needed blockchains, inform the blockchain worker
				if len(neededBCs) != 0 {
					w.Messages() <- events.NewReportNeededBlockchainsMessage(events.BC_NEEDED, bcType, neededBCs)
				}

			} else {
				glog.Errorf(logString(fmt.Sprintf("unable to read agreements from database for protocol %v, error: %v", agp, err)))
			}

		}
	}
	return 0
}

// global log record prefix
var logString = func(v interface{}) string {
	return fmt.Sprintf("AgreementBot Governance: %v", v)
}
