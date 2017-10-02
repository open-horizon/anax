// +build unit

package policy

import (
	"testing"
)

func Test_hagroup_factory(t *testing.T) {

	partners1 := []string{}

	if hag := HAGroup_Factory(partners1); hag == nil {
		t.Errorf("Factory returned nil, should not.")
	} else if len(hag.Partners) != len(partners1) {
		t.Errorf("Partner list has wrong length, should be %v", len(partners1))
	} else if str := hag.String(); len(str) == 0 {
		t.Errorf("no formatted output")
	}

}

func Test_hagroup_issame1(t *testing.T) {

	partners1 := []string{}
	partners2 := []string{}

	if hag1 := HAGroup_Factory(partners1); hag1 == nil {
		t.Errorf("Factory returned nil, should not.")
	} else if hag2 := HAGroup_Factory(partners2); hag2 == nil {
		t.Errorf("Factory returned nil, should not.")
	} else if !hag1.IsSame(hag2) {
		t.Errorf("HA Groups are not the same, should be %v and %v", hag1, hag2)
	}

}

func Test_hagroup_issame2(t *testing.T) {

	partners1 := []string{"a", "b"}
	partners2 := []string{"b", "a"}

	if hag1 := HAGroup_Factory(partners1); hag1 == nil {
		t.Errorf("Factory returned nil, should not.")
	} else if hag2 := HAGroup_Factory(partners2); hag2 == nil {
		t.Errorf("Factory returned nil, should not.")
	} else if !hag1.IsSame(hag2) {
		t.Errorf("HA Groups are not the same, should be %v and %v", hag1, hag2)
	}

}

func Test_hagroup_isNOTsame1(t *testing.T) {

	partners1 := []string{}
	partners2 := []string{"a"}

	if hag1 := HAGroup_Factory(partners1); hag1 == nil {
		t.Errorf("Factory returned nil, should not.")
	} else if hag2 := HAGroup_Factory(partners2); hag2 == nil {
		t.Errorf("Factory returned nil, should not.")
	} else if hag1.IsSame(hag2) {
		t.Errorf("HA Groups are the same, should not be %v and %v", hag1, hag2)
	}

}

func Test_hagroup_isNOTsame2(t *testing.T) {

	partners1 := []string{"b"}
	partners2 := []string{"a"}

	if hag1 := HAGroup_Factory(partners1); hag1 == nil {
		t.Errorf("Factory returned nil, should not.")
	} else if hag2 := HAGroup_Factory(partners2); hag2 == nil {
		t.Errorf("Factory returned nil, should not.")
	} else if hag1.IsSame(hag2) {
		t.Errorf("HA Groups are the same, should not be %v and %v", hag1, hag2)
	}

}

func Test_hagroup_isNOTsame3(t *testing.T) {

	partners1 := []string{"a", "b"}
	partners2 := []string{"a", "c"}

	if hag1 := HAGroup_Factory(partners1); hag1 == nil {
		t.Errorf("Factory returned nil, should not.")
	} else if hag2 := HAGroup_Factory(partners2); hag2 == nil {
		t.Errorf("Factory returned nil, should not.")
	} else if hag1.IsSame(hag2) {
		t.Errorf("HA Groups are the same, should not be %v and %v", hag1, hag2)
	}

}

func Test_hagroup_compatible(t *testing.T) {

	partners1 := []string{"a", "b"}
	partners2 := []string{"b", "a"}

	if hag1 := HAGroup_Factory(partners1); hag1 == nil {
		t.Errorf("Factory returned nil, should not.")
	} else if hag2 := HAGroup_Factory(partners2); hag2 == nil {
		t.Errorf("Factory returned nil, should not.")
	} else if !hag1.Compatible_With(hag2) {
		t.Errorf("HA Groups are not compatible, should be %v and %v", hag1, hag2)
	}

}

func Test_hagroup_NOTcompatible(t *testing.T) {

	partners1 := []string{"a", "b"}
	partners2 := []string{"a", "c"}

	if hag1 := HAGroup_Factory(partners1); hag1 == nil {
		t.Errorf("Factory returned nil, should not.")
	} else if hag2 := HAGroup_Factory(partners2); hag2 == nil {
		t.Errorf("Factory returned nil, should not.")
	} else if hag1.Compatible_With(hag2) {
		t.Errorf("HA Groups are compatible, should not be %v and %v", hag1, hag2)
	}

}

func Test_hagroup_merge(t *testing.T) {

	partners1 := []string{"a", "b"}
	partners2 := []string{"a", "b"}

	if hag1 := HAGroup_Factory(partners1); hag1 == nil {
		t.Errorf("Factory returned nil, should not.")
	} else if hag2 := HAGroup_Factory(partners2); hag2 == nil {
		t.Errorf("Factory returned nil, should not.")
	} else if merged := hag1.Merge(hag2); merged == nil {
		t.Errorf("Merged HAGroups nil, should not be")
	} else if len(merged.Partners) != len(hag2.Partners) {
		t.Errorf("Merged HAGroup partners have different lengths, %v and %v", len(merged.Partners), len(hag2.Partners))
	}

}
