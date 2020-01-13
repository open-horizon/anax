package persistence

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/cutil"
	"time"
)

// Constants used throughout the code.
const EXCHANGE_CHANGES = "exchange-change-state" // The bucket name in the bolt DB.

type ChangeState struct {
	ChangeID    uint64 `json:"changeId"`
	LastUpdated int64  `json:"lastUpdated"`
}

func (c ChangeState) String() string {
	lu := time.Unix(c.LastUpdated, 0).Format(cutil.ExchangeTimeFormat)
	return fmt.Sprintf("Change State ID: %v, last updated: %v", c.ChangeID, lu)
}

// Retrieve the change state object from the database. The bolt APIs assume there is more than 1 object in a bucket,
// so this function has to be prepared for that case, even though there should only ever be 1.
func FindExchangeChangeState(db *bolt.DB) (*ChangeState, error) {

	chg := make([]ChangeState, 0)

	readErr := db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(EXCHANGE_CHANGES)); b != nil {
			return b.ForEach(func(k, v []byte) error {
				var c ChangeState

				if err := json.Unmarshal(v, &c); err != nil {
					return fmt.Errorf("Unable to deserialize exchange change state %v, error: %v", v, err)
				}

				chg = append(chg, c)
				return nil
			})
		}

		return nil // end transaction
	})

	if readErr != nil {
		return nil, readErr
	}

	glog.V(5).Infof("Demarshalled saved exchange change state: %v", chg)

	if len(chg) > 1 {
		return nil, fmt.Errorf("Unsupported db state: more than one change state stored in bucket: %v", chg)
	} else if len(chg) == 1 {
		return &chg[0], nil
	} else {
		return nil, nil
	}
}

// There is only 1 object in the bucket so we can use the bucket name as the object key.
func SaveExchangeChangeState(db *bolt.DB, changeID uint64) error {

	writeErr := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(EXCHANGE_CHANGES))
		if err != nil {
			return err
		}

		chg := ChangeState{
			ChangeID:    changeID,
			LastUpdated: time.Now().Unix(),
		}

		if serial, err := json.Marshal(chg); err != nil {
			return fmt.Errorf("Failed to serialize change state %v, error: %v", chg, err)
		} else if err := b.Put([]byte(EXCHANGE_CHANGES), serial); err != nil {
			return fmt.Errorf("Failed to save change state %v, error: %v", chg, err)
		} else {
			glog.V(3).Infof("Successfully saved exchange change state: %v", chg)
			return nil
		}
	})

	return writeErr
}

// Remove the change state object from the local database.
func DeleteExchangeChangeState(db *bolt.DB) error {

	if chg, err := FindExchangeChangeState(db); err != nil {
		return err
	} else if chg == nil {
		return nil
	} else {

		return db.Update(func(tx *bolt.Tx) error {

			if b, err := tx.CreateBucketIfNotExists([]byte(EXCHANGE_CHANGES)); err != nil {
				return err
			} else if err := b.Delete([]byte(EXCHANGE_CHANGES)); err != nil {
				return fmt.Errorf("Unable to delete exchange change state, error: %v", err)
			} else {
				return nil
			}
		})
	}
}
