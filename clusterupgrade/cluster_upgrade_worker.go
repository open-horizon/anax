package clusterupgrade

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/common"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/eventlog"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/nodemanagement"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/version"
	"github.com/open-horizon/anax/worker"
	"os"
	"path"
	"reflect"
	"strconv"
	"time"
)

const (
	AGENT_CONFIG_FILE  = "agent-install.cfg"
	AGENT_CERT_FILE    = "agent-install.crt"
	AGENT_IMAGE_TAR_GZ = "%v_anax_k8s.tar.gz"
	AGENT_IMAGE_TAR    = "%v_anax_k8s.tar"
	AGENT_IMAGE_NAME   = "%v_anax_k8s"
)

const (
	AGENT_CONFIGMAP               = "openhorizon-agent-config"
	AGENT_SECRET                  = "openhorizon-agent-secrets"
	AGENT_SERVICE_ACCOUNT         = "agent-service-account"
	AGENT_DEPLOYMENT              = "agent"
	AGENT_IMAGE_PULL_SECRETS_NAME = "registry-creds"
)

const (
	RESOURCE_CONFIGMAP     = "configmap"
	RESOURCE_SECRET        = "secret"
	RESOURCE_IMAGE_VERSION = "imageVersion"
)

const (
	DEFAULT_CERT_PATH                    = "/etc/default/cert/"
	DEFAULT_IMAGE_REGISTRY_IN_DEPLOYMENT = "__ImageRegistryHost__"
)

const (
	AGENT_NAMESPACE_ENV_NAME      = "AGENT_NAMESPACE"
	HZN_NAMESPACE_SCOPED_ENV_NAME = "HZN_NAMESPACE_SCOPED"
)

var AGENT_NAMESPACE string

var cuwlog = func(v interface{}) string {
	return fmt.Sprintf("Cluster upgrade worker: %v", v)
}

type ClusterUpgradeWorker struct {
	worker.BaseWorker
	db         *bolt.DB
	kubeClient *KubeClient
}

func NewClusterUpgradeWorker(name string, config *config.HorizonConfig, db *bolt.DB) *ClusterUpgradeWorker {
	kubeClient, err := NewKubeClient()
	if err != nil {
		glog.Errorf("Failed to instantiate kube Client for cluster upgrad worker: %v", err)
		panic("Unable to instantiate kube Client for cluster upgrade worker")
	}
	ec := getEC(config, db)

	worker := &ClusterUpgradeWorker{
		BaseWorker: worker.NewBaseWorker(name, config, ec),
		db:         db,
		kubeClient: kubeClient,
	}

	glog.Infof(cuwlog(fmt.Sprintf("Starting Cluster Upgrade Worker.")))
	worker.Start(worker, 0)
	return worker
}

func (w *ClusterUpgradeWorker) Initialize() bool {
	AGENT_NAMESPACE = os.Getenv("AGENT_NAMESPACE")
	glog.Infof(cuwlog(fmt.Sprintf("Agent namespace is %v", AGENT_NAMESPACE)))

	if dev, _ := persistence.FindExchangeDevice(w.db); dev != nil && dev.Config.State == persistence.CONFIGSTATE_CONFIGURED {
		baseWorkingDir := w.Config.Edge.GetNodeMgmtDirectory() // baseWorkingDir is: /var/horizon/nmp
		if err := w.syncOnInit(w.db, w.kubeClient, baseWorkingDir); err != nil {
			glog.Infof(cuwlog(fmt.Sprintf("Failed to sync up during Initialization of cluster upgrade worker, error:  %v", err)))
		}
	}
	return true
}

func (w *ClusterUpgradeWorker) syncOnInit(db *bolt.DB, kubeClient *KubeClient, baseWorkingDir string) error {
	glog.Infof(cuwlog(fmt.Sprintf("In syncOnInit, now FindInitiatedNMPStatuses")))
	if statuses, err := persistence.FindInitiatedNMPStatuses(db); err != nil {
		return fmt.Errorf("failed to find nmp statuses in the local db: %v", err)
	} else {
		glog.Infof(cuwlog(fmt.Sprintf("In syncOnInit, find %v status from db", len(statuses))))
		for name, status := range statuses {
			if status != nil {
				glog.Infof(cuwlog(fmt.Sprintf("Handling status %v during initialization", name)))
				workDir := path.Join(baseWorkingDir, name)
				if statusInStatusFile, err := checkDeploymentStatus(kubeClient, baseWorkingDir, name); err != nil {
					errMessage := fmt.Sprintf("Failed to check deployment status during syncOnInit for nmp: %v, error: %v", name, err)
					if err = setErrorMessageInStatusFile(workDir, exchangecommon.STATUS_FAILED_JOB, errMessage); err != nil {
						glog.Errorf(fmt.Sprintf("Failed to set error message (%v) for nmp: %v in the status file, error: %v", errMessage, name, err))
					}
				} else if statusInStatusFile == exchangecommon.STATUS_ROLLBACK_STARTED {
					if err = setNMPStatusInStatusFile(workDir, exchangecommon.STATUS_ROLLBACK_SUCCESSFUL); err != nil {
						glog.Errorf(fmt.Sprintf("Failed to set status to %v for nmp: %v in the status file, error: %v", exchangecommon.STATUS_ROLLBACK_SUCCESSFUL, name, err))
					}
				} else if statusInStatusFile == exchangecommon.STATUS_INITIATED {
					if err = setNMPStatusInStatusFile(workDir, exchangecommon.STATUS_SUCCESSFUL); err != nil {
						glog.Errorf(fmt.Sprintf("Failed to set status to %v for nmp: %v in the status file, error: %v", exchangecommon.STATUS_ROLLBACK_SUCCESSFUL, name, err))
					}
				}

				if err = w.collectStatus(baseWorkingDir, name, status); err != nil {
					glog.Errorf(cuwlog(fmt.Sprintf("Failed to collect status for nmp %v during initilization: %v", name, err)))
				}
			}
		}
	}
	return nil
}

func checkDeploymentStatus(kubeClient *KubeClient, baseWorkingDir string, nmpName string) (string, error) {
	glog.Infof(cuwlog(fmt.Sprintf("Checking deployment status for nmp: %v", nmpName)))
	workDir := path.Join(baseWorkingDir, nmpName)

	// check status in the status file, if status is "rollback started", return status, nil
	nmPolicyStatus, err := getStatusFromFile(workDir)
	if err != nil {
		return "", err
	}

	if nmPolicyStatus.AgentUpgrade.Status == exchangecommon.STATUS_ROLLBACK_STARTED || nmPolicyStatus.AgentUpgrade.Status == exchangecommon.STATUS_ROLLBACK_SUCCESSFUL || nmPolicyStatus.AgentUpgrade.Status == exchangecommon.STATUS_ROLLBACK_FAILED {
		glog.Infof(cuwlog(fmt.Sprintf("Status for %v in the status file is %v", nmpName, nmPolicyStatus.AgentUpgrade.Status)))
		return nmPolicyStatus.AgentUpgrade.Status, nil
	}

	configMapNeedChange, secretNeedChange, imageVerNeedChange, err := checkResourceNeedChange(workDir)
	if err != nil {
		return "", err
	}
	glog.Infof(cuwlog(fmt.Sprintf("Deployment Status checked for nmp %v in status file under dirctory: %v, configMapNeedChange: %v, secretNeedChange: %v, imageVerNeedChange: %v", nmpName, workDir, configMapNeedChange, secretNeedChange, imageVerNeedChange)))
	if configMapNeedChange {
		if configIsSame, _, _, err := checkAgentConfig(kubeClient, workDir); err != nil {
			glog.Errorf(cuwlog(fmt.Sprintf("Failed to check config for nmp %v during initializtion, error:  %v", nmpName, err)))
			return "", err
		} else if !configIsSame {
			return "", fmt.Errorf(fmt.Sprintf("agent configmap content doesn't match agent config for nmp %v", nmpName))
		}
		glog.Infof(cuwlog(fmt.Sprintf("Agent configmap matches agent config for nmp %v", nmpName)))
	}

	if secretNeedChange {
		if secretIsSame, _, _, err := checkAgentCert(kubeClient, workDir); err != nil {
			glog.Errorf(cuwlog(fmt.Sprintf("Failed to check cert for nmp %v during initializtion, error:  %v", nmpName, err)))
			return "", err
		} else if !secretIsSame {
			return "", fmt.Errorf(fmt.Sprintf("agent secret content doesn't match agent cert for nmp %v", nmpName))
		}
		glog.Infof(cuwlog(fmt.Sprintf("Agent secret matches agent cert for nmp %v", nmpName)))
	}

	if imageVerNeedChange {
		if imageVersionIsSame, err := checkAgentImageAgainstStatusFile(workDir); err != nil {
			glog.Errorf(cuwlog(fmt.Sprintf("Failed to check agent version during initializtion, error:  %v", err)))
			return "", err
		} else if !imageVersionIsSame {
			return "", fmt.Errorf(fmt.Sprintf("agent version doesn't match agent version in status file for nmp %v", nmpName))
		} else {
			glog.Infof(cuwlog(fmt.Sprintf("Agent version matches agent version in status file for nmp %v", nmpName)))
			glog.Infof(cuwlog(fmt.Sprintf("Mark agentVersion is updated in status file for nmp %v", nmpName)))
			// update status.json, set k8s.imageVersion.updated = true
			if err = setResourceUpdatedInStatusFile(workDir, RESOURCE_IMAGE_VERSION, true); err != nil {
				return "", fmt.Errorf(fmt.Sprintf("failed to set updated to true for imageVersion for nmp: %v, error: %v", nmpName, err))
			}
			glog.Infof(cuwlog(fmt.Sprintf("Set updated to true for imageVersion for nmp %v", nmpName)))

		}
	}
	return nmPolicyStatus.AgentUpgrade.Status, nil

}

func (w *ClusterUpgradeWorker) collectStatus(workingFolderPath string, policyName string, dbStatus *exchangecommon.NodeManagementPolicyStatus) error {
	// policyName is {org}/{nmpName}
	filePath := path.Join(workingFolderPath, policyName, nodemanagement.STATUS_FILE_NAME)
	// Read in the status file
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("Failed to open status file %v for management job %v. Error was: %v", filePath, policyName, err)
	}
	if openPath, err := os.Open(filePath); err != nil {
		return fmt.Errorf("Failed to open status file %v for management job %v. Errorf was: %v", filePath, policyName, err)
	} else {
		contents := exchangecommon.NodeManagementPolicyStatus{}
		err = json.NewDecoder(openPath).Decode(&contents)
		if err != nil {
			return fmt.Errorf("Failed to decode status file %v for management job %v. Error was %v.", filePath, policyName, err)
		}

		exchDev, err := persistence.FindExchangeDevice(w.db)
		if err != nil {
			glog.Errorf(cuwlog(fmt.Sprintf("Error getting device from database: %v", err)))
			exchDev = nil
		}

		// 1. save the status to local db
		// 2. put the status to the exchange
		status_changed, err := common.SetNodeManagementPolicyStatus(w.db, exchDev, policyName, &contents, dbStatus,
			exchange.GetPutNodeManagementPolicyStatusHandler(w),
			exchange.GetHTTPDeviceHandler(w),
			exchange.GetHTTPPatchDeviceHandler(w))
		if err != nil {
			glog.Errorf(cuwlog(fmt.Sprintf("Error saving nmp status for %v: %v", policyName, err)))
			return err
		} else {
			// log the event
			if status_changed {
				pattern := ""
				configState := ""
				if exchDev != nil {
					pattern = exchDev.Pattern
					configState = exchDev.Config.State
				}
				status_string := contents.AgentUpgrade.Status
				if status_string == "" {
					status_string = exchangecommon.STATUS_UNKNOWN
				}
				if contents.AgentUpgrade.ErrorMessage != "" {
					status_string += fmt.Sprintf(", ErrorMessage: %v", contents.AgentUpgrade.ErrorMessage)
				}
				eventlog.LogNodeEvent(w.db, persistence.SEVERITY_INFO, persistence.NewMessageMeta(nodemanagement.EL_NMP_STATUS_CHANGED, policyName, status_string), persistence.EC_NMP_STATUS_UPDATE_NEW, exchange.GetId(w.GetExchangeId()), exchange.GetOrg(w.GetExchangeId()), pattern, configState)
			}
		}
	}
	return nil
}

// This function will set status in status file, in local db and in exchang
func (w *ClusterUpgradeWorker) setStatusInDBAndFile(baseWorkingDir string, nmpName string, statusToSet string, errorMessage string) error {
	glog.Infof(cuwlog(fmt.Sprintf("Set status to %v in db and status file for nmp %v", statusToSet, nmpName)))

	workDir := path.Join(baseWorkingDir, nmpName)
	if statusToSet == exchangecommon.STATUS_FAILED_JOB || statusToSet == exchangecommon.STATUS_PRECHECK_FAILED {
		if err := setErrorMessageInStatusFile(workDir, statusToSet, errorMessage); err != nil {
			glog.Errorf(fmt.Sprintf("Failed to update NMP sataus to %v for nmp: %v in the status file, error: %v", statusToSet, nmpName, err))
			return err
		}

	} else {
		if err := setNMPStatusInStatusFile(workDir, statusToSet); err != nil {
			glog.Errorf(fmt.Sprintf("Failed to update NMP sataus to %v for nmp: %v in the status file, error: %v", statusToSet, nmpName, err))
			return err
		}
	}

	status, err := persistence.FindNMPStatus(w.db, nmpName)
	if err != nil {
		glog.Errorf(cuwlog(fmt.Sprintf("Failed to get nmp status %v from the database: %v", nmpName, err)))
		return err
	}

	if err = w.collectStatus(baseWorkingDir, nmpName, status); err != nil {
		glog.Errorf(cuwlog(fmt.Sprintf("Failed to update NMP sataus to %v in local db and exchange for nmp: %v, error: %v", statusToSet, nmpName, err)))
		return err
	}

	glog.Infof(cuwlog(fmt.Sprintf("Status is updated to %v for nmp %v", status, nmpName)))
	return nil
}

func (w *ClusterUpgradeWorker) Messages() chan events.Message {
	return w.BaseWorker.Manager.Messages
}

func getEC(config *config.HorizonConfig, db *bolt.DB) *worker.BaseExchangeContext {
	var ec *worker.BaseExchangeContext
	if dev, _ := persistence.FindExchangeDevice(db); dev != nil {
		ec = worker.NewExchangeContext(fmt.Sprintf("%v/%v", dev.Org, dev.Id), dev.Token, config.Edge.ExchangeURL, config.GetCSSURL(), config.Edge.AgbotURL, config.Collaborators.HTTPClientFactory)
	}
	return ec
}

func (w ClusterUpgradeWorker) NewEvent(incoming events.Message) {
	glog.Infof(cuwlog(fmt.Sprintf("Handling event: %v", incoming)))
	switch incoming.(type) {
	case *events.AgentPackageDownloadedMessage:
		msg, _ := incoming.(*events.AgentPackageDownloadedMessage)
		switch msg.Event().Id {
		case events.AGENT_PACKAGE_DOWNLOADED:
			cmd := NewClusterUpgradeCommand(msg)
			w.Commands <- cmd

		}
	case *events.EdgeRegisteredExchangeMessage:
		msg, _ := incoming.(*events.EdgeRegisteredExchangeMessage)
		switch msg.Event().Id {
		case events.NEW_DEVICE_REG:
			cmd := NewNodeRegisteredCommand(msg)
			w.Commands <- cmd
		}
	case *events.NodeShutdownCompleteMessage:
		msg, _ := incoming.(*events.NodeShutdownCompleteMessage)
		switch msg.Event().Id {
		case events.UNCONFIGURE_COMPLETE:
			w.Commands <- worker.NewTerminateCommand("shutdown")
		}
	}

}

func (w *ClusterUpgradeWorker) CommandHandler(command worker.Command) bool {
	glog.Infof(cuwlog(fmt.Sprintf("Handling command %v", command)))
	switch command.(type) {
	case *ClusterUpgradeCommand:
		cmd := command.(*ClusterUpgradeCommand)
		w.HandleClusterUpgrade(exchange.GetOrg(w.GetExchangeId()), cmd.Msg.Message.NMPStatus.AgentUpgrade.BaseWorkingDirectory, cmd.Msg.Message.NMPName, cmd.Msg.Message.NMPStatus.AgentUpgrade.UpgradedVersions)
	case *NodeRegisteredCommand:
		w.EC = getEC(w.Config, w.db)
	default:
		return false
	}
	return true
}

// Returns the agreement IDs for agreements that has AgreementExecutionStartTime == 0.
func (w *ClusterUpgradeWorker) GetUncompletedAgreements() ([]string, error) {
	notStartedFilter := func() persistence.EAFilter {
		return func(a persistence.EstablishedAgreement) bool {
			return a.AgreementExecutionStartTime == 0
		}
	}

	ag_ids := []string{}
	if uncompleted_ags, err := persistence.FindEstablishedAgreementsAllProtocols(w.db, policy.AllAgreementProtocols(), []persistence.EAFilter{persistence.UnarchivedEAFilter(), notStartedFilter()}); err != nil {
		return ag_ids, fmt.Errorf("Unable to retrieve uncompleted agreements from database. Error: %v", err)
	} else {
		for _, ag := range uncompleted_ags {
			ag_ids = append(ag_ids, ag.CurrentAgreementId)
		}
	}

	return ag_ids, nil
}

func (w *ClusterUpgradeWorker) HandleClusterUpgrade(org string, baseWorkingDir string, nmpName string, agentUpgradeVersions exchangecommon.AgentUpgradeVersions) {
	// nmpName: {org}/{nmpName}
	// baseWorkingDir: /var/horizon/nmp/
	glog.Infof(cuwlog(fmt.Sprintf("Start handling edge cluster upgrade for nmp: %v", nmpName)))

	// check if the agent is making agreements with the agbot
	for {
		if uncompleted_ags, err := w.GetUncompletedAgreements(); err != nil {
			glog.Errorf(cuwlog(fmt.Sprintf("Unable to retrieve uncompleted agreements from database. Error: %v", err)))
			return
		} else {
			if len(uncompleted_ags) > 0 {
				glog.Infof(cuwlog(fmt.Sprintf("Cannot start running nmp %v because there are agreements not completed yet: %v", nmpName, uncompleted_ags)))
				time.Sleep(time.Duration(5) * time.Second)
			} else {
				glog.Infof(cuwlog(fmt.Sprintf("Agreement checking done.")))
				break
			}
		}
	}

	status, err := persistence.FindNMPStatus(w.db, nmpName)
	if err != nil {
		glog.Errorf(cuwlog(fmt.Sprintf("Failed to get nmp status %v from the database: %v", nmpName, err)))
		return
	}

	workDir := path.Join(baseWorkingDir, nmpName)
	if err = createNMPStatusFile(workDir, exchangecommon.STATUS_INITIATED); err != nil {
		glog.Errorf(cuwlog(fmt.Sprintf("Failed to create NMP sataus file under %v for nmp: %v, error: %v", workDir, nmpName, err)))
		return
	}

	// collect status from status file, update local db and exchange
	if err := w.collectStatus(baseWorkingDir, nmpName, status); err != nil {
		glog.Errorf(cuwlog(fmt.Sprintf("Failed to collect sataus from status file under %v for nmp: %v, error: %v", workDir, nmpName, err)))
		return
	}

	// check resources:
	// check /var/horizon/nmp/<org>/nmpName directory,
	// 1) check config: agent-install.cfg
	// 2) cert: agent-install.crt
	// 3) image file: amd64_anax_k8s.tar.gz. After extract, it will be: hyc-edge-team-nightly-docker-virtual.artifactory.swg-devops.com/amd64_anax_k8s:2.29.0-595

	var errMessage string
	configIsSame, newConfigInAgentFile, _, err := checkAgentConfig(w.kubeClient, workDir)
	if err != nil {
		errMessage = fmt.Sprintf("Failed to compare config values for nmp: %v, error: %v", nmpName, err)
		glog.Errorf(cuwlog(errMessage))
		w.setStatusInDBAndFile(baseWorkingDir, nmpName, exchangecommon.STATUS_PRECHECK_FAILED, errMessage)
		return
	}
	if !configIsSame {
		if err = setResourceNeedChangeInStatusFile(workDir, RESOURCE_CONFIGMAP, true); err != nil {
			errMessage = fmt.Sprintf("Failed to update set needChange to true for configmap for nmp: %v, error: %v", nmpName, err)
			glog.Errorf(cuwlog(errMessage))
			w.setStatusInDBAndFile(baseWorkingDir, nmpName, exchangecommon.STATUS_PRECHECK_FAILED, errMessage)
			return
		}
	}

	certIsSame, newCertInAgentFile, _, err := checkAgentCert(w.kubeClient, workDir)
	if err != nil {
		errMessage = fmt.Sprintf("Failed to compare cert values for nmp: %v, error: %v", nmpName, err)
		glog.Errorf(cuwlog(errMessage))
		w.setStatusInDBAndFile(baseWorkingDir, nmpName, exchangecommon.STATUS_PRECHECK_FAILED, errMessage)
		return
	}
	if !certIsSame {
		if err = setResourceNeedChangeInStatusFile(workDir, RESOURCE_SECRET, true); err != nil {
			errMessage = fmt.Sprintf("Failed to update set needChange to true for secret for nmp: %v, error: %v", nmpName, err)
			glog.Errorf(cuwlog(errMessage))
			w.setStatusInDBAndFile(baseWorkingDir, nmpName, exchangecommon.STATUS_PRECHECK_FAILED, errMessage)
			return
		}
	}

	if !configIsSame || !certIsSame {
		glog.Infof(cuwlog(fmt.Sprintf("configIsSame: %v, certIsSame: %v, will need to validate config and cert for nmp %v", configIsSame, certIsSame, nmpName)))
		exchangeURL := cliutils.GetExchangeUrl()
		if !configIsSame {
			if newConfigInAgentFile["HZN_EXCHANGE_URL"] != "" {
				exchangeURL = newConfigInAgentFile["HZN_EXCHANGE_URL"]
			}
		}

		certPath := path.Join(DEFAULT_CERT_PATH, AGENT_CERT_FILE)
		if !certIsSame {
			certPath = path.Join(workDir, AGENT_CERT_FILE)
		}

		if err = ValidateConfigAndCert(exchangeURL, certPath); err != nil {
			// precheck failed
			errMessage = fmt.Sprintf("Failed to validate exchangeURL and/or cert for nmp: %v, error: %v", nmpName, err)
			glog.Errorf(cuwlog(errMessage))
			w.setStatusInDBAndFile(baseWorkingDir, nmpName, exchangecommon.STATUS_PRECHECK_FAILED, errMessage)
			return
		}
		glog.Infof(cuwlog(fmt.Sprintf("exchangeURL and/or cert are validated for nmp %v", nmpName)))
	}

	// get cluster arch
	agentArch, err := w.GetClusterArch()
	if err != nil {
		glog.Errorf(cuwlog(fmt.Sprintf("Failed to get cluster agent arch for nmp: %v, error: %v", nmpName, err)))
		return
	}
	imageVersionIsSame, newImageVersion, currentImageVersion, err := checkAgentImage(w.kubeClient, workDir, agentUpgradeVersions.SoftwareVersion, agentArch)
	if err != nil {
		errMessage = fmt.Sprintf("Failed to compare agent image version for nmp: %v, error: %v", nmpName, err)
		glog.Errorf(cuwlog(errMessage))
		w.setStatusInDBAndFile(baseWorkingDir, nmpName, exchangecommon.STATUS_PRECHECK_FAILED, errMessage)
		return
	}
	glog.Infof(cuwlog(fmt.Sprintf("current image version: %v, image version to update: %v", currentImageVersion, newImageVersion)))

	if !imageVersionIsSame {
		if err = setResourceNeedChangeInStatusFile(workDir, RESOURCE_IMAGE_VERSION, true); err != nil {
			errMessage = fmt.Sprintf("Failed to update set needChange to true for image version for nmp: %v, error: %v", nmpName, err)
			glog.Errorf(cuwlog(errMessage))
			w.setStatusInDBAndFile(baseWorkingDir, nmpName, exchangecommon.STATUS_PRECHECK_FAILED, errMessage)
			return
		}
		if err = setImageInfoInStatusFile(workDir, currentImageVersion, newImageVersion); err != nil {
			errMessage = fmt.Sprintf("Failed to set image versions(from: %v, to: %v) for nmp: %v, error: %v", currentImageVersion, newImageVersion, nmpName, err)
			glog.Errorf(cuwlog(errMessage))
			w.setStatusInDBAndFile(baseWorkingDir, nmpName, exchangecommon.STATUS_PRECHECK_FAILED, errMessage)
			return
		}
	}

	if configIsSame && certIsSame && imageVersionIsSame {
		glog.Infof("agent config, cert and image version are same, set status to %v for nmp: %v", exchangecommon.STATUS_SUCCESSFUL, nmpName)
		// set nmp status to successful in db and status.json
		if err = w.setStatusInDBAndFile(baseWorkingDir, nmpName, exchangecommon.STATUS_SUCCESSFUL, ""); err != nil {
			errMessage = fmt.Sprintf("Failed to update status to %v in db and status file for nmp: %v, error: %v", exchangecommon.STATUS_SUCCESSFUL, nmpName, err)
			glog.Errorf(cuwlog(errMessage))
			w.setStatusInDBAndFile(baseWorkingDir, nmpName, exchangecommon.STATUS_PRECHECK_FAILED, errMessage)
			return
		}
		glog.Infof(cuwlog(fmt.Sprintf("NMP sataus is set to to %v for nmp: %v and return", exchangecommon.STATUS_SUCCESSFUL, nmpName)))
		return
	}

	// backup process, update current configmap, set k8s.configMap.updated = true in status.json
	if !configIsSame {
		glog.Infof(cuwlog(fmt.Sprintf("agent config is different for nmp %v, starting configmap backup and update process...", nmpName)))
		// backup configmap
		if err = w.kubeClient.CreateBackupConfigmap(AGENT_NAMESPACE, AGENT_CONFIGMAP); err != nil {
			errMessage = fmt.Sprintf("Failed to backup configmap for nmp: %v, error: %v", nmpName, err)
			glog.Errorf(cuwlog(errMessage))
			w.setStatusInDBAndFile(baseWorkingDir, nmpName, exchangecommon.STATUS_FAILED_JOB, errMessage)
			return
		}
		// update original configmap with new horizon env value
		newConfigMapData := prepareConfigmapData(newConfigInAgentFile)
		if err = w.kubeClient.UpdateAgentConfigmap(AGENT_NAMESPACE, AGENT_CONFIGMAP, newConfigMapData); err != nil {
			errMessage = fmt.Sprintf("Failed to update configmap for nmp: %v, error: %v", nmpName, err)
			glog.Errorf(cuwlog(errMessage))
			w.setStatusInDBAndFile(baseWorkingDir, nmpName, exchangecommon.STATUS_FAILED_JOB, errMessage)
			return
		}
		// update status.json, set k8s.configMap.updated = true
		if err = setResourceUpdatedInStatusFile(workDir, RESOURCE_CONFIGMAP, true); err != nil {
			errMessage = fmt.Sprintf("Failed to  set updated to true for configmap for nmp: %v, error: %v", nmpName, err)
			glog.Errorf(cuwlog(errMessage))
			w.setStatusInDBAndFile(baseWorkingDir, nmpName, exchangecommon.STATUS_FAILED_JOB, errMessage)
			return
		}
		glog.Infof(cuwlog(fmt.Sprintf("agent configmap is handled for nmp: %v", nmpName)))
	}

	if !certIsSame {
		glog.Infof(cuwlog(fmt.Sprintf("agent cert is different for nmp %v, starting secret (cert) backup and update process...", nmpName)))
		if err = w.kubeClient.CreateBackupSecret(AGENT_NAMESPACE, AGENT_SECRET); err != nil {
			errMessage = fmt.Sprintf("Failed to backup secret for nmp: %v, error: %v", nmpName, err)
			glog.Errorf(cuwlog(errMessage))
			w.setStatusInDBAndFile(baseWorkingDir, nmpName, exchangecommon.STATUS_FAILED_JOB, errMessage)
			return
		}

		if err = w.kubeClient.UpdateAgentSecret(AGENT_NAMESPACE, AGENT_SECRET, newCertInAgentFile); err != nil {
			errMessage = fmt.Sprintf("Failed to update secret for nmp: %v, error: %v", nmpName, err)
			glog.Errorf(cuwlog(errMessage))
			w.setStatusInDBAndFile(baseWorkingDir, nmpName, exchangecommon.STATUS_FAILED_JOB, errMessage)
			return
		}

		// update status.json, set k8s.secret.updated = true
		if err = setResourceUpdatedInStatusFile(workDir, RESOURCE_SECRET, true); err != nil {
			errMessage = fmt.Sprintf("Failed to  set updated to true for secret for nmp: %v, error: %v", nmpName, err)
			glog.Errorf(cuwlog(errMessage))
			w.setStatusInDBAndFile(baseWorkingDir, nmpName, exchangecommon.STATUS_FAILED_JOB, errMessage)
			return
		}
		glog.Infof(cuwlog(fmt.Sprintf("agent secret is handled for nmp: %v", nmpName)))
	}

	if !imageVersionIsSame {
		glog.Infof(cuwlog(fmt.Sprintf("agent image version is different for nmp %v, setting agent image version to %v in agent deployment...", nmpName, newImageVersion)))
		// update the deployment will restart agent
		if err = w.kubeClient.UpdateAgentDeploymentImageVersion(AGENT_NAMESPACE, AGENT_DEPLOYMENT, newImageVersion); err != nil {
			errMessage = fmt.Sprintf("Failed to update image version in agent deployment for nmp: %v, error: %v", nmpName, err)
			glog.Errorf(cuwlog(errMessage))
			w.setStatusInDBAndFile(baseWorkingDir, nmpName, exchangecommon.STATUS_FAILED_JOB, errMessage)
			return
		}
		// agent restarting, status will updated to "successful" after new agent is up

		glog.Infof(cuwlog(fmt.Sprintf("agent image update is handled for nmp: %v", nmpName)))
	} else {
		glog.Infof(cuwlog(fmt.Sprintf("agent image version is same, config and/or secret are already updated, check status in status file for nmp: %v", nmpName)))

		// imageVersion is same, config is diff or/and cert is diff,
		// if status is initiated, set it to successful
		statusFromFile, err := getStatusFromFile(workDir)
		if err != nil {
			errMessage = fmt.Sprintf("Failed to retrieve status from status file for nmp: %v, error: %v", nmpName, err)
			glog.Errorf(cuwlog(errMessage))
			w.setStatusInDBAndFile(baseWorkingDir, nmpName, exchangecommon.STATUS_FAILED_JOB, errMessage)
			return
		}

		if statusFromFile.AgentUpgrade.Status == exchangecommon.STATUS_INITIATED {
			glog.Infof(cuwlog(fmt.Sprintf("agent image version is same, config and/or secret are already updated, set status to %v for nmp: %v", exchangecommon.STATUS_SUCCESSFUL, nmpName)))
			// set nmp status to successful in db and status.json
			if err = w.setStatusInDBAndFile(baseWorkingDir, nmpName, exchangecommon.STATUS_SUCCESSFUL, ""); err != nil {
				errMessage = fmt.Sprintf("Failed to update status to %v in db and status file for nmp: %v, error: %v", exchangecommon.STATUS_SUCCESSFUL, nmpName, err)
				glog.Errorf(cuwlog(errMessage))
				w.setStatusInDBAndFile(baseWorkingDir, nmpName, exchangecommon.STATUS_FAILED_JOB, errMessage)
				return
			}
			glog.Infof(cuwlog(fmt.Sprintf("NMP sataus is set to to %v for nmp: %v and return", exchangecommon.STATUS_SUCCESSFUL, nmpName)))
			return
		}

		glog.Infof(cuwlog(fmt.Sprintf("status (%v) in status file is not %v for nmp: %v, will not update status", statusFromFile.AgentUpgrade.Status, exchangecommon.STATUS_INITIATED, nmpName)))
	}
}

// checkAgentConfig returns bool, configInAgentFile, configInK8sConfigMap, error
func checkAgentConfig(kubeClient *KubeClient, workDir string) (bool, map[string]string, map[string]string, error) {
	// workDir is /var/horizon/nmp/<org>/nmpID
	configFilePath := path.Join(workDir, AGENT_CONFIG_FILE)
	glog.Infof(cuwlog(fmt.Sprintf("reading in agent config file: %v", configFilePath)))

	var configInAgentFile map[string]string
	var configInK8S map[string]string
	var err error

	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		// cfg is not exist, means download worker doesn't download it (same version of config)
		return true, configInAgentFile, configInK8S, nil
	}

	// Read the file
	if configInAgentFile, err = ReadAgentConfigFile(configFilePath); err != nil {
		return false, configInAgentFile, configInK8S, err
	}

	if configInK8S, err = kubeClient.ReadConfigMap(AGENT_NAMESPACE, AGENT_CONFIGMAP); err != nil {
		return false, configInAgentFile, configInK8S, err
	}

	// value of AGENT_NAMESPACE and HZN_NAMESPACE_SCOPED can't be changed during auto-upgrade
	// change or add current AGENT_NAMESPACE value to config
	if AGENT_NAMESPACE != configInAgentFile[AGENT_NAMESPACE_ENV_NAME] {
		configInAgentFile[AGENT_NAMESPACE_ENV_NAME] = AGENT_NAMESPACE
	}

	if _, ok := configInAgentFile[HZN_NAMESPACE_SCOPED_ENV_NAME]; ok {
		agentIsNamespaceScope := cutil.IsNamespaceScoped()
		configInAgentFile[HZN_NAMESPACE_SCOPED_ENV_NAME] = strconv.FormatBool(agentIsNamespaceScope)
	}

	// compare to agent configmap
	configIsSame := reflect.DeepEqual(configInAgentFile, configInK8S)
	glog.Infof(cuwlog(fmt.Sprintf("agent install config is same: %v", configIsSame)))
	return configIsSame, configInAgentFile, configInK8S, nil

}

func checkAgentCert(kubeClient *KubeClient, workDir string) (bool, []byte, []byte, error) {
	// workDir is /var/horizon/nmp/<org>/nmpID
	certFilePath := path.Join(workDir, AGENT_CERT_FILE)
	glog.Infof(cuwlog(fmt.Sprintf("reading in agent cert file: %v", certFilePath)))

	var certInAgentFile []byte
	var certInK8S []byte
	var err error

	if _, err := os.Stat(certFilePath); os.IsNotExist(err) {
		// cert is not exist, means download worker doesn't download it (same version of cert)
		return true, certInAgentFile, certInK8S, nil
	}

	if certInAgentFile, err = ReadAgentCertFile(certFilePath); err != nil {
		return false, certInAgentFile, certInK8S, err
	}

	if certInK8S, err = kubeClient.ReadSecret(AGENT_NAMESPACE, AGENT_SECRET); err != nil {
		return false, certInAgentFile, certInK8S, err
	}

	// compare cert content
	certIsSame := compareCertContent(certInAgentFile, certInK8S)
	glog.Infof(cuwlog(fmt.Sprintf("agent install cert is same: %v", certIsSame)))
	return certIsSame, certInAgentFile, certInK8S, nil

}

func compareCertContent(certInAgentFile []byte, certInK8S []byte) bool {
	if compareRes := bytes.Compare(certInAgentFile, certInK8S); compareRes == 0 {
		return true
	} else {
		return false
	}

}

// checkAgentImage returns compare result of current image version and image version to update, image version to update, current image version, error
func checkAgentImage(kubeClient *KubeClient, workDir string, agentSoftwareVersionToUpgrade string, agentArch interface{}) (bool, string, string, error) {
	// image file is: /var/horizon/nmp/<org>/nmpID/{arch}_anax_k8s.tar.gz
	currentAgentVersion := version.HORIZON_VERSION
	if currentAgentVersion != agentSoftwareVersionToUpgrade {
		agent_image_targz := fmt.Sprintf(AGENT_IMAGE_TAR_GZ, agentArch)
		agent_image_tar := fmt.Sprintf(AGENT_IMAGE_TAR, agentArch)
		agent_image_name := fmt.Sprintf(AGENT_IMAGE_NAME, agentArch)

		imageTarGzFilePath := path.Join(workDir, agent_image_targz)
		glog.Infof(cuwlog(fmt.Sprintf("Getting image tar file: %v", imageTarGzFilePath)))

		if _, err := os.Stat(imageTarGzFilePath); os.IsNotExist(err) {
			// image tar.gz is not exist, means download worker doesn't download it (same version of image)
			return true, currentAgentVersion, currentAgentVersion, nil
		}

		imageTarballPath := path.Join(workDir, agent_image_tar)

		// get amd64_anax_k8s.tar from amd64_anax_k8s.tar.gz
		if err := getAgentTarballFromGzip(imageTarGzFilePath, imageTarballPath); err != nil {
			glog.Errorf(cuwlog(fmt.Sprintf("Failed to extract agent image tarball from %v, error: %v", imageTarGzFilePath, err)))
			return false, "", "", err
		}

		decompressTargetFolder := fmt.Sprintf("./%s", agent_image_name)

		// extract the docker manifest file from image tarball
		if err := extractImageManifest(imageTarballPath, decompressTargetFolder); err != nil {
			glog.Errorf(cuwlog(fmt.Sprintf("Failed to extract docker manifest file from agent image tallball %v, error: %v", imageTarballPath, err)))
			return false, "", "", err
		}

		_, imageTagInPackage, err := getImageTagFromManifestFile(decompressTargetFolder)
		if err != nil {
			glog.Errorf(cuwlog(fmt.Sprintf("Failed to get image tag from manifest file in side %v, error: %v", imageTarGzFilePath, err)))
			return false, "", "", err
		}
		glog.Infof(cuwlog(fmt.Sprintf("Get image from tar file, extracted image tag: %v", imageTagInPackage)))

		if imageTagInPackage != agentSoftwareVersionToUpgrade {
			glog.Errorf(cuwlog(fmt.Sprintf("image version from docker manifest file (%v) does not match the image version specified in the NMP manifest (%v). Please check the %v of %v in the CSS.", imageTagInPackage, agentSoftwareVersionToUpgrade, agent_image_targz, agentSoftwareVersionToUpgrade)))
			return false, "", "", fmt.Errorf("image version from docker manifest file (%v) does not match the image version specified in the NMP manifest (%v)", imageTagInPackage, agentSoftwareVersionToUpgrade)
		}

		// push image to image registry
		imageRegistry := os.Getenv("AGENT_CLUSTER_IMAGE_REGISTRY_HOST")
		if imageRegistry == "" {
			return false, "", "", fmt.Errorf("failed to get edge cluster image registry host from environment veriable: %v", imageRegistry)
		}

		// $ docker load --input amd64_anax_k8s.tar.gz
		// Loaded image: hyc-edge-team-staging-docker-local.artifactory.swg-devops.com/amd64_anax_k8s:2.30.0-689
		glog.Infof(cuwlog(fmt.Sprintf("Loading docker image from: %v", imageTarballPath)))
		loadImage, err := crane.Load(imageTarballPath)
		if err != nil {
			glog.Errorf(cuwlog(fmt.Sprintf("Failed to crane load image tar at %v, error: %v", imageTarballPath, err)))
			return false, "", "", err
		}

		usingRemoteICR := false
		if hasPullSecrets, _ := kubeClient.DeploymentHasImagePullSecrets(AGENT_NAMESPACE, AGENT_DEPLOYMENT); hasPullSecrets {
			glog.Infof(cuwlog(fmt.Sprintf("detected deployment has pull secrets, %v", hasPullSecrets)))
			usingRemoteICR = true
		}

		// docker tag hyc-edge-team-staging-docker-local.artifactory.swg-devops.com/amd64_anax_k8s:2.30.0-689 default-route-openshift-image-registry.apps.prowler.cp.fyre.ibm.com/openhorizon-agent/amd64_anax_k8s:2.30.0-689
		// docker tag ${fullImageTag} ${newImageTag}
		// new tag for agent using local registry:
		//  - ocp: default-route-openshift-image-registry.apps.prowler.cp.fyre.ibm.com/openhorizon-agent/amd64_anax_k8s:2.30.0-689
		//  - k3s: 10.43.100.65:5000/openhorizon-agent/amd64_anax_k8s:2.30.0-689
		//  - cluster use remote ICR: <remote-host>/<agent-namespace>/amd64_anax_k8s:2.30.0-689
		newImageRepoWithTag := fmt.Sprintf("%s/%s/%s:%s", imageRegistry, AGENT_NAMESPACE, agent_image_name, agentSoftwareVersionToUpgrade)
		if usingRemoteICR {
			newImageRepoWithTag = fmt.Sprintf("%s/%s:%s", imageRegistry, agent_image_name, agentSoftwareVersionToUpgrade)
		}
		glog.Infof(cuwlog(fmt.Sprintf("New image repo with tag: %v", newImageRepoWithTag)))

		tag, err := name.NewTag(newImageRepoWithTag)
		if err != nil {
			glog.Errorf(cuwlog(fmt.Sprintf("Failed to create new tag %v, error: %v", newImageRepoWithTag, err)))
			return false, "", "", err
		}

		// If deployment have image pull secret, it means it uses remote image registry
		var kc authn.Keychain
		skipImagePush := false
		if usingRemoteICR {
			_, err := kubeClient.GetSecret(AGENT_NAMESPACE, AGENT_IMAGE_PULL_SECRETS_NAME)
			if err != nil {
				glog.Errorf(cuwlog(fmt.Sprintf("Failed to get image pull secrets %v, error: %v", AGENT_IMAGE_PULL_SECRETS_NAME, err)))
				return false, "", "", err
			}
			imagePullSecrets := []string{AGENT_IMAGE_PULL_SECRETS_NAME}
			kc, err = kubeClient.GetImagePullSecretKeyChain(AGENT_NAMESPACE, AGENT_SERVICE_ACCOUNT, imagePullSecrets)
			if err != nil {
				glog.Errorf(cuwlog(fmt.Sprintf("Failed to get key chain from serviceaccount and imagePullSecrets, error: %v", err)))
				return false, "", "", err
			}

			// if image exists skip pushing
			glog.Infof(cuwlog(fmt.Sprintf("checking if image %v exists on remote registry...", tag.String())))
			if imageExistInRemoteRegistry(tag.String(), agentSoftwareVersionToUpgrade, kc) {
				glog.Infof(cuwlog(fmt.Sprintf("image tag %v exists on remote registry, skip pushing image", tag.String())))
				skipImagePush = true
			}
		} else {
			kc, err = kubeClient.GetKeyChain(AGENT_NAMESPACE, AGENT_SERVICE_ACCOUNT)
			if err != nil {
				glog.Errorf(cuwlog(fmt.Sprintf("Failed to get key chain from serviceaccount, error: %v", err)))
				return false, "", "", err
			}
		}

		if !skipImagePush {
			glog.Infof(cuwlog(fmt.Sprintf("pushing image %v...", newImageRepoWithTag)))
			if err := crane.Push(loadImage, tag.String(), crane.WithAuthFromKeychain(kc)); err != nil {
				glog.Errorf(cuwlog(fmt.Sprintf("Failed to push image %v, error: %v", newImageRepoWithTag, err)))
				return false, "", "", err
			}
			glog.Infof(cuwlog(fmt.Sprintf("Successfully pushed image %v", newImageRepoWithTag)))
		}

	}
	return (currentAgentVersion == agentSoftwareVersionToUpgrade), agentSoftwareVersionToUpgrade, currentAgentVersion, nil
}

func checkAgentImageAgainstStatusFile(workDir string) (bool, error) {
	glog.Infof(cuwlog(fmt.Sprintf("Compare agent image version in deployment and in status file")))
	if statusFile, err := getStatusFromFile(workDir); err != nil {
		return false, err
	} else if toVersion := statusFile.AgentUpgrade.K8S.ImageVersion.To; toVersion == "" {
		return false, fmt.Errorf("imageVersion.To is empty in status file")
	} else if toVersion != version.HORIZON_VERSION {
		return false, fmt.Errorf("agent current version (%v) is different from imageVersion.To (%v) in status file", version.HORIZON_VERSION, toVersion)
	} else {
		return true, nil
	}
}

func agentUseRemoteRegistry() bool {
	useRemoteRegistry := false
	imageRegistry := os.Getenv("AGENT_CLUSTER_IMAGE_REGISTRY_HOST")
	if imageRegistry == DEFAULT_IMAGE_REGISTRY_IN_DEPLOYMENT {
		useRemoteRegistry = true
	}
	return useRemoteRegistry
}

func (w *ClusterUpgradeWorker) GetClusterArch() (interface{}, error) {
	pol, err := persistence.FindNodePolicy(w.db)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve node policy from local db: %v", err)
	} else if pol == nil {
		return "", fmt.Errorf("no node policy found in the local db")
	}

	archProp, err := pol.Properties.GetProperty(externalpolicy.PROP_NODE_ARCH)
	if err != nil {
		return "", err
	}
	return archProp.Value, nil
}
