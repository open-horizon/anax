package policy

import (
	"errors"
	"fmt"
)

// The purpose of this file is to provide APIs for working with the API spec list in a Policy.

type APISpecList []APISpecification

func (a APISpecList) IsSame(compare APISpecList, checkVersion bool) bool {

	if len(a) != len(compare) {
		return false
	}

	for _, apis := range a {
		found := false
		for _, compareAPIs := range compare {
			if apis.IsSame(compareAPIs, checkVersion) {
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

type APISpecification struct {
	SpecRef          string `json:"specRef"`          // A URL pointing to the definition of the API spec
	Version          string `json:"version"`          // The version of the API spec in OSGI version format
	ExclusiveAccess  bool   `json:"exclusiveAccess"`  // Whether or not exclusive access to this API spec is required
	// API will allow. For a Consumer (agbot), this is likely to be 1.
	// For a Producer, if this is zero, then no agreements, if 1 then
	// then it's essentially exclusive access. For more than 1, then it's
	// shared access. Added in version 2.
	Arch string `json:"arch"` // The hardware architecture of the API spec impl. Added in version 2.
}

func (a APISpecification) IsSame(compare APISpecification, checkVersion bool) bool {
	if a.SpecRef != compare.SpecRef || a.ExclusiveAccess != compare.ExclusiveAccess || a.Arch != compare.Arch {
		return false
	} else if checkVersion {
		return a.Version == compare.Version
	} else {
		return true
	}
}

// This function creates API Spec objects
func APISpecification_Factory(ref string, vers string, arch string) *APISpecification {
	a := new(APISpecification)
	a.SpecRef = ref
	a.Version = vers
	a.ExclusiveAccess = true
	a.Arch = arch

	return a
}

// This function compares 2 APISpecification arrays, returning no error if one of them
// is a subset (or equal to) the other.
func (self *APISpecList) Is_Subset_Of(super_set *APISpecList) error {

	for _, sub_ele := range *self {
		found := false
		for _, super_ele := range *super_set {
			if sub_ele.SpecRef == super_ele.SpecRef && sub_ele.Arch == super_ele.Arch {
				if super_ver, err := Version_Expression_Factory(super_ele.Version); err != nil {
					continue
				} else if ok, err := super_ver.Is_within_range(sub_ele.Version); err != nil {
					continue
				} else if ok {
					found = true
					break
				}
			}
		}
		if !found {
			return errors.New(fmt.Sprintf("APISpec Subset Error: %v was not found in superset %v", sub_ele, super_set))
		}
	}

	return nil
}

// This function merges 2 API spec lists into one list, there should never be duplicates in the input list.
func (self *APISpecList) Concatenate(new_list *APISpecList) {
	for _, new_ele := range *new_list {
		(*self) = append((*self), new_ele)
	}
}

// This function adds an API spec to the list. Return an error if there are duplicates.
func (self *APISpecList) Add_API_Spec(new_ele *APISpecification) error {
	for _, ele := range *self {
		if ele.SpecRef == new_ele.SpecRef {
			return errors.New(fmt.Sprintf("APISpecList %v already has the element being added: %v", *self, *new_ele))
		}
	}
	(*self) = append(*self, *new_ele)
	return nil
}

// This function return true if an api spec list contains the input spec ref url
func (self APISpecList) ContainsSpecRef(url string) bool {
	for _, ele := range self {
		if ele.SpecRef == url {
			return true
		}
	}
	return false
}

// This function compares 2 APISpecification arrays, returning no error if the APISpec list
// meets the requirements of input APISpec list.
func (self APISpecList) Supports(required APISpecList) error {

	if len(self) != len(required) {
		return errors.New(fmt.Sprintf("API Spec lists are different lengths, self: %v and required: %v", self, required))
	}

	for _, sub_ele := range self {
		found := false
		for _, req_ele := range required {
			if sub_ele.SpecRef == req_ele.SpecRef && sub_ele.Arch == req_ele.Arch {
				if req_ver, err := Version_Expression_Factory(req_ele.Version); err != nil {
					continue
				} else if ok, err := req_ver.Is_within_range(sub_ele.Version); err != nil {
					continue
				} else if ok {
					found = true
					break
				}
			}
		}
		if !found {
			return errors.New(fmt.Sprintf("APISpec %v does not support required API Spec %v", sub_ele, required))
		}
	}

	return nil
}
