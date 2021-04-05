package imagefetch

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/containermessage"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/worker"
	"strings"
)

type ImageFetchWorker struct {
	worker.BaseWorker // embedded field
	db                *bolt.DB
	client            *docker.Client
}

func NewImageFetchWorker(name string, config *config.HorizonConfig, db *bolt.DB) *ImageFetchWorker {

	// do not start this container if the the node is registered and the type is cluster
	dev, _ := persistence.FindExchangeDevice(db)
	if dev != nil && dev.GetNodeType() == persistence.DEVICE_TYPE_CLUSTER {
		return nil
	}

	var client *docker.Client
	var err error
	if config.Edge.DockerEndpoint != "" {
		client, err = docker.NewClient(config.Edge.DockerEndpoint)
		if err != nil {
			glog.Errorf("Failed to instantiate docker Client: %v", err)
			panic("Unable to instantiate docker Client")
		}
	}

	worker := &ImageFetchWorker{
		BaseWorker: worker.NewBaseWorker(name, config, nil),
		db:         db,
		client:     client,
	}

	worker.Start(worker, 0)
	return worker
}

func (w *ImageFetchWorker) Messages() chan events.Message {
	return w.BaseWorker.Manager.Messages
}

func (w *ImageFetchWorker) NewEvent(incoming events.Message) {

	switch incoming.(type) {
	case *events.EdgeRegisteredExchangeMessage:
		msg, _ := incoming.(*events.EdgeRegisteredExchangeMessage)

		// stop the container worker for the cluster device type
		if msg.DeviceType() == persistence.DEVICE_TYPE_CLUSTER {
			w.Commands <- worker.NewTerminateCommand("cluster node")
		}
	case *events.AgreementReachedMessage:
		msg, _ := incoming.(*events.AgreementReachedMessage)

		fCmd := w.NewFetchCommand(msg.LaunchContext())
		w.Commands <- fCmd

	case *events.LoadContainerMessage:
		msg, _ := incoming.(*events.LoadContainerMessage)

		fCmd := w.NewFetchCommand(msg.LaunchContext())
		w.Commands <- fCmd

	case *events.NodeShutdownCompleteMessage:
		msg, _ := incoming.(*events.NodeShutdownCompleteMessage)
		switch msg.Event().Id {
		case events.UNCONFIGURE_COMPLETE:
			w.Commands <- worker.NewTerminateCommand("shutdown")
		}

	default: //nothing

	}

	return
}

// append the auth attribute to the given auth maps
func ExtractAuthAttributes(attributes []persistence.Attribute, dockerAuthConfigurations map[string][]docker.AuthConfiguration) error {

	for _, attr := range attributes {
		if attr.GetMeta().Type == "DockerRegistryAuthAttributes" {
			a := attr.(persistence.DockerRegistryAuthAttributes)

			// may container multiple auths
			for _, auth := range a.Auths {
				username := "token" // default user name if auth.UserName is empty
				if auth.UserName != "" {
					username = auth.UserName
				}
				a_single := docker.AuthConfiguration{
					Email:         "",
					Username:      username,
					Password:      auth.Token,
					ServerAddress: auth.Registry,
				}
				dockerAuthConfigurations = AppendDockerAuth(dockerAuthConfigurations, a_single)
			}
		}
	}
	return nil
}

// this function append the docker auth object into the map if it does not exists in the map.
func AppendDockerAuth(dockerAuths map[string][]docker.AuthConfiguration, auth docker.AuthConfiguration) map[string][]docker.AuthConfiguration {
	if auth.ServerAddress == "" {
		return dockerAuths
	}

	url := auth.ServerAddress
	if auth_array, ok := dockerAuths[url]; !ok {
		dockerAuths[url] = make([]docker.AuthConfiguration, 0)
		dockerAuths[url] = append(dockerAuths[url], auth)
	} else {
		found := false
		for _, a := range auth_array {
			if a.Username == auth.Username && a.Password == auth.Password {
				found = true
				break
			}
		}
		if !found {
			dockerAuths[url] = append(dockerAuths[url], auth)
		}
	}
	return dockerAuths
}

// append the auth attribute to the given auth maps
func authAttributes(db *bolt.DB, dockerAuthConfigurations map[string][]docker.AuthConfiguration) error {

	// assemble credentials from attributes
	attributes, err := persistence.FindApplicableAttributes(db, "", "")
	if err != nil {
		return fmt.Errorf("Error fetching attributes. Error: %v", err)
	}

	return ExtractAuthAttributes(attributes, dockerAuthConfigurations)
}

// append the image auth from exchange to the given auth maps
func authExchange(imageAuths []events.ImageDockerAuth, dockerAuthConfigurations map[string][]docker.AuthConfiguration) error {

	if imageAuths == nil {
		return nil
	}

	for _, auth := range imageAuths {
		a_single := docker.AuthConfiguration{
			Email:         "",
			Username:      auth.UserName,
			Password:      auth.Password,
			ServerAddress: auth.Registry,
		}
		if a_single.Username == "" {
			a_single.Username = "token"
		}
		dockerAuthConfigurations = AppendDockerAuth(dockerAuthConfigurations, a_single)
	}
	return nil
}

func processDeployment(cfg *config.HorizonConfig, containerConfig events.ContainerConfig) ([]string, *containermessage.DeploymentDescription, error) {
	var pemFiles []string
	var err error

	pemFiles, err = cfg.Collaborators.KeyFileNamesFetcher.GetKeyFileNames(cfg.Edge.PublicKeyPath, cfg.Edge.UserPublicKeyPath)
	if err != nil {
		return pemFiles, nil, fmt.Errorf("Unable to read pemFiles from KeyFileNamesFetcher. Error: %v", err)
	}

	glog.V(3).Infof("Deployment signature for deployment %v validated, continuing to process deployment", containerConfig.Deployment)

	var deploymentDesc containermessage.DeploymentDescription
	if err := json.Unmarshal([]byte(containerConfig.Deployment), &deploymentDesc); err != nil {
		return pemFiles, nil, fmt.Errorf("Error Unmarshalling deployment string %v, error: %v", containerConfig.Deployment, err)
	}

	return pemFiles, &deploymentDesc, nil
}

func processFetch(cfg *config.HorizonConfig, client *docker.Client, db *bolt.DB, deploymentDesc *containermessage.DeploymentDescription, imageDockerAuths []events.ImageDockerAuth) error {
	if client == nil {
		return fmt.Errorf("Docker client is nil. Please make sure DockerEndpoint is set in the configuration file.")
	}

	dockerAuthConfigurations := make(map[string][]docker.AuthConfiguration, 0)

	var err error
	if cfg.Edge.TrustDockerAuthFromOrg {
		err = authExchange(imageDockerAuths, dockerAuthConfigurations)
		if err != nil {
			glog.Errorf("Failed to add authentication facts from exchange before processing packages and / or Docker pulls: %v. Continuing anyway", err)
		}
	}
	err = authAttributes(db, dockerAuthConfigurations)
	if err != nil {
		glog.Errorf("Failed to fetch authentication facts from the attributes before processing packages and / or Docker pulls: %v. Continuing anyway", err)
	}

	return fetchImage(cfg, client, db, deploymentDesc, dockerAuthConfigurations)
}

func fetchImage(cfg *config.HorizonConfig, client *docker.Client, db *bolt.DB, deploymentDesc *containermessage.DeploymentDescription, dockerAuthConfigurations map[string][]docker.AuthConfiguration) error {

	skipCheckFn := SkipCheckFn(client)
	// using Docker pull (newer option, uses docker client to pull images from repos in image names in deployment description)
	// Note: we don't want to make this a fallback option, it's a potential security vector
	glog.V(3).Infof("Using Docker pull mechanism to retrieve and load Docker images into local registry")

	fetchErr := pullImageFromRepos(cfg.Edge, dockerAuthConfigurations, client, &skipCheckFn, deploymentDesc)
	return fetchErr
}

// This function is used by external caller such as hzn command to load the container images.
// containerConfig: it contains the deployment info and the docker auth from the exchange for the service image docker repository.
// dockerAuthConfigurations: additional docker auths for fetching the container images from the docker repository.
//
// If the images are from a docker repo, the order of docker auths that will be used are
// 1) from the exchagne (contained in containerConfig)
// 2) from the dockerAuthConfigurations
// 3) from the config.DockerCredFilePath file.
// 4) from /root/.docker/config.json if 3) is not set.
func ProcessImageFetch(cfg *config.HorizonConfig, client *docker.Client, containerConfig *events.ContainerConfig, dockerAuthConfigurations map[string][]docker.AuthConfiguration) error {

	dockerAuthNew := make(map[string][]docker.AuthConfiguration, 0)

	//make sure that the docker auth from the image overwrites the user defined docker auth for the same repo
	var err error
	if cfg.Edge.TrustDockerAuthFromOrg {
		err = authExchange(containerConfig.ImageDockerAuths, dockerAuthNew)
		if err != nil {
			glog.Errorf("Failed to add authentication facts from exchange before processing packages and / or Docker pulls: %v. Continuing anyway", err)
		}
	}
	for _, v := range dockerAuthConfigurations {
		for _, auth_single := range v {
			dockerAuthNew = AppendDockerAuth(dockerAuthNew, auth_single)
		}
	}

	// unmarshal the deployment string
	var deploymentDesc containermessage.DeploymentDescription
	if err := json.Unmarshal([]byte(containerConfig.Deployment), &deploymentDesc); err != nil {
		return fmt.Errorf("Error Unmarshalling deployment string %v, error: %v", containerConfig.Deployment, err)
	}

	return fetchImage(cfg, client, nil, &deploymentDesc, dockerAuthNew)
}

func (b *ImageFetchWorker) CommandHandler(command worker.Command) bool {

	switch command.(type) {
	case *FetchCommand:

		cmd := command.(*FetchCommand)
		if lc := events.GetLaunchContext(cmd.LaunchContext); lc == nil {
			glog.Errorf("Incoming event was not a known launch context: %T", cmd.LaunchContext)
		} else {
			glog.V(5).Infof("LaunchContext(%T): %v", lc, lc)

			// ignore the cluster deployment
			if lc.ContainerConfig().ClusterDeployment != "" {
				glog.V(5).Infof("Image fetching process ignoring the cluster deployment.")
				return true
			}

			// Check the deployment string to see if it's a native Horizon deployment. If not, ignore the event.
			deploymentConfig := lc.ContainerConfig().Deployment

			if _, err := containermessage.GetNativeDeployment(deploymentConfig); err != nil {
				glog.Warningf("Ignoring deployment: %v", err)
				return true
			}

			_, deploymentDesc, err := processDeployment(b.Config, lc.ContainerConfig())
			if err != nil {
				err = fmt.Errorf("Failed to process deployment description and signature after agreement negotiation: %v", err)
				glog.Errorf(err.Error())
				b.Messages() <- events.NewImageFetchMessage(events.IMAGE_FETCH_ERROR, deploymentDesc, lc, err)
				return true
			}

			if fetchErr := processFetch(b.Config, b.client, b.db, deploymentDesc, lc.ContainerConfig().ImageDockerAuths); fetchErr != nil {
				var id events.EventId
				if strings.Contains(fetchErr.Error(), "Auth error") {
					id = events.IMAGE_FETCH_AUTH_ERROR
				} else {
					id = events.IMAGE_FETCH_ERROR
				}
				glog.Errorf("Failed to fetch image files: %v", fetchErr)
				b.Messages() <- events.NewImageFetchMessage(id, deploymentDesc, lc, fetchErr)
			} else {
				b.Messages() <- events.NewImageFetchMessage(events.IMAGE_FETCHED, deploymentDesc, lc, nil)
			}

		}

	default:
		return false
	}
	return true

}

type FetchCommand struct {
	LaunchContext interface{}
}

func (f FetchCommand) ShortString() string {
	lc := ""
	lcObj := events.GetLaunchContext(f.LaunchContext)
	if lcObj != nil {
		lc = lcObj.ShortString()
	}
	return fmt.Sprintf("LaunchContext: %v", lc)
}

func (t *ImageFetchWorker) NewFetchCommand(launchContext interface{}) *FetchCommand {
	return &FetchCommand{
		LaunchContext: launchContext,
	}
}
