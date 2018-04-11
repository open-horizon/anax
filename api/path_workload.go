package api

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/config"
)

func FindWorkloadForOutput(db *bolt.DB, config *config.HorizonConfig) (*AllWorkloads, error) {

	work := NewWorkloadOutput()

	cfgs, err := FindWorkloadConfigForOutput(db)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to get workloadconfig objects, error %v", err))
	}

	work.Config = cfgs["config"]

	containers, err := GetWorkloadContainers(config.Edge.DockerEndpoint, "")
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to get running workload containers, error %v", err))
	}

	work.Containers = &containers

	return work, nil

}
