package persistence

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"time"
)

const DEVICES = "devices"

type ExchangeDevice struct {
	Id                 string `json:"id"`
	Org                string `json:"organization"`
	Name               string `json:"name"`
	Token              string `json:"token"`
	TokenLastValidTime uint64 `json:"token_last_valid_time"`
	TokenValid         bool   `json:"token_valid"`
	HADevice           bool   `json:"ha_device"`
}

func (e ExchangeDevice) String() string {
	var tokenShadow string
	if e.Token != "" {
		tokenShadow = "set"
	} else {
		tokenShadow = "unset"
	}

	return fmt.Sprintf("Org: %v, Token: <%s>, Name: %v, TokenLastValidTime: %v, TokenValid: %v", e.Org, tokenShadow, e.Name, e.TokenLastValidTime, e.TokenValid)
}

func (e ExchangeDevice) GetId() string {
	return fmt.Sprintf("%v/%v", e.Org, e.Id)
}

func newExchangeDevice(id string, token string, name string, tokenLastValidTime uint64, ha bool, org string) (*ExchangeDevice, error) {
	if id == "" || token == "" || name == "" || tokenLastValidTime == 0 || org == "" {
		return nil, errors.New("Cannot create exchange device, illegal arguments")
	}

	return &ExchangeDevice{
		Id:                 id,
		Name:               name,
		Token:              token,
		TokenLastValidTime: tokenLastValidTime,
		TokenValid:         true,
		HADevice:           ha,
		Org:                org,
	}, nil
}

// a convenience function b/c we know there is really only one device
func InvalidateExchangeToken(db *bolt.DB) (*ExchangeDevice, error) {
	exchDev, err := FindExchangeDevice(db)
	if err != nil {
		return nil, err
	}

	return updateExchangeDeviceToken(db, exchDev.Id, "")
}

func SetExchangeDeviceToken(db *bolt.DB, deviceId string, token string) (*ExchangeDevice, error) {
	if deviceId == "" || token == "" {
		return nil, errors.New("Argument null and mustn't be")
	}

	return updateExchangeDeviceToken(db, deviceId, token)
}

// always assumed the given token is valid at the time of call
func updateExchangeDeviceToken(db *bolt.DB, deviceId string, token string) (*ExchangeDevice, error) {
	// TODO: factor out duplication b/n serialization here and in SaveNewExchangeDevice

	if deviceId == "" {
		return nil, fmt.Errorf("Illegal arguments specified.")
	}

	var mod ExchangeDevice

	return &mod, db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(DEVICES))
		if err != nil {
			return err
		}

		// b/c it's only possible to save one device in the bucket, we use "DEVICES" as the key name
		current := b.Get([]byte(DEVICES))

		if current == nil {
			return fmt.Errorf("No device with given device id to update: %v", deviceId)
		} else if err := json.Unmarshal(current, &mod); err != nil {
			return fmt.Errorf("Failed to unmarshal device data: %v. Error: %v", string(current), err)
		} else {

			// a little weird since there is only one key in the bucket, but we want to make sure the token update is for the right device
			if mod.Id != deviceId {
				return fmt.Errorf("No device with given device id to update: %v", deviceId)
			}

			// invalidate
			if token == "" {
				mod.Token = ""
				mod.TokenValid = false

			} else {
				// store, assume valid

				// write updates only to the fields we expect should be updateable
				mod.Token = token
				mod.TokenValid = true
				mod.TokenLastValidTime = uint64(time.Now().Unix())
			}

			// note: DEVICES is used as the key b/c we only want to store one value in this bucket

			if serialized, err := json.Marshal(mod); err != nil {
				return fmt.Errorf("Failed to serialize device record: %v. Error: %v", mod, err)
			} else if err := b.Put([]byte(DEVICES), serialized); err != nil {
				return fmt.Errorf("Failed to write device record with key: %v. Error: %v", DEVICES, err)
			} else {
				glog.V(2).Infof("Succeeded updating device record to %v", mod)
				return nil
			}
		}
	})

}

// always assumed the given token is valid at the time of call
func SaveNewExchangeDevice(db *bolt.DB, id string, token string, name string, ha bool, organization string) (*ExchangeDevice, error) {

	if id == "" || token == "" || name == "" || organization == "" {
		return nil, errors.New("Argument null and must not be")
	}

	duplicate := false

	dErr := db.View(func(tx *bolt.Tx) error {
		bd := tx.Bucket([]byte(DEVICES))
		if bd != nil {
			duplicate = (bd.Get([]byte(name)) != nil)
		}

		return nil

	})

	if dErr != nil {
		return nil, fmt.Errorf("Error checking duplicates of device named %v from db. Error: %v", name, dErr)
	} else if duplicate {
		return nil, fmt.Errorf("Duplicate record found in devices for %v.", name)
	}

	exDevice, err := newExchangeDevice(id, token, name, uint64(time.Now().Unix()), ha, organization)

	if err != nil {
		return nil, err
	}

	writeErr := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(DEVICES))
		if err != nil {
			return err
		}

		// note: DEVICES is used as the key b/c we only want to store one value in this bucket

		if serial, err := json.Marshal(&exDevice); err != nil {
			return fmt.Errorf("Failed to serialize device: %v. Error: %v", exDevice, err)
		} else {
			return b.Put([]byte(DEVICES), serial)
		}
	})

	return exDevice, writeErr
}

func FindExchangeDevice(db *bolt.DB) (*ExchangeDevice, error) {

	devices := make([]ExchangeDevice, 0)

	readErr := db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(DEVICES)); b != nil {
			return b.ForEach(func(k, v []byte) error {
				var dev ExchangeDevice

				if err := json.Unmarshal(v, &dev); err != nil {
					return fmt.Errorf("Unable to deserializer db record: %v", v)
				}

				devices = append(devices, dev)
				return nil
			})
		}

		return nil // end transaction
	})

	if readErr != nil {
		return nil, readErr
	}

	if len(devices) > 1 {
		return nil, fmt.Errorf("Unsupported state: more than one exchange device stored in bucket. Devices: %v", devices)
	} else if len(devices) == 1 {
		return &devices[0], nil
	} else {
		return nil, nil
	}
}
