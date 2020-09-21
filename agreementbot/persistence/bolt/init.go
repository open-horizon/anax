package bolt

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/config"
	"os"
	"path"
	"time"
)

// Setup everything bolt DB needs to be able to run an agbot. Since bolt is a simple document based database,
// all we need to setup is database file itself. There are no tables or indexes to create for bolt DB.
func (db *AgbotBoltDB) Initialize(cfg *config.HorizonConfig) error {

	if err := os.MkdirAll(cfg.AgreementBot.DBPath, 0700); err != nil {
		return errors.New(fmt.Sprintf("unable to create directory %v for bolt DB configuration, error: %v", cfg.AgreementBot.DBPath, err))
	}

	dbname := path.Join(cfg.AgreementBot.DBPath, BOLTDB_DATABASE_NAME)

	if agdb, err := bolt.Open(dbname, 0600, &bolt.Options{Timeout: 10 * time.Second}); err != nil {
		return errors.New(fmt.Sprintf("unable to open bolt database %v, error: %v", dbname, err))
	} else {
		db.db = agdb

	}

	// Initialize the one and only search session object
	if err := db.InitSearchSession(); err != nil {
		return errors.New(fmt.Sprintf("unable to init search session object in database %v, error: %v", dbname, err))
	}

	return nil

}
