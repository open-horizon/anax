package kube_operator

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/worker"
)

type KubeWorker struct {
	worker.BaseWorker
	db *bolt.DB
}

func NewKubeWorker(name string, config *config.HorizonConfig, db *bolt.DB) *KubeWorker {
	worker := &KubeWorker{
		BaseWorker: worker.NewBaseWorker(name, config, nil),
		db:         db,
	}
	glog.Info(kwlog(fmt.Sprintf("Starting Kubernetes Worker")))
	worker.Start(worker, 0)
	return worker
}

func (w *KubeWorker) Messages() chan events.Message {
	return w.BaseWorker.Manager.Messages
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

	client, err := NewKubeClient()
	if err != nil {
		return err
	}
	err = client.Install(kd.OperatorYamlArchive, kd.Metadata, *(lc.EnvironmentAdditions), lc.AgreementId, lc.Configure.ClusterNamespace, crInstallTimeout)
	if err != nil {
		return err
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

var kwlog = func(v interface{}) string {
	return fmt.Sprintf("Kubernetes Worker: %v", v)
}
