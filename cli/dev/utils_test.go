// +build unit

package dev

import (
	"os"
	"path"
	"strings"
	"testing"
)

// pwd is /tmp, no input path, use the default which exists.
func Test_workingdir_success_nodashd1(t *testing.T) {

	tempDir := "/tmp"
	workingDir := path.Join(tempDir, DEFAULT_WORKING_DIR)

	if err := os.MkdirAll(workingDir, 0755); err != nil {
		t.Errorf("error creating working dir %v", err)
	}

	if err := os.Chdir(tempDir); err != nil {
		t.Errorf("error %v changing to temp dir", err)
	}

	inputDir := ""
	if dir, err := GetWorkingDir(inputDir, true); err != nil {
		t.Errorf("error getting working dir: %v", err)
	} else if dir != workingDir {
		t.Errorf("returned wrong dir, shoiuld be %v, was %v", workingDir, dir)
	}

	os.Remove(DEFAULT_WORKING_DIR)

}

// pwd is /tmp, no input path, use the default which does not exist.
func Test_workingdir_success_nodashd2(t *testing.T) {

	tempDir := "/tmp"
	workingDir := path.Join(tempDir, DEFAULT_WORKING_DIR)

	if err := os.Chdir(tempDir); err != nil {
		t.Errorf("error %v changing to temp dir", err)
	}

	inputDir := ""
	if dir, err := GetWorkingDir(inputDir, false); err != nil {
		t.Errorf("error getting working dir: %v", err)
	} else if dir != workingDir {
		t.Errorf("returned wrong dir, shoiuld be %v, was %v", workingDir, dir)
	}

}

// pwd is go project, input path is absolute /tmp dir.
func Test_workingdir_success_dashd_abs(t *testing.T) {

	inputDir := "/tmp"

	if dir, err := GetWorkingDir(inputDir, true); err != nil {
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

	if dir, err := GetWorkingDir(inputDir, true); err != nil {
		t.Errorf("error getting working dir: %v", err)
	} else if dir != "/" {
		t.Errorf("returned wrong dir, should be %v, was %v", "/", dir)
	}

}

// pwd is go project, input path is abs, doesnt exist.
func Test_workingdir_bad_dashd_abs(t *testing.T) {

	tempDir := "/tmp/doesnotexist"

	if dir, err := GetWorkingDir(tempDir, true); err == nil {
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

	if dir, err := GetWorkingDir(inputDir, true); err == nil {
		t.Errorf("expected error getting working dir")
	} else if dir != "" {
		t.Errorf("expected empty string for directory, but was %v", dir)
	} else if !strings.Contains(err.Error(), "no such file") {
		t.Errorf("wrong error returned, expected no such file")
	}
}

// generates a service specRef and version from the image name
func Test_GetServiceSpecFromImage(t *testing.T) {
	specRef, version, err := GetServiceSpecFromImage("image for gps")
	if err == nil {
		t.Errorf("GetServiceSpecFromImage should have returned error but not. It returned specRef=%v, version=%v", specRef, version)
	} else {
		if !strings.Contains(err.Error(), "invalid image format") {
			t.Errorf("GetServiceSpecFromImage returned wrong error: %v", err)
		}
	}

	specRef, version, err = GetServiceSpecFromImage("open_horizon/gps")
	if err != nil {
		t.Errorf("GetServiceSpecFromImage returned error but should not. Error: %v", err)
	} else {
		if specRef != "open_horizon/gps" && version != "" {
			t.Errorf("GetServiceSpecFromImage returned wrong specReg %v or version %v", specRef, version)
		}
	}

	specRef, version, err = GetServiceSpecFromImage("myrepo_host:123:open_horizon/gps")
	if err != nil {
		t.Errorf("GetServiceSpecFromImage returned error but should not. Error: %v", err)
	} else {
		if specRef != "open_horizon/gps" && version != "" {
			t.Errorf("GetServiceSpecFromImage returned wrong specReg %v or version %v", specRef, version)
		}
	}

	specRef, version, err = GetServiceSpecFromImage("myrepo_host:123:open_horizon/gps:1.2.3")
	if err != nil {
		t.Errorf("GetServiceSpecFromImage returned error but should not. Error: %v", err)
	} else {
		if specRef != "open_horizon/gps" && version != "1.2.3" {
			t.Errorf("GetServiceSpecFromImage returned wrong specReg %v or version %v", specRef, version)
		}

	}

	specRef, version, err = GetServiceSpecFromImage("myrepo.host.open_horizon/gps:1.2.3")
	if err != nil {
		t.Errorf("GetServiceSpecFromImage returned error but should not. Error: %v", err)
	} else {
		if specRef != "open_horizon/gps" && version != "1.2.3" {
			t.Errorf("GetServiceSpecFromImage returned wrong specReg %v or version %v", specRef, version)
		}
	}
}

func Test_GetImageInfoFromImageList(t *testing.T) {
	imageInfo, image_base, err := GetImageInfoFromImageList([]string{}, "0.0.1", false)
	if err != nil {
		t.Errorf("GetImageInfoFromImageList returned error but should not. Error: %v", err)
	} else if len(imageInfo) != 1 {
		t.Errorf("The length of the imageInfo should be 1 but got %v", len(imageInfo))
	} else if imageInfo["$SERVICE_NAME"] != "${DOCKER_IMAGE_BASE}_$ARCH:$SERVICE_VERSION" {
		t.Errorf("imageInfo['$SERVICE_NAME'] should be '${DOCKER_IMAGE_BASE}_$ARCH:$SERVICE_VERSION' but got %v", imageInfo["$SERVICE_NAME"])
	} else if image_base != "" {
		t.Errorf("image_base should be an empty string but got %v", image_base)
	}

	imageInfo, image_base, err = GetImageInfoFromImageList([]string{"path/myimage"}, "0.0.1", false)
	if err != nil {
		t.Errorf("GetImageInfoFromImageList returned error but should not. Error: %v", err)
	} else if len(imageInfo) != 1 {
		t.Errorf("The length of the imageInfo should be 1 but got %v", len(imageInfo))
	} else if imageInfo["myimage"] != "${DOCKER_IMAGE_BASE}_$ARCH:$SERVICE_VERSION" {
		t.Errorf("imageInfo['myimage'] should be '${DOCKER_IMAGE_BASE}_$ARCH:$SERVICE_VERSION' but got %v", imageInfo["myimage"])
	} else if image_base != "path/myimage" {
		t.Errorf("image_base should be path/myimage but got %v", image_base)
	}

	imageInfo, image_base, err = GetImageInfoFromImageList([]string{"repo.mycom.com:1234/path/myimage:1.2.3"}, "0.0.1", false)
	if err != nil {
		t.Errorf("GetImageInfoFromImageList returned error but should not. Error: %v", err)
	} else if len(imageInfo) != 1 {
		t.Errorf("The length of the imageInfo should be 1 but got %v", len(imageInfo))
	} else if imageInfo["myimage"] != "${DOCKER_IMAGE_BASE}_$ARCH:$SERVICE_VERSION" {
		t.Errorf("imageInfo['myimage'] should be '${DOCKER_IMAGE_BASE}_$ARCH:$SERVICE_VERSION' but got %v", imageInfo["myimage"])
	} else if image_base != "repo.mycom.com:1234/path/myimage" {
		t.Errorf("image_base should be repo.mycom.com:1234/path/myimage but got %v", image_base)
	}

	imageInfo, image_base, err = GetImageInfoFromImageList([]string{"repo.mycom.com:1234/path/myimage"}, "0.0.1", true)
	if err != nil {
		t.Errorf("GetImageInfoFromImageList returned error but should not. Error: %v", err)
	} else if len(imageInfo) != 1 {
		t.Errorf("The length of the imageInfo should be 1 but got %v", len(imageInfo))
	} else if imageInfo["myimage"] != "${DOCKER_IMAGE_BASE}_$ARCH:$SERVICE_VERSION" {
		t.Errorf("imageInfo['myimage'] should be '${DOCKER_IMAGE_BASE}_$ARCH:$SERVICE_VERSION' but got %v", imageInfo["myimage"])
	} else if image_base != "repo.mycom.com:1234/path/myimage" {
		t.Errorf("image_base should be repo.mycom.com:1234/path/myimage but got %v", image_base)
	}

	imageInfo, image_base, err = GetImageInfoFromImageList([]string{"repo.mycom.com:1234/path/myimage:1.2.3"}, "0.0.1", true)
	if err != nil {
		t.Errorf("GetImageInfoFromImageList returned error but should not. Error: %v", err)
	} else if len(imageInfo) != 1 {
		t.Errorf("The length of the imageInfo should be 1 but got %v", len(imageInfo))
	} else if imageInfo["myimage"] != "repo.mycom.com:1234/path/myimage:1.2.3" {
		t.Errorf("imageInfo['myimage'] should be 'repo.mycom.com:1234/path/myimage:1.2.3' but got %v", imageInfo["myimage"])
	} else if image_base != "" {
		t.Errorf("image_base should be an empty string but got %v", image_base)
	}

	imageInfo, image_base, err = GetImageInfoFromImageList([]string{"repo.mycom.com:1234/path/myimage1:2.3.4", "repo.mycom.com:1234/path/myimage2"}, "0.0.1", true)
	if err != nil {
		t.Errorf("GetImageInfoFromImageList returned error but should not. Error: %v", err)
	} else if len(imageInfo) != 2 {
		t.Errorf("The length of the imageInfo should be 2 but got %v", len(imageInfo))
	} else if imageInfo["myimage1"] != "repo.mycom.com:1234/path/myimage1:2.3.4" {
		t.Errorf("imageInfo['myimage1'] should be 'repo.mycom.com:1234/path/myimage1:2.3.4' but got %v", imageInfo["myimage1"])
	} else if imageInfo["myimage2"] != "repo.mycom.com:1234/path/myimage2_$ARCH:$SERVICE_VERSION" {
		t.Errorf("imageInfo['myimage'] should be 'repo.mycom.com:1234/path/myimage2_$ARCH:$SERVICE_VERSION' but got %v", imageInfo["myimage2"])
	} else if image_base != "" {
		t.Errorf("image_base should be an empty string but got %v", image_base)
	}
}
