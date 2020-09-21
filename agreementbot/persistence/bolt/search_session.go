package bolt

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"strconv"
	"time"
)

const SEARCH_SESSION_BUCKET = "search_session" // The bolt DB bucket name for the search session object.

// There is only one of these in the database.
type SearchSession struct {
	ChangedSince  uint64 `json:"changed_since"`
	SessionToken  uint64 `json:"session_token"`
	SessionEnded  bool   `json:"session_ended"`
	UpdatingAgbot string `json:"updating_agbot"`
	Updated       uint64 `json:"updated"`
}

func (r SearchSession) String() string {
	return fmt.Sprintf("ChangedSince: %v, SessionToken: %v, SessionEnded: %v, Agbot: %v, Updated: %v", r.ChangedSince, r.SessionToken, r.SessionEnded, r.UpdatingAgbot, r.Updated)
}

// Called by the database Init hook to setup the initial object in the database.
func (db *AgbotBoltDB) InitSearchSession() error {
	ss := SearchSession{
		ChangedSince:  0,
		SessionToken:  1,
		SessionEnded:  true,
		UpdatingAgbot: "this",
		Updated:       uint64(time.Now().Unix()),
	}
	return db.saveSearchSession(&ss)
}

// Functions that are part of the database interface, which all agbot database implementations must support.

// Get a session token and changedSince values so that the caller can use it to perform a node search. The
// returned token might be a new session token or it might be the current session, depends whether or not the
// session has ended.
func (db *AgbotBoltDB) ObtainSearchSession(policyName string) (string, uint64, error) {
	ss, err := db.findSearchSession()
	if err != nil {
		return "", 0, err
	} else {
		// If the current session has ended (because the search exhausted all the nodes), then allocate a new
		// session token for a new search session. The session token is actually a number so be careful of the
		// number rolling over.
		if ss.SessionEnded {
			if ss.SessionToken >= 2000000000 {
				ss.SessionToken = 1
			} else {
				ss.SessionToken += 1
			}

			if err := db.saveSearchSession(ss); err != nil {
				return "", 0, err
			}
		}
		return strconv.FormatUint(ss.SessionToken, 10), ss.ChangedSince, nil
	}
}

func (db *AgbotBoltDB) UpdateSearchSessionChangedSince(currentChangedSince uint64, newChangedSince uint64, policyName string) (bool, error) {
	ss, err := db.findSearchSession()
	if err != nil {
		return false, err
	}

	// Save the current session state for return at the end.
	currentSessionState := ss.SessionEnded

	if ss.ChangedSince == currentChangedSince && ss.SessionEnded == false {
		ss.ChangedSince = newChangedSince
		ss.SessionEnded = true
		ss.Updated = uint64(time.Now().Unix())

		if err := db.saveSearchSession(ss); err != nil {
			return false, err
		}
	}
	return currentSessionState, nil

}

// No need to implement this in low scale bolt DB use cases.
func (db *AgbotBoltDB) ResetAllChangedSince(newChangedSince uint64) error {
	return nil
}

// No need to implement this in low scale bolt DB use cases.
func (db *AgbotBoltDB) ResetPolicyChangedSince(policy string, newChangedSince uint64) error {
	return nil
}

func (db *AgbotBoltDB) DumpSearchSessions() error {
	ss, err := db.findSearchSession()
	if err != nil {
		return err
	}
	glog.V(4).Infof("Search Session: %v", ss)
	return nil
}

// These are functions used internally by the search session object to provide the search session capability for the agbot database interface.

// Return the one and only SearchSession object in the DB.
func (db *AgbotBoltDB) findSearchSession() (*SearchSession, error) {
	mod := new(SearchSession)

	readErr := db.db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(ssBucketName())); b != nil {
			return b.ForEach(func(k, v []byte) error {

				if err := json.Unmarshal(v, mod); err != nil {
					return fmt.Errorf("Unable to deserialize search session record: %v", v)
				}

				return nil
			})
		}

		return nil // end transaction
	})

	if readErr != nil {
		return nil, readErr
	}
	return mod, nil
}

// Saves the one and only SearchSession object to the DB.
func (db *AgbotBoltDB) saveSearchSession(ss *SearchSession) error {
	writeErr := db.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(ssBucketName()))
		if err != nil {
			return err
		}

		if serial, err := json.Marshal(ss); err != nil {
			return fmt.Errorf("Failed to serialize search session: %v. Error: %v", *ss, err)
		} else {
			return b.Put([]byte(ssBucketName()), serial)
		}
	})

	return writeErr
}

func ssBucketName() string {
	return SEARCH_SESSION_BUCKET
}
