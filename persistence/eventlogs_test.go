// +build unit

package persistence

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
	"time"
)

func Test_EventLog_Matches(t *testing.T) {
	sp11 := ServiceSpec{Url: "http://mycom.com", Org: "mycom"}
	sp12 := ServiceSpec{Url: "http://service12.com", Org: "service12"}
	sp21 := ServiceSpec{Url: "http://mycom21.com", Org: "mycom21"}
	sp22 := ServiceSpec{Url: "http://mycome22.com", Org: "mycome22"}
	sp31 := ServiceSpec{Url: "http://service31.com", Org: "service31"}
	sp32 := ServiceSpec{Url: "http://service32.com", Org: "service32"}

	source1 := NewAgreementEventSource("agreement id 1", WorkloadInfo{"http://top1.com", "mycomp", "1.0.0", "amd64"}, []ServiceSpec{sp11, sp12}, "agbot1", "basic")
	source2 := NewAgreementEventSource("agreement id 2", WorkloadInfo{"http://top2.com", "mycomp", "1.0.0", "amd64"}, []ServiceSpec{sp21, sp22}, "agbot2", "cs")
	source3 := NewAgreementEventSource("agreement id 3", WorkloadInfo{"http://top3.com", "mycomp", "1.0.0", "amd64"}, []ServiceSpec{sp31, sp32}, "agbot1", "basic")

	e1 := newEventLog1(SEVERITY_INFO, "message 11", nil, EC_START_NODE_CONFIG_REG, SRC_TYPE_AG, *source1)
	e2 := newEventLog1(SEVERITY_INFO, "message 21", nil, EC_START_NODE_CONFIG_REG, SRC_TYPE_AG, *source2)
	e3 := newEventLog1(SEVERITY_ERROR, "message 12", nil, EC_NODE_CONFIG_REG_COMPLETE, SRC_TYPE_AG, *source1)
	e4 := newEventLog1(SEVERITY_INFO, "test 31", nil, EC_START_NODE_CONFIG_REG, SRC_TYPE_AG, *source3)
	e5 := newEventLog1(SEVERITY_WARN, "test 22", nil, EC_NODE_CONFIG_REG_COMPLETE, SRC_TYPE_AG, *source2)
	e6 := newEventLog1(SEVERITY_INFO, "message 32", nil, EC_NODE_CONFIG_REG_COMPLETE, SRC_TYPE_AG, *source3)
	e7 := newEventLog1(SEVERITY_INFO, "message 13", nil, EC_START_NODE_UPDATE, SRC_TYPE_AG, *source1)
	e8 := newEventLog1(SEVERITY_ERROR, "test 23", nil, EC_ERROR_NODE_CONFIG_REG, SRC_TYPE_AG, *source2)

	selectors := make(map[string][]Selector)
	selectors["source_type"] = []Selector{{"=", SRC_TYPE_AG}}
	selectors["timestamp"] = []Selector{{">", 100}}
	selectors["message"] = []Selector{{"~", "message"}}
	selectors["severity"] = []Selector{{"=", SEVERITY_INFO}}
	selectors["agreement_id"] = []Selector{{"~", "id"}}
	selectors["service_url"] = []Selector{{"~", "service"}}
	selectors["consumer_id"] = []Selector{{"=", "agbot1"}}

	assert.True(t, e1.Matches(selectors), "Test eventlog Matches.")
	assert.False(t, e2.Matches(selectors), "Test eventlog Matches.")
	assert.False(t, e3.Matches(selectors), "Test eventlog Matches.")
	assert.False(t, e4.Matches(selectors), "Test eventlog Matches.")
	assert.False(t, e5.Matches(selectors), "Test eventlog Matches.")
	assert.True(t, e6.Matches(selectors), "Test eventlog Matches.")
	assert.True(t, e7.Matches(selectors), "Test eventlog Matches.")
	assert.False(t, e8.Matches(selectors), "Test eventlog Matches.")

	selectors["event_code"] = []Selector{{"=", EC_START_NODE_CONFIG_REG}}
	assert.True(t, e1.Matches(selectors), "Test eventlog Matches.")
	assert.False(t, e2.Matches(selectors), "Test eventlog Matches.")
	assert.False(t, e3.Matches(selectors), "Test eventlog Matches.")
	assert.False(t, e4.Matches(selectors), "Test eventlog Matches.")
	assert.False(t, e5.Matches(selectors), "Test eventlog Matches.")
	assert.False(t, e6.Matches(selectors), "Test eventlog Matches.")
	assert.False(t, e7.Matches(selectors), "Test eventlog Matches.")
	assert.False(t, e8.Matches(selectors), "Test eventlog Matches.")

	// not tolerte the extra selectors
	selectors["extra"] = []Selector{{"=", "extra"}}
	assert.False(t, e1.Matches(selectors), "Test eventlog Matches.")
	assert.False(t, e2.Matches(selectors), "Test eventlog Matches.")
	assert.False(t, e3.Matches(selectors), "Test eventlog Matches.")
	assert.False(t, e4.Matches(selectors), "Test eventlog Matches.")
	assert.False(t, e5.Matches(selectors), "Test eventlog Matches.")
	assert.False(t, e6.Matches(selectors), "Test eventlog Matches.")
	assert.False(t, e7.Matches(selectors), "Test eventlog Matches.")
	assert.False(t, e8.Matches(selectors), "Test eventlog Matches.")

}

func Test_EventLog_Matches2(t *testing.T) {
	sp11 := ServiceSpec{Url: "http://mycom.com", Org: "mycom"}
	sp12 := ServiceSpec{Url: "http://service12.com", Org: "service12"}
	sp21 := ServiceSpec{Url: "http://mycom21.com", Org: "mycom21"}
	sp22 := ServiceSpec{Url: "http://mycome22.com", Org: "mycome22"}
	sp31 := ServiceSpec{Url: "http://service31.com", Org: "service31"}
	sp32 := ServiceSpec{Url: "http://service32.com", Org: "service32"}

	source1 := NewAgreementEventSource("agreement id 1", WorkloadInfo{"http://top1.com", "mycomp", "1.0.0", "amd64"}, []ServiceSpec{sp11, sp12}, "agbot1", "basic")
	source2 := NewAgreementEventSource("agreement id 2", WorkloadInfo{"http://top2.com", "mycomp", "1.0.0", "amd64"}, []ServiceSpec{sp21, sp22}, "agbot2", "cs")
	source3 := NewAgreementEventSource("agreement id 3", WorkloadInfo{"http://top3.com", "mycomp", "1.0.0", "amd64"}, []ServiceSpec{sp31, sp32}, "agbot1", "basic")

	e1 := newEventLog1(SEVERITY_INFO, "message 11", nil, EC_START_NODE_CONFIG_REG, SRC_TYPE_AG, *source1)
	e2 := newEventLog1(SEVERITY_INFO, "message 21", nil, EC_START_NODE_CONFIG_REG, SRC_TYPE_AG, *source2)
	e3 := newEventLog1(SEVERITY_ERROR, "message 12", nil, EC_NODE_CONFIG_REG_COMPLETE, SRC_TYPE_AG, *source1)
	e4 := newEventLog1(SEVERITY_INFO, "message 31", nil, EC_START_NODE_CONFIG_REG, SRC_TYPE_AG, *source3)
	e5 := newEventLog1(SEVERITY_WARN, "message 22", nil, EC_NODE_CONFIG_REG_COMPLETE, SRC_TYPE_AG, *source2)
	e6 := newEventLog1(SEVERITY_INFO, "message 32", nil, EC_NODE_CONFIG_REG_COMPLETE, SRC_TYPE_AG, *source3)
	e7 := newEventLog1(SEVERITY_INFO, "message 13", nil, EC_ERROR_NODE_CONFIG_REG, SRC_TYPE_AG, *source1)
	e8 := newEventLog1(SEVERITY_ERROR, "message 23", nil, EC_ERROR_NODE_CONFIG_REG, SRC_TYPE_AG, *source2)

	base_selectors := make(map[string][]Selector)
	base_selectors["source_type"] = []Selector{{"=", SRC_TYPE_AG}}
	base_selectors["timestamp"] = []Selector{{">", "100"}}
	base_selectors["message"] = []Selector{{"~", "message"}}
	base_selectors["severity"] = []Selector{{"=", SEVERITY_INFO}}

	source_selectors := make(map[string][]Selector)
	source_selectors["agreement_id"] = []Selector{{"~", "id"}}
	source_selectors["service_url"] = []Selector{{"~", "service"}}
	source_selectors["consumer_id"] = []Selector{{"=", "agbot1"}}

	assert.True(t, e1.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	assert.False(t, e2.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	assert.False(t, e3.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	assert.True(t, e4.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	assert.False(t, e5.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	assert.True(t, e6.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	assert.True(t, e7.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	assert.False(t, e8.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")

	base_selectors["event_code"] = []Selector{{"=", EC_START_NODE_CONFIG_REG}}
	assert.True(t, e1.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	assert.False(t, e2.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	assert.False(t, e3.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	assert.True(t, e4.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	assert.False(t, e5.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	assert.False(t, e6.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	assert.False(t, e7.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	assert.False(t, e8.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")

	// test invalid base selector
	base_selectors["event_code222"] = []Selector{{"=", EC_START_NODE_CONFIG_REG}}
	assert.False(t, e1.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	delete(base_selectors, "event_code222")

	// not tolerate extra base selectors
	base_selectors["extra"] = []Selector{{"=", "extra value"}}
	assert.False(t, e1.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	assert.False(t, e2.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	assert.False(t, e3.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	assert.False(t, e4.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	assert.False(t, e5.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	assert.False(t, e6.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	assert.False(t, e7.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	assert.False(t, e8.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	delete(base_selectors, "extra")

	// not tolerate extra source selectors
	source_selectors["extra"] = []Selector{{"=", "extra value"}}
	assert.False(t, e1.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	assert.False(t, e2.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	assert.False(t, e3.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	assert.False(t, e4.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	assert.False(t, e5.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	assert.False(t, e6.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	assert.False(t, e7.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
	assert.False(t, e8.Matches2(base_selectors, source_selectors), "Test eventlog Matches2.")
}

func Test_MatchAttributeValue_string(t *testing.T) {

	// test string
	selectors := []Selector{{"=", "This is a test1."}, {"~", "test1"}}
	matched, handled, err := MatchAttributeValue("This is a test.", selectors)
	assert.False(t, matched, "Should match")
	assert.True(t, handled, "Should be handled")
	assert.Nil(t, err, "Error should be nil")

	matched, handled, err = MatchAttributeValue("This is a test1.", selectors)
	assert.True(t, matched, "Should match")
	assert.True(t, handled, "Should be handled")
	assert.Nil(t, err, "Error should be nil")

	matched, handled, err = MatchAttributeValue("This is a test2.", selectors)
	assert.False(t, matched, "Should not match")
	assert.True(t, handled, "Should be handled")
	assert.Nil(t, err, "Error should be nil")

	selectors = []Selector{{">", "bbb"}}
	matched, handled, err = MatchAttributeValue("c", selectors)
	assert.True(t, matched, "Should match")
	assert.True(t, handled, "Should be handled")
	assert.Nil(t, err, "Error should be nil")

	matched, handled, err = MatchAttributeValue("a", selectors)
	assert.False(t, matched, "Should not match")
	assert.True(t, handled, "Should be handled")
	assert.Nil(t, err, "Error should be nil")

	selectors = []Selector{{"<", "bbb"}}
	matched, handled, err = MatchAttributeValue("c", selectors)
	assert.False(t, matched, "Should not match")
	assert.True(t, handled, "Should be handled")
	assert.Nil(t, err, "Error should be nil")

	matched, handled, err = MatchAttributeValue("a this is a test", selectors)
	assert.True(t, matched, "Should match")
	assert.True(t, handled, "Should be handled")
	assert.Nil(t, err, "Error should be nil")

}

func Test_MatchAttributeValue_numbers(t *testing.T) {

	// test string
	selectors := []Selector{{">", 100}, {"<", 300}}
	matched, handled, err := MatchAttributeValue(int(200), selectors)
	assert.True(t, matched, "Should match")
	assert.True(t, handled, "Should be handled")
	assert.Nil(t, err, "Error should be nil")

	matched, handled, err = MatchAttributeValue(uint64(301), selectors)
	assert.False(t, matched, "Should match")
	assert.True(t, handled, "Should be handled")
	assert.Nil(t, err, "Error should be nil")

	matched, handled, err = MatchAttributeValue(float32(99), selectors)
	assert.False(t, matched, "Should match")
	assert.True(t, handled, "Should be handled")
	assert.Nil(t, err, "Error should be nil")

	matched, handled, err = MatchAttributeValue(float64(201), selectors)
	assert.True(t, matched, "Should not match")
	assert.True(t, handled, "Should be handled")
	assert.Nil(t, err, "Error should be nil")

	matched, handled, err = MatchAttributeValue("200", selectors)
	assert.True(t, matched, "Should match")
	assert.True(t, handled, "Should be handled")
	assert.Nil(t, err, "Error should be nil")

	matched, handled, err = MatchAttributeValue("301", selectors)
	assert.False(t, matched, "Should match")
	assert.True(t, handled, "Should be handled")
	assert.Nil(t, err, "Error should be nil")

	matched, handled, err = MatchAttributeValue("99", selectors)
	assert.False(t, matched, "Should match")
	assert.True(t, handled, "Should be handled")
	assert.Nil(t, err, "Error should be nil")

	matched, handled, err = MatchAttributeValue("201", selectors)
	assert.True(t, matched, "Should not match")
	assert.True(t, handled, "Should be handled")
	assert.Nil(t, err, "Error should be nil")

	matched, handled, err = MatchAttributeValue("99c", selectors)
	assert.False(t, matched, "Should not match")
	assert.True(t, handled, "Should be handled")
	assert.NotNil(t, err, "Error should not be nil")

	selectors = []Selector{{"~", 200}}
	matched, handled, err = MatchAttributeValue("99", selectors)
	assert.False(t, matched, "Should not match")
	assert.True(t, handled, "Should be handled")
	assert.NotNil(t, err, "Error should not be nil")

}

func Test_MatchAttributeValue_boolean(t *testing.T) {

	// test string
	selectors := []Selector{{"=", true}}
	matched, handled, err := MatchAttributeValue(true, selectors)
	assert.True(t, matched, "Should match")
	assert.True(t, handled, "Should be handled")
	assert.Nil(t, err, "Error should be nil")

	matched, handled, err = MatchAttributeValue(false, selectors)
	assert.False(t, matched, "Should not match")
	assert.True(t, handled, "Should be handled")
	assert.Nil(t, err, "Error should be nil")

	matched, handled, err = MatchAttributeValue("This is a test2.", selectors)
	assert.False(t, matched, "Should not match")
	assert.True(t, handled, "Should be handled")
	assert.NotNil(t, err, "Error should not be nil")

	selectors = []Selector{{"=", false}}
	matched, handled, err = MatchAttributeValue(false, selectors)
	assert.True(t, matched, "Should match")
	assert.True(t, handled, "Should be handled")
	assert.Nil(t, err, "Error should be nil")

	matched, handled, err = MatchAttributeValue(true, selectors)
	assert.False(t, matched, "Should not match")
	assert.True(t, handled, "Should be handled")
	assert.Nil(t, err, "Error should be nil")

	selectors = []Selector{{">", false}}
	matched, handled, err = MatchAttributeValue(false, selectors)
	assert.False(t, matched, "Should not match")
	assert.True(t, handled, "Should be handled")
	assert.NotNil(t, err, "Error should not be nil")

}

func Test_GroupSelectors(t *testing.T) {
	selectors := make(map[string][]Selector)
	s1 := []Selector{{"=", "agreement"}}
	s2 := []Selector{{">", 100}, {"<", 300}}
	s3 := []Selector{{"=", "this is a test1."}}
	s4 := []Selector{{"~", "456"}, {"~", "test"}}

	selectors["source_type"] = s1
	selectors["x1"] = s1
	selectors["message"] = s1
	selectors["timestamp"] = s2
	selectors["x2"] = s2
	selectors["record_id"] = s3
	selectors["x3"] = s3
	selectors["severity"] = s4
	selectors["x4"] = s4

	base_selectors, source_selectors := GroupSelectors(selectors)

	base_s := "source_type,severity,message,record_id,timestamp"
	for attr_name, _ := range base_selectors {
		assert.True(t, strings.Contains(base_s, attr_name), "Should be in the base attribute name list.")
	}
	for attr_name, _ := range source_selectors {
		assert.False(t, strings.Contains(base_s, attr_name), "Should not be in the base attribute name list.")
	}

	assert.Equal(t, s1, base_selectors["source_type"], "Selector value should be correct.")
	assert.Equal(t, s4, base_selectors["severity"], "Selector value should be correct.")
	assert.Equal(t, s1, base_selectors["message"], "Selector value should be correct.")
	assert.Equal(t, s3, base_selectors["record_id"], "Selector value should be correct.")
	assert.Equal(t, s2, base_selectors["timestamp"], "Selector value should be correct.")
	assert.Equal(t, s1, source_selectors["x1"], "Selector value should be correct.")
	assert.Equal(t, s2, source_selectors["x2"], "Selector value should be correct.")
	assert.Equal(t, s3, source_selectors["x3"], "Selector value should be correct.")
	assert.Equal(t, s4, source_selectors["x4"], "Selector value should be correct.")

	assert.Equal(t, 5, len(base_selectors), "The base selectors should have 5 items.")
	assert.Equal(t, 4, len(source_selectors), "The source selectors should have 4 items.")
}

func Test_ConvertToSelectors(t *testing.T) {
	selections := make(map[string][]string)
	selections["source_type"] = []string{"this is a test", "~test1", ">abc", "<hhh"}
	selections["record_id"] = []string{">100", "<50", "75"}
	selections["publishable"] = []string{"true", "FALSE"}

	selectors, err := ConvertToSelectors(selections)
	assert.Equal(t, 3, len(selectors), "The selectors should have 3 items.")
	assert.Nil(t, err, "Error should be nil")

	assert.Equal(t, len(selections["source_type"]), len(selectors["source_type"]), "Selectors should stay same for each item.")
	assert.Equal(t, "=", selectors["source_type"][0].Op, "Check operator")
	assert.Equal(t, "this is a test", selectors["source_type"][0].MatchValue, "Check match value.")
	assert.Equal(t, "~", selectors["source_type"][1].Op, "Check operator")
	assert.Equal(t, "test1", selectors["source_type"][1].MatchValue, "Check match value.")
	assert.Equal(t, ">", selectors["source_type"][2].Op, "Check operator")
	assert.Equal(t, "abc", selectors["source_type"][2].MatchValue, "Check match value.")
	assert.Equal(t, "<", selectors["source_type"][3].Op, "Check operator")
	assert.Equal(t, "hhh", selectors["source_type"][3].MatchValue, "Check match value.")

	assert.Equal(t, len(selections["record_id"]), len(selectors["record_id"]), "Selectors should stay same for each item.")
	assert.Equal(t, ">", selectors["record_id"][0].Op, "Check operator")
	assert.Equal(t, float64(100), selectors["record_id"][0].MatchValue, "Check match value.")
	assert.Equal(t, "<", selectors["record_id"][1].Op, "Check operator")
	assert.Equal(t, float64(50), selectors["record_id"][1].MatchValue, "Check match value.")
	assert.Equal(t, "=", selectors["record_id"][2].Op, "Check operator")
	assert.Equal(t, float64(75), selectors["record_id"][2].MatchValue, "Check match value.")

	assert.Equal(t, len(selections["publishable"]), len(selectors["publishable"]), "Selectors should stay same for each item.")
	assert.Equal(t, "=", selectors["publishable"][0].Op, "Check operator")
	assert.True(t, true, selectors["publishable"][0].MatchValue, "Check match value.")
	assert.Equal(t, "=", selectors["publishable"][1].Op, "Check operator")
	assert.Equal(t, false, selectors["publishable"][1].MatchValue, "Check match value.")

}

func Test_Save_and_Get_LastUnregistrationTime(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	now := uint64(time.Now().Unix())

	if err := SaveLastUnregistrationTime(db, now); err != nil {
		t.Errorf("Erorr saving last unreg time into db. %v", err)
	}

	// get the last unregister time
	tl, err := GetLastUnregistrationTime(db)
	if err != nil {
		t.Errorf("Erorr retrieving last unregistration time. %v", err)
	}

	assert.Equal(t, now, tl, "Saved and retrieved for last unregistration time should be same.")
}

func Test_SaveLastUnregistrationTime(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	sp11 := ServiceSpec{Url: "http://mycom.com", Org: "mycom"}
	sp12 := ServiceSpec{Url: "http://service12.com", Org: "service12"}
	sp21 := ServiceSpec{Url: "http://mycom21.com", Org: "mycom21"}
	sp22 := ServiceSpec{Url: "http://mycome22.com", Org: "mycome22"}
	sp31 := ServiceSpec{Url: "http://service31.com", Org: "service31"}
	sp32 := ServiceSpec{Url: "http://service32.com", Org: "service32"}

	source1 := NewAgreementEventSource("agreement id 1", WorkloadInfo{"http://top1.com", "mycomp", "1.0.0", "amd64"}, []ServiceSpec{sp11, sp12}, "agbot1", "basic")
	source2 := NewAgreementEventSource("agreement id 2", WorkloadInfo{"http://top2.com", "mycomp", "1.0.0", "amd64"}, []ServiceSpec{sp21, sp22}, "agbot2", "cs")
	source3 := NewAgreementEventSource("agreement id 3", WorkloadInfo{"http://top3.com", "mycomp", "1.0.0", "amd64"}, []ServiceSpec{sp31, sp32}, "agbot1", "basic")

	e1 := newEventLog1(SEVERITY_INFO, "message 11", nil, EC_START_NODE_CONFIG_REG, SRC_TYPE_AG, *source1)
	e2 := newEventLog1(SEVERITY_INFO, "message 21", nil, EC_START_NODE_CONFIG_REG, SRC_TYPE_AG, *source2)
	e3 := newEventLog1(SEVERITY_ERROR, "message 12", nil, EC_NODE_CONFIG_REG_COMPLETE, SRC_TYPE_AG, *source1)
	if err := SaveEventLog(db, e1); err != nil {
		t.Errorf("Erorr saving eventlog into db. %v", err)
	} else if err := SaveEventLog(db, e2); err != nil {
		t.Errorf("Erorr saving eventlog into db. %v", err)
	} else if err := SaveEventLog(db, e3); err != nil {
		t.Errorf("Erorr saving eventlog into db. %v", err)
	}

	if err := SaveLastUnregistrationTime(db, uint64(time.Now().Unix())); err != nil {
		t.Errorf("Erorr saving last unreg time into db. %v", err)
	} else {
		time.Sleep(1 * time.Second)
	}

	e4 := newEventLog1(SEVERITY_INFO, "test 31", nil, EC_START_NODE_CONFIG_REG, SRC_TYPE_AG, *source3)
	e5 := newEventLog1(SEVERITY_WARN, "test 22", nil, EC_NODE_CONFIG_REG_COMPLETE, SRC_TYPE_AG, *source2)
	e6 := newEventLog1(SEVERITY_INFO, "message 32", nil, EC_NODE_CONFIG_REG_COMPLETE, SRC_TYPE_AG, *source3)
	e7 := newEventLog1(SEVERITY_INFO, "message 13", nil, EC_START_NODE_UPDATE, SRC_TYPE_AG, *source1)
	e8 := newEventLog1(SEVERITY_ERROR, "test 23", nil, EC_ERROR_NODE_CONFIG_REG, SRC_TYPE_AG, *source2)
	if err := SaveEventLog(db, e4); err != nil {
		t.Errorf("Erorr saving eventlog into db. %v", err)
	} else if err := SaveEventLog(db, e5); err != nil {
		t.Errorf("Erorr saving eventlog into db. %v", err)
	} else if err := SaveEventLog(db, e6); err != nil {
		t.Errorf("Erorr saving eventlog into db. %v", err)
	} else if err := SaveEventLog(db, e7); err != nil {
		t.Errorf("Erorr saving eventlog into db. %v", err)
	} else if err := SaveEventLog(db, e8); err != nil {
		t.Errorf("Erorr saving eventlog into db. %v", err)
	}

	// get all event logs
	if elogs, err := FindEventLogsWithSelectors(db, true, map[string][]Selector{}, nil); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 8, len(elogs), "Test FindEventLogsWithSelectors without selection. Total 8 entries.")
	}

	// get the logs for current registration
	if elogs, err := FindEventLogsWithSelectors(db, false, map[string][]Selector{}, nil); err != nil {
		t.Errorf("error getting event logs: %v", err)
	} else {
		assert.Equal(t, 5, len(elogs), "Test FindEventLogsWithSelectors for current registration without selection. Total 5 entries.")
	}

	selectors := make(map[string][]Selector)
	selectors["source_type"] = []Selector{{"=", SRC_TYPE_AG}}
	selectors["timestamp"] = []Selector{{">", 100}}
	selectors["message"] = []Selector{{"~", "message"}}
	selectors["severity"] = []Selector{{"=", SEVERITY_INFO}}
	selectors["agreement_id"] = []Selector{{"~", "id"}}
	selectors["service_url"] = []Selector{{"~", "service"}}
	selectors["consumer_id"] = []Selector{{"=", "agbot1"}}

	assert.True(t, e1.Matches(selectors), "Test eventlog Matches.")
	assert.False(t, e2.Matches(selectors), "Test eventlog Matches.")
	assert.False(t, e3.Matches(selectors), "Test eventlog Matches.")
	assert.False(t, e4.Matches(selectors), "Test eventlog Matches.")
	assert.False(t, e5.Matches(selectors), "Test eventlog Matches.")
	assert.True(t, e6.Matches(selectors), "Test eventlog Matches.")
	assert.True(t, e7.Matches(selectors), "Test eventlog Matches.")
	assert.False(t, e8.Matches(selectors), "Test eventlog Matches.")

}
