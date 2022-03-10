// +build integration
// +build go1.9

package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const listenOn = "127.0.0.1"

// generated with: ALTNAME=DNS:localhost,DNS:localhost.localdomain,IP:127.0.0.1 openssl req -newkey rsa:1024 -nodes -keyout collaborators-test-key.pem -x509 -days 36500 -out collaborators-test-cert.pem -subj "/C=US/ST=UT/O=Salt Lake City/CN=localhost.localdomain" -config /etc/ssl/openssl.cnf
// requires: /etc/ssl/openssl.cnf w/ section '[ v3_ca ]' having keyUsage = digitalSignature, keyEncipherment ; subjectAltName = $ENV::ALTNAME

// verify with: openssl req -in domain.csr -text -noout

const collaboratorsTestCert = `-----BEGIN CERTIFICATE-----
MIICuzCCAiSgAwIBAgIJAN7JXdbhU4bsMA0GCSqGSIb3DQEBCwUAMFMxCzAJBgNV
BAYTAlVTMQswCQYDVQQIDAJVVDEXMBUGA1UECgwOU2FsdCBMYWtlIENpdHkxHjAc
BgNVBAMMFWxvY2FsaG9zdC5sb2NhbGRvbWFpbjAgFw0xNzA5MTAyMjAwNTlaGA8y
MTE3MDgxNzIyMDA1OVowUzELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAlVUMRcwFQYD
VQQKDA5TYWx0IExha2UgQ2l0eTEeMBwGA1UEAwwVbG9jYWxob3N0LmxvY2FsZG9t
YWluMIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQCju4NREasdEKNIRB2bghrh
wcc7A5nW6TN8WG/LuDKzZbodLNbivB3BWeUNi64rn9TB5XhpMom0nOQRo4uKPuJd
2s8cKeOLpcR7Al6CUZkkrFn+i7BxShnnvR22wBdbfR+nXwNRc8/kq073U6YmLq/l
G59+b2kWmkNNgTfJbARYPwIDAQABo4GUMIGRMB0GA1UdDgQWBBTrA+IuPqqVJuhH
muQR5Krpw5GWGDAfBgNVHSMEGDAWgBTrA+IuPqqVJuhHmuQR5Krpw5GWGDAPBgNV
HRMBAf8EBTADAQH/MAsGA1UdDwQEAwIFoDAxBgNVHREEKjAogglsb2NhbGhvc3SC
FWxvY2FsaG9zdC5sb2NhbGRvbWFpbocEfwAAATANBgkqhkiG9w0BAQsFAAOBgQAu
JJZX6caJ31h1hG4X3rWTaX0K/8nNXq3XtYGPP88K0uJMWGrtTs5XLlgTxhYCo6JH
2D2YUnmSLDfIEhFDUiQGBSfABR0YDAe/3nC8q8Gk5m4VGHmdszY/EbzCdl7Nf/MZ
vSDcr4UB3UEwhhdfJ+hzPzSSPknSmUczZ9UoVvk48Q==
-----END CERTIFICATE-----`
const collaboratorsTestKey = `-----BEGIN PRIVATE KEY-----
MIICdgIBADANBgkqhkiG9w0BAQEFAASCAmAwggJcAgEAAoGBAKO7g1ERqx0Qo0hE
HZuCGuHBxzsDmdbpM3xYb8u4MrNluh0s1uK8HcFZ5Q2Lriuf1MHleGkyibSc5BGj
i4o+4l3azxwp44ulxHsCXoJRmSSsWf6LsHFKGee9HbbAF1t9H6dfA1Fzz+SrTvdT
piYur+Ubn35vaRaaQ02BN8lsBFg/AgMBAAECgYBufulMGKRl5QiMiIuCmvcRS/js
Nq3nf1GjpPstfI2azBgiAFS0h0d9aPFPhuhvwFmQ0Q/FzrloDklMLhbJoU6Z++kz
+RLy3rKX4zv1Sv5R4M0gBNRzr10EzCmJS5rOOrwPABLAfEnlagV4T4sDARY+V0Iz
N2u50DNoben+cQsywQJBAM9lzhbC0J1I4C17iIIOLsUIQuruWxLJ0iiYXS+zxPeH
marEe70sh6A7w6H9xZ4vRxdtr1xQYbuFjfqGUSbzeEcCQQDKGiV5ien0SQBGwLTq
CCR4B52/5VhvG11S3oyKLvlr54D1LcSb7eHe/6XDAMhnbBWFr04+SFMYr3t+5l8t
gpRJAkAAyBpxvYQ5w4eMxFVsYA9PEMvnxMQ1Guue2YwoXN4WLL2ohhsNSHiuYutG
1gUDppv2+6PYjjkAEu3JDu6JXguLAkEApLrPFNOu2CiwivsD+0YLw7IhiIo9nMJn
POadEvza3HLkD/PwL1CkLImf6OQ4dOQKXt7XHbkB0jsmo/bOWV/30QJAHXH3aoKT
P3RnSaBJcRIbi8VEGBP4DKSV5wky/KpGSRoNVqONuPnUAHOSyNGAqi402aq/tBLk
3CfGb5vgRdDMJQ==
-----END PRIVATE KEY-----`

// generated with: ALTNAME=DNS:testo openssl req -newkey rsa:1024 -nodes -keyout collaborators-test-key.pem -x509 -days 36500 -out collaborators-test-cert.pem -subj "/C=US/ST=UT/O=Salt Lake City/CN=testo" -config /etc/ssl/openssl.cnf
// requires: /etc/ssl/openssl.cnf w/ section '[ v3_ca ]' having keyUsage = digitalSignature, keyEncipherment ; subjectAltName = $ENV::ALTNAME

// verify with: openssl req -in domain.csr -text -noout

const collaboratorsOtherTestCert = `-----BEGIN CERTIFICATE-----
MIICeDCCAeGgAwIBAgIJALUrEoK32k14MA0GCSqGSIb3DQEBCwUAMEMxCzAJBgNV
BAYTAlVTMQswCQYDVQQIDAJVVDEXMBUGA1UECgwOU2FsdCBMYWtlIENpdHkxDjAM
BgNVBAMMBXRlc3RvMCAXDTE3MDkxMDIxNDAzNVoYDzIxMTcwODE3MjE0MDM1WjBD
MQswCQYDVQQGEwJVUzELMAkGA1UECAwCVVQxFzAVBgNVBAoMDlNhbHQgTGFrZSBD
aXR5MQ4wDAYDVQQDDAV0ZXN0bzCBnzANBgkqhkiG9w0BAQEFAAOBjQAwgYkCgYEA
2yy2sJu5DSJl6Cbdc78IlrvL9GBJ55sjQPco0Yhqef/5y0I/OMXMPTYTfEOB1boz
ErbpZYu/O5PLLXC6J6Foqi8IKQm++yv7pzWUHRvgh7B4gv/vVrDF9XtggVmRCZ2G
q0NLGhI7GU2j5r1gxVlpSsxjJ9Tf9AvLWK4KlWGhhgMCAwEAAaNyMHAwHQYDVR0O
BBYEFI40iczJUPBB5FLj8SSFgWGI0JC5MB8GA1UdIwQYMBaAFI40iczJUPBB5FLj
8SSFgWGI0JC5MA8GA1UdEwEB/wQFMAMBAf8wCwYDVR0PBAQDAgWgMBAGA1UdEQQJ
MAeCBXRlc3RvMA0GCSqGSIb3DQEBCwUAA4GBAF5wX7P8SfGg+KlT4RYybBoXuIHz
1Z/a6SKdkdOe6UimwH5M2Jievbz7qpISRohIXfd+HRClx15XgqSlXduvBUieqk+a
BKx9kxNOWtep48m/1caJnsS6GTrtc18jB0CzGeGxeIL1cJftL9N0lUSjehbsYGmz
XseH1jRdJJGVGJw7
-----END CERTIFICATE-----`
const collaboratorsOtherTestKey = `-----BEGIN PRIVATE KEY-----
MIICeAIBADANBgkqhkiG9w0BAQEFAASCAmIwggJeAgEAAoGBANsstrCbuQ0iZegm
3XO/CJa7y/RgSeebI0D3KNGIann/+ctCPzjFzD02E3xDgdW6MxK26WWLvzuTyy1w
uiehaKovCCkJvvsr+6c1lB0b4IeweIL/71awxfV7YIFZkQmdhqtDSxoSOxlNo+a9
YMVZaUrMYyfU3/QLy1iuCpVhoYYDAgMBAAECgYEAxT/fhuAO0cBEYIMhyDqD20xW
CK/js1oOhzgo9zJDSVrTD1emmEyDPA9/x9Tlc1ko/824DZiQWWjwcQvDrUj5bIxo
662cdPyEJHAs4nDbZeN6EdMJIEwSmpQOXGwoaUQMKyXKhm43uDGJfGfhYwQ4iFQV
GDxY35H1cPDt7q/3/VkCQQD/WYW6o6uC5bamBW74dVVQ6h+UkVV4VpAg52c1ZLOr
kAEIWrxVpkFyyT5GO4pYxTG/MD25riN0lHm51OFmBWLXAkEA27ubSj6v4M+w2e2p
lh9n+SHP+wVgiMsa9xDJpHDbAHwMYSK9Wh1pVslSpVP2u/7ZNwRBBlfKBj/6O0wN
p9H8tQJAQabwvSXrqQIKzfDDsVnpj55CdF5RjVkkQXF9lbrIfynNOiqqFZNjbHHV
cxVH4r8ApVlv5VeiggzSpzbWpPZpjQJBALqjJYnwqQ85GixhVDRxRK017SR4MsC+
U48bsUp9mWdV9mXjThZm+PyAUDShluej1fiHInwywSSB3xfSx56OHCkCQQDsTAih
sh5/kd46V/jJbNBiporBuz/kVJTXrvFrZNnKgYv2BVC7jQfgOv6gkkQI4tHvuStE
DuvdvPhYKonxjjLv
-----END PRIVATE KEY-----`

func setupTesting(listenerCert string, listenerKey string, trustSystemCerts bool, t *testing.T) (string, net.Listener) {
	err := os.Setenv("GODEBUG", "1")
	if err != nil {
		t.Error(err)
	}

	dir, err := ioutil.TempDir("", "config-collaborators-")
	if err != nil {
		t.Error(err)
	}

	keyPath := filepath.Join(dir, "collaborators-test-key.pem")
	if err := ioutil.WriteFile(keyPath, []byte(collaboratorsTestKey), 0660); err != nil {
		t.Error(err)
	}

	// mostly for building up a CA cert path
	certPath := filepath.Join(dir, "collaborators-test-cert.pem")
	if err := ioutil.WriteFile(certPath, []byte(collaboratorsTestCert), 0660); err != nil {
		t.Error(err)
	}

	config := &HorizonConfig{
		Edge: Config{
			TrustSystemCACerts: trustSystemCerts,
			CACertsPath:        certPath,
			PublicKeyPath:      "",
		},
	}

	configBytes, err := json.Marshal(config)
	if err != nil {
		t.Error(err)
	}

	configPath := filepath.Join(dir, "config.json")
	if err := ioutil.WriteFile(configPath, configBytes, 0660); err != nil {
		t.Error(err)
	}

	// write listener key and cert
	listenerKeyPath := filepath.Join(dir, "listener-key.pem")
	if err := ioutil.WriteFile(listenerKeyPath, []byte(listenerKey), 0660); err != nil {
		t.Error(err)
	}

	listenerCertPath := filepath.Join(dir, "listener-cert.pem")
	if err := ioutil.WriteFile(listenerCertPath, []byte(listenerCert), 0660); err != nil {
		t.Error(err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/boosh", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("yup"))
	})

	listener, err := net.Listen("tcp", fmt.Sprintf("%s:0", listenOn))
	if err != nil {
		t.Error(err)
	}

	// setup https server
	go func() {
		err = http.ServeTLS(listener, mux, listenerCertPath, listenerKeyPath)
		if err != nil {
			t.Error(err)
		}
	}()

	return dir, listener
}

func cleanup(dir string, t *testing.T) {

	if err := os.RemoveAll(dir); err != nil {
		t.Error(err)
	}
}

func Test_HTTPClientFactory_Suite(t *testing.T) {
	timeoutS := uint(2)

	setupForTest := func(listenerCert, listenerKey string, trustSystemCerts bool) (*HorizonConfig, string, string) {
		dir, listener := setupTesting(listenerCert, listenerKey, trustSystemCerts, t)

		t.Logf("listening on %s", listener.Addr().String())

		cfg, err := Read(filepath.Join(dir, "config.json"))
		if err != nil {
			t.Error(nil)
		}
		return cfg, strings.Split(listener.Addr().String(), ":")[1], dir
	}

	cfg, port, dir := setupForTest(collaboratorsTestCert, collaboratorsTestKey, false)
	t.Run("HTTP client rejects trusted cert for wrong domain", func(t *testing.T) {

		client := cfg.Collaborators.HTTPClientFactory.NewHTTPClient(&timeoutS)
		// this'll fail b/c we're making a request to 127.0.1.1 but that isn't the CN or subjectAltName IP in the cert
		_, err := client.Get(fmt.Sprintf("https://%s:%s/boosh", "127.0.1.1", port))
		if err == nil {
			t.Error("Expected TLS error for sending request to wrong domain")
		}
	})

	t.Run("HTTP client accepts trusted cert for right domain", func(t *testing.T) {

		client := cfg.Collaborators.HTTPClientFactory.NewHTTPClient(&timeoutS)
		// all of these should pass b/c they are the subjectAltNames of the cert (either names or IPs) note that Golang doesn't verify the CA of the cert if it's localhost or an IP
		for _, dom := range []string{listenOn, "localhost"} {

			resp, err := client.Get(fmt.Sprintf("https://%s:%s/boosh", dom, port))

			if err != nil {
				t.Error("Unxpected error sending request to trusted domain", err)
			}

			if resp != nil {
				if resp.StatusCode != 200 {
					t.Errorf("Unexpected error from HTTP request (wanted 200). HTTP response status code: %v", resp.StatusCode)
				}

				content, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					t.Error("Unexpected error reading response from HTTP server", err)
				}

				if string(content) != "yup" {
					t.Error("Unexpected returned content from test")
				}
			}
		}

		cleanup(dir, t)
	})
	t.Run("HTTP client rejects untrusted cert", func(t *testing.T) {
		// need a new config and setup
		cfg, port, dir := setupForTest(collaboratorsOtherTestCert, collaboratorsOtherTestKey, false)

		client := cfg.Collaborators.HTTPClientFactory.NewHTTPClient(&timeoutS)
		// this should fail b/c even though we're sending a request to a trusted domain, the CA trust doesn't contain the cert
		_, err := client.Get(fmt.Sprintf("https://%s:%s/boosh", listenOn, port))
		if err == nil {
			t.Error("Expected TLS error for sending request to untrusted domain")
		}

		cleanup(dir, t)
	})

	t.Run("HTTP client trusts system certs", func(t *testing.T) {
		// important that the cert and key match for setup to succeed even though that's not what we're testing
		setupForTest(collaboratorsTestCert, collaboratorsTestKey, true)
		cleanup(dir, t)

		// if we got this far we're ok (an error gets raised during setup if the system ca certs couldn't be loaded)
	})
}

func Test_KeyFileNamesFetcher_Suite(t *testing.T) {
	setupForTest := func(listenerCert, listenerKey string, trustSystemCerts bool) (string, *HorizonConfig) {
		dir, _ := setupTesting(listenerCert, listenerKey, trustSystemCerts, t)

		cfg, err := Read(filepath.Join(dir, "config.json"))
		if err != nil {
			t.Error(nil)
		}

		err = os.Setenv("HZN_VAR_BASE", dir)
		if err != nil {
			t.Error(err)
		}

		cfg.Edge.PublicKeyPath = filepath.Join(dir, "/trusted/keyfile1.pem")
		return dir, cfg
	}

	dir, cfg := setupForTest(collaboratorsTestCert, collaboratorsTestKey, false)

	t.Run("Test zero *.pem files under user key path", func(t *testing.T) {

		fnames, err := cfg.Collaborators.KeyFileNamesFetcher.GetKeyFileNames(cfg.Edge.PublicKeyPath, cfg.UserPublicKeyPath())
		if err != nil {
			t.Error("Got error but should not.")
		}
		if len(fnames) != 0 {
			t.Errorf("Number of files should be 0 but got %v.", len(fnames))
		}
	})

	t.Run("Test filter out non .pem files", func(t *testing.T) {
		userKeyPath := cfg.UserPublicKeyPath()
		if err := os.Mkdir(userKeyPath, 0777); err != nil {
			t.Error(err)
		}

		if err := os.Mkdir(dir+"/trusted", 0777); err != nil {
			t.Error(err)
		}

		nonpemfile1 := filepath.Join(dir, "/trusted/non_pem_file1")
		if err := ioutil.WriteFile(nonpemfile1, []byte("hello from non pem file 1"), 0660); err != nil {
			t.Error(err)
		}

		nonpemfile2 := filepath.Join(userKeyPath, "/non_pem_file2")
		if err := ioutil.WriteFile(nonpemfile2, []byte("hello from non pem file 2"), 0660); err != nil {
			t.Error(err)
		}

		fnames, err := cfg.Collaborators.KeyFileNamesFetcher.GetKeyFileNames(cfg.Edge.PublicKeyPath, userKeyPath)
		if err != nil {
			t.Error("Got error but should not.")
		}
		if len(fnames) != 0 {
			t.Errorf("Number of files should be 0 but got %v.", len(fnames))
		}
	})

	t.Run("Test getting pem files", func(t *testing.T) {
		userKeyPath := cfg.UserPublicKeyPath()

		pemfile1 := filepath.Join(dir, "/trusted/realfile1.pem")
		if err := ioutil.WriteFile(pemfile1, []byte("hello from pem file 1"), 0660); err != nil {
			t.Error(err)
		}
		pemfile2 := filepath.Join(dir, "/trusted/realfile2.pem")
		if err := ioutil.WriteFile(pemfile2, []byte("hello from pem file 2"), 0660); err != nil {
			t.Error(err)
		}
		pemfile3 := filepath.Join(userKeyPath, "realfile3.pem")
		if err := ioutil.WriteFile(pemfile3, []byte("hello from pem file 3"), 0660); err != nil {
			t.Error(err)
		}
		pemfile4 := filepath.Join(userKeyPath, "realfile4.pem")
		if err := ioutil.WriteFile(pemfile4, []byte("hello from pem file 4"), 0660); err != nil {
			t.Error(err)
		}

		fnames, err := cfg.Collaborators.KeyFileNamesFetcher.GetKeyFileNames(cfg.Edge.PublicKeyPath, userKeyPath)
		if err != nil {
			t.Error("Got error but should not.")
		}
		if len(fnames) != 4 {
			t.Errorf("Number of files should be 4 but got %v.", len(fnames))
		} else {
			for _, fn := range fnames {
				if !strings.Contains(fn, "realfile") {
					t.Errorf("File %v should not be returned as a pem file.", fn)
				}
			}
		}
	})

	t.Run("Cleaning up", func(t *testing.T) {
		cleanup(dir, t)
	})
}
