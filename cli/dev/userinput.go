package dev

import (
	"errors"
	"fmt"
	"github.com/open-horizon/anax/cli/register"
	"os"
	"path"
	"path/filepath"
)

const USERINPUT_FILE = "userinput.json"

func GetUserInputs(homeDirectory string, userInputFile string) (*register.InputFile, error) {

	userInputFilePath := ""
	if userInputFile == "" {
		userInputFilePath = path.Join(homeDirectory, USERINPUT_FILE)
	} else if fullPath, err := filepath.Abs(userInputFile); err != nil {
		return nil, errors.New(fmt.Sprintf("unable to convert %v to an absolute file path, %v", userInputFile, err))
	} else if _, err := os.Stat(fullPath); err != nil {
		return nil, errors.New(fmt.Sprintf("%v, %v", fullPath, err))
	} else {
		userInputFilePath = fullPath
	}

	userInputs := new(register.InputFile)
	register.ReadInputFile(userInputFilePath, userInputs)
	return userInputs, nil

}
