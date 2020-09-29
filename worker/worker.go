package worker

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"runtime"
	"time"
)

// The core of anax is an event handling system that distributes events to workers, where the workers
// process events that they are about.

type Command interface {
	ShortString() string
}

// A builtin command that workers use to terminate subworkers.
type BeginShutdownCommand struct {
}

func (b *BeginShutdownCommand) String() string {
	return b.ShortString()
}

func (b *BeginShutdownCommand) ShortString() string {
	return fmt.Sprintf("BeginShutdownCommand")
}

func NewBeginShutdownCommand() *BeginShutdownCommand {
	return &BeginShutdownCommand{}
}

// A builtin command that subworkers can use to tell their parent that they are terminating.
type SubWorkerTerminationCommand struct {
	name string
}

func (s *SubWorkerTerminationCommand) Name() string {
	return s.name
}

func (s *SubWorkerTerminationCommand) String() string {
	return s.ShortString()
}

func (s *SubWorkerTerminationCommand) ShortString() string {
	return fmt.Sprintf("SubWorkerTermination Name: %v", s.name)
}

func NewSubWorkerTerminationCommand(name string) *SubWorkerTerminationCommand {
	return &SubWorkerTerminationCommand{
		name: name,
	}
}

// A builtin comand that workers can use to terminate themselves.
type TerminateCommand struct {
	reason string
}

func (t *TerminateCommand) String() string {
	return t.ShortString()
}

func (t *TerminateCommand) ShortString() string {
	return fmt.Sprintf("Terminate Command: %v", t.reason)
}

func NewTerminateCommand(reason string) *TerminateCommand {
	return &TerminateCommand{
		reason: reason,
	}
}

// The manager that holds the outbound message queue for this worker.
type Manager struct {
	Config   *config.HorizonConfig
	Messages chan events.Message // managers send messages
}

func (m *Manager) String() string {
	return fmt.Sprintf("Num Messages: %v", len(m.Messages))
}

// A go routine that is managed by an anax worker. It is not a full blown worker.
type SubWorker struct {
	TermChan   chan bool
	Terminated bool
}

func (s *SubWorker) String() string {
	return fmt.Sprintf("Num messages: %v, Terminated: %v", len(s.TermChan), s.Terminated)
}

// A worker is a first class event/message handler that performs async functions based on event input or by polling
// a remote resource.
type Worker interface {
	Start(worker Worker, noWorkInterval int) // Called by a concrete worker to get the worker framework started
	Initialize() bool                        // Called by the worker FW to allow the concrete worker to initialize itself before starting the command loop
	CommandHandler(command Command) bool     // Called by the worker framework when there is a command for the worker to handle
	NoWorkHandler()                          // Called by the worker framework when the worker has been idle for noWorkInterval seconds

	// Methods that implement the ExchangeContext interface in the exchange package. The exchange package does not need
	// to be imported for this to work. And in fact, doing so would create cyclical dependencies, which are forbidden in go.
	GetExchangeId() string
	GetExchangeToken() string
	GetExchangeURL() string
	GetCSSURL() string
	GetHTTPFactory() *config.HTTPClientFactory
}

type BaseExchangeContext struct {
	Id          string
	Token       string
	URL         string
	CSSURL      string
	HTTPFactory *config.HTTPClientFactory
}

func NewExchangeContext(id string, token string, url string, cssurl string, httpFactory *config.HTTPClientFactory) *BaseExchangeContext {
	return &BaseExchangeContext{
		Id:          id,
		Token:       token,
		URL:         url,
		CSSURL:      cssurl,
		HTTPFactory: httpFactory,
	}
}

// This function should return the id in the form org/id.
func (w *BaseWorker) GetExchangeId() string {
	if w.EC != nil {
		return w.EC.Id
	} else {
		return ""
	}
}

func (w *BaseWorker) GetExchangeToken() string {
	if w.EC != nil {
		return w.EC.Token
	} else {
		return ""
	}
}

func (w *BaseWorker) GetExchangeURL() string {
	if w.EC != nil {
		return w.EC.URL
	} else {
		return ""
	}
}

func (w *BaseWorker) GetCSSURL() string {
	if w.EC != nil {
		return w.EC.CSSURL
	} else {
		return ""
	}
}

func (w *BaseWorker) GetHTTPFactory() *config.HTTPClientFactory {
	if w.EC != nil {
		return w.EC.HTTPFactory
	} else {
		return w.Config.Collaborators.HTTPClientFactory
	}
}

type BaseWorker struct {
	Name string
	Manager
	Commands         chan Command          // workers can receive commands
	DeferredCommands []Command             // commands can be deferred
	DeferredDelay    int                   // the number of seconds to delay before retrying
	SubWorkers       map[string]*SubWorker // workers can have sub go routines that they own
	ShuttingDown     bool
	EC               *BaseExchangeContext // Holds the exchange context state
	noWorkInterval   int
}

func NewBaseWorker(name string, cfg *config.HorizonConfig, ec *BaseExchangeContext) BaseWorker {
	commandQueueSize := 200
	// In order for agbot worker threads to be able to send events, the command queue on the agbot worker
	// has to be large enough to handle messages queued to the command handler from every worker for each
	// node in the batch.
	if cfg.GetAgbotAgreementQueueSize() != 0 {
		commandQueueSize = int(cfg.GetAgbotAgreementQueueSize() * 5)
	}

	return BaseWorker{
		Name: name,
		Manager: Manager{
			Config:   cfg,
			Messages: make(chan events.Message),
		},
		Commands:         make(chan Command, commandQueueSize),
		DeferredCommands: make([]Command, 0, 10),
		DeferredDelay:    10,
		SubWorkers:       make(map[string]*SubWorker),
		ShuttingDown:     false,
		EC:               ec,
		noWorkInterval:   0,
	}
}

func (w *BaseWorker) String() string {
	return fmt.Sprintf("Worker Name: %v, Manager: %v, Num commands: %v, Subworkers: %v", w.Name, &w.Manager, len(w.Commands), w.SubWorkers)
}

func (w *BaseWorker) GetName() string {
	return w.Name
}

func (w *BaseWorker) SetWorkerShuttingDown(retries int, interval int) {
	w.ShuttingDown = true
	if retries != 0 {
		w.EC.HTTPFactory.RetryCount = retries
	}
	if interval != 0 {
		w.EC.HTTPFactory.RetryInterval = interval
	}
}

func (w *BaseWorker) IsWorkerShuttingDown() bool {
	return w.ShuttingDown
}

func (w *BaseWorker) AddDeferredCommand(command Command) {
	glog.V(5).Infof(cdLogString(fmt.Sprintf("%v framework requeuing %v", w.GetName(), command)))
	w.DeferredCommands = append(w.DeferredCommands, command)
}

func (w *BaseWorker) RequeueDeferredCommands() {
	// Any commands that have been deferred should be written back to the command queue now. The commands have been
	// accumulating and have endured a time delay since they were last tried.
	glog.V(5).Infof(cdLogString(fmt.Sprintf("%v requeue-ing deferred commands", w.GetName())))
	for _, c := range w.DeferredCommands {
		w.Commands <- c
	}
	w.DeferredCommands = make([]Command, 0, 10)
}

func (w *BaseWorker) HasDeferredCommands() bool {
	return len(w.DeferredCommands) != 0
}

func (w *BaseWorker) SetDeferredDelay(delay int) {
	w.DeferredDelay = delay
}

func (w *BaseWorker) SetNoWorkInterval(interval int) {
	w.noWorkInterval = interval
}

func (w *BaseWorker) GetNoWorkInterval() int {
	return w.noWorkInterval
}

// Return handled (boolean) and terminate(boolean)
func (w *BaseWorker) HandleFrameworkCommands(command Command) (bool, bool) {
	switch command.(type) {
	case *BeginShutdownCommand:
		glog.V(3).Infof(cdLogString(fmt.Sprintf("%v terminating subworkers", w.GetName())))
		w.TerminateSubworkers()
		return true, false

	case *SubWorkerTerminationCommand:
		cmd, _ := command.(*SubWorkerTerminationCommand)
		glog.V(3).Infof(cdLogString(fmt.Sprintf("%v framework handling %v", w.GetName(), cmd)))
		w.SetSubworkerTerminated(cmd.Name())
		return true, false

	case *TerminateCommand:
		cmd, _ := command.(*TerminateCommand)
		glog.V(3).Infof(cdLogString(fmt.Sprintf("%v framework handling %v", w.GetName(), cmd)))

		workerStatusManager.SetWorkerStatus(w.GetName(), STATUS_TERMINATING)
		// If we can terminate, do it. Otherwise requeue the termination.
		if w.AreAllSubworkersTerminated() {
			w.Messages <- events.NewWorkerStopMessage(events.WORKER_STOP, w.GetName())
			return true, true
		} else {
			w.AddDeferredCommand(cmd)
			return true, false
		}
	}
	return false, false
}

// This function handles commands for the worker. Returns true when the worker should terminate.
func (w *BaseWorker) internalCommandhandler(worker Worker, command Command) bool {
	glog.V(2).Infof(cdLogString(fmt.Sprintf("%v received command (%T): %v", w.GetName(), command, command.ShortString())))
	glog.V(5).Infof(cdLogString(fmt.Sprintf("%v received command: %v", w.GetName(), command)))

	// Let the framework handle the command first
	if handled, terminate := w.HandleFrameworkCommands(command); terminate {
		return true
	} else if handled {
		return false
	}

	// Handle domain specific commands
	if handled := worker.CommandHandler(command); !handled {
		glog.Errorf(cdLogString(fmt.Sprintf("%v received unknown command (%T): %v", w.GetName(), command, command)))
	} else {
		glog.V(2).Infof(cdLogString(fmt.Sprintf("%v handled command (%T)", w.GetName(), command)))
	}
	return false
}

// This function kicks off the go routine that the worker's logic runs in.
func (w *BaseWorker) Start(worker Worker, noWorkInterval int) {

	w.SetNoWorkInterval(noWorkInterval)

	go func() {

		// log worker status
		workerStatusManager.SetWorkerStatus(w.GetName(), STATUS_STARTED)

		// Allow the worker to initialize itself, or stop it if initialization determines that.
		if !worker.Initialize() {
			workerStatusManager.SetWorkerStatus(w.GetName(), STATUS_INIT_FAILED)
			w.Messages <- events.NewWorkerStopMessage(events.WORKER_STOP, w.GetName())
			return
		} else {
			workerStatusManager.SetWorkerStatus(w.GetName(), STATUS_INITIALIZED)
		}

		// Process commands in blocking or non-blocking fashion, depending on how we were called.
		for {

			if w.GetNoWorkInterval() == 0 && !w.HasDeferredCommands() {
				glog.V(2).Infof(cdLogString(fmt.Sprintf("%v command processor blocking for commands", w.GetName())))

				// Get a command from the channel and dispatch to the command handler.
				command := <-w.Commands
				if terminate := w.internalCommandhandler(worker, command); terminate {
					glog.V(2).Infof(cdLogString(fmt.Sprintf("%v terminated", w.GetName())))
					return
				}

			} else {
				glog.V(2).Infof(cdLogString(fmt.Sprintf("%v command processor non-blocking for commands", w.GetName())))
				waitTime := w.GetNoWorkInterval()

				// If there are deferred commands, then we need to use the non-blocking receive with a timeout.
				if w.GetNoWorkInterval() == 0 {
					waitTime = 5
				}

				// Get commands from the channel and dispatch to the command handler.
				select {
				case command := <-w.Commands:
					if terminate := w.internalCommandhandler(worker, command); terminate {
						glog.V(2).Infof(cdLogString(fmt.Sprintf("%v terminated", w.GetName())))
						return
					}

				case <-time.After(time.Duration(waitTime) * time.Second):
					// Call the no work to do handler if it was requested.
					if w.GetNoWorkInterval() != 0 {
						worker.NoWorkHandler()
					}

					// Requeue any deferred commands that have been accumulating.
					if w.HasDeferredCommands() {
						w.RequeueDeferredCommands()
					}

				}
			}

			// Give the go subdispatcher a chance to run something else
			runtime.Gosched()
		}
	}()
}

// This function is called one time, when the worker first starts. The function returns false
// when it was not successful and the worker should terminate.
func (w *BaseWorker) Initialize() bool {
	return true
}

// This function is called every time a command arrives on the worker's command queue.
func (w *BaseWorker) CommandHandler(command Command) bool {
	return false
}

// This function is called if the worker wants a chance to run after no doing anything for a given period of time.
// If the Start() method is called with a zero noWorkInterval, then this function will never be called.
func (w *BaseWorker) NoWorkHandler() {}

// Subworker framework functions
func (w *BaseWorker) AddSubworker(name string) chan bool {
	rc := make(chan bool, 2)
	w.SubWorkers[name] = &SubWorker{
		TermChan:   rc,
		Terminated: false,
	}
	workerStatusManager.SetSubworkerStatus(w.GetName(), name, STATUS_ADDED)
	return rc
}

func (w *BaseWorker) SetSubworkerTerminated(name string) {
	if sw, ok := w.SubWorkers[name]; ok {
		workerStatusManager.SetSubworkerStatus(w.GetName(), name, STATUS_TERMINATED)
		sw.Terminated = true
	}
}

func (w *BaseWorker) TerminateSubworker(name string) {
	if sw, ok := w.SubWorkers[name]; ok && !w.IsSubworkerTerminated(name) {
		workerStatusManager.SetSubworkerStatus(w.GetName(), name, STATUS_TERMINATING)
		glog.V(5).Infof(cdLogString(fmt.Sprintf("telling subworker %v %v to terminate", name, sw)))
		sw.TermChan <- true
	}
	runtime.Gosched()
}

func (w *BaseWorker) TerminateSubworkers() {
	for name, _ := range w.SubWorkers {
		w.TerminateSubworker(name)
	}
	glog.V(5).Infof(cdLogString(fmt.Sprintf("done telling subworkers of %v to terminate", w.GetName())))
	// runtime.Gosched()
}

func (w *BaseWorker) IsSubworkerTerminated(name string) bool {
	if sw, ok := w.SubWorkers[name]; !ok || !sw.Terminated {
		return false
	}
	return true
}

func (w *BaseWorker) AreAllSubworkersTerminated() bool {
	for name, sw := range w.SubWorkers {
		if !sw.Terminated {
			glog.V(5).Infof(cdLogString(fmt.Sprintf("subworker %v %v not terminated yet", name, sw)))
			return false
		}
	}
	return true
}

func (w *BaseWorker) DispatchSubworker(name string, runSubWorker func() int, interval int, logOptOut bool) {
	quit := w.AddSubworker(name)
	nextWaitTime := interval
	go func() {
		workerStatusManager.SetSubworkerStatus(w.GetName(), name, STATUS_STARTED)
		glog.V(3).Infof(cdLogString(fmt.Sprintf("starting subworker %v", name)))
		for {
			select {
			case <-quit:
				w.Commands <- NewSubWorkerTerminationCommand(name)
				glog.V(3).Infof(cdLogString(fmt.Sprintf("exiting subworker %v", name)))
				return
			case <-time.After(time.Duration(nextWaitTime) * time.Second):
				if !logOptOut {
					glog.V(3).Infof(cdLogString(fmt.Sprintf("Running subworker %v", name)))
				}
				returnedWait := runSubWorker()
				if !logOptOut {
					glog.V(3).Infof(cdLogString(fmt.Sprintf("Finished run of subworker %v", name)))
				}
				if returnedWait > 0 {
					nextWaitTime = returnedWait
				}

			}
		}
	}()
}

// All workers need to implement this interface in order to have events/messages dispatched to them.
type MessageHandler interface {
	GetName() string
	NewEvent(events.Message)
	Messages() chan events.Message
}

type MessageHandlerRegistry struct {
	Handlers map[string]*MessageHandler
}

func NewMessageHandlerRegistry() *MessageHandlerRegistry {
	mhr := new(MessageHandlerRegistry)
	mhr.Handlers = make(map[string]*MessageHandler)
	return mhr
}

func (m *MessageHandlerRegistry) Add(mh interface {
	MessageHandler
}) {
	if y, ok := mh.(MessageHandler); ok {
		m.Handlers[y.GetName()] = &y
	}
}

func (m *MessageHandlerRegistry) Remove(name string) {
	if _, ok := m.Handlers[name]; ok {
		delete(m.Handlers, name)
	}
}

func (m *MessageHandlerRegistry) IsEmpty() bool {
	return len(m.Handlers) == 0
}

func (m *MessageHandlerRegistry) Contains(name string) bool {
	_, exists := m.Handlers[name]
	return exists
}

// This is the Event Handler Main control flow area: it receives incoming Message messages and operates on them by pushing them
// out to each worker. Workers then receive messages and, for messages they care about, the worker pushes them out as commands
// onto their own channels to operate on them.
//
func eventHandler(incoming events.Message, workers *MessageHandlerRegistry) (string, error) {
	successMsg := "propagated event to all workers"

	// If the message is the special worker stop message, then remove that worker from the dispatch pool.
	switch incoming.(type) {
	case *events.WorkerStopMessage:
		msg, _ := incoming.(*events.WorkerStopMessage)
		workers.Remove(msg.Name())
		workerStatusManager.SetWorkerStatus(msg.Name(), STATUS_TERMINATED)
		return successMsg, nil
	}

	// Dispatch the message to all workers
	for name, worker := range workers.Handlers {
		glog.V(5).Infof(mdLogString(fmt.Sprintf("Delivering message to %v", name)))
		(*worker).NewEvent(incoming)
		glog.V(5).Infof(mdLogString(fmt.Sprintf("Delivered message to %v", name)))
	}

	return successMsg, nil
}

// This function combines all messages (events) from workers into a single global message queue. From this
// global queue, each message will get delivered to each worker by the event handler function.
//
func mux(workers *MessageHandlerRegistry, muxed chan events.Message) chan events.Message {

	for _, w := range workers.Handlers {
		select {
		case ev := <-(*w).Messages():
			muxed <- ev
		default: // nothing
		}
	}

	return muxed
}

func (workers *MessageHandlerRegistry) ProcessEventMessages() {

	// 200 messages should be plenty. We will never get more than 1 message from every worker each time
	// we write into this stream.
	messageStream := make(chan events.Message, 200)

	last := int64(0)

	for {
		// Exit the event processing loop if all workers have deregistered.
		if workers.IsEmpty() {
			glog.V(3).Infof(mdLogString(fmt.Sprintf("Terminating")))
			break
		}

		// Grab messages that are outbound from the workers.
		messageStream = mux(workers, messageStream)

		// Process any new messages on the combined worker message queue.
		done := false
		for !done {
			select {
			case msg := <-messageStream:
				glog.V(3).Infof(mdLogString(fmt.Sprintf("Handling Message (%T): %v\n", msg, msg.ShortString())))
				glog.V(5).Infof(mdLogString(fmt.Sprintf("Handling Message (%T): %v\n", msg, msg)))

				// Push outbound messages into each worker.
				if successMsg, err := eventHandler(msg, workers); err != nil {
					// error! do some barfing and then continue
					glog.Errorf(mdLogString(fmt.Sprintf("Error occurred handling message: %s, Error: %v\n", msg, err)))
				} else {
					glog.V(2).Infof(mdLogString(fmt.Sprintf("Success handling message: %s\n", successMsg)))
				}
			default:
				now := time.Now().Unix()
				if now-last > 30 {
					glog.V(5).Infof(mdLogString(fmt.Sprintf("No incoming messages for router to handle")))
					last = now
				}
				done = true
			}
		}

		time.Sleep(200 * time.Millisecond)
	}

	// Brief delay just in case.
	time.Sleep(6 * time.Second)
}

var mdLogString = func(v interface{}) string {
	return fmt.Sprintf("MessageDispatcher: %v", v)
}

var cdLogString = func(v interface{}) string {
	return fmt.Sprintf("CommandDispatcher: %v", v)
}
