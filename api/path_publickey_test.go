//go:build unit
// +build unit

package api

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

func init() {
	flag.Set("alsologtostderr", "true")
	flag.Set("v", "7")
	// no need to parse flags, that's done by test framework
}

// Create output public key object
func Test_PKTest1(t *testing.T) {

	cfg := getBasicConfig()
	myKeyFile := "testkey1.pem"
	var keyBytes []byte

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Errorf("Could not generate private key, error %v", err)
	}
	publicKey := &privateKey.PublicKey

	pubFilepath := path.Join(cfg.UserPublicKeyPath(), myKeyFile)
	if _, ferr := os.Stat(pubFilepath); !os.IsNotExist(ferr) {
		os.Remove(pubFilepath)
	}

	pubFile, err := os.Create(pubFilepath)
	if err != nil {
		t.Errorf("could not create public key file %v, error %v", pubFilepath, err)
	} else if err := pubFile.Chmod(0600); err != nil {
		t.Errorf("Could not chmod public key file %v, error %v", pubFilepath, err)
	} else if pubKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey); err != nil {
		t.Errorf("Could not marshal public key, error %v", err)
	} else {
		pubEnc := &pem.Block{
			Type:    "PUBLIC KEY",
			Headers: nil,
			Bytes:   pubKeyBytes}
		if err := pem.Encode(pubFile, pubEnc); err != nil {
			t.Errorf("Could not encode public key to file, error %v", err)
		} else {
			pubFile.Close()
		}
	}

	if pf, err := os.Open(pubFilepath); err != nil {
		t.Errorf("Unable to open public key file %v, error: %v", pubFilepath, err)
	} else if pubBytes, err := ioutil.ReadAll(pf); err != nil {
		t.Errorf("Unable to read public key file %v, error: %v", pubFilepath, err)
	} else {
		keyBytes = pubBytes
	}

	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)

	errHandled := UploadPublicKey(myKeyFile, keyBytes, cfg, errorhandler)
	if errHandled {
		t.Errorf("unexpected error creating key %v", myError)
	}

	filename, err := FindPublicKeyForOutput(myKeyFile, cfg)
	if err != nil {
		t.Errorf("unexpected error finding key %v", err)
	} else if filename != pubFilepath {
		t.Errorf("wrong file name returned: %v", filename)
	}

	keys, err := FindPublicKeysForOutput(cfg, false)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	} else if len(keys) != 1 {
		t.Errorf("returned map should only have 1 key %v", keys)
	} else {
		found := false
		for _, k := range keys["pem"] {
			if k == myKeyFile {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("did not find test key in returned map: %v", keys)
		}
	}

	errHandled = DeletePublicKey(myKeyFile, cfg, errorhandler)
	if errHandled {
		t.Errorf("unexpected error %v", myError)
	}

	keys, err = FindPublicKeysForOutput(cfg, false)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	} else if len(keys) != 1 {
		t.Errorf("returned map should only have 1 key %v", keys)
	} else {
		found := false
		for _, k := range keys["pem"] {
			if k == myKeyFile {
				found = true
				break
			}
		}
		if found {
			t.Errorf("should not find test key in returned map: %v", keys)
		}
	}

}
