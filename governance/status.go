package governance

import (
	"encoding/json"
	"fmt"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/container"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"time"
)

var HORIZON_SERVERS = [...]string{"firmware.bluehorizon.network", "images.bluehorizon.network"}

type ContainerStatus struct {
	Name    string `json:"name"`
	Image   string `json:"image"`
	Created int64  `json:"created"`
	State   string `json:"state"`
}

func (w ContainerStatus) String() string {
	return fmt.Sprintf("Name: %v, "+
		"Image: %v, "+
		"Created: %v, "+
		"State: %v",
		w.Name, w.Image, w.Created, w.State)
}

type MicroserviceStatus struct {
	SpecRef    string            `json:"specRef"`
	Org        string            `json:"orgid"`
	Version    string            `json:"version"`
	Arch       string            `json:"arch"`
	Containers []ContainerStatus `json:"containerStatus"`
}

func (w MicroserviceStatus) String() string {
	return fmt.Sprintf("SpecRef: %v, "+
		"Org: %v, "+
		"Version: %v, "+
		"Arch: %v, "+
		"Containers: %v",
		w.SpecRef, w.Org, w.Version, w.Arch, w.Containers)
}

type WorkloadStatus struct {
	AgreementId string            `json:"agreementId"`
	WorkloadURL string            `json:"workloadUrl"`
	Org         string            `json:"orgid"`
	Version     string            `json:"version"`
	Arch        string            `json:"arch"`
	Containers  []ContainerStatus `json:"containerStatus"`
}

func (w WorkloadStatus) String() string {
	return fmt.Sprintf("AgreementId: %v, "+
		"WorkloadURL: %v, "+
		"Org: %v, "+
		"Version: %v, "+
		"Arch: %v, "+
		"Containers: %v",
		w.AgreementId, w.WorkloadURL, w.Org, w.Version, w.Arch, w.Containers)
}

type DeviceStatus struct {
	Connectivity  map[string]bool      `json:"connectivity"` //  hosts and whether this device can reach them or not
	Microservices []MicroserviceStatus `json:"microservices"`
	Workloads     []WorkloadStatus     `json:"workloads"`
	LastUpdated   string               `json:"lastUpdated"`
}

func (w DeviceStatus) String() string {
	return fmt.Sprintf(
		"Connectivity: %v, "+
			"Microservices: %v, "+
			"Workloads: %v,"+
			"LastUpdated: %v",
		w.Connectivity, w.Microservices, w.Workloads, w.LastUpdated)
}

func NewDeviceStatus() *DeviceStatus {
	var ds DeviceStatus
	ds.Connectivity = make(map[string]bool, 0)
	ds.Microservices = make([]MicroserviceStatus, 0)
	ds.Workloads = make([]WorkloadStatus, 0)
	return &ds
}

// Report the containers status and connectivity status to the exchange.
func (w *GovernanceWorker) ReportDeviceStatus() {

	if !w.Config.Edge.ReportDeviceStatus {
		glog.Info("ReportDeviceStatus is false. The status report to the exchange is turned off.")
		return
	}

	glog.Info("started the status report to the exchange.")

	w.deviceStatus = nil
	var device_status DeviceStatus

	// get connectivity
	connect := make(map[string]bool, 0)
	for _, host := range HORIZON_SERVERS {
		if err := cutil.CheckConnectivity(host); err != nil {
			glog.Errorf(logString(fmt.Sprintf("Error checking connectivity for %s: %v", host, err)))
			connect[host] = false
		} else {
			connect[host] = true
		}
	}
	device_status.Connectivity = connect

	// get docker containers
	containers := make([]docker.APIContainers, 0)
	if client, err := docker.NewClient(w.Config.Edge.DockerEndpoint); err != nil {
		glog.Errorf(logString(fmt.Sprintf("Failed to instantiate docker Client: %v", err)))
	} else {
		containers, err = client.ListContainers(docker.ListContainersOptions{})
		if err != nil {
			glog.Errorf(logString(fmt.Sprintf("Unable to get list of running containers: %v", err)))
		}
	}

	// get microservice status
	if ms_status, err := w.getMicroserviceStatus(containers); err != nil {
		glog.Errorf(logString(fmt.Sprintf("Error getting microservice container status: %v", err)))
	} else {
		device_status.Microservices = ms_status
	}

	// get workload status
	if wl_status, err := w.getWorkloadStatus(containers); err != nil {
		glog.Errorf(logString(fmt.Sprintf("Error getting microservice container status: %v", err)))
	} else {
		device_status.Workloads = wl_status
	}

	// report the status to the exchange
	if jbytes, err := json.Marshal(&device_status); err != nil {
		glog.V(5).Infof(logString(fmt.Sprintf("Failed to convert the device status %v to json: %v", device_status, err)))
	} else {

		w.deviceStatus = &device_status
		glog.V(5).Infof(logString(fmt.Sprintf("device status to report to the exchange: %v", string(jbytes[:]))))

		if err := w.writeStatusToExchange(&device_status); err != nil {
			glog.Errorf(logString(err))
		}
	}

}

// Find the status for all the microservices
func (w *GovernanceWorker) getMicroserviceStatus(containers []docker.APIContainers) ([]MicroserviceStatus, error) {

	// Filter to return all instances for a msdef
	msdefFilter := func(msdef_id string) persistence.MIFilter {
		return func(a persistence.MicroserviceInstance) bool {
			return a.MicroserviceDefId == msdef_id
		}
	}

	status := make([]MicroserviceStatus, 0)

	if msdefs, err := persistence.FindMicroserviceDefs(w.db, []persistence.MSFilter{persistence.UnarchivedMSFilter()}); err != nil {
		return nil, fmt.Errorf(logString(fmt.Sprintf("Error retrieving all microservice definitions from database, error: %v", err)))
	} else if msdefs != nil {
		for _, msdef := range msdefs {
			var msdef_status MicroserviceStatus
			msdef_status.SpecRef = msdef.SpecRef
			msdef_status.Org = msdef.Org
			msdef_status.Version = msdef.Version
			msdef_status.Arch = msdef.Arch
			msdef_status.Containers = make([]ContainerStatus, 0)
			if msinsts, err := persistence.FindMicroserviceInstances(w.db, []persistence.MIFilter{persistence.UnarchivedMIFilter(), msdefFilter(msdef.Id)}); err != nil {
				return nil, fmt.Errorf(logString(fmt.Sprintf("Error retrieving all microservice instances for %v from database, error: %v", msdef.SpecRef, err)))
			} else if msinsts != nil {
				for _, msi := range msinsts {
					if msdef.Workloads != nil && len(msdef.Workloads) > 0 {
						for _, wl := range msdef.Workloads {
							if cstatus, err := GetContainerStatus(wl.Deployment, msi.GetKey(), true, containers); err != nil {
								return nil, fmt.Errorf(logString(fmt.Sprintf("Error getting microservice container status for %v. %v", msdef.SpecRef, err)))
							} else {
								msdef_status.Containers = append(msdef_status.Containers, cstatus...)
							}
						}
					}
				}
			}
			status = append(status, msdef_status)
		}
	}

	return status, nil
}

// Find the status for all the workloads
func (w *GovernanceWorker) getWorkloadStatus(containers []docker.APIContainers) ([]WorkloadStatus, error) {

	status := make([]WorkloadStatus, 0)

	if establishedAgreements, err := persistence.FindEstablishedAgreementsAllProtocols(w.db, policy.AllAgreementProtocols(), []persistence.EAFilter{persistence.UnarchivedEAFilter()}); err != nil {
		return nil, fmt.Errorf(logString(fmt.Sprintf("Unable to retrieve not yet final agreements from database: %v. Error: %v", err, err)))
	} else {
		for _, ag := range establishedAgreements {
			if ag.AgreementTerminatedTime != 0 {
				continue // skip those terminated but not archived ones
			}

			if ag.Proposal != "" {
				protocolHandler := w.producerPH[ag.AgreementProtocol].AgreementProtocolHandler("", "", "")
				if proposal, err := protocolHandler.DemarshalProposal(ag.Proposal); err != nil {
					return nil, fmt.Errorf(logString(fmt.Sprintf("unable to demarshal proposal for agreement %v from database, error %v", ag.CurrentAgreementId, err)))
				} else if tcPolicy, err := policy.DemarshalPolicy(proposal.TsAndCs()); err != nil {
					return nil, fmt.Errorf(logString(fmt.Sprintf("error demarshalling TsAndCs policy for agreement %v, error %v", ag.CurrentAgreementId, err)))
				} else if tcPolicy.Workloads != nil {
					for _, wl := range tcPolicy.Workloads {
						var wl_status WorkloadStatus
						wl_status.AgreementId = ag.CurrentAgreementId
						wl_status.WorkloadURL = wl.WorkloadURL
						wl_status.Org = wl.Org
						wl_status.Version = wl.Version
						wl_status.Arch = wl.Arch

						if cstatus, err := GetContainerStatus(wl.Deployment, ag.CurrentAgreementId, false, containers); err != nil {
							return nil, fmt.Errorf(logString(fmt.Sprintf("Error finding workload status for %v. %v", ag, err)))
						} else {
							wl_status.Containers = append(wl_status.Containers, cstatus...)
						}
						status = append(status, wl_status)
					}
				}
			}
		}
	}

	return status, nil
}

// find container status
func GetContainerStatus(deployment string, key string, infrastructure bool, containers []docker.APIContainers) ([]ContainerStatus, error) {

	status := make([]ContainerStatus, 0)

	deploymentDesc := new(container.DeploymentDescription)
	if err := json.Unmarshal([]byte(deployment), &deploymentDesc); err != nil {
		return nil, fmt.Errorf(logString(fmt.Sprintf("Error Unmarshalling deployment string %v. %v", deployment, err)))
	} else {
		label := container.LABEL_PREFIX + ".agreement_id"
		if infrastructure {
			label = container.LABEL_PREFIX + ".infrastructure"
		}

		for serviceName, s_details := range deploymentDesc.Services {
			var container_status ContainerStatus
			container_status.Name = serviceName
			container_status.Image = s_details.Image
			container_status.State = "not started"
			for _, container := range containers {
				if _, ok := container.Labels[label]; ok {
					cname := container.Names[0]
					if cname == "/"+key+"-"+serviceName {
						container_status.Name = container.Names[0]
						container_status.Image = container.Image
						container_status.Created = container.Created
						container_status.State = container.State
						break
					}
				}
			}
			glog.Infof("container_status=%v", container_status)
			status = append(status, container_status)
		}
	}
	return status, nil
}

// write to the exchange
func (w *GovernanceWorker) writeStatusToExchange(device_status *DeviceStatus) error {
	var resp interface{}
	resp = new(exchange.PostDeviceResponse)

	httpClient := w.Config.Collaborators.HTTPClientFactory.NewHTTPClient(nil)
	targetURL := w.Config.Edge.ExchangeURL + "orgs/" + exchange.GetOrg(w.deviceId) + "/nodes/" + exchange.GetId(w.deviceId) + "/status"

	for {
		if err, tpErr := exchange.InvokeExchange(httpClient, "PUT", targetURL, w.deviceId, w.deviceToken, device_status, &resp); err != nil {
			glog.Errorf(logString(fmt.Sprintf(err.Error())))
			return err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			time.Sleep(10 * time.Second)
			continue
		} else {
			glog.V(5).Infof(logString(fmt.Sprintf("saved device status to the exchange")))
			return nil
		}
	}
}
