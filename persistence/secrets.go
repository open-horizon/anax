package persistence

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/semanticversion"
	"time"
)

// Secrets associated with their service instances. Keyed by microservice instance
const SECRETS = "secrets"

// The secrets from the agreements. Keyed by agreement id
const AGREEMENT_SECRETS = "agreement_secrets"

type PersistedServiceSecret struct {
	SvcOrgid        string
	SvcUrl          string
	SvcArch         string
	SvcVersionRange string
	SvcSecretName   string
	SvcSecretValue  string
	AgreementIds    []string
	ContainerIds    []string
	TimeCreated     uint64
	TimeLastUpdated uint64
}

type PersistedServiceSecrets struct {
	MsInstKey  string
	MsInstVers string // Version in the individual secrets is the range from the secret binding. This is the specific version associated with the msdef
	MsInstUrl  string
	MsInstOrg  string
	SecretsMap map[string]*PersistedServiceSecret
}

func PersistedSecretFromPolicySecret(inputSecretBindings []exchangecommon.SecretBinding, agId string) []PersistedServiceSecret {
	outputSecrets := []PersistedServiceSecret{}
	for _, secBind := range inputSecretBindings {
		for _, secArray := range secBind.Secrets {
			for secName, secDetails := range secArray {
				outputSecrets = append(outputSecrets, PersistedServiceSecret{SvcOrgid: secBind.ServiceOrgid, SvcUrl: secBind.ServiceUrl, SvcArch: secBind.ServiceArch, SvcVersionRange: secBind.ServiceVersionRange, SvcSecretName: secName, SvcSecretValue: secDetails, AgreementIds: []string{agId}})
			}
		}
	}

	return outputSecrets
}

// Save the secret bindings from an agreement
// This bucket is used to keep the secret information until such time that the microservice instance id is created
// After that id exists, the secrets will be saved in the SECRETS bucket keyed by ms instance id
func SaveAgreementSecrets(db *bolt.DB, agId string, secretsList *[]PersistedServiceSecret) error {
	if secretsList == nil {
		return nil
	}

	writeErr := db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(AGREEMENT_SECRETS))
		if err != nil {
			return err
		}

		if serial, err := json.Marshal(secretsList); err != nil {
			return fmt.Errorf("Failed to serialize agreement secrets list: Error: %v", err)
		} else {
			return bucket.Put([]byte(agId), serial)
		}
	})

	return writeErr
}

func FindAgreementSecrets(db *bolt.DB, agId string) (*[]PersistedServiceSecret, error) {
	if db == nil {
		return nil, nil
	}

	var psecretRec *[]PersistedServiceSecret
	readErr := db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(AGREEMENT_SECRETS)); b != nil {
			s := b.Get([]byte(agId))
			if s != nil {
				secretRec := []PersistedServiceSecret{}
				if err := json.Unmarshal(s, &secretRec); err != nil {
					glog.Errorf("Unable to deserialize agreement secret db record: %v. Error: %v", agId, err)
					return err
				} else {
					psecretRec = &secretRec
				}
			}
		}
		return nil
	})
	return psecretRec, readErr
}

func DeleteAgreementSecrets(db *bolt.DB, agId string) error {
	if db == nil {
		return nil
	}

	if agSecrets, err := FindAgreementSecrets(db, agId); err != nil {
		return err
	} else if agSecrets == nil {
		return nil
	} else {
		return db.Update(func(tx *bolt.Tx) error {

			if b, err := tx.CreateBucketIfNotExists([]byte(AGREEMENT_SECRETS)); err != nil {
				return err
			} else if err := b.Delete([]byte(agId)); err != nil {
				return fmt.Errorf("Unable to delete agreement secrets object: %v", err)
			} else {
				return nil
			}
		})
	}
}

// Saves the given secret to the agent db
func SaveSecret(db *bolt.DB, secretName string, msInstKey string, msInstVers string, secretToSave *PersistedServiceSecret) error {
	if secretToSave == nil {
		return nil
	}
	secretToSaveAll := &PersistedServiceSecrets{MsInstKey: msInstKey, MsInstOrg: secretToSave.SvcOrgid, MsInstUrl: secretToSave.SvcUrl, MsInstVers: msInstVers, SecretsMap: map[string]*PersistedServiceSecret{}}
	if allSecsForServiceInDB, err := FindAllSecretsForMS(db, msInstKey); err != nil {
		return fmt.Errorf("Failed to get all secrets for microservice %v. Error was: %v", msInstKey, err)
	} else if allSecsForServiceInDB != nil {
		secretToSaveAll = allSecsForServiceInDB
	}

	// if TimeCreated == TimeLastUpdated, then secrets API won't return secret name as updated secret
	timestamp := uint64(time.Now().Unix())
	if secretToSave.TimeCreated == 0 {
		secretToSave.TimeCreated = timestamp
	}

	if mergedSec, ok := secretToSaveAll.SecretsMap[secretName]; ok {
		mergedSec.AgreementIds = cutil.MergeSlices(mergedSec.AgreementIds, secretToSave.AgreementIds)
		mergedSec.ContainerIds = cutil.MergeSlices(mergedSec.ContainerIds, secretToSave.ContainerIds)
		if mergedSec.SvcSecretValue != secretToSave.SvcSecretValue {
			mergedSec.TimeLastUpdated = timestamp
			mergedSec.SvcSecretValue = secretToSave.SvcSecretValue
		}
		secretToSaveAll.SecretsMap[secretName] = mergedSec
	} else {
		secretToSave.TimeLastUpdated = uint64(time.Now().Unix())
		secretToSaveAll.SecretsMap[secretName] = secretToSave
	}

	return SaveAllSecretsForService(db, msInstKey, secretToSaveAll)
}

func SaveAllSecretsForService(db *bolt.DB, msInstId string, secretToSaveAll *PersistedServiceSecrets) error {
	if db == nil {
		return nil
	}
	writeErr := db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(SECRETS))
		if err != nil {
			return err
		}

		if serial, err := json.Marshal(secretToSaveAll); err != nil {
			return fmt.Errorf("Failed to serialize secrets: Error: %v", err)
		} else {
			return bucket.Put([]byte(msInstId), serial)
		}
	})

	return writeErr
}

// Gets the secret from the database, no error returned if none is found in the db
func FindAllSecretsForMS(db *bolt.DB, msInstId string) (*PersistedServiceSecrets, error) {
	if db == nil {
		return nil, nil
	}
	var psecretRec *PersistedServiceSecrets
	readErr := db.View(func(tx *bolt.Tx) error {

		if b := tx.Bucket([]byte(SECRETS)); b != nil {
			s := b.Get([]byte(msInstId))
			if s != nil {
				secretRec := PersistedServiceSecrets{}
				if err := json.Unmarshal(s, &secretRec); err != nil {
					glog.Errorf("Unable to deserialize service secret db record: %v. Error: %v", msInstId, err)
					return err
				} else {
					psecretRec = &secretRec
				}
			}
		}
		return nil
	})

	if readErr != nil {
		return nil, readErr
	} else {
		return psecretRec, nil
	}
}

// Find a particular secret, if a version range is provided it must match the exact range on the secret, if a specific version is given return the first matching secret range
func FindSingleSecretForService(db *bolt.DB, secName string, msInstId string) (*PersistedServiceSecret, error) {
	allSec, err := FindAllSecretsForMS(db, msInstId)
	if err != nil {
		return nil, err
	}

	if allSec != nil {
		retSec := allSec.SecretsMap[secName]
		return retSec, nil
	}

	return nil, nil
}

func AddContainerIdToSecret(db *bolt.DB, secName string, msInstId string, msDefVers string, containerId string) error {
	sec, err := FindSingleSecretForService(db, secName, msInstId)
	if err != nil {
		return err
	}

	if !cutil.SliceContains(sec.ContainerIds, containerId) {
		sec.ContainerIds = append(sec.ContainerIds, containerId)
		err = SaveSecret(db, secName, msInstId, msDefVers, sec)
		if err != nil {
			return err
		}
	}
	return nil
}

type SecFilter func(PersistedServiceSecrets) bool

func UrlSecFilter(serviceUrl string) SecFilter {
	return func(e PersistedServiceSecrets) bool {
		return cutil.FormExchangeIdWithSpecRef(e.MsInstUrl) == cutil.FormExchangeIdWithSpecRef(serviceUrl)
	}
}

func OrgSecFilter(serviceOrg string) SecFilter {
	return func(e PersistedServiceSecrets) bool { return e.MsInstOrg == serviceOrg }
}

func VersRangeSecFilter(versRange string) SecFilter {
	return func(e PersistedServiceSecrets) bool {
		if versExp, err := semanticversion.Version_Expression_Factory(e.MsInstVers); err != nil {
			return false
		} else if inRange, err := versExp.Is_within_range(versRange); err != nil {
			return false
		} else {
			return inRange
		}
	}
}

func FindAllServiceSecretsWithFilters(db *bolt.DB, filters []SecFilter) ([]PersistedServiceSecrets, error) {
	matchingSecrets := []PersistedServiceSecrets{}
	if db == nil {
		return matchingSecrets, nil
	}

	readErr := db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(SECRETS)); b != nil {
			b.ForEach(func(k, v []byte) error {
				var s PersistedServiceSecrets

				if err := json.Unmarshal(v, &s); err != nil {
					glog.Errorf("Unable to deserialize db record: %v", v)
				} else {
					exclude := false

					for _, filterFn := range filters {
						if !filterFn(s) {
							exclude = true
						}
					}

					if !exclude {
						matchingSecrets = append(matchingSecrets, s)
					}
				}
				return nil
			})
			return nil
		}
		return nil
	})

	if readErr != nil {
		return nil, readErr
	} else {
		return matchingSecrets, nil
	}
}

func FindAllServiceSecretsWithSpecs(db *bolt.DB, svcUrl string, svcOrg string) ([]PersistedServiceSecrets, error) {
	filters := []SecFilter{UrlSecFilter(svcUrl), OrgSecFilter(svcOrg)}

	return FindAllServiceSecretsWithFilters(db, filters)
}

// Returns the secret from the db if it was there. No error returned if it is not in the db
func DeleteSecrets(db *bolt.DB, secName string, msInstId string) (*PersistedServiceSecret, error) {
	if db == nil {
		return nil, nil
	}

	if allSec, err := FindAllSecretsForMS(db, msInstId); err != nil {
		return nil, err
	} else if allSec != nil {
		if _, ok := allSec.SecretsMap[secName]; ok {
			delete(allSec.SecretsMap, secName)
		}
		if len(allSec.SecretsMap) == 0 {
			retSec := allSec.SecretsMap[secName]
			return retSec, db.Update(func(tx *bolt.Tx) error {
				if b, err := tx.CreateBucketIfNotExists([]byte(SECRETS)); err != nil {
					return err
				} else if err := b.Delete([]byte(msInstId)); err != nil {
					return fmt.Errorf("Unable to delete secret %v for microservice def %v: %v", secName, msInstId, err)
				} else {
					return nil
				}
			})
		} else {
			SaveAllSecretsForService(db, msInstId, allSec)
		}
	}
	return nil, nil
}

func DeleteSecretsSpec(db *bolt.DB, secName string, svcName string, svcOrg string, svcVersionRange string) error {
	if allServiceSecrets, err := FindAllServiceSecretsWithSpecs(db, svcName, svcOrg); err != nil {
		return err
	} else {
		for _, dbServiceSecrets := range allServiceSecrets {
			if _, ok := dbServiceSecrets.SecretsMap[secName]; ok {
				delete(dbServiceSecrets.SecretsMap, secName)
				if err = SaveAllSecretsForService(db, dbServiceSecrets.MsInstKey, &dbServiceSecrets); err != nil {
					glog.Errorf("Failed to delete secret %v for service %v from db: %v", dbServiceSecrets.MsInstKey, secName, err)
				}
			}
		}
	}
	return nil
}
