package policy

import (
	"errors"
	"fmt"
)

// The purpose of this file is to abstract the operations on the Blockchain object and
// its List type.
//

const Ethereum_bc = "ethereum"
const Default_Blockchain_name = "bluehorizon"
const Default_Blockchain_org = "IBM"

// This struct indicates the type and instance of blockchain to be used by the policy
type BlockchainList []Blockchain

func (a BlockchainList) IsSame(compare BlockchainList) bool {
	for _, bc := range a {
		found := false
		for _, compareBC := range compare {
			if bc.Same_Blockchain(&compareBC, "", "") {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

type Blockchain struct {
	Type string `json:"type"`         // The type of blockchain
	Name string `json:"name"`         // The name of the blockchain instance in the exchange,it is specific to the value of the type
	Org  string `json:"organization"` // The organization that owns the blockchain definition
}

// This function creates Blockchain objects
func Blockchain_Factory(bc_type string, name string, org string) *Blockchain {
	b := new(Blockchain)
	b.Type = bc_type
	b.Name = name
	b.Org = org

	return b
}

// This function compares 2 BlockchainList arrays, returning all the blockchains that intersect
// between the 2 lists.
func (self *BlockchainList) Intersects_With(other *BlockchainList, defaultType string, defaultOrg string) (*BlockchainList, error) {
	inter := new(BlockchainList)

	// If both lists are empty then they intersect on the empty list, which is legal
	if len(*self) == 0 && len(*other) == 0 {
		return inter, nil
	}

	// If one list is empty use the other list
	if len(*self) == 0 {
		(*inter) = append(*inter, (*other)...)
		return inter, nil
	} else if len(*other) == 0 {
		(*inter) = append(*inter, (*self)...)
		return inter, nil
	}

	// If the lists are not empty then we need to find an intersection
	for _, sub_ele := range *self {
		for _, other_ele := range *other {
			if sub_ele.Same_Blockchain(&other_ele, defaultType, defaultOrg) {
				(*inter) = append(*inter, sub_ele)
			}
		}
	}

	if len(*inter) == 0 {
		return nil, errors.New(fmt.Sprintf("Blockchain Intersection Error: %v was not found in %v", (*self), (*other)))
	} else {
		return inter, nil
	}
}

// This function merges 2 Blockchain lists into one list, removing duplicates
func (self *BlockchainList) Concatenate(new_list *BlockchainList) {
	for _, new_ele := range *new_list {
		found := false
		for _, self_ele := range *self {
			if new_ele.Same_Blockchain(&self_ele, "", "") {
				found = true
				break
			}
		}
		if !found {
			(*self) = append((*self), new_ele)
		}
	}
}

func (self *BlockchainList) Single_Element() *BlockchainList {
	single := new(BlockchainList)
	if len(*self) != 0 {
		(*single) = append(*single, (*self)[0])
	}
	return single
}

// This function compares 2 Blockchain objects to see if they are equal. If the caller
// wants this function to consider absent default for the type, then they can pass in
// the actual default for comparison. If defaultType is the empty string then
// an exact match will be performed. Same is true for the default Org.
func (self *Blockchain) Same_Blockchain(second *Blockchain, defaultType string, defaultOrg string) bool {

	if self.Name != second.Name {
		return false
	}

	if self.Type != second.Type {
		// The types might be different because one of them is not specified and the other has
		// the default value specified. The default type is defined by the agreement protocol.
		if defaultType != "" {
			if self.Type == "" && defaultType == second.Type {
				// continue
			} else if second.Type == "" && defaultType == self.Type {
				// continue
			} else {
				return false
			}
		} else {
			return false
		}
	}

	if self.Org != second.Org {
		// The orgs might be different because one of them is not specified and the other has
		// the default value specified. The default org is system wide.
		if defaultOrg != "" {
			if self.Org == "" && defaultOrg == second.Org {
				return true
			} else if second.Org == "" && defaultOrg == self.Org {
				return true
			}
		}
		return false
	}

	return true

}

func (self Blockchain) String() string {
	return fmt.Sprintf("BC Name: %v, BC Type: %v, BC Org: %v", self.Name, self.Type, self.Org)
}

// This function adds a blockchain object to the list. Return an error if there are duplicates.
func (self *BlockchainList) Add_Blockchain(new_ele *Blockchain) error {
	for _, ele := range *self {
		if ele.Same_Blockchain(new_ele, "", "") {
			return errors.New(fmt.Sprintf("Blockchain %v already has the element being added: %v", *self, *new_ele))
		}
	}
	(*self) = append(*self, *new_ele)
	return nil
}

// Utility function
//
// This function compares 2 string arrays of unknown length (go slices) to see if they have the same
// contents. The array elements dont have to be in the same order. Assume the lengths have already been compared
// and found to be equivalent.
func array_contents_equal(a1 []interface{}, a2 []interface{}) bool {
	for ix, _ := range a1 {
		found := false
		for ix2, _ := range a2 {
			if a1[ix].(string) == a2[ix2].(string) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
