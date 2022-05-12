//go:build integration && go1.17
// +build integration,go1.17

package api

import (
	"bytes"
	"encoding/json"
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
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/worker"
	"github.com/open-horizon/rsapss-tool/listkeys"
	"github.com/stretchr/testify/assert"
)

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

	sp := persistence.ServiceSpec{Url: "https://bluehorizon.network/doc/locsampler", Org: "myorg"}
	sps := new(persistence.ServiceSpecs)
	sps.AppendServiceSpec(sp)

	bF := false

	userInput, err := persistence.SaveOrUpdateAttribute(db, &persistence.UserInputAttributes{
		Meta: &persistence.AttributeMeta{
			Id:          "",
			Label:       "Defs",
			HostOnly:    &bF,
			Publishable: &bF,
			Type:        "UserInputAttributes",
		},
		ServiceSpecs: sps,
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
			Messages: make(chan events.Message, 20),
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

	// The following cert generated via: openssl req -newkey rsa:2048 -nodes -addext keyUsage=digitalSignature -addext basicConstraints=critical,CA:false -keyout key.pem -days 1000 -x509 -out certificate.pem
	// then copy/paste content of certificate.pem in here

	req, err := http.NewRequest(http.MethodPut, putURL.String(), bytes.NewReader([]byte(`-----BEGIN CERTIFICATE-----
MIIDcDCCAligAwIBAgIUD7YETXojmUUJNwF3VNAKLYC0jJIwDQYJKoZIhvcNAQEL
BQAwOjELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAk5ZMQwwCgYDVQQKDANJQk0xEDAO
BgNVBAMMB2libS5jb20wHhcNMjAwMTA5MTc1MDIxWhcNMjIxMDA1MTc1MDIxWjA6
MQswCQYDVQQGEwJVUzELMAkGA1UECAwCTlkxDDAKBgNVBAoMA0lCTTEQMA4GA1UE
AwwHaWJtLmNvbTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAJXpQHxV
5ckBoa/K62iswb9lzsO4tHQX1Bl957y1Y/hKVXyOvAa90Zy8xt7GY+NyvJTojEdf
Mz6RhY+43J97SiU0PRs3J73dYz7Kb64e1Zt2rjYl9WZdTkVi4lhZgNMiw1KKndht
WMPJU2SLDZk5pS3EhwRrJo3tP/NOZiSfOMo9HBvJSUOVyUaKrxSKyZxrwx/Omm9p
d+Dlip32dd1yqfq3XVDHGMtJQNJYOpObXiIQPTGuh8SCMnDr5/y3zqWoHILPCkrb
/vK20wfam8P5XXaxZJFTpV07rqqUTWIbfJ94yFX0ACWsjQapq84POVxJIut/rqBk
hosSYXBElM/m158CAwEAAaNuMGwwHQYDVR0OBBYEFF/YMJkse4F3SGHjzGy/oIsn
VXW/MB8GA1UdIwQYMBaAFF/YMJkse4F3SGHjzGy/oIsnVXW/MA8GA1UdEwEB/wQF
MAMBAf8wCwYDVR0PBAQDAgeAMAwGA1UdEwEB/wQCMAAwDQYJKoZIhvcNAQELBQAD
ggEBAHMUwqBf6h6W9zKQp+6mvj9Kdx0R6sxc7HtLA1JOrO4BVRARHg7VGvgdwKnK
RWNjw/i71WUD/JKnnqnzV76e7tEUAbvjjU/wQomklR9cqHx27EX/D3ClteTYhe7y
mRR3Vj2NwMNiDrDNPCKzGguaVbf0gntleMFSvd1D7hKNkCDhyUkTJ+6hIsJqhDoh
psdYUa44KM8ue7oRUz4F0CxLeyjO83tyalWeAgu5nRRd/UkGsdX0kCpHiQy2VaG+
nF3ewq1o585M21iHNHsS7BHjYnhE7zFyGqNpH6l+xEkeWgsCk/DxhX5JQ9PHKp0d
JMEaRpo4HBqI78UOex31m3MMA2s=
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

	_, err = persistence.SaveNewExchangeDevice(db, "device-22", "tokenval", "Device 22", "device", false, "myorg", ".*", persistence.CONFIGSTATE_CONFIGURING, persistence.SoftwareVersion{persistence.AGENT_VERSION: "1.0.0"})
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

	suite.Run("Single return from GET /attribute/{id}", func(t *testing.T) {
		// querying w/ sensorUrl

		pp, _ := url.Parse(fmt.Sprintf("%s/%s/%s", recordingServer.URL, "attribute", url.PathEscape((*userInput).GetMeta().Id)))
		attrs := simpleGET(t, pp, http.StatusOK, true)
		assert.EqualValues(t, 1, len(attrs))
	})

	suite.Run("OK returned for HEAD /attribute/{id}", func(t *testing.T) {
		pp, _ := url.Parse(fmt.Sprintf("%s/%s/%s", recordingServer.URL, "attribute", url.PathEscape((*userInput).GetMeta().Id)))
		res, err := http.Head(pp.String())
		assert.Nil(t, err)
		assert.NotEqual(t, 0, res.ContentLength)
	})

	pubMapping := []byte(`{
			"type": "UserInputAttributes",
			"label": "Application Mappings",
			"publishable":	false,
			"service_specs": [
			    {"url": "https://foo", "organization": "myorg"},
			    {"url": "https://goo", "organization": "myorg2"}
			],
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
           "publishable":  false,
			"service_specs": [
			    {"url": "https://zoo", "organization": "myorg"}
			],
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

	suite.Run("New attribute given via PUT /attribute is rejected with 405", func(t *testing.T) {
		pp, _ := url.Parse(fmt.Sprintf("%s/%s", recordingServer.URL, "attribute"))
		simpleMod(t, http.MethodPut, pp, http.StatusMethodNotAllowed, pubMapping, false)
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
		assert.Contains(t, (*(*responseAttr).ServiceSpecs), persistence.ServiceSpec{Url: "https://zoo", Org: "myorg"})
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

	suite.Run("Removal using DELETE /to/attribute/{id} succeeds for unknown ID", func(t *testing.T) {
		pp, _ := url.Parse(fmt.Sprintf("%s/%s/%s", recordingServer.URL, "attribute", url.PathEscape("fooo")))
		simpleMod(t, http.MethodDelete, pp, http.StatusOK, nil, false)
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

		// Obtain cert serial number via: openssl x509 -in certificate.pem -text
		// copy/paste the value of the Serial Number: field in here

		assert.EqualValues(t, "0f:b6:04:4d:7a:23:99:45:09:37:01:77:54:d0:0a:2d:80:b4:8c:92", certList["pem"][0].SerialNumber)
	})

	// shutdown
	recordingServer.Finish()
}
