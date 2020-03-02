package persistence

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"strconv"
	"time"
)

// container volume table name
const CONTAINER_VOLUMES = "container_volumes"

type ContainerVolume struct {
	RecordId     string `json:"record_id"` // unique primary key for records
	Name         string `json:"name"`
	CreationTime uint64 `json:"creation_time"`
	ArchiveTime  uint64 `json:"archive_time"`
}

func NewContainerVolume(name string) *ContainerVolume {
	return &ContainerVolume{
		Name:         name,
		CreationTime: uint64(time.Now().Unix()),
		ArchiveTime:  0,
	}
}

func (w ContainerVolume) String() string {
	return fmt.Sprintf("RecordId: %v, "+
		"Name: %v, "+
		"CreationTime: %v, "+
		"ArchiveTime: %v",
		w.RecordId, w.Name, w.CreationTime, w.ArchiveTime)
}

func (w ContainerVolume) ShortString() string {
	return w.String()
}

// save the ContainerVolume record into db.
func SaveContainerVolume(db *bolt.DB, container_volume *ContainerVolume) error {
	writeErr := db.Update(func(tx *bolt.Tx) error {
		if bucket, err := tx.CreateBucketIfNotExists([]byte(CONTAINER_VOLUMES)); err != nil {
			return err
		} else {
			// use the old key if it has one, otherwise generate one
			key := container_volume.RecordId
			if key == "" {
				if nextKey, err := bucket.NextSequence(); err != nil {
					return fmt.Errorf("Unable to get sequence key for saving new container volume %v. Error: %v", container_volume, err)
				} else {
					key = strconv.FormatUint(nextKey, 10)
					container_volume.RecordId = key
				}
			}

			serial, err := json.Marshal(*container_volume)
			if err != nil {
				return fmt.Errorf("Failed to serialize the container volume object: %v. Error: %v", *container_volume, err)
			}
			return bucket.Put([]byte(key), serial)
		}
	})

	return writeErr
}

// save the container volume into db.
func SaveContainerVolumeByName(db *bolt.DB, name string) error {
	pcv := NewContainerVolume(name)
	return SaveContainerVolume(db, pcv)
}

// Find the container volumes that are not deleted yet
func FindAllUndeletedContainerVolumes(db *bolt.DB) ([]ContainerVolume, error) {
	return FindContainerVolumes(db, []ContainerVolumeFilter{UnarchivedCVFilter()})
}

// Mark the given volume as archived.
func ArchiveContainerVolumes(db *bolt.DB, cv *ContainerVolume) error {
	if cv == nil {
		return nil
	}
	cv.ArchiveTime = uint64(time.Now().Unix())
	if err := SaveContainerVolume(db, cv); err != nil {
		return fmt.Errorf("Failed to archive the container volume %v. %v", cv.Name, err)
	}
	return nil
}

// filter on ContainerVolume
type ContainerVolumeFilter func(ContainerVolume) bool

// filter on ArchiveTime time
func UnarchivedCVFilter() ContainerVolumeFilter {
	return func(c ContainerVolume) bool { return c.ArchiveTime == 0 }
}

// filter on name
func NameCVFilter(name string) ContainerVolumeFilter {
	return func(c ContainerVolume) bool { return c.Name == name }
}

// find container volumes from the db for the given filters
func FindContainerVolumes(db *bolt.DB, filters []ContainerVolumeFilter) ([]ContainerVolume, error) {
	cvs := make([]ContainerVolume, 0)

	// fetch container volumes
	readErr := db.View(func(tx *bolt.Tx) error {

		if b := tx.Bucket([]byte(CONTAINER_VOLUMES)); b != nil {
			b.ForEach(func(k, v []byte) error {

				var cv ContainerVolume

				if err := json.Unmarshal(v, &cv); err != nil {
					glog.Errorf("Unable to deserialize ContainerVolume db record: %v. Error: %v", v, err)
				} else {
					exclude := false
					for _, filterFn := range filters {
						if !filterFn(cv) {
							exclude = true
						}
					}
					if !exclude {
						cvs = append(cvs, cv)
					}
				}
				return nil
			})
		}

		return nil // end the transaction
	})

	if readErr != nil {
		return nil, readErr
	} else {
		return cvs, nil
	}
}
