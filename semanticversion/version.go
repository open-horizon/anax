package semanticversion

import (
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/i18n"
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
const preReleaseSeperator = "-"

type Version_Expression struct {
	full_expression string
	start           string
	start_inclusive bool
	end             string
	end_inclusive   bool
}

func (ve Version_Expression) String() string {
	return fmt.Sprintf("Vers Exp: %v", ve.full_expression)
}

func Version_Expression_Factory(ver_string string) (*Version_Expression, error) {

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if len(ver_string) == 0 {
		errorString := msgPrinter.Sprintf("Version_Expression: Empty string is not a valid version.")
		return nil, errors.New(errorString)
	}

	startVersion := ""
	endVersion := ""
	expr := ver_string
	if strings.Contains(expr, " ") {
		errorString := msgPrinter.Sprintf("Version_Expression: Whitespace is not permitted in %v.", expr)
		return nil, errors.New(errorString)
	}

	if singleVersion(ver_string) {
		if !IsVersionString(ver_string) {
			errorString := msgPrinter.Sprintf("Version_Expression: %v is not a valid version string.", ver_string)
			return nil, errors.New(errorString)
		}
		expr = "[" + ver_string + "," + INF + ")"
		glog.V(6).Infof("Version_Expression: Detected single version input, converted to %v", expr)
	}

	versions := expr
	if leftIncluded(versions) || leftExcluded(versions) {
		versions = versions[1:]
	} else {
		errorString := msgPrinter.Sprintf("Version_Expression: %v does not begin with an inclusion or exclusion directive.", ver_string)
		return nil, errors.New(errorString)
	}

	if rightIncluded(versions) || rightExcluded(versions) {
		versions = versions[:len(versions)-1]
	} else {
		errorString := msgPrinter.Sprintf("Version_Expression: %v does not end with an inclusion or exclusion directive.", ver_string)
		return nil, errors.New(errorString)
	}

	vers := strings.Split(versions, versionSeperator)
	if len(vers) != 2 || vers[0] == "" || vers[1] == "" {
		errorString := msgPrinter.Sprintf("Version_Expression: Incorrect number of versions in expression: %v.", expr)
		return nil, errors.New(errorString)
	}

	glog.V(6).Infof("Version_Expression: Seperated expression into %v %v", vers[0], vers[1])

	if !IsVersionString(vers[0]) {
		errorString := msgPrinter.Sprintf("Version_Expression: %v is not a valid version string.", vers[0])
		return nil, errors.New(errorString)
	} else {
		startVersion = normalize(vers[0])
	}

	if !IsVersionString(vers[1]) {
		errorString := msgPrinter.Sprintf("Version_Expression: %v is not a valid version string.", vers[1])
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

	// nomalize the versions in the expression
	ve.recalc_expression()

	glog.V(6).Infof("Version_Expression: Created %v from %v", ve, expr)

	return ve, nil
}

// Re caculate the full expression for this version range
func (self *Version_Expression) recalc_expression() {
	expr := ""

	if self.start_inclusive {
		expr = leftInc
	} else {
		expr = leftEx
	}

	expr = expr + normalize(self.start) + versionSeperator + normalize(self.end)

	if self.end_inclusive {
		expr = expr + rightInc
	} else {
		expr = expr + rightEx
	}

	self.full_expression = expr
}

// Return the version expression that was used as input to create this object
//
func (self *Version_Expression) Get_expression() string {
	return self.full_expression
}

// Return the start version
//
func (self *Version_Expression) Get_start_version() string {
	return self.start
}

// Return the end version
//
func (self *Version_Expression) Get_end_version() string {
	return self.end
}

// Return true if the input version string in a valid version string and
// if it falls within the boundaries of this object's version range.
//
func (self *Version_Expression) Is_within_range(expr string) (bool, error) {
	if !IsVersionString(expr) {
		errorString := fmt.Sprintf("Version_Expression: %v is not a valid version string.", expr)
		return false, errors.New(errorString)
	}

	normalizedExpr := normalize(expr)

	// Exit early in the easy cases
	if (normalizedExpr == self.start && self.start_inclusive) || (normalizedExpr == self.end && self.end_inclusive) {
		return true, nil
	} else if (normalizedExpr == self.start && !self.start_inclusive) || (normalizedExpr == self.end && !self.end_inclusive) {
		return false, nil
	}

	if c, err := CompareVersions(self.start, normalizedExpr); err != nil {
		return false, err
	} else if c > 1 {
		return false, nil
	}

	// Compare the end version to see if the input is in this object's range. An end range of
	// "INFINITY" will always be in range.
	if self.end == INF {
		return true, nil
	}

	if c, err := CompareVersions(self.end, normalizedExpr); err != nil {
		return false, err
	} else if c < 1 {
		return false, nil
	}

	return true, nil
}

// make this version equals to the intersection of self and the given version
func (self *Version_Expression) IntersectsWith(other *Version_Expression) error {

	// compare the start part
	if c, err := CompareVersions(self.start, other.start); err != nil {
		return err
	} else if c == 0 {
		if self.start_inclusive != other.start_inclusive {
			self.start_inclusive = false
		}
	} else if c == -1 {
		self.start = other.start
		self.start_inclusive = other.start_inclusive
	}

	// compare the end part
	if c, err := CompareVersions(self.end, other.end); err != nil {
		return err
	} else if c == 0 {
		if self.end_inclusive != other.end_inclusive {
			self.end_inclusive = false
		}
	} else if self.end == INF || c == 1 {
		self.end = other.end
		self.end_inclusive = other.end_inclusive
	}

	// make sure start is smaller or equal to the end
	if self.end != INF {
		if c, err := CompareVersions(self.start, self.end); err != nil {
			return err
		} else if c == 0 {
			if !self.start_inclusive && !self.end_inclusive {
				return fmt.Errorf("No intersection found.")
			}
		} else if c == 1 {
			return fmt.Errorf("No intersection found.")
		}
	}

	self.recalc_expression()

	return nil
}

// change the ceiling of this version range.
func (self *Version_Expression) ChangeCeiling(ceiling_version string, inclusive bool) error {

	if len(ceiling_version) == 0 {
		return fmt.Errorf("The input string is not a version string, it is an empty string.")
	}

	if ceiling_version == INF {
		self.end = INF
		// always set the false, ignore the inclusive input
		self.end_inclusive = false
	} else if !IsVersionString(ceiling_version) {
		return fmt.Errorf("The input string %v is not a version string.", ceiling_version)
	} else {

		if c, err := CompareVersions(ceiling_version, self.start); err != nil {
			return err
		} else if c < 0 {
			return fmt.Errorf("The input ceiling version %v is lower than the start version %v.", ceiling_version, self.start)
		} else if c == 0 {
			if !(inclusive && self.start_inclusive) {
				return fmt.Errorf("The input ceiling version %v is the same as the start version, but either the start or the end is not inclusive.", ceiling_version)
			}
		}

		self.end = ceiling_version
		self.end_inclusive = inclusive
	}

	self.recalc_expression()

	return nil
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
// A number with leading 0's, for example 1.02.1, is not a valid version string.
func IsVersionString(expr string) bool {

	if len(expr) == 0 {
		return false
	}

	if expr == INF {
		return true
	}

	splitExpr := strings.Split(expr, preReleaseSeperator)
	prerelease := strings.Join(splitExpr[1:], preReleaseSeperator)
	nums := strings.Split(splitExpr[0], numberSeperator)

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
	}
	for _, val := range prerelease {
		if !strings.Contains("0123456789abcdefghijklmnopqrstuvwxyz-", strings.ToLower(string(val))) {
			return false
		}
	}
	return true
}

// Return true if the input version string is a full version expression
func IsVersionExpression(expr string) bool {

	if len(expr) == 0 {
		return false
	}

	if !(leftIncluded(expr) || leftExcluded(expr)) && !(rightIncluded(expr) || rightExcluded(expr)) {
		return false
	}

	vers := strings.Split(expr, versionSeperator)
	if len(vers) != 2 || vers[0] == "" || vers[1] == "" {
		return false
	}

	glog.V(6).Infof("Version_Expression: Seperated expression into %v %v", vers[0], vers[1])

	if !IsVersionString(vers[0][1:]) {
		return false
	}

	if !IsVersionString(vers[1][:len(vers[1])-1]) {
		return false
	}

	return true
}

// Return a normalized version string containing all 3 version numbers. The input version string is ASSUMED to
// be a valid version string. For example, an input version string of 1 will be returned as 1.0.0
func normalize(expr string) string {
	if expr == INF {
		return expr
	}
	preRelease := strings.Join(strings.Split(expr, preReleaseSeperator)[1:], preReleaseSeperator)
	result := strings.Split(expr, preReleaseSeperator)[0]
	nums := strings.Split(expr, numberSeperator)
	if len(nums) < 3 {
		result += strings.Repeat(".0", 3-len(nums))
	}
	if preRelease != "" {
		result += "-" + preRelease
	}
	return result
}

// Return 1 if the input version v1 is higher than v2
//        0 if the input version v1 equals to v2
//        -1 if the input version v1 is lower than v2
//        error if v1 or v2 is no a valid sigle version string
func CompareVersions(v1 string, v2 string) (int, error) {
	// make sure it is a single version string
	if !IsVersionString(v1) || !IsVersionString(v2) {
		return 0, fmt.Errorf(i18n.GetMessagePrinter().Sprintf("Input version string %v or %v is not a valid single version string.", v1, v2))
	}

	// same version
	if strings.Compare(v1, v2) == 0 {
		return 0, nil
	}

	// check for infinity
	if v1 == INF {
		return 1, nil
	}

	if v2 == INF {
		return -1, nil
	}

	// make each has 3 fields
	v1n := normalize(v1)
	v2n := normalize(v2)

	splitExpr1 := strings.Split(v1n, preReleaseSeperator)
	splitExpr2 := strings.Split(v2n, preReleaseSeperator)

	pr1 := strings.Join(splitExpr1[1:], preReleaseSeperator)
	pr2 := strings.Join(splitExpr2[1:], preReleaseSeperator)

	// convert each field into integer and then compare
	v1s := strings.Split(splitExpr1[0], numberSeperator)
	v2s := strings.Split(splitExpr2[0], numberSeperator)

	for i := 0; i < 3; i++ {
		if v1s[i] == v2s[i] {
			continue
		}
		n1, _ := strconv.Atoi(v1s[i])
		n2, _ := strconv.Atoi(v2s[i])

		if n1 < n2 {
			return -1, nil
		} else if n1 > n2 {
			return 1, nil
		}
	}

	return strings.Compare(pr1, pr2), nil
}
