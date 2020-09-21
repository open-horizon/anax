package postgresql

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/golang/glog"
	_ "github.com/lib/pq"
	"github.com/open-horizon/anax/config"
	"github.com/satori/go.uuid"
)

// This function is called by the anax main to allow the configured database a chance to initialize itself.
// This function is called every time the agbot starts, so it has to handle the following cases:
// - Nothing exists in the database
// - The database contains structures with schema that are not at the latest version
// - The database is completely up to date WRT the schemas
func (db *AgbotPostgresqlDB) Initialize(cfg *config.HorizonConfig) error {

	connectInfo, trace := cfg.AgreementBot.Postgresql.MakeConnectionString()

	glog.V(1).Infof("Connecting to Postgresql database: %v", trace)

	if pgdb, err := sql.Open("postgres", connectInfo); err != nil {
		return errors.New(fmt.Sprintf("unable to open Postgresql database, error: %v", err))
	} else if err := pgdb.Ping(); err != nil {
		return errors.New(fmt.Sprintf("unable to ping Postgresql database, error: %v", err))
	} else {
		db.db = pgdb

		// Set the max open connections
		db.db.SetMaxOpenConns(cfg.AgreementBot.Postgresql.MaxOpenConnections)

		// Initialize the DB instance fields.
		if id, err := uuid.NewV4(); err != nil {
			return errors.New(fmt.Sprintf("unable to get UUID identity for this agbot, error: %v", err))
		} else {
			db.identity = id.String()
		}
		glog.V(1).Infof("Agreementbot %v initializing partitions", db.identity)

		// Now create the tables and initialize them as necessary.
		glog.V(3).Infof("Postgresql database tables initializing.")

		// Create the version table if necessary, and insert the current version row if necessary.
		if _, err := db.db.Exec(VERSION_CREATE_TABLE); err != nil {
			return errors.New(fmt.Sprintf("unable to create version table, error: %v", err))
		} else if _, err := db.db.Exec(VERSION_INSERT); err != nil {
			return errors.New(fmt.Sprintf("unable to insert singleton version row, error: %v", err))
		}

		// Create the search session table if necessary, and initialize the stored procedure functions.
		if _, err := db.db.Exec(SEARCH_SESSIONS_CREATE_MAIN_TABLE); err != nil {
			return errors.New(fmt.Sprintf("unable to create search session table, error: %v", err))
		} else if _, err := db.db.Exec(SEARCH_SESSIONS_UPDATE_SESSION); err != nil {
			return errors.New(fmt.Sprintf("unable to create search session update function, error: %v", err))
		} else if _, err := db.db.Exec(SEARCH_SESSIONS_RESET_CHANGED_SINCE); err != nil {
			return errors.New(fmt.Sprintf("unable to create search session reset function, error: %v", err))
		}

		// Create the partition tables and create the postgresql procedure that manages the table.
		if _, err := db.db.Exec(PARTITION_CREATE_MAIN_TABLE); err != nil {
			return errors.New(fmt.Sprintf("unable to create partition table, error: %v", err))
		} else if _, err := db.db.Exec(PARTITION_CLAIM_UNOWNED_FUNCTION); err != nil {
			return errors.New(fmt.Sprintf("unable to create claim unowned partition function, error: %v", err))
		}

		// Claim a partition for ourselves.
		if partition, err := db.ClaimPartition(cfg.GetPartitionStale()); err != nil {
			return errors.New(fmt.Sprintf("unable to claim a partition, error: %v", err))
		} else {
			db.primaryPartition = partition
			db.partitions = append(db.partitions, partition)
		}

		// Create the workload usage table, partition and index if necessary.
		if _, err := db.db.Exec(WORKLOAD_USAGE_CREATE_MAIN_TABLE); err != nil {
			return errors.New(fmt.Sprintf("unable to create workload usage table, error: %v", err))
		} else if _, err := db.db.Exec(db.GetPrimaryWorkloadUsagePartitionTableCreate()); err != nil {
			return errors.New(fmt.Sprintf("unable to create workload usage partition table, error: %v", err))
		} else if _, err := db.db.Exec(db.GetPrimaryWorkloadUsagePartitionTableIndexCreate()); err != nil {
			return errors.New(fmt.Sprintf("unable to create workload usage partition table index, error: %v", err))
		}

		// Create the agreement table, partition and index if necessary.
		if _, err := db.db.Exec(AGREEMENT_CREATE_MAIN_TABLE); err != nil {
			return errors.New(fmt.Sprintf("unable to create agreements table, error: %v", err))
		} else if _, err := db.db.Exec(db.GetPrimaryAgreementPartitionTableCreate()); err != nil {
			return errors.New(fmt.Sprintf("unable to create agreements partition table, error: %v", err))
		} else if _, err := db.db.Exec(db.GetPrimaryAgreementPartitionTableIndexCreate()); err != nil {
			return errors.New(fmt.Sprintf("unable to create agreements partition table index, error: %v", err))
		}

		glog.V(3).Infof("Postgresql primary partition database tables exist.")

		// Migrate the database tables if necessary. Extract the current schema version from the version table,
		// and then run each version's migration SQL to bring the database up to the current version supported
		// by this code.
		var dbVersion int
		var description string
		var timestamp string
		if err := db.db.QueryRow(VERSION_QUERY).Scan(&dbVersion, &description, &timestamp); err != nil {
			return errors.New(fmt.Sprintf("error scanning row for current version, error: %v", err))
		} else {
			glog.V(3).Infof("Postgresql database tables are at version %v, %v, as of %v.", dbVersion, description, timestamp)
		}

		if dbVersion < HIGHEST_DATABASE_VERSION {
			glog.V(3).Infof("Postgresql database tables upgrading from version %v to %v.", dbVersion, HIGHEST_DATABASE_VERSION)

			// Each new database version has it's own key in the migration SQL map.
			for v := dbVersion + 1; v <= HIGHEST_DATABASE_VERSION; v++ {

				// Run each SQL statement in the array of SQL statements for the current verion.
				for si := 0; si < len(migrationSQL[v].sql); si++ {
					if _, err := db.db.Exec(migrationSQL[v].sql[si]); err != nil {
						return errors.New(fmt.Sprintf("unable to run SQL migration statement version %v, index %v, statement %v, error: %v", v, si, migrationSQL[v].sql[si], err))
					} else if _, err := db.db.Exec(VERSION_UPDATE, HIGHEST_DATABASE_VERSION, migrationSQL[v].description); err != nil {
						return errors.New(fmt.Sprintf("unable to create version table, error: %v", err))
					} else {
						glog.V(3).Infof("Postgresql database tables upgraded to version %v, %v", v, migrationSQL[v].description)
					}
				}
			}

			glog.V(3).Infof("Postgresql database tables upgraded to version %v", HIGHEST_DATABASE_VERSION)
		}

		glog.V(3).Infof("Postgresql database tables initialized.")

	}
	return nil

}
