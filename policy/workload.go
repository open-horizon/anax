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

type WorkloadPriority struct {
    PriorityValue  int `json:"priority_value"`  // The priority of the workload
    Retries        int `json:"retries"`         // The number of retries before giving up and moving to the next priority
    RetryDurationS int `json:"retry_durations"` // The number of seconds in which the specified number of retries must occur in order for the next priority workload to be attempted.
}

func (wp WorkloadPriority) String() string {
    return fmt.Sprintf("PriorityValue: %v, " +
        "Retries: %v, " +
        "RetryDurationS: %v",
        wp.PriorityValue, wp.Retries, wp.RetryDurationS)
}

func (wp WorkloadPriority) IsSame(compare WorkloadPriority) bool {
    return wp.PriorityValue == compare.PriorityValue &&
            wp.Retries == compare.Retries &&
            wp.RetryDurationS == compare.RetryDurationS
}

type Workload struct {
    Deployment          string           `json:"deployment"`
    DeploymentSignature string           `json:"deployment_signature"`
    DeploymentUserInfo  string           `json:"deployment_user_info"`
    Torrent             Torrent          `json:"torrent"`
    WorkloadPassword    string           `json:"workload_password"` // The password used to create the bcrypt hash that is passed to the workload so that the workload can verify the caller
    Priority            WorkloadPriority `json:"priority,omitempty"` // The highest priority workload is tried first for an agrement, if it fails, the next priority is tried. Priority 1 is the highest, priority 2 is next, etc.
}

func (w Workload) String() string {
    return fmt.Sprintf("Priority: %v, " +
        "Deployment: %v, " +
        "DeploymentSignature: %v, " +
        "DeploymentUserInfo: %v, " +
        "Torrent: %v, " +
        "WorkloadPassword: %v",
        w.Priority, w.Deployment, w.DeploymentSignature, w.DeploymentUserInfo, w.Torrent, w.WorkloadPassword)
}

func (w Workload) ShortString() string {
    return fmt.Sprintf("Priority: %v, " +
        "Deployment: %v",
        w.Priority, w.Deployment)
}

// This function specifically omits checking the Priority section of the workload because that section is not relevant to the equivalence between 2 workload definitions.
func (wl Workload) IsSame(compare Workload) bool {
    return wl.Deployment == compare.Deployment &&
            wl.DeploymentSignature == compare.DeploymentSignature &&
            wl.DeploymentUserInfo == compare.DeploymentUserInfo &&
            wl.Torrent.IsSame(compare.Torrent) &&
            wl.WorkloadPassword == compare.WorkloadPassword &&
            wl.Priority.IsSame(compare.Priority)
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

func (w Workload) HasEmptyPriority() bool {
    if w.Priority.PriorityValue == 0 && w.Priority.Retries == 0 && w.Priority.RetryDurationS == 0 {
        return true
    }
    return false
}
