package dev

import (
	"errors"
	"github.com/open-horizon/anax/i18n"
)

const HORIZON_GITIGNORE_FILE = ".gitignore"
const DEPENDENCY_GITIGNORE_FILE = "dependencies/.gitignore"

const HORIZON_GITIGNORE_FILE_CONTENT = `/.hzn.json.tmp.mk
`
const DEPENDENCY_GITIGNORE_FILE_CONTENT = `*.service.definition.json
`

// It creates gitignore files
func CreateSourceCodeManagementFiles(directory string) error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if err := CreateHorizonGitIgnoreFile(directory); err != nil {
		return errors.New(msgPrinter.Sprintf("error creating %v for source code management. %v", HORIZON_GITIGNORE_FILE, err))
	}
	if err := CreateDependencyGitIgnoreFile(directory); err != nil {
		return errors.New(msgPrinter.Sprintf("error creating %v for source code management. %v", DEPENDENCY_GITIGNORE_FILE, err))
	}
	return nil
}

func CreateHorizonGitIgnoreFile(directory string) error {
	return CreateFileWithConent(directory, HORIZON_GITIGNORE_FILE, HORIZON_GITIGNORE_FILE_CONTENT, nil, false)
}

func CreateDependencyGitIgnoreFile(directory string) error {
	return CreateFileWithConent(directory, DEPENDENCY_GITIGNORE_FILE, DEPENDENCY_GITIGNORE_FILE_CONTENT, nil, false)
}
