package policy

import (
	"testing"
)

// AgreementProtocolList Tests
// First, some tests where the lists intersect
func Test_AgreementProtocolList_intersects(t *testing.T) {
	var pl1 *AgreementProtocolList
	var pl2 *AgreementProtocolList

	p1 := `[{"name":"ap1"}]`
	p2 := `[{"name":"ap1"}]`
	if pl1 = create_AgreementProtocolList(p1, t); pl1 != nil {
		if pl2 = create_AgreementProtocolList(p2, t); pl2 != nil {
			if pl3, err := pl1.Intersects_With(pl2); err != nil {
				t.Errorf("Error: %v intersects with %v, error was %v\n", p1, p2, err)
			} else if len(*pl3) != 1 {
				t.Errorf("Error: Intersection of %v with %v should have produced 1 intersection, produced %v\n", p1, p2, len(*pl3))
			}
		}
	}

	p1 = `[{"name":"ap2"},{"name":"ap1"}]`
	if pl1 = create_AgreementProtocolList(p1, t); pl1 != nil {
		if pl2 = create_AgreementProtocolList(p2, t); pl2 != nil {
			if pl3, err := pl1.Intersects_With(pl2); err != nil {
				t.Errorf("Error: %v intersects with %v, error was %v\n", p1, p2, err)
			} else if len(*pl3) != 1 {
				t.Errorf("Error: Intersection of %v with %v should have produced 1 intersection, produced %v\n", p1, p2, len(*pl3))
			}
		}
	}

	p2 = `[{"name":"ap2"},{"name":"ap1"}]`
	if pl1 = create_AgreementProtocolList(p1, t); pl1 != nil {
		if pl2 = create_AgreementProtocolList(p2, t); pl2 != nil {
			if pl3, err := pl1.Intersects_With(pl2); err != nil {
				t.Errorf("Error: %v intersects with %v, error was %v\n", p1, p2, err)
			} else if len(*pl3) != 2 {
				t.Errorf("Error: Intersection of %v with %v should have produced 2 intersections, produced %v\n", p1, p2, len(*pl3))
			}
		}
	}

	p2 = `[{"name":"ap2"},{"name":"ap1"},{"name":"ap0"}]`
	if pl1 = create_AgreementProtocolList(p1, t); pl1 != nil {
		if pl2 = create_AgreementProtocolList(p2, t); pl2 != nil {
			if pl3, err := pl1.Intersects_With(pl2); err != nil {
				t.Errorf("Error: %v intersects with %v, error was %v\n", p1, p2, err)
			} else if len(*pl3) != 2 {
				t.Errorf("Error: Intersection of %v with %v should have produced 2 intersections, produced %v\n", p1, p2, len(*pl3))
			}
		}
	}

	p2 = `[]`
	if pl1 = create_AgreementProtocolList(p1, t); pl1 != nil {
		if pl2 = create_AgreementProtocolList(p2, t); pl2 != nil {
			if pl3, err := pl1.Intersects_With(pl2); err != nil {
				t.Errorf("Error: %v intersects with %v, error was %v\n", p1, p2, err)
			} else if len(*pl3) != 2 {
				t.Errorf("Error: Intersection of %v with %v should have produced 2 intersections, produced %v\n", p1, p2, len(*pl3))
			}
		}
	}

	p1 = `[]`
	p2 = `[]`
	if pl1 = create_AgreementProtocolList(p1, t); pl1 != nil {
		if pl2 = create_AgreementProtocolList(p2, t); pl2 != nil {
			if pl3, err := pl1.Intersects_With(pl2); err != nil {
				t.Errorf("Error: %v intersects with %v, error was %v\n", p1, p2, err)
			} else if len(*pl3) != 1 {
				t.Errorf("Error: Intersection of %v with %v should have produced 1 intersections, produced %v\n", p1, p2, len(*pl3))
			}
		}
	}

	p2 = `[{"name":"ap2"}]`
	if pl1 = create_AgreementProtocolList(p1, t); pl1 != nil {
		if pl2 = create_AgreementProtocolList(p2, t); pl2 != nil {
			if pl3, err := pl1.Intersects_With(pl2); err != nil {
				t.Errorf("Error: %v intersects with %v, error was %v\n", p1, p2, err)
			} else if len(*pl3) != 1 {
				t.Errorf("Error: Intersection of %v with %v should have produced 1 intersections, produced %v\n", p1, p2, len(*pl3))
			}
		}
	}

	p1 = `[{"name":"ap2","blockchains":[]}]`
	p2 = `[{"name":"ap2","blockchains":[]}]`
	if pl1 = create_AgreementProtocolList(p1, t); pl1 != nil {
		if pl2 = create_AgreementProtocolList(p2, t); pl2 != nil {
			if pl3, err := pl1.Intersects_With(pl2); err != nil {
				t.Errorf("Error: %v intersects with %v, error was %v\n", p1, p2, err)
			} else if len(*pl3) != 1 {
				t.Errorf("Error: Intersection of %v with %v should have produced 1 intersections, produced %v\n", p1, p2, len(*pl3))
			}
		}
	}

	p1 = `[{"name":"ap2","blockchains":[{"type":"eth","name":"bc1"}]}]`
	p2 = `[{"name":"ap2","blockchains":[{"type":"eth","name":"bc1"}]}]`
	if pl1 = create_AgreementProtocolList(p1, t); pl1 != nil {
		if pl2 = create_AgreementProtocolList(p2, t); pl2 != nil {
			if pl3, err := pl1.Intersects_With(pl2); err != nil {
				t.Errorf("Error: %v intersects with %v, error was %v\n", p1, p2, err)
			} else if len(*pl3) != 1 {
				t.Errorf("Error: Intersection of %v with %v should have produced 1 intersections, produced %v\n", p1, p2, len(*pl3))
			}
		}
	}

	p1 = `[{"name":"ap2","blockchains":[]}]`
	p2 = `[{"name":"ap2","blockchains":[{"type":"eth","name":"bc1"}]}]`
	if pl1 = create_AgreementProtocolList(p1, t); pl1 != nil {
		if pl2 = create_AgreementProtocolList(p2, t); pl2 != nil {
			if pl3, err := pl1.Intersects_With(pl2); err != nil {
				t.Errorf("Error: %v intersects with %v, error was %v\n", p1, p2, err)
			} else if len(*pl3) != 1 {
				t.Errorf("Error: Intersection of %v with %v should have produced 1 intersections, produced %v\n", p1, p2, len(*pl3))
			}
		}
	}

	p1 = `[{"name":"ap2","blockchains":[{"type":"eth","name":"bc1"}]}]`
	p2 = `[{"name":"ap2","blockchains":[]}]`
	if pl1 = create_AgreementProtocolList(p1, t); pl1 != nil {
		if pl2 = create_AgreementProtocolList(p2, t); pl2 != nil {
			if pl3, err := pl1.Intersects_With(pl2); err != nil {
				t.Errorf("Error: %v intersects with %v, error was %v\n", p1, p2, err)
			} else if len(*pl3) != 1 {
				t.Errorf("Error: Intersection of %v with %v should have produced 1 intersections, produced %v\n", p1, p2, len(*pl3))
			}
		}
	}

	p1 = `[{"name":"ap2","blockchains":[{"type":"eth","name":"bc1"}]}]`
	p2 = `[{"name":"ap2","blockchains":[{"type":"eth","name":"bc2"},{"type":"eth","name":"bc1"}]}]`
	if pl1 = create_AgreementProtocolList(p1, t); pl1 != nil {
		if pl2 = create_AgreementProtocolList(p2, t); pl2 != nil {
			if pl3, err := pl1.Intersects_With(pl2); err != nil {
				t.Errorf("Error: %v intersects with %v, error was %v\n", p1, p2, err)
			} else if len(*pl3) != 1 {
				t.Errorf("Error: Intersection of %v with %v should have produced 1 intersections, produced %v\n", p1, p2, len(*pl3))
			}
		}
	}

	p1 = `[{"name":"ap2","blockchains":[{"type":"eth","name":"bc2"},{"type":"eth","name":"bc1"}]}]`
	p2 = `[{"name":"ap2","blockchains":[{"type":"eth","name":"bc1"}]}]`
	if pl1 = create_AgreementProtocolList(p1, t); pl1 != nil {
		if pl2 = create_AgreementProtocolList(p2, t); pl2 != nil {
			if pl3, err := pl1.Intersects_With(pl2); err != nil {
				t.Errorf("Error: %v intersects with %v, error was %v\n", p1, p2, err)
			} else if len(*pl3) != 1 {
				t.Errorf("Error: Intersection of %v with %v should have produced 1 intersections, produced %v\n", p1, p2, len(*pl3))
			} else if len((*pl3)[0].Blockchains) != 1 {
				t.Errorf("Error: Intersection of %v with %v should have produced 1 blockchain intersection, produced %v\n", p1, p2, len((*pl3)[0].Blockchains))
			}
		}
	}

	p1 = `[{"name":"ap2","blockchains":[{"type":"eth","name":"bc2"},{"type":"eth","name":"bc1"}]}]`
	p2 = `[{"name":"ap2","blockchains":[{"type":"eth","name":"bc1"},{"type":"eth","name":"bc2"}]}]`
	if pl1 = create_AgreementProtocolList(p1, t); pl1 != nil {
		if pl2 = create_AgreementProtocolList(p2, t); pl2 != nil {
			if pl3, err := pl1.Intersects_With(pl2); err != nil {
				t.Errorf("Error: %v intersects with %v, error was %v\n", p1, p2, err)
			} else if len(*pl3) != 1 {
				t.Errorf("Error: Intersection of %v with %v should have produced 1 intersections, produced %v\n", p1, p2, len(*pl3))
			} else if len((*pl3)[0].Blockchains) != 2 {
				t.Errorf("Error: Intersection of %v with %v should have produced 2 blockchain intersections, produced %v\n", p1, p2, len((*pl3)[0].Blockchains))
			}
		}
	}

	// Now test with the protocol names that we actually support
	p1 = `[{"name":"Basic","protocolVersion":1}]`
	p2 = `[{"name":"Basic","protocolVersion":1}]`
	if pl1 = create_AgreementProtocolList(p1, t); pl1 != nil {
		if pl2 = create_AgreementProtocolList(p2, t); pl2 != nil {
			if pl3, err := pl1.Intersects_With(pl2); err != nil {
				t.Errorf("Error: %v intersects with %v, error was %v\n", p1, p2, err)
			} else if len(*pl3) != 1 {
				t.Errorf("Error: Intersection of %v with %v should have produced 1 intersections, produced %v\n", p1, p2, len(*pl3))
			} else if (*pl3)[0].ProtocolVersion != 1 {
				t.Errorf("Error: Intersection of %v with %v should have produced protocol version 1, produced %v\n", p1, p2, (*pl3)[0].ProtocolVersion)
			}
		}
	}

	// Now test with the protocol names that we actually support
	p1 = `[{"name":"Basic","protocolVersion":1}]`
	p2 = `[{"name":"Basic"}]`
	if pl1 = create_AgreementProtocolList(p1, t); pl1 != nil {
		if pl2 = create_AgreementProtocolList(p2, t); pl2 != nil {
			if pl3, err := pl1.Intersects_With(pl2); err != nil {
				t.Errorf("Error: %v intersects with %v, error was %v\n", p1, p2, err)
			} else if len(*pl3) != 1 {
				t.Errorf("Error: Intersection of %v with %v should have produced 1 intersections, produced %v\n", p1, p2, len(*pl3))
			} else if (*pl3)[0].ProtocolVersion != 1 {
				t.Errorf("Error: Intersection of %v with %v should have produced protocol version 1, produced %v\n", p1, p2, (*pl3)[0].ProtocolVersion)
			}
		}
	}

	// Now test with the protocol names that we actually support
	p1 = `[{"name":"Citizen Scientist","protocolVersion":2}]`
	p2 = `[{"name":"Citizen Scientist"}]`
	if pl1 = create_AgreementProtocolList(p1, t); pl1 != nil {
		if pl2 = create_AgreementProtocolList(p2, t); pl2 != nil {
			if pl3, err := pl1.Intersects_With(pl2); err != nil {
				t.Errorf("Error: %v intersects with %v, error was %v\n", p1, p2, err)
			} else if len(*pl3) != 1 {
				t.Errorf("Error: Intersection of %v with %v should have produced 1 intersections, produced %v\n", p1, p2, len(*pl3))
			} else if (*pl3)[0].ProtocolVersion != 2 {
				t.Errorf("Error: Intersection of %v with %v should have produced protocol version 1, produced %v\n", p1, p2, (*pl3)[0].ProtocolVersion)
			}
		}
	}

	// Now test with the protocol names that we actually support
	p1 = `[{"name":"Citizen Scientist","protocolVersion":2,"blockchains":[{"type":"ethereum","name":"bc2"},{"type":"ethereum","name":"bc1"}]}]`
	p2 = `[{"name":"Citizen Scientist"}]`
	if pl1 = create_AgreementProtocolList(p1, t); pl1 != nil {
		if pl2 = create_AgreementProtocolList(p2, t); pl2 != nil {
			if pl3, err := pl1.Intersects_With(pl2); err != nil {
				t.Errorf("Error: %v intersects with %v, error was %v\n", p1, p2, err)
			} else if len(*pl3) != 1 {
				t.Errorf("Error: Intersection of %v with %v should have produced 1 intersections, produced %v\n", p1, p2, len(*pl3))
			} else if (*pl3)[0].ProtocolVersion != 2 {
				t.Errorf("Error: Intersection of %v with %v should have produced protocol version 1, produced %v\n", p1, p2, (*pl3)[0].ProtocolVersion)
			} else if len((*pl3)[0].Blockchains) != 2 {
				t.Errorf("Error: Intersection of %v with %v should have produced 2 blockchains, produced %v\n", p1, p2, len((*pl3)[0].Blockchains))
			}
		}
	}

}

// Second, some tests where the lists are incompatible
func Test_AgreementProtocolList_no_intersect(t *testing.T) {
	var pl1 *AgreementProtocolList
	var pl2 *AgreementProtocolList

	p1 := `[{"name":"ap1"}]`
	p2 := `[{"name":"ap2"}]`
	if pl1 = create_AgreementProtocolList(p1, t); pl1 != nil {
		if pl2 = create_AgreementProtocolList(p2, t); pl2 != nil {
			if pl3, err := pl1.Intersects_With(pl2); err == nil {
				t.Errorf("Error: %v does not intersect with %v, detected intersection of %v\n", p1, p2, *pl3)
			}
		}
	}

	p1 = `[{"name":"ap1"},{"name":"ap3"}]`
	if pl1 = create_AgreementProtocolList(p1, t); pl1 != nil {
		if pl2 = create_AgreementProtocolList(p2, t); pl2 != nil {
			if pl3, err := pl1.Intersects_With(pl2); err == nil {
				t.Errorf("Error: %v does not intersect with %v, detected intersection of %v\n", p1, p2, *pl3)
			}
		}
	}

	p2 = `[{"name":"ap2"},{"name":"ap4"}]`
	if pl1 = create_AgreementProtocolList(p1, t); pl1 != nil {
		if pl2 = create_AgreementProtocolList(p2, t); pl2 != nil {
			if pl3, err := pl1.Intersects_With(pl2); err == nil {
				t.Errorf("Error: %v does not intersect with %v, detected intersection of %v\n", p1, p2, *pl3)
			}
		}
	}

	p1 = `[{"name":"ap2","blockchains":[{"type":"eth","name":"bc2"}]}]`
	p2 = `[{"name":"ap2","blockchains":[{"type":"eth","name":"bc1"}]}]`
	if pl1 = create_AgreementProtocolList(p1, t); pl1 != nil {
		if pl2 = create_AgreementProtocolList(p2, t); pl2 != nil {
			if pl3, err := pl1.Intersects_With(pl2); err == nil {
				t.Errorf("Error: %v does not intersect with %v, detected intersection of %v\n", p1, p2, *pl3)
			}
		}
	}

	p1 = `[{"name":"ap2","blockchains":[{"type":"eth1","name":"bc2"},{"type":"eth1","name":"bc1"}]}]`
	p2 = `[{"name":"ap2","blockchains":[{"type":"eth2","name":"bc1"},{"type":"eth2","name":"bc2"}]}]`
	if pl1 = create_AgreementProtocolList(p1, t); pl1 != nil {
		if pl2 = create_AgreementProtocolList(p2, t); pl2 != nil {
			if pl3, err := pl1.Intersects_With(pl2); err == nil {
				t.Errorf("Error: %v does not intersect with %v, detected intersection of %v\n", p1, p2, *pl3)
			}
		}
	}

	p1 = `[{"name":"ap2","blockchains":[{"type":"eth1","name":"bc1"}]}]`
	p2 = `[{"name":"ap2","blockchains":[{"type":"eth2","name":"bc1"}]}]`
	if pl1 = create_AgreementProtocolList(p1, t); pl1 != nil {
		if pl2 = create_AgreementProtocolList(p2, t); pl2 != nil {
			if pl3, err := pl1.Intersects_With(pl2); err == nil {
				t.Errorf("Error: %v does not intersect with %v, detected intersection of %v\n", p1, p2, *pl3)
			}
		}
	}

}

//Some tests on the Single_Element API
func Test_AgreementProtocolList_single_element(t *testing.T) {
	var pl1 *AgreementProtocolList

	pb := `[{"name":"`+BasicProtocol+`"}]`
	if pb1 := create_AgreementProtocolList(pb, t); pb1 == nil || len(*pb1) != 1 {
		t.Errorf("Error: returned %v, should have returned %v\n", pb1, pb)
	} else {

		p1 := `[{"name":"ap1"}]`
		if pl1 = create_AgreementProtocolList(p1, t); pl1 != nil {
			if pl2 := pl1.Single_Element(); !pl1.IsSame(*pl2) {
				t.Errorf("Error: returned %v, should have returned %v\n", pl2, pl1)
			}
		}

		p1 = `[{"name":"ap1"},{"name":"Basic"}]`
		if pl1 = create_AgreementProtocolList(p1, t); pl1 != nil {
			if pl2 := pl1.Single_Element(); !pl2.IsSame(*pb1) {
				t.Errorf("Error: returned %v, should have returned %v\n", pl2, pb1)
			}
		}
	}

	p1 := `[{"name":"ap2","blockchains":[{"type":"eth","name":"bc2"},{"type":"eth","name":"bc1"}]}]`
	p2 := `[{"name":"ap2","blockchains":[{"type":"eth","name":"bc1"},{"type":"eth","name":"bc2"}]}]`
	if pl1 := create_AgreementProtocolList(p1, t); pl1 != nil {
		if pl2 := create_AgreementProtocolList(p2, t); pl2 != nil {
			if pl3, err := pl1.Intersects_With(pl2); err != nil {
				t.Errorf("Error: %v intersects with %v, error was %v\n", p1, p2, err)
			} else if len(*pl3) != 1 {
				t.Errorf("Error: Intersection of %v with %v should have produced 1 intersections, produced %v\n", p1, p2, len(*pl3))
			} else if len((*pl3)[0].Blockchains) != 2 {
				t.Errorf("Error: Intersection of %v with %v should have produced 2 blockchains, produced %v\n", p1, p2, len((*pl3)[0].Blockchains))
			} else if pl4 := pl3.Single_Element(); len((*pl4)[0].Blockchains) != 1 {
				t.Errorf("Error: Single_Element of %v should have produced 1 blockchains, produced %v\n", pl3, len((*pl4)[0].Blockchains))
			}
		}
	}

}

func Test_AgreementProtocol_init(t *testing.T) {
	agp := AgreementProtocol_Factory(CitizenScientist)
	agp.Initialize()
	if agp.Blockchains[0].Type != Ethereum_bc {
		t.Errorf("Error: blockchain type was not correctly inited, is %v\n", agp.Blockchains[0])
	}
}

func Test_AgreementProtocol_isvalid(t *testing.T) {
	agp := AgreementProtocol_Factory(CitizenScientist)
	if err := agp.IsValid(); err != nil {
		t.Errorf("Error: agreement protocol object is valid %v\n", agp)
	}

	p1 := `[{"name":"Basic","blockchains":[]},{"name":"Basic"},{"name":"Citizen Scientist"},{"name":"Citizen Scientist","blockchains":[]},{"name":"Citizen Scientist","blockchains":[{}]},{"name":"Citizen Scientist","blockchains":[{"name":"fred"}]},{"name":"Citizen Scientist","blockchains":[{"name":"fred","type":"ethereum"}]}]`
	if pl1 := create_AgreementProtocolList(p1, t); pl1 != nil {
		for _, agp := range (*pl1) {
			if err := agp.IsValid(); err != nil {
				t.Errorf("Error: agreement protocol object is valid %v\n", agp)
			}
		}
	}
}

func Test_AgreementProtocol_is_notvalid(t *testing.T) {

	p1 := `[{"name":"Basic","blockchains":[{"type":"ethereum"}]}]`
	if pl1 := create_AgreementProtocolList(p1, t); pl1 != nil {
		for _, agp := range (*pl1) {
			if err := agp.IsValid(); err == nil {
				t.Errorf("Error: agreement protocol object is not valid %v\n", agp)
			}
		}
	}

	p1 = `[{"name":"Basic","blockchains":[{"type":"fred"}]}]`
	if pl1 := create_AgreementProtocolList(p1, t); pl1 != nil {
		for _, agp := range (*pl1) {
			if err := agp.IsValid(); err == nil {
				t.Errorf("Error: agreement protocol object is not valid %v\n", agp)
			}
		}
	}

	p1 = `[{"name":"fred","blockchains":[{"type":"ethereum"}]}]`
	if pl1 := create_AgreementProtocolList(p1, t); pl1 != nil {
		for _, agp := range (*pl1) {
			if err := agp.IsValid(); err == nil {
				t.Errorf("Error: agreement protocol object is not valid %v\n", agp)
			}
		}
	}

	p1 = `[{"name":"Citizen Scientist","blockchains":[{"type":"fred"}]}]`
	if pl1 := create_AgreementProtocolList(p1, t); pl1 != nil {
		for _, agp := range (*pl1) {
			if err := agp.IsValid(); err == nil {
				t.Errorf("Error: agreement protocol object is not valid %v\n", agp)
			}
		}
	}
}

func Test_AgreementProtocol_convert1(t *testing.T) {

	bc := map[string]interface{}{"name":"blue"}
	bcList := []interface{}{bc}
	agp := map[string]interface{}{"name":"blue","blockchains":bcList}
	agpList := []interface{}{agp}

	if cList, err := ConvertToAgreementProtocolList(agpList); err != nil {
		t.Errorf("Error: converting list, error: %v\n", err)
	} else if cList == nil {
		t.Errorf("Error: converted list is empty\n")
	} else if len(*cList) != 1 {
		t.Errorf("Error: converted list should have 1 element, is %v\n", *cList)
	} else if len((*cList)[0].Blockchains) != 1 {
		t.Errorf("Error: converted list should have 1 blockchain element, is %v\n", (*cList)[0].Blockchains)
	}

}

func Test_AgreementProtocol_convert2(t *testing.T) {

	bc1 := map[string]interface{}{"name":"blue"}
	bc2 := map[string]interface{}{"name":"red","type":"hyper"}
	bcList1 := []interface{}{bc1, bc2}
	agp1 := map[string]interface{}{"name":"colors","blockchains":bcList1}

	bc3 := map[string]interface{}{"name":"one","type":"arthimetic"}
	bcList2 := []interface{}{bc3}
	agp2 := map[string]interface{}{"name":"numbers","blockchains":bcList2}
	agpList := []interface{}{agp1, agp2}

	if cList, err := ConvertToAgreementProtocolList(agpList); err != nil {
		t.Errorf("Error: converting list, error: %v\n", err)
	} else if cList == nil {
		t.Errorf("Error: converted list is empty\n")
	} else if len(*cList) != 2 {
		t.Errorf("Error: converted list should have 2 element, is %v\n", *cList)
	} else if len((*cList)[0].Blockchains) != 2 {
		t.Errorf("Error: first converted list element should have 2 blockchain elements, is %v\n", (*cList)[0].Blockchains)
	} else if len((*cList)[1].Blockchains) != 1 {
		t.Errorf("Error: second converted list element should have 1 blockchain elements, is %v\n", (*cList)[1].Blockchains)
	}

}

func Test_AgreementProtocolList_isSame(t *testing.T) {

	var pl1 *AgreementProtocolList
	var pl2 *AgreementProtocolList

	pa := `[{"name":"`+BasicProtocol+`"}]`
	pb := `[{"name":"`+BasicProtocol+`"}]`
	if pl1 = create_AgreementProtocolList(pa, t); pl1 == nil || len(*pl1) != 1 {
		t.Errorf("Error: returned %v, should have returned %v\n", pl1, pa)
	} else if pl2 = create_AgreementProtocolList(pb, t); pl2 == nil || len(*pl2) != 1 {
		t.Errorf("Error: returned %v, should have returned %v\n", pl2, pb)
	} else if !pl1.IsSame(*pl2) {
		t.Errorf("Error: the lists are the same: %v %v\n", pl1, pl2)
	}

	pa = `[]`
	pb = `[]`
	if pl1 = create_AgreementProtocolList(pa, t); pl1 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", pl1, pa)
	} else if pl2 = create_AgreementProtocolList(pb, t); pl2 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", pl2, pb)
	} else if !pl1.IsSame(*pl2) {
		t.Errorf("Error: the lists are the same: %v %v\n", pl1, pl2)
	}

}

func Test_AgreementProtocolList_is_not_Same(t *testing.T) {

	var pl1 *AgreementProtocolList
	var pl2 *AgreementProtocolList

	pa := `[{"name":"`+CitizenScientist+`"}]`
	pb := `[{"name":"`+BasicProtocol+`"}]`
	if pl1 = create_AgreementProtocolList(pa, t); pl1 == nil || len(*pl1) != 1 {
		t.Errorf("Error: returned %v, should have returned %v\n", pl1, pa)
	} else if pl2 = create_AgreementProtocolList(pb, t); pl2 == nil || len(*pl2) != 1 {
		t.Errorf("Error: returned %v, should have returned %v\n", pl2, pb)
	} else if pl1.IsSame(*pl2) {
		t.Errorf("Error: the lists are not the same: %v %v\n", pl1, pl2)
	}

	pa = `[]`
	pb = `[{"name":"`+BasicProtocol+`"}]`
	if pl1 = create_AgreementProtocolList(pa, t); pl1 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", pl1, pa)
	} else if pl2 = create_AgreementProtocolList(pb, t); pl2 == nil || len(*pl2) != 1 {
		t.Errorf("Error: returned %v, should have returned %v\n", pl2, pb)
	} else if pl1.IsSame(*pl2) {
		t.Errorf("Error: the lists are not the same: %v %v\n", pl1, pl2)
	}

	pa = `[{"name":"`+BasicProtocol+`"}]`
	pb = `[]`
	if pl1 = create_AgreementProtocolList(pa, t); pl1 == nil || len(*pl1) != 1 {
		t.Errorf("Error: returned %v, should have returned %v\n", pl1, pa)
	} else if pl2 = create_AgreementProtocolList(pb, t); pl2 == nil {
		t.Errorf("Error: returned %v, should have returned %v\n", pl2, pb)
	} else if pl1.IsSame(*pl2) {
		t.Errorf("Error: the lists are not the same: %v %v\n", pl1, pl2)
	}

	pa = `[{"name":"`+BasicProtocol+`"}]`
	pb = `[{"name":"`+BasicProtocol+`"},{"name":"`+CitizenScientist+`"}]`
	if pl1 = create_AgreementProtocolList(pa, t); pl1 == nil || len(*pl1) != 1 {
		t.Errorf("Error: returned %v, should have returned %v\n", pl1, pa)
	} else if pl2 = create_AgreementProtocolList(pb, t); pl2 == nil || len(*pl2) != 2 {
		t.Errorf("Error: returned %v, should have returned %v\n", pl2, pb)
	} else if pl1.IsSame(*pl2) {
		t.Errorf("Error: the lists are not the same: %v %v\n", pl1, pl2)
	}

}

func Test_AgreementProtocolList_FindByName(t *testing.T) {

	var pl1 *AgreementProtocolList

	pa := `[{"name":"`+CitizenScientist+`","blockchains":[{"name":"fred"}]}]`

	if pl1 = create_AgreementProtocolList(pa, t); pl1 == nil || len(*pl1) != 1 {
		t.Errorf("Error: returned %v, should have returned %v\n", pl1, pa)
	} else if agp := pl1.FindByName(CitizenScientist); agp == nil {
		t.Errorf("Error: the list contains AGP %v that we searched for in %v\n", CitizenScientist, pl1)
	} else if len(agp.Blockchains) != 1 {
		t.Errorf("Error: lost the blockchain list, should have 1 element, is %v", agp.Blockchains)
	}

	pa = `[{"name":"`+CitizenScientist+`","blockchains":[{"name":"fred"}]},{"name":"fred","blockchains":[{"name":"fred"}]}]`

	if pl1 = create_AgreementProtocolList(pa, t); pl1 == nil || len(*pl1) != 2 {
		t.Errorf("Error: returned %v, should have returned %v\n", pl1, pa)
	} else if agp := pl1.FindByName(CitizenScientist); agp == nil {
		t.Errorf("Error: the list contains AGP %v that we searched for in %v\n", CitizenScientist, pl1)
	} else if len(agp.Blockchains) != 1 {
		t.Errorf("Error: lost the blockchain list, should have 1 element, is %v", agp.Blockchains)
	}

	pa = `[{"name":"ethel","blockchains":[{"name":"fred"}]},{"name":"`+CitizenScientist+`","blockchains":[{"name":"fred"}]},{"name":"fred","blockchains":[{"name":"fred"}]}]`

	if pl1 = create_AgreementProtocolList(pa, t); pl1 == nil || len(*pl1) != 3 {
		t.Errorf("Error: returned %v, should have returned %v\n", pl1, pa)
	} else if agp := pl1.FindByName(CitizenScientist); agp == nil {
		t.Errorf("Error: the list contains AGP %v that we searched for in %v\n", CitizenScientist, pl1)
	} else if len(agp.Blockchains) != 1 {
		t.Errorf("Error: lost the blockchain list, should have 1 element, is %v", agp.Blockchains)
	}

	pa = `[{"name":"ethel","blockchains":[{"name":"fred"}]},{"name":"`+CitizenScientist+`","blockchains":[{"name":"fred"}]}]`

	if pl1 = create_AgreementProtocolList(pa, t); pl1 == nil || len(*pl1) != 2 {
		t.Errorf("Error: returned %v, should have returned %v\n", pl1, pa)
	} else if agp := pl1.FindByName(CitizenScientist); agp == nil {
		t.Errorf("Error: the list contains AGP %v that we searched for in %v\n", CitizenScientist, pl1)
	} else if len(agp.Blockchains) != 1 {
		t.Errorf("Error: lost the blockchain list, should have 1 element, is %v", agp.Blockchains)
	}

}

func Test_AgreementProtocolList_not_FindByName(t *testing.T) {

	var pl1 *AgreementProtocolList

	pa := `[{"name":"`+CitizenScientist+`","blockchains":[{"name":"fred"}]}]`

	if pl1 = create_AgreementProtocolList(pa, t); pl1 == nil || len(*pl1) != 1 {
		t.Errorf("Error: returned %v, should have returned %v\n", pl1, pa)
	} else if agp := pl1.FindByName("fred"); agp != nil {
		t.Errorf("Error: the list does not contain AGP %v that we searched for in %v\n", "fred", pl1)
	}

	pa = `[]`

	if pl1 = create_AgreementProtocolList(pa, t); pl1 == nil || len(*pl1) != 0 {
		t.Errorf("Error: returned %v, should have returned %v\n", pl1, pa)
	} else if agp := pl1.FindByName("fred"); agp != nil {
		t.Errorf("Error: the list does not contain AGP %v that we searched for in %v\n", "fred", pl1)
	}

	pa = `[{"name":"`+CitizenScientist+`","blockchains":[{"name":"fred"}]},{"name":"`+CitizenScientist+`","blockchains":[{"name":"fred"}]}]`

	if pl1 = create_AgreementProtocolList(pa, t); pl1 == nil || len(*pl1) != 2 {
		t.Errorf("Error: returned %v, should have returned %v\n", pl1, pa)
	} else if agp := pl1.FindByName("fred"); agp != nil {
		t.Errorf("Error: the list does not contain AGP %v that we searched for in %v\n", "fred", pl1)
	}

}
