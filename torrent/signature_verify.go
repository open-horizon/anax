package torrent

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/golang/glog"
)

func verify(pubKeyFile string, signature string, image *os.File) (bool, error) {
	pubKeyData, err := ioutil.ReadFile(pubKeyFile)
	if err != nil {
		return false, err
	}

	block, _ := pem.Decode(pubKeyData)
	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return false, err
	}

	glog.Infof("Using RSA pubkey: %v", publicKey)

	if decoded, err := base64.StdEncoding.DecodeString(signature); err != nil {
		return false, fmt.Errorf("Error decoding base64 signature: %v, for image: %v. Error: %v", signature, image.Name(), err)
	} else {

		hasher := sha256.New()
		if _, err := io.Copy(hasher, image); err != nil {
			return false, err
		} else {
			if err := rsa.VerifyPSS(publicKey.(*rsa.PublicKey), crypto.SHA256, hasher.Sum(nil), decoded, nil); err != nil {
				return false, err
			} else {
				return true, nil
			}
		}
	}
}
