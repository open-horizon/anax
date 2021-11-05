package persistence

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/exchangecommon"
)

// Constants used throughout the code.
const NODE_POLICY = "nodepolicy"                                   // The bucket name in the bolt DB.
const EXCHANGE_NP_LAST_UPDATED = "exchange_nodepolicy_lastupdated" // The buucket for the exchange last updated string

// The node policy object in the local db
type PersistenceNodePolicy struct {
	exchangecommon.NodePolicy
	NodePolicyVersion string `json:"node_policy_version,omitempty"` // the version of the node policy
}

func (e PersistenceNodePolicy) String() string {
	return fmt.Sprintf("%v, "+
		"NodePolicyVersion: %v",
		e.NodePolicy, e.NodePolicyVersion)
}

// Retrieve the node policy object from the database. The bolt APIs assume there is more than 1 object in a bucket,
// so this function has to be prepared for that case, even though there should only ever be 1.
func FindNodePolicy(db *bolt.DB) (*exchangecommon.NodePolicy, error) {

	policy := make([]exchangecommon.NodePolicy, 0)

	readErr := db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(NODE_POLICY)); b != nil {
			return b.ForEach(func(k, v []byte) error {
				var pol PersistenceNodePolicy

				if err := json.Unmarshal(v, &pol); err != nil {
					return fmt.Errorf("Unable to deserialize node policy record: %v", v)
				}

				if pol.NodePolicyVersion == "" {
					// convert the version1 node policy to version2 format
					pol_converted := exchangecommon.ConvertNodePolicy_v1Tov2(pol.NodePolicy.ExternalPolicy)
					policy = append(policy, *pol_converted)
				} else if pol.NodePolicyVersion == exchangecommon.NODEPOLICY_VERSION_VERSION_2 {
					// this is version 2, keep
					policy = append(policy, pol.NodePolicy)
				} else {
					return fmt.Errorf("Unsupported node policy version: %v", pol.NodePolicyVersion)
				}

				return nil
			})
		}

		return nil // end transaction
	})

	if readErr != nil {
		return nil, readErr
	}

	if len(policy) > 1 {
		return nil, fmt.Errorf("Unsupported db state: more than one node policy stored in bucket. Policies: %v", policy)
	} else if len(policy) == 1 {
		return &policy[0], nil
	} else {
		return nil, nil
	}
}

// There is only 1 object in the bucket so we can use the bucket name as the object key.
func SaveNodePolicy(db *bolt.DB, nodePolicy *exchangecommon.NodePolicy) error {

	writeErr := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(NODE_POLICY))
		if err != nil {
			return err
		}

		persNostPol := PersistenceNodePolicy{NodePolicy: *nodePolicy, NodePolicyVersion: exchangecommon.NODEPOLICY_VERSION_VERSION_2}
		if serial, err := json.Marshal(&persNostPol); err != nil {
			return fmt.Errorf("Failed to serialize node policy: %v. Error: %v", persNostPol, err)
		} else {
			return b.Put([]byte(NODE_POLICY), serial)
		}
	})

	return writeErr
}

// Remove the node policy object from the local database.
func DeleteNodePolicy(db *bolt.DB) error {

	if pol, err := FindNodePolicy(db); err != nil {
		return err
	} else if pol == nil {
		return nil
	} else {

		return db.Update(func(tx *bolt.Tx) error {

			if b, err := tx.CreateBucketIfNotExists([]byte(NODE_POLICY)); err != nil {
				return err
			} else if err := b.Delete([]byte(NODE_POLICY)); err != nil {
				return fmt.Errorf("Unable to delete node policy object: %v", err)
			} else {
				return nil
			}
		})
	}
}

// Retrieve the exchange node policy lastUpdated string from the database.
func GetNodePolicyLastUpdated_Exch(db *bolt.DB) (string, error) {

	lastUpdated := ""

	readErr := db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(EXCHANGE_NP_LAST_UPDATED)); b != nil {
			return b.ForEach(func(k, v []byte) error {
				lastUpdated = string(v)
				return nil
			})
		}

		return nil // end transaction
	})

	if readErr != nil {
		return "", readErr
	}

	return lastUpdated, nil
}

// save the exchange node policy lastUpdated string.
func SaveNodePolicyLastUpdated_Exch(db *bolt.DB, lastUpdated string) error {

	writeErr := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(EXCHANGE_NP_LAST_UPDATED))
		if err != nil {
			return err
		}

		return b.Put([]byte(EXCHANGE_NP_LAST_UPDATED), []byte(lastUpdated))

	})

	return writeErr
}

// Remove the exchange node policy lastUpdated string from the local database.
func DeleteNodePolicyLastUpdated_Exch(db *bolt.DB) error {

	if lastUpdated, err := GetNodePolicyLastUpdated_Exch(db); err != nil {
		return err
	} else if lastUpdated == "" {
		return nil
	} else {
		return db.Update(func(tx *bolt.Tx) error {

			if b, err := tx.CreateBucketIfNotExists([]byte(EXCHANGE_NP_LAST_UPDATED)); err != nil {
				return err
			} else if err := b.Delete([]byte(EXCHANGE_NP_LAST_UPDATED)); err != nil {
				return fmt.Errorf("Unable to delete exchange node policy last updated string: %v", err)
			} else {
				return nil
			}
		})
	}
}
