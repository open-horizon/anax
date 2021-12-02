package cutil

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/edge-sync-service/common"
	"hash"
	"io"
	"os"
)

func VerifyDataSig(dataReader io.Reader, publicKey string, signature string, hashAlgo string, fileName string) (bool, error) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if hashAlgo == "" {
		return false, errors.New(msgPrinter.Sprintf("Failed to verify digital signature because the hashAlgorithm is empty"))
	} else if publicKey == "" {
		return false, errors.New(msgPrinter.Sprintf("Failed to verify digital signature because the publicKey string is empty"))
	} else if signature == "" {
		return false, errors.New(msgPrinter.Sprintf("Failed to verify digital signature because the signature string is empty"))
	}

	if publicKeyBytes, err := base64.StdEncoding.DecodeString(publicKey); err != nil {
		return false, err
	} else if signatureBytes, err := base64.StdEncoding.DecodeString(signature); err != nil {
		return false, err
	} else {
		if dataHash, err := GetHash(hashAlgo); err != nil {
			return false, err
		} else if pubKey, err := x509.ParsePKIXPublicKey(publicKeyBytes); err != nil {
			return false, err
		} else {
			dr2 := io.TeeReader(dataReader, dataHash)

			// write dr2 to a tmp file
			tmpFileName := fmt.Sprintf("%s.tmp", fileName)
			if err := WriteDateStreamToFile(dr2, tmpFileName); err != nil {
				return false, err
			}

			// verify datahash
			dataHashSum := dataHash.Sum(nil)
			pubKeyToUse := pubKey.(*rsa.PublicKey)
			if cryptoHashType, err := GetCryptoHashType(hashAlgo); err != nil {
				return false, err
			} else if err = rsa.VerifyPSS(pubKeyToUse, cryptoHashType, dataHashSum, signatureBytes, nil); err != nil {
				return false, err
			}

			// rename the .tmp file
			if err := os.Rename(tmpFileName, fileName); err != nil {
				return false, err
			}

			return true, nil
		}
	}
}

func WriteDateStreamToFile(dataReader io.Reader, fileName string) error {
	file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	if _, err := io.Copy(file, dataReader); err != nil && err != io.EOF {
		return err
	}

	return nil
}

func GetHash(hashAlgo string) (hash.Hash, error) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if hashAlgo == common.Sha1 {
		return sha1.New(), nil
	} else if hashAlgo == common.Sha256 {
		return sha256.New(), nil
	} else {
		return nil, errors.New(msgPrinter.Sprintf("Hash algorithm %s is not supported", hashAlgo))
	}

}

func GetCryptoHashType(hashAlgo string) (crypto.Hash, error) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if hashAlgo == common.Sha1 {
		return crypto.SHA1, nil
	} else if hashAlgo == common.Sha256 {
		return crypto.SHA256, nil
	} else {
		return 0, errors.New(msgPrinter.Sprintf("Hash algorithm %s is not supported", hashAlgo))
	}
}
