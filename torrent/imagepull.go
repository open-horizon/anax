package torrent

import (
	docker "github.com/fsouza/go-dockerclient"
	"github.com/open-horizon/horizon-pkg-fetch/fetcherrors"
	"strings"

	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/containermessage"
	"github.com/open-horizon/anax/cutil"
	"os"
	"time"
)

const (
	pullAttemptDelayS = 15

	maxPullAttempts = 3
)

// read the given docker file and get the auths
func dockerCredsFromConfigFile(configFilePath string) (*docker.AuthConfigurations, error) {

	f, err := os.Open(configFilePath)
	defer f.Close()
	if err != nil {
		return nil, err
	}

	auths, err := docker.NewAuthConfigurations(f)
	if err != nil {
		return nil, err
	}

	return auths, nil
}

// read the docker file and append the image auths from the docker file to the given map
func authDockerFile(config config.Config, authConfigs map[string][]docker.AuthConfiguration) error {
	// auth from creds file
	file_name := ""
	if config.DockerCredFilePath != "" {
		file_name = config.DockerCredFilePath
	} else {
		// if the config does not exist, use default provided the default file is there
		default_cred_fn := "/root/.docker/config.json"
		if _, err := os.Stat(default_cred_fn); err == nil {
			file_name = default_cred_fn
		}
	}

	if file_name != "" {
		glog.V(5).Infof("Using auth config file: %v", file_name)
		authFromFile, err := dockerCredsFromConfigFile(file_name)
		if err != nil {
			glog.Errorf("Failed to read creds file %v. Error: %v", file_name, err)
		} else {
			// do not overwrite incoming authconfigs entries, only augment them
			for _, v := range authFromFile.Configs {
				authConfigs = AppendDockerAuth(authConfigs, v)
			}
		}
	}
	return nil
}

func pullImageFromRepos(config config.Config, authConfigs map[string][]docker.AuthConfiguration, client *docker.Client, skipPartFetchFn *func(repotag string) (bool, error), deploymentDesc *containermessage.DeploymentDescription) error {

	// append docker auth from docker file
	authDockerFile(config, authConfigs)

	// TODO: can we fetch in parallel with the docker client? If so, lift pattern from https://github.com/open-horizon/horizon-pkg-fetch/blob/master/fetch.go#L350
	for name, service := range deploymentDesc.Services {

		glog.V(3).Infof("Pulling image %v for service %v", service.Image, name)

		var opts docker.PullImageOptions

		domain, path, tag, digest := cutil.ParseDockerImagePath(service.Image)
		if path == "" {
			glog.Errorf("Invalid image name format specified: %v", service.Image)
			return fmt.Errorf("Invalid image name format specified: %v", service.Image)
		}

		// the image name format is [[repo][:port]/][somedir/]image[:tag][@digest].
		// tag and digest do not contain '/'
		if digest != "" {
			// this is the case where image repo digest is used, just put whole name there
			opts = docker.PullImageOptions{
				Repository: service.Image,
			}
		} else {
			// this is case where image name:tag is used. The image repo may contain :, image tag itself cannot contain : or /.
			// These are valid formats:
			//  repo/a/b:tag
			//  repo:port/a/b:tag
			//  repo:port/a/b

			var repo string
			if domain == "" {
				repo = path
			} else {
				repo = fmt.Sprintf("%v/%v", domain, path)
			}

			if tag == "" {
				tag = "latest"
			}

			// TODO: check the on-disk image to make sure it still verifies
			// N.B. It's possible to specify an outputstream here which means we could fetch a docker image and hash it, check the sig like we used to
			opts = docker.PullImageOptions{
				Repository: repo,
				Tag:        tag,
			}
		}

		var err error
		if domain == "" {
			err = pullSingleImageFromRepo(client, opts, docker.AuthConfiguration{})
		} else if auth_array, ok := authConfigs[domain]; !ok {
			err = pullSingleImageFromRepo(client, opts, docker.AuthConfiguration{})
		} else {
			for i, auth := range auth_array {
				err = pullSingleImageFromRepo(client, opts, auth)
				if err == nil {
					break
				} else if i < len(auth_array)-1 {
					glog.V(5).Infof("Docker image pull(s) failed for service %v docker image %v with auth name %v. Error: %v. Try next auth.", name, service.Image, auth.Username, err)
				}
			}
		}
		if err != nil {
			glog.Errorf("Docker image pull(s) failed for docker image %v. Error: %v.", service.Image, err)
			return err
		} else {
			glog.V(3).Infof("Succeeded fetching image %v for service", service.Image, name)
		}
	}

	return nil
}

//  This function try maxPullAttempts times to pull the image from the repo. It exits out imediately if there is auth error.
func pullSingleImageFromRepo(client *docker.Client, opts docker.PullImageOptions, auth docker.AuthConfiguration) error {
	glog.V(5).Infof("Pulling image %v with auth name %v.", opts, auth.Username)

	var pullAttempts int

	for pullAttempts <= maxPullAttempts {
		if err := client.PullImage(opts, auth); err == nil {
			return nil
		} else {
			pullAttempts++

			// no need to try more times if it is auth error
			switch err.(type) {
			case *docker.Error:
				dErr := err.(*docker.Error)
				if strings.Contains(dErr.Message, "cred") {
					msg := fmt.Sprintf("Aborting fetch of Docker image %v.", opts.Repository)
					return fetcherrors.PkgSourceFetchAuthError{Msg: msg, InternalError: dErr}
				}
			}

			if pullAttempts != maxPullAttempts {
				glog.V(5).Infof("Waiting %d seconds before retry. Error: %v", pullAttemptDelayS, err)
				time.Sleep(pullAttemptDelayS * time.Second)
			} else {
				msg := fmt.Sprintf("Max pull attempts reached (%d) for fetching Docker image %v.", pullAttempts, opts.Repository)

				switch err.(type) {
				case *docker.Error:
					glog.V(5).Infof(msg+"Docker client error occurred %v", err)
					return err

				default:
					glog.V(5).Infof(msg+"(Unknown error type, %T) Internal error of unidentifiable type: %v. Original: %v", err, msg, err)
					return err

				}
			}
		}
	}
	return nil
}
