package policy

import (
    "golang.org/x/crypto/bcrypt"
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

func (w *Workload) Obscure(agreementId string) error {
    if w.WorkloadPassword == "" {
        return nil
    }

    // Convert the workload password into a hash by first concatenating the agreement id onto the end of the password
    if hash, err := bcrypt.GenerateFromPassword([]byte(w.WorkloadPassword + agreementId), bcrypt.DefaultCost); err != nil {
        return err
    } else {
        w.WorkloadPassword = string(hash)
        return nil
    }
}
