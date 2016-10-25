package whisper

import (
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/worker"
	gwhisper "github.com/open-horizon/go-whisper"
	"runtime"
	"sync"
	"time"
)

type WhisperWorker struct {
    worker.Worker            // embedded field
    topics         map[string]bool
    topicLock      sync.Mutex
    topicChange    bool
}

func NewWhisperWorker(config *config.HorizonConfig) *WhisperWorker {
	messages := make(chan events.Message)
	commands := make(chan worker.Command, 200)

	worker := &WhisperWorker{
		Worker: worker.Worker{
			Manager: worker.Manager{
				Config:   config,
				Messages: messages,
			},

			Commands: commands,
		},
        topics: make(map[string]bool),
        topicLock: sync.Mutex{},
        topicChange: false,
	}
	worker.start()
	return worker
}

func (w *WhisperWorker) Messages() chan events.Message {
    return w.Worker.Manager.Messages
}

func (w *WhisperWorker) NewEvent(incoming events.Message) {
    switch incoming.(type) {
    case *events.WhisperSubscribeToMessage:
        msg, _ := incoming.(*events.WhisperSubscribeToMessage)
        w.topicLock.Lock()
        if _, ok := w.topics[msg.Topic()]; !ok {
            w.topics[msg.Topic()] = true
            w.topicChange = true
        }
        w.topicLock.Unlock()

    default: //nothing

    }
}

// func (w *WhisperWorker) pollIncoming(topics []string, proposal *citizenscientist.Proposal, gethURL string) (*citizenscientist.ProposalReply, error) {
func (w *WhisperWorker) pollIncoming() {

	gethURL := ""
	if len(w.Config.Edge.GethURL) != 0 {
		gethURL = w.Config.Edge.GethURL
	} else {
		gethURL = w.Config.AgreementBot.GethURL
	}

    var topics []string
    var topicChange bool
    var read func(time.Duration, int64) ([]gwhisper.Result, error)
    for {

        // Grab the set of topics (under lock) and convert them into a topic list for whisper.

        w.topicLock.Lock()
        if w.topicChange {
            topics = make([]string, 0, 10)
            for topic, _ := range w.topics {
                topics = append(topics, topic)
            }
            w.topicChange = false
            topicChange = true
        }
    	w.topicLock.Unlock()

        if len(topics) != 0 {

            // If the topics we're listening on havent changed then use the same reader. The reader
            // is stateful and remembers messages that it's seen before so we want to reuse it if we can.

            if read == nil || topicChange {
                glog.V(4).Infof("WhisperWorker getting new reader for topics %v", topics)
                read = gwhisper.WhisperReader(gethURL, [][]string{topics})
                topicChange = false
            }

            glog.V(4).Infof("WhisperWorker Reading from whisper for topics %v", topics)

            // On call to read(), reader will block polling for proper topic messages; will return a list of them if any arrive.
            // There is a logical OR on topics in nested array, logical ANDs between topics in top-level array

            // poll really often and do so until we get a message
            if results, err := read(time.Duration(10)*time.Second, 30); err != nil {
                glog.Errorf("Error reading messages from whisper: %v", err)
                // return nil, err
            } else {
                glog.V(4).Info("Whisper read returned results or timed-out waiting")
                for _, r := range results {
                    glog.V(3).Infof("WhisperWorker saw result w/ hash: %v", r.Hash)

                    // Send the whisper message to the internal message bus
                    wm := events.NewWhisperReceivedMessage(events.RECEIVED_MSG, r.Payload, r.From)
                    w.Messages() <- wm

                }
            }
        }

        time.Sleep(1 * time.Second)
    }
}

func (w *WhisperWorker) start() {

	go func() {
		for {
			glog.V(2).Infof("WhisperWorker command processor blocking for commands")
			command := <-w.Commands
			glog.V(2).Infof("WhisperWorker received command: %v", command)


			glog.V(5).Infof("WhisperWorker handled command")
			runtime.Gosched()
		}
	}()

	// start polling for incoming messages
	go w.pollIncoming()
}

