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

func Test_SaveArchitectureAttributes(t *testing.T) {

	// TODO: make a factory so the types don't have to be added by caller here
	attr := &ArchitectureAttributes{
		Meta: &AttributeMeta{
			Id:          "arch",
			SensorUrls:  []string{},
			Label:       "Supported Architecture",
			Publishable: pT,
			Type:        reflect.TypeOf(ArchitectureAttributes{}).Name(),
		},
		Architecture: "armhf",
	}

	attr.GetMeta().AppendSensorUrl("zoo").AppendSensorUrl("boo")

	_, err := SaveOrUpdateAttribute(testDb, attr, "", true)
	if err != nil {
		panic(err)
	}

	matches, err := FindApplicableAttributes(testDb, "zoo")
	if err != nil {
		panic(err)
	}

	fmt.Printf("matches: %v", matches)
	sAttr := matches[0]

	switch sAttr.(type) {
	case ArchitectureAttributes:
		arch := sAttr.(ArchitectureAttributes)

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
	pub := func(attr Attribute, url ...string) {
		for _, u := range url {
			attr.GetMeta().AppendSensorUrl(u)
		}

		// db is shared, ok for now
		_, err := SaveOrUpdateAttribute(testDb, attr, "", true)
		if err != nil {
			panic(err)
		}
	}

	illZ := "http://illuminated.z/v/2"
	illK := "http://illuminated.k/v/2"

	arch := &ArchitectureAttributes{
		Meta: &AttributeMeta{
			Id:          "architecture",
			SensorUrls:  []string{},
			Label:       "Supported Architecture",
			Publishable: pT,
			Type:        reflect.TypeOf(ArchitectureAttributes{}).Name(),
		},
		Architecture: "amd64",
	}

	pub(arch, illZ, illK)

	comp := &ComputeAttributes{
		Meta: &AttributeMeta{
			Id:          "compute",
			SensorUrls:  []string{},
			Label:       "Compute Resources",
			Publishable: pT,
			Type:        reflect.TypeOf(ComputeAttributes{}).Name(),
		},
		CPUs: 4,
		RAM:  1024,
	}

	pub(comp, illZ)

	comp2 := &ComputeAttributes{
		Meta: &AttributeMeta{
			Id:          "compute",
			SensorUrls:  []string{},
			Label:       "Compute Resources",
			Publishable: pT,
			Type:        reflect.TypeOf(ComputeAttributes{}).Name(),
		},
		CPUs: 2,
		RAM:  2048,
	}

	pub(comp2, illK)

	loc := &LocationAttributes{
		Meta: &AttributeMeta{
			Id:          "location",
			SensorUrls:  []string{},
			Label:       "Location",
			Publishable: pF,
			Type:        reflect.TypeOf(LocationAttributes{}).Name(),
		},
		Lat: -140.03,
		Lon: 20.12,
	}

	// no sensors URLs in meta means apply to all
	pub(loc)

	misc := &UserInputAttributes{
		Meta: &AttributeMeta{
			Id:          "misc",
			SensorUrls:  []string{},
			Publishable: pT,
			Type:        reflect.TypeOf(UserInputAttributes{}).Name(),
		},
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
			SensorUrls:  []string{},
			Publishable: pF,
			Type:        reflect.TypeOf(UserInputAttributes{}).Name(),
		},
		Mappings: map[string]interface{}{
			"user":         "fred",
			"pass":         "pinkfloydfan4ever",
			"device_label": "home",
		},
	}

	pub(creds, illK)

	services, err := FindApplicableAttributes(testDb, illK)
	if err != nil {
		panic(err)
	}

	var kComp *ComputeAttributes
	var kCreds *UserInputAttributes
	var kArch *ArchitectureAttributes
	var kLoc *LocationAttributes

	for _, serv := range services {
		switch serv.(type) {
		case ComputeAttributes:
			k := serv.(ComputeAttributes)
			kComp = &k
			if kComp.CPUs != 2 {
				t.Errorf("wrong CPU count")
			}
		case UserInputAttributes:
			k := serv.(UserInputAttributes)
			kCreds = &k
			if it, ok := kCreds.Mappings["device_label"]; !ok || it != "home" {
				t.Errorf("wrong cred fact")
			}
		case LocationAttributes:
			k := serv.(LocationAttributes)
			kLoc = &k
			if *kLoc.GetMeta().Publishable {
				t.Errorf("wrong pub fact")
			}
		case ArchitectureAttributes:
			k := serv.(ArchitectureAttributes)
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
	envvars, err := AttributesToEnvvarMap(services, "HZN_")
	if err != nil {
		t.Errorf("Failed to get envvar map: %v", err)
	}

	fmt.Printf("%v", envvars)
}
