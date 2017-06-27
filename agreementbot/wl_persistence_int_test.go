// +build integration

package agreementbot

import (
    "github.com/boltdb/bolt"
    "io/ioutil"
    "os"
    "testing"
    "time"
)

var testDb *bolt.DB

func TestMain(m *testing.M) {
    testDbFile, err := ioutil.TempFile("", "agreementbot_test.db")
    if err != nil {
        panic(err)
    }
    defer os.Remove(testDbFile.Name())

    var dbErr error
    testDb, dbErr = bolt.Open(testDbFile.Name(), 0600, &bolt.Options{Timeout: 10 * time.Second})
    if dbErr != nil {
        panic(err)
    }

    m.Run()
}
