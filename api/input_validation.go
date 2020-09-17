package api

import (
	"fmt"
	"regexp"
)

// \pL -- unicode letter
// \pN -- unicode number
var IllegalInputCharRegex = `[^-*+()?&! _\w\d.@,:/\\\pL\pN]`

func InputIsIllegal(str string) (string, error) {
	reg, err := regexp.Compile(IllegalInputCharRegex)
	if err != nil {
		return "", fmt.Errorf("Unable to compile regex: %v, returning false for input check. Error: %v", IllegalInputCharRegex, err)
	}

	if reg.MatchString(str) {
		return fmt.Sprintf("Value violates regex illegal char match: %v", IllegalInputCharRegex), nil
	}

	// a-ok!
	return "", nil
}

// returns: faulty value, msg, error
func MapInputIsIllegal(m map[string]interface{}) (string, string, error) {
	for k, _ := range m {
		if bogus, err := InputIsIllegal(k); err != nil || bogus != "" {
			return k, bogus, err
		}
		// disable checking for input values for now
		//switch v.(type) {
		//case string:
		//	if bogus, err := InputIsIllegal(v.(string)); err != nil || bogus != "" {
		//		return fmt.Sprintf("%v: %v", k, v), bogus, err
		//	}
		//}
	}

	// all good
	return "", "", nil
}

// Verify that the input string value is valid according to the list of supported characters. if there
// is an error, this function returns true to indicate that there was an error.
func checkInputString(errorHandler ErrorHandler, fieldId string, input *string) bool {
	nErrMsg := "null and must not be"

	// if true, bail
	if input == nil {
		return errorHandler(NewAPIUserInputError(nErrMsg, fieldId))
	}
	inputErr, err := InputIsIllegal(*input)
	if err != nil {
		return errorHandler(NewSystemError(fmt.Sprintf("Failed to check input: %v", err)))
	}

	if inputErr != "" {
		return errorHandler(NewAPIUserInputError(inputErr, fieldId))
	}

	return false
}
