package torrent

import (
	"fmt"
	"runtime"

	"github.com/golang/glog"
	"github.com/michaeldye/torrent"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/worker"
)

type TorrentWorker struct {
	worker.Worker // embedded field
	client        *torrent.Client
}

func NewTorrentWorker(config *config.HorizonConfig) *TorrentWorker {
	messages := make(chan events.Message)
	commands := make(chan worker.Command, 200)

	client, err := torrent.NewClient(&torrent.Config{
		DataDir:         config.Edge.TorrentDir,
		Debug:           true,
		Seed:            true,
		NoUpload:        false,
		DisableTrackers: false,
		NoDHT:           true,
		ListenAddr:      config.Edge.TorrentListenAddr,
	})
	if err != nil {
		panic(fmt.Sprintf("Unable to instantiate torrent client: %s", err))
	}

	worker := &TorrentWorker{
		worker.Worker{
			Manager: worker.Manager{
				Config:   config,
				Messages: messages,
			},

			Commands: commands,
		},
		client,
	}

	worker.start()
	return worker
}

func (w *TorrentWorker) Messages() chan events.Message {
	return w.Worker.Manager.Messages
}

func (w *TorrentWorker) NewEvent(incoming events.Message) {

	switch incoming.(type) {
	case *events.AgreementReachedMessage:
		msg, _ := incoming.(*events.AgreementReachedMessage)

		fCmd := w.NewFetchCommand(msg.LaunchContext())
		w.Commands <- fCmd

	case *events.LoadContainerMessage:
		msg, _ := incoming.(*events.LoadContainerMessage)

		fCmd := w.NewFetchCommand(msg.LaunchContext())
		w.Commands <- fCmd

	default: //nothing

	}

	return
}

func (b *TorrentWorker) start() {
	go func() {
		defer b.client.Close()

		for {
			glog.V(4).Infof("TorrentWorker command processor blocking waiting to receive incoming commands")

			command := <-b.Commands
			glog.V(3).Infof("TorrentWorker received command: %v", command)

			switch command.(type) {
			case *FetchCommand:

				cmd := command.(*FetchCommand)
				if lc := b.getLaunchContext(cmd.LaunchContext); lc == nil {
					glog.Errorf("Incoming event was not a known launch context: %T", cmd.LaunchContext)
				} else {
					glog.V(2).Infof("URL to fetch: %s\n", lc.URL())
					imageFiles, err := Fetch(lc.URL(), lc.Hashes(), lc.Signatures(), b.Config.Edge.CACertsPath, b.Config.Edge.TorrentDir, b.Config.Edge.PublicKeyPath, b.client)
					if err != nil {
						// TODO: write error out, then:
						// 1. retry to fetch up to a limit
						// 2. if failure persists, propagate a contract cancelation event with some meaningful reason for termination
						b.Messages() <- events.NewTorrentMessage(events.TORRENT_FAILURE, make([]string, 0), lc)
						glog.Errorf("Failed to fetch image files: %v", err)
					} else {
						b.Messages() <- events.NewTorrentMessage(events.TORRENT_FETCHED, imageFiles, lc)
					}
				}
			}

			runtime.Gosched()
		}
	}()
}

type FetchCommand struct {
	LaunchContext interface{}
}

func (f FetchCommand) ShortString() string {
	return fmt.Sprintf("%v", f)
}

func (t *TorrentWorker) NewFetchCommand(launchContext interface{}) *FetchCommand {
	return &FetchCommand{
		LaunchContext: launchContext,
	}
}

func (t *TorrentWorker) getLaunchContext(launchContext interface{}) events.LaunchContext {
	switch launchContext.(type) {
	case *events.ContainerLaunchContext:
		lc := launchContext.(events.LaunchContext)
		return lc
	case *events.AgreementLaunchContext:
		lc := launchContext.(events.LaunchContext)
		return lc
	}
	return nil
}
