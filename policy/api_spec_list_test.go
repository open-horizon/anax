package policy

import (
	"testing"
)

// APISpecification Tests
// First, some tests where a subset (or equal) is found
func Test_APISpecification_subset_found(t *testing.T) {
	var prod_as *APISpecList
	var con_as *APISpecList

	prod1 := `[{"specRef": "http://mycompany.com/dm/gps","version": "[1.0.0,2.0.0)","exclusiveAccess": false}]`
	con1 := `[]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := con_as.Is_Subset_Of(prod_as); err != nil {
				t.Errorf("Error: %v is a subset of %v, subset error was %v\n", con1, prod1, err)
			}
		}
	}

	con1 = `[{"specRef": "http://mycompany.com/dm/gps","version": "1.0.0","exclusiveAccess": false}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := con_as.Is_Subset_Of(prod_as); err != nil {
				t.Errorf("Error: %v is a subset of %v, subset error was %v\n", con1, prod1, err)
			}
		}
	}

	prod1 = `[{"specRef": "http://mycompany.com/dm/gps","version": "[1.0.0,2.0.0)","exclusiveAccess": false},
              {"specRef": "http://mycompany.com/dm/cpu_temp","version": "[1.0.0,2.0.0)","exclusiveAccess": false}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := con_as.Is_Subset_Of(prod_as); err != nil {
				t.Errorf("Error: %v is a subset of %v, subset error was %v\n", con1, prod1, err)
			}
		}
	}

	con1 = `[{"specRef": "http://mycompany.com/dm/gps","version": "1.5","exclusiveAccess": false},
             {"specRef": "http://mycompany.com/dm/cpu_temp","version": "1","exclusiveAccess": false}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := con_as.Is_Subset_Of(prod_as); err != nil {
				t.Errorf("Error: %v is a subset of %v, subset error was %v\n", con1, prod1, err)
			}
		}
	}

	prod1 = `[{"specRef": "http://mycompany.com/dm/gps","version": "[1.0.0,2.0.0)","exclusiveAccess": false},
              {"specRef": "http://mycompany.com/dm/cpu_temp","version": "[1.0.0,2.0.0)","exclusiveAccess": false},
              {"specRef": "http://mycompany.com/dm/weather","version": "[1.0.0,2.0.0)","exclusiveAccess": false}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := con_as.Is_Subset_Of(prod_as); err != nil {
				t.Errorf("Error: %v is a subset of %v, subset error was %v\n", con1, prod1, err)
			}
		}
	}

	prod1 = `[{"specRef": "http://mycompany.com/dm/gps","version": "[1.0.0,2.0.0)","exclusiveAccess": false}]`
	con1 = `[{"specRef": "http://mycompany.com/dm/gps","version": "1.0.0","exclusiveAccess": false}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := con_as.Is_Subset_Of(prod_as); err != nil {
				t.Errorf("Error: %v is a subset of %v, subset error was %v\n", con1, prod1, err)
			}
		}
	}

	prod1 = `[{"specRef": "http://mycompany.com/dm/gps","version": "[1.0.0,2.0.0)","exclusiveAccess": false}]`
	con1 = `[{"specRef": "http://mycompany.com/dm/gps","version": "1.0.0","exclusiveAccess": false}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := con_as.Is_Subset_Of(prod_as); err != nil {
				t.Errorf("Error: %v is a subset of %v, subset error was %v\n", con1, prod1, err)
			}
		}
	}

	prod1 = `[{"specRef": "http://mycompany.com/dm/gps","version": "[1.0.0,2.0.0)","exclusiveAccess": false,"arch":"x86"}]`
	con1 = `[{"specRef": "http://mycompany.com/dm/gps","version": "1.0.0","exclusiveAccess": false,"arch":"x86"}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := con_as.Is_Subset_Of(prod_as); err != nil {
				t.Errorf("Error: %v is a subset of %v, subset error was %v\n", con1, prod1, err)
			}
		}
	}

	prod1 = `[{"specRef": "http://mycompany.com/dm/gps","version": "[1.0.0,2.0.0)","exclusiveAccess": false,"arch":"x86"}]`
	con1 = `[{"specRef": "http://mycompany.com/dm/gps","version": "1.0.0","exclusiveAccess": false,"arch":"x86"}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := con_as.Is_Subset_Of(prod_as); err != nil {
				t.Errorf("Error: %v is a subset of %v, subset error was %v\n", con1, prod1, err)
			}
		}
	}
}

// Second, some tests where a subset (or equal) is NOT found
func Test_APISpecification_subset_not_found(t *testing.T) {
	var prod_as *APISpecList
	var con_as *APISpecList

	prod1 := `[{"specRef": "http://mycompany.com/dm/gps","version": "[1.0.0,2.0.0)","exclusiveAccess": false}]`
	con1 := `[{"specRef": "http://mycompany.com/dm/cpu_temp","version": "1","exclusiveAccess": false}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := con_as.Is_Subset_Of(prod_as); err == nil {
				t.Errorf("Error: %v is not a subset of %v.\n", con1, prod1)
			}
		}
	}

	prod1 = `[{"specRef": "http://mycompany.com/dm/gps","version": "[1.0.0,2.0.0)","exclusiveAccess": false},
              {"specRef": "http://mycompany.com/dm/weather","version": "[1.0.0,2.0.0)","exclusiveAccess": false}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := con_as.Is_Subset_Of(prod_as); err == nil {
				t.Errorf("Error: %v is not a subset of %v.\n", con1, prod1)
			}
		}
	}

	con1 = `[{"specRef": "http://mycompany.com/dm/gps","version": "2","exclusiveAccess": false}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := con_as.Is_Subset_Of(prod_as); err == nil {
				t.Errorf("Error: %v is not a subset of %v.\n", con1, prod1)
			}
		}
	}

	con1 = `[{"specRef": "http://mycompany.com/dm/gps","version": "1.2","exclusiveAccess": false},
             {"specRef": "http://mycompany.com/dm/cpu_temp","version": "1","exclusiveAccess": false}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := con_as.Is_Subset_Of(prod_as); err == nil {
				t.Errorf("Error: %v is not a subset of %v.\n", con1, prod1)
			}
		}
	}

	con1 = `[{"specRef": "http://mycompany.com/dm/gps","version": "1.2","exclusiveAccess": false},
             {"specRef": "http://mycompany.com/dm/weather","version": "2","exclusiveAccess": false}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := con_as.Is_Subset_Of(prod_as); err == nil {
				t.Errorf("Error: %v is not a subset of %v.\n", con1, prod1)
			}
		}
	}

	prod1 = `[]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := con_as.Is_Subset_Of(prod_as); err == nil {
				t.Errorf("Error: %v is not a subset of %v.\n", con1, prod1)
			}
		}
	}

	prod1 = `[{"specRef": "http://mycompany.com/dm/gps","version": "[1.0.0,2.0.0)","exclusiveAccess": false,"arch":"arm"}]`
	con1 = `[{"specRef": "http://mycompany.com/dm/gps","version": "1","exclusiveAccess": false,"arch":"x86"}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := con_as.Is_Subset_Of(prod_as); err == nil {
				t.Errorf("Error: %v is not a subset of %v.\n", con1, prod1)
			}
		}
	}

}
