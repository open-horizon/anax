package sync_service

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/edge-sync-service/common"
	"io"
	"os"
	"path"
	"strings"
)

// ObjectDownLoad is to download data to a file named ${objectType}_${objectId}
func ObjectDownLoad(org string, userPw string, objType string, objId string, filePath string, overwrite bool, skipDigitalSigVerify bool) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if userPw == "" {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("must specify exchange credentials to access the model management service"))
	}

	// For this command, object type and id are required parameters, No null checking is needed.
	// Set the API key env var if that's what we're using.
	cliutils.SetWhetherUsingApiKey(userPw)

	// Establish the HTTP request override because the download could take some time.
	setHTTPOverride := false
	if os.Getenv(config.HTTPRequestTimeoutOverride) == "" {
		setHTTPOverride = true
		os.Setenv(config.HTTPRequestTimeoutOverride, "0")
	}

	// Call the MMS service over HTTP to download the object data.
	var data io.Reader
	urlPath := path.Join("api/v1/objects/", org, objType, objId, "/data")
	resp := cliutils.ExchangeGetResponse("Model Management Service", cliutils.GetMMSUrl(), urlPath, cliutils.OrgAndCreds(org, userPw))
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	if resp.StatusCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("object '%s' of type '%s' not found in org %s", objId, objType, org))
	}
	data = resp.Body

	// Restore HTTP request override if necessary.
	if setHTTPOverride {
		os.Setenv(config.HTTPRequestTimeoutOverride, "")
	}

	var fileName string
	// if no fileName and filePath specified, data will be saved in current dir, with name {objectType}_{objectId}
	if filePath == "" {
		fileName = fmt.Sprintf("%s_%s", objType, objId)
	}

	if filePath != "" {
		// trim the ending "/" if there are more than 1 "/"
		for strings.HasSuffix(filePath, "//") {
			filePath = strings.TrimSuffix(filePath, "/")
		}

		fi, _ := os.Stat(filePath)
		if fi == nil {
			// filePath is not an existing dir, then consider it as fileName, need to remove "/" in the end
			if strings.HasSuffix(filePath, "/") {
				filePath = strings.TrimSuffix(filePath, "/")
			}
			fileName = filePath
		} else {
			if fi.IsDir() {
				if !strings.HasSuffix(filePath, "/") {
					filePath = filePath + "/"
				}
				fileName = fmt.Sprintf("%s%s_%s", filePath, objType, objId)
			} else {
				fileName = filePath
			}
		}
	}

	if !overwrite {
		if _, err := os.Stat(fileName); err == nil {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("File %s already exists. Please specify a different file path or file name. To overwrite the existing file, use the '--overwrite' flag.", fileName))
		}
	}

	var verified bool
	var err error
	if !skipDigitalSigVerify {
		// Verify digital signature, save to a tmp file, rename tmp file
		// Call the MMS service over HTTP to get object metadata
		var objectMeta common.MetaData
		urlPath = path.Join("api/v1/objects/", org, objType, objId)
		httpCode := cliutils.ExchangeGet("Model Management Service", cliutils.GetMMSUrl(), urlPath, cliutils.OrgAndCreds(org, userPw), []int{200, 404}, &objectMeta)
		if httpCode == 404 {
			cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("object metadata '%s' of type '%s' not found in org %s", objId, objType, org))
		}

		if objectMeta.HashAlgorithm != "" && objectMeta.PublicKey != "" && objectMeta.Signature != "" {
			// verify data
			//dataReader := bytes.NewReader(data)
			msgPrinter.Println("Verifying data with digital signature....")
			if verified, err = VerifyDataSig(data, objectMeta.PublicKey, objectMeta.Signature, objectMeta.HashAlgorithm, fileName); !verified {
				cliutils.Fatal(cliutils.INTERNAL_ERROR, msgPrinter.Sprintf("Failed to verify data: %s", err.Error()))
			}
			msgPrinter.Println("Verifying digital signature is done.")
		}

	}
	if !verified {
		// verify process will save the data to file, if verify process not execute, then stream data directly to a file
		// Reach here if:
		// 1) use --noIntegrity flag,
		// or
		// 2) object metadata doesn't have HashAlgorithm, or publicKey or signature field
		if err := writeDateStreamToFile(data, fileName); err != nil {
			cliutils.Fatal(cliutils.INTERNAL_ERROR, msgPrinter.Sprintf("Failed to save data for object '%s' of type '%s' to file %s, err: %v", objId, objType, fileName, err))
		}
	}

	msgPrinter.Printf("Data of object %v saved to file %v", objId, fileName)
	msgPrinter.Println()

}

func VerifyDataSig(dataReader io.Reader, publicKey string, signature string, hashAlgo string, fileName string) (bool, error) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if hashAlgo == "" {
		return false, errors.New(msgPrinter.Sprintln("Failed to verify digital signature because the hashAlgorithm is empty"))
	} else if publicKey == "" {
		return false, errors.New(msgPrinter.Sprintln("Failed to verify digital signature because the publicKey string is empty"))
	} else if signature == "" {
		return false, errors.New(msgPrinter.Sprintln("Failed to verify digital signature because the signature string is empty"))
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
			if err := writeDateStreamToFile(dr2, tmpFileName); err != nil {
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

func writeDateStreamToFile(dataReader io.Reader, fileName string) error {
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

