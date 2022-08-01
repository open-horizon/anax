package persistence

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang/glog"
)

type WorkloadUsage struct {
	Id                 uint64   `json:"record_id"`            // unique primary key for records
	DeviceId           string   `json:"device_id"`            // the device id we are working with, immutable after construction
	HAGroupName        string   `json:"ha_group_name"`        // the ha group name that this device belongs to
	HAPartners         []string `json:"ha_partners"`          // list of device id(s) which are partners to this device
	PendingUpgradeTime uint64   `json:"pending_upgrade_time"` // time when this usage was marked for pending upgrade
	Policy             string   `json:"policy"`               // the policy containing the workloads we're managing
	PolicyName         string   `json:"policy_name"`          // the name of the policy containing the workloads we're managing
	Priority           int      `json:"priority"`             // the workload priority that we're working with
	RetryCount         int      `json:"retry_count"`          // The number of retries attempted so far
	RetryDurationS     int      `json:"retry_durations"`      // The number of seconds in which the specified number of retries must occur in order for the next priority workload to be attempted.
	CurrentAgreementId string   `json:"current_agreement_id"` // the agreement id currently in use
	FirstTryTime       uint64   `json:"first_try_time"`       // time when first agrement attempt was made, used to count retries per time
	LatestRetryTime    uint64   `json:"latest_retry_time"`    // time when the newest retry has occurred
	DisableRetry       bool     `json:"disable_retry"`        // when true, retry and retry durations are disbled which effectively disables workload rollback
	VerifiedDurationS  int      `json:"verified_durations"`   // the number of seconds for successful data verification before disabling workload rollback retries
	ReqsNotMet         bool     `json:"requirements_not_met"` // this workload usage record is not at the highest priority because the device did not meet the API spec requirements at one of the higher priorities
}

func (w WorkloadUsage) String() string {
	return fmt.Sprintf("Id: %v, "+
		"DeviceId: %v, "+
		"HAGroupName: %v"+
		"HA Partners: %v, "+
		"Pending Upgrade Time: %v, "+
		"PolicyName: %v, "+
		"Priority: %v, "+
		"RetryCount: %v, "+
		"RetryDurationS: %v, "+
		"CurrentAgreementId: %v, "+
		"FirstTryTime: %v, "+
		"LatestRetryTime: %v, "+
		"DisableRetry: %v, "+
		"VerifiedDurationS: %v, "+
		"ReqsNotMet: %v, "+
		"Policy: %v",
		w.Id, w.DeviceId, w.HAGroupName, w.HAPartners, w.PendingUpgradeTime, w.PolicyName, w.Priority, w.RetryCount,
		w.RetryDurationS, w.CurrentAgreementId, w.FirstTryTime, w.LatestRetryTime, w.DisableRetry, w.VerifiedDurationS, w.ReqsNotMet, w.Policy)
}

func (w WorkloadUsage) ShortString() string {
	return fmt.Sprintf("Id: %v, "+
		"DeviceId: %v, "+
		"HAGroupName: %v"+
		"HA Partners: %v, "+
		"Pending Upgrade Time: %v, "+
		"PolicyName: %v, "+
		"Priority: %v, "+
		"RetryCount: %v, "+
		"RetryDurationS: %v, "+
		"CurrentAgreementId: %v, "+
		"FirstTryTime: %v, "+
		"LatestRetryTime: %v, "+
		"DisableRetry: %v, "+
		"VerifiedDurationS: %v, "+
		"ReqsNotMet: %v",
		w.Id, w.DeviceId, w.HAGroupName, w.HAPartners, w.PendingUpgradeTime, w.PolicyName, w.Priority, w.RetryCount,
		w.RetryDurationS, w.CurrentAgreementId, w.FirstTryTime, w.LatestRetryTime, w.DisableRetry, w.VerifiedDurationS, w.ReqsNotMet)
}

// private factory method for workloadusage w/out persistence safety:
func NewWorkloadUsage(deviceId string, haGroupName string, hapartners []string, policy string, policyName string, priority int, retryDurationS int, verifiedDurationS int, reqsNotMet bool, agid string) (*WorkloadUsage, error) {

	if deviceId == "" || policyName == "" || priority == 0 || retryDurationS == 0 || agid == "" {
		return nil, errors.New("Illegal input: one of deviceId, policy, policyName, priority, retryDurationS, retryLimit or agreement id is empty")
	} else {
		return &WorkloadUsage{
			DeviceId:           deviceId,
			HAGroupName:        haGroupName,
			HAPartners:         hapartners,
			PendingUpgradeTime: 0,
			Policy:             policy,
			PolicyName:         policyName,
			Priority:           priority,
			RetryCount:         0,
			RetryDurationS:     retryDurationS,
			CurrentAgreementId: agid,
			FirstTryTime:       uint64(time.Now().Unix()),
			LatestRetryTime:    0,
			DisableRetry:       false,
			VerifiedDurationS:  verifiedDurationS,
			ReqsNotMet:         reqsNotMet,
		}, nil
	}
}

func UpdateRetryCount(db AgbotDatabase, deviceid string, policyName string, retryCount int, agid string) (*WorkloadUsage, error) {
	if wlUsage, err := db.SingleWorkloadUsageUpdate(deviceid, policyName, func(w WorkloadUsage) *WorkloadUsage {
		w.CurrentAgreementId = agid
		w.RetryCount = retryCount
		// Reset the retry interval time. There is a big assumption here, which is that the caller has already made sure
		// that it's not time to switch the workload usage to a different priority, and therefore the reason for updating
		// the retry count is because the caller thinks they want to stay with the current workload. Since we know it's ok
		// to stay with the current workload priority, then we can safely start a new retry interval. It's important to have
		// an accurate current workload interval in case the workload starts misbehaving.
		now := uint64(time.Now().Unix())
		w.LatestRetryTime = now
		if w.FirstTryTime+uint64(w.RetryDurationS) < now {
			w.FirstTryTime = uint64(time.Now().Unix())
			w.RetryCount = 1 // We used one retry simply because we are here updating retry counts.
		}
		return &w
	}); err != nil {
		return nil, err
	} else {
		return wlUsage, nil
	}
}

func UpdatePriority(db AgbotDatabase, deviceid string, policyName string, priority int, retryDurationS int, verifiedDurationS int, agid string) (*WorkloadUsage, error) {
	if wlUsage, err := db.SingleWorkloadUsageUpdate(deviceid, policyName, func(w WorkloadUsage) *WorkloadUsage {
		w.CurrentAgreementId = agid
		w.Priority = priority
		w.RetryCount = 0
		w.RetryDurationS = retryDurationS
		w.VerifiedDurationS = verifiedDurationS
		w.FirstTryTime = uint64(time.Now().Unix())
		return &w
	}); err != nil {
		return nil, err
	} else {
		return wlUsage, nil
	}
}

func UpdatePendingUpgrade(db AgbotDatabase, deviceid string, policyName string) (*WorkloadUsage, error) {
	if wlUsage, err := db.SingleWorkloadUsageUpdate(deviceid, policyName, func(w WorkloadUsage) *WorkloadUsage {
		w.PendingUpgradeTime = uint64(time.Now().Unix())
		return &w
	}); err != nil {
		return nil, err
	} else {
		return wlUsage, nil
	}
}

func UpdateWUAgreementId(db AgbotDatabase, deviceid string, policyName string, agid string) (*WorkloadUsage, error) {
	if wlUsage, err := db.SingleWorkloadUsageUpdate(deviceid, policyName, func(w WorkloadUsage) *WorkloadUsage {
		w.CurrentAgreementId = agid
		return &w
	}); err != nil {
		return nil, err
	} else {
		return wlUsage, nil
	}
}

func DisableRollbackChecking(db AgbotDatabase, deviceid string, policyName string) (*WorkloadUsage, error) {
	if wlUsage, err := db.SingleWorkloadUsageUpdate(deviceid, policyName, func(w WorkloadUsage) *WorkloadUsage {
		w.DisableRetry = true
		w.RetryCount = 0
		return &w
	}); err != nil {
		return nil, err
	} else {
		return wlUsage, nil
	}
}

func UpdatePolicy(db AgbotDatabase, deviceid string, policyName string, pol string) (*WorkloadUsage, error) {
	if wlUsage, err := db.SingleWorkloadUsageUpdate(deviceid, policyName, func(w WorkloadUsage) *WorkloadUsage {
		w.Policy = pol
		return &w
	}); err != nil {
		return nil, err
	} else {
		return wlUsage, nil
	}
}

func UpdateHAGroupNameAndPartners(db AgbotDatabase, deviceid string, policyName string, haGroupName string, haPartners []string) (*WorkloadUsage, error) {
	if wlUsage, err := db.SingleWorkloadUsageUpdate(deviceid, policyName, func(w WorkloadUsage) *WorkloadUsage {
		w.HAGroupName = haGroupName
		w.HAPartners = haPartners
		glog.V(2).Infof("Lily - Set w.HAPartnerners (%v) to haPartners: %v", w.HAPartners, haPartners)
		return &w
	}); err != nil {
		return nil, err
	} else {
		return wlUsage, nil
	}
}

func UpdateHAPartners(db AgbotDatabase, deviceid string, policyName string, haPartners []string) (*WorkloadUsage, error) {
	if wlUsage, err := db.SingleWorkloadUsageUpdate(deviceid, policyName, func(w WorkloadUsage) *WorkloadUsage {
		w.HAPartners = haPartners
		return &w
	}); err != nil {
		return nil, err
	} else {
		return wlUsage, nil
	}
}

// This code is running in a database transaction. Within the tx, the current record is
// read and then updated according to the updates within the input update record. It is critical
// to check for correct data transitions within the tx .
func ValidateWUStateTransition(mod *WorkloadUsage, update *WorkloadUsage) {
	// write updates only to the fields we expect should be updateable
	mod.Priority = update.Priority
	mod.RetryCount = update.RetryCount
	mod.RetryDurationS = update.RetryDurationS
	mod.HAGroupName = update.HAGroupName
	mod.HAPartners = update.HAPartners

	// This field goes from empty to non-empty to empty, ad infinitum
	if (mod.CurrentAgreementId == "" && update.CurrentAgreementId != "") || (mod.CurrentAgreementId != "" && update.CurrentAgreementId == "") {
		mod.CurrentAgreementId = update.CurrentAgreementId
	}
	if mod.FirstTryTime < update.FirstTryTime { // Always moves forward
		mod.FirstTryTime = update.FirstTryTime
	}
	if mod.PendingUpgradeTime == 0 { // 1 transition from zero to non-zero
		mod.PendingUpgradeTime = update.PendingUpgradeTime
	}
	if mod.LatestRetryTime < update.LatestRetryTime { // Always moves forward
		mod.LatestRetryTime = update.LatestRetryTime
	}
	if !mod.DisableRetry { // 1 transition from false to true
		mod.DisableRetry = update.DisableRetry
	}
	if !mod.ReqsNotMet { // 1 transition from false to true
		mod.ReqsNotMet = update.ReqsNotMet
	}
	if mod.Policy == "" { // 1 transition from empty to set
		mod.Policy = update.Policy
	}
	mod.VerifiedDurationS = update.VerifiedDurationS
}

// Filters
func DaPWUFilter(deviceid string, policyName string) WUFilter {
	return func(a WorkloadUsage) bool { return a.DeviceId == deviceid && a.PolicyName == policyName }
}

func DWUFilter(deviceid string) WUFilter {
	return func(a WorkloadUsage) bool { return a.DeviceId == deviceid }
}

func PWUFilter(policyName string) WUFilter {
	return func(a WorkloadUsage) bool { return a.PolicyName == policyName }
}

func AllWUFilter() WUFilter {
	return func(A WorkloadUsage) bool { return true }
}

type WUFilter func(WorkloadUsage) bool
