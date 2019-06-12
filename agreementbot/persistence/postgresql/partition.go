package postgresql

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/golang/glog"
)

// Constants for the SQL statements that are used to work with partitions. Each agbot owns a single partition. Each agbot has
// an instance id (uuid) that it uses only once and only when it's running. When an agbot starts, it always creates a new identity
// for itself. Clustered agbots each have the same Horizon exchange identity, therefore they each need a unique identifier to
// indicate ownership of a partition. Agbots also periodically scan the partition table looking for partitions that are no longer
// being used by an agbot. There are 2 times when this can occur, when an agbot quiesces or when it terminates suddenly and
// unexpectedly. In either case, running agbots periodically take ownership of unowned partitions and move those agreements into
// its own partition. Move is meant literally, the database records are deleted form the old partition tables and inserted into
// the current partition, then the old partition tables are dropped. How does an agbot detect that another agbot has terminated
// and is no longer using its partition? The agbot is configured with a "stale" timeout. When a partition is not heartbeated
// within the "stale" timeout time, the partition is considered stale and can be taken over by another agbot.
//
// Postgresql supports automatic partitioning, which is what we want to use but cannot because Postgresql 10 is too new. Instead
// we have implemented our own partitioning scheme using table inheritance for agreement related records. This table keeps track of
// which partitions exist and who owns them if any.
//
// partitions schema:
// id:        The partition id, serially incremented by the database when a new partition is created.
// owner:     The UUID of the agbot that owns this partition. NULL means that the previous owner quiesced so the partition is
//            available to be taken over immediately.
// heartbeat: A timestamp to record last heartbeat time. If the owning agbot stops heartbeating, the partition becomes eligible to
//            be taken over by another agbot.
//

const PARTITION_CREATE_MAIN_TABLE = `CREATE TABLE IF NOT EXISTS partitions (
	id SERIAL PRIMARY KEY,
	owner text,
	heartbeat timestamp with time zone
);`

const PARTITION_OWNER = `SELECT owner FROM partitions WHERE id = $1;`

const PARTITION_INSERT = `INSERT INTO partitions (owner, heartbeat) VALUES ($1,current_timestamp) RETURNING id, owner;`

const PARTITION_HEARTBEAT = `UPDATE partitions SET heartbeat = current_timestamp WHERE id = $1 AND owner = $2;`

const PARTITION_QUIESCE = `UPDATE partitions SET owner = NULL, heartbeat = NULL WHERE owner = $1;`

const PARTITION_DELETE = `DELETE FROM partitions WHERE id = $1;`

// The complexity of the WHERE clause should not be underestimated. Each row is scanned whlie the table is locked
// so we are sure that no other agbot can even read this table until this query is complete. This query runs in a
// transaction that is controlled by the functions in this package.
const PARTITION_CLAIM_UNOWNED_FUNCTION = `
CREATE OR REPLACE FUNCTION claim_ownerless(
	new_owner CHARACTER VARYING,
	timeout int)
	RETURNS TABLE (
		retId int,
		retOwner text
	) AS $$
BEGIN
LOCK TABLE partitions;
RETURN QUERY
UPDATE partitions SET owner = new_owner, heartbeat = current_timestamp
	WHERE id = (
		SELECT id FROM partitions
			WHERE
				(owner IS NULL AND heartbeat IS NULL)
				OR
				(owner IS NOT NULL AND (
					SELECT EXTRACT ('epoch' FROM (SELECT AGE(current_timestamp, heartbeat)))
				) > timeout)
			LIMIT 1
			FOR UPDATE
		)
	RETURNING id, owner;
END $$ LANGUAGE plpgsql;
`
const PARTITION_CLAIM_UNOWNED_BY_FUNCTION = `SELECT * FROM claim_ownerless($1, $2);`

// Functions related to partitions in the postgresql database. The workload usages should always be using the same partitions
// as the agreements, or fewer partitions if an agreement partition contains only archived records.

// Look for an ownerless or stale partition. If none exist, create a new partition.
func (db *AgbotPostgresqlDB) ClaimPartition(timeout uint64) (string, error) {

	// Loop until we find a partition that we can take over or until we find out that we have to create one for ourselves.
	for {
		if unownedPartition, err := db.findUnownedPartition(timeout); err != nil {
			return "", errors.New(fmt.Sprintf("unable to claim an unowned partition, error: %v", err))
		} else if unownedPartition == "" {
			// There were no claimable partitions, so create a new partition.
			var id string
			var rowowner sql.NullString
			if err := db.db.QueryRow(PARTITION_INSERT, db.identity).Scan(&id, &rowowner); err != nil {
				return "", errors.New(fmt.Sprintf("AgreementBot %v unable to insert new partition, error: %v", rowowner, err))
			} else {
				glog.V(5).Infof("AgreementBot %v creating new partition %v", rowowner, id)
				return id, nil
			}
		} else {
			return unownedPartition, nil
		}
	}
}

func (db *AgbotPostgresqlDB) findUnownedPartition(timeout uint64) (string, error) {
	var id string
	var rowowner sql.NullString

	for {
		tx, err := db.db.Begin()
		if err != nil {
			return "", errors.New(fmt.Sprintf("unable to start transaction, error: %v", err))
		}
		defer tx.Rollback()

		if err := tx.QueryRow(PARTITION_CLAIM_UNOWNED_BY_FUNCTION, db.identity, timeout).Scan(&id, &rowowner); err != nil && err != sql.ErrNoRows {
			return "", errors.New(fmt.Sprintf("unable to claim stale, error: %v", err))
		} else if err == nil {
			// Nothing to do, we claimed a previously unowned row.
			if err := tx.Commit(); err != nil {
				return "", errors.New(fmt.Sprintf("unable to commit claim on unowned row, error: %v", err))
			}
			glog.Infof("AgreementBot %v claimed partition %v", rowowner, id)

			// Verify that the partition has tables that exist. If not, get rid of this partition and find a new one.
			if existingPartitions, err := db.VerifyPartitions([]string{id}); err != nil {
				glog.Errorf(fmt.Sprintf("AgreementBot unable to verify that partition %v tables exist, error: %v", id, err))
				// Delete partition
				if _, err := db.db.Exec(PARTITION_DELETE, id); err != nil {
					glog.Warningf("unable to remove partition %v, error: %v", id, err)
				}
				continue
			} else if existingPartitions[0] != id {
				glog.Errorf(fmt.Sprintf("AgreementBot chosen partition %v has no partition tables, retrying.", id))
				// Delete partition
				if _, err := db.db.Exec(PARTITION_DELETE, id); err != nil {
					glog.Warningf("unable to remove partition %v, error: %v", id, err)
				}
				continue
			}
			return id, nil
		} else {
			// The no rows error was returned, so there were no partitions to be claimed.
			return "", tx.Commit()
		}
	}
}

// Locate all the partitions currently found in the database, for all agbots.
func (db *AgbotPostgresqlDB) FindPartitions() ([]string, error) {

	if allPartitions, err := db.FindAgreementPartitions(); err != nil {
		return nil, err
	} else {
		return allPartitions, nil
	}

}

// Retrieve the partition owner for a given partition.
func (db *AgbotPostgresqlDB) GetPartitionOwner(id string) (string, error) {

	var owner sql.NullString
	if err := db.db.QueryRow(PARTITION_OWNER, id).Scan(&owner); err != nil {
		return "", errors.New(fmt.Sprintf("error scanning partition %v owner result, error: %v", id, err))
	} else if !owner.Valid {
		return "NO OWNER", nil
	} else {
		return owner.String, nil
	}

}

// If any partition is owned by this agbot where the table is missing for any of the persisted objects, then remove that
// partition from the database.
func (db *AgbotPostgresqlDB) VerifyPartitions(partitions []string) ([]string, error) {

	existingPartitions := make([]string, 0, 5)

	for _, partition := range partitions {

		var tableName []byte
		// This query always retuns a row in the result set. The returned table name is empty if the table does not exist.
		if err := db.db.QueryRow(db.GetAgreementPartitionTableExists(partition)).Scan(&tableName); err != nil {
			return nil, errors.New(fmt.Sprintf("error scanning result for agreement partition %v table check, error: %v", partition, err))
		} else if string(tableName) == "" {
			continue
		} else {
			existingPartitions = append(existingPartitions, partition)
		}

	}

	return existingPartitions, nil

}

// Update the hearbeat for our partition.
func (db *AgbotPostgresqlDB) HeartbeatPartition() error {

	if res, err := db.db.Exec(PARTITION_HEARTBEAT, db.PrimaryPartition(), db.identity); err != nil {
		return errors.New(fmt.Sprintf("AgreementBot %v unable to heartbeat, error: %v", db.identity, err))
	} else if num, err := res.RowsAffected(); err != nil {
		return errors.New(fmt.Sprintf("AgreementBot %v error getting rows affected, error: %v", db.identity, err))
	} else if num != 1 {
		return errors.New(fmt.Sprintf("AgreementBot %v, heartbeat update should have changed 1 row, but changed %v", db.identity, num))
	} else {
		glog.V(3).Infof("AgreementBot %v heartbeat", db.identity)
	}
	return nil
}

// Quiesce our partition.
func (db *AgbotPostgresqlDB) QuiescePartition() error {

	if _, err := db.db.Exec(PARTITION_QUIESCE, db.identity); err != nil {
		return errors.New(fmt.Sprintf("Agbot %v unable to quiesce partition, error: %v", db.identity, err))
	} else {
		glog.V(3).Infof("AgreementBot %v quiesced partition", db.identity)
	}
	return nil
}

// Move all records from one partition to another if there is a stale or unowned partition in the database.
func (db *AgbotPostgresqlDB) MovePartition(timeout uint64) error {

	if fromPartition, err := db.findUnownedPartition(timeout); err != nil {
		return err
	} else if fromPartition == "" {
		glog.V(3).Infof("AgreementBot %v did not find an unowned database partition.", db.identity)
		return nil
	} else {
		// We have found a partition and we have claimed it (in a transaction) so no other agbot can grab it now. Move all the
		// agreement related records in the partition into our primary partition, remove the partition tables and remove the partition
		// row from the partitions table. This is all done under a single transactions so that if the agbot were to terminate during
		// this time, another agbot will eventually claim this partition and attempt this same cleanup again.
		tx, err := db.db.Begin()
		if err != nil {
			return errors.New(fmt.Sprintf("unable to start transaction for moving agreements, error: %v", err))
		}
		defer tx.Rollback()

		if _, err := tx.Exec(db.GetAgreementPartitionMove(fromPartition, db.PrimaryPartition())); err != nil {
			return err
		} else if _, err := tx.Exec(db.GetWorkloadUsagePartitionMove(fromPartition, db.PrimaryPartition())); err != nil {
			return err
		} else if _, err := tx.Exec(db.GetAgreementPartitionTableDrop(fromPartition)); err != nil {
			return err
		} else if _, err := tx.Exec(db.GetWorkloadUsagePartitionTableDrop(fromPartition)); err != nil {
			return err
		} else if _, err := tx.Exec(PARTITION_DELETE, fromPartition); err != nil {
			return err
		} else {
			if err := tx.Commit(); err != nil {
				return errors.New(fmt.Sprintf("unable to commit transaction for moving agreements, error: %v", err))
			}
			glog.V(3).Infof("AgreementBot %v moved agreements from partition %v to %v", db.identity, fromPartition, db.PrimaryPartition())
		}
	}
	return nil
}
