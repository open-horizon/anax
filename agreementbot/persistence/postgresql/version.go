package postgresql

import ()

// Constants for the SQL statements that are used to work with the database version. The entire database schema has a single
// version that is kept in the version table. Agbots automatically upgrade the database during initialization based on their version
// and the version in the database.

// version schema:
// ver:     The current version of the database schema.
// updated: A timestamp to record last updated time.
//
const VERSION_CREATE_TABLE = `CREATE TABLE IF NOT EXISTS version (
	id serial PRIMARY KEY,
	ver int NOT NULL,
	description text NOT NULL,
	updated timestamp with time zone DEFAULT current_timestamp
);`

const VERSION_QUERY = `SELECT ver, description, updated FROM version WHERE id = 1;`

// There should only be 1 row in this table.
const VERSION_INSERT = `DO $$
BEGIN
	IF NOT EXISTS (SELECT 1 FROM version WHERE id = 1) THEN
		INSERT INTO version (id, ver, description) VALUES (1, 0, 'initial tables');
	END IF;
END $$`

const VERSION_UPDATE = `UPDATE version SET ver = $1, description = $2, updated = current_timestamp WHERE id = 1;`

const HIGHEST_DATABASE_VERSION = v1
const v1 = 0

type SchemaUpdate struct {
	sql         []string // The SQL statements to run for an update to the schema.
	description string   // A description of the schema change.
}

var migrationSQL = map[int]SchemaUpdate{}
