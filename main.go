package main

import (
	"flag"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/agreement"
	"github.com/open-horizon/anax/agreementbot"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/container"
	"github.com/open-horizon/anax/ethblockchain"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/governance"
	"github.com/open-horizon/anax/policy"
	"github.com/open-horizon/anax/torrent"
	"github.com/open-horizon/anax/whisper"
	"github.com/open-horizon/anax/worker"
	"os"
	"os/signal"
	"path"
	"runtime"
	"runtime/pprof"
	"syscall"
	"time"
)

// This function combines all messages (events) from workers into a single global message quueue. From this
// global queue, each message will get delivered to each worker by the event handler function.
//
func mux(workers *worker.MessageHandlerRegistry) chan events.Message {

	muxed := make(chan events.Message)

	go func() {
		// continually combine input from each by writing Messages to 'muxed' shared channel

		for {
			for _, w := range workers.Handlers {
				select {
				case ev := <-(*w).Messages():
					muxed <- ev
				default: // nothing
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
	}() // immediately invoked, start operating on input

	return muxed
}

// eventHandler Main control flow area: receives incoming Message messages and operates on them by pushing them
// out to each worker. Workers then receive messages and, for messages they care about, the worker pushes out as commands
// onto their own channels to operate on them.
//
func eventHandler(incoming events.Message, workers *worker.MessageHandlerRegistry) (string, error) {
	successMsg := "propagated event to destination worker"

	for _, worker := range workers.Handlers {
		(*worker).NewEvent(incoming)
	}

	return successMsg, nil
}

// The core of anax is an event handling system that distributes events to workers, where the workers
// process events that they are about. However, to get started, anax needs to do a bunch of initialization
// tasks. The config file has to be read in, the databases have to get created, and then the eventing system
// and the workers can be fired up.
//
func main() {
	configFile := flag.String("config", "/etc/colonus/anax.config", "Config file location")
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to file")

	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			panic(err)
		}
		pprof.StartCPUProfile(f)
		glog.V(2).Infof("Started CPU profiling. Writing to: %v", f.Name())
	}

	config, err := config.Read(*configFile)
	if err != nil {
		panic(err)
	}
	glog.V(2).Infof("Using config: %v", config)
	glog.V(2).Infof("GOMAXPROCS: %v", runtime.GOMAXPROCS(-1))

	// open edge DB if necessary
	var db *bolt.DB
	if len(config.Edge.DBPath) != 0 {
		if err := os.MkdirAll(config.Edge.DBPath, 0700); err != nil {
			panic(err)
		}

		edgeDB, err := bolt.Open(path.Join(config.Edge.DBPath, "anax.db"), 0600, &bolt.Options{Timeout: 10 * time.Second})
		if err != nil {
			panic(err)
		}
		db = edgeDB
	}

	// open Agreement Bot DB if necessary
	var agbotdb *bolt.DB
	if len(config.AgreementBot.DBPath) != 0 {
		if err := os.MkdirAll(config.AgreementBot.DBPath, 0700); err != nil {
			panic(err)
		}

		agdb, err := bolt.Open(path.Join(config.AgreementBot.DBPath, "agreementbot.db"), 0600, &bolt.Options{Timeout: 10 * time.Second})
		if err != nil {
			panic(err)
		}
		agbotdb = agdb
	}

	// start control signal handler
	control := make(chan os.Signal, 1)
	signal.Notify(control, os.Interrupt)
	signal.Notify(control, syscall.SIGTERM)
	go func() {
		<-control
		glog.Infof("Closing up shop.")

		pprof.StopCPUProfile()
		if db != nil {
			db.Close()
		}
		if agbotdb != nil {
			agbotdb.Close()
		}

		os.Exit(0)
	}()

	// start API server
	var apiServer *api.API
	if db != nil {
		apiServer = api.NewAPIListener(config, db)
	}

	// block here on blockchain account balance ; TODO: generalize, make a part of all work with ethereum in lib
	// TODO: probably this should be in a worker that issues an event when funding is complete.
	funded := false
	gethURL := ""
	if len(config.Edge.GethURL) != 0 {
		gethURL = config.Edge.GethURL
	} else {
		gethURL = config.AgreementBot.GethURL
	}
	now := func() int64 { return time.Now().Unix() }
	printed := now() - 31

	for !funded {
		e := now()
		if e-printed > 30 {
			glog.Infof("Waiting for account to be funded")
			printed = e
		}

		var err error
		if funded, err = ethblockchain.AccountFunded(gethURL); err != nil {
			// bury these because they're expected for some time up-front
			glog.V(4).Infof("Account not yet funded: %v", err)
		}
		time.Sleep(900 * time.Millisecond)
	}

	// Get the device side policy manager started early so that all the workers can use it.
	// Make sure the policy directory is in place.
	var pm *policy.PolicyManager
	if config.Edge.PolicyPath == "" {
		// nothing to initialize
	} else if err := os.MkdirAll(config.Edge.PolicyPath, 0644); err != nil {
		glog.Errorf("Cannot create edge policy file path %v, terminating.", config.Edge.PolicyPath)
		panic(err)
	} else if policyManager, err := policy.Initialize(config.Edge.PolicyPath); err != nil {
		glog.Errorf("Unable to initialize policy manager, terminating.")
		panic(err)
	} else {
		pm = policyManager
	}

	// start workers
	workers := worker.NewMessageHandlerRegistry()

	workers.Add("whisper", whisper.NewWhisperWorker(config))
	workers.Add("agreementBot", agreementbot.NewAgreementBotWorker(config, agbotdb))

	if db != nil {
		workers.Add("api", apiServer)
		workers.Add("agreement", agreement.NewAgreementWorker(config, db, pm))
		workers.Add("torrent", torrent.NewTorrentWorker(config))
		workers.Add("container", container.NewContainerWorker(config))
		workers.Add("governance", governance.NewGovernanceWorker(config, db, pm))
	}

	messageStream := mux(workers)

	last := int64(0)

	for {

		select {
		case msg := <-messageStream:
			glog.V(3).Infof("Handling Message (%T): %v\n", msg, msg)

			if successMsg, err := eventHandler(msg, workers); err != nil {
				// error! do some barfing and then continue
				glog.Errorf("Error occurred handling message: %s, Error: %v\n", msg, err)
			} else {
				glog.V(2).Infof("Success handling message: %s\n", successMsg)
			}
		default:
			now := time.Now().Unix()
			if now-last > 30 {
				glog.V(5).Infof("No incoming messages for router to handle")
				last = now
			}
		}

		time.Sleep(400 * time.Millisecond)
	}
}
