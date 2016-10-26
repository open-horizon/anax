// +build integration

package container

import (
	"archive/tar"
	"bytes"
	"flag"
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	gwhisper "github.com/open-horizon/go-whisper"
	"io"
	"net/url"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func contentFromTar(fname string, in *bytes.Buffer) (*bytes.Buffer, error) {
	tr := tar.NewReader(in)
	buf := new(bytes.Buffer)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of tar archive
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Name == fname {
			if _, err := io.Copy(buf, tr); err != nil {
				return nil, err
			}
		}
	}

	return buf, nil
}

func init() {
	flag.Set("alsologtostderr", "true")
	flag.Set("v", "7")
	// no need to parse flags, that's done by test framework
}

func tConfig() *config.Config {
	workloadStorageDir := "/tmp/workload_storage"

	if err := os.MkdirAll(workloadStorageDir, 777); err != nil {
		panic(err)
	}

	return &config.Config{
		DockerEndpoint:    "unix:///var/run/docker.sock",
		DefaultCPUSet:     "0-1",
		WorkloadROStorage: workloadStorageDir,
	}
}

func tWorker(config *config.Config) *ContainerWorker {
	return NewContainerWorker(config)
}

func tMsg(messages chan events.Message, expectedEvent events.EventId, t *testing.T) *ContainerMessage {
	// block on this read
	msg := <-messages

	if msg == nil {
		t.Log("Message is nil")
	} else {
		t.Logf("msg: %v", msg)
	}

	switch msg.(type) {

	case *ContainerMessage:
		m, _ := msg.(*ContainerMessage)
		if m.Event().Id == expectedEvent {
			t.Logf("m: %v", m)
			return m
		} else {
			t.Errorf("Execution failed. Original message: %v, type: %T; ContainerMessage asserted: %v", msg, msg, m)
			return nil
		}

	default:
		t.Errorf("%v", msg)
		return nil
	}
}

// try hard to clean up; report failures if expectClean is true (means logic under test should've left env clean and failed to)
func tClean(t *testing.T, tName string, worker *ContainerWorker, setupVerification func(*docker.APIContainers, *docker.Container) error, expectClean bool) {
	t.Logf("Cleaning up resources for %v", tName)

	containers, err := worker.client.ListContainers(docker.ListContainersOptions{All: true})
	if err != nil {
		t.Logf("Error inspecting containers for cleanup: %v", err)
	}

	var verifyErr error
	cleanFail := false

	for _, c := range containers {
		conDetail, err := worker.client.InspectContainer(c.ID)
		if err != nil {
			t.Logf("Error inspecting container for validation: %v", err)
		}

		t.Logf("Executing setup verification on %v", conDetail.Name)
		verifyErr = setupVerification(&c, conDetail)
	}

	// loop again after verification loop
	for _, c := range containers {
		for _, name := range c.Names {
			if strings.Contains(name, tName) {
				cleanFail = true
				if expectClean {
					t.Logf("Logic didn't clean up container %v as expected", name)
				}

				if err := worker.client.RemoveContainer(docker.RemoveContainerOptions{ID: c.ID, RemoveVolumes: true, Force: true}); err != nil {
					t.Logf("Error removing container during cleanup: %v", err)
				}
			}
		}
	}

	// now hose networks with same tag

	networks, err := worker.client.ListNetworks()
	if err != nil {
		t.Errorf("Error retrieving network list: %v", networks)
	}

	// TODO: handle
	os.RemoveAll(worker.Config.WorkloadROStorage + "/" + tName)

	for _, net := range networks {
		if strings.Contains(net.Name, tName) {
			cleanFail = true
			t.Logf("Logic didn't clean up network %v on its own (this is not necessarily an error)", net.Name)

			t.Logf("Test system attempting to remove network: %v", net.Name)
			if err := worker.client.RemoveNetwork(net.Name); err != nil {
				t.Errorf("Error removing network during cleanup: %v. Error: %v", net, err)
			}
		}
	}

	if verifyErr != nil {
		t.Error(verifyErr)
	}

	if expectClean && cleanFail {
		t.Error("Logic failed to clean up after itself as test expected")
	}
}

func tConnectivity(t *testing.T, client *docker.Client, container *docker.APIContainers) bool {
	t.Logf("Checking connectivity of %v", container.Names)

	start := time.Now().Unix()
	// this validation mechanism is not as cool as a socket listener from the test runner, evaluate if the latter is necessary
	// read /tmp/cping_success.stamp; wait if it's not written yet
	for {
		// max wait for network to get set up
		if time.Now().Unix()-start > 10 {
			t.Logf("Timed-out waiting for %v to report success connecting to other container over bridge", container.Names)
			break
		}

		fname := "cping_success.stamp"
		buf := new(bytes.Buffer)
		opt := docker.DownloadFromContainerOptions{
			OutputStream:      buf,
			Path:              path.Join("/tmp", fname),
			InactivityTimeout: 2 * time.Second,
		}

		if err := client.DownloadFromContainer(container.ID, opt); err == nil {

			// evaluate data
			stampB, err := contentFromTar(fname, buf)
			if err == nil {
				stamp := strings.Trim((*stampB).String(), "\r\n ")
				if stamp != "" {
					t.Logf("Success connectivity stamp val: %v read from container: %v", stamp, container.Names)
					return true
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	return false
}

func pickImage() string {
	if runtime.GOARCH == "arm" {
		return "armhf/alpine:3.3"
	} else {
		return "alpine:3.3"
	}
}

func commonPatterned(t *testing.T, agreementId string, tFn func(worker *ContainerWorker, env map[string]string, agreementId string), deployment string) {

	// used to name stuff for easy teardown
	namePrefix := "container-int-test"

	config := tConfig()
	worker := tWorker(config)

	var myAgreementId string

	if agreementId != "" {
		myAgreementId = agreementId
	} else {
		myAgreementId = fmt.Sprintf("ctest-%v", time.Now().UnixNano())
	}

	imageName := pickImage()

	var myDeployment string
	if deployment == "" {
		// the commands write newest success to fs; test validator should wait a time to allow success then exit
		// note that the names used for networking are container_name.myAgreementId, this is the safest way to do name resolution with Docker's DNS
		myDeployment = fmt.Sprintf("{\"services\": {\"%v-someserviceA\": {\"image\": \"%v\", \"command\": [\"/bin/sh\",\"-c\",\"while true; do ping -c1 %v > /dev/null 2>&1 && echo $(date +%%s) > /tmp/cping_success.stamp; sleep 0.5; done\"] }, \"%v-someserviceB\": { \"image\": \"%v\", \"privileged\": true, \"command\": [\"/bin/sh\",\"-c\",\"while true; do ping -c1 %v > /dev/null 2>&1 && echo $(date +%%s) > /tmp/cping_success.stamp; sleep 0.5; done\"] }}}", namePrefix, imageName, fmt.Sprintf("%v%v.%v", namePrefix, "-someserviceB", myAgreementId), namePrefix, imageName, fmt.Sprintf("%v%v.%v", namePrefix, "-someserviceA", myAgreementId))
	} else {
		myDeployment = deployment
	}

	env := map[string]string{
		"MTN_AGREEMENTID": myAgreementId,
		"MTN_RAM":         "64",
	}

	cmd := worker.NewContainerConfigureCommand([]string{}, &events.AgreementLaunchContext{
		Protocol:             "Citizen Scientist",
		AgreementId:          myAgreementId,
		Configure:            gwhisper.NewConfigure("", url.URL{}, nil, nil, myDeployment, "", ""),
		ConfigureRaw:         []byte("someRawConfigureData"),
		EnvironmentAdditions: &env,
	})

	// load this command
	worker.Commands <- cmd

	tFn(worker, env, myAgreementId)
}

func Test_resourcesCreate_failLoad(t *testing.T) {
	thisTest := func(worker *ContainerWorker, env map[string]string, agreementId string) {
		defer tClean(t, agreementId, worker, func(container *docker.APIContainers, containerDetail *docker.Container) error { return nil }, false)

		// fire the msg off
		tMsg(worker.Messages, events.EXECUTION_FAILED, t)
	}

	agreementId := fmt.Sprintf("ctest-%v", time.Now().UnixNano())
	pattern := `
  {
    "services": {
      "netspeed5": {
        "image": "summit.hovitos.engineering/x86/netspeed5:volcanostaging"
      },
      "pitcherd": {
        "image": "summit.hovitos.engineering/x86/pitcherd:volcano",
        "environment": [
          "MTN_CATCHERS=https://catcher.staging.bluehorizon.hovitos.engineering:443",
          "MTN_PITCHERD_PORT=8081"
        ]
      }
    }
	}
  `

	config := tConfig()
	config.TorrentDir = "/somethingfake"
	config.WorkloadROStorage = "/tmp"
	worker := tWorker(config)

	protocol := "Citizen Scientist"

	env := map[string]string{
		"MTN_CONTRACT":    protocol,
		"MTN_AGREEMENTID": agreementId,
		"MTN_RAM":         "64",
	}

	dl, _ := url.Parse("https://images.bluehorizon.network/f3a39c78edf78d415ff7e634e396d8e34b1656a3.torrent")

	// launch setup
	cmd := worker.NewContainerConfigureCommand([]string{
		"6fcc9d89326a42d48fa596e9f61dba5730b7a20e.tar.gz",
		"aef3b0fe6536092014cb29e9723ccc661d321d35.tar.gz",
	}, &events.AgreementLaunchContext{
		Protocol:             "Citizen Scientist",
		AgreementId:          agreementId,
		Configure:            gwhisper.NewConfigure("", *dl, nil, nil, pattern, "wpDdJ60JG1MvThKLMRX0eJf6/LHGUes79FDypYCOkgDAmA96BsREKpEHzl3OVM15z1vop6mpkLH5ka6vvbG0xJBYzZQl9HvyCSA7oJ/dQOqodjy2CySNWmzlFC842QXhrZO9yZxHZX0EcaPr2BdGu9p/9q17LzH9BcBYmBo7dZNqKSqkphErdqc1BOGSnjGlk/FfwnQGZM5SFz8mXa3ZW1/8yQ7w9/vvjTpcyB/X0Rv8qy0hfN0LKUfjfsZJ6O/aij0RkQ0w5ioGorGawOzQGvijs17KN8qfyVNn6QGqa03d4+e0mEQalhG9xsZKWSviSY92ifdSpBs7DohevyYMfT2mCRafP4lF2luu61Ho3pBQPEUjVEhvWch6b0FbsiH4iVcIVFTR+7SZhcv6oVwLDawvdT4aDo6Q1JhEuUrLMJhs6fb9q0cHl8SBpkPfcua33F4XDRCJoiYwTj3a8TtEKfaGmMDuIQq/5mJI8DSdCKasKDitLYFTE4z7+i2uKYlmXD1tzC2hNIWgdgAIXg0meQymWPqNIxDoo+pzTgDvv+tQ9usDRAvd+aYIDcf7IXAlAomtg7GE4v8KTCJHkM1aHVKE1ZCy0yI5uEWQoce9v8DukyZVwRTxfSX83F8Y/zfhxneSAeGkHHKO1PyFp82/fQDRyWhaSDdqO1uFPFvfbBU=", ""),
		ConfigureRaw:         []byte("someRawConfigureData"),
		EnvironmentAdditions: &env,
	})

	// load this command
	worker.Commands <- cmd

	thisTest(worker, env, agreementId)
}

// tests creation of a networked container pattern; does not test all features of container creation, that is left to a less complicated test
func Test_resourcesCreate_patterned(t *testing.T) {
	thisTest := func(worker *ContainerWorker, env map[string]string, agreementId string) {

		setupVerification := func(container *docker.APIContainers, containerDetail *docker.Container) error {
			if container.Labels["network.bluehorizon.colonus.agreement_id"] == agreementId {

				if _, present := container.Labels["network.bluehorizon.colonus.service_name"]; !present {
					return fmt.Errorf("service_name label not set on workload container: %v", container.Labels)
				}

				if containerDetail.HostConfig.CPUSetCPUs != "0-1" {
					return fmt.Errorf("Wrong CPUSet on running container: %v. Entire HostConfig: %v", containerDetail.HostConfig.CPUSetCPUs, containerDetail.HostConfig)
				}

				if containerDetail.HostConfig.LogConfig.Type != "syslog" || !strings.HasPrefix(containerDetail.HostConfig.LogConfig.Config["tag"], "workload-") {
					return fmt.Errorf("missing tagged syslog logging config")
				}

				if containerDetail.HostConfig.RestartPolicy != docker.AlwaysRestart() {
					return fmt.Errorf("restart policy wrong for test container: %v", containerDetail)
				}

				ramMb, _ := strconv.ParseInt(env["MTN_RAM"], 10, 64)
				if containerDetail.HostConfig.Memory != ramMb*1024*1024 {
					return fmt.Errorf("RAM not set correctly")
				}

				//if container.Names[0] == tName {
				//	if containerDetail.HostConfig.Privileged {
				//		return fmt.Errorf("container %v should not be privileged but is", container.Names[0])
				//	}

				//} else if container.Names[0] == tName2 {
				//	if !containerDetail.HostConfig.Privileged {
				//		return fmt.Errorf("container %v should be privileged but is not", container.Names[0])
				//	}
				//}

				if !tConnectivity(t, worker.client, container) {
					return fmt.Errorf("container connectivity test failed for %v", container.Names)
				}
			}
			return nil
		}

		defer tClean(t, agreementId, worker, setupVerification, false)

		// fire the msg off
		tMsg(worker.Messages, events.EXECUTION_BEGUN, t)
	}

	// launch test
	commonPatterned(t, "", thisTest, "")
}

func Test_resourcesRemove(t *testing.T) {

	var createMsg *ContainerMessage
	var w *ContainerWorker

	setup := func(worker *ContainerWorker, env map[string]string, agreementId string) {
		w = worker // gank that ref

		// capture event message
		createMsg = tMsg(worker.Messages, events.EXECUTION_BEGUN, t)
	}

	// launch setup
	commonPatterned(t, "", setup, "")

	defer tClean(t, createMsg.AgreementId, w, func(container *docker.APIContainers, containerDetail *docker.Container) error { return nil }, true)

	cmd := w.NewContainerShutdownCommand(createMsg.Protocol, createMsg.AgreementId, createMsg.Deployment, []string{})
	w.Commands <- cmd
	tMsg(w.Messages, events.PATTERN_DESTROYED, t)
}

func Test_resourcesCreate_shared(t *testing.T) {
	var w *ContainerWorker

	p1AgreementId := fmt.Sprintf("ctest-%v", time.Now().UnixNano())

	createMsgs := make([]*ContainerMessage, 0) // can verify this
	setup := func(worker *ContainerWorker, env map[string]string, agreementId string) {
		w = worker // gank that ref

		// capture event message
		createMsgs = append(createMsgs, tMsg(worker.Messages, events.EXECUTION_BEGUN, t))
	}

	imageName := pickImage()

	// all pings should succeed except the one by D
	pattern1 := `
	{
		"service_pattern": {
			"shared": {
				"singleton": ["container-int-test-culex"]
			}
		},
		"services": {
			"container-int-test-culex": {
				"image": "%v",
				"command": ["/bin/sh","-c","while true; do ping -c1 localhost > /dev/null 2>&1 && echo $(date +%%s) > /tmp/cping_success.stamp; sleep 0.5; done"]
			},
			"container-int-test-someServiceC": {
				"image": "%v",
				"command": ["/bin/sh","-c","while true; do ping -c1 container-int-test-someServiceD > /dev/null 2>&1 && echo $(date +%%s) > /tmp/cping_success.stamp; sleep 0.5; done"]
			},
			"container-int-test-someServiceD": {
				"image": "%v",
				"command": ["/bin/sh","-c","while true; do ping -c1 container-int-test-someServiceE > /dev/null 2>&1 && echo $(date +%%s) > /tmp/cping_success.stamp; sleep 0.5; done"],
				"privileged": true,
				"environment": [
          "MTN_MQTT_TOKEN=ZZbrT4ON5rYzoBi7H1VK3Ak9n0Fwjcod",
	        "SLEEP_INTERVAL=20"
				]
			}
		}
	}
	`

	// all pings should succeed
	pattern2 := `
		{
			"service_pattern": {
				"shared": {
					"singleton": ["container-int-test-culex"]
				}
			},
			"services": {
				"container-int-test-culex": {
					"image": "%v",
					"command": ["/bin/sh","-c","while true; do ping -c1 localhost > /dev/null 2>&1 && echo $(date +%%s) > /tmp/cping_success.stamp; sleep 0.5; done"]
				},
				"container-int-test-someServiceE": {
					"image": "%v",
					"command": ["/bin/sh","-c","while true; do ping -c1 container-int-test-culex > /dev/null 2>&1 && echo $(date +%%s) > /tmp/cping_success.stamp; sleep 0.5; done"],
					"environment": [
						"MTN_ROO=zxcvnwmernqw154",
						"BOO=1234"
					]
				},
				"container-int-test-someServiceF": {
					"image": "%v",
					"command": ["/bin/sh","-c","while true; do ping -c1 images.bluehorizon.network > /dev/null 2>&1 && echo $(date +%%s) > /tmp/cping_success.stamp; sleep 0.5; done"],
					"network_isolation": {
						"outbound_permit_only": ["4.2.2.2", "198.60.52.64/26"]
					},
					"ports": [
						{
							"localhost_only": false,
							"port_and_protocol": "8080/tcp"
						}
					],
					"environment": [
						"MTN_GOO=asdfj1n541kjh5klhas;lkdfj",
						"zoo=1"
					]
				}
			}
		}
	`

	p2AgreementId := fmt.Sprintf("ctest-%v", time.Now().UnixNano())

	setupVerify := func(worker *ContainerWorker, env map[string]string, agreementId string) {

		// capture event message
		createMsgs = append(createMsgs, tMsg(worker.Messages, events.EXECUTION_BEGUN, t))

		cons, err := worker.client.ListContainers(docker.ListContainersOptions{})
		if err != nil {
			t.Error(err)
		}

		for _, con := range cons {
			conAg := con.Labels["network.bluehorizon.colonus.agreement_id"]

			if conAg == p2AgreementId || conAg == p1AgreementId {

				connectivity := tConnectivity(t, worker.client, &con)

				// D isn't supposed to have connectivity
				if con.Labels["network.bluehorizon.colonus.service_name"] == "container-int-test-someServiceD" {
					if connectivity {
						t.Errorf("container %v isn't supposed to have connectivity but it does", con.Names)
					}
				} else {
					if !connectivity {
						t.Errorf("container %v doesn't have connectivity but it should", con.Names)
					}
				}
			}
		}
	}
	commonPatterned(t, p1AgreementId, setup, fmt.Sprintf(pattern1, imageName, imageName, imageName))
	commonPatterned(t, p2AgreementId, setupVerify, fmt.Sprintf(pattern2, imageName, imageName, imageName))

	defer tClean(t, p1AgreementId, w, func(container *docker.APIContainers, containerDetail *docker.Container) error { return nil }, true)
	defer tClean(t, p2AgreementId, w, func(container *docker.APIContainers, containerDetail *docker.Container) error { return nil }, true)

	// do the shutdown
	for _, createMsg := range createMsgs {
		sCmd := w.NewContainerShutdownCommand(createMsg.Protocol, createMsg.AgreementId, createMsg.Deployment, []string{})
		w.Commands <- sCmd
		tMsg(w.Messages, events.PATTERN_DESTROYED, t)
	}
}
