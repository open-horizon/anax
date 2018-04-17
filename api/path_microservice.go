package api

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"sort"
)

// This API returns everything we know about microservices configured to and running on an
// edge node. It includes microservice definition metadata that is cached from the exchange,
// userInput variable config for each microservice, running containers for each microservice,
// and the state of each microservice as it is being managed by anax.
func FindMicroServicesForOutput(pm *policy.PolicyManager,
	db *bolt.DB,
	config *config.HorizonConfig) (*AllMicroservices, error) {

	// Get all the ms instances that we know about.
	msinsts, err := persistence.FindMicroserviceInstances(db, []persistence.MIFilter{})
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to read microservice instances, error %v", err))
	}

	msdefs, err := persistence.FindMicroserviceDefs(db, []persistence.MSFilter{})
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to read microservice definitions, error %v", err))
	}

	// Setup the output map keys and a sub-map for each one
	var archivedKey = "archived"
	var activeKey = "active"

	wrap := NewMicroserviceOutput()

	// wrap.Instances = make(map[string][]interface{}, 0)
	wrap.Instances[archivedKey] = make([]interface{}, 0, 5)
	wrap.Instances[activeKey] = make([]interface{}, 0, 5)

	// wrap[msdefKey] = make(map[string][]interface{}, 0)
	wrap.Definitions[archivedKey] = make([]interface{}, 0, 5)
	wrap.Definitions[activeKey] = make([]interface{}, 0, 5)

	// Iterate through each instance and generate the output object for each one.
	for _, msinst := range msinsts {
		if msinst.Archived {
			wrap.Instances[archivedKey] = append(wrap.Instances[archivedKey], *NewMicroserviceInstanceOutput(msinst, nil))
		} else {
			containers, err := GetMicroserviceContainer(config.Edge.DockerEndpoint, msinst.SpecRef, msinst.Version, msinst.InstanceId)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("unable to get docker container info, error %v", err))
			}
			wrap.Instances[activeKey] = append(wrap.Instances[activeKey], *NewMicroserviceInstanceOutput(msinst, &containers))
		}
	}

	// Iterate through each microserivce definition and dump it to the output directly.
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

	// Add the microservice config sub-object to the output
	cfg, err := FindMicroServiceConfigForOutput(pm, db)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to get microservice config, error %v", err))
	}

	wrap.Config = cfg["config"]

	return wrap, nil
}
