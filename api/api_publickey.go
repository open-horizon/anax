package api

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
)

func (a *API) publickey(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case "GET":

		pathVars := mux.Vars(r)
		fileName := pathVars["filename"]

		// Get a list of all valid public key PEM files in the configured location
		pubKeyDir := a.Config.UserPublicKeyPath()
		files, err := getPemFiles(pubKeyDir)
		if err != nil {
			glog.Errorf(apiLogString(fmt.Sprintf("APIWorker %v /publickey unable to read public key directory %v, error: %v", r.Method, pubKeyDir, err)))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}

		if fileName != "" {

			// If the input file name is not in the list of valid pem files, then return an error
			found := false
			for _, f := range files {
				if f.Name() == fileName {
					found = true
				}
			}
			if !found {
				glog.Errorf(apiLogString(fmt.Sprintf("APIWorker %v /publickey unable to find input file %v", r.Method, fileName)))
				w.WriteHeader(http.StatusNotFound)
				return
			}

			// Open the file so that we can read any header info that might be there.
			pemFile, err := os.Open(pubKeyDir + "/" + fileName)
			defer pemFile.Close()

			if err != nil {
				glog.Errorf(apiLogString(fmt.Sprintf("APIWorker %v /publickey unable to open requested key file %v, error: %v", r.Method, fileName, err)))
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			// Get the Content-Type of the file.
			fileHeader := make([]byte, 512)
			pemFile.Read(fileHeader)
			fileContentType := http.DetectContentType(fileHeader)

			// Get the file size.
			fileStat, _ := pemFile.Stat()
			fileSize := strconv.FormatInt(fileStat.Size(), 10)

			// Set the headers for a file atachment.
			w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
			w.Header().Set("Content-Type", fileContentType)
			w.Header().Set("Content-Length", fileSize)

			// Reset the file so that we can read from the beginning again.
			pemFile.Seek(0, 0)
			io.Copy(w, pemFile)
			w.WriteHeader(http.StatusOK)
			return

		} else {
			files, err := getPemFiles(pubKeyDir)
			if err != nil {
				glog.Errorf(apiLogString(fmt.Sprintf("APIWorker %v /publickey unable to read public key directory %v, error: %v", r.Method, pubKeyDir, err)))
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}

			response := make(map[string][]string)
			response["pem"] = make([]string, 0, 10)
			for _, pf := range files {
				response["pem"] = append(response["pem"], pf.Name())
			}

			serial, err := json.Marshal(response)
			if err != nil {
				glog.Errorf(apiLogString(fmt.Sprintf("APIWorker %v /publickey unable to serialize response %v, error %v", r.Method, response, err)))
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write(serial); err != nil {
				glog.Errorf(apiLogString(fmt.Sprintf("APIWorker %v /publickey error writing response: %v, error %v", r.Method, serial, err)))
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)

		}

	case "PUT":

		pathVars := mux.Vars(r)
		fileName := pathVars["filename"]

		if fileName == "" {
			writeInputErr(w, http.StatusBadRequest, NewAPIUserInputError("no filename specified", "public key file"))
			return
		} else if !strings.HasSuffix(fileName, ".pem") {
			writeInputErr(w, http.StatusBadRequest, NewAPIUserInputError("filename must have .pem suffix", "public key file"))
			return
		}

		glog.V(3).Infof(apiLogString(fmt.Sprintf("APIWorker %v /publickey of %v", r.Method, fileName)))
		targetPath := a.Config.UserPublicKeyPath()
		targetFile := targetPath + "/" + fileName

		// Receive the uploaded file content and verify that it is a valid public key. If it's valid then
		// save it into the configured PublicKeyPath location from the config. The name of the uploaded file
		// is specified on the HTTP PUT. It does not have to have the same file name used by the HTTP caller.

		if nkBytes, err := ioutil.ReadAll(r.Body); err != nil {
			glog.Errorf(apiLogString(fmt.Sprintf("APIWorker %v /publickey unable to read uploaded public key file, error: %v", r.Method, err)))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		} else if nkBlock, _ := pem.Decode(nkBytes); nkBlock == nil {
			writeInputErr(w, http.StatusBadRequest, NewAPIUserInputError("not a pem encoded file", "public key file"))
			return
		} else if _, err := x509.ParsePKIXPublicKey(nkBlock.Bytes); err != nil {
			writeInputErr(w, http.StatusBadRequest, NewAPIUserInputError("not a PKIX public key", "public key file"))
			return
		} else if err := os.MkdirAll(targetPath, 0644); err != nil {
			glog.Errorf(apiLogString(fmt.Sprintf("APIWorker %v /publickey unable to create user key directory, error %v", r.Method, err)))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		} else if err := ioutil.WriteFile(targetFile, nkBytes, 0644); err != nil {
			glog.Errorf(apiLogString(fmt.Sprintf("APIWorker %v /publickey unable to write uploaded public key file %v, error: %v", r.Method, targetFile, err)))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		} else {
			glog.V(5).Infof(apiLogString(fmt.Sprintf("APIWorker %v /publickey successfully uploaded and verified public key in %v", r.Method, targetFile)))
			w.WriteHeader(http.StatusOK)
		}

	case "DELETE":

		pathVars := mux.Vars(r)
		fileName := pathVars["filename"]

		if fileName == "" {
			writeInputErr(w, http.StatusBadRequest, NewAPIUserInputError("no filename specified", "public key file"))
			return
		}
		glog.V(3).Infof(apiLogString(fmt.Sprintf("APIWorker %v /publickey of %v", r.Method, fileName)))

		// Get a list of all valid public key PEM files in the configured location
		pubKeyDir := a.Config.UserPublicKeyPath()
		files, err := getPemFiles(pubKeyDir)
		if err != nil {
			glog.Errorf(apiLogString(fmt.Sprintf("APIWorker %v /publickey unable to read public key directory %v, error: %v", r.Method, pubKeyDir, err)))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}

		// If the input file name is not in the list of valid pem files, then return an error
		found := false
		for _, f := range files {
			if f.Name() == fileName {
				found = true
			}
		}
		if !found {
			glog.Errorf(apiLogString(fmt.Sprintf("APIWorker %v /publickey unable to find input file %v", r.Method, fileName)))
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// The input file is a valid public key, remove it
		err = os.Remove(pubKeyDir + "/" + fileName)
		if err != nil {
			glog.Errorf(apiLogString(fmt.Sprintf("APIWorker %v /publickey unable to delete public key file %v, error: %v", r.Method, fileName, err)))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		w.WriteHeader(http.StatusNoContent)
		return

	case "OPTIONS":
		w.Header().Set("Allow", "GET, PUT, DELETE, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}

}

func getPemFiles(homePath string) ([]os.FileInfo, error) {

	res := make([]os.FileInfo, 0, 10)

	if files, err := ioutil.ReadDir(homePath); err != nil && !os.IsNotExist(err) {
		return res, errors.New(fmt.Sprintf("Unable to get list of PEM files in %v, error: %v", homePath, err))
	} else if os.IsNotExist(err) {
		return res, nil
	} else {
		for _, fileInfo := range files {
			if strings.HasSuffix(fileInfo.Name(), ".pem") && !fileInfo.IsDir() {
				fName := homePath + "/" + fileInfo.Name()
				if pubKeyData, err := ioutil.ReadFile(fName); err != nil {
					continue
				} else if block, _ := pem.Decode(pubKeyData); block == nil {
					continue
				} else if _, err := x509.ParsePKIXPublicKey(block.Bytes); err != nil {
					continue
				} else {
					res = append(res, fileInfo)
				}
			}
		}
		return res, nil
	}
}
