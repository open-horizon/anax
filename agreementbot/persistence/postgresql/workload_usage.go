package postgresql

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/agreementbot/persistence"
	"strings"
)

// Constants for the SQL statements that are used to work with workload usages. These records are used to track what workload
// is running on each device so that we can do proper management of HA devices. Workload usages are partitioned by agbot instances.
// Each agbot instance "owns" 1 partition in the database. We are implementing our own partitioning scheme. There is a
// "main" table (called workload_usages) that defines the schema for all the partitions, and then there is a
// set of separate tables (called workload_usages_<partition_name>, one for each partition) which inherit from the main table and
// that we have to create. When an agbot comes up it will attempt to create the main table and a partition table for its primary
// partition. Our code will correctly route INSERTs/UPDATEs and SELECTs to the correct partition based on the partition value
// provided in the SQL statement. Postgresql allows you to query the main table for cases where the caller might not know the
// partition holding the record of interest. We exploit both forms of query.
//

// workload_usages schema:
// device_id:      The device's exchange id.
// policy_name:    The name of the policy that is placing this workload on the device.
// partition:      The agbot partition that this workload usage lives in. This is used to divide up ownership of worklaod usages to specific agbot instances.
// workload_usage: The worload_usage object which is a JSON blob. The blob schema is defined by the WorkloadUsage struct in the persistence package.
// updated:        A timestamp to record last updated time.
//

/* When we migrate to postgresql 10, we can use these constant definitions because they will allow us to use
   the built in partitioning support.
const WORKLOAD_USAGE_CREATE_MAIN_TABLE = `CREATE TABLE IF NOT EXISTS workload_usages (
	device_id text NOT NULL,
	policy_name text NOT NULL,
	partition text NOT NULL,
	workload_usage jsonb NOT NULL,
	updated timestamp with time zone DEFAULT current_timestamp
) PARTITION BY LIST (partition);`

const WORKLOAD_USAGE_CREATE_PARTITION_TABLE = `CREATE TABLE IF NOT EXISTS "workload_usages_ PARTITION OF workload_usages FOR VALUES IN ('partition_name');`
const WORKLOAD_USAGE_TABLE_NAME_ROOT = `"workload_usages_`
const WORKLOAD_USAGE_PARTITION_FILLIN = `partition_name`

const WORKLOAD_USAGE_QUERY = `SELECT workload_usage FROM workload_usages WHERE device_id = $1 AND policy_name = $2 AND partition = $3;`
const ALL_WORKLOAD_USAGE_QUERY = `SELECT workload_usage FROM workload_usages WHERE partition = $1;`

const WORKLOAD_USAGE_PARTITIONS = `SELECT partition FROM workload_usages;`

const WORKLOAD_USAGE_COUNT = `SELECT COUNT(*) FROM "workload_usages_;`

const WORKLOAD_USAGE_INSERT = `INSERT INTO workload_usages (device_id, policy_name, partition, workload_usage) VALUES ($1, $2, $3, $4);`
const WORKLOAD_USAGE_UPDATE = `UPDATE workload_usages SET workload_usage = $4, updated = current_timestamp WHERE device_id = $1 AND policy_name = $2 AND partition = $3;`
const WORKLOAD_USAGE_DELETE = `DELETE FROM workload_usages WHERE device_id = $1 AND policy_name = $2 AND partition = $3;`

const WORKLOAD_USAGE_DROP_PARTITION = `DROP TABLE "workload_usages_;`
*/

const WORKLOAD_USAGE_CREATE_MAIN_TABLE = `CREATE TABLE IF NOT EXISTS workload_usages (
	device_id text NOT NULL,
	policy_name text NOT NULL,
	partition text NOT NULL,
	workload_usage jsonb NOT NULL,
	updated timestamp with time zone DEFAULT current_timestamp
);`
const WORKLOAD_USAGE_CREATE_PARTITION_TABLE = `CREATE TABLE IF NOT EXISTS "workload_usages_ (
	CHECK ( partition = 'partition_name' )
) INHERITS (workload_usages);`
const WORKLOAD_USAGE_CREATE_PARTITION_INDEX = `CREATE INDEX IF NOT EXISTS "device_index_on_workload_usages_ ON "workload_usages_ (device_id, policy_name);`

// Please note that the following SQL statement has a different syntax where the table name is specified. Note the use of
// single quotes instead of double quotes that are used in all the other SQL. Don't ya just love SQL syntax consistency.
const WORKLOAD_USAGE_PARTITION_TABLE_EXISTS = `SELECT to_regclass('workload_usages_');`

const WORKLOAD_USAGE_TABLE_NAME_ROOT = `workload_usages_`
const WORKLOAD_USAGE_PARTITION_FILLIN = `partition_name`

const WORKLOAD_USAGE_QUERY = `SELECT workload_usage FROM "workload_usages_ WHERE device_id = $1 AND policy_name = $2;`
const ALL_WORKLOAD_USAGE_QUERY = `SELECT workload_usage FROM "workload_usages_;`

const WORKLOAD_USAGE_COUNT = `SELECT COUNT(*) FROM "workload_usages_;`

const WORKLOAD_USAGE_INSERT = `INSERT INTO "workload_usages_ (device_id, policy_name, partition, workload_usage) VALUES ($1, $2, $3, $4);`
const WORKLOAD_USAGE_UPDATE = `UPDATE "workload_usages_ SET workload_usage = $3, updated = current_timestamp WHERE device_id = $1 AND policy_name = $2;`
const WORKLOAD_USAGE_DELETE = `DELETE FROM "workload_usages_ WHERE device_id = $1 AND policy_name = $2;`

const WORKLOAD_USAGE_MOVE = `WITH moved_rows AS (
    DELETE FROM "workload_usages_ a
    RETURNING a.device_id, a.policy_name, a.workload_usage
)
INSERT INTO "workload_usages_ (device_id, policy_name, partition, workload_usage) SELECT device_id, policy_name, 'partition_name', workload_usage FROM moved_rows;
`

const WORKLOAD_USAGE_DROP_PARTITION = `DROP TABLE "workload_usages_;`

func (db *AgbotPostgresqlDB) GetWorkloadUsagePartitionTableName(partition string) string {
	return WORKLOAD_USAGE_TABLE_NAME_ROOT + partition + `"`
}

func (db *AgbotPostgresqlDB) GetPrimaryWorkloadUsagePartitionTableCreate() string {
	sql := strings.Replace(WORKLOAD_USAGE_CREATE_PARTITION_TABLE, WORKLOAD_USAGE_TABLE_NAME_ROOT, db.GetWorkloadUsagePartitionTableName(db.PrimaryPartition()), 1)
	sql = strings.Replace(sql, WORKLOAD_USAGE_PARTITION_FILLIN, db.PrimaryPartition(), 1)
	return sql
}

func (db *AgbotPostgresqlDB) GetPrimaryWorkloadUsagePartitionTableIndexCreate() string {
	sql := strings.Replace(WORKLOAD_USAGE_CREATE_PARTITION_INDEX, WORKLOAD_USAGE_TABLE_NAME_ROOT, db.GetWorkloadUsagePartitionTableName(db.PrimaryPartition()), 2)
	return sql
}

func (db *AgbotPostgresqlDB) GetWorkloadUsagePartitionTableDrop(partition string) string {
	sql := strings.Replace(WORKLOAD_USAGE_DROP_PARTITION, WORKLOAD_USAGE_TABLE_NAME_ROOT, db.GetWorkloadUsagePartitionTableName(partition), 1)
	return sql
}

func (db *AgbotPostgresqlDB) GetWorkloadUsagePartitionUsageTableCount(partition string) string {
	sql := strings.Replace(WORKLOAD_USAGE_COUNT, WORKLOAD_USAGE_TABLE_NAME_ROOT, db.GetWorkloadUsagePartitionTableName(partition), 1)
	return sql
}

// The SQL template used by this function is slightly different than the others and therefore does it's own calculation
// of how the table partition is substituted into the SQL. The difference is in the required use of single quotes.
func (db *AgbotPostgresqlDB) GetWorkloadUsagePartitionTableExists(partition string) string {
	sql := strings.Replace(WORKLOAD_USAGE_PARTITION_TABLE_EXISTS, WORKLOAD_USAGE_TABLE_NAME_ROOT, WORKLOAD_USAGE_TABLE_NAME_ROOT+partition, 1)
	return sql
}

// The partition table name replacement scheme used in this function is slightly different from the others above.
func (db *AgbotPostgresqlDB) GetWorkloadUsagePartitionMove(fromPartition string, toPartition string) string {
	sql := strings.Replace(WORKLOAD_USAGE_MOVE, WORKLOAD_USAGE_TABLE_NAME_ROOT, db.GetWorkloadUsagePartitionTableName(toPartition), 2)
	sql = strings.Replace(sql, db.GetWorkloadUsagePartitionTableName(toPartition), db.GetWorkloadUsagePartitionTableName(fromPartition), 1)
	sql = strings.Replace(sql, WORKLOAD_USAGE_PARTITION_FILLIN, toPartition, 1)
	return sql
}

// The partition table name replacement scheme used in this function is slightly different from the others above.
func (db *AgbotPostgresqlDB) GetWorkloadUsagesCount(partition string) (int64, error) {
	var num int64
	if err := db.db.QueryRow(db.GetWorkloadUsagePartitionUsageTableCount(partition)).Scan(&num); err != nil && err != sql.ErrNoRows && !strings.Contains(err.Error(), "not exist") {
		return 0, errors.New(fmt.Sprintf("error scanning result for workload usage count in partition %v, error: %v", partition, err))
	} else {
		return num, nil
	}
}

// Find the workload usage record, but constrain the search to partitions owned by this agbot.
func (db *AgbotPostgresqlDB) internalFindSingleWorkloadUsageByDeviceAndPolicyName(tx *sql.Tx, deviceid string, policyName string) (*persistence.WorkloadUsage, string, error) {

	wuBytes := make([]byte, 0, 2048)
	wu := new(persistence.WorkloadUsage)

	for _, currentPartition := range db.AllPartitions() {

		// Find the workload usage row and read in the workload usage object column, then unmarshal the blob into an
		// in memory workload usage object which gets returned to the caller.
		sqlStr := strings.Replace(WORKLOAD_USAGE_QUERY, WORKLOAD_USAGE_TABLE_NAME_ROOT, db.GetWorkloadUsagePartitionTableName(currentPartition), 1)
		var qerr error
		if tx == nil {
			qerr = db.db.QueryRow(sqlStr, deviceid, policyName).Scan(&wuBytes)
		} else {
			qerr = tx.QueryRow(sqlStr, deviceid, policyName).Scan(&wuBytes)
		}

		if qerr != nil && qerr != sql.ErrNoRows && !strings.Contains(qerr.Error(), "not exist") {
			return nil, "", errors.New(fmt.Sprintf("error scanning row for workload usage for device id %v and policy name %v, error: %v", deviceid, policyName, qerr))
		} else if qerr == sql.ErrNoRows || (qerr != nil && strings.Contains(qerr.Error(), "not exist")) {
			continue
		}

		if err := json.Unmarshal(wuBytes, wu); err != nil {
			return nil, "", errors.New(fmt.Sprintf("error demarshalling row: %v, error: %v", string(wuBytes), err))
		} else {
			return wu, currentPartition, nil
		}
	}
	// No records found.
	return nil, "", nil

}

func (db *AgbotPostgresqlDB) FindSingleWorkloadUsageByDeviceAndPolicyName(deviceid string, policyName string) (*persistence.WorkloadUsage, error) {
	wu, _, err := db.internalFindSingleWorkloadUsageByDeviceAndPolicyName(nil, deviceid, policyName)
	return wu, err
}

func (db *AgbotPostgresqlDB) FindWorkloadUsages(filters []persistence.WUFilter) ([]persistence.WorkloadUsage, error) {
	wus := make([]persistence.WorkloadUsage, 0, 100)

	for _, currentPartition := range db.AllPartitions() {

		// Find all the workload usage objects, read them in and run them through the filters (after unmarshalling the blob into an
		// in memory workload usage object).
		sqlStr := strings.Replace(ALL_WORKLOAD_USAGE_QUERY, WORKLOAD_USAGE_TABLE_NAME_ROOT, db.GetWorkloadUsagePartitionTableName(currentPartition), 1)
		rows, err := db.db.Query(sqlStr)
		if err != nil && strings.Contains(err.Error(), "not exist") {
			continue
		} else if err != nil {
			return nil, errors.New(fmt.Sprintf("error querying for workload usages, error: %v", err))
		}

		// If the rows object doesnt get closed, memory and connections will grow and/or leak.
		defer rows.Close()
		for rows.Next() {
			wuBytes := make([]byte, 0, 2048)
			wu := new(persistence.WorkloadUsage)
			if err := rows.Scan(&wuBytes); err != nil {
				return nil, errors.New(fmt.Sprintf("error scanning row: %v", err))
			} else if err := json.Unmarshal(wuBytes, wu); err != nil {
				return nil, errors.New(fmt.Sprintf("error demarshalling row: %v, error: %v", string(wuBytes), err))
			} else {
				exclude := false
				for _, filterFn := range filters {
					if !filterFn(*wu) {
						exclude = true
					}
				}
				if !exclude {
					wus = append(wus, *wu)
				}
			}
		}

		// The rows.Next() function will exit with false when done or an error occurred. Get any error encountered during iteration.
		if err = rows.Err(); err != nil {
			return nil, errors.New(fmt.Sprintf("error iterating: %v", err))
		}
	}

	return wus, nil
}

func (db *AgbotPostgresqlDB) NewWorkloadUsage(deviceId string, hapartners []string, policy string, policyName string, priority int, retryDurationS int, verifiedDurationS int, reqsNotMet bool, agid string) error {
	if wlUsage, err := persistence.NewWorkloadUsage(deviceId, hapartners, policy, policyName, priority, retryDurationS, verifiedDurationS, reqsNotMet, agid); err != nil {
		return err
	} else if existing, partition, err := db.internalFindSingleWorkloadUsageByDeviceAndPolicyName(nil, deviceId, policyName); err != nil {
		return err
	} else if existing != nil {
		return fmt.Errorf("Workload usage record for device %v and policy name %v already exists in partition %v.", deviceId, policyName, partition)
	} else if err := db.insertWorkloadUsage(nil, wlUsage); err != nil {
		return err
	} else {
		return nil
	}
}

func (db *AgbotPostgresqlDB) UpdatePendingUpgrade(deviceid string, policyName string) (*persistence.WorkloadUsage, error) {
	return persistence.UpdatePendingUpgrade(db, deviceid, policyName)
}

func (db *AgbotPostgresqlDB) UpdateRetryCount(deviceid string, policyName string, retryCount int, agid string) (*persistence.WorkloadUsage, error) {
	return persistence.UpdateRetryCount(db, deviceid, policyName, retryCount, agid)
}

func (db *AgbotPostgresqlDB) UpdatePriority(deviceid string, policyName string, priority int, retryDurationS int, verifiedDurationS int, agid string) (*persistence.WorkloadUsage, error) {
	return persistence.UpdatePriority(db, deviceid, policyName, priority, retryDurationS, verifiedDurationS, agid)
}

func (db *AgbotPostgresqlDB) UpdatePolicy(deviceid string, policyName string, pol string) (*persistence.WorkloadUsage, error) {
	return persistence.UpdatePolicy(db, deviceid, policyName, pol)
}

// Updating the agreement id in the existing record is easy. However, the record might be in the wrong partition. It is possible that
// the agbot was restarted with a new primary partition, and then the agreement that was using this record was cancelled and moved to
// the new primary partition. If that's the case, we need to mkae sure the workload usage record gets moved to the new partition also.
// This is where that will happen.
//
// Maybe you're asking yourself, why dont we just delete the workload usage record because it will get recreated in the new primary
// partition. The whole point of this record is that it retains state about which workload priority is being used in the agreement,
// which is what needs to be retained from one agreement to the next.
func (db *AgbotPostgresqlDB) UpdateWUAgreementId(deviceid string, policyName string, agid string, protocol string) (*persistence.WorkloadUsage, error) {

	// Get the partition of the workload usage record and the partition of the agreement. If they are different then we need to
	// move the workload usage record.
	if wlUsage, wlPartition, err := db.internalFindSingleWorkloadUsageByDeviceAndPolicyName(nil, deviceid, policyName); err != nil {
		return nil, err
	} else if _, agPartition, err := db.internalFindSingleAgreementByAgreementId(nil, agid, protocol, []persistence.AFilter{}); err != nil {
		return nil, err
	} else if wlPartition != agPartition {
		// Move the existing record to the new partition. Inserts are always done in the primary partition.
		if tx, err := db.db.Begin(); err != nil {
			return nil, err
		} else if err := db.deleteWU(tx, deviceid, policyName); err != nil {
			if err := tx.Rollback(); err != nil {
				glog.Errorf(fmt.Sprintf("Unable to rollback Workload Usage delete when moving to another partition, error %v", err))
			}
			return nil, err
		} else if err := db.insertWorkloadUsage(tx, wlUsage); err != nil {
			if err := tx.Rollback(); err != nil {
				glog.Errorf(fmt.Sprintf("Unable to rollback Workload Usage insert when moving to another partition, error %v", err))
			}
			return nil, err
		} else {
			if err := tx.Commit(); err != nil {
				return nil, errors.New(fmt.Sprintf("Unable to commit movement of workload usage record to new partition, error %v", err))
			}
		}
	}

	// Finally, update the agreement id in the workload usage object.
	return persistence.UpdateWUAgreementId(db, deviceid, policyName, agid)
}

func (db *AgbotPostgresqlDB) DisableRollbackChecking(deviceid string, policyName string) (*persistence.WorkloadUsage, error) {
	return persistence.DisableRollbackChecking(db, deviceid, policyName)
}

func (db *AgbotPostgresqlDB) DeleteWorkloadUsage(deviceid string, policyName string) error {
	tx, err := db.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := db.deleteWU(tx, deviceid, policyName); err != nil {
		return err
	} else {
		return tx.Commit()
	}
}

func (db *AgbotPostgresqlDB) SingleWorkloadUsageUpdate(deviceid string, policyName string, fn func(persistence.WorkloadUsage) *persistence.WorkloadUsage) (*persistence.WorkloadUsage, error) {
	if wlUsage, err := db.FindSingleWorkloadUsageByDeviceAndPolicyName(deviceid, policyName); err != nil {
		return nil, err
	} else if wlUsage == nil {
		return nil, fmt.Errorf("Unable to locate workload usage for device: %v, and policy: %v", deviceid, policyName)
	} else {
		updated := fn(*wlUsage)
		return updated, db.wrapWUTransaction(deviceid, policyName, updated)
	}
}

func (db *AgbotPostgresqlDB) wrapWUTransaction(deviceid string, policyName string, updated *persistence.WorkloadUsage) error {

	if tx, err := db.db.Begin(); err != nil {
		return err
	} else if err := db.persistUpdatedWorkloadUsage(tx, deviceid, policyName, updated); err != nil {
		tx.Rollback()
		return err
	} else {
		return tx.Commit()
	}

}

// This function runs inside a transaction. It will atomicly read the workload usage from the DB, verify that the updated
// workload usage object contains valid state transitions, and then write the updated workload usage back to the database.
func (db *AgbotPostgresqlDB) persistUpdatedWorkloadUsage(tx *sql.Tx, deviceid string, policyName string, update *persistence.WorkloadUsage) error {

	if mod, partition, err := db.internalFindSingleWorkloadUsageByDeviceAndPolicyName(tx, deviceid, policyName); err != nil {
		return err
	} else if mod == nil {
		return errors.New(fmt.Sprintf("No workload usage with device id %v and policy name %v available to update.", deviceid, policyName))
	} else {
		// This code is running in a database transaction. Within the tx, the current record (mod) is
		// read and then updated according to the updates within the input update record. It is critical
		// to check for correct data transitions within the tx.
		persistence.ValidateWUStateTransition(mod, update)
		return db.updateWorkloadUsage(tx, mod, partition)
	}
}

func (db *AgbotPostgresqlDB) insertWorkloadUsage(tx *sql.Tx, wu *persistence.WorkloadUsage) error {

	sqlStr := strings.Replace(WORKLOAD_USAGE_INSERT, WORKLOAD_USAGE_TABLE_NAME_ROOT, db.GetWorkloadUsagePartitionTableName(db.PrimaryPartition()), 1)

	if wum, err := json.Marshal(wu); err != nil {
		return err
	} else if tx == nil {
		if _, err = db.db.Exec(sqlStr, wu.DeviceId, wu.PolicyName, db.PrimaryPartition(), wum); err != nil {
			return err
		}
	} else {
		if _, err = tx.Exec(sqlStr, wu.DeviceId, wu.PolicyName, db.PrimaryPartition(), wum); err != nil {
			return err
		}
	}
	glog.V(2).Infof("Succeeded creating workload usage record %v", wu.ShortString())

	return nil
}

func (db *AgbotPostgresqlDB) updateWorkloadUsage(tx *sql.Tx, wu *persistence.WorkloadUsage, partition string) error {

	sqlStr := strings.Replace(WORKLOAD_USAGE_UPDATE, WORKLOAD_USAGE_TABLE_NAME_ROOT, db.GetWorkloadUsagePartitionTableName(partition), 1)

	if wum, err := json.Marshal(wu); err != nil {
		return err
	} else if _, err = tx.Exec(sqlStr, wu.DeviceId, wu.PolicyName, wum); err != nil {
		return err
	} else {
		glog.V(2).Infof("Succeeded writing workload usage record %v", wu.ShortString())
	}

	return nil
}

func (db *AgbotPostgresqlDB) deleteWU(tx *sql.Tx, deviceid string, policyName string) error {
	// Query the device id and policy name to retrieve the partition for this workload usage. Then compare the workload usage's
	// partition with the DB's primary and if they are different, delete this workload usage and then check to see if the
	// partition specific table is now empty. If so, delete it.

	checkTableDeletion := false
	wu, partition, err := db.internalFindSingleWorkloadUsageByDeviceAndPolicyName(tx, deviceid, policyName)
	if err != nil {
		return err
	} else if partition != "" && partition != db.PrimaryPartition() {
		checkTableDeletion = true
	}

	// Delete the workload usage if it's there.
	if wu != nil {
		sqlStr := strings.Replace(WORKLOAD_USAGE_DELETE, WORKLOAD_USAGE_TABLE_NAME_ROOT, db.GetWorkloadUsagePartitionTableName(partition), 1)
		if _, err := tx.Exec(sqlStr, deviceid, policyName); err != nil {
			return err
		}
		glog.V(5).Infof("Succeeded deleting workload usage for device %v and policy %v from database.", deviceid, policyName)
	}

	// Remove the secondary partition table if necessary.
	if checkTableDeletion {
		sqlStr := strings.Replace(ALL_WORKLOAD_USAGE_QUERY, WORKLOAD_USAGE_TABLE_NAME_ROOT, db.GetWorkloadUsagePartitionTableName(partition), 1)
		rows, err := tx.Query(sqlStr)
		if err != nil {
			return errors.New(fmt.Sprintf("error querying for empty workload usage partition %v error: %v", partition, err))
		}

		// If the rows object doesnt get closed, memory and connections will grow and/or leak.
		defer rows.Close()

		// If there are no rows then this table is empty and should be deleted.
		if !rows.Next() {
			glog.V(5).Infof("Deleting secondary workload usage partition %v from database.", partition)
			if _, err := tx.Exec(db.GetWorkloadUsagePartitionTableDrop(partition)); err != nil {
				return err
			}
			glog.V(5).Infof("Deleted secondary workload usage partition %v from database.", partition)
		}
	}

	return nil

}
