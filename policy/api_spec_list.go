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
	SpecRef         string `json:"specRef"`         // A URL pointing to the definition of the API spec
	Version         string `json:"version"`         // The version of the API spec in OSGI version format
	ExclusiveAccess bool   `json:"exclusiveAccess"` // Whether or not exclusive access to this API spec is required
	Arch            string `json:"arch"`            // The hardware architecture of the API spec impl. Added in version 2.
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
func (self APISpecList) ContainsSpecRef(url string, version string) bool {
	for _, ele := range self {
		if ele.SpecRef == url && ele.Version == version {
			return true
		}
	}
	return false
}

// This function compares 2 APISpecification arrays, returning no error if the APISpec list
// meets the requirements of input APISpec list. Usually the self list is from a producer and
// the required list is from a consumer (i.e. workload).
func (self APISpecList) Supports(required APISpecList) error {

	// If nothing is required then self supports required, by definition.
	if len(required) == 0 {
		return nil
	}

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

// This function merges 2 APISpecification arrays, returning the merged list.
func (self *APISpecList) MergeWith(other *APISpecList) APISpecList {

	merged := new(APISpecList)

	// If both lists are empty then they are really easy to merge
	if len(*self) == 0 && len(*other) == 0 {
		return *merged
	}

	// If one list is empty use the other list
	if len(*self) == 0 {
		(*merged) = append(*merged, (*other)...)
		return *merged
	} else if len(*other) == 0 {
		(*merged) = append(*merged, (*self)...)
		return *merged
	}

	// Neither list is empty, so merge them
	(*merged) = append(*merged, (*self)...)
	for _, other_ele := range *other {
		found := false
		for _, sub_ele := range *self {
			if sub_ele.IsSame(other_ele, true) {
				found = true
			}
		}
		if !found {
			(*merged) = append(*merged, other_ele)
		}
	}

	return *merged
}

// This function extracts the APISpec URLs from a list of API Specs and returns the URLs in an array.
func (self *APISpecList) AsStringArray() []string {
	res := make([]string, 0, 10)
	for _, apiSpec := range *self {
		res = append(res, apiSpec.SpecRef)
	}
	return res
}
