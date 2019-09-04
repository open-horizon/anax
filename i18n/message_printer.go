package i18n

import (
	"fmt"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"os"
	"strings"
)

const HZN_LANG = "HZN_LANG"

const DEFAULT_LANGUAGE = "en"

var messagePrinter *message.Printer

//en,zh_CN,zh_TW,fr,de,it,ja,pt_BR,es,ko
var supportedLangs = []language.Tag{
	language.Make("en"), // english fallback
	language.Make("zh_CN"),
	language.Make("zh_TW"),
	language.Make("fr"),
	language.Make("de"),
	language.Make("it"),
	language.Make("ja"),
	language.Make("pt-BR"),
	language.Make("es"),
	language.Make("ko"),
}

// use HZN_LANG or LANG env variables
func GetLocale() (language.Tag, error) {
	locale := os.Getenv(HZN_LANG)
	if locale == "" {
		locale = os.Getenv("LANG")
		if locale == "" {
			// default
			return language.English, nil
		}
	}

	current_locale := strings.Split(locale, ".")[0]

	// some os had default LANG setting as "C" or "C.UTF-8", GO does not recognize it, we will default it to en_US
	if current_locale == "C" {
		current_locale = DEFAULT_LANGUAGE
	}

	tag, err := language.Parse(current_locale)
	if err != nil {
		return language.English, fmt.Errorf("Could not parse locale %v.: %v\n", current_locale, err)
	} else {
		return tag, nil
	}
}

// find the default matching language for locae laguage. The fallback is English
func FindMatchingLanguage(tag language.Tag) language.Tag {
	var matcher = language.NewMatcher(supportedLangs)
	matchTag, _, _ := matcher.Match(tag)
	return matchTag
}

// create a message printer with locale defined in HZN_LANG or LANG. Fallback to English
func InitMessagePrinter(useEnglish bool) error {
	messagePrinter = message.NewPrinter(language.English)
	if !useEnglish {
		locale_tag, err := GetLocale()
		if err != nil {
			return err
		}
		matchTag := FindMatchingLanguage(locale_tag)
		//fmt.Printf("The matching language for %v is %v\n", locale_tag, matchTag)
		messagePrinter = message.NewPrinter(matchTag)
	}
	return nil
}

// Get the default message printer.
func GetMessagePrinter() *message.Printer {
	// This is the case InitMessagePrinter has not been called by the main program.
	if messagePrinter == nil {
		InitMessagePrinter(true)
	}
	return messagePrinter
}

// Get the message printer with the given locale. The fallback is English if the given
// locale is not a valid locale string. If it is a valid locale string, but the language is
// not in the supported list, go text will find the best match for it.
func GetMessagePrinterWithLocale(locale string) *message.Printer {
	current_locale := strings.Split(locale, ".")[0]
	tag, err := language.Parse(current_locale)
	if err != nil {
		tag = language.English
	}
	matchTag := FindMatchingLanguage(tag)
	return message.NewPrinter(matchTag)
}
