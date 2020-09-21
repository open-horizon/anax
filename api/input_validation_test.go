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

	if b, _, _ := MapInputIsIllegal(map[string]interface{}{"one": "g oo", "two": "()", "t hree": "3", "f()our": " ", "germ": "X%"}); b != "" {
		t.Errorf("Map input found to be illegal but isn't")
	}

	if b, _, _ := MapInputIsIllegal(map[string]interface{}{"one": "g oo", "t#wo": "()", "t hree": "3", "f()our": " ", "germ": "foo"}); b == "" {
		t.Errorf("Map input found to be legal but isn't")
	}
}

func Test_CheckInputString_valid1(t *testing.T) {

	var myError error
	handler := GetPassThroughErrorHandler(&myError)

	aField := "input1"
	theInput := "instring"
	errorOccurred := checkInputString(handler, aField, &theInput)

	if errorOccurred {
		t.Errorf("found problem with input, but it is ok: %v", theInput)
	}

	if myError != nil && len(myError.Error()) != 0 {
		t.Errorf("error should be empty, is %v", myError.Error())
	}

}

func Test_CheckInputString_valid2(t *testing.T) {

	var myError error
	handler := GetPassThroughErrorHandler(&myError)

	aField := "input1"
	theInput := ""
	errorOccurred := checkInputString(handler, aField, &theInput)

	if errorOccurred {
		t.Errorf("found problem with input, but it is ok: %v", theInput)
	}

	if myError != nil && len(myError.Error()) != 0 {
		t.Errorf("error should be empty, is %v", myError.Error())
	}

}

func Test_CheckInputString_invalid1(t *testing.T) {

	var myError error
	handler := GetPassThroughErrorHandler(&myError)

	aField := "input1"
	theInput := ">0"
	errorOccurred := checkInputString(handler, aField, &theInput)

	if !errorOccurred {
		t.Errorf("no problem found with input, but it illegal: %v", theInput)
	}

	if myError == nil || len(myError.Error()) == 0 {
		t.Errorf("error should not be empty, but is")
	}

}

func Test_CheckInputString_invalid2(t *testing.T) {

	var myError error
	handler := GetPassThroughErrorHandler(&myError)

	aField := "input1"
	errorOccurred := checkInputString(handler, aField, nil)

	if !errorOccurred {
		t.Errorf("no problem found with input, but it illegal")
	}

	if myError == nil || len(myError.Error()) == 0 {
		t.Errorf("error should not be empty, but is")
	}

}

func Test_CheckMapIsLegal1(t *testing.T) {

	inMap := map[string]interface{}{}

	key, errorMsg, err := MapInputIsIllegal(inMap)

	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	if key != "" {
		t.Errorf("unexpected key value %v", key)
	}

	if errorMsg != "" {
		t.Errorf("unexpected error message %v", errorMsg)
	}

}

func Test_CheckMapIsLegal2(t *testing.T) {

	inMap := map[string]interface{}{
		"key1": "value1",
	}

	key, errorMsg, err := MapInputIsIllegal(inMap)

	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	if key != "" {
		t.Errorf("unexpected key value %v", key)
	}

	if errorMsg != "" {
		t.Errorf("unexpected error message %v", errorMsg)
	}

}

func Test_CheckMapIsIllegal1(t *testing.T) {

	badKey := "<0"

	inMap := map[string]interface{}{
		badKey: "value1",
	}

	key, errorMsg, err := MapInputIsIllegal(inMap)

	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	if key != badKey {
		t.Errorf("expected error key value %v", badKey)
	}

	if errorMsg == "" {
		t.Errorf("expected error message")
	}
}

func Test_CheckInputString_unicode(t *testing.T) {

	// inputs from different languages
	inputs := []string{"für", "Não", "organización", "désérialisé", "ユーザー組", "사용자", "启动服务", "資料庫"}
	aField := "input1"

	for _, theInput := range inputs {
		var myError error
		handler := GetPassThroughErrorHandler(&myError)
		errorOccurred := checkInputString(handler, aField, &theInput)

		if errorOccurred {
			t.Errorf("found problem with input, but it is ok: %v", theInput)
		}

		if myError != nil && len(myError.Error()) != 0 {
			t.Errorf("error should be empty, is %v", myError.Error())
		}
	}

}
