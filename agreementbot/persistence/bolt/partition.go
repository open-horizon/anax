package bolt

import ()

// Functions related to partitions in the bolt database. It does not use partitions, or rather has only 1 global partition.
func (db *AgbotBoltDB) FindPartitions() ([]string, error) {

	return []string{"global"}, nil

}
