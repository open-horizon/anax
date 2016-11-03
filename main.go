package main

import (
	"flag"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/blockchain"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/container"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/governance"
	"github.com/open-horizon/anax/torrent"
	"github.com/open-horizon/anax/whisper"
	"os"
	"os/signal"
	"path"
	"runtime"
	"runtime/pprof"
	"syscall"
	"time"
)

func mux(apiMessages, blockchainMessages, whisperMessages, torrentMessages, containerMessages, governanceMessages <-chan events.Message) chan events.Message {
	// TODO:
	// 1. refactor: reduce code volume by eliminating typed *Workers w/ members; prefer instead instances of single Worker type w/ standalone functions in container, blockchain, torrent modules to do work currently done in methods on typed workers
	// 2. refactor: eliminate container message boilerplate. Still needs types ('cause need handler function to have general Message type in signature), but maybe there can be one factory in the events module that can instantiate typed messages; could be hairy since they have different members

	muxed := make(chan events.Message)

	go func() {
		// continually combine input from each by writing Messages to 'muxed' shared channel
		for {
			select {
			case ev := <-apiMessages:
				muxed <- ev

			case ev := <-blockchainMessages:
				muxed <- ev

			case ev := <-whisperMessages:
				muxed <- ev

			case ev := <-torrentMessages:
				muxed <- ev

			case ev := <-containerMessages:
				muxed <- ev

			case ev := <-governanceMessages:
				muxed <- ev
			default: // nothing
			}
			time.Sleep(100 * time.Millisecond)
		}
	}() // immediately invoked, start operating on input

	return muxed
}

// eventHandler Main control flow area: receives incoming Message messages and operates on them by constructing worker-specific control messages to do work. Workers then receive messages over their own channels and operate on them.
func eventHandler(incoming events.Message, blockchainWorker *blockchain.BlockchainWorker, whisperWorker *whisper.WhisperWorker, torrentWorker *torrent.TorrentWorker, containerWorker *container.ContainerWorker, governanceWorker *governance.GovernanceWorker) (string, error) {
	successMsg := "propagated event to destination worker"

	// TODO: consider factoring out some of the common switch work on event types to clarify event handling

	// TODO: handle command instantiation errors here

	switch incoming.(type) {
	case *blockchain.BlockchainMessage:
		msg, _ := incoming.(*blockchain.BlockchainMessage)

		switch msg.Event().Id {
		case events.CONTRACT_ACCEPTED:
			cmd := whisperWorker.NewAnnounceCommand(msg.Agreement.EstablishedContract.ContractAddress, msg.Agreement.EstablishedContract.CurrentAgreementId, msg.Agreement.WhisperId, msg.Agreement.EstablishedContract.ConfigureNonce)
			whisperWorker.Commands <- cmd

		case events.CONTRACT_ENDED:
			containerCmd := containerWorker.NewContainerShutdownCommand(msg.Agreement.EstablishedContract.ContractAddress, msg.Agreement.EstablishedContract.CurrentAgreementId, &msg.Agreement.EstablishedContract.CurrentDeployment, []string{})
			containerWorker.Commands <- containerCmd

		default:
			return "", fmt.Errorf("Unsupported event: %v", incoming.Event().Id)
		}
	case *blockchain.BlockchainRegMessage:
		msg, _ := incoming.(*blockchain.BlockchainRegMessage)

		switch msg.Event().Id {
		case events.CONTRACT_REGISTERED:
			// use local governor for development mode

		default:
			return "", fmt.Errorf("Unsupported event: %v", incoming.Event().Id)
		}

	case *whisper.WhisperConfigMessage:
		msg, _ := incoming.(*whisper.WhisperConfigMessage)

		switch msg.Event().Id {
		case events.DIRECT_CONFIGURE:
			cmd := torrentWorker.NewFetchCommand(msg.AgreementLaunchContext)
			torrentWorker.Commands <- cmd

		case events.CONFIGURE_ERROR:
			// amounts to a configure security failure or other error from the perspective of the blockchain worker
			blockchainCmd := blockchainWorker.NewBlockchainEndContractCommand(events.CT_ERROR, msg.AgreementLaunchContext.ContractId, msg.AgreementLaunchContext.AgreementId)
			blockchainWorker.Commands <- blockchainCmd

		default:
			return "", fmt.Errorf("Unsupported event: %v", incoming.Event().Id)
		}

	case *torrent.TorrentMessage:
		msg, _ := incoming.(*torrent.TorrentMessage)

		switch msg.Event().Id {
		case events.TORRENT_FETCHED:
			glog.Infof("Fetched image files from torrent: %v", msg.ImageFiles)
			cmd := containerWorker.NewContainerConfigureCommand(msg.ImageFiles, msg.AgreementLaunchContext)
			containerWorker.Commands <- cmd

		case events.TORRENT_FAILURE:
			// amounts to a container startup error from the perspective of the blockchain worker
			blockchainCmd := blockchainWorker.NewBlockchainEndContractCommand(events.CT_ERROR, msg.AgreementLaunchContext.ContractId, msg.AgreementLaunchContext.AgreementId)
			blockchainWorker.Commands <- blockchainCmd

		default:
			return "", fmt.Errorf("Unsupported event: %v", incoming.Event().Id)
		}

	case *container.ContainerMessage:
		msg, _ := incoming.(*container.ContainerMessage)

		switch msg.Event().Id {
		case events.EXECUTION_BEGUN:
			glog.Infof("Begun execution of containers according to agreement %v", msg.AgreementId)

			cmd := governanceWorker.NewStartGovernExecutionCommand(msg.Deployment, msg.ContractId, msg.AgreementId)
			governanceWorker.Commands <- cmd

		case events.EXECUTION_FAILED:
			blockchainCmd := blockchainWorker.NewBlockchainEndContractCommand(events.CT_ERROR, msg.ContractId, msg.AgreementId)
			blockchainWorker.Commands <- blockchainCmd

			containerCmd := containerWorker.NewContainerShutdownCommand(msg.ContractId, msg.AgreementId, msg.Deployment, []string{})
			containerWorker.Commands <- containerCmd

		case events.PATTERN_DESTROYED:
			cmd := governanceWorker.NewCleanupExecutionCommand(msg.ContractId, msg.AgreementId)
			governanceWorker.Commands <- cmd

		case events.PREVIOUS_AGREEMENT_REAP:
			glog.Infof("Completed reaping old agreements")

		default:
			return "", fmt.Errorf("Unsupported event: %v", incoming.Event().Id)
		}

	case *governance.GovernanceMaintenanceMessage:
		msg, _ := incoming.(*governance.GovernanceMaintenanceMessage)

		switch msg.Event().Id {
		case events.CONTAINER_MAINTAIN:
			containerCmd := containerWorker.NewContainerMaintenanceCommand(msg.ContractId(), msg.AgreementId(), msg.Deployment())
			containerWorker.Commands <- containerCmd
		default:
			return "", fmt.Errorf("Unsupported event: %v", incoming.Event().Id)
		}

	case *governance.GovernanceCancelationMessage:
		msg, _ := incoming.(*governance.GovernanceCancelationMessage)

		switch msg.Event().Id {
		case events.CONTRACT_ENDED:
			containerCmd := containerWorker.NewContainerShutdownCommand(msg.ContractId(), msg.AgreementId(), msg.Deployment(), []string{})
			containerWorker.Commands <- containerCmd

			blockchainCmd := blockchainWorker.NewBlockchainEndContractCommand(msg.Cause, msg.ContractId(), msg.AgreementId())
			blockchainWorker.Commands <- blockchainCmd

		case events.PREVIOUS_AGREEMENT_REAP:
			containerCmd := containerWorker.NewContainerShutdownCommand(msg.ContractId(), "", msg.Deployment(), msg.PreviousAgreements)
			containerWorker.Commands <- containerCmd

		default:
			return "", fmt.Errorf("Unsupported event: %v", incoming.Event().Id)
		}

	default:
		return "", fmt.Errorf("Unsupported message type: %T", incoming.Event)
	}
	return successMsg, nil
}

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

	// open db
	if err := os.MkdirAll(config.DBPath, 0700); err != nil {
		panic(err)
	}

	db, err := bolt.Open(path.Join(config.DBPath, "anax.db"), 0600, &bolt.Options{Timeout: 10 * time.Second})
	if err != nil {
		panic(err)
	}

	// start control signal handler
	control := make(chan os.Signal, 1)
	signal.Notify(control, os.Interrupt)
	signal.Notify(control, syscall.SIGTERM)
	go func() {
		<-control
		glog.Infof("Closing up shop.")

		pprof.StopCPUProfile()
		db.Close()

		os.Exit(0)
	}()

	// start API server
	api := api.NewAPIListener(config, db)

	// block here on blockchain account balance ; TODO: generalize, make a part of all work with ethereum in lib
	funded := false
	now := func() int64 { return time.Now().Unix() }
	printed := now() - 31

	for !funded {
		e := now()
		if e-printed > 30 {
			glog.Infof("Waiting for account to be funded")
			printed = e
		}

		var err error
		if funded, err = blockchain.AccountFunded(config); err != nil {
			// bury these because they're expected for some time up-front
			glog.V(4).Infof("Account not yet funded: %v", err)
		}
		time.Sleep(900 * time.Millisecond)
	}

	// start anax workers
	governanceWorker := governance.NewGovernanceWorker(config, db)
	whisperWorker := whisper.NewWhisperWorker(config, db)
	torrentWorker := torrent.NewTorrentWorker(config)
	containerWorker := container.NewContainerWorker(config)
	blockchainWorker := blockchain.NewBlockchainWorker(config, db)

	messageStream := mux(api.Messages, blockchainWorker.Messages, whisperWorker.Messages, torrentWorker.Messages, containerWorker.Messages, governanceWorker.Messages)

	last := int64(0)

	for {
		select {
		case msg := <-messageStream:
			glog.V(3).Infof("Handling Message (%T): %v\n", msg, msg)

			if successMsg, err := eventHandler(msg, blockchainWorker, whisperWorker, torrentWorker, containerWorker, governanceWorker); err != nil {
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
