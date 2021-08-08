package secrets

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
)

// ----- ERROR WRAPPER -----

// overall SecretsProviderError structure that wraps the below errors; every error coming out of the secrets manager (which will be one
// of the defined errors below) can be casted to this
type SecretsProviderError struct {
	// record the http response code and response of the secrets manager
	ResponseCode int
	Response     string

	// implement error interface
	Err error
}

func (e *SecretsProviderError) Error() string {
	return e.Error()
}

// ----- HELPER FUNCTION -----

// takes one of the errors below and wraps it into a SecretsProviderError
// that records the response code and secrets provider response in addition to the error itself
func WrapSecretsError(err error) *SecretsProviderError {

	// if no error, ignore
	if err == nil {
		return nil
	}

	// case on the type of error
	switch e := err.(type) {
	case *PermissionDenied:
		return &SecretsProviderError{ResponseCode: 403, Response: RespToString(e.Response), Err: err}
	case *Unauthenticated:
		return &SecretsProviderError{ResponseCode: 401, Response: e.LoginError.Error(), Err: err}
	case *SecretsProviderUnavailable:
		return &SecretsProviderError{ResponseCode: 503, Response: e.ProviderError.Error(), Err: err}
	case *InvalidResponse:
		// find the secrets provider response in the struct
		var errString string
		if e.ReadError != nil {
			errString = e.ReadError.Error()
		} else {
			errString = string(e.Response)
		}

		return &SecretsProviderError{ResponseCode: 500, Response: errString, Err: err}
	case *BadRequest:
		return &SecretsProviderError{ResponseCode: e.ResponseCode, Response: RespToString(e.Response), Err: err}
	case *NoSecretFound:
		return &SecretsProviderError{ResponseCode: 404, Response: RespToString(e.Response), Err: err}
	case *Unknown:
		return &SecretsProviderError{ResponseCode: e.ResponseCode, Response: RespToString(e.Response), Err: err}
	default:
		return nil
	}
}

// ----- ERRORS -----

// PermissionDenied - 403
// the user provided correct credentials but does not have permission to access the resource
type PermissionDenied struct {
	Response     map[string][]string
	HttpMethod   string
	SecretPath   string
	ExchangeUser string
}

func (e *PermissionDenied) Error() string {
	return fmt.Sprintf("Permission denied, user \"%s\" does not have %s access to %s.", e.ExchangeUser, e.HttpMethod, e.SecretPath)
}

// Unauthenticated - 401
// the user provided wrong credentials, was not able to authenticate with the exchange
type Unauthenticated struct {
	LoginError   error
	ExchangeUser string
}

func (e *Unauthenticated) Error() string {
	return fmt.Sprintf("Unable to authenticate user \"%s\" with exchange: \"%s\".", e.ExchangeUser, e.LoginError.Error())
}

// SecretsProviderUnavailable - 503
// the secrets manager service is unavailable
type SecretsProviderUnavailable struct {
	ProviderError error
}

func (e *SecretsProviderUnavailable) Error() string {
	return fmt.Sprintf("Secrets manager service unavailable: \"%s\"", e.ProviderError.Error())
}

// InvalidResponse - 500
// the plugin was unable to read or parse the secret manager's response
type InvalidResponse struct {
	ReadError  error
	ParseError error
	Response   []byte
	HttpMethod string
	SecretPath string
}

func (e *InvalidResponse) Error() string {
	if e.ReadError != nil {
		return fmt.Sprintf("Unable to read the secrets manager response to \"%s %s\": \"%s\"", e.HttpMethod, e.SecretPath, e.ReadError.Error())
	} else {
		// e.ParseError != nil
		resp := fmt.Sprintf("Unable to parse the secrets manager response to \"%s %s\": \"%s\"", e.HttpMethod, e.SecretPath, e.ParseError.Error())
		resp += fmt.Sprintf("\nResponse body: \"%s\"", string(e.Response))
		return resp
	}
}

// BadRequest - 400, 405
// the secrets manager cannot handle the request because it is malformed/not handled
type BadRequest struct {
	ResponseCode int
	Response     map[string][]string
	HttpMethod   string
	SecretPath   string
	RequestBody  *SecretDetails
}

func (e *BadRequest) Error() string {
	if e.ResponseCode == 400 {
		resp := fmt.Sprintf("Bad request: \"%s %s\"", e.HttpMethod, e.SecretPath)
		if e.RequestBody != nil {
			jsonBytes, err := json.MarshalIndent(e.RequestBody, "", cliutils.JSON_INDENT)
			if err == nil {
				resp += fmt.Sprintf("\nRequest body: \"%s\"", jsonBytes)
			}
		}
		return resp
	} else {
		// e.ResponseCode == 405
		return fmt.Sprintf("Bad request, HTTP method not supported: \"%s %s\"", e.HttpMethod, e.SecretPath)
	}
}

// NoSecretFound - 404
// when listing secrets, no secrets were found, or when requesting a specific secret, the secret was not found
type NoSecretFound struct {
	Response   map[string][]string
	SecretPath string
}

func (e *NoSecretFound) Error() string {
	return fmt.Sprintf("No secret(s) found at %s.", e.SecretPath)
}

// Unknown
// error returned by the secrets manager is unexpected and unknown
type Unknown struct {
	Response     map[string][]string
	ResponseCode int
	HttpMethod   string
	SecretPath   string
}

func (e *Unknown) Error() string {
	// format the secrets manager response
	response := RespToString(e.Response)

	// return the error message
	return fmt.Sprintf("Unknown error occurred. Request: \"%s %s \"\nResponse Code: %d\nSecrets manager response: \"%s\"", e.HttpMethod, e.SecretPath, e.ResponseCode, response)
}

// ----- HELPER FUNCTIONS -----
func RespToString(response map[string][]string) string {
	jsonBytes, err := json.MarshalIndent(response, "", cliutils.JSON_INDENT)
	var respString string
	if err != nil {
		respString = fmt.Sprintf("%v", response)
	} else {
		respString = fmt.Sprintf("%s", jsonBytes)
	}
	return respString
}
