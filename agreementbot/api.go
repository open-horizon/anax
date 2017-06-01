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
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/worker"
	"io/ioutil"
	"net/http"
	"sort"
)

type API struct {
	worker.Manager // embedded field
	db             *bolt.DB
	pm             *policy.PolicyManager
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
	if apiListen == "" {
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
		router.HandleFunc("/policy/{name}/upgrade", a.policyUpgrade).Methods("POST", "OPTIONS")
		router.HandleFunc("/workloadusage", a.workloadusage).Methods("GET", "OPTIONS")

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

func (a *API) policyUpgrade(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		pathVars := mux.Vars(r)
		policyName := pathVars["name"]

		if policyName == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		glog.V(3).Infof(APIlogString(fmt.Sprintf("handling POST of policy: %v", policyName)))

		// Verify the input policy name. It can be either the name of the policy within the header of the policy file or the name
		// of the file itself.
		found := false
		if pm, err := policy.Initialize(a.Config.AgreementBot.PolicyPath); err != nil {
			glog.Error(APIlogString(fmt.Sprintf("error initializing policy manager, error: %v", err)))
			w.WriteHeader(http.StatusInternalServerError)
			return
		} else {
			if pol := pm.GetPolicy(policyName); pol != nil {
				found = true
			} else {
				for fileName, _ := range pm.WatcherContent {
					if fileName == policyName {
						policyName = pm.WatcherContent[fileName].Pol.Header.Name
						found = true
						break
					}
				}
			}
		}

		if !found {
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "policy name", Error: fmt.Sprintf("no policies with the name %v", policyName)})
			return
		}

		// Demarshal the input body and verify it.
		var upgrade UpgradeDevice
		body, _ := ioutil.ReadAll(r.Body)
		if err := json.Unmarshal(body, &upgrade); err != nil {
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "body", Error: fmt.Sprintf("user submitted data couldn't be deserialized to struct: %v. Error: %v", string(body), err)})
			return
		} else if ok, msg := upgrade.IsValid(); !ok {
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "body", Error: msg})
			return
		} else {

			// The body is syntacticly correct, verify that the agreement id matches up with the device id and policy name.
			if upgrade.AgreementId != "" {
				if ag, err := FindSingleAgreementByAgreementId(a.db, upgrade.AgreementId, citizenscientist.PROTOCOL_NAME, []AFilter{UnarchivedAFilter()}); err != nil {
					glog.Error(APIlogString(fmt.Sprintf("error finding agreement %v, error: %v", upgrade.AgreementId, err)))
					w.WriteHeader(http.StatusInternalServerError)
					return
				} else if ag == nil {
					writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementId", Error: "agreement id not found"})
					return
				} else if ag.AgreementTimedout != 0 {
					writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementId", Error: fmt.Sprintf("agreement %v not upgraded, already timed out at %v", upgrade.AgreementId, ag.AgreementTimedout)})
					return
				} else if upgrade.Device != "" && ag.DeviceId != upgrade.Device {
					writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementId", Error: fmt.Sprintf("agreement %v not upgraded, not with specified device id %v", upgrade.AgreementId, upgrade.Device)})
					return
				} else if ag.PolicyName != policyName {
					writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "agreementId", Error: fmt.Sprintf("agreement %v not upgraded, not using policy %v", upgrade.AgreementId, policyName)})
					return
				} else {
					// We have a valid agreement. Make sure we get the device id to pass along in the event if it isnt in the input.
					if upgrade.Device == "" {
						upgrade.Device = ag.DeviceId
					}
				}

			}

			// Verfiy that the device is using the workload rollback feature
			if upgrade.Device != "" {
				if wlUsage, err := FindSingleWorkloadUsageByDeviceAndPolicyName(a.db, upgrade.Device, policyName); err != nil {
					glog.Error(APIlogString(fmt.Sprintf("error finding workload usage record for %v with policy %v, error: %v", upgrade.AgreementId, policyName, err)))
					w.WriteHeader(http.StatusInternalServerError)
					return
				} else if wlUsage == nil {
					writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "device", Error: fmt.Sprintf("device %v with policy %v is not using the workload rollback feature", upgrade.Device, policyName)})
					return
				}
			}

			// If we got this far, begin workload upgrade processing.
			a.Messages() <- events.NewABApiWorkloadUpgradeMessage(events.WORKLOAD_UPGRADE, citizenscientist.PROTOCOL_NAME, upgrade.AgreementId, upgrade.Device, policyName)
			w.WriteHeader(http.StatusOK)
		}

	case "OPTIONS":
		w.Header().Set("Allow", "POST, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) workloadusage(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case "GET":
		if wlusages, err := FindWorkloadUsages(a.db, []WUFilter{}); err != nil {
			glog.Error(APIlogString(fmt.Sprintf("error finding all workload usages, error: %v", err)))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		} else {

			// do sort
			sort.Sort(WorkloadUsagesByDeviceId(wlusages))

			serial, err := json.Marshal(wlusages)
			if err != nil {
				glog.Errorf(APIlogString(fmt.Sprintf("error serializing workload usage output %v, error: %v", wlusages, err)))
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

	case "OPTIONS":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// ==========================================================================================
// Utility functions used by many of the API endpoints.
//
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

// Helper functions for sorting agreements
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

// Helper functions for sorting workload usages
type WorkloadUsagesByDeviceId []WorkloadUsage

func (s WorkloadUsagesByDeviceId) Len() int {
	return len(s)
}

func (s WorkloadUsagesByDeviceId) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s WorkloadUsagesByDeviceId) Less(i, j int) bool {
	return s[i].DeviceId < s[j].DeviceId
}

// Log string prefix api
var APIlogString = func(v interface{}) string {
	return fmt.Sprintf("AgreementBotWorker API %v", v)
}

type UpgradeDevice struct {
	Device      string `json:"device"`
	AgreementId string `json:"agreementId"`
}

func (b *UpgradeDevice) IsValid() (bool, string) {
	if b.Device == "" && b.AgreementId == "" {
		return false, "must specify either device or agreementId"
	}
	return true, ""
}
