package persistence

import (
	"encoding/json"
	"fmt"
	bolt "go.etcd.io/bbolt"
)

const NODE_STATUS = "node_status"

type WorkloadStatus struct {
	AgreementId    string            `json:"agreementId"`
	ServiceURL     string            `json:"serviceUrl,omitempty"`
	Org            string            `json:"orgid,omitempty"`
	Version        string            `json:"version,omitempty"`
	Arch           string            `json:"arch,omitempty"`
	Containers     []ContainerStatus `json:"containerStatus"`
	OperatorStatus interface{}       `json:"operatorStatus,omitempty"`
	ConfigState    string            `json:"configState,omitempty"`
}

type ContainerStatus struct {
	Name    string `json:"name"`
	Image   string `json:"image"`
	Created int64  `json:"created"`
	State   string `json:"state"`
}

// FindNodeStatus returns the node status currently in the local db
func FindNodeStatus(db *bolt.DB) ([]WorkloadStatus, error) {
	var nodeStatus []WorkloadStatus

	readErr := db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(NODE_STATUS)); b != nil {
			return b.ForEach(func(k, v []byte) error {

				if err := json.Unmarshal(v, &nodeStatus); err != nil {
					return fmt.Errorf("Unable to deserialize node status record: %v", v)
				}

				return nil
			})
		}

		return nil // end transaction
	})

	if readErr != nil {
		return nil, readErr
	}
	return nodeStatus, nil
}

// SaveNodeStatus saves the provided node status to the local db
func SaveNodeStatus(db *bolt.DB, status []WorkloadStatus) error {
	writeErr := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(NODE_STATUS))
		if err != nil {
			return err
		}

		if serial, err := json.Marshal(status); err != nil {
			return fmt.Errorf("Failed to serialize node status: %v. Error: %v", status, err)
		} else {
			return b.Put([]byte(NODE_STATUS), serial)
		}
	})

	return writeErr
}

// DeleteSurfaceErrors delete node status from the local database
func DeleteNodeStatus(db *bolt.DB) error {
	if seList, err := FindNodeStatus(db); err != nil {
		return err
	} else if len(seList) == 0 {
		return nil
	} else {
		return db.Update(func(tx *bolt.Tx) error {

			if b, err := tx.CreateBucketIfNotExists([]byte(NODE_STATUS)); err != nil {
				return err
			} else if err := b.Delete([]byte(NODE_STATUS)); err != nil {
				return fmt.Errorf("Unable to delete node status object: %v", err)
			} else {
				return nil
			}
		})
	}
}
