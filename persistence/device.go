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

type Configstate struct {
	State          string `json:"state"`
	LastUpdateTime uint64 `json:"last_update_time"`
}

func (c Configstate) String() string {
	return fmt.Sprintf("State: %v, Time: %v", c.State, c.LastUpdateTime)
}

type ExchangeDevice struct {
	Id                 string      `json:"id"`
	Org                string      `json:"organization"`
	Pattern            string      `json:"pattern"`
	Name               string      `json:"name"`
	Token              string      `json:"token"`
	TokenLastValidTime uint64      `json:"token_last_valid_time"`
	TokenValid         bool        `json:"token_valid"`
	HA                 bool        `json:"ha"`
	Config             Configstate `json:"configstate"`
	ServiceBased       bool        `json:"serviceBased"`  // The device is service based if this flag is on, but the flag being off could mean that service or workload based is not yet known.
	WorkloadBased      bool        `json:"workloadBased"` // The device is workload based if this flag is on, but the flag being off could mean that service or workload based is not yet known.
}

func (e ExchangeDevice) String() string {
	var tokenShadow string
	if e.Token != "" {
		tokenShadow = "set"
	} else {
		tokenShadow = "unset"
	}

	return fmt.Sprintf("Org: %v, Token: <%s>, Name: %v, TokenLastValidTime: %v, TokenValid: %v, Pattern: %v, ServiceBased: %v, WorkloadBased: %v, %v", e.Org, tokenShadow, e.Name, e.TokenLastValidTime, e.TokenValid, e.Pattern, e.ServiceBased, e.WorkloadBased, e.Config)
}

func (e ExchangeDevice) GetId() string {
	return fmt.Sprintf("%v/%v", e.Org, e.Id)
}

func newExchangeDevice(id string, token string, name string, tokenLastValidTime uint64, ha bool, org string, pattern string, configstate string, serviceBased bool, workloadBased bool) (*ExchangeDevice, error) {
	if id == "" || token == "" || name == "" || tokenLastValidTime == 0 || org == "" {
		return nil, errors.New("Cannot create exchange device, illegal arguments")
	}

	cfg := Configstate{
		State:          configstate,
		LastUpdateTime: uint64(time.Now().Unix()),
	}

	return &ExchangeDevice{
		Id:                 id,
		Name:               name,
		Token:              token,
		TokenLastValidTime: tokenLastValidTime,
		TokenValid:         true,
		HA:                 ha,
		Org:                org,
		Pattern:            pattern,
		Config:             cfg,
		ServiceBased:       serviceBased,
		WorkloadBased:      workloadBased,
	}, nil
}

// a convenience function b/c we know there is really only one device
func (e *ExchangeDevice) InvalidateExchangeToken(db *bolt.DB) (*ExchangeDevice, error) {
	exchDev, err := FindExchangeDevice(db)
	if err != nil {
		return nil, err
	}

	return updateExchangeDevice(db, e, exchDev.Id, true, func(d ExchangeDevice) *ExchangeDevice {
		d.Token = ""
		return &d
	})
}

func (e *ExchangeDevice) SetExchangeDeviceToken(db *bolt.DB, deviceId string, token string) (*ExchangeDevice, error) {
	if deviceId == "" || token == "" {
		return nil, errors.New("Argument null and mustn't be")
	}

	return updateExchangeDevice(db, e, deviceId, false, func(d ExchangeDevice) *ExchangeDevice {
		d.Token = token
		return &d
	})
}

func (e *ExchangeDevice) SetConfigstate(db *bolt.DB, deviceId string, state string, serviceBased bool) (*ExchangeDevice, error) {
	if deviceId == "" || state == "" {
		return nil, errors.New("Argument null and mustn't be")
	}

	return updateExchangeDevice(db, e, deviceId, false, func(d ExchangeDevice) *ExchangeDevice {
		d.Config.State = state
		d.Config.LastUpdateTime = uint64(time.Now().Unix())
		d.ServiceBased = serviceBased
		d.WorkloadBased = !serviceBased
		return &d
	})
}

func (e *ExchangeDevice) SetServiceBased(db *bolt.DB) (*ExchangeDevice, error) {
	return updateExchangeDevice(db, e, e.Id, false, func(d ExchangeDevice) *ExchangeDevice {
		d.ServiceBased = true
		return &d
	})
}

func (e *ExchangeDevice) SetWorkloadBased(db *bolt.DB) (*ExchangeDevice, error) {
	return updateExchangeDevice(db, e, e.Id, false, func(d ExchangeDevice) *ExchangeDevice {
		d.WorkloadBased = true
		return &d
	})
}

func (e *ExchangeDevice) IsState(state string) bool {
	return e.Config.State == state
}

func (e *ExchangeDevice) IsServiceBased() bool {
	return e.ServiceBased && !e.WorkloadBased
}

func (e *ExchangeDevice) IsWorkloadBased() bool {
	return !e.ServiceBased && e.WorkloadBased
}

func updateExchangeDevice(db *bolt.DB, self *ExchangeDevice, deviceId string, invalidateToken bool, fn func(d ExchangeDevice) *ExchangeDevice) (*ExchangeDevice, error) {
	if deviceId == "" {
		return nil, fmt.Errorf("Illegal arguments specified.")
	}

	update := fn(*self)

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

			// Even though there is only one key in the bucket, make sure the update is for the right device
			if mod.Id != deviceId {
				return fmt.Errorf("No device with given device id to update: %v", deviceId)
			}

			// Differentiate token invalidation from updating a token.
			if invalidateToken {
				mod.Token = ""
				mod.TokenValid = false

			} else if update.Token != mod.Token && update.Token != "" {
				mod.Token = update.Token
				mod.TokenValid = true
				mod.TokenLastValidTime = uint64(time.Now().Unix())
			}

			// Write updates only to the fields we expect should be updateable
			if mod.Config.State != update.Config.State {
				mod.Config.State = update.Config.State
				mod.Config.LastUpdateTime = update.Config.LastUpdateTime
			}
			if mod.ServiceBased == false && mod.WorkloadBased == false && update.ServiceBased == true {
				mod.ServiceBased = update.ServiceBased
			}
			if mod.ServiceBased == false && mod.WorkloadBased == false && update.WorkloadBased == true {
				mod.WorkloadBased = update.WorkloadBased
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
func SaveNewExchangeDevice(db *bolt.DB, id string, token string, name string, ha bool, organization string, pattern string, configstate string, serviceBased bool, workloadBased bool) (*ExchangeDevice, error) {

	if id == "" || token == "" || name == "" || organization == "" || configstate == "" {
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

	exDevice, err := newExchangeDevice(id, token, name, uint64(time.Now().Unix()), ha, organization, pattern, configstate, serviceBased, workloadBased)

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

func DeleteExchangeDevice(db *bolt.DB) error {

	if dev, err := FindExchangeDevice(db); err != nil {
		return err
	} else if dev == nil {
		return fmt.Errorf("could not find record for device")
	} else {

		return db.Update(func(tx *bolt.Tx) error {

			if b, err := tx.CreateBucketIfNotExists([]byte(DEVICES)); err != nil {
				return err
			} else if err := b.Delete([]byte(DEVICES)); err != nil {
				return fmt.Errorf("Unable to delete horizon device object: %v", err)
			} else {
				return nil
			}
		})
	}
}
