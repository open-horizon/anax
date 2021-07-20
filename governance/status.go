package governance

import (
	"fmt"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/container"
	"github.com/open-horizon/anax/containermessage"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangesync"
	"github.com/open-horizon/anax/helm"
	"github.com/open-horizon/anax/kube_operator"
	"github.com/open-horizon/anax/persistence"
	"reflect"
	"time"
)

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
	AgreementId    string            `json:"agreementId"`
	ServiceURL     string            `json:"serviceUrl,omitempty"`
	Org            string            `json:"orgid,omitempty"`
	Version        string            `json:"version,omitempty"`
	Arch           string            `json:"arch,omitempty"`
	Containers     []ContainerStatus `json:"containerStatus"`
	OperatorStatus interface{}       `json:"operatorStatus,omitempty"`
	ConfigState    string            `json:"configState,omitempty"`
}

func (w WorkloadStatus) String() string {
	return fmt.Sprintf("AgreementId: %v, "+
		"ServiceURL: %v, "+
		"Org: %v, "+
		"Version: %v, "+
		"Arch: %v, "+
		"Containers: %v"+
		"OperatorStatus: %v"+
		"ConfigState: %v",
		w.AgreementId, w.ServiceURL, w.Org, w.Version, w.Arch, w.Containers, w.OperatorStatus, w.ConfigState)
}

type DeviceStatus struct {
	Connectivity map[string]bool  `json:"connectivity,omitempty"` //  hosts and whether this device can reach them or not
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
func (w *GovernanceWorker) ReportDeviceStatus() int {
	return w.reportDeviceStatus(nil)
}

func (w *GovernanceWorker) reportDeviceStatus(cfgStates []events.ServiceConfigState) int {
	if !w.Config.Edge.ReportDeviceStatus {
		glog.Info("ReportDeviceStatus is false. The status report to the exchange is turned off.")
		return 3600
	}

	// for 'device' node type, it has to have DockerEndpoint set
	if w.Config.Edge.DockerEndpoint == "" && w.deviceType == persistence.DEVICE_TYPE_DEVICE {
		glog.Infof("Skip reporting the status to the exchange because DockerEndpoint is not set in the configuration.")
		return 3600
	}

	glog.Info("started the status report to the exchange.")

	w.deviceStatus = nil
	var device_status DeviceStatus

	// get docker containers
	containers := make([]docker.APIContainers, 0)
	if w.deviceType == persistence.DEVICE_TYPE_DEVICE {
		if client, err := docker.NewClient(w.Config.Edge.DockerEndpoint); err != nil {
			glog.Errorf(logString(fmt.Sprintf("Failed to instantiate docker Client: %v", err)))
		} else {
			containers, err = client.ListContainers(docker.ListContainersOptions{})
			if err != nil {
				glog.Errorf(logString(fmt.Sprintf("Unable to get list of running containers: %v", err)))
			}
		}
	}

	// get service status
	if ms_status, err := w.getServiceStatus(containers); err != nil {
		glog.Errorf(logString(fmt.Sprintf("Error getting service container status: %v", err)))
	} else {
		device_status.Services = ms_status
	}

	// Add the old suspended ones
	statusChanged := true
	oldWlStatus, err := persistence.FindNodeStatus(w.db)
	if err != nil {
		glog.Errorf(logString(fmt.Sprintf("Failed to retrieve previous device status from local database: %v", err)))
	} else {
		nonChangedServices := getNonChangedServices(device_status.Services, oldWlStatus)
		device_status.Services = append(device_status.Services, nonChangedServices...)
	}

	// When cfgStates is not empty, it contains the config state for all the services.
	// The getServiceStatus() may miss the ones that already suspended and the ones just
	// turned into active.
	// This part update the service config states with the new ones .
	if cfgStates != nil && len(cfgStates) != 0 {
		// updating services cfg states
		for _, cfgState := range cfgStates {
			found := false

			// fill the default for the empty config state
			cfs := cfgState.ConfigState
			if cfs == "" {
				cfs = exchange.SERVICE_CONFIGSTATE_ACTIVE
			}

			for i, workload := range device_status.Services {
				if cfgState.Org == workload.Org && cfgState.Url == workload.ServiceURL {
					device_status.Services[i].ConfigState = cfs
					found = true
				}
			}

			// add the new suspended ones
			if cfs == exchange.SERVICE_CONFIGSTATE_SUSPENDED && !found {
				// service has been suspended so it wasn't found, adding it to the status
				device_status.Services = append(device_status.Services, WorkloadStatus{
					ServiceURL:  cfgState.Url,
					Org:         cfgState.Org,
					Version:     cfgState.Version,
					Arch:        cfgState.Arch,
					ConfigState: cfs,
				})
			}
		}
	}

	// only save the ones that have non empty containers or config state as suspended
	var device_status_new DeviceStatus
	device_status_new.Services = make([]WorkloadStatus, 0)
	for i, workload := range device_status.Services {
		if workload.ConfigState == exchange.SERVICE_CONFIGSTATE_SUSPENDED || len(workload.Containers) > 0 {
			device_status_new.Services = append(device_status_new.Services, device_status.Services[i])
		}
	}

	// report the status to the exchange
	w.deviceStatus = &device_status_new

	statusChanged = changeInWorkloadStatuses(device_status_new.Services, oldWlStatus)

	if statusChanged {
		glog.V(5).Infof(logString(fmt.Sprintf("device status to report to the exchange: %v", device_status_new)))

		if err := w.writeStatusToExchange(&device_status_new); err != nil {
			glog.Errorf(logString(err))
		}
		if err := persistence.SaveNodeStatus(w.db, convertToPersistenceType(device_status_new.Services)); err != nil {
			glog.Errorf(logString(err))
		}
	} else {
		glog.V(5).Infof(logString(fmt.Sprintf("device status unchanged, skipping report to exchange: %v", device_status_new)))
	}
	return 60
}

// getNonChangedServices returns old services that don't have any updates, so their statuses will not be lost
func getNonChangedServices(updatedServices []WorkloadStatus, oldServices []persistence.WorkloadStatus) (suspendedSvcs []WorkloadStatus) {
	for _, svc := range oldServices {
		hasUpdates := false
		for _, updSvc := range updatedServices {
			if updSvc.ServiceURL == svc.ServiceURL && updSvc.Org == svc.Org {
				hasUpdates = true
			}
		}
		if hasUpdates {
			continue
		}
		if svc.ConfigState == exchange.SERVICE_CONFIGSTATE_SUSPENDED {
			suspendedSvcs = append(suspendedSvcs, WorkloadStatus{
				ServiceURL:  svc.ServiceURL,
				Org:         svc.Org,
				Version:     svc.Version,
				Arch:        svc.Arch,
				ConfigState: svc.ConfigState,
			})
		}
	}
	return suspendedSvcs
}

// Find the status for all the Services.
func (w *GovernanceWorker) getServiceStatus(containers []docker.APIContainers) ([]WorkloadStatus, error) {
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
			deployment := ""
			if w.deviceType == persistence.DEVICE_TYPE_DEVICE {
				deployment, _ = msdef.GetDeployment()
			} else {
				deployment = msdef.ClusterDeployment
				if deployment != "" {
					opStatus, err := GetOperatorStatus(deployment)
					if err != nil {
						glog.Errorf(logString(fmt.Sprintf("Error getting operator status: %v", err)))
					} else {
						msdef_status.OperatorStatus = opStatus
					}
				}
			}
			if msinsts, err := persistence.GetAllMicroserviceInstancesWithDefId(w.db, msdef.Id, false, false); err != nil {
				return nil, fmt.Errorf(logString(fmt.Sprintf("Error retrieving all service instances for %v from database, error: %v", msdef.SpecRef, err)))
			} else if msinsts != nil {
				for _, msi := range msinsts {
					glog.V(3).Infof("Gathering status for msdef: %v/%v, working on instance %v", msdef.Org, msdef.SpecRef, msi.GetKey())
					if deployment != "" {
						if cstatus, err := GetContainerStatus(deployment, msi.GetKey(), !msi.IsTopLevelService(), containers); err != nil {
							return nil, fmt.Errorf(logString(fmt.Sprintf("Error getting service container status for %v. %v", msdef.SpecRef, err)))
						} else {
							msdef_status.Containers = append(msdef_status.Containers, cstatus...)
						}
					}

					if msi.IsTopLevelService() {
						msdef_status.AgreementId = msi.GetKey()
					}
				}
			}
			if msdef_status.ConfigState == "" {
				msdef_status.ConfigState = exchange.SERVICE_CONFIGSTATE_ACTIVE
			}

			status = append(status, msdef_status)
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
	} else if kdc, err := persistence.GetKubeDeployment(deployment); err == nil {
		var container_status ContainerStatus

		if kc, err := kube_operator.NewKubeClient(); err != nil {
			container_status.State = fmt.Sprintf("Unknown, error: %v", err)
			status = append(status, container_status)
		} else {
			if kubeStatus, err := kc.Status(kdc.OperatorYamlArchive, ""); err != nil {
				container_status.State = fmt.Sprintf("Unknown, error: %v", err)
				status = append(status, container_status)
			} else {
				for _, container := range kubeStatus {
					container_status.State = container.State
					container_status.Name = container.Name
					container_status.Created = container.CreatedTime
					container_status.Image = container.Image
					status = append(status, container_status)
				}
			}
		}
	} else {
		return nil, fmt.Errorf(logString(fmt.Sprintf("Error Unmarshalling deployment string %v. %v", deployment, err)))
	}
	return status, nil
}

// GetOperatorStatus will check if the given deployment is for a kube operator and return the operator defined status if it is
// Will return nil for the interface and no error if the deployment is not for a kube operator
func GetOperatorStatus(deployment string) (interface{}, error) {
	if kd, err := persistence.GetKubeDeployment(deployment); err == nil {
		client, err := kube_operator.NewKubeClient()
		if err != nil {
			return nil, fmt.Errorf(logString(fmt.Sprintf("Error retrieving operator status from cluster, error: %v", err)))
		}
		opStatus, err := client.OperatorStatus(kd.OperatorYamlArchive, "")
		if err != nil {
			return nil, fmt.Errorf(logString(fmt.Sprintf("Error retrieving operator status from cluster, error: %v", err)))
		}
		return opStatus, nil
	}
	return nil, nil
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

	// Grab the current list of surfaced errors from the cached copy in the worker.
	currentExchangeErrors := &exchange.ExchangeSurfaceError{}
	cachedObj := w.exchErrors.Get(EXCHANGE_ERRORS)
	if cachedObj != nil {
		currentExchangeErrors = cachedObj.(*exchange.ExchangeSurfaceError)
	}

	putErrorsHandler := exchange.GetHTTPPutSurfaceErrorsHandler(w.limitedRetryEC)
	serviceResolverHandler := exchange.GetHTTPServiceResolverHandler(w.limitedRetryEC)
	return exchangesync.UpdateSurfaceErrors(w.db, *pDevice, currentExchangeErrors.ErrorList, putErrorsHandler, serviceResolverHandler, w.BaseWorker.Manager.Config.Edge.SurfaceErrorTimeoutS, w.BaseWorker.Manager.Config.Edge.SurfaceErrorAgreementPersistentS)
}

func changeInWorkloadStatuses(newStatuses []WorkloadStatus, oldStatuses []persistence.WorkloadStatus) bool {
	if len(oldStatuses) != len(newStatuses) {
		return true
	}
	matches := 0
	for _, oldStatus := range oldStatuses {
		for _, newStatus := range newStatuses {
			if statusMatch(newStatus, oldStatus) {
				if !reflect.DeepEqual(newStatus.OperatorStatus, oldStatus.OperatorStatus) {
					return true
				}
				if changeInContainerStatuses(newStatus.Containers, oldStatus.Containers) {
					return true
				}
				if oldStatus.ConfigState != newStatus.ConfigState {
					return true
				}
				matches++
			}
		}
	}
	if matches != len(newStatuses) {
		return true
	}
	return false
}

func statusMatch(newStatus WorkloadStatus, oldStatus persistence.WorkloadStatus) bool {
	return oldStatus.AgreementId == newStatus.AgreementId && oldStatus.ServiceURL == newStatus.ServiceURL && oldStatus.Org == newStatus.Org
}

func changeInContainerStatuses(newContainers []ContainerStatus, oldContainers []persistence.ContainerStatus) bool {
	if len(oldContainers) != len(newContainers) {
		return true
	}
	matches := 0
	for _, oldContainer := range oldContainers {
		for _, newContainer := range newContainers {
			if oldContainer.Name == newContainer.Name && oldContainer.Image == newContainer.Image && oldContainer.Created == newContainer.Created {
				if oldContainer.State == newContainer.State {
					matches++
				} else {
					return true
				}
			}
		}
	}
	if matches != len(newContainers) {
		return true
	}
	return false
}

func convertToPersistenceType(workload []WorkloadStatus) []persistence.WorkloadStatus {
	persistentWls := []persistence.WorkloadStatus{}
	for _, wlStatus := range workload {
		newPersistentWlStatus := persistence.WorkloadStatus{AgreementId: wlStatus.AgreementId,
			ServiceURL: wlStatus.ServiceURL, Org: wlStatus.Org, Version: wlStatus.Version,
			Arch: wlStatus.Arch, OperatorStatus: wlStatus.OperatorStatus, ConfigState: wlStatus.ConfigState}
		newPersistentWlStatus.Containers = converContainerStatusToPersistenceType(wlStatus.Containers)
		persistentWls = append(persistentWls, newPersistentWlStatus)
	}
	return persistentWls
}

func converContainerStatusToPersistenceType(containers []ContainerStatus) []persistence.ContainerStatus {
	persistentCStatuses := []persistence.ContainerStatus{}
	for _, cStatus := range containers {
		persistentCStatuses = append(persistentCStatuses, persistence.ContainerStatus{Name: cStatus.Name, Image: cStatus.Image, Created: cStatus.Created, State: cStatus.State})
	}
	return persistentCStatuses
}
