package policy

import (
	"testing"
)

// APISpecification Tests
// Some positive tests for the API Spec search
func Test_APISpecification_contains_specref(t *testing.T) {
	var as1 *APISpecList
	asString := `[{"specRef": "http://mycompany.com/dm/gps","version": "1.0.0","exclusiveAccess": false,"arch":"arm"}]`

	searchURL := "http://mycompany.com/dm/gps"
	if as1 = create_APISpecification(asString, t); as1 != nil {
		if !(*as1).ContainsSpecRef(searchURL, "1.0.0") {
			t.Errorf("Error: %v is in %v.\n", searchURL, *as1)
		}
	}

	asString = `[{"specRef": "http://mycompany.com/dm/gps","version": "1.0.0","exclusiveAccess": false,"arch":"arm"},
				{"specRef": "http://mycompany.com/dm/cpu","version": "1.0.0","exclusiveAccess": false,"arch":"arm"},
				{"specRef": "http://mycompany.com/dm/net","version": "1.0.0","exclusiveAccess": false,"arch":"arm"}]`

	searchURL = "http://mycompany.com/dm/net"
	if as1 = create_APISpecification(asString, t); as1 != nil {
		if !(*as1).ContainsSpecRef(searchURL, "1.0.0") {
			t.Errorf("Error: %v is in %v.\n", searchURL, *as1)
		}
	}

	searchURL = "http://mycompany.com/dm/gps"
	if as1 = create_APISpecification(asString, t); as1 != nil {
		if !(*as1).ContainsSpecRef(searchURL, "1.0.0") {
			t.Errorf("Error: %v is in %v.\n", searchURL, *as1)
		}
	}

	searchURL = "http://mycompany.com/dm/cpu"
	if as1 = create_APISpecification(asString, t); as1 != nil {
		if !(*as1).ContainsSpecRef(searchURL, "1.0.0") {
			t.Errorf("Error: %v is in %v.\n", searchURL, *as1)
		}
	}
}

// Some negative tests for the API Spec search
func Test_APISpecification_not_contains_specref(t *testing.T) {
	var as1 *APISpecList
	asString := `[{"specRef": "http://mycompany.com/dm/gps","version": "1.0.0","exclusiveAccess": false,"arch":"arm"}]`

	searchURL := "http://mycompany.com/dm/net"
	if as1 = create_APISpecification(asString, t); as1 != nil {
		if (*as1).ContainsSpecRef(searchURL, "1.0.0") {
			t.Errorf("Error: %v is not in %v.\n", searchURL, *as1)
		}
	}

	asString = `[{"specRef": "http://mycompany.com/dm/gps","version": "1.0.0","exclusiveAccess": false,"arch":"arm"},
				{"specRef": "http://mycompany.com/dm/cpu","version": "1.0.0","exclusiveAccess": false,"arch":"arm"},
				{"specRef": "http://mycompany.com/dm/net","version": "1.0.0","exclusiveAccess": false,"arch":"arm"}]`

	searchURL = "http://mycompany.com/dm/nit"
	if as1 = create_APISpecification(asString, t); as1 != nil {
		if (*as1).ContainsSpecRef(searchURL, "1.0.0") {
			t.Errorf("Error: %v is not in %v.\n", searchURL, *as1)
		}
	}

	searchURL = "http://mycompany.com/dm/gps"
	if as1 = create_APISpecification(asString, t); as1 != nil {
		if (*as1).ContainsSpecRef(searchURL, "2.0.0") {
			t.Errorf("Error: %v is not in %v.\n", searchURL, *as1)
		}
	}

	searchURL = ""
	if as1 = create_APISpecification(asString, t); as1 != nil {
		if (*as1).ContainsSpecRef(searchURL, "1.0.0") {
			t.Errorf("Error: %v is not in %v.\n", searchURL, *as1)
		}
	}

	asString = `[]`

	searchURL = "http://mycompany.com/dm/nit"
	if as1 = create_APISpecification(asString, t); as1 != nil {
		if (*as1).ContainsSpecRef(searchURL, "1.0.0") {
			t.Errorf("Error: %v is not in %v.\n", searchURL, *as1)
		}
	}
}

// Some sameness tests - API spec lists which are the same
func Test_APISpecification_same(t *testing.T) {
	var as1, as2 *APISpecList
	asString1 := `[{"specRef": "http://mycompany.com/dm/gps","version":"1.0.0","exclusiveAccess":false,"arch":"arm"}]`
	asString2 := `[{"specRef": "http://mycompany.com/dm/gps","version":"1.0.0","exclusiveAccess":false,"arch":"arm"}]`

	if as1 = create_APISpecification(asString1, t); as1 != nil {
		if as2 = create_APISpecification(asString2, t); as2 != nil {
			if !as1.IsSame(*as2, true) {
				t.Errorf("Error: %v and %v are the same.", as1, as2)
			}
		}
	}

	asString1 = `[{"specRef": "http://mycompany.com/dm/gps","version":"1.0.0","exclusiveAccess":false,"arch":"arm"}]`
	asString2 = `[{"specRef": "http://mycompany.com/dm/gps","version":"2.0.0","exclusiveAccess":false,"arch":"arm"}]`

	if as1 = create_APISpecification(asString1, t); as1 != nil {
		if as2 = create_APISpecification(asString2, t); as2 != nil {
			if !as1.IsSame(*as2, false) {
				t.Errorf("Error: %v and %v are the same ignoring version.", as1, as2)
			}
		}
	}

}

// Some sameness tests - API spec lists which are NOT the same
func Test_APISpecification_not_same(t *testing.T) {
	var as1, as2 *APISpecList
	asString1 := `[{"specRef": "http://mycompany.com/dm/gps","version":"1.0.0","exclusiveAccess":false,"arch":"arm"}]`
	asString2 := `[{"specRef": "http://mycompany.com/dm/gps","version":"2.0.0","exclusiveAccess":false,"arch":"arm"}]`

	if as1 = create_APISpecification(asString1, t); as1 != nil {
		if as2 = create_APISpecification(asString2, t); as2 != nil {
			if as1.IsSame(*as2, true) {
				t.Errorf("Error: %v and %v are NOT the same.", as1, as2)
			}
		}
	}

	asString1 = `[{"specRef": "http://mycompany.com/dm/gps","version":"1.0.0","exclusiveAccess":false,"arch":"arm"}]`
	asString2 = `[{"specRef": "http://mycompany.com/dm/gps2","version":"1.0.0","exclusiveAccess":false,"arch":"arm"}]`

	if as1 = create_APISpecification(asString1, t); as1 != nil {
		if as2 = create_APISpecification(asString2, t); as2 != nil {
			if as1.IsSame(*as2, true) {
				t.Errorf("Error: %v and %v are NOT the same.", as1, as2)
			}
		}
	}

	asString1 = `[{"specRef": "http://mycompany.com/dm/gps","version":"1.0.0","exclusiveAccess":false,"arch":"arm"}]`
	asString2 = `[{"specRef": "http://mycompany.com/dm/gps","version":"1.0.0","exclusiveAccess":true,"arch":"arm"}]`

	if as1 = create_APISpecification(asString1, t); as1 != nil {
		if as2 = create_APISpecification(asString2, t); as2 != nil {
			if as1.IsSame(*as2, true) {
				t.Errorf("Error: %v and %v are NOT the same.", as1, as2)
			}
		}
	}

	asString1 = `[{"specRef": "http://mycompany.com/dm/gps","version":"1.0.0","exclusiveAccess":false,"arch":"arm"}]`
	asString2 = `[{"specRef": "http://mycompany.com/dm/gps","version":"1.0.0","exclusiveAccess":false,"arch":"amd64"}]`

	if as1 = create_APISpecification(asString1, t); as1 != nil {
		if as2 = create_APISpecification(asString2, t); as2 != nil {
			if as1.IsSame(*as2, true) {
				t.Errorf("Error: %v and %v are NOT the same.", as1, as2)
			}
		}
	}

	asString1 = `[{"specRef": "http://mycompany.com/dm/gps","version":"1.0.0","exclusiveAccess":false,"arch":"arm"}]`
	asString2 = `[{"specRef": "http://mycompany.com/dm/gps2","version":"2.0.0","exclusiveAccess":false,"arch":"arm"}]`

	if as1 = create_APISpecification(asString1, t); as1 != nil {
		if as2 = create_APISpecification(asString2, t); as2 != nil {
			if as1.IsSame(*as2, false) {
				t.Errorf("Error: %v and %v are NOT the same even ignoring version.", as1, as2)
			}
		}
	}

	asString1 = `[{"specRef": "http://mycompany.com/dm/gps","version":"1.0.0","exclusiveAccess":false,"arch":"arm"}]`
	asString2 = `[{"specRef": "http://mycompany.com/dm/gps","version":"2.0.0","exclusiveAccess":true,"arch":"arm"}]`

	if as1 = create_APISpecification(asString1, t); as1 != nil {
		if as2 = create_APISpecification(asString2, t); as2 != nil {
			if as1.IsSame(*as2, false) {
				t.Errorf("Error: %v and %v are NOT the same even ignoring version.", as1, as2)
			}
		}
	}

	asString1 = `[{"specRef": "http://mycompany.com/dm/gps","version":"1.0.0","exclusiveAccess":false,"arch":"arm"}]`
	asString2 = `[{"specRef": "http://mycompany.com/dm/gps","version":"2.0.0","exclusiveAccess":false,"arch":"amd64"}]`

	if as1 = create_APISpecification(asString1, t); as1 != nil {
		if as2 = create_APISpecification(asString2, t); as2 != nil {
			if as1.IsSame(*as2, false) {
				t.Errorf("Error: %v and %v are NOT the same even ignoring version.", as1, as2)
			}
		}
	}

}

func Test_APISpecification_supports1(t *testing.T) {
	var prod_as *APISpecList
	var con_as *APISpecList

	prod1 := `[{"specRef":"http://mycompany.com/dm/gps","version":"1.0.0","exclusiveAccess":false,"arch":"amd64"}]`
	con1 := `[{"specRef":"http://mycompany.com/dm/gps","version":"1.0.0","exclusiveAccess":false,"arch":"amd64"}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := (*prod_as).Supports(*con_as); err != nil {
				t.Errorf("Error: %v supports %v, error was %v\n", con1, prod1, err)
			}
		}
	}
}

func Test_APISpecification_supports2(t *testing.T) {
	var prod_as *APISpecList
	var con_as *APISpecList

	prod1 := `[{"specRef":"http://mycompany.com/dm/gps","version":"1.5.0","exclusiveAccess":false,"arch":"amd64"}]`
	con1 := `[{"specRef":"http://mycompany.com/dm/gps","version":"1.0.0","exclusiveAccess":false,"arch":"amd64"}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := (*prod_as).Supports(*con_as); err != nil {
				t.Errorf("Error: %v supports %v, error was %v\n", con1, prod1, err)
			}
		}
	}
}

func Test_APISpecification_supports3(t *testing.T) {
	var prod_as *APISpecList
	var con_as *APISpecList

	prod1 := `[{"specRef":"http://mycompany.com/dm/gps","version":"2.5.0","exclusiveAccess":false,"arch":"amd64"}]`
	con1 := `[{"specRef":"http://mycompany.com/dm/gps","version":"1.0.0","exclusiveAccess":false,"arch":"amd64"}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := (*prod_as).Supports(*con_as); err != nil {
				t.Errorf("Error: %v supports %v, error was %v\n", con1, prod1, err)
			}
		}
	}
}

func Test_APISpecification_supports4(t *testing.T) {
	var prod_as *APISpecList
	var con_as *APISpecList

	prod1 := `[{"specRef":"http://mycompany.com/dm/gps","version":"1.5.9","exclusiveAccess":false,"arch":"amd64"}]`
	con1 := `[{"specRef":"http://mycompany.com/dm/gps","version":"[1.0.0,2.0.0)","exclusiveAccess":false,"arch":"amd64"}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := (*prod_as).Supports(*con_as); err != nil {
				t.Errorf("Error: %v supports %v, error was %v\n", con1, prod1, err)
			}
		}
	}
}

func Test_APISpecification_supports5(t *testing.T) {
	var prod_as *APISpecList
	var con_as *APISpecList

	prod1 := `[{"specRef":"http://mycompany.com/dm/gps","version":"1.0.0","exclusiveAccess":false,"arch":"amd64"}]`
	con1 := `[]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := (*prod_as).Supports(*con_as); err != nil {
				t.Errorf("Error: %v supports %v, error was %v\n", con1, prod1, err)
			}
		}
	}
}

func Test_APISpecification_notsupports1(t *testing.T) {
	var prod_as *APISpecList
	var con_as *APISpecList

	prod1 := `[{"specRef":"http://mycompany.com/dm/gps","version":"1.0.0","exclusiveAccess":false,"arch":"amd64"}]`
	con1 := `[{"specRef":"http://mycompany.com/dm/gps","version":"1.0.0","exclusiveAccess":false,"arch":"armhf"}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := (*prod_as).Supports(*con_as); err == nil {
				t.Errorf("Error: %v does not support %v\n", con1, prod1, err)
			}
		}
	}
}

func Test_APISpecification_notsupports2(t *testing.T) {
	var prod_as *APISpecList
	var con_as *APISpecList

	prod1 := `[{"specRef":"http://mycompany.com/dm/gps","version":"1.0.0","exclusiveAccess":false,"arch":"amd64"}]`
	con1 := `[{"specRef":"http://mycompany.com/dm/gps2","version":"1.0.0","exclusiveAccess":false,"arch":"amd64"}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := (*prod_as).Supports(*con_as); err == nil {
				t.Errorf("Error: %v does not support %v\n", con1, prod1, err)
			}
		}
	}
}

func Test_APISpecification_notsupports3(t *testing.T) {
	var prod_as *APISpecList
	var con_as *APISpecList

	prod1 := `[]`
	con1 := `[{"specRef":"http://mycompany.com/dm/gps","version":"1.0.0","exclusiveAccess":false,"arch":"amd64"}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := (*prod_as).Supports(*con_as); err == nil {
				t.Errorf("Error: %v does not support %v\n", con1, prod1, err)
			}
		}
	}
}

func Test_APISpecification_notsupports4(t *testing.T) {
	var prod_as *APISpecList
	var con_as *APISpecList

	prod1 := `[{"specRef":"http://mycompany.com/dm/gps","version":"1.0.0","exclusiveAccess":false,"arch":"amd64"},{"specRef":"http://mycompany.com/dm/gps2","version":"1.0.0","exclusiveAccess":false,"arch":"amd64"}]`
	con1 := `[{"specRef":"http://mycompany.com/dm/gps","version":"1.0.0","exclusiveAccess":false,"arch":"amd64"}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := (*prod_as).Supports(*con_as); err == nil {
				t.Errorf("Error: %v does not support %v\n", con1, prod1, err)
			}
		}
	}
}

func Test_APISpecification_notsupports5(t *testing.T) {
	var prod_as *APISpecList
	var con_as *APISpecList

	prod1 := `[{"specRef":"http://mycompany.com/dm/gps","version":"2.0.0","exclusiveAccess":false,"arch":"amd64"}]`
	con1 := `[{"specRef":"http://mycompany.com/dm/gps","version":"[1.0.0,2)","exclusiveAccess":false,"arch":"amd64"}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := (*prod_as).Supports(*con_as); err == nil {
				t.Errorf("Error: %v does not support %v\n", con1, prod1, err)
			}
		}
	}
}
