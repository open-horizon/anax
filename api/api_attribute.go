package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"github.com/open-horizon/anax/persistence"
)

func (a *API) attribute(w http.ResponseWriter, r *http.Request) {

	errorhandler := GetHTTPErrorHandler(w)

	vars := mux.Vars(r)
	glog.V(5).Infof(apiLogString(fmt.Sprintf("Attribute vars: %v", vars)))
	id := vars["id"]

	existingDevice, errWritten := a.existingDeviceOrError(w)
	if errWritten {
		return
	}

	var decodedID string
	if id != "" {
		var err error
		decodedID, err = url.PathUnescape(id)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	// shared logic between payload-handling update functions
	handlePayload := func(permitPartial bool, doModifications func(permitPartial bool, attr persistence.Attribute)) {
		defer r.Body.Close()

		if attrs, inputErr, err := payloadToAttributes(errorhandler, r.Body, permitPartial, existingDevice); err != nil {
			glog.Error(apiLogString(fmt.Sprintf("Error processing incoming attributes %v", err)))
			w.WriteHeader(http.StatusInternalServerError)
		} else if !inputErr {
			glog.V(6).Infof(apiLogString(fmt.Sprintf("persistent-type attributes: %v", attrs)))

			if len(attrs) != 1 {
				// only one attr may be specified to add at a time
				w.WriteHeader(http.StatusBadRequest)
			} else {
				doModifications(permitPartial, attrs[0])
			}
		}
	}

	handleUpdateFn := func() func(bool, persistence.Attribute) {
		return func(permitPartial bool, attr persistence.Attribute) {
			if added, err := persistence.SaveOrUpdateAttribute(a.db, attr, decodedID, permitPartial); err != nil {
				switch err.(type) {
				case *persistence.OverwriteCandidateNotFound:
					glog.V(3).Infof(apiLogString(fmt.Sprintf("User attempted attribute update but there isn't a matching persisting attribute to modify.")))
					w.WriteHeader(http.StatusNotFound)
				default:
					glog.Error(apiLogString(fmt.Sprintf("Error persisting attribute: %v", err)))
					w.WriteHeader(http.StatusInternalServerError)
				}
			} else if added != nil {
				writeResponse(w, toOutModel(*added), http.StatusOK)
			} else {
				glog.Error(apiLogString(fmt.Sprintf("Attribute was not successfully persisted but no error was returned from persistence module")))
				w.WriteHeader(http.StatusInternalServerError)
			}
		}
	}

	switch r.Method {
	case "OPTIONS":
		w.Header().Set("Allow", "OPTIONS, HEAD, GET, POST, PUT, PATCH, DELETE")

	case "HEAD":
		returned, err := persistence.FindAttributeByKey(a.db, decodedID)
		if err != nil {
			glog.Error(apiLogString(fmt.Sprintf("Attribute was not successfully deleted %v", err)))
			w.WriteHeader(http.StatusInternalServerError)
		}
		out := wrapAttributesForOutput([]persistence.Attribute{*returned}, decodedID)

		if serial, errWritten := serializeResponse(w, out); !errWritten {
			w.Header().Add("Content-Length", strconv.Itoa(len(serial)))
			w.WriteHeader(http.StatusOK)
		}

	case "GET":
		out, err := FindAndWrapAttributesForOutput(a.db, decodedID)
		glog.V(5).Infof(apiLogString(fmt.Sprintf("returning %v for query of %v", out, decodedID)))
		if err != nil {
			glog.Error(apiLogString(fmt.Sprintf("Error reading persisted attributes %v", err)))
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			writeResponse(w, out, http.StatusOK)
		}

	case "POST":
		// can't POST with an id, POST is only for new records
		if decodedID != "" {
			w.WriteHeader(http.StatusBadRequest)
		} else {

			// call handlePayload with function to do additions
			handlePayload(false, func(permitPartial bool, attr persistence.Attribute) {

				if added, err := persistence.SaveOrUpdateAttribute(a.db, attr, decodedID, permitPartial); err != nil {
					glog.Infof(apiLogString(fmt.Sprintf("Got error from attempted save: <%T>, %v", err, err == nil)))
					switch err.(type) {
					case *persistence.ConflictingAttributeFound:
						w.WriteHeader(http.StatusConflict)
					default:
						glog.Error(apiLogString(fmt.Sprintf("Error persisting attribute: %v", err)))
						w.WriteHeader(http.StatusInternalServerError)
					}
				} else if added != nil {
					writeResponse(w, toOutModel(*added), http.StatusCreated)
				} else {
					glog.Error(apiLogString(fmt.Sprintf("Attribute was not successfully persisted but no error was returned from persistence module")))
					w.WriteHeader(http.StatusInternalServerError)
				}
			})
		}

	case "PUT":
		// must PUT with an id, this is a complete replacement of the document body
		if decodedID == "" {
			w.WriteHeader(http.StatusNotFound)
		} else {
			// call handlePayload with function to do updates but prohibit partial updates
			handlePayload(false, handleUpdateFn())
		}

	case "PATCH":
		if decodedID == "" {
			w.WriteHeader(http.StatusNotFound)
		} else {
			// call handlePayload with function to do updates and allow partial updates
			handlePayload(true, handleUpdateFn())
		}

	case "DELETE":
		if decodedID == "" {
			w.WriteHeader(http.StatusNotFound)
		} else {
			deleted, err := persistence.DeleteAttribute(a.db, decodedID)
			if err != nil {
				glog.Error(apiLogString(fmt.Sprintf("Attribute was not successfully deleted %v", err)))
				w.WriteHeader(http.StatusInternalServerError)
			} else if deleted == nil {
				// nothing deleted, 200 w/ no return
				w.WriteHeader(http.StatusOK)
			} else {
				writeResponse(w, toOutModel(*deleted), http.StatusOK)
			}
		}

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
