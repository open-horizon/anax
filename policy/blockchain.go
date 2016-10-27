package policy

import (
	"errors"
	"fmt"
)

// The purpose of this file is to abtrsact the operations on the Blockchain object and
// its List type.
//

// This struct represents a specific instance of blockchain. Please note that the
// EthereumBlockchain struct is defined here for documentation purposes only, it is not
// used in the code because the Details field of the Blockchaim struct is dynamic, based
// on the type of blockchain. This means that the policy code dynamically inspects the
// contents of the details field. It is never deserialized into one of these structs.
const Ethereum_bc = "ethereum"

type EthereumBlockchain struct {
	Genesis   []string `json:"genesis"`   // Array of URLs for the genesis block
	NetworkId []string `json:"networkid"` // Array of URLs for the networkid
	Bootnodes []string `json:"bootnodes"` // Array of URLs for the bootnodes
	Directory []string `json:"directory"` // Array of URLs for the directory contract
}

// This struct indicates the type and instance of blockchain to be used by the policy
type BlockchainList []Blockchain

func (a BlockchainList) IsSame(compare BlockchainList) bool {
	for _, bc := range a {
		found := false
		for _, compareBC := range compare {
			if bc.Same_Blockchain(&compareBC) {
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
	Type    string      `json:"type"`    // The type of blockchain
	Details interface{} `json:"details"` // The details of how to bootstrap to the blockchain, what's
	// in this field is specific to the value of the type
}

// This function creates Blockchain objects
func Blockchain_Factory(bc_type string, details interface{}) *Blockchain {
	b := new(Blockchain)
	b.Type = bc_type
	b.Details = details

	return b
}

// This function compares 2 BlockchainList arrays, returning no error if one of them
// is a subset (or equal to) the other.
func (self *BlockchainList) Is_Subset_Of(super_set *BlockchainList) error {

	for _, sub_ele := range *self {
		found := false
		for _, super_ele := range *super_set {
			if sub_ele.Same_Blockchain(&super_ele) {
				found = true
				break
			}
		}
		if !found {
			return errors.New(fmt.Sprintf("Blockchain Subset Error: %v was not found in superset %v", sub_ele, super_set))
		}
	}

	return nil
}

// This function compares 2 BlockchainList arrays, returning all the blockchains that intersect
// between the 2 lists.
func (self *BlockchainList) Intersects_With(other *BlockchainList) (*BlockchainList, error) {
	inter := new(BlockchainList)
	for _, sub_ele := range *self {
		for _, other_ele := range *other {
			if sub_ele.Same_Blockchain(&other_ele) {
				(*inter) = append(*inter, sub_ele)
			}
		}
	}

	if len(*inter) == 0 {
		return nil, errors.New(fmt.Sprintf("Agreement Protocol Intersection Error: %v was not found in %v", (*self), (*other)))
	} else {
		return inter, nil
	}
}

// This function merges 2 Blockchain lists into one list, removing duplicates
func (self *BlockchainList) Concatenate(new_list *BlockchainList) {
	for _, new_ele := range *new_list {
		found := false
		for _, self_ele := range *self {
			if new_ele.Same_Blockchain(&self_ele) {
				found = true
				break
			}
		}
		if !found {
			(*self) = append((*self), new_ele)
		}
	}
}

// This function compares 2 Blockchain objects to see if they are equal.
func (self *Blockchain) Same_Blockchain(second *Blockchain) bool {

	if self.Type != second.Type {
		return false
	}

	switch self.Details.(type) {
	case map[string]interface{}:
		sub_d := self.Details.(map[string]interface{})
		super_d := second.Details.(map[string]interface{})
		if self.Type == Ethereum_bc {
			if _, ok := sub_d["genesis"]; !ok {
				return false
			} else if _, ok := super_d["genesis"]; !ok {
				return false
			} else if len(sub_d["genesis"].([]interface{})) != len(super_d["genesis"].([]interface{})) {
				return false
			} else if !array_contents_equal(sub_d["genesis"].([]interface{}), super_d["genesis"].([]interface{})) {
				return false
			} else if _, ok := sub_d["bootnodes"]; !ok {
				return false
			} else if _, ok := super_d["bootnodes"]; !ok {
				return false
			} else if len(sub_d["bootnodes"].([]interface{})) != len(super_d["bootnodes"].([]interface{})) {
				return false
			} else if !array_contents_equal(sub_d["bootnodes"].([]interface{}), super_d["bootnodes"].([]interface{})) {
				return false
			} else if _, ok := sub_d["directory"]; !ok {
				return false
			} else if _, ok := super_d["directory"]; !ok {
				return false
			} else if len(sub_d["directory"].([]interface{})) != len(super_d["directory"].([]interface{})) {
				return false
			} else if !array_contents_equal(sub_d["directory"].([]interface{}), super_d["directory"].([]interface{})) {
				return false
			} else if _, ok := sub_d["networkid"]; !ok {
				return false
			} else if _, ok := super_d["networkid"]; !ok {
				return false
			} else if len(sub_d["networkid"].([]interface{})) != len(super_d["networkid"].([]interface{})) {
				return false
			} else if !array_contents_equal(sub_d["networkid"].([]interface{}), super_d["networkid"].([]interface{})) {
				return false
			} else {
				return true
			}
		} else {
			// Unsupported blockchain type
			return false
		}
	default:
		return false
	}
}

// This function adds a blockchain object to the list. Return an error if there are duplicates.
func (self *BlockchainList) Add_Blockchain(new_ele *Blockchain) error {
	for _, ele := range *self {
		if ele.Same_Blockchain(new_ele) {
			return errors.New(fmt.Sprintf("Blockchain %v already has the element being added: %v", *self, *new_ele))
		}
	}
	(*self) = append(*self, *new_ele)
	return nil
}

// Utility function
//
// This function compares 2 string arrays of unknown length (go slices) to see if they have the same
// contents. Assume the lengths have already been compared and found to be equivalent.
func array_contents_equal(a1 []interface{}, a2 []interface{}) bool {
	for ix, _ := range a1 {
		if a1[ix].(string) != a2[ix].(string) {
			return false
		}
	}
	return true
}
