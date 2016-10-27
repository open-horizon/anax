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
}
