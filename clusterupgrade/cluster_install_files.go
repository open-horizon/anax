package clusterupgrade

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/nodemanagement"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

const DOCKER_MANIFEST_FILE = "manifest.json"

// ----------------config file----------------
func ReadAgentConfigFile(filename string) (map[string]string, error) {
	configInMap := make(map[string]string)

	if len(filename) == 0 {
		return configInMap, nil
	}

	file, err := os.Open(filename)
	if err != nil {
		glog.Errorf(fmt.Sprintf("Failed to get read agent config %v: %v", filename, err))
		return configInMap, err
	}
	defer file.Close()

	sc := bufio.NewScanner(file)
	for sc.Scan() {
		line := sc.Text()
		if len(line) == 0 {
			continue
		}
		if keyValue := strings.Split(line, "="); len(keyValue) != 2 {
			return configInMap, fmt.Errorf(fmt.Sprintf("failed to parse content in agent config file %v", filename))
		} else {
			glog.V(5).Infof(cuwlog(fmt.Sprintf("get %v=%v", keyValue[0], keyValue[1])))
			configInMap[keyValue[0]] = keyValue[1]
		}
	}

	if err = sc.Err(); err != nil {
		glog.Errorf(fmt.Sprintf("Failed to get scan agent config %v: %v", filename, err))
	}

	return configInMap, err
}

// ----------------cert file----------------
func ReadAgentCertFile(filename string) ([]byte, error) {
	certFile, err := ioutil.ReadFile(filename)
	if err != nil {
		return make([]byte, 0), err
	}
	glog.V(5).Infof(cuwlog(fmt.Sprintf("get cert content %v", string(certFile))))
	return certFile, nil
}

// TrustNewCert adds the icp cert file to be trusted in calls made by the given http client
func TrustNewCert(httpClient *http.Client, certPath string) error {
	if certPath != "" {
		icpCert, err := ioutil.ReadFile(certPath)
		if err != nil {
			return err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(icpCert)

		transport := httpClient.Transport.(*http.Transport)
		transport.TLSClientConfig.RootCAs = caCertPool

	}
	return nil
}

func ValidateConfigAndCert(exchangeURL string, certPath string) error {
	httpClient := cliutils.GetHTTPClient(config.HTTPRequestTimeoutS)

	if err := TrustNewCert(httpClient, certPath); err != nil {
		return err
	}

	// get retry count and retry interval from env
	maxRetries, retryInterval, err := cliutils.GetHttpRetryParameters(5, 2)
	if err != nil {
		return err
	}

	retryCount := 0

	// make a call to exchangeURL/admin/version
	url := exchangeURL + "/admin/version"
	glog.Infof(cuwlog(fmt.Sprintf("Making call to %v", url)))

	for {
		retryCount++
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			glog.Errorf(fmt.Sprintf("Failed to create request, error was %v", err))
			return err
		}
		req.Close = true

		resp, err := httpClient.Do(req)
		if exchange.IsTransportError(resp, err) {
			http_status := ""
			if resp != nil {
				http_status = resp.Status
				if resp.Body != nil {
					resp.Body.Close()
				}
			}
			if retryCount <= maxRetries {
				glog.Infof(cuwlog(fmt.Sprintf("Encountered HTTP error: %v calling exchange REST API %v. HTTP status: %v. Will retry.", err, url, http_status)))
				// retry for network tranport errors
				time.Sleep(time.Duration(retryInterval) * time.Second)
				continue
			} else {
				glog.Errorf(fmt.Sprintf("Out of retry when calling exchange REST API %v, error was %v", url, err))
				return err
			}
		} else if err != nil {
			return err
		} else {
			return nil
		}
	}

}

// ----------------status.json file----------------
func createNMPStatusFile(workDir string, status string) error {
	fileName := path.Join(workDir, nodemanagement.STATUS_FILE_NAME)
	glog.Infof(cuwlog(fmt.Sprintf("Creating status.json file at %v", fileName)))

	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		glog.Infof(cuwlog(fmt.Sprintf("Work dir %v does not exist, create it...", workDir)))
		if err = os.MkdirAll(workDir, 755); err != nil {
			glog.Infof(cuwlog(fmt.Sprintf("Failed to create dir %v, err: %v", workDir, err)))
			return err
		}
	}

	startTime := time.Unix(time.Now().Unix(), 0).Format(time.RFC3339)
	agentUpgradeStatus := &exchangecommon.AgentUpgradePolicyStatus{
		Status:          status,
		K8S:             &exchangecommon.K8SResourcesStatus{},
		ActualStartTime: startTime,
		CompletionTime:  "",
		ErrorMessage:    "",
	}

	statusFile := exchangecommon.NodeManagementPolicyStatus{
		AgentUpgrade: agentUpgradeStatus,
	}

	if statusFileByte, err := json.Marshal(statusFile); err != nil {
		glog.Infof(cuwlog(fmt.Sprintf("Failed to marshal to status file, err: %v", err)))
		return err
	} else if err := ioutil.WriteFile(fileName, statusFileByte, 0755); err != nil {
		glog.Infof(cuwlog(fmt.Sprintf("Failed marshal to status file, err: %v", err)))
		return err
	}

	return nil
}

func setNMPStatusInStatusFile(workDir string, status string) error {
	fileName := path.Join(workDir, nodemanagement.STATUS_FILE_NAME)

	jsonByte, err := ioutil.ReadFile(fileName)
	if err != nil {
		return err
	}

	var statusFile exchangecommon.NodeManagementPolicyStatus
	if err := json.Unmarshal(jsonByte, &statusFile); err != nil {
		return err
	}

	statusFile.AgentUpgrade.Status = status
	updatedJsonByte, err := json.Marshal(statusFile)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(fileName, updatedJsonByte, 0755)
	return err
}

func setResourceNeedChangeInStatusFile(workDir string, resourceName string, needChange bool) error {
	fileName := path.Join(workDir, nodemanagement.STATUS_FILE_NAME)
	jsonByte, err := ioutil.ReadFile(fileName)
	if err != nil {
		return err
	}

	var statusFile exchangecommon.NodeManagementPolicyStatus
	if err := json.Unmarshal(jsonByte, &statusFile); err != nil {
		return err
	}

	switch resourceName {
	case RESOURCE_CONFIGMAP:
		statusFile.AgentUpgrade.K8S.ConfigMap.NeedChange = needChange
	case RESOURCE_SECRET:
		statusFile.AgentUpgrade.K8S.Secret.NeedChange = needChange
	case RESOURCE_IMAGE_VERSION:
		statusFile.AgentUpgrade.K8S.ImageVersion.NeedChange = needChange
	default:
		glog.Errorf(cuwlog(fmt.Sprintf("Unsupported resource type to set k8s status %v", resourceName)))
		return fmt.Errorf("unsupported resource type to set k8s needChange status %v", resourceName)
	}

	updatedJsonByte, err := json.Marshal(statusFile)
	if err != nil {
		return err
	}

	if err = ioutil.WriteFile(fileName, updatedJsonByte, 0755); err != nil {
		return err
	}
	glog.V(3).Infof(cuwlog(fmt.Sprintf("%v.needChange is set to %v in status file", resourceName, needChange)))
	return nil
}

func setResourceUpdatedInStatusFile(workDir string, resourceName string, updated bool) error {
	fileName := path.Join(workDir, nodemanagement.STATUS_FILE_NAME)
	jsonByte, err := ioutil.ReadFile(fileName)
	if err != nil {
		return err
	}

	var statusFile exchangecommon.NodeManagementPolicyStatus
	if err := json.Unmarshal(jsonByte, &statusFile); err != nil {
		return err
	}

	switch resourceName {
	case RESOURCE_CONFIGMAP:
		statusFile.AgentUpgrade.K8S.ConfigMap.Updated = updated
	case RESOURCE_SECRET:
		statusFile.AgentUpgrade.K8S.Secret.Updated = updated
	case RESOURCE_IMAGE_VERSION:
		statusFile.AgentUpgrade.K8S.ImageVersion.Updated = updated
	default:
		glog.Errorf(cuwlog(fmt.Sprintf("Unsupported resource type to set k8s status %v", resourceName)))
		return fmt.Errorf("unsupported resource type to set k8s updated status %v", resourceName)
	}

	updatedJsonByte, err := json.Marshal(statusFile)
	if err != nil {
		return err
	}

	if err = ioutil.WriteFile(fileName, updatedJsonByte, 0755); err != nil {
		return err
	}
	glog.V(3).Infof(cuwlog(fmt.Sprintf("%v.updated is set to %v in status file", resourceName, updated)))
	return nil
}

func setImageInfoInStatusFile(workDir string, from string, to string) error {
	statusFile, err := getStatusFromFile(workDir)
	if err != nil {
		return err
	}

	statusFile.AgentUpgrade.K8S.ImageVersion.From = from
	statusFile.AgentUpgrade.K8S.ImageVersion.To = to

	updatedJsonByte, err := json.Marshal(statusFile)
	if err != nil {
		return err
	}

	fileName := path.Join(workDir, nodemanagement.STATUS_FILE_NAME)
	err = ioutil.WriteFile(fileName, updatedJsonByte, 0755)
	return err
}

func getStatusFromFile(workDir string) (*exchangecommon.NodeManagementPolicyStatus, error) {
	fileName := path.Join(workDir, nodemanagement.STATUS_FILE_NAME)
	jsonByte, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	var statusFile exchangecommon.NodeManagementPolicyStatus
	if err := json.Unmarshal(jsonByte, &statusFile); err != nil {
		return nil, err
	}

	return &statusFile, err
}

func checkResourceNeedChange(workDir string) (bool, bool, bool, error) {
	if statusFile, err := getStatusFromFile(workDir); err != nil {
		return false, false, false, err
	} else {
		configNeedChange := statusFile.AgentUpgrade.K8S.ConfigMap.NeedChange
		secretNeedChange := statusFile.AgentUpgrade.K8S.Secret.NeedChange
		imageVersionNeedChange := statusFile.AgentUpgrade.K8S.ImageVersion.NeedChange
		return configNeedChange, secretNeedChange, imageVersionNeedChange, nil
	}
}

func setErrorMessageInStatusFile(workDir string, statusToSet string, errorMessage string) error {
	statusFile, err := getStatusFromFile(workDir)
	if err != nil {
		return err
	}
	statusFile.AgentUpgrade.ErrorMessage = errorMessage
	statusFile.AgentUpgrade.Status = statusToSet

	updatedJsonByte, err := json.Marshal(statusFile)
	if err != nil {
		return err
	}
	fileName := path.Join(workDir, nodemanagement.STATUS_FILE_NAME)
	err = ioutil.WriteFile(fileName, updatedJsonByte, 0755)
	return err
}

//----------------image tar.gz file----------------
/*
In side amd64_anax_k8s.tar.gz: check "manifest.json" file:
[
  {
    "Config": "952b3a2d2a06ddb5fdd97f2f032428d02d2c34941a2cc1c7a4c31c2140d56717.json",
    "RepoTags": [
      "hyc-edge-team-staging-docker-local.artifactory.swg-devops.com/amd64_anax_k8s:2.30.0-689"
    ],
    "Layers": [
      "f2161b5022c944b534b2b23409f363772df6c939e69ac7c2239aefec1f6218be/layer.tar",
      ....
      "362f62605291a69eda7391093b37a335359fb41b34775b38876af26a605d1da8/layer.tar"
    ]
  }
]

repositories file:
{
	"hyc-edge-team-staging-docker-local.artifactory.swg-devops.com/amd64_anax_k8s":{
		"2.30.0-689":"362f62605291a69eda7391093b37a335359fb41b34775b38876af26a605d1da8"
	}
}
*/

type DockerManifestObject struct {
	Config   string   `json:"Config"`
	RepoTags []string `json:"RepoTags"`
	Layers   []string `json:"Layers"`
}

var dockerManifestData []DockerManifestObject

// getImageTagFromManifestFile returns Image full tag, version tag, error
func getImageTagFromManifestFile(manifestFolder string) (string, string, error) {
	fileName := path.Join(manifestFolder, DOCKER_MANIFEST_FILE)
	f, err := os.Open(fileName)
	if err != nil {
		return "", "", err
	}
	fileInfo, err := f.Stat()
	if err != nil {
		return "", "", err
	}

	size := fileInfo.Size()
	jsonByte, err := ioutil.ReadFile(fileName)
	if err != nil {
		return "", "", err
	}

	if err := json.Unmarshal(jsonByte[:size], &dockerManifestData); err != nil {
		return "", "", err
	}

	if len(dockerManifestData) == 1 && len(dockerManifestData[0].RepoTags) == 1 {
		// repoFullTag is "hyc-edge-team-staging-docker-local.artifactory.swg-devops.com(:optionalPort)/amd64_anax_k8s:2.30.0-689"
		repoFullTag := dockerManifestData[0].RepoTags[0]
		parts := strings.Split(repoFullTag, ":")
		if len(parts) > 0 {
			repoTag := parts[len(parts)-1] //2.30.0-689
			return repoFullTag, repoTag, nil
		}
		return "", "", fmt.Errorf("failed to get image tag from %v", repoFullTag)
	}
	return "", "", fmt.Errorf("failed to get RepoTags from docker manifest file, docker manifest: %v", dockerManifestData)
}

func getAgentTarballFromGzip(tarGZFilePath, imageTarballPath string) error {
	reader, err := os.Open(tarGZFilePath)
	if err != nil {
		glog.Errorf(cuwlog(fmt.Sprintf("Failed to open %v, error was: %v", tarGZFilePath, err)))
		return err
	}
	defer reader.Close()

	uncompressStream, err := gzip.NewReader(reader)
	if err != nil {
		glog.Errorf(cuwlog(fmt.Sprintf("Failed to create new reader during decompression: %v", err)))
		return err
	}
	defer uncompressStream.Close()

	tarfile, err := os.Create(imageTarballPath)
	if err != nil {
		return err
	}
	defer tarfile.Close()

	_, err = io.Copy(tarfile, uncompressStream)
	return err
}

func extractImageManifest(tarballPath, targetFolder string) error {
	reader, err := os.Open(tarballPath)
	if err != nil {
		glog.Errorf(cuwlog(fmt.Sprintf("Failed to open %v, error was: %v", tarballPath, err)))
		return err
	}
	defer reader.Close()

	// create the target folder if it is not exist
	if _, err := os.Stat(targetFolder); err != nil {
		if err := os.MkdirAll(targetFolder, 0755); err != nil {
			return err
		}
	}

	tarReader := tar.NewReader(reader)
	for {
		header, err := tarReader.Next()
		switch {
		case err == io.EOF:
			return nil

		case err != nil:
			return err

		case header == nil:
			continue
		}

		target := path.Join(targetFolder, header.Name)
		switch header.Typeflag {
		// if it's a manifest file, create it
		case tar.TypeReg:
			if header.Name == DOCKER_MANIFEST_FILE {
				f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
				if err != nil {
					return err
				}
				if _, err := io.Copy(f, tarReader); err != nil {
					f.Close()
					return err
				}
				f.Close()
			}
		}
	}
}

func imageExistInRemoteRegistry(fullTag string, versiontTag string, kc authn.Keychain) bool {
	repo := fullTag
	if idx := strings.Index(fullTag, ":"); idx != -1 {
		repo = fullTag[:idx]
	}
	allTags, err := crane.ListTags(repo, crane.WithAuthFromKeychain(kc))
	glog.Infof(cuwlog(fmt.Sprintf("all tags under repo %v are: %v", repo, allTags)))
	if err != nil {
		return false
	}
	for _, t := range allTags {
		if t == versiontTag {
			return true
		}
	}
	return false
}
