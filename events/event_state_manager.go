package events

import (
	"fmt"
	"sync"
)

type EventStateManager struct {
	Lock        sync.Mutex      // The lock to ensure reading and writing event state is serialized.
	EventRecord map[string]bool // Simply record the presence or absence of event ocurrences. The key is the event type name.
}

func NewEventStateManager() *EventStateManager {
	return &EventStateManager{
		Lock:        sync.Mutex{},
		EventRecord: make(map[string]bool),
	}
}

func (em *EventStateManager) String() string {
	return fmt.Sprintf("Known Events: %v", em.EventRecord)
}

type CustomEventRecorder func(e Message)
type CustomReceivedEvent func(e Message) bool

func (em *EventStateManager) RecordEvent(event Message, customFunc CustomEventRecorder) {
	em.Lock.Lock()
	defer em.Lock.Unlock()

	eType := fmt.Sprintf("%T", event)
	em.EventRecord[eType] = true

	if customFunc != nil {
		customFunc(event)
	}

}

func (em *EventStateManager) ReceivedEvent(event Message, customFunc CustomReceivedEvent) bool {
	em.Lock.Lock()
	defer em.Lock.Unlock()

	if customFunc != nil {
		return customFunc(event)
	}

	eType := fmt.Sprintf("%T", event)
	if received, ok := em.EventRecord[eType]; !ok {
		return false
	} else {
		return received
	}

}
