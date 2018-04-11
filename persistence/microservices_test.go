// +build unit

package persistence

import (
	"github.com/boltdb/bolt"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"
)

// Parent and child, simple case.
func Test_ServiceInstancePath_HasDirectParent_simple(t *testing.T) {

	// Setup the DB for the UT environment
	dir, db, err := utsetup()
	if err != nil {
		t.Errorf("Error setting up UT DB: %v", err)
	}

	defer cleanTestDir(dir)

	// Setup initial variable values
	parentURL := "url1"
	parentVersion := "1.0.0"
	childURL := "url2"
	childVersion := "2.0.0"

	// Establish the dependency path objects
	parent := NewServiceInstancePathElement(parentURL, parentVersion)
	child := NewServiceInstancePathElement(childURL, childVersion)

	depPath := []ServiceInstancePathElement{*parent, *child}
	// Create the test microservice instance to represent the child
	if msi, err := NewMicroserviceInstance(db, childURL, childVersion, "1234", depPath); err != nil {
		t.Errorf("Error creating instance: %v", err)
	} else if !msi.HasDirectParent(parent) {
		t.Errorf("Child %v has direct parent: %v", child, depPath)
	}
}

// Parent is not in child's dependency path
func Test_ServiceInstancePath_HasDirectParent_fail1(t *testing.T) {

	// Setup the DB for the UT environment
	dir, db, err := utsetup()
	if err != nil {
		t.Errorf("Error setting up UT DB: %v", err)
	}

	defer cleanTestDir(dir)

	// Setup initial variable values
	parentURL := "url1"
	parentVersion := "1.0.0"
	childURL := "url2"
	childVersion := "2.0.0"

	// Establish the dependency path objects
	parent := NewServiceInstancePathElement(parentURL, parentVersion)
	child := NewServiceInstancePathElement(childURL, childVersion)

	notParent := NewServiceInstancePathElement("other", parentVersion)

	depPath := []ServiceInstancePathElement{*parent, *child}
	// Create the test microservice instance to represent the child
	if msi, err := NewMicroserviceInstance(db, childURL, childVersion, "1234", depPath); err != nil {
		t.Errorf("Error creating instance: %v", err)
	} else if msi.HasDirectParent(notParent) {
		t.Errorf("Child %v does not have direct parent: %v", child, depPath)
	}
}

// Parent is in child dependency path, but not directly the child's parent.
func Test_ServiceInstancePath_HasDirectParent_fail2(t *testing.T) {

	// Setup the DB for the UT environment
	dir, db, err := utsetup()
	if err != nil {
		t.Errorf("Error setting up UT DB: %v", err)
	}

	defer cleanTestDir(dir)

	// Setup initial variable values
	parentURL := "url1"
	parentVersion := "1.0.0"
	childURL := "url2"
	childVersion := "2.0.0"
	child2URL := "url3"
	child2Version := "3.0.0"

	// Establish the dependency path objects
	parent := NewServiceInstancePathElement(parentURL, parentVersion)
	child := NewServiceInstancePathElement(childURL, childVersion)
	child2 := NewServiceInstancePathElement(child2URL, child2Version)

	depPath := []ServiceInstancePathElement{*parent, *child, *child2}
	// Create the test microservice instance to represent the child
	if msi, err := NewMicroserviceInstance(db, child2URL, child2Version, "1234", depPath); err != nil {
		t.Errorf("Error creating instance: %v", err)
	} else if msi.HasDirectParent(parent) {
		t.Errorf("Child %v does not have direct parent: %v", child2, depPath)
	}
}

// Parent is not in either of child's dependency paths.
func Test_ServiceInstancePath_HasDirectParent_fail3(t *testing.T) {

	// Setup the DB for the UT environment
	dir, db, err := utsetup()
	if err != nil {
		t.Errorf("Error setting up UT DB: %v", err)
	}

	defer cleanTestDir(dir)

	// Setup initial variable values
	parentURL := "url1"
	parentVersion := "1.0.0"
	childURL := "url2"
	childVersion := "2.0.0"
	child2URL := "url3"
	child2Version := "3.0.0"

	// Establish the dependency path objects
	parent := NewServiceInstancePathElement(parentURL, parentVersion)
	child := NewServiceInstancePathElement(childURL, childVersion)
	child2 := NewServiceInstancePathElement(child2URL, child2Version)

	notParent := NewServiceInstancePathElement("other", parentVersion)

	depPath := []ServiceInstancePathElement{*parent, *child, *child2}
	dp2 := []ServiceInstancePathElement{*parent, *child2}

	// Create the test microservice instance to represent the child
	if msi, err := NewMicroserviceInstance(db, child2URL, child2Version, "1234", depPath); err != nil {
		t.Errorf("Error creating instance: %v", err)
	} else if _, err := UpdateMSInstanceAddDependencyPath(db, msi.GetKey(), &dp2); err != nil {
		t.Errorf("Error updating instance: %v", err)
	} else if msi.HasDirectParent(notParent) {
		t.Errorf("Child %v does not have direct parent: %v %v", child2, depPath, dp2)
	}
}

// Parent is in second of child's dependency paths.
func Test_ServiceInstancePath_HasDirectParent_second(t *testing.T) {

	// Setup the DB for the UT environment
	dir, db, err := utsetup()
	if err != nil {
		t.Errorf("Error setting up UT DB: %v", err)
	}

	defer cleanTestDir(dir)

	// Setup initial variable values
	parentURL := "url1"
	parentVersion := "1.0.0"
	childURL := "url2"
	childVersion := "2.0.0"
	child2URL := "url3"
	child2Version := "3.0.0"

	// Establish the dependency path objects
	parent := NewServiceInstancePathElement(parentURL, parentVersion)
	child := NewServiceInstancePathElement(childURL, childVersion)
	child2 := NewServiceInstancePathElement(child2URL, child2Version)

	depPath := []ServiceInstancePathElement{*parent, *child, *child2}
	dp2 := []ServiceInstancePathElement{*parent, *child2}

	// Create the test microservice instance to represent the child
	if msi, err := NewMicroserviceInstance(db, child2URL, child2Version, "1234", depPath); err != nil {
		t.Errorf("Error creating instance: %v", err)
	} else if newmsi, err := UpdateMSInstanceAddDependencyPath(db, msi.GetKey(), &dp2); err != nil {
		t.Errorf("Error updating instance: %v", err)
	} else if !newmsi.HasDirectParent(parent) {
		t.Errorf("Child %v does have direct parent: %v", child2, dp2)
	}
}

// Utility functions needed by tests
func utsetup() (string, *bolt.DB, error) {
	dir, err := ioutil.TempDir("", "utdb-")
	if err != nil {
		return "", nil, err
	}

	db, err := bolt.Open(path.Join(dir, "anax-ut.db"), 0600, &bolt.Options{Timeout: 10 * time.Second})
	if err != nil {
		return dir, nil, err
	}

	return dir, db, nil
}

// Make a deferred call to this function after calling setup(), passing the output dirpath of the setup() function.
func cleanTestDir(dirPath string) error {
	if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
		if err := os.RemoveAll(dirPath); err != nil {
			return err
		}
	}
	return nil
}
