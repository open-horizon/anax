// +build unit

package agreementbot

import (
	"flag"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/edge-sync-service/common"
	"testing"
)

func init() {
	flag.Set("alsologtostderr", "true")
	flag.Set("v", "7")
	// no need to parse flags, that's done by test framework
}

func Test_object_policy_entry_success1(t *testing.T) {

	p := &exchange.ObjectDestinationPolicy{
		OrgID:      "org1",
		ObjectType: "type1",
		ObjectID:   "obj1",
		DestinationPolicy: exchange.DestinationPolicy{
			Properties:  externalpolicy.PropertyList{},
			Constraints: externalpolicy.ConstraintExpression{},
			Services:    []common.ServiceID{},
			Timestamp:   1,
		},
	}

	if pe := NewMMSObjectPolicyEntry(p); pe == nil {
		t.Errorf("Error creating new MMSObjectPolicyEntry")
	} else if pe.Policy.OrgID != "org1" {
		t.Errorf("Error: OrgID should be %v but is %v", "org1", pe.Policy.OrgID)
	} else {
		t.Log(pe)
	}

}

// No existing served patterns, no new served patterns
func Test_object_manager_setorgs0(t *testing.T) {

	servedOrgs := map[string]exchange.ServedBusinessPolicy{}

	if op := NewMMSObjectPolicyManager(); op == nil {
		t.Errorf("Error: object manager not created")
	} else if err := op.SetCurrentPolicyOrgs(servedOrgs); err != nil {
		t.Errorf("Error %v consuming served orgs %v", err, servedOrgs)
	} else if len(op.orgMap) != 0 {
		t.Errorf("Error: should have 0 orgs in the Object Manager, but have %v", len(op.orgMap))
	} else {
		t.Log(op)
	}

}

// Add a new served org and pattern
func Test_object_manager_setorgs1(t *testing.T) {

	servedOrgs := map[string]exchange.ServedBusinessPolicy{
		"org1_bp": {
			BusinessPolOrg: "org1",
			BusinessPol:    "bp1",
			NodeOrg:        "",
		},
	}

	if op := NewMMSObjectPolicyManager(); op == nil {
		t.Errorf("Error: object manager not created")
	} else if err := op.SetCurrentPolicyOrgs(servedOrgs); err != nil {
		t.Errorf("Error %v consuming served orgs %v", err, servedOrgs)
	} else if len(op.orgMap) != 1 {
		t.Errorf("Error: should have 1 org in the Object Manager, have %v", len(op.orgMap))
	} else {
		t.Log(op)
	}

}

// Remove an org, replace with a new org
func Test_object_manager_setorgs2(t *testing.T) {

	myorg1 := "myorg1"
	myorg2 := "myorg2"
	myorg3 := "myorg3"
	bp1 := "bp1"
	bp2 := "bp2"

	servedOrgs1 := map[string]exchange.ServedBusinessPolicy{
		"myorg1_bp1": {
			BusinessPolOrg: myorg1,
			BusinessPol:    bp1,
			NodeOrg:        myorg1,
		},
	}

	servedOrgs2 := map[string]exchange.ServedBusinessPolicy{
		"myorg2_bp2": {
			BusinessPolOrg: myorg2,
			BusinessPol:    bp2,
			NodeOrg:        myorg2,
		},
	}

	sid1 := common.ServiceID{
		OrgID:       myorg3,
		Arch:        "amd64",
		ServiceName: "myservice.com",
		Version:     "1.0.0",
	}

	p1 := exchange.ObjectDestinationPolicy{
		OrgID:      myorg1,
		ObjectType: "type1",
		ObjectID:   "obj1",
		DestinationPolicy: exchange.DestinationPolicy{
			Properties:  externalpolicy.PropertyList{},
			Constraints: externalpolicy.ConstraintExpression{},
			Services:    []common.ServiceID{sid1},
			Timestamp:   1,
		},
	}

	p1a := exchange.ObjectDestinationPolicy{
		OrgID:      myorg1,
		ObjectType: "type2",
		ObjectID:   "obj2",
		DestinationPolicy: exchange.DestinationPolicy{
			Properties:  externalpolicy.PropertyList{},
			Constraints: externalpolicy.ConstraintExpression{},
			Services:    []common.ServiceID{sid1},
			Timestamp:   1,
		},
	}

	objPolicies1 := &exchange.ObjectDestinationPolicies{p1, p1a}

	sid2 := common.ServiceID{
		OrgID:       myorg3,
		Arch:        "amd64",
		ServiceName: "otherservice.com",
		Version:     "1.0.0",
	}

	p2 := exchange.ObjectDestinationPolicy{
		OrgID:      myorg2,
		ObjectType: "type1",
		ObjectID:   "obj1",
		DestinationPolicy: exchange.DestinationPolicy{
			Properties:  externalpolicy.PropertyList{},
			Constraints: externalpolicy.ConstraintExpression{},
			Services:    []common.ServiceID{sid2},
			Timestamp:   1,
		},
	}

	objPolicies2 := &exchange.ObjectDestinationPolicies{p2}

	// run test
	if op := NewMMSObjectPolicyManager(); op == nil {
		t.Errorf("Error: object manager not created")
	} else if err := op.SetCurrentPolicyOrgs(servedOrgs1); err != nil {
		t.Errorf("Error %v consuming served orgs %v", err, servedOrgs1)
	} else if events, err := op.UpdatePolicies(myorg1, objPolicies1, getDummyPolicyReceivedHandler()); err != nil {
		t.Errorf("Error: error updating object policies, %v", err)
	} else if len(op.orgMap) != 1 {
		t.Errorf("Error: should have 1 org in the Object Manager, have %v", len(op.orgMap))
	} else if !op.hasOrg(myorg1) {
		t.Errorf("Error: OM should have org %v but doesnt, has %v", myorg1, op)
	} else if len(events) != 2 {
		t.Errorf("Error: should have 2 events from the Object Manager, have %v", events)
	} else if objPols := op.GetObjectPolicies(myorg1, cutil.FormOrgSpecUrl(sid1.ServiceName, sid1.OrgID)); objPols == nil {
		t.Errorf("Error getting object policy for %v %v", myorg1, sid1)
	} else if len(*objPols) != 2 {
		t.Errorf("Error wrong number of policies returned, expecting 2, was %v", len(*objPols))
	} else if err := op.SetCurrentPolicyOrgs(servedOrgs2); err != nil {
		t.Errorf("Error %v consuming served orgs %v", err, servedOrgs2)
	} else if events, err := op.UpdatePolicies(myorg2, objPolicies2, getDummyPolicyReceivedHandler()); err != nil {
		t.Errorf("Error: error updating object policies, %v", err)
	} else if len(op.orgMap) != 1 {
		t.Errorf("Error: should have 1 org in the Object Manager, have %v", len(op.orgMap))
	} else if len(events) != 1 {
		t.Errorf("Error: should have 1 event from the Object Manager, have %v", events)
	} else if !op.hasOrg(myorg2) {
		t.Errorf("Error: OM should have org %v but doesnt, has %v", myorg2, op)
	} else if op.hasOrg(myorg1) {
		t.Errorf("Error: OM should NOT have org %v but does %v", myorg1, op)
	} else if objPols := op.GetObjectPolicies(myorg2, cutil.FormOrgSpecUrl(sid2.ServiceName, sid2.OrgID)); objPols == nil {
		t.Errorf("Error getting object policy for %v %v", myorg2, sid2)
	} else if len(*objPols) != 1 {
		t.Errorf("Error wrong number of policies returned, expecting 1, was %v", len(*objPols))
	} else if objPols := op.GetObjectPolicies(myorg1, cutil.FormOrgSpecUrl(sid1.ServiceName, sid1.OrgID)); objPols != nil && len(*objPols) != 0 {
		t.Errorf("Error should not be able to get object policy for %v %v %v", myorg1, sid1, objPols)
	} else {
		t.Log(op)
	}

}

// Replace an object policy
func Test_object_manager_replace1(t *testing.T) {

	myorg1 := "myorg1"
	myorg3 := "myorg3"
	bp1 := "bp1"

	servedOrgs1 := map[string]exchange.ServedBusinessPolicy{
		"myorg1_bp1": {
			BusinessPolOrg: myorg1,
			BusinessPol:    bp1,
			NodeOrg:        myorg1,
		},
	}

	sid1 := common.ServiceID{
		OrgID:       myorg3,
		Arch:        "amd64",
		ServiceName: "myservice.com",
		Version:     "1.0.0",
	}

	p1 := exchange.ObjectDestinationPolicy{
		OrgID:      myorg1,
		ObjectType: "type1",
		ObjectID:   "obj1",
		DestinationPolicy: exchange.DestinationPolicy{
			Properties:  externalpolicy.PropertyList{},
			Constraints: externalpolicy.ConstraintExpression{},
			Services:    []common.ServiceID{sid1},
			Timestamp:   1,
		},
	}

	p2 := exchange.ObjectDestinationPolicy{
		OrgID:      myorg1,
		ObjectType: "type1",
		ObjectID:   "obj1",
		DestinationPolicy: exchange.DestinationPolicy{
			Properties: externalpolicy.PropertyList{
				externalpolicy.Property{
					Name:  "test",
					Value: "testvalue",
				},
			},
			Constraints: externalpolicy.ConstraintExpression{},
			Services:    []common.ServiceID{sid1},
			Timestamp:   2,
		},
	}

	objPolicies1 := &exchange.ObjectDestinationPolicies{p1}
	objPolicies2 := &exchange.ObjectDestinationPolicies{p2}

	// run test
	if op := NewMMSObjectPolicyManager(); op == nil {
		t.Errorf("Error: object manager not created")
	} else if err := op.SetCurrentPolicyOrgs(servedOrgs1); err != nil {
		t.Errorf("Error %v consuming served orgs %v", err, servedOrgs1)
	} else if events, err := op.UpdatePolicies(myorg1, objPolicies1, getDummyPolicyReceivedHandler()); err != nil {
		t.Errorf("Error: error updating object policies, %v", err)
	} else if len(op.orgMap) != 1 {
		t.Errorf("Error: should have 1 org in the Object Manager, have %v", len(op.orgMap))
	} else if len(events) != 1 {
		t.Errorf("Error: should have 1 event from the Object Manager, have %v", len(events))
	} else if !op.hasOrg(myorg1) {
		t.Errorf("Error: OM should have org %v but doesnt, has %v", myorg1, op)
	} else if objPols := op.GetObjectPolicies(myorg1, cutil.FormOrgSpecUrl(sid1.ServiceName, sid1.OrgID)); objPols == nil {
		t.Errorf("Error getting object policy for %v %v", myorg1, sid1)
	} else if len(*objPols) != 1 {
		t.Errorf("Error wrong number of policies returned, expecting 1, was %v", len(*objPols))
	} else if len((*objPols)[0].DestinationPolicy.Properties) != 0 {
		t.Errorf("Error should not be any properties in the policy, have %v", (*objPols)[0].DestinationPolicy.Properties)
	} else if events, err := op.UpdatePolicies(myorg1, objPolicies2, getDummyPolicyReceivedHandler()); err != nil {
		t.Errorf("Error: error updating object policies, %v", err)
	} else if len(op.orgMap) != 1 {
		t.Errorf("Error: should have 1 org in the Object Manager, have %v", len(op.orgMap))
	} else if !op.hasOrg(myorg1) {
		t.Errorf("Error: OM should have org %v but doesnt, has %v", myorg1, op)
	} else if len(events) != 1 {
		t.Errorf("Error: should have 1 event from the Object Manager, have %v", len(events))
	} else if objPols := op.GetObjectPolicies(myorg1, cutil.FormOrgSpecUrl(sid1.ServiceName, sid1.OrgID)); objPols == nil {
		t.Errorf("Error getting object policy for %v %v", myorg1, sid1)
	} else if len(*objPols) != 1 {
		t.Errorf("Error wrong number of policies returned, expecting 1, was %v", len(*objPols))
	} else if len((*objPols)[0].DestinationPolicy.Properties) == 0 {
		t.Errorf("Error should have properties in the policy now, have %v", (*objPols)[0].DestinationPolicy.Properties)
	} else {
		t.Log(op)
	}

}

func getDummyPolicyReceivedHandler() exchange.ObjectPolicyUpdateReceivedHandler {
	return func(objPol *exchange.ObjectDestinationPolicy) error {
		return nil
	}
}
