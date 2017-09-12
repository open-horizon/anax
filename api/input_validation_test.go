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

	if b, _ := InputIsIllegal("Fo_o ()2, .@"); b != "" {
		t.Errorf("Input found to be illegal but isn't")
	}

	if !fails("z00!") || !fails("fro{") || !fails("go[") || !fails(">oog") {
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
