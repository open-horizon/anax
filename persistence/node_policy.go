package persistence

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/externalpolicy"
)

// Constants used throughout the code.
const NODE_POLICY = "nodepolicy" // The bucket name inthe bolt DB.

// Retrieve the node policy object from the database. The bolt APIs assume there is more than 1 object in a bucket,
// so this function has to be prepared for that case, even though there should only ever be 1.
func FindNodePolicy(db *bolt.DB) (*externalpolicy.ExternalPolicy, error) {

	policy := make([]externalpolicy.ExternalPolicy, 0)

	readErr := db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(NODE_POLICY)); b != nil {
			return b.ForEach(func(k, v []byte) error {
				var pol externalpolicy.ExternalPolicy

				if err := json.Unmarshal(v, &pol); err != nil {
					return fmt.Errorf("Unable to deserialize node policy record: %v", v)
				}

				policy = append(policy, pol)
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
func SaveNodePolicy(db *bolt.DB, nodePolicy *externalpolicy.ExternalPolicy) error {

	writeErr := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(NODE_POLICY))
		if err != nil {
			return err
		}

		if serial, err := json.Marshal(nodePolicy); err != nil {
			return fmt.Errorf("Failed to serialize node policy: %v. Error: %v", nodePolicy, err)
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
		return fmt.Errorf("could not find record for node policy")
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
