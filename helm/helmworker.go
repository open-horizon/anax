package helm

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/worker"
)

type HelmWorker struct {
	worker.BaseWorker // embedded field
	db                *bolt.DB
}

func NewHelmWorker(name string, config *config.HorizonConfig, db *bolt.DB) *HelmWorker {

	worker := &HelmWorker{
		BaseWorker: worker.NewBaseWorker(name, config, nil),
		db:         db,
	}

	glog.Info(hpwlog(fmt.Sprintf("Starting Helm worker")))
	worker.Start(worker, 0)
	return worker
}

func (w *HelmWorker) Messages() chan events.Message {
	return w.BaseWorker.Manager.Messages
}

func (w *HelmWorker) NewEvent(incoming events.Message) {

	switch incoming.(type) {
	case *events.AgreementReachedMessage:
		msg, _ := incoming.(*events.AgreementReachedMessage)

		fCmd := NewInstallCommand(msg.LaunchContext())
		w.Commands <- fCmd

	case *events.GovernanceWorkloadCancelationMessage:
		msg, _ := incoming.(*events.GovernanceWorkloadCancelationMessage)

		switch msg.Event().Id {
		case events.AGREEMENT_ENDED:
			cmd := NewUnInstallCommand(msg.AgreementProtocol, msg.AgreementId, msg.Deployment)
			w.Commands <- cmd
		}

	case *events.GovernanceMaintenanceMessage:
		msg, _ := incoming.(*events.GovernanceMaintenanceMessage)

		switch msg.Event().Id {
		case events.CONTAINER_MAINTAIN:
			cmd := NewMaintenanceCommand(msg.AgreementProtocol, msg.AgreementId, msg.Deployment)
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

func (w *HelmWorker) CommandHandler(command worker.Command) bool {

	switch command.(type) {
	case *InstallCommand:

		cmd := command.(*InstallCommand)
		if lc := w.getLaunchContext(cmd.LaunchContext); lc == nil {
			glog.Errorf(hpwlog(fmt.Sprintf("incoming event was not a known launch context: %T", cmd.LaunchContext)))
		} else {
			glog.V(5).Infof(hpwlog(fmt.Sprintf("LaunchContext(%T): %v", lc, lc)))

			// Check the deployment string to see if it's a Helm deployment.
			deploymentConfig := lc.ContainerConfig().Deployment
			if hd, err := persistence.GetHelmDeployment(deploymentConfig); err != nil {
				glog.Warningf(hpwlog(fmt.Sprintf("ignoring non-Helm deployment: %v", err)))
				return true
			} else if _, err := persistence.AgreementDeploymentStarted(w.db, lc.AgreementId, lc.AgreementProtocol, hd); err != nil {
				glog.Errorf(hpwlog(fmt.Sprintf("received error updating database deployment state, %v", err)))
				w.Messages() <- events.NewWorkloadMessage(events.EXECUTION_FAILED, lc.AgreementProtocol, lc.AgreementId, hd)
				return true
			} else if err := w.processHelmPackage(lc, hd); err != nil {
				// Since we have a Helm package, process it and install the package.
				glog.Errorf(hpwlog(fmt.Sprintf("failed to process helm package after agreement negotiation: %v", err)))
				w.Messages() <- events.NewWorkloadMessage(events.EXECUTION_FAILED, lc.AgreementProtocol, lc.AgreementId, hd)
				return true
			} else {
				w.Messages() <- events.NewWorkloadMessage(events.EXECUTION_BEGUN, lc.AgreementProtocol, lc.AgreementId, hd)
			}

		}

	case *UnInstallCommand:

		cmd := command.(*UnInstallCommand)
		glog.V(5).Infof(hpwlog(fmt.Sprintf("uninstalling %v", cmd.Deployment)))

		// Make sure it's a Helm deployment.
		hdc, ok := cmd.Deployment.(*persistence.HelmDeploymentConfig)
		if !ok {
			glog.Warningf(hpwlog(fmt.Sprintf("ignoring non-Helm deployment: %v", cmd.Deployment)))
			return true
		} else if err := w.uninstallHelmPackage(hdc); err != nil {
			// Since we have a Helm deployment package, uninstall it.
			glog.Errorf(hpwlog(fmt.Sprintf("failed to uninstall helm package after agreement cancellation: %v", err)))
		}

		w.Messages() <- events.NewWorkloadMessage(events.WORKLOAD_DESTROYED, cmd.AgreementProtocol, cmd.CurrentAgreementId, hdc)

	case *MaintenanceCommand:
		cmd := command.(*MaintenanceCommand)
		glog.V(3).Infof(hpwlog(fmt.Sprintf("received maintenance command: %v", cmd)))

		hdc, ok := cmd.Deployment.(*persistence.HelmDeploymentConfig)
		if !ok {
			glog.Warningf(hpwlog(fmt.Sprintf("ignoring non-Helm maintenance command: %v", cmd)))
			return true
		} else if err := w.releaseStatus(hdc, "DEPLOYED"); err != nil {
			glog.Errorf(hpwlog(fmt.Sprintf("%v", err)))
			// Ask governer to cancel the agreement.
			w.Messages() <- events.NewWorkloadMessage(events.EXECUTION_FAILED, cmd.AgreementProtocol, cmd.AgreementId, hdc)
		}

	default:
		return false
	}
	return true

}

func (w *HelmWorker) getLaunchContext(launchContext interface{}) *events.AgreementLaunchContext {
	switch launchContext.(type) {
	case *events.AgreementLaunchContext:
		lc := launchContext.(*events.AgreementLaunchContext)
		return lc
	}
	return nil
}

func (w *HelmWorker) processHelmPackage(launchContext *events.AgreementLaunchContext, hd *persistence.HelmDeploymentConfig) error {

	glog.V(5).Infof(hpwlog(fmt.Sprintf("begin install of Helm Deployment release %v", hd.ReleaseName)))

	// TODO: Verify signature

	c := NewHelmClient()
	if err := c.Install(hd.ChartArchive, hd.ReleaseName); err != nil {
		return errors.New(fmt.Sprintf("unable to install Helm package %v, error: %v", hd, err))
	}

	glog.V(5).Infof(hpwlog(fmt.Sprintf("completed install of Helm Deployment release %v", hd.ReleaseName)))

	return nil
}

func (w *HelmWorker) uninstallHelmPackage(hd *persistence.HelmDeploymentConfig) error {

	glog.V(5).Infof(hpwlog(fmt.Sprintf("begin uninstall of Helm Deployment release %v", hd.ReleaseName)))

	c := NewHelmClient()
	if err := c.UnInstall(hd.ReleaseName); err != nil {
		return errors.New(fmt.Sprintf("unable to uninstall Helm package %v, error: %v", hd, err))
	}

	glog.V(5).Infof(hpwlog(fmt.Sprintf("completed uninstall of Helm Deployment release %v", hd.ReleaseName)))

	return nil
}

func (w *HelmWorker) releaseStatus(hd *persistence.HelmDeploymentConfig, desiredStatus string) error {

	glog.V(5).Infof(hpwlog(fmt.Sprintf("begin listing Helm Deployment release %v", hd.ReleaseName)))

	c := NewHelmClient()
	status, err := c.Status(hd.ReleaseName)
	if err != nil {
		return errors.New(fmt.Sprintf("unable to list Helm release %v, error: %v", hd.ReleaseName, err))
	} else if status.Status != desiredStatus {
		return errors.New(fmt.Sprintf("Helm release %v is not in desiredStatus %v, in %v", hd.ReleaseName, desiredStatus, status))
	}

	glog.V(5).Infof(hpwlog(fmt.Sprintf("completed listing Helm Deployment release %v, status %v", hd.ReleaseName, status)))

	return nil
}

var hpwlog = func(v interface{}) string {
	return fmt.Sprintf("Helm Package Worker: %v", v)
}
