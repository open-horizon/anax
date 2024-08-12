package kube_operator

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/resource"
	"github.com/open-horizon/anax/worker"
	"path"
)

type KubeWorker struct {
	worker.BaseWorker
	config    *config.HorizonConfig
	db        *bolt.DB
	authMgr   *resource.AuthenticationManager
	secretMgr *resource.SecretsManager
}

func NewKubeWorker(name string, config *config.HorizonConfig, db *bolt.DB, am *resource.AuthenticationManager, sm *resource.SecretsManager) *KubeWorker {
	worker := &KubeWorker{
		BaseWorker: worker.NewBaseWorker(name, config, nil),
		config:     config,
		db:         db,
		authMgr:    am,
		secretMgr:  sm,
	}
	glog.Info(kwlog(fmt.Sprintf("Starting Kubernetes Worker")))
	worker.Start(worker, 0)
	return worker
}

func (w *KubeWorker) Messages() chan events.Message {
	return w.BaseWorker.Manager.Messages
}

func (kw *KubeWorker) GetAuthenticationManager() *resource.AuthenticationManager {
	return kw.authMgr
}

func (w *KubeWorker) GetSecretManager() *resource.SecretsManager {
	return w.secretMgr
}

func (w *KubeWorker) NewEvent(incoming events.Message) {
	switch incoming.(type) {
	case *events.AgreementReachedMessage:
		msg, _ := incoming.(*events.AgreementReachedMessage)

		fCmd := NewInstallCommand(msg.LaunchContext())
		w.Commands <- fCmd
	case *events.GovernanceWorkloadCancelationMessage:
		msg, _ := incoming.(*events.GovernanceWorkloadCancelationMessage)

		switch msg.Event().Id {
		case events.AGREEMENT_ENDED:
			cmd := NewUnInstallCommand(msg.AgreementProtocol, msg.AgreementId, msg.ClusterNamespace, msg.Deployment)
			w.Commands <- cmd
		}

	case *events.GovernanceMaintenanceMessage:
		msg, _ := incoming.(*events.GovernanceMaintenanceMessage)

		switch msg.Event().Id {
		case events.CONTAINER_MAINTAIN:
			cmd := NewMaintenanceCommand(msg.AgreementProtocol, msg.AgreementId, msg.ClusterNamespace, msg.Deployment)
			w.Commands <- cmd
		}

	case *events.WorkloadUpdateMessage:
		msg, _ := incoming.(*events.WorkloadUpdateMessage)

		switch msg.Event().Id {
		case events.UPDATE_SECRETS_IN_AGREEMENT:
			cmd := NewUpdateSecretCommand(msg.AgreementProtocol, msg.AgreementId, msg.ClusterNamespaceInAgreement, msg.Deployment, msg.SecretsUpdate)
			w.Commands <- cmd
		}

	case *events.NodeShutdownCompleteMessage:
		msg, _ := incoming.(*events.NodeShutdownCompleteMessage)
		switch msg.Event().Id {
		case events.UNCONFIGURE_COMPLETE:
			w.Commands <- worker.NewTerminateCommand("shutdown")
		}

	default: //nothing

	}
	return
}

func (w *KubeWorker) CommandHandler(command worker.Command) bool {
	switch command.(type) {
	case *InstallCommand:
		cmd := command.(*InstallCommand)
		if lc := w.getLaunchContext(cmd.LaunchContext); lc == nil {
			glog.Errorf(kwlog(fmt.Sprintf("incoming event was not a known launch context %T", cmd.LaunchContext)))
		} else {
			glog.V(5).Infof(kwlog(fmt.Sprintf("LaunchContext(%T) for agreement: %v", lc, lc.AgreementId)))

			// ignore the native deployment
			if lc.ContainerConfig().Deployment != "" {
				glog.V(5).Infof(kwlog(fmt.Sprintf("ignoring non-Kube deployment.")))
				return true
			}

			// Save service secrets from agreement into the microservice instance
			if err := w.GetSecretManager().ProcessServiceSecretsWithInstanceId(lc.AgreementId, lc.AgreementId); err != nil {
				glog.Errorf(kwlog(fmt.Sprintf("received error saving secrets from agreement into microservice in database, %v", err)))
				return true
			}

			// Check the deployment to check if it is a kube deployment
			deploymentConfig := lc.ContainerConfig().ClusterDeployment
			if kd, err := persistence.GetKubeDeployment(deploymentConfig); err != nil {
				glog.Errorf(kwlog(fmt.Sprintf("error getting kube deployment configuration: %v", err)))
				return true
			} else if _, err := persistence.AgreementDeploymentStarted(w.db, lc.AgreementId, lc.AgreementProtocol, kd); err != nil {
				glog.Errorf(kwlog(fmt.Sprintf("received error updating database deployment state, %v", err)))
				w.Messages() <- events.NewWorkloadMessage(events.EXECUTION_FAILED, lc.AgreementProtocol, lc.AgreementId, kd)
				return true
			} else if err := w.processKubeOperator(lc, kd, w.Config.GetK8sCRInstallTimeouts()); err != nil {
				glog.Errorf(kwlog(fmt.Sprintf("failed to process kube package after agreement negotiation: %v", err)))
				w.Messages() <- events.NewWorkloadMessage(events.EXECUTION_FAILED, lc.AgreementProtocol, lc.AgreementId, kd)
				return true
			} else {
				w.Messages() <- events.NewWorkloadMessage(events.EXECUTION_BEGUN, lc.AgreementProtocol, lc.AgreementId, kd)
			}
		}
	case *UnInstallCommand:
		cmd := command.(*UnInstallCommand)
		glog.V(3).Infof(kwlog(fmt.Sprintf("uninstalling operator from agreement %v", cmd.CurrentAgreementId)))

		kdc, ok := cmd.Deployment.(*persistence.KubeDeploymentConfig)
		if !ok {
			glog.Warningf(kwlog(fmt.Sprintf("ignoring non-Kube cancelation command %v", cmd)))
			return true
		} else if err := w.uninstallKubeOperator(kdc, cmd.CurrentAgreementId, cmd.AgreementProtocol, cmd.ClusterNamespace); err != nil {
			glog.Errorf(kwlog(fmt.Sprintf("failed to uninstall kube operator %v", cmd.Deployment)))
		}

		w.Messages() <- events.NewWorkloadMessage(events.WORKLOAD_DESTROYED, cmd.AgreementProtocol, cmd.CurrentAgreementId, kdc)
	case *MaintenanceCommand:
		cmd := command.(*MaintenanceCommand)
		glog.V(3).Infof(kwlog(fmt.Sprintf("received maintenance command %v", cmd)))

		kdc, ok := cmd.Deployment.(*persistence.KubeDeploymentConfig)
		if !ok {
			glog.Warningf(kwlog(fmt.Sprintf("ignoring non-Kube maintenence command: %v", cmd)))
		} else if err := w.operatorStatus(kdc, "Running", cmd.AgreementId, cmd.AgreementProtocol, cmd.ClusterNamespace); err != nil {
			glog.Errorf(kwlog(fmt.Sprintf("%v", err)))
			w.Messages() <- events.NewWorkloadMessage(events.EXECUTION_FAILED, cmd.AgreementProtocol, cmd.AgreementId, kdc)
		}
	case *UpdateSecretCommand:
		cmd := command.(*UpdateSecretCommand)
		glog.V(3).Infof(kwlog(fmt.Sprintf("receive secret update for agreement: %v", cmd.AgreementId)))

		kdc, ok := cmd.Deployment.(*persistence.KubeDeploymentConfig)
		if !ok {
			glog.Warningf(kwlog(fmt.Sprintf("ignoring non-Kube secret update command: %v", cmd)))
		} else if err := w.updateKubeOperatorSecrets(kdc, cmd.AgreementId, cmd.ClusterNamespace, cmd.UpdatedSecrets); err != nil {
			glog.Errorf(kwlog(fmt.Sprintf("%v", err)))
			w.Messages() <- events.NewWorkloadMessage(events.EXECUTION_FAILED, cmd.AgreementProtocol, cmd.AgreementId, kdc)
		}
	default:
		return true
	}
	return true
}

func (w *KubeWorker) getLaunchContext(launchContext interface{}) *events.AgreementLaunchContext {
	switch launchContext.(type) {
	case *events.AgreementLaunchContext:
		lc := launchContext.(*events.AgreementLaunchContext)
		return lc
	}
	return nil
}

func (w *KubeWorker) processKubeOperator(lc *events.AgreementLaunchContext, kd *persistence.KubeDeploymentConfig, crInstallTimeout int64) error {
	glog.V(3).Infof(kwlog(fmt.Sprintf("begin install of Kube Deployment %s", lc.AgreementId)))

	glog.V(3).Infof(kwlog(fmt.Sprintf("save service secrets into microservice in the agent database from agreement %s", lc.AgreementId)))
	secretsMap, err := w.GetSecretManager().ProcessServiceSecretsWithInstanceIdForCluster(lc.AgreementId, lc.AgreementId)
	if err != nil {
		return err
	}
	// eg: secretsMap is map[secret1:eyJrZXki...]

	// create auth in agent pod and mount it to service pod
	if ags, err := persistence.FindEstablishedAgreements(w.db, lc.AgreementProtocol, []persistence.EAFilter{persistence.UnarchivedEAFilter(), persistence.IdEAFilter(lc.AgreementId)}); err != nil {
		glog.Errorf("Unable to retrieve agreement %v from database, error %v", lc.AgreementId, err)
	} else if len(ags) != 1 {
		glog.V(3).Infof(kwlog(fmt.Sprintf("Ignoring the configure event for agreement %v, the agreement is no longer active.", lc.AgreementId)))
		return nil
	} else if ags[0].AgreementTerminatedTime != 0 {
		glog.V(3).Infof(kwlog(fmt.Sprintf("Received configure command for agreement %v. Ignoring it because this agreement has been terminated.", lc.AgreementId)))
		return nil
	} else if ags[0].AgreementExecutionStartTime != 0 {
		glog.V(3).Infof(kwlog(fmt.Sprintf("Received configure command for agreement %v. Ignoring it because the containers for this agreement has been configured.", lc.AgreementId)))
		return nil
	} else {
		serviceIdentity := cutil.FormOrgSpecUrl(cutil.NormalizeURL(ags[0].RunningWorkload.URL), ags[0].RunningWorkload.Org)
		sVer := ags[0].RunningWorkload.Version
		glog.V(3).Infof(kwlog(fmt.Sprintf("Creating ESS creds for svc: %v svcVer: %v", serviceIdentity, sVer)))

		_, err := w.GetAuthenticationManager().CreateCredential(lc.AgreementId, serviceIdentity, sVer, false)
		if err != nil {
			return err
		}

		client, err := NewKubeClient()
		if err != nil {
			return err
		}

		fssAuthFilePath := path.Join(w.GetAuthenticationManager().GetCredentialPath(lc.AgreementId), config.HZN_FSS_AUTH_FILE) // /var/horizon/ess-auth/<agreementId>/auth.json
		fssCertFilePath := path.Join(w.config.GetESSSSLClientCertPath(), config.HZN_FSS_CERT_FILE)                             // /var/horizon/ess-auth/SSL/cert/cert.pem
		err = client.Install(kd.OperatorYamlArchive, kd.Metadata, kd.MMSPVC, *(lc.EnvironmentAdditions), fssAuthFilePath, fssCertFilePath, secretsMap, lc.AgreementId, lc.Configure.ClusterNamespace, crInstallTimeout)
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *KubeWorker) uninstallKubeOperator(kd *persistence.KubeDeploymentConfig, agId string, agp string, reqNamespace string) error {
	glog.V(3).Infof(kwlog(fmt.Sprintf("begin uninstall of Kube Deployment %s", agId)))

	client, err := NewKubeClient()
	if err != nil {
		return err
	}
	err = client.Uninstall(kd.OperatorYamlArchive, kd.Metadata, agId, reqNamespace)
	if err != nil {
		return err
	}
	return nil
}

func (w *KubeWorker) operatorStatus(kd *persistence.KubeDeploymentConfig, intendedState string, agId string, agp string, reqnamespace string) error {
	glog.V(5).Infof(kwlog(fmt.Sprintf("begin listing operator status %v", kd.ToString())))

	client, err := NewKubeClient()
	if err != nil {
		return err
	}
	opStatus, err := client.Status(kd.OperatorYamlArchive, kd.Metadata, agId, reqnamespace)
	if err != nil {
		return err
	}
	retErrorStr := ""
	for _, container := range opStatus {
		if container.State != intendedState {
			retErrorStr = fmt.Sprintf("%s %s", retErrorStr, fmt.Sprintf("Container %s has status %s.", container.Name, container.State))
		}
	}
	if retErrorStr != "" {
		return fmt.Errorf(retErrorStr)
	}
	return nil
}

func (w *KubeWorker) updateKubeOperatorSecrets(kd *persistence.KubeDeploymentConfig, agId string, reqnamespace string, updatedSecrets []persistence.PersistedServiceSecret) error {
	glog.V(5).Infof(kwlog(fmt.Sprintf("begin updating service secrets for operator %v", kd.ToString())))

	client, err := NewKubeClient()
	if err != nil {
		return err
	}

	err = client.Update(kd.OperatorYamlArchive, kd.Metadata, agId, reqnamespace, map[string]string{}, updatedSecrets)
	if err != nil {
		return err
	}
	return nil
}

var kwlog = func(v interface{}) string {
	return fmt.Sprintf("Kubernetes Worker: %v", v)
}
