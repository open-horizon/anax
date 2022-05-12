//go:build unit
// +build unit

package policy

import (
	"testing"
)

// BlockchainList Tests

func Test_IntersectsWith(t *testing.T) {

	var prod_bl *BlockchainList
	var con_bl *BlockchainList

	con1 := `[]`
	prod1 := `[]`

	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if bl, err := prod_bl.Intersects_With(con_bl, "", ""); err != nil {
				t.Errorf("Error: %v intersects with %v\n", prod1, con1)
			} else if len(*bl) != 0 {
				t.Errorf("Error: %v is not empty\n", bl)
			}
		}
	}

	con1 = `[{"type":"ethereum","name":"bc1"}]`
	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if bl, err := prod_bl.Intersects_With(con_bl, "", ""); err != nil {
				t.Errorf("Error: %v intersects with %v\n", prod1, con1)
			} else if len(*bl) != 1 {
				t.Errorf("Error: %v is not empty\n", bl)
			}
		}
	}

	con1 = `[]`
	prod1 = `[{"type":"ethereum","name":"bc1"}]`
	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if bl, err := prod_bl.Intersects_With(con_bl, "", ""); err != nil {
				t.Errorf("Error: %v intersects with %v\n", prod1, con1)
			} else if len(*bl) != 1 {
				t.Errorf("Error: %v is not empty\n", bl)
			}
		}
	}

	con1 = `[{"type":"ethereum","name":""}]`
	prod1 = `[{"type":"ethereum","name":""}]`
	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if bl, err := prod_bl.Intersects_With(con_bl, "", ""); err != nil {
				t.Errorf("Error: %v intersects with %v\n", prod1, con1)
			} else if len(*bl) != 1 {
				t.Errorf("Error: %v is not empty\n", bl)
			}
		}
	}

	con1 = `[{"type":"ethereum"}]`
	prod1 = `[{"type":"ethereum"}]`
	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if bl, err := prod_bl.Intersects_With(con_bl, "", ""); err != nil {
				t.Errorf("Error: %v intersects with %v\n", prod1, con1)
			} else if len(*bl) != 1 {
				t.Errorf("Error: %v is not empty\n", bl)
			}
		}
	}

	con1 = `[{"type":"ethereum","name":"bc1"}]`
	prod1 = `[{"type":"ethereum","name":"bc1"}]`
	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if bl, err := prod_bl.Intersects_With(con_bl, "", ""); err != nil {
				t.Errorf("Error: %v intersects with %v\n", prod1, con1)
			} else if len(*bl) != 1 {
				t.Errorf("Error: %v is not empty\n", bl)
			}
		}
	}

	con1 = `[{"type":"ethereum","name":"bc2"},{"type":"ethereum","name":"bc1"}]`
	prod1 = `[{"type":"ethereum","name":"bc1"}]`
	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if bl, err := prod_bl.Intersects_With(con_bl, "", ""); err != nil {
				t.Errorf("Error: %v intersects with %v\n", prod1, con1)
			} else if len(*bl) != 1 {
				t.Errorf("Error: %v is not empty\n", bl)
			}
		}
	}

	con1 = `[{"type":"ethereum","name":"bc1"}]`
	prod1 = `[{"type":"ethereum","name":"bc2"},{"type":"ethereum","name":"bc1"}]`
	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if bl, err := prod_bl.Intersects_With(con_bl, "", ""); err != nil {
				t.Errorf("Error: %v intersects with %v\n", prod1, con1)
			} else if len(*bl) != 1 {
				t.Errorf("Error: %v is not empty\n", bl)
			}
		}
	}

	con1 = `[{"name":"bc1"}]`
	prod1 = `[{"type":"ethereum","name":"bc1"}]`
	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if bl, err := prod_bl.Intersects_With(con_bl, "ethereum", ""); err != nil {
				t.Errorf("Error: %v intersects with %v\n", prod1, con1)
			} else if len(*bl) != 1 {
				t.Errorf("Error: %v is not empty\n", bl)
			}
		}
	}

	con1 = `[{"name":"bc1"}]`
	prod1 = `[{"type":"fred","name":"bc1"},{"type":"ethereum","name":"bc1"}]`
	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if bl, err := prod_bl.Intersects_With(con_bl, "ethereum", ""); err != nil {
				t.Errorf("Error: %v intersects with %v\n", prod1, con1)
			} else if len(*bl) != 1 {
				t.Errorf("Error: %v is not empty\n", bl)
			}
		}
	}

	prod1 = `[{"name":"bc1"}]`
	con1 = `[{"type":"fred","name":"bc1"},{"type":"ethereum","name":"bc1"}]`
	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if bl, err := prod_bl.Intersects_With(con_bl, "ethereum", ""); err != nil {
				t.Errorf("Error: %v intersects with %v\n", prod1, con1)
			} else if len(*bl) != 1 {
				t.Errorf("Error: %v is not empty\n", bl)
			}
		}
	}

	prod1 = `[]`
	con1 = `[{"type":"fred","name":"bc1"},{"type":"ethereum","name":"bc1"}]`
	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if bl, err := prod_bl.Intersects_With(con_bl, "ethereum", ""); err != nil {
				t.Errorf("Error: %v intersects with %v\n", prod1, con1)
			} else if len(*bl) != 2 {
				t.Errorf("Error: %v should have 2 elements\n", bl)
			}
		}
	}

	prod1 = `[{"type":"fred","name":"bc1"},{"type":"ethereum","name":"bc1"},{"type":"ethel","name":"bc1"}]`
	con1 = `[{"type":"ethel","name":"bc1"},{"type":"fred","name":"bc1"}]`
	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if bl, err := prod_bl.Intersects_With(con_bl, "ethereum", ""); err != nil {
				t.Errorf("Error: %v intersects with %v\n", prod1, con1)
			} else if len(*bl) != 2 {
				t.Errorf("Error: %v should have 2 elements\n", bl)
			} else if (*bl)[0].Type != "fred" {
				t.Errorf("Error: first element in intersect is %v, should be fred.", (*bl)[0])
			} else if (*bl)[1].Type != "ethel" {
				t.Errorf("Error: second element in intersect is %v, should be ethel.", (*bl)[1])
			}
		}
	}

	con1 = `[{"name":"bc1","organization":"ibm"}]`
	prod1 = `[{"type":"ethereum","name":"bc1"}]`
	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if bl, err := prod_bl.Intersects_With(con_bl, "ethereum", "ibm"); err != nil {
				t.Errorf("Error: %v intersects with %v\n", prod1, con1)
			} else if len(*bl) != 1 {
				t.Errorf("Error: %v is not empty\n", bl)
			}
		}
	}

	con1 = `[{"name":"bc1"}]`
	prod1 = `[{"type":"fred","name":"bc1"},{"type":"ethereum","name":"bc1","organization":"ibm"}]`
	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if bl, err := prod_bl.Intersects_With(con_bl, "ethereum", "ibm"); err != nil {
				t.Errorf("Error: %v intersects with %v\n", prod1, con1)
			} else if len(*bl) != 1 {
				t.Errorf("Error: %v is not empty\n", bl)
			}
		}
	}

	prod1 = `[{"name":"bc1","organization":"ibm"}]`
	con1 = `[{"type":"fred","name":"bc1"},{"type":"ethereum","name":"bc1"}]`
	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if bl, err := prod_bl.Intersects_With(con_bl, "ethereum", "ibm"); err != nil {
				t.Errorf("Error: %v intersects with %v\n", prod1, con1)
			} else if len(*bl) != 1 {
				t.Errorf("Error: %v is not empty\n", bl)
			}
		}
	}

	prod1 = `[]`
	con1 = `[{"type":"fred","name":"bc1"},{"type":"ethereum","name":"bc1"}]`
	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if bl, err := prod_bl.Intersects_With(con_bl, "ethereum", "ibm"); err != nil {
				t.Errorf("Error: %v intersects with %v\n", prod1, con1)
			} else if len(*bl) != 2 {
				t.Errorf("Error: %v should have 2 elements\n", bl)
			}
		}
	}

	prod1 = `[{"type":"fred","name":"bc1","organization":"ibm"},{"type":"ethereum","name":"bc1"},{"type":"ethel","name":"bc1"}]`
	con1 = `[{"type":"ethel","name":"bc1","organization":"ibm"},{"type":"fred","name":"bc1"}]`
	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if bl, err := prod_bl.Intersects_With(con_bl, "ethereum", "ibm"); err != nil {
				t.Errorf("Error: %v intersects with %v\n", prod1, con1)
			} else if len(*bl) != 2 {
				t.Errorf("Error: %v should have 2 elements\n", bl)
			} else if (*bl)[0].Type != "fred" {
				t.Errorf("Error: first element in intersect is %v, should be fred.", (*bl)[0])
			} else if (*bl)[1].Type != "ethel" {
				t.Errorf("Error: second element in intersect is %v, should be ethel.", (*bl)[1])
			}
		}
	}

	con1 = `[{"name":"bc1","organization":"me"}]`
	prod1 = `[{"name":"bc1","organization":"me"}]`
	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if bl, err := prod_bl.Intersects_With(con_bl, "ethereum", ""); err != nil {
				t.Errorf("Error: %v intersects with %v\n", prod1, con1)
			} else if len(*bl) != 1 {
				t.Errorf("Error: %v is not empty\n", bl)
			}
		}
	}
}

func Test_NonIntersectsWith(t *testing.T) {

	var prod_bl *BlockchainList
	var con_bl *BlockchainList

	con1 := `[{"type":"ethereum","name":"bc1"}]`
	prod1 := `[{"type":"ethereum","name":"bc2"},{"type":"ethereum","name":"bc3"}]`

	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if _, err := prod_bl.Intersects_With(con_bl, "", ""); err == nil {
				t.Errorf("Error: %v doesnt intersect with %v\n", prod1, con1)
			}
		}
	}

	con1 = `[{"type":"ethereum","name":"bc2"},{"type":"ethereum","name":"bc3"}]`
	prod1 = `[{"type":"ethereum","name":"bc1"}]`

	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if _, err := prod_bl.Intersects_With(con_bl, "", ""); err == nil {
				t.Errorf("Error: %v doesnt intersect with %v\n", prod1, con1)
			}
		}
	}

	con1 = `[{"type":"hyperledger","name":"bc1"}]`
	prod1 = `[{"type":"ethereum","name":"bc1"}]`

	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if _, err := prod_bl.Intersects_With(con_bl, "", ""); err == nil {
				t.Errorf("Error: %v doesnt intersect with %v\n", prod1, con1)
			}
		}
	}

	con1 = `[{"type":"hyperledger","name":"bc1"}]`
	prod1 = `[{"type":"hyperledger","name":"bc2"}]`

	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if _, err := prod_bl.Intersects_With(con_bl, "", ""); err == nil {
				t.Errorf("Error: %v doesnt intersect with %v\n", prod1, con1)
			}
		}
	}

	con1 = `[{"type":"ethereum","name":"bc2"},{"type":"hyperledger","name":"bc1"}]`
	prod1 = `[{"type":"ethereum","name":"bc1"},{"type":"hyperledger","name":"bc2"}]`

	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if _, err := prod_bl.Intersects_With(con_bl, "", ""); err == nil {
				t.Errorf("Error: %v doesnt intersect with %v\n", prod1, con1)
			}
		}
	}

	con1 = `[{"name":"bc2"},{"type":"hyperledger","name":"bc1"}]`
	prod1 = `[{"name":"bc1"},{"type":"hyperledger","name":"bc2"}]`

	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if _, err := prod_bl.Intersects_With(con_bl, "ethereum", ""); err == nil {
				t.Errorf("Error: %v doesnt intersect with %v\n", prod1, con1)
			}
		}
	}

	con1 = `[{"name":"bc2"},{"type":"hyperledger","name":"bc1"}]`
	prod1 = `[{"type":"ethereum","name":"bc1"},{"type":"hyperledger","name":"bc2"}]`

	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if _, err := prod_bl.Intersects_With(con_bl, "ethereum", ""); err == nil {
				t.Errorf("Error: %v doesnt intersect with %v\n", prod1, con1)
			}
		}
	}

	prod1 = `[{"name":"bc2"},{"type":"hyperledger","name":"bc1"}]`
	con1 = `[{"type":"ethereum","name":"bc1"},{"type":"hyperledger","name":"bc2"}]`

	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if _, err := prod_bl.Intersects_With(con_bl, "ethereum", ""); err == nil {
				t.Errorf("Error: %v doesnt intersect with %v\n", prod1, con1)
			}
		}
	}

	con1 = `[{"name":"bc2","organization":"me"},{"type":"hyperledger","name":"bc1","organization":"me"}]`
	prod1 = `[{"name":"bc1"},{"type":"hyperledger","name":"bc2"}]`

	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if _, err := prod_bl.Intersects_With(con_bl, "hyperledger", "ibm"); err == nil {
				t.Errorf("Error: %v doesnt intersect with %v\n", prod1, con1)
			}
		}
	}

	con1 = `[{"name":"bc2","organization":"me"}]`
	prod1 = `[{"name":"bc2"}]`

	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if _, err := prod_bl.Intersects_With(con_bl, "hyperledger", "ibm"); err == nil {
				t.Errorf("Error: %v doesnt intersect with %v\n", prod1, con1)
			}
		}
	}

	prod1 = `[{"name":"bc2","organization":"me"},{"type":"hyperledger","name":"bc1"}]`
	con1 = `[{"type":"hyperledger","name":"bc1","organization":"me"},{"type":"hyperledger","name":"bc2"}]`

	if prod_bl = create_BlockchainList(prod1, t); prod_bl != nil {
		if con_bl = create_BlockchainList(con1, t); con_bl != nil {
			if _, err := prod_bl.Intersects_With(con_bl, "hyperledger", "ibm"); err == nil {
				t.Errorf("Error: %v doesnt intersect with %v\n", prod1, con1)
			}
		}
	}
}
