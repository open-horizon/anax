package i18n

import (
	"fmt"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"os"
	"strings"
)

const HZN_LANG = "HZN_LANG"

var messagePrinter *message.Printer

// use HZN_LANG env variable or 'locale |grep "LANG=" |cut -d= -f2|cut -d. -f1'
func GetLocale() (language.Tag, error) {
	locale := os.Getenv(HZN_LANG)
	if locale == "" {
		locale = os.Getenv("LC_ALL")
		if locale == "" {
			locale = os.Getenv("LANG")
			if locale == "" {
				// default
				return language.English, nil
			}
		}
	}

	// locale may looks like this: en_US.UTF-8. Need to remove the part after .
	current_locale := strings.Split(locale, ".")[0]
	tag, err := language.Parse(current_locale)
	if err != nil {
		return language.English, fmt.Errorf("Could not parse language %v.: %v", current_locale, err)
	} else {
		return tag, nil
	}
}

// create a message printer with locale
func InitMessagePrinter(useEnglish bool) error {
	messagePrinter = message.NewPrinter(language.English)
	if !useEnglish {
		locale_tag, err := GetLocale()
		if err != nil {
			return err
		}
		messagePrinter = message.NewPrinter(locale_tag)
	}

	return nil
}

func GetMessagePrinter() *message.Printer {
	// This is the case InitMessagePrinter has not been called by the main program.
	if messagePrinter == nil {
		InitMessagePrinter(true)
	}
	return messagePrinter
}

func GetMessagePrinterWithLocale(locale string) *message.Printer {
	return message.NewPrinter(language.Make(locale))
}
