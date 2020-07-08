package postgresql

import (
	"fmt"
	"github.com/open-horizon/anax/agreementbot/persistence"
	"strings"
)

// SQL constants used by these filters
// const UNARCHIVED_AGREEMENT = `CAST(agreement->>'archived' AS BOOLEAN) = false`
// const ARCHIVED_AGREEMENT = `CAST(agreement->>'archived' AS BOOLEAN) = true`
// const ACTIVE_AGREEMENT = `CAST(agreement->>'agreement_creation_time' AS INTEGER) != 0 AND CAST(agreement->>'agreement_timeout' AS INTEGER) = 0`
// const NOT_PATTERN = `agreement->>'pattern' = ''`
// const AGEDOUT_AGREEMENT = `CAST(agreement->>'agreement_timeout' AS INTEGER) != 0 AND CAST(agreement->>'agreement_timeout' AS INTEGER) + $1 <= $2`
const NODE_AGREEMENT = `agreement @> '{"device_id": "$1"}'`
// const POLICY_AGREEMENT = `agreement->>'org' = '$1' AND agreement->>'policy_name' = '$2'`
// const PENDING_AGREEMENT = `CAST(agreement->>'agreement_finalized_time' AS INTEGER) = 0 AND CAST(agreement->>'agreement_timeout' AS INTEGER) = 0`
// const SERVICE_ID_AGREEMENT = `(agreement->>'service_id')::jsonb->>0 = '$1'`

// Filter implementations specific to Postgresql

// Include unarchived agreements in the result set.
type UnarchivedFilter struct {
}

func (f *UnarchivedFilter) ConditionSQL(sqlStr string) string {
	// return strings.Replace(sqlStr, ";", fmt.Sprintf(" AND %s;", UNARCHIVED_AGREEMENT), 1)
	return sqlStr
}

func (f *UnarchivedFilter) KeepResult(a *persistence.Agreement) bool {
	// return true
	return a.Archived == false
}

func (db *AgbotPostgresqlDB) GetUnarchivedFilter() persistence.AgbotDBFilter {
	return &UnarchivedFilter{}
}

// Include archived agreements in the result set.
type ArchivedFilter struct {
}

func (f *ArchivedFilter) ConditionSQL(sqlStr string) string {
	// return strings.Replace(sqlStr, ";", fmt.Sprintf(" AND %s;", ARCHIVED_AGREEMENT), 1)
	return sqlStr
}

func (f *ArchivedFilter) KeepResult(a *persistence.Agreement) bool {
	// return true
	return a.Archived == true
}

func (db *AgbotPostgresqlDB) GetArchivedFilter() persistence.AgbotDBFilter {
	return &ArchivedFilter{}
}

// Include active agreements in the result set.
type ActiveFilter struct {
}

func (f *ActiveFilter) ConditionSQL(sqlStr string) string {
	// return strings.Replace(sqlStr, ";", fmt.Sprintf(" AND %s;", ACTIVE_AGREEMENT), 1)
	return sqlStr
}

func (f *ActiveFilter) KeepResult(a *persistence.Agreement) bool {
	// return true
	return a.AgreementCreationTime != 0 && a.AgreementTimedout == 0
}

func (db *AgbotPostgresqlDB) GetActiveFilter() persistence.AgbotDBFilter {
	return &ActiveFilter{}
}

// Exclude pattern based agreements from the result set.
type NoPatternFilter struct {
}

func (f *NoPatternFilter) ConditionSQL(sqlStr string) string {
	// return strings.Replace(sqlStr, ";", fmt.Sprintf(" AND %s;", NOT_PATTERN), 1)
	return sqlStr
}

func (f *NoPatternFilter) KeepResult(a *persistence.Agreement) bool {
	// return true
	return a.Pattern == ""
}

func (db *AgbotPostgresqlDB) GetNoPatternFilter() persistence.AgbotDBFilter {
	return &NoPatternFilter{}
}

// Include old agreements that have been around for a certain period of time.
type AgedOutFilter struct {
	now      int64
	ageLimit int
}

func (f *AgedOutFilter) ConditionSQL(sqlStr string) string {
	// sql := strings.Replace(AGEDOUT_AGREEMENT, "$1", strconv.Itoa(f.ageLimit*3600), 1)
	// sql = strings.Replace(sql, "$2", strconv.FormatInt(f.now, 10), 1)

	// sql = strings.Replace(sqlStr, ";", fmt.Sprintf(" AND %s;", sql), 1)
	// return sql
	return sqlStr
}

func (f *AgedOutFilter) KeepResult(a *persistence.Agreement) bool {
	// return true
	return a.AgreementTimedout != 0 && (a.AgreementTimedout+uint64(f.ageLimit*3600) <= uint64(f.now))
}

func (db *AgbotPostgresqlDB) GetAgedOutFilter(now int64, ageLimit int) persistence.AgbotDBFilter {
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
	sql := strings.Replace(NODE_AGREEMENT, "$1", f.id, 1)
	sql = strings.Replace(sqlStr, ";", fmt.Sprintf(" AND %s;", sql), 1)
	return sql
}

func (f *NodeFilter) KeepResult(a *persistence.Agreement) bool {
	return true
}

func (db *AgbotPostgresqlDB) GetNodeFilter(id string) persistence.AgbotDBFilter {
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
	// sql := strings.Replace(POLICY_AGREEMENT, "$1", f.org, 1)
	// sql = strings.Replace(string(sql), "$2", f.name, 1)

	// sql = strings.Replace(sqlStr, ";", fmt.Sprintf(" AND %s;", sql), 1)
	// return sql
	return sqlStr
}

func (f *PolicyFilter) KeepResult(a *persistence.Agreement) bool {
	// return true
	return a.Org == f.org && a.PolicyName == f.name
}

func (db *AgbotPostgresqlDB) GetPolicyFilter(org string, name string) persistence.AgbotDBFilter {
	return &PolicyFilter{
		org:  org,
		name: name,
	}
}

// Include agreements that are in the process of forming.
type PendingFilter struct {
}

func (f *PendingFilter) ConditionSQL(sqlStr string) string {
	// return strings.Replace(sqlStr, ";", fmt.Sprintf(" AND %s;", PENDING_AGREEMENT), 1)
	return sqlStr
}

func (f *PendingFilter) KeepResult(a *persistence.Agreement) bool {
	// return true
	return a.AgreementFinalizedTime == 0 && a.AgreementTimedout == 0
}

func (db *AgbotPostgresqlDB) GetPendingFilter() persistence.AgbotDBFilter {
	return &PendingFilter{}
}

// Include only a specific agreement id. For the Postgresql DB plugin, this does nothing.
type AgreementIdFilter struct {
	id string
}

func (f *AgreementIdFilter) ConditionSQL(sqlStr string) string {
	return sqlStr
}

func (f *AgreementIdFilter) KeepResult(a *persistence.Agreement) bool {
	// return true
	return a.CurrentAgreementId == f.id
}

func (db *AgbotPostgresqlDB) GetAgreementIdFilter(id string) persistence.AgbotDBFilter {
	return &AgreementIdFilter{
		id: id,
	}
}

// Include only agreements for a specific service id.
type ServiceIdFilter struct {
	id string
}

func (f *ServiceIdFilter) ConditionSQL(sqlStr string) string {
	// sql := strings.Replace(SERVICE_ID_AGREEMENT, "$1", f.id, 1)
	// return strings.Replace(sqlStr, ";", fmt.Sprintf(" AND %s;", sql), 1)
	return sqlStr
}

func (f *ServiceIdFilter) KeepResult(a *persistence.Agreement) bool {
	// return true
	return a.ServiceId[0] == f.id
}

func (db *AgbotPostgresqlDB) GetServiceIdFilter(id string) persistence.AgbotDBFilter {
	return &ServiceIdFilter{
		id: id,
	}
}
