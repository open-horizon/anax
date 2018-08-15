// +build unit

package eventlog

import (
	"flag"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/persistence"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"
)

func init() {
	flag.Set("alsologtostderr", "true")
	flag.Set("v", "7")
	// no need to parse flags, that's done by test framework
}

func Test_LogEvent(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	// agreement source type
	wi, _ := persistence.NewWorkloadInfo("url", "org", "version", "")
	ag1, err1 := persistence.NewEstablishedAgreement(db, "name1", "agreementId1", "consumerId", "{}", "Basic", 1, []string{"http://sensor1.org"}, "signature", "address", "bcType", "bcName", "bcOrg", wi)
	if err1 != nil {
		t.Errorf("error writing agreement1: %v", err1)
	}
	src1 := persistence.NewAgreementEventSourceFromAg(*ag1)

	// service source type
	svc1, err1 := persistence.NewMicroserviceInstance(db, "http://sensor1.org", "1.2.0", "1", []persistence.ServiceInstancePathElement{})
	if err1 != nil {
		t.Errorf("error writing agreement1: %v", err1)
	}
	src2 := persistence.NewServiceEventSourceFromServiceInstance(*svc1)

	// save event logs
	if err := LogEvent(db, persistence.SEVERITY_INFO, "proposal received.", "proposal_received.", persistence.SRC_TYPE_AG, *src1); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogEvent(db, persistence.SEVERITY_INFO, "reply sent.", "reply _ent", persistence.SRC_TYPE_AG, *src1); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogEvent(db, persistence.SEVERITY_INFO, "Started service configuration", "service_configuration_started", persistence.SRC_TYPE_SVC, *src2); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogEvent(db, persistence.SEVERITY_INFO, "Saved service into db", "saved_service", persistence.SRC_TYPE_SVC, *src2); err != nil {
		t.Errorf("error saving event log: %v", err)
	}

	// get event logs
	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 4, len(elogs), "Test GetEventLogs without selection. Total 12 entries.")
	}

	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{"service_url": {{"~", "sensor"}}}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 4, len(elogs), "Test GetEventLogs with selection.")
	}

	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{"source_type": {{"=", "service"}}}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 2, len(elogs), "Test GetEventLogs with selection.")
	}

	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{"source_type": {{"=", "agreement"}}}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 2, len(elogs), "Test GetEventLogs with selection.")
	}

	if elogs, err := GetEventLogs(db, false, map[string][]persistence.Selector{"source_type": {{"=", "agreement"}}}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 2, len(elogs), "Test GetEventLogs with selection.")
	}
}

func Test_LogAgreementEvent(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	wi, _ := persistence.NewWorkloadInfo("url", "org", "version", "")
	ag1, err1 := persistence.NewEstablishedAgreement(db, "name1", "agreementId1", "consumerId", "{}", "Basic", 1, []string{"http://sensor1.org"}, "signature", "address", "bcType", "bcName", "bcOrg", wi)
	if err1 != nil {
		t.Errorf("error writing agreement1: %v", err1)
	}
	ag2, err2 := persistence.NewEstablishedAgreement(db, "name2", "agreementId2", "consumerId", "{}", "Basic", 1, []string{"http://mycomp.org"}, "signature", "address", "bcType", "bcName", "bcOrg", wi)
	if err2 != nil {
		t.Errorf("error writing agreement2: %v", err2)
	}
	ag3, err3 := persistence.NewEstablishedAgreement(db, "name3", "agreementId3", "consumerId", "{}", "Basic", 1, []string{"http://sensor3.org"}, "signature", "address", "bcType", "bcName", "bcOrg", wi)
	if err3 != nil {
		t.Errorf("error writing agreement3: %v", err3)
	}

	// save event logs
	if err := LogAgreementEvent(db, persistence.SEVERITY_INFO, "proposal received.", persistence.EC_RECEIVED_PROPOSAL, *ag1); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogAgreementEvent(db, persistence.SEVERITY_INFO, "reply sent.", persistence.EC_RECEIVED_REPLYACK_MESSAGE, *ag1); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogAgreementEvent(db, persistence.SEVERITY_INFO, "proposal received.", persistence.EC_RECEIVED_PROPOSAL, *ag2); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogAgreementEvent(db, persistence.SEVERITY_INFO, "proposal received.", persistence.EC_RECEIVED_PROPOSAL, *ag3); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogAgreementEvent(db, persistence.SEVERITY_INFO, "reply sent.", persistence.EC_RECEIVED_REPLYACK_MESSAGE, *ag2); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogAgreementEvent(db, persistence.SEVERITY_INFO, "agreement finalized.", persistence.EC_AGREEMENT_REACHED, *ag1); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogAgreementEvent(db, persistence.SEVERITY_INFO, "reply sent.", persistence.EC_RECEIVED_REPLYACK_MESSAGE, *ag3); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogAgreementEvent(db, persistence.SEVERITY_INFO, "agreement finalized.", persistence.EC_AGREEMENT_REACHED, *ag3); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogAgreementEvent(db, persistence.SEVERITY_INFO, "agreement finalized.", persistence.EC_AGREEMENT_REACHED, *ag2); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogAgreementEvent(db, persistence.SEVERITY_ERROR, "something is wrong.", persistence.EC_ERROR_START_CONTAINER, *ag1); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogAgreementEvent(db, persistence.SEVERITY_WARN, "something is wrong.", persistence.EC_ERROR_START_CONTAINER, *ag2); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogAgreementEvent(db, persistence.SEVERITY_ERROR, "something is really wrong.", persistence.EC_DATABASE_ERROR, *ag1); err != nil {
		t.Errorf("error saving event log: %v", err)
	}

	// get event logs
	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 12, len(elogs), "Test GetEventLogs without selection. Total 12 entries.")
	}

	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{"record_id": {{">", 5}, {"=", 7}, {"<", 10}}}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 1, len(elogs), "Test GetEventLogs with selection.")
	}

	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{"message": {{"~", "proposal"}}}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 3, len(elogs), "Test GetEventLogs with selection.")
	}

	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{"agreement_id": {{"=", "agreementId2"}}}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 4, len(elogs), "Test GetEventLogs with selection.")
	}

	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{"event_code": {{"=", persistence.EC_RECEIVED_PROPOSAL}}}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 3, len(elogs), "Test GetEventLogs with selection.")
	}

}

func Test_LogAgreementEvent2(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	// save event logs
	if err := LogAgreementEvent2(db, persistence.SEVERITY_INFO, "proposal received.", persistence.EC_RECEIVED_PROPOSAL, "agreementId1", persistence.WorkloadInfo{"http://top1.com", "myorg", "1.0.0", "amd64"}, []string{"http://sensor1.org"}, "consumerId", "Basic"); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogAgreementEvent2(db, persistence.SEVERITY_INFO, "reply sent.", persistence.EC_RECEIVED_REPLYACK_MESSAGE, "agreementId1", persistence.WorkloadInfo{"http://top1.com", "myorg", "1.0.0", "amd64"}, []string{"http://sensor1.org"}, "consumerId", "Basic"); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogAgreementEvent2(db, persistence.SEVERITY_INFO, "proposal received.", persistence.EC_RECEIVED_PROPOSAL, "agreementId2", persistence.WorkloadInfo{"http://top2.com", "myorg", "1.0.0", "amd64"}, []string{"http://mycomp.org"}, "consumerId", "Basic"); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogAgreementEvent2(db, persistence.SEVERITY_INFO, "proposal received.", persistence.EC_RECEIVED_PROPOSAL, "agreementId3", persistence.WorkloadInfo{"http://top3.com", "myorg", "1.0.0", "amd64"}, []string{"http://sensor3.org"}, "consumerId", "Basic"); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogAgreementEvent2(db, persistence.SEVERITY_INFO, "reply sent.", persistence.EC_RECEIVED_REPLYACK_MESSAGE, "agreementId2", persistence.WorkloadInfo{"http://top2.com", "myorg", "1.0.0", "amd64"}, []string{"http://mycomp.org"}, "consumerId", "Basic"); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogAgreementEvent2(db, persistence.SEVERITY_INFO, "agreement finalized.", persistence.EC_AGREEMENT_REACHED, "agreementId1", persistence.WorkloadInfo{"http://top1.com", "myorg", "1.0.0", "amd64"}, []string{"http://sensor1.org"}, "consumerId", "Basic"); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogAgreementEvent2(db, persistence.SEVERITY_INFO, "reply sent.", persistence.EC_RECEIVED_REPLYACK_MESSAGE, "agreementId3", persistence.WorkloadInfo{"http://top3.com", "myorg", "1.0.0", "amd64"}, []string{"http://sensor3.org"}, "consumerId", "Basic"); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogAgreementEvent2(db, persistence.SEVERITY_INFO, "agreement finalized.", persistence.EC_AGREEMENT_REACHED, "agreementId3", persistence.WorkloadInfo{"http://top3.com", "myorg", "1.0.0", "amd64"}, []string{"http://sensor3.org"}, "consumerId", "Basic"); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogAgreementEvent2(db, persistence.SEVERITY_INFO, "agreement finalized.", persistence.EC_AGREEMENT_REACHED, "agreementId2", persistence.WorkloadInfo{"http://top2.com", "myorg", "1.0.0", "amd64"}, []string{"http://mycomp.org"}, "consumerId", "Basic"); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogAgreementEvent2(db, persistence.SEVERITY_ERROR, "something is wrong.", persistence.EC_ERROR_START_CONTAINER, "agreementId1", persistence.WorkloadInfo{"http://top1.com", "myorg", "1.0.0", "amd64"}, []string{"http://sensor1.org"}, "consumerId", "Basic"); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogAgreementEvent2(db, persistence.SEVERITY_WARN, "something is wrong.", persistence.EC_ERROR_START_CONTAINER, "agreementId2", persistence.WorkloadInfo{"http://top2.com", "myorg", "1.0.0", "amd64"}, []string{"http://mycomp.org"}, "consumerId", "Basic"); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogAgreementEvent2(db, persistence.SEVERITY_ERROR, "something is really wrong.", persistence.EC_DATABASE_ERROR, "agreementId1", persistence.WorkloadInfo{"http://top1.com", "myorg", "1.0.0", "amd64"}, []string{"http://sensor1.org"}, "consumerId", "Basic"); err != nil {
		t.Errorf("error saving event log: %v", err)
	}

	// get event logs
	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 12, len(elogs), "Test GetEventLogs without selection. Total 12 entries.")
	}

	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{"record_id": {{">", 5}, {"<", 10}}}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 4, len(elogs), "Test GetEventLogs with selection.")
	}

	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{"message": {{"~", "proposal"}}}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 3, len(elogs), "Test GetEventLogs with selection.")
	}

	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{"agreement_id": {{"=", "agreementId3"}}}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 3, len(elogs), "Test GetEventLogs with selection.")
	}

	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{"consumer_id": {{"=", "consumerId"}}}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 12, len(elogs), "Test GetEventLogs with selection.")
	}

	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{"service_url": {{"~", "top1"}, {"~", "mycomp"}}}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 0, len(elogs), "Test GetEventLogs with selection.")
	}

	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{"event_code": {{"=", persistence.EC_AGREEMENT_REACHED}}}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 3, len(elogs), "Test GetEventLogs with selection.")
	}

}

func Test_LogServiceEvent(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	src1, err1 := persistence.NewMicroserviceInstance(db, "http://sensor1.org", "1.2.0", "1", []persistence.ServiceInstancePathElement{})
	if err1 != nil {
		t.Errorf("error writing agreement1: %v", err1)
	}
	src2, err2 := persistence.NewMicroserviceInstance(db, "http://sensor2.org", "1.0.0", "2", []persistence.ServiceInstancePathElement{})
	if err2 != nil {
		t.Errorf("error writing agreement2: %v", err2)
	}

	// save event logs
	if err := LogServiceEvent(db, persistence.SEVERITY_INFO, "Started service configuration", persistence.EC_START_SERVICE_CONFIG, *src1); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogServiceEvent(db, persistence.SEVERITY_INFO, "Saved service into db", persistence.EC_COMPLETE_UPGRADE_SERVICE, *src1); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogServiceEvent(db, persistence.SEVERITY_INFO, "Started service configuration", persistence.EC_START_SERVICE_CONFIG, *src2); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogServiceEvent(db, persistence.SEVERITY_INFO, "Saved service into db", persistence.EC_COMPLETE_UPGRADE_SERVICE, *src2); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogServiceEvent(db, persistence.SEVERITY_INFO, "Succeeded service configuration", persistence.EC_SERVICE_CONFIG_COMPLETE, *src1); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogServiceEvent(db, persistence.SEVERITY_INFO, "Succeded service configuration", persistence.EC_SERVICE_CONFIG_COMPLETE, *src2); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogServiceEvent(db, persistence.SEVERITY_ERROR, "Failed to start service.", persistence.EC_ERROR_SERVICE_CONFIG, *src1); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogServiceEvent(db, persistence.SEVERITY_INFO, "Service up an running", persistence.EC_COMPLETE_SERVICE_STARTUP, *src2); err != nil {
		t.Errorf("error saving event log: %v", err)
	}

	// get event logs
	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 8, len(elogs), "Test GetEventLogs without selection. Total 8 entries.")
	}

	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{"message": {{"~", "Saved"}, {"=", "Service up an running"}}}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 0, len(elogs), "Test GetEventLogs with selection.")
	}

	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{"service_url": {{"~", "sensor1"}}}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 4, len(elogs), "Test GetEventLogs with selection.")
	}

	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{"event_code": {{"=", persistence.EC_SERVICE_CONFIG_COMPLETE}}}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 2, len(elogs), "Test GetEventLogs with selection.")
	}
}

func Test_LogServiceEvent2(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	// save event logs
	if err := LogServiceEvent2(db, persistence.SEVERITY_INFO, "Started service configuration", persistence.EC_START_SERVICE_CONFIG, "", "http://sensor1.org", "mycomp", "1.2.0", "amd64", []string{}); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogServiceEvent2(db, persistence.SEVERITY_INFO, "Saved service into db", persistence.EC_COMPLETE_UPGRADE_SERVICE, "", "http://sensor1.org", "mycomp", "1.2.0", "amd64", []string{}); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogServiceEvent2(db, persistence.SEVERITY_INFO, "Started service configuration", persistence.EC_START_SERVICE_CONFIG, "", "http://sensor2.org", "e2edev", "1.0.0", "amd64", []string{}); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogServiceEvent2(db, persistence.SEVERITY_INFO, "Saved service into db", persistence.EC_COMPLETE_UPGRADE_SERVICE, "", "http://sensor2.org", "e2edev", "1.0.0", "amd64", []string{}); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogServiceEvent2(db, persistence.SEVERITY_INFO, "Succeeded service configuration", persistence.EC_SERVICE_CONFIG_COMPLETE, "", "http://sensor1.org", "mycomp", "1.2.0", "amd64", []string{}); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogServiceEvent2(db, persistence.SEVERITY_INFO, "Succeded service configuration", persistence.EC_SERVICE_CONFIG_COMPLETE, "", "http://sensor2.org", "e2edev", "1.0.0", "amd64", []string{}); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogServiceEvent2(db, persistence.SEVERITY_ERROR, "Failed to start service.", persistence.EC_ERROR_SERVICE_CONFIG, "instance id1", "http://sensor1.org", "mycomp", "1.2.0", "amd64", []string{"agreementId1"}); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogServiceEvent2(db, persistence.SEVERITY_INFO, "Service up an running", persistence.EC_COMPLETE_SERVICE_STARTUP, "instance id2", "http://sensor2.org", "e2edev", "1.0.0", "amd64", []string{}); err != nil {
		t.Errorf("error saving event log: %v", err)
	}

	// get event logs
	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 8, len(elogs), "Test GetEventLogs without selection. Total 8 entries.")
	}

	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{"message": {{"~", "Saved"}, {"=", "Service up an running"}}}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 0, len(elogs), "Test GetEventLogs with selection.")
	}

	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{"agreement_id": {{"=", "agreementId1"}}}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 1, len(elogs), "Test GetEventLogs with selection.")
	}

	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{"service_url": {{"~", "sensor1"}}}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 4, len(elogs), "Test GetEventLogs with selection.")
	}

	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{"event_code": {{"=", persistence.EC_SERVICE_CONFIG_COMPLETE}}}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 2, len(elogs), "Test GetEventLogs with selection.")
	}
}

func Test_LogNodeEvent(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	// save event logs
	if err := LogNodeEvent(db, persistence.SEVERITY_INFO, "Started node configuration", persistence.EC_START_NODE_CONFIG_REG, "node1", "mycomp1", "mycomp1/pattern1", "configuring"); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogNodeEvent(db, persistence.SEVERITY_INFO, "Completed node configuration", persistence.EC_NODE_CONFIG_REG_COMPLETE, "node1", "mycomp1", "mycomp1/pattern1", "configured"); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogNodeEvent(db, persistence.SEVERITY_INFO, "Started node configuration", persistence.EC_START_NODE_CONFIG_REG, "node2", "mycomp2", "e2edev/pattern2", "configuring"); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogNodeEvent(db, persistence.SEVERITY_INFO, "Completed node configuration", persistence.EC_NODE_CONFIG_REG_COMPLETE, "node2", "mycomp2", "e2edev/pattern2", "configuring"); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogNodeEvent(db, persistence.SEVERITY_ERROR, "Something is wrong with the node", persistence.EC_ERROR_NODE_CONFIG_REG, "node1", "mycomp1", "mycomp1/pattern1", "configuring"); err != nil {
		t.Errorf("error saving event log: %v", err)
	}
	// get event logs
	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 5, len(elogs), "Test GetEventLogs without selection. Total 5 entries.")
	}

	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{"node_id": {{"=", "node1"}}}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 3, len(elogs), "Test GetEventLogs with selection.")
	}

	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{"message": {{"~", "configuration"}}}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 4, len(elogs), "Test GetEventLogs with selection.")
	}

	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{"severity": {{"=", "error"}}}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 1, len(elogs), "Test GetEventLogs with selection.")
	}

	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{"event_code": {{"=", persistence.EC_ERROR_NODE_CONFIG_REG}}}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 1, len(elogs), "Test GetEventLogs with selection.")
	}

}

func Test_LogDatabaseEvent(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	// save event logs
	if err := LogDatabaseEvent(db, persistence.SEVERITY_ERROR, "Error saving blah into db.", persistence.EC_DATABASE_ERROR); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogDatabaseEvent(db, persistence.SEVERITY_ERROR, "Error saving aaa into db", persistence.EC_DATABASE_ERROR); err != nil {
		t.Errorf("error saving event log: %v", err)
	}

	// get event logs
	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 2, len(elogs), "Test GetEventLogs without selection. Total 2 entries.")
	}

	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{"severity": {{"=", persistence.SEVERITY_ERROR}}}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 2, len(elogs), "Test GetEventLogs with selection.")
	}
}

func Test_LogExchangeEvent(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	// save event logs
	if err := LogExchangeEvent(db, persistence.SEVERITY_ERROR, "Error saving blah into exchange.", persistence.EC_EXCHANGE_ERROR, "http://exchange.con/v1"); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := LogExchangeEvent(db, persistence.SEVERITY_ERROR, "Error saving aaa into exchange.", persistence.EC_EXCHANGE_ERROR, "http://exchange.con/v2"); err != nil {
		t.Errorf("error saving event log: %v", err)
	}

	// get event logs
	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 2, len(elogs), "Test GetEventLogs without selection. Total 2 entries.")
	}

	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{"event_code": {{"=", persistence.EC_EXCHANGE_ERROR}}}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 2, len(elogs), "Test GetEventLogs with selection.")
	}

	if elogs, err := GetEventLogs(db, true, map[string][]persistence.Selector{"exchange_url": {{"=", "http://exchange.con/v1"}}}); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 1, len(elogs), "Test GetEventLogs with selection.")
	}

}

func utsetup() (string, *bolt.DB, error) {
	dir, err := ioutil.TempDir("", "utdb-")
	if err != nil {
		return "", nil, err
	}

	db, err := bolt.Open(path.Join(dir, "anax-int.db"), 0600, &bolt.Options{Timeout: 10 * time.Second})
	if err != nil {
		return dir, nil, err
	}

	return dir, db, nil
}

// Make a deferred call to this function after calling setup(), passing the output dirpath of the setup() function.
func cleanTestDir(dirPath string) error {
	if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
		if err := os.RemoveAll(dirPath); err != nil {
			return err
		}
	}
	return nil
}
