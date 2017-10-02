package policy

import (
	"fmt"
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

func (g *HighAvailabilityGroup) String() string {
	return fmt.Sprintf("HAGroup partners: %v", g.Partners)
}

// Return true if 2 HAGroups are the same, meaning their partner lists contain the same
// partners. The partners dont have to be in the same order in both lists.
func (g *HighAvailabilityGroup) IsSame(other *HighAvailabilityGroup) bool {

	// Different length, not the same groups
	if len(g.Partners) != len(other.Partners) {
		return false
	}

	for _, partner := range g.Partners {
		found := false
		for _, otherPartner := range other.Partners {
			if otherPartner == partner {
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

// Check the compatibility of 2 HAGroups.  HAGroups are only specified on the producer side so
// this check is really just comparing 2 producer side policies. A single prdducer is supposed to
// have the same partner list for all services, so the IsSame check should suffice as a
// compatibility check.
func (g *HighAvailabilityGroup) Compatible_With(other *HighAvailabilityGroup) bool {
	return g.IsSame(other)
}

// Merge 2 HA groups. This should be easy because compatibility is assumes and is defined to be
// indentical.
func (g *HighAvailabilityGroup) Merge(other *HighAvailabilityGroup) *HighAvailabilityGroup {
	return g
}
