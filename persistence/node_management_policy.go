package persistence

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/exchangecommon"
)

const NODE_MANAGEMENT_POLICY = "nodemanagementpolicy"

func SaveOrUpdateNodeManagementPolicy(db *bolt.DB, policyKey string, policy exchangecommon.ExchangeNodeManagementPolicy) error {
	writeErr := db.Update(func(tx *bolt.Tx) error {
		if bucket, err := tx.CreateBucketIfNotExists([]byte(NODE_MANAGEMENT_POLICY)); err != nil {
			return err
		} else if serial, err := json.Marshal(policy); err != nil {
			return fmt.Errorf("Failed to serialize node management policy: Error: %v", err)
		} else {
			return bucket.Put([]byte(policyKey), serial)
		}
	})

	return writeErr
}

func FindNodeManagementPolicy(db *bolt.DB, policyKey string) (*exchangecommon.ExchangeNodeManagementPolicy, error) {
	var nmpRecord *exchangecommon.ExchangeNodeManagementPolicy
	readErr := db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(NODE_MANAGEMENT_POLICY)); b != nil {
			nmpRecSerial := b.Get([]byte(policyKey))
			if nmpRecSerial != nil {
				nmpRecUnmarsh := exchangecommon.ExchangeNodeManagementPolicy{}
				if err := json.Unmarshal(nmpRecSerial, &nmpRecUnmarsh); err != nil {
					return fmt.Errorf("Error unmarshaling node management policy record: %v", err)
				} else {
					nmpRecord = &nmpRecUnmarsh
				}
			}
		}
		return nil
	})

	if readErr != nil {
		return nil, readErr
	}
	return nmpRecord, nil
}

func DeleteNodeManagementPolicy(db *bolt.DB, policyKey string) (*exchangecommon.ExchangeNodeManagementPolicy, error) {
	nmpRecord, err := FindNodeManagementPolicy(db, policyKey)
	if err != nil {
		return nil, err
	} else if nmpRecord == nil {
		return nil, nil
	}
	return nmpRecord, db.Update(func(tx *bolt.Tx) error {
		if b, err := tx.CreateBucketIfNotExists([]byte(NODE_MANAGEMENT_POLICY)); err != nil {
			return err
		} else if err := b.Delete([]byte(policyKey)); err != nil {
			return fmt.Errorf("Error deleting node management policy %v: %v", policyKey, err)
		}
		return nil
	})
}

func DeleteAllNodeManagementPolicies(db *bolt.DB) error {
	return db.Update(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(NODE_MANAGEMENT_POLICY)); b != nil {
			return tx.DeleteBucket([]byte(NODE_MANAGEMENT_POLICY))
		}
		return nil
	})
}
