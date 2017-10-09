package api

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"net/http"
)

// This function type is used to enable plug replaceable error handlers within the API
// implementation so that business logic can be tested independently from the HTTP
// transport that is used to access the API.
type ErrorHandler func(err error) bool

// APIUserInputError is for problems found with input path variables or input bodies. The Input field is flexible;
// could be a field name or other. Note: the info in this field is intended to be consumed by humans, either API
// consumers or developers of the UI. Add enum codes if these are to be evaluated in frontend code.
type APIUserInputError struct {
	Err   string `json:"error"`
	Input string `json:"input,omitempty"`
}

func (e APIUserInputError) Error() string {
	return fmt.Sprintf("Input: %v, Error: %v", e.Input, e.Err)
}

func NewAPIUserInputError(err string, input string) *APIUserInputError {
	return &APIUserInputError{
		Err:   err,
		Input: input,
	}
}

// MSMissingVariableConfigError is for problems found with microservice configuration where the microservice definition
// requires 1 or more input variables to be set but 1 or more of those variables has not been set.
type MSMissingVariableConfigError struct {
	Err   string `json:"error"`
	Input string `json:"input,omitempty"`
}

func (e MSMissingVariableConfigError) Error() string {
	return fmt.Sprintf("Input: %v, Error: %v", e.Input, e.Err)
}

func NewMSMissingVariableConfigError(err string, input string) *MSMissingVariableConfigError {
	return &MSMissingVariableConfigError{
		Err:   err,
		Input: input,
	}
}

// DuplicateServiceError occurs when a microservice configuration is attempted for a service that has already been
// configured.
type DuplicateServiceError struct {
	Err   string `json:"error"`
	Input string `json:"input,omitempty"`
}

func (e DuplicateServiceError) Error() string {
	return fmt.Sprintf("Input: %v, Error: %v", e.Input, e.Err)
}

func NewDuplicateServiceError(err string, input string) *DuplicateServiceError {
	return &DuplicateServiceError{
		Err:   err,
		Input: input,
	}
}

// Conflict Errors are expected, since they can occur as the result of incorrect usage of the API.
type ConflictError struct {
	msg string
}

func (e ConflictError) Error() string {
	return e.msg
}

func NewConflictError(err string) *ConflictError {
	return &ConflictError{
		msg: err,
	}
}

// System Errors are generally unexpected, infrastructural problems that just need to be reported out to the caller.
type SystemError struct {
	msg string
}

func (e SystemError) Error() string {
	return e.msg
}

func NewSystemError(err string) *SystemError {
	return &SystemError{
		msg: err,
	}
}

// Use this function to obtain an error handler that simply passes the error through itself back to caller. This is
// done by modifying the error variable passed to this function.
func GetPassThroughErrorHandler(passthruErr *error) ErrorHandler {
	//return func(err error) bool {
	return func(err error) bool {
		*passthruErr = err
		return true
	}
}

// Use this function to obtain an error handler that writes errors to the HTTP response.
func GetHTTPErrorHandler(w http.ResponseWriter) ErrorHandler {
	// returned value indicates whether or not processing can continue
	return func(err error) bool {
		if err != nil {
			switch err.(type) {
			case *APIUserInputError:
				apiErr := err.(*APIUserInputError)
				writeInputErr(w, http.StatusBadRequest, apiErr)

			case *MSMissingVariableConfigError:
				// convert to an API Input Error
				msErr := err.(*MSMissingVariableConfigError)
				apiErr := NewAPIUserInputError(msErr.Err, msErr.Input)
				writeInputErr(w, http.StatusBadRequest, apiErr)

			case *DuplicateServiceError:
				// convert to an API Input Error
				dupErr := err.(*DuplicateServiceError)
				apiErr := NewAPIUserInputError(dupErr.Err, dupErr.Input)
				writeInputErr(w, http.StatusBadRequest, apiErr)

			case *SystemError:
				sysErr := err.(*SystemError)
				glog.Errorf(apiLogString(sysErr.Error()))
				http.Error(w, sysErr.Error(), http.StatusInternalServerError)

			case *ConflictError:
				conErr := err.(*ConflictError)
				glog.Errorf(apiLogString(conErr.Error()))
				http.Error(w, conErr.Error(), http.StatusConflict)

			default:
				glog.Errorf(apiLogString(fmt.Sprintf("unknown error (%T) %v", err, err.Error())))
				http.Error(w, "Internal server error", http.StatusInternalServerError)

			}
			// tell the caller they should not continue processing
			return true

		} else {
			return false
		}
	}
}

// use this function to properly write a User Input Error to the http response.
func writeInputErr(writer http.ResponseWriter, status int, inputErr *APIUserInputError) {
	if serial, err := json.Marshal(inputErr); err != nil {
		glog.Errorf(apiLogString(fmt.Sprintf("Error serializing input error: %v, error %v", inputErr, err)))
		http.Error(writer, "Internal server error", http.StatusInternalServerError)
	} else {
		writer.WriteHeader(status)
		writer.Header().Set("Content-Type", "application/json")
		if _, err := writer.Write(serial); err != nil {
			glog.Errorf(apiLogString(fmt.Sprintf("Error writing response: %v, error %v", serial, err)))
			http.Error(writer, "Internal server error", http.StatusInternalServerError)
		} else {
			glog.Errorf(apiLogString(fmt.Sprintf("Returning status %v for error %v", status, string(serial))))
		}
	}
}
