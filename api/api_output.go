package api

import (
	"github.com/open-horizon/anax/persistence"
	"strings"
)

type EstablishedAgreementsByAgreementCreationTime []persistence.EstablishedAgreement

func (s EstablishedAgreementsByAgreementCreationTime) Len() int {
	return len(s)
}

func (s EstablishedAgreementsByAgreementCreationTime) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s EstablishedAgreementsByAgreementCreationTime) Less(i, j int) bool {
	return s[i].AgreementCreationTime < s[j].AgreementCreationTime
}

type EstablishedAgreementsByAgreementTerminatedTime []persistence.EstablishedAgreement

func (s EstablishedAgreementsByAgreementTerminatedTime) Len() int {
	return len(s)
}

func (s EstablishedAgreementsByAgreementTerminatedTime) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s EstablishedAgreementsByAgreementTerminatedTime) Less(i, j int) bool {
	return s[i].AgreementTerminatedTime < s[j].AgreementTerminatedTime
}

type WorkloadConfigByWorkloadURLAndVersion []persistence.WorkloadConfig

func (s WorkloadConfigByWorkloadURLAndVersion) Len() int {
	return len(s)
}

func (s WorkloadConfigByWorkloadURLAndVersion) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s WorkloadConfigByWorkloadURLAndVersion) Less(i, j int) bool {

	// Just compare the starting version in the two ranges
	first := s[i].VersionExpression[1:strings.Index(s[i].VersionExpression,",")]
	second := s[j].VersionExpression[1:strings.Index(s[j].VersionExpression,",")]

	return (strings.Compare(s[i].WorkloadURL, s[j].WorkloadURL) == -1) && (strings.Compare(first, second) == -1)
}
