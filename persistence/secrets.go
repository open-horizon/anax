package persistence

import (
	"encoding/json"
	"fmt"

	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/policy"
)

const SECRETS = "secrets"

const (
	Initial  = "initial"
	Received = "received"
)

type PersistedServiceSecret struct {
	SvcOrgid        string
	SvcUrl          string
	SvcArch         string
	SvcVersion      string
	SvcSecretName   string
	SvcSecretValue  string
	SvcSecretStatus string
}

func persistedSecretFromPolicySecret(inputSecrets policy.ServiceSecret) []PersistedServiceSecret {
	outputSecrets := []PersistedServiceSecret{}
	for secName, secValue := range inputSecrets.ServiceSecrets {
		outputSecrets = append(outputSecrets, PersistedServiceSecret{SvcOrgid: inputSecrets.ServiceOrgid, SvcUrl: inputSecrets.ServiceUrl, SvcArch: inputSecrets.ServiceArch, SvcVersion: inputSecrets.ServiceVersion, SvcSecretName: secName, SvcSecretValue: secValue})
	}
	return outputSecrets
}

func secretKey(svcName string, secretName string) string {
	return fmt.Sprintf("%s/%s", svcName, secretName)
}

// Converts the policy form secrets to the persistent form and saves them in the db one by one. Can be used for a new agreement or for a change to an existing agreement.
func PersistSecretsFromProposal(db *bolt.DB, secretsList []policy.ServiceSecret) error {
	for _, secrets := range secretsList {
		pSvcSecrets := persistedSecretFromPolicySecret(secrets)
		for _, secToSave := range pSvcSecrets {
			if db != nil {
				secToSave.SvcSecretStatus = Initial
				err := SaveSecrets(db, secToSave)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Saves the given secret to the agent db
func SaveSecrets(db *bolt.DB, secretToSave PersistedServiceSecret) error {
	writeErr := db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(SECRETS))
		if err != nil {
			return err
		}

		if serial, err := json.Marshal(secretToSave); err != nil {
			return fmt.Errorf("Failed to serialize secrets: Error: %v", err)
		} else {
			return bucket.Put([]byte(secretKey(secretToSave.SvcUrl, secretToSave.SvcSecretName)), serial)
		}
	})

	return writeErr
}

// Gets the secret from the database, no error returned if none is found in the db
func FindSecrets(db *bolt.DB, secName string, svcName string) (*PersistedServiceSecret, error) {
	psecret := &PersistedServiceSecret{}

	readErr := db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(SECRETS)); b != nil {
			s := b.Get([]byte(secretKey(svcName, secName)))
			if s != nil {
				if err := json.Unmarshal(s, psecret); err != nil {
					glog.Errorf("Unable to deserialize service secret db record: %v. Error: %v", secretKey(svcName, secName), err)
					return err
				}
			} else {
				glog.Errorf("no entry found for %v", secretKey(svcName, secName))
			}

		}
		return nil
	})

	if readErr != nil {
		return nil, readErr
	} else {
		return psecret, nil
	}
}

// Gets secrets which secret status is "initial"
func FindUpdatedSecrets(db *bolt.DB, svcName string) ([]PersistedServiceSecret, error) {
	result := make([]PersistedServiceSecret, 0)

	function := func(secret PersistedServiceSecret) {
		if svcName == secret.SvcUrl && Initial == secret.SvcSecretStatus {
			result = append(result, secret)
		}
	}

	readErr := db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(SECRETS)); b != nil {
			cursor := b.Cursor()
			for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
				var secret PersistedServiceSecret
				if err := json.Unmarshal(value, &secret); err != nil {
					return err
				}
				function(secret)
			}
		}
		return nil
	})

	if readErr != nil {
		return nil, readErr
	} else {
		return result, nil
	}

}

// Updates the status of secret
func UpdateSecretStatus(db *bolt.DB, secName string, svcName string, secStatus string) error {
	psecret := PersistedServiceSecret{}
	function := func(secret PersistedServiceSecret) (PersistedServiceSecret, error) {
		secret.SvcSecretStatus = secStatus
		return secret, nil
	}

	readErr := db.Update(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(SECRETS)); b != nil {
			id := []byte(secretKey(svcName, secName))
			s := b.Get(id)
			if s != nil {
				if err := json.Unmarshal(s, &psecret); err != nil {
					glog.Errorf("Unable to deserialize service secret db record: %v. Error: %v", secretKey(svcName, secName), err)
					return err
				} else if secret, err := function(psecret); err != nil {
					glog.Errorf("Unable set secret status for db record: %v. Error: %v", secretKey(svcName, secName), err)
					return err
				} else if encodedSecret, err := json.Marshal(secret); err != nil {
					glog.Errorf("Unable to serialize the updated secret db record: %v. Error: %v", secretKey(svcName, secName), err)
					return err
				} else if err = tx.Bucket([]byte(SECRETS)).Put([]byte(id), []byte(encodedSecret)); err != nil {
					glog.Errorf("Unable to updated secret db record: %v. Error: %v", secretKey(svcName, secName), err)
					return err
				}
			} else {
				glog.Errorf("no entry found for %v", secretKey(svcName, secName))
			}
		}
		return nil
	})

	return readErr

}

// Returns the secret from the db if it was there. No error returned if it is not in the db
func DeleteSecrets(db *bolt.DB, secName string, svcName string) (*PersistedServiceSecret, error) {
	if sec, err := FindSecrets(db, secName, svcName); err != nil {
		return nil, err
	} else {
		return sec, db.Update(func(tx *bolt.Tx) error {
			if b, err := tx.CreateBucketIfNotExists([]byte(MICROSERVICE_INSTANCES)); err != nil {
				return err
			} else if err := b.Delete([]byte(secretKey(svcName, secName))); err != nil {
				return fmt.Errorf("Unable to delete service instance %v: %v", secretKey(svcName, secName), err)
			} else {
				return nil
			}
		})
	}
}

