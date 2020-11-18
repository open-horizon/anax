package sync_service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cli/dev"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/container"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/resource"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

const LABEL_PREFIX = "openhorizon.hzn-dev"
const NETWORK_NAME = "hzn-dev"
const MONGO_NAME = "mongo"
const CSS_NAME = "css-api"
const ESS_NAME = "ess-api"
const CSS_INITIAL_WAITING_TIME = 10

func Start(cw *container.ContainerWorker, org string, configFiles []string, configType string) error {

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	dc := cw.GetClient()

	// Create a network for all the sync service containers.
	network, err := container.CreateNetwork(dc, NETWORK_NAME)
	if err != nil {
		return errors.New(msgPrinter.Sprintf("unable to create network %v for file sync service, error %v", NETWORK_NAME, err))
	}

	// Start the CSS.
	if err := startCSS(dc, network); err != nil {
		return errors.New(msgPrinter.Sprintf("unable to start CSS, error %v", err))
	}

	// Wait a few seconds to give the CSS a chance to initialize itself.
	time.Sleep(time.Second * 2)

	// Get this host's IP address because that's where the CSS is listening.
	hostIP := os.Getenv("HZN_DEV_HOST_IP")
	if hostIP == "" {
		hostIP = "localhost"
	}

	// Call CSS /status API to check if CSS is ready (retry for 10 seconds)
	if err := checkCSSStatus(org, CSS_INITIAL_WAITING_TIME); err != nil {
		cliutils.Verbose(msgPrinter.Sprintf("Error checking CSS status: %v", err))

		// Stop and remove the CSS container.
		if err := stopContainer(dc, makeLabelName(CSS_NAME)); err != nil {
			return errors.New(msgPrinter.Sprintf("Unable to stop %v, error %v, please stop manually, Remove the css image and try 'hzn dev service start' again", makeLabelName(CSS_NAME), err))
		}

		// remove the image
		if err := removeImage(getFSSImageName(), getFSSImageTagName(), dc); err != nil {
			return errors.New(msgPrinter.Sprintf("Unable to remove CSS image, err: %v", err))
		}

		// pull CSS image again try again and start CSS container
		if err := startCSS(dc, network); err != nil {
			return errors.New(msgPrinter.Sprintf("Unable to start CSS with new pulled image, error %v", err))
		}

		// Wait a few seconds to give the CSS a chance to initialize itself.
		time.Sleep(time.Second * 2)
		if err := checkCSSStatus(org, CSS_INITIAL_WAITING_TIME); err != nil {
			return errors.New(msgPrinter.Sprintf("CSS is not running correctly with new pulled image, exit."))
		}
	}

	// Load the input file objects into the CSS
	if err := loadCSS(org, configType, configFiles); err != nil {
		return errors.New(msgPrinter.Sprintf("unable to load file objects into CSS, error %v", err))
	}

	// Start the ESS.
	if err := startESS(cw, network, org); err != nil {
		return errors.New(msgPrinter.Sprintf("unable to start ESS, error %v", err))
	}

	return nil
}

func Stop(dc *docker.Client) error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Stop and remove the ESS container.
	if err := stopContainer(dc, makeLabelName(ESS_NAME)); err != nil {
		cliutils.Verbose(msgPrinter.Sprintf("Unable to stop %v, error %v", makeLabelName(ESS_NAME), err))
	}

	// Stop and remove the CSS container.
	if err := stopContainer(dc, makeLabelName(CSS_NAME)); err != nil {
		cliutils.Verbose(msgPrinter.Sprintf("Unable to stop %v, error %v", makeLabelName(CSS_NAME), err))
	}

	// Delete the hzn-dev network.
	if err := container.RemoveNetwork(dc, NETWORK_NAME); err != nil {
		cliutils.Verbose(msgPrinter.Sprintf("Unable to remove network %v for file sync service, error %v", NETWORK_NAME, err))
	}

	return nil
}

// Make sure the file sync service docker images are available locally. Either they are already present in the
// local docker repo or we need to pull them in. This function checks for an exact match of image and tag name.
// It does not try to re-pull if the image is already local.
func getImage(imageName string, tagName string, dc *docker.Client) error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Check if the image already exists locally. If it does then skip the pull.
	name := fmt.Sprintf("%v:%v", imageName, tagName)
	skipPull := false
	if images, err := dc.ListImages(docker.ListImagesOptions{
		All: true,
	}); err != nil {
		return errors.New(msgPrinter.Sprintf("unable to list existing docker images, error %v", err))
	} else {
		for _, image := range images {
			for _, r := range image.RepoTags {
				if r == name {
					skipPull = true
					cliutils.Verbose(msgPrinter.Sprintf("Found docker image %v locally.", name))
					break
				}
			}
			// Exit the outter loop if we found the image locally.
			if skipPull {
				break
			}
		}
	}

	// If the image was not found locally, pull it from docker.
	if !skipPull {
		opts := docker.PullImageOptions{
			Repository: imageName,
			Tag:        tagName,
		}

		if err := dc.PullImage(opts, docker.AuthConfiguration{}); err != nil {
			return errors.New(msgPrinter.Sprintf("unable to pull CSS container using image %v, error %v. Set environment variable %v to use a different image tag.", getFSSFullImageName(), err, dev.DEVTOOL_HZN_FSS_IMAGE_TAG))
		} else {
			cliutils.Verbose(msgPrinter.Sprintf("Pulled docker image %v.", name))
		}
	}

	return nil
}

// remove image. Ignore error if image does not exist
func removeImage(imageName string, tagName string, dc *docker.Client) error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	name := fmt.Sprintf("%v:%v", imageName, tagName)
	cliutils.Verbose(msgPrinter.Sprintf("Removing docker image %v.", name))
	if err := dc.RemoveImage(name); err != nil {
		cliutils.Verbose(msgPrinter.Sprintf("RemoveImageErr: %v", err))
		if err.Error() != fmt.Sprintf("Error: No such image: %s", name) {
			return errors.New(msgPrinter.Sprintf("unable to remove CSS image: %s, please manually remove it 'docker rmi %s'", name, name))
		}
	} else {
		cliutils.Verbose(msgPrinter.Sprintf("Docker image %v removed.", name))
	}
	return nil
}

// Start the CSS container.
func startCSS(dc *docker.Client, network *docker.Network) error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// First load the image.
	if err := getImage(getFSSImageName(), getFSSImageTagName(), dc); err != nil {
		return errors.New(msgPrinter.Sprintf("unable to pull CSS container using image %v, error %v. Set environment variable %v to use a different image tag.", getFSSFullImageName(), err, dev.DEVTOOL_HZN_FSS_IMAGE_TAG))
	}

	// Now create the container from this image.
	var emptyS struct{}

	// Setup the CSS to listen on a host port so that the CLI can preload file objects.
	port := docker.Port(getCSSPort())

	pb := map[docker.Port][]docker.PortBinding{}
	pb[port] = []docker.PortBinding{
		docker.PortBinding{
			HostIP:   "0.0.0.0",
			HostPort: getCSSPort(),
		},
	}

	ep := map[docker.Port]struct{}{}
	ep[port] = emptyS

	// Setup the env var configuration of the CSS.
	dockerConfig := docker.Config{
		Image: getFSSFullImageName(),
		Env: []string{"NODE_TYPE=CSS",
			"UNSECURE_LISTENING_PORT=" + getCSSPort(),
			"COMMUNICATION_PROTOCOL=http",
			"LOG_LEVEL=TRACE",
			"LOG_ROOT_PATH=/tmp/",
			"LOG_TRACE_DESTINATION=stdout",
			"TRACE_LEVEL=TRACE",
			"TRACE_ROOT_PATH=/tmp/",
			"STORAGE_PROVIDER=bolt"},
		CPUSet:       "",
		Labels:       makeLabel(CSS_NAME),
		ExposedPorts: ep,
	}

	dockerHostConfig := docker.HostConfig{
		PublishAllPorts: false,
		PortBindings:    pb,
		Links:           nil,
		RestartPolicy:   docker.AlwaysRestart(),
		Memory:          1000 * 1024 * 1024,
		MemorySwap:      0,
		Devices:         []docker.Device{},
		LogConfig:       docker.LogConfig{},
		Binds:           []string{},
	}

	endpointsConfig := map[string]*docker.EndpointConfig{
		network.Name: &docker.EndpointConfig{
			Aliases:   []string{CSS_NAME},
			Links:     nil,
			NetworkID: network.ID,
		},
	}

	containerOpts := docker.CreateContainerOptions{
		Name:       makeLabelName(CSS_NAME),
		Config:     &dockerConfig,
		HostConfig: &dockerHostConfig,
		NetworkingConfig: &docker.NetworkingConfig{
			EndpointsConfig: endpointsConfig,
		},
	}

	if container, err := dc.CreateContainer(containerOpts); err != nil {
		return errors.New(msgPrinter.Sprintf("unable to create CSS container using image %v, error %v. Set environment variable %v to use a diferent image tag.", getFSSImageName(), err, dev.DEVTOOL_HZN_FSS_IMAGE_TAG))
	} else if err := dc.StartContainer(container.ID, nil); err != nil {
		return errors.New(msgPrinter.Sprintf("unable to start CSS container, error %v", err))
	}

	cliutils.Verbose(msgPrinter.Sprintf("Created %v container, listening on host port %v", makeLabelName(CSS_NAME), getCSSPort()))
	msgPrinter.Printf("File sync service container %v listening on host port %v\n", makeLabelName(CSS_NAME), getCSSPort())

	return nil

}

// Start the ESS container.
func startESS(cw *container.ContainerWorker, network *docker.Network, org string) error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Create a self signed SSL cert for the workload to use.
	if err := resource.CreateCertificate(org, cw.Config.GetESSSSLCertKeyPath(), cw.Config.GetESSSSLClientCertPath()); err != nil {
		return errors.New(msgPrinter.Sprintf("unable to create SSL certificate for ESS, error %v", err))
	}

	// Pass our certificate and key into the ESS config by value, as a string of bytes.

	certFile := path.Join(cw.Config.GetESSSSLClientCertPath(), config.HZN_FSS_CERT_FILE)
	certKeyFile := path.Join(cw.Config.GetESSSSLCertKeyPath(), config.HZN_FSS_CERT_KEY_FILE)

	serverCert := ""
	serverCertKey := ""

	if essCert, err := os.Open(certFile); err != nil {
		return errors.New(msgPrinter.Sprintf("unable to open ESS SSL Certificate file %v, error %v", cw.Config.GetESSSSLClientCertPath(), err))
	} else if essCertBytes, err := ioutil.ReadAll(essCert); err != nil {
		return errors.New(msgPrinter.Sprintf("unable to read ESS SSL Certificate file %v, error %v", cw.Config.GetESSSSLClientCertPath(), err))
	} else if essCertKey, err := os.Open(certKeyFile); err != nil {
		return errors.New(msgPrinter.Sprintf("unable to open ESS SSL Certificate Key file %v, error %v", cw.Config.GetESSSSLCertKeyPath(), err))
	} else if essCertKeyBytes, err := ioutil.ReadAll(essCertKey); err != nil {
		return errors.New(msgPrinter.Sprintf("unable to read ESS SSL Certificate Key file %v, error %v", cw.Config.GetESSSSLCertKeyPath(), err))
	} else {
		serverCert = string(essCertBytes)
		serverCertKey = string(essCertKeyBytes)
	}

	// Get the docker client API out of the container worker.
	dc := cw.GetClient()

	// Setup the env vars to configure the ESS for this test environment.

	workingDir := path.Join(dev.GetDevWorkingDirectory(), "essapi.sock")

	envVars := []string{
		"NODE_TYPE=ESS",
		"LISTENING_TYPE=secure-unix",
		"LISTENING_ADDRESS=" + workingDir,
		"COMMUNICATION_PROTOCOL=http",
		"SERVER_CERTIFICATE=" + serverCert,
		"SERVER_KEY=" + serverCertKey,
		"HTTP_CSS_HOST=" + CSS_NAME,
		"HTTP_CSS_PORT=" + getCSSPort(),
		"PERSISTENCE_ROOT_PATH=/tmp/",
		"LOG_LEVEL=TRACE",
		"LOG_ROOT_PATH=/tmp/",
		"LOG_TRACE_DESTINATION=stdout",
		"TRACE_LEVEL=TRACE",
		"TRACE_ROOT_PATH=/tmp/",
	}
	envVars = append(envVars, "ORG_ID="+org)
	envVars = append(envVars, "DESTINATION_ID="+dev.GetNodeId())

	nodeType := os.Getenv(dev.DEVTOOL_HZN_PATTERN)
	if nodeType == "" {
		nodeType = "hzn-dev-test"
	}
	envVars = append(envVars, "DESTINATION_TYPE="+nodeType)

	dockerConfig := docker.Config{
		Image:        getFSSFullImageName(),
		Env:          envVars,
		CPUSet:       "",
		Labels:       makeLabel(ESS_NAME),
		ExposedPorts: map[docker.Port]struct{}{},
	}

	dockerHostConfig := docker.HostConfig{
		PublishAllPorts: false,
		PortBindings:    map[docker.Port][]docker.PortBinding{},
		Links:           nil,
		RestartPolicy:   docker.AlwaysRestart(),
		Memory:          1000 * 1024 * 1024,
		MemorySwap:      0,
		Devices:         []docker.Device{},
		LogConfig:       docker.LogConfig{},
		Binds:           []string{fmt.Sprintf("%v:%v", dev.GetDevWorkingDirectory(), dev.GetDevWorkingDirectory())},
	}

	endpointsConfig := map[string]*docker.EndpointConfig{
		network.Name: &docker.EndpointConfig{
			Aliases:   []string{ESS_NAME},
			Links:     nil,
			NetworkID: network.ID,
		},
	}

	containerOpts := docker.CreateContainerOptions{
		Name:       makeLabelName(ESS_NAME),
		Config:     &dockerConfig,
		HostConfig: &dockerHostConfig,
		NetworkingConfig: &docker.NetworkingConfig{
			EndpointsConfig: endpointsConfig,
		},
	}

	if container, err := dc.CreateContainer(containerOpts); err != nil {
		return errors.New(msgPrinter.Sprintf("unable to create ESS container, error %v", err))
	} else if err := dc.StartContainer(container.ID, nil); err != nil {
		return errors.New(msgPrinter.Sprintf("unable to start ESS container, error %v", err))
	}

	cliutils.Verbose(msgPrinter.Sprintf("Created %v container", makeLabelName(ESS_NAME)))

	return nil

}

// Stop the container.
func stopContainer(dc *docker.Client, name string) error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	devFilter := docker.ListContainersOptions{
		All: true,
		Filters: map[string][]string{
			"label": []string{name},
		},
	}

	allContainers, err := dc.ListContainers(devFilter)
	if err != nil {
		return errors.New(msgPrinter.Sprintf("unable to list docker containers, error %v", err))
	}

	for _, con := range allContainers {
		if strings.Contains(con.Names[0], name) {
			if err := dc.KillContainer(docker.KillContainerOptions{ID: con.ID}); err != nil {
				return errors.New(msgPrinter.Sprintf("unable to stop docker container %v, error %v", name, err))
			} else if err := dc.RemoveContainer(docker.RemoveContainerOptions{ID: con.ID, RemoveVolumes: true, Force: true}); err != nil {
				return errors.New(msgPrinter.Sprintf("unable to remove docker container %v, error %v", name, err))
			} else {
				cliutils.Verbose(msgPrinter.Sprintf("Stopped %v container", name))
			}
		}
	}

	return nil
}

func loadCSS(org string, fileType string, fileObjects []string) error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	for _, fileName := range fileObjects {
		cliutils.Verbose(msgPrinter.Sprintf("Loading %v into CSS", fileName))

		if fileObject, err := os.Open(fileName); err != nil {
			return errors.New(msgPrinter.Sprintf("unable to open file object %v, error %v", fileName, err))
		} else if fileBytes, err := ioutil.ReadAll(fileObject); err != nil {
			return errors.New(msgPrinter.Sprintf("unable to read file object %v, error %v", fileName, err))
		} else {

			defer fileObject.Close()

			fileObjectName := path.Base(fileName)

			metadata := &cssFileMeta{
				ObjectID:   fileObjectName,
				ObjectType: fileType,
			}

			// Get this host's IP address because that's where the CSS is listening.
			hostIP := os.Getenv("HZN_DEV_HOST_IP")
			if hostIP == "" {
				hostIP = "localhost"
			}

			// Form the CSS URL and then PUT the file into the CSS.
			url := fmt.Sprintf("http://%v:%v/api/v1/objects/%v/%v/%v", hostIP, getCSSPort(), org, fileType, fileObjectName)

			if err := putFile(url, org, metadata, fileBytes); err != nil {
				return errors.New(msgPrinter.Sprintf("unable to add file %v to the CSS, error %v", fileName, err))
			}
		}
	}

	if len(fileObjects) > 0 {
		msgPrinter.Printf("Configuration files %v loaded into the File sync service.", fileObjects)
		msgPrinter.Println()
	}

	return nil

}

type cssFileMeta struct {
	ObjectID   string `json:"objectID"`
	ObjectType string `json:"objectType"`
}

type cssFilePutBody struct {
	Data []byte      `json:"data"`
	Meta cssFileMeta `json:"meta"`
}

func putFile(url string, org string, metadata *cssFileMeta, file []byte) error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Tell the user what API we're about to use.
	apiMsg := http.MethodPut + " " + url
	cliutils.Verbose(apiMsg)

	// Construct the PUT body
	body := cssFilePutBody{
		Data: file,
		Meta: *metadata,
	}

	// Convert the body to JSON form.
	jsonBytes, err := json.Marshal(body)
	if err != nil {
		return errors.New(msgPrinter.Sprintf("unable to marshal CSS file PUT for %v, error %v", *metadata, err))
	}
	requestBody := bytes.NewBuffer(jsonBytes)

	// First put the metadata into the CSS.
	httpClient := cliutils.GetHTTPClient(0)
	req, err := http.NewRequest(http.MethodPut, url, requestBody)
	if err != nil {
		return errors.New(msgPrinter.Sprintf("unable to create CSS file PUT request for %v, error %v", *metadata, err))
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	// Add a dummy basic auth header. The CSS should be configured with a dummy basic auth authenticator.
	req.SetBasicAuth(org+"/hzndev", "password")

	resp, err := httpClient.Do(req)
	if err != nil {
		return errors.New(msgPrinter.Sprintf("unable to send CSS file PUT request to CSS for %v, error %v", *metadata, err))
	}

	defer resp.Body.Close()
	cliutils.Verbose(msgPrinter.Sprintf("Received HTTP code: %d", resp.StatusCode))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New(msgPrinter.Sprintf("unable to PUT file %v into CSS, HTTP code %v", *metadata, resp.StatusCode))
	}

	return nil
}

func makeLabelName(name string) string {
	return fmt.Sprintf("%v.%v", LABEL_PREFIX, name)
}

func makeLabel(name string) map[string]string {
	lm := make(map[string]string)
	lm[makeLabelName(name)] = ""
	return lm
}

func checkCSSStatus(org string, timeout int) error {
	msgPrinter := i18n.GetMessagePrinter()

	// Get this host's IP address because that's where the CSS is listening.
	hostIP := os.Getenv("HZN_DEV_HOST_IP")
	if hostIP == "" {
		hostIP = "localhost"
	}

	// Form the CSS URL
	url := fmt.Sprintf("http://%v:%v/api/v1/health", hostIP, getCSSPort())
	apiMsg := http.MethodGet + " " + url
	cliutils.Verbose(apiMsg)

	httpClient := cliutils.GetHTTPClient(0)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return errors.New(msgPrinter.Sprintf("unable to create get CSS status request, error %v", err))
	}
	req.SetBasicAuth(org+"/hzndev", "password")

	c := make(chan string, 1)

	go func() {
		resp, err := httpClient.Do(req)
		if err != nil {
			c <- err.Error()
		}
		respCode := 0
		if resp != nil {
			respCode = resp.StatusCode
		}
		c <- fmt.Sprintf("%d", respCode)
	}()

	for {
		select {
		case httpCodeString := <-c:
			httpCode, err := strconv.Atoi(httpCodeString)
			cliutils.Verbose(msgPrinter.Sprintf("Received HTTP code: %d", httpCode))

			if err == nil {
				if httpCode == 200 {
					return nil
				}
			}
		case <-time.After(time.Duration(timeout) * time.Second):
			return fmt.Errorf(msgPrinter.Sprintf("CSS status is not ready after waiting %d seconds, timeout", timeout))
		}

	}

}
