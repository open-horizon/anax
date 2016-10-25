package main

import (
    "bytes"
    // "encoding/json"
    // "fmt"
    "io/ioutil"
    "os"
    "os/exec"
    // "runtime"
    "strings"
    "testing"
    // "time"
)

// This file tests the yaml_payloads_edit command. The command is used by devops to modify
// the old payloads.json file when a new workload container is ready for deployment to
// devices in the system.

// Test the no action usage output to make sure it looks like usage output.
func Test_Usage(t *testing.T) {
    if out, err := exec.Command("yaml_payloads_edit").Output(); err != nil {
        t.Error(err)
    } else if !strings.Contains(string(out), "Usage:") {
        t.Errorf("Expecting usage output, received %v", string(out))
    }
}

// Test the inserttorrent action of the yanl_payloads_edit function. This is a complex test, where
// the point of the test is to verify that the command correct transforms the ./test/governor/sls
// file into the ./test/compare_governor.sls file. The input for the transform comes from the
// ./test/torrent.json file.
//
// If the transform is not correct, then the point of difference between the two files will be found
// and displayed. The newly generated file will also be saved to ./test/new_governor.sls. This is done so
// that the problem can be debugged.
//
// If the transform is successful, the newly generated file is not created, there is no need to create it.
func Test_Inserttorrent(t *testing.T) {

    // Make sure this file is gone. It might have been left around if the previous test run ended in error.
    if _, err := os.Stat("./test/new_governor.sls"); !os.IsNotExist(err) {
        os.Remove("./test/new_governor.sls")
    }

    // Here is the command that devops uses to insert new info into the governor.sls file:
    // cat ./test/torrent.json | yaml_payloads_edit inserttorrent ./test/governor.sls prod is_bandwidth_test_enabled:true,arch:arm > ./test/new_governor.sls"
    // The following code will invoke the same command through go APIs.
    commandString := "yaml_payloads_edit"
    args := make([]string, 0, 10)
    args = append(args, "inserttorrent")
    args = append(args, "./test/governor.sls")
    args = append(args, "prod")
    args = append(args, "is_bandwidth_test_enabled:true,arch:arm")

    // Create the command object with its args
    if cmd := exec.Command(commandString, args...); cmd == nil {
        t.Errorf("Did not return a command object, returned nil")
    // Create the std in pipe, this is how torrent.json gets passed into the command
    } else if stdin, err := cmd.StdinPipe(); err != nil {
        t.Errorf("Could not get Stdin pipe, error: %v", err)
    // Create the std out pipe, this is how the transformed file is returned from the command
    } else if outp, err := cmd.StdoutPipe(); err != nil {
        t.Errorf("Error retrieving output from command, error: %v", err)
    // Open the input file
    } else if jIn, err := os.Open("./test/torrent.json"); err != nil {
        t.Errorf("Unable to open ./test/torrent.json file, error: %v", err)
    // Read it into a byte array
    } else if jInbytes, err := ioutil.ReadAll(jIn); err != nil {
        t.Errorf("Unable to read ./test/torrent.json file, error: %v", err)
    // Start the command, which will block for input to std in
    } else if err := cmd.Start(); err != nil {
        t.Errorf("Unable to start command, error: %v", err)
    // Send in the std in bytes
    } else if _, err := stdin.Write(jInbytes); err != nil {
        t.Errorf("Unable to write to stdin of command, error: %v", err)
    // Close std in so that the command will begin to execute
    } else if err := stdin.Close(); err != nil {
        t.Errorf("Unable to close stdin, error: %v", err)
    } else {
        // Get a byte array big enough to hld the transformed file
        out := make([]byte, 35000, 35000)
        err := error(nil)
        var num int
        // Read the transformed output from std out into the byte array
        if num, err = outp.Read(out); err != nil {
            t.Errorf("Error reading output, error: %v byte %v", err, num)
        // Now block waiting for the command to complete
        } else if err := cmd.Wait(); err != nil {
            t.Errorf("Error waiting for command, error %v:", err)
        } else {
            // Create a second byte array that is only as big as the output from std out.
            // This effectively will truncate zero bytes from the end of the byte array.
            out2 := make([]byte, num, num)
            copy(out2, out)
            // Open the base line comparison file
            if compareSLS, err := os.Open("./test/compare_governor.sls"); err != nil {
                t.Errorf("Unable to open compare_governor.sls file, error: %v", err)
            // Read the file into it's own byte array
            } else if compbytes, err := ioutil.ReadAll(compareSLS); err != nil {
                t.Errorf("Unable to read compare_governor.sls file, error: %v", err)
            // Compare the bytes from the transform (std out) abd the base line file.
            // If there is a different, then we have a problem so a bunch of diagnostics
            // will be written out.
            } else if bytes.Compare(out2, compbytes) != 0 {
                t.Errorf("Newly created /test/new_governor.sls file does not match /test/compare_governor.sls file.")
                if err := ioutil.WriteFile("./test/new_governor.sls", out2, 0644); err != nil {
                    t.Errorf("Unable to write ./test/new_governor.sls file, error: %v", err)
                }
                for idx, val := range out2 {
                    if val == compbytes[idx] {
                        continue
                    } else {
                        t.Errorf("Found difference at index %v", idx)
                        t.Errorf("bytes around diff in new       file: %v", string(out2[idx-10:idx+10]))
                        t.Errorf("bytes around diff in base line file: %v", string(compbytes[idx-10:idx+10]))
                        break
                    }
                }
            }
        }
    }
}
