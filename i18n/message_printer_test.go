//go:build unit
// +build unit

package i18n

import (
	//"fmt"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"os"
	//"strings"
	"testing"
)

func init() {
	en_tag := language.MustParse("en")
	fr_tag := language.MustParse("fr")
	message.SetString(en_tag, "Hello", "Hello in English")
	message.SetString(fr_tag, "Hello", "Hello in French")
}

func Test_InitMessagePrinter(t *testing.T) {
	// setup English message printer
	if err := InitMessagePrinter(true); err != nil {
		t.Errorf("InitMessagePrinter returned error but should not. Error: %v", err)
	} else {
		msgPrinter := GetMessagePrinter()
		s := msgPrinter.Sprintf("Hello")
		if s != "Hello in English" {
			t.Errorf("msgPrinter should print 'Hello in English' but got '%v'.", s)
		}
	}

	// setup French message printer
	os.Setenv("HZN_LANG", "fr")
	if err := InitMessagePrinter(false); err != nil {
		t.Errorf("InitMessagePrinter returned error but should not. Error: %v", err)
	} else {
		msgPrinter := GetMessagePrinter()
		s := msgPrinter.Sprintf("Hello")
		if s != "Hello in French" {
			t.Errorf("msgPrinter should print 'Hello in French' but got '%v'.", s)
		}
	}

	// setup a printer for an invalid locale
	os.Setenv("HZN_LANG", "zh-CB")
	if err := InitMessagePrinter(false); err == nil {
		t.Errorf("InitMessagePrinter should have returned error but not.")
	} else {
		// Fallback to English
		msgPrinter := GetMessagePrinter()
		s := msgPrinter.Sprintf("Hello")
		if s != "Hello in English" {
			t.Errorf("msgPrinter should print 'Hello in English' but got '%v'.", s)
		}
	}
}

func Test_GetMessagePrinterWithLocale(t *testing.T) {
	// English
	s1 := GetMessagePrinterWithLocale("en").Sprintf("Hello")
	if s1 != "Hello in English" {
		t.Errorf("msgPrinter should print 'Hello in English' but got '%v'.", s1)
	}

	// French
	s2 := GetMessagePrinterWithLocale("fr").Sprintf("Hello")
	if s2 != "Hello in French" {
		t.Errorf("msgPrinter should print 'Hello in French' but got '%v'.", s2)
	}

	// Bad locale fallback to English
	s3 := GetMessagePrinterWithLocale("f123").Sprintf("Hello")
	if s3 != "Hello in English" {
		t.Errorf("msgPrinter should print 'Hello in English' but got '%v'.", s3)
	}

	// Unsupported locale fallback to English
	s4 := GetMessagePrinterWithLocale("el").Sprintf("Hello")
	if s4 != "Hello in English" {
		t.Errorf("msgPrinter should print 'Hello in English' but got '%v'.", s4)
	}
}
