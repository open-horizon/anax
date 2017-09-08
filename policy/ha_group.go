package policy

import (
)

// The purpose of this file is to abstract the operations on the HA Group type.

type HighAvailabilityGroup struct {
    Partners []string `json:"partners,omitempty"`
}

// This function creates HAGroup objects
func HAGroup_Factory(partners []string) *HighAvailabilityGroup {
    g := new(HighAvailabilityGroup)
    g.Partners = partners

    return g
}
