package resource

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
        "github.com/golang/glog"
        "github.com/open-horizon/anax/persistence"
	"net/http"
	"strconv"
	"strings"
)

const (
	getSecretsURL        = "/secrets"
	secretsURL           = "/secrets/"
	contentType          = "Content-Type"
	applicationJSON      = "application/json"
)

type secretObject struct {
	Details string `json:"details"`
}

var unauthorizedBytes = []byte("Unauthorized")

type SecretAPI struct {
	db *bolt.DB
	authenticator *SecretsAPIAuthenticate
}

func NewSecretAPI(db *bolt.DB, am *AuthenticationManager) *SecretAPI {
	auth := &SecretsAPIAuthenticate{
		AuthMgr: am,
	}
	return &SecretAPI{
		db: db,
		authenticator: auth,
	}
}

func (api *SecretAPI) SetUpHttpHandler() {
	http.Handle(getSecretsURL, http.StripPrefix(getSecretsURL, http.HandlerFunc(api.handleGetSecrets)))
	http.Handle(secretsURL, http.StripPrefix(secretsURL, http.HandlerFunc(api.handleSecrets)))

}

func (api *SecretAPI) SetupAuthenticator(auth *SecretsAPIAuthenticate) {
	api.authenticator = auth
}

func (api *SecretAPI) handleGetSecrets(writer http.ResponseWriter, request *http.Request) {
	setResponseHeaders(writer)

	// GET /secrets
        if request.Method != http.MethodGet {
                writer.WriteHeader(http.StatusMethodNotAllowed)
                return
        }

	glog.V(3).Infof(secLogString(fmt.Sprintf("Calling GET /secrets")))
	authenticated, serviceName, err := api.authenticator.Authenticate(request)
	if !authenticated {
		glog.Errorf(secLogString(fmt.Sprintf("handleGetSecrets authenticate error: %v", err)))
		writer.WriteHeader(http.StatusForbidden)
		writer.Write(unauthorizedBytes)
		return
	} else if secs, err := persistence.FindUpdatedSecrets(api.db, serviceName); err != nil {
		message := fmt.Sprintf("Failed to fetch the secret names.")
		returnErrorResponse(writer, err, message, http.StatusInternalServerError)
	} else if len(secs) == 0 {
		writer.WriteHeader(http.StatusNotFound)
		return
	} else {
		result := make([]string, 0)
		for _, sec := range secs {
			result = append(result, sec.SvcSecretName)
		}

		if data, err := json.MarshalIndent(result, "", "  "); err != nil {
			returnErrorResponse(writer, err, "Failed to marshal the list of secret names.", http.StatusInternalServerError)
		} else {
			writer.Header().Add(contentType, applicationJSON)
			writer.WriteHeader(http.StatusOK)
			if _, err := writer.Write(data); err != nil {
				glog.Errorf(secLogString(fmt.Sprintf("GET /secrets, failed to write to response body: %v", err)))
			}
			return
		}
	}

}

func (api *SecretAPI) handleSecrets(writer http.ResponseWriter, request *http.Request) {
	setResponseHeaders(writer)

	authenticated, serviceName, err := api.authenticator.Authenticate(request)
	if !authenticated {
		glog.Errorf(secLogString(fmt.Sprintf("handleSecrets authenticate error: %v", err)))
                writer.WriteHeader(http.StatusForbidden)
                writer.Write(unauthorizedBytes)
                return
        }

	parts := strings.Split(request.URL.Path, "/")

	if len(parts) == 0 {
		api.handleGetSecrets(writer, request)
		return
	} else if len(parts) == 1 {
		// GET /secrets/<secret-name>
		// POST /secrets/<secret-name>?received=true
		secretName := parts[0]
		if secretName == "" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}

		switch request.Method {
		case http.MethodPost:
			//POST /secrets/<secret-name>?received=true
			receivedString := request.URL.Query().Get("received")

			glog.V(3).Infof(secLogString(fmt.Sprintf("Calling POST /secrets/%s?received=%s", secretName, receivedString)))
			received := false
			if receivedString != "" {
				var err error
				if received, err = strconv.ParseBool(receivedString); err != nil {
					writer.WriteHeader(http.StatusBadRequest)
					return
				} else if received {
					// call persistent to find secret
					if psecret, err := persistence.FindSecrets(api.db, secretName, serviceName); err != nil {
						returnErrorResponse(writer, err, "Failed to find secret to mark received.", http.StatusInternalServerError)
					} else if isEmptySecretObject(*psecret) {
						returnErrorResponse(writer, err, "Secret not found to mark received.", http.StatusNotFound)
					} else if psecret.SvcSecretStatus == persistence.Received {
						writer.WriteHeader(http.StatusCreated)
						return
					}

					// call persistent to update secret status
					if err := persistence.UpdateSecretStatus(api.db, secretName, serviceName, persistence.Received); err != nil {
						returnErrorResponse(writer, err, "Failed to mark secret received.", http.StatusInternalServerError)
					}
					writer.WriteHeader(http.StatusCreated)
					return
				} else {
					// received == false
					glog.V(3).Infof(secLogString(fmt.Sprintf("POST /secrets/%s?received=%s, return 201 directly", secretName, received)))
					writer.WriteHeader(http.StatusCreated)
					return
				}
			}

		case http.MethodGet:
			//GET /secrets/<secret-name>
			glog.V(3).Infof(secLogString(fmt.Sprintf("Calling GET /secrets/%s", secretName)))

			// call persistent to get secret object (key: serviceName/secretName)
			// get secret value: secret.SvcSecretValue
			if psecret, err := persistence.FindSecrets(api.db, secretName, serviceName); err != nil {
				returnErrorResponse(writer, err, "Failed to find secret.", http.StatusInternalServerError)
			} else if isEmptySecretObject(*psecret) {
				returnErrorResponse(writer, err, "Secret not found.", http.StatusNotFound)
			} else {
				// return secret
				m := make(map[string]*secretObject, 0)

				secretName := psecret.SvcSecretName
				secretObj := &secretObject{Details: psecret.SvcSecretValue}
				m[secretName] = secretObj

				if data, err := json.MarshalIndent(m, "", "  "); err != nil {
					returnErrorResponse(writer, err, "Failed to marshal the secret.", http.StatusInternalServerError)
				} else {
					writer.Header().Add(contentType, applicationJSON)
					writer.WriteHeader(http.StatusOK)
					if _, err := writer.Write(data); err != nil {
						glog.Errorf(secLogString(fmt.Sprintf("GET /secrets/<secretName>, failed to write to response body: %v", err)))
					}
					return
				}

			}
		default:
			writer.WriteHeader(http.StatusMethodNotAllowed)
		}
	} else {
		// error
		writer.WriteHeader(http.StatusBadRequest)
	}
}

func returnErrorResponse(writer http.ResponseWriter, err error, message string, statusCode int) {
	writer.WriteHeader(statusCode)
	if message != "" || err != nil {
		writer.Header().Add("Content-Type", "Text/Plain")
		buffer := bytes.NewBufferString(message)
		if err != nil {
			buffer.WriteString(fmt.Sprintf("Error: %s", err.Error()))
		}
		buffer.WriteString("\n")
		writer.Write(buffer.Bytes())
	}
}

func isEmptySecretObject(psecret persistence.PersistedServiceSecret) bool {
	if psecret.SvcOrgid == "" && psecret.SvcUrl == "" && psecret.SvcSecretName == "" &&
		psecret.SvcSecretValue == "" && psecret.SvcSecretStatus == "" {
		return true
	}
	return false
}

// Set HTTP cache control headers for http 1.0 and 1.1 clients.
func setResponseHeaders(writer http.ResponseWriter) {
	// Set HTTP cache control headers for http 1.0 and 1.1 clients.
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Pragma", "no-cache")
}

// Logging function
var secLogString = func(v interface{}) string {
	return fmt.Sprintf("Secrets API: %v", v)
}
