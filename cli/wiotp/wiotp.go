package wiotp

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"strings"
)

// WIoTP API Reference: https://console.bluemix.net/docs/services/IoT/reference/api.html

// We don't need to know specifically about the rest of the attributes, so don't include them.
type WiotpType struct {
	Id string `json:"id"`
}

type WiotpTypes struct {
	Results []WiotpType `json:"results"`
}

// TypeList returns a list of the type names, or if wType is specified, the details of that type.
func TypeList(org, apiKeyTok, wType string) {
	if wType != "" {
		wType = "/" + wType
	}
	if wType == "" {
		// Only display the names
		var resp WiotpTypes
		cliutils.WiotpGet(cliutils.GetWiotpUrl(org), "device/types"+wType, apiKeyTok, []int{200, 404}, &resp)
		wTypes := []string{}
		for _, t := range resp.Results {
			wTypes = append(wTypes, t.Id)
		}
		jsonBytes, err := json.MarshalIndent(wTypes, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'wiotp type list' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		// Display the full resources
		var output string
		httpCode := cliutils.WiotpGet(cliutils.GetWiotpUrl(org), "device/types"+wType, apiKeyTok, []int{200, 404}, &output)
		if httpCode == 404 && wType != "" {
			cliutils.Fatal(cliutils.NOT_FOUND, "type '%s' not found in org %s", strings.TrimPrefix(wType, "/"), org)
		}
		fmt.Println(output)
	}
}

// We don't need to know specifically about the rest of the attributes, so don't include them.
type WiotpDevice struct {
	DeviceId string `json:"deviceId"`
	TypeId   string `json:"typeId"`
}

type WiotpDevices struct {
	Results []WiotpDevice `json:"results"`
}

// DeviceList returns a list of the device/gateway names of this type, or if device is specified, the details of that device/gateway.
func DeviceList(org, apiKeyTok, wType, device string) {
	// Note: wType is a required arg
	if device != "" {
		device = "/" + device
	}
	if device == "" {
		// Only display the names
		var resp WiotpDevices
		cliutils.WiotpGet(cliutils.GetWiotpUrl(org), "device/types/"+wType+"/devices"+device, apiKeyTok, []int{200, 404}, &resp)
		devices := []string{}
		for _, t := range resp.Results {
			devices = append(devices, t.DeviceId)
		}
		jsonBytes, err := json.MarshalIndent(devices, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'wiotp device list' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		// Display the full resources
		var output string
		httpCode := cliutils.WiotpGet(cliutils.GetWiotpUrl(org), "device/types/"+wType+"/devices"+device, apiKeyTok, []int{200, 404}, &output)
		if httpCode == 404 && device != "" {
			cliutils.Fatal(cliutils.NOT_FOUND, "device '%s' of type '%s' not found in org %s", strings.TrimPrefix(device, "/"), wType, org)
		}
		fmt.Println(output)
	}
}
