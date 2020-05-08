// +build integration

package persistence

import (
	"fmt"
	"github.com/boltdb/bolt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
	"time"
)

var testDb *bolt.DB

func TestMain(m *testing.M) {
	testDbFile, err := ioutil.TempFile("", "anax_persistence_int_test.db")
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

var pT *bool
var pF *bool

func init() {
	t := true
	pT = &(t)

	f := false
	pF = &(f)
}

func Test_DiscriminateSavedAttributes(t *testing.T) {
	pub := func(attr Attribute, url ...string) {
		for _, u := range url {
			sps_a := GetAttributeServiceSpecs(&attr)
			if sps_a != nil {
				sps_a.AppendServiceSpec(ServiceSpec{Url: u, Org: "myorg"})
			}
		}

		// db is shared, ok for now
		_, err := SaveOrUpdateAttribute(testDb, attr, "", true)
		if err != nil {
			panic(err)
		}
	}

	illZ := "mycomp/http://illuminated.z/v/2"
	illK := "mycomp/http://illuminated.k/v/2"

	misc := &UserInputAttributes{
		Meta: &AttributeMeta{
			Id:          "misc",
			Publishable: pT,
			Type:        reflect.TypeOf(UserInputAttributes{}).Name(),
		},
		ServiceSpecs: new(ServiceSpecs),
		Mappings: map[string]interface{}{
			"x": "xoo",
			"y": "yoo",
			"z": "zoo",
		},
	}

	pub(misc, illZ)

	creds := &UserInputAttributes{
		Meta: &AttributeMeta{
			Id:          "credentials",
			Publishable: pF,
			Type:        reflect.TypeOf(UserInputAttributes{}).Name(),
		},
		ServiceSpecs: new(ServiceSpecs),
		Mappings: map[string]interface{}{
			"user":         "fred",
			"pass":         "pinkfloydfan4ever",
			"device_label": "home",
		},
	}

	pub(creds, illK)

	services, err := FindApplicableAttributes(testDb, "http://illuminated.k/v/2", "mycomp")
	if err != nil {
		panic(err)
	}

	var kCreds *UserInputAttributes

	for _, serv := range services {
		switch serv.(type) {
		case UserInputAttributes:
			k := serv.(UserInputAttributes)
			kCreds = &k
			if it, ok := kCreds.Mappings["device_label"]; !ok || it != "home" {
				t.Errorf("wrong cred fact")
			}
		default:
			t.Errorf("Unhandled service attribute: %v", serv)
		}
	}

	if kCreds == nil {
		t.Errorf("Nil attributes that mustn't be")
	}

	// TODO: separate into another test
	envvars := make(map[string]string)
	envvars, err = AttributesToEnvvarMap(services, envvars, "HZN_", 0, nil, false)
	if err != nil {
		t.Errorf("Failed to get envvar map: %v", err)
	}

	fmt.Printf("%v", envvars)
}
