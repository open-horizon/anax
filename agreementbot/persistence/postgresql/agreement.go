package postgresql

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/agreementbot/persistence"
	"github.com/open-horizon/anax/policy"
	"strings"
)

// This function registers an uninitialized agbot DB instance with the DB plugin registry. The plugin's Initialize
// method is used to configure the object.
func init() {
	persistence.Register("postgresql", new(AgbotPostgresqlDB))
}

// Constants for the SQL statements that are used to work with agreements. Agreements are partitioned by agbot instances. Each
// agbot instance "owns" 1 partitions in the database. Postgresql supports automatic partitioning, which is what we want
// to use but cannot because Postgresql 10 is too new. Instead we have implemented our own partitioning scheme using table inheritance.
// There is a "main" table (called agreements) that defines the schema for all the partitions, and then there is a
// set of separate tables (called agreements_<partition_name>, one for each partition) that we have to create. When an agbot
// comes up it will attempt to create the main table and a partition table (and index) for its primary partition that inherites
// from the main table. The main table will never have any rows in it. Our code will use the partition name to figure out which table
// to use for INSERTs/UPDATEs and SELECTs based on the partition value provided in the SQL statement. Postgresql allows us
// to query the main table for cases where the caller might not know the partition holding the record of interest. We exploit both
// forms of query.
//
// IMPORTANT NOTE ====================================================================================================================
// The lifecycle of the workload usage records is non-standard and therefore likely to be unexpected. Each record is loosely
// tied to an agreement. Records are specific to a device that the agbot is working with, and retains state outside the scope
// of any agreement between the device and agbot. However. the state is specific to how to make the next agreement with the
// device and therefore these records need to be kept in the same partition where the agbot is making new agreements. That means
// these records sometimes need to be "moved" from one partition to another. It also means that the partition coud be removed
// well before all the agreement records in the same partition. For example. agreement 1 is in partition zero. The agbot is restarted
// with a new primary partition called one. If/when the agreement in partition zero ends, a new agreement with that device will be
// written into partition one. Partition zero will have the archived agreement in it. When the new agreement is made, the workload
// usage record will be moved to partition one. Partition zero has no more workload usage records in it and the table will be removed.
// The code in this module needs to be tolerant of this possibilty.
// IMPORTANT NOTE =====================================================================================================================

// agreements schema:
// agreement_id: The stringified agreement id for the agreement object in the record.
// protocol:     The agreement protocol in use. It is a way of partitioning the database so that an agbot can focus on handling
//               all agreements for a given protocol on at a time.
// partition:    The agbot partition that this agreement lives in. This is used to divide up ownership of agreements to specific agbot instances.
// agreement:    The agreement object which is a JSON blob. The blob schema is defined by the Agreement struct in the
//               persistence package.
// updated:      A timestamp to record last updated time.
//

/* When we migrate to postgresql 10, we can use these constant definitions because they will allow us to use
   the built in partitioning support.

const AGREEMENT_CREATE_MAIN_TABLE = `CREATE TABLE IF NOT EXISTS agreements (
	agreement_id text NOT NULL,
	protocol text NOT NULL,
	partition text NOT NULL,
	agreement jsonb NOT NULL,
	updated timestamp with time zone DEFAULT current_timestamp
) PARTITION BY LIST (partition);`

const AGREEMENT_CREATE_PARTITION_TABLE = `CREATE TABLE IF NOT EXISTS "agreements_ PARTITION OF agreements FOR VALUES IN ('partition_name');`
const AGREEMENT_TABLE_NAME_ROOT = `"agreements_`
const AGREEMENT_PARTITION_FILLIN = `partition_name`

const AGREEMENT_QUERY = `SELECT agreement FROM agreements WHERE agreement_id = $1 AND protocol = $2 AND partition = $3;`
const ALL_AGREEMENTS_QUERY = `SELECT agreement FROM agreements WHERE protocol = $1 AND partition = $2;`
const AGREEMENT_PARTITION_EMPTY = `SELECT agreement_id FROM agreements WHERE partition = $1;`

const AGREEMENT_COUNT = `SELECT COUNT(*) FROM "agreements_;`

const AGREEMENT_INSERT = `INSERT INTO agreements (agreement_id, protocol, partition, agreement) VALUES ($1, $2, $3, $4);`
const AGREEMENT_UPDATE = `UPDATE agreements SET agreement = $4, updated = current_timestamp WHERE agreement_id = $1 AND protocol = $2 AND partition = $3;`
const AGREEMENT_DELETE = `DELETE FROM agreements WHERE agreement_id = $1 AND partition = $2;`

const AGREEMENT_PARTITIONS = `SELECT partition FROM agreements;`

const AGREEMENT_DROP_PARTITION = `DROP TABLE "agreements_;`
*/

const AGREEMENT_CREATE_MAIN_TABLE = `CREATE TABLE IF NOT EXISTS agreements (
	agreement_id text NOT NULL,
	protocol text NOT NULL,
	partition text NOT NULL,
	agreement jsonb NOT NULL,
	updated timestamp with time zone DEFAULT current_timestamp
);`
const AGREEMENT_CREATE_PARTITION_TABLE = `CREATE TABLE IF NOT EXISTS "agreements_ (
	CHECK ( partition = 'partition_name' )
) INHERITS (agreements);`
const AGREEMENT_CREATE_PARTITION_INDEX = `CREATE INDEX IF NOT EXISTS "agreement_id_index_on_agreements_ ON "agreements_ (agreement_id);`

// Please note that the following SQL statement has a different syntax where the table name is specified. Note the use of
// single quotes instead of double quotes that are used in all the other SQL. Don't ya just love SQL syntax consistency.
const AGREEMENT_PARTITION_TABLE_EXISTS = `SELECT to_regclass('agreements_');`

const AGREEMENT_TABLE_NAME_ROOT = `agreements_`
const AGREEMENT_PARTITION_FILLIN = `partition_name`

const AGREEMENT_QUERY = `SELECT agreement FROM "agreements_ WHERE agreement_id = $1 AND protocol = $2;`
const ALL_AGREEMENTS_QUERY = `SELECT agreement FROM "agreements_ WHERE protocol = $1;`
const AGREEMENT_PARTITION_EMPTY = `SELECT agreement_id FROM "agreements_;`

const AGREEMENT_COUNT = `SELECT agreement FROM "agreements_;`

const AGREEMENT_INSERT = `INSERT INTO "agreements_ (agreement_id, protocol, partition, agreement) VALUES ($1, $2, $3, $4);`
const AGREEMENT_UPDATE = `UPDATE "agreements_ SET agreement = $3, updated = current_timestamp WHERE agreement_id = $1 AND protocol = $2;`
const AGREEMENT_DELETE = `DELETE FROM "agreements_ WHERE agreement_id = $1;`

const AGREEMENT_MOVE = `WITH moved_rows AS (
    DELETE FROM "agreements_ a
    RETURNING a.agreement_id, a.protocol, a.agreement
)
INSERT INTO "agreements_ (agreement_id, protocol, partition, agreement) SELECT agreement_id, protocol, 'partition_name', agreement FROM moved_rows;
`

const AGREEMENT_PARTITIONS = `SELECT partition FROM agreements;`

const AGREEMENT_DROP_PARTITION = `DROP TABLE "agreements_;`

// The fields in this object are initialized in the Initialize method in this package.
type AgbotPostgresqlDB struct {
	identity         string   // The identity of this agbot in the partitions table.
	db               *sql.DB  // A handle to the underlying database.
	primaryPartition string   // The partition to use when creating new agreements.
	partitions       []string // The list of partitions this agbot is responsible to maintain.
}

func (db *AgbotPostgresqlDB) String() string {
	return fmt.Sprintf("Instance: %v, PrimaryPartition: %v, All Partitions: %v, DB Handle: %v", db.identity, db.primaryPartition, db.partitions, db.db)
}

func (db *AgbotPostgresqlDB) PrimaryPartition() string {
	return db.primaryPartition
}

func (db *AgbotPostgresqlDB) AllPartitions() []string {
	return db.partitions
}

func (db *AgbotPostgresqlDB) GetAgreementPartitionTableName(partition string) string {
	return AGREEMENT_TABLE_NAME_ROOT + partition + `"`
}

func (db *AgbotPostgresqlDB) GetPrimaryAgreementPartitionTableCreate() string {
	sql := strings.Replace(AGREEMENT_CREATE_PARTITION_TABLE, AGREEMENT_TABLE_NAME_ROOT, db.GetAgreementPartitionTableName(db.PrimaryPartition()), 1)
	sql = strings.Replace(sql, AGREEMENT_PARTITION_FILLIN, db.PrimaryPartition(), 1)
	return sql
}

func (db *AgbotPostgresqlDB) GetPrimaryAgreementPartitionTableIndexCreate() string {
	sql := strings.Replace(AGREEMENT_CREATE_PARTITION_INDEX, AGREEMENT_TABLE_NAME_ROOT, db.GetAgreementPartitionTableName(db.PrimaryPartition()), 2)
	return sql
}

func (db *AgbotPostgresqlDB) GetAgreementPartitionTableDrop(partition string) string {
	sql := strings.Replace(AGREEMENT_DROP_PARTITION, AGREEMENT_TABLE_NAME_ROOT, db.GetAgreementPartitionTableName(partition), 1)
	return sql
}

func (db *AgbotPostgresqlDB) GetAgreementPartitionTableCount(partition string) string {
	sql := strings.Replace(AGREEMENT_COUNT, AGREEMENT_TABLE_NAME_ROOT, db.GetAgreementPartitionTableName(partition), 1)
	return sql
}

// The SQL template used by this function is slightly different than the others and therefore does it's own calculation
// of how the table partition is substituted into the SQL. The difference is in the required use of single quotes.
func (db *AgbotPostgresqlDB) GetAgreementPartitionTableExists(partition string) string {
	sql := strings.Replace(AGREEMENT_PARTITION_TABLE_EXISTS, AGREEMENT_TABLE_NAME_ROOT, AGREEMENT_TABLE_NAME_ROOT+partition, 1)
	return sql
}

// The partition table name replacement scheme used in this function is slightly different from the others above.
func (db *AgbotPostgresqlDB) GetAgreementPartitionMove(fromPartition string, toPartition string) string {
	sql := strings.Replace(AGREEMENT_MOVE, AGREEMENT_TABLE_NAME_ROOT, db.GetAgreementPartitionTableName(toPartition), 2)
	sql = strings.Replace(sql, db.GetAgreementPartitionTableName(toPartition), db.GetAgreementPartitionTableName(fromPartition), 1)
	sql = strings.Replace(sql, AGREEMENT_PARTITION_FILLIN, toPartition, 1)
	return sql
}

func (db *AgbotPostgresqlDB) FindAgreementPartitions() ([]string, error) {

	// Find all the agreement partitions.
	partitions := make([]string, 0, 10)
	foundPrimary := false

	rows, err := db.db.Query(AGREEMENT_PARTITIONS)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("error querying for agreement partitions: %v", err))
	}

	// If the rows object doesnt get closed, memory and connections will grow and/or leak.
	defer rows.Close()
	for rows.Next() {
		var partition string
		if err := rows.Scan(&partition); err != nil {
			return nil, errors.New(fmt.Sprintf("error scanning row: %v", err))
		} else {
			partitions = append(partitions, partition)
			if partition == db.PrimaryPartition() {
				foundPrimary = true
			}
		}
	}

	// The rows.Next() function will exit with false when done or an error occurred. Get any error encountered during iteration.
	if err = rows.Err(); err != nil {
		return nil, errors.New(fmt.Sprintf("error iterating: %v", err))
	}

	// Make sure the primary partition appears (even if it doesnt have any agreements yet), if it has not already been added
	if !foundPrimary {
		partitions = append(partitions, db.PrimaryPartition())
	}

	return partitions, nil
}

func (db *AgbotPostgresqlDB) GetAgreementCount(partition string) (int64, int64, error) {

	var activeNum, archivedNum int64

	rows, err := db.db.Query(db.GetAgreementPartitionTableCount(partition))
	if err != nil {
		return 0, 0, errors.New(fmt.Sprintf("error getting rows for agreement counts, error: %v", err))
	}

	// If the rows object doesnt get closed, memory and connections will grow and/or leak.
	defer rows.Close()
	for rows.Next() {
		agBytes := make([]byte, 0, 2048)
		ag := new(persistence.Agreement)
		if err := rows.Scan(&agBytes); err != nil {
			return 0, 0, errors.New(fmt.Sprintf("error scanning row for agreement counts: %v", err))
		} else if err := json.Unmarshal(agBytes, ag); err != nil {
			return 0, 0, errors.New(fmt.Sprintf("error demarshalling row for agreement count: %v, error: %v", string(agBytes), err))
		} else if ag.Archived {
			archivedNum += 1
		} else {
			activeNum += 1
		}
	}

	// The rows.Next() function will exit with false when done or an error occurred. Get any error encountered during iteration.
	if err = rows.Err(); err != nil {
		return 0, 0, errors.New(fmt.Sprintf("error iterating rows for agreement counts: %v", err))
	}

	return activeNum, archivedNum, nil
}

// Retrieve all agreements from the database and filter them out based on the input filters.
func (db *AgbotPostgresqlDB) FindAgreements(filters []persistence.AFilter, protocol string) ([]persistence.Agreement, error) {

	ags := make([]persistence.Agreement, 0, 100)

	for _, currentPartition := range db.AllPartitions() {
		// Find all the agreement objects, read them in and run them through the filters (after unmarshalling the blob into an
		// in memory agreement object).
		sql := strings.Replace(ALL_AGREEMENTS_QUERY, AGREEMENT_TABLE_NAME_ROOT, db.GetAgreementPartitionTableName(currentPartition), 1)
		if glog.V(5) {
			glog.Infof("Find agreements using SQL: %v for partition %v", sql, currentPartition)
		}
		rows, err := db.db.Query(sql, protocol)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("error querying for agreements error: %v", err))
		}

		// If the rows object doesnt get closed, memory and connections will grow and/or leak.
		defer rows.Close()
		for rows.Next() {
			agBytes := make([]byte, 0, 2048)
			ag := new(persistence.Agreement)
			if err := rows.Scan(&agBytes); err != nil {
				return nil, errors.New(fmt.Sprintf("error scanning row: %v", err))
			} else if err := json.Unmarshal(agBytes, ag); err != nil {
				return nil, errors.New(fmt.Sprintf("error demarshalling row: %v, error: %v", string(agBytes), err))
			} else {
				if !ag.Archived {
					if glog.V(5) {
						glog.Infof("Demarshalled agreement in partition %v from DB: %v", currentPartition, ag)
					}
				}
				if agPassed := persistence.RunFilters(ag, filters); agPassed != nil {
					ags = append(ags, *ag)
				}
			}
		}

		// The rows.Next() function will exit with false when done or an error occurred. Get any error encountered during iteration.
		if err = rows.Err(); err != nil {
			return nil, errors.New(fmt.Sprintf("error iterating: %v", err))
		}
	}

	return ags, nil

}

// Find a specific agreement in the database. The input filters are ignored for this query. They are needed by the bolt implementation.
func (db *AgbotPostgresqlDB) internalFindSingleAgreementByAgreementId(tx *sql.Tx, agreementId string, protocol string, filters []persistence.AFilter) (*persistence.Agreement, string, error) {

	agBytes := make([]byte, 0, 2048)
	ag := new(persistence.Agreement)

	for _, currentPartition := range db.AllPartitions() {

		// Find the agreement row and read in the agreement object column, run the returned agreement through the filters, then unmarshal
		// the blob into an in memory agreement object which gets returned to the caller.
		var qerr error
		sqlStr := strings.Replace(AGREEMENT_QUERY, AGREEMENT_TABLE_NAME_ROOT, db.GetAgreementPartitionTableName(currentPartition), 1)
		if tx == nil {
			qerr = db.db.QueryRow(sqlStr, agreementId, protocol).Scan(&agBytes)
		} else {
			qerr = tx.QueryRow(sqlStr, agreementId, protocol).Scan(&agBytes)
		}

		if qerr != nil && qerr != sql.ErrNoRows {
			return nil, "", errors.New(fmt.Sprintf("error scanning row for agreement %v error: %v", agreementId, qerr))
		} else if qerr == sql.ErrNoRows {
			continue
		}

		if err := json.Unmarshal(agBytes, ag); err != nil {
			return nil, "", errors.New(fmt.Sprintf("error demarshalling row: %v, error: %v", string(agBytes), err))
		} else if agPassed := persistence.RunFilters(ag, filters); agPassed == nil {
			return nil, "", nil // Agreement ids are unique. If we found the one we want but the filters rejected it, then we're done. No need to look at more partitions.
		} else {
			return ag, currentPartition, nil
		}
	}
	return nil, "", nil

}

func (db *AgbotPostgresqlDB) FindSingleAgreementByAgreementId(agreementId string, protocol string, filters []persistence.AFilter) (*persistence.Agreement, error) {
	ag, _, err := db.internalFindSingleAgreementByAgreementId(nil, agreementId, protocol, filters)
	return ag, err
}

func (db *AgbotPostgresqlDB) FindSingleAgreementByAgreementIdAllProtocols(agreementid string, protocols []string, filters []persistence.AFilter) (*persistence.Agreement, error) {
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

func (db *AgbotPostgresqlDB) AgreementAttempt(agreementid string, org string, deviceid string, deviceType string, policyName string, bcType string, bcName string, bcOrg string, agreementProto string, pattern string, serviceId []string, nhPolicy policy.NodeHealth, protocolTimeout uint64, agreementTimeout uint64) error {
	if agreement, err := persistence.NewAgreement(agreementid, org, deviceid, deviceType, policyName, bcType, bcName, bcOrg, agreementProto, pattern, serviceId, nhPolicy, protocolTimeout, agreementTimeout); err != nil {
		return err
	} else if err := db.insertAgreement(agreement, agreementProto); err != nil {
		return err
	} else {
		return nil
	}
}

func (db *AgbotPostgresqlDB) AgreementFinalized(agreementId string, protocol string) (*persistence.Agreement, error) {
	return persistence.AgreementFinalized(db, agreementId, protocol)
}

func (db *AgbotPostgresqlDB) AgreementUpdate(agreementid string, proposal string, policy string, dvPolicy policy.DataVerification, defaultCheckRate uint64, hash string, sig string, protocol string, agreementProtoVersion int) (*persistence.Agreement, error) {
	return persistence.AgreementUpdate(db, agreementid, proposal, policy, dvPolicy, defaultCheckRate, hash, sig, protocol, agreementProtoVersion)
}

func (db *AgbotPostgresqlDB) AgreementMade(agreementId string, counterParty string, signature string, protocol string, bcType string, bcName string, bcOrg string) (*persistence.Agreement, error) {
	return persistence.AgreementMade(db, agreementId, counterParty, signature, protocol, bcType, bcName, bcOrg)
}

func (db *AgbotPostgresqlDB) AgreementTimedout(agreementid string, protocol string) (*persistence.Agreement, error) {
	return persistence.AgreementTimedout(db, agreementid, protocol)
}

func (db *AgbotPostgresqlDB) AgreementBlockchainUpdate(agreementId string, consumerSig string, hash string, counterParty string, signature string, protocol string) (*persistence.Agreement, error) {
	return persistence.AgreementBlockchainUpdate(db, agreementId, consumerSig, hash, counterParty, signature, protocol)
}

func (db *AgbotPostgresqlDB) AgreementBlockchainUpdateAck(agreementId string, protocol string) (*persistence.Agreement, error) {
	return persistence.AgreementBlockchainUpdateAck(db, agreementId, protocol)
}

func (db *AgbotPostgresqlDB) DataVerified(agreementid string, protocol string) (*persistence.Agreement, error) {
	return persistence.DataVerified(db, agreementid, protocol)
}

func (db *AgbotPostgresqlDB) DataNotVerified(agreementid string, protocol string) (*persistence.Agreement, error) {
	return persistence.DataNotVerified(db, agreementid, protocol)
}

func (db *AgbotPostgresqlDB) DataNotification(agreementid string, protocol string) (*persistence.Agreement, error) {
	return persistence.DataNotification(db, agreementid, protocol)
}

func (db *AgbotPostgresqlDB) MeteringNotification(agreementid string, protocol string, mn string) (*persistence.Agreement, error) {
	return persistence.MeteringNotification(db, agreementid, protocol, mn)
}

func (db *AgbotPostgresqlDB) ArchiveAgreement(agreementid string, protocol string, reason uint, desc string) (*persistence.Agreement, error) {
	return persistence.ArchiveAgreement(db, agreementid, protocol, reason, desc)
}

func (db *AgbotPostgresqlDB) AgreementSecretUpdateTime(agreementid string, protocol string, secretUpdateTime uint64) (*persistence.Agreement, error) {
	return persistence.AgreementSecretUpdateTime(db, agreementid, protocol, secretUpdateTime)
}

func (db *AgbotPostgresqlDB) AgreementSecretUpdateAckTime(agreementid string, protocol string, secretUpdateAckTime uint64) (*persistence.Agreement, error) {
	return persistence.AgreementSecretUpdateAckTime(db, agreementid, protocol, secretUpdateAckTime)
}

func (db *AgbotPostgresqlDB) DeleteAgreement(agreementid string, protocol string) error {
	tx, err := db.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := db.deleteAgreement(tx, agreementid, protocol); err != nil {
		return err
	} else {
		return tx.Commit()
	}
}

func (db *AgbotPostgresqlDB) Close() {
	glog.V(2).Infof("Closing Postgresql database")
	db.db.Close()
	glog.V(2).Infof("Closed Postgresql database")
}

// Utility functions used by the public functions in this package.

// This function is used by all functions that want to change something in the database. It first locates the agreement
// to be updated (the query is done in it's own transaction), then calls the input function to update the agreement in
// memory, and finally calls wrapTransaction to start a transaction that will actually perform the update.
func (db *AgbotPostgresqlDB) SingleAgreementUpdate(agreementid string, protocol string, fn func(persistence.Agreement) *persistence.Agreement) (*persistence.Agreement, error) {
	if agreement, err := db.FindSingleAgreementByAgreementId(agreementid, protocol, []persistence.AFilter{}); err != nil {
		return nil, err
	} else if agreement == nil {
		return nil, errors.New(fmt.Sprintf("unable to locate agreement id: %v", agreementid))
	} else {
		updated := fn(*agreement)
		return updated, db.wrapTransaction(agreementid, protocol, updated)
	}
}

// This function is used to wrap a database transaction around an update to an agreement object.
func (db *AgbotPostgresqlDB) wrapTransaction(agreementid string, protocol string, updated *persistence.Agreement) error {

	if tx, err := db.db.Begin(); err != nil {
		return err
	} else if err := db.persistUpdatedAgreement(tx, agreementid, protocol, updated); err != nil {
		tx.Rollback()
		return err
	} else {
		return tx.Commit()
	}

}

// This function runs inside a transaction. It will atomicly read the agreement from the DB, verify that the updated
// agreement object contains valid state transitions, and then write the updated agreement back to the database.
func (db *AgbotPostgresqlDB) persistUpdatedAgreement(tx *sql.Tx, agreementid string, protocol string, update *persistence.Agreement) error {

	if mod, partition, err := db.internalFindSingleAgreementByAgreementId(tx, agreementid, protocol, []persistence.AFilter{}); err != nil {
		return err
	} else if mod == nil {
		return errors.New(fmt.Sprintf("No agreement with given id available to update: %v", agreementid))
	} else {
		// This code is running in a database transaction. Within the tx, the current record (mod) is
		// read and then updated according to the updates within the input update record. It is critical
		// to check for correct data transitions within the tx.
		persistence.ValidateStateTransition(mod, update)
		return db.updateAgreement(tx, mod, protocol, partition)
	}
}

func (db *AgbotPostgresqlDB) insertAgreement(ag *persistence.Agreement, protocol string) error {

	sql := strings.Replace(AGREEMENT_INSERT, AGREEMENT_TABLE_NAME_ROOT, db.GetAgreementPartitionTableName(db.PrimaryPartition()), 1)

	if agm, err := json.Marshal(ag); err != nil {
		return err
	} else if _, err = db.db.Exec(sql, ag.CurrentAgreementId, protocol, db.PrimaryPartition(), agm); err != nil {
		return err
	} else {
		glog.V(2).Infof("Succeeded creating agreement record %v", *ag)
	}

	return nil
}

func (db *AgbotPostgresqlDB) updateAgreement(tx *sql.Tx, ag *persistence.Agreement, protocol string, partition string) error {

	sql := strings.Replace(AGREEMENT_UPDATE, AGREEMENT_TABLE_NAME_ROOT, db.GetAgreementPartitionTableName(partition), 1)

	if agm, err := json.Marshal(ag); err != nil {
		return err
	} else if _, err = tx.Exec(sql, ag.CurrentAgreementId, protocol, agm); err != nil {
		return err
	} else {
		glog.V(2).Infof("Succeeded writing agreement record %v", *ag)
	}

	return nil
}

func (db *AgbotPostgresqlDB) deleteAgreement(tx *sql.Tx, agreementId string, protocol string) error {

	// Query the agreement id to retrieve the partition for this agreement. We dont need the agreement object in this case.
	// Compare the agreement's partition with the DB's primary and if they are different, delete this agreement and then
	// check to see if the partition specific table is now empty.

	checkTableDeletion := false
	_, partition, err := db.internalFindSingleAgreementByAgreementId(tx, agreementId, protocol, []persistence.AFilter{})
	if err != nil {
		return err
	} else if partition != db.PrimaryPartition() {
		checkTableDeletion = true
	}

	// Delete the agreement.
	sql := strings.Replace(AGREEMENT_DELETE, AGREEMENT_TABLE_NAME_ROOT, db.GetAgreementPartitionTableName(partition), 1)
	if _, err := tx.Exec(sql, agreementId); err != nil {
		return err
	}

	if glog.V(5) {
		glog.Infof("Agreement %v deleted from database.", agreementId)
	}

	// Remove the secondary partition table if necessary.
	if checkTableDeletion {
		sql := strings.Replace(AGREEMENT_PARTITION_EMPTY, AGREEMENT_TABLE_NAME_ROOT, db.GetAgreementPartitionTableName(partition), 1)
		rows, err := tx.Query(sql)
		if err != nil {
			return errors.New(fmt.Sprintf("error querying for empty agreement partition %v error: %v", partition, err))
		}

		// If the rows object doesnt get closed, memory and connections will grow and/or leak.
		defer rows.Close()

		// If there are no rows then this table is empty and should be deleted.
		if !rows.Next() {
			glog.V(5).Infof("Deleting secondary agreement partition %v from database.", partition)
			if _, err := tx.Exec(db.GetAgreementPartitionTableDrop(partition)); err != nil {
				return err
			}
			glog.V(5).Infof("Deleted secondary agreement partition %v from database.", partition)
		}
	}

	return nil

}
