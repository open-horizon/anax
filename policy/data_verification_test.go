//go:build unit
// +build unit

package policy

import (
	"encoding/json"
	"testing"
)

func Test_data_verification_section_success(t *testing.T) {

	dv1 := `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":50,"check_rate":10}`
	dv2 := `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":50,"check_rate":10}`
	if dva := create_DataVerification(dv1, t); dva != nil {
		if dvb := create_DataVerification(dv2, t); dvb != nil {
			if !dva.IsSame(*dvb) {
				t.Errorf("DV section %v is the same as %v\n", dva, dvb)
			}
		}
	}

	dv1 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"my2secret","interval":0,"check_rate":10}`
	dv2 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":0,"check_rate":10}`
	if dva := create_DataVerification(dv1, t); dva != nil {
		if dvb := create_DataVerification(dv2, t); dvb != nil {
			if !dva.IsSame(*dvb) {
				t.Errorf("DV section %v is the same as %v\n", dva, dvb)
			}
		}
	}

}

func Test_data_verification_section_failure(t *testing.T) {

	dv1 := `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":0,"check_rate":10}`
	dv2 := `{"enabled":true,"URL":"http://othercompany.com/verify","URLUser":"me","URLPassword":"mysecret","interval":0,"check_rate":10}`
	if dva := create_DataVerification(dv1, t); dva != nil {
		if dvb := create_DataVerification(dv2, t); dvb != nil {
			if dva.IsSame(*dvb) {
				t.Errorf("DV section %v is not the same as %v\n", dva, dvb)
			}
		}
	}

	dv1 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me2","URLPassword":"mysecret","interval":0,"check_rate":10}`
	dv2 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":0,"check_rate":10}`
	if dva := create_DataVerification(dv1, t); dva != nil {
		if dvb := create_DataVerification(dv2, t); dvb != nil {
			if dva.IsSame(*dvb) {
				t.Errorf("DV section %v is not the same as %v\n", dva, dvb)
			}
		}
	}

	dv1 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":50,"check_rate":10}`
	dv2 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":0,"check_rate":10}`
	if dva := create_DataVerification(dv1, t); dva != nil {
		if dvb := create_DataVerification(dv2, t); dvb != nil {
			if dva.IsSame(*dvb) {
				t.Errorf("DV section %v is not the same as %v\n", dva, dvb)
			}
		}
	}

	dv1 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":50,"check_rate":5}`
	dv2 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":50,"check_rate":10}`
	if dva := create_DataVerification(dv1, t); dva != nil {
		if dvb := create_DataVerification(dv2, t); dvb != nil {
			if dva.IsSame(*dvb) {
				t.Errorf("DV section %v is not the same as %v\n", dva, dvb)
			}
		}
	}

}

func Test_data_verification_obscure(t *testing.T) {

	dv1 := `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":0}`
	dv2 := `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":0}`
	if dva := create_DataVerification(dv1, t); dva != nil {
		upb4 := dva.URLPassword
		dva.Obscure()
		if dva.URLPassword == upb4 || dva.URLPassword != "********" {
			t.Errorf("DV section %v was not obscured correctly\n", dva)
		} else if dvb := create_DataVerification(dv2, t); dvb != nil {
			if !dva.IsSame(*dvb) {
				t.Errorf("DV section %v is not the same as %v\n", dva, dvb)
			}
		}
	}

}

func Test_dv_compatwith(t *testing.T) {

	dv1 := `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":0}`
	dv2 := `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":0}`
	if dva := create_DataVerification(dv1, t); dva != nil {
		if dvb := create_DataVerification(dv2, t); dvb != nil {
			if !dva.IsCompatibleWith(*dvb) {
				t.Errorf("DV section %v is compatible with %v\n", dva, dvb)
			}
		}
	}

	dv1 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":0}`
	dv2 = `{"enabled":true,"URL":"http://company.com/verify2","URLUser":"me","URLPassword":"mysecret","interval":0}`
	if dva := create_DataVerification(dv1, t); dva != nil {
		if dvb := create_DataVerification(dv2, t); dvb != nil {
			if dva.IsCompatibleWith(*dvb) {
				t.Errorf("DV section %v is not compatible with %v\n", dva, dvb)
			}
		}
	}

	dv1 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":0}`
	dv2 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me2","URLPassword":"mysecret","interval":0}`
	if dva := create_DataVerification(dv1, t); dva != nil {
		if dvb := create_DataVerification(dv2, t); dvb != nil {
			if dva.IsCompatibleWith(*dvb) {
				t.Errorf("DV section %v is not compatible with %v\n", dva, dvb)
			}
		}
	}

	dv1 = `{"enabled":false,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":0}`
	dv2 = `{"enabled":false,"URL":"http://company.com/verify2","URLUser":"me2","URLPassword":"mysecret","interval":0}`
	if dva := create_DataVerification(dv1, t); dva != nil {
		if dvb := create_DataVerification(dv2, t); dvb != nil {
			if !dva.IsCompatibleWith(*dvb) {
				t.Errorf("DV section %v is compatible with %v\n", dva, dvb)
			}
		}
	}

	dv1 = `{"enabled":false,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":0,"metering":{"tokens":3,"per_time_unit":"min"}}`
	dv2 = `{"enabled":false,"URL":"http://company.com/verify2","URLUser":"me2","URLPassword":"mysecret","interval":0,"metering":{"tokens":4,"per_time_unit":"min"}}`
	if dva := create_DataVerification(dv1, t); dva != nil {
		if dvb := create_DataVerification(dv2, t); dvb != nil {
			if !dva.IsCompatibleWith(*dvb) {
				t.Errorf("DV section %v is compatible with %v\n", dva, dvb)
			}
		}
	}

	dv1 = `{"enabled":true,"URL":"","URLUser":"me","URLPassword":"mysecret","interval":0,"metering":{"tokens":3,"per_time_unit":"min"}}`
	dv2 = `{"enabled":true,"URL":"http://company.com/verify2","URLUser":"","URLPassword":"mysecret","interval":0,"metering":{"tokens":4,"per_time_unit":"min"}}`
	if dva := create_DataVerification(dv1, t); dva != nil {
		if dvb := create_DataVerification(dv2, t); dvb != nil {
			if !dva.IsCompatibleWith(*dvb) {
				t.Errorf("DV section %v is compatible with %v\n", dva, dvb)
			}
		}
	}

	dv1 = `{"enabled":true,"URL":"","URLUser":"me","URLPassword":"mysecret","interval":0,"metering":{"tokens":3,"per_time_unit":"min"}}`
	dv2 = `{"enabled":false,"URL":"http://company.com/verify2","URLUser":"","URLPassword":"mysecret","interval":0,"metering":{"tokens":4,"per_time_unit":"min"}}`
	if dva := create_DataVerification(dv1, t); dva != nil {
		if dvb := create_DataVerification(dv2, t); dvb != nil {
			if !dva.IsCompatibleWith(*dvb) {
				t.Errorf("DV section %v is compatible with %v\n", dva, dvb)
			}
		}
	}

}

func Test_dv_mergewith(t *testing.T) {

	dv1 := `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":30,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":25}}`
	dv2 := `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":30,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":25}}`
	dv3 := `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"********","interval":30,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":25}}`

	if dva := create_DataVerification(dv1, t); dva != nil {
		if dvb := create_DataVerification(dv2, t); dvb != nil {
			if dvc := create_DataVerification(dv3, t); dvc != nil {
				if dvm := dva.MergeWith(*dvb, 60); !dvm.IsSame(*dvc) {
					t.Errorf("Merged DV section %v should be the same as %v\n", dvm, dvc)
				}
			}
		}
	}

	dv1 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":30,"check_rate":10,"metering":{"tokens":3,"per_time_unit":"min"}}`
	dv2 = `{"enabled":false,"URL":"http://company.com/verify2","URLUser":"me2","URLPassword":"mysecret","interval":40,"metering":{"tokens":4,"per_time_unit":"min"}}`
	dv3 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"********","interval":30,"check_rate":10,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":10}}`

	if dva := create_DataVerification(dv1, t); dva != nil {
		if dvb := create_DataVerification(dv2, t); dvb != nil {
			if dvc := create_DataVerification(dv3, t); dvc != nil {
				if dvm := dva.MergeWith(*dvb, 60); !dvm.IsSame(*dvc) {
					t.Errorf("Merged DV section %v should be the same as %v\n", dvm, dvc)
				}
			}
		}
	}

	dv1 = `{"enabled":false,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":30,"metering":{"tokens":3,"per_time_unit":"min"}}`
	dv2 = `{"enabled":true,"URL":"http://company.com/verify2","URLUser":"me2","URLPassword":"mysecret","interval":40,"check_rate":10,"metering":{"tokens":4,"per_time_unit":"min"}}`
	dv3 = `{"enabled":true,"URL":"http://company.com/verify2","URLUser":"me2","URLPassword":"********","interval":40,"check_rate":10,"metering":{"tokens":4,"per_time_unit":"min","notification_interval":10}}`

	if dva := create_DataVerification(dv1, t); dva != nil {
		if dvb := create_DataVerification(dv2, t); dvb != nil {
			if dvc := create_DataVerification(dv3, t); dvc != nil {
				if dvm := dva.MergeWith(*dvb, 60); !dvm.IsSame(*dvc) {
					t.Errorf("Merged DV section %v should be the same as %v\n", dvm, dvc)
				}
			}
		}
	}

	dv1 = `{"enabled":false,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":30,"metering":{"tokens":3,"per_time_unit":"min"}}`
	dv2 = `{"enabled":false,"URL":"http://company.com/verify2","URLUser":"me2","URLPassword":"mysecret","interval":40,"metering":{"tokens":4,"per_time_unit":"min"}}`
	dv3 = `{"enabled":false,"URL":"","URLUser":"","URLPassword":"","interval":0,"check_rate":0,"metering":{"tokens":0,"per_time_unit":"","notification_interval":0}}`

	if dva := create_DataVerification(dv1, t); dva != nil {
		if dvb := create_DataVerification(dv2, t); dvb != nil {
			if dvc := create_DataVerification(dv3, t); dvc != nil {
				if dvm := dva.MergeWith(*dvb, 60); !dvm.IsSame(*dvc) {
					t.Errorf("Merged DV section %v should be the same as %v\n", dvm, dvc)
				}
			}
		}
	}

	dv1 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":30,"check_rate":10,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":25}}`
	dv2 = `{"enabled":true,"URL":"","URLUser":"","URLPassword":"","interval":0,"metering":{"tokens":0,"per_time_unit":"","notification_interval":0}}`
	dv3 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"********","interval":30,"check_rate":10,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":25}}`

	if dva := create_DataVerification(dv1, t); dva != nil {
		if dvb := create_DataVerification(dv2, t); dvb != nil {
			if dvc := create_DataVerification(dv3, t); dvc != nil {
				if dvm := dva.MergeWith(*dvb, 60); !dvm.IsSame(*dvc) {
					t.Errorf("Merged DV section %v should be the same as %v\n", dvm, dvc)
				}
			}
		}
	}

	dv1 = `{"enabled":true,"URL":"","URLUser":"","URLPassword":"","interval":0,"metering":{"tokens":0,"per_time_unit":"","notification_interval":0}}`
	dv2 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":30,"check_rate":10,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":25}}`
	dv3 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"********","interval":30,"check_rate":10,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":25}}`

	if dva := create_DataVerification(dv1, t); dva != nil {
		if dvb := create_DataVerification(dv2, t); dvb != nil {
			if dvc := create_DataVerification(dv3, t); dvc != nil {
				if dvm := dva.MergeWith(*dvb, 60); !dvm.IsSame(*dvc) {
					t.Errorf("Merged DV section %v should be the same as %v\n", dvm, dvc)
				}
			}
		}
	}

	dv1 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":0,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":0}}`
	dv2 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":0,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":0}}`
	dv3 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"********","interval":60,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":10}}`

	if dva := create_DataVerification(dv1, t); dva != nil {
		if dvb := create_DataVerification(dv2, t); dvb != nil {
			if dvc := create_DataVerification(dv3, t); dvc != nil {
				if dvm := dva.MergeWith(*dvb, 60); !dvm.IsSame(*dvc) {
					t.Errorf("Merged DV section %v should be the same as %v\n", dvm, dvc)
				}
			}
		}
	}

	dv1 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":30,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":0}}`
	dv2 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":30,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":0}}`
	dv3 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"********","interval":30,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":10}}`

	if dva := create_DataVerification(dv1, t); dva != nil {
		if dvb := create_DataVerification(dv2, t); dvb != nil {
			if dvc := create_DataVerification(dv3, t); dvc != nil {
				if dvm := dva.MergeWith(*dvb, 60); !dvm.IsSame(*dvc) {
					t.Errorf("Merged DV section %v should be the same as %v\n", dvm, dvc)
				}
			}
		}
	}

	// check rate tests
	dv1 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":30,"check_rate":10,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":0}}`
	dv2 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":30,"check_rate":10,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":0}}`
	dv3 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"********","interval":30,"check_rate":10,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":10}}`

	if dva := create_DataVerification(dv1, t); dva != nil {
		if dvb := create_DataVerification(dv2, t); dvb != nil {
			if dvc := create_DataVerification(dv3, t); dvc != nil {
				if dvm := dva.MergeWith(*dvb, 60); !dvm.IsSame(*dvc) {
					t.Errorf("Merged DV section %v should be the same as %v\n", dvm, dvc)
				}
			}
		}
	}

	dv1 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":30,"check_rate":10,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":0}}`
	dv2 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":30,"check_rate":15,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":0}}`
	dv3 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"********","interval":30,"check_rate":10,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":10}}`

	if dva := create_DataVerification(dv1, t); dva != nil {
		if dvb := create_DataVerification(dv2, t); dvb != nil {
			if dvc := create_DataVerification(dv3, t); dvc != nil {
				if dvm := dva.MergeWith(*dvb, 60); !dvm.IsSame(*dvc) {
					t.Errorf("Merged DV section %v should be the same as %v\n", dvm, dvc)
				}
			}
		}
	}

	dv1 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":30,"check_rate":15,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":0}}`
	dv2 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":30,"check_rate":10,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":0}}`
	dv3 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"********","interval":30,"check_rate":10,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":10}}`

	if dva := create_DataVerification(dv1, t); dva != nil {
		if dvb := create_DataVerification(dv2, t); dvb != nil {
			if dvc := create_DataVerification(dv3, t); dvc != nil {
				if dvm := dva.MergeWith(*dvb, 60); !dvm.IsSame(*dvc) {
					t.Errorf("Merged DV section %v should be the same as %v\n", dvm, dvc)
				}
			}
		}
	}

	dv1 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":30,"check_rate":0,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":0}}`
	dv2 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":30,"check_rate":10,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":0}}`
	dv3 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"********","interval":30,"check_rate":10,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":10}}`

	if dva := create_DataVerification(dv1, t); dva != nil {
		if dvb := create_DataVerification(dv2, t); dvb != nil {
			if dvc := create_DataVerification(dv3, t); dvc != nil {
				if dvm := dva.MergeWith(*dvb, 60); !dvm.IsSame(*dvc) {
					t.Errorf("Merged DV section %v should be the same as %v\n", dvm, dvc)
				}
			}
		}
	}

	dv1 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":30,"check_rate":10,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":0}}`
	dv2 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","interval":30,"check_rate":0,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":0}}`
	dv3 = `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"********","interval":30,"check_rate":10,"metering":{"tokens":3,"per_time_unit":"min","notification_interval":10}}`

	if dva := create_DataVerification(dv1, t); dva != nil {
		if dvb := create_DataVerification(dv2, t); dvb != nil {
			if dvc := create_DataVerification(dv3, t); dvc != nil {
				if dvm := dva.MergeWith(*dvb, 60); !dvm.IsSame(*dvc) {
					t.Errorf("Merged DV section %v should be the same as %v\n", dvm, dvc)
				}
			}
		}
	}

}

func Test_min_max(t *testing.T) {

	if minOf(0, 8) == 8 {
		t.Errorf("0 is min of 8\n")
	} else if minOf(5, 9) == 9 {
		t.Errorf("5 is min of 9\n")
	} else if minOf(8, 0) == 8 {
		t.Errorf("0 is min of 8\n")
	} else if minOf(9, 5) == 9 {
		t.Errorf("5 is min of 9\n")
	}

	if maxOf(0, 8) == 0 {
		t.Errorf("8 is max of 0\n")
	} else if maxOf(5, 9) == 5 {
		t.Errorf("9 is max of 5\n")
	} else if maxOf(8, 0) == 0 {
		t.Errorf("8 is max of 0\n")
	} else if maxOf(9, 5) == 5 {
		t.Errorf("9 is max of 5\n")
	}

	if minOf(0, 8) != 0 {
		t.Errorf("0 is min of 8\n")
	} else if minOf(5, 9) != 5 {
		t.Errorf("5 is min of 9\n")
	} else if minOf(8, 0) != 0 {
		t.Errorf("0 is min of 8\n")
	} else if minOf(9, 5) != 5 {
		t.Errorf("5 is min of 9\n")
	}

	if maxOf(0, 8) != 8 {
		t.Errorf("8 is max of 0\n")
	} else if maxOf(5, 9) != 9 {
		t.Errorf("9 is max of 5\n")
	} else if maxOf(8, 0) != 8 {
		t.Errorf("8 is max of 0\n")
	} else if maxOf(9, 5) != 9 {
		t.Errorf("9 is max of 5\n")
	}
}

func Test_normalize_tokens(t *testing.T) {
	if a := normalizeTokens(2, "day"); a != 2 {
		t.Errorf("Should be 2, was %v\n", a)
	} else if a := normalizeTokens(2, "hour"); a != 2*24 {
		t.Errorf("Should be %v, was %v\n", 2*24, a)
	} else if a := normalizeTokens(2, "min"); a != 2*1440 {
		t.Errorf("Should be %v, was %v\n", 2*1440, a)
	}
}

func Test_isvalid_meter(t *testing.T) {

	// Valid Tests
	m1 := Meter{Tokens: 3, PerTimeUnit: "day", NotificationIntervalS: 300}
	if !m1.IsValid() {
		t.Errorf("Meter %v is valid\n", m1)
	}

	m1 = Meter{Tokens: 3, PerTimeUnit: "day", NotificationIntervalS: 0}
	if !m1.IsValid() {
		t.Errorf("Meter %v is valid\n", m1)
	}

	m1 = Meter{Tokens: 0, PerTimeUnit: "", NotificationIntervalS: 0}
	if !m1.IsValid() {
		t.Errorf("Meter %v is valid\n", m1)
	}

	// Invalid tests
	m1 = Meter{Tokens: 3, PerTimeUnit: "", NotificationIntervalS: 300}
	if m1.IsValid() {
		t.Errorf("Meter %v is not valid\n", m1)
	}

	m1 = Meter{Tokens: 0, PerTimeUnit: "day", NotificationIntervalS: 300}
	if m1.IsValid() {
		t.Errorf("Meter %v is not valid\n", m1)
	}

	m1 = Meter{Tokens: 0, PerTimeUnit: "", NotificationIntervalS: 300}
	if m1.IsValid() {
		t.Errorf("Meter %v is not valid\n", m1)
	}

	m1 = Meter{Tokens: 0, PerTimeUnit: "fred", NotificationIntervalS: 0}
	if m1.IsValid() {
		t.Errorf("Meter %v is not valid\n", m1)
	}

}

func Test_isempty_meter(t *testing.T) {

	m1 := Meter{Tokens: 0, PerTimeUnit: "", NotificationIntervalS: 0}
	if !m1.IsEmpty() {
		t.Errorf("Meter %v is empty\n", m1)
	}

	m1 = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 10}
	if m1.IsEmpty() {
		t.Errorf("Meter %v is not empty\n", m1)
	}

	m1 = Meter{Tokens: 0, PerTimeUnit: "min", NotificationIntervalS: 10}
	if m1.IsEmpty() {
		t.Errorf("Meter %v is not empty\n", m1)
	}

	m1 = Meter{Tokens: 0, PerTimeUnit: "", NotificationIntervalS: 10}
	if m1.IsEmpty() {
		t.Errorf("Meter %v is not empty\n", m1)
	}

}

func Test_issame_meter(t *testing.T) {

	// The same
	m1 := Meter{Tokens: 0, PerTimeUnit: "", NotificationIntervalS: 0}
	m2 := Meter{Tokens: 0, PerTimeUnit: "", NotificationIntervalS: 0}
	if !m1.IsSame(m2) {
		t.Errorf("Meters %v and %v are the same\n", m1, m2)
	}

	m1 = Meter{Tokens: 3, PerTimeUnit: "", NotificationIntervalS: 0}
	m2 = Meter{Tokens: 3, PerTimeUnit: "", NotificationIntervalS: 0}
	if !m1.IsSame(m2) {
		t.Errorf("Meters %v and %v are the same\n", m1, m2)
	}

	m1 = Meter{Tokens: 3, PerTimeUnit: "day", NotificationIntervalS: 0}
	m2 = Meter{Tokens: 3, PerTimeUnit: "day", NotificationIntervalS: 0}
	if !m1.IsSame(m2) {
		t.Errorf("Meters %v and %v are the same\n", m1, m2)
	}

	m1 = Meter{Tokens: 3, PerTimeUnit: "day", NotificationIntervalS: 100}
	m2 = Meter{Tokens: 3, PerTimeUnit: "day", NotificationIntervalS: 100}
	if !m1.IsSame(m2) {
		t.Errorf("Meters %v and %v are the same\n", m1, m2)
	}

	// Not the same
	m1 = Meter{Tokens: 0, PerTimeUnit: "day", NotificationIntervalS: 100}
	m2 = Meter{Tokens: 3, PerTimeUnit: "day", NotificationIntervalS: 100}
	if m1.IsSame(m2) {
		t.Errorf("Meters %v and %v are the same\n", m1, m2)
	}

	m1 = Meter{Tokens: 3, PerTimeUnit: "day", NotificationIntervalS: 100}
	m2 = Meter{Tokens: 3, PerTimeUnit: "hour", NotificationIntervalS: 100}
	if m1.IsSame(m2) {
		t.Errorf("Meters %v and %v are the same\n", m1, m2)
	}

	m1 = Meter{Tokens: 3, PerTimeUnit: "day", NotificationIntervalS: 0}
	m2 = Meter{Tokens: 3, PerTimeUnit: "day", NotificationIntervalS: 100}
	if m1.IsSame(m2) {
		t.Errorf("Meters %v and %v are the same\n", m1, m2)
	}

}

func Test_isSatisfiedBy_meter(t *testing.T) {

	// Satisfied
	m1 := Meter{Tokens: 0, PerTimeUnit: "", NotificationIntervalS: 0}
	m2 := Meter{Tokens: 0, PerTimeUnit: "", NotificationIntervalS: 0}
	if !m1.IsSatisfiedBy(m2) {
		t.Errorf("Meters %v is satisfied by %v.\n", m1, m2)
	}

	m1 = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 100}
	m2 = Meter{Tokens: 0, PerTimeUnit: "", NotificationIntervalS: 0}
	if !m1.IsSatisfiedBy(m2) {
		t.Errorf("Meters %v is satisfied by %v.\n", m1, m2)
	}

	m1 = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 100}
	m2 = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 100}
	if !m1.IsSatisfiedBy(m2) {
		t.Errorf("Meters %v is satisfied by %v.\n", m1, m2)
	}

	m1 = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 100}
	m2 = Meter{Tokens: 4, PerTimeUnit: "min", NotificationIntervalS: 100}
	if !m1.IsSatisfiedBy(m2) {
		t.Errorf("Meters %v is satisfied by %v.\n", m1, m2)
	}

	m1 = Meter{Tokens: 10, PerTimeUnit: "hour", NotificationIntervalS: 100}
	m2 = Meter{Tokens: 4, PerTimeUnit: "min", NotificationIntervalS: 100}
	if !m1.IsSatisfiedBy(m2) {
		t.Errorf("Meters %v is satisfied by %v.\n", m1, m2)
	}

	m1 = Meter{Tokens: 10, PerTimeUnit: "day", NotificationIntervalS: 100}
	m2 = Meter{Tokens: 4, PerTimeUnit: "min", NotificationIntervalS: 100}
	if !m1.IsSatisfiedBy(m2) {
		t.Errorf("Meters %v is satisfied by %v.\n", m1, m2)
	}

	m1 = Meter{Tokens: 3, PerTimeUnit: "hour", NotificationIntervalS: 100}
	m2 = Meter{Tokens: 3, PerTimeUnit: "hour", NotificationIntervalS: 100}
	if !m1.IsSatisfiedBy(m2) {
		t.Errorf("Meters %v is satisfied by %v.\n", m1, m2)
	}

	m1 = Meter{Tokens: 3, PerTimeUnit: "hour", NotificationIntervalS: 100}
	m2 = Meter{Tokens: 4, PerTimeUnit: "hour", NotificationIntervalS: 100}
	if !m1.IsSatisfiedBy(m2) {
		t.Errorf("Meters %v is satisfied by %v.\n", m1, m2)
	}

	m1 = Meter{Tokens: 10, PerTimeUnit: "day", NotificationIntervalS: 100}
	m2 = Meter{Tokens: 4, PerTimeUnit: "hour", NotificationIntervalS: 100}
	if !m1.IsSatisfiedBy(m2) {
		t.Errorf("Meters %v is satisfied by %v.\n", m1, m2)
	}

	m1 = Meter{Tokens: 3, PerTimeUnit: "day", NotificationIntervalS: 100}
	m2 = Meter{Tokens: 3, PerTimeUnit: "day", NotificationIntervalS: 100}
	if !m1.IsSatisfiedBy(m2) {
		t.Errorf("Meters %v is satisfied by %v.\n", m1, m2)
	}

	m1 = Meter{Tokens: 3, PerTimeUnit: "day", NotificationIntervalS: 100}
	m2 = Meter{Tokens: 4, PerTimeUnit: "day", NotificationIntervalS: 100}
	if !m1.IsSatisfiedBy(m2) {
		t.Errorf("Meters %v is satisfied by %v.\n", m1, m2)
	}

	// Not satisfied
	m1 = Meter{Tokens: 4, PerTimeUnit: "min", NotificationIntervalS: 100}
	m2 = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 100}
	if m1.IsSatisfiedBy(m2) {
		t.Errorf("Meters %v is not satisfied by %v.\n", m1, m2)
	}

	m1 = Meter{Tokens: 200, PerTimeUnit: "hour", NotificationIntervalS: 100}
	m2 = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 100}
	if m1.IsSatisfiedBy(m2) {
		t.Errorf("Meters %v is not satisfied by %v.\n", m1, m2)
	}

	m1 = Meter{Tokens: 4800, PerTimeUnit: "day", NotificationIntervalS: 100}
	m2 = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 100}
	if m1.IsSatisfiedBy(m2) {
		t.Errorf("Meters %v is not satisfied by %v.\n", m1, m2)
	}

	m1 = Meter{Tokens: 4, PerTimeUnit: "hour", NotificationIntervalS: 100}
	m2 = Meter{Tokens: 3, PerTimeUnit: "hour", NotificationIntervalS: 100}
	if m1.IsSatisfiedBy(m2) {
		t.Errorf("Meters %v is not satisfied by %v.\n", m1, m2)
	}

	m1 = Meter{Tokens: 73, PerTimeUnit: "day", NotificationIntervalS: 100}
	m2 = Meter{Tokens: 3, PerTimeUnit: "hour", NotificationIntervalS: 100}
	if m1.IsSatisfiedBy(m2) {
		t.Errorf("Meters %v is not satisfied by %v.\n", m1, m2)
	}

	m1 = Meter{Tokens: 4, PerTimeUnit: "day", NotificationIntervalS: 100}
	m2 = Meter{Tokens: 3, PerTimeUnit: "day", NotificationIntervalS: 100}
	if m1.IsSatisfiedBy(m2) {
		t.Errorf("Meters %v is not satisfied by %v.\n", m1, m2)
	}

}

// Tests for merging a producer and consumer metering section
func Test_merge_with_meter(t *testing.T) {

	// Time unit and token calculations
	m1 := Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 100}
	m2 := Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 100}
	if am := m1.MergeWith(m2, 30); !am.IsSame(m1) {
		t.Errorf("Meter %v was not merged correclty, expecting %v\n", am, m1)
	}

	m1 = Meter{Tokens: 3, PerTimeUnit: "hour", NotificationIntervalS: 100}
	m2 = Meter{Tokens: 3, PerTimeUnit: "hour", NotificationIntervalS: 100}
	if am := m1.MergeWith(m2, 30); !am.IsSame(m1) {
		t.Errorf("Meter %v was not merged correclty, expecting %v\n", am, m1)
	}

	m1 = Meter{Tokens: 3, PerTimeUnit: "day", NotificationIntervalS: 100}
	m2 = Meter{Tokens: 3, PerTimeUnit: "day", NotificationIntervalS: 100}
	if am := m1.MergeWith(m2, 30); !am.IsSame(m1) {
		t.Errorf("Meter %v was not merged correclty, expecting %v\n", am, m1)
	}

	m1 = Meter{Tokens: 3, PerTimeUnit: "hour", NotificationIntervalS: 100}
	m2 = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 100}
	mm := Meter{Tokens: 180, PerTimeUnit: "hour", NotificationIntervalS: 100}
	if am := m1.MergeWith(m2, 30); !am.IsSame(mm) {
		t.Errorf("Meter %v was not merged correclty, expecting %v\n", am, mm)
	}

	m1 = Meter{Tokens: 3, PerTimeUnit: "day", NotificationIntervalS: 100}
	m2 = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 100}
	mm = Meter{Tokens: 4320, PerTimeUnit: "day", NotificationIntervalS: 100}
	if am := m1.MergeWith(m2, 30); !am.IsSame(mm) {
		t.Errorf("Meter %v was not merged correclty, expecting %v\n", am, mm)
	}

	m1 = Meter{Tokens: 3, PerTimeUnit: "day", NotificationIntervalS: 100}
	m2 = Meter{Tokens: 3, PerTimeUnit: "hour", NotificationIntervalS: 100}
	mm = Meter{Tokens: 72, PerTimeUnit: "day", NotificationIntervalS: 100}
	if am := m1.MergeWith(m2, 30); !am.IsSame(mm) {
		t.Errorf("Meter %v was not merged correclty, expecting %v\n", am, mm)
	}

	m1 = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 100}
	m2 = Meter{Tokens: 200, PerTimeUnit: "hour", NotificationIntervalS: 100}
	mm = Meter{Tokens: 200, PerTimeUnit: "hour", NotificationIntervalS: 100}
	if am := m1.MergeWith(m2, 30); !am.IsSame(mm) {
		t.Errorf("Meter %v was not merged correclty, expecting %v\n", am, mm)
	}

	m1 = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 100}
	m2 = Meter{Tokens: 4321, PerTimeUnit: "day", NotificationIntervalS: 100}
	mm = Meter{Tokens: 4321, PerTimeUnit: "day", NotificationIntervalS: 100}
	if am := m1.MergeWith(m2, 30); !am.IsSame(mm) {
		t.Errorf("Meter %v was not merged correclty, expecting %v\n", am, mm)
	}

	m1 = Meter{Tokens: 3, PerTimeUnit: "hour", NotificationIntervalS: 100}
	m2 = Meter{Tokens: 73, PerTimeUnit: "day", NotificationIntervalS: 100}
	mm = Meter{Tokens: 73, PerTimeUnit: "day", NotificationIntervalS: 100}
	if am := m1.MergeWith(m2, 30); !am.IsSame(mm) {
		t.Errorf("Meter %v was not merged correclty, expecting %v\n", am, mm)
	}

	// Notification interval time
	m1 = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 90}
	m2 = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 100}
	if am := m1.MergeWith(m2, 30); !am.IsSame(m1) {
		t.Errorf("Meter %v was not merged correclty, expecting %v\n", am, m1)
	}

	m1 = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 0}
	m2 = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 0}
	mm = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 10}
	if am := m1.MergeWith(m2, 0); !am.IsSame(mm) {
		t.Errorf("Meter %v was not merged correclty, expecting %v\n", am, mm)
	}

	m1 = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 0}
	m2 = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 0}
	mm = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 10}
	if am := m1.MergeWith(m2, 10); !am.IsSame(mm) {
		t.Errorf("Meter %v was not merged correclty, expecting %v\n", am, mm)
	}

	m1 = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 0}
	m2 = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 0}
	mm = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 30}
	if am := m1.MergeWith(m2, 30); !am.IsSame(mm) {
		t.Errorf("Meter %v was not merged correclty, expecting %v\n", am, mm)
	}

	m1 = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 0}
	m2 = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 40}
	mm = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 40}
	if am := m1.MergeWith(m2, 30); !am.IsSame(mm) {
		t.Errorf("Meter %v was not merged correclty, expecting %v\n", am, mm)
	}

	m1 = Meter{Tokens: 0, PerTimeUnit: "", NotificationIntervalS: 0}
	m2 = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 40}
	mm = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 40}
	if am := m1.MergeWith(m2, 30); !am.IsSame(mm) {
		t.Errorf("Meter %v was not merged correclty, expecting %v\n", am, mm)
	}

	m1 = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 40}
	m2 = Meter{Tokens: 0, PerTimeUnit: "", NotificationIntervalS: 0}
	mm = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 40}
	if am := m1.MergeWith(m2, 30); !am.IsSame(mm) {
		t.Errorf("Meter %v was not merged correclty, expecting %v\n", am, mm)
	}

	m1 = Meter{Tokens: 0, PerTimeUnit: "", NotificationIntervalS: 0}
	m2 = Meter{Tokens: 0, PerTimeUnit: "", NotificationIntervalS: 0}
	mm = Meter{Tokens: 0, PerTimeUnit: "", NotificationIntervalS: 0}
	if am := m1.MergeWith(m2, 30); !am.IsSame(mm) {
		t.Errorf("Meter %v was not merged correclty, expecting %v\n", am, mm)
	}

}

// Tests for merging 2 producer metering sections.
func Test_merge_with_meter2(t *testing.T) {

	m1 := Meter{Tokens: 2, PerTimeUnit: "min", NotificationIntervalS: 60}
	m2 := Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 40}
	mm := Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 40}
	if am := m1.ProducerMergeWith(&m2, 30); !am.IsSame(mm) {
		t.Errorf("Meter %v was not merged correclty, expecting %v\n", am, mm)
	}

	m1 = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 40}
	m2 = Meter{Tokens: 2, PerTimeUnit: "min", NotificationIntervalS: 60}
	mm = Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 40}
	if am := m1.ProducerMergeWith(&m2, 30); !am.IsSame(mm) {
		t.Errorf("Meter %v was not merged correclty, expecting %v\n", am, mm)
	}

	m1 = Meter{Tokens: 0, PerTimeUnit: "min", NotificationIntervalS: 40}
	m2 = Meter{Tokens: 0, PerTimeUnit: "min", NotificationIntervalS: 60}
	mm = Meter{Tokens: 0, PerTimeUnit: "min", NotificationIntervalS: 40}
	if am := m1.ProducerMergeWith(&m2, 30); !am.IsSame(mm) {
		t.Errorf("Meter %v was not merged correclty, expecting %v\n", am, mm)
	}

	m1 = Meter{}
	m2 = Meter{}
	mm = Meter{}
	if am := m1.ProducerMergeWith(&m2, 30); !am.IsSame(mm) {
		t.Errorf("Meter %v was not merged correclty, expecting %v\n", am, mm)
	}

	m1 = Meter{}
	m2 = Meter{Tokens: 1, PerTimeUnit: "min", NotificationIntervalS: 60}
	mm = Meter{Tokens: 1, PerTimeUnit: "min", NotificationIntervalS: 60}
	if am := m1.ProducerMergeWith(&m2, 30); !am.IsSame(mm) {
		t.Errorf("Meter %v was not merged correclty, expecting %v\n", am, mm)
	}

	m1 = Meter{Tokens: 1, PerTimeUnit: "min", NotificationIntervalS: 60}
	m2 = Meter{}
	mm = Meter{Tokens: 1, PerTimeUnit: "min", NotificationIntervalS: 60}
	if am := m1.ProducerMergeWith(&m2, 30); !am.IsSame(mm) {
		t.Errorf("Meter %v was not merged correclty, expecting %v\n", am, mm)
	}

	m1 = Meter{Tokens: 1, PerTimeUnit: "min", NotificationIntervalS: 80}
	m2 = Meter{Tokens: 1, PerTimeUnit: "hour", NotificationIntervalS: 60}
	mm = Meter{Tokens: 60, PerTimeUnit: "hour", NotificationIntervalS: 60}
	if am := m1.ProducerMergeWith(&m2, 30); !am.IsSame(mm) {
		t.Errorf("Meter %v was not merged correclty, expecting %v\n", am, mm)
	}

	m1 = Meter{Tokens: 1, PerTimeUnit: "day", NotificationIntervalS: 40}
	m2 = Meter{Tokens: 1, PerTimeUnit: "hour", NotificationIntervalS: 60}
	mm = Meter{Tokens: 24, PerTimeUnit: "day", NotificationIntervalS: 40}
	if am := m1.ProducerMergeWith(&m2, 30); !am.IsSame(mm) {
		t.Errorf("Meter %v was not merged correclty, expecting %v\n", am, mm)
	}

}

// Create an Data Verification section from a JSON serialization. The JSON serialization
// does not have to be a valid DataVerification serialization, just has to be a valid
// JSON serialization.
func create_DataVerification(jsonString string, t *testing.T) *DataVerification {
	dv := new(DataVerification)

	if err := json.Unmarshal([]byte(jsonString), &dv); err != nil {
		t.Errorf("Error unmarshalling DataVerification json string: %v error:%v\n", jsonString, err)
		return nil
	} else {
		return dv
	}
}
