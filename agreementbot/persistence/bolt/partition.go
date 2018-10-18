package bolt

import ()

// Functions related to partitions in the bolt database. It does not use partitions, or rather has only 1 global partition.
func (db *AgbotBoltDB) FindPartitions() ([]string, error) {

	return []string{"global"}, nil

}

func (db *AgbotBoltDB) ClaimPartition(timeout uint64) (string, error) {
	return "global", nil
}

func (db *AgbotBoltDB) HeartbeatPartition() error {
	return nil
}

func (db *AgbotBoltDB) QuiescePartition() error {
	return nil
}

func (db *AgbotBoltDB) GetPartitionOwner(id string) (string, error) {
	return "global", nil
}

func (db *AgbotBoltDB) MovePartition(timeout uint64) error {
	return nil
}
