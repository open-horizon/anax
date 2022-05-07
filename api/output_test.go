//go:build unit
// +build unit

package api

import (
	"github.com/open-horizon/anax/persistence"
	"sort"
	"testing"
)

var agreements = []persistence.EstablishedAgreement{
	persistence.EstablishedAgreement{
		Name:                    "a",
		Archived:                false,
		AgreementCreationTime:   1990,
		AgreementTerminatedTime: 34049,
	},
	persistence.EstablishedAgreement{
		Name:                    "b",
		Archived:                false,
		AgreementCreationTime:   1580,
		AgreementTerminatedTime: 64049,
	},
	persistence.EstablishedAgreement{
		Name:                    "c",
		Archived:                false,
		AgreementCreationTime:   1997,
		AgreementTerminatedTime: 23092,
	},
}

func partialDeep(old []persistence.EstablishedAgreement) []persistence.EstablishedAgreement {
	ag := make([]persistence.EstablishedAgreement, 0)

	for _, v := range old {
		ag = append(ag, persistence.EstablishedAgreement{
			Name:                    v.Name,
			Archived:                v.Archived,
			AgreementCreationTime:   v.AgreementCreationTime,
			AgreementTerminatedTime: v.AgreementTerminatedTime,
		})
	}

	return ag
}

func Test_EstablishedAgreementsByCreationTime(t *testing.T) {

	cTime := partialDeep(agreements)

	if cTime[0].AgreementCreationTime != 1990 {
		t.Errorf("Unexpected initial test state")
	}

	sort.Sort(EstablishedAgreementsByAgreementCreationTime(cTime))

	if cTime[0].AgreementCreationTime != 1580 || cTime[2].AgreementCreationTime != 1997 {
		t.Errorf("Unexpected sorted state %v", cTime)
	}
}

func Test_EstablishedAgreementsByTerminatedTime(t *testing.T) {

	tTime := partialDeep(agreements)

	if tTime[0].AgreementTerminatedTime != 34049 {
		t.Errorf("Unexpected initial test state")
	}

	sort.Sort(EstablishedAgreementsByAgreementTerminatedTime(tTime))

	if tTime[0].AgreementTerminatedTime != 23092 || tTime[2].AgreementTerminatedTime != 64049 {
		t.Errorf("Unexpected sorted state %v", tTime)
	}
}
