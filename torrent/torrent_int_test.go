// +build ci

// N.B. !! In order for a test in this suite to succeed you must do the following config:
// - set the envvar HORIZON_TEST_DOCKER_CREDFILE_PATH to the location of a docker cred file path (like ~/.docker/config.json)
// - execute this test with docker permissions and a working docker instance

package torrent

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/rsapss-tool/sign"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// using a different key to verify Pkg than deployment
const validPkgSigningCert = `-----BEGIN CERTIFICATE-----
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
-----END CERTIFICATE-----`

// from rsapss-tool, not paired with other keys in this file; using an x509 cert to sign and verify the deployment description
var validTestPrivKey = `-----BEGIN RSA PRIVATE KEY-----
MIICXQIBAAKBgQDAzJUe8MrDpOFu8uJT2rKLo0pic0fksDny6RRszKeRF6uz8ewp
9zTox/ZcLAo7q/XRCos3LMxf7aoXdPY2livwmu7S0CvjcmnxOrZGtH7mwy+Ls5UK
WJ5nPZeZoWogMofQJymgpfXyVABm7AnIuA2hHQjmqFqpxcjFi2RLc6bhawIDAQAB
AoGBAIfIjc14sJURbmOBU7zS7aRCoIStxBhftLBLT0NA71LUZO0amMUFgZHgIrXP
nnVgKoPK9Tkqp9V3wK88hJr1MIPOE3Yi4CgHe8eQ8Q5Z62bb1kUa/yc3nn6MI/Uz
Kn6q7wIYjpSpFQHUNeJZJ3hrU6NfYJbiKVHe0n0ip5WkcjUBAkEA7xWP5cA2Dmra
bze9Thn9Twk+M4UEEEGUUAhkq3QKjTaTi2JTUjd6jue9TKSEcGCNd+rMXsiJ5ucX
EZPCjAphYQJBAM5wrVlybYUqPtBTyfdBsvKlVRpXDPekS0U5HoOHi6pYG8xiLFbG
McooADfvEzv2NTHzwozWJT0fx4Re9wMImksCQHuPezTT55v/4TAFcJKCoAVO05Sw
s+7q1YmfLNfnOuTMReiNQl6FSZO9dHm9tKyXWcWV1VVO8uYgnC17XdoeK0ECQHt4
PuXZn5Few/TbuFbu73Va1zyKxhGzLOW5FPv77Ne0HOQv727y2UKcjAzoK6vYRNac
gUa0qc8WG8Ga/sfMtGMCQQCMWudwltirtK4+U9G1phKiSZcew6O/BlMDM1UjjZQQ
nBKZcF0+H62TmtIIHvRm0wTq+nPPTtoEH8NrNwRZZ2hC
-----END RSA PRIVATE KEY-----`
var validTestDeploymentCert = `-----BEGIN CERTIFICATE-----
MIICHTCCAYagAwIBAgIUYqKtvgqzrCoAUi0aX6WViO/RpOYwDQYJKoZIhvcNAQEL
BQAwOjEeMBwGA1UEChMVUlNBUFNTIFRvb2wgdGVzdCBjZXJ0MRgwFgYDVQQDEw9k
ZXZlbG9wbWVudC1vbmUwHhcNMTcxMjAyMTk1ODMyWhcNMjcxMTMwMDc1ODMyWjA6
MR4wHAYDVQQKExVSU0FQU1MgVG9vbCB0ZXN0IGNlcnQxGDAWBgNVBAMTD2RldmVs
b3BtZW50LW9uZTCBnzANBgkqhkiG9w0BAQEFAAOBjQAwgYkCgYEAwMyVHvDKw6Th
bvLiU9qyi6NKYnNH5LA58ukUbMynkRers/HsKfc06Mf2XCwKO6v10QqLNyzMX+2q
F3T2NpYr8Jru0tAr43Jp8Tq2RrR+5sMvi7OVClieZz2XmaFqIDKH0CcpoKX18lQA
ZuwJyLgNoR0I5qhaqcXIxYtkS3Om4WsCAwEAAaMgMB4wDgYDVR0PAQH/BAQDAgeA
MAwGA1UdEwEB/wQCMAAwDQYJKoZIhvcNAQELBQADgYEAcs5DAT+frZfJsoSKEMOu
WJh0S/UVYC+InMv9iUnPF3f0KjVBXTE45GDG1zxY6SFLpOVskNp9mMkH9PLqDMrb
kWsF7xOtgBrzIaibDeEhhcQvvHb6Yct1bSgYxWpS1oGKicXA9PFyXxigUW2e8+DH
SoxItJkxfl2adAjY2DVzdhY=
-----END CERTIFICATE-----`

func init() {
	flag.Set("logtostderr", "true")
	flag.Set("v", "4")
	// no need to parse flags, that's done by test framework
}

func tConfig(t *testing.T, dir string) *config.HorizonConfig {
	workloadStorageDir := path.Join(dir, "workload_storage")
	if err := os.MkdirAll(workloadStorageDir, 0755); err != nil {
		panic(err)
	}

	torrentDir := path.Join(dir, "torrent_dir")
	if err := os.MkdirAll(torrentDir, 0755); err != nil {
		panic(err)
	}

	dockerCredFile := os.Getenv("HORIZON_TEST_DOCKER_CREDFILE_PATH")
	if dockerCredFile == "" {
		t.Fatalf("Suite setup failed: envvar HORIZON_TEST_DOCKER_CREDFILE_PATH not set (it must point to a docker config file with creds for summit.hovitos.engineering")
	} else {
		t.Logf("Using docker cred config file: %v (identified by envvar HORIZON_TEST_DOCKER_CREDFILE_PATH)", os.Getenv("HORIZON_TEST_DOCKER_CREDFILE_PATH"))
	}

	cfg := config.HorizonConfig{
		Edge: config.Config{
			DockerEndpoint:     "unix:///var/run/docker.sock",
			DockerCredFilePath: dockerCredFile,
			DefaultCPUSet:      "0-1",
			TorrentDir:         torrentDir,
			WorkloadROStorage:  workloadStorageDir,
			//		DockerCredFilePath: "/config.json",
			PublicKeyPath: path.Join(dir, "validpkgcert.pem"),
			// consistent with setup()'s dirs
			UserPublicKeyPath: path.Join(dir, "userkeys"),
		},
	}

	// now make collaborators instance and assign it to member in this config
	collaborators, err := config.NewCollaborators(cfg)
	if err != nil {
		return nil
	}

	cfg.Collaborators = *collaborators

	return &cfg
}

func setup(t *testing.T) (string, *bolt.DB, error) {
	dir, err := ioutil.TempDir("", "container-")
	if err != nil {
		return "", nil, err
	}

	db, err := bolt.Open(path.Join(dir, "anax-int.db"), 0600, &bolt.Options{Timeout: 10 * time.Second})
	if err != nil {
		return dir, nil, err
	}

	bxRegToken := os.Getenv("HORIZON_TEST_BX_DOCKER_REG_TOKEN")
	if bxRegToken == "" {
		t.Fatalf("Suite setup failed: envvar HORIZON_TEST_BX_DOCKER_REG_TOKEN not set (it must contain a token for the bluemix-hosted docker registry to authenticated when pulling images stored there)")
	} else {
		t.Logf("Using bx docker registry token: %v (identified by envvar HORIZON_TEST_BX_DOCKER_REG_TOKEN)", os.Getenv("HORIZON_TEST_BX_DOCKER_REG_TOKEN"))
	}

	tt := true
	ff := false

	// add the bluemix registry token as an attribute
	attr := &persistence.BXDockerRegistryAuthAttributes{
		Token: bxRegToken,
		Meta: &persistence.AttributeMeta{
			Id:          "bxauth",
			Label:       "bxauth",
			Type:        "BXDockerRegistryAuthAttributes",
			SensorUrls:  []string{"registry.ng.bluemix.net"},
			HostOnly:    &tt,
			Publishable: &ff,
		},
	}

	if _, err := persistence.SaveOrUpdateAttribute(db, attr, "", false); err != nil {
		t.Logf("error persisting bxauth: %v", err)
		panic(err)
	}

	err = ioutil.WriteFile(path.Join(dir, "validpkgcert.pem"), []byte(validPkgSigningCert), 0644)
	if err != nil {
		return dir, nil, err
	}

	certpath := path.Join(dir, "userkeys")
	if err := os.MkdirAll(certpath, 0755); err != nil {
		panic(err)
	}

	keypath := path.Join(dir, "private")
	if err := os.MkdirAll(keypath, 0755); err != nil {
		panic(err)
	}

	err = ioutil.WriteFile(path.Join(certpath, "validdepcert.pem"), []byte(validTestDeploymentCert), 0644)
	if err != nil {
		return dir, nil, err
	}

	err = ioutil.WriteFile(path.Join(keypath, "validprivatekey.pem"), []byte(validTestPrivKey), 0644)
	if err != nil {
		return dir, nil, err
	}

	return dir, db, nil
}

func tWorker(config *config.HorizonConfig, db *bolt.DB) *TorrentWorker {
	tw := NewTorrentWorker("tworker", config, db)
	return tw
}

func tMsg(messages chan events.Message, expectedEvent events.EventId, t *testing.T) *events.TorrentMessage {
	// block on this read
	msg := <-messages

	switch msg.(type) {
	case *events.TorrentMessage:
		m, _ := msg.(*events.TorrentMessage)
		if m.Event().Id == expectedEvent {
			t.Logf("m: %v", m)
			return m
		} else {
			t.Errorf("Execution failed. Original message: %v, type: %T; WorkloadMessage asserted: %v", msg, msg, m)
			return nil
		}
	default:
		t.Errorf("%v", msg)
		return nil
	}
}

func tCleanup(t *testing.T, worker *TorrentWorker, images []string) {

	t.Logf("Cleaning up: %v", images)
	for _, image := range images {
		err := worker.client.RemoveImage(image)

		if err != nil {
			t.Errorf("ERROR: cleanup failed for docker image, it was supposed to be there and perhaps wasn't. Expected to find: %v", image)
		}
	}
}

func Test_Torrent_Event_Suite(suite *testing.T) {
	dir, db, err := setup(suite)
	assert.Nil(suite, err)
	defer os.RemoveAll(dir)

	config := tConfig(suite, dir)
	worker := tWorker(config, db)
	env := make(map[string]string, 0)

	ur, _ := url.Parse("http://1DD40.http.tor01.cdn.softlayer.net/horizon-test-ci/4bf023c831cff7924378e79a4c51cd426b1ea442.json")
	resp, err := http.Get("http://1DD40.http.tor01.cdn.softlayer.net/horizon-test-ci/4bf023c831cff7924378e79a4c51cd426b1ea442.json.sig")
	assert.Nil(suite, err)
	assert.EqualValues(suite, http.StatusOK, resp.StatusCode)
	defer resp.Body.Close()

	sigBytes, err := ioutil.ReadAll(resp.Body)
	assert.Nil(suite, err)

	images := []string{
		"summit.hovitos.engineering/amd64/neo4j:3.3.1",               // 189MB
		"summit.hovitos.engineering/amd64/clojure:boot-2.7.2-alpine", // 147MB
		"ubuntu:yakkety",                                             // 107MB
		"registry.ng.bluemix.net/glendarling/x86/cpu:1.2.1",          // 9MB
	}

	var buf bytes.Buffer
	for ix, image := range images {
		repotag := strings.Split(image, ":")

		repo := repotag[0]
		var sname string
		if strings.Contains(repo, "/") {
			sname = strings.Split(repotag[0], "/")[len(repotag)]
		} else {
			sname = repo
		}

		buf.WriteString(fmt.Sprintf("\"%s\":{\"image\":\"%s\"}", sname, image))

		if ix != len(images)-1 {
			buf.WriteString(",")
		}
	}

	deployment := fmt.Sprintf("{\"services\":{%s}}", buf.String())

	dSig, err := sign.Input(path.Join(dir, "private", "validprivatekey.pem"), []byte(deployment))
	assert.Nil(suite, err)

	// N.B. the following tests use this suite setup; there is cleanup between each

	suite.Run("Torrent event with torrent url and signature causes Horizon Pkg pull", func(t *testing.T) {
		defer tCleanup(t, worker, images)

		cfg := events.NewContainerConfig(*ur, string(sigBytes), deployment, dSig, "", "")

		cmd := worker.NewFetchCommand(&events.ContainerLaunchContext{
			Configure:            *cfg,
			EnvironmentAdditions: &env,
			Blockchain:           events.BlockchainConfig{"", "", ""},
			Name:                 "Pkg fetch test",
		})

		worker.Commands <- cmd
		tMsg(worker.Messages(), events.IMAGE_FETCHED, t)

		// do it again to make sure the load skip behavior works

		cmdAgain := worker.NewFetchCommand(&events.ContainerLaunchContext{
			Configure:            *cfg,
			EnvironmentAdditions: &env,
			Blockchain:           events.BlockchainConfig{"", "", ""},
			Name:                 "Pkg fetch test 2 (a clone)",
		})

		worker.Commands <- cmdAgain
		// TODO: consider adding an event that distinguishes this case (already exists in docker images repo) from newly-pulled
		tMsg(worker.Messages(), events.IMAGE_FETCHED, t)
	})
	suite.Run("Torrent event without torrent url and signature causes Docker pull (w/ authentication)", func(t *testing.T) {
		defer tCleanup(t, worker, images)

		emptyUr, _ := url.Parse("")

		// N.B. empty torrent URL and empty torrent signature mean docker pull should be used
		cfg := events.NewContainerConfig(*emptyUr, "", deployment, dSig, "", "")

		cmd := worker.NewFetchCommand(&events.ContainerLaunchContext{
			Configure:            *cfg,
			EnvironmentAdditions: &env,
			Blockchain:           events.BlockchainConfig{"", "", ""},
			Name:                 "Authenticated docker pull test",
		})

		worker.Commands <- cmd
		tMsg(worker.Messages(), events.IMAGE_FETCHED, t)
	})
}
