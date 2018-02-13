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
		BaseWorker: worker.NewBaseWorker(name, config),
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

func ExtractAuthAttributes(attributes []persistence.Attribute, httpAuthAttrs map[string]map[string]string, dockerAuthConfigurations map[string]docker.AuthConfiguration) (map[string]map[string]string, map[string]docker.AuthConfiguration, error) {

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
		} else if attr.GetMeta().Type == "BXDockerRegistryAuthAttributes" {
			a := attr.(persistence.BXDockerRegistryAuthAttributes)

			for _, url := range attr.GetMeta().SensorUrls {
				// TODO: replace this string business with whatever is parsed out of a docker config file
				dockerAuthConfigurations[url] = docker.AuthConfiguration{
					Email:         "",
					Username:      "token",
					Password:      a.Token,
					ServerAddress: url,
				}
			}
		}
	}
	return httpAuthAttrs, dockerAuthConfigurations, nil
}

func authAttributes(db *bolt.DB) (map[string]map[string]string, *docker.AuthConfigurations, error) {

	httpAuthAttrs := make(map[string]map[string]string, 0)
	dockerAuthConfigurations := make(map[string]docker.AuthConfiguration, 0)

	wrapDockerAuth := func() *docker.AuthConfigurations {
		return &docker.AuthConfigurations{Configs: dockerAuthConfigurations}
	}

	// assemble credentials from attributes
	attributes, err := persistence.FindApplicableAttributes(db, "")
	if err != nil {
		return httpAuthAttrs, wrapDockerAuth(), fmt.Errorf("Error fetching attributes. Error: %v", err)
	}

	httpAuthAttrs, dockerAuthConfigurations, err = ExtractAuthAttributes(attributes, httpAuthAttrs, dockerAuthConfigurations)
	return httpAuthAttrs, wrapDockerAuth(), err
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

func processFetch(cfg *config.HorizonConfig, client *docker.Client, db *bolt.DB, pemFiles []string, deploymentDesc *containermessage.DeploymentDescription, torrentUrl url.URL, torrentSig string) error {
	httpAuth, dockerAuth, err := authAttributes(db)
	if err != nil {
		glog.Errorf("Failed to fetch authentication facts before processing packages and / or Docker pulls: %v. Continuing anyway", err)
	}

	// N.B. Using fetcherrors types even for docker pull errors
	var fetchErr error

	skipCheckFn := SkipCheckFn(client)
	if torrentUrl.String() == "" && torrentSig == "" {
		// using Docker pull (newer option, uses docker client to pull images from repos in image names in deployment description)
		// Note: we don't want to make this a fallback option, it's a potential security vector
		glog.V(3).Infof("Empty torrent URL '%v' and Signature '%v' provided in LaunchContext, using Docker pull mechanism to retrieve and load Docker images into local registry", torrentUrl.String(), torrentSig)

		fetchErr = pullImageFromRepos(cfg.Edge, dockerAuth, client, &skipCheckFn, deploymentDesc)

	} else {
		// using Pkg fetch and image load (traditional option, content of images is packaged completely, all content is checked for signature)
		// imageFiles is of form {<repotag>: <part abspath> or empty string}
		var imageFiles map[string]string

		imageFiles, fetchErr = fetch.PkgFetch(cfg.Collaborators.HTTPClientFactory.WrappedNewHTTPClient(), &skipCheckFn, torrentUrl, torrentSig, cfg.Edge.TorrentDir, pemFiles, httpAuth)

		if fetchErr == nil {
			// now load those imageFiles using Docker client
			fetchErr = LoadImagesFromPkgParts(client, imageFiles)
		}
	}

	return fetchErr
}

func (b *TorrentWorker) CommandHandler(command worker.Command) bool {

	switch command.(type) {
	case *FetchCommand:

		cmd := command.(*FetchCommand)
		if lc := b.getLaunchContext(cmd.LaunchContext); lc == nil {
			glog.Errorf("Incoming event was not a known launch context: %T", cmd.LaunchContext)
		} else {
			glog.V(5).Infof("LaunchContext(%T): %v", lc, lc)

			pemFiles, deploymentDesc, err := processDeployment(b.Config, lc.ContainerConfig())
			if err != nil {
				glog.Errorf("Failed to process deployment description and signature after agreement negotiation: %v", err)
				b.Messages() <- events.NewTorrentMessage(events.IMAGE_FETCH_ERROR, deploymentDesc, lc)
				return true
			}

			if fetchErr := processFetch(b.Config, b.client, b.db, pemFiles, deploymentDesc, lc.ContainerConfig().TorrentURL, lc.ContainerConfig().TorrentSignature); fetchErr != nil {
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
