package bolt

import (
	"github.com/open-horizon/anax/agreementbot/persistence"
)

// Filter implementations specific to bolt

// Include unarchived agreements in the result set.
type UnarchivedFilter struct {
}

func (f *UnarchivedFilter) ConditionSQL(sqlStr string) string {
	return sqlStr
}

func (f *UnarchivedFilter) KeepResult(a *persistence.Agreement) bool {
	return a.Archived == false
}

func (db *AgbotBoltDB) GetUnarchivedFilter() persistence.AgbotDBFilter {
	return &UnarchivedFilter{}
}

// Include archived agreements in the result set.
type ArchivedFilter struct {
}

func (f *ArchivedFilter) ConditionSQL(sqlStr string) string {
	return sqlStr
}

func (f *ArchivedFilter) KeepResult(a *persistence.Agreement) bool {
	return a.Archived == true
}

func (db *AgbotBoltDB) GetArchivedFilter() persistence.AgbotDBFilter {
	return &ArchivedFilter{}
}

// Include active agreements in the result set.
type ActiveFilter struct {
}

func (f *ActiveFilter) ConditionSQL(sqlStr string) string {
	return sqlStr
}

func (f *ActiveFilter) KeepResult(a *persistence.Agreement) bool {
	return a.AgreementCreationTime != 0 && a.AgreementTimedout == 0
}

func (db *AgbotBoltDB) GetActiveFilter() persistence.AgbotDBFilter {
	return &ActiveFilter{}
}

// Exclude pattern based agreements from the result set.
type NoPatternFilter struct {
}

func (f *NoPatternFilter) ConditionSQL(sqlStr string) string {
	return sqlStr
}

func (f *NoPatternFilter) KeepResult(a *persistence.Agreement) bool {
	return a.Pattern == ""
}

func (db *AgbotBoltDB) GetNoPatternFilter() persistence.AgbotDBFilter {
	return &NoPatternFilter{}
}

// Include old agreements that have been around for a certain period of time.
type AgedOutFilter struct {
	now      int64
	ageLimit int
}

func (f *AgedOutFilter) ConditionSQL(sqlStr string) string {
	return sqlStr
}

func (f *AgedOutFilter) KeepResult(a *persistence.Agreement) bool {
	return a.AgreementTimedout != 0 && (a.AgreementTimedout+uint64(f.ageLimit*3600) <= uint64(f.now))
}

func (db *AgbotBoltDB) GetAgedOutFilter(now int64, ageLimit int) persistence.AgbotDBFilter {
	return &AgedOutFilter{
		now:      now,
		ageLimit: ageLimit,
	}
}

// Include agreements with a specific node.
type NodeFilter struct {
	id string
}

func (f *NodeFilter) ConditionSQL(sqlStr string) string {
	return sqlStr
}

func (f *NodeFilter) KeepResult(a *persistence.Agreement) bool {
	return a.DeviceId == f.id
}

func (db *AgbotBoltDB) GetNodeFilter(id string) persistence.AgbotDBFilter {
	return &NodeFilter{
		id: id,
	}
}

// Include agreements with a specific policy.
type PolicyFilter struct {
	org  string
	name string
}

func (f *PolicyFilter) ConditionSQL(sqlStr string) string {
	return sqlStr
}

func (f *PolicyFilter) KeepResult(a *persistence.Agreement) bool {
	return a.Org == f.org && a.PolicyName == f.name
}

func (db *AgbotBoltDB) GetPolicyFilter(org string, name string) persistence.AgbotDBFilter {
	return &PolicyFilter{
		org:  org,
		name: name,
	}
}

// Include agreements that are in the process of forming.
type PendingFilter struct {
}

func (f *PendingFilter) ConditionSQL(sqlStr string) string {
	return sqlStr
}

func (f *PendingFilter) KeepResult(a *persistence.Agreement) bool {
	return a.AgreementFinalizedTime == 0 && a.AgreementTimedout == 0
}

func (db *AgbotBoltDB) GetPendingFilter() persistence.AgbotDBFilter {
	return &PendingFilter{}
}

// Include only a specific agreement id.
type AgreementIdFilter struct {
	id string
}

func (f *AgreementIdFilter) ConditionSQL(sqlStr string) string {
	return sqlStr
}

func (f *AgreementIdFilter) KeepResult(a *persistence.Agreement) bool {
	return a.CurrentAgreementId == f.id
}

func (db *AgbotBoltDB) GetAgreementIdFilter(id string) persistence.AgbotDBFilter {
	return &AgreementIdFilter{
		id: id,
	}
}

// Include only agreements for a specific service id.
type ServiceIdFilter struct {
	id string
}

func (f *ServiceIdFilter) ConditionSQL(sqlStr string) string {
	return sqlStr
}

func (f *ServiceIdFilter) KeepResult(a *persistence.Agreement) bool {
	return a.ServiceId[0] == f.id
}

func (db *AgbotBoltDB) GetServiceIdFilter(id string) persistence.AgbotDBFilter {
	return &ServiceIdFilter{
		id: id,
	}
}
