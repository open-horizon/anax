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
	"os"
)

type ResourceManager struct {
	config *config.HorizonConfig
	org string
	pattern string
	id string
	token string
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
}

func (r ResourceManager) String() string {
	return fmt.Sprintf("ResourceManager: Org %v"+
		", Pattern: %v"+
		", ID: %v"+
		", Token: %v",
		r.org, r.pattern, r.id, r.token)
}

func (r ResourceManager) StartFileSyncService(am *AuthenticationManager) error {

	// Configure the embedded ESS using configuration from the node.
	common.Configuration.NodeType = "ESS"
	common.Configuration.DestinationType = exchange.GetId(r.pattern)
	common.Configuration.DestinationID = r.id
	common.Configuration.OrgID = r.org
	common.Configuration.ListeningType = r.config.GetFileSyncServiceProtocol()
	common.Configuration.ListeningAddress = r.config.GetFileSyncServiceAPIListen()
	common.Configuration.SecureListeningPort = r.config.GetFileSyncServiceAPIPort()
	common.Configuration.CommunicationProtocol = "http"
	common.Configuration.HTTPPollingInterval = r.config.GetESSPollingRate()
	common.Configuration.PersistenceRootPath = r.config.GetFileSyncServiceStoragePath()
	common.Configuration.HTTPCSSHost = r.config.GetCSSURL()
	common.Configuration.HTTPCSSPort = r.config.GetCSSPort()
	common.Configuration.HTTPCSSUseSSL = true
	common.Configuration.HTTPCSSCACertificate = r.config.GetCSSSSLCert()
	common.Configuration.LogTraceDestination = "glog"

	if glog.V(5) {
		common.Configuration.LogLevel = "TRACE"
		common.Configuration.TraceLevel = "TRACE"
	} else {
		common.Configuration.LogLevel = "INFO"
		common.Configuration.TraceLevel = "INFO"
	}

	common.Configuration.ESSPersistentStorage = true

	// Generate a self signed certificate to be used for TLS between a service and the embedded ESS API.
	// The SSL private key is stored in a different location from the certificate so that the services
	// only have access to the certificate with the public key.
	//
	// This function directly modifies the ESS common.Configuration object, which is also set below.
	if err := CreateCertificate(r.org, r.config.GetESSSSLCertKeyPath(), r.config.GetESSSSLClientCertPath()); err != nil {
		return errors.New(fmt.Sprintf("unable to create SSL certificate for ESS API, error %v", err))
	}

	// Set the fully formed CSS API URL in the global configuration object.
	common.HTTPCSSURL = fmt.Sprintf("%ss://%s:%d", common.Configuration.CommunicationProtocol, common.Configuration.HTTPCSSHost, common.Configuration.HTTPCSSPort)

	// Init the sync service log and trace.
	parameters := logger.Parameters{
		Destinations:             common.Configuration.LogTraceDestination,
		Prefix:                   common.Configuration.NodeType + ": ",
		Level:                    common.Configuration.LogLevel,
		MaintenanceInterval:      common.Configuration.LogTraceMaintenanceInterval,
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
	glog.V(3).Infof(rmLogString(fmt.Sprintf("ESS Config: %v", common.Configuration)))
	censorAndDumpConfig()

	// Set the authenticator that we're going to use.
	security.SetAuthentication(&FSSAuthenticate{nodeOrg:r.org, nodeID:r.id, nodeToken:r.token, AuthMgr:am})

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
		base.Stop(20)
		glog.Infof(rmLogString(fmt.Sprintf("ESS Stopped")))
	}
}

// Logging function
var rmLogString = func(v interface{}) string {
	return fmt.Sprintf("Resource Manager: %v", v)
}