package bolt

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/agreementbot/persistence"
	"github.com/open-horizon/anax/policy"
)

func init() {
	persistence.Register("bolt", new(AgbotBoltDB))
}

// Functions that implement the database replacement abstraction.

// Constants used in this package.
const BOLTDB_DATABASE_NAME = "agreementbot.db"
const AGREEMENTS = "agreements" // The bolt DB bucket name for agreement objects.

// This is the object that represents the handle to the bolt func (db *AgbotBoltDB)
type AgbotBoltDB struct {
	db *bolt.DB
}

func (db *AgbotBoltDB) String() string {
	return fmt.Sprintf("DB Handle: %v", db.db)
}

func (db *AgbotBoltDB) GetAgreementCount(partition string) (int64, int64, error) {
	var activeNum, archivedNum int64
	for _, protocol := range policy.AllAgreementProtocols() {
		if activeAgreements, err := db.FindAgreements([]persistence.AFilter{persistence.UnarchivedAFilter()}, protocol); err != nil {
			return 0, 0, err
		} else if archivedAgreements, err := db.FindAgreements([]persistence.AFilter{persistence.ArchivedAFilter()}, protocol); err != nil {
			return 0, 0, err
		} else {
			activeNum += int64(len(activeAgreements))
			archivedNum += int64(len(archivedAgreements))
		}
	}
	return activeNum, archivedNum, nil
}

func (db *AgbotBoltDB) FindAgreements(filters []persistence.AFilter, protocol string) ([]persistence.Agreement, error) {
	agreements := make([]persistence.Agreement, 0)

	readErr := db.db.View(func(tx *bolt.Tx) error {

		if b := tx.Bucket([]byte(bucketName(protocol))); b != nil {
			b.ForEach(func(k, v []byte) error {

				var a persistence.Agreement

				if err := json.Unmarshal(v, &a); err != nil {
					glog.Errorf("Unable to deserialize db record: %v", v)
				} else {
					if !a.Archived {
						glog.V(5).Infof("Demarshalled agreement in DB: %v", a)
					}
					exclude := false
					for _, filterFn := range filters {
						if !filterFn(a) {
							exclude = true
						}
					}
					if !exclude {
						agreements = append(agreements, a)
					}
				}
				return nil
			})
		}

		return nil // end the transaction
	})

	if readErr != nil {
		return nil, readErr
	} else {
		return agreements, nil
	}
}

func (db *AgbotBoltDB) AgreementAttempt(agreementid string, org string, deviceid string, policyName string, bcType string, bcName string, bcOrg string, agreementProto string, pattern string, serviceId string, nhPolicy policy.NodeHealth) error {
	if agreement, err := persistence.NewAgreement(agreementid, org, deviceid, policyName, bcType, bcName, bcOrg, agreementProto, pattern, serviceId, nhPolicy); err != nil {
		return err
	} else if err := db.persistNew(agreement.CurrentAgreementId, bucketName(agreementProto), &agreement); err != nil {
		return err
	} else {
		return nil
	}
}

func (db *AgbotBoltDB) AgreementUpdate(agreementid string, proposal string, policy string, dvPolicy policy.DataVerification, defaultCheckRate uint64, hash string, sig string, protocol string, agreementProtoVersion int) (*persistence.Agreement, error) {
	return persistence.AgreementUpdate(db, agreementid, proposal, policy, dvPolicy, defaultCheckRate, hash, sig, protocol, agreementProtoVersion)
}

func (db *AgbotBoltDB) AgreementMade(agreementId string, counterParty string, signature string, protocol string, hapartners []string, bcType string, bcName string, bcOrg string) (*persistence.Agreement, error) {
	return persistence.AgreementMade(db, agreementId, counterParty, signature, protocol, hapartners, bcType, bcName, bcOrg)
}

func (db *AgbotBoltDB) AgreementBlockchainUpdate(agreementId string, consumerSig string, hash string, counterParty string, signature string, protocol string) (*persistence.Agreement, error) {
	return persistence.AgreementBlockchainUpdate(db, agreementId, consumerSig, hash, counterParty, signature, protocol)
}

func (db *AgbotBoltDB) AgreementBlockchainUpdateAck(agreementId string, protocol string) (*persistence.Agreement, error) {
	return persistence.AgreementBlockchainUpdateAck(db, agreementId, protocol)
}

func (db *AgbotBoltDB) AgreementFinalized(agreementId string, protocol string) (*persistence.Agreement, error) {
	return persistence.AgreementFinalized(db, agreementId, protocol)
}

func (db *AgbotBoltDB) AgreementTimedout(agreementid string, protocol string) (*persistence.Agreement, error) {
	return persistence.AgreementTimedout(db, agreementid, protocol)
}

func (db *AgbotBoltDB) DataVerified(agreementid string, protocol string) (*persistence.Agreement, error) {
	return persistence.DataVerified(db, agreementid, protocol)
}

func (db *AgbotBoltDB) DataNotVerified(agreementid string, protocol string) (*persistence.Agreement, error) {
	return persistence.DataNotVerified(db, agreementid, protocol)
}

func (db *AgbotBoltDB) DataNotification(agreementid string, protocol string) (*persistence.Agreement, error) {
	return persistence.DataNotification(db, agreementid, protocol)
}

func (db *AgbotBoltDB) MeteringNotification(agreementid string, protocol string, mn string) (*persistence.Agreement, error) {
	return persistence.MeteringNotification(db, agreementid, protocol, mn)
}

func (db *AgbotBoltDB) ArchiveAgreement(agreementid string, protocol string, reason uint, desc string) (*persistence.Agreement, error) {
	return persistence.ArchiveAgreement(db, agreementid, protocol, reason, desc)
}

// no error on not found, only nil
func (db *AgbotBoltDB) FindSingleAgreementByAgreementId(agreementid string, protocol string, filters []persistence.AFilter) (*persistence.Agreement, error) {
	filters = append(filters, persistence.IdAFilter(agreementid))

	if agreements, err := db.FindAgreements(filters, protocol); err != nil {
		return nil, err
	} else if len(agreements) > 1 {
		return nil, fmt.Errorf("Expected only one record for agreementid: %v, but retrieved: %v", agreementid, agreements)
	} else if len(agreements) == 0 {
		return nil, nil
	} else {
		return &agreements[0], nil
	}
}

// no error on not found, only nil
func (db *AgbotBoltDB) FindSingleAgreementByAgreementIdAllProtocols(agreementid string, protocols []string, filters []persistence.AFilter) (*persistence.Agreement, error) {
	filters = append(filters, persistence.IdAFilter(agreementid))

	for _, protocol := range protocols {
		if agreements, err := db.FindAgreements(filters, protocol); err != nil {
			return nil, err
		} else if len(agreements) > 1 {
			return nil, fmt.Errorf("Expected only one record for agreementid: %v, but retrieved: %v", agreementid, agreements)
		} else if len(agreements) == 0 {
			continue
		} else {
			return &agreements[0], nil
		}
	}
	return nil, nil
}

func (db *AgbotBoltDB) SingleAgreementUpdate(agreementid string, protocol string, fn func(persistence.Agreement) *persistence.Agreement) (*persistence.Agreement, error) {
	if agreement, err := db.FindSingleAgreementByAgreementId(agreementid, protocol, []persistence.AFilter{}); err != nil {
		return nil, err
	} else if agreement == nil {
		return nil, fmt.Errorf("Unable to locate agreement id: %v", agreementid)
	} else {
		updated := fn(*agreement)
		return updated, db.persistUpdatedAgreement(agreementid, protocol, updated)
	}
}

// does whole-member replacements of values that are legal to change during the course of an agreement's life
func (db *AgbotBoltDB) persistUpdatedAgreement(agreementid string, protocol string, update *persistence.Agreement) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		if b, err := tx.CreateBucketIfNotExists([]byte(AGREEMENTS + "-" + protocol)); err != nil {
			return err
		} else {
			current := b.Get([]byte(agreementid))
			var mod persistence.Agreement

			if current == nil {
				return fmt.Errorf("No agreement with given id available to update: %v", agreementid)
			} else if err := json.Unmarshal(current, &mod); err != nil {
				return fmt.Errorf("Failed to unmarshal agreement DB data: %v", string(current))
			} else {

				// This code is running in a database transaction. Within the tx, the current record (mod) is
				// read and then updated according to the updates within the input update record. It is critical
				// to check for correct data transitions within the tx.
				persistence.ValidateStateTransition(&mod, update)

				if serialized, err := json.Marshal(mod); err != nil {
					return fmt.Errorf("Failed to serialize agreement record: %v", mod)
				} else if err := b.Put([]byte(agreementid), serialized); err != nil {
					return fmt.Errorf("Failed to write record with key: %v", agreementid)
				} else {
					glog.V(2).Infof("Succeeded updating agreement record to %v", mod)
				}
			}
		}
		return nil
	})
}

func (db *AgbotBoltDB) DeleteAgreement(pk string, protocol string) error {
	if pk == "" {
		return fmt.Errorf("Missing required arg pk")
	} else {

		return db.db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(bucketName(protocol)))
			if b == nil {
				return fmt.Errorf("Unknown bucket: %v", bucketName(protocol))
			} else if existing := b.Get([]byte(pk)); existing == nil {
				glog.Errorf("Warning: record deletion requested, but record does not exist: %v", pk)
				return nil // handle already-deleted agreement as success
			} else {
				var record persistence.Agreement

				if err := json.Unmarshal(existing, &record); err != nil {
					glog.Errorf("Error deserializing agreement: %v. This is a pre-deletion warning message function so deletion will still proceed", record)
				} else if record.CurrentAgreementId != "" && !record.Archived {
					glog.Warningf("Warning! Deleting an agreement record with an agreement id, this operation should only be done after cancelling on the blockchain.")
				}
			}

			return b.Delete([]byte(pk))
		})
	}
}

func (db *AgbotBoltDB) persistNew(pk string, bucket string, record interface{}) error {
	if pk == "" || bucket == "" {
		return fmt.Errorf("Missing required args, pk and/or bucket")
	} else {
		writeErr := db.db.Update(func(tx *bolt.Tx) error {

			if b, err := tx.CreateBucketIfNotExists([]byte(bucket)); err != nil {
				return err
			} else if existing := b.Get([]byte(pk)); existing != nil {
				return fmt.Errorf("Bucket %v already contains record with primary key: %v", bucket, pk)
			} else if bytes, err := json.Marshal(record); err != nil {
				return fmt.Errorf("Unable to serialize record %v. Error: %v", record, err)
			} else if err := b.Put([]byte(pk), bytes); err != nil {
				return fmt.Errorf("Unable to write to record to bucket %v. Primary key of record: %v", bucket, pk)
			} else {
				glog.V(2).Infof("Succeeded writing record identified by %v in %v", pk, bucket)
				return nil
			}
		})

		return writeErr
	}
}

func (db *AgbotBoltDB) Close() {
	glog.V(2).Infof("Closing bolt database")
	db.db.Close()
	glog.V(2).Infof("Closed bolt database")
}

// Utility functions specific to the bolt database implementation
func bucketName(protocol string) string {
	return AGREEMENTS + "-" + protocol
}
