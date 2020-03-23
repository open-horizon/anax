// +build unit

package persistence

import (
	"encoding/json"
	"flag"
	"testing"
)

func init() {
	// Enable glog tracing in the tested functions. The output will be displayed when -v is
	// passed on the go test command.
	flag.Set("alsologtostderr", "true")
	flag.Set("v", "7")
	// no need to parse flags, that's done by test framework
}

func Test_DecodeHelmDeployment(t *testing.T) {

	hd := NewHelmDeployment("11223344", "test-release")

	hdBytes, err := json.Marshal(hd)
	if err != nil {
		t.Errorf("Error marshalling %v, error: %v", hd, err)
	}

	newHD, ghdErr := GetHelmDeployment(string(hdBytes))
	if ghdErr != nil {
		t.Errorf("Error extracting helm deployment %v, error: %v", string(hdBytes), err)
	} else if newHD.ChartArchive != hd.ChartArchive || newHD.ReleaseName != hd.ReleaseName {
		t.Errorf("Extracted helm deployment %v does not match original %v", newHD, hd)
	}

}

func Test_DecodeNonHelmDeployment(t *testing.T) {

	fake := `{"test":"nope"}`
	newHD, ghdErr := GetHelmDeployment(fake)
	if ghdErr == nil {
		t.Errorf("Should be an error returned for %v", fake)
	} else if newHD != nil {
		t.Errorf("Should not return an object %v", newHD)
	}

}

func Test_PersistToFrom(t *testing.T) {

	hd := HelmDeploymentConfig{
		ChartArchive: "1234567890",
		ReleaseName:  "test",
	}

	// Convert the object to persistent form.
	if pf, err := hd.ToPersistentForm(); err != nil {
		t.Errorf("unexpected error changing to persistent form: %v", err)
	} else if pf == nil || len(pf) != 2 {
		t.Errorf("persistent form not as expected, is: %v", pf)
	} else {

		// Now change it back to non-persistent form.
		nhd := HelmDeploymentConfig{}
		if err := nhd.FromPersistentForm(pf); err != nil {
			t.Errorf("unexpected error changing from persistent form: %v", err)
		} else if (hd.ChartArchive != nhd.ChartArchive) || (hd.ReleaseName != nhd.ReleaseName) {
			t.Errorf("object from persistent form: %v doesnt match original: %v", nhd, hd)
		}

	}

}
