package bolt

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/agreementbot/persistence"
)

const WORKLOAD_USAGE = "workload_usage"

func (db *AgbotBoltDB) NewWorkloadUsage(deviceId string, haGroupName string, hapartners []string, policy string, policyName string, priority int, retryDurationS int, verifiedDurationS int, reqsNotMet bool, agid string) error {
	if wlUsage, err := persistence.NewWorkloadUsage(deviceId, haGroupName, hapartners, policy, policyName, priority, retryDurationS, verifiedDurationS, reqsNotMet, agid); err != nil {
		return err
	} else if existing, err := db.FindSingleWorkloadUsageByDeviceAndPolicyName(deviceId, policyName); err != nil {
		return err
	} else if existing != nil {
		return fmt.Errorf("Workload usage record for device %v and policy name %v already exists.", deviceId, policyName)
	} else if err := db.WUPersistNew(wuBucketName(), wlUsage); err != nil {
		return err
	} else {
		return nil
	}
}

func (db *AgbotBoltDB) GetWorkloadUsagesCount(partition string) (int64, error) {
	if wus, err := db.FindWorkloadUsages([]persistence.WUFilter{}); err != nil {
		return 0, err
	} else {
		return int64(len(wus)), nil
	}
}

func (db *AgbotBoltDB) FindSingleWorkloadUsageByDeviceAndPolicyName(deviceid string, policyName string) (*persistence.WorkloadUsage, error) {
	filters := make([]persistence.WUFilter, 0)
	filters = append(filters, persistence.DaPWUFilter(deviceid, policyName))

	if wlUsages, err := db.FindWorkloadUsages(filters); err != nil {
		return nil, err
	} else if len(wlUsages) > 1 {
		return nil, fmt.Errorf("Expected only one record for device: %v and policy: %v, but retrieved: %v", deviceid, policyName, wlUsages)
	} else if len(wlUsages) == 0 {
		return nil, nil
	} else {
		return &wlUsages[0], nil
	}
}

func (db *AgbotBoltDB) UpdatePendingUpgrade(deviceid string, policyName string) (*persistence.WorkloadUsage, error) {
	return persistence.UpdatePendingUpgrade(db, deviceid, policyName)
}

func (db *AgbotBoltDB) UpdateRetryCount(deviceid string, policyName string, retryCount int, agid string) (*persistence.WorkloadUsage, error) {
	return persistence.UpdateRetryCount(db, deviceid, policyName, retryCount, agid)
}

func (db *AgbotBoltDB) UpdatePriority(deviceid string, policyName string, priority int, retryDurationS int, verifiedDurationS int, agid string) (*persistence.WorkloadUsage, error) {
	return persistence.UpdatePriority(db, deviceid, policyName, priority, retryDurationS, verifiedDurationS, agid)
}

func (db *AgbotBoltDB) UpdatePolicy(deviceid string, policyName string, pol string) (*persistence.WorkloadUsage, error) {
	return persistence.UpdatePolicy(db, deviceid, policyName, pol)
}

func (db *AgbotBoltDB) UpdateWUAgreementId(deviceid string, policyName string, agid string, protocol string) (*persistence.WorkloadUsage, error) {
	return persistence.UpdateWUAgreementId(db, deviceid, policyName, agid)
}

func (db *AgbotBoltDB) DisableRollbackChecking(deviceid string, policyName string) (*persistence.WorkloadUsage, error) {
	return persistence.DisableRollbackChecking(db, deviceid, policyName)
}

func (db *AgbotBoltDB) UpdateHAGroupNameAndPartners(deviceid string, policyName string, haGroupName string, haPartners []string) (*persistence.WorkloadUsage, error) {
	return persistence.UpdateHAGroupNameAndPartners(db, deviceid, policyName, haGroupName, haPartners)
}

func (db *AgbotBoltDB) UpdateHAPartners(deviceid string, policyName string, haPartners []string) (*persistence.WorkloadUsage, error) {
	return persistence.UpdateHAPartners(db, deviceid, policyName, haPartners)
}

func (db *AgbotBoltDB) SingleWorkloadUsageUpdate(deviceid string, policyName string, fn func(persistence.WorkloadUsage) *persistence.WorkloadUsage) (*persistence.WorkloadUsage, error) {
	if wlUsage, err := db.FindSingleWorkloadUsageByDeviceAndPolicyName(deviceid, policyName); err != nil {
		return nil, err
	} else if wlUsage == nil {
		return nil, fmt.Errorf("Unable to locate workload usage for device: %v, and policy: %v", deviceid, policyName)
	} else {
		updated := fn(*wlUsage)
		return updated, db.persistUpdatedWorkloadUsage(wlUsage.Id, updated)
	}
}

// does whole-member replacements of values that are legal to change during the course of a workload usage
func (db *AgbotBoltDB) persistUpdatedWorkloadUsage(id uint64, update *persistence.WorkloadUsage) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		if b, err := tx.CreateBucketIfNotExists([]byte(wuBucketName())); err != nil {
			return err
		} else {
			pKey := strconv.FormatUint(id, 10)
			current := b.Get([]byte(pKey))
			var mod persistence.WorkloadUsage

			if current == nil {
				return fmt.Errorf("No workload usage with id %v available to update", pKey)
			} else if err := json.Unmarshal(current, &mod); err != nil {
				return fmt.Errorf("Failed to unmarshal workload usage DB data: %v", string(current))
			} else {

				// This code is running in a database transaction. Within the tx, the current record (mod) is
				// read and then updated according to the updates within the input update record. It is critical
				// to check for correct data transitions within the tx.
				persistence.ValidateWUStateTransition(&mod, update)

				if serialized, err := json.Marshal(mod); err != nil {
					return fmt.Errorf("Failed to serialize workload usage record: %v", mod)
				} else if err := b.Put([]byte(pKey), serialized); err != nil {
					return fmt.Errorf("Failed to write workload usage record with key: %v", pKey)
				} else {
					glog.V(2).Infof("Succeeded updating workload usage record to %v", mod.ShortString())
				}
			}
		}
		return nil
	})
}

func (db *AgbotBoltDB) DeleteWorkloadUsage(deviceid string, policyName string) error {
	if deviceid == "" || policyName == "" {
		return fmt.Errorf("Missing required arg deviceid or policyName")
	} else {

		if wlUsage, err := db.FindSingleWorkloadUsageByDeviceAndPolicyName(deviceid, policyName); err != nil {
			return err
		} else if wlUsage == nil {
			return fmt.Errorf("Unable to locate workload usage for device: %v, and policy: %v", deviceid, policyName)
		} else {

			pk := wlUsage.Id
			return db.db.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte(wuBucketName()))
				if b == nil {
					return fmt.Errorf("Unknown bucket: %v", wuBucketName())
				} else if existing := b.Get([]byte(strconv.FormatUint(pk, 10))); existing == nil {
					glog.Errorf("Warning: record deletion requested, but record does not exist: %v", pk)
					return nil // handle already-deleted workload usage as success
				} else {
					var record persistence.WorkloadUsage

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

func (db *AgbotBoltDB) FindWorkloadUsages(filters []persistence.WUFilter) ([]persistence.WorkloadUsage, error) {
	wlUsages := make([]persistence.WorkloadUsage, 0)

	readErr := db.db.View(func(tx *bolt.Tx) error {

		if b := tx.Bucket([]byte(wuBucketName())); b != nil {
			b.ForEach(func(k, v []byte) error {

				var a persistence.WorkloadUsage

				if err := json.Unmarshal(v, &a); err != nil {
					glog.Errorf("Unable to deserialize db record: %v", v)
				} else {
					glog.V(5).Infof("Demarshalled workload usage in DB: %v", a.ShortString())
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
func (db *AgbotBoltDB) WUPersistNew(bucket string, record *persistence.WorkloadUsage) error {
	if bucket == "" {
		return fmt.Errorf("Missing required arg bucket")
	} else {
		writeErr := db.db.Update(func(tx *bolt.Tx) error {

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
