package agreementbot

import (
    "bytes"
    "encoding/json"
    "errors"
    "fmt"
    "github.com/golang/glog"
    "github.com/open-horizon/anax/config"
    "net/http"
    "os"
    "time"
)


// Following test data was given for JSON schema
// const testActiveData = `[{"id":"1","lat":41.0064,"lon":-111.9393,"contracts":[{"id":"0x0abcde","type":1,"ts":1447195061},{"id":"0x0ddddd","type":2,"ts":1447195061}]},`+
//                          `{"id":"2","lat":-41.0064,"lon":111.9393,"contracts":[{"id":"0x0deadbeef","type":1,"ts":1447195061},{"id":"0x0beefdead","type":2,"ts":1447195061}]}]`

type AgreementEntry struct {
    Id   string `json:"id"`
    Type int    `json:"type"`
    Ts   uint64 `json:"ts"`
}

type DeviceEntry struct {
    Id        string          `json:"id"`
    Lat       float64         `json:"lat"`
    Lon       float64         `json:"lon"`
    Agreements []AgreementEntry `json:"contracts"`
}

func GetActiveAgreements(in_devices map[string][]string, agreement Agreement, config *config.AGConfig) ([]string, error) {
    err := error(nil)

    // If the agreement record was created with the ActiveContractsURL field, then it means that the policy which created the
    // agreement specified a specific data verification URL. If not, then the default data verification URL is used from
    // the config.

    // This field is true when data verification is explicitly turned off in agreement's policy file.
    if agreement.DisableDataVerificationChecks == true {
        res := make([]string, 0, 1)
        return res, err
    }

    activeAgreementsURL := agreement.DataVerificationURL
    if activeAgreementsURL == "" {
        activeAgreementsURL = config.ActiveAgreementsURL
    }

    activeAgreementsUser := agreement.DataVerificationUser
    if activeAgreementsUser == "" {
        activeAgreementsUser = config.ActiveAgreementsUser
    }

    activeAgreementsPW := agreement.DataVerificationPW
    if activeAgreementsPW == "" {
        activeAgreementsPW = config.ActiveAgreementsPW
    }

    if _, ok := in_devices[activeAgreementsURL]; !ok {
        devices := make([]string, 0, 10)
        response := make([]DeviceEntry, 0, 10)

        // Assume the REST API is unreliable. If it fails, retry a few times before returning an
        // error to the caller.
        retries := 0
        for {
            if err = Invoke_rest("GET", activeAgreementsURL, activeAgreementsUser, activeAgreementsPW, nil, &response); err != nil {
                glog.Errorf("Error getting active agreements: %v", err)
                if retries == 2 {
                    break
                } else {
                    retries += 1
                    time.Sleep(1 * time.Second)
                }
            } else {
                glog.V(4).Infof("Active agreement response: %v", response)
                for _, dev := range response {
                    for _, con := range dev.Agreements {
                        devices = append(devices, con.Id)
                    }
                }
                glog.V(3).Infof("For URL %v Gathered agreements: %v", activeAgreementsURL, devices)
                err = error(nil)
                break
            }
        }
        in_devices[activeAgreementsURL] = devices
        return devices, err
    } else {
        return in_devices[activeAgreementsURL], err
    }
}

func ActiveAgreementsContains(activeAgreements []string, agreement Agreement, prefix string) bool {

    inttest_mode := os.Getenv("mtn_integration_test")
    if inttest_mode != "" || agreement.DisableDataVerificationChecks == true {
        return true
    }

    for _, dev := range activeAgreements {
        if dev == agreement.CurrentAgreementId || dev == prefix + agreement.CurrentAgreementId {
            return true
        }
    }

    return false
}

func Invoke_rest(method string, url string, user string, pw string, body []byte, outstruct interface{}) error {
    req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
    req.Header.Set("Content-Type", "application/json")
    if user != "" && pw != "" {
        req.SetBasicAuth(user, pw)
    }

    req.Close = true // work around to ensure that Go doesn't get connections confused. Supposed to be fixed in Go 1.6.

    client := &http.Client{Timeout: time.Duration(config.HTTPDEFAULTTIMEOUT*time.Millisecond)}
    rawresp, err := client.Do(req)
    if err != nil {
        return err
    } else if method == "GET" && rawresp.StatusCode != 200 {
        return errors.New(fmt.Sprintf("Error response from REST GET %v call: %v", url, rawresp.Status))
    } else if (method == "POST" || method == "PUT") && rawresp.StatusCode != 201 {
        return errors.New(fmt.Sprintf("Error response from REST %v %v call: %v", method, url, rawresp.Status))
    }

    defer rawresp.Body.Close()
    // glog.Infof("Raw Response: %v", rawresp.Body)

    if outstruct != nil {
        if err = json.NewDecoder(rawresp.Body).Decode(&outstruct); err != nil {
            glog.Errorf("Error decoding http response: %v", err)
        } else {
            // glog.Infof("Decoded Response: %v", outstruct)
        }
    }

    return err
}

