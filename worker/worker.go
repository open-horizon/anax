package worker

import (
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
)

type Command interface{}

type Manager struct {
	Config   *config.Config
	Messages chan events.Message // managers send messages
}

type Worker struct {
	Manager
	Commands chan Command // workers can receive commands
}
