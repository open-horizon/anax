package main

import (
	"bytes"
	// "encoding/json"
	// "fmt"
	"errors"
	// "io"
	"io/ioutil"
	"os"
	"os/exec"
	// "runtime"
	"strings"
	"testing"
	// "time"
	"fmt"
)

// This file tests the bhgovconfig command. The command is used by a developer to build the policy and
// config file for a governor.

// Test the no action usage output to make sure it looks like usage output.
func Test_Usage(t *testing.T) {
	if out, err := exec.Command("bhgovconfig").Output(); err != nil {
		t.Error(err)
	} else if !strings.Contains(string(out), "Usage:") {
		t.Errorf("Expecting usage output, received %v", string(out))
	}
}

// Compare the actual output file with the expected output
func compareFiles(outputFilename, expectedFilename string, t *testing.T) {
	if outputFile, err := ioutil.ReadFile(outputFilename); err != nil {
		t.Errorf("Unable to open %v file, error: %v", outputFilename, err)
		// Read the file into it's own byte array
	} else if expectedFile, err := ioutil.ReadFile(expectedFilename); err != nil {
		t.Errorf("Unable to open %v file, error: %v", expectedFilename, err)
		// Compare the bytes of both files. If there is a difference, then we have a problem so a bunch
		// of diagnostics will be written out.
	} else if bytes.Compare(outputFile, expectedFile) != 0 {
		t.Errorf("Newly created %v file does not match %v file.", outputFilename, expectedFilename)
		// if err := ioutil.WriteFile("./test/new_governor.sls", out2, 0644); err != nil {
		//     t.Errorf("Unable to write ./test/new_governor.sls file, error: %v", err)
		// }
		for idx, val := range outputFile {
			if val == expectedFile[idx] {
				continue
			} else {
				t.Errorf("Found difference at index %v", idx)
				t.Errorf("bytes around diff in output   file: %v", string(outputFile[idx-10:idx+10]))
				t.Errorf("bytes around diff in expected file: %v", string(expectedFile[idx-10:idx+10]))
				break
			}
		}
	}
}

// Run a command with stdin and args, and return stdout, stderr
func runCmd(commandString, stdinFilename string, args ...string) ([]byte, []byte, error) {
	// For debug, build the full cmd string
	cmdStr := "cat " + stdinFilename + " | " + commandString
	for _, a := range args {
		cmdStr += " " + a
	}
	fmt.Printf("running: %v\n", cmdStr)

	// Create the command object with its args
	cmd := exec.Command(commandString, args...)
	if cmd == nil {
		return nil, nil, errors.New("Did not return a command object, returned nil")
	}
	// Create the std in pipe, this is how govconfig.json gets passed into the command
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, errors.New("Could not get Stdin pipe, error: " + err.Error())
	}
	// Create the stdout pipe to hold the output from the command
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, errors.New("Error retrieving output from command, error: " + err.Error())
	}
	// Create the stderr pipe to hold the errors from the command
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, errors.New("Error retrieving stderr from command, error: " + err.Error())
	}
	// Read the input file
	jInbytes, err := ioutil.ReadFile(stdinFilename)
	if err != nil {
		return nil, nil, errors.New("Unable to read " + stdinFilename + " file, error: " + err.Error())
	}
	// Start the command, which will block for input to std in
	err = cmd.Start()
	if err != nil {
		return nil, nil, errors.New("Unable to start command, error: " + err.Error())
	}
	// Send in the std in bytes
	_, err = stdin.Write(jInbytes)
	if err != nil {
		return nil, nil, errors.New("Unable to write to stdin of command, error: " + err.Error())
	}
	// Close std in so that the command will begin to execute
	err = stdin.Close()
	if err != nil {
		return nil, nil, errors.New("Unable to close stdin, error: " + err.Error())
	}
	err = error(nil)
	// Read the output from stdout and stderr into byte arrays
	// stdoutBytes, err := readPipe(stdout)
	stdoutBytes, err := ioutil.ReadAll(stdout)
	if err != nil {
		return nil, nil, errors.New("Error reading stdout, error: " + err.Error())
	}
	// stderrBytes, err := readPipe(stderr)
	stderrBytes, err := ioutil.ReadAll(stderr)
	if err != nil {
		return nil, nil, errors.New("Error reading stderr, error: " + err.Error())
	}
	// Now block waiting for the command to complete
	err = cmd.Wait()
	if err != nil {
		return stdoutBytes, stderrBytes, errors.New("Error waiting for command: " + err.Error())
	}

	return stdoutBytes, stderrBytes, error(nil)
}

/*
Test the building of the gov policy file and provider.config file based on known input.
The stdin json input comes from 1 of the input/*.json files, the generated output files are placed in /tmp,
and compared to the 2 expected output files in the dir named output.

If the output is not correct, then the point of difference between the two files will be found and displayed.
*/
// const templateDir = "./test/templates"
const outputDir = "/tmp/etc/provider-tremor"
const expectedDir = "./test/output"
const rootDir = "/tmp/root"

/*
Test quarks deployment pattern, sending data to neutron, and in dev mode.
*/
func Test_NeutronDevMode(t *testing.T) {
	inputJson := "./test/input/govconfig-neutron.json"
	policyName := "netspeed-arm.policy"
	configName := "provider.config"
	outputPolicy := outputDir + "/policy.d/" + policyName
	outputConfig := outputDir + "/" + configName
	expectedPolicy := expectedDir + "/netspeed-arm-neutron-devmode.policy"
	expectedConfig := expectedDir + "/provider-devmode.config"
	ethAccts := "1a2b3c,4d5e6f"

	// Make sure this file is gone. It might have been left around if the previous test run ended in error.
	if _, err := os.Stat(outputPolicy); !os.IsNotExist(err) {
		os.Remove(outputPolicy)
	}
	if _, err := os.Stat(outputConfig); !os.IsNotExist(err) {
		os.Remove(outputConfig)
	}

	// Run this cmd:  cat test/input/govconfig.json | go run bh-gov-config.go -d /tmp -t test/templates
	commandString := "bhgovconfig"

	// Create the command object with its args
	stdoutBytes, stderrBytes, err := runCmd(commandString, inputJson, "-d", outputDir, "-root-dir", rootDir, "-ethereum-accounts", ethAccts)
	compare := true
	if err != nil {
		t.Errorf("Error running command %v: %v", commandString, err)
		compare = false
	}

	t.Logf("stdout: %s", stdoutBytes)
	if len(stderrBytes) > 0 {
		t.Logf("stderr: %s", stderrBytes)
	}

	if compare {
		// Open the actual and expected the policy files
		compareFiles(outputPolicy, expectedPolicy, t)

		// Open the actual and expected the policy files
		compareFiles(outputConfig, expectedConfig, t)
	}
}

/*
Test quarks deployment pattern, sending data to iotf, and *not* in dev mode.
*/
// func Test_Iotf(t *testing.T) {
//     inputJson := "./test/input/govconfig-iotf.json"
//     policyName := "netspeed-amd64.policy"
//     configName := "provider.config"
//     outputPolicy := outputDir + "/policy.d/" + policyName
//     outputConfig := outputDir + "/" + configName
//     expectedPolicy := expectedDir + "/netspeed-amd64-iotf.policy"
//     expectedConfig := expectedDir + "/provider.config"

// Make sure this file is gone. It might have been left around if the previous test run ended in error.
// if _, err := os.Stat(outputPolicy); !os.IsNotExist(err) { os.Remove(outputPolicy) }
// if _, err := os.Stat(outputConfig); !os.IsNotExist(err) { os.Remove(outputConfig) }

// Run this cmd:  cat test/input/govconfig.json | go run bh-gov-config.go -d /tmp -t test/templates
// commandString := "bhgovconfig"

// Create the command object with its args
// stdoutBytes, stderrBytes, err := runCmd(commandString, inputJson, "-d", outputDir, "-root-dir", rootDir)
// compare := true
// if err != nil {
//     t.Errorf("Error running command %v: %v", commandString, err)
//     compare = false
// }

// t.Logf("stdout: %s", stdoutBytes)
// if len(stderrBytes) > 0 { t.Logf("stderr: %s", stderrBytes) }

// if compare {
// Open the actual and expected the policy files
// compareFiles(outputPolicy, expectedPolicy, t)

// Open the actual and expected the policy files
// compareFiles(outputConfig, expectedConfig, t)
//     }
// }

/*
Test quarks deployment pattern, sending data to iotf, and *not* in dev mode, with the new way to specify arch.
*/
func Test_Iotf(t *testing.T) {
	inputJson := "./test/input/govconfig-iotf.json"
	policyName := "netspeed-amd64.policy"
	configName := "provider.config"
	outputPolicy := outputDir + "/policy.d/" + policyName
	outputConfig := outputDir + "/" + configName
	expectedPolicy := expectedDir + "/netspeed-amd64-iotf.policy"
	expectedConfig := expectedDir + "/provider-devmode.config"

	// Make sure this file is gone. It might have been left around if the previous test run ended in error.
	if _, err := os.Stat(outputPolicy); !os.IsNotExist(err) {
		os.Remove(outputPolicy)
	}
	if _, err := os.Stat(outputConfig); !os.IsNotExist(err) {
		os.Remove(outputConfig)
	}

	// Run this cmd:  cat test/input/govconfig.json | go run bh-gov-config.go -d /tmp -t test/templates
	commandString := "bhgovconfig"

	// Create the command object with its args
	stdoutBytes, stderrBytes, err := runCmd(commandString, inputJson, "-d", outputDir, "-root-dir", rootDir)
	compare := true
	if err != nil {
		t.Errorf("Error running command %v: %v", commandString, err)
		compare = false
	}

	t.Logf("stdout: %s", stdoutBytes)
	if len(stderrBytes) > 0 {
		t.Logf("stderr: %s", stderrBytes)
	}

	if compare {
		// Open the actual and expected the policy files
		compareFiles(outputPolicy, expectedPolicy, t)

		// Open the actual and expected the policy files
		compareFiles(outputConfig, expectedConfig, t)
	}
}

/*
Test quarks deployment pattern for the cpu_temp poc, with broker host empty, and in dev mode.
*/
func Test_CpuTempDevMode(t *testing.T) {
	inputJson := "./test/input/govconfig-cputemp.json"
	policyName := "cpu_temp-arm.policy"
	configName := "provider.config"
	outputPolicy := outputDir + "/policy.d/" + policyName
	outputConfig := outputDir + "/" + configName
	expectedPolicy := expectedDir + "/cpu_temp-arm-devmode.policy"
	expectedConfig := expectedDir + "/provider-devmode.config"
	ethAccts := "1a2b3c,4d5e6f"

	// Make sure this file is gone. It might have been left around if the previous test run ended in error.
	if _, err := os.Stat(outputPolicy); !os.IsNotExist(err) {
		os.Remove(outputPolicy)
	}
	if _, err := os.Stat(outputConfig); !os.IsNotExist(err) {
		os.Remove(outputConfig)
	}

	// Run this cmd:  cat test/input/govconfig.json | go run bh-gov-config.go -d /tmp -t test/templates
	commandString := "bhgovconfig"

	// Create the command object with its args
	// stdoutBytes, stderrBytes, err := runCmd(commandString, inputJson, "-d", outputDir, "-root-dir", rootDir, "-ethereum-accounts", ethAccts, "-t", "./template-samples")
	stdoutBytes, stderrBytes, err := runCmd(commandString, inputJson, "-d", outputDir, "-root-dir", rootDir, "-ethereum-accounts", ethAccts)
	compare := true
	if err != nil {
		t.Errorf("Error running command %v: %v", commandString, err)
		compare = false
	}

	t.Logf("stdout: %s", stdoutBytes)
	if len(stderrBytes) > 0 {
		t.Logf("stderr: %s", stderrBytes)
	}

	if compare {
		// Open the actual and expected the policy files
		compareFiles(outputPolicy, expectedPolicy, t)

		// Open the actual and expected the policy files
		compareFiles(outputConfig, expectedConfig, t)
	}
}
