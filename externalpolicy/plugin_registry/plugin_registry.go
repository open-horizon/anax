package plugin_registry

import (
	"errors"
	"fmt"
)

// Each constraint language plugin implements this interface.
type ConstraintLanguagePlugin interface {
	Validate(constraints interface{}) (bool, []string, error)
	GetNextExpression(expression string) (string, string, error)
	GetNextOperator(expression string) (string, string, error)
}

// Global constraint language registry.
type ConstraintLanguageRegistry map[string]ConstraintLanguagePlugin

var ConstraintLanguagePlugins = ConstraintLanguageRegistry{}

// Plugin instances call this function to register themselves in the global registry.
func Register(name string, p ConstraintLanguagePlugin) {
	ConstraintLanguagePlugins[name] = p
}

// Ask each plugin to attempt to validate the input constraint language. Plugins are called
// until one of them claims ownership of the constraint language field. If no error is
// returned, then one of the plugins has validated the constraint expression.
func (d ConstraintLanguageRegistry) ValidatedByOne(constraints interface{}) ([]string, error) {
	for _, p := range d {
		if owned, constraints, err := p.Validate(constraints); owned {
			return constraints, err
		}
	}

	return nil, errors.New(fmt.Sprintf("constraint language %v is not supported", constraints))
}

// Ask each plugin to claim the input constraint language. Plugins are called
// until one of them claims ownership of the constraint expression. If no error is
// returned, then one of the plugins has claimed ownership.
func (d ConstraintLanguageRegistry) GetLanguageHandlerByOne(constraints interface{}) (ConstraintLanguagePlugin, error) {
	for _, p := range d {
		if owned, _, err := p.Validate(constraints); owned {
			return p, err
		}
	}

	return nil, errors.New(fmt.Sprintf("constraint language %v is not supported", constraints))
}

// Utility methods that can be used by other parts of the system to ask the global registry about plugins.
func (d ConstraintLanguageRegistry) HasPlugin(name string) bool {
	if _, ok := d[name]; ok {
		return true
	}
	return false
}

func (d ConstraintLanguageRegistry) Get(name string) ConstraintLanguagePlugin {
	if val, ok := d[name]; ok {
		return val
	}
	return nil
}
