package agreementbot

import (
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/agreementbot/persistence"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/policy"
	"math"
	"net/http"
	"time"
)

func (w *AgreementBotWorker) GovernAgreements() int {

	unarchived := []persistence.AFilter{persistence.UnarchivedAFilter()}

	// The length of time this governance routine waits is based on several factors. The data verification check rate
	// of any agreements that are being maintained and the default time specified in the agbot config. Assume that we
	// start with the default and adjust as necessary. The node health check rate also applies to the amount of time
	// this routine can wait.
	waitTime := w.BaseWorker.Manager.Config.AgreementBot.ProcessGovernanceIntervalS

	// This is the amount of time for the routine to wait as discovered through scanning active agreements. Node health
	// checks and data verification checks might be skipped if they each dont have to occur every time this function
	// wakes up. The idea is to do one scan of all agreements and do as much checking as necessary, but not more.
	discoveredDVWaitTime := uint64(0) // Shortest data verification check rate value across all agreements.
	discoveredNHWaitTime := uint64(0) // Shortest node health check rate value across all agreements.
	w.GovTiming.dvSkip = uint64(0)    // Number of times to skip data verification checks before actually doing the check.
	w.GovTiming.nhSkip = uint64(0)    // Number of times to skip node health checks before actually doing the check.

	// A filter for limiting the returned set of agreements just to those that are in progress and not yet timed out.
	notYetFinalFilter := func() persistence.AFilter {
		return func(a persistence.Agreement) bool { return a.AgreementCreationTime != 0 && a.AgreementTimedout == 0 }
	}

	// Reset the updated status of the Node Health manager. This ensures that the agbot will attempt to get updated status
	// info from the exchange. The exchange might return no updates, but at least the agbot asked for updates.
	w.NHManager.ResetUpdateStatus()

	// Look at all agreements across all protocols
	for _, agp := range policy.AllAgreementProtocols() {

		protocolHandler := w.consumerPH.Get(agp)

		// Find all agreements that are in progress. They might be waiting for a reply or not yet finalized on blockchain.
		if agreements, err := w.db.FindAgreements([]persistence.AFilter{notYetFinalFilter(), persistence.UnarchivedAFilter()}, agp); err == nil {
			activeDataVerification := true
			allActiveAgreements := make(map[string][]string)

			// set the node orgs for the given agreement protocol
			glog.V(5).Infof("AgreementBot Governance saving the node orgs to the node health manager for all active agreements under %v protocol.", agp)
			w.NHManager.SetNodeOrgs(agreements, agp)

			for _, ag := range agreements {

				// Govern agreements that have seen a reply from the device
				if protocolHandler.AlreadyReceivedReply(&ag) {

					// For agreements that havent seen a blockchain write yet, check timeout
					if ag.AgreementFinalizedTime == 0 {

						glog.V(5).Infof("AgreementBot Governance detected agreement %v not yet final.", ag.CurrentAgreementId)
						now := uint64(time.Now().Unix())
						if ag.AgreementCreationTime+w.BaseWorker.Manager.Config.AgreementBot.AgreementTimeoutS < now {
							// Start timing out the agreement
							w.TerminateAgreement(&ag, protocolHandler.GetTerminationCode(TERM_REASON_NOT_FINALIZED_TIMEOUT))
						}
					}

					// Do DV check only if not skipping it this time.
					if w.GovTiming.dvSkip == 0 {

						// Check for the receipt of data in the data ingest system (if necessary)
						if !ag.DisableDataVerificationChecks {

							// Capture the data verification check rate for later
							if discoveredDVWaitTime == 0 || (discoveredDVWaitTime != 0 && uint64(ag.DataVerificationCheckRate) < discoveredDVWaitTime) {
								discoveredDVWaitTime = uint64(ag.DataVerificationCheckRate)
							}

							// First check to see if this agreement is just not sending data. If so, terminate the agreement.
							now := uint64(time.Now().Unix())
							noDataLimit := w.BaseWorker.Manager.Config.AgreementBot.NoDataIntervalS
							if ag.DataVerificationNoDataInterval != 0 {
								noDataLimit = uint64(ag.DataVerificationNoDataInterval)
							}
							if now-ag.DataVerifiedTime >= noDataLimit {
								// No data is being received, terminate the agreement
								glog.V(3).Infof(logString(fmt.Sprintf("cancelling agreement %v due to lack of data", ag.CurrentAgreementId)))
								w.TerminateAgreement(&ag, protocolHandler.GetTerminationCode(TERM_REASON_NO_DATA_RECEIVED))

							} else if activeDataVerification {
								// Otherwise make sure the device is still sending data
								if ag.DataVerifiedTime+uint64(ag.DataVerificationCheckRate) > now {
									// It's not time to check again
									continue
								} else if activeAgreements, err := GetActiveAgreements(allActiveAgreements, ag, w.BaseWorker.Manager.Config); err != nil {
									glog.Errorf(logString(fmt.Sprintf("unable to retrieve active agreement list. Terminating data verification loop early, error: %v", err)))
									activeDataVerification = false
								} else if ActiveAgreementsContains(activeAgreements, ag, w.Config.AgreementBot.DVPrefix) {
									if _, err := w.db.DataVerified(ag.CurrentAgreementId, agp); err != nil {
										glog.Errorf(logString(fmt.Sprintf("unable to record data verification, error: %v", err)))
									}

									if ag.DataNotificationSent == 0 {
										// Get message address of the device from the exchange. The device ensures that the exchange is kept current.
										// If the address happens to be invalid, that should be a temporary condition. We will keep sending until
										// we get an ack to our verification message.
										if whisperTo, pubkeyTo, err := protocolHandler.GetDeviceMessageEndpoint(ag.DeviceId, "Governance"); err != nil {
											glog.Errorf(logString(fmt.Sprintf("error obtaining message target for data notification: %v", err)))
										} else if mt, err := exchange.CreateMessageTarget(ag.DeviceId, nil, pubkeyTo, whisperTo); err != nil {
											glog.Errorf(logString(fmt.Sprintf("error creating message target: %v", err)))
										} else if err := protocolHandler.AgreementProtocolHandler("", "", "").NotifyDataReceipt(ag.CurrentAgreementId, mt, protocolHandler.GetSendMessage()); err != nil {
											glog.Errorf(logString(fmt.Sprintf("unable to send data notification, error: %v", err)))
										}
									}

									// Check to see if it's time to send a metering notification
									// Create Metering notification. If the policy is empty, there's nothing to do.
									mp := policy.Meter{Tokens: ag.MeteringTokens, PerTimeUnit: ag.MeteringPerTimeUnit, NotificationIntervalS: ag.MeteringNotificationInterval}
									if mp.IsEmpty() {
										continue
									} else if ag.MeteringNotificationSent == 0 || (ag.MeteringNotificationSent != 0 && (ag.MeteringNotificationSent+uint64(ag.MeteringNotificationInterval)) <= now) {
										// Grab the blockchain info from the agreement if there is any

										bcType, bcName, bcOrg := protocolHandler.GetKnownBlockchain(&ag)
										glog.V(5).Info(logString(fmt.Sprintf("metering on %v %v", bcType, bcName)))

										// If we can write to the blockchain then we have all the info we need to do metering.
										if protocolHandler.IsBlockchainWritable(bcType, bcName, bcOrg) && protocolHandler.CanSendMeterRecord(&ag) {
											if mn, err := protocolHandler.CreateMeteringNotification(mp, &ag); err != nil {
												glog.Errorf(logString(fmt.Sprintf("unable to create metering notification, error: %v", err)))
											} else if whisperTo, pubkeyTo, err := protocolHandler.GetDeviceMessageEndpoint(ag.DeviceId, "Governance"); err != nil {
												glog.Errorf(logString(fmt.Sprintf("error obtaining message target for metering notification: %v", err)))
											} else if mt, err := exchange.CreateMessageTarget(ag.DeviceId, nil, pubkeyTo, whisperTo); err != nil {
												glog.Errorf(logString(fmt.Sprintf("error creating message target: %v", err)))
											} else if msg, err := protocolHandler.AgreementProtocolHandler(bcType, bcName, bcOrg).NotifyMetering(ag.CurrentAgreementId, mn, mt, protocolHandler.GetSendMessage()); err != nil {
												glog.Errorf(logString(fmt.Sprintf("unable to send metering notification, error: %v", err)))
											} else if _, err := w.db.MeteringNotification(ag.CurrentAgreementId, agp, msg); err != nil {
												glog.Errorf(logString(fmt.Sprintf("unable to record metering notification, error: %v", err)))
											}
										}
									}

									// Data verification has occured. If it has been maintained for the specified duration then we can turn off the
									// workload rollback retry checking feature.
									if wlUsage, err := w.db.FindSingleWorkloadUsageByDeviceAndPolicyName(ag.DeviceId, ag.PolicyName); err != nil {
										glog.Errorf(logString(fmt.Sprintf("unable to find workload usage record, error: %v", err)))
									} else if wlUsage != nil && !wlUsage.DisableRetry {
										if wlUsage.VerifiedDurationS == 0 || (wlUsage.VerifiedDurationS != 0 && ag.DataNotificationSent != 0 && ag.DataVerifiedTime != ag.AgreementCreationTime && (ag.DataVerifiedTime > ag.DataNotificationSent) && ((ag.DataVerifiedTime - ag.DataNotificationSent) >= uint64(wlUsage.VerifiedDurationS))) {
											glog.V(5).Infof(logString(fmt.Sprintf("disabling workload rollback for %v after %v seconds", ag.CurrentAgreementId, (ag.DataVerifiedTime - ag.DataNotificationSent))))
											if _, err := w.db.DisableRollbackChecking(ag.DeviceId, ag.PolicyName); err != nil {
												glog.Errorf(logString(fmt.Sprintf("unable to disable workload rollback retries, error: %v", err)))
											}
										}
									}

								} else if _, err := w.db.DataNotVerified(ag.CurrentAgreementId, agp); err != nil {
									glog.Errorf(logString(fmt.Sprintf("unable to record data not verified, error: %v", err)))
								}
							}
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
					now := uint64(time.Now().Unix())
					if ag.AgreementCreationTime+w.BaseWorker.Manager.Config.AgreementBot.ProtocolTimeoutS < now {
						w.nodeSearch.AddRetry(ag.PolicyName, ag.AgreementCreationTime-w.BaseWorker.Manager.Config.GetAgbotRetryLookBackWindow())
						w.TerminateAgreement(&ag, protocolHandler.GetTerminationCode(TERM_REASON_NO_REPLY))
					}
				}
			}
		} else {
			glog.Errorf(logString(fmt.Sprintf("unable to read agreements from database, error: %v", err)))
		}
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

	glog.V(5).Infof(logString(fmt.Sprintf("checking for HA partners needing a workload upgrade.")))

	HAPartnerUpgradeWUFilter := func() persistence.WUFilter {
		return func(a persistence.WorkloadUsage) bool { return len(a.HAPartners) != 0 && a.PendingUpgradeTime != 0 }
	}

	if upgrades, err := w.db.FindWorkloadUsages([]persistence.WUFilter{HAPartnerUpgradeWUFilter()}); err != nil {
		glog.Errorf(logString(fmt.Sprintf("error searching for HA devices that need their workloads upgraded, error: %v", err)))
	} else if len(upgrades) != 0 {

		for _, wlu := range upgrades {

			// Setup variables to track the state of the HA group that the current workload usage record belongs to.
			partnerUpgrading := ""
			upgradedPartnerFound := ""

			// Run through all the partners (wlUsage.HAPartners) of the current workload usage record.
			for _, partnerId := range wlu.HAPartners {
				glog.V(5).Infof(logString(fmt.Sprintf("analyzing HA group containing %v with partners %v", wlu.DeviceId, wlu.HAPartners)))

				if partnerWLU, err := w.db.FindSingleWorkloadUsageByDeviceAndPolicyName(partnerId, wlu.PolicyName); err != nil {
					glog.Errorf(logString(fmt.Sprintf("error obtaining partner workload usage record for device %v and policy %v, error: %v", partnerId, wlu.PolicyName, err)))
				} else if partnerWLU == nil {
					// If the partner doesnt have a workload usage record, then it is because that partner is upgrading.
					// Workload usage records are deleted when we want to upgrade a device. We also cancel the previous agreement.
					partnerUpgrading = partnerId
					glog.V(3).Infof(logString(fmt.Sprintf("HA group containing %v and %v has a member %v currently upgrading.", wlu.DeviceId, wlu.HAPartners, partnerId)))
					break

				} else if partnerWLU.PendingUpgradeTime != 0 {
					// Skip partners that are pending upgrade, they dont help us figure out if we can upgrade the current member.
					continue

				} else {
					// At this point we know that the partner WLU record is not pending an upgrade. Since it has a workload usage record,
					// then we know it has been attempting to make an agreement in the past. Check the state of the agreement that it
					// points to.

					partnerUpgrading, upgradedPartnerFound = w.checkWorkloadUsageAgreement(partnerWLU, &wlu)

					// If this partner is upgrading, then there is no reason to do further checks on the HA group.
					if partnerUpgrading != "" {
						break
					}
				}

			}

			// If there is already one partner successfully upgraded and there are no partners in the middle of an upgrade, then
			// begin upgrading the partner who needs it.
			if upgradedPartnerFound != "" && partnerUpgrading == "" {
				glog.V(3).Infof(logString(fmt.Sprintf("beginning upgrade of HA member %v in group %v.", wlu.DeviceId, wlu.HAPartners)))
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
			} else {
				glog.V(3).Infof(logString(fmt.Sprintf("no HA group members can be upgraded in group %v %v.", wlu.HAPartners, wlu.DeviceId)))
			}

		}

	}

	// Dynamically adjust wait time to account for large differential between DV check rates and NH check rates.
	if w.GovTiming.dvSkip == 0 && w.GovTiming.nhSkip == 0 {
		w.GovTiming.dvSkip, w.GovTiming.nhSkip, waitTime = calculateSkipTime(discoveredDVWaitTime, discoveredNHWaitTime, w.BaseWorker.Manager.Config.AgreementBot.ProcessGovernanceIntervalS)
	} else {
		// Decrement skip counts here to prepare for next iteration
		if w.GovTiming.dvSkip > 0 {
			w.GovTiming.dvSkip = w.GovTiming.dvSkip - 1
		}
		if w.GovTiming.nhSkip > 0 {
			w.GovTiming.nhSkip = w.GovTiming.nhSkip - 1
		}
	}
	glog.V(5).Infof(logString(fmt.Sprintf("sleeping for %v seconds, skipping data verification %v time(s), and skipping node health %v time(s).", waitTime, w.GovTiming.dvSkip, w.GovTiming.nhSkip)))
	return int(waitTime)

}

// Calculate wait time intervals for data verification and node health checks and come up with an aggregate wait time before we run
// the next agreement iteration(s) again. When all skip counts are zero, this function will get called again to recalculate wait and skips.
func calculateSkipTime(dvCheckrate uint64, nhCheckrate uint64, pgi uint64) (uint64, uint64, uint64) {

	var dvSkip, nhSkip, waitTime uint64

	if dvCheckrate == 0 && nhCheckrate == 0 {
		// No skips, default wait time.
		waitTime = pgi
	} else if cutil.Minuint64(dvCheckrate, nhCheckrate) == 0 {
		// No skips, non-zero wait time.
		waitTime = cutil.Maxuint64(dvCheckrate, nhCheckrate)
		if waitTime > 60 {
			waitTime = uint64(float64(waitTime) / 2)
		}
	} else {
		// Wait time is min of check rates and skips are calculated.
		waitTime = cutil.Minuint64(dvCheckrate, nhCheckrate)
		// Skips are a multiple of the 2 check rates, minus 1.
		if dvCheckrate == waitTime {
			diffRatio := math.Abs((float64(nhCheckrate) / float64(dvCheckrate)) - 1.0)
			nhSkip = uint64(diffRatio)
		} else {
			diffRatio := math.Abs((float64(dvCheckrate) / float64(nhCheckrate)) - 1.0)
			dvSkip = uint64(diffRatio)
		}
		// Down scale the wait time for long times
		if waitTime > 60 {
			waitTime = uint64(float64(waitTime) / 2)
		}
	}

	return dvSkip, nhSkip, waitTime
}

// This function is used to determine if a device is actively trying to make an agreement. This is important to know because
// a device in an HA group that is in the midst of making an agreement will prevent the agbot from upgrading other HA
// partners. This function also considers the possibility that an HA partner has stopped heart beating (because it died), and
// therefore wont be making any agreements right now. In that case, we skip that device and look for others to start upgrading.
func (w *AgreementBotWorker) checkWorkloadUsageAgreement(partnerWLU *persistence.WorkloadUsage, currentWLU *persistence.WorkloadUsage) (string, string) {

	partnerUpgrading := ""
	upgradedPartnerFound := ""

	if ag, err := w.db.FindSingleAgreementByAgreementIdAllProtocols(partnerWLU.CurrentAgreementId, policy.AllAgreementProtocols(), []persistence.AFilter{persistence.UnarchivedAFilter()}); err != nil {
		glog.Errorf(logString(fmt.Sprintf("unable to read agreement %v from database, error: %v", partnerWLU.CurrentAgreementId, err)))
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
			glog.V(5).Infof(logString(fmt.Sprintf("HA group member %v is upgrading, has partners %v %v.", partnerWLU.DeviceId, currentWLU.HAPartners, currentWLU.DeviceId)))
			partnerUpgrading = partnerWLU.DeviceId
		} else {
			// If the device is not alive then ignore it. We dont want this failed device to hold up the workload
			// upgrade of other devices.
			glog.V(5).Infof(logString(fmt.Sprintf("HA group member %v is not heartbeating, has partners %v %v.", partnerWLU.DeviceId, currentWLU.HAPartners, currentWLU.DeviceId)))
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
				glog.V(5).Infof(logString(fmt.Sprintf("HA group member %v has upgraded, has partners %v %v.", partnerWLU.DeviceId, currentWLU.HAPartners, currentWLU.DeviceId)))
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
//
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
