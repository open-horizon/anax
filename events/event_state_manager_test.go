//go:build unit
// +build unit

package events

import (
	"testing"
)

func Test_Simple_eventmanager0(t *testing.T) {
	em := NewEventStateManager()

	if em == nil {
		t.Errorf("should have returned an object")
	}

	ev := NewLoadContainerMessage(LOAD_CONTAINER, nil)

	em.RecordEvent(ev, nil)

	if !em.ReceivedEvent(ev, nil) {
		t.Errorf("event %v should have been recorded: %v", ev, em)
	}

}

func Test_Simple_eventmanager1(t *testing.T) {
	em := NewEventStateManager()

	if em == nil {
		t.Errorf("should have returned an object")
	}

	ev := NewLoadContainerMessage(LOAD_CONTAINER, nil)

	if em.ReceivedEvent(ev, nil) {
		t.Errorf("event %v should not have been recorded: %v", ev, em)
	}

}

func Test_functions_eventmanager0(t *testing.T) {
	em := NewEventStateManager()

	if em == nil {
		t.Errorf("should have returned an object")
	}

	recorderRan := false
	recorder := func(e Message) {
		recorderRan = true
	}

	receiverRan := false
	receiver := func(e Message) bool {
		receiverRan = true
		return receiverRan
	}

	ev := NewLoadContainerMessage(LOAD_CONTAINER, nil)

	em.RecordEvent(ev, recorder)

	if !em.ReceivedEvent(ev, receiver) {
		t.Errorf("event %v should have been recorded: %v", ev, em)
	} else if !recorderRan {
		t.Errorf("custom recorder did not run")
	} else if !receiverRan {
		t.Errorf("custom receiver did not run")
	}

}

func Test_functions_eventmanager1(t *testing.T) {
	em := NewEventStateManager()

	if em == nil {
		t.Errorf("should have returned an object")
	}

	recorderRan := false
	recorder := func(e Message) {
	}

	receiverRan := false
	receiver := func(e Message) bool {
		return false
	}

	ev := NewLoadContainerMessage(LOAD_CONTAINER, nil)

	em.RecordEvent(ev, recorder)

	if em.ReceivedEvent(ev, receiver) {
		t.Errorf("event state manager %v receiver override should have responded with false: %v", em, ev)
	} else if recorderRan {
		t.Errorf("somehow recorderRan got set")
	} else if receiverRan {
		t.Errorf("somehow receiverRan got set")
	}

}

func Test_functions_eventmanager2(t *testing.T) {
	em := NewEventStateManager()

	if em == nil {
		t.Errorf("should have returned an object")
	}

	recorderRan := false
	recorder := func(e Message) {
	}

	ev := NewLoadContainerMessage(LOAD_CONTAINER, nil)

	em.RecordEvent(ev, recorder)

	if !em.ReceivedEvent(ev, nil) {
		t.Errorf("event %v should have been recorded: %v", ev, em)
	} else if recorderRan {
		t.Errorf("somehow recorderRan got set")
	}
}
