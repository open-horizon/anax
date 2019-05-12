// +build unit

package policy

import (
	"encoding/json"
	"testing"
)

// PropertyList Tests
// First, some tests where the lists are compatible
func Test_PropertyList_compatible(t *testing.T) {
	var pl1 *PropertyList
	var pl2 *PropertyList

	p1 := `[{"name":"prop1","value":"val1"}]`
	p2 := `[{"name":"prop1","value":"val1"}]`
	if pl1 = create_PropertyList(p1, t); pl1 != nil {
		if pl2 = create_PropertyList(p2, t); pl2 != nil {
			if err := pl1.Compatible_With(pl2); err != nil {
				t.Errorf("Error: %v is compatible with %v, error was %v\n", p1, p2, err)
			}
		}
	}

	p2 = `[{"name":"prop2","value":"val1"}]`
	if pl1 = create_PropertyList(p1, t); pl1 != nil {
		if pl2 = create_PropertyList(p2, t); pl2 != nil {
			if err := pl1.Compatible_With(pl2); err != nil {
				t.Errorf("Error: %v is compatible with %v, error was %v\n", p1, p2, err)
			}
		}
	}

	p1 = `[{"name":"prop1","value":"val1"},{"name":"prop2","value":"val2"}]`
	p2 = `[{"name":"prop1","value":"val1"}]`
	if pl1 = create_PropertyList(p1, t); pl1 != nil {
		if pl2 = create_PropertyList(p2, t); pl2 != nil {
			if err := pl1.Compatible_With(pl2); err != nil {
				t.Errorf("Error: %v is compatible with %v, error was %v\n", p1, p2, err)
			}
		}
	}

	p1 = `[{"name":"prop1","value":"val1"},{"name":"prop2","value":"val2"},{"name":"prop3","value":"val3"}]`
	p2 = `[{"name":"prop1","value":"val1"},{"name":"prop4","value":"val4"}]`
	if pl1 = create_PropertyList(p1, t); pl1 != nil {
		if pl2 = create_PropertyList(p2, t); pl2 != nil {
			if err := pl1.Compatible_With(pl2); err != nil {
				t.Errorf("Error: %v is compatible with %v, error was %v\n", p1, p2, err)
			}
		}
	}
}

// Second, some tests where the lists are incompatible
func Test_PropertyList_incompatible(t *testing.T) {
	var pl1 *PropertyList
	var pl2 *PropertyList

	p1 := `[{"name":"prop1","value":"val1"}]`
	p2 := `[{"name":"prop1","value":"val2"}]`
	if pl1 = create_PropertyList(p1, t); pl1 != nil {
		if pl2 = create_PropertyList(p2, t); pl2 != nil {
			if err := pl1.Compatible_With(pl2); err == nil {
				t.Errorf("Error: %v is not compatible with %v\n", p1, p2)
			}
		}
	}

	p1 = `[{"name":"prop1","value":"val2"}]`
	p2 = `[{"name":"prop2","value":"val2"},{"name":"prop1","value":"val1"}]`
	if pl1 = create_PropertyList(p1, t); pl1 != nil {
		if pl2 = create_PropertyList(p2, t); pl2 != nil {
			if err := pl1.Compatible_With(pl2); err == nil {
				t.Errorf("Error: %v is not compatible with %v\n", p1, p2)
			}
		}
	}

	p1 = `[{"name":"prop1","value":"val1"},{"name":"prop2","value":"val2"}]`
	p2 = `[{"name":"prop2","value":"val2"},{"name":"prop1","value":"val2"}]`
	if pl1 = create_PropertyList(p1, t); pl1 != nil {
		if pl2 = create_PropertyList(p2, t); pl2 != nil {
			if err := pl1.Compatible_With(pl2); err == nil {
				t.Errorf("Error: %v is not compatible with %v\n", p1, p2)
			}
		}
	}
}

// Create a Property array from a JSON serialization. The JSON serialization
// does not have to be a valid Property serialization, just has to be a valid
// JSON serialization.
func create_PropertyList(jsonString string, t *testing.T) *PropertyList {
	pl := new(PropertyList)

	if err := json.Unmarshal([]byte(jsonString), &pl); err != nil {
		t.Errorf("Error unmarshalling PropertyList json string: %v error:%v\n", jsonString, err)
		return nil
	} else {
		return pl
	}
}
