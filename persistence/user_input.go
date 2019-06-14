package persistence

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/policy"
)

const NODE_USERINPUT = "nodeuserinput"                              // The bucket name in the bolt DB.
const EXCHANGE_NODE_USERINPUT_HASH = "exchange_node_userinput_hash" // The buucket for the exchange node userinput hash

// Retrieve the node user input object from the database. The bolt APIs assume there is more than 1 object in a bucket,
// so this function has to be prepared for that case, even though there should only ever be 1.
func FindNodeUserInput(db *bolt.DB) ([]policy.UserInput, error) {

	var userInput []policy.UserInput

	readErr := db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(NODE_USERINPUT)); b != nil {
			return b.ForEach(func(k, v []byte) error {

				if err := json.Unmarshal(v, &userInput); err != nil {
					return fmt.Errorf("Unable to deserialize node user input record: %v", v)
				}

				return nil
			})
		}

		return nil // end transaction
	})

	if readErr != nil {
		return nil, readErr
	}

	return userInput, nil
}

// There is only 1 object in the bucket so we can use the bucket name as the object key.
func SaveNodeUserInput(db *bolt.DB, userInput []policy.UserInput) error {

	writeErr := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(NODE_USERINPUT))
		if err != nil {
			return err
		}

		if serial, err := json.Marshal(userInput); err != nil {
			return fmt.Errorf("Failed to serialize node user input: %v. Error: %v", userInput, err)
		} else {
			return b.Put([]byte(NODE_USERINPUT), serial)
		}
	})

	return writeErr
}

// Remove the node user input object from the local database.
func DeleteNodeUserInput(db *bolt.DB) error {

	if ui, err := FindNodeUserInput(db); err != nil {
		return err
	} else if ui == nil {
		return nil
	} else {

		return db.Update(func(tx *bolt.Tx) error {

			if b, err := tx.CreateBucketIfNotExists([]byte(NODE_USERINPUT)); err != nil {
				return err
			} else if err := b.Delete([]byte(NODE_USERINPUT)); err != nil {
				return fmt.Errorf("Unable to delete node user input object: %v", err)
			} else {
				return nil
			}
		})
	}
}

// Delete all UserInputAttributes from the local db.
func DeleteAllUserInputAttributes(db *bolt.DB) error {
	if attributes, err := FindApplicableAttributes(db, "", ""); err != nil {
		return fmt.Errorf("Failed to get all the UserInputAttributes. %v", err)
	} else {
		for _, attr := range attributes {
			switch attr.(type) {
			case UserInputAttributes:
				if _, err := DeleteAttribute(db, attr.GetMeta().Id); err != nil {
					return fmt.Errorf("Failed to delete UserInputAttributes %v. %v", attr, err)
				}
			}
		}
	}

	return nil
}

// Get all UerInputAttributes from db
func GetAllUserInputAttributes(db *bolt.DB) ([]UserInputAttributes, error) {
	allUI := []UserInputAttributes{}
	if attributes, err := FindApplicableAttributes(db, "", ""); err != nil {
		return nil, fmt.Errorf("Failed to get all the UserInputAttributes. %v", err)
	} else {
		for _, attr := range attributes {
			switch attr.(type) {
			case UserInputAttributes:
				allUI = append(allUI, attr.(UserInputAttributes))
			}
		}
	}

	return allUI, nil
}

// Retrieve the exchange node user input hash from the database.
func GetNodeUserInputHash_Exch(db *bolt.DB) ([]byte, error) {

	userInputHash := []byte{}

	readErr := db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(EXCHANGE_NODE_USERINPUT_HASH)); b != nil {
			return b.ForEach(func(k, v []byte) error {
				userInputHash = v
				return nil
			})
		}

		return nil // end transaction
	})

	if readErr != nil {
		return nil, readErr
	}

	return userInputHash, nil
}

// save the exchange node user input hash.
func SaveNodeUserInputHash_Exch(db *bolt.DB, userInputHash []byte) error {

	writeErr := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(EXCHANGE_NODE_USERINPUT_HASH))
		if err != nil {
			return err
		}

		return b.Put([]byte(EXCHANGE_NODE_USERINPUT_HASH), userInputHash)

	})

	return writeErr
}

// Remove the exchange node user input hash from the local database.
func DeleteNodeUserInputHash_Exch(db *bolt.DB) error {

	if userInputHash, err := GetNodeUserInputHash_Exch(db); err != nil {
		return err
	} else if userInputHash == nil || len(userInputHash) == 0 {
		return nil
	} else {
		return db.Update(func(tx *bolt.Tx) error {

			if b, err := tx.CreateBucketIfNotExists([]byte(EXCHANGE_NODE_USERINPUT_HASH)); err != nil {
				return err
			} else if err := b.Delete([]byte(EXCHANGE_NODE_USERINPUT_HASH)); err != nil {
				return fmt.Errorf("Unable to delete exchange node user input hash from local db: %v", err)
			} else {
				return nil
			}
		})
	}
}
