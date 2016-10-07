package worker

import (
	"repo.hovitos.engineering/MTN/anax/config"
	"repo.hovitos.engineering/MTN/anax/events"
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
