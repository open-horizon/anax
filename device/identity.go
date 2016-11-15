package device

import (
	"errors"
	"os"
)

func Id() (string, error) {
	id := os.Getenv("CMTN_DEVICE_ID")
	if id == "" {
		return "", errors.New("Unspecified device id; envvar 'CMTN_DEVICE_ID' does not have a value")
	}
	return id, nil
}
