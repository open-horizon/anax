package agreementbot

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/citizenscientist"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/worker"
	"net/http"
	"sort"
)

type API struct {
	worker.Manager // embedded field
	db             *bolt.DB
}

func NewAPIListener(config *config.HorizonConfig, db *bolt.DB) *API {
	messages := make(chan events.Message)

	listener := &API{
		Manager: worker.Manager{
			Config:   config,
			Messages: messages,
		},

		db: db,
	}

	listener.listen(config.AgreementBot.APIListen)
	return listener
}

// Worker framework functions
func (a *API) Messages() chan events.Message {
	return a.Manager.Messages
}

func (a *API) NewEvent(ev events.Message) {
	if a.Config.AgreementBot.APIListen == "" {
		return
	}

	return
}

func (a *API) listen(apiListen string) {
	glog.Info("Starting AgreementBot API server")

	// If there is no Agbot config, we will terminate
	if a.Config.AgreementBot.APIListen == "" {
		glog.Errorf("AgreementBotWorker API terminating, no AgreementBot API config.")
		return
	} else if a.db == nil {
		glog.Errorf("AgreementBotWorker API terminating, no AgreementBot database configured.")
		return
	}

	nocache := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Add("Pragma", "no-cache, no-store")
			w.Header().Add("Access-Control-Allow-Origin", "*")
			w.Header().Add("Access-Control-Allow-Headers", "X-Requested-With, content-type, Authorization")
			w.Header().Add("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
			h.ServeHTTP(w, r)
		})
	}

	go func() {
		router := mux.NewRouter()

		router.HandleFunc("/agreement", a.agreement).Methods("GET", "OPTIONS")
		router.HandleFunc("/agreement/{id}", a.agreement).Methods("GET", "DELETE", "OPTIONS")

		http.ListenAndServe(apiListen, nocache(router))
	}()
}

func (a *API) agreement(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case "GET":
		pathVars := mux.Vars(r)
		id := pathVars["id"]

		if id != "" {
			if ag, err := FindSingleAgreementByAgreementId(a.db, id, citizenscientist.PROTOCOL_NAME, []AFilter{}); err != nil {
				glog.Error(APIlogString(fmt.Sprintf("error finding agreement %v, error: %v", id, err)))
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			} else if ag == nil {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "id", Error: "agreement id not found"})
			} else {
				serial, err := json.Marshal(*ag)
				if err != nil {
					glog.Errorf(APIlogString(fmt.Sprintf("error serializing agreement output %v, error: %v", *ag, err)))
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				if _, err := w.Write(serial); err != nil {
					glog.Infof(APIlogString(fmt.Sprintf("error writing response %v, error: %v", serial, err)))
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				}
			}
		} else {
			if ags, err := FindAgreements(a.db, []AFilter{}, citizenscientist.PROTOCOL_NAME); err != nil {
				glog.Error(APIlogString(fmt.Sprintf("error finding all agreements, error: %v", err)))
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			} else {
				var agreementsKey = "agreements"
				var archivedKey = "archived"
				var activeKey = "active"

				wrap := make(map[string]map[string][]Agreement, 0)
				wrap[agreementsKey] = make(map[string][]Agreement, 0)
				wrap[agreementsKey][archivedKey] = []Agreement{}
				wrap[agreementsKey][activeKey] = []Agreement{}

				for _, agreement := range ags {
					// The archived agreements and the agreements being terminated are returned as archived.
					if agreement.Archived || agreement.AgreementTimedout != 0 {
					 	wrap[agreementsKey][archivedKey] = append(wrap[agreementsKey][archivedKey], agreement)
					} else {
						wrap[agreementsKey][activeKey] = append(wrap[agreementsKey][activeKey], agreement)
					}
				}

				// do sorts
				sort.Sort(AgreementsByAgreementCreationTime(wrap[agreementsKey][activeKey]))
				sort.Sort(AgreementsByAgreementTimeoutTime(wrap[agreementsKey][archivedKey]))

				serial, err := json.Marshal(wrap)
				if err != nil {
					glog.Errorf(APIlogString(fmt.Sprintf("error serializing agreement output %v, error: %v", wrap, err)))
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				if _, err := w.Write(serial); err != nil {
					glog.Infof(APIlogString(fmt.Sprintf("error writing response %v, error: %v", serial, err)))
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				}
			}
		}

	case "DELETE":
		pathVars := mux.Vars(r)
		id := pathVars["id"]

		if id == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		glog.V(3).Infof(APIlogString(fmt.Sprintf("handling DELETE of agreement: %v", r)))

		if ag, err := FindSingleAgreementByAgreementId(a.db, id, citizenscientist.PROTOCOL_NAME, []AFilter{UnarchivedAFilter()}); err != nil {
			glog.Error(APIlogString(fmt.Sprintf("error finding agreement %v, error: %v", id, err)))
			w.WriteHeader(http.StatusInternalServerError)
		} else if ag == nil {
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "id", Error: "agreement id not found"})
		} else {
			if ag.AgreementTimedout == 0 {
				// Update the database
				if _, err := AgreementTimedout(a.db, ag.CurrentAgreementId, citizenscientist.PROTOCOL_NAME); err != nil {
					glog.Errorf(APIlogString(fmt.Sprintf("error marking agreement %v terminated: %v", ag.CurrentAgreementId, err)))
				}
				a.Messages() <- events.NewABApiAgreementCancelationMessage(events.AGREEMENT_ENDED, citizenscientist.AB_USER_REQUESTED, ag.AgreementProtocol, ag.CurrentAgreementId)
			} else {
				glog.V(3).Infof(APIlogString(fmt.Sprintf("agreement %v not deleted, already timed out at %v", id, ag.AgreementTimedout)))
			}
			w.WriteHeader(http.StatusOK)
		}

	case "OPTIONS":
		w.Header().Set("Allow", "GET, DELETE, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

type APIUserInputError struct {
	Error string `json:"error"`
	Input string `json:"input,omitempty"`
}

func writeInputErr(writer http.ResponseWriter, status int, inputErr *APIUserInputError) {
	if serial, err := json.Marshal(inputErr); err != nil {
		glog.Infof("Error serializing agreement output: %v", err)
		http.Error(writer, "Internal server error", http.StatusInternalServerError)
	} else {
		writer.WriteHeader(status)
		writer.Header().Set("Content-Type", "application/json")
		if _, err := writer.Write(serial); err != nil {
			glog.Infof("Error writing response: %v", err)
			http.Error(writer, "Internal server error", http.StatusInternalServerError)
		}
	}
}

// Helper functions for sorting
type AgreementsByAgreementCreationTime []Agreement

func (s AgreementsByAgreementCreationTime) Len() int {
	return len(s)
}

func (s AgreementsByAgreementCreationTime) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s AgreementsByAgreementCreationTime) Less(i, j int) bool {
	return s[i].AgreementInceptionTime < s[j].AgreementInceptionTime
}

type AgreementsByAgreementTimeoutTime []Agreement

func (s AgreementsByAgreementTimeoutTime) Len() int {
	return len(s)
}

func (s AgreementsByAgreementTimeoutTime) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s AgreementsByAgreementTimeoutTime) Less(i, j int) bool {
	return s[i].AgreementTimedout < s[j].AgreementTimedout
}

// Log string prefix api
var APIlogString = func(v interface{}) string {
	return fmt.Sprintf("AgreementBotWorker API %v", v)
}
