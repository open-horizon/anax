package externalpolicy

import (
	"github.com/open-horizon/anax/externalpolicy/plugin_registry"
	_ "github.com/open-horizon/anax/externalpolicy/text_language"
)

// This type implements all the ConstraintLanguage Plugin methods and delegates to plugin system.
type ConstraintExpression []string

func (c *ConstraintExpression) Validate() error {
	return plugin_registry.ConstraintLanguagePlugins.ValidatedByOne(*c)
}

func (c *ConstraintExpression) GetLanguageHandler() (plugin_registry.ConstraintLanguagePlugin, error) {
	return plugin_registry.ConstraintLanguagePlugins.GetLanguageHandlerByOne(*c)
}
