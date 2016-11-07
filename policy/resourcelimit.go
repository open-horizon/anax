package policy

import (
    "fmt"
)

type ResourceLimit struct {
    NetworkUpload   int `json:"networkUpload"`   // The max network upload allowed to be consumed Kbps
    NetworkDownload int `json:"networkDownload"` // The max network download allowed to be consumed Kbps
    Memory          int `json:"memory"`          // The max memory allowed to be consumed MB
    CPUs            int `json:"cpus"`            // The max number of CPUs allowed to be consumed
}

func (r ResourceLimit) String() string {
    return fmt.Sprintf("NetworkUpload: %v, NetworkDownload: %v, Memory: %v, CPUs: %v", r.NetworkUpload, r.NetworkDownload, r.Memory, r.CPUs)
}

func (self *ResourceLimit) IsSatisfiedBy(other *ResourceLimit) bool {

    // If the producer doesn't care then it is easily satisfied
    if other.NetworkUpload != 0 && self.NetworkUpload > other.NetworkUpload {
        return false
    } else if other.NetworkDownload != 0 && self.NetworkDownload > other.NetworkDownload {
        return false
    } else if other.Memory != 0 && self.Memory > other.Memory {
        return false
    } else if other.CPUs != 0 && self.CPUs > other.CPUs {
        return false
    }

    return true
}

func (self *ResourceLimit) MergeProducers(other *ResourceLimit) *ResourceLimit {

    max := func(a, b int) int {
        if a > b {
            return a
        } else {
            return b
        }
    }

    // Setup the new structure to hold the merged limits
    merged_rl := new(ResourceLimit)
    merged_rl.NetworkUpload = max(self.NetworkUpload, other.NetworkUpload)
    merged_rl.NetworkDownload = max(self.NetworkDownload, other.NetworkDownload)
    merged_rl.Memory = max(self.Memory, other.Memory)
    merged_rl.CPUs = max(self.CPUs, other.CPUs)

    return merged_rl
}
