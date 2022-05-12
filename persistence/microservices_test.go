//go:build unit
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
	parentOrg := "myorg"
	childURL := "url2"
	childOrg := "childorg"
	childVersion := "2.0.0"

	// Establish the dependency path objects
	parent := NewServiceInstancePathElement(parentURL, parentOrg, parentVersion)
	child := NewServiceInstancePathElement(childURL, childOrg, childVersion)

	depPath := []ServiceInstancePathElement{*parent, *child}
	// Create the test microservice instance to represent the child
	if msi, err := NewMicroserviceInstance(db, childURL, childOrg, childVersion, "1234", depPath, false); err != nil {
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
	parentOrg := "myorg"
	parentVersion := "1.0.0"
	childURL := "url2"
	childOrg := "childorg"
	childVersion := "2.0.0"

	// Establish the dependency path objects
	parent := NewServiceInstancePathElement(parentURL, parentOrg, parentVersion)
	child := NewServiceInstancePathElement(childURL, childOrg, childVersion)

	notParent := NewServiceInstancePathElement("other", parentOrg, parentVersion)

	depPath := []ServiceInstancePathElement{*parent, *child}
	// Create the test microservice instance to represent the child
	if msi, err := NewMicroserviceInstance(db, childURL, childOrg, childVersion, "1234", depPath, false); err != nil {
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
	parentOrg := "myorg"
	parentVersion := "1.0.0"
	childURL := "url2"
	childOrg := "childorg"
	childVersion := "2.0.0"
	child2URL := "url3"
	child2Org := "child2org"
	child2Version := "3.0.0"

	// Establish the dependency path objects
	parent := NewServiceInstancePathElement(parentURL, parentOrg, parentVersion)
	child := NewServiceInstancePathElement(childURL, childOrg, childVersion)
	child2 := NewServiceInstancePathElement(child2URL, child2Org, child2Version)

	depPath := []ServiceInstancePathElement{*parent, *child, *child2}
	// Create the test microservice instance to represent the child
	if msi, err := NewMicroserviceInstance(db, child2URL, child2Org, child2Version, "1234", depPath, false); err != nil {
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
	parentOrg := "myorg"
	parentVersion := "1.0.0"
	childURL := "url2"
	childOrg := "childorg2"
	childVersion := "2.0.0"
	child2URL := "url3"
	child2Org := "child2Org"
	child2Version := "3.0.0"

	// Establish the dependency path objects
	parent := NewServiceInstancePathElement(parentURL, parentOrg, parentVersion)
	child := NewServiceInstancePathElement(childURL, childOrg, childVersion)
	child2 := NewServiceInstancePathElement(child2URL, child2Org, child2Version)

	notParent := NewServiceInstancePathElement("other", parentOrg, parentVersion)

	depPath := []ServiceInstancePathElement{*parent, *child, *child2}
	dp2 := []ServiceInstancePathElement{*parent, *child2}

	// Create the test microservice instance to represent the child
	if msi, err := NewMicroserviceInstance(db, child2URL, child2Org, child2Version, "1234", depPath, false); err != nil {
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
	parentOrg := "myorg"
	parentVersion := "1.0.0"
	childURL := "url2"
	childOrg := "childorg"
	childVersion := "2.0.0"
	child2URL := "url3"
	child2Org := "childorg3"
	child2Version := "3.0.0"

	// Establish the dependency path objects
	parent := NewServiceInstancePathElement(parentURL, parentOrg, parentVersion)
	child := NewServiceInstancePathElement(childURL, childOrg, childVersion)
	child2 := NewServiceInstancePathElement(child2URL, child2Org, child2Version)

	depPath := []ServiceInstancePathElement{*parent, *child, *child2}
	dp2 := []ServiceInstancePathElement{*parent, *child2}

	// Create the test microservice instance to represent the child
	if msi, err := NewMicroserviceInstance(db, child2URL, child2Org, child2Version, "1234", depPath, false); err != nil {
		t.Errorf("Error creating instance: %v", err)
	} else if newmsi, err := UpdateMSInstanceAddDependencyPath(db, msi.GetKey(), &dp2); err != nil {
		t.Errorf("Error updating instance: %v", err)
	} else if !newmsi.HasDirectParent(parent) {
		t.Errorf("Child %v does have direct parent: %v", child2, dp2)
	}
}

func Test_CompareServiceInstancePath(t *testing.T) {
	// Establish the dependency path objects
	parentURL := "url1"
	parentOrg := "myorg"
	parentVersion := "1.0.0"
	childURL := "url2"
	childOrg := "childorg"
	childVersion := "2.0.0"
	child2URL := "url3"
	child2Org := "childorg3"
	child2Version := "3.0.0"

	parent := NewServiceInstancePathElement(parentURL, parentOrg, parentVersion)
	child := NewServiceInstancePathElement(childURL, childOrg, childVersion)
	child2 := NewServiceInstancePathElement(child2URL, child2Org, child2Version)

	dep1 := []ServiceInstancePathElement{*parent, *child, *child2}
	dep2 := []ServiceInstancePathElement{*parent, *child}

	if !CompareServiceInstancePath(nil, nil) {
		t.Errorf("CompareServiceInstancePath failed: nil and nil should not be equal.")
	}

	if CompareServiceInstancePath(nil, dep2) {
		t.Errorf("CompareServiceInstancePath failed: nil and %v should not be equal.", dep2)
	}

	if CompareServiceInstancePath(dep1, nil) {
		t.Errorf("CompareServiceInstancePath failed: %v and nil should not be equal.", dep1)
	}

	if CompareServiceInstancePath([]ServiceInstancePathElement{}, nil) {
		t.Errorf("CompareServiceInstancePath failed: empty slice and nil should not be equal.")
	}

	if CompareServiceInstancePath(dep1, dep2) {
		t.Errorf("CompareServiceInstancePath failed: %v and %v should not be equal.", dep1, dep2)
	}

	if !CompareServiceInstancePath(dep1, dep1) {
		t.Errorf("CompareServiceInstancePath failed: %v and %v should be equal.", dep1, dep1)
	}
}

func Test_UpdateMSInstanceAddDependencyPath(t *testing.T) {

	// Setup the DB for the UT environment
	dir, db, err := utsetup()
	if err != nil {
		t.Errorf("Error setting up UT DB: %v", err)
	}

	defer cleanTestDir(dir)

	parentURL := "parentUrl"
	parentOrg := "myorg1"
	parentVersion := "1.0.0"
	childURL := "childURL"
	childOrg := "childorg"
	childVersion := "2.0.0"
	child2URL := "child2URL"
	child2Org := "childorg2"
	child2Version := "3.0.0"

	parent := NewServiceInstancePathElement(parentURL, parentOrg, parentVersion)
	child := NewServiceInstancePathElement(childURL, childOrg, childVersion)
	child2 := NewServiceInstancePathElement(child2URL, child2Org, child2Version)

	dep1 := []ServiceInstancePathElement{*parent, *child, *child2}
	dep2 := []ServiceInstancePathElement{*parent, *child}

	// Create the test microservice instance to represent the child
	if msi, err := NewMicroserviceInstance(db, "child2UR", "childorg2", "2.0.0", "1234", dep1, false); err != nil {
		t.Errorf("Error creating instance: %v", err)
	} else if newmsi, err := UpdateMSInstanceAddDependencyPath(db, msi.GetKey(), &dep2); err != nil {
		t.Errorf("Error updating instance: %v", err)
	} else if len(newmsi.ParentPath) != 2 {
		t.Errorf("UpdateMSInstanceAddDependencyPath failed")
	} else if newmsi, err := UpdateMSInstanceAddDependencyPath(db, msi.GetKey(), nil); err != nil {
		t.Errorf("Error updating instance: %v", err)
	} else if len(newmsi.ParentPath) != 2 {
		t.Errorf("UpdateMSInstanceAddDependencyPath failed")
	} else if newmsi, err := UpdateMSInstanceAddDependencyPath(db, msi.GetKey(), &dep2); err != nil {
		t.Errorf("Error updating instance: %v", err)
	} else if len(newmsi.ParentPath) != 2 {
		t.Errorf("UpdateMSInstanceAddDependencyPath failed")
	}
}

func Test_UpdateMSInstanceRemoveDependencyPath(t *testing.T) {

	// Setup the DB for the UT environment
	dir, db, err := utsetup()
	if err != nil {
		t.Errorf("Error setting up UT DB: %v", err)
	}

	defer cleanTestDir(dir)

	parent1 := NewServiceInstancePathElement("parentUrl", "parentOrg", "1.0.0")
	parent2 := NewServiceInstancePathElement("parentUrl2", "parentOrg2", "1.0.0")
	parent3 := NewServiceInstancePathElement("parentUrl", "parentOrg", "2.0.0")
	child := NewServiceInstancePathElement("childURL", "childorg", "2.0.0")
	child2 := NewServiceInstancePathElement("child2URL", "childorg2", "3.0.0")

	dep1 := []ServiceInstancePathElement{*parent1, *child, *child2}
	dep2 := []ServiceInstancePathElement{*parent1, *child}
	dep3 := []ServiceInstancePathElement{*parent2, *child}
	dep4 := []ServiceInstancePathElement{*parent3, *child, *child2}

	// Create the test microservice instance to represent the child
	if msi, err := NewMicroserviceInstance(db, "child2UR", "childorg2", "2.0.0", "1234", dep1, false); err != nil {
		t.Errorf("Error creating instance: %v", err)
	} else if _, err := UpdateMSInstanceAddDependencyPath(db, msi.GetKey(), &dep2); err != nil {
		t.Errorf("Error updating instance: %v", err)
	} else if _, err := UpdateMSInstanceAddDependencyPath(db, msi.GetKey(), &dep3); err != nil {
		t.Errorf("Error updating instance: %v", err)
	} else if newmsi, err := UpdateMSInstanceAddDependencyPath(db, msi.GetKey(), &dep4); err != nil {
		t.Errorf("Error updating instance: %v", err)
	} else if len(newmsi.ParentPath) != 4 {
		t.Errorf("UpdateMSInstanceAddDependencyPath failed")
	} else if newmsi, err := UpdateMSInstanceRemoveDependencyPath(db, msi.GetKey(), &dep1); err != nil {
		t.Errorf("Error removing parent path: %v", err)
	} else if len(newmsi.ParentPath) != 3 {
		t.Errorf("UpdateMSInstanceRemoveDependencyPath failed")
	} else if !CompareServiceInstancePath(newmsi.ParentPath[0], dep2) {
		t.Errorf("UpdateMSInstanceRemoveDependencyPath failed")
	} else if !CompareServiceInstancePath(newmsi.ParentPath[1], dep3) {
		t.Errorf("UpdateMSInstanceRemoveDependencyPath failed")
	} else if !CompareServiceInstancePath(newmsi.ParentPath[2], dep4) {
		t.Errorf("UpdateMSInstanceRemoveDependencyPath failed")
	}
}

func Test_UpdateMSInstanceRemoveDependencyPath2(t *testing.T) {
	// Setup the DB for the UT environment
	dir, db, err := utsetup()
	if err != nil {
		t.Errorf("Error setting up UT DB: %v", err)
	}

	defer cleanTestDir(dir)

	parent1 := NewServiceInstancePathElement("parentUrl", "parentOrg", "1.0.0")
	parent2 := NewServiceInstancePathElement("parentUrl2", "parentOrg2", "1.0.0")
	parent3 := NewServiceInstancePathElement("parentUrl", "parentOrg", "2.0.0")
	child := NewServiceInstancePathElement("childURL", "childorg", "2.0.0")
	child2 := NewServiceInstancePathElement("child2URL", "childorg2", "3.0.0")

	dep1 := []ServiceInstancePathElement{*parent1, *child, *child2}
	dep2 := []ServiceInstancePathElement{*parent1, *child}
	dep3 := []ServiceInstancePathElement{*parent2, *child}
	dep4 := []ServiceInstancePathElement{*parent3, *child, *child2}

	// Create the test microservice instance to represent the child
	if msi, err := NewMicroserviceInstance(db, "child2UR", "childorg2", "2.0.0", "1234", dep1, false); err != nil {
		t.Errorf("Error creating instance: %v", err)
	} else if _, err := UpdateMSInstanceAddDependencyPath(db, msi.GetKey(), &dep2); err != nil {
		t.Errorf("Error updating instance: %v", err)
	} else if _, err := UpdateMSInstanceAddDependencyPath(db, msi.GetKey(), &dep3); err != nil {
		t.Errorf("Error updating instance: %v", err)
	} else if newmsi, err := UpdateMSInstanceAddDependencyPath(db, msi.GetKey(), &dep4); err != nil {
		t.Errorf("Error updating instance: %v", err)
	} else if len(newmsi.ParentPath) != 4 {
		t.Errorf("UpdateMSInstanceAddDependencyPath failed")
	} else if newmsi, err := UpdateMSInstanceRemoveDependencyPath2(db, msi.GetKey(), parent1); err != nil {
		t.Errorf("Error removing parent path: %v", err)
	} else if len(newmsi.ParentPath) != 2 {
		t.Errorf("UpdateMSInstanceRemoveDependencyPath failed")
	} else if !CompareServiceInstancePath(newmsi.ParentPath[0], dep3) {
		t.Errorf("UpdateMSInstanceRemoveDependencyPath failed")
	} else if !CompareServiceInstancePath(newmsi.ParentPath[1], dep4) {
		t.Errorf("UpdateMSInstanceRemoveDependencyPath failed")
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
