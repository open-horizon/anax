// +build integration

package persistence_test

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/persistence"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
	"time"
)

// TODO: why does the persistence package need importing here even though this file is in the same package

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

func Test_SaveArchitectureAttributes(t *testing.T) {

	// TODO: make a factory so the types don't have to be added by caller here
	attr := &persistence.ArchitectureAttributes{
		Meta: &persistence.AttributeMeta{
			Id:          "arch",
			SensorUrls:  []string{},
			Label:       "Supported Architecture",
			Publishable: true,
			Type:        reflect.TypeOf(persistence.ArchitectureAttributes{}).String(),
		},
		Architecture: "armhf",
	}

	attr.GetMeta().AppendSensorUrl("zoo").AppendSensorUrl("boo")

	_, err := persistence.SaveOrUpdateServiceAttribute(testDb, attr)
	if err != nil {
		panic(err)
	}

	matches, err := persistence.FindApplicableAttributes(testDb, "zoo")
	if err != nil {
		panic(err)
	}

	fmt.Printf("matches: %v", matches)
	sAttr := matches[0]

	switch sAttr.(type) {
	case persistence.ArchitectureAttributes:
		arch := sAttr.(persistence.ArchitectureAttributes)

		if arch.Architecture != "armhf" {
			t.Errorf("Architecture in persisted type not accessible (b/c of ill deserialization or wrong value")
		}
	}

	urls := sAttr.GetMeta().SensorUrls
	if urls[len(urls)-1] != "zoo" {
		t.Errorf("SensorUrl match failed")
	}
}

func Test_DiscriminateSavedAttributes(t *testing.T) {
	pub := func(attr persistence.ServiceAttribute, url ...string) {
		for _, u := range url {
			attr.GetMeta().AppendSensorUrl(u)
		}

		// db is shared, ok for now
		_, err := persistence.SaveOrUpdateServiceAttribute(testDb, attr)
		if err != nil {
			panic(err)
		}
	}

	illZ := "http://illuminated.z/v/2"
	illK := "http://illuminated.k/v/2"

	arch := &persistence.ArchitectureAttributes{
		Meta: &persistence.AttributeMeta{
			Id:          "architecture",
			SensorUrls:  []string{},
			Label:       "Supported Architecture",
			Publishable: true,
			Type:        reflect.TypeOf(persistence.ArchitectureAttributes{}).String(),
		},
		Architecture: "amd64",
	}

	pub(arch, illZ, illK)

	comp := &persistence.ComputeAttributes{
		Meta: &persistence.AttributeMeta{
			Id:          "compute",
			SensorUrls:  []string{},
			Label:       "Compute Resources",
			Publishable: true,
			Type:        reflect.TypeOf(persistence.ComputeAttributes{}).String(),
		},
		CPUs: 4,
		RAM:  1024,
	}

	pub(comp, illZ)

	comp2 := &persistence.ComputeAttributes{
		Meta: &persistence.AttributeMeta{
			Id:          "compute",
			SensorUrls:  []string{},
			Label:       "Compute Resources",
			Publishable: true,
			Type:        reflect.TypeOf(persistence.ComputeAttributes{}).String(),
		},
		CPUs: 2,
		RAM:  2048,
	}

	pub(comp2, illK)

	loc := &persistence.LocationAttributes{
		Meta: &persistence.AttributeMeta{
			Id:          "location",
			SensorUrls:  []string{},
			Label:       "Location",
			Publishable: false,
			Type:        reflect.TypeOf(persistence.LocationAttributes{}).String(),
		},
		Lat: "-140.03",
		Lon: "20.12",
	}

	// no sensors URLs in meta means apply to all
	pub(loc)

	misc := &persistence.MappedAttributes{
		Meta: &persistence.AttributeMeta{
			Id:          "misc",
			SensorUrls:  []string{},
			Publishable: true,
			Type:        reflect.TypeOf(persistence.MappedAttributes{}).String(),
		},
		Mappings: map[string]string{
			"x": "xoo",
			"y": "yoo",
			"z": "zoo",
		},
	}

	pub(misc, illZ)

	creds := &persistence.MappedAttributes{
		Meta: &persistence.AttributeMeta{
			Id:          "credentials",
			SensorUrls:  []string{},
			Publishable: false,
			Type:        reflect.TypeOf(persistence.MappedAttributes{}).String(),
		},
		Mappings: map[string]string{
			"user":         "fred",
			"pass":         "pinkfloydfan4ever",
			"device_label": "home",
		},
	}

	pub(creds, illK)

	services, err := persistence.FindApplicableAttributes(testDb, illK)
	if err != nil {
		panic(err)
	}

	var kComp *persistence.ComputeAttributes
	var kCreds *persistence.MappedAttributes
	var kArch *persistence.ArchitectureAttributes
	var kLoc *persistence.LocationAttributes

	for _, serv := range services {
		switch serv.(type) {
		case persistence.ComputeAttributes:
			k := serv.(persistence.ComputeAttributes)
			kComp = &k
			if kComp.CPUs != 2 {
				t.Errorf("wrong CPU count")
			}
		case persistence.MappedAttributes:
			k := serv.(persistence.MappedAttributes)
			kCreds = &k
			if it, ok := kCreds.Mappings["device_label"]; !ok || it != "home" {
				t.Errorf("wrong cred fact")
			}
		case persistence.LocationAttributes:
			k := serv.(persistence.LocationAttributes)
			kLoc = &k
			if kLoc.GetMeta().Publishable {
				t.Errorf("wrong pub fact")
			}
		case persistence.ArchitectureAttributes:
			k := serv.(persistence.ArchitectureAttributes)
			kArch = &k
			if kArch.Architecture != "amd64" {
				t.Errorf("wrong arch fact")
			}
		default:
			t.Errorf("Unhandled service attribute: %v", serv)
		}
	}

	if kComp == nil || kCreds == nil || kLoc == nil || kArch == nil {
		t.Errorf("Nil attributes that mustn't be")
	}

	// TODO: separate into another test
	envvars, err := persistence.AttributesToEnvvarMap(services, "HZN_")
	if err != nil {
		t.Errorf("Failed to get envvar map: %v", err)
	}

	fmt.Printf("%v", envvars)
}
