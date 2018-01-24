// +build unit

package worker

import (
	"flag"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"os"
	"testing"
	"time"
)

func init() {
	flag.Set("alsologtostderr", "true")
	flag.Set("v", "7")
	// no need to parse flags, that's done by test framework
}

func Test_No_workers(t *testing.T) {
	mhr := NewMessageHandlerRegistry()

	// Monitor the test to make sure it doesnt get stuck in a wait loop.
	monitorWaitTime := 5
	testEnded := false
	go monitorTest(t, &testEnded, monitorWaitTime)

	// Should return quickly
	mhr.ProcessEventMessages()

	// Tell the monitor the test is done.
	testEnded = true

}

func Test_One_worker(t *testing.T) {

	// Setup the worker handler registry
	mhr := NewMessageHandlerRegistry()

	// Init the worker and tell it to run some number of times
	expectedCommandCount := 3
	w := NewTestWorker("testworker", getBasicConfig(), expectedCommandCount, 0)

	// Add the worker to registry
	mhr.Add(w)

	// Monitor the test to make sure it doesnt get stuck in a wait loop.
	monitorWaitTime := 10
	testEnded := false
	go monitorTest(t, &testEnded, monitorWaitTime)

	// Start the event handler
	mhr.ProcessEventMessages()

	// Tell the monitor the test is done.
	testEnded = true

	// Verify that the correct number of comands were handled.
	if w.CommandCount != expectedCommandCount {
		t.Errorf("command count is %v, should be %v", w.CommandCount, expectedCommandCount)
	}
}

// One worker with blocking command selection
func Test_One_worker_non_blocking(t *testing.T) {

	// Setup the worker handler registry
	mhr := NewMessageHandlerRegistry()

	// Init the worker and tell it to run some number of times
	expectedCommandCount := 3
	w := NewTestWorker("blockingtest", getBasicConfig(), expectedCommandCount, 0)

	// Add the worker to registry
	mhr.Add(w)

	// Monitor the test to make sure it doesnt get stuck in a wait loop.
	monitorWaitTime := 10
	testEnded := false
	go monitorTest(t, &testEnded, monitorWaitTime)

	// Wait for all commands to be run. If they dont get run, then the
	// test monitor will timeout the test.
	go func() {
		for {
			// Verify that the correct number of comands were handled.
			if w.CommandCount == expectedCommandCount {
				// Tell the monitor the test is done.
				testEnded = true

				// Shutdown the test worker
				w.Commands <- NewBeginShutdownCommand()
				w.Commands <- NewTerminateCommand("shutdown")

				break
			}
			time.Sleep(200 * time.Millisecond)
		}
	}()

	// Start the event handler
	mhr.ProcessEventMessages()

}

// Worker doesnt initialize
func Test_One_worker_no_init(t *testing.T) {

	// Setup the worker handler registry
	mhr := NewMessageHandlerRegistry()

	// Init the worker and tell it to run some number of times
	expectedCommandCount := 0
	w := NewTestWorker("testworker", getBasicConfig(), 1, 0)
	w.setFailInit()

	// Add the worker to registry
	mhr.Add(w)

	// Monitor the test to make sure it doesnt get stuck in a wait loop.
	monitorWaitTime := 10
	testEnded := false
	go monitorTest(t, &testEnded, monitorWaitTime)

	// Start the event handler
	mhr.ProcessEventMessages()

	// Tell the monitor the test is done.
	testEnded = true

	// Verify that the correct number of comands were handled. Should be no commands.
	if w.CommandCount != expectedCommandCount {
		t.Errorf("command count is %v, should be %v", w.CommandCount, expectedCommandCount)
	}
}

func Test_two_workers(t *testing.T) {

	// Setup the worker handler registry
	mhr := NewMessageHandlerRegistry()

	// Init the worker and tell it to run some number of times
	expectedCommandCount := 3
	w1 := NewTestWorker("testworker1", getBasicConfig(), expectedCommandCount, 0)
	w2 := NewTestWorker("testworker2", getBasicConfig(), expectedCommandCount, 0)

	// Add the worker to registry
	mhr.Add(w1)
	mhr.Add(w2)

	// Monitor the test to make sure it doesnt get stuck in a wait loop.
	monitorWaitTime := 15
	testEnded := false
	go monitorTest(t, &testEnded, monitorWaitTime)

	// Start the event handler
	mhr.ProcessEventMessages()

	// Tell the monitor the test is done.
	testEnded = true

	// Verify that the correct number of comands were handled.
	if w1.CommandCount != (expectedCommandCount * 2) {
		t.Errorf("command count for %v is %v, should be %v", w1.GetName(), w1.CommandCount, expectedCommandCount)
	} else if w2.CommandCount != (expectedCommandCount * 2) {
		t.Errorf("command count for %v is %v, should be %v", w2.GetName(), w2.CommandCount, expectedCommandCount)
	}
}

func Test_One_worker_one_sub(t *testing.T) {

	// Setup the worker handler registry
	mhr := NewMessageHandlerRegistry()

	// Init the worker and tell it to run some number of times
	expectedCommandCount := 3
	startSub := 1
	w := NewTestWorker("testworker", getBasicConfig(), expectedCommandCount, startSub)

	// Add the worker to registry
	mhr.Add(w)

	// Monitor the test to make sure it doesnt get stuck in a wait loop.
	monitorWaitTime := 15
	testEnded := false
	go monitorTest(t, &testEnded, monitorWaitTime)

	// Start the event handler
	mhr.ProcessEventMessages()

	// Tell the monitor the test is done.
	testEnded = true

	// Verify that the correct number of comands were handled.
	if w.CommandCount != expectedCommandCount {
		t.Errorf("command count is %v, should be %v", w.CommandCount, expectedCommandCount)
	}
}

func Test_One_worker_two_sub(t *testing.T) {

	// Setup the worker handler registry
	mhr := NewMessageHandlerRegistry()

	// Init the worker and tell it to run some number of times
	expectedCommandCount := 3
	startSub := 2
	w := NewTestWorker("testworker", getBasicConfig(), expectedCommandCount, startSub)

	// Add the worker to registry
	mhr.Add(w)

	// Monitor the test to make sure it doesnt get stuck in a wait loop.
	monitorWaitTime := 15
	testEnded := false
	go monitorTest(t, &testEnded, monitorWaitTime)

	// Start the event handler
	mhr.ProcessEventMessages()

	// Tell the monitor the test is done.
	testEnded = true

	// Verify that the correct number of comands were handled.
	if w.CommandCount != expectedCommandCount {
		t.Errorf("command count is %v, should be %v", w.CommandCount, expectedCommandCount)
	}
}

// This function monitors the test to prevent hung tests.
func monitorTest(t *testing.T, state *bool, wait int) {
	wc := 0
	for {
		if *state == true {
			break
		}
		time.Sleep(1 * time.Second)
		wc += 1
		if wc >= wait {
			t.Errorf("test monitor timeout, test failed.")
			fmt.Println("Test monitor timeout, test failed.")
			os.Exit(1)
		}
	}

}

// The worker used in this test and the methods that allow it to meet the worker interface.
type TestWorker struct {
	BaseWorker       // embedded field
	CommandCount     int
	ExpectedCommands int
	NumSubworkers    int
	FailInit         bool
}

func (t *TestWorker) Messages() chan events.Message {
	return t.BaseWorker.Manager.Messages
}

func (t *TestWorker) NewEvent(incoming events.Message) {
	switch incoming.(type) {
	case *TestMessage:
		msg, _ := incoming.(*TestMessage)
		t.Commands <- NewTestCommand1(msg)

	case *events.WorkerStopMessage:
		glog.V(2).Infof(testLogString(fmt.Sprintf("SHOULD NOT BE HERE %v", incoming)))
		os.Exit(1)

	default: //nothing

	}
}

// This function is called one time, when the worker first starts.
func (t *TestWorker) Initialize() bool {

	if t.FailInit {
		return false
	}

	subWorkerName := "sub1"

	// Put some messages on the queue to handle
	for i := 0; i < t.ExpectedCommands; i++ {
		t.Messages() <- NewTestMessage()
	}

	// Start a subworker if necessary. The single subworker path uses the helpful API. The multiple
	// subworker path uses the custom API so that we test both paths.
	for i := 0; i < t.NumSubworkers; i++ {
		if t.NumSubworkers == 1 {
			dispatchIntervalSecs := 1
			t.DispatchSubworker(subWorkerName, t.runSubWorker, dispatchIntervalSecs)
		} else {
			ch := t.AddSubworker(subWorkerName + fmt.Sprintf("%v", i))
			go t.startSubWorker(subWorkerName+fmt.Sprintf("%v", i), ch)
		}
	}
	return true
}

// This function is called every time a command arrives on the worker's command queue.
func (t *TestWorker) CommandHandler(command Command) bool {
	// Handle domain specific commands
	switch command.(type) {
	case *TestCommand1:
		cmd, _ := command.(*TestCommand1)
		t.CommandCount += 1
		glog.V(2).Infof(testLogString(fmt.Sprintf("performing command %v actions %v", cmd, t.CommandCount)))

	// Should not get here
	case *SubWorkerTerminationCommand:
		cmd, _ := command.(*SubWorkerTerminationCommand)
		glog.V(2).Infof(testLogString(fmt.Sprintf("SHOULD NOT BE HERE %v", cmd)))
		os.Exit(1)

	// Should not get here
	case *TerminateCommand:
		cmd, _ := command.(*TerminateCommand)
		glog.V(2).Infof(testLogString(fmt.Sprintf("SHOULD NOT BE HERE %v", cmd)))
		os.Exit(1)

	default:
		glog.Errorf(testLogString(fmt.Sprintf("unknown command (%T): %v", command, command)))
		return false
	}
	return true
}

// This function is called if the worker wants a chance to run after not doing anything for a given period of time.
// If the Start() method is called with a zero noWorkInterval, then this function will never be called.
func (t *TestWorker) NoWorkHandler() {
	// Nothing left to do, start terminating.
	glog.V(2).Infof(testLogString(fmt.Sprintf("%v has no more work, beginning termination", t.GetName())))
	t.Commands <- NewBeginShutdownCommand()
	t.Commands <- NewTerminateCommand("shutdown")
}

// Tell the test worker to fail during init
func (t *TestWorker) setFailInit() {
	t.FailInit = true
}

// Subworker functions
func (t *TestWorker) startSubWorker(name string, quit chan bool) {
	glog.V(2).Infof(testLogString("entering the subworker"))
	for {
		select {
		case <-quit:
			t.Commands <- NewSubWorkerTerminationCommand(name)
			glog.V(2).Infof(testLogString("exiting the subworker"))
			return
		case <-time.After(500 * time.Millisecond):
			glog.V(2).Infof(testLogString("executing in the subworker"))
		}
	}
}

func (t *TestWorker) runSubWorker() int {
	glog.V(2).Infof(testLogString("executing in the subworker"))
	return 1
}

func NewTestWorker(name string, cfg *config.HorizonConfig, cc int, startSub int) *TestWorker {

	nonBlockDuration := 3
	if name == "blockingtest" {
		nonBlockDuration = 0
	}

	ec := NewExchangeContext("myorg/myid", "token", cfg.Edge.ExchangeURL, false, cfg.Collaborators.HTTPClientFactory)
	worker := &TestWorker{
		BaseWorker:       NewBaseWorker(name, cfg, ec),
		CommandCount:     0,
		ExpectedCommands: cc,
		NumSubworkers:    startSub,
	}

	worker.SetDeferredDelay(3)

	worker.Start(worker, nonBlockDuration)
	return worker
}

// Commands for the worker and messages for the message bus
type TestCommand1 struct {
	Msg *TestMessage
}

func (t *TestCommand1) String() string {
	return t.ShortString()
}

func (t *TestCommand1) ShortString() string {
	return fmt.Sprintf("TestCommand Msg: %v", t.Msg)
}

func NewTestCommand1(msg *TestMessage) *TestCommand1 {
	return &TestCommand1{
		Msg: msg,
	}
}

type UnknownTestCommand struct {
}

func (t *UnknownTestCommand) String() string {
	return t.ShortString()
}

func (t *UnknownTestCommand) ShortString() string {
	return fmt.Sprintf("UnknownTestCommand")
}

func NewUnknownTestCommand(msg *TestMessage) *UnknownTestCommand {
	return &UnknownTestCommand{}
}

type TestMessage struct {
	event events.Event
}

func NewTestMessage() *TestMessage {
	return &TestMessage{
		event: events.Event{
			Id: "testid",
		},
	}
}

func (e *TestMessage) Event() events.Event {
	return e.event
}
func (e *TestMessage) String() string {
	return e.ShortString()
}
func (e *TestMessage) ShortString() string {
	return fmt.Sprintf("TestMessage event: %v", e.event)
}

// Utility functions
func getBasicConfig() *config.HorizonConfig {
	return &config.HorizonConfig{
		Edge: config.Config{
			DefaultServiceRegistrationRAM: 256,
			PolicyPath:                    "/tmp/",
		},
		AgreementBot: config.AGConfig{},
		Collaborators: config.Collaborators{
			HTTPClientFactory: nil,
		},
	}
}

var testLogString = func(v interface{}) string {
	return fmt.Sprintf("TestWorker %v", v)
}
