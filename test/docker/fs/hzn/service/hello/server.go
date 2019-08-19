package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

type CPU struct {
	Utilization float64 `json:"cpu"`
}

func hello(w http.ResponseWriter, r *http.Request) {
	httpClient := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, "http://amd64_cpu:8347/v1/cpu", nil)
	if err != nil {
		io.WriteString(w, fmt.Sprintf("Error creating HTTP request: %v", err))
		return
	}
	req.Header.Add("Accept", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		io.WriteString(w, fmt.Sprintf("Error executing HTTP request: %v", err))
		return
	}
	defer resp.Body.Close()
	httpCode := resp.StatusCode
	if httpCode != http.StatusOK {
		io.WriteString(w, fmt.Sprintf("Error received HTTP code %v", httpCode))
		return
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		io.WriteString(w, fmt.Sprintf("Error reading HTTP response: %v", err))
		return
	}

	if len(bodyBytes) == 0 {
		io.WriteString(w, fmt.Sprintf("Error, response body length is 0: %v", err))
		return
	}
	var cpu CPU
	if err := json.Unmarshal(bodyBytes, &cpu); err != nil {
		io.WriteString(w, fmt.Sprintf("Error, failed to unmarshal cpu API response body %s: %v", bodyBytes, err))
	}

	io.WriteString(w, fmt.Sprintf("Hello world, cpu utilization is: %v", cpu.Utilization))
}

func movie(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Star Wars: Rogue 1")
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", hello)
	mux.HandleFunc("/movie", movie)
	http.ListenAndServe(":8000", mux)
}
