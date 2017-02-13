package persistence

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	anaxdevice "github.com/open-horizon/anax/device"
	"time"
)

const DEVICES = "devices"

type ExchangeAccount struct {
	Id    string `json:"id"`
	Email string `json:"email"`
}

func (e ExchangeAccount) String() string {
	return fmt.Sprintf("Id: %v, Email: %v", e.Id, e.Email)
}

// as of v2.1.0, id will always be what device.Id() returns
type ExchangeDevice struct {
	Id                 string          `json:"id"`
	Account            ExchangeAccount `json:"account"`
	Name               string          `json:"name"`
	Token              string          `json:"token"`
	TokenLastValidTime uint64          `json:"token_last_valid_time"`
	TokenValid         bool            `json:"token_valid"`
}

func (e ExchangeDevice) String() string {
	var tokenShadow string
	if e.Token != "" {
		tokenShadow = "set"
	} else {
		tokenShadow = "unset"
	}

	return fmt.Sprintf("Account: %v, Token: <%s>, Name: %v, TokenLastValidTime: %v, TokenValid: %v", e.Account, tokenShadow, e.Name, e.TokenLastValidTime, e.TokenValid)
}

// TODO: removed check for email set temporarily until the new account mgmt. stuff is released
func newExchangeDevice(token string, name string, tokenLastValidTime uint64, account *ExchangeAccount) (*ExchangeDevice, error) {
	if token == "" || name == "" || tokenLastValidTime == 0 || account == nil || account.Id == "" {
		return nil, errors.New("Cannot create exchange account, illegal arguments")
	}

	id, _ := anaxdevice.Id()

	return &ExchangeDevice{
		Id:                 id,
		Name:               name,
		Token:              token,
		TokenLastValidTime: tokenLastValidTime,
		TokenValid:         true,
		Account:            *account,
	}, nil
}

// a convenience function b/c we know there is really only one device
func InvalidateExchangeToken(db *bolt.DB) (*ExchangeDevice, error) {
	exchDev, err := FindExchangeDevice(db)
	if err != nil {
		return nil, err
	}

	return updateExchangeDeviceToken(db, exchDev.Account.Id, "")
}

func SetExchangeDeviceToken(db *bolt.DB, accountId string, token string) (*ExchangeDevice, error) {
	if accountId == "" || token == "" {
		return nil, errors.New("Argument null and mustn't be")
	}

	return updateExchangeDeviceToken(db, accountId, token)
}

// always assumed the given token is valid at the time of call
func updateExchangeDeviceToken(db *bolt.DB, accountId string, token string) (*ExchangeDevice, error) {
	// TODO: factor out duplication b/n serialization here and in SaveNewExchangeDevice

	if accountId == "" {
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
			return fmt.Errorf("No device with given account id to update: %v", accountId)
		} else if err := json.Unmarshal(current, &mod); err != nil {
			return fmt.Errorf("Failed to unmarshal device data: %v. Error: %v", string(current), err)
		} else {

			// a little weird since there is only one key in the bucket, but we want to make sure the token update is for the right account since that's the association that is made in the exchange
			if mod.Account.Id != accountId {
				return fmt.Errorf("No device with given account id to update: %v", accountId)
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
func SaveNewExchangeDevice(db *bolt.DB, token string, name string, accountId string, accountEmail string) (*ExchangeDevice, error) {

	if token == "" || name == "" || accountId == "" {
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

	exDevice, err := newExchangeDevice(token, name, uint64(time.Now().Unix()), &ExchangeAccount{
		Id:    accountId,
		Email: accountEmail,
	})

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
		return nil
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
