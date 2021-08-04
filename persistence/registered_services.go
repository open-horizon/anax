package persistence

import (
	"fmt"
	"github.com/boltdb/bolt"
)

const EXCHANGE_NODE_REGISTERED_SERVICES_HASH = "exchange_node_registered_services_hash" // The bucket for the exchange node registeredServices hash

// Retrieve the exchange node registeredServices hash from the database.
func GetNodeRegisteredServicesHash_Exch(db *bolt.DB) ([]byte, error) {

	regServHash := []byte{}

	readErr := db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(EXCHANGE_NODE_REGISTERED_SERVICES_HASH)); b != nil {
			return b.ForEach(func(k, v []byte) error {
				regServHash = v
				return nil
			})
		}

		return nil // end transaction
	})

	if readErr != nil {
		return nil, readErr
	}

	return regServHash, nil
}

// save the exchange node RegisteredServices hash.
func SaveNodeRegisteredServicesHash_Exch(db *bolt.DB, regServHash []byte) error {

	writeErr := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(EXCHANGE_NODE_REGISTERED_SERVICES_HASH))
		if err != nil {
			return err
		}

		return b.Put([]byte(EXCHANGE_NODE_REGISTERED_SERVICES_HASH), regServHash)

	})

	return writeErr
}

// Remove the exchange node RegisteredServices hash from the local database.
func DeleteNodeRegisteredServicesHash_Exch(db *bolt.DB) error {

	if regServHash, err := GetNodeRegisteredServicesHash_Exch(db); err != nil {
		return err
	} else if regServHash == nil || len(regServHash) == 0 {
		return nil
	} else {
		return db.Update(func(tx *bolt.Tx) error {

			if b, err := tx.CreateBucketIfNotExists([]byte(EXCHANGE_NODE_REGISTERED_SERVICES_HASH)); err != nil {
				return err
			} else if err := b.Delete([]byte(EXCHANGE_NODE_REGISTERED_SERVICES_HASH)); err != nil {
				return fmt.Errorf("Unable to delete exchange node RegisteredServices hash from local db: %v", err)
			} else {
				return nil
			}
		})
	}
}
