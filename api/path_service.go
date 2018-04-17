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

// This API returns everything we know about services configured to and running on an
// edge node. It includes service definition metadata that is cached from the exchange,
// userInput variable config for each service, running containers for each service,
// and the state of each service as it is being managed by anax.
func FindServicesForOutput(pm *policy.PolicyManager,
	db *bolt.DB,
	config *config.HorizonConfig) (*AllServices, error) {

	// Get all the service instances that we know about from the dependent service database.
	msinsts, err := persistence.FindMicroserviceInstances(db, []persistence.MIFilter{})
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to read service instances, error %v", err))
	}

	// Get all the agreement (top-level) services so that we can display them in the instances section.
	agInsts, err := persistence.FindEstablishedAgreementsAllProtocols(db, policy.AllAgreementProtocols(), []persistence.EAFilter{})
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to read agreement services, error %v", err))
	}

	// Get all the service definitions so that we can show them in the definitions section.
	msdefs, err := persistence.FindMicroserviceDefs(db, []persistence.MSFilter{})
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to read mservice definitions, error %v", err))
	}

	// Setup the output map keys and a sub-map for each one
	var archivedKey = "archived"
	var activeKey = "active"

	wrap := NewServiceOutput()

	wrap.Instances[archivedKey] = make([]interface{}, 0, 5)
	wrap.Instances[activeKey] = make([]interface{}, 0, 5)

	wrap.Definitions[archivedKey] = make([]interface{}, 0, 5)
	wrap.Definitions[activeKey] = make([]interface{}, 0, 5)

	// Iterate through each service instance from the ms database and generate the output object for each one.
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

	// Iterate through each agreement service instance and generate the output object for each one.
	for _, agInst := range agInsts {
		if agInst.Archived {
			wrap.Instances[archivedKey] = append(wrap.Instances[archivedKey], *NewAgreementServiceInstanceOutput(&agInst, nil))
		} else {
			containers, err := GetWorkloadContainers(config.Edge.DockerEndpoint, agInst.CurrentAgreementId)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("unable to get docker container info, error %v", err))
			}
			wrap.Instances[activeKey] = append(wrap.Instances[activeKey], *NewAgreementServiceInstanceOutput(&agInst, &containers))
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
