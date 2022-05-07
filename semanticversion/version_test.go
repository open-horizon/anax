//go:build unit
// +build unit

package semanticversion

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

// This series of tests verifies that constructor works correctly, by handling invalid input
// correctly.
func TestConstructor(t *testing.T) {
	if c, err := Version_Expression_Factory("1.2.3"); c == nil {
		t.Errorf("Factory returned nil, but should not. Error: %v \n", err)
	} else if c.Get_expression() != "[1.2.3,INFINITY)" {
		t.Errorf("Factory did not correctly save the full expression, returned %v\n", c.Get_expression())
	} else if c, err := Version_Expression_Factory(""); c != nil {
		t.Errorf("Factory did not return nil but should have, it returned %v Error: %v \n", c, err)
	} else if c, err := Version_Expression_Factory("a"); c != nil {
		t.Errorf("Factory did not return nil but should have, it returned %v Error: %v \n", c, err)
	} else if c, err := Version_Expression_Factory("1a"); c != nil {
		t.Errorf("Factory did not return nil but should have, it returned %v Error: %v \n", c, err)
	} else if c, err := Version_Expression_Factory("1.2.a"); c != nil {
		t.Errorf("Factory did not return nil but should have, it returned %v Error: %v \n", c, err)
	} else if c, err := Version_Expression_Factory("[1.2.3"); c != nil {
		t.Errorf("Factory did not return nil but should have, it returned %v Error: %v \n", c, err)
	} else if c, err := Version_Expression_Factory("[1.2.3]"); c != nil {
		t.Errorf("Factory did not return nil but should have, it returned %v Error: %v \n", c, err)
	} else if c, err := Version_Expression_Factory("(1.2.3"); c != nil {
		t.Errorf("Factory did not return nil but should have, it returned %v Error: %v \n", c, err)
	} else if c, err := Version_Expression_Factory("(1.2.3)"); c != nil {
		t.Errorf("Factory did not return nil but should have, it returned %v Error: %v \n", c, err)
	} else if c, err := Version_Expression_Factory("1a.2.3"); c != nil {
		t.Errorf("Factory did not return nil but should have, it returned %v Error: %v \n", c, err)
	} else if c, err := Version_Expression_Factory("1.2.3.4"); c != nil {
		t.Errorf("Factory did not return nil but should have, it returned %v Error: %v \n", c, err)
	} else if c, err := Version_Expression_Factory("1..2..3..4"); c != nil {
		t.Errorf("Factory did not return nil but should have, it returned %v Error: %v \n", c, err)
	} else if c, err := Version_Expression_Factory("1.2.3."); c != nil {
		t.Errorf("Factory did not return nil but should have, it returned %v Error: %v \n", c, err)
	} else if c, err := Version_Expression_Factory("1.2.3,"); c != nil {
		t.Errorf("Factory did not return nil but should have, it returned %v Error: %v \n", c, err)
	} else if c, err := Version_Expression_Factory("1,2a"); c != nil {
		t.Errorf("Factory did not return nil but should have, it returned %v Error: %v \n", c, err)
	} else if c, err := Version_Expression_Factory("1,1.2.3.a"); c != nil {
		t.Errorf("Factory did not return nil but should have, it returned %v Error: %v \n", c, err)
	} else if c, err := Version_Expression_Factory("1,1.2..3"); c != nil {
		t.Errorf("Factory did not return nil but should have, it returned %v Error: %v \n", c, err)
	} else if c, err := Version_Expression_Factory("1,1a.2.3"); c != nil {
		t.Errorf("Factory did not return nil but should have, it returned %v Error: %v \n", c, err)
	} else if c, err := Version_Expression_Factory(".1,.2"); c != nil {
		t.Errorf("Factory did not return nil but should have, it returned %v Error: %v \n", c, err)
	} else if c, err := Version_Expression_Factory("1.2.3, 1.2.3"); c != nil {
		t.Errorf("Factory did not return nil but should have, it returned %v Error: %v \n", c, err)
	} else if c, err := Version_Expression_Factory("[1.2,3.4)a"); c != nil {
		t.Errorf("Factory did not return nil but should have, it returned %v Error: %v \n", c, err)
	} else if c, err := Version_Expression_Factory("[1.2,3.4a)"); c != nil {
		t.Errorf("Factory did not return nil but should have, it returned %v Error: %v \n", c, err)
	} else if c, err := Version_Expression_Factory("[1.2.3.4,3.4)"); c != nil {
		t.Errorf("Factory did not return nil but should have, it returned %v Error: %v \n", c, err)
	} else if c, err := Version_Expression_Factory("(1.2,3..4]"); c != nil {
		t.Errorf("Factory did not return nil but should have, it returned %v Error: %v \n", c, err)
	} else if c, err := Version_Expression_Factory("[(1.2,3.4)"); c != nil {
		t.Errorf("Factory did not return nil but should have, it returned %v Error: %v \n", c, err)
	} else if c, err := Version_Expression_Factory("[1.2,3.4))"); c != nil {
		t.Errorf("Factory did not return nil but should have, it returned %v Error: %v \n", c, err)
	} else if c, err := Version_Expression_Factory("[1.2,3.4,5.6)"); c != nil {
		t.Errorf("Factory did not return nil but should have, it returned %v Error: %v \n", c, err)
	} else if c, err := Version_Expression_Factory("1.2,3.4"); c != nil {
		t.Errorf("Factory did not return nil but should have, it returned %v Error: %v \n", c, err)
	} else if c, err := Version_Expression_Factory("[1.2,3.4"); c != nil {
		t.Errorf("Factory did not return nil but should have, it returned %v Error: %v \n", c, err)
	}
}

// This series of tests verifies that constructor works correctly, by handling valid input
// correctly.
func TestPositive(t *testing.T) {
	if c, err := Version_Expression_Factory("1"); c == nil {
		t.Errorf("Factory returned nil, but should not. Error: %v \n", err)
	} else if c, err := Version_Expression_Factory("1.1"); c == nil {
		t.Errorf("Factory returned nil, but should not. Error: %v \n", err)
	} else if c, err := Version_Expression_Factory("1.1.1"); c == nil {
		t.Errorf("Factory returned nil, but should not. Error: %v \n", err)
	} else if c, err := Version_Expression_Factory("[1,2]"); c == nil {
		t.Errorf("Factory returned nil, but should not. Error: %v \n", err)
	} else if c, err := Version_Expression_Factory("[1.1,2.2]"); c == nil {
		t.Errorf("Factory returned nil, but should not. Error: %v \n", err)
	} else if c, err := Version_Expression_Factory("[1.1.1,2.2.2]"); c == nil {
		t.Errorf("Factory returned nil, but should not. Error: %v \n", err)
	} else if c, err := Version_Expression_Factory("(1,2)"); c == nil {
		t.Errorf("Factory returned nil, but should not. Error: %v \n", err)
	} else if c, err := Version_Expression_Factory("(1.1,2.2)"); c == nil {
		t.Errorf("Factory returned nil, but should not. Error: %v \n", err)
	} else if c, err := Version_Expression_Factory("(1.1.1,2.2.2)"); c == nil {
		t.Errorf("Factory returned nil, but should not. Error: %v \n", err)
	} else if c, err := Version_Expression_Factory("[1,2)"); c == nil {
		t.Errorf("Factory returned nil, but should not. Error: %v \n", err)
	} else if c, err := Version_Expression_Factory("[1.1,2.2)"); c == nil {
		t.Errorf("Factory returned nil, but should not. Error: %v \n", err)
	} else if c, err := Version_Expression_Factory("[1.1.1,2.2.2)"); c == nil {
		t.Errorf("Factory returned nil, but should not. Error: %v \n", err)
	} else if c, err := Version_Expression_Factory("(1,2]"); c == nil {
		t.Errorf("Factory returned nil, but should not. Error: %v \n", err)
	} else if c, err := Version_Expression_Factory("(1.1,2.2]"); c == nil {
		t.Errorf("Factory returned nil, but should not. Error: %v \n", err)
	} else if c, err := Version_Expression_Factory("(1.1.1,2.2.2]"); c == nil {
		t.Errorf("Factory returned nil, but should not. Error: %v \n", err)
	} else if c, err := Version_Expression_Factory("(1.1-alpha,2.2-beta]"); c == nil {
		t.Errorf("Factory returned nil, but should not. Error: %v \n", err)
	} else if c, err := Version_Expression_Factory("(1.1-a-b,2]"); c == nil {
		t.Errorf("Factory returned nil, but should not. Error: %v \n", err)
	} else if c, err := Version_Expression_Factory("(1.1-a-b,2.01.2]"); c == nil {
		t.Errorf("Factory returned nil, but should not. Error: %v \n", err)
	} else if c, err := Version_Expression_Factory("(1.02,2]"); c == nil {
		t.Errorf("Factory returned nil, but should not. Error: %v \n", err)
	}
}

// This series of tests verifies that Is_within_range correctly detects that a version
// expression is within a specific range that is inclusive on both ends, or not. It also
// handles invalid inputs to the range test.
func TestRanges1(t *testing.T) {
	if c, err := Version_Expression_Factory("[1,2]"); c == nil {
		t.Errorf("Factory returned nil, but should not. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("2"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.0"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("2.0"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.0.0"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("2.0.0"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.a"); err == nil || inrange {
		t.Errorf("Input is in error, should have returned an error.\n")
	} else if inrange, err := c.Is_within_range("2.a"); err == nil || inrange {
		t.Errorf("Input is in error, should have returned an error.\n")
	} else if inrange, err := c.Is_within_range("1.1"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.9"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.0.1"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.1.1"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.1.01"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.09.0"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("2.1"); err != nil || inrange {
		t.Errorf("Input is NOT in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("2.01"); err != nil || inrange {
		t.Errorf("Input is NOT in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range(""); err == nil || inrange {
		t.Errorf("Input is invalid, but did not return an error\n")
	}

}

// This series of tests verifies that Is_within_range correctly detects that a version
// expression is within a specific range that is inclusive on one end and exclusive on
// the other end, or not. It also handles invalid inputs to the range test.
func TestRanges2(t *testing.T) {
	if c, err := Version_Expression_Factory("[1,2)"); c == nil {
		t.Errorf("Factory returned nil, but should not. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("2"); err != nil || inrange {
		t.Errorf("Input is NOT in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.0"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("2.0"); err != nil || inrange {
		t.Errorf("Input is NOT in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.0.0"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("2.0.0"); err != nil || inrange {
		t.Errorf("Input is NOT in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.a"); err == nil || inrange {
		t.Errorf("Input is in error, should have returned an error.\n")
	} else if inrange, err := c.Is_within_range("2.a"); err == nil || inrange {
		t.Errorf("Input is in error, should have returned an error.\n")
	} else if inrange, err := c.Is_within_range("1.2a"); err == nil || inrange {
		t.Errorf("Input is in error, should have returned an error.\n")
	} else if inrange, err := c.Is_within_range("1.1"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.9"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.0.1"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.1.1"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.01.90-z"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.1.1-custom-tag"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("2.1"); err != nil || inrange {
		t.Errorf("Input is NOT in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("2.1-a"); err != nil || inrange {
		t.Errorf("Input is NOT in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("0.0.1-beta"); err != nil || inrange {
		t.Errorf("Input is NOT in range. Error: %v \n", err)
	}

}

// This series of tests verifies that Is_within_range correctly detects that a version
// expression is within a specific range that is inclusive on one end and exclusive on
// the other end (opposite of the previous test series), or not. It also handles invalid
// inputs to the range test.
func TestRanges3(t *testing.T) {
	if c, err := Version_Expression_Factory("(1,2]"); c == nil {
		t.Errorf("Factory returned nil, but should not. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1"); err != nil || inrange {
		t.Errorf("Input is NOT in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("2"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.0"); err != nil || inrange {
		t.Errorf("Input is NOT in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("2.0"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.0.0"); err != nil || inrange {
		t.Errorf("Input is NOT in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("2.0.0"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.a"); err == nil || inrange {
		t.Errorf("Input is in error, should have returned an error.\n")
	} else if inrange, err := c.Is_within_range("2.a"); err == nil || inrange {
		t.Errorf("Input is in error, should have returned an error.\n")
	} else if inrange, err := c.Is_within_range("1.1"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.9"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.0.1"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.1.1"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.1.1-beta4"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("2.1"); err != nil || inrange {
		t.Errorf("Input is NOT in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("2.1-tag"); err != nil || inrange {
		t.Errorf("Input is NOT in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("0.0.01"); err != nil || inrange {
		t.Errorf("Input is NOT in range. Error: %v \n", err)
	}

}

// This series of tests verifies that Is_within_range correctly detects that a version
// expression is within a specific range that is exclusive on both ends, or not. It also
// handles invalid inputs to the range test.
func TestRanges4(t *testing.T) {
	if c, err := Version_Expression_Factory("(1,2)"); c == nil {
		t.Errorf("Factory returned nil, but should not. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1"); err != nil || inrange {
		t.Errorf("Input is NOT in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("2"); err != nil || inrange {
		t.Errorf("Input is NOT in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.0"); err != nil || inrange {
		t.Errorf("Input is NOT in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("2.0"); err != nil || inrange {
		t.Errorf("Input is NOT in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.0.0"); err != nil || inrange {
		t.Errorf("Input is NOT in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("2.0.0"); err != nil || inrange {
		t.Errorf("Input is NOT in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("2.0.0-beta"); err != nil || inrange {
		t.Errorf("Input is NOT in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.a"); err == nil || inrange {
		t.Errorf("Input is in error, should have returned an error.\n")
	} else if inrange, err := c.Is_within_range("2.a"); err == nil || inrange {
		t.Errorf("Input is in error, should have returned an error.\n")
	} else if inrange, err := c.Is_within_range("1.1"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.9"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.0.1"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.1.1"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.1.1-prerelease"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("2.1"); err != nil || inrange {
		t.Errorf("Input is NOT in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("0.0.1"); err != nil || inrange {
		t.Errorf("Input is NOT in range. Error: %v \n", err)
	}

}

// This series of tests verifies that Is_within_range correctly detects that a version
// expression is within a version range that includes INFINITY.
func TestRanges5(t *testing.T) {
	if c, err := Version_Expression_Factory("[1,INFINITY)"); c == nil {
		t.Errorf("Factory returned nil, but should not. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.5.0"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("2.5.0"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.5.1"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.46.71-alpha"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("0.03.1"); err != nil || inrange {
		t.Errorf("Input is NOT in range. Error: %v \n", err)
	}

}

// This series of tests verifies that IsVersionExpression correctly detects that a version
// expression is valid.
func TestVersionExpressionSuccess(t *testing.T) {
	if exp := IsVersionExpression("[1.2.3,INFINITY)"); !exp {
		t.Errorf("Input is a version expression\n")
	} else if exp := IsVersionExpression("[1.2.3,4.5.6)"); !exp {
		t.Errorf("Input is a version expression\n")
	} else if exp := IsVersionExpression("[1.2.3,4.5.6]"); !exp {
		t.Errorf("Input is a version expression\n")
	} else if exp := IsVersionExpression("(1.2.3,INFINITY)"); !exp {
		t.Errorf("Input is a version expression\n")
	} else if exp := IsVersionExpression("(1.2.3,4.5.6]"); !exp {
		t.Errorf("Input is a version expression\n")
	} else if exp := IsVersionExpression("(1.2.0-a,1.2.0-z]"); !exp {
		t.Errorf("Input is a version expression\n")
	} else if exp := IsVersionExpression("(1.2-test,1.2-release]"); !exp {
		t.Errorf("Input is a version expression\n")
	} else if exp := IsVersionExpression("(1.0,1.2-beta]"); !exp {
		t.Errorf("Input is a version expression\n")
	}
}

// This series of tests verifies that IsVersionExpression correctly detects that a version
// expression is NOT valid.
func TestVersionExpressionFailure(t *testing.T) {
	if exp := IsVersionExpression("1"); exp {
		t.Errorf("Input is NOT a version expression\n")
	} else if exp := IsVersionExpression("1.2"); exp {
		t.Errorf("Input is a NOT version expression\n")
	} else if exp := IsVersionExpression("1.2.3"); exp {
		t.Errorf("Input is NOT a version expression\n")
	} else if exp := IsVersionExpression("[1.2)"); exp {
		t.Errorf("Input is NOT a version expression\n")
	} else if exp := IsVersionExpression("1,1"); exp {
		t.Errorf("Input is NOT a version expression\n")
	} else if exp := IsVersionExpression("(1"); exp {
		t.Errorf("Input is NOT a version expression\n")
	} else if exp := IsVersionExpression("(1,"); exp {
		t.Errorf("Input is NOT a version expression\n")
	} else if exp := IsVersionExpression("a"); exp {
		t.Errorf("Input is NOT a version expression\n")
	} else if exp := IsVersionExpression("[a,2]"); exp {
		t.Errorf("Input is NOT a version expression\n")
	} else if exp := IsVersionExpression("(a,b)"); exp {
		t.Errorf("Input is NOT a version expression\n")
	} else if exp := IsVersionExpression("(1.2.3,a]"); exp {
		t.Errorf("Input is NOT a version expression\n")
	} else if exp := IsVersionExpression("(1.1.1,1.2a]"); exp {
		t.Errorf("Input is NOT a version expression\n")
	} else if exp := IsVersionExpression("(1-b,a-b]"); exp {
		t.Errorf("Input is NOT a version expression\n")
	}
}

// This test tests if the version string is a valide string.
func TestIsVersionString(t *testing.T) {
	v_good := []string{"1.0", "1.2", "1.234.567", "3.0.0", "234", "1.2.03", "1.02.3", "1.2.3-abc", "1.0-abc", "1-a", "1.2.0-testing", "1-a", "1.0.0-1-a"}
	for _, v := range v_good {
		if !IsVersionString(v) {
			t.Errorf("Version string %v is valid, however the IsVersionString function returned false.\n", v)
		}
	}

	v_bad := []string{"1.0.0.1", "1.2.3a", "[1.2, 1.3]", "1.2.a", "1.-a", "1.0.1-d*s"}
	for _, v := range v_bad {
		if IsVersionString(v) {
			t.Errorf("Version string %v is invalid, however the IsVersionString function returned true.\n", v)
		}
	}
}

// This series of tests verifies recalc_expression updates the full_expression of a version range.
func TestReCalcExpression(t *testing.T) {
	v1, err := Version_Expression_Factory("[1,INFINITY)")
	if err != nil {
		t.Errorf("Factory returned nil, but should not. Error: %v \n", err)
	}
	v1.recalc_expression()
	assert.Equal(t, "[1.0.0,INFINITY)", v1.Get_expression(), "")

	// change a memeber and test
	v1.start = "2.0"
	assert.NotEqual(t, "[2.0.0,INFINITY)", v1.Get_expression(), "")

	v1.recalc_expression()
	assert.Equal(t, "[2.0.0,INFINITY)", v1.Get_expression(), "")

	v1.end = "3"
	v1.end_inclusive = true
	assert.NotEqual(t, "[2.0.0,3.0.0", v1.Get_expression(), "")

	v1.recalc_expression()
	assert.Equal(t, "[2.0.0,3.0.0]", v1.Get_expression(), "")

	// same test with pre-release tags
	v2, err := Version_Expression_Factory("[1,INFINITY)")
	if err != nil {
		t.Errorf("Factory returned nil, but should not. Error: %v \n", err)
	}
	v2.recalc_expression()
	assert.Equal(t, "[1.0.0,INFINITY)", v2.Get_expression(), "")

	v2.start = "2.0-alpha1"
	assert.NotEqual(t, "[2.0.0-alpha1,INFINITY)", v2.Get_expression(), "")

	v2.recalc_expression()
	assert.Equal(t, "[2.0.0-alpha1,INFINITY)", v2.Get_expression(), "")

	v2.end = "3-release"
	v2.end_inclusive = true
	assert.NotEqual(t, "[2.0.0-alpha1,3.0.0-release", v2.Get_expression(), "")

	v2.recalc_expression()
	assert.Equal(t, "[2.0.0-alpha1,3.0.0-release]", v2.Get_expression(), "")

}

// This series of tests verifies IntersectsWith gets the intersection between the two version ranges.
func TestIntersectsWith(t *testing.T) {
	v1, err := Version_Expression_Factory("[1,INFINITY)")
	assert.Nil(t, err, fmt.Sprintf("Factory returned nil, but should not. Error: %v \n", err))
	v2, err := Version_Expression_Factory("(2.1,INFINITY)")
	assert.Nil(t, err, fmt.Sprintf("Factory returned nil, but should not. Error: %v \n", err))
	err = v1.IntersectsWith(v2)
	assert.Nil(t, err, "Shold return no error")
	v_result, err := Version_Expression_Factory("(2.1,INFINITY)")
	assert.Nil(t, err, fmt.Sprintf("Factory returned nil, but should not. Error: %v \n", err))
	assert.Equal(t, v_result, v1, "Intersection should be [1,INFINITY).")

	v3, err := Version_Expression_Factory("[0.0,2.1]")
	assert.Nil(t, err, fmt.Sprintf("Factory returned nil, but should not. Error: %v \n", err))
	v4, err := Version_Expression_Factory("(1.0,3.1)")
	assert.Nil(t, err, fmt.Sprintf("Factory returned nil, but should not. Error: %v \n", err))
	err = v3.IntersectsWith(v4)
	assert.Nil(t, err, "Shold return no error")
	v_result, err = Version_Expression_Factory("(1.0,2.1]")
	assert.Nil(t, err, fmt.Sprintf("Factory returned nil, but should not. Error: %v \n", err))
	assert.Equal(t, v_result, v3, "Intersection should be (1.0,2.1].")

	v5, err := Version_Expression_Factory("[2.0,INFINITY)")
	assert.Nil(t, err, fmt.Sprintf("Factory returned nil, but should not. Error: %v \n", err))
	v6, err := Version_Expression_Factory("(1.0,3.1)")
	assert.Nil(t, err, fmt.Sprintf("Factory returned nil, but should not. Error: %v \n", err))
	err = v5.IntersectsWith(v6)
	assert.Nil(t, err, "Shold return no error")
	v_result, err = Version_Expression_Factory("[2.0,3.1)")
	assert.Nil(t, err, fmt.Sprintf("Factory returned nil, but should not. Error: %v \n", err))
	assert.Equal(t, v_result, v5, "Intersection should be [2.0,3.1).")

	// no intersction, should return error
	v7, err := Version_Expression_Factory("[4.0,5.0)")
	assert.Nil(t, err, fmt.Sprintf("Factory returned nil, but should not. Error: %v \n", err))
	v8, err := Version_Expression_Factory("(1.0,2.1)")
	assert.Nil(t, err, fmt.Sprintf("Factory returned nil, but should not. Error: %v \n", err))
	err = v7.IntersectsWith(v8)
	assert.NotNil(t, v7, "Should return error.")

	// no intersction, should return error
	v9, err := Version_Expression_Factory("[4.0,5.0)")
	assert.Nil(t, err, fmt.Sprintf("Factory returned nil, but should not. Error: %v \n", err))
	v10, err := Version_Expression_Factory("(5.0,6.0]")
	assert.Nil(t, err, fmt.Sprintf("Factory returned nil, but should not. Error: %v \n", err))
	err = v9.IntersectsWith(v10)
	assert.NotNil(t, v9, "Should return error.")

	// exact same versions
	v11, err := Version_Expression_Factory("2.0")
	assert.Nil(t, err, fmt.Sprintf("Factory returned nil, but should not. Error: %v \n", err))
	v12, err := Version_Expression_Factory("2.0")
	assert.Nil(t, err, fmt.Sprintf("Factory returned nil, but should not. Error: %v \n", err))
	err = v11.IntersectsWith(v12)
	assert.Nil(t, err, "Shold return no error")
	v_result, err = Version_Expression_Factory("[2.0,INFINITY)")
	assert.Nil(t, err, fmt.Sprintf("Factory returned nil, but should not. Error: %v \n", err))
	assert.Equal(t, v_result, v11, "Intersection should be [2.0,INFINITY).")

	// intersection with pre-release tags
	v13, err := Version_Expression_Factory("[0.0,2.1-release]")
	assert.Nil(t, err, fmt.Sprintf("Factory returned nil, but should not. Error: %v \n", err))
	v14, err := Version_Expression_Factory("(1.0,3.1-testing)")
	assert.Nil(t, err, fmt.Sprintf("Factory returned nil, but should not. Error: %v \n", err))
	err = v13.IntersectsWith(v14)
	assert.Nil(t, err, "Shold return no error")
	v_result, err = Version_Expression_Factory("(1.0,2.1-release]")
	assert.Nil(t, err, fmt.Sprintf("Factory returned nil, but should not. Error: %v \n", err))
	assert.Equal(t, v_result, v13, "Intersection should be (1.0,2.1-release].")
}

// This series of tests verifies the ChangeCeiling function
func TestLimitCeiling(t *testing.T) {
	v1, err := Version_Expression_Factory("[1,INFINITY)")
	assert.Nil(t, err, fmt.Sprintf("Factory returned nil, but should not. Error: %v \n", err))

	err = v1.ChangeCeiling("2.0", false)
	assert.Nil(t, err, fmt.Sprintf("ChangeCeiling returned error, but should not. Error: %v \n", err))
	assert.Equal(t, "[1.0.0,2.0.0)", v1.Get_expression(), "Version range should be [1.0.0,2.0.0)")

	err = v1.ChangeCeiling("2.0", true)
	assert.Nil(t, err, fmt.Sprintf("ChangeCeiling returned error, but should not. Error: %v \n", err))
	assert.Equal(t, "[1.0.0,2.0.0]", v1.Get_expression(), "Version range should be [1.0.0,2.0.0]")

	err = v1.ChangeCeiling("3.0.1-release", false)
	assert.Nil(t, err, fmt.Sprintf("ChangeCeiling returned error, but should not. Error: %v \n", err))
	assert.Equal(t, "[1.0.0,3.0.1-release)", v1.Get_expression(), "Version range should be [1.0.0,3.0.1-release)")

	err = v1.ChangeCeiling("3.0.1-release", true)
	assert.Nil(t, err, fmt.Sprintf("ChangeCeiling returned error, but should not. Error: %v \n", err))
	assert.Equal(t, "[1.0.0,3.0.1-release]", v1.Get_expression(), "Version range should be [1.0.0,3.0.1-release]")

	err = v1.ChangeCeiling("INFINITY", false)
	assert.Nil(t, err, fmt.Sprintf("ChangeCeiling returned error, but should not. Error: %v \n", err))
	assert.Equal(t, "[1.0.0,INFINITY)", v1.Get_expression(), "Version range should be [1.0.0,INFINITY)")

	err = v1.ChangeCeiling("0.1", false)
	assert.NotNil(t, err, "ChangeCeiling should return error, but it did not. %v", v1.Get_expression())

	err = v1.ChangeCeiling("1.0", false)
	assert.NotNil(t, err, "ChangeCeiling should return error, but it did not. %v", v1.Get_expression())

	err = v1.ChangeCeiling("2.0a", false)
	assert.NotNil(t, err, "ChangeCeiling should return error, but it did not. %v", v1.Get_expression())

	err = v1.ChangeCeiling("1.0", true)
	assert.Nil(t, err, fmt.Sprintf("ChangeCeiling returned error, but should not. Error: %v \n", err))
	assert.Equal(t, "[1.0.0,1.0.0]", v1.Get_expression(), "Version range should be [1.0.0,1.0.0]")

}

// This series of tests version comparison
func TestCompareVersions(t *testing.T) {
	v1 := "1"
	v2 := "2"
	c, err := CompareVersions(v1, v2)
	assert.Nil(t, err, fmt.Sprintf("Error should be nil, but got:%v \n", err))
	assert.Equal(t, -1, c, fmt.Sprintf("%v should be lower than %v.", v1, v2))
	c, err = CompareVersions(v2, v1)
	assert.Nil(t, err, fmt.Sprintf("Error should be nil, but got:%v \n", err))
	assert.Equal(t, 1, c, fmt.Sprintf("%v should be higher than %v.", v2, v1))

	v1 = "1.5"
	v2 = "2.0.5"
	c, err = CompareVersions(v1, v2)
	assert.Nil(t, err, fmt.Sprintf("Error should be nil, but got:%v \n", err))
	assert.Equal(t, -1, c, fmt.Sprintf("%v should be lower than %v.", v1, v2))
	c, err = CompareVersions(v2, v1)
	assert.Nil(t, err, fmt.Sprintf("Error should be nil, but got:%v \n", err))
	assert.Equal(t, 1, c, fmt.Sprintf("%v should be higher than %v.", v2, v1))

	v1 = "0.9"
	v2 = "0.10"
	c, err = CompareVersions(v1, v2)
	assert.Nil(t, err, fmt.Sprintf("Error should be nil, but got:%v \n", err))
	assert.Equal(t, -1, c, fmt.Sprintf("%v should be lower than %v.", v1, v2))
	c, err = CompareVersions(v2, v1)
	assert.Nil(t, err, fmt.Sprintf("Error should be nil, but got:%v \n", err))
	assert.Equal(t, 1, c, fmt.Sprintf("%v should be higher than %v.", v2, v1))

	v1 = "1.0.9"
	v2 = "1.0.10"
	c, err = CompareVersions(v1, v2)
	assert.Nil(t, err, fmt.Sprintf("Error should be nil, but got:%v \n", err))
	assert.Equal(t, -1, c, fmt.Sprintf("%v should be lower than %v.", v1, v2))
	c, err = CompareVersions(v2, v1)
	assert.Nil(t, err, fmt.Sprintf("Error should be nil, but got:%v \n", err))
	assert.Equal(t, 1, c, fmt.Sprintf("%v should be higher than %v.", v1, v2))

	v1 = "1.0.9"
	v2 = "INFINITY"
	c, err = CompareVersions(v1, v2)
	assert.Nil(t, err, fmt.Sprintf("Error should be nil, but got:%v \n", err))
	assert.Equal(t, -1, c, fmt.Sprintf("%v should be lower than %v.", v1, v2))
	c, err = CompareVersions(v2, v1)
	assert.Nil(t, err, fmt.Sprintf("Error should be nil, but got:%v \n", err))
	assert.Equal(t, 1, c, fmt.Sprintf("%v should be higher than %v.", v1, v2))

	v1 = "1.0.1-a"
	v2 = "1.0.1-b"
	c, err = CompareVersions(v1, v2)
	assert.Nil(t, err, fmt.Sprintf("Error should be nil, but got:%v \n", err))
	assert.Equal(t, -1, c, fmt.Sprintf("%v should be lower than %v.", v1, v2))
	c, err = CompareVersions(v2, v1)
	assert.Nil(t, err, fmt.Sprintf("Error should be nil, but got:%v \n", err))
	assert.Equal(t, 1, c, fmt.Sprintf("%v should be higher than %v.", v1, v2))

	v1 = "1.0.1-c"
	v2 = "2.0.1-a"
	c, err = CompareVersions(v1, v2)
	assert.Nil(t, err, fmt.Sprintf("Error should be nil, but got:%v \n", err))
	assert.Equal(t, -1, c, fmt.Sprintf("%v should be lower than %v.", v1, v2))
	c, err = CompareVersions(v2, v1)
	assert.Nil(t, err, fmt.Sprintf("Error should be nil, but got:%v \n", err))
	assert.Equal(t, 1, c, fmt.Sprintf("%v should be higher than %v.", v1, v2))

	v1 = "INFINITY"
	v2 = "INFINITY"
	c, err = CompareVersions(v1, v2)
	assert.Nil(t, err, fmt.Sprintf("Error should be nil, but got:%v \n", err))
	assert.Equal(t, 0, c, fmt.Sprintf("%v should be equal to %v.", v1, v2))

	v1 = "2.0"
	v2 = "2.0.0"
	c, err = CompareVersions(v1, v2)
	assert.Nil(t, err, fmt.Sprintf("Error should be nil, but got:%v \n", err))
	assert.Equal(t, 0, c, fmt.Sprintf("%v should be equal to %v.", v1, v2))
	c, err = CompareVersions(v2, v1)
	assert.Nil(t, err, fmt.Sprintf("Error should be nil, but got:%v \n", err))
	assert.Equal(t, 0, c, fmt.Sprintf("%v should be equal to %v.", v1, v2))

	v1 = "2.0-a"
	v2 = "2.0.0-a"
	c, err = CompareVersions(v1, v2)
	assert.Nil(t, err, fmt.Sprintf("Error should be nil, but got:%v \n", err))
	assert.Equal(t, 0, c, fmt.Sprintf("%v should be equal to %v.", v1, v2))
	c, err = CompareVersions(v2, v1)
	assert.Nil(t, err, fmt.Sprintf("Error should be nil, but got:%v \n", err))
	assert.Equal(t, 0, c, fmt.Sprintf("%v should be equal to %v.", v1, v2))

	v1 = "2-a"
	v2 = "2-a"
	c, err = CompareVersions(v1, v2)
	assert.Nil(t, err, fmt.Sprintf("Error should be nil, but got:%v \n", err))
	assert.Equal(t, 0, c, fmt.Sprintf("%v should be equal to %v.", v1, v2))
	c, err = CompareVersions(v2, v1)
	assert.Nil(t, err, fmt.Sprintf("Error should be nil, but got:%v \n", err))
	assert.Equal(t, 0, c, fmt.Sprintf("%v should be equal to %v.", v1, v2))

	v1 = "2.0.x"
	v2 = "2.0.0"
	c, err = CompareVersions(v1, v2)
	assert.NotNil(t, err, fmt.Sprintf("Should get error, but did not. \n"))

	v1 = "2.0.1x"
	v2 = "2.0.0"
	c, err = CompareVersions(v1, v2)
	assert.NotNil(t, err, fmt.Sprintf("Should get error, but did not. \n"))

	v1 = "[2.0, 1.1)"
	v2 = "2.0.0"
	c, err = CompareVersions(v1, v2)
	assert.NotNil(t, err, fmt.Sprintf("Should get error, but did not. \n"))
}
