package persistence

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/exchangecommon"
)

const NODE_MANAGEMENT_STATUS = "nodemanagementstatus"

func SaveOrUpdateNMPStatus(db *bolt.DB, nmpKey string, status exchangecommon.NodeManagementPolicyStatus) error {
	writeErr := db.Update(func(tx *bolt.Tx) error {
		if bucket, err := tx.CreateBucketIfNotExists([]byte(NODE_MANAGEMENT_STATUS)); err != nil {
			return err
		} else if serial, err := json.Marshal(status); err != nil {
			return fmt.Errorf("Failed to serialize node management status: Error: %v", err)
		} else {
			return bucket.Put([]byte(nmpKey), serial)
		}
	})

	return writeErr
}

func DeleteNMPStatus(db *bolt.DB, nmpKey string) (*exchangecommon.NodeManagementPolicyStatus, error) {
	if pol, err := FindNMPStatus(db, nmpKey); err != nil {
		return nil, err
	} else if pol != nil {
		return pol, db.Update(func(tx *bolt.Tx) error {
			if b, err := tx.CreateBucketIfNotExists([]byte(NODE_MANAGEMENT_STATUS)); err != nil {
				return err
			} else if err = b.Delete([]byte(nmpKey)); err != nil {
				return fmt.Errorf("Failed to delete node management policy status %v from the database. Error was: %v", nmpKey, err)
			}
			return nil
		})
	} else {
		return nil, nil
	}
}

func FindNMPStatus(db *bolt.DB, nmpKey string) (*exchangecommon.NodeManagementPolicyStatus, error) {
	var nmStatusRecord *exchangecommon.NodeManagementPolicyStatus
	readErr := db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(NODE_MANAGEMENT_STATUS)); b != nil {
			nmsSerialRec := b.Get([]byte(nmpKey))
			if nmsSerialRec != nil {
				nmsUnmarsh := exchangecommon.NodeManagementPolicyStatus{}
				if err := json.Unmarshal(nmsSerialRec, &nmsUnmarsh); err != nil {
					return fmt.Errorf("Error unmarshaling node management status: %v", err)
				} else {
					nmStatusRecord = &nmsUnmarsh
				}
			}
		}
		return nil
	})

	if readErr != nil {
		return nil, readErr
	}

	return nmStatusRecord, nil
}

func FindAllNMPStatus(db *bolt.DB) (map[string]*exchangecommon.NodeManagementPolicyStatus, error) {
	statuses := make(map[string]*exchangecommon.NodeManagementPolicyStatus, 0)

	readErr := db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(NODE_MANAGEMENT_STATUS)); b != nil {
			b.ForEach(func(k, v []byte) error {
				var s exchangecommon.NodeManagementPolicyStatus

				if err := json.Unmarshal(v, &s); err != nil {
					return fmt.Errorf("Unable to demarshal node management status record: %v", err)
				} else {
					statuses[string(k)] = &s
				}
				return nil
			})
		}
		return nil
	})

	if readErr != nil {
		return nil, readErr
	}
	return statuses, nil
}

type NMStatusFilter func(exchangecommon.NodeManagementPolicyStatus) bool

func StatusNMSFilter(status string) NMStatusFilter {
	return func(e exchangecommon.NodeManagementPolicyStatus) bool { return e.Status() == status }
}

func FindNMPStatusWithFilters(db *bolt.DB, filters []NMStatusFilter) (map[string]*exchangecommon.NodeManagementPolicyStatus, error) {
	statuses := make(map[string]*exchangecommon.NodeManagementPolicyStatus, 0)

	readErr := db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(NODE_MANAGEMENT_STATUS)); b != nil {
			b.ForEach(func(k, v []byte) error {
				var s exchangecommon.NodeManagementPolicyStatus

				if err := json.Unmarshal(v, &s); err != nil {
					return fmt.Errorf("Unable to demarshal node management status record: %v", err)
				} else {
					include := true
					for _, filter := range filters {
						if !filter(s) {
							include = false
							break
						}
					}
					if include {
						statuses[string(k)] = &s
					}
				}
				return nil
			})
		}
		return nil
	})

	if readErr != nil {
		return nil, readErr
	}
	return statuses, nil
}

func FindWaitingNMPStatuses(db *bolt.DB) (map[string]*exchangecommon.NodeManagementPolicyStatus, error) {
	return FindNMPStatusWithFilters(db, []NMStatusFilter{StatusNMSFilter(exchangecommon.STATUS_NEW)})
}
