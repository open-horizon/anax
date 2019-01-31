package resource

import (
//	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/exchange"
	"github.ibm.com/edge-sync-service/edge-sync-service/common"
	"github.ibm.com/edge-sync-service/edge-sync-service/core/base"
	"github.ibm.com/edge-sync-service/edge-utilities/logger"
	"github.ibm.com/edge-sync-service/edge-utilities/logger/log"
	"github.ibm.com/edge-sync-service/edge-utilities/logger/trace"
	"os"
)

type ResourceManager struct {
	config *config.HorizonConfig
	pattern string
	id string
	org string
}

func NewResourceManager(cfg *config.HorizonConfig) *ResourceManager {
	return &ResourceManager{
		config: cfg,
	}
}

func (r *ResourceManager) NodeConfigUpdate(pattern string, id string, org string) {
	r.pattern = pattern
	r.id = id
	r.org = org
}

func (r ResourceManager) String() string {
//	return fmt.Sprintf("Loaded Resources: %v", *r.loadedResources)
	return "ResourceManager:"
}

func (r ResourceManager) StartFileSyncService() error {

	common.Configuration.NodeType = "ESS"
	common.Configuration.DestinationType = exchange.GetId(r.pattern)
	common.Configuration.DestinationID = r.id
	common.Configuration.OrgID = r.org
	common.Configuration.ListeningType = "unsecure"
	common.Configuration.ListeningAddress = "localhost"
	common.Configuration.UnsecureListeningPort = 8090
	common.Configuration.CommunicationProtocol = "http"
	common.Configuration.PersistenceRootPath = r.config.GetFileSyncServiceStoragePath()
	common.Configuration.HTTPCSSHost = "css-api"
	common.Configuration.HTTPCSSPort = 8500
	common.Configuration.LogRootPath = "/tmp/"
	common.Configuration.LogTraceDestination = "stdout,file"
	common.Configuration.LogFileName = "fss"
	common.Configuration.TraceRootPath = "/tmp/trace/"
	common.Configuration.MongoAddressCsv ="css-db:27017"
	common.Configuration.MongoAuthDbName = "authdb"
	common.Configuration.ESSPersistentStorage = true

	common.HTTPCSSURL = fmt.Sprintf("%s://%s:%d", common.Configuration.CommunicationProtocol, common.Configuration.HTTPCSSHost, common.Configuration.HTTPCSSPort)

	logFileSize, err := logger.AdjustMaxLogfileSize(common.Configuration.LogTraceFileSizeKB, common.DefaultLogTraceFileSize, common.Configuration.LogRootPath)
	if err != nil {
		fmt.Printf("WARNING: Unable to get disk statistics for the path %s. Error: %s\n", common.Configuration.LogRootPath, err)
	}

	parameters := logger.Parameters{
		RootPath:                 common.Configuration.LogRootPath,
		FileName:                 common.Configuration.LogFileName,
		MaxFileSize:              logFileSize,
		MaxCompressedFilesNumber: common.Configuration.MaxCompressedlLogTraceFilesNumber,
		Destinations:             common.Configuration.LogTraceDestination,
		Prefix:                   common.Configuration.NodeType + ": ",
		Level:                    common.Configuration.LogLevel,
		MaintenanceInterval:      common.Configuration.LogTraceMaintenanceInterval,
	}
	if err = log.Init(parameters); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize the log. Error: %s\n", err)
		os.Exit(98)
	}
	defer log.Stop()

	logFileSize, err = logger.AdjustMaxLogfileSize(common.Configuration.LogTraceFileSizeKB, common.DefaultLogTraceFileSize,	common.Configuration.TraceRootPath)
	if err != nil {
		fmt.Printf("WARNING: Unable to get disk statistics for the path %s. Error: %s\n", common.Configuration.TraceRootPath, err)
	}

	parameters.RootPath = common.Configuration.TraceRootPath
	parameters.FileName = common.Configuration.TraceFileName
	parameters.Level = common.Configuration.TraceLevel
	parameters.MaxFileSize = logFileSize
	if err = trace.Init(parameters); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize the trace. Error: %s\n", err)
		os.Exit(98)
	}
	defer trace.Stop()

	glog.Infof(rmLogString(fmt.Sprintf("ESS Config: %v", common.Configuration)))
	censorAndDumpConfig()

	err = base.Start("", true)
	if err != nil {
		if log.IsLogging(logger.FATAL) {
			log.Fatal(err.Error())
		}
		glog.Errorf(rmLogString(fmt.Sprintf("ESS Start error: %v", err)))
	}

	glog.Infof(rmLogString(fmt.Sprintf("ESS Started")))

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

// Logging function
var rmLogString = func(v interface{}) string {
	return fmt.Sprintf("Resource Manager: %v", v)
}