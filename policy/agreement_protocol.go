package policy

import (
	"errors"
	"fmt"
)

// All known and supported agreement protocols
const CitizenScientist = "Citizen Scientist"
const BasicProtocol = "Basic"

var AllProtocols = []string{CitizenScientist, BasicProtocol}

var RequiresBCType = map[string]string{CitizenScientist:Ethereum_bc}

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

func RequiresBlockchainType(protocolName string) string {
	if bctype, ok := RequiresBCType[protocolName]; ok {
		return bctype
	}
	return ""
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

func ConvertToAgreementProtocolList(list []interface{}) (*[]AgreementProtocol, error) {
	newList := make([]AgreementProtocol, 0, 5)
	for _, agpEle := range list {
		if mapEle, ok := agpEle.(map[string]interface{}); ok {
			if pName, ok := mapEle["name"].(string); ok {
				newAGP := AgreementProtocol_Factory(pName)
				if mapEle["blockchains"] == nil {
					newList = append(newList, *newAGP)
					continue
				} else if bcList, ok := mapEle["blockchains"].([]interface{}); ok {
					for _, bcDef := range bcList {
						if bc, ok := bcDef.(map[string]interface{}); ok {
							bcType := ""
							if bc["type"] != nil {
								bcType = bc["type"].(string)
							}
							bcName := ""
							if bc["name"] != nil {
								bcName = bc["name"].(string)
							}
							(&newAGP.Blockchains).Add_Blockchain(Blockchain_Factory(bcType, bcName))
						} else {
							return nil, errors.New(fmt.Sprintf("could not convert blockchain list element to map[string]interface{}, %v is %T", bcDef, bcDef))
						}
					}
					newList = append(newList, *newAGP)
				} else {
					return nil, errors.New(fmt.Sprintf("could not convert blockchain list to []interface{}, %v is %T", mapEle["blockchains"], mapEle["blockchains"]))
				}
			} else {
				return nil, errors.New(fmt.Sprintf("could not convert agreement protocol name to string, %v is %T", mapEle["name"], mapEle["name"]))
			}
		} else {
			return nil, errors.New(fmt.Sprintf("could not convert agreement protocol list element to map[string]interface{}, %v is %T", agpEle, agpEle))
		}
	}
	return &newList, nil
}

type AgreementProtocol struct {
	Name            string         `json:"name"` // The name of the agreement protocol to be used
	ProtocolVersion int            `json:"protocolVersion,omitempty"` // The max protocol version supported
	Blockchains     BlockchainList `json:"blockchains,omitempty"` // The blockchain to be used if the protocol requires one.
}

func (a AgreementProtocol) IsSame(compare AgreementProtocol) bool {
	return a.Name == compare.Name && a.Blockchains.IsSame(compare.Blockchains) && ((a.ProtocolVersion == 0 && compare.ProtocolVersion == 1) || (compare.ProtocolVersion == 0 && a.ProtocolVersion == 1) || (a.ProtocolVersion == compare.ProtocolVersion))
}

func (a *AgreementProtocol) Initialize() {
	if a.Name == CitizenScientist && len(a.Blockchains) == 0 {
		a.Blockchains.Add_Blockchain(Blockchain_Factory("", ""))
	}
	for ix, bc := range a.Blockchains {
		if a.Name == CitizenScientist && bc.Type == "" {
			a.Blockchains[ix].Type = Ethereum_bc
		}
		if a.Name == CitizenScientist && bc.Name == "" {
			a.Blockchains[ix].Name = "bluehorizon"
		}
	}
}

func (a AgreementProtocol) String() string {
	res := fmt.Sprintf("Agreement Protocol name: %v, protocolVersion: %v, Blockchains:", a.Name, a.ProtocolVersion)
	for _, bc := range a.Blockchains {
		res += bc.String() + ","
	}
	return res
}

func (a *AgreementProtocol) IsValid() error {

	if !SupportedAgreementProtocol(a.Name) {
		return errors.New(fmt.Sprintf("AgreementProtocol %v is not supported.", a.Name))
	} else {
		for _, bc := range a.Blockchains {
			if bc.Type != "" && bc.Type != RequiresBlockchainType(a.Name) {
				return errors.New(fmt.Sprintf("AgreementProtocol %v has blockchain type %v that is incompatible.", a.Name, bc.Type))
			}
		}
	}
	return nil
}

// Used to figoure out what protocol version to use for the initial agreement message. All subsequent
// messages MUST use the same protocol version. Anax will store the protocol version of the initial
// message for the agreement and will use the stored version for all future messages.
func (a *AgreementProtocol) MinimumProtocolVersion(other *AgreementProtocol, maxSupportedVersion int) int {
	if a.ProtocolVersion == 0 { 	// old Anax, before it always exported a protocol version in policy files.
		return 1
	} else if other.ProtocolVersion != 0 && other.ProtocolVersion <= a.ProtocolVersion { 	// Agbot policy file specified something lower than what device supports
		return other.ProtocolVersion
	} else if other.ProtocolVersion == 0 && maxSupportedVersion < a.ProtocolVersion { 	// For agbot policy files that dont specify a protocol version
		return maxSupportedVersion
	} else {
		return a.ProtocolVersion 	// Producer always exports a protocol version at 2 or higher.
	}
}

// This function creates AgreementProtocol objects
func AgreementProtocol_Factory(name string) *AgreementProtocol {
	a := new(AgreementProtocol)
	a.Name = name
	a.Blockchains = (*new(BlockchainList))
	if name == CitizenScientist {
		a.ProtocolVersion = 2
	} else {
		a.ProtocolVersion = 1 	// this might have to be zero
	}
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
				if bcIntersect, err := sub_ele.Blockchains.Intersects_With(&other_ele.Blockchains, RequiresBlockchainType(sub_ele.Name)); err != nil {
					return nil, errors.New(fmt.Sprintf("Agreement Protocol Intersection Error on blockchains: %v was not found in %v", (*self), (*other)))
				} else {
					new_ele := AgreementProtocol{
						Name: sub_ele.Name,
						Blockchains: *bcIntersect,
					}
					(*inter) = append(*inter, new_ele)
				}
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
		newAGP := (*self)[0]
		if len(newAGP.Blockchains) > 1 {
			bc := newAGP.Blockchains[0]
			newAGP.Blockchains = nil
			newAGP.Blockchains = append(newAGP.Blockchains, bc)
		}
		single := new(AgreementProtocolList)
		(*single) = append(*single, newAGP)
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

// This function returns a specific agreement protocol object from the list
func (self *AgreementProtocolList) FindByName(name string) *AgreementProtocol {
	for ix, ele := range *self {
		if ele.Name == name {
			return &(*self)[ix]
		}
	}

	return nil
}
