package api

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	dockerclient "github.com/fsouza/go-dockerclient"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/container"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"sort"
)

// This API returns everything we know about services configured to and running on an
// edge node. It includes service definition metadata that is cached from the exchange,
// userInput variable config for each service, running containers for each service,
// and the state of each service as it is being managed by anax.
func FindServicesForOutput(pm *policy.PolicyManager,
	db *bolt.DB,
	config *config.HorizonConfig) (*AllServices, error) {

	// Get all the service instances from database.
	msinsts, err := persistence.GetAllMicroserviceInstances(db, true, true)
	if err != nil {
		return nil, err
	}

	// Get all the service definitions so that we can show them in the definitions section.
	msdefs, err := persistence.FindMicroserviceDefs(db, []persistence.MSFilter{})
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to read service definitions, error %v", err))
	}

	// Setup the output map keys and a sub-map for each one
	var archivedKey = "archived"
	var activeKey = "active"

	wrap := NewServiceOutput()

	wrap.Instances[archivedKey] = make([]*MicroserviceInstanceOutput, 0, 5)
	wrap.Instances[activeKey] = make([]*MicroserviceInstanceOutput, 0, 5)

	wrap.Definitions[archivedKey] = make([]persistence.MicroserviceDefinition, 0, 5)
	wrap.Definitions[activeKey] = make([]persistence.MicroserviceDefinition, 0, 5)

	// Iterate through each service instance from the ms database and generate the output object for each one.
	for _, msinst := range msinsts {
		mi, ok := msinst.(*persistence.MicroserviceInstance)

		// this should never happen because GetAllMicroserviceInstances converts all to MicroserviceInterface
		// object if the last parameter is true.
		if !ok {
			continue
		}

		if msinst.IsArchived() {
			wrap.Instances[archivedKey] = append(wrap.Instances[archivedKey], NewMicroserviceInstanceOutput(*mi, nil))
		} else {
			containers, err := GetMicroserviceContainers(config.Edge.DockerEndpoint, mi)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("unable to get docker container info, error %v", err))
			}
			wrap.Instances[activeKey] = append(wrap.Instances[activeKey], NewMicroserviceInstanceOutput(*mi, &containers))
		}
	}

	// Iterate through each serivce definition and dump it to the output directly.
	for _, msdef := range msdefs {
		if msdef.Archived {
			wrap.Definitions[archivedKey] = append(wrap.Definitions[archivedKey], msdef)
		} else {
			wrap.Definitions[activeKey] = append(wrap.Definitions[activeKey], msdef)
		}
	}

	// Sort the instance and definition info. The config info doesnt need to be sorted.
	sort.Sort(MicroserviceInstanceByMicroserviceDefId(wrap.Instances[activeKey]))
	sort.Sort(MicroserviceInstanceByCleanupStartTime(wrap.Instances[archivedKey]))
	sort.Sort(MicroserviceDefById(wrap.Definitions[activeKey]))
	sort.Sort(MicroserviceDefByUpgradeStartTime(wrap.Definitions[archivedKey]))

	// Add the service config sub-object to the output
	cfg, err := FindServiceConfigForOutput(pm, db)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to get service config, error %v", err))
	}

	wrap.Config = cfg["config"]

	return wrap, nil
}

// Get docker container metadata from the docker API for microservice containers
func GetMicroserviceContainers(dockerEndpoint string, msinst *persistence.MicroserviceInstance) ([]dockerclient.APIContainers, error) {
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

			// Iterate through containers looking for lable 'openhorizon.anax.agreement_id' that
			// matches the microservice instance key.
			for _, c := range containers {
				if agid, exists := c.Labels[container.LABEL_PREFIX+".agreement_id"]; exists {
					if agid == msinst.GetKey() {
						ret = append(ret, c)
					}
				}
			}
			return ret, nil
		}
	}
}
