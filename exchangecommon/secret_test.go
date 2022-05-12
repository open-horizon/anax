//go:build unit
// +build unit

package exchangecommon

import (
	"testing"
)

func Test_SecretBindingCompare(t *testing.T) {
	sb1 := SecretBinding{
		ServiceOrgid:        "mycomp1",
		ServiceUrl:          "cpu",
		ServiceArch:         "amd64",
		ServiceVersionRange: "[2.0.1,INFINITY)",
		Secrets:             []BoundSecret{BoundSecret{"sec1": "secmSec1"}, BoundSecret{"sec2": "secmSec2"}},
	}

	sb2 := SecretBinding{
		ServiceOrgid:        "mycomp2",
		ServiceUrl:          "cpu",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
		Secrets:             []BoundSecret{BoundSecret{"sec2": "secmSec3"}, BoundSecret{"sec1": "secmSec4"}},
	}

	sbArray1 := []SecretBinding{sb1, sb2}
	sbArray2 := []SecretBinding{sb2, sb1}
	same := SecretBindingIsSame(sbArray1, sbArray2)
	if !same {
		t.Errorf("SecretBindingIsSame should have returned true but got false.")
	}

	sbArray1 = []SecretBinding{sb1}
	sbArray2 = []SecretBinding{sb2}
	same = SecretBindingIsSame(sbArray1, sbArray2)
	if same {
		t.Errorf("SecretBindingIsSame should have returned false but got true.")
	}

	sbArray1 = []SecretBinding{}
	sbArray2 = []SecretBinding{sb2}
	same = SecretBindingIsSame(sbArray1, sbArray2)
	if same {
		t.Errorf("SecretBindingIsSame should have returned false but got true.")
	}

	sbArray1 = []SecretBinding{}
	sbArray2 = []SecretBinding{}
	same = SecretBindingIsSame(sbArray1, sbArray2)
	if !same {
		t.Errorf("SecretBindingIsSame should have returned true but got false.")
	}

	sbArray1 = nil
	sbArray2 = []SecretBinding{}
	same = SecretBindingIsSame(sbArray1, sbArray2)
	if !same {
		t.Errorf("SecretBindingIsSame should have returned true but got false.")
	}

	sbArray1 = []SecretBinding{sb2}
	sbArray2 = nil
	same = SecretBindingIsSame(sbArray1, sbArray2)
	if same {
		t.Errorf("SecretBindingIsSame should have returned false but got true.")
	}

	sbArray1 = nil
	sbArray2 = nil
	same = SecretBindingIsSame(sbArray1, sbArray2)
	if !same {
		t.Errorf("SecretBindingIsSame should have returned true but got false.")
	}

	// changed the service ServiceOrgid
	sb3 := SecretBinding{
		ServiceOrgid:        "mycomp1",
		ServiceUrl:          "cpu",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
		Secrets:             []BoundSecret{BoundSecret{"sec1": "secmSec1"}, BoundSecret{"sec2": "secmSec2"}},
	}
	sbArray1 = []SecretBinding{sb1, sb2}
	sbArray2 = []SecretBinding{sb1, sb3}
	same = SecretBindingIsSame(sbArray1, sbArray2)
	if same {
		t.Errorf("SecretBindingIsSame should have returned false but got true.")
	}

	sb3 = SecretBinding{
		ServiceOrgid:        "mycomp1",
		ServiceUrl:          "cpu",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
		Secrets:             []BoundSecret{},
	}
	sbArray1 = []SecretBinding{sb2}
	sbArray2 = []SecretBinding{sb3}
	same = SecretBindingIsSame(sbArray1, sbArray2)
	if same {
		t.Errorf("SecretBindingIsSame should have returned false but got true.")
	}

	sb2 = SecretBinding{
		ServiceOrgid:        "mycomp1",
		ServiceUrl:          "cpu",
		ServiceArch:         "amd64",
		ServiceVersionRange: "",
		Secrets:             nil,
	}
	sbArray1 = []SecretBinding{sb2}
	sbArray2 = []SecretBinding{sb3}
	same = SecretBindingIsSame(sbArray1, sbArray2)
	if !same {
		t.Errorf("SecretBindingIsSame should have returned true but got false.")
	}

	sbArray1 = []SecretBinding{sb2}
	sbArray2 = []SecretBinding{sb2}
	same = SecretBindingIsSame(sbArray1, sbArray2)
	if !same {
		t.Errorf("SecretBindingIsSame should have returned true but got false.")
	}
}
