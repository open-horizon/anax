package persistence

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
)

const SECRET_STATUS = "secret_status"

type MicroserviceSecretStatusInst struct {
	MsInstKey     string
	ESSToken      string
	SecretsStatus map[string]*SecretStatus
}

type SecretStatus struct {
	SecretName string `json:"secret_name"`
	UpdateTime uint64 `json:"update_time"`
}

// for log and testing
func (w MicroserviceSecretStatusInst) String() string {
	return fmt.Sprintf("MsInstKey: %v, "+
		"ESSToken: ********, "+
		"SecretsStatus: %v",
		w.MsInstKey, w.SecretsStatus)
}

func (s SecretStatus) String() string {
	return fmt.Sprintf("{SecretName: %v, "+
		"UpdateTime: %v}",
		s.SecretName, s.UpdateTime)
}

func NewMSSInst(db *bolt.DB, msInstKey string, essToken string) (*MicroserviceSecretStatusInst, error) {
	if msInstKey == "" || essToken == "" {
		return nil, errors.New("microserviceInstanceKey or essToken is empty, cannot persist")
	}

	new_secret_status_inst := &MicroserviceSecretStatusInst{
		MsInstKey:     msInstKey,
		ESSToken:      essToken,
		SecretsStatus: make(map[string]*SecretStatus),
	}
	return saveMSSInst(db, new_secret_status_inst)
}

func NewSecretStatus(secretName string, updateTime uint64) *SecretStatus {
	return &SecretStatus{
		SecretName: secretName,
		UpdateTime: updateTime,
	}
}

func (w MicroserviceSecretStatusInst) GetKey() string {
	return w.MsInstKey
}

// save the given microserviceSecretStatus instance into the db. Key: MsInstKey, Value: MicroserviceSecretStatus Object
func saveMSSInst(db *bolt.DB, new_secret_status_inst *MicroserviceSecretStatusInst) (*MicroserviceSecretStatusInst, error) {
	return new_secret_status_inst, db.Update(func(tx *bolt.Tx) error {
		if b, err := tx.CreateBucketIfNotExists([]byte(SECRET_STATUS)); err != nil {
			return err
		} else if bytes, err := json.Marshal(new_secret_status_inst); err != nil {
			return fmt.Errorf("Unable to marshal new record: %v", err)
		} else if err := b.Put([]byte(new_secret_status_inst.MsInstKey), []byte(bytes)); err != nil {
			return fmt.Errorf("Unable to persist service instance: %v", err)
		}
		// success, close tx
		return nil
	})
}

func FindMSSInstWithKey(db *bolt.DB, ms_inst_key string) (*MicroserviceSecretStatusInst, error) {
	var pmsSecretStatusInst *MicroserviceSecretStatusInst
	pmsSecretStatusInst = nil

	// fetch microserviceSecretStatus instances
	readErr := db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(SECRET_STATUS)); b != nil {
			v := b.Get([]byte(ms_inst_key))

			var msSecretStatusInst MicroserviceSecretStatusInst

			if err := json.Unmarshal(v, &msSecretStatusInst); err != nil {
				glog.Errorf("Unable to deserialize microserviceSecretStatus instance db record: %v. Error: %v", v, err)
				return err
			} else {
				pmsSecretStatusInst = &msSecretStatusInst
				return nil
			}
		}

		return nil // end the transaction
	})

	if readErr != nil {
		return nil, readErr
	} else {
		return pmsSecretStatusInst, nil
	}
}

func FindMSSInstWithESSToken(db *bolt.DB, ess_token string) (*MicroserviceSecretStatusInst, error) {
	var pms *MicroserviceSecretStatusInst
	pms = nil

	// fetch microserviceSecretStatus instances
	readErr := db.View(func(tx *bolt.Tx) error {

		if b := tx.Bucket([]byte(SECRET_STATUS)); b != nil {
			cursor := b.Cursor()
			for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
				var msSecretStatusInstance MicroserviceSecretStatusInst
				if err := json.Unmarshal(value, &msSecretStatusInstance); err != nil {
					return err
				}

				if msSecretStatusInstance.ESSToken == ess_token {
					pms = &msSecretStatusInstance
					return nil
				}
			}
		}

		return nil // end the transaction
	})

	if readErr != nil {
		return nil, readErr
	} else {
		return pms, nil
	}
}

// delete a microserviceSecretStatus instance from db. It will NOT return error if it does not exist in the db
func DeleteMSSInstWithKey(db *bolt.DB, ms_inst_key string) (*MicroserviceSecretStatusInst, error) {
	if ms_inst_key == "" {
		return nil, errors.New("microserviceInstantKey (key) is empty, cannot remove")
	} else {
		if ms, err := FindMSSInstWithKey(db, ms_inst_key); err != nil {
			return nil, err
		} else if ms == nil {
			return nil, nil
		} else {
			return ms, db.Update(func(tx *bolt.Tx) error {
				if b, err := tx.CreateBucketIfNotExists([]byte(SECRET_STATUS)); err != nil {
					return err
				} else if err := b.Delete([]byte(ms_inst_key)); err != nil {
					return fmt.Errorf("Unable to delete microserviceSecretStatus instance %v: %v", ms_inst_key, err)
				} else {
					return nil
				}
			})
		}
	}
}

func DeleteMSSInstWithESSToken(db *bolt.DB, ess_token string) (*MicroserviceSecretStatusInst, error) {
	if ess_token == "" {
		return nil, errors.New("ess_token(key) is empty, cannot remove")
	} else {
		if ms, err := FindMSSInstWithESSToken(db, ess_token); err != nil {
			return nil, err
		} else if ms == nil {
			return nil, nil
		} else {
			return ms, db.Update(func(tx *bolt.Tx) error {
				if b, err := tx.CreateBucketIfNotExists([]byte(SECRET_STATUS)); err != nil {
					return err
				} else if err := b.Delete([]byte(ms.MsInstKey)); err != nil {
					return fmt.Errorf("Unable to delete microserviceSecretStatus instance with ess_token %v: %v", ess_token, err)
				} else {
					return nil
				}
			})
		}
	}
}

func SaveSecretStatus(db *bolt.DB, ms_inst_key string, secret_status *SecretStatus) (*MicroserviceSecretStatusInst, error) {
	return mssInstStateUpdate(db, ms_inst_key, func(c MicroserviceSecretStatusInst) *MicroserviceSecretStatusInst {
		c.SecretsStatus[secret_status.SecretName] = secret_status
		return &c
	})
}

func FindSecretStatus(db *bolt.DB, ms_inst_key string, secret_name string) (*SecretStatus, error) {
	secStatus := &SecretStatus{}
	mssinst, err := FindMSSInstWithKey(db, ms_inst_key)
	if err != nil {
		return secStatus, err
	}

	return mssinst.SecretsStatus[secret_name], nil
}

func FindUpdatedSecretsForMSSInstance(db *bolt.DB, ms_inst_key string) ([]string, error) {
	updatedSecretNames := make([]string, 0)
	if mssInst, err := FindMSSInstWithKey(db, ms_inst_key); err != nil {
		return updatedSecretNames, err
	} else {
		// get secretsStatus map for this instance
		secretsStatus := mssInst.SecretsStatus
		// Find secrets for given mssInst Key  in "Secrets" bucket
		secrets, err := FindAllSecretsForMS(db, mssInst.GetKey()) // list of secrets retrieved from "Secret" bucket
		if err != nil {
			return updatedSecretNames, err
		} else if secrets == nil || len(secrets.SecretsMap) == 0 {
			return updatedSecretNames, nil
		}

		// Go through the "Secrets" bucket secrets
		for secName, secret := range secrets.SecretsMap {
			// 1. If secret TimeLastUpdated == TimeCreated, no update
			// 2. If MSInstance secretStatus has no record, and update time > create time, has been updated
			// 3. If MSInstance secretStatus doesn't have this secret, and update time > create time, has been update
			// 4. If MSInstance secretStatus has this secret, and secret update time > secret status update time, has been updated

			if secret.TimeLastUpdated == 0 || secret.TimeLastUpdated == secret.TimeCreated {
				// the secret is not updated since created
				continue
			} else if secret.TimeLastUpdated > secret.TimeCreated {
				if len(secretsStatus) == 0 {
					updatedSecretNames = append(updatedSecretNames, secName)
				} else if secStat, ok := secretsStatus[secName]; !ok {
					updatedSecretNames = append(updatedSecretNames, secName)
				} else if secret.TimeLastUpdated > secStat.UpdateTime {
					updatedSecretNames = append(updatedSecretNames, secName)
				}
			}
		}
	}
	return updatedSecretNames, nil
}

// update the microserviceSecretStatus instance
func mssInstStateUpdate(db *bolt.DB, ms_inst_key string, fn func(MicroserviceSecretStatusInst) *MicroserviceSecretStatusInst) (*MicroserviceSecretStatusInst, error) {

	if mss, err := FindMSSInstWithKey(db, ms_inst_key); err != nil {
		return nil, err
	} else if mss == nil {
		return nil, fmt.Errorf("No record with key: %v", ms_inst_key)
	} else if updated := fn(*mss); updated == nil {
		// if SecretsStatus[secretName] doesn't exist, the field is not set, return nil with no error
		return nil, nil
	} else {
		// run this single contract through provided update function and persist it
		return updated, persistUpdatedMSSInst(db, ms_inst_key, updated)
	}
}

func persistUpdatedMSSInst(db *bolt.DB, ms_inst_key string, update *MicroserviceSecretStatusInst) error {
	return db.Update(func(tx *bolt.Tx) error {
		if b, err := tx.CreateBucketIfNotExists([]byte(SECRET_STATUS)); err != nil {
			return err
		} else {
			current := b.Get([]byte(ms_inst_key))
			var mod MicroserviceSecretStatusInst

			if current == nil {
				return fmt.Errorf("No service with given key available to update: %v", ms_inst_key)
			} else if err := json.Unmarshal(current, &mod); err != nil {
				return fmt.Errorf("Failed to unmarshal service DB data: %v. Error: %v", string(current), err)
			} else {

				// This code is running in a database transaction. Within the tx, the current record is
				// read and then updated according to the updates within the input update record. It is critical
				// to check for correct data transitions within the tx.
				mod.ESSToken = update.ESSToken
				mod.SecretsStatus = update.SecretsStatus

				if serialized, err := json.Marshal(mod); err != nil {
					return fmt.Errorf("Failed to serialize contract record: %v. Error: %v", mod, err)
				} else if err := b.Put([]byte(ms_inst_key), serialized); err != nil {
					return fmt.Errorf("Failed to write microserviceSecretStatus instance with key: %v. Error: %v", ms_inst_key, err)
				} else {
					glog.V(2).Infof("Succeeded updating microserviceSecretStatus instance record to %v", mod)
					return nil
				}
			}
		}
	})
}
