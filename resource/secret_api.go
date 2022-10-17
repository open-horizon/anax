// Package resource Model Management System and Agent Secrets API
//
// The Model Management System (MMS) delivers AI models and other files needed by edge services to the edge nodes where those services are running. MMS has two components, and therefore two APIs: Cloud Sync Service (CSS) is the MMS component that runs on the management hub that users or devops processes use to load models/files into MMS. The Edge Sync Service (ESS) runs on each edge node and is the API that edge services interact with to get the models/files and find out about updates.
//
// The Agent Secrets APIs enables service containers to receive updated secrets
//
//	schemes: http, https
//	host: localhost
//	basePath: /
//	version: 1.0.0
//
//	consumes:
//	- application/json
//
//	produces:
//	- application/json
//
// swagger:meta
package resource

import (
	"bytes"
	"encoding/base64"
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
	getSecretsURL   = "/api/v1/secrets"
	secretsURL      = "/api/v1/secrets/"
	contentType     = "Content-Type"
	applicationJSON = "application/json"
)

var unauthorizedBytes = []byte("Unauthorized")

type SecretAPI struct {
	db            *bolt.DB
	authenticator *SecretsAPIAuthenticate
}

// secretObject includes the secret key and secret value.
// swagger:model
type secretObject struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func NewSecretAPI(db *bolt.DB, am *AuthenticationManager) *SecretAPI {
	auth := &SecretsAPIAuthenticate{
		AuthMgr: am,
	}
	return &SecretAPI{
		db:            db,
		authenticator: auth,
	}
}

func (api *SecretAPI) SetupHttpHandler() {
	// curl -X GET https://localhost/api/v1/secrets --cacert /ess-cert/cert.pem --unix-socket /var/run/horizon/essapi.sock
	http.Handle(getSecretsURL, http.StripPrefix(getSecretsURL, http.HandlerFunc(api.handleGetSecrets)))
	http.Handle(secretsURL, http.StripPrefix(secretsURL, http.HandlerFunc(api.handleSecrets)))
}

func (api *SecretAPI) SetupAuthenticator(auth *SecretsAPIAuthenticate) {
	api.authenticator = auth
}

// swagger:operation GET /api/v1/secrets handleGetSecrets
//
// Get secrets.
//
// Get the list of updated secrets.
//
// ---
//
// tags:
// - Secrets
//
// produces:
// - application/json
// - text/plain
//
// parameters:
//
// responses:
//
//	'200':
//	  description: Secrets response
//	  schema:
//	    type: array
//	    items:
//	      type: string
//	'404':
//	  description: No updated secrets found
//	  schema:
//	    type: string
//	'500':
//	  description: Failed to retrieve the secret names of updated secrets
//	  schema:
//	    type: string
func (api *SecretAPI) handleGetSecrets(writer http.ResponseWriter, request *http.Request) {
	glog.V(3).Infof(secAPILogString(fmt.Sprintf("GET /api/v1/secrets")))

	// GET /secrets
	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// hzn dev returns 404
	if api.isDevEnv() {
		writer.WriteHeader(http.StatusNotFound)
		return
	}

	if authenticated, token, err := api.authenticator.Authenticate(request); !authenticated {
		glog.Errorf(secAPILogString(fmt.Sprintf("GET /api/v1/secrets authenticate error: %v", err)))
		writer.WriteHeader(http.StatusForbidden)
		writer.Write(unauthorizedBytes)
		return
	} else if mssInst, err := persistence.FindMSSInstWithESSToken(api.db, token); err != nil {
		message := fmt.Sprintf("Failed to fetch the microserviceservice secret status instance by token.")
		returnErrorResponse(writer, err, message, http.StatusInternalServerError)
	} else if updatedSecretNames, err := persistence.FindUpdatedSecretsForMSSInstance(api.db, mssInst.GetKey()); err != nil {
		message := fmt.Sprintf("Failed to fetch the updated secret names for microserviceservice secret status instance %v.", mssInst.GetKey())
		returnErrorResponse(writer, err, message, http.StatusInternalServerError)
	} else if len(updatedSecretNames) == 0 {
		writer.WriteHeader(http.StatusNotFound)
		return
	} else if data, err := json.MarshalIndent(updatedSecretNames, "", "  "); err != nil {
		message := fmt.Sprintf("Failed to marshal the list of secret names.")
		returnErrorResponse(writer, err, message, http.StatusInternalServerError)
	} else {
		writer.Header().Add(contentType, applicationJSON)
		writer.WriteHeader(http.StatusOK)
		if _, err := writer.Write(data); err != nil {
			glog.Errorf(secAPILogString(fmt.Sprintf("GET /api/v1/secrets, failed to write to response body: %v", err)))
		}
	}
}

func (api *SecretAPI) handleSecrets(writer http.ResponseWriter, request *http.Request) {
	glog.V(3).Infof(secAPILogString(fmt.Sprintf("In handleSecrets.")))
	// hzn dev env returns 404 for GET and POST, returns 405 for other HTTP method
	if api.isDevEnv() {
		if request.Method != http.MethodPost && request.Method != http.MethodGet {
			writer.WriteHeader(http.StatusMethodNotAllowed)
		} else {
			writer.WriteHeader(http.StatusNotFound)
		}
		return
	}

	// authenticate and extract service identity (serviceOrg, serviceName, serviceVersion)
	authenticated, token, err := api.authenticator.Authenticate(request)
	if !authenticated {
		glog.Errorf(secAPILogString(fmt.Sprintf("handleSecrets authenticate error: %v", err)))
		writer.WriteHeader(http.StatusForbidden)
		writer.Write(unauthorizedBytes)
		return
	}

	// MSS instance is created in container.go (ResourcesCreate func)
	mssInst, err := persistence.FindMSSInstWithESSToken(api.db, token)
	if err != nil || mssInst == nil {
		message := fmt.Sprintf("Failed to fetch the service instance.")
		returnErrorResponse(writer, err, message, http.StatusInternalServerError)
	}
	glog.V(5).Infof(secAPILogString(fmt.Sprintf("Find mssInst: %s", mssInst.String())))

	// parts is the content after /secrets/. But for this case: secretName == "" (request.URL.Path == ""), len(parts) == 1.
	// len(parts) always >= 1
	parts := strings.Split(request.URL.Path, "/")
	if len(parts) == 0 {
		// GET /secrets
		api.handleGetSecrets(writer, request)
		return

	} else if len(parts) == 1 {
		secretName := parts[0]
		if secretName == "" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		switch request.Method {

		// swagger:operation POST /api/v1/secrets/{secretName}?received=true handleSecretReceived
		//
		// Mark a secret as received.
		//
		// Mark the secret as having been received by the service.
		// After the secret is marked as received, it will no longer be returned by GET /secrets again.
		//
		// ---
		//
		// tags:
		// - Secrets
		//
		// produces:
		// - text/plain
		//
		// parameters:
		// - name: secretName
		//   in: path
		//   description: The secret name of the secret to mark as received
		//   required: true
		//   type: string
		//
		// responses:
		//   '201':
		//     description: Secret marked as received
		//     schema:
		//       type: string
		//   '400':
		//     description: Secret name not specified
		//     schema:
		//       type: string
		//   '404':
		//     description: Secret not found
		//     schema:
		//       type: string
		//   '500':
		//     description: Failed to mark the object consumed
		//     schema:
		//       type: string
		case http.MethodPost:
			//POST /secrets/<secret-name>?received=true
			receivedString := request.URL.Query().Get("received")
			received := false
			if receivedString != "" {
				var err error
				if received, err = strconv.ParseBool(receivedString); err != nil || !received {
					glog.Errorf(secAPILogString(fmt.Sprintf("POST /api/v1/secrets/%s?received=%t, err: %v", secretName, received, err)))
					writer.WriteHeader(http.StatusBadRequest)
					return
				} else {
					// received == true
					glog.V(3).Infof(secAPILogString(fmt.Sprintf("POST /api/v1/secrets/%s?received=true", secretName)))
					// call persistent to find secret
					if psecret, err := persistence.FindSingleSecretForService(api.db, secretName, mssInst.GetKey()); err != nil {
						returnErrorResponse(writer, err, "Failed to find secret.", http.StatusInternalServerError)
					} else if psecret == nil || isEmptySecretObject(*psecret) {
						returnErrorResponse(writer, err, "Secret not found.", http.StatusNotFound)
					} else {
						secStatus := persistence.NewSecretStatus(secretName, psecret.TimeLastUpdated)
						savedMSSInst, err := persistence.SaveSecretStatus(api.db, mssInst.GetKey(), secStatus)
						if err != nil {
							returnErrorResponse(writer, err, "Failed to update secret status.", http.StatusInternalServerError)
						}
						glog.V(3).Infof(secAPILogString(fmt.Sprintf("MSS secret %v is received for MSS Inst %v", secretName, savedMSSInst.String())))
						writer.WriteHeader(http.StatusCreated)
						return
					}
				}
			}

		// swagger:operation GET /api/v1/secrets/{secretName} handleGetSecret
		//
		// Get a secret.
		//
		// Get the details of a secret.
		//
		// ---
		//
		// tags:
		// - Secrets
		//
		// produces:
		// - application/json
		// - text/plain
		//
		// parameters:
		// - name: secretName
		//   in: path
		//   description: The secret name of the secret object to return
		//   required: true
		//   type: string
		//
		// responses:
		//   '200':
		//     description: Secret response
		//     schema:
		//       "$ref": "#/definitions/secretObject"
		//   '400':
		//     description: Secret name not specified
		//     schema:
		//       type: string
		//   '404':
		//     description: Secret not found
		//     schema:
		//       type: string
		//   '500':
		//     description: Failed to retrieve the secret
		//     schema:
		//       type: string
		case http.MethodGet:
			//GET /secrets/<secret-name>
			glog.V(3).Infof(secAPILogString(fmt.Sprintf("GET /api/v1/secrets/%s", secretName)))

			// call persistent to get secret object (key: serviceName/secretName)
			// get secret value: secret.SvcSecretValue
			if psecret, err := persistence.FindSingleSecretForService(api.db, secretName, mssInst.GetKey()); err != nil {
				returnErrorResponse(writer, err, "Failed to find secret.", http.StatusInternalServerError)
			} else if psecret == nil || isEmptySecretObject(*psecret) {
				returnErrorResponse(writer, err, "Secret not found.", http.StatusNotFound)
			} else {
				var sobj secretObject
				if dbyte, err := base64.StdEncoding.DecodeString(psecret.SvcSecretValue); err != nil {
					returnErrorResponse(writer, err, "Failed to decode the secret details.", http.StatusInternalServerError)
				} else if err := json.Unmarshal(dbyte, &sobj); err != nil {
					returnErrorResponse(writer, err, "Failed to unmarshal the secret byte to object.", http.StatusInternalServerError)
				} else if data, err := json.MarshalIndent(sobj, "", "  "); err != nil {
					returnErrorResponse(writer, err, "Failed to marshal the secret.", http.StatusInternalServerError)
				} else {
					writer.Header().Add(contentType, applicationJSON)
					writer.WriteHeader(http.StatusOK)
					if _, err := writer.Write(data); err != nil {
						glog.Errorf(secLogString(fmt.Sprintf("GET /api/v1/secrets/<secretName>, failed to write to response body: %v", err)))
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

func (api *SecretAPI) isDevEnv() bool {
	if api.authenticator.AuthMgr == nil && api.db == nil {
		return true
	}
	return false
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
		psecret.SvcSecretValue == "" {
		return true
	}
	return false
}

// Logging function
var secAPILogString = func(v interface{}) string {
	return fmt.Sprintf("Secrets API: %v", v)
}
