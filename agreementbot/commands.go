package agreementbot

import (
    "github.com/open-horizon/anax/events"
)

type ReceivedWhisperMessageCommand struct {
    Msg events.WhisperReceivedMessage
}

func NewReceivedWhisperMessageCommand(msg events.WhisperReceivedMessage) *ReceivedWhisperMessageCommand {
    return &ReceivedWhisperMessageCommand{
        Msg: msg,
    }
}
