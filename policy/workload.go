package policy

import (
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/rsapss-tool/verify"
	"golang.org/x/crypto/bcrypt"
)

type WorkloadList []Workload

// This function adds a workload to the list. Return an error if there are duplicates.
func (self *WorkloadList) Add_Workload(new_ele *Workload) error {
	for _, ele := range *self {
		if ele.IsSame(*new_ele) {
			return errors.New(fmt.Sprintf("WorkloadList %v already has the element being added: %v", *self, *new_ele))
		}
	}
	(*self) = append(*self, *new_ele)
	return nil
}

type Torrent struct {
	Url       string `json:"url,omitempty"`
	Signature string `json:"signature,omitempty"`
}

func (t Torrent) IsSame(compare Torrent) bool {
	return t.Url == compare.Url && t.Signature == compare.Signature
}

type WorkloadPriority struct {
	PriorityValue     int `json:"priority_value,omitempty"`     // The priority of the workload
	Retries           int `json:"retries,omitempty"`            // The number of retries before giving up and moving to the next priority
	RetryDurationS    int `json:"retry_durations,omitempty"`    // The number of seconds in which the specified number of retries must occur in order for the next priority workload to be attempted.
	VerifiedDurationS int `json:"verified_durations,omitempty"` // The number of second in which verified data must exist before the rollback retry feature is turned off
}

func (wp WorkloadPriority) String() string {
	return fmt.Sprintf("PriorityValue: %v, "+
		"Retries: %v, "+
		"RetryDurationS: %v, "+
		"VerifiedDurationS: %v",
		wp.PriorityValue, wp.Retries, wp.RetryDurationS, wp.VerifiedDurationS)
}

// This function creates workload priority objects
func Workload_Priority_Factory(priority int, retries int, retryDur int, verifiedDur int) *WorkloadPriority {
	w := new(WorkloadPriority)
	w.PriorityValue = priority
	w.Retries = retries
	w.RetryDurationS = retryDur
	w.VerifiedDurationS = verifiedDur
	return w
}

func (wp WorkloadPriority) IsSame(compare WorkloadPriority) bool {
	return wp.PriorityValue == compare.PriorityValue &&
		wp.Retries == compare.Retries &&
		wp.RetryDurationS == compare.RetryDurationS &&
		wp.VerifiedDurationS == compare.VerifiedDurationS
}

type Workload struct {
	Deployment                   string           `json:"deployment,omitempty"`
	DeploymentSignature          string           `json:"deployment_signature,omitempty"`
	DeploymentUserInfo           string           `json:"deployment_user_info,omitempty"`
	Torrent                      Torrent          `json:"torrent,omitempty"`
	WorkloadPassword             string           `json:"workload_password,omitempty"`              // The password used to create the bcrypt hash that is passed to the workload so that the workload can verify the caller
	Priority                     WorkloadPriority `json:"priority,omitempty"`                       // The highest priority workload is tried first for an agrement, if it fails, the next priority is tried. Priority 1 is the highest, priority 2 is next, etc.
	WorkloadURL                  string           `json:"workloadUrl,omitempty"`                    // Added with MS split, refers to a workload definition in the exchange
	Org                          string           `json:"organization,omitempty"`                   // Added woth org support, refers to the organization where the workload is defined
	Version                      string           `json:"version,omitempty"`                        // Added with MS split, refers to the version of the workload
	Arch                         string           `json:"arch,omitempty"`                           // Added with MS split, refers to the hardware architecture of the workload definition
	DeploymentOverrides          string           `json:"deployment_overrides,omitempty"`           // Added with MS split, env var overrides for the workload
	DeploymentOverridesSignature string           `json:"deployment_overrides_signature,omitempty"` // Added with MS split, signature of env var overrides
}

func (w Workload) String() string {
	return fmt.Sprintf("Priority: %v, "+
		"Deployment: %v, "+
		"DeploymentSignature: %v, "+
		"DeploymentUserInfo: %v, "+
		"Torrent: %v, "+
		"Workload Password: %v, "+
		"Workload URL: %v, "+
		"Org: %v, "+
		"Version: %v, "+
		"Arch: %v, "+
		"Deployment Overrides: %v, "+
		"Deployment Overrides Signature: %v",
		w.Priority, w.Deployment, w.DeploymentSignature, w.DeploymentUserInfo, w.Torrent, w.WorkloadPassword,
		w.WorkloadURL, w.Org, w.Version, w.Arch, w.DeploymentOverrides, w.DeploymentOverridesSignature)
}

func (w Workload) ShortString() string {
	return fmt.Sprintf(
		"Workload URL: %v, "+
			"Version: %v, "+
			"Org: %v, "+
			"Priority: %v, "+
			"Deployment: %v",
		w.WorkloadURL, w.Version, w.Org, w.Priority, w.Deployment)
}

// This function creates workload objects
func Workload_Factory(url string, org string, version string, arch string) *Workload {
	w := new(Workload)
	w.WorkloadURL = url
	w.Org = org
	w.Version = version
	w.Arch = arch
	return w
}

// This function compares 2 workload objects for sameness. This is slightly complicated because 2 workloads can be
// semantically the same without having identical state. For example, a workload entry that has the WorkloadURL set
// might also have the other workloads details that can be found at the other end of he URL. In this case, we can
// ignore comparing the details fields and just stick with a comparison of the URL.
func (wl Workload) IsSame(compare Workload) bool {

	// Common comparison checks
	if wl.WorkloadPassword != compare.WorkloadPassword || !wl.Priority.IsSame(compare.Priority) {
		return false
	}

	// old style policy file with workload details embedded in it
	if wl.WorkloadURL == "" {
		return wl.Deployment == compare.Deployment &&
			wl.DeploymentSignature == compare.DeploymentSignature &&
			wl.DeploymentUserInfo == compare.DeploymentUserInfo &&
			wl.Torrent.IsSame(compare.Torrent)

	} else {
		return wl.WorkloadURL == compare.WorkloadURL &&
			wl.Version == compare.Version &&
			wl.Org == compare.Org &&
			wl.Arch == compare.Arch &&
			wl.DeploymentOverrides == compare.DeploymentOverrides &&
			wl.DeploymentOverridesSignature == compare.DeploymentOverridesSignature
	}

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
	if hash, err := bcrypt.GenerateFromPassword([]byte(wpw+agreementId), bcrypt.DefaultCost); err != nil {
		return err
	} else {
		w.WorkloadPassword = string(hash)
		return nil
	}
}

func (w Workload) HasValidSignature(keyFileNames []string) error {
	glog.V(3).Infof("Verifying workload signature with keys (bare or wrapped in x509 cert): %v", keyFileNames)

	if w.Deployment != "" {
		if verified, fn_success, failed_map := verify.InputVerifiedByAnyKey(keyFileNames, w.DeploymentSignature, []byte(w.Deployment)); !verified {
			return fmt.Errorf("Error verifying deployment signature: %v for deployment: %v, Error: %v", w.DeploymentSignature, w.Deployment, failed_map)
		} else {
			glog.Infof("Deployment verification successful with RSA pubkey in file: %v", fn_success)
		}
	}

	if w.DeploymentOverrides == "" {
		return nil
	} else {
		if verified, fn_success, failed_map := verify.InputVerifiedByAnyKey(keyFileNames, w.DeploymentOverridesSignature, []byte(w.DeploymentOverrides)); !verified {
			return fmt.Errorf("Error verifying deployment overrides signature: %v for deployment: %v, Error: %v", w.DeploymentOverridesSignature, w.DeploymentOverrides, failed_map)
		} else {
			glog.Infof("Deployment overrides verification successful with RSA pubkey in file: %v", fn_success)
		}
		return nil
	}

}

func (w Workload) HasEmptyPriority() bool {
	if w.Priority.PriorityValue == 0 && w.Priority.Retries == 0 && w.Priority.RetryDurationS == 0 {
		return true
	}
	return false
}
