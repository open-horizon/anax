//go:build unit
// +build unit

package api

import (
	"strings"
	"testing"
)

func Test_APIInputError(t *testing.T) {

	theError := "the error"
	theInput := "bad input"

	apiErr := NewAPIUserInputError(theError, theInput)
	if apiErr == nil {
		t.Errorf("API User input constructor returned nil")
	}

	errString := apiErr.Error()
	if !strings.Contains(errString, theError) && !strings.Contains(errString, theInput) {
		t.Errorf("Error string does not contain the input values %v and %v, it is %v", theError, theInput, apiErr)
	}

}

func Test_SystemError(t *testing.T) {

	theError := "the error"

	sysErr := NewSystemError(theError)
	if sysErr == nil {
		t.Errorf("System error constructor returned nil")
	}

	errString := sysErr.Error()
	if !strings.Contains(errString, theError) {
		t.Errorf("Error string does not contain the input value %v, it is %v", theError, sysErr)
	}

}

func Test_PassthruHandler(t *testing.T) {

	var myError error
	handler := GetPassThroughErrorHandler(&myError)
	if handler == nil {
		t.Errorf("Return nil handler")
	}

	sysErr := NewSystemError("passthru test error")

	handled := handler(sysErr)

	if !handled {
		t.Errorf("Handler should always return true")
	}

	if myError.Error() != sysErr.Error() {
		t.Errorf("Passed through error was %T %v, should be %T %v", myError, myError, sysErr, sysErr)
	}

}
