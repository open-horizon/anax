package agreementbot

import (
    "encoding/json"
    "errors"
    "fmt"
    "github.com/boltdb/bolt"
    "github.com/golang/glog"
    "strconv"
    "time"
)

const WORKLOAD_USAGE = "workload_usage"

type WorkloadUsage struct {
    Id                 uint64   `json:"record_id"`             // unique primary key for records
    DeviceId           string   `json:"device_id"`             // the device id we are working with, immutable after construction
    HAPartners         []string `json:"ha_partners"`           // list of deviceis which are partners to this device
    PendingUpgradeTime uint64   `json:"pending_upgrade_time"`  // time when this usage was marked for pending upgrade
    Policy             string   `json:"policy"`                // the policy containing the workloads we're managing
    PolicyName         string   `json:"policy_name"`           // the name of the policy containing the workloads we're managing
    Priority           int      `json:"priority"`              // the workload priority that we're working with 
    RetryCount         int      `json:"retry_count"`           // The number of retries attempted so far
    RetryDurationS     int      `json:"retry_durations"`       // The number of seconds in which the specified number of retries must occur in order for the next priority workload to be attempted.
    CurrentAgreementId string   `json:"current_agreement_id"`  // the agreement id currently in use
    FirstTryTime       uint64   `json:"first_try_time"`        // time when first agrement attempt was made, used to count retries per time
    LatestRetryTime    uint64   `json:"latest_retry_time"`     // time when the newest retry has occurred
    DisableRetry       bool     `json:"disable_retry"`         // when true, retry and retry durations are disbled which effectively disables workload rollback
    VerifiedDurationS  int      `json:"verified_durations"`    // the number of seconds for successful data verification before disabling workload rollback retries
}

func (w WorkloadUsage) String() string {
    return fmt.Sprintf("Id: %v, " +
        "DeviceId: %v, " +
        "HA Partners: %v, " +
        "Pending Upgrade Time: %v, " +
        "PolicyName: %v, " +
        "Priority: %v, " +
        "RetryCount: %v, " +
        "RetryDurationS: %v, " +
        "CurrentAgreementId: %v, " +
        "FirstTryTime: %v, " +
        "LatestRetryTime: %v, " +
        "DisableRetry: %v, " +
        "VerifiedDurationS: %v, " +
        "Policy: %v",
        w.Id, w.DeviceId, w.HAPartners, w.PendingUpgradeTime, w.PolicyName, w.Priority, w.RetryCount,
        w.RetryDurationS, w.CurrentAgreementId, w.FirstTryTime, w.LatestRetryTime, w.DisableRetry, w.VerifiedDurationS, w.Policy)
}

// private factory method for workloadusage w/out persistence safety:
func workloadUsage(deviceid string, hapartners []string, policy string, policyName string, priority int, retryDurationS int, verifiedDurationS int, agid string) (*WorkloadUsage, error) {

    if deviceid == "" || policy == "" || policyName == "" || priority == 0 || retryDurationS == 0 || agid == "" {
        return nil, errors.New("Illegal input: one of deviceid, policy, policyName, priority, retryDurationS, retryLimit or agreement id is empty")
    } else {
        return &WorkloadUsage{
            DeviceId:           deviceid,
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
        }, nil
    }
}

func NewWorkloadUsage(db *bolt.DB, deviceid string, hapartners []string, policy string, policyName string, priority int, retryDurationS int, verifiedDurationS int, agid string) error {
    if wlUsage, err := workloadUsage(deviceid, hapartners, policy, policyName, priority, retryDurationS, verifiedDurationS, agid); err != nil {
        return err
    } else if existing, err := FindSingleWorkloadUsageByDeviceAndPolicyName(db, deviceid, policyName); err != nil {
        return err
    } else if existing != nil {
        return fmt.Errorf("Workload usage record with device id %v and policy name %v already exists.", deviceid, policyName)
    } else if err := WUPersistNew(db, wuBucketName(), wlUsage); err != nil {
        return err
    } else {
        return nil
    }
}

func UpdateRetryCount(db *bolt.DB, deviceid string, policyName string, retryCount int, agid string) (*WorkloadUsage, error) {
    if wlUsage, err := singleWorkloadUsageUpdate(db, deviceid, policyName, func(w WorkloadUsage) *WorkloadUsage {
        w.CurrentAgreementId = agid
        w.RetryCount = retryCount
        // Reset the retry interval time. There is a big assumption here, which is that the caller has already made sure
        // that it's not time to switch the workload usage to a different priority, and therefore the reason for updating
        // the retry count is because the caller thinks they want to stay with the current workload. Since we know it's ok
        // to stay with the current workload priority, then we can safely start a new retry interval. It's important to have
        // an accurate current workload interval in case the workload starts misbehaving.
        now := uint64(time.Now().Unix())
        w.LatestRetryTime = now
        if w.FirstTryTime + uint64(w.RetryDurationS) < now {
            w.FirstTryTime = uint64(time.Now().Unix())
            w.RetryCount = 1       // We used one retry simply because we are here updating retry counts.
        }
        return &w
    }); err != nil {
        return nil, err
    } else {
        return wlUsage, nil
    }
}

func UpdatePriority(db *bolt.DB, deviceid string, policyName string, priority int, retryDurationS int, verifiedDurationS int, agid string) (*WorkloadUsage, error) {
    if wlUsage, err := singleWorkloadUsageUpdate(db, deviceid, policyName, func(w WorkloadUsage) *WorkloadUsage {
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

func UpdatePendingUpgrade(db *bolt.DB, deviceid string, policyName string) (*WorkloadUsage, error) {
    if wlUsage, err := singleWorkloadUsageUpdate(db, deviceid, policyName, func(w WorkloadUsage) *WorkloadUsage {
        w.PendingUpgradeTime = uint64(time.Now().Unix())
        return &w
    }); err != nil {
        return nil, err
    } else {
        return wlUsage, nil
    }
}

func UpdateWUAgreementId(db *bolt.DB, deviceid string, policyName string, agid string) (*WorkloadUsage, error) {
    if wlUsage, err := singleWorkloadUsageUpdate(db, deviceid, policyName, func(w WorkloadUsage) *WorkloadUsage {
        w.CurrentAgreementId = agid
        return &w
    }); err != nil {
        return nil, err
    } else {
        return wlUsage, nil
    }
}

func DisableRollbackChecking(db *bolt.DB, deviceid string, policyName string) (*WorkloadUsage, error) {
    if wlUsage, err := singleWorkloadUsageUpdate(db, deviceid, policyName, func(w WorkloadUsage) *WorkloadUsage {
        w.DisableRetry = true
        w.RetryCount = 0
        return &w
    }); err != nil {
        return nil, err
    } else {
        return wlUsage, nil
    }
}

func FindSingleWorkloadUsageByDeviceAndPolicyName(db *bolt.DB, deviceid string, policyName string) (*WorkloadUsage, error) {
    filters := make([]WUFilter, 0)
    filters = append(filters, DaPWUFilter(deviceid, policyName))

    if wlUsages, err := FindWorkloadUsages(db, filters); err != nil {
        return nil, err
    } else if len(wlUsages) > 1 {
        return nil, fmt.Errorf("Expected only one record for device: %v and policy: %v, but retrieved: %v", deviceid, policyName, wlUsages)
    } else if len(wlUsages) == 0 {
        return nil, nil
    } else {
        return &wlUsages[0], nil
    }
}

func singleWorkloadUsageUpdate(db *bolt.DB, deviceid string, policyName string, fn func(WorkloadUsage) *WorkloadUsage) (*WorkloadUsage, error) {
    if wlUsage, err := FindSingleWorkloadUsageByDeviceAndPolicyName(db, deviceid, policyName); err != nil {
        return nil, err
    } else if wlUsage == nil {
        return nil, fmt.Errorf("Unable to locate workload usage for device: %v, and policy: %v", deviceid, policyName)
    } else {
        updated := fn(*wlUsage)
        return updated, persistUpdatedWorkloadUsage(db, wlUsage.Id, updated)
    }
}

// does whole-member replacements of values that are legal to change during the course of a workload usage
func persistUpdatedWorkloadUsage(db *bolt.DB, id uint64, update *WorkloadUsage) error {
    return db.Update(func(tx *bolt.Tx) error {
        if b, err := tx.CreateBucketIfNotExists([]byte(wuBucketName())); err != nil {
            return err
        } else {
            pKey := strconv.FormatUint(id, 10)
            current := b.Get([]byte(pKey))
            var mod WorkloadUsage

            if current == nil {
                return fmt.Errorf("No workload usage with id %v available to update", pKey)
            } else if err := json.Unmarshal(current, &mod); err != nil {
                return fmt.Errorf("Failed to unmarshal workload usage DB data: %v", string(current))
            } else {

                // write updates only to the fields we expect should be updateable
                mod.Priority = update.Priority
                mod.RetryCount = update.RetryCount
                mod.RetryDurationS = update.RetryDurationS
                mod.CurrentAgreementId = update.CurrentAgreementId
                mod.FirstTryTime = update.FirstTryTime
                mod.PendingUpgradeTime = update.PendingUpgradeTime
                mod.LatestRetryTime = update.LatestRetryTime
                mod.DisableRetry = update.DisableRetry
                mod.VerifiedDurationS = update.VerifiedDurationS

                if serialized, err := json.Marshal(mod); err != nil {
                    return fmt.Errorf("Failed to serialize workload usage record: %v", mod)
                } else if err := b.Put([]byte(pKey), serialized); err != nil {
                    return fmt.Errorf("Failed to write workload usage record with key: %v", pKey)
                } else {
                    glog.V(2).Infof("Succeeded updating workload usage record to %v", mod)
                }
            }
        }
        return nil
    })
}

func DeleteWorkloadUsage(db *bolt.DB, deviceid string, policyName string) error {
    if deviceid == "" || policyName == "" {
        return fmt.Errorf("Missing required arg deviceid or policyName")
    } else {

        if wlUsage, err := FindSingleWorkloadUsageByDeviceAndPolicyName(db, deviceid, policyName); err != nil {
            return err
        } else if wlUsage == nil {
            return fmt.Errorf("Unable to locate workload usage for device: %v, and policy: %v", deviceid, policyName)
        } else {

            pk := wlUsage.Id
            return db.Update(func(tx *bolt.Tx) error {
                b := tx.Bucket([]byte(wuBucketName()))
                if b == nil {
                    return fmt.Errorf("Unknown bucket: %v", wuBucketName())
                } else if existing := b.Get([]byte(strconv.FormatUint(pk, 10))); existing == nil {
                    glog.Errorf("Warning: record deletion requested, but record does not exist: %v", pk)
                    return nil // handle already-deleted workload usage as success
                } else {
                    var record WorkloadUsage

                    if err := json.Unmarshal(existing, &record); err != nil {
                        glog.Errorf("Error deserializing workload usage: %v. This is a pre-deletion warning message function so deletion will still proceed", record)
                    }
                }

                glog.V(3).Infof("Deleting workload usage record for %v with policy %v", deviceid, policyName)
                return b.Delete([]byte(strconv.FormatUint(pk, 10)))
            })
        }
    }
}

func DaPWUFilter(deviceid string, policyName string) WUFilter {
    return func(a WorkloadUsage) bool { return a.DeviceId == deviceid && a.PolicyName == policyName }
}

func DWUFilter(deviceid string) WUFilter {
    return func(a WorkloadUsage) bool { return a.DeviceId == deviceid }
}

func PWUFilter(policyName string) WUFilter {
    return func(a WorkloadUsage) bool { return a.PolicyName == policyName }
}

type WUFilter func(WorkloadUsage) bool

func FindWorkloadUsages(db *bolt.DB, filters []WUFilter) ([]WorkloadUsage, error) {
    wlUsages := make([]WorkloadUsage, 0)

    readErr := db.View(func(tx *bolt.Tx) error {

        if b := tx.Bucket([]byte(wuBucketName())); b != nil {
            b.ForEach(func(k, v []byte) error {

                var a WorkloadUsage

                if err := json.Unmarshal(v, &a); err != nil {
                    glog.Errorf("Unable to deserialize db record: %v", v)
                } else {
                    glog.V(5).Infof("Demarshalled workload usage in DB: %v", a)
                    exclude := false
                    for _, filterFn := range filters {
                        if !filterFn(a) {
                            exclude = true
                        }
                    }
                    if !exclude {
                        wlUsages = append(wlUsages, a)
                    }
                }
                return nil
            })
        }

        return nil // end the transaction
    })

    if readErr != nil {
        return nil, readErr
    } else {
        return wlUsages, nil
    }
}

// This function allocates the record's primary key from the DB's internal sequence counter. The record
// being created is updated with this key right before it is written. This function assumes that duplicate
// record checks have already occurred before it is called.
func WUPersistNew(db *bolt.DB, bucket string, record *WorkloadUsage) error {
    if bucket == "" {
        return fmt.Errorf("Missing required arg bucket")
    } else {
        writeErr := db.Update(func(tx *bolt.Tx) error {

            if b, err := tx.CreateBucketIfNotExists([]byte(bucket)); err != nil {
                return err
            } else if nextKey, err := b.NextSequence(); err != nil {
                return fmt.Errorf("Unable to get sequence key for new record %v. Error: %v", record, err)
            } else {
                strKey := strconv.FormatUint(nextKey, 10)
                record.Id = nextKey
                if bytes, err := json.Marshal(record); err != nil {
                    return fmt.Errorf("Unable to serialize record %v. Error: %v", record, err)
                } else if err := b.Put([]byte(strKey), bytes); err != nil {
                    return fmt.Errorf("Unable to write record to bucket %v. Primary key of record: %v", bucket, strKey)
                } else {
                    glog.V(2).Infof("Succeeded writing workload usage record identified by key %v, record %v in %v", strKey, *record, bucket)
                    return nil
                }
            }
        })

        return writeErr
    }
}

func wuBucketName() string {
    return WORKLOAD_USAGE
}
