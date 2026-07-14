package persistence

import (
	"fmt"
	bolt "go.etcd.io/bbolt"
)

// Constants used throughout the code.
const NODE_EXCH_PATTERN = "nodeexchpattern" // The bucket name in the bolt DB.

// Retrieve the node exchange pattern name from the database. It is set when the exchange node pattern is different
// from the local registered node pattern. It will be cleared once the device pattern get changed.
// The bolt APIs assume there is more than 1 object in a bucket,
// so this function has to be prepared for that case, even though there should only ever be 1.
func FindSavedNodeExchPattern(db *bolt.DB) (string, error) {

	pattern_name := ""

	readErr := db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(NODE_EXCH_PATTERN)); b != nil {
			return b.ForEach(func(k, v []byte) error {
				pattern_name = string(v)
				return nil
			})
		}

		return nil // end transaction
	})

	if readErr != nil {
		return "", readErr
	}

	return pattern_name, nil
}

// There is only 1 object in the bucket so we can use the bucket name as the object key.
func SaveNodeExchPattern(db *bolt.DB, nodePatternName string) error {

	writeErr := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(NODE_EXCH_PATTERN))
		if err != nil {
			return err
		}

		return b.Put([]byte(NODE_EXCH_PATTERN), []byte(nodePatternName))

	})

	return writeErr
}

// Remove the node exchange pattern name from the local database.
func DeleteNodeExchPattern(db *bolt.DB) error {

	if pattern_name, err := FindSavedNodeExchPattern(db); err != nil {
		return err
	} else if pattern_name == "" {
		return nil
	} else {

		return db.Update(func(tx *bolt.Tx) error {

			if b, err := tx.CreateBucketIfNotExists([]byte(NODE_EXCH_PATTERN)); err != nil {
				return err
			} else if err := b.Delete([]byte(NODE_EXCH_PATTERN)); err != nil {
				return fmt.Errorf("Unable to delete node pattern from local db: %v", err)
			} else {
				return nil
			}
		})
	}
}
