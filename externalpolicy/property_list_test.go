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
	if err := pl1.Add_Property(Property_Factory("iame2edev", "true"), false); err != nil {
		t.Errorf("Error valid property could not be added: %v", err)
	}

	pl1 = new(PropertyList)
	if err := pl1.Add_Property(Property_Factory("cpu", 3.0), false); err != nil {
		t.Errorf("Error valid property could not be added: %v", err)
	}

	pl1 = new(PropertyList)
	if err := pl1.Add_Property(Property_Factory("hello", "\"world\""), false); err != nil {
		t.Errorf("Error valid property could not be added: %v", err)
	}

	pl1 = new(PropertyList)
	if err := pl1.Add_Property(Property_Factory("version", "1.1.1"), false); err != nil {
		t.Errorf("Error valid property could not be added: %v", err)
	}

	pl1 = new(PropertyList)
	if err := pl1.Add_Property(Property_Factory("eggs", "truck load"), false); err != nil {
		t.Errorf("Error valid property could not be added: %v", err)
	}

	pl1 = new(PropertyList)
	if err := pl1.Add_Property(Property_Factory("certification", "USDA, organic"), false); err != nil {
		t.Errorf("Error valid property could not be added: %v", err)
	}
}

func Test_PropertyList_MergeWith(t *testing.T) {
	var pl1 *PropertyList
	var pl2 *PropertyList

	p1 := `[{"name":"prop1","value":"val1"}]`
	p2 := `[{"name":"prop1","value":"val1"}]`
	if pl1 = create_PropertyList(p1, t); pl1 != nil {
		if pl2 = create_PropertyList(p2, t); pl2 != nil {
			pl1.MergeWith(pl2, false)
			if len(*pl1) != 1 {
				t.Errorf("Error: p1 %v should have 1 elelments but got %v\n", pl1, len(*pl1))
			}
		}
	}

	p1 = `[{"name":"prop1","value":"val1"}]`
	p2 = `[{"name":"prop2","value":"val2"}]`
	if pl1 = create_PropertyList(p1, t); pl1 != nil {
		if pl2 = create_PropertyList(p2, t); pl2 != nil {
			pl1.MergeWith(pl2, false)
			if len(*pl1) != 2 {
				t.Errorf("Error: p1 %v should have 2 elelments but got %v\n", pl1, len(*pl1))
			}
		}
	}

	p1 = `[{"name":"prop1","value":"val1","type":"string"},{"name":"prop2","value":"val2"},{"name":"prop3","value":"423"}]`
	p2 = `[{"name":"prop4","value":"val4","type":"string"},{"name":"prop5","value":"val5"},{"name":"prop3","value":"423"}]`
	if pl1 = create_PropertyList(p1, t); pl1 != nil {
		if pl2 = create_PropertyList(p2, t); pl2 != nil {
			pl1.MergeWith(pl2, false)
			if len(*pl1) != 5 {
				t.Errorf("Error: p1 %v should have 5 elelments but got %v\n", pl1, len(*pl1))
			}
		}
	}

	p1 = `[{"name":"prop1","value":"val1","type":"string"},{"name":"prop2","value":"val2"},{"name":"prop3","value":"423"}]`
	p2 = `[{"name":"prop4","value":"val4","type":"string"},{"name":"prop5","value":"val5"},{"name":"prop3","value":"42355"}]`
	if pl1 = create_PropertyList(p1, t); pl1 != nil {
		if pl2 = create_PropertyList(p2, t); pl2 != nil {
			pl1.MergeWith(pl2, true)
			if len(*pl1) != 5 {
				t.Errorf("Error: p1 %v should have 5 elelments but got %v\n", pl1, len(*pl1))
			} else {
				for _, prop := range *pl1 {
					if prop.Name == "prop3" && prop.Value != "42355" {
						t.Errorf("Error: The valuse for prop3 should be 43255 but got: %v", prop.Value)
					}
				}
			}
		}
	}

}

func Test_PropertyList_HasProperty(t *testing.T) {
	var pl1 *PropertyList
	var pl2 *PropertyList

	p1 := `[{"name":"prop1","value":"val1"}]`
	p2 := `[{"name":"prop4","value":"val4","type":"string"},{"name":"prop5","value":"val5"},{"name":"prop3","value":"423"}]`
	pl1 = create_PropertyList(p1, t)
	pl2 = create_PropertyList(p2, t)

	if !pl1.HasProperty("prop1") {
		t.Errorf("Error: pl1 %v should have property prop3 but not.n", pl1)
	}
	if !pl2.HasProperty("prop4") {
		t.Errorf("Error: pl2 %v should have property prop4 but not.\n", pl2)
	}
	if !pl2.HasProperty("prop5") {
		t.Errorf("Error: pl2 %v should have property prop5 but not.\n", pl2)
	}
	if !pl2.HasProperty("prop3") {
		t.Errorf("Error: pl2 %v should have property prop3 but not.\n", pl2)
	}
	if pl2.HasProperty("prop1") {
		t.Errorf("Error: pl2 %v should not have property prop1 but it does.\n", pl2)
	}
}

func Test_PropertyList_AddProperty(t *testing.T) {
	p1 := `[{"name":"prop4","value":"val4","type":"string"},{"name":"prop5","value":"val5"},{"name":"prop3","value":"423"}]`
	pl1 := create_PropertyList(p1, t)

	if err := pl1.Add_Property(Property_Factory("prop4", "val4plue"), false); err == nil {
		t.Errorf("Should have returned an error but not.")
	}

	if err := pl1.Add_Property(Property_Factory("prop4", "val4plue"), true); err != nil {
		t.Errorf("Should not have returned an error but got: %v", err)
	} else {
		for _, p := range *pl1 {
			if p.Name == "prop4" && p.Value != "val4plue" {
				t.Errorf("Should not have set prop4 to val4plue, but has: %v", p.Value)
				break
			}
		}
	}

	if err := pl1.Add_Property(Property_Factory("prop5", "val5plue"), true); err != nil {
		t.Errorf("Should not have returned an error but got: %v", err)
	} else {
		for _, p := range *pl1 {
			if p.Name == "prop5" && p.Value != "val5plue" {
				t.Errorf("Should not have set prop5 to val5plue, but has: %v", p.Value)
				break
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
