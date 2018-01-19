package api

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/golang/glog"

	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/rsapss-tool/listkeys"
	"github.com/open-horizon/rsapss-tool/verify"
)

func FindPublicKeyForOutput(fileName string, config *config.HorizonConfig) (string, error) {

	// Get a list of all valid public key PEM files in the configured location
	pubKeyDir := config.UserPublicKeyPath()
	files, err := getPemFiles(pubKeyDir)
	if err != nil {
		return "", errors.New(fmt.Sprintf("unable to read public key directory %v, error: %v", pubKeyDir, err))
	}

	// If the input file name is not in the list of valid pem files, then return an error
	for _, f := range files {
		if f.Name() == fileName {
			return path.Join(pubKeyDir, f.Name()), nil
		}
	}
	return "", errors.New(fmt.Sprintf("unable to find input file %v in %v", path.Join(pubKeyDir, fileName), pubKeyDir))

}

type KeyPairSimpleRecord struct {
	// embedded
	listkeys.KeyPairSimple
	ID string `json:"id"`
}

func FindPublicKeysForOutput(config *config.HorizonConfig, verbose bool) (map[string][]interface{}, error) {

	// Get a list of all valid public key PEM files in the configured location
	pubKeyDir := config.UserPublicKeyPath()
	files, err := getPemFiles(pubKeyDir)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to read trusted cert (public key) directory %v, error: %v", pubKeyDir, err))
	}

	response := map[string][]interface{}{}
	response["pem"] = make([]interface{}, 0, 10)

	for _, pf := range files {

		var value interface{}
		if verbose {
			keyPath := path.Join(pubKeyDir, pf.Name())
			keyPair, err := listkeys.ReadKeyPair(keyPath)

			if err != nil {
				glog.Errorf("Error reading user x509 cert from file path: %v. Error: %v", keyPath, err)
				continue
			}

			// right now, verbose entails including raw
			kp, err := keyPair.ToKeyPairSimple(true)
			if err != nil {
				glog.Errorf("Error reading user x509 cert from file path: %v. Error: %v", keyPath, err)
				continue
			}

			// add the filename as an id in the returned record (so that the REST part of the HTTP interface makes sense)
			value = KeyPairSimpleRecord{
				ID:            pf.Name(),
				KeyPairSimple: *kp,
			}

		} else {
			value = pf.Name()
		}

		response["pem"] = append(response["pem"], value)
	}

	return response, nil

}

func UploadPublicKey(filename string,
	inBytes []byte,
	config *config.HorizonConfig,
	errorhandler ErrorHandler) bool {

	if filename == "" {
		return errorhandler(NewAPIUserInputError("no filename specified", "trusted cert file"))
	} else if !strings.HasSuffix(filename, ".pem") {
		return errorhandler(NewAPIUserInputError("filename must have .pem suffix", "trusted cert file"))
	}

	targetPath := config.UserPublicKeyPath()
	targetFile := path.Join(targetPath, filename)

	// Receive the uploaded file content and verify that it is a valid public key or x509 cert. If it's
	// valid then save it into the configured PublicKeyPath location from the config. The name of the
	// uploaded file is specified on the HTTP PUT. It does not have to have the same file name used
	// by the HTTP caller.

	if _, err := verify.ValidKeyOrCert(inBytes); err != nil {
		return errorhandler(NewAPIUserInputError(fmt.Sprintf("provided public key or cert is not valid; error: %v", err), "trusted cert file"))
	} else if err := os.MkdirAll(targetPath, 0644); err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("unable to create trusted cert directory %v, error: %v", targetPath, err)))
	} else if err := ioutil.WriteFile(targetFile, inBytes, 0644); err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("unable to write uploaded trusted cert file %v, error: %v", targetFile, err)))
	}
	return false
}

func DeletePublicKey(fileName string,
	config *config.HorizonConfig,
	errorhandler ErrorHandler) bool {

	if fileName == "" {
		return errorhandler(NewAPIUserInputError("no filename specified", "trusted cert file"))
	}

	// Get a list of all valid public key PEM files in the configured location
	pubKeyDir := config.UserPublicKeyPath()
	files, err := getPemFiles(pubKeyDir)
	if err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("unable to read trusted cert directory %v, error: %v", pubKeyDir, err)))
	}

	// If the input file name is not in the list of valid pem files, then return an error
	found := false
	for _, f := range files {
		if f.Name() == fileName {
			found = true
		}
	}
	if !found {
		return errorhandler(NewNotFoundError(fmt.Sprintf("unable to find input file %v", path.Join(pubKeyDir, fileName)), "filename"))
	}

	// The input filename is present, remove it
	err = os.Remove(pubKeyDir + "/" + fileName)
	if err != nil {
		return errorhandler(NewSystemError(fmt.Sprintf("unable to delete trusted cert file %v, error: %v", path.Join(pubKeyDir, fileName), err)))
	}
	return false

}

func getPemFiles(homePath string) ([]os.FileInfo, error) {

	res := make([]os.FileInfo, 0, 10)

	if files, err := ioutil.ReadDir(homePath); err != nil && !os.IsNotExist(err) {
		return res, errors.New(fmt.Sprintf("Unable to get list of PEM files in %v, error: %v", homePath, err))
	} else if os.IsNotExist(err) {
		return res, nil
	} else {
		for _, fileInfo := range files {
			if strings.HasSuffix(fileInfo.Name(), ".pem") && !fileInfo.IsDir() {
				fName := homePath + "/" + fileInfo.Name()
				if pubKeyData, err := ioutil.ReadFile(fName); err != nil {
					continue
				} else if _, err := verify.ValidKeyOrCert(pubKeyData); err != nil {
					continue
				} else {
					res = append(res, fileInfo)
				}
			}
		}
		return res, nil
	}
}
