package blockchain

import (
	"fmt"
	"os"
	"github.com/open-horizon/go-solidity/contract_api"
)

const dirVersionEnvvarName = "CMTN_DIRECTORY_VERSION"

func address(name string, directoryContract *contract_api.SolidityContract) (string, error) {
	if dirVersion := os.Getenv(dirVersionEnvvarName); dirVersion == "" {
		return "", fmt.Errorf("Unspecified but required envar: %v, unable to load contract", dirVersionEnvvarName)
	} else {
		dirVersion := ContractParam(name)
		dirVersion = append(dirVersion, os.Getenv("CMTN_DIRECTORY_VERSION"))

		if addr, err := directoryContract.Invoke_method("get_entry_by_version", dirVersion); err != nil {
			return "", fmt.Errorf("Could not find %v address in directory: %v\n", addr, err)
		} else {
			return addr.(string), nil
		}
	}
}

func ContractInDirectory(gethURL string, name string, deviceOwner string, directoryContract *contract_api.SolidityContract) (*contract_api.SolidityContract, error) {
	contract := contract_api.SolidityContractFactory(name)
	contract.Set_skip_eventlistener()

	if _, err := contract.Load_contract(deviceOwner, gethURL); err != nil {
		return nil, fmt.Errorf("Could not load %v contract: %v\n", name, err)
	} else if addr, err := address(name, directoryContract); err != nil {
		return nil, err
	} else {
		contract.Set_contract_address(addr)
		return contract, err
	}
}

func (w *BlockchainWorker) deviceContract(gethURL string, contractAddress string) (*contract_api.SolidityContract, error) {

	contract := contract_api.SolidityContractFactory("container_executor")
	contract.Set_skip_eventlistener() // use polling on getters

	if _, err := contract.Load_contract(w.Config.BlockchainAccountId, gethURL); err != nil {
		return nil, fmt.Errorf("Could not load contract for this account: %v\n", err)
	} else {
		contract.Set_contract_address(contractAddress)
		return contract, nil
	}
}

// TODO: consolidate all of these contract factories
func DirectoryContract(gethURL string, deviceOwner string, directoryAddress string) (*contract_api.SolidityContract, error) {
	dirc := contract_api.SolidityContractFactory("directory")
	dirc.Set_skip_eventlistener()

	if _, err := dirc.Load_contract(deviceOwner, gethURL); err != nil {
		return nil, fmt.Errorf("Could not load directory contract: %v\n", err)
	} else {
		dirc.Set_contract_address(directoryAddress)
		return dirc, nil
	}
}
