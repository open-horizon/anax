package policy

import (
	"errors"
	"fmt"
)

// The purpose of this file is to abstract the Property type and its List type.

// This struct represents property values advertised by the policy
type PropertyList []Property

func (p PropertyList) IsSame(compare PropertyList) bool {
	for _, prop := range p {
		found := false
		for _, compareProp := range compare {
			if prop.IsSame(compareProp) {
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

type Property struct {
	Name  string      `json:"name"`  // The Property name
	Value interface{} `json:"value"` // The Property value
}

func (p Property) IsSame(compare Property) bool {
	return p.Name == compare.Name && p.Value == compare.Value
}

// This function creates Property objects
func Property_Factory(name string, value interface{}) *Property {
	p := new(Property)
	p.Name = name
	p.Value = value

	return p
}

// This function compares 2 property lists to determine if they set different values on the
// same property. This would make them incompatible.
func (self *PropertyList) Compatible_With(other *PropertyList) error {

	for _, self_ele := range *self {
		for _, other_ele := range *other {
			if self_ele.Name == other_ele.Name && self_ele.Value != other_ele.Value {
				return errors.New(fmt.Sprintf("Property %v has value %v and %v.", self_ele.Name, self_ele.Value, other_ele.Value))
			}
		}
	}

	return nil
}

// This function merges 2 Property lists into one list, removing duplicates.
func (self *PropertyList) Concatenate(new_list *PropertyList) {
	for _, new_ele := range *new_list {
		found := false
		for _, self_ele := range *self {
			if new_ele.Name == self_ele.Name {
				found = true
				break
			}
		}
		if !found {
			(*self) = append((*self), new_ele)
		}
	}
}

// This function adds a Property to the list. Return an error if there are duplicates.
func (self *PropertyList) Add_Property(new_ele *Property) error {
	for _, ele := range *self {
		if ele.Name == new_ele.Name {
			return errors.New(fmt.Sprintf("PropertyList %v already has the element being added: %v", *self, *new_ele))
		}
	}
	(*self) = append(*self, *new_ele)
	return nil
}
