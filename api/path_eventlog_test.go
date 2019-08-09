// +build unit

package api

import (
	"flag"
	"github.com/open-horizon/anax/eventlog"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/persistence"
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
)

func init() {
	flag.Set("alsologtostderr", "true")
	flag.Set("v", "7")
	// no need to parse flags, that's done by test framework
}

func Test_FindEventLogsForOutput(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	msgPrinter := i18n.GetMessagePrinterWithLocale("en")

	sp1 := persistence.ServiceSpec{Url: "http://sensor1.org", Org: "sensor1"}
	sp2 := persistence.ServiceSpec{Url: "http://mycomp.org", Org: "myorg"}
	sp3 := persistence.ServiceSpec{Url: "http://sensor3.org", Org: "sensor3"}

	// save event logs
	if err := eventlog.LogAgreementEvent2(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta("proposal received."), persistence.EC_RECEIVED_PROPOSAL, "agreementId1", persistence.WorkloadInfo{"http://top1.com", "myorg", "1.0.0", "amd64"}, []persistence.ServiceSpec{sp1}, "consumerId", "Basic"); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := eventlog.LogAgreementEvent2(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta("reply sent."), persistence.EC_RECEIVED_REPLYACK_MESSAGE, "agreementId1", persistence.WorkloadInfo{"http://top1.com", "myorg", "1.0.0", "amd64"}, []persistence.ServiceSpec{sp1}, "consumerId", "Basic"); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := eventlog.LogAgreementEvent2(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta("proposal received."), persistence.EC_RECEIVED_PROPOSAL, "agreementId2", persistence.WorkloadInfo{"http://top2.com", "myorg", "1.0.0", "amd64"}, []persistence.ServiceSpec{sp2}, "consumerId", "Basic"); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := eventlog.LogAgreementEvent2(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta("proposal received."), persistence.EC_RECEIVED_PROPOSAL, "agreementId3", persistence.WorkloadInfo{"http://top3.com", "myorg", "1.0.0", "amd64"}, []persistence.ServiceSpec{sp3}, "consumerId", "Basic"); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := eventlog.LogAgreementEvent2(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta("reply sent."), persistence.EC_RECEIVED_REPLYACK_MESSAGE, "agreementId2", persistence.WorkloadInfo{"http://top2.com", "myorg", "1.0.0", "amd64"}, []persistence.ServiceSpec{sp2}, "consumerId", "Basic"); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := eventlog.LogAgreementEvent2(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta("agreement finalized."), persistence.EC_AGREEMENT_REACHED, "agreementId1", persistence.WorkloadInfo{"http://top1.com", "myorg", "1.0.0", "amd64"}, []persistence.ServiceSpec{sp1}, "consumerId", "Basic"); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := eventlog.LogAgreementEvent2(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta("reply sent."), persistence.EC_RECEIVED_REPLYACK_MESSAGE, "agreementId3", persistence.WorkloadInfo{"http://top3.com", "myorg", "1.0.0", "amd64"}, []persistence.ServiceSpec{sp3}, "consumerId", "Basic"); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := eventlog.LogAgreementEvent2(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta("agreement finalized."), persistence.EC_AGREEMENT_REACHED, "agreementId3", persistence.WorkloadInfo{"http://top3.com", "myorg", "1.0.0", "amd64"}, []persistence.ServiceSpec{sp3}, "consumerId", "Basic"); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := eventlog.LogAgreementEvent2(db, persistence.SEVERITY_INFO, persistence.NewMessageMeta("agreement finalized."), persistence.EC_AGREEMENT_REACHED, "agreementId2", persistence.WorkloadInfo{"http://top2.com", "myorg", "1.0.0", "amd64"}, []persistence.ServiceSpec{sp2}, "consumerId", "Basic"); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := eventlog.LogAgreementEvent2(db, persistence.SEVERITY_ERROR, persistence.NewMessageMeta("something is wrong."), persistence.EC_ERROR_START_CONTAINER, "agreementId1", persistence.WorkloadInfo{"http://top1.com", "myorg", "1.0.0", "amd64"}, []persistence.ServiceSpec{sp1}, "consumerId", "Basic"); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := eventlog.LogAgreementEvent2(db, persistence.SEVERITY_WARN, persistence.NewMessageMeta("something is wrong."), persistence.EC_ERROR_START_CONTAINER, "agreementId2", persistence.WorkloadInfo{"http://top2.com", "myorg", "1.0.0", "amd64"}, []persistence.ServiceSpec{sp2}, "consumerId", "Basic"); err != nil {
		t.Errorf("error saving event log: %v", err)
	} else if err := eventlog.LogAgreementEvent2(db, persistence.SEVERITY_ERROR, persistence.NewMessageMeta("something is really wrong."), persistence.EC_EXCHANGE_ERROR, "agreementId1", persistence.WorkloadInfo{"http://top1.com", "myorg", "1.0.0", "amd64"}, []persistence.ServiceSpec{sp1}, "consumerId", "Basic"); err != nil {
		t.Errorf("error saving event log: %v", err)
	}

	// get event logs
	if elogs, err := FindEventLogsForOutput(db, true, map[string][]string{}, msgPrinter); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 12, len(elogs), "Test FindEventLogsForOutput without selection. Total 12 entries.")

		//make sure the logs are sorted
		for i, elog := range elogs {
			if rec_id, err := strconv.Atoi(elog.Id); err != nil {
				t.Errorf("error converting string %v to interger: %v", elog.Id, err)
			} else if rec_id != i+1 {
				t.Errorf("The event logs are not sorted by the record id.")
			}
		}
	}

	if elogs, err := FindEventLogsForOutput(db, true, map[string][]string{"record_id": {"<5", "7", ">10"}}, msgPrinter); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 0, len(elogs), "Test FindEventLogsForOutput with selection.")
	}

	if elogs, err := FindEventLogsForOutput(db, false, map[string][]string{"message": {"~proposal", "~received"}}, msgPrinter); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 3, len(elogs), "Test FindEventLogsForOutput with selection.")
	}

	if elogs, err := FindEventLogsForOutput(db, true, map[string][]string{"agreement_id": {"agreementId3"}}, msgPrinter); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 3, len(elogs), "Test FindEventLogsForOutput with selection.")
	}

	if elogs, err := FindEventLogsForOutput(db, false, map[string][]string{"consumer_id": {"consumerId"}}, msgPrinter); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 12, len(elogs), "Test FindEventLogsForOutput with selection.")

		//make sure the logs are sorted
		for i, elog := range elogs {
			if rec_id, err := strconv.Atoi(elog.Id); err != nil {
				t.Errorf("error converting string %v to interger: %v", elog.Id, err)
			} else if rec_id != i+1 {
				t.Errorf("The event logs are not sorted by the record id.")
			}
		}
	}

	if elogs, err := FindEventLogsForOutput(db, true, map[string][]string{"dependent_services.url": {"http://mycomp.org"}}, msgPrinter); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 4, len(elogs), "Test FindEventLogsForOutput with selection.")
	}

}
