// +build integration
// +build go1.9

package api

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/adams-sarah/test2doc/test"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/worker"
	"github.com/stretchr/testify/assert"
)

func init() {
	flag.Parse()
}

func handleResp(r *http.Response, expectedStatus int) ([]byte, error) {
	if r.StatusCode != expectedStatus {
		return nil, fmt.Errorf("Unexpected status code in response: %v", r.StatusCode)
	}

	defer r.Body.Close()

	return ioutil.ReadAll(r.Body)
}

func deserialArray(b []byte) ([]Attribute, error) {
	var attrs map[string][]Attribute
	glog.V(6).Infof("Deserializing: %v", string(b[:]))

	return attrs["attributes"], json.Unmarshal(b, &attrs)
}

func deserialSingle(b []byte) (*Attribute, error) {
	var attr Attribute
	glog.V(6).Infof("Deserializing: %v", string(b))

	return &attr, json.Unmarshal(b, &attr)
}

func serial(t *testing.T, attrInput []byte) []byte {

	glog.V(6).Infof("To validate: %v", string(attrInput))

	if !json.Valid(attrInput) {
		t.Errorf("Invalid JSON input %v", attrInput)
	}

	serialized, err := json.Marshal(attrInput)
	if err != nil {
		t.Error(err)
	}

	return serialized
}

func setup() (string, *bolt.DB, error) {
	dir, err := ioutil.TempDir("", "api-attribute-")
	if err != nil {
		return "", nil, err
	}

	db, err := bolt.Open(path.Join(dir, "anax-int.db"), 0600, &bolt.Options{Timeout: 10 * time.Second})
	if err != nil {
		return dir, nil, err
	}

	return dir, db, nil
}

func simpleGET(t *testing.T, pp *url.URL, expectedCode int, shouldDeserialize bool) []Attribute {
	r, err := http.Get(pp.String())
	assert.Nil(t, err)

	b, err := handleResp(r, expectedCode)
	assert.Nil(t, err)

	if shouldDeserialize {
		attrs, err := deserialArray(b)
		assert.Nil(t, err)

		return attrs
	}

	return nil
}

func simpleMod(t *testing.T, method string, pp *url.URL, expectedCode int, payload []byte, expectReturnPayload bool) *Attribute {
	client := http.Client{}

	req, err := http.NewRequest(method, pp.String(), bytes.NewReader(payload))
	assert.Nil(t, err)
	assert.NotNil(t, req)

	req.Header.Add("Content-Type", "application/json; charset=utf-8")

	glog.Infof("Request: %v", req)

	resp, err := client.Do(req)

	glog.Infof("Response: %v", resp)

	b, err := handleResp(resp, expectedCode)
	assert.Nil(t, err)

	if resp.StatusCode-400 > 0 {
		return nil
	}

	if expectReturnPayload {
		attr, err := deserialSingle(b)
		assert.Nil(t, err)

		return attr
	}

	return nil
}

// TODO: fix this test after refactor of service: needs a mock exchange

//func Test_API_service_attribute_Suite(suite *testing.T) {
//	_, db, err := setup()
//	if err != nil {
//		suite.Error(err)
//	}
//	// we construct our own API instance so we can set route facts
//	api := &API{
//		Manager: worker.Manager{
//			Config:   &config.HorizonConfig{},
//			Messages: nil,
//		},
//
//		db:          db,
//		pm:          nil,
//		bcState:     make(map[string]map[string]BlockchainState),
//		bcStateLock: sync.Mutex{},
//	}
//
//	router := api.router(false)
//
//	// TODO: replace this with a recording server after the destination files are determined not to conflict b/n suites in the same package
//	server := httptest.NewServer(router)
//
//	// register first
//	_, err = persistence.SaveNewExchangeDevice(db, "device-22", "tokenval", "Device 22", false, "myorg", ".*")
//	assert.Nil(suite, err)
//
//	suite.Run("POST to /service with attributes is accepted", func(t *testing.T) {
//		payload := `{
//  "sensor_url": "https://bluehorizon.network/microservices/no-such-service",
//  "sensor_name": "no-such",
//  "sensor_version": "1.0.0",
//  "attributes": [
//    {
//      "type": "AgreementProtocolAttributes",
//      "label": "Agreement Protocols",
//      "publishable": true,
//      "host_only": false,
//      "mappings": {
//        "protocols": [
//          {
//            "Citizen Scientist": [
//              {
//                  "name": "hl2",
//                  "type": "ethereum",
//                  "organization": "IBM"
//              }
//            ]
//          }
//        ]
//      }
//    }
//  ]
//}`
//
//		pp, _ := url.Parse(fmt.Sprintf("%s/%s", server.URL, "service"))
//		client := http.Client{}
//
//		req, err := http.NewRequest(http.MethodPost, pp.String(), bytes.NewReader([]byte(payload)))
//		assert.Nil(t, err)
//		assert.NotNil(t, req)
//
//		req.Header.Add("Content-Type", "application/json; charset=utf-8")
//
//		resp, err := client.Do(req)
//		fmt.Printf("******** %v", err.Error())
//		assert.Nil(t, err)
//		assert.NotNil(t, resp)
//		//assert.EqualValues(t, http.StatusOK, resp.StatusCode)
//	})
//
//}

func Test_API_attribute_Suite(suite *testing.T) {
	_, db, err := setup()
	if err != nil {
		suite.Error(err)
	}

	sensorUrl := `https://bluehorizon.network/doc/locsampler`

	bF := false
	loc, err := persistence.SaveOrUpdateAttribute(db, &persistence.LocationAttributes{
		Meta: &persistence.AttributeMeta{
			Id:          "",
			SensorUrls:  []string{fmt.Sprintf("%v%s", sensorUrl, "-1")},
			Label:       "My Location",
			HostOnly:    &bF,
			Publishable: &bF,
			Type:        "LocationAttributes",
		},
		Lat:                40,
		Lon:                0.55,
		LocationAccuracyKM: 1,
		UseGps:             true,
	}, "", true)
	assert.Nil(suite, err)

	_, err = persistence.SaveOrUpdateAttribute(db, &persistence.MappedAttributes{
		Meta: &persistence.AttributeMeta{
			Id:          "",
			SensorUrls:  []string{sensorUrl},
			Label:       "Defs",
			HostOnly:    &bF,
			Publishable: &bF,
			Type:        "MappedAttributes",
		},
		Mappings: map[string]string{
			"SAMPLE_INTERVAL": "5s",
		},
	}, "", false)
	assert.Nil(suite, err)

	// we construct our own API instance so we can set route facts
	api := &API{
		Manager: worker.Manager{
			Config:   &config.HorizonConfig{},
			Messages: nil,
		},

		db:          db,
		pm:          nil,
		bcState:     make(map[string]map[string]BlockchainState),
		bcStateLock: sync.Mutex{},
	}

	router := api.router(false)
	router.KeepContext = true
	test.RegisterURLVarExtractor(mux.Vars)

	recordingServer, err := test.NewServer(router)
	if err != nil {
		suite.Error("Error setting up test", err)
	}

	// another server, this one won't record
	server := httptest.NewServer(router)

	suite.Run("GET fails unless device is registered", func(t *testing.T) {
		pp, _ := url.Parse(fmt.Sprintf("%s/%s", recordingServer.URL, "attribute"))
		simpleGET(t, pp, http.StatusFailedDependency, false)
	})

	_, err = persistence.SaveNewExchangeDevice(db, "device-22", "tokenval", "Device 22", false, "myorg", ".*", CONFIGSTATE_CONFIGURING)
	assert.Nil(suite, err)

	suite.Run("OPTIONS returns methods", func(t *testing.T) {
		pp, _ := url.Parse(fmt.Sprintf("%s/%s", recordingServer.URL, "attribute"))
		client := http.Client{}

		req, err := http.NewRequest("OPTIONS", pp.String(), nil)
		assert.Nil(t, err)
		resp, err := client.Do(req)
		assert.Nil(t, err)
		assert.EqualValues(t, resp.StatusCode, http.StatusOK)
	})

	suite.Run("Empty slice from GET /attribute/{id} when no attributes are valid for id in pathvar", func(t *testing.T) {

		pp, _ := url.Parse(fmt.Sprintf("%s/%s/%s", recordingServer.URL, "attribute", url.PathEscape("an_unknown_id")))
		attrs := simpleGET(t, pp, http.StatusOK, true)
		assert.EqualValues(t, 0, len(attrs))
	})

	suite.Run("Multiple attributes returned from GET /attribute", func(t *testing.T) {
		pp, _ := url.Parse(fmt.Sprintf("%s/%s", recordingServer.URL, "attribute"))
		attrs := simpleGET(t, pp, http.StatusOK, true)
		assert.EqualValues(t, 2, len(attrs))

		found := false
		for _, attr := range attrs {
			if *(attr.Type) == "LocationAttributes" {
				found = true
			}
		}

		if !found {
			t.Error("Didn't find LocationAttributes type in returned attrs as expected")
		}
	})

	suite.Run("Single return from GET /attribute/{id}", func(t *testing.T) {
		// querying w/ sensorUrl

		pp, _ := url.Parse(fmt.Sprintf("%s/%s/%s", recordingServer.URL, "attribute", url.PathEscape((*loc).GetMeta().Id)))
		attrs := simpleGET(t, pp, http.StatusOK, true)
		assert.EqualValues(t, 1, len(attrs))
	})

	suite.Run("OK returned for HEAD /attribute/{id}", func(t *testing.T) {
		pp, _ := url.Parse(fmt.Sprintf("%s/%s/%s", recordingServer.URL, "attribute", url.PathEscape((*loc).GetMeta().Id)))
		res, err := http.Head(pp.String())
		assert.Nil(t, err)
		assert.NotEqual(t, 0, res.ContentLength)
	})

	pubMapping := []byte(`{
			"type": "MappedAttributes",
			"label": "Application Mappings",
			"SensorUrls": ["https://foo", "https://goo"],
			"publishable":	false,
			"mappings": {
				"KEY": "VALUE"
			}}`)

	// intentionally missing required "type"
	bogusPubMapping := []byte(`{
				"label": "other mappings",
				"publishable":	true,
				"mappings": {
					"K": "V"
				}}`)

	var pubMappingId string
	suite.Run("New attribute given via POST to /attribute stores new attribute", func(t *testing.T) {
		pp, _ := url.Parse(fmt.Sprintf("%s/%s", recordingServer.URL, "attribute"))
		attr := simpleMod(t, http.MethodPost, pp, http.StatusCreated, pubMapping, true)
		pubMappingId = *attr.Id
		assert.NotEqual(t, "", pubMappingId, "Computed ID for new record must not be empty in returned object")
	})

	suite.Run("Already stored attribute with duplicate id yields 409 when attempting re-POST to /attribute", func(t *testing.T) {
		pp, _ := url.Parse(fmt.Sprintf("%s/%s", server.URL, "attribute"))
		simpleMod(t, http.MethodPost, pp, http.StatusConflict, pubMapping, false)
	})

	suite.Run("Legitimate PUT update to /attribute/{id} is accepted and changes are shown in returned object", func(t *testing.T) {
		pp, _ := url.Parse(fmt.Sprintf("%s/%s/%s", recordingServer.URL, "attribute", url.PathEscape(pubMappingId)))
		responseAttr := simpleMod(t, http.MethodPut, pp, http.StatusOK, []byte(`
			{
            "type": "MappedAttributes",
            "label": "Application Mappings",
			"sensor_urls": ["https://zoo"],
            "publishable":  false,
            "mappings": {
                "KEY": "NEWVALUE"
            }}
		`), true)

		assert.NotEqual(t, "", *(*responseAttr).Id)

		m := (*responseAttr).Mappings
		val, exists := (*m)["KEY"]
		assert.True(t, exists)
		assert.EqualValues(t, "NEWVALUE", val)

		dbAttr, err := persistence.FindAttributeByKey(db, pubMappingId)
		assert.Nil(t, err)

		dbM := (*dbAttr).(persistence.MappedAttributes).Mappings
		dbVal, exists := dbM["KEY"]
		assert.True(t, exists)
		assert.EqualValues(t, "NEWVALUE", dbVal)
	})

	suite.Run("New attribute given via PUT /attribute is rejected with 404", func(t *testing.T) {
		pp, _ := url.Parse(fmt.Sprintf("%s/%s", recordingServer.URL, "attribute"))
		simpleMod(t, http.MethodPut, pp, http.StatusNotFound, pubMapping, false)
	})

	suite.Run("PUT to /attribute/{id} is rejected with 404 if no matching record is found", func(t *testing.T) {
		pp, _ := url.Parse(fmt.Sprintf("%s/%s/%s", server.URL, "attribute", url.PathEscape("an_unknown_id")))
		simpleMod(t, http.MethodPut, pp, http.StatusNotFound, pubMapping, false)
	})

	suite.Run("Unknown id in PUT to /attribute/{id} is rejected with 400", func(t *testing.T) {
		pp, _ := url.Parse(fmt.Sprintf("%s/%s/%s", server.URL, "attribute", url.PathEscape("an_unknown_id")))
		simpleMod(t, http.MethodPut, pp, http.StatusNotFound, pubMapping, false)
	})

	suite.Run("Ill-formed attribute body in PUT to /attribute/{id} is rejected with 400", func(t *testing.T) {
		pp, _ := url.Parse(fmt.Sprintf("%s/%s/%s", recordingServer.URL, "attribute", url.PathEscape(pubMappingId)))
		simpleMod(t, http.MethodPut, pp, http.StatusBadRequest, bogusPubMapping, false)
	})

	suite.Run("Update of just 'publishable' and label values succeed using PATCH to /attribute/{id}", func(t *testing.T) {
		pp, _ := url.Parse(fmt.Sprintf("%s/%s/%s", recordingServer.URL, "attribute", url.PathEscape(pubMappingId)))
		responseAttr := simpleMod(t, http.MethodPatch, pp, http.StatusOK, []byte(`
			{
			"type": "MappedAttributes",
			"label": "New Application Mappings",
			"publishable": true
			}
		`), true)
		assert.EqualValues(t, true, (*(*responseAttr).Publishable))
		assert.Contains(t, (*(*responseAttr).SensorUrls), "https://zoo")
	})

	suite.Run("Update using PATCH to /attribute/{id} is rejected if no type is specified", func(t *testing.T) {
		pp, _ := url.Parse(fmt.Sprintf("%s/%s/%s", recordingServer.URL, "attribute", url.PathEscape(pubMappingId)))
		simpleMod(t, http.MethodPatch, pp, http.StatusBadRequest, []byte(`
			{
			"label": "Application M",
			"publishable": false
			}
		`), false)
	})

	suite.Run("Update using PATCH to /attribute/{id} is rejected for unsupported type", func(t *testing.T) {
		pp, _ := url.Parse(fmt.Sprintf("%s/%s/%s", recordingServer.URL, "attribute", url.PathEscape((*loc).GetMeta().Id)))
		simpleMod(t, http.MethodPatch, pp, http.StatusBadRequest, []byte(`
			{
			"type": "LocationAttributes",
			"label": "My New Location",
			"publishable": true
			}
		`), false)
	})

	suite.Run("Removal using DELETE /to/attribute/{id} succeeds for unknown ID", func(t *testing.T) {
		pp, _ := url.Parse(fmt.Sprintf("%s/%s/%s", recordingServer.URL, "attribute", url.PathEscape("fooo")))
		simpleMod(t, http.MethodDelete, pp, http.StatusOK, nil, false)
	})

	suite.Run("Removal using DELETE /to/attribute/{id} succeeds for known ID; record is returned", func(t *testing.T) {
		pp, _ := url.Parse(fmt.Sprintf("%s/%s/%s", recordingServer.URL, "attribute", url.PathEscape((*loc).GetMeta().Id)))
		responseAttr := simpleMod(t, http.MethodDelete, pp, http.StatusOK, nil, true)
		assert.EqualValues(t, "My Location", (*(*responseAttr).Label))

		dbAttr, err := persistence.FindAttributeByKey(db, (*loc).GetMeta().Id)
		assert.Nil(t, err)
		assert.Nil(t, *dbAttr)
	})

	// shutdown
	recordingServer.Finish()
}
