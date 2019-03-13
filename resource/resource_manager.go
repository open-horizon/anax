package resource

import (
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/edge-sync-service/common"
	"github.com/open-horizon/edge-sync-service/core/base"
	"github.com/open-horizon/edge-sync-service/core/security"
	"github.com/open-horizon/edge-utilities/logger"
	"github.com/open-horizon/edge-utilities/logger/log"
	"github.com/open-horizon/edge-utilities/logger/trace"
	"io/ioutil"
	"os"
	"path"
	"time"
)

type ResourceManager struct {
	config  *config.HorizonConfig
	org     string
	pattern string
	id      string
	token   string
}

func NewResourceManager(cfg *config.HorizonConfig) *ResourceManager {
	return &ResourceManager{
		config: cfg,
	}
}

func (r *ResourceManager) NodeConfigUpdate(org string, pattern string, id string, token string) {
	r.pattern = pattern
	r.id = id
	r.org = org
	r.token = token
}

func (r ResourceManager) String() string {
	return fmt.Sprintf("ResourceManager: Org %v"+
		", Pattern: %v"+
		", ID: %v"+
		", Token: %v",
		r.org, r.pattern, r.id, r.token)
}

func (r ResourceManager) StartFileSyncService(am *AuthenticationManager) error {

	// Generate a self signed certificate to be used for TLS between a service and the embedded ESS API.
	// The SSL private key is stored in a different location from the certificate so that the services
	// only have access to the certificate with the public key.
	//
	// This function directly modifies the ESS common.Configuration object, which is also set below.
	if err := CreateCertificate(r.org, r.config.GetESSSSLCertKeyPath(), r.config.GetESSSSLClientCertPath()); err != nil {
		return errors.New(fmt.Sprintf("unable to create SSL certificate for ESS API, error %v", err))
	}

	// In order to override the ESS SSL certificate and key that the ESS uses to listen on the ESS API,
	// we have to pass our certificate and key into the ESS config by value, as a string of bytes.

	certFile := path.Join(r.config.GetESSSSLClientCertPath(), config.HZN_FSS_CERT_FILE)
	certKeyFile := path.Join(r.config.GetESSSSLCertKeyPath(), config.HZN_FSS_CERT_KEY_FILE)

	if essCert, err := os.Open(certFile); err != nil {
		return errors.New(fmt.Sprintf("unable to open ESS SSL Certificate file %v, error %v", r.config.GetESSSSLClientCertPath(), err))
	} else if essCertBytes, err := ioutil.ReadAll(essCert); err != nil {
		return errors.New(fmt.Sprintf("unable to read ESS SSL Certificate file %v, error %v", r.config.GetESSSSLClientCertPath(), err))
	} else if essCertKey, err := os.Open(certKeyFile); err != nil {
		return errors.New(fmt.Sprintf("unable to open ESS SSL Certificate Key file %v, error %v", r.config.GetESSSSLCertKeyPath(), err))
	} else if essCertKeyBytes, err := ioutil.ReadAll(essCertKey); err != nil {
		return errors.New(fmt.Sprintf("unable to read ESS SSL Certificate Key file %v, error %v", r.config.GetESSSSLCertKeyPath(), err))
	} else {

		// Configure the embedded ESS using configuration from the node.
		common.Configuration.NodeType = "ESS"
		common.Configuration.DestinationType = exchange.GetId(r.pattern)
		common.Configuration.DestinationID = r.id
		common.Configuration.OrgID = r.org
		common.Configuration.ListeningType = r.config.GetFileSyncServiceProtocol()
		common.Configuration.ListeningAddress = r.config.GetFileSyncServiceAPIListen()
		common.Configuration.SecureListeningPort = r.config.GetFileSyncServiceAPIPort()
		common.Configuration.ServerCertificate = string(essCertBytes)
		common.Configuration.ServerKey = string(essCertKeyBytes)
		common.Configuration.CommunicationProtocol = "http"
		common.Configuration.HTTPPollingInterval = r.config.GetESSPollingRate()
		common.Configuration.PersistenceRootPath = r.config.GetFileSyncServiceStoragePath()
		common.Configuration.HTTPCSSUseSSL = true
		common.Configuration.HTTPCSSCACertificate = r.config.GetCSSSSLCert()
		common.Configuration.LogTraceDestination = "glog"
	}

	if glog.V(5) {
		common.Configuration.LogLevel = "TRACE"
		common.Configuration.TraceLevel = "TRACE"
	} else {
		common.Configuration.LogLevel = "INFO"
		common.Configuration.TraceLevel = "INFO"
	}

	common.Configuration.ESSPersistentStorage = true

	// Set the fully formed CSS API URL in the global configuration object.
	common.HTTPCSSURL = r.config.GetCSSURL()

	// Init the sync service log and trace.
	parameters := logger.Parameters{
		Destinations:        common.Configuration.LogTraceDestination,
		Prefix:              common.Configuration.NodeType + ": ",
		Level:               common.Configuration.LogLevel,
		MaintenanceInterval: common.Configuration.LogTraceMaintenanceInterval,
	}
	if err := log.Init(parameters); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize the log. Error: %s\n", err)
		os.Exit(98)
	}
	defer log.Stop()

	parameters.Level = common.Configuration.TraceLevel
	if err := trace.Init(parameters); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize the trace. Error: %s\n", err)
		os.Exit(98)
	}
	defer trace.Stop()

	// Log the embedded ESS config now that it's complete.
	glog.V(5).Infof(rmLogString(fmt.Sprintf("ESS Config: %v", common.Configuration)))
	censorAndDumpConfig()

	// Set the authenticator that we're going to use.
	security.SetAuthentication(&FSSAuthenticate{nodeOrg: r.org, nodeID: r.id, nodeToken: r.token, AuthMgr: am})

	// Start the embedded ESS.
	if err := base.Start("", true); err != nil {
		glog.Errorf(rmLogString(fmt.Sprintf("ESS Start error: %v", err)))
		os.Exit(98)
	}

	glog.V(3).Infof(rmLogString(fmt.Sprintf("ESS Started")))

	return nil

}

func censorAndDumpConfig() {
	toBeCensored := []*string{&common.Configuration.ServerCertificate, &common.Configuration.ServerKey,
		&common.Configuration.HTTPCSSCACertificate,
		&common.Configuration.MQTTUserName, &common.Configuration.MQTTPassword,
		&common.Configuration.MQTTCACertificate, &common.Configuration.MQTTSSLCert, &common.Configuration.MQTTSSLKey,
		&common.Configuration.MongoUsername, &common.Configuration.MongoPassword, &common.Configuration.MongoCACertificate}
	backups := make([]string, len(toBeCensored))

	for index, fieldPointer := range toBeCensored {
		backups[index] = *fieldPointer
		if len(*fieldPointer) != 0 {
			*fieldPointer = "<...>"
		}
	}

	trace.Dump("Loaded configuration:", common.Configuration)

	for index, fieldPointer := range toBeCensored {
		*fieldPointer = backups[index]
	}
}

func (r ResourceManager) StopFileSyncService() {
	if r.pattern != "" {
		glog.Infof(rmLogString(fmt.Sprintf("ESS Stopping")))

		// Use a channel to communicate that ESS stop is complete.
		stopChan := make(chan bool)
		done := false

		// Initiate the ESS stop in a go routine in case it hangs.
		go func() {
			base.Stop(0)
			stopChan <- true
		}()

		// Give the ESS 3 seconds to shutdown
		timerChan := time.NewTimer(time.Duration(3) * time.Second).C

		// Wait for either our timer to expire or for the ESS to indicate that it is stopped.
		for !done {
			select {
			case <- timerChan:
			    glog.Warningf(rmLogString(fmt.Sprintf("Embedded ESS Stop timer expired while waiting for the ESS to stop, continuing with termination.")))
			    done = true
			case <- stopChan:
			    glog.V(5).Infof(rmLogString(fmt.Sprintf("Embedded ESS Stop completed.")))
			    done = true
			    return
			}
		}

		// Complete the final steps of cleanup.
		r.RemovePersistencePath()
		glog.Infof(rmLogString(fmt.Sprintf("ESS Stopped")))
	}
}

// Remove any remaining FSS objects from the Agent's host file system.
func (r *ResourceManager) RemovePersistencePath() {
	syncPath := path.Join(r.config.GetFileSyncServiceStoragePath(), "sync")
	if err := os.RemoveAll(syncPath); err != nil {
		glog.Errorf(rmLogString(fmt.Sprintf("unable to remove file sync service persistence path %v, error: %v", syncPath, err)))
	}
}

// Logging function
var rmLogString = func(v interface{}) string {
	return fmt.Sprintf("Resource Manager: %v", v)
}
