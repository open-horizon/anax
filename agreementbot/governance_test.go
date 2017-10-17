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

// both check rates zero, choose pgi wait time
func Test_calc_skiptime0(t *testing.T) {
	lowestDVCR := uint64(0)
	lowestNHCR := uint64(0)
	pgi := uint64(10)

	expectedDVSkip := uint64(0)
	expectedNHSkip := uint64(0)
	expectedWaitTime := pgi

	dvSkip, nhSkip, waitTime := calculateSkipTime(lowestDVCR, lowestNHCR, pgi)

	if dvSkip != expectedDVSkip {
		t.Errorf("expected dvSkip %v, was %v", expectedDVSkip, dvSkip)
	} else if nhSkip != expectedNHSkip {
		t.Errorf("expected nhSkip %v, was %v", expectedNHSkip, nhSkip)
	} else if waitTime != expectedWaitTime {
		t.Errorf("expected waitTime %v, was %v", expectedWaitTime, waitTime)
	}

}

// one check rate zero, choose the other for the wait time
func Test_calc_skiptime1(t *testing.T) {
	lowestDVCR := uint64(0)
	lowestNHCR := uint64(30)
	pgi := uint64(10)

	expectedDVSkip := uint64(0)
	expectedNHSkip := uint64(0)
	expectedWaitTime := lowestNHCR

	dvSkip, nhSkip, waitTime := calculateSkipTime(lowestDVCR, lowestNHCR, pgi)

	if dvSkip != expectedDVSkip {
		t.Errorf("expected dvSkip %v, was %v", expectedDVSkip, dvSkip)
	} else if nhSkip != expectedNHSkip {
		t.Errorf("expected nhSkip %v, was %v", expectedNHSkip, nhSkip)
	} else if waitTime != expectedWaitTime {
		t.Errorf("expected waitTime %v, was %v", expectedWaitTime, waitTime)
	}

}

// the other check rate is zero, choose that for the wait time
func Test_calc_skiptime2(t *testing.T) {
	lowestDVCR := uint64(30)
	lowestNHCR := uint64(0)
	pgi := uint64(10)

	expectedDVSkip := uint64(0)
	expectedNHSkip := uint64(0)
	expectedWaitTime := lowestDVCR

	dvSkip, nhSkip, waitTime := calculateSkipTime(lowestDVCR, lowestNHCR, pgi)

	if dvSkip != expectedDVSkip {
		t.Errorf("expected dvSkip %v, was %v", expectedDVSkip, dvSkip)
	} else if nhSkip != expectedNHSkip {
		t.Errorf("expected nhSkip %v, was %v", expectedNHSkip, nhSkip)
	} else if waitTime != expectedWaitTime {
		t.Errorf("expected waitTime %v, was %v", expectedWaitTime, waitTime)
	}

}

// both check rates are non-zero, choose the shorter and no skips for the other
func Test_calc_skiptime3(t *testing.T) {
	lowestDVCR := uint64(30)
	lowestNHCR := uint64(20)
	pgi := uint64(10)

	expectedDVSkip := uint64(0)
	expectedNHSkip := uint64(0)
	expectedWaitTime := lowestNHCR

	dvSkip, nhSkip, waitTime := calculateSkipTime(lowestDVCR, lowestNHCR, pgi)

	if dvSkip != expectedDVSkip {
		t.Errorf("expected dvSkip %v, was %v", expectedDVSkip, dvSkip)
	} else if nhSkip != expectedNHSkip {
		t.Errorf("expected nhSkip %v, was %v", expectedNHSkip, nhSkip)
	} else if waitTime != expectedWaitTime {
		t.Errorf("expected waitTime %v, was %v", expectedWaitTime, waitTime)
	}

}

// reverse of previous test
func Test_calc_skiptime4(t *testing.T) {
	lowestDVCR := uint64(20)
	lowestNHCR := uint64(30)
	pgi := uint64(10)

	expectedDVSkip := uint64(0)
	expectedNHSkip := uint64(0)
	expectedWaitTime := lowestDVCR

	dvSkip, nhSkip, waitTime := calculateSkipTime(lowestDVCR, lowestNHCR, pgi)

	if dvSkip != expectedDVSkip {
		t.Errorf("expected dvSkip %v, was %v", expectedDVSkip, dvSkip)
	} else if nhSkip != expectedNHSkip {
		t.Errorf("expected nhSkip %v, was %v", expectedNHSkip, nhSkip)
	} else if waitTime != expectedWaitTime {
		t.Errorf("expected waitTime %v, was %v", expectedWaitTime, waitTime)
	}

}

// one check rate zero, choose half the other for the wait time
func Test_calc_skiptime5(t *testing.T) {
	lowestDVCR := uint64(0)
	lowestNHCR := uint64(100)
	pgi := uint64(10)

	expectedDVSkip := uint64(0)
	expectedNHSkip := uint64(0)
	expectedWaitTime := lowestNHCR / 2

	dvSkip, nhSkip, waitTime := calculateSkipTime(lowestDVCR, lowestNHCR, pgi)

	if dvSkip != expectedDVSkip {
		t.Errorf("expected dvSkip %v, was %v", expectedDVSkip, dvSkip)
	} else if nhSkip != expectedNHSkip {
		t.Errorf("expected nhSkip %v, was %v", expectedNHSkip, nhSkip)
	} else if waitTime != expectedWaitTime {
		t.Errorf("expected waitTime %v, was %v", expectedWaitTime, waitTime)
	}

}

// the other check rate is zero, choose half the former for the wait time
func Test_calc_skiptime6(t *testing.T) {
	lowestDVCR := uint64(100)
	lowestNHCR := uint64(0)
	pgi := uint64(10)

	expectedDVSkip := uint64(0)
	expectedNHSkip := uint64(0)
	expectedWaitTime := lowestDVCR / 2

	dvSkip, nhSkip, waitTime := calculateSkipTime(lowestDVCR, lowestNHCR, pgi)

	if dvSkip != expectedDVSkip {
		t.Errorf("expected dvSkip %v, was %v", expectedDVSkip, dvSkip)
	} else if nhSkip != expectedNHSkip {
		t.Errorf("expected nhSkip %v, was %v", expectedNHSkip, nhSkip)
	} else if waitTime != expectedWaitTime {
		t.Errorf("expected waitTime %v, was %v", expectedWaitTime, waitTime)
	}

}

// both check rates are non-zero, choose half the shorter and one skip for the other
func Test_calc_skiptime7(t *testing.T) {
	lowestDVCR := uint64(250)
	lowestNHCR := uint64(100)
	pgi := uint64(10)

	expectedDVSkip := uint64(1)
	expectedNHSkip := uint64(0)
	expectedWaitTime := lowestNHCR / 2

	dvSkip, nhSkip, waitTime := calculateSkipTime(lowestDVCR, lowestNHCR, pgi)

	if dvSkip != expectedDVSkip {
		t.Errorf("expected dvSkip %v, was %v", expectedDVSkip, dvSkip)
	} else if nhSkip != expectedNHSkip {
		t.Errorf("expected nhSkip %v, was %v", expectedNHSkip, nhSkip)
	} else if waitTime != expectedWaitTime {
		t.Errorf("expected waitTime %v, was %v", expectedWaitTime, waitTime)
	}

}

// reverse of previous test
func Test_calc_skiptime8(t *testing.T) {
	lowestDVCR := uint64(100)
	lowestNHCR := uint64(250)
	pgi := uint64(10)

	expectedDVSkip := uint64(0)
	expectedNHSkip := uint64(1)
	expectedWaitTime := lowestDVCR / 2

	dvSkip, nhSkip, waitTime := calculateSkipTime(lowestDVCR, lowestNHCR, pgi)

	if dvSkip != expectedDVSkip {
		t.Errorf("expected dvSkip %v, was %v", expectedDVSkip, dvSkip)
	} else if nhSkip != expectedNHSkip {
		t.Errorf("expected nhSkip %v, was %v", expectedNHSkip, nhSkip)
	} else if waitTime != expectedWaitTime {
		t.Errorf("expected waitTime %v, was %v", expectedWaitTime, waitTime)
	}

}

// exact multiples, short
func Test_calc_skiptime9(t *testing.T) {
	lowestDVCR := uint64(15)
	lowestNHCR := uint64(30)
	pgi := uint64(10)

	expectedDVSkip := uint64(0)
	expectedNHSkip := uint64(1)
	expectedWaitTime := lowestDVCR

	dvSkip, nhSkip, waitTime := calculateSkipTime(lowestDVCR, lowestNHCR, pgi)

	if dvSkip != expectedDVSkip {
		t.Errorf("expected dvSkip %v, was %v", expectedDVSkip, dvSkip)
	} else if nhSkip != expectedNHSkip {
		t.Errorf("expected nhSkip %v, was %v", expectedNHSkip, nhSkip)
	} else if waitTime != expectedWaitTime {
		t.Errorf("expected waitTime %v, was %v", expectedWaitTime, waitTime)
	}

}

// exact multiples, long
func Test_calc_skiptime10(t *testing.T) {
	lowestDVCR := uint64(120)
	lowestNHCR := uint64(60)
	pgi := uint64(10)

	expectedDVSkip := uint64(1)
	expectedNHSkip := uint64(0)
	expectedWaitTime := lowestDVCR / 2

	dvSkip, nhSkip, waitTime := calculateSkipTime(lowestDVCR, lowestNHCR, pgi)

	if dvSkip != expectedDVSkip {
		t.Errorf("expected dvSkip %v, was %v", expectedDVSkip, dvSkip)
	} else if nhSkip != expectedNHSkip {
		t.Errorf("expected nhSkip %v, was %v", expectedNHSkip, nhSkip)
	} else if waitTime != expectedWaitTime {
		t.Errorf("expected waitTime %v, was %v", expectedWaitTime, waitTime)
	}

}
