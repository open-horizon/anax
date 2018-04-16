package wiotp

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"net/http"
)

// WIoTP API Reference: https://docs.internetofthings.ibmcloud.com/apis/swagger/v0002/org-admin.html

// We don't need to know specifically about the rest of the attributes, so don't include them.
type WiotpType struct {
	Id string `json:"id"`
}

type WiotpTypes struct {
	Results []WiotpType `json:"results"`
}

// OrgList returns info about their wiotp org.
func OrgList(org, apiKeyTok string) {
	// Display the full resources
	var output string
	httpCode := cliutils.WiotpGet(cliutils.GetWiotpUrl(org), "", apiKeyTok, []int{200, 404}, &output)
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, "org '%s' not found", org)
	}
	fmt.Println(output)
}

// Status returns wiotp service status.
func Status(org, apiKeyTok string) {
	// Display the full resources
	var output string
	cliutils.WiotpGet(cliutils.GetWiotpUrl(org), "service-status", apiKeyTok, []int{200}, &output)
	fmt.Println(output)
}

// TypeList returns a list of the type names, or if wType is specified, the details of that type.
func TypeList(org, apiKeyTok, wType string) {
	if wType == "" {
		// Only display the names
		var resp WiotpTypes
		cliutils.WiotpGet(cliutils.GetWiotpUrl(org), "device/types"+cliutils.AddSlash(wType), apiKeyTok, []int{200, 404}, &resp)
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
		httpCode := cliutils.WiotpGet(cliutils.GetWiotpUrl(org), "device/types"+cliutils.AddSlash(wType), apiKeyTok, []int{200, 404}, &output)
		if httpCode == 404 && wType != "" {
			cliutils.Fatal(cliutils.NOT_FOUND, "type '%s' not found in org %s", wType, org)
		}
		fmt.Println(output)
	}
}

type WiotpService struct {
	Id string `json:"id"`
}

type WiotpTypeEdgeConf struct {
	Enabled      bool           `json:"enabled"`
	Architecture string         `json:"architecture"`
	EdgeServices []WiotpService `json:"edgeServices"`
}

type WiotpTypePost struct {
	Id                string            `json:"id"`
	ClassId           string            `json:"classId"`
	EdgeConfiguration WiotpTypeEdgeConf `json:"edgeConfiguration"`
}

func TypeCreate(org, apiKeyTok, wType, arch string, services []string) {
	// Transform the arch value
	switch arch {
	case "AMD64", "ARM32", "ARM64": // these are the valid values, so don't need to do anything
	case "amd64": // these are the horizon values
		arch = "AMD64"
	case "arm":
		arch = "ARM32"
	case "arm64":
		arch = "ARM64"
	}

	// Fill in the services array
	svcs := []WiotpService{}
	for _, s := range services {
		svcs = append(svcs, WiotpService{Id: s})
	}

	devType := WiotpTypePost{Id: wType, ClassId: "Gateway", EdgeConfiguration: WiotpTypeEdgeConf{Enabled: true, Architecture: arch, EdgeServices: svcs}}
	cliutils.WiotpPutPost(http.MethodPost, cliutils.GetWiotpUrl(org), "device/types", apiKeyTok, []int{201}, &devType)
	fmt.Printf("Device type %s created\n", wType)
}

func TypeRemove(org, apiKeyTok, wType string) {
	cliutils.WiotpDelete(cliutils.GetWiotpUrl(org), "device/types/"+wType, apiKeyTok, []int{204})
	fmt.Printf("Device type %s removed\n", wType)
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
	if device == "" {
		// Only display the names
		var resp WiotpDevices
		cliutils.WiotpGet(cliutils.GetWiotpUrl(org), "device/types/"+wType+"/devices"+cliutils.AddSlash(device), apiKeyTok, []int{200, 404}, &resp)
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
		httpCode := cliutils.WiotpGet(cliutils.GetWiotpUrl(org), "device/types/"+wType+"/devices"+cliutils.AddSlash(device), apiKeyTok, []int{200, 404}, &output)
		if httpCode == 404 && device != "" {
			cliutils.Fatal(cliutils.NOT_FOUND, "device '%s' of type '%s' not found in org %s", device, wType, org)
		}
		fmt.Println(output)
	}
}

func DeviceEdgeStatus(org, apiKeyTok, wType, device string) {
	// Display the full resources
	var output string
	httpCode := cliutils.WiotpGet(cliutils.GetWiotpUrl(org), "device/types/"+wType+"/devices/"+device+"/edgestatus", apiKeyTok, []int{200, 404}, &output)
	if httpCode == 404 && device != "" {
		cliutils.Fatal(cliutils.NOT_FOUND, "device '%s' of type '%s' not found in org %s", device, wType, org)
	}
	fmt.Println(output)
}

/*
type WiotpDeviceInfo struct {
	Description string `json:"description"`
}
*/

type WiotpDevicePost struct {
	DeviceId  string `json:"deviceId"`
	AuthToken string `json:"authToken"`
	//DeviceInfo WiotpDeviceInfo `json:"deviceInfo"`
	//Metadata map[string]string `json:"metadata"`
}

func DeviceCreate(org, apiKeyTok, wType, device, token string) {
	//dev := WiotpDevicePost{DeviceId: device, AuthToken: token, DeviceInfo: WiotpDeviceInfo{Description: "foo"}}
	dev := WiotpDevicePost{DeviceId: device, AuthToken: token}
	cliutils.WiotpPutPost(http.MethodPost, cliutils.GetWiotpUrl(org), "device/types/"+wType+"/devices", apiKeyTok, []int{201}, &dev)
	fmt.Printf("Device %s created\n", device)
}

func DeviceRemove(org, apiKeyTok, wType, device string) {
	cliutils.WiotpDelete(cliutils.GetWiotpUrl(org), "device/types/"+wType+"/devices/"+device, apiKeyTok, []int{204})
	fmt.Printf("Device %s removed\n", device)
}
