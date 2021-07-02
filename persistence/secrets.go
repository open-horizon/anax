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

const SECRETS = "secrets"

type PersistedServiceSecret struct {
	SvcOrgid        string
	SvcUrl          string
	SvcArch         string
	SvcVersionRange string
	SvcSecretName   string
	SvcSecretValue  string
	AgreementIds    []string
	TimeCreated     uint64
	TimeLastUpdated uint64
}

type PersistedServiceSecrets struct {
	MsDefId    string
	MsDefVers  string // Version in the individual secrets is the range from the secret binding. This is the specific version associated with the msdef
	MsDefUrl   string
	MsDefOrg   string
	SecretsMap map[string]PersistedServiceSecret
}

func PersistedSecretFromPolicySecret(inputSecretBindings []exchangecommon.SecretBinding, inputBoundSecrets []exchangecommon.BoundSecret, agId string) []PersistedServiceSecret {
	boundSecFullMap := exchangecommon.BoundSecret{}
	for _, boundSecMap := range inputBoundSecrets {
		for key, val := range boundSecMap {
			boundSecFullMap[key] = val
		}
	}

	outputSecrets := []PersistedServiceSecret{}
	for _, secBind := range inputSecretBindings {
		for _, secArray := range secBind.Secrets {
			for secName, _ := range secArray {
				outputSecrets = append(outputSecrets, PersistedServiceSecret{SvcOrgid: secBind.ServiceOrgid, SvcUrl: secBind.ServiceUrl, SvcArch: secBind.ServiceArch, SvcVersionRange: secBind.ServiceVersionRange, SvcSecretName: secName, SvcSecretValue: boundSecFullMap[secName], AgreementIds: []string{agId}})
			}
		}
	}

	return outputSecrets
}

// Saves the given secret to the agent db
func SaveSecret(db *bolt.DB, secretName string, msDefId string, msDefVers string, secretToSave *PersistedServiceSecret) error {
	if secretToSave == nil {
		return nil
	}
	secretToSaveAll := PersistedServiceSecrets{MsDefId: msDefId, MsDefOrg: secretToSave.SvcOrgid, MsDefUrl: secretToSave.SvcUrl, MsDefVers: msDefVers, SecretsMap: map[string]PersistedServiceSecret{}}
	if allSecsForServiceInDB, err := FindAllSecretsForMS(db, msDefId); err != nil {
		return fmt.Errorf("Failed to get all secrets for microservice %v. Error was: %v", msDefId, err)
	} else if allSecsForServiceInDB != nil {
		secretToSaveAll = *allSecsForServiceInDB
	}

	if secretToSave.TimeCreated == 0 {
		secretToSave.TimeCreated = uint64(time.Now().Unix())
	}

	secretToSave.TimeLastUpdated = uint64(time.Now().Unix())

	if mergedSec, ok := secretToSaveAll.SecretsMap[secretName]; ok {
		mergedSec.AgreementIds = append(mergedSec.AgreementIds, secretToSave.AgreementIds...)
		secretToSaveAll.SecretsMap[secretName] = mergedSec
	} else {
		secretToSaveAll.SecretsMap[secretName] = *secretToSave
	}

	return SaveAllServiceSecrets(db, msDefId, secretToSaveAll)
}

func SaveAllServiceSecrets(db *bolt.DB, msDefId string, secretToSaveAll PersistedServiceSecrets) error {
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
			return bucket.Put([]byte(msDefId), serial)
		}
	})

	return writeErr
}

// Gets the secret from the database, no error returned if none is found in the db
func FindAllSecretsForMS(db *bolt.DB, msDefId string) (*PersistedServiceSecrets, error) {
	if db == nil {
		return nil, nil
	}
	
	var psecretRec *PersistedServiceSecrets
	readErr := db.View(func(tx *bolt.Tx) error {

		if b := tx.Bucket([]byte(SECRETS)); b != nil {
			s := b.Get([]byte(msDefId))
			if s != nil {
				secretRec := PersistedServiceSecrets{}
				if err := json.Unmarshal(s, &secretRec); err != nil {
					glog.Errorf("Unable to deserialize service secret db record: %v. Error: %v", msDefId, err)
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
func FindSingleSecretForService(db *bolt.DB, secName string, msDefId string) (*PersistedServiceSecret, error) {
	allSec, err := FindAllSecretsForMS(db, msDefId)
	if err != nil {
		return nil, err
	}

	if allSec != nil {
		retSec := allSec.SecretsMap[secName]
		return &retSec, nil
	}

	return nil, nil
}

type SecFilter func(PersistedServiceSecrets) bool

func UrlSecFilter(serviceUrl string) SecFilter {
	return func(e PersistedServiceSecrets) bool {
		return cutil.FormExchangeIdWithSpecRef(e.MsDefUrl) == cutil.FormExchangeIdWithSpecRef(serviceUrl)
	}
}

func OrgSecFilter(serviceOrg string) SecFilter {
	return func(e PersistedServiceSecrets) bool { return e.MsDefOrg == serviceOrg }
}

func VersRangeSecFilter(versRange string) SecFilter {
	return func(e PersistedServiceSecrets) bool {
		if versExp, err := semanticversion.Version_Expression_Factory(e.MsDefVers); err != nil {
			return false
		} else if inRange, err := versExp.Is_within_range(versRange); err != nil {
			return false
		} else {
			return inRange
		}
	}
}

func VersSecFilter(vers string) SecFilter {
	return func(e PersistedServiceSecrets) bool { return e.MsDefVers == vers }
}

func FindAllServiceSecretsWithFilters(db *bolt.DB, filters []SecFilter) ([]PersistedServiceSecrets, error) {
	if db == nil {
		return nil, nil
	}

	matchingSecrets := []PersistedServiceSecrets{}

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

func FindAllServiceSecretsWithSpecs(db *bolt.DB, svcUrl string, svcOrg string, svcVersionRange string) ([]PersistedServiceSecrets, error) {
	filters := []SecFilter{UrlSecFilter(svcUrl), OrgSecFilter(svcOrg), VersRangeSecFilter(svcVersionRange)}

	return FindAllServiceSecretsWithFilters(db, filters)
}

// Returns the secret from the db if it was there. No error returned if it is not in the db
func DeleteSecrets(db *bolt.DB, secName string, msDefId string) (*PersistedServiceSecret, error) {
	if db == nil {
		return nil, nil
	}

	if allSec, err := FindAllSecretsForMS(db, msDefId); err != nil {
		return nil, err
	} else if allSec != nil {
		if _, ok := allSec.SecretsMap[secName]; ok {
			delete(allSec.SecretsMap, secName)
		}
		if len(allSec.SecretsMap) == 0 {
			retSec := allSec.SecretsMap[secName]
			return &retSec, db.Update(func(tx *bolt.Tx) error {
				if b, err := tx.CreateBucketIfNotExists([]byte(SECRETS)); err != nil {
					return err
				} else if err := b.Delete([]byte(msDefId)); err != nil {
					return fmt.Errorf("Unable to delete secret %v for microservice def %v: %v", secName, msDefId, err)
				} else {
					return nil
				}
			})
		} else {
			SaveAllServiceSecrets(db, msDefId, *allSec)
		}
	}
	return nil, nil
}

// Remove this agId from all the secrets it is in. If it is the only agId for that secret, remove the secret
func DeleteAllSecForAgreement(db *bolt.DB, agreementId string) error {
	if allSec, err := FindAllServiceSecretsWithFilters(db, []SecFilter{}); err != nil {
		return err
	} else {
		for _, svcAllSec := range allSec {
			for secName, svcSec := range svcAllSec.SecretsMap {
				if cutil.SliceContains(svcSec.AgreementIds, agreementId) {
					if len(svcSec.AgreementIds) == 1 {
						if _, err := DeleteSecrets(db, secName, svcAllSec.MsDefId); err != nil {
							glog.Errorf("Error deleting secret %v for agreement %v: %v", secName, agreementId, err)
						}
					} else {
						for ix, agId := range svcSec.AgreementIds {
							if agId == agreementId {
								svcSec.AgreementIds = append(svcSec.AgreementIds[:ix], svcSec.AgreementIds[ix+1:]...)
								break
							}
						}
						if err := SaveAllServiceSecrets(db, svcAllSec.MsDefId, svcAllSec); err != nil {
							glog.Errorf("Error saving updated secret object after deleting secret %v for agreement %v: %v", secName, agreementId, err)
						}
					}
				}
			}
		}
	}
	return nil
}

func DeleteSecretsSpec(db *bolt.DB, secName string, svcName string, svcOrg string, svcVersionRange string) error {
	if allServiceSecrets, err := FindAllServiceSecretsWithSpecs(db, svcName, svcOrg, svcVersionRange); err != nil {
		return err
	} else {
		for _, dbServiceSecrets := range allServiceSecrets {
			if _, ok := dbServiceSecrets.SecretsMap[secName]; ok {
				delete(dbServiceSecrets.SecretsMap, secName)
				if err = SaveAllServiceSecrets(db, dbServiceSecrets.MsDefId, dbServiceSecrets); err != nil {
					glog.Errorf("Failed to delete secret %v for service %v from db: %v", dbServiceSecrets.MsDefId, secName, err)
				}
			}
		}
	}
	return nil
}
