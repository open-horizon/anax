// +build unit

package persistence

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_AgreementEventSource_Matches(t *testing.T) {

	source1 := NewAgreementEventSource("agreement id 1", WorkloadInfo{"http://top1.com", "mycomp", "1.0.0", "amd64"}, []string{"http://mycom.com", "http://service12.com"}, "agbot1", "basic")
	source2 := NewAgreementEventSource("agreement id 2", WorkloadInfo{"http://top2.com", "mycomp", "1.0.0", "amd64"}, []string{"http://mycom21.com", "http://mycome22.com"}, "agbot2", "cs")
	source3 := NewAgreementEventSource("agreement id 3", WorkloadInfo{"http://top3.com", "mycomp", "1.0.0", "amd64"}, []string{"http://service31.com", "http://service32.com"}, "agbot1", "basic")

	s1 := []Selector{{"~", "service"}}
	s2 := []Selector{{"~", "id"}}
	s3 := []Selector{{"~", "top1"}}
	s4 := []Selector{{"=", "agbot1"}}

	sel1 := make(map[string][]Selector)
	sel1["service_url"] = s1
	assert.True(t, source1.Matches(sel1), "Test source1")
	assert.False(t, source2.Matches(sel1), "Test source2")
	assert.True(t, source3.Matches(sel1), "Test source3")

	sel2 := make(map[string][]Selector)
	sel2["agreement_id"] = s2
	assert.True(t, source1.Matches(sel2), "Test source1")
	assert.True(t, source2.Matches(sel2), "Test source2")
	assert.True(t, source3.Matches(sel2), "Test source3")

	sel3 := make(map[string][]Selector)
	sel3["workload_to_run"] = s3
	assert.True(t, source1.Matches(sel3), "Test source1")
	assert.False(t, source2.Matches(sel3), "Test source2")
	assert.False(t, source3.Matches(sel3), "Test source3")

	sel4 := make(map[string][]Selector)
	sel4["consumer_id"] = s4
	assert.True(t, source1.Matches(sel4), "Test source1")
	assert.False(t, source2.Matches(sel4), "Test source2")
	assert.True(t, source3.Matches(sel4), "Test source3")

	sel_all := make(map[string][]Selector)
	sel_all["service_url"] = s1
	sel_all["agreement_id"] = s2
	sel_all["workload_to_run"] = s3
	sel_all["consumer_id"] = s4
	assert.True(t, source1.Matches(sel_all), "Test source1")
	assert.False(t, source2.Matches(sel_all), "Test source2")
	assert.False(t, source3.Matches(sel_all), "Test source3")
}

func Test_ServiceEventSource_Matches(t *testing.T) {

	var msdef MicroserviceDefinition
	msdef.Id = "msid"
	msdef.SpecRef = "http://test2.com"
	msdef.Org = "e2edev"
	msdef.Version = "1.0.0"
	msdef.Arch = "arm"

	var msi MicroserviceInstance
	msi.InstanceId = "instance_id2"
	msi.SpecRef = "http://test.com"
	msi.Version = "1.0.0"
	msi.Arch = "arm"
	msi.AssociatedAgreements = []string{"agreement id 2", "agreement id 3"}

	source1 := NewServiceEventSource("instance_id1", "http://test1.com", "mycomp1", "1.2.3", "amd64", []string{"agreement id 1", "agreement id 2"})
	source2 := NewServiceEventSourceFromServiceInstance(msi)
	source3 := NewServiceEventSource("instance_id3", "http://mycom.com", "mycomp3", "2.1", "amd64", []string{"agreement id 1"})
	source4 := NewServiceEventSourceFromServiceDef(msdef)

	sel2 := make(map[string][]Selector)
	sel2["agreement_id"] = []Selector{{"=", "agreement id 1"}}
	assert.True(t, source1.Matches(sel2), "Test source1")
	assert.False(t, source2.Matches(sel2), "Test source2")
	assert.True(t, source3.Matches(sel2), "Test source3")
	assert.False(t, source4.Matches(sel2), "Test source4")

	sel3 := make(map[string][]Selector)
	sel3["service_url"] = []Selector{{"~", "test"}}
	sel3["version"] = []Selector{{"=", "1.0.0"}}
	assert.False(t, source1.Matches(sel3), "Test source1")
	assert.True(t, source2.Matches(sel3), "Test source2")
	assert.False(t, source3.Matches(sel3), "Test source3")
	assert.True(t, source4.Matches(sel3), "Test source4")

	sel4 := make(map[string][]Selector)
	sel4["instance_id"] = []Selector{{"~", "id"}}
	assert.True(t, source1.Matches(sel4), "Test source1")
	assert.True(t, source2.Matches(sel4), "Test source2")
	assert.True(t, source3.Matches(sel4), "Test source3")
	assert.False(t, source4.Matches(sel4), "Test source4")

	sel5 := make(map[string][]Selector)
	sel5["organization"] = []Selector{{"=", "e2edev"}}
	assert.False(t, source1.Matches(sel5), "Test source1")
	assert.False(t, source2.Matches(sel5), "Test source2")
	assert.False(t, source3.Matches(sel5), "Test source3")
	assert.True(t, source4.Matches(sel5), "Test source4")
}

func Test_NodeEventSource_Matches(t *testing.T) {

	source1 := NewNodeEventSource("node1", "mycomp1", "e2edev/pattern1", "unconfigured")
	source2 := NewNodeEventSource("node2", "mycomp2", "e2edev/pattern2", "configured")
	source3 := NewNodeEventSource("node3", "mycomp3", "mycomp3/pattern3", "configuring")

	sel1 := make(map[string][]Selector)
	sel1["node_id"] = []Selector{{"~", "node"}}
	assert.True(t, source1.Matches(sel1), "Test source1")
	assert.True(t, source2.Matches(sel1), "Test source2")
	assert.True(t, source3.Matches(sel1), "Test source3")

	sel2 := make(map[string][]Selector)
	sel2["node_org"] = []Selector{{"=", "mycomp1"}}
	assert.True(t, source1.Matches(sel2), "Test source1")
	assert.False(t, source2.Matches(sel2), "Test source2")
	assert.False(t, source3.Matches(sel2), "Test source3")

	sel3 := make(map[string][]Selector)
	sel3["pattern"] = []Selector{{"~", "e2edev"}}
	assert.True(t, source1.Matches(sel3), "Test source1")
	assert.True(t, source2.Matches(sel3), "Test source2")
	assert.False(t, source3.Matches(sel3), "Test source3")

	sel4 := make(map[string][]Selector)
	sel4["config_state"] = []Selector{{"=", "configuring"}}
	assert.False(t, source1.Matches(sel4), "Test source1")
	assert.False(t, source2.Matches(sel4), "Test source2")
	assert.True(t, source3.Matches(sel4), "Test source3")

	sel5 := make(map[string][]Selector)
	sel5["node_org"] = []Selector{{"=", "mycomp3"}}
	sel5["pattern"] = []Selector{{"~", "mycomp3"}}
	assert.False(t, source1.Matches(sel5), "Test source1")
	assert.False(t, source2.Matches(sel5), "Test source2")
	assert.True(t, source3.Matches(sel5), "Test source3")

}
