// +build unit

package dev

import (
	"os"
	"strings"
	"testing"
)

// pwd is /tmp, no input path.
func Test_workingdir_success_nodashd(t *testing.T) {

	tempDir := "/tmp"

	if err := os.Chdir(tempDir); err != nil {
		t.Errorf("error %v changing to temp dir", err)
	}

	inputDir := ""
	if dir, err := GetWorkingDir(inputDir); err != nil {
		t.Errorf("error getting working dir: %v", err)
	} else if dir != tempDir {
		t.Errorf("returned wrong dir, shoiuld be %v, was %v", tempDir, dir)
	}

}

// pwd is go project, input path is absolute /tmp dir.
func Test_workingdir_success_dashd_abs(t *testing.T) {

	inputDir := "/tmp"

	if dir, err := GetWorkingDir(inputDir); err != nil {
		t.Errorf("error getting working dir: %v", err)
	} else if dir != inputDir {
		t.Errorf("returned wrong dir, should be %v, was %v", inputDir, dir)
	}

}

// pwd is /tmp, input path is relative - back up 1 level
func Test_workingdir_success_dashd_rel(t *testing.T) {

	tempDir := "/tmp"

	if err := os.Chdir(tempDir); err != nil {
		t.Errorf("error %v changing to temp dir", err)
	}

	inputDir := ".."

	if dir, err := GetWorkingDir(inputDir); err != nil {
		t.Errorf("error getting working dir: %v", err)
	} else if dir != "/" {
		t.Errorf("returned wrong dir, should be %v, was %v", "/", dir)
	}

}

// pwd is go project, input path is abs, doesnt exist.
func Test_workingdir_bad_dashd_abs(t *testing.T) {

	tempDir := "/tmp/doesnotexist"

	if dir, err := GetWorkingDir(tempDir); err == nil {
		t.Errorf("expected error getting working dir")
	} else if dir != "" {
		t.Errorf("expected empty string for directory, but was %v", dir)
	} else if !strings.Contains(err.Error(), "no such file") {
		t.Errorf("wrong error returned, expected no such file")
	}

}

// pwd is go project, input path is relative, back up to something that doesnt
func Test_workingdir_bad_dashd_rel(t *testing.T) {

	tempDir := "/tmp"

	if err := os.Chdir(tempDir); err != nil {
		t.Errorf("error %v changing to temp dir", err)
	}

	inputDir := "../doesnotexist"

	if dir, err := GetWorkingDir(inputDir); err == nil {
		t.Errorf("expected error getting working dir")
	} else if dir != "" {
		t.Errorf("expected empty string for directory, but was %v", dir)
	} else if !strings.Contains(err.Error(), "no such file") {
		t.Errorf("wrong error returned, expected no such file")
	}

}
