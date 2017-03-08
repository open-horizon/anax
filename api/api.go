package api

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"io/ioutil"
	"net/http"

	"bytes"
	"github.com/boltdb/bolt"
	dockerclient "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/citizenscientist"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/device"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/worker"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

type API struct {
	worker.Manager // embedded field
	db             *bolt.DB
	pm             *policy.PolicyManager
}

func NewAPIListener(config *config.HorizonConfig, db *bolt.DB, pm *policy.PolicyManager) *API {
	messages := make(chan events.Message)

	listener := &API{
		Manager: worker.Manager{
			Config:   config,
			Messages: messages,
		},

		db: db,
		pm: pm,
	}

	listener.listen(config.Edge.APIListen)
	return listener
}

// Worker framework functions
func (a *API) Messages() chan events.Message {
	return a.Manager.Messages
}

func (a *API) NewEvent(ev events.Message) {
	return
}

func (a *API) listen(apiListen string) {
	glog.Info("Starting Anax API server")

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

		// N.B. the following two paths are the primary registration endpoints as of v2.1.0; these notions
		// get split apart when a proper microservice / workload prefs split is established in the future
		router.HandleFunc("/service", a.service).Methods("GET", "POST", "OPTIONS")
		router.HandleFunc("/service/attribute", a.serviceAttribute).Methods("GET", "POST", "DELETE", "OPTIONS")

		router.HandleFunc("/status", a.status).Methods("GET", "OPTIONS")
		router.HandleFunc("/token/random", tokenRandom).Methods("GET", "OPTIONS")
		router.HandleFunc("/horizondevice", a.horizonDevice).Methods("GET", "POST", "PATCH", "OPTIONS")
		router.HandleFunc("/workload", a.workload).Methods("GET", "OPTIONS")

		// redirect to index.html because SPA
		router.HandleFunc(`/{p:[\w\/]+}`, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		})
		router.PathPrefix("/").Handler(http.FileServer(http.Dir(a.Config.Edge.StaticWebContent)))

		glog.Infof("Serving static web content from: %v", a.Config.Edge.StaticWebContent)
		http.ListenAndServe(apiListen, nocache(router))
	}()
}

func (a *API) agreement(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case "GET":
		pathVars := mux.Vars(r)
		id := pathVars["id"]

		// we don't support getting just one yet
		if id != "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		agreements, err := persistence.FindEstablishedAgreements(a.db, citizenscientist.PROTOCOL_NAME, []persistence.EAFilter{})
		if err != nil {
			glog.Error(err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		var agreementsKey = "agreements"
		var archivedKey = "archived"
		var activeKey = "active"

		wrap := make(map[string]map[string][]persistence.EstablishedAgreement, 0)
		wrap[agreementsKey] = make(map[string][]persistence.EstablishedAgreement, 0)
		wrap[agreementsKey][archivedKey] = []persistence.EstablishedAgreement{}
		wrap[agreementsKey][activeKey] = []persistence.EstablishedAgreement{}

		for _, agreement := range agreements {
			// The archived agreements and the agreements being terminated are returned as archived.
			if agreement.Archived || agreement.AgreementTerminatedTime != 0 {
				wrap[agreementsKey][archivedKey] = append(wrap[agreementsKey][archivedKey], agreement)
			} else {
				wrap[agreementsKey][activeKey] = append(wrap[agreementsKey][activeKey], agreement)
			}
		}

		// do sorts
		sort.Sort(EstablishedAgreementsByAgreementCreationTime(wrap[agreementsKey][activeKey]))
		sort.Sort(EstablishedAgreementsByAgreementTerminatedTime(wrap[agreementsKey][archivedKey]))

		serial, err := json.Marshal(wrap)
		if err != nil {
			glog.Infof("Error serializing agreement output: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(serial); err != nil {
			glog.Infof("Error writing response: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	case "DELETE":
		pathVars := mux.Vars(r)
		id := pathVars["id"]

		if id == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		glog.V(3).Infof("Handling DELETE of agreement: %v", r)

		var filters []persistence.EAFilter
		filters = append(filters, persistence.UnarchivedEAFilter())
		filters = append(filters, persistence.IdEAFilter(id))

		if agreements, err := persistence.FindEstablishedAgreements(a.db, citizenscientist.PROTOCOL_NAME, filters); err != nil {
			glog.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
		} else if len(agreements) == 0 {
			w.WriteHeader(http.StatusNotFound)
		} else {
			// write message
			ct := agreements[0]
			if ct.AgreementTerminatedTime == 0 {
				a.Messages() <- events.NewApiAgreementCancelationMessage(events.AGREEMENT_ENDED, events.AG_TERMINATED, ct.AgreementProtocol, ct.CurrentAgreementId, ct.CurrentDeployment)
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

func (a *API) devmode(w http.ResponseWriter, r *http.Request) {
	glog.V(3).Infof("devmode handling request: %v", r)

	switch r.Method {
	case "GET":
		// get the devmode status
		if mode, err := persistence.GetDevmode(a.db); err != nil {
			glog.Infof("Error getting devmode from db:%v", err)
		} else {
			if serial, err := json.Marshal(mode); err != nil {
				glog.Error(err)
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				w.Header().Set("Content-Type", "application/json")
				if _, err := w.Write(serial); err != nil {
					glog.Error(err)
					w.WriteHeader(http.StatusInternalServerError)
				} else {
					w.WriteHeader(http.StatusOK)
				}
			}
		}
	case "OPTIONS":
		w.Header().Set("Allow", "OPTIONS, POST, GET")
		w.WriteHeader(http.StatusOK)
	case "POST":
		var mode persistence.DevMode
		body, _ := ioutil.ReadAll(r.Body)
		if err := json.Unmarshal(body, &mode); err != nil {
			glog.Infof("User submitted data couldn't be deserialized to Devmode struct: %v. Error: %v", string(body), err)
			w.WriteHeader(http.StatusBadRequest)
		} else {
			glog.Infof("devemode=%v", mode)

			if err := persistence.SaveDevmode(a.db, mode); err != nil {
				glog.Error("Error saving devmode: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				w.WriteHeader(http.StatusOK)
			}
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) horizonDevice(w http.ResponseWriter, r *http.Request) {

	// returns existing device ref and boolean if error occured during fetch (error output handled by this func)
	fetch := func(device *HorizonDevice) (*persistence.ExchangeDevice, bool) {
		existing, err := persistence.FindExchangeDevice(a.db)
		if err != nil {
			glog.Errorf("Failed fetching existing exchange device. Error: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return nil, true
		}

		return existing, false
	}

	writeResponse := func(exDevice *persistence.ExchangeDevice, successStatusCode int) (*HorizonDevice, bool) {
		id, _ := device.Id()

		var outModel *HorizonDevice

		if exDevice == nil {
			outModel = &HorizonDevice{
				Id: &id,
			}
		} else {
			// assume input struct is well-formed, should come from persisted record
			outModel = &HorizonDevice{
				Name:               &exDevice.Name,
				Id:                 &id,
				TokenValid:         &exDevice.TokenValid,
				TokenLastValidTime: &exDevice.TokenLastValidTime,
				Account: &HorizonAccount{
					Id:    &exDevice.Account.Id,
					Email: &exDevice.Account.Email,
				},
			}
		}

		serial, err := json.Marshal(outModel)
		if err != nil {
			glog.Error(err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return nil, true
		}

		w.WriteHeader(successStatusCode)
		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write(serial); err != nil {
			glog.Error(err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return nil, true
		}

		return outModel, false
	}

	switch r.Method {
	case "GET":
		existingDevice, err := persistence.FindExchangeDevice(a.db)
		if err != nil {
			glog.Errorf("Failed fetching existing exchange device. Error: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		writeResponse(existingDevice, http.StatusOK)

	case "POST":
		var device HorizonDevice

		body, _ := ioutil.ReadAll(r.Body)
		if err := json.Unmarshal(body, &device); err != nil {
			glog.Infof("User submitted data couldn't be deserialized to Device struct: %v. Error: %v", string(body), err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if device.Account == nil {
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "account", Error: "null and must not be"})
			return
		}
		if bail := checkInputString(w, "device.account.id", device.Account.Id); bail {
			return
		}
		if bail := checkInputString(w, "device.account.email", device.Account.Email); bail {
			return
		}
		if bail := checkInputString(w, "device.name", device.Name); bail {
			return
		}
		if device.Token == nil {
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "device.token", Error: "null and must not be"})
			return
		}
		// don't bother sanitizing token data; we *never* output it, and we *never* compute it

		existing, fetchErrWritten := fetch(&device)
		if fetchErrWritten {
			// errors already written to response writer by fetch function call
			return
		}

		// handle conflict here; should never be a conflict in POST method, PATCH is for update
		if existing != nil {
			w.WriteHeader(http.StatusConflict)
			return
		}

		exDev, err := persistence.SaveNewExchangeDevice(a.db, *device.Token, *device.Name, *device.Account.Id, *device.Account.Email)
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			glog.Errorf("Error persisting new exchange device: %v", err)
			return
		}

		a.Messages() <- events.NewEdgeRegisteredExchangeMessage(events.NEW_DEVICE_REG, *device.Token)
		writeResponse(exDev, http.StatusCreated)

	case "PATCH":
		var device HorizonDevice

		body, _ := ioutil.ReadAll(r.Body)
		if err := json.Unmarshal(body, &device); err != nil {
			glog.Infof("User submitted data couldn't be deserialized to Device struct: %v. Error: %v", string(body), err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if device.Account == nil {
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "account", Error: "null and must not be"})
			return
		}
		if bail := checkInputString(w, "device.account.id", device.Account.Id); bail {
			return
		}
		if device.Token == nil {
			writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "device.token", Error: "null and must not be"})
			return
		}

		existing, fetchErrWritten := fetch(&device)
		if fetchErrWritten {
			// errors already written to response writer by fetch function call
			return
		}

		if existing == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		updatedDevice, err := persistence.SetExchangeDeviceToken(a.db, *device.Account.Id, *device.Token)
		if err != nil {
			glog.Errorf("Error doing token update on horizon device object: %v. Error: %v", existing, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}

		writeResponse(updatedDevice, http.StatusOK)

	case "OPTIONS":
		w.Header().Set("Allow", "GET, POST, PATCH, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// It gets the iotf configuration from the api and saves it to /root/.colonus/ directory
// in .json format.
func (a *API) iotfconf(w http.ResponseWriter, r *http.Request) {
	glog.V(3).Infof("iotfconf handling request: %v", r)

	switch r.Method {
	case "OPTIONS":
		w.Header().Set("Allow", "OPTIONS, POST")
		w.WriteHeader(http.StatusOK)
	case "POST":
		var iotf_conf persistence.IoTFConf
		body, _ := ioutil.ReadAll(r.Body)
		if err := json.Unmarshal(body, &iotf_conf); err != nil {
			glog.Infof("User submitted data couldn't be deserialized to IoTFConf struct: %v. Error: %v", string(body), err)
			w.WriteHeader(http.StatusBadRequest)
		} else {
			// assign the correct arch
			if strings.Contains(strings.ToLower(runtime.GOARCH), "amd") ||
				strings.Contains(strings.ToLower(runtime.GOARCH), "x86") {
				iotf_conf.Arch = "amd64"
			} else {
				iotf_conf.Arch = "arm"
			}

			glog.Infof("iotf_conf=%v", iotf_conf)
			if err := persistence.SaveIoTFConf(a.Config.Edge.DBPath, iotf_conf); err != nil {
				glog.Error("Error saving IoTF configuration in file.: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				w.WriteHeader(http.StatusOK)
			}
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// for registering what *should* be microservices but as of v2.1.0, are more
// like the old contracts
func (a *API) service(w http.ResponseWriter, r *http.Request) {

	findAdditions := func(attrs []persistence.ServiceAttribute, incoming []persistence.ServiceAttribute) []persistence.ServiceAttribute {

		toAdd := []persistence.ServiceAttribute{}

		for _, in := range incoming {
			c := false
			for _, attr := range attrs {
				if in.GetMeta().Id == attr.GetMeta().Id {
					c = true
					break
				}
			}

			if !c {
				toAdd = append(toAdd, in)
			}
		}

		// return the mutated copy
		return toAdd
	}

	switch r.Method {
	case "GET":
		type outServiceWrapper struct {
			Policy     policy.Policy                  `json:"policy"`
			Attributes []persistence.ServiceAttribute `json:"attributes"`
		}

		outServices := make(map[string]interface{}, 0)

		allPolicies := a.pm.GetAllPolicies()

		for _, pol := range allPolicies {

			var applicable []persistence.ServiceAttribute

			for _, apiSpec := range pol.APISpecs {
				pAttr, err := persistence.FindApplicableAttributes(a.db, apiSpec.SpecRef)
				if err != nil {
					glog.Errorf("Failed fetching attributes. Error: %v", err)
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				}

				applicable = append(applicable, findAdditions(applicable, pAttr)...)
			}

			// TODO: consider sorting the attributes returned
			outServices[pol.Header.Name] = outServiceWrapper{
				Policy:     pol,
				Attributes: applicable,
			}
		}

		wrapper := make(map[string]map[string]interface{}, 0)
		wrapper["services"] = outServices

		serial, err := json.Marshal(wrapper)
		if err != nil {
			glog.Infof("Error serializing agreement output: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(serial); err != nil {
			glog.Infof("Error writing response: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

	case "POST":
		existingDevice, err := persistence.FindExchangeDevice(a.db)
		if err != nil {
			glog.Errorf("Failed fetching existing exchange device. Error: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if existingDevice == nil {
			writeInputErr(w, http.StatusFailedDependency, &APIUserInputError{Error: "Exchange registration not recorded. Complete account and device registration with an exchange and then record device registration using this API's /horizondevice path."})
			return
		}

		// input should be: Service type w/ zero or more ServiceAttribute types
		var service Service
		body, _ := ioutil.ReadAll(r.Body)

		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.UseNumber()

		if err := decoder.Decode(&service); err != nil {
			glog.Infof("User submitted data that couldn't be deserialized to service: %v. Error: %v", string(body), err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		glog.V(5).Infof("Service POST: %v", &service)

		if bail := checkInputString(w, "sensor_url", service.SensorUrl); bail {
			return
		}

		if bail := checkInputString(w, "sensor_name", service.SensorName); bail {
			return
		}

		var attributes []persistence.ServiceAttribute
		if service.Attributes != nil {
			// build a serviceAttribute for each one
			var err error
			var inputErrWritten bool
			attributes, err, inputErrWritten = deserializeAttributes(w, *service.Attributes)
			if err != nil {
				glog.Errorf("Failure deserializing attributes: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			} else if inputErrWritten {
				// signifies an already-written user api error
				return
			}
		}

		// check for errors in attribute input, like specifying a sensorUrl
		for _, attr := range attributes {
			if len(attr.GetMeta().SensorUrls) != 0 {
				writeInputErr(w, http.StatusBadRequest, &APIUserInputError{Input: "service.[attribute].sensor_urls", Error: "sensor_urls not permitted on attributes specified on a service"})
				return
			}
			// now make sure we add our own sensorUrl to each attribute

			attr.GetMeta().AppendSensorUrl(*service.SensorUrl)
			glog.Infof("SensorUrls for %v: %v", attr.GetMeta().Id, attr.GetMeta().SensorUrls)
		}

		// check for required
		cType := reflect.TypeOf(persistence.ComputeAttributes{}).String()
		if attributesContains(attributes, *service.SensorUrl, cType) == nil {
			// make a default
			// TODO: create a factory for this
			attributes = append(attributes, persistence.ComputeAttributes{
				Meta: &persistence.AttributeMeta{
					Id:          "compute",
					SensorUrls:  []string{*service.SensorUrl},
					Label:       "Compute Resources",
					Publishable: true,
					Type:        cType,
				},
				CPUs: 1,
				RAM:  a.Config.Edge.DefaultServiceRegistrationRAM,
			})
		}

		aType := reflect.TypeOf(persistence.ArchitectureAttributes{}).String()
		// a little weird; could a user give us an alternate architecture than the one we're going to publising in the prop?
		if attributesContains(attributes, *service.SensorUrl, aType) == nil {
			// make a default

			attributes = append(attributes, persistence.ArchitectureAttributes{
				Meta: &persistence.AttributeMeta{
					Id:          "architecture",
					SensorUrls:  []string{*service.SensorUrl},
					Label:       "Architecture",
					Publishable: true,
					Type:        aType,
				},
				Architecture: cutil.ArchString(),
			})
		}

		// what's advertised
		var policyArch string

		// props to store in file; stuff that is enforced; need to convert from serviceattributes to props. *CAN NEVER BE* unpublishable ServiceAttributes
		props := map[string]string{}

		// persist all prefs; while we're at it, fetch the props we want to publish and the arch
		for _, attr := range attributes {

			_, err := persistence.SaveOrUpdateServiceAttribute(a.db, attr)
			if err != nil {
				glog.Errorf("Error saving attribute: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			switch attr.(type) {
			case persistence.ComputeAttributes:
				compute := attr.(persistence.ComputeAttributes)
				props["cpus"] = strconv.FormatInt(compute.CPUs, 10)
				props["ram"] = strconv.FormatInt(compute.RAM, 10)

			case persistence.ArchitectureAttributes:
				policyArch = attr.(persistence.ArchitectureAttributes).Architecture
			default:
				glog.V(4).Infof("Unhandled attr type (%T): %v", attr, attr)
			}
		}

		glog.V(5).Infof("Complete Attr list for registration of service %v: %v", *service.SensorUrl, attributes)

		if genErr := policy.GeneratePolicy(a.Messages(), *service.SensorName, policyArch, &props, a.Config.Edge.PolicyPath); genErr != nil {
			glog.Errorf("Error: %v", genErr)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// TODO: when there is a way to represent services for output, write it out w/ the 201
		w.WriteHeader(http.StatusCreated)

	case "OPTIONS":
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// for editing prefs used by one or more workloads *and* pushing shared attributes (like location)
func (a *API) serviceAttribute(w http.ResponseWriter, r *http.Request) {

	toOutModel := func(persisted persistence.ServiceAttribute) *Attribute {
		mappings := persisted.GetGenericMappings()

		return &Attribute{
			Id:          &persisted.GetMeta().Id,
			SensorUrls:  &persisted.GetMeta().SensorUrls,
			Label:       &persisted.GetMeta().Label,
			Publishable: &persisted.GetMeta().Publishable,
			Mappings:    &mappings,
		}
	}

	switch r.Method {
	case "GET":
		// empty string to match all
		serviceAttributes, err := persistence.FindApplicableAttributes(a.db, "")
		if err != nil {
			glog.Errorf("Failed fetching existing service attributes. Error: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		outAttrs := []Attribute{}

		for _, persisted := range serviceAttributes {
			// convert persistence model to API model

			outAttr := toOutModel(persisted)
			outAttrs = append(outAttrs, *outAttr)
		}

		wrap := map[string][]Attribute{}
		wrap["attributes"] = outAttrs

		serial, err := json.Marshal(wrap)
		if err != nil {
			glog.Infof("Error serializing agreement output: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(serial); err != nil {
			glog.Infof("Error writing response: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

	case "POST":
		existingDevice, err := persistence.FindExchangeDevice(a.db)
		if err != nil {
			glog.Errorf("Failed fetching existing exchange device. Error: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if existingDevice == nil {
			writeInputErr(w, http.StatusFailedDependency, &APIUserInputError{Error: "Exchange registration not recorded. Complete account and device registration with an exchange and then record device registration using this API's /horizondevice path."})
			return
		}

		body, _ := ioutil.ReadAll(r.Body)
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.UseNumber()

		var attribute Attribute
		if err := decoder.Decode(&attribute); err != nil {
			glog.Infof("User submitted data that couldn't be deserialized to attribute: %v. Error: %v", string(body), err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		serviceAttrs, err, inputErr := deserializeAttributes(w, []Attribute{attribute})
		if inputErr {
			return
		}
		if err != nil {
			glog.Errorf("Error deserializing attributes: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}

		// save to db; we know there was only one
		saved, err := persistence.SaveOrUpdateServiceAttribute(a.db, serviceAttrs[0])
		if err != nil {
			glog.Errorf("Error deserializing attributes: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}

		outAttr := toOutModel(*saved)
		serial, err := json.Marshal(outAttr)
		if err != nil {
			glog.Error(err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write(serial); err != nil {
			glog.Error(err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

	case "OPTIONS":
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) status(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		info := NewInfo(a.Config)

		if err := WriteGethStatus(a.Config.Edge.GethURL, info.Geth); err != nil {
			glog.Errorf("Unable to determine geth service facts: %v", err)
		}

		if err := WriteConnectionStatus(info); err != nil {
			glog.Errorf("Unable to get connectivity status: %v", err)
		}

		if serial, err := json.Marshal(info); err != nil {
			glog.Errorf("Failed to serialize status object: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "application/json")

			if _, err := w.Write(serial); err != nil {
				glog.Errorf("Failed to write response: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}
	case "OPTIONS":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func tokenRandom(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		str, err := cutil.SecureRandomString()
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		out := map[string]string{
			"token": str,
		}

		serial, err := json.Marshal(out)
		if err != nil {
			glog.Error(err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write(serial); err != nil {
			glog.Error(err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

	case "OPTIONS":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) workload(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		if client, err := dockerclient.NewClient(a.Config.Edge.DockerEndpoint); err != nil {
			glog.Error(err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		} else {
			opts := dockerclient.ListContainersOptions{
				All: true,
			}

			if containers, err := client.ListContainers(opts); err != nil {
				glog.Error(err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			} else {
				ret := make(map[string][]dockerclient.APIContainers, 0)
				ret["workloads"] = []dockerclient.APIContainers{}

				for _, c := range containers {
					if _, exists := c.Labels["network.bluehorizon.colonus.service_name"]; exists {
						ret["workloads"] = append(ret["workloads"], c)
					}
				}

				if serial, err := json.Marshal(ret); err != nil {
					glog.Error(err)
					http.Error(w, "Internal server error", http.StatusInternalServerError)
				} else {
					w.Header().Set("Content-Type", "application/json")
					if _, err := w.Write(serial); err != nil {
						glog.Error(err)
						http.Error(w, "Internal server error", http.StatusInternalServerError)
					}
				}
			}
		}

	case "OPTIONS":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}

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
