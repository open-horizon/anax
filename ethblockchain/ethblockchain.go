package ethblockchain

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/go-solidity/contract_api"
	"os"
	"strconv"
	"time"
)

type BaseContracts struct {
	Directory  *contract_api.SolidityContract
	Agreements *contract_api.SolidityContract
	Metering   *contract_api.SolidityContract
}

func InitBaseContracts(account string, gethUrl string, directoryAddress string) (*BaseContracts, error) {

	var err error

	// Using input directory adrdess, get the device registry address
	dir := contract_api.SolidityContractFactory("directory")
	dir.Set_skip_eventlistener()
	dir.Set_contract_address(directoryAddress)
	if _, err := dir.Load_contract(account, gethUrl); err != nil {
		glog.Errorf("Error loading directory contract: %v", err)
		return nil, err
	}

	ver, _ := strconv.Atoi(os.Getenv("CMTN_DIRECTORY_VERSION"))

	agreementAddress := "0x0000000000000000000000000000000000000000"
	for agreementAddress == "0x0000000000000000000000000000000000000000" {
		agreementAddress, err = getContractAddress(dir, "agreements", ver)
		if err != nil {
			glog.Errorf("Error finding Agreements contract address: %v\n", err)
			panic(err)
		}
		time.Sleep(1 * time.Second)
	}

	ag := contract_api.SolidityContractFactory("agreements")
	ag.Set_skip_eventlistener()
	ag.Set_contract_address(agreementAddress)
	if _, err := ag.Load_contract(account, gethUrl); err != nil {
		glog.Errorf("Error loading agreement contract: %v", err)
		return nil, err
	}

	meteringAddress := "0x0000000000000000000000000000000000000000"
	for meteringAddress == "0x0000000000000000000000000000000000000000" {
		meteringAddress, err = getContractAddress(dir, "metering", ver)
		if err != nil {
			glog.Errorf("Error finding Metering contract address: %v\n", err)
			panic(err)
		}
		time.Sleep(1 * time.Second)
	}

	m := contract_api.SolidityContractFactory("metering")
	m.Set_skip_eventlistener()
	m.Set_contract_address(meteringAddress)
	if _, err := m.Load_contract(account, gethUrl); err != nil {
		glog.Errorf("Error loading metering contract: %v", err)
		return nil, err
	}

	return &BaseContracts{
		Directory:  dir,
		Agreements: ag,
		Metering:   m,
	}, nil

}

// Construct a parameter array for passing params into solidity contract methods.
func ContractParam(key interface{}) []interface{} {
	param := make([]interface{}, 0, 10)
	param = append(param, key)

	return param
}

func getContractAddress(dir *contract_api.SolidityContract, contract string, version int) (string, error) {
	p := make([]interface{}, 0, 10)
	p = append(p, contract)
	p = append(p, version)
	if draddr, err := dir.Invoke_method("get_entry_by_version", p); err != nil {
		glog.Errorf("Could not find %v in directory: %v\n", contract, err)
		return "", err
	} else {
		return draddr.(string), nil
	}
}

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
