package governance

import (
	"encoding/json"
	"fmt"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/container"
	"github.com/open-horizon/anax/containermessage"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangesync"
	"github.com/open-horizon/anax/helm"
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

type WorkloadStatus struct {
	AgreementId string            `json:"agreementId"`
	ServiceURL  string            `json:"serviceUrl,omitempty"`
	Org         string            `json:"orgid,omitempty"`
	Version     string            `json:"version,omitempty"`
	Arch        string            `json:"arch,omitempty"`
	Containers  []ContainerStatus `json:"containerStatus"`
}

func (w WorkloadStatus) String() string {
	return fmt.Sprintf("AgreementId: %v, "+
		"ServiceURL: %v, "+
		"Org: %v, "+
		"Version: %v, "+
		"Arch: %v, "+
		"Containers: %v",
		w.AgreementId, w.ServiceURL, w.Org, w.Version, w.Arch, w.Containers)
}

type DeviceStatus struct {
	Connectivity map[string]bool  `json:"connectivity"` //  hosts and whether this device can reach them or not
	Services     []WorkloadStatus `json:"services"`
	LastUpdated  string           `json:"lastUpdated,omitempty"`
}

func (w DeviceStatus) String() string {
	return fmt.Sprintf(
		"Connectivity: %v, "+
			"Services: %v,"+
			"LastUpdated: %v",
		w.Connectivity, w.Services, w.LastUpdated)
}

func NewDeviceStatus() *DeviceStatus {
	return &DeviceStatus{}
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

	// get service status
	if ms_status, err := w.getServiceStatus(containers); err != nil {
		glog.Errorf(logString(fmt.Sprintf("Error getting service container status: %v", err)))
	} else {
		device_status.Services = ms_status
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

// Find the status for all the Services.
func (w *GovernanceWorker) getServiceStatus(containers []docker.APIContainers) ([]WorkloadStatus, error) {

	// Get all top level (agreement) services and related metadata.
	tempWS, err := w.getWorkloadStatus(containers)
	if err != nil {
		return nil, fmt.Errorf(logString(fmt.Sprintf("Error retrieving agreement services from database, error: %v", err)))
	}

	// Get all dependent services and related metadata.
	tempMS, err := w.getMicroserviceStatus(containers)
	if err != nil {
		return nil, fmt.Errorf(logString(fmt.Sprintf("Error retrieving services from database, error: %v", err)))
	}

	for _, ms := range tempMS {
		// In the services model, there will be duplicates in the microservice list and the workload list. Skip the duplicates
		// from the microservice list, and prefer the service from the workload list.
		duplicate := false
		for _, ws := range tempWS {
			if ws.ServiceURL == ms.ServiceURL {
				duplicate = true
				break
			}
		}

		if !duplicate {
			tempWS = append(tempWS, ms)
		}
	}

	return tempWS, nil
}

// Find the status for all the microservices
func (w *GovernanceWorker) getMicroserviceStatus(containers []docker.APIContainers) ([]WorkloadStatus, error) {

	// Filter to return all instances for a msdef
	msdefFilter := func(msdef_id string) persistence.MIFilter {
		return func(a persistence.MicroserviceInstance) bool {
			return a.MicroserviceDefId == msdef_id
		}
	}

	status := make([]WorkloadStatus, 0)

	if msdefs, err := persistence.FindMicroserviceDefs(w.db, []persistence.MSFilter{persistence.UnarchivedMSFilter()}); err != nil {
		return nil, fmt.Errorf(logString(fmt.Sprintf("Error retrieving all service definitions from database, error: %v", err)))
	} else if msdefs != nil {
		for _, msdef := range msdefs {
			var msdef_status WorkloadStatus
			msdef_status.ServiceURL = msdef.SpecRef
			msdef_status.Org = msdef.Org
			msdef_status.Version = msdef.Version
			msdef_status.Arch = msdef.Arch
			msdef_status.Containers = make([]ContainerStatus, 0)
			if msinsts, err := persistence.FindMicroserviceInstances(w.db, []persistence.MIFilter{persistence.UnarchivedMIFilter(), msdefFilter(msdef.Id)}); err != nil {
				return nil, fmt.Errorf(logString(fmt.Sprintf("Error retrieving all service instances for %v from database, error: %v", msdef.SpecRef, err)))
			} else if msinsts != nil {
				for _, msi := range msinsts {
					deployment, _ := msdef.GetDeployment()
					if deployment != "" {
						if cstatus, err := GetContainerStatus(deployment, msi.GetKey(), true, containers); err != nil {
							return nil, fmt.Errorf(logString(fmt.Sprintf("Error getting service container status for %v. %v", msdef.SpecRef, err)))
						} else {
							msdef_status.Containers = append(msdef_status.Containers, cstatus...)
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
						wl_status.ServiceURL = wl.WorkloadURL
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

	if deploymentDesc, err := containermessage.GetNativeDeployment(deployment); err == nil {
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
	} else if hdc, err := persistence.GetHelmDeployment(deployment); err == nil {
		var container_status ContainerStatus
		container_status.Name = fmt.Sprintf("Helm release: %v", hdc.ReleaseName)

		hc := helm.NewHelmClient()
		releaseState := "Not Running"
		if rs, err := hc.Status(hdc.ReleaseName); err != nil {
			releaseState = fmt.Sprintf("Unknown, error: %v", err)
		} else {
			releaseState = rs.Status
			cDate := cutil.TimeInSeconds(rs.Updated, hc.ReleaseTimeFormat())
			container_status.Created = cDate
			container_status.Image = rs.ChartName
		}
		container_status.State = releaseState
		status = append(status, container_status)
	} else {
		return nil, fmt.Errorf(logString(fmt.Sprintf("Error Unmarshalling deployment string %v. %v", deployment, err)))
	}
	return status, nil
}

// write to the exchange
func (w *GovernanceWorker) writeStatusToExchange(device_status *DeviceStatus) error {
	var resp interface{}
	resp = new(exchange.PostDeviceResponse)

	targetURL := w.Config.Edge.ExchangeURL + "orgs/" + exchange.GetOrg(w.GetExchangeId()) + "/nodes/" + exchange.GetId(w.GetExchangeId()) + "/status"

	httpClientFactory := w.limitedRetryEC.GetHTTPFactory()
	retryCount := httpClientFactory.RetryCount
	retryInterval := httpClientFactory.GetRetryInterval()

	for {
		if err, tpErr := exchange.InvokeExchange(httpClientFactory.NewHTTPClient(nil), "PUT", targetURL, w.GetExchangeId(), w.GetExchangeToken(), device_status, &resp); err != nil {
			glog.Errorf(logString(fmt.Sprintf(err.Error())))
			return err
		} else if tpErr != nil {
			glog.Warningf(tpErr.Error())
			if httpClientFactory.RetryCount == 0 {
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else if retryCount == 0 {
				return fmt.Errorf(logString(fmt.Sprintf("exceeded %v retries trying to write node status for %v", httpClientFactory.RetryCount, tpErr)))
			} else {
				retryCount--
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			}
		} else {
			glog.V(5).Infof(logString(fmt.Sprintf("saved device status to the exchange")))
			return nil
		}
	}
}

func (w *GovernanceWorker) surfaceErrors() int {
	pDevice, err := persistence.FindExchangeDevice(w.db)
	if err != nil {
		glog.V(3).Infof(logString(fmt.Sprintf("Error getting persistence device. %v", err)))
	}
	errorsHandler := exchange.GetHTTPSurfaceErrorsHandler(w.limitedRetryEC)
	putErrorsHandler := exchange.GetHTTPPutSurfaceErrorsHandler(w.limitedRetryEC)
	serviceResolverHandler := exchange.GetHTTPServiceResolverHandler(w.limitedRetryEC)
	return exchangesync.UpdateSurfaceErrors(w.db, *pDevice, errorsHandler, putErrorsHandler, serviceResolverHandler, w.BaseWorker.Manager.Config.Edge.SurfaceErrorTimeoutS, w.BaseWorker.Manager.Config.Edge.SurfaceErrorAgreementPersistentS)
}
