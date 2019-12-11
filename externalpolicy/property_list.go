package externalpolicy

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/open-horizon/anax/i18n"
	"strings"
)

// The purpose of this file is to abstract the Property type and its List type.

const (
	STRING_TYPE     = "string"
	VERSION_TYPE    = "version"
	BOOLEAN_TYPE    = "boolean"
	INTEGER_TYPE    = "int"
	FLOAT_TYPE      = "float"
	LIST_TYPE       = "list of string"
	UNDECLARED_TYPE = ""
)

// This struct represents property values advertised by the policy
type PropertyList []Property

type Property struct {
	Name  string      `json:"name"`           // The Property name
	Value interface{} `json:"value"`          // The Property value
	Type  string      `json:"type,omitempty"` // The type of the Property value
}

// This function creates Property objects
func Property_Factory(name string, value interface{}) *Property {
	p := new(Property)
	p.Name = name
	p.Value = value

	return p
}

// IsSame will return true if the given properties have the same value
func (p Property) IsSame(compare Property) bool {
	if p.Name == compare.Name {
		if p.Type != compare.Type {
			if p.Type != UNDECLARED_TYPE && compare.Type != UNDECLARED_TYPE {
				return false
			}
		}
		switch p.Value.(type) {
		case string:
			if _, ok := compare.Value.(string); ok {
				if p.Type == LIST_TYPE || compare.Type == LIST_TYPE {
					return isSameList(strings.Split(p.Value.(string), ","), strings.Split(compare.Value.(string), ","))
				}
				return p.Value == compare.Value
			}
		case float64, json.Number:
			_, okJson := compare.Value.(float64)
			_, okFloat := compare.Value.(json.Number)
			if okJson || okFloat {
				return p.Value == compare.Value
			}
		case bool:
			if _, ok := compare.Value.(bool); ok {
				return p.Value == compare.Value
			}
		}

	}
	return false
}

// This function will return true if both lists contain the same strings
func isSameList(list1 []string, list2 []string) bool {
	found := false
	for _, elem1 := range list1 {
		for _, elem2 := range list2 {
			if elem1 == elem2 {
				found = true
			}
		}
		if !found {
			return false
		}
		found = false
	}
	for _, elem1 := range list2 {
		for _, elem2 := range list1 {
			if elem1 == elem2 {
				found = true
			}
		}
		if !found {
			return false
		}
		found = false
	}
	return true
}

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

// This function compares 2 property lists to determine if they set different values on the
// same property. This would make them incompatible.
func (self *PropertyList) Compatible_With(other *PropertyList, ignoreBuiltIn bool) error {
	for _, self_ele := range *self {
		for _, other_ele := range *other {
			if self_ele.Name == other_ele.Name && !self_ele.IsSame(other_ele) {
				// the built-in properties available memory and cpu could change from time to time.
				// so we ingnore the error here if ignoreBuiltIn is true
				if ignoreBuiltIn && (self_ele.Name == PROP_NODE_MEMORY || self_ele.Name == PROP_NODE_CPU) {
					continue
				} else {
					return errors.New(fmt.Sprintf("Property %v has value %v and %v.", self_ele.Name, self_ele.Value, other_ele.Value))
				}
			}
		}
	}

	return nil
}

// This function merges two PropertyList into one list. If both have the same property, ignore
// the ones from new_list unless replaceExsiting is true.
func (self *PropertyList) MergeWith(new_list *PropertyList, replaceExsiting bool) {
	if new_list == nil {
		return
	}

	for _, new_ele := range *new_list {
		self.Add_Property(&new_ele, replaceExsiting)
	}
}

// This function adds a Property to the list. Return an error if there are duplicates and replaceExisting is false.
func (self *PropertyList) Add_Property(new_ele *Property, replaceExisting bool) error {
	if new_ele == nil {
		return nil
	}

	tempList := new(PropertyList)
	*tempList = append(*tempList, *new_ele)
	if err := tempList.Validate(); err != nil {
		return fmt.Errorf("Could not validate new property %v: %v", new_ele, err)
	}

	for i, ele := range *self {
		if ele.Name == new_ele.Name {
			if replaceExisting {
				(*self)[i] = *new_ele
				return nil
			} else {
				return errors.New(fmt.Sprintf("PropertyList %v already has the element being added: %v", *self, *new_ele))
			}
		}
	}
	(*self) = append(*self, *new_ele)
	return nil
}

// This function checks the property list to see if the given property is present.
func (self PropertyList) HasProperty(name string) bool {
	for _, ele := range self {
		if ele.Name == name {
			return true
		}
	}
	return false
}

func (self PropertyList) GetProperty(name string) (Property, error) {
	for _, ele := range self {
		if ele.Name == name {
			return ele, nil
		}
	}
	return Property{}, fmt.Errorf("Error: property %s not found in list %v.", name, self)
}

// Validate will return an error if any property in the list has an invalid format or a value that does not match a declared type
func (self *PropertyList) Validate() error {
	// get message printer because this function is called by CLI
	msgPrinter := i18n.GetMessagePrinter()

	for _, property := range *self {
		if property.Name == "" || property.Value == nil {
			return fmt.Errorf(msgPrinter.Sprintf("Property must include a name and a value: %v", property))
		}
		declaredType := property.Type

		if !isValidPropertyType(declaredType) {
			return fmt.Errorf(msgPrinter.Sprintf("Property %s has invalid property type %s. Allowed property types are: version, string, int, boolean, float, and list of string.", property.Name, declaredType))
		}

		switch actualType := property.Value.(type) {
		case bool:
			if declaredType != BOOLEAN_TYPE && declaredType != UNDECLARED_TYPE {
				return fmt.Errorf(msgPrinter.Sprintf("Property value is of type %T, expected type %s", actualType, declaredType))
			}
		case float64, json.Number:
			if declaredType == INTEGER_TYPE && fmt.Sprintf("%T", actualType) == "float64" {
				if float64(int(property.Value.(float64))) != property.Value.(float64) {
					return fmt.Errorf(msgPrinter.Sprintf("Value %v of property %s is not an integer type", property.Value, property.Name))
				}
			} else if declaredType == INTEGER_TYPE && fmt.Sprintf("%T", actualType) == "json.Number" {
				_, err := property.Value.(json.Number).Int64()
				if err != nil {
					return fmt.Errorf(msgPrinter.Sprintf("Value %v of property %s is not an integer type", property.Value, property.Name))
				}
			}
			if declaredType != INTEGER_TYPE && declaredType != FLOAT_TYPE && declaredType != UNDECLARED_TYPE {
				return fmt.Errorf(msgPrinter.Sprintf("Property value is of type %T, expected type %s", actualType, declaredType))
			}
		case string:
			stringVal, canBeString := property.Value.(string)
			if !canBeString {
				return fmt.Errorf(msgPrinter.Sprintf("Value %v of property %s is not a valid string. Please define type or change value to a string.", property.Value, property.Name))
			}
			if declaredType == VERSION_TYPE {
				if !IsVersionString(stringVal) {
					return fmt.Errorf(msgPrinter.Sprintf("Property %s with value %v is not a valid verion string", property.Name, property.Value))
				}
			} else if declaredType != STRING_TYPE && declaredType != UNDECLARED_TYPE && declaredType != LIST_TYPE {
				return fmt.Errorf(msgPrinter.Sprintf("Property value is of type %T, expected type %s", actualType, declaredType))
			}
		default:
			return fmt.Errorf(msgPrinter.Sprintf("Property %s has invalid value type %T", property.Name, actualType))
		}
	}
	return nil
}

// IsVersionString will return true if the input version string is a valid version according to the version string schema outlined in anax/policy/version.go.
// A number with leading 0's, for example 1.02.1, is not a valid version string.
func IsVersionString(expr string) bool {
	if expr == "INFINITY" {
		return true
	}
	nums := strings.Split(expr, ".")
	if len(nums) == 0 || len(nums) > 3 {
		return false
	} else {
		for _, val := range nums {
			if val == "" {
				return false
			} else if len(val) > 1 { // not allow the leadng 0s.
				if s := strings.TrimLeft(val, "0"); s != val {
					return false
				}
			}
			for _, val2 := range val {
				if !strings.Contains("0123456789", string(val2)) {
					return false
				}
			}
		}
		return true
	}
}

func isValidPropertyType(typeInput string) bool {
	validTypes := []string{STRING_TYPE, VERSION_TYPE, BOOLEAN_TYPE, INTEGER_TYPE, FLOAT_TYPE, LIST_TYPE, UNDECLARED_TYPE}
	for _, validType := range validTypes {
		if validType == typeInput {
			return true
		}
	}
	return false
}
