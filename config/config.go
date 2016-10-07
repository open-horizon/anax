package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	WorkloadROStorage    string
	TorrentDir           string
	APIListen            string
	DBPath               string
	GethURL              string
	DockerEndpoint       string
	DefaultCPUSet        string
	StaticWebContent     string
	PublicKeyPath        string
	CACertsPath          string
	MicropaymentEnforced bool

	// these Ids could be provided in config or discovered after startup by the system
	BlockchainAccountId        string
	BlockchainDirectoryAddress string
}

func Read(file string) (*Config, error) {

	if _, err := os.Stat(file); err != nil {
		return nil, fmt.Errorf("Config file not found: %s. Error: %v", file, err)
	}

	// attempt to parse config file
	path, err := os.Open(filepath.Clean(file))
	if err != nil {
		return nil, fmt.Errorf("Unable to read config file: %s. Error: %v", file, err)
	} else {
		// instantiate empty which will be filled
		config := Config{}

		err := json.NewDecoder(path).Decode(&config)
		if err != nil {
			return nil, fmt.Errorf("Unable to decode content of config file: %v", err)
		}

		// success at last!
		return &config, nil
	}
}
