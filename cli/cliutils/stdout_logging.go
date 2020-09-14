package cliutils

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/open-horizon/anax/i18n"
)

func LogMac(instanceId string, tailing bool) {
	msgPrinter := i18n.GetMessagePrinter()
	dockerCommand := "docker logs $(docker ps -q --filter name=" + instanceId + ")"
	if tailing {
		dockerCommand = "docker logs -f $(docker ps -q --filter name=" + instanceId + ")"
	}
	fmt.Print(dockerCommand)
	fmt.Print("\n")
	cmd := exec.Command("/bin/sh", "-c", dockerCommand)
	cmdReader, err := cmd.StdoutPipe()
	if err != nil {
		Fatal(EXEC_CMD_ERROR, msgPrinter.Sprintf("Error creating StdoutPipe for command: %v", err))
	}
	// Assign a single pipe to Command.Stdout and Command.Stderr
	cmd.Stderr = cmd.Stdout
	// Combine stdout and stderr to single reader to be able to print messages in correct order.
	scanner := bufio.NewScanner(cmdReader)
	// Goroutine to print Stdout and Stderr while Docker logs command is running
	go func() {
		// scanner.Scan() == scanner.EOF go func() will end.
		for scanner.Scan() {
			msg := scanner.Text()
			fmt.Println(msg)
		}
	}()
	err = cmd.Start()
	if err != nil {
		Fatal(EXEC_CMD_ERROR, msgPrinter.Sprintf("Error starting command: %v", err))
	}
	err = cmd.Wait()
	if err != nil {
		Fatal(EXEC_CMD_ERROR, msgPrinter.Sprintf("Error waiting for command: %v", err))
	}
	return
}

func LogLinux(instanceId string, tailing bool) {
	msgPrinter := i18n.GetMessagePrinter()
	// Determine the system log file based on linux distribution
	sysLogPath := "/var/log/messages"
	_, err := os.Stat("/etc/redhat-release")
	if os.IsNotExist(err) {
		_, err := os.Stat("/etc/centos-release")
		if os.IsNotExist(err) {
			sysLogPath = "/var/log/syslog"
		}
	}
	// The requested service is running, so grab the records from syslog for this service.
	file, err := os.Open(sysLogPath)
	if err != nil {
		Fatal(NOT_FOUND, msgPrinter.Sprintf("%v could not be opened or does not exist: %v", sysLogPath, err))
	}
	defer file.Close()
	// Check file stats and capture the current size of the file if we will be tailing it.
	var file_size int64
	if tailing {
		fi, err := file.Stat()
		if err != nil {
			Fatal(NOT_FOUND, msgPrinter.Sprintf("%v could not get stats: %v", sysLogPath, err))
		}
		file_size = fi.Size()
	}
	// Setup a file reader
	reader := bufio.NewReader(file)
	// Start reading records. The syslog could be rotated while we're tailing it. Log rotation occurs when
	// the current syslog file reaches its maximum size. When this happens, the current syslog is copied
	// to another file and a new (empty) syslog file is created. The only way we can tell that a log
	// rotation happened is when the size of the file gets smaller as we are reading it.
	for {
		// Get a record (delimited by EOL) from syslog.
		if line, err := reader.ReadString('\n'); err != nil {
			// Any error we get back, even EOF, is treated the same if we are not tailing. Just return to the caller.
			if !tailing {
				return
			}
			// When we're tailing and we hit EOF, briefly sleep to allow more records to appear in syslog.
			if err == io.EOF {
				time.Sleep(1 * time.Second)
			} else {
				// If the error is not EOF then we assume the error is due to log rotation so we silently
				// ignore the error and keep trying.
				Verbose(msgPrinter.Sprintf("Error reading from %v: %v", sysLogPath, err))
			}
		} else if strings.Contains(line, "workload-"+instanceId) {
			// If the requested service id is in the current syslog record, display it.
			fmt.Print(string(line))
		}
		// Re-check syslog file size via stats in case syslog was logrotated.
		// If were tailing and there was a non-EOF error, we will always come here.
		if tailing {
			fi_new, err := os.Stat(sysLogPath)
			if err != nil {
				Verbose(msgPrinter.Sprintf("Unable to state %v: %v", sysLogPath, err))
				time.Sleep(1 * time.Second)
				continue
			}
			new_file_size := fi_new.Size()
			// If syslog is smaller than the last time we checked, then a log rotation has occurred.
			if new_file_size >= file_size {
				file_size = new_file_size
			} else {
				// Log rotation has occurred. Re-open the new syslog file and capture the current size.
				file.Close()
				file, err = os.Open(sysLogPath)
				if err != nil {
					Fatal(NOT_FOUND, msgPrinter.Sprintf("%v could not be opened or does not exist: %v", sysLogPath, err))
				}
				defer file.Close()
				// Setup a new reader on the new file.
				reader = bufio.NewReader(file)
				fi, err := file.Stat()
				if err != nil {
					Fatal(NOT_FOUND, msgPrinter.Sprintf("%v could not get stats: %v", sysLogPath, err))
				}
				file_size = fi.Size()
			}
		}
	}
}
