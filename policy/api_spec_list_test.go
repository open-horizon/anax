//go:build unit
// +build unit

package policy

import (
	"testing"
)

// APISpecification Tests
// Some positive tests for the API Spec search
func Test_APISpecification_contains_specref(t *testing.T) {
	var as1 *APISpecList
	asString := `[{"specRef":"http://mycompany.com/dm/gps","organization":"myorg","version":"1.0.0","exclusiveAccess":false,"arch":"arm"}]`

	searchURL := "http://mycompany.com/dm/gps"
	if as1 = create_APISpecification(asString, t); as1 != nil {
		if !(*as1).ContainsSpecRef(searchURL, "myorg", "1.0.0") {
			t.Errorf("Error: %v is in %v.\n", searchURL, *as1)
		}
	}

	asString = `[{"specRef": "http://mycompany.com/dm/gps","organization":"myorg","version": "1.0.0","exclusiveAccess": false,"arch":"arm"},
				{"specRef": "http://mycompany.com/dm/cpu","organization":"myorg","version": "1.0.0","exclusiveAccess": false,"arch":"arm"},
				{"specRef": "http://mycompany.com/dm/net","organization":"myorg","version": "1.0.0","exclusiveAccess": false,"arch":"arm"}]`

	searchURL = "http://mycompany.com/dm/net"
	if as1 = create_APISpecification(asString, t); as1 != nil {
		if !(*as1).ContainsSpecRef(searchURL, "myorg", "1.0.0") {
			t.Errorf("Error: %v is in %v.\n", searchURL, *as1)
		}
	}

	searchURL = "http://mycompany.com/dm/gps"
	if as1 = create_APISpecification(asString, t); as1 != nil {
		if !(*as1).ContainsSpecRef(searchURL, "myorg", "1.0.0") {
			t.Errorf("Error: %v is in %v.\n", searchURL, *as1)
		}
	}

	searchURL = "http://mycompany.com/dm/cpu"
	if as1 = create_APISpecification(asString, t); as1 != nil {
		if !(*as1).ContainsSpecRef(searchURL, "myorg", "1.0.0") {
			t.Errorf("Error: %v is in %v.\n", searchURL, *as1)
		}
	}
}

// Some negative tests for the API Spec search
func Test_APISpecification_not_contains_specref(t *testing.T) {
	var as1 *APISpecList
	asString := `[{"specRef": "http://mycompany.com/dm/gps","organization":"myorg","version": "1.0.0","exclusiveAccess": false,"arch":"arm"}]`

	searchURL := "http://mycompany.com/dm/net"
	if as1 = create_APISpecification(asString, t); as1 != nil {
		if (*as1).ContainsSpecRef(searchURL, "myorg", "1.0.0") {
			t.Errorf("Error: %v is not in %v.\n", searchURL, *as1)
		}
	}

	asString = `[{"specRef": "http://mycompany.com/dm/gps","organization":"myorg","version": "1.0.0","exclusiveAccess": false,"arch":"arm"},
				{"specRef": "http://mycompany.com/dm/cpu","organization":"myorg","version": "1.0.0","exclusiveAccess": false,"arch":"arm"},
				{"specRef": "http://mycompany.com/dm/net","organization":"myorg","version": "1.0.0","exclusiveAccess": false,"arch":"arm"}]`

	searchURL = "http://mycompany.com/dm/nit"
	if as1 = create_APISpecification(asString, t); as1 != nil {
		if (*as1).ContainsSpecRef(searchURL, "myorg", "1.0.0") {
			t.Errorf("Error: %v is not in %v.\n", searchURL, *as1)
		}
	}

	searchURL = "http://mycompany.com/dm/gps"
	if as1 = create_APISpecification(asString, t); as1 != nil {
		if (*as1).ContainsSpecRef(searchURL, "myorg", "2.0.0") {
			t.Errorf("Error: %v is not in %v.\n", searchURL, *as1)
		}
	}

	searchURL = "http://mycompany.com/dm/gps"
	if as1 = create_APISpecification(asString, t); as1 != nil {
		if (*as1).ContainsSpecRef(searchURL, "yourorg", "1.0.0") {
			t.Errorf("Error: %v is not in %v.\n", searchURL, *as1)
		}
	}

	searchURL = ""
	if as1 = create_APISpecification(asString, t); as1 != nil {
		if (*as1).ContainsSpecRef(searchURL, "myorg", "1.0.0") {
			t.Errorf("Error: %v is not in %v.\n", searchURL, *as1)
		}
	}

	asString = `[]`

	searchURL = "http://mycompany.com/dm/nit"
	if as1 = create_APISpecification(asString, t); as1 != nil {
		if (*as1).ContainsSpecRef(searchURL, "myorg", "1.0.0") {
			t.Errorf("Error: %v is not in %v.\n", searchURL, *as1)
		}
	}
}

// Some sameness tests - API spec lists which are the same
func Test_APISpecification_same(t *testing.T) {
	var as1, as2 *APISpecList
	asString1 := `[{"specRef": "http://mycompany.com/dm/gps","organization":"myorg","version":"1.0.0","exclusiveAccess":false,"arch":"arm"}]`
	asString2 := `[{"specRef": "http://mycompany.com/dm/gps","organization":"myorg","version":"1.0.0","exclusiveAccess":false,"arch":"arm"}]`

	if as1 = create_APISpecification(asString1, t); as1 != nil {
		if as2 = create_APISpecification(asString2, t); as2 != nil {
			if !as1.IsSame(*as2, true) {
				t.Errorf("Error: %v and %v are the same.", as1, as2)
			}
		}
	}

	asString1 = `[{"specRef": "http://mycompany.com/dm/gps","organization":"myorg","version":"1.0.0","exclusiveAccess":false,"arch":"arm"}]`
	asString2 = `[{"specRef": "http://mycompany.com/dm/gps","organization":"myorg","version":"2.0.0","exclusiveAccess":false,"arch":"arm"}]`

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
	asString1 := `[{"specRef": "http://mycompany.com/dm/gps","organization":"myorg","version":"1.0.0","exclusiveAccess":false,"arch":"arm"}]`
	asString2 := `[{"specRef": "http://mycompany.com/dm/gps","organization":"myorg","version":"2.0.0","exclusiveAccess":false,"arch":"arm"}]`

	if as1 = create_APISpecification(asString1, t); as1 != nil {
		if as2 = create_APISpecification(asString2, t); as2 != nil {
			if as1.IsSame(*as2, true) {
				t.Errorf("Error: %v and %v are NOT the same.", as1, as2)
			}
		}
	}

	asString1 = `[{"specRef": "http://mycompany.com/dm/gps","organization":"myorg","version":"1.0.0","exclusiveAccess":false,"arch":"arm"}]`
	asString2 = `[{"specRef": "http://mycompany.com/dm/gps2","organization":"myorg","version":"1.0.0","exclusiveAccess":false,"arch":"arm"}]`

	if as1 = create_APISpecification(asString1, t); as1 != nil {
		if as2 = create_APISpecification(asString2, t); as2 != nil {
			if as1.IsSame(*as2, true) {
				t.Errorf("Error: %v and %v are NOT the same.", as1, as2)
			}
		}
	}

	asString1 = `[{"specRef": "http://mycompany.com/dm/gps","organization":"myorg","version":"1.0.0","exclusiveAccess":false,"arch":"arm"}]`
	asString2 = `[{"specRef": "http://mycompany.com/dm/gps","organization":"myorg","version":"1.0.0","exclusiveAccess":true,"arch":"arm"}]`

	if as1 = create_APISpecification(asString1, t); as1 != nil {
		if as2 = create_APISpecification(asString2, t); as2 != nil {
			if as1.IsSame(*as2, true) {
				t.Errorf("Error: %v and %v are NOT the same.", as1, as2)
			}
		}
	}

	asString1 = `[{"specRef": "http://mycompany.com/dm/gps","organization":"myorg","version":"1.0.0","exclusiveAccess":false,"arch":"arm"}]`
	asString2 = `[{"specRef": "http://mycompany.com/dm/gps","organization":"myorg","version":"1.0.0","exclusiveAccess":false,"arch":"amd64"}]`

	if as1 = create_APISpecification(asString1, t); as1 != nil {
		if as2 = create_APISpecification(asString2, t); as2 != nil {
			if as1.IsSame(*as2, true) {
				t.Errorf("Error: %v and %v are NOT the same.", as1, as2)
			}
		}
	}

	asString1 = `[{"specRef": "http://mycompany.com/dm/gps","organization":"myorg","version":"1.0.0","exclusiveAccess":false,"arch":"arm"}]`
	asString2 = `[{"specRef": "http://mycompany.com/dm/gps","organization":"yourorg","version":"1.0.0","exclusiveAccess":false,"arch":"arm"}]`

	if as1 = create_APISpecification(asString1, t); as1 != nil {
		if as2 = create_APISpecification(asString2, t); as2 != nil {
			if as1.IsSame(*as2, true) {
				t.Errorf("Error: %v and %v are NOT the same.", as1, as2)
			}
		}
	}

	asString1 = `[{"specRef": "http://mycompany.com/dm/gps","organization":"myorg","version":"1.0.0","exclusiveAccess":false,"arch":"arm"}]`
	asString2 = `[{"specRef": "http://mycompany.com/dm/gps2","organization":"myorg","version":"2.0.0","exclusiveAccess":false,"arch":"arm"}]`

	if as1 = create_APISpecification(asString1, t); as1 != nil {
		if as2 = create_APISpecification(asString2, t); as2 != nil {
			if as1.IsSame(*as2, false) {
				t.Errorf("Error: %v and %v are NOT the same even ignoring version.", as1, as2)
			}
		}
	}

	asString1 = `[{"specRef": "http://mycompany.com/dm/gps","organization":"myorg","version":"1.0.0","exclusiveAccess":false,"arch":"arm"}]`
	asString2 = `[{"specRef": "http://mycompany.com/dm/gps","organization":"myorg","version":"2.0.0","exclusiveAccess":true,"arch":"arm"}]`

	if as1 = create_APISpecification(asString1, t); as1 != nil {
		if as2 = create_APISpecification(asString2, t); as2 != nil {
			if as1.IsSame(*as2, false) {
				t.Errorf("Error: %v and %v are NOT the same even ignoring version.", as1, as2)
			}
		}
	}

	asString1 = `[{"specRef": "http://mycompany.com/dm/gps","organization":"myorg","version":"1.0.0","exclusiveAccess":false,"arch":"arm"}]`
	asString2 = `[{"specRef": "http://mycompany.com/dm/gps","organization":"myorg","version":"2.0.0","exclusiveAccess":false,"arch":"amd64"}]`

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

	prod1 := `[{"specRef":"http://mycompany.com/dm/gps","organization":"myorg","version":"1.0.0","exclusiveAccess":false,"arch":"amd64"}]`
	con1 := `[{"specRef":"http://mycompany.com/dm/gps","organization":"myorg","version":"1.0.0","exclusiveAccess":false,"arch":"amd64"}]`
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

	prod1 := `[{"specRef":"http://mycompany.com/dm/gps","organization":"myorg","version":"1.5.0","exclusiveAccess":false,"arch":"amd64"}]`
	con1 := `[{"specRef":"http://mycompany.com/dm/gps","organization":"myorg","version":"1.0.0","exclusiveAccess":false,"arch":"amd64"}]`
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

	prod1 := `[{"specRef":"http://mycompany.com/dm/gps","organization":"myorg","version":"2.5.0","exclusiveAccess":false,"arch":"amd64"}]`
	con1 := `[{"specRef":"http://mycompany.com/dm/gps","organization":"myorg","version":"1.0.0","exclusiveAccess":false,"arch":"amd64"}]`
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

	prod1 := `[{"specRef":"http://mycompany.com/dm/gps","organization":"myorg","version":"1.5.9","exclusiveAccess":false,"arch":"amd64"}]`
	con1 := `[{"specRef":"http://mycompany.com/dm/gps","organization":"myorg","version":"[1.0.0,2.0.0)","exclusiveAccess":false,"arch":"amd64"}]`
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

	prod1 := `[{"specRef":"http://mycompany.com/dm/gps","organization":"myorg","version":"1.0.0","exclusiveAccess":false,"arch":"amd64"}]`
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

	prod1 := `[{"specRef":"http://mycompany.com/dm/gps","organization":"myorg","version":"1.0.0","exclusiveAccess":false,"arch":"amd64"}]`
	con1 := `[{"specRef":"http://mycompany.com/dm/gps","organization":"myorg","version":"1.0.0","exclusiveAccess":false,"arch":"armhf"}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := (*prod_as).Supports(*con_as); err == nil {
				t.Errorf("Error: %v does not support %v\n", con1, prod1)
			}
		}
	}
}

func Test_APISpecification_notsupports2(t *testing.T) {
	var prod_as *APISpecList
	var con_as *APISpecList

	prod1 := `[{"specRef":"http://mycompany.com/dm/gps","organization":"myorg","version":"1.0.0","exclusiveAccess":false,"arch":"amd64"}]`
	con1 := `[{"specRef":"http://mycompany.com/dm/gps2","organization":"myorg","version":"1.0.0","exclusiveAccess":false,"arch":"amd64"}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := (*prod_as).Supports(*con_as); err == nil {
				t.Errorf("Error: %v does not support %v\n", con1, prod1)
			}
		}
	}
}

func Test_APISpecification_notsupports3(t *testing.T) {
	var prod_as *APISpecList
	var con_as *APISpecList

	prod1 := `[]`
	con1 := `[{"specRef":"http://mycompany.com/dm/gps","organization":"myorg","version":"1.0.0","exclusiveAccess":false,"arch":"amd64"}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := (*prod_as).Supports(*con_as); err == nil {
				t.Errorf("Error: %v does not support %v\n", con1, prod1)
			}
		}
	}
}

func Test_APISpecification_notsupports4(t *testing.T) {
	var prod_as *APISpecList
	var con_as *APISpecList

	prod1 := `[{"specRef":"http://mycompany.com/dm/gps","organization":"myorg","version":"1.0.0","exclusiveAccess":false,"arch":"amd64"},{"specRef":"http://mycompany.com/dm/gps2","organization":"myorg","version":"1.0.0","exclusiveAccess":false,"arch":"amd64"}]`
	con1 := `[{"specRef":"http://mycompany.com/dm/gps","organization":"myorg","version":"1.0.0","exclusiveAccess":false,"arch":"amd64"}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := (*prod_as).Supports(*con_as); err == nil {
				t.Errorf("Error: %v does not support %v\n", con1, prod1)
			}
		}
	}
}

func Test_APISpecification_notsupports5(t *testing.T) {
	var prod_as *APISpecList
	var con_as *APISpecList

	prod1 := `[{"specRef":"http://mycompany.com/dm/gps","organization":"myorg","version":"2.0.0","exclusiveAccess":false,"arch":"amd64"}]`
	con1 := `[{"specRef":"http://mycompany.com/dm/gps","organization":"myorg","version":"[1.0.0,2)","exclusiveAccess":false,"arch":"amd64"}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := (*prod_as).Supports(*con_as); err == nil {
				t.Errorf("Error: %v does not support %v\n", con1, prod1)
			}
		}
	}
}

func Test_APISpecification_notsupports6(t *testing.T) {
	var prod_as *APISpecList
	var con_as *APISpecList

	prod1 := `[{"specRef":"http://mycompany.com/dm/gps","organization":"myorg","version":"1.0.0","exclusiveAccess":false,"arch":"amd64"}]`
	con1 := `[{"specRef":"http://mycompany.com/dm/gps","organization":"yourorg","version":"[1.0.0,2)","exclusiveAccess":false,"arch":"amd64"}]`
	if prod_as = create_APISpecification(prod1, t); prod_as != nil {
		if con_as = create_APISpecification(con1, t); con_as != nil {
			if err := (*prod_as).Supports(*con_as); err == nil {
				t.Errorf("Error: %v does not support %v\n", con1, prod1)
			}
		}
	}
}

// test no intersection
func Test_APISpecification_GetCommonVersionRanges_success1(t *testing.T) {
	var apiSpecList *APISpecList

	prod := `[{"specRef":"http://mycompany.com/dm/gps","organization":"myorg","version":"2.0.3","exclusiveAccess":false,"arch":"amd64"},
	          {"specRef":"http://mycompany.com/dm/gps","organization":"myorg","version":"[1.0.0,3.0]","exclusiveAccess":false,"arch":"amd64"},
	          {"specRef":"http://mycompany.com/dm/network","organization":"myorg","version":"[1.0.0,3.0)","exclusiveAccess":false,"arch":"amd64"},
	          {"specRef":"http://mycompany.com/dm/network","organization":"myorg","version":"[2.1,5.0)","exclusiveAccess":false,"arch":"amd64"}]`
	if apiSpecList = create_APISpecification(prod, t); apiSpecList != nil {
		common_apispec_list, err := apiSpecList.GetCommonVersionRanges()
		if err != nil {
			t.Errorf("Error: got error but shoulg not be. %v\n", err)
		} else {
			for _, as := range *common_apispec_list {
				if as.SpecRef == "http://mycompany.com/dm/gps" && as.Version != "[2.0.3,3.0.0]" {
					t.Errorf("Error: should have version range [2.0.3,5.0.0], but is %v\n", as)
				}

				if as.SpecRef == "http://mycompany.com/dm/network" && as.Version != "[2.1.0,3.0.0)" {
					t.Errorf("Error: should have version range [2.1.0,3.0.0), but is %v\n", as)
				}
			}
		}
	}
}

func Test_APISpecification_GetCommonVersionRanges_success2(t *testing.T) {
	var apiSpecList *APISpecList

	prod := `[{"specRef":"http://mycompany.com/dm/gps","organization":"myorg","version":"2.0.3","exclusiveAccess":false,"arch":"amd64"},
	          {"specRef":"http://mycompany.com/dm/gps","organization":"myorg","version":"[1.0.0,3.0]","exclusiveAccess":false,"arch":"amd64"},
	          {"specRef":"http://mycompany.com/dm/gps","organization":"myorg","version":"[4.0.0,5.0]","exclusiveAccess":false,"arch":"amd64"},
	          {"specRef":"http://mycompany.com/dm/network","organization":"myorg","version":"5.0","exclusiveAccess":false,"arch":"amd64"}]`
	if apiSpecList = create_APISpecification(prod, t); apiSpecList != nil {
		common_apispec_list, err := apiSpecList.GetCommonVersionRanges()
		if err != nil {
			t.Errorf("Error: got error but shoulg not be. %v\n", err)
		} else {
			for _, as := range *common_apispec_list {
				if as.SpecRef == "http://mycompany.com/dm/gps" {
					t.Errorf("Error: should not have have gps, but has %v\n", as)

				}
			}
		}
	}
}

// test different org
func Test_APISpecification_GetCommonVersionRanges_success3(t *testing.T) {
	var apiSpecList *APISpecList

	prod := `[{"specRef":"http://mycompany.com/dm/gps","organization":"myorg","version":"2.0.3","exclusiveAccess":false,"arch":"amd64"},
	          {"specRef":"http://mycompany.com/dm/gps","organization":"myorg","version":"[1.0.0,3.0]","exclusiveAccess":false,"arch":"amd64"},
	          {"specRef":"http://mycompany.com/dm/gps","organization":"myorg1","version":"[4.0.0,5.0]","exclusiveAccess":false,"arch":"amd64"},
	          {"specRef":"http://mycompany.com/dm/network","organization":"myorg","version":"5.0","exclusiveAccess":false,"arch":"amd64"}]`
	if apiSpecList = create_APISpecification(prod, t); apiSpecList != nil {
		common_apispec_list, err := apiSpecList.GetCommonVersionRanges()
		if err != nil {
			t.Errorf("Error: got error but shoulg not be. %v\n", err)
		} else {
			for _, as := range *common_apispec_list {
				if as.SpecRef == "http://mycompany.com/dm/gps" && as.Org == "myorg" && as.Version != "[2.0.3,3.0.0]" {
					t.Errorf("Error: should have version range [2.0.3,5.0.0], but is %v\n", as)
				}
				if as.SpecRef == "http://mycompany.com/dm/gps" && as.Org == "myorg1" && as.Version != "[4.0.0,5.0.0]" {
					t.Errorf("Error: should have version range [4.0.0,5.0.0], but is %v\n", as)
				}
				if as.SpecRef == "http://mycompany.com/dm/network" && as.Version != "[5.0.0,INFINITY)" {
					t.Errorf("Error: should have version range 5.0.0,INFINITY), but is %v\n", as)
				}

			}
		}
	}
}
