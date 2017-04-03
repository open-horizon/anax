package worker

import (
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
)

type Command interface{
	ShortString() string
}

type Manager struct {
	Config   *config.HorizonConfig
	Messages chan events.Message // managers send messages
}

type Worker struct {
	Name string
	Manager
	Commands chan Command // workers can receive commands
}

// All workers need to implement this interface
type MessageHandler interface {
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

func (m *MessageHandlerRegistry) Add(name string, mh interface {
	MessageHandler
}) {

	if y, ok := mh.(MessageHandler); ok {
		m.Handlers[name] = &y
	}

}

func (m *MessageHandlerRegistry) Contains(name string) bool {
	_, exists := m.Handlers[name]
	return exists
}
