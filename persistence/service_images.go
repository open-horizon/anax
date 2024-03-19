package persistence

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"regexp"
	"time"
)

// service image table name
const SERVICE_IMAGES = "service_images"

type ServiceImageUsage struct {
	ImageId      string `json:"image_id"`
	TimeLastUsed uint64 `json:"time_last_used"`
}

func NewServiceImageUsage(imageId string) *ServiceImageUsage {
	return &ServiceImageUsage{ImageId: imageId, TimeLastUsed: uint64(time.Now().Unix())}
}

func (s ServiceImageUsage) String() string {
	return fmt.Sprintf("ImageId: %v, "+
		"TimeLastUsed: %v",
		s.ImageId, s.TimeLastUsed)
}

func (s ServiceImageUsage) ShortString() string {
	return s.String()
}

// save or update the given image info
// image use info is keyed with ImageName:ImageTag
func SaveOrUpdateServiceImage(db *bolt.DB, serviceImage *ServiceImageUsage) error {
	writeErr := db.Update(func(tx *bolt.Tx) error {
		if bucket, err := tx.CreateBucketIfNotExists([]byte(SERVICE_IMAGES)); err != nil {
			return err
		} else if serial, err := json.Marshal(serviceImage); err != nil {
			return fmt.Errorf("Failed to serialize service image usage: %v", err)
		} else {
			return bucket.Put([]byte(serviceImage.ImageId), serial)
		}
	})

	return writeErr
}

func DeleteServiceImage(db *bolt.DB, imageId string) error {
	return db.Update(func(tx *bolt.Tx) error {
		if bucket, err := tx.CreateBucketIfNotExists([]byte(SERVICE_IMAGES)); err != nil {
			return err
		} else if err := bucket.Delete([]byte(imageId)); err != nil {
			return fmt.Errorf("Unable to delete service image usage record for %v: %v.", imageId, err)
		}
		return nil
	})
}

func FindServiceImageUsageWithFilters(db *bolt.DB, filters []IUFilter) ([]ServiceImageUsage, error) {
	imgUsages := make([]ServiceImageUsage, 0)

	readErr := db.View(func(tx *bolt.Tx) error {
		if bucket := tx.Bucket([]byte(SERVICE_IMAGES)); bucket != nil {
			bucket.ForEach(func(k, v []byte) error {
				imgRec := ServiceImageUsage{}
				if err := json.Unmarshal(v, &imgRec); err != nil {
					return fmt.Errorf("Unable to deserialize service image usage record %v: %v", k, err)
				} else {
					exclude := false
					for _, filter := range filters {
						if !filter(imgRec) {
							exclude = true
						}
					}
					if !exclude {
						imgUsages = append(imgUsages, imgRec)
					}
				}
				return nil
			})
		}
		return nil
	})

	return imgUsages, readErr
}

type IUFilter func(ServiceImageUsage) bool

func ImageNameRegexFilter(imageId string) IUFilter {
	regEx, err := regexp.Compile(imageId)
	if err != nil {
		return func(iu ServiceImageUsage) bool { return false }
	}

	return func(iu ServiceImageUsage) bool { return regEx.Match([]byte(iu.ImageId)) }
}
