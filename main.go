package main

import (
	"flag"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/agreement"
	"github.com/open-horizon/anax/agreementbot"
	agbotPersistence "github.com/open-horizon/anax/agreementbot/persistence"
	_ "github.com/open-horizon/anax/agreementbot/persistence/bolt"
	_ "github.com/open-horizon/anax/agreementbot/persistence/postgresql"
	agbotSecretsImpl "github.com/open-horizon/anax/agreementbot/secrets"
	_ "github.com/open-horizon/anax/agreementbot/secrets/vault"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/changes"
	"github.com/open-horizon/anax/clusterupgrade"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/container"
	"github.com/open-horizon/anax/download"
	"github.com/open-horizon/anax/exchange"
	_ "github.com/open-horizon/anax/externalpolicy/text_language"
	"github.com/open-horizon/anax/governance"
	"github.com/open-horizon/anax/i18n"
	_ "github.com/open-horizon/anax/i18n_messages"
	"github.com/open-horizon/anax/imagefetch"
	"github.com/open-horizon/anax/kube_operator"
	"github.com/open-horizon/anax/nodemanagement"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/resource"
	"github.com/open-horizon/anax/worker"
	"os"
	"os/signal"
	"path"
	"runtime"
	"runtime/pprof"
	"syscall"
	"time"
)

// The core of anax is an event handling system that distributes events to workers, where the workers
// process events that they are about. However, to get started, anax needs to do a bunch of initialization
// tasks. The config file has to be read in, the databases have to get created, and then the eventing system
// and the workers can be fired up.
func main() {
	configFile := flag.String("config", "/etc/colonus/anax.config", "Config file location")
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to file")

	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			panic(err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			panic(err)
		}
		glog.V(2).Infof("Started CPU profiling. Writing to: %v", f.Name())
	}

	cfg, err := config.Read(*configFile)
	if err != nil {
		panic(err)
	}
	glog.V(2).Infof("Using config: %v", cfg.String())
	glog.V(2).Infof("GOMAXPROCS: %v", runtime.GOMAXPROCS(-1))

	// initialize the message printer for globalization, the anax will produce English messages.
	// However, in order to extract messages for eventlog for translation, we need to use the message printer for
	// eventlog messages.
	i18n.InitMessagePrinter(true)

	// open edge DB if necessary
	var db *bolt.DB
	if len(cfg.Edge.DBPath) != 0 {
		if err := os.MkdirAll(cfg.Edge.DBPath, 0700); err != nil {
			panic(err)
		}

		edgeDB, err := bolt.Open(path.Join(cfg.Edge.DBPath, "anax.db"), 0600, &bolt.Options{Timeout: 10 * time.Second})
		if err != nil {
			panic(err)
		}
		db = edgeDB
	}

	// open Agreement Bot DB if necessary

	var agbotDB agbotPersistence.AgbotDatabase
	agbotDB, dberr := agbotPersistence.InitDatabase(cfg)
	if db == nil && dberr != nil {
		panic(fmt.Sprintf("Unable to initialize Agreement Bot: %v", dberr))
	} else if db != nil && dberr != nil {
		glog.Warningf("Unable to initialize Agreement Bot database on this node: %v", dberr)
	}

	// Initialize the secrets implementation
	var agbotSecrets agbotSecretsImpl.AgbotSecrets
	as, aserr := agbotSecretsImpl.InitSecrets(cfg)
	// If the agbot is configured, then check if there is a secrets plugin. If not, just issue a warning message.
	if agbotDB != nil && aserr != nil {
		glog.Warningf("Unable to initialize secrets plugin, continuing without secret support: %v", aserr)
	}
	agbotSecrets = as

	// start control signal handler
	control := make(chan os.Signal, 1)
	signal.Notify(control, os.Interrupt)
	signal.Notify(control, syscall.SIGTERM)

	// This routine does not need to be a subworker because it has no parent worker and it will terminate on its own
	// when the main anax process terminates.
	go func() {
		<-control
		glog.Infof("Closing up shop.")

		pprof.StopCPUProfile()
		if db != nil {
			db.Close()
			// remove the local db
			dbFile := path.Join(cfg.Edge.DBPath, "anax.db")
			glog.Infof("Removing local db file %v.", dbFile)
			if persistence.GetRemoveDatabaseOnExit() {
				if err := os.Remove(dbFile); err != nil {
					glog.Infof("Error Removing local db file %v, error %v", dbFile, err)
				}
			}
		}
		if agbotDB != nil {
			agbotDB.Close()
		}

		os.Exit(0)
	}()

	// The anax runtime might have been upgraded an restarted with an existing database. If so, the
	// device object might need to be upgraded.
	usingPattern := false
	if usingPattern, err = persistence.MigrateExchangeDevice(db); err != nil {
		panic(err)
	}

	// Get the device side policy manager started early so that all the workers can use it.
	// Make sure the policy directory is in place.
	var pm *policy.PolicyManager
	if cfg.Edge.PolicyPath == "" {
		// nothing to initialize
	} else if err := os.MkdirAll(cfg.Edge.PolicyPath, 0644); err != nil {
		glog.Errorf("Cannot create edge policy file path %v, terminating.", cfg.Edge.PolicyPath)
		panic(err)
	} else if policyManager, err := policy.Initialize(cfg.Edge.PolicyPath, cfg.ArchSynonyms, nil, !usingPattern, true); err != nil {
		glog.Errorf("Unable to initialize policy manager, terminating.")
		panic(err)
	} else {
		pm = policyManager
	}

	// Initialize the shared authentication manager for service containers to authentication to the agent.
	authm := resource.NewAuthenticationManager(cfg.GetFileSyncServiceAuthPath())

	// Initialize the secrets manager to store secrets in the local db and in agent file system.
	secretm := resource.NewSecretsManager(cfg.GetSecretsManagerFilePath(), db)

	// start workers
	workers := worker.NewMessageHandlerRegistry()

	workers.Add(agreementbot.NewAgreementBotWorker("AgBot", cfg, agbotDB, agbotSecrets))
	if cfg.AgreementBot.APIListen != "" {
		workers.Add(agreementbot.NewAPIListener("AgBot API", cfg, agbotDB, *configFile, agbotSecrets))
	}
	if cfg.AgreementBot.SecureAPIListenHost != "" {
		workers.Add(agreementbot.NewSecureAPIListener("AgBot Secure API", cfg, agbotDB, agbotSecrets))
	}
	if agbotDB != nil {
		workers.Add(agreementbot.NewChangesWorker("AgBot ExchangeChanges", cfg))
	}

	if db != nil {
		workers.Add(api.NewAPIListener("API", cfg, db, pm))
		workers.Add(agreement.NewAgreementWorker("Agreement", cfg, db, pm))
		workers.Add(governance.NewGovernanceWorker("Governance", cfg, db, pm))
		workers.Add(exchange.NewExchangeMessageWorker("ExchangeMessages", cfg, db))
		if containerWorker := container.NewContainerWorker("Container", cfg, db, authm, secretm); containerWorker != nil {
			workers.Add(containerWorker)
		}
		if imageWorker := imagefetch.NewImageFetchWorker("ImageFetch", cfg, db); imageWorker != nil {
			workers.Add(imageWorker)
		}
		workers.Add(kube_operator.NewKubeWorker("Kube", cfg, db))
		workers.Add(resource.NewResourceWorker("Resource", cfg, db, authm))
		workers.Add(changes.NewChangesWorker("ExchangeChanges", cfg, db))
		workers.Add(nodemanagement.NewNodeManagementWorker("NodeManagement", cfg, db))
		workers.Add(download.NewDownloadWorker("Download", cfg, db))

		// add cluster upgrade worker only when it is edge cluster
		if cfg.Edge.DockerEndpoint == "" {
			workers.Add(clusterupgrade.NewClusterUpgradeWorker("ClusterUpgrade", cfg, db))
		}
	}

	// Get into the event processing loop until anax shuts itself down.
	workers.ProcessEventMessages()

	if db != nil {
		db.Close()

		// remove the local db
		dbFile := path.Join(cfg.Edge.DBPath, "anax.db")
		glog.Infof("Removing local db file %v.", dbFile)
		if persistence.GetRemoveDatabaseOnExit() {
			if err := os.Remove(dbFile); err != nil {
				glog.Infof("Error Removing local db file %v, error %v", dbFile, err)
			}
		}
	}

	if agbotDB != nil {
		agbotDB.Close()
	}

	glog.Info("Main process terminating")
}
