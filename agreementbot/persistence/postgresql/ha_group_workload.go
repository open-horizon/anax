package postgresql

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/agreementbot/persistence"
	"strings"
)

// Constants for the sql table operations required to manage workload upgrades for service in HA groups

// Create the ha workload upgrade table. This table will not be partitioned as it is shared between agbots
const CREATE_HA_WORKLOAD_UPGRADE_MAIN_TABLE = `CREATE TABLE IF NOT EXISTS ha_workload_upgrade (
	group_name text NOT NULL,
	org_id text NOT NULL,
	policy_name	text NOT NULL,
	node_id text NOT NULL,
	updated timestamp with time zone DEFAULT current_timestamp
);`

// Check if the ha group is in the table. If not add the group, node, and policy name that it is upgrading with
// These operations are in the same transaction to prevent a situation where 2 agbots check for the group name, before either can add it to the table
const HA_WORKLOAD_ADD_IF_NOT_PRESENT = `
CREATE OR REPLACE FUNCTION ha_workload_add_if_not_present(
 	ha_group_name CHARACTER VARYING,
 	ha_org_id CHARACTER VARYING,
 	ha_policy_name CHARACTER VARYING,
 	ha_node_id CHARACTER VARYING)
 	RETURNS TABLE(db_node_id text) AS $$

BEGIN
LOCK TABLE ha_workload_upgrade;

IF NOT EXISTS (SELECT node_id FROM ha_workload_upgrade WHERE group_name = ha_group_name AND org_id = ha_org_id AND policy_name = ha_policy_name) THEN
 	INSERT INTO ha_workload_upgrade (group_name, org_id, policy_name, node_id) VALUES (ha_group_name, ha_org_id, ha_policy_name, ha_node_id);
END IF;

RETURN QUERY SELECT node_id FROM ha_workload_upgrade WHERE group_name = ha_group_name AND org_id = ha_org_id AND policy_name = ha_policy_name;

END $$ LANGUAGE plpgsql;`

const HA_WORKLOAD_ADD_IF_NOT_PRESENT_BY_FUNCTION = `SELECT * FROM ha_workload_add_if_not_present($1,$2,$3,$4);`

const HA_WORKLOAD_DELETE = `DELETE FROM ha_workload_upgrade WHERE group_name = $1 AND org_id = $2 AND policy_name = $3 AND node_id = $4;`

const HA_WORKLOAD_DELETE_ALL = `DELETE FROM ha_workload_upgrade;`

const HA_WORKLOAD_DELETE_ALL_IN_HA_GROUP = `DELETE FROM ha_workload_upgrade WHERE group_name = $1 AND org_id = $2;`

const HA_WORKLOAD_DELETE_ALL_BY_GROUP_AND_NODE = `DELETE FROM ha_workload_upgrade WHERE group_name = $1 AND org_id = $2 AND node_id =$3;`

const HA_WORKLOAD_GET_ALL_IN_HA_GROUP = `SELECT policy_name, node_id FROM ha_workload_upgrade WHERE group_name = $1 AND org_id = $2;`

const HA_WORKLOAD_GET = `SELECT group_name, org_id, policy_name, node_id FROM ha_workload_upgrade WHERE group_name = $1 AND org_id = $2 AND policy_name = $3;`

const HA_WORKLOAD_UPDATE = `UPDATE ha_workload_upgrade SET node_id = $4 WHERE group_name = $1 AND org_id = $2 AND policy_name = $3;`

const HA_WORKLOAD_INSERT = `INSERT INTO ha_workload_upgrade (group_name, org_id, policy_name, node_id) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING;`

const HA_WORKLOAD_GET_ALL = `SELECT group_name, org_id, policy_name, node_id FROM ha_workload_upgrade;`

func (db *AgbotPostgresqlDB) DeleteAllHAUpgradingWorkload() error {
	_, qerr := db.db.Exec(HA_WORKLOAD_DELETE_ALL)
	return qerr
}

func (db *AgbotPostgresqlDB) DeleteHAUpgradingWorkload(workloadToDelete persistence.UpgradingHAGroupWorkload) error {
	_, qerr := db.db.Exec(HA_WORKLOAD_DELETE, workloadToDelete.GroupName, workloadToDelete.OrgId, workloadToDelete.PolicyName, workloadToDelete.NodeId)
	return qerr
}

func (db *AgbotPostgresqlDB) DeleteHAUpgradingWorkloadsByGroupName(org string, haGroupName string) error {
	_, qerr := db.db.Exec(HA_WORKLOAD_DELETE_ALL_IN_HA_GROUP, haGroupName, org)
	return qerr
}

func (db *AgbotPostgresqlDB) ListHAUpgradingWorkloadsByGroupName(org string, haGroupName string) ([]persistence.UpgradingHAGroupWorkload, error) {
	upgradingWorkloads := []persistence.UpgradingHAGroupWorkload{}
	rows, err := db.db.Query(HA_WORKLOAD_GET_ALL_IN_HA_GROUP, haGroupName, org)

	if err != nil {
		return nil, fmt.Errorf("error querying database for all upgrading workloads in org/hagroup %v/%v. Error was: %v", org, haGroupName, err)
	}

	defer rows.Close()
	for rows.Next() {
		var dbPolicyName sql.NullString
		var dbNodeId sql.NullString

		if err = rows.Scan(&dbPolicyName, &dbNodeId); err != nil {
			return nil, fmt.Errorf("error scanning row for ha workloads in org/hagroup %v/%v currently upgrading error was: %v", org, haGroupName, err)
		}

		upgradingWorkloads = append(upgradingWorkloads, persistence.UpgradingHAGroupWorkload{GroupName: haGroupName, OrgId: org, PolicyName: dbPolicyName.String, NodeId: dbNodeId.String})
	}

	return upgradingWorkloads, nil
}

func (db *AgbotPostgresqlDB) GetHAUpgradingWorkload(org string, haGroupName string, policyName string) (*persistence.UpgradingHAGroupWorkload, error) {
	var dbHAGroup sql.NullString
	var dbOrg sql.NullString
	var dbPolicyName sql.NullString
	var dbNodeId sql.NullString

	qerr := db.db.QueryRow(HA_WORKLOAD_GET, haGroupName, org, policyName).Scan(&dbHAGroup, &dbOrg, &dbPolicyName, &dbNodeId)
	if qerr != nil && qerr != sql.ErrNoRows && !strings.Contains(qerr.Error(), "not exist") {
		return nil, errors.New(fmt.Sprintf("error scanning row for ha upgrading workload for org: %v, hagroup: %v and policy name %v, error: %v", org, haGroupName, policyName, qerr))
	} else if qerr == sql.ErrNoRows || (qerr != nil && strings.Contains(qerr.Error(), "not exist")) {
		// error is: sql: no rows in result set
		return nil, nil
	}

	if uwl, err := persistence.NewUpgradingHAGroupWorkload(dbHAGroup.String, dbOrg.String, dbPolicyName.String, dbNodeId.String); err != nil {
		return nil, errors.New(fmt.Sprintf("error creating UpgradingHAGroupWorkload object from row %v %v %v %v, error: %v", dbHAGroup.String, dbOrg.String, dbPolicyName.String, dbNodeId.String, err))
	} else {
		return uwl, nil
	}
}

func (db *AgbotPostgresqlDB) ListAllHAUpgradingWorkloads() ([]persistence.UpgradingHAGroupWorkload, error) {
	upgradingWorkloads := []persistence.UpgradingHAGroupWorkload{}
	rows, err := db.db.Query(HA_WORKLOAD_GET_ALL)

	if err != nil {
		return nil, fmt.Errorf("error querying database for all upgrading workloads. Error was: %v", err)
	}

	defer rows.Close()
	for rows.Next() {
		var dbHAGroup sql.NullString
		var dbOrg sql.NullString
		var dbPolicyName sql.NullString
		var dbNodeId sql.NullString

		if err = rows.Scan(&dbHAGroup, &dbOrg, &dbPolicyName, &dbNodeId); err != nil {
			return nil, fmt.Errorf("error scanning row for ha workloads, error was: %v", err)
		}

		upgradingWorkloads = append(upgradingWorkloads, persistence.UpgradingHAGroupWorkload{GroupName: dbHAGroup.String, OrgId: dbOrg.String, PolicyName: dbPolicyName.String, NodeId: dbNodeId.String})
	}

	return upgradingWorkloads, nil
}

func (db *AgbotPostgresqlDB) UpdateHAUpgradingWorkloadForGroupAndPolicy(org string, haGroupName string, policyName string, deviceId string) error {
	if _, err := db.db.Exec(HA_WORKLOAD_UPDATE, haGroupName, org, policyName, deviceId); err != nil {
		return errors.New(fmt.Sprintf("error updating ha upgrading workload to %v for %v/%v/%v, error: %v", deviceId, org, haGroupName, policyName, err))
	} else {
		glog.V(2).Infof(fmt.Sprintf("Succeeded updating ha upgrading workload to %v for %v/%v/%v.", deviceId, org, haGroupName, policyName))
	}
	return nil
}

// Check if there is an entry for the given haGroupName, org, policyName. If exists, return the node id of the existing row. If not, insert a new row.
func (db *AgbotPostgresqlDB) InsertHAUpgradingWorkloadForGroupAndPolicy(org string, haGroupName string, policyName string, deviceId string) (string, error) {
	var dbNodeId sql.NullString
	qerr := db.db.QueryRow(HA_WORKLOAD_ADD_IF_NOT_PRESENT_BY_FUNCTION, haGroupName, org, policyName, deviceId).Scan(&dbNodeId)

	if qerr != nil && qerr != sql.ErrNoRows {
		return deviceId, fmt.Errorf("error scanning row for ha workloads currently upgrading in group %v/%v for policy %v. %v", org, haGroupName, policyName, qerr)
	}

	if !dbNodeId.Valid {
		return deviceId, fmt.Errorf("node id returned from ha workload updates table search is not valid")
	}

	if dbNodeId.String == deviceId {
		glog.V(2).Infof(fmt.Sprintf("Succeeded inserting ha upgrading workload for node %v for %v/%v/%v.", deviceId, org, haGroupName, policyName))
	}
	return dbNodeId.String, nil
}
