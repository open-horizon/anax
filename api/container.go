package api

import (
	"errors"
	"fmt"
	dockerclient "github.com/fsouza/go-dockerclient"
	"github.com/open-horizon/anax/cutil"
)

// Get docker container metadata from the docker API for workload containers
func GetWorkloadContainers(dockerEndpoint string, agreementId string) ([]dockerclient.APIContainers, error) {
	if client, err := dockerclient.NewClient(dockerEndpoint); err != nil {
		return nil, errors.New(fmt.Sprintf("unable to create docker client from %v, error %v", dockerEndpoint, err))
	} else {
		opts := dockerclient.ListContainersOptions{
			All: true,
		}

		if containers, err := client.ListContainers(opts); err != nil {
			return nil, errors.New(fmt.Sprintf("unable to list docker containers from %v, error %v", dockerEndpoint, err))
		} else {
			ret := make([]dockerclient.APIContainers, 0, 10)

			for _, c := range containers {
				if _, exists := c.Labels["network.bluehorizon.colonus.service_name"]; exists {
					if _, exists := c.Labels["network.bluehorizon.colonus.infrastructure"]; !exists {
						// If we are filtering by agreement id, then check the agreement id label
						if agreementId != "" && (c.Labels["network.bluehorizon.colonus.agreement_id"] != agreementId) {
							continue
						}
						ret = append(ret, c)
					}
				}
			}
			return ret, nil
		}
	}
}

// Get docker container metadata from the docker API for microservice containers
func GetMicroserviceContainer(dockerEndpoint string, mURL string, mVersion string, mInstanceId string) ([]dockerclient.APIContainers, error) {
	if client, err := dockerclient.NewClient(dockerEndpoint); err != nil {
		return nil, errors.New(fmt.Sprintf("unable to create docker client from %v, error %v", dockerEndpoint, err))
	} else {
		opts := dockerclient.ListContainersOptions{
			All: true,
		}

		if containers, err := client.ListContainers(opts); err != nil {
			return nil, errors.New(fmt.Sprintf("unable to list docker containers from %v, error %v", dockerEndpoint, err))
		} else {
			ret := make([]dockerclient.APIContainers, 0, 10)

			// Iterate through containers looking for the infrastructure container with an agreement id that
			// matches the microservice instance.
			for _, c := range containers {
				if _, exists := c.Labels["network.bluehorizon.colonus.infrastructure"]; exists {
					if agid, exists := c.Labels["network.bluehorizon.colonus.agreement_id"]; exists {
						name := cutil.MakeMSInstanceKey(mURL, mVersion, mInstanceId)
						if agid == name {
							ret = append(ret, c)
						}
					}
				}
			}
			return ret, nil
		}
	}
}
