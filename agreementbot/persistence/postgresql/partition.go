package postgresql

import (
	"errors"
	"fmt"
)

// Functions related to partitions in the postgresql database. The workload usages should always be using the same partitions
// as the agreements , or fewer partitions if an agreement partition contains only archived records.
func (db *AgbotPostgresqlDB) FindPartitions() ([]string, error) {

	if allPartitions, err := db.FindAgreementPartitions(); err != nil {
		return nil, err
	} else {
		return allPartitions, nil
	}

}

// If any partition is specified in the config where the table is missing for any of the persisted objects, then remove that
// partition from the working config.
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
