package torrent

import (
	"fmt"
	"runtime"

	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/worker"
	fetch "github.com/open-horizon/horizon-pkg-fetch"
	"github.com/open-horizon/horizon-pkg-fetch/fetcherrors"
)

type TorrentWorker struct {
	worker.Worker // embedded field
	db            *bolt.DB
}

func NewTorrentWorker(config *config.HorizonConfig, db *bolt.DB) *TorrentWorker {
	messages := make(chan events.Message)
	commands := make(chan worker.Command, 200)

	worker := &TorrentWorker{
		worker.Worker{
			Manager: worker.Manager{
				Config:   config,
				Messages: messages,
			},

			Commands: commands,
		},
		db,
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

// TODO: extract this, make common via collaborators
func authAttributes(db *bolt.DB) (map[string]map[string]string, error) {
	authAttrs := make(map[string]map[string]string, 0)

	// TODO: fill this with the device token, just need to know the URLs

	// assemble credentials from attributes
	attributes, err := persistence.FindApplicableAttributes(db, "")
	if err != nil {
		return nil, fmt.Errorf("Error fetching attributes. Error: %v", err)
	}
	for _, attr := range attributes {
		if attr.GetMeta().Type == "HTTPSBasicAuthAttributes" {
			a := attr.(persistence.HTTPSBasicAuthAttributes)
			cred := map[string]string{
				"username": a.Username,
				"password": a.Password,
			}
			//cred := fmt.Sprintf("Basic %v", base64.StdEncoding.EncodeToString([]byte(a.Username+":"+a.Password)))

			// we don't care about apply-all settings, they're a security problem (TODO: add an API check for this case)
			for _, url := range attr.GetMeta().SensorUrls {
				authAttrs[url] = cred
			}
		}
	}

	return authAttrs, nil
}

func (b *TorrentWorker) start() {
	go func() {
		for {
			glog.V(4).Infof("FetchWorker command processor blocking waiting to receive incoming commands")

			command := <-b.Commands
			glog.V(3).Infof("FetchWorker received command: %v", command)

			switch command.(type) {
			case *FetchCommand:

				authAttribs, err := authAttributes(b.db)
				if err != nil {
					glog.Error(err)
				} else {

					cmd := command.(*FetchCommand)
					if lc := b.getLaunchContext(cmd.LaunchContext); lc == nil {
						glog.Errorf("Incoming event was not a known launch context: %T", cmd.LaunchContext)
					} else {
						glog.V(2).Infof("URL to fetch: %s\n", lc.URL())

						// TODO: decide where the best place is to shortcut the fetch call if the docker images it names are already in the local repo
						// (could be here or bypass this worker altogether)
						// (this is really important because we want to be able to delete the downloaded image files after docker load)

						imageFiles, err := fetch.PkgFetch(b.Config.Collaborators.HTTPClientFactory.WrappedNewHTTPClient(), lc.URL(), lc.Signature(), b.Config.Edge.TorrentDir, b.Config.Edge.CACertsPath, b.Config.UserPublicKeyPath(), authAttribs)

						if err != nil {
							var id events.EventId
							switch err.(type) {
							case fetcherrors.PkgMetaError, fetcherrors.PkgSourceError, fetcherrors.PkgPrecheckError:
								id = events.IMAGE_DATA_ERROR

							case fetcherrors.PkgSourceFetchError:
								id = events.IMAGE_FETCH_ERROR

							case fetcherrors.PkgSourceFetchAuthError:
								id = events.IMAGE_FETCH_AUTH_ERROR

							case fetcherrors.PkgSignatureVerificationError:
								id = events.IMAGE_SIG_VERIF_ERROR

							default:
								id = events.IMAGE_FETCH_ERROR
							}
							b.Messages() <- events.NewTorrentMessage(id, make([]string, 0), lc)
							glog.Errorf("Failed to fetch image files: %v", err)
						} else {
							b.Messages() <- events.NewTorrentMessage(events.IMAGE_FETCHED, imageFiles, lc)
						}
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
