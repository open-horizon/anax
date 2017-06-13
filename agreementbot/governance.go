package agreementbot

import (
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/ethblockchain"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/metering"
	"github.com/open-horizon/anax/policy"
	"net/http"
	"time"
)

func (w *AgreementBotWorker) GovernAgreements() {

	glog.Info(logString(fmt.Sprintf("started agreement governance")))

	unarchived := []AFilter{UnarchivedAFilter()}

	// The length of time this governance routine waits is based on several factors. The data verification check rate
	// of any agreements that are being maintained and the default time specified in the agbot config. Assume that we
	// start with the default and adjust as necessary.
	waitTime := w.Worker.Manager.Config.AgreementBot.ProcessGovernanceIntervalS

	for {

		// This is the amount of time for the routine to wait as discovered through scanning active agreements.
		discoveredWaitTime := uint64(0)

		// A filter for limiting the resturned set of agreements just to those that are in progress and not yet timed out.
		notYetFinalFilter := func() AFilter {
			return func(a Agreement) bool { return a.AgreementCreationTime != 0 && a.AgreementTimedout == 0 }
		}

		// Look at all agreements across all protocols
		for _, agp := range policy.AllAgreementProtocols() {

			protocolHandler := w.consumerPH[agp]

			// Find all agreements that are in progress. They might be waiting for a reply or not yet finalized on blockchain.
			if agreements, err := FindAgreements(w.db, []AFilter{notYetFinalFilter(),UnarchivedAFilter()}, agp); err == nil {
				activeDataVerification := true
				allActiveAgreements := make(map[string][]string)
				for _, ag := range agreements {

					// Govern agreements that have seen a reply from the device
					if ag.CounterPartyAddress != "" {

						// For agreements that havent seen a blockchain write yet, check timeout
						if ag.AgreementFinalizedTime == 0 {

							glog.V(5).Infof("AgreementBot Governance detected agreement %v not yet final.", ag.CurrentAgreementId)
							now := uint64(time.Now().Unix())
							if ag.AgreementCreationTime+w.Worker.Manager.Config.AgreementBot.AgreementTimeoutS < now {
								// Start timing out the agreement
								w.TerminateAgreement(&ag, protocolHandler.GetTerminationCode(TERM_REASON_NOT_FINALIZED_TIMEOUT))
							}
						}

						// Check for the receipt of data in the data ingest system (if necessary)
						if !ag.DisableDataVerificationChecks {

							// Capture the data verification check rate for later
							if discoveredWaitTime == 0 || (discoveredWaitTime != 0 && uint64(ag.DataVerificationCheckRate) < discoveredWaitTime) {
								discoveredWaitTime = uint64(ag.DataVerificationCheckRate)
							}

							// First check to see if this agreement is just not sending data. If so, terminate the agreement.
							now := uint64(time.Now().Unix())
							noDataLimit := w.Worker.Manager.Config.AgreementBot.NoDataIntervalS
							if ag.DataVerificationNoDataInterval != 0 {
								noDataLimit = uint64(ag.DataVerificationNoDataInterval)
							}
							if now-ag.DataVerifiedTime >= noDataLimit {
								// No data is being received, terminate the agreement
								glog.V(3).Infof(logString(fmt.Sprintf("cancelling agreement %v due to lack of data", ag.CurrentAgreementId)))
								w.TerminateAgreement(&ag, protocolHandler.GetTerminationCode(TERM_REASON_NO_DATA_RECEIVED))

							} else if activeDataVerification {
								// Otherwise make sure the device is still sending data
								if ag.DataVerifiedTime + uint64(ag.DataVerificationCheckRate) > now {
									// It's not time to check again
									continue
								} else if activeAgreements, err := GetActiveAgreements(allActiveAgreements, ag, &w.Worker.Manager.Config.AgreementBot); err != nil {
									glog.Errorf(logString(fmt.Sprintf("unable to retrieve active agreement list. Terminating data verification loop early, error: %v", err)))
									activeDataVerification = false
								} else if ActiveAgreementsContains(activeAgreements, ag, w.Config.AgreementBot.DVPrefix) {
									if _, err := DataVerified(w.db, ag.CurrentAgreementId, agp); err != nil {
										glog.Errorf(logString(fmt.Sprintf("unable to record data verification, error: %v", err)))
									}

									if ag.DataNotificationSent == 0 {
										// Get message address of the device from the exchange. The device ensures that the exchange is kept current.
										// If the address happens to be invalid, that should be a temporary condition. We will keep sending until
										// we get an ack to our verification message.
										if whisperTo, pubkeyTo, err := getDeviceMessageEndpoint(ag.DeviceId, w.Config.AgreementBot.ExchangeURL, w.agbotId, w.token); err != nil {
											glog.Errorf(logString(fmt.Sprintf("error obtaining message target for data notification: %v", err)))
										} else if mt, err := exchange.CreateMessageTarget(ag.DeviceId, nil, pubkeyTo, whisperTo); err != nil {
											glog.Errorf(logString(fmt.Sprintf("error creating message target: %v", err)))
										} else if err := protocolHandler.AgreementProtocolHandler().NotifyDataReceipt(ag.CurrentAgreementId, mt, protocolHandler.GetSendMessage()); err != nil {
											glog.Errorf(logString(fmt.Sprintf("unable to send data notification, error: %v", err)))
										}
									}

									// Check to see if it's time to send a metering notification
									if ag.MeteringNotificationSent == 0 || (ag.MeteringNotificationSent != 0 && (ag.MeteringNotificationSent + uint64(ag.MeteringNotificationInterval)) <= now) {
										// Create Metering notification. If the policy is empty, there's nothing to do.
										mp := policy.Meter{Tokens: ag.MeteringTokens, PerTimeUnit: ag.MeteringPerTimeUnit, NotificationIntervalS: ag.MeteringNotificationInterval}
										if mp.IsEmpty() {
											continue
										}
										myAddress, _ := ethblockchain.AccountId()

										if mn, err := metering.NewMeteringNotification(mp, ag.AgreementCreationTime, uint64(ag.DataVerificationCheckRate), ag.DataVerificationMissedCount, ag.CurrentAgreementId, ag.ProposalHash, ag.ConsumerProposalSig, myAddress, ag.ProposalSig, "ethereum"); err != nil {
											glog.Errorf(logString(fmt.Sprintf("unable to create metering notification, error: %v", err)))
										} else if whisperTo, pubkeyTo, err := getDeviceMessageEndpoint(ag.DeviceId, w.Config.AgreementBot.ExchangeURL, w.agbotId, w.token); err != nil {
											glog.Errorf(logString(fmt.Sprintf("error obtaining message target for metering notification: %v", err)))
										} else if mt, err := exchange.CreateMessageTarget(ag.DeviceId, nil, pubkeyTo, whisperTo); err != nil {
											glog.Errorf(logString(fmt.Sprintf("error creating message target: %v", err)))
										} else if msg, err := protocolHandler.AgreementProtocolHandler().NotifyMetering(ag.CurrentAgreementId, mn, mt, protocolHandler.GetSendMessage()); err != nil {
											glog.Errorf(logString(fmt.Sprintf("unable to send metering notification, error: %v", err)))
										} else if _, err := MeteringNotification(w.db, ag.CurrentAgreementId, agp, msg); err != nil {
											glog.Errorf(logString(fmt.Sprintf("unable to record metering notification, error: %v", err)))
										}
									}

									// Data verification has occured. If it has been maintained for the specified duration then we can turn off the
									// workload rollback retry checking feature.
									if wlUsage, err := FindSingleWorkloadUsageByDeviceAndPolicyName(w.db, ag.DeviceId, ag.PolicyName); err != nil {
										glog.Errorf(logString(fmt.Sprintf("unable to find workload usage record, error: %v", err)))
									} else if wlUsage != nil && !wlUsage.DisableRetry {
										if wlUsage.VerifiedDurationS == 0 || (wlUsage.VerifiedDurationS != 0 && ag.DataNotificationSent != 0 && ag.DataVerifiedTime != ag.AgreementCreationTime && (ag.DataVerifiedTime > ag.DataNotificationSent) && ((ag.DataVerifiedTime-ag.DataNotificationSent) >= uint64(wlUsage.VerifiedDurationS))) {
											glog.V(5).Infof(logString(fmt.Sprintf("disabling workload rollback for %v after %v seconds", ag.CurrentAgreementId, (ag.DataVerifiedTime-ag.DataNotificationSent))))
											if _, err := DisableRollbackChecking(w.db, ag.DeviceId, ag.PolicyName); err != nil {
												glog.Errorf(logString(fmt.Sprintf("unable to disable workload rollback retries, error: %v", err)))
											}
										}
									}

								} else if _, err := DataNotVerified(w.db, ag.CurrentAgreementId, agp); err != nil {
									glog.Errorf(logString(fmt.Sprintf("unable to record data not verified, error: %v", err)))
								}
							}
						}

						// Govern agreements that havent seen a proposal reply yet
					} else {
						// We are waiting for a reply
						glog.V(5).Infof("AgreementBot Governance waiting for reply to %v.", ag.CurrentAgreementId)
						now := uint64(time.Now().Unix())
						if ag.AgreementCreationTime+w.Worker.Manager.Config.AgreementBot.ProtocolTimeoutS < now {
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

		HAPartnerUpgradeWUFilter := func() WUFilter {
		    return func(a WorkloadUsage) bool { return len(a.HAPartners) != 0 && a.PendingUpgradeTime != 0 }
		}

		if upgrades, err := FindWorkloadUsages(w.db, []WUFilter{HAPartnerUpgradeWUFilter()}); err != nil {
			glog.Errorf(logString(fmt.Sprintf("error searching for HA devices that need their workloads upgraded, error: %v", err)))
		} else if len(upgrades) != 0 {

			for _, wlu := range upgrades {

				// Setup variables to track the state of the HA group that the current workload usage record belongs to.
				partnerUpgrading := ""
				upgradedPartnerFound := ""

				// Run through all the partners (wlUsage.HAPartners) of the current workload usage record.
				for _, partnerId := range wlu.HAPartners {
					glog.V(5).Infof(logString(fmt.Sprintf("analyzing HA group containing %v with partners %v", wlu.DeviceId, wlu.HAPartners)))

					if partnerWLU, err := FindSingleWorkloadUsageByDeviceAndPolicyName(w.db, partnerId, wlu.PolicyName); err != nil {
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
					if ag, err := FindSingleAgreementByAgreementIdAllProtocols(w.db, wlu.CurrentAgreementId, policy.AllAgreementProtocols(), unarchived); err != nil {
						glog.Errorf(logString(fmt.Sprintf("unable to read agreement %v from database, error: %v", wlu.CurrentAgreementId, err)))
					} else {
						// Make sure the workload usage record is gone,this will allow the device to pick up the newest workload.
						if err := DeleteWorkloadUsage(w.db, wlu.DeviceId, wlu.PolicyName); err != nil {
							glog.Errorf(logString(fmt.Sprintf("error deleting workload usage for %v using policy %v, error: %v", wlu.DeviceId, wlu.PolicyName, err)))
						}

						// Cancel the agreement if there is one
						if ag == nil {
							glog.V(5).Infof(logString(fmt.Sprintf("agreement for %v already terminated.", wlu.DeviceId)))

						} else {
							w.TerminateAgreement(ag, w.consumerPH[ag.AgreementProtocol].GetTerminationCode(TERM_REASON_POLICY_CHANGED))
						}
					}
				} else {
					glog.V(3).Infof(logString(fmt.Sprintf("no HA group members can be upgraded in group %v %v.", wlu.HAPartners, wlu.DeviceId)))
				}

			}

		}

		// Dynamically adjust wait time to account for a very short Data Verification check rate. We are imposing an upper limit
		// of 30 seconds on the wait time. Even if all check rate times are > 30 seconds, we wont wait more than 30 seconds between
		// governance cycles.
		if discoveredWaitTime != 0 && discoveredWaitTime <= 30 {
			waitTime = discoveredWaitTime
		} else if discoveredWaitTime != 0 {
			waitTime = 30
		} else {
			waitTime = w.Worker.Manager.Config.AgreementBot.ProcessGovernanceIntervalS
		}
		glog.V(5).Infof(logString(fmt.Sprintf("sleeping for %v seconds.", waitTime)))
		time.Sleep(time.Duration(waitTime) * time.Second)
	}

	glog.Info(logString(fmt.Sprintf("terminated agreement governance")))

}

// This function is used to determine if a device is actively trying to make an agreement. This is important to know because
// a device in an HA group that is in the midst of making an agreement will prevent the agbot from upgrading other HA
// partners. This function also considers the possibility that an HA partner has stopped heart beating (because it died), and
// therefore wont be making any agreements right now. In that case, we skip that device and look for others to start upgrading.
func (w *AgreementBotWorker) checkWorkloadUsageAgreement(partnerWLU *WorkloadUsage, currentWLU *WorkloadUsage) (string, string) {

	partnerUpgrading := ""
	upgradedPartnerFound := ""

	if ag, err := FindSingleAgreementByAgreementIdAllProtocols(w.db, partnerWLU.CurrentAgreementId, policy.AllAgreementProtocols(), []AFilter{UnarchivedAFilter()}); err != nil {
		glog.Errorf(logString(fmt.Sprintf("unable to read agreement %v from database, error: %v", partnerWLU.CurrentAgreementId, err)))
	} else if ag == nil {
		// If we dont find an agreement for a partner, then it is because a previous agreement with that partner has failed and we
		// managed to catch the workload usage record in a transition state between agreement attempts.
		// Check to make sure the partner is heart-beating to the exchange. This should tell us if we can expect this device to
		// complete an agreement at some time, or not.

		if dev, err := getDevice(partnerWLU.DeviceId, w.Config.AgreementBot.ExchangeURL, w.agbotId, w.token); err != nil {
			glog.Errorf(logString(fmt.Sprintf("error obtaining device %v heartbeat state: %v", partnerWLU.DeviceId, err)))
		} else if len(dev.LastHeartbeat) != 0 && (uint64(timeInSeconds(dev.LastHeartbeat) + 300) > uint64(time.Now().Unix())) {
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
			workload := pol.NextHighestPriorityWorkload(0,0,0)
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

func timeInSeconds(timestamp string) int64 {
	timeFormat := "2006-01-02T15:04:05.999Z[MST]"  // exchange time format

	if t, err := time.Parse(timeFormat, timestamp); err != nil {
		glog.Errorf(logString(fmt.Sprintf("error converting heartbeat time %v into seconds, error: %v", timestamp, err)))
		return 0
	} else {
		return t.Unix()
	}
}



func (w *AgreementBotWorker) TerminateAgreement(ag *Agreement, reason uint) {
	// Start timing out the agreement
	glog.V(3).Infof(logString(fmt.Sprintf("detected agreement %v needs to terminate.", ag.CurrentAgreementId)))

	// Update the database
	if _, err := AgreementTimedout(w.db, ag.CurrentAgreementId, ag.AgreementProtocol); err != nil {
		glog.Errorf(logString(fmt.Sprintf("error marking agreement %v terminate: %v", ag.CurrentAgreementId, err)))
	}

	// Queue up a command for an agreement worker to do the blockchain work
	w.consumerPH[ag.AgreementProtocol].HandleAgreementTimeout(NewAgreementTimeoutCommand(ag.CurrentAgreementId, ag.AgreementProtocol, reason), w.consumerPH[ag.AgreementProtocol])
}

func getDeviceMessageEndpoint(deviceId string, url string, agbotId string, token string) (string, []byte, error) {

	glog.V(5).Infof(logString(fmt.Sprintf("retrieving device %v msg endpoint from exchange", deviceId)))

	if dev, err := getDevice(deviceId, url, agbotId, token); err != nil {
		return "", nil, err
	} else {
		glog.V(5).Infof(logString(fmt.Sprintf("retrieved device %v msg endpoint from exchange %v", deviceId, dev.MsgEndPoint)))
		return dev.MsgEndPoint, dev.PublicKey, nil
	}

}

func getDevice(deviceId string, url string, agbotId string, token string) (*exchange.Device, error) {

	glog.V(5).Infof(logString(fmt.Sprintf("retrieving device %v from exchange", deviceId)))

	var resp interface{}
	resp = new(exchange.GetDevicesResponse)
	targetURL := url + "devices/" + deviceId
	for {
		if err, tpErr := exchange.InvokeExchange(&http.Client{Timeout: time.Duration(config.HTTPDEFAULTTIMEOUT*time.Millisecond)}, "GET", targetURL, agbotId, token, nil, &resp); err != nil {
			glog.Errorf(logString(fmt.Sprintf(err.Error())))
			return nil, err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			devs := resp.(*exchange.GetDevicesResponse).Devices
			if dev, there := devs[deviceId]; !there {
				return nil, errors.New(fmt.Sprintf("device %v not in GET response %v as expected", deviceId, devs))
			} else {
				glog.V(5).Infof(logString(fmt.Sprintf("retrieved device %v from exchange %v", deviceId, dev)))
				return &dev, nil
			}
		}
	}

}

// Govern the archived agreements, periodically deleting them from the database if they are old enough. The
// age limit is defined by the agbot configuration, PurgeArchivedAgreementHours.
//
func (w *AgreementBotWorker) GovernArchivedAgreements() {

	glog.Info(logString(fmt.Sprintf("started archived agreement governance")))

	// Default to purging archived agreements an hour after they are terminated.
	ageLimit := 1
	if w.Config.AgreementBot.PurgeArchivedAgreementHours != 0 {
		ageLimit = w.Config.AgreementBot.PurgeArchivedAgreementHours
	} else {
		glog.Info(logString(fmt.Sprintf("archive purge using default age limit of %v hour.", ageLimit)))
	}

	// This is the amount of time for the governance routine to wait.
	waitTime := uint64(1800)

	for {

		glog.V(5).Infof(logString(fmt.Sprintf("archive purge scanning for agreements archived more than %v hour(s) ago.", ageLimit)))

		// A filter for limiting the returned set of agreements to just those that are too old.
		agedOutFilter := func(now int64, limitH int) AFilter {
			return func(a Agreement) bool {
				return a.AgreementTimedout != 0 && (a.AgreementTimedout + uint64(limitH * 3600) <= uint64(now))
			}
		}

		// Find all archived agreements that are old enough and delete them.
		for _, agp := range policy.AllAgreementProtocols() {
			now := time.Now().Unix()
			if agreements, err := FindAgreements(w.db, []AFilter{ArchivedAFilter(), agedOutFilter(now, ageLimit)}, agp); err == nil {
				for _, ag := range agreements {
					if err := DeleteAgreement(w.db, ag.CurrentAgreementId, agp); err != nil {
						glog.Error(logString(fmt.Sprintf("error deleting archived agreement %v, error: %v", ag.CurrentAgreementId, err)))
					} else {
						glog.V(3).Infof(logString(fmt.Sprintf("archive purge deleted %v", ag.CurrentAgreementId)))
					}
				}

			} else {
				glog.Errorf(logString(fmt.Sprintf("unable to read archived agreements from database for protocol %v, error: %v", agp, err)))
			}
		}

		// Sleep
		glog.V(5).Infof(logString(fmt.Sprintf("archive purge sleeping for %v seconds.", waitTime)))
		time.Sleep(time.Duration(waitTime) * time.Second)
	}

}

// global log record prefix
var logString = func(v interface{}) string {
	return fmt.Sprintf("AgreementBot Governance: %v", v)
}
