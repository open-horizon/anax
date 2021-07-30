package agreementbot

import (
	"flag"
	"testing"
)

func init() {
	flag.Set("alsologtostderr", "true")
	flag.Set("v", "7")
	// no need to parse flags, that's done by test framework
}

// check rate zero
func Test_calc_skiptime0(t *testing.T) {
	lowestNHCR := uint64(0)
	pgi := uint64(10)

	expectedNHSkip := uint64(0)

	nhSkip := calculateSkipTime(lowestNHCR, pgi)

	if nhSkip != expectedNHSkip {
		t.Errorf("expected nhSkip %v, was %v", expectedNHSkip, nhSkip)
	}

}

// check rate non-zero
func Test_calc_skiptime1(t *testing.T) {
	lowestNHCR := uint64(30)
	pgi := uint64(10)

	expectedNHSkip := uint64(2)

	nhSkip := calculateSkipTime(lowestNHCR, pgi)

	if nhSkip != expectedNHSkip {
		t.Errorf("expected nhSkip %v, was %v", expectedNHSkip, nhSkip)
	}

}

// large checkrate
func Test_calc_skiptime2(t *testing.T) {
	lowestNHCR := uint64(100)
	pgi := uint64(10)

	expectedNHSkip := uint64(4)

	nhSkip := calculateSkipTime(lowestNHCR, pgi)

	if nhSkip != expectedNHSkip {
		t.Errorf("expected nhSkip %v, was %v", expectedNHSkip, nhSkip)
	}

}

// longer pgi
func Test_calc_skiptime3(t *testing.T) {
	lowestNHCR := uint64(35)
	pgi := uint64(40)

	expectedNHSkip := uint64(0)

	nhSkip := calculateSkipTime(lowestNHCR, pgi)

	if nhSkip != expectedNHSkip {
		t.Errorf("expected nhSkip %v, was %v", expectedNHSkip, nhSkip)
	}

}

// odd multiple
func Test_calc_skiptime4(t *testing.T) {
	lowestNHCR := uint64(35)
	pgi := uint64(20)

	expectedNHSkip := uint64(0)

	nhSkip := calculateSkipTime(lowestNHCR, pgi)

	if nhSkip != expectedNHSkip {
		t.Errorf("expected nhSkip %v, was %v", expectedNHSkip, nhSkip)
	}

}
