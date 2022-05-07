//go:build unit
// +build unit

package helm

import (
	"encoding/base64"
	"flag"
	"io/ioutil"
	"testing"
)

func init() {
	// Enable glog tracing in the tested functions. The output will be displayed when -v is
	// passed on the go test command.
	flag.Set("alsologtostderr", "true")
	flag.Set("v", "7")
	// no need to parse flags, that's done by test framework
}

func Test_DecodeAndCreate(t *testing.T) {

	str := "testtesttesttest"
	sEnc := base64.StdEncoding.EncodeToString([]byte(str))

	if fileName, err := ConvertB64StringToFile(sEnc); err != nil {
		t.Errorf("Source: %v, encoded: %v, error: %v", str, sEnc, err)
	} else if dat, err := ioutil.ReadFile(fileName); err != nil {
		t.Errorf("Source: %v, encoded: %v, error: %v", str, sEnc, err)
	} else if str != string(dat) {
		t.Errorf("Encoded: %v, read: %v", str, string(dat))
	} else if b64, err := ConvertFileToB64String(fileName); err != nil {
		t.Errorf("File: %v, error: %v", fileName, err)
	} else if sEnc != b64 {
		t.Errorf("Encoded: %v, read: %v", sEnc, b64)
	}

}
