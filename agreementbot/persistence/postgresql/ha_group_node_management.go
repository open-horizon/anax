package postgresql

import (
	"database/sql"
	"fmt"
	"github.com/open-horizon/anax/agreementbot/persistence"
)

// Constants for the sql table operations required to manage node upgrades for nodes in HA groups

// Create the ha group table. This table will not be partitioned as it is shared between agbots
const CREATE_HA_GROUP_UPGRADE_MAIN_TABLE = `CREATE TABLE IF NOT EXISTS ha_group_updates (
	group_name text NOT NULL,
	org_id text NOT NULL,
	node_id text NOT NULL,
	nmp_id text NOT NULL,
	updated timestamp with time zone DEFAULT current_timestamp
);`

// Check if the ha group is in the table. If not add the group, node, and nmp that it is upgrading with
// These operations are in the same transaction to prevent a situation where 2 agbots check for the group name, before either can add it to the table
const HA_GROUP_ADD_IF_NOT_PRESENT = `
CREATE OR REPLACE FUNCTION ha_group_add_if_not_present(
	ha_group_name CHARACTER VARYING,
	ha_org_id CHARACTER VARYING,
	ha_node_id CHARACTER VARYING, 
	ha_nmp_id CHARACTER VARYING)
	RETURNS TABLE(db_node_id text, db_nmp_id text) AS $$

BEGIN
LOCK TABLE ha_group_updates;

IF NOT EXISTS (SELECT node_id FROM ha_group_updates WHERE group_name = ha_group_name AND org_id = ha_org_id) THEN
	INSERT INTO ha_group_updates (group_name, org_id, node_id, nmp_id) VALUES (ha_group_name, ha_org_id, ha_node_id, ha_nmp_id);
END IF;

RETURN QUERY SELECT node_id, nmp_id FROM ha_group_updates WHERE group_name = ha_group_name AND org_id = ha_org_id;

END $$ LANGUAGE plpgsql;`

const HA_GROUP_ADD_IF_NOT_PRESENT_BY_FUNCTION = `SELECT * FROM ha_group_add_if_not_present($1,$2,$3,$4);`

const HA_GROUP_DELETE_NODE = `DELETE FROM ha_group_updates WHERE group_name = $1 AND org_id = $2 AND node_id = $3 AND nmp_id = $4 `

const HA_GROUP_GET_IN_ORG_GROUP = `SELECT node_id, nmp_id FROM ha_group_updates WHERE org_id = $1 AND group_name = $2`

const HA_GROUP_GET_ALL_NODES = `SELECT group_name, org_id, node_id, nmp_id FROM ha_group_updates`

func (db *AgbotPostgresqlDB) CheckIfGroupPresentAndUpdateHATable(requestingNode persistence.UpgradingHAGroupNode) (*persistence.UpgradingHAGroupNode, error) {
	var dbNodeId sql.NullString
	var dbNmpId sql.NullString
	qerr := db.db.QueryRow(HA_GROUP_ADD_IF_NOT_PRESENT_BY_FUNCTION, requestingNode.GroupName, requestingNode.OrgId, requestingNode.NodeId, requestingNode.NMPName).Scan(&dbNodeId, &dbNmpId)

	if qerr != nil && qerr != sql.ErrNoRows {
		return nil, fmt.Errorf("error scanning row for ha nodes in group %v currently updating error: %v", requestingNode.GroupName, qerr)
	}

	if !dbNodeId.Valid {
		return nil, fmt.Errorf("node id returned from ha group updates table search is not valid")
	} else if !dbNmpId.Valid {
		return nil, fmt.Errorf("nmp id returned from ha group updates table search is not valid")
	} else {
		return &persistence.UpgradingHAGroupNode{GroupName: requestingNode.GroupName, OrgId: requestingNode.OrgId, NodeId: dbNodeId.String, NMPName: dbNmpId.String}, nil
	}
}

func (db *AgbotPostgresqlDB) DeleteHAUpgradeNode(nodeToDelete persistence.UpgradingHAGroupNode) error {
	_, qerr := db.db.Exec(HA_GROUP_DELETE_NODE, nodeToDelete.GroupName, nodeToDelete.OrgId, nodeToDelete.NodeId, nodeToDelete.NMPName)
	return qerr
}

func (db *AgbotPostgresqlDB) ListUpgradingNodeInGroup(orgId string, groupName string) (*persistence.UpgradingHAGroupNode, error) {
	var dbNodeId sql.NullString
	var dbNmpId sql.NullString
	qerr := db.db.QueryRow(HA_GROUP_GET_IN_ORG_GROUP, orgId, groupName).Scan(&dbNodeId, &dbNmpId)
	if qerr != nil && qerr != sql.ErrNoRows {
		return nil, fmt.Errorf("error querying database for upgrading node in group %v. Error was: %v", orgId, groupName, qerr)
	}

	if dbNodeId.Valid {
		return &persistence.UpgradingHAGroupNode{GroupName: groupName, OrgId: orgId, NodeId: dbNodeId.String, NMPName: dbNmpId.String}, nil
	}

	return nil, nil
}

func (db *AgbotPostgresqlDB) ListAllUpgradingHANode() ([]persistence.UpgradingHAGroupNode, error) {
	upgradingNodes := []persistence.UpgradingHAGroupNode{}
	rows, err := db.db.Query(HA_GROUP_GET_ALL_NODES)

	if err != nil {
		return nil, fmt.Errorf("error querying database for all upgrading HA nodes. Error was: %v", err)
	}

	defer rows.Close()
	for rows.Next() {
		var dbHAGroup sql.NullString
		var dbOrg sql.NullString
		var dbNodeId sql.NullString
		var dbNmpId sql.NullString

		if err = rows.Scan(&dbHAGroup, &dbOrg, &dbNodeId, &dbNmpId); err != nil {
			return nil, fmt.Errorf("error scanning row for ha nodes, error was: %v", err)
		}

		upgradingNodes = append(upgradingNodes, persistence.UpgradingHAGroupNode{GroupName: dbHAGroup.String, OrgId: dbOrg.String, NodeId: dbNodeId.String, NMPName: dbNmpId.String})
	}

	return upgradingNodes, nil
}
