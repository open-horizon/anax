package helm

import (
	"encoding/base64"
	"fmt"
	"github.com/golang/glog"
	"io/ioutil"
	"os"
)

// Constants returned by the Status function inthe HelmClient interface.
const (
	STATUS_RUNNING int = iota // 0
	STATUS_NOT_RUNNING
)

type HelmClient interface {
	Install(b64Package string, releaseName string) error
	UnInstall(releaseName string) error
	Status(releaseName string) (int, error)
}

func NewHelmClient() HelmClient {
	return NewCliClient()
}

// ========================================================================================
// Utility functions that all clients will need.

const TEMP_PACKAGE_PREFIX = "anax-helm-package-"

// Convert a base 64 encoded string into its original bytes and then write the bytes to a file
// in the file system.
func ConvertB64StringToFile(b64Package string) (string, error) {
	if sDec, err := base64.StdEncoding.DecodeString(b64Package); err != nil {
		return "", err
	} else if f, err := ioutil.TempFile("", TEMP_PACKAGE_PREFIX); err != nil {
		return "", err
	} else {
		defer f.Close()
		num, err := f.Write(sDec)
		if err != nil {
			return "", err
		}
		glog.V(5).Infof(clilogString(fmt.Sprintf("Wrote %v bytes to temp Helm package to file: %v", num, f.Name())))
		return f.Name(), nil
	}
}

// Convert a Helm chart archive file into a base 64 encoded string. The input filepath is assumed to be absolute.
func ConvertFileToB64String(filePath string) (string, error) {

	// Make sure the archive file actually exists.
	if _, err := os.Stat(filePath); err != nil {
		return "", err
	}

	// Read in the file and convert the contents to a base 64 encoded string.
	if fileBytes, err := ioutil.ReadFile(filePath); err != nil {
		return "", err
	} else {
		b64String := base64.StdEncoding.EncodeToString(fileBytes)
		return b64String, nil
	}
}
