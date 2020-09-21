// +build integration

package dev

import (
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/common"
	"github.com/open-horizon/anax/container"
	"github.com/open-horizon/anax/containermessage"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/policy"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

// Test the recursive service start code to ensure that the right networks are associated to the services.
func Test_nested_dependencies(t *testing.T) {

	debug := false

	projectDir, cw := setupUT(t, debug)

	defer cleanTestDir(projectDir)

	horizonDir := createTempProject(t, projectDir)

	// Create leaf node dependencies.
	c21 := createServiceDef(t, horizonDir, "child21", []*common.ServiceFile{}, true)
	c22 := createServiceDef(t, horizonDir, "child22", []*common.ServiceFile{}, true)
	c11 := createServiceDef(t, horizonDir, "child11", []*common.ServiceFile{}, true)

	// Create intermediate dependencies.
	c1 := createServiceDef(t, horizonDir, "child1", []*common.ServiceFile{c11}, true)
	c2 := createServiceDef(t, horizonDir, "child2", []*common.ServiceFile{c21, c22}, true)

	// Create top level service.
	sDef := createServiceDef(t, horizonDir, "parent", []*common.ServiceFile{c1, c2}, false)

	// Start the test. Grab the dependencies and start the dependencies.
	deps, derr := GetServiceDependencies(horizonDir, sDef.RequiredServices)
	if derr != nil {
		t.Errorf("unable to get service dependencies, %v", derr)
	}

	networks, perr := ProcessStartDependencies(horizonDir, deps, []common.GlobalSet{}, []policy.AbstractUserInput{}, cw)
	if perr != nil {
		t.Errorf("unable to process dependencies, %v", perr)
	}

	// Make sure the networks were setup correctly.
	if len(networks) != len(sDef.RequiredServices) {
		t.Errorf("expected %v networks to be returned, but got %v", len(sDef.RequiredServices), len(networks))
	}

	// Shut everything down.
	if err := ProcessStopDependencies(horizonDir, deps, cw); err != nil {
		t.Errorf("error shutting down test containers, %v", err)
	}

}

func setupUT(t *testing.T, debug bool) (string, *container.ContainerWorker) {

	// Setup CLI environment.
	cliutils.Opts.Verbose = &debug
	cliutils.Opts.IsDryRun = &debug
	cliutils.Opts.UsingApiKey = false

	// Create the containerWorker.
	cw, cerr := createContainerWorker()
	if cerr != nil {
		t.Errorf("unable to create Container Worker, %v", cerr)
	}

	// Setup a fake project.
	projectDir, terr := ioutil.TempDir("", "hzndev-util-test-")
	if terr != nil {
		t.Errorf("unable to create temp directory for project, %v", terr)
	}

	return projectDir, cw
}

func createTempProject(t *testing.T, projectDir string) string {
	horizonDir := path.Join(projectDir, DEFAULT_WORKING_DIR)
	werr := CreateWorkingDir(horizonDir)
	if werr != nil {
		t.Errorf("unable to create project directories, %v", werr)
	}

	// The userinputs file just needs to be in the project.
	uierr := createSkeletalUserInputs(horizonDir)
	if uierr != nil {
		t.Errorf("unable to create user inputs, %v", uierr)
	}
	return horizonDir
}

func createServiceDef(t *testing.T, horizonDir string, serviceName string, children []*common.ServiceFile, dependency bool) *common.ServiceFile {
	depSDef1 := createSkeletalServiceDef(serviceName)
	depSDef1.URL = "http://service/" + serviceName

	if len(children) != 0 {
		for _, child := range children {
			sdep := exchange.ServiceDependency{URL: child.GetURL(), Org: "testorg", Version: "1.0.0", Arch: cutil.ArchString()}
			depSDef1.RequiredServices = append(depSDef1.RequiredServices, sdep)
		}
	}

	fileName := SERVICE_DEFINITION_FILE
	filePath := horizonDir
	if dependency {
		fileName = createDependencyFileName(depSDef1.GetOrg(), depSDef1.GetURL(), depSDef1.GetVersion(), SERVICE_DEFINITION_FILE)
		filePath = path.Join(horizonDir, DEFAULT_DEPENDENCY_DIR)
	}
	if err := CreateFile(filePath, fileName, depSDef1); err != nil {
		t.Errorf("unable to create service def, %v", err)
	}
	return depSDef1
}

func createSkeletalServiceDef(serviceName string) *common.ServiceFile {
	res := new(common.ServiceFile)
	res.Label = ""
	res.Description = ""
	res.Public = true
	res.URL = DEFAULT_SDEF_URL
	res.Version = "1.0.0"
	res.Arch = cutil.ArchString()
	res.Sharable = exchange.MS_SHARING_MODE_MULTIPLE
	res.UserInputs = []exchange.UserInput{}
	res.MatchHardware = map[string]interface{}{}
	res.RequiredServices = []exchange.ServiceDependency{}
	res.Deployment = map[string]interface{}{
		"services": map[string]*containermessage.Service{
			serviceName: &containermessage.Service{
				Image:       "alpine:latest",
				Environment: []string{},
				Command:     []string{"/bin/sleep", "600"}},
		},
	}
	res.DeploymentSignature = ""
	res.Org = "testorg"

	return res
}

func createSkeletalUserInputs(directory string) error {

	// Create a skeletal user input config object with fillins/place-holders for configuration.
	res := new(common.UserInputFile)
	res.Global = []common.GlobalSet{}

	// Create a skeletal array with one element for variable configuration.
	res.Services = []policy.AbstractUserInput{}

	// Convert the object to JSON and write it into the project.
	return CreateFile(directory, USERINPUT_FILE, res)

}
func cleanTestDir(dirPath string) error {
	if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
		if err := os.RemoveAll(dirPath); err != nil {
			return err
		}
	}
	return nil
}
