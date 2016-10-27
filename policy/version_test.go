package policy

import (
	// "fmt"
	"testing"
)

// This series of tests verifies that constructor works correctly, by handling invalid input
// correctly.
func TestConstructor(t *testing.T) {
	if c, err := Version_Expression_Factory("1.2.3"); c == nil {
		t.Errorf("Factory returned nil, but should not. Error: %v \n", err)
	} else if c.Get_expression() != "[1.2.3,INFINITY)" {
		t.Errorf("Factory did not correctly save the full expression, returned %v\n", c.Get_expression())
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
	} else if inrange, err := c.Is_within_range("2.1"); err != nil || inrange {
		t.Errorf("Input is NOT in range. Error: %v \n", err)
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
	} else if inrange, err := c.Is_within_range("1.1"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.9"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.0.1"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("1.1.1"); err != nil || !inrange {
		t.Errorf("Input is in range. Error: %v \n", err)
	} else if inrange, err := c.Is_within_range("2.1"); err != nil || inrange {
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
	} else if inrange, err := c.Is_within_range("2.1"); err != nil || inrange {
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
	} else if inrange, err := c.Is_within_range("2.1"); err != nil || inrange {
		t.Errorf("Input is NOT in range. Error: %v \n", err)
	}

}
