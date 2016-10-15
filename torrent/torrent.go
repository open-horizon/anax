package torrent

import (
	"fmt"
	"runtime"

	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/worker"
	"repo.hovitos.engineering/mdye/torrent"
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
				glog.V(2).Infof("URL to fetch: %s\n", cmd.AgreementLaunchContext.Configure.TorrentURL)
				imageFiles, err := Fetch(cmd.AgreementLaunchContext.Configure.TorrentURL, cmd.AgreementLaunchContext.Configure.ImageHashes, cmd.AgreementLaunchContext.Configure.ImageSignatures, b.Config.Edge.CACertsPath, b.Config.Edge.TorrentDir, b.Config.Edge.PublicKeyPath, b.client)
				if err != nil {
					// TODO: write error out, then:
					// 1. retry to fetch up to a limit
					// 2. if failure persists, propagate a contract cancelation event with some meaningful reason for termination
					b.Messages() <- NewTorrentMessage(events.TORRENT_FAILURE, make([]string, 0), cmd.AgreementLaunchContext)
					glog.Errorf("Failed to fetch image files: %v", err)
				} else {
					b.Messages() <- NewTorrentMessage(events.TORRENT_FETCHED, imageFiles, cmd.AgreementLaunchContext)
				}
			}

			runtime.Gosched()
		}
	}()
}

type FetchCommand struct {
	AgreementLaunchContext *events.AgreementLaunchContext
}

func (t *TorrentWorker) NewFetchCommand(agreementLaunchContext *events.AgreementLaunchContext) *FetchCommand {
	return &FetchCommand{
		AgreementLaunchContext: agreementLaunchContext,
	}
}

type TorrentMessage struct {
	event                  events.Event
	ImageFiles             []string
	AgreementLaunchContext *events.AgreementLaunchContext
}

// fulfill interface of events.Message
func (b *TorrentMessage) Event() events.Event {
	return b.event
}

func NewTorrentMessage(id events.EventId, imageFiles []string, agreementLaunchContext *events.AgreementLaunchContext) *TorrentMessage {

	return &TorrentMessage{
		event: events.Event{
			Id: id,
		},
		ImageFiles:             imageFiles,
		AgreementLaunchContext: agreementLaunchContext,
	}
}
