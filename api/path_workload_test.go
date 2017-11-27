// +build unit

package api

import (
	"flag"
	"testing"
)

func init() {
	flag.Set("alsologtostderr", "true")
	flag.Set("v", "7")
	// no need to parse flags, that's done by test framework
}

func Test_FindWorkloadForOutput0(t *testing.T) {

	dir, db, err := utsetup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	// No workloads yet.

	// Now test the GET /agreements function.
	if wcsout, err := FindWorkloadForOutput(db, getBasicConfig()); err != nil {
		t.Errorf("error finding workloads and configs: %v", err)
	} else if wcsout == nil {
		t.Errorf("expecting an output object")
	} else if len(wcsout.Config) != 0 {
		t.Errorf("expecting 0 active workloadconfigs have %v", wcsout.Config)
	} else if len(*wcsout.Containers) != 0 {
		t.Errorf("expecting 0 containers have %v", *wcsout.Containers)
	}
}
