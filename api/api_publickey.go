package api

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"github.com/open-horizon/anax/cutil"
)

func (a *API) publickey(w http.ResponseWriter, r *http.Request) {

	resource := "publickey"

	errorHandler := GetHTTPErrorHandler(w)

	switch r.Method {
	case "GET":

		pathVars := mux.Vars(r)
		fileName := pathVars["filename"]
		verbose := r.FormValue("verbose")

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v/%v", r.Method, resource, fileName)))

		if fileName != "" {
			if fName, err := FindPublicKeyForOutput(fileName, a.Config); err != nil {
				errorHandler(NewNotFoundError(fmt.Sprintf("Error getting %v/%v for output, error %v", resource, fileName, err), "filename"))
			} else if err := returnFileBytes(fName, w); err != nil {
				errorHandler(NewSystemError(fmt.Sprintf("Error returning content of %v/%v, error %v", resource, fileName, err)))
			}
		} else {
			if out, err := FindPublicKeysForOutput(a.Config, verbose == "true"); err != nil {
				errorHandler(NewSystemError(fmt.Sprintf("Error getting %v for output, error %v", resource, err)))
			} else {
				writeResponse(w, out, http.StatusOK)
			}
		}

	case "PUT":

		pathVars := mux.Vars(r)
		fileName := pathVars["filename"]

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v/%v", r.Method, resource, fileName)))

		nkBytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			errorHandler(NewSystemError(fmt.Sprintf("Unable to read uploaded trusted cert file %v, error: %v", fileName, err)))
			return
		}

		errHandled := UploadPublicKey(fileName, nkBytes, a.Config, errorHandler)
		if errHandled {
			return
		}

		w.WriteHeader(http.StatusOK)

	case "DELETE":

		pathVars := mux.Vars(r)
		fileName := pathVars["filename"]

		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v/%v", r.Method, resource, fileName)))

		errHandled := DeletePublicKey(fileName, a.Config, errorHandler)
		if errHandled {
			return
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

func returnFileBytes(filename string, w http.ResponseWriter) error {
	// Open the file so that we can read any header info that might be there.
	file, err := os.Open(filename)
	if file != nil {
		defer cutil.CloseFileLogError(file)
	}

	if err != nil {
		return errors.New(fmt.Sprintf("unable to open requested key file %v, error: %v", filename, err))
	}

	// Get the Content-Type of the file.
	fileHeader := make([]byte, 512)
	if _, err := file.Read(fileHeader); err != nil {
		return err
	}
	fileContentType := http.DetectContentType(fileHeader)

	// Get the file size.
	fileStat, _ := file.Stat()
	fileSize := strconv.FormatInt(fileStat.Size(), 10)

	// Set the headers for a file attachment.
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	w.Header().Set("Content-Type", fileContentType)
	w.Header().Set("Content-Length", fileSize)

	// Reset the file so that we can read from the beginning again.
	if _, err := file.Seek(0, 0); err != nil {
		return err
	}
	if _, err := io.Copy(w, file); err != nil {
		return err
	}
	w.WriteHeader(http.StatusOK)
	return nil
}
