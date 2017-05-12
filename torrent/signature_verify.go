package torrent

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
)

func verify(pubKeyFile string, signature string, image *os.File) (bool, error) {

	glog.V(3).Infof("Verifying signature %v of image %v", signature, image.Name())

	// Read the file content into the hash function once.
	hasher := sha256.New()
	if _, err := io.Copy(hasher, image); err != nil {
		return false, fmt.Errorf("Unable to copy image file content into hash function for image: %v, error: %v", image.Name(), err)
	}

	// Decode the signature into its binary form.
	var signatureBytes []byte
	if decoded, err := base64.StdEncoding.DecodeString(signature); err != nil {
		return false, fmt.Errorf("Unable to base64 decode signature %v, error: %v", signature, err)
	} else {
		signatureBytes = decoded
	}

	// Compute the public key directory based on the configured platform public key file location.
	pubKeyDir := pubKeyFile[:strings.LastIndex(pubKeyFile, "/")]

	// Grab all PEM files from that location and try to verify the signature against each one.
	if pemFiles, err := getPemFiles(pubKeyDir); err != nil {
		return false, err
	} else if checkAllKeys(pubKeyDir, pemFiles, hasher, signatureBytes, image) {
		return true, nil
	}

	// Compute the public key directory for user keys based on the configured platform public key file location.
	pubKeyDir = pubKeyDir + config.USERKEYDIR

	// Grab all PEM files from that location and try to verify the signature against each one.
	if pemFiles, err := getPemFiles(pubKeyDir); err != nil {
		return false, err
	} else if checkAllKeys(pubKeyDir, pemFiles, hasher,signatureBytes, image) {
		return true, nil
	}

	return false, fmt.Errorf("No keys found to verify image %v with signature %v", image.Name(), signature)

}

func checkAllKeys(pubKeyDir string, pemFiles []os.FileInfo, hasher hash.Hash, signatureBytes []byte, image *os.File) bool {
	for _, fileInfo := range pemFiles {
		fName := pubKeyDir + "/" + fileInfo.Name()
		if publicKey := isValidPublickKey(fName); publicKey == nil {
			continue
		} else {
			// Given a valid public key file, try to verify the signature of the image file.
			glog.V(3).Infof("Using RSA pubkey file: %v and key: %v", fName, publicKey)

			if err := rsa.VerifyPSS(publicKey.(*rsa.PublicKey), crypto.SHA256, hasher.Sum(nil), signatureBytes, nil); err == nil {
				return true
			} else {
				glog.Warningf("Unable to verify signature of %v using pubkey file: %v and key: %v, error %v", image.Name(), fName, publicKey, err)
			}
		}
	}
	return false
}

func isValidPublickKey(fName string) interface{} {
	if pubKeyData, err := ioutil.ReadFile(fName); err != nil {
		glog.Warningf("Unable to read key file: %v, error: %v", fName, err)
	} else if block, _ := pem.Decode(pubKeyData); block == nil {
		glog.Warningf("Unable to decode key file: %v as PEM encoded file", fName)
	} else if publicKey, err := x509.ParsePKIXPublicKey(block.Bytes); err != nil {
		glog.Warningf("Unable to parse key file: %v, as a public key, error: %v", fName, err)
	} else {
		return publicKey
	}
	return nil
}

func getPemFiles(homePath string) ([]os.FileInfo, error) {
	res := make([]os.FileInfo, 0, 10)

	if files, err := ioutil.ReadDir(homePath); err != nil && !os.IsNotExist(err) {
		return nil, errors.New(fmt.Sprintf("Unable to get list of PEM files in %v, error: %v", homePath, err))
	} else if os.IsNotExist(err) {
		return res, nil
	} else {
		for _, fileInfo := range files {
			if strings.HasSuffix(fileInfo.Name(), ".pem") && !fileInfo.IsDir() {
				res = append(res, fileInfo)
			}
		}
		return res, nil
	}
}
