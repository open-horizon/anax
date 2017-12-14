package dev

import (
	"os"
	"path/filepath"
)

const DEVTOOL_HZN_ORG = "HZN_ORG"
const DEVTOOL_HZN_USER = "HZN_USER"
const DEVTOOL_HZN_PASSWORD = "HZN_PASSWORD"
const DEVTOOL_HZN_EXCHANGE_URL = "HZN_EXCHANGE_URL"

// The current working directory could be specified via input (as an absolute or relative path) or
// it could be defaulted if there is no input. It must exist, otherwise an error is returned.
func GetWorkingDir(dashD string) (string, error) {
	dir := dashD
	var err error
	if dir == "" {
		if dir, err = os.Getwd(); err != nil {
			return "", err
		}
	} else if dir, err = filepath.Abs(dashD); err != nil {
		return "", err
	} else if _, err := os.Stat(dir); err != nil {
		return "", err
	}
	return dir, nil
}
