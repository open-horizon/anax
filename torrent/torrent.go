package torrent

import (
	"fmt"
	"net/url"

	"encoding/json"
	"github.com/boltdb/bolt"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/containermessage"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/worker"
	fetch "github.com/open-horizon/horizon-pkg-fetch"
	"github.com/open-horizon/horizon-pkg-fetch/fetcherrors"
	"os"
	"strings"
)

type TorrentWorker struct {
	worker.BaseWorker // embedded field
	db                *bolt.DB
	client            *docker.Client
}

func NewTorrentWorker(name string, config *config.HorizonConfig, db *bolt.DB) *TorrentWorker {

	cl, err := docker.NewClient(config.Edge.DockerEndpoint)
	if err != nil {
		glog.Errorf("Failed to instantiate docker Client: %v", err)
		panic("Unable to instantiate docker Client")
	}

	worker := &TorrentWorker{
		BaseWorker: worker.NewBaseWorker(name, config, nil),
		db:         db,
		client:     cl,
	}

	worker.Start(worker, 0)
	return worker
}

func (w *TorrentWorker) Messages() chan events.Message {
	return w.BaseWorker.Manager.Messages
}

func (w *TorrentWorker) NewEvent(incoming events.Message) {

	switch incoming.(type) {
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
func ExtractAuthAttributes(attributes []persistence.Attribute, httpAuthAttrs map[string]map[string]string, dockerAuthConfigurations map[string][]docker.AuthConfiguration) error {

	for _, attr := range attributes {
		if attr.GetMeta().Type == "HTTPSBasicAuthAttributes" {
			a := attr.(persistence.HTTPSBasicAuthAttributes)
			cred := map[string]string{
				"username": a.Username,
				"password": a.Password,
			}

			// we don't care about apply-all settings, they're a security problem (TODO: add an API check for this case)
			for _, url := range attr.GetMeta().SensorUrls {
				httpAuthAttrs[url] = cred
			}
		} else if attr.GetMeta().Type == "DockerRegistryAuthAttributes" {
			a := attr.(persistence.DockerRegistryAuthAttributes)

			// should have one url, but we iterate through it anyway
			for _, url := range attr.GetMeta().SensorUrls {
				// may container multiple auths
				for _, auth := range a.Auths {
					a_single := docker.AuthConfiguration{
						Email:         "",
						Username:      "token",
						Password:      auth.Token,
						ServerAddress: url,
					}
					dockerAuthConfigurations = AppendDockerAuth(dockerAuthConfigurations, a_single)
				}
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
func authAttributes(db *bolt.DB, httpAuthAttrs map[string]map[string]string, dockerAuthConfigurations map[string][]docker.AuthConfiguration) error {

	// assemble credentials from attributes
	attributes, err := persistence.FindApplicableAttributes(db, "")
	if err != nil {
		return fmt.Errorf("Error fetching attributes. Error: %v", err)
	}

	return ExtractAuthAttributes(attributes, httpAuthAttrs, dockerAuthConfigurations)
}

// append the image auth from exchange to the given auth maps
func authExchange(imageAuths []events.ImageDockerAuth, dockerAuthConfigurations map[string][]docker.AuthConfiguration) error {

	if imageAuths == nil {
		return nil
	}

	for _, auth := range imageAuths {
		a_single := docker.AuthConfiguration{
			Email:         "",
			Username:      "token",
			Password:      auth.Password,
			ServerAddress: auth.Registry,
		}
		dockerAuthConfigurations = AppendDockerAuth(dockerAuthConfigurations, a_single)
	}
	return nil
}

// Copy the given http auth to a new map and then add the default http auth (org/device_id:device_token) to the new map.
// The given httpAuthAttrs is unchanged.
func addDefaultHttpAuth(db *bolt.DB, pkgUrl string, httpAuthAttrs map[string]map[string]string) (map[string]map[string]string, error) {
	// copy the given map to a new map
	new_auth := make(map[string]map[string]string, 0)
	for k, v := range httpAuthAttrs {
		new_auth[k] = v
	}

	//get the device org, id and token and create the default http auth and add it to the new map

	var cred map[string]string
	if db != nil {
		if dev, err := persistence.FindExchangeDevice(db); err != nil {
			return new_auth, fmt.Errorf("Received error getting device: %v", err)
		} else if dev == nil {
			return new_auth, fmt.Errorf("Could not get device because no device was registered yet.")
		} else {
			cred = map[string]string{
				"username": fmt.Sprintf("%v/%v", dev.Org, dev.Id),
				"password": dev.Token,
			}
		}
	} else if os.Getenv("HZN_ORG_ID") != "" && os.Getenv("HZN_DEVICE_ID") != "" && os.Getenv("HZN_DEVICE_TOKEN") != "" {
		// this is the case where the function is called from hzn
		cred = map[string]string{
			"username": fmt.Sprintf("%v/%v", os.Getenv("HZN_ORG_ID"), os.Getenv("HZN_DEVICE_ID")),
			"password": os.Getenv("HZN_DEVICE_TOKEN"),
		}
	} else {
		return new_auth, fmt.Errorf("The HZN_ORG_ID, HZN_DEVICE_ID or HZN_DEVICE_TOKEN is not set.")
	}

	// get the http path from the torrent url
	i := strings.LastIndex(pkgUrl, "/")
	repo_url := pkgUrl
	if i > 0 {
		repo_url = pkgUrl[0:i]
	}

	// add the default http auth
	new_auth[repo_url] = cred

	return new_auth, nil
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

func processFetch(cfg *config.HorizonConfig, client *docker.Client, db *bolt.DB, pemFiles []string, deploymentDesc *containermessage.DeploymentDescription, torrentUrl url.URL, torrentSig string, imageDockerAuths []events.ImageDockerAuth) error {
	httpAuthAttrs := make(map[string]map[string]string, 0)
	dockerAuthConfigurations := make(map[string][]docker.AuthConfiguration, 0)

	var err error
	if cfg.Edge.TrustDockerAuthFromOrg {
		err = authExchange(imageDockerAuths, dockerAuthConfigurations)
		if err != nil {
			glog.Errorf("Failed to add authentication facts from exchange before processing packages and / or Docker pulls: %v. Continuing anyway", err)
		}
	}
	err = authAttributes(db, httpAuthAttrs, dockerAuthConfigurations)
	if err != nil {
		glog.Errorf("Failed to fetch authentication facts from the attributes before processing packages and / or Docker pulls: %v. Continuing anyway", err)
	}

	return fetchImage(cfg, client, db, pemFiles, deploymentDesc, torrentUrl, torrentSig, httpAuthAttrs, dockerAuthConfigurations)
}

func fetchImage(cfg *config.HorizonConfig, client *docker.Client, db *bolt.DB, pemFiles []string, deploymentDesc *containermessage.DeploymentDescription, torrentUrl url.URL, torrentSig string, httpAuthAttrs map[string]map[string]string, dockerAuthConfigurations map[string][]docker.AuthConfiguration) error {
	// N.B. Using fetcherrors types even for docker pull errors
	var fetchErr error

	skipCheckFn := SkipCheckFn(client)
	if torrentUrl.String() == "" && torrentSig == "" {
		// using Docker pull (newer option, uses docker client to pull images from repos in image names in deployment description)
		// Note: we don't want to make this a fallback option, it's a potential security vector
		glog.V(3).Infof("Empty torrent URL '%v' and Signature '%v' provided in LaunchContext, using Docker pull mechanism to retrieve and load Docker images into local registry", torrentUrl.String(), torrentSig)

		fetchErr = pullImageFromRepos(cfg.Edge, dockerAuthConfigurations, client, &skipCheckFn, deploymentDesc)

	} else {
		// using Pkg fetch and image load (traditional option, content of images is packaged completely, all content is checked for signature)
		// imageFiles is of form {<repotag>: <part abspath> or empty string}
		var imageFiles map[string]string

		foundHttpAuth := false
		for k, _ := range httpAuthAttrs {
			if strings.HasPrefix(torrentUrl.String(), k) {
				foundHttpAuth = true
			}
		}

		if foundHttpAuth {
			// use the user defined http auth to fetch the image files
			glog.V(5).Infof("Try to fetch the image files for %v with user defined auth attributes %v.", torrentUrl.String(), httpAuthAttrs)
			imageFiles, fetchErr = fetch.PkgFetch(cfg.Collaborators.HTTPClientFactory.WrappedNewHTTPClient(), &skipCheckFn, torrentUrl, torrentSig, cfg.Edge.TorrentDir, pemFiles, httpAuthAttrs)
		} else {
			// try to add org/device_id:device_token as the http username and password
			new_auth := make(map[string]map[string]string, 0)
			if new_auth, fetchErr = addDefaultHttpAuth(db, torrentUrl.String(), httpAuthAttrs); fetchErr != nil {
				glog.Errorf("Failed to get the default http auth for package %v. %v Continuing anyway", torrentUrl.String(), fetchErr)
			}

			if fetchErr == nil {
				glog.V(5).Infof("Try to fetch the image files for %v with default auth %v.", torrentUrl.String(), new_auth)
				imageFiles, fetchErr = fetch.PkgFetch(cfg.Collaborators.HTTPClientFactory.WrappedNewHTTPClient(), &skipCheckFn, torrentUrl, torrentSig, cfg.Edge.TorrentDir, pemFiles, new_auth)
			}
			if fetchErr != nil {
				// try without added default http auth
				glog.V(5).Infof("Default auth failed, try to fetch the image files for %v without default auth. The auth used are: %v.", torrentUrl.String(), httpAuthAttrs)
				imageFiles, fetchErr = fetch.PkgFetch(cfg.Collaborators.HTTPClientFactory.WrappedNewHTTPClient(), &skipCheckFn, torrentUrl, torrentSig, cfg.Edge.TorrentDir, pemFiles, httpAuthAttrs)
			}
		}

		if fetchErr == nil {
			// now load those imageFiles using Docker client
			fetchErr = LoadImagesFromPkgParts(client, imageFiles)
		}
	}

	return fetchErr
}

// This function is used by external caller such as hzn command to load the container images.
// containerConfig: it contains the deployment info and the docker auth from the exchange for the service image docker repository.
// httpAuthAttrs: the http auths for fetching the packages fromt he image server if torrent url is set in containerConfig.
// dockerAuthConfigurations: additional docker auths for fetching the container images from the docker repository.
// pemFiles: public key files for verifying the deployment signature or torrent signature.
//
// If the images are from a docker repo, the order of docker auths that will be used are
// 1) from the exchagne (contained in containerConfig)
// 2) from the dockerAuthConfigurations
// 3) from the config.DockerCredFilePath file.
// 4) from /root/.docker/config.json if 3) is not set.
//
// If the image is from a image server, the order of the http auth that will be used are:
// 1) from the httpAuthAttrs
// 2) from the env variable $HZN_ORG_ID/$HZN_DEVICE_ID:$HZN_DEVICE_TOKEN if httpAuthAttrs is not set for this image.
// 3) try without auth if 2) fails.
func ProcessImageFetch(cfg *config.HorizonConfig, client *docker.Client, containerConfig *events.ContainerConfig, httpAuthAttrs map[string]map[string]string, dockerAuthConfigurations map[string][]docker.AuthConfiguration, pemFiles []string) error {

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

	return fetchImage(cfg, client, nil, pemFiles, &deploymentDesc, containerConfig.TorrentURL, containerConfig.TorrentSignature, httpAuthAttrs, dockerAuthNew)
}

func (b *TorrentWorker) CommandHandler(command worker.Command) bool {

	switch command.(type) {
	case *FetchCommand:

		cmd := command.(*FetchCommand)
		if lc := b.getLaunchContext(cmd.LaunchContext); lc == nil {
			glog.Errorf("Incoming event was not a known launch context: %T", cmd.LaunchContext)
		} else {
			glog.V(5).Infof("LaunchContext(%T): %v", lc, lc)

			// Check the deployment string to see if it's a native Horizon deployment. If not, ignore the event.
			deploymentConfig := lc.ContainerConfig().Deployment
			if _, err := containermessage.GetNativeDeployment(deploymentConfig); err != nil {
				glog.Warningf("Ignoring deployment: %v", err)
				return true
			}

			pemFiles, deploymentDesc, err := processDeployment(b.Config, lc.ContainerConfig())
			if err != nil {
				glog.Errorf("Failed to process deployment description and signature after agreement negotiation: %v", err)
				b.Messages() <- events.NewTorrentMessage(events.IMAGE_FETCH_ERROR, deploymentDesc, lc)
				return true
			}

			if fetchErr := processFetch(b.Config, b.client, b.db, pemFiles, deploymentDesc, lc.ContainerConfig().TorrentURL, lc.ContainerConfig().TorrentSignature, lc.ContainerConfig().ImageDockerAuths); fetchErr != nil {
				var id events.EventId
				switch fetchErr.(type) {
				case fetcherrors.PkgMetaError, fetcherrors.PkgSourceError, fetcherrors.PkgPrecheckError:
					id = events.IMAGE_DATA_ERROR

				case fetcherrors.PkgSourceFetchError:
					id = events.IMAGE_FETCH_ERROR

				case fetcherrors.PkgSourceFetchAuthError:
					id = events.IMAGE_FETCH_AUTH_ERROR

				case fetcherrors.PkgSignatureVerificationError:
					id = events.IMAGE_SIG_VERIF_ERROR

				default:
					id = events.IMAGE_FETCH_ERROR
				}
				glog.Errorf("Failed to fetch image files: %v", fetchErr)
				b.Messages() <- events.NewTorrentMessage(id, deploymentDesc, lc)
			} else {
				b.Messages() <- events.NewTorrentMessage(events.IMAGE_FETCHED, deploymentDesc, lc)
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
	return fmt.Sprintf("%v", f)
}

func (t *TorrentWorker) NewFetchCommand(launchContext interface{}) *FetchCommand {
	return &FetchCommand{
		LaunchContext: launchContext,
	}
}

func (t *TorrentWorker) getLaunchContext(launchContext interface{}) events.LaunchContext {
	switch launchContext.(type) {
	case *events.ContainerLaunchContext:
		lc := launchContext.(events.LaunchContext)
		return lc
	case *events.AgreementLaunchContext:
		lc := launchContext.(events.LaunchContext)
		return lc
	}
	return nil
}
