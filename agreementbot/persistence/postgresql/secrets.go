package postgresql

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"strings"
)

// Constants for the SQL statements that are used to manage secret updates.

const SECRET_CREATE_MAIN_TABLE_POLICY = `CREATE TABLE IF NOT EXISTS secrets_policy (
	secret_org text NOT NULL,
	secret_name text NOT NULL,
	policy_org text NOT NULL,
	policy_name text NOT NULL,
	last_update_check int NOT NULL,
	partition text NOT NULL,
	updated timestamp with time zone DEFAULT current_timestamp
);`

const SECRET_CREATE_MAIN_TABLE_PATTERN = `CREATE TABLE IF NOT EXISTS secrets_pattern (
	secret_org text NOT NULL,
	secret_name text NOT NULL,
	pattern_org text NOT NULL,
	pattern_name text NOT NULL,
	last_update_check int NOT NULL,
	partition text NOT NULL,
	updated timestamp with time zone DEFAULT current_timestamp
);`

const SECRET_CREATE_PARTITION_TABLE_POLICY = `CREATE TABLE IF NOT EXISTS "secrets_policy_ (
	CHECK ( partition = 'partition_name' ),
	PRIMARY KEY (secret_org, secret_name, policy_org, policy_name)
) INHERITS (secrets_policy);`
const SECRET_CREATE_PARTITION_INDEX_POLICY = `CREATE INDEX IF NOT EXISTS "secret_index_on_secrets_policy_ ON "secrets_policy_ (secret_name);`

const SECRET_CREATE_PARTITION_TABLE_PATTERN = `CREATE TABLE IF NOT EXISTS "secrets_pattern_ (
	CHECK ( partition = 'partition_name' ),
	PRIMARY KEY (secret_org, secret_name, pattern_org, pattern_name)
) INHERITS (secrets_pattern);`
const SECRET_CREATE_PARTITION_INDEX_PATTERN = `CREATE INDEX IF NOT EXISTS "secret_index_on_secrets_pattern_ ON "secrets_pattern_ (secret_name);`

// Please note that the following SQL statement has a different syntax where the table name is specified. Note the use of
// single quotes instead of double quotes that are used in all the other SQL. Don't ya just love SQL syntax consistency.
const SECRET_PARTITION_TABLE_EXISTS_POLICY = `SELECT to_regclass('secrets_policy_');`
const SECRET_PARTITION_TABLE_EXISTS_PATTERN = `SELECT to_regclass('secrets_pattern_');`

const SECRET_TABLE_NAME_ROOT_POLICY = `secrets_policy_`
const SECRET_TABLE_NAME_ROOT_PATTERN = `secrets_pattern_`
const SECRET_PARTITION_FILLIN = `partition_name`

const SECRET_INSERT_POLICY = `INSERT INTO "secrets_policy_ (secret_org, secret_name, policy_org, policy_name, last_update_check, partition) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING;`
const SECRET_UPDATE_TIME_POLICY = `UPDATE "secrets_policy_ SET last_update_check = $1, updated = current_timestamp WHERE secret_org = $2 AND secret_name = $3;`
const SECRET_DELETE_POLICY = `DELETE FROM "secrets_policy_ WHERE secret_org = $1 AND secret_name = $2 AND policy_org = $3 AND policy_name = $4;`

const SECRET_INSERT_PATTERN = `INSERT INTO "secrets_pattern_ (secret_org, secret_name, pattern_org, pattern_name, last_update_check, partition) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING;`
const SECRET_UPDATE_TIME_PATTERN = `UPDATE "secrets_pattern_ SET last_update_check = $1, updated = current_timestamp WHERE secret_org = $2 AND secret_name = $3;`
const SECRET_DELETE_PATTERN = `DELETE FROM "secrets_pattern_ WHERE secret_org = $1 AND secret_name = $2 AND pattern_org = $3 AND pattern_name = $4;`

const SECRET_MOVE = `WITH moved_rows AS (
    DELETE FROM "secrets_pattern_ a
    RETURNING a.secret_org, a.secret_name, a.pattern_org, a.pattern_name, a.last_update_check
)
INSERT INTO "secrets_pattern_ (secret_org, secret_name, pattern_org, pattern_name, last_update_check, partition) SELECT secret_org, secret_name, pattern_org, pattern_name, last_update_check, 'partition_name' FROM moved_rows WHERE secret_org <> pattern_org;
`

const SECRET_DROP_PARTITION_POLICY = `DROP TABLE "secrets_policy_;`
const SECRET_DROP_PARTITION_PATTERN = `DROP TABLE "secrets_pattern_;`

const SECRET_DISTINCT_NAMES_POLICY = `SELECT DISTINCT secret_org, secret_name FROM "secrets_policy_;`
const SECRET_DISTINCT_NAMES_PATTERN = `SELECT DISTINCT secret_org, secret_name FROM "secrets_pattern_;`

const SECRET_DISTINCT_NAMES_BY_POLICY = `SELECT DISTINCT secret_org, secret_name FROM "secrets_policy_ WHERE policy_org = $1 AND policy_name = $2;`
const SECRET_DISTINCT_NAMES_BY_PATTERN = `SELECT DISTINCT secret_org, secret_name FROM "secrets_pattern_ WHERE pattern_org = $1 AND pattern_name = $2;`

const SECRET_POLICIES_TO_UPDATE = `SELECT DISTINCT policy_org, policy_name FROM "secrets_policy_ WHERE secret_org = $1 AND secret_name = $2 AND last_update_check < $3;`
const SECRET_PATTERNS_TO_UPDATE = `SELECT DISTINCT pattern_org, pattern_name FROM "secrets_pattern_ WHERE secret_org = $1 AND secret_name = $2 AND last_update_check < $3;`

const SECRET_DISTINCT_POLICIES = `SELECT DISTINCT policy_name FROM "secrets_policy_ WHERE policy_org = $1;`
const SECRET_DISTINCT_PATTERNS = `SELECT DISTINCT pattern_name FROM "secrets_pattern_ WHERE pattern_org = $1;`

const SECRET_DELETE_BY_POLICY = `DELETE FROM "secrets_policy_ WHERE policy_org = $1 AND policy_name = $2;`
const SECRET_DELETE_BY_PATTERN = `DELETE FROM "secrets_pattern_ WHERE pattern_org = $1 AND pattern_name = $2;`

// Functions that create SQL strings, filling in partition names as necessary.
func (db *AgbotPostgresqlDB) GetSecretPartitionTableNamePolicy(partition string) string {
	return SECRET_TABLE_NAME_ROOT_POLICY + partition + `"`
}

func (db *AgbotPostgresqlDB) GetSecretPartitionTableNamePattern(partition string) string {
	return SECRET_TABLE_NAME_ROOT_PATTERN + partition + `"`
}

func (db *AgbotPostgresqlDB) GetPrimarySecretPartitionTableCreatePolicy() string {
	sql := strings.Replace(SECRET_CREATE_PARTITION_TABLE_POLICY, SECRET_TABLE_NAME_ROOT_POLICY, db.GetSecretPartitionTableNamePolicy(db.PrimaryPartition()), 1)
	sql = strings.Replace(sql, SECRET_PARTITION_FILLIN, db.PrimaryPartition(), 1)
	return sql
}

func (db *AgbotPostgresqlDB) GetPrimarySecretPartitionTableCreatePattern() string {
	sql := strings.Replace(SECRET_CREATE_PARTITION_TABLE_PATTERN, SECRET_TABLE_NAME_ROOT_PATTERN, db.GetSecretPartitionTableNamePattern(db.PrimaryPartition()), 1)
	sql = strings.Replace(sql, SECRET_PARTITION_FILLIN, db.PrimaryPartition(), 1)
	return sql
}

func (db *AgbotPostgresqlDB) GetPrimarySecretPartitionTableIndexCreatePolicy() string {
	sql := strings.Replace(SECRET_CREATE_PARTITION_INDEX_POLICY, SECRET_TABLE_NAME_ROOT_POLICY, db.GetSecretPartitionTableNamePolicy(db.PrimaryPartition()), 2)
	return sql
}

func (db *AgbotPostgresqlDB) GetPrimarySecretPartitionTableIndexCreatePattern() string {
	sql := strings.Replace(SECRET_CREATE_PARTITION_INDEX_PATTERN, SECRET_TABLE_NAME_ROOT_PATTERN, db.GetSecretPartitionTableNamePattern(db.PrimaryPartition()), 2)
	return sql
}

func (db *AgbotPostgresqlDB) GetSecretPartitionTableDropPolicy(partition string) string {
	sql := strings.Replace(SECRET_DROP_PARTITION_POLICY, SECRET_TABLE_NAME_ROOT_POLICY, db.GetSecretPartitionTableNamePolicy(partition), 1)
	return sql
}

func (db *AgbotPostgresqlDB) GetSecretPartitionTableDropPattern(partition string) string {
	sql := strings.Replace(SECRET_DROP_PARTITION_PATTERN, SECRET_TABLE_NAME_ROOT_PATTERN, db.GetSecretPartitionTableNamePattern(partition), 1)
	return sql
}

// ==================
func (db *AgbotPostgresqlDB) GetUniqueSecretsQueryPolicy() string {
	sql := strings.Replace(SECRET_DISTINCT_NAMES_POLICY , SECRET_TABLE_NAME_ROOT_POLICY, db.GetSecretPartitionTableNamePolicy(db.PrimaryPartition()), 1)
	return sql
}

func (db *AgbotPostgresqlDB) GetUniqueSecretsQueryPattern() string {
	sql := strings.Replace(SECRET_DISTINCT_NAMES_PATTERN, SECRET_TABLE_NAME_ROOT_PATTERN, db.GetSecretPartitionTableNamePattern(db.PrimaryPartition()), 1)
	return sql
}

func (db *AgbotPostgresqlDB) GetUniquePolicySecretsQueryByPolicy() string {
	sql := strings.Replace(SECRET_DISTINCT_NAMES_BY_POLICY, SECRET_TABLE_NAME_ROOT_POLICY, db.GetSecretPartitionTableNamePolicy(db.PrimaryPartition()), 1)
	return sql
}

func (db *AgbotPostgresqlDB) GetUniquePatternSecretsQueryByPattern() string {
	sql := strings.Replace(SECRET_DISTINCT_NAMES_BY_PATTERN, SECRET_TABLE_NAME_ROOT_PATTERN, db.GetSecretPartitionTableNamePattern(db.PrimaryPartition()), 1)
	return sql
}

func (db *AgbotPostgresqlDB) GetPoliciesForUpdatedSecretQuery() string {
	sql := strings.Replace(SECRET_POLICIES_TO_UPDATE, SECRET_TABLE_NAME_ROOT_POLICY, db.GetSecretPartitionTableNamePolicy(db.PrimaryPartition()), 1)
	return sql
}

func (db *AgbotPostgresqlDB) GetPatternsForUpdatedSecretQuery() string {
	sql := strings.Replace(SECRET_PATTERNS_TO_UPDATE, SECRET_TABLE_NAME_ROOT_PATTERN, db.GetSecretPartitionTableNamePattern(db.PrimaryPartition()), 1)
	return sql
}

func (db *AgbotPostgresqlDB) GetUpdateSecretUpdateTimeQueryPolicy() string {
	sql := strings.Replace(SECRET_UPDATE_TIME_POLICY, SECRET_TABLE_NAME_ROOT_POLICY, db.GetSecretPartitionTableNamePolicy(db.PrimaryPartition()), 1)
	return sql
}

func (db *AgbotPostgresqlDB) GetUpdateSecretUpdateTimeQueryPattern() string {
	sql := strings.Replace(SECRET_UPDATE_TIME_PATTERN, SECRET_TABLE_NAME_ROOT_PATTERN, db.GetSecretPartitionTableNamePattern(db.PrimaryPartition()), 1)
	return sql
}

func (db *AgbotPostgresqlDB) GetUniquePoliciesQuery() string {
	sql := strings.Replace(SECRET_DISTINCT_POLICIES, SECRET_TABLE_NAME_ROOT_POLICY, db.GetSecretPartitionTableNamePolicy(db.PrimaryPartition()), 1)
	return sql
}

func (db *AgbotPostgresqlDB) GetUniquePatternsQuery() string {
	sql := strings.Replace(SECRET_DISTINCT_PATTERNS, SECRET_TABLE_NAME_ROOT_PATTERN, db.GetSecretPartitionTableNamePattern(db.PrimaryPartition()), 1)
	return sql
}

func (db *AgbotPostgresqlDB) GetDeletePolicy() string {
	sql := strings.Replace(SECRET_DELETE_BY_POLICY, SECRET_TABLE_NAME_ROOT_POLICY, db.GetSecretPartitionTableNamePolicy(db.PrimaryPartition()), 1)
	return sql
}

func (db *AgbotPostgresqlDB) GetDeletePattern() string {
	sql := strings.Replace(SECRET_DELETE_BY_PATTERN, SECRET_TABLE_NAME_ROOT_PATTERN, db.GetSecretPartitionTableNamePattern(db.PrimaryPartition()), 1)
	return sql
}

func (db *AgbotPostgresqlDB) GetDeleteSecretPolicy() string {
	sql := strings.Replace(SECRET_DELETE_POLICY, SECRET_TABLE_NAME_ROOT_POLICY, db.GetSecretPartitionTableNamePolicy(db.PrimaryPartition()), 1)
	return sql
}

func (db *AgbotPostgresqlDB) GetDeleteSecretPattern() string {
	sql := strings.Replace(SECRET_DELETE_PATTERN, SECRET_TABLE_NAME_ROOT_PATTERN, db.GetSecretPartitionTableNamePattern(db.PrimaryPartition()), 1)
	return sql
}

// The partition table name replacement scheme used in this function is slightly different from the others above.
func (db *AgbotPostgresqlDB) GetSecretPartitionMove(fromPartition string, toPartition string) string {
	sql := strings.Replace(SECRET_MOVE, SECRET_TABLE_NAME_ROOT_PATTERN, db.GetSecretPartitionTableNamePattern(toPartition), 2)
	sql = strings.Replace(sql, db.GetSecretPartitionTableNamePattern(toPartition), db.GetSecretPartitionTableNamePattern(fromPartition), 1)
	sql = strings.Replace(sql, SECRET_PARTITION_FILLIN, toPartition, 1)
	return sql
}

func (db *AgbotPostgresqlDB) GetManagedPolicySecretNames(policyOrg, policyName string) ([]string, error) {
	sql := ""
	if policyOrg == "" {
		sql = db.GetUniqueSecretsQueryPolicy()
	} else {
		sql = db.GetUniquePolicySecretsQueryByPolicy()
	}
	return db.getManagedSecretNames(sql, policyOrg, policyName)
}

func (db *AgbotPostgresqlDB) GetManagedPatternSecretNames(patternOrg, patternName string) ([]string, error) {
	sql := ""
	if patternOrg == "" {
		sql = db.GetUniqueSecretsQueryPattern()
	} else {
		sql = db.GetUniquePatternSecretsQueryByPattern()
	}
	return db.getManagedSecretNames(sql, patternOrg, patternName)
}

func (db *AgbotPostgresqlDB) getManagedSecretNames(sqlString, org, name string) ([]string, error) {

	// Find all the unique org/secretname combinations in the secrets table.
	secretNames := make([]string, 0, 10)

	var rows *sql.Rows
	var err error

	if org == "" {
		rows, err = db.db.Query(sqlString)
	} else {
		rows, err = db.db.Query(sqlString, org, name)
	}

	if err != nil {
		return nil, errors.New(fmt.Sprintf("error querying for unique org/secret names: %v", err))
	}

	// If the rows object doesnt get closed, memory and connections will grow and/or leak.
	defer rows.Close()
	for rows.Next() {
		var secretOrg, secretName string
		if err := rows.Scan(&secretOrg, &secretName); err != nil {
			return nil, errors.New(fmt.Sprintf("error scanning unique secret name result set row: %v", err))
		} else {
			secretNames = append(secretNames, fmt.Sprintf("%s/%s", secretOrg, secretName))
		}
	}

	// The rows.Next() function will exit with false when done or an error occurred. Get any error encountered during iteration.
	if err = rows.Err(); err != nil {
		return nil, errors.New(fmt.Sprintf("error iterating unique secret name result set: %v", err))
	}

	return secretNames, nil
}

// Returns a list of unique org qualified secret names.
func (db *AgbotPostgresqlDB) GetPoliciesWithUpdatedSecrets(secretOrg, secretName string, lastUpdate int64) ([]string, error) {
	return db.getUpdatedSecrets(db.GetPoliciesForUpdatedSecretQuery(), secretOrg, secretName, lastUpdate)
}

func (db *AgbotPostgresqlDB) GetPatternsWithUpdatedSecrets(secretOrg, secretName string, lastUpdate int64) ([]string, error) {
	return db.getUpdatedSecrets(db.GetPatternsForUpdatedSecretQuery(), secretOrg, secretName, lastUpdate)
}

func (db *AgbotPostgresqlDB) getUpdatedSecrets(sql, org, name string, lastUpdate int64) ([]string, error) {

	// Find all the unique org/secretname combinations in the secrets table.
	names := make([]string, 0, 10)

	rows, err := db.db.Query(sql, org, name, lastUpdate)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("error querying for unique org/secret names: %v", err))
	}

	// If the rows object doesnt get closed, memory and connections will grow and/or leak.
	defer rows.Close()
	for rows.Next() {
		var retOrg, retName string
		if err := rows.Scan(&retOrg, &retName); err != nil {
			return nil, errors.New(fmt.Sprintf("error scanning for updated secrets: %v", err))
		} else {
			names = append(names, fmt.Sprintf("%s/%s", retOrg, retName))
		}
	}

	// The rows.Next() function will exit with false when done or an error occurred. Get any error encountered during iteration.
	if err = rows.Err(); err != nil {
		return nil, errors.New(fmt.Sprintf("error iterating rows with updated secrets: %v", err))
	}

	return names, nil
}

func (db *AgbotPostgresqlDB) SetSecretUpdate(secretOrg, secretName string, secretUpdateTime int64) error {

	err := db.setInternalSecretUpdate(db.GetUpdateSecretUpdateTimeQueryPolicy(), secretOrg, secretName, secretUpdateTime)
	if err != nil {
		return errors.New(fmt.Sprintf("error updating policy secret %s/%s: %v", secretOrg, secretName, err))
	}

	err = db.setInternalSecretUpdate(db.GetUpdateSecretUpdateTimeQueryPattern(), secretOrg, secretName, secretUpdateTime)
	if err != nil {
		return errors.New(fmt.Sprintf("error updating pattern secret %s/%s: %v", secretOrg, secretName, err))
	}

	return nil
}

func (db *AgbotPostgresqlDB) setInternalSecretUpdate(sql, secretOrg, secretName string, secretUpdateTime int64) error {

	updated, err := db.db.Exec(sql, secretUpdateTime, secretOrg, secretName)
	if err != nil {
		return errors.New(fmt.Sprintf("error setting update time for %s/%s: %v", secretOrg, secretName, err))
	}

	// Not all DB drivers support the rows affected function.
	rowsAffected, err := updated.RowsAffected()
	if err == nil {
		glog.V(2).Infof("Succeeded setting update time in %v rows for %s/%s", rowsAffected, secretOrg, secretName)
	} else {
		glog.V(2).Infof("Succeeded setting update time for %s/%s", secretOrg, secretName)
	}

	return nil

}

func (db *AgbotPostgresqlDB) GetPoliciesInOrg(org string) ([]string, error) {
	return db.getDeploymentInOrg(db.GetUniquePoliciesQuery(), org)
}

func (db *AgbotPostgresqlDB) GetPatternsInOrg(org string) ([]string, error) {
	return db.getDeploymentInOrg(db.GetUniquePatternsQuery(), org)
}

func (db *AgbotPostgresqlDB) getDeploymentInOrg(sql, org string) ([]string, error) {

	// Find all the unique policy/pattern names for a given org in the secrets table.
	names := make([]string, 0, 10)

	rows, err := db.db.Query(sql, org)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("error querying for unique names: %v", err))
	}

	// If the rows object doesnt get closed, memory and connections will grow and/or leak.
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, errors.New(fmt.Sprintf("error scanning unique name result set row: %v", err))
		} else {
			names = append(names, fmt.Sprintf("%s/%s", org, name))
		}
	}

	// The rows.Next() function will exit with false when done or an error occurred. Get any error encountered during iteration.
	if err = rows.Err(); err != nil {
		return nil, errors.New(fmt.Sprintf("error iterating unique  name result set: %v", err))
	}

	return names, nil

}

func (db *AgbotPostgresqlDB) DeleteSecretsForPolicy(polOrg, polName string) error {

	_, err := db.db.Exec(db.GetDeletePolicy(), polOrg, polName)
	if err != nil {
		return errors.New(fmt.Sprintf("error deleting secrets for %s/%s: %v", polOrg, polName, err))
	}

	return nil
}

func (db *AgbotPostgresqlDB) DeleteSecretsForPattern(patternOrg, patternName string) error {

	_, err := db.db.Exec(db.GetDeletePattern(), patternOrg, patternName)
	if err != nil {
		return errors.New(fmt.Sprintf("error deleting secrets for %s/%s: %v", patternOrg, patternName, err))
	}

	return nil
}

func (db *AgbotPostgresqlDB) DeletePolicySecret(secretOrg, secretName, policyOrg, policyName string) error {

	_, err := db.db.Exec(db.GetDeleteSecretPolicy(), secretOrg, secretName, policyOrg, policyName)
	if err != nil {
		return errors.New(fmt.Sprintf("error deleting secret %s/%s from policy %s/%s: %v", secretOrg, secretName, policyOrg, policyName, err))
	}

	return nil
}

func (db *AgbotPostgresqlDB) DeletePatternSecret(secretOrg, secretName, patternOrg, patternName string) error {

	_, err := db.db.Exec(db.GetDeleteSecretPattern(), secretOrg, secretName, patternOrg, patternName)
	if err != nil {
		return errors.New(fmt.Sprintf("error deleting secret %s/%s from pattern %s/%s: %v", secretOrg, secretName, patternOrg, patternName, err))
	}

	return nil
}

func (db *AgbotPostgresqlDB) AddManagedPolicySecret(secretOrg, secretName, policyOrg, policyName string, updateTime int64) error {

	sql := strings.Replace(SECRET_INSERT_POLICY, SECRET_TABLE_NAME_ROOT_POLICY, db.GetSecretPartitionTableNamePolicy(db.PrimaryPartition()), 1)

	if _, err := db.db.Exec(sql, secretOrg, secretName, policyOrg, policyName, updateTime, db.PrimaryPartition()); err != nil {
		return err
	} else {
		glog.V(2).Infof("Succeeded creating managed policy secret record")
	}

	return nil
}

func (db *AgbotPostgresqlDB) AddManagedPatternSecret(secretOrg, secretName, patternOrg, patternName string, updateTime int64) error {

	sql := strings.Replace(SECRET_INSERT_PATTERN, SECRET_TABLE_NAME_ROOT_PATTERN, db.GetSecretPartitionTableNamePattern(db.PrimaryPartition()), 1)

	if _, err := db.db.Exec(sql, secretOrg, secretName, patternOrg, patternName, updateTime, db.PrimaryPartition()); err != nil {
		return err
	} else {
		glog.V(2).Infof("Succeeded creating managed pattern secret record")
	}

	return nil
}