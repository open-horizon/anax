package torrent

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"

	"github.com/golang/glog"
	"github.com/open-horizon/anax/policy"
)

func verify(pubKeyFile string, userKeys string, signature string, image *os.File) (bool, error) {

	glog.V(3).Infof("Verifying signature %v of image %v", signature, image.Name())

	// Read the file content into the hash function.
	hasher := sha256.New()
	if _, err := io.Copy(hasher, image); err != nil {
		return false, fmt.Errorf("Unable to copy image file content into hash function for image: %v, error: %v", image.Name(), err)
	}

	// Verify the workload image.
	return policy.VerifyWorkload(pubKeyFile, signature, hasher, userKeys)

}
