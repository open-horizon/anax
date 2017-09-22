// +build unit

package api

import (
	"testing"
)

func Test_InputIsIllegal(t *testing.T) {
	fails := func(str string) bool {
		if b, _ := InputIsIllegal(str); b != "" {
			return true
		}
		return false
	}

	if fails("Fo_o ()2, .@ +") {
		t.Errorf("Input found to be illegal but isn't")
	}

	if fails("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!&()@?_-*+.") {
		t.Errorf("Input found to be illegal but isn't (this is the superset of the legal character collections for WIoTP API tokens, orgs and device / type ids)")
	}

	if !fails("fro{") || !fails("go[") || !fails(">oog") {
		t.Errorf("Input found to be legal but isn't")
	}

}

func Test_MapInputIsIllegal(t *testing.T) {
	if b, _, _ := MapInputIsIllegal(map[string]interface{}{"one": "g oo", "two": "()", "t hree": "3", "f()our": " ", "germ": "fooo"}); b != "" {
		t.Errorf("Map input found to be illegal but isn't")
	}

	if b, _, _ := MapInputIsIllegal(map[string]interface{}{"one": "g oo", "two": "()", "t hree": "3", "f()our": " ", "germ": "X%"}); b == "" {
		t.Errorf("Map input found to be legal but isn't")
	}

	if b, _, _ := MapInputIsIllegal(map[string]interface{}{"one": "g oo", "t#wo": "()", "t hree": "3", "f()our": " ", "germ": "foo"}); b == "" {
		t.Errorf("Map input found to be legal but isn't")
	}
}
