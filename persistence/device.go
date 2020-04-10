package persistence

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"strings"
	"time"
)

const DEVICES = "devices"

// device types
const DEVICE_TYPE_DEVICE = "device"
const DEVICE_TYPE_CLUSTER = "cluster"

const CONFIGSTATE_UNCONFIGURING = "unconfiguring"
const CONFIGSTATE_UNCONFIGURED = "unconfigured"
const CONFIGSTATE_CONFIGURING = "configuring"
const CONFIGSTATE_CONFIGURED = "configured"

type Configstate struct {
	State          string `json:"state"`
	LastUpdateTime uint64 `json:"last_update_time"`
}

func (c Configstate) String() string {
	return fmt.Sprintf("State: %v, Time: %v", c.State, c.LastUpdateTime)
}

// This function returns the pattern org, pattern name and formatted pattern string 'pattern org/pattern name'.
// If the input pattern does not contain the org name, the device org name will be used as the pattern org name.
// The input is a pattern string 'pattern org/pattern name' or just 'pattern name' for backward compatibility.
// The device org is the org name for the device.
func GetFormatedPatternString(pattern string, device_org string) (string, string, string) {
	if pattern == "" {
		return "", "", ""
	} else if ix := strings.Index(pattern, "/"); ix < 0 {
		if device_org == "" {
			return device_org, pattern, pattern
		} else {
			return device_org, pattern, fmt.Sprintf("%v/%v", device_org, pattern)
		}
	} else {
		return pattern[:ix], pattern[ix+1:], pattern
	}
}

type ExchangeDevice struct {
	Id                 string      `json:"id"`
	Org                string      `json:"organization"`
	Pattern            string      `json:"pattern"`
	Name               string      `json:"name"`
	NodeType           string      `json:"nodeType"`
	Token              string      `json:"token"`
	TokenLastValidTime uint64      `json:"token_last_valid_time"`
	TokenValid         bool        `json:"token_valid"`
	HA                 bool        `json:"ha"`
	Config             Configstate `json:"configstate"`
}

func (e ExchangeDevice) String() string {
	var tokenShadow string
	if e.Token != "" {
		tokenShadow = "set"
	} else {
		tokenShadow = "unset"
	}

	return fmt.Sprintf("Org: %v, Token: <%s>, Name: %v, NodeType: %v, TokenLastValidTime: %v, TokenValid: %v, Pattern: %v, %v", e.Org, tokenShadow, e.Name, e.NodeType, e.TokenLastValidTime, e.TokenValid, e.Pattern, e.Config)
}

func (e ExchangeDevice) GetId() string {
	return fmt.Sprintf("%v/%v", e.Org, e.Id)
}

func newExchangeDevice(id string, token string, name string, nodeType string, tokenLastValidTime uint64, ha bool, org string, pattern string, configstate string) (*ExchangeDevice, error) {
	if id == "" || token == "" || name == "" || tokenLastValidTime == 0 || org == "" {
		return nil, errors.New("Cannot create exchange device, illegal arguments")
	}

	cfg := Configstate{
		State:          configstate,
		LastUpdateTime: uint64(time.Now().Unix()),
	}

	// make the pattern to the standard "org/pattern" format
	if pattern != "" {
		_, _, pat := GetFormatedPatternString(pattern, org)
		pattern = pat
	}

	return &ExchangeDevice{
		Id:                 id,
		Name:               name,
		NodeType:           nodeType,
		Token:              token,
		TokenLastValidTime: tokenLastValidTime,
		TokenValid:         true,
		HA:                 ha,
		Org:                org,
		Pattern:            pattern,
		Config:             cfg,
	}, nil
}

func (e *ExchangeDevice) GetNodeType() string {
	if e.NodeType == "" {
		return DEVICE_TYPE_DEVICE
	} else {
		return e.NodeType
	}
}

func (e *ExchangeDevice) IsEdgeCluster() bool {
	return e.NodeType == DEVICE_TYPE_CLUSTER
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

func (e *ExchangeDevice) SetConfigstate(db *bolt.DB, deviceId string, state string) (*ExchangeDevice, error) {
	if deviceId == "" || state == "" {
		return nil, errors.New("Argument null and mustn't be")
	}

	return updateExchangeDevice(db, e, deviceId, false, func(d ExchangeDevice) *ExchangeDevice {
		d.Config.State = state
		d.Config.LastUpdateTime = uint64(time.Now().Unix())
		return &d
	})
}

func (e *ExchangeDevice) SetNodeType(db *bolt.DB, deviceId string, nodeType string) (*ExchangeDevice, error) {
	if deviceId == "" || nodeType == "" {
		return nil, errors.New("The argument deviceId or nodeType cannot be empty.")
	}

	return updateExchangeDevice(db, e, deviceId, false, func(d ExchangeDevice) *ExchangeDevice {
		d.NodeType = nodeType
		return &d
	})
}

func (e *ExchangeDevice) SetPattern(db *bolt.DB, deviceId string, pattern string) (*ExchangeDevice, error) {
	if deviceId == "" {
		return nil, errors.New("Argument null and mustn't be")
	}

	return updateExchangeDevice(db, e, deviceId, false, func(d ExchangeDevice) *ExchangeDevice {
		d.Pattern = pattern
		return &d
	})
}

func (e *ExchangeDevice) IsState(state string) bool {
	return e.Config.State == state
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

			// Update the node type
			if mod.NodeType != update.NodeType {
				mod.NodeType = update.NodeType
			}

			// Update the pattern
			if mod.Pattern != update.Pattern {
				mod.Pattern = update.Pattern
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
func SaveNewExchangeDevice(db *bolt.DB, id string, token string, name string, nodeType string, ha bool, organization string, pattern string, configstate string) (*ExchangeDevice, error) {

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

	exDevice, err := newExchangeDevice(id, token, name, nodeType, uint64(time.Now().Unix()), ha, organization, pattern, configstate)

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
		// convert the pattern string to standard "org/pattern" format.
		if devices[0].Pattern != "" {
			_, _, pattern := GetFormatedPatternString(devices[0].Pattern, devices[0].Org)
			devices[0].Pattern = pattern
		}

		if devices[0].NodeType == "" {
			devices[0].NodeType = DEVICE_TYPE_DEVICE
		}
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

// Migrate a device object if it is restarted ona newer level of code.
func MigrateExchangeDevice(db *bolt.DB) (bool, error) {
	usingPattern := false
	// If the device object already exists, make sure its service or workload mode is set correctly. If not, set it.
	// This code handles devices that upgrade to an anax runtime that supports service mode but the device is still
	// using workloads.
	if db != nil {
		if dev, _ := FindExchangeDevice(db); dev != nil {

			// If the existing device is using a pattern then we need to turn off agreement tracking when we create the policy manager.
			if dev.Pattern != "" {
				usingPattern = true
			}
		}
	}
	return usingPattern, nil
}
