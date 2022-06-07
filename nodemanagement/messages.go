package nodemanagement

import (
	"github.com/open-horizon/anax/i18n"
)

const (
	EL_NMP_STATUS_CREATED = "New node management policy status created for policy %v."
	EL_NMP_STATUS_CHANGED = "Node management status for %v changed to %v."
)

// This is does nothing useful at run time.
// This code is only used in compileing time to make the eventlog messages gets into the catalog so that
// they can be translated.
// The event log messages will be saved in English. But the CLI can request them in different languages.
func MarkI18nMessages() {
	// get message printer. anax default language is English
	msgPrinter := i18n.GetMessagePrinter()

	msgPrinter.Sprintf(EL_NMP_STATUS_CREATED)
	msgPrinter.Sprintf(EL_NMP_STATUS_CHANGED)
}
