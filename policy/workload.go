package policy

import (
    "crypto"
    "crypto/rsa"
    "crypto/sha256"
    "crypto/x509"
    "encoding/base64"
    "encoding/pem"
    "errors"
    "fmt"
    "github.com/golang/glog"
    "golang.org/x/crypto/bcrypt"
    "io"
    "io/ioutil"
)

type Image struct {
    File      string `json:"file"`
    Signature string `json:"signature"`
}

func (i Image) IsSame(compare Image) bool {
    return i.File == compare.File && i.Signature == compare.Signature
}

type Torrent struct {
    Url    string  `json:"url"`
    Images []Image `json:"images"`
}

func (t Torrent) IsSame(compare Torrent) bool {
    if t.Url != compare.Url {
        return false
    } else {
        for _, i := range t.Images {
            found := false
            for _, compareI := range compare.Images {
                if i.IsSame(compareI) {
                    found = true
                    break
                }
            }
            if !found {
                return false
            }
        }
        return true
    }
}

type Workload struct {
    Deployment          string  `json:"deployment"`
    DeploymentSignature string  `json:"deployment_signature"`
    DeploymentUserInfo  string  `json:"deployment_user_info"`
    Torrent             Torrent `json:"torrent"`
    WorkloadPassword    string  `json:"workload_password"` // The password used to create the bcrypt hash that is passed to the workload so that the workload can verify the caller
}

func (wl Workload) IsSame(compare Workload) bool {
    return wl.Deployment == compare.Deployment && wl.DeploymentSignature == compare.DeploymentSignature && wl.DeploymentUserInfo == compare.DeploymentUserInfo && wl.Torrent.IsSame(compare.Torrent) && wl.WorkloadPassword == compare.WorkloadPassword
}

func (w *Workload) Obscure(agreementId string, defaultPW string) error {

    if w.WorkloadPassword == "" && defaultPW == "" {
        return nil
    }

    // Workload password in a policy file overrides the default workload PW from the config
    wpw := w.WorkloadPassword
    if defaultPW != "" {
        wpw = defaultPW
    }

    // Convert the workload password into a hash by first concatenating the agreement id onto the end of the password
    if hash, err := bcrypt.GenerateFromPassword([]byte(wpw + agreementId), bcrypt.DefaultCost); err != nil {
        return err
    } else {
        w.WorkloadPassword = string(hash)
        return nil
    }
}

func (w Workload) HasValidSignature(pubKeyFile string) error {
    glog.V(3).Infof("Verifying workload signature")

    if pubKeyData, err := ioutil.ReadFile(pubKeyFile); err != nil {
        return errors.New(fmt.Sprintf("Unable to read public key file %v, Error: %v", pubKeyFile, err))
    } else {

        block, _ := pem.Decode(pubKeyData)
        if publicKey, err := x509.ParsePKIXPublicKey(block.Bytes); err != nil {
            return errors.New(fmt.Sprintf("Unable to demarshal public key file %v, Error: %v", pubKeyFile, err))
        } else {
            glog.V(5).Infof("Using RSA pubkey: %v", publicKey)
            glog.V(5).Infof("Checking signature of deployment string: %v", w.Deployment)

            if decoded, err := base64.StdEncoding.DecodeString(w.DeploymentSignature); err != nil {
                return errors.New(fmt.Sprintf("Error decoding base64 signature: %v, Error: %v", w.DeploymentSignature, err))
            } else {

                hasher := sha256.New()
                if _, err := io.WriteString(hasher, w.Deployment); err != nil {
                    return errors.New(fmt.Sprintf("Error hashing deployment string: %v, Error: %v", w.Deployment, err))
                } else {
                    if err := rsa.VerifyPSS(publicKey.(*rsa.PublicKey), crypto.SHA256, hasher.Sum(nil), decoded, nil); err != nil {
                        return errors.New(fmt.Sprintf("Error verifying deployment signature: %v for deployment: %v, Error: %v", w.DeploymentSignature, w.Deployment, err))
                    } else {
                        return nil
                    }
                }

            }

        }
    }
}
