package policy

import (
    "encoding/json"
	"testing"
)

func Test_data_verification_section_success(t *testing.T) {

    dv1 := `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","Interval":0}`
    dv2 := `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","Interval":0}`
    if dva := create_DataVerification(dv1, t); dva != nil {
        if dvb := create_DataVerification(dv2, t); dvb != nil {
            if !dva.IsSame(*dvb) {
                t.Errorf("DV section %v is the same as %v\n", dva, dvb)
            }
        }
    }

}

func Test_data_verification_section_failure(t *testing.T) {

    dv1 := `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","Interval":0}`
    dv2 := `{"enabled":true,"URL":"http://othercompany.com/verify","URLUser":"me","URLPassword":"mysecret","Interval":0}`
    if dva := create_DataVerification(dv1, t); dva != nil {
        if dvb := create_DataVerification(dv2, t); dvb != nil {
            if dva.IsSame(*dvb) {
                t.Errorf("DV section %v is not the same as %v\n", dva, dvb)
            }
        }
    }

}

func Test_data_verification_obscure(t *testing.T) {

    dv1 := `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","Interval":0}`
    dv2 := `{"enabled":true,"URL":"http://company.com/verify","URLUser":"me","URLPassword":"mysecret","Interval":0}`
    if dva := create_DataVerification(dv1, t); dva != nil {
        upb4 := dva.URLPassword
        dva.Obscure()
        if dva.URLPassword == upb4 || dva.URLPassword != "********" {
            t.Errorf("DV section %v was not obscured correctly\n", dva)
        } else if dvb := create_DataVerification(dv2, t); dvb != nil {
            if dva.IsSame(*dvb) {
                t.Errorf("DV section %v is the same as %v\n", dva, dvb)
            }
        }
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
