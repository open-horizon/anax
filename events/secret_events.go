package events

import (
	"fmt"
	"github.com/open-horizon/anax/cutil"
)

type SecretUpdate struct {
	SecretOrg        string
	SecretFullName   string // This name is not org qualified
	SecretUpdateTime int64
	PolicyNames      []string // An array of org qualified policy names
	PatternNames     []string // An array of org qualified pattern names
}

// Returns a new SecretUpdate object with a copied policy and pattern name list.
func NewSecretUpdate(secretOrg, secretName string, secretUpdateTime int64, policyNames []string, patternNames []string) (su *SecretUpdate) {
	su = new(SecretUpdate)
	su.SecretOrg = secretOrg
	su.SecretFullName = secretName
	su.SecretUpdateTime = secretUpdateTime
	su.PolicyNames = make([]string, len(policyNames))
	copy(su.PolicyNames, policyNames)
	su.PatternNames = make([]string, len(patternNames))
	copy(su.PatternNames, patternNames)
	return
}

type SecretUpdates struct {
	Updates []*SecretUpdate
}

// Returns a new SecretUpdates object.
func NewSecretUpdates() (sus *SecretUpdates) {
	sus = new(SecretUpdates)
	sus.Updates = make([]*SecretUpdate, 0)
	return
}

func (sus *SecretUpdates) ShortString() string {
	res := "Updated secrets: "
	for _, su := range sus.Updates {
		res += fmt.Sprintf("%s/%s,", su.SecretOrg, su.SecretFullName)
	}
	return res
}

func (sus *SecretUpdates) AddSecretUpdate(update *SecretUpdate) {
	sus.Updates = append(sus.Updates, update)
	return
}

func (sus *SecretUpdates) Length() int {
	return len(sus.Updates)
}

// Returns a list of the fully qualified secret names that have change recently, which are used/referenced
// by the input policy.
func (sus *SecretUpdates) GetUpdatedSecretsForPolicy(policyName string, lastUpdateTime uint64) (uint64, []string) {

	res := make([]string, 0)
	newestUpdate := uint64(0)

	if policyName == "" {
		return newestUpdate, res
	}

	for _, su := range sus.Updates {
		if cutil.SliceContains(su.PolicyNames, policyName) && uint64(su.SecretUpdateTime) > lastUpdateTime {
			if uint64(su.SecretUpdateTime) > newestUpdate {
				newestUpdate = uint64(su.SecretUpdateTime)
			}
			res = append(res, fmt.Sprintf("%s/%s", su.SecretOrg, su.SecretFullName))
		}
	}
	return newestUpdate, res

}

func (sus *SecretUpdates) GetUpdatedSecretsForPattern(patternName string, lastUpdateTime uint64) (uint64, []string) {

	res := make([]string, 0)
	newestUpdate := uint64(0)

	if patternName == "" {
		return newestUpdate, res
	}

	for _, su := range sus.Updates {
		if cutil.SliceContains(su.PatternNames, patternName) && uint64(su.SecretUpdateTime) > lastUpdateTime {
			if uint64(su.SecretUpdateTime) > newestUpdate {
				newestUpdate = uint64(su.SecretUpdateTime)
			}
			res = append(res, fmt.Sprintf("%s/%s", su.SecretOrg, su.SecretFullName))
		}
	}
	return newestUpdate, res

}
