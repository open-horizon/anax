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
	"github.com/open-horizon/anax/apicommon"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/worker"
	"github.com/open-horizon/rsapss-tool/listkeys"
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

func Test_API_attribute_Suite(suite *testing.T) {
	dir, db, err := setup()
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

	_, err = persistence.SaveOrUpdateAttribute(db, &persistence.UserInputAttributes{
		Meta: &persistence.AttributeMeta{
			Id:          "",
			SensorUrls:  []string{sensorUrl},
			Label:       "Defs",
			HostOnly:    &bF,
			Publishable: &bF,
			Type:        "UserInputAttributes",
		},
		Mappings: map[string]interface{}{
			"SAMPLE_INTERVAL": "5s",
		},
	}, "", false)
	assert.Nil(suite, err)

	// we construct our own API instance so we can set route facts
	api := &API{
		Manager: worker.Manager{
			Config: &config.HorizonConfig{
				Edge: config.Config{
					UserPublicKeyPath: dir}},
			Messages: nil,
		},

		db:          db,
		pm:          nil,
		bcState:     make(map[string]map[string]apicommon.BlockchainState),
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

	// setup: PUT x509 cert to anax for evaluation later
	testKeyName := "Horizon-2111fe38d0aad1887dec4e1b7fb5e083fde3a393-public.pem"

	putURL, _ := url.Parse(fmt.Sprintf("%s/%s/%s", server.URL, "trust", testKeyName))
	req, err := http.NewRequest(http.MethodPut, putURL.String(), bytes.NewReader([]byte(`-----BEGIN CERTIFICATE-----
MIIFJTCCAw2gAwIBAgIUIRH+ONCq0Yh97E4bf7Xgg/3jo5MwDQYJKoZIhvcNAQEL
BQAwPDEQMA4GA1UEChMHSG9yaXpvbjEoMCYGA1UEAwwfZGV2ZWxvcG1lbnRAYmx1
ZWhvcml6b24ubmV0d29yazAeFw0xNzEyMDgwNTIxMzhaFw0yMDAxMDcxNzIxMzVa
MDwxEDAOBgNVBAoTB0hvcml6b24xKDAmBgNVBAMMH2RldmVsb3BtZW50QGJsdWVo
b3Jpem9uLm5ldHdvcmswggIhMA0GCSqGSIb3DQEBAQUAA4ICDgAwggIJAoICAAsN
jfwgD1eGct8Xewl7sk3qF1xPgovZPDi6OQxGU4VuOg15LyfS6hYK83kx205BMzb+
eS1lvP2eLUD/73iheJ5dTUGb/L8DvywF/cYYROUxjxCUMYEKTlM6Y9s4Amyc21Fh
NGH+ADpP9JieYA1A+zwqY+g5aa7/O5JmAIln1RBlNWPH84Qh5iq/jyY/uUDpJJzm
Xd2XgCSR9hDagJQzJ1b6Y6d3wB+t7jXJL4usVBR9k+IjKPT9fTDlH85modWKA+fx
jXD29W2iYcKR36QGJAf4+uW6j1ZHPqzDfnbuy5NCegWAFOnUZFo9TwpwMRfBzH9w
zyaJvuuTwcW5JDRy+SJFuRWthLChIvZCC8RignFM05oFUjBTnZFWfOYVdioe3mqp
15LpH4Rr11XvR8ZwFpl1s9zeugwQZAa9vnTzrCSOnqQ4JBZ6XrtuqsEwM+2NonCQ
GX/B4ldB4QchiIys4PN+Qv35BCEYuCEvXpr3wv9Lnq4tkUF7sTnilMSYuTASCI1p
leE1Bb+Iz4pwi0R4UmtAKlNsUjtmXDKNvIvAjMfLkFwWhoelsFXFdsi7CXGJpsyo
y8el5I2ZF9HlZCla4L63Ye/vaAee6qXY+0Bt5Vb/++885d5Zq07ymJBbf18si0uK
q7JSw4YB3ol3W5r8q3vxKQFssVl2DJpWxNm4GgaFAgMBAAGjIDAeMA4GA1UdDwEB
/wQEAwIHgDAMBgNVHRMBAf8EAjAAMA0GCSqGSIb3DQEBCwUAA4ICAQAI3qc5KCnJ
cxrAVHUVlEQ34L2g1w3OB3Y2IhzrDPwfZWyT/IUq4J2yIjgMt2UzfHlJD/V8cunb
Hsz41pxr+Oqifnzj0uTXctX8eS0EsLYu6OG0nhgrx2Z6Q9sHk6GvWYfDHNfJ1ETo
SpmcN1F2Ro/ngqt3N5nYsKYQub/x4VozK3FI1Lr245+6AF5BJBfZfZtCukNOgOnz
5jgRJqNE2n5ADEwwIf0lsAckBM6uMMIWzqYRCZQe5n+h8e8avk8+9MOW5EKmExvq
9TundbWDSOxJv28i8sEp5apVKWHgTCaQNb/xSXDEruKUQVDfm80F2HZPsQdUsFrK
MyOV+rLKymN9UJxbbszi02X7q/+hVH7eCn4UJ970AgVKRTREOPwzZyQHBvUymnZQ
1Gm/I8Z+zWcbpViYzNkmCdaQTVnSo60ewNDdeDBcbDyvJLs9VutQgVooHP5u2iWe
5lZQlr6OBX+B5LRujJBNKD58quwAsKNFbuOx+yTNS4NbR9Gf4aQj5neUA2UPn+62
RyutzM1amwOzzalhBcHJQaTiHR0QKQWyvvzlT3vDyg1DBZC2ReyDSmkT7CkJoC0O
vTLlpah1Y8Dvd1Mg6DorvN7eHb+R9pRYz6m/ll84KeLHyX+ml9Yj9Xem+H7MMYh7
6OPMAFGj9NP8jRVx/m71mKa1rIqqa+Wpew==
-----END CERTIFICATE-----`)))
	assert.Nil(suite, err)
	assert.NotNil(suite, req)

	client := http.Client{}
	resp, err := client.Do(req)
	assert.Nil(suite, err)

	if resp.StatusCode-399 > 0 {
		suite.Fatalf("Failure putting cert: %v", resp)
	}

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
			"type": "UserInputAttributes",
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
            "type": "UserInputAttributes",
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

		dbM := (*dbAttr).(persistence.UserInputAttributes).Mappings
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
			"type": "UserInputAttributes",
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

	suite.Run("Upload using PUT /publickey/{filename} succeeds with x509 certificate", func(t *testing.T) {
		pp, _ := url.Parse(fmt.Sprintf("%s/%s/%s", recordingServer.URL, "publickey", "foo.pem"))
		client := http.Client{}

		payload := []byte(`-----BEGIN CERTIFICATE-----
MIICHTCCAYagAwIBAgIUYqKtvgqzrCoAUi0aX6WViO/RpOYwDQYJKoZIhvcNAQEL
BQAwOjEeMBwGA1UEChMVUlNBUFNTIFRvb2wgdGVzdCBjZXJ0MRgwFgYDVQQDEw9k
ZXZlbG9wbWVudC1vbmUwHhcNMTcxMjAyMTk1ODMyWhcNMjcxMTMwMDc1ODMyWjA6
MR4wHAYDVQQKExVSU0FQU1MgVG9vbCB0ZXN0IGNlcnQxGDAWBgNVBAMTD2RldmVs
b3BtZW50LW9uZTCBnzANBgkqhkiG9w0BAQEFAAOBjQAwgYkCgYEAwMyVHvDKw6Th
bvLiU9qyi6NKYnNH5LA58ukUbMynkRers/HsKfc06Mf2XCwKO6v10QqLNyzMX+2q
F3T2NpYr8Jru0tAr43Jp8Tq2RrR+5sMvi7OVClieZz2XmaFqIDKH0CcpoKX18lQA
ZuwJyLgNoR0I5qhaqcXIxYtkS3Om4WsCAwEAAaMgMB4wDgYDVR0PAQH/BAQDAgeA
MAwGA1UdEwEB/wQCMAAwDQYJKoZIhvcNAQELBQADgYEAcs5DAT+frZfJsoSKEMOu
WJh0S/UVYC+InMv9iUnPF3f0KjVBXTE45GDG1zxY6SFLpOVskNp9mMkH9PLqDMrb
kWsF7xOtgBrzIaibDeEhhcQvvHb6Yct1bSgYxWpS1oGKicXA9PFyXxigUW2e8+DH
SoxItJkxfl2adAjY2DVzdhY=
-----END CERTIFICATE-----`)

		req, err := http.NewRequest(http.MethodPut, pp.String(), bytes.NewReader(payload))
		assert.Nil(t, err)
		assert.NotNil(t, req)

		glog.Infof("Request: %v", req)
		resp, err := client.Do(req)
		glog.Infof("Response: %v", resp)

		b, err := handleResp(resp, http.StatusOK)
		assert.Nil(t, err)

		if resp.StatusCode-400 > 0 {
			t.Errorf("Unexpected status code returned from API: %v. Response data: %v", resp.StatusCode, b)
		}
	})

	suite.Run("GET /trust returns array of cert / key filenames", func(t *testing.T) {
		pp, _ := url.Parse(fmt.Sprintf("%s/%s", recordingServer.URL, "trust"))
		resp, err := http.Get(pp.String())
		assert.Nil(t, err)

		b, err := handleResp(resp, http.StatusOK)
		assert.Nil(t, err)

		var certList map[string][]string
		err = json.Unmarshal(b, &certList)
		assert.Nil(t, err)

		found := false
		for _, certName := range certList["pem"] {
			if certName == testKeyName {
				found = true
			}
		}

		if !found {
			t.Errorf("Expected key name not found in returned list: %v. Complete return object: %v", testKeyName, certList)
		}
	})

	suite.Run("GET /trust?verbose=true returns array of cert objects with detail", func(t *testing.T) {
		pp, _ := url.Parse(fmt.Sprintf("%s/%s?verbose=true", recordingServer.URL, "trust"))
		resp, err := http.Get(pp.String())
		assert.Nil(t, err)

		b, err := handleResp(resp, http.StatusOK)
		assert.Nil(t, err)

		var certList map[string][]listkeys.KeyPairSimple
		err = json.Unmarshal(b, &certList)
		assert.Nil(t, err)

		assert.EqualValues(t, "21:11:fe:38:d0:aa:d1:88:7d:ec:4e:1b:7f:b5:e0:83:fd:e3:a3:93", certList["pem"][0].SerialNumber)
	})

	// shutdown
	recordingServer.Finish()
}
