package api

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/i18n"
	"net/http"
	"strings"
)

// get the eventlogs for current registration.
func (a *API) eventlog(w http.ResponseWriter, r *http.Request) {

	resource := "eventlog"

	errorHandler := GetHTTPErrorHandler(w)

	switch r.Method {
	case "GET":
		// get message printer with the language passed in from the header
		lan := r.Header.Get("Accept-Language")
		if lan == "" {
			lan = i18n.DEFAULT_LANGUAGE
		}
		msgPrinter := i18n.GetMessagePrinterWithLocale(lan)

		all_loags := false
		if r.URL != nil && strings.Contains(r.URL.Path, "all") {
			all_loags = true
		}

		if err := r.ParseForm(); err != nil {
			errorHandler(NewAPIUserInputError(msgPrinter.Sprintf("Error parsing the selections %v. %v", r.Form, err), "selection"))
			return
		}

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v with selection %v. Language: %v", r.Method, resource, r.Form, lan)))

		if out, err := FindEventLogsForOutput(a.db, all_loags, r.Form, msgPrinter); err != nil {
			errorHandler(NewSystemError(msgPrinter.Sprintf("Error getting %v for output, error %v", resource, err)))
		} else {
			writeResponse(w, out, http.StatusOK)
		}
	case "DELETE":
		// get message printer with the language passed in from the header
		lan := r.Header.Get("Accept-Language")
		if lan == "" {
			lan = i18n.DEFAULT_LANGUAGE
		}
		msgPrinter := i18n.GetMessagePrinterWithLocale(lan)

		prune := false
		if r.URL != nil && strings.Contains(r.URL.Path, "prune") {
			prune = true
		}

		if err := r.ParseForm(); err != nil {
			errorHandler(NewAPIUserInputError(msgPrinter.Sprintf("Error parsing the selections %v. %v", r.Form, err), "selection"))
			return
		}

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v with selection %v. Language: %v", r.Method, resource, r.Form, lan)))

		if count, err := DeleteEventLogs(a.db, prune, r.Form, msgPrinter); err != nil {
			errorHandler(NewSystemError(msgPrinter.Sprintf("Error deleting %v, error %v", resource, err)))
		} else if count > 0 {
			if prune {
				writeResponse(w, fmt.Sprintf("%v", count), http.StatusOK)
			} else {
				writeResponse(w, fmt.Sprintf("%v", count), http.StatusOK)
			}
		} else {
			writeResponse(w, fmt.Sprintf("No matching event log entries found."), http.StatusNoContent)
		}
	case "OPTIONS":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}

}

func (a *API) surface(w http.ResponseWriter, r *http.Request) {
	resource := "eventlog/surface"
	errorHandler := GetHTTPErrorHandler(w)

	switch r.Method {
	case "GET":
		lan := r.Header.Get("Accept-Language")
		if lan == "" {
			lan = i18n.DEFAULT_LANGUAGE
		}
		msgPrinter := i18n.GetMessagePrinterWithLocale(lan)

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v. Language: %v", r.Method, resource, lan)))

		if out, err := FindSurfaceLogsForOutput(a.db, msgPrinter); err != nil {
			errorHandler(NewSystemError(msgPrinter.Sprintf("Error getting %v for output, error %v", resource, err)))
		} else {
			writeResponse(w, out, http.StatusOK)
		}
	case "OPTIONS":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
