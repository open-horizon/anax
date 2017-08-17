package policy

import (
	"errors"
	"fmt"
	"github.com/golang/glog"
	"strconv"
	"strings"
)

// Syntax of the input expression is based on OSGI version ranges.
// [<left-spec>] <version> [, <version> <right-spec>]
//
// The square brackets in the above schema indicate optionality. This is not to be
// confused with the use of square brackets in the version range expression itself
// as described below.
//
// <left-spec> if specified is one of:
// '(' following version is excluded from the range
// '[' following version is included in the range
//
// <version> is a string of x or x.y or x.y.z
//
// <right-spec> if specified is one of:
// ')' previous version is excluded from the range
// ']' previous version is included in the range
//
// e.g. [1.2.3, 4.5.6) means 1.2.3 <= a <  4.5.6, where a is valid version in the
// version range.
//
// There is also a special case in this schema for indicating greater than or equal to.
// e.g. by simply specifying a valid version string, the expression is equivalent to
// specifying [x.y.z, INFINITY) which is also expressed as:
// x.y.z <= a
//

const leftEx = "("
const leftInc = "["
const rightEx = ")"
const rightInc = "]"
const INF = "INFINITY"
const versionSeperator = ","
const numberSeperator = "."

type Version_Expression struct {
	full_expression string
	start           string
	start_inclusive bool
	end             string
	end_inclusive   bool
}

func Version_Expression_Factory(ver_string string) (*Version_Expression, error) {

	startVersion := ""
	endVersion := ""
	expr := ver_string
	if strings.Contains(expr, " ") {
		errorString := fmt.Sprintf("Version_Expression: Whitespace is not permitted in %v.", expr)
		// glog.Errorf(errorString)
		return nil, errors.New(errorString)
	}

	if singleVersion(ver_string) {
		if !IsVersionString(ver_string) {
			errorString := fmt.Sprintf("Version_Expression: %v is not a valid version string.", ver_string)
			// glog.Errorf(errorString)
			return nil, errors.New(errorString)
		}
		expr = "[" + ver_string + "," + INF + ")"
		glog.V(5).Infof("Version_Expression: Detected single version input, converted to %v", expr)
	}

	versions := expr
	if leftIncluded(versions) || leftExcluded(versions) {
		versions = versions[1:]
	} else {
		errorString := fmt.Sprintf("Version_Expression: %v does not begin with an inclusion or exclusion directive.", ver_string)
		// glog.Errorf(errorString)
		return nil, errors.New(errorString)
	}

	if rightIncluded(versions) || rightExcluded(versions) {
		versions = versions[:len(versions)-1]
	} else {
		errorString := fmt.Sprintf("Version_Expression: %v does not end with an inclusion or exclusion directive.", ver_string)
		// glog.Errorf(errorString)
		return nil, errors.New(errorString)
	}

	vers := strings.Split(versions, versionSeperator)
	if len(vers) != 2 || vers[0] == "" || vers[1] == "" {
		errorString := fmt.Sprintf("Version_Expression: Incorrect number of versions in expression: %v.", expr)
		// glog.Errorf(errorString)
		return nil, errors.New(errorString)
	}

	glog.V(5).Infof("Version_Expression: Seperated expression into %v %v", vers[0], vers[1])

	if !IsVersionString(vers[0]) {
		errorString := fmt.Sprintf("Version_Expression: %v is not a valid version string.", vers[0])
		// glog.Errorf(errorString)
		return nil, errors.New(errorString)
	} else {
		startVersion = normalize(vers[0])
	}

	if !IsVersionString(vers[1]) {
		errorString := fmt.Sprintf("Version_Expression: %v is not a valid version string.", vers[1])
		// glog.Errorf(errorString)
		return nil, errors.New(errorString)
	} else {
		endVersion = normalize(vers[1])
	}

	ve := &Version_Expression{
		full_expression: expr,
		start:           startVersion,
		start_inclusive: leftIncluded(expr),
		end:             endVersion,
		end_inclusive:   rightIncluded(expr),
	}

	glog.V(4).Infof("Version_Expression: Created %v from %v", ve, expr)

	return ve, nil
}

// Return the version expression that was used as input to create this object
//
func (self *Version_Expression) Get_expression() string {
	return self.full_expression
}

// Return true if the input version string in a valid version string and
// if it falls within the boundaries of this object's version range.
//
func (self *Version_Expression) Is_within_range(expr string) (bool, error) {
	if !IsVersionString(expr) {
		errorString := fmt.Sprintf("Version_Expression: %v is not a valid version string.", expr)
		// glog.Errorf(errorString)
		return false, errors.New(errorString)
	}

	normalizedExpr := normalize(expr)

	// Exit early in the easy cases
	if (normalizedExpr == self.start && self.start_inclusive) || (normalizedExpr == self.end && self.end_inclusive) {
		return true, nil
	} else if (normalizedExpr == self.start && !self.start_inclusive) || (normalizedExpr == self.end && !self.end_inclusive) {
		return false, nil
	}

	// Compare the start version to see if the input is in this object's range
	exprNums := strings.Split(normalizedExpr, numberSeperator)
	startNums := strings.Split(self.start, numberSeperator)
	for idx, startVal := range startNums {
		startInt, _ := strconv.Atoi(startVal)
		exprInt, _ := strconv.Atoi(exprNums[idx])
		if startInt == exprInt {
			continue
		} else if startInt < exprInt {
			break
		} else {
			return false, nil
		}
	}

	// Compare the start version to see if the input is in this object's range
	endNums := strings.Split(self.end, numberSeperator)
	for idx, endVal := range endNums {
		endInt, _ := strconv.Atoi(endVal)
		exprInt, _ := strconv.Atoi(exprNums[idx])
		if endInt == exprInt {
			continue
		} else if endInt > exprInt {
			return true, nil
		} else {
			return false, nil
		}
	}

	// Should never get here
	errorString := fmt.Sprintf("Version_Expression: Unable to compare versions %v %v.", expr, self)
	// glog.Errorf(errorString)
	return false, errors.New(errorString)
}

// ================================================================================================
// Utility functions

// Return true if the input string has no inclusive or exclusive operators on either end and no commas in the middle.
// The characters might not make up a valid version string, but at least we know the input is not an attempt to create
// a range of versions.
func singleVersion(expr string) bool {
	return !strings.Contains(leftEx+leftInc, string(expr[0])) && !strings.Contains(expr, versionSeperator) && !strings.Contains(rightEx+rightInc, expr[len(expr)-1:])
}

// Return true if the input version expression is using the inclusive operator on the left side.
func leftIncluded(expr string) bool {
	return expr[0] == leftInc[0]
}

// Return true if the input version expression is using the exclusive operator on the left side.
func leftExcluded(expr string) bool {
	return expr[0] == leftEx[0]
}

// Return true if the input version expression is using the inclusive operator on the right side.
func rightIncluded(expr string) bool {
	return expr[len(expr)-1:] == rightInc
}

// Return true if the input version expression is using the exclusive operator on the right side.
func rightExcluded(expr string) bool {
	return expr[len(expr)-1:] == rightEx
}

// Return true if the input version expression might be an attempt at a multiple version expression.
// At least we know it has a comma in the middle.
func multipleVersions(expr string) bool {
	return strings.Contains(expr, versionSeperator)
}

// Return true if the input version string is a valid version according to the version string schema above.
func IsVersionString(expr string) bool {
	if expr == INF {
		return true
	}

	nums := strings.Split(expr, numberSeperator)
	if len(nums) == 0 || len(nums) > 3 {
		return false
	} else {
		for _, val := range nums {
			if val == "" {
				return false
			}
			for _, val2 := range val {
				if !strings.Contains("0123456789", string(val2)) {
					return false
				}
			}
		}
		return true
	}
}

// Return a normalized version string containing all 3 version numbers. The input version string is ASSUMED to
// be a valid version string. For example, an input version string of 1 will be returned as 1.0.0
func normalize(expr string) string {
	result := expr
	nums := strings.Split(expr, numberSeperator)
	if len(nums) < 3 {
		result += strings.Repeat(".0", 3-len(nums))
	}
	return result
}
