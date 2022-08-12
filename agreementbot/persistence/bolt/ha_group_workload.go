package bolt

import (
	"encoding/json"
	"fmt"

	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/agreementbot/persistence"
)

const HA_WORKLOAD_USAGE_BUCKET = "ha_workload_usage"

/*
func (db *AgbotBoltDB) InsertOrUpdateHATable(requestingWorkload persistence.UpgradingHAGroupWorkload) (*persistence.UpgradingHAGroupWorkload, error) {
	var updatedDBWorkload persistence.UpgradingHAGroupWorkload
	key := haWLUId(requestingWorkload.OrgId, requestingWorkload.GroupName, requestingWorkload.PolicyName)

	dbErr := db.db.Update(func(tx *bolt.Tx) error {
		if b, err := tx.CreateBucketIfNotExists([]byte(HA_WORKLOAD_USAGE_BUCKET)); err != nil {
			return err
		} else {
			dbWorkloadJson := b.Get([]byte(key))
			var mod persistence.UpgradingHAGroupWorkload

			// there is no workload in this group updating. put the requesting workload into the table
			if dbWorkloadJson == nil {
				if serialized, err := json.Marshal(requestingWorkload); err != nil {
					return err
				} else if err = b.Put([]byte(key), serialized); err != nil {
					return err
				}
				updatedDBWorkload = requestingWorkload
				return nil
			} else {
				mod.GroupName = requestingWorkload.GroupName
				mod.OrgId = requestingWorkload.OrgId
				mod.PolicyName = requestingWorkload.PolicyName
				mod.NodeId = requestingWorkload.NodeId

				if serialized, err := json.Marshal(mod); err != nil {
					return fmt.Errorf("Failed to serialize contract record: %v. Error: %v", mod, err)
				} else if err := b.Put([]byte(key), serialized); err != nil {
					return fmt.Errorf("Failed to write ha upgrading workload with key: %v. Error: %v", key, err)
				} else {
					glog.V(2).Infof("Succeeded updating ha upgrading workload record to %v", mod)
					updatedDBWorkload = mod
					return nil
				}
			}
		}
	})

	return &updatedDBWorkload, dbErr
} */

/*
// if workload not in the table, insert it, otherwise return existing workload from db
func (db *AgbotBoltDB) CheckIfWorkloadPresentAndUpdateHATable(requestingWorkload persistence.UpgradingHAGroupWorkload) (*persistence.UpgradingHAGroupWorkload, error) {
	var updatedDBWorkload persistence.UpgradingHAGroupWorkload
	key := haWLUId(requestingWorkload.OrgId, requestingWorkload.GroupName, requestingWorkload.PolicyName)

	dbErr := db.db.Update(func(tx *bolt.Tx) error {
		if b, err := tx.CreateBucketIfNotExists([]byte(HA_WORKLOAD_USAGE_BUCKET)); err != nil {
			return err
		} else {
			dbWorkloadJson := b.Get([]byte(key))
			var dbWorkload persistence.UpgradingHAGroupWorkload

			// there is no workload in this group updating. put the requesting workload into the table
			if dbWorkloadJson == nil {
				if serialized, err := json.Marshal(requestingWorkload); err != nil {
					return err
				} else if err = b.Put([]byte(key), serialized); err != nil {
					return err
				}
				updatedDBWorkload = requestingWorkload
				return nil
			}

			// otherwise, return workload from db
			if err := json.Unmarshal(dbWorkloadJson, &dbWorkload); err != nil {
				return nil
			}
			updatedDBWorkload = dbWorkload
			return nil
		}
	})

	return &updatedDBWorkload, dbErr
} */

func (db *AgbotBoltDB) DeleteHAUpgradingWorkload(workloadToDelete persistence.UpgradingHAGroupWorkload) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(HA_WORKLOAD_USAGE_BUCKET)); b == nil {
			return fmt.Errorf("Unknown bucket %v", HA_WORKLOAD_USAGE_BUCKET)
		} else {
			//b.Get([]byte(haWLUId(workloadToDelete.OrgId, workloadToDelete.GroupName, workloadToDelete.PolicyName)))
			return b.Delete([]byte(haWLUId(workloadToDelete.OrgId, workloadToDelete.GroupName, workloadToDelete.PolicyName)))
		}
	})
}

func (db *AgbotBoltDB) DeleteHAUpgradingWorkloadsByGroupName(org string, haGroupName string) error {
	if upgradingHAWorkloads, err := db.FindHAUpgradeWorkloadsWithFilters([]persistence.HAWorkloadUpgradeFilter{persistence.HAWorkloadUpgradeGroupFilter(org, haGroupName)}); err != nil {
		return err
	} else if upgradingHAWorkloads != nil {
		for _, upgrupgradingHAWorkload := range upgradingHAWorkloads {
			// delete upgrading ha workload
			if err = db.DeleteHAUpgradingWorkload(upgrupgradingHAWorkload); err != nil {
				return err
			}
		}
	}
	return nil
}

func (db *AgbotBoltDB) DeleteHAUpgradingWorkloadsByGroupNameAndDeviceId(org string, haGroupName string, deviceId string) error {
	if upgradingHAWorkloads, err := db.FindHAUpgradeWorkloadsWithFilters([]persistence.HAWorkloadUpgradeFilter{persistence.HAWorkloadUpgradeGroupAndNodeFilter(org, haGroupName, deviceId)}); err != nil {
		return err
	} else if upgradingHAWorkloads != nil {
		for _, upgrupgradingHAWorkload := range upgradingHAWorkloads {
			// delete upgrading ha workload
			if err = db.DeleteHAUpgradingWorkload(upgrupgradingHAWorkload); err != nil {
				return err
			}
		}
	}
	return nil
}

func (db *AgbotBoltDB) FindHAUpgradeWorkloadsWithFilters(filterSlice []persistence.HAWorkloadUpgradeFilter) ([]persistence.UpgradingHAGroupWorkload, error) {
	upgradingHAWorkloads := make([]persistence.UpgradingHAGroupWorkload, 0)

	readErr := db.db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(HA_WORKLOAD_USAGE_BUCKET)); b != nil {
			b.ForEach(func(k, v []byte) error {
				var haWorkload persistence.UpgradingHAGroupWorkload

				if err := json.Unmarshal(v, &haWorkload); err != nil {
					return err
				} else {
					include := true
					for _, filter := range filterSlice {
						if !filter(haWorkload) {
							include = false
						}
					}

					if include {
						upgradingHAWorkloads = append(upgradingHAWorkloads, haWorkload)
					}
				}
				return nil
			})
		}
		return nil
	})

	if readErr == nil {
		return upgradingHAWorkloads, nil
	}
	return upgradingHAWorkloads, readErr
}

func (db *AgbotBoltDB) ListHAUpgradingWorkloadsByGroupName(org string, haGroupName string) ([]persistence.UpgradingHAGroupWorkload, error) {
	return db.FindHAUpgradeWorkloadsWithFilters([]persistence.HAWorkloadUpgradeFilter{persistence.HAWorkloadUpgradeGroupFilter(org, haGroupName)})
}

func (db *AgbotBoltDB) GetHAUpgradingWorkload(org string, haGroupName string, policyName string) (*persistence.UpgradingHAGroupWorkload, error) {
	key := haWLUId(org, haGroupName, policyName)

	var puw *persistence.UpgradingHAGroupWorkload
	puw = nil

	// fetch ha upgrading workload
	readErr := db.db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(HA_WORKLOAD_USAGE_BUCKET)); b != nil {
			v := b.Get([]byte(key))
			if v == nil {
				return nil
			}

			var uw persistence.UpgradingHAGroupWorkload
			if err := json.Unmarshal(v, &uw); err != nil {
				glog.Errorf("Lily - Unable to deserialize ha upgrading workload db record: %v. Error: %v", v, err)
				return err
			} else {
				puw = &uw
				return nil
			}
		}
		return nil // end the transaction
	})

	if readErr != nil {
		return nil, readErr
	} else {
		return puw, nil
	}
}

func (db *AgbotBoltDB) UpdateHAUpgradingWorkloadForGroupAndPolicy(org string, haGroupName string, policyName string, deviceId string) (bool, error) {
	key := haWLUId(org, haGroupName, policyName)
	dbErr := db.db.Update(func(tx *bolt.Tx) error {
		if b, err := tx.CreateBucketIfNotExists([]byte(HA_WORKLOAD_USAGE_BUCKET)); err != nil {
			return err
		} else {
			current := b.Get([]byte(key))
			var mod persistence.UpgradingHAGroupWorkload
			if current == nil {
				return fmt.Errorf("No ha upgrading workload with key available to update: %v", key)
			} else if err := json.Unmarshal(current, &mod); err != nil {
				return fmt.Errorf("Failed to unmarshal ha upgrading workload DB data: %v. Error: %v", string(current), err)
			} else {
				mod.NodeId = deviceId

				if serialized, err := json.Marshal(mod); err != nil {
					return fmt.Errorf("Failed to serialize ha upgrading workload record: %v. Error: %v", mod, err)
				} else if err := b.Put([]byte(key), serialized); err != nil {
					return fmt.Errorf("Failed to write ha upgrading workload with key: %v. Error: %v", key, err)
				} else {
					glog.V(2).Infof("Succeeded updating ha upgrading workload record to %v", mod)
					return nil
				}
			}
		}
	})

	if dbErr != nil {
		return false, dbErr
	} else {
		return true, nil
	}
}

func (db *AgbotBoltDB) InsertHAUpgradingWorkloadForGroupAndPolicy(org string, haGroupName string, policyName string, deviceId string) error {
	key := haWLUId(org, haGroupName, policyName)
	dbErr := db.db.Update(func(tx *bolt.Tx) error {
		if b, err := tx.CreateBucketIfNotExists([]byte(HA_WORKLOAD_USAGE_BUCKET)); err != nil {
			return err
		} else {
			current := b.Get([]byte(key))
			if current != nil {
				// if already exit, do nothing (be consistent with postgresql/ha_group_workload.go)
				return nil
			} else {
				haUpgradingWorkloadToPersist, err := persistence.NewUpgradingHAGroupWorkload(haGroupName, org, policyName, deviceId)
				if err != nil {
					return err
				}
				if serialized, err := json.Marshal(haUpgradingWorkloadToPersist); err != nil {
					return err
				} else if err = b.Put([]byte(key), serialized); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return dbErr
}

func haWLUId(orgId string, groupName string, policyName string) string {
	return fmt.Sprintf("%s/%s/%s", orgId, groupName, policyName)
}
