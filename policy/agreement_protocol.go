package policy

import (
	"errors"
	"fmt"
)

// All known and supported agreement protocols
const CitizenScientist = "Citizen Scientist"
const BasicProtocol = "Basic"

var AllProtocols = []string{CitizenScientist, BasicProtocol}

func SupportedAgreementProtocol(name string)  bool {
    for _, p := range AllProtocols {
        if p == name {
            return true
        }
    }
    return false
}

func AllAgreementProtocols() []string {
    return AllProtocols
}

// The purpose of this file is to abstract the operations on the AgreementProtocol type
// and its list type.

type AgreementProtocolList []AgreementProtocol

func (a AgreementProtocolList) IsSame(compare AgreementProtocolList) bool {

	if len(a) != len(compare) {
		return false
	}

	for _, agps := range a {
		found := false
		for _, compareAGPs := range compare {
			if agps.IsSame(compareAGPs) {
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

type AgreementProtocol struct {
	Name string `json:"name"` // The name of the agreement protocol to be used
}

func (a AgreementProtocol) IsSame(compare AgreementProtocol) bool {
	return a.Name == compare.Name
}

// This function creates AgreementProtocol objects
func AgreementProtocol_Factory(name string) *AgreementProtocol {
	a := new(AgreementProtocol)
	a.Name = name

	return a
}

// This function converts an AgreementProtocolList into a list of strings based on the names
// of the agreement protocols in the original list.
func (self AgreementProtocolList) As_String_Array() []string {
	r := make([]string, 0, 10)
	for _, e := range self {
		r = append(r, e.Name)
	}
	return r
}

// This function compares 2 AgreementProtocolList arrays, returning no error if they have
// at least 1 agreement protocol in common.
func (self *AgreementProtocolList) Intersects_With(other *AgreementProtocolList) (*AgreementProtocolList, error) {

	inter := new(AgreementProtocolList)

	if len(*self) == 0 && len(*other) == 0 {
		(*inter) = append(*inter, *AgreementProtocol_Factory(BasicProtocol))
		return inter, nil
	} else if len(*self) == 0 {
		(*inter) = append(*inter, *other...)
		return inter, nil
	} else if len(*other) == 0 {
		(*inter) = append(*inter, *self...)
		return inter, nil
	}

	for _, sub_ele := range *self {
		for _, other_ele := range *other {
			if sub_ele.Name == other_ele.Name {
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

// This function merges 2 Agreement protocol lists into one list, removing duplicates.
func (self *AgreementProtocolList) Concatenate(new_list *AgreementProtocolList) {
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

// This function returns an Agreement Protocol List with just a single element. This function will prefer the
// Basic protocol if available.
func (self *AgreementProtocolList) Single_Element() *AgreementProtocolList {

	basic := new(AgreementProtocolList)
	(*basic) = append(*basic, *AgreementProtocol_Factory(BasicProtocol))

	if intersect, err := (*self).Intersects_With(basic); err == nil {
		return intersect
	} else {
		single := new(AgreementProtocolList)
		(*single) = append(*single, (*self)[0])
		return single
	}
}

// This function adds an Agreement protocol to the list. Return an error if there are duplicates.
func (self *AgreementProtocolList) Add_Agreement_Protocol(new_ele *AgreementProtocol) error {
	for _, ele := range *self {
		if ele.Name == new_ele.Name {
			return errors.New(fmt.Sprintf("AgreementProtocolList %v already has the element being added: %v", *self, *new_ele))
		}
	}
	(*self) = append(*self, *new_ele)
	return nil
}
