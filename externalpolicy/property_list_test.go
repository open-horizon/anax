// +build unit

package externalpolicy

import (
	"encoding/json"
	"strings"
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

	p1 = `[{"name":"prop1","value":"val1","type":"string"}]`
	p2 = `[{"name":"prop1","value":"val1"}]`
	if pl1 = create_PropertyList(p1, t); pl1 != nil {
		if pl2 = create_PropertyList(p2, t); pl2 != nil {
			if err := pl1.Compatible_With(pl2); err != nil {
				t.Errorf("Error: %v is compatible with %v, error was %v\n", p1, p2, err)
			}
		}
	}

	p1 = `[{"name":"prop1","value":12.0},{"name":"prop2","value":"1.0.3","type":"version"}]`
	p2 = `[{"name":"prop1","value":12,"type":"float"},{"name":"prop3","value":"val4"}]`
	if pl1 = create_PropertyList(p1, t); pl1 != nil {
		if pl2 = create_PropertyList(p2, t); pl2 != nil {
			if err := pl1.Compatible_With(pl2); err != nil {
				t.Errorf("Error: %v is compatible with %v, error was %v\n", p1, p2, err)
			}
		}
	}

	p1 = `[{"name":"prop1","value":"1.2.53"},{"name":"prop2","value":"val2"}]`
	p2 = `[{"name":"prop2","value":"val2"},{"name":"prop1","value":"1.2.53","type":"version"}]`
	if pl1 = create_PropertyList(p1, t); pl1 != nil {
		if pl2 = create_PropertyList(p2, t); pl2 != nil {
			if err := pl1.Compatible_With(pl2); err != nil {
				t.Errorf("Error: %v is compatible with %v, error was %v\n", p1, p2, err)
			}
		}
	}

	p1 = `[{"name":"prop1","value":"a,b,c","type":"list of string"},{"name":"prop2","value":"val2"}]`
	p2 = `[{"name":"prop2","value":"val2"},{"name":"prop1","value":"b,a,c","type":"list of string"}]`
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

	p1 = `[{"name":"prop1","value":"a,b,c","type":"list of string"},{"name":"prop2","value":"val2"}]`
	p2 = `[{"name":"prop2","value":"val2"},{"name":"prop1","value":"a,b,d","type":"list of string"}]`
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

func create_PropertyList_UseNumbers(jsonString string, t *testing.T) *PropertyList {
	pl := new(PropertyList)
	dec := json.NewDecoder(strings.NewReader(jsonString))

	if err := dec.Decode(&pl); err != nil {
		t.Errorf("Error unmarshalling PropertyList json string: %v error:%v\n", jsonString, err)
		return nil
	} else {
		return pl
	}
}

//Test the property validation with valid properties
func Test_Validate_valid(t *testing.T) {
	p1 := `[{"name":"prop1","value":"val1","type":"string"},{"name":"prop2","value":"val2"},{"name":"prop3","value":"423"}]`
	if pl1 := create_PropertyList(p1, t); pl1 != nil {
		if err := pl1.Validate(); err != nil {
			t.Errorf("Error: %v has only valid properties but gave error: %v\n", p1, err)
		}
	}

	p1 = `[{"name":"prop1","value":12,"type":"float"},{"name":"prop2","value":10.5,"type":"float"},{"name":"prop2","value":-10,"type":"int"}]`
	if pl1 := create_PropertyList(p1, t); pl1 != nil {
		if err := pl1.Validate(); err != nil {
			t.Errorf("Error: %v has only valid properties but gave error: %v\n", p1, err)
		}
	}

	p1 = `[{"name":"prop1","value":12,"type":"float"},{"name":"prop2","value":10.5,"type":"float"},{"name":"prop2","value":-10,"type":"int"}]`
	if pl1 := create_PropertyList_UseNumbers(p1, t); pl1 != nil {
		if err := pl1.Validate(); err != nil {
			t.Errorf("Error: %v has only valid properties but gave error: %v\n", p1, err)
		}
	}

	p1 = `[{"name":"prop1","value":"0.0.1","type":"version"},{"name":"prop2","value":"5.32.15","type":"version"}]`
	if pl1 := create_PropertyList(p1, t); pl1 != nil {
		if err := pl1.Validate(); err != nil {
			t.Errorf("Error: %v has only valid properties but gave error: %v\n", p1, err)
		}
	}

	p1 = `[{"name":"prop1","value":"val1,val2,val4","type":"list of string"}]`
	if pl1 := create_PropertyList(p1, t); pl1 != nil {
		if err := pl1.Validate(); err != nil {
			t.Errorf("Error: %v has only valid properties but gave error: %v\n", p1, err)
		}
	}
	p1 = `[{"name":"prop1","value":[1,2]}]`
	if pl1 := create_PropertyList(p1, t); pl1 != nil {
		if err := pl1.Validate(); err == nil {
			t.Errorf("Error: %v has invalid properties but gave no error\n", p1)
		}
	}
}

//Test the property validation with invalid properties
func Test_Validate_invalid(t *testing.T) {
	p1 := `[{"name":"prop1","value":["val1","val2","val4"]}]`
	if pl1 := create_PropertyList(p1, t); pl1 != nil {
		if err := pl1.Validate(); err == nil {
			t.Errorf("Error: %v has invalid properties but gave no error\n", p1)
		}
	}

	p1 = `[{"name":"prop1"}]`
	if pl1 := create_PropertyList(p1, t); pl1 != nil {
		if err := pl1.Validate(); err == nil {
			t.Errorf("Error: %v has invalid properties but gave no error\n", p1)
		}
	}

	p1 = `[{"name":"prop1","value":["val1","val2","val4"],"type":"string"}]`
	if pl1 := create_PropertyList(p1, t); pl1 != nil {
		if err := pl1.Validate(); err == nil {
			t.Errorf("Error: %v has invalid properties but gave no error\n", p1)
		}
	}

	p1 = `[{"name":"prop1","value":1003.56,"type":"int"}]`
	if pl1 := create_PropertyList(p1, t); pl1 != nil {
		if err := pl1.Validate(); err == nil {
			t.Errorf("Error: %v has invalid properties but gave no error\n", p1)
		}
	}

	p1 = `[{"name":"prop1","value":1003.56,"type":"int"}]`
	if pl1 := create_PropertyList_UseNumbers(p1, t); pl1 != nil {
		if err := pl1.Validate(); err == nil {
			t.Errorf("Error: %v has invalid properties but gave no error\n", p1)
		}
	}
	p1 = `[{"name":"prop1","value":true,"type":"float"}]`
	if pl1 := create_PropertyList(p1, t); pl1 != nil {
		if err := pl1.Validate(); err == nil {
			t.Errorf("Error: %v has invalid properties but gave no error\n", p1)
		}
	}
	p1 = `[{"name":"prop1","value":1,"type":"boolean"}]`
	if pl1 := create_PropertyList(p1, t); pl1 != nil {
		if err := pl1.Validate(); err == nil {
			t.Errorf("Error: %v has invalid properties but gave no error\n", p1)
		}
	}
	p1 = `[{"name":"prop1","value":"1,0,5","type":"version"}]`
	if pl1 := create_PropertyList(p1, t); pl1 != nil {
		if err := pl1.Validate(); err == nil {
			t.Errorf("Error: %v has invalid properties but gave no error\n", p1)
		}
	}
}

func Test_add_property(t *testing.T) {
	pl1 := new(PropertyList)
	if err := pl1.Add_Property(Property_Factory("iame2edev", "true")); err != nil {
		t.Errorf("Error valid property could not be added: %v", err)
	}

	pl1 = new(PropertyList)
	if err := pl1.Add_Property(Property_Factory("cpu", 3.0)); err != nil {
		t.Errorf("Error valid property could not be added: %v", err)
	}

	pl1 = new(PropertyList)
	if err := pl1.Add_Property(Property_Factory("hello", "\"world\"")); err != nil {
		t.Errorf("Error valid property could not be added: %v", err)
	}

	pl1 = new(PropertyList)
	if err := pl1.Add_Property(Property_Factory("version", "1.1.1")); err != nil {
		t.Errorf("Error valid property could not be added: %v", err)
	}

	pl1 = new(PropertyList)
	if err := pl1.Add_Property(Property_Factory("eggs", "truck load")); err != nil {
		t.Errorf("Error valid property could not be added: %v", err)
	}

	pl1 = new(PropertyList)
	if err := pl1.Add_Property(Property_Factory("certification", "USDA, organic")); err != nil {
		t.Errorf("Error valid property could not be added: %v", err)
	}
}
