// +build unit

package download

import (
	"github.com/boltdb/bolt"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/externalpolicy"
	"github.com/open-horizon/anax/persistence"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"
)

func Test_ResolveUpgradeVersions(t *testing.T) {
	dir, db, err := setupDB()
	if err != nil {
		t.Errorf("Error setting up db for tests: %v", err)
	}
	defer cleanupDB(dir)

	w := NewDownloadWorker("download", &config.HorizonConfig{}, db)

	dev, err := persistence.SaveNewExchangeDevice(db, "testNode", "testNodeTok", "testNode", persistence.DEVICE_TYPE_DEVICE, false, "userdev", "", persistence.CONFIGSTATE_CONFIGURED, persistence.SoftwareVersion{persistence.AGENT_VERSION: "2.1.1", persistence.CONFIG_VERSION: "", persistence.CERT_VERSION: "1.2.3"})
	if err != nil {
		t.Errorf("Error saving exchange device in db: %v", err)
	}

	upgradeVers := exchangecommon.AgentUpgradeVersions{SoftwareVersion: "2.1.2", ConfigVersion: "1.1.2", CertVersion: "1.5.2"}
	nmpStatus := exchangecommon.NodeManagementPolicyStatus{AgentUpgradeInternal: &exchangecommon.AgentUpgradeInternalStatus{AllowDowngrade: false, ScheduledUnixTime: time.Unix(1649503122, 0)}}
	versToUpgrade, err := w.ResolveUpgradeVersions(&upgradeVers, "testNMP", &nmpStatus)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if versToUpgrade.SoftwareVersion != "2.1.2" {
		t.Errorf("Expected software upgrade version \"%v\". Got \"%v\".", "2.1.2", versToUpgrade.SoftwareVersion)
	} else if versToUpgrade.ConfigVersion != "1.1.2" {
		t.Errorf("Expected config upgrade version \"%v\". Got \"%v\".", "1.1.2", versToUpgrade.ConfigVersion)
	} else if versToUpgrade.CertVersion != "1.5.2" {
		t.Errorf("Expected cert upgrade version \"%v\". Got \"%v\".", "1.5.2", versToUpgrade.CertVersion)
	}

	dev, err = dev.SetConfigVersion(db, dev.Id, "1.1.1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	upgradeVers = exchangecommon.AgentUpgradeVersions{ConfigVersion: "1.1.2", CertVersion: "1.1.3"}
	versToUpgrade, err = w.ResolveUpgradeVersions(&upgradeVers, "testNMP", &nmpStatus)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if versToUpgrade.SoftwareVersion != "" {
		t.Errorf("Expected software upgrade version \"%v\". Got \"%v\".", "", versToUpgrade.SoftwareVersion)
	} else if versToUpgrade.ConfigVersion != "1.1.2" {
		t.Errorf("Expected config upgrade version \"%v\". Got \"%v\".", "1.1.2", versToUpgrade.ConfigVersion)
	} else if versToUpgrade.CertVersion != "" {
		t.Errorf("Expected cert upgrade version \"%v\". Got \"%v\".", "", versToUpgrade.CertVersion)
	}

	nmpStatus.AgentUpgradeInternal.AllowDowngrade = true
	// scheduled time after the upgrade we are checking
	err = persistence.SaveOrUpdateNMPStatus(db, "userdev/nmp1", exchangecommon.NodeManagementPolicyStatus{AgentUpgradeInternal: &exchangecommon.AgentUpgradeInternalStatus{ScheduledUnixTime: time.Unix(1649503222, 0)}, AgentUpgrade: &exchangecommon.AgentUpgradePolicyStatus{UpgradedVersions: exchangecommon.AgentUpgradeVersions{SoftwareVersion: "2.1.1"}}})
	upgradeVers = exchangecommon.AgentUpgradeVersions{SoftwareVersion: "1.2.2", ConfigVersion: "1.0.1"}
	versToUpgrade, err = w.ResolveUpgradeVersions(&upgradeVers, "testNMP", &nmpStatus)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if versToUpgrade.SoftwareVersion != "" {
		t.Errorf("Expected software upgrade version \"%v\". Got \"%v\".", "", versToUpgrade.SoftwareVersion)
	} else if versToUpgrade.ConfigVersion != "1.0.1" {
		t.Errorf("Expected config upgrade version \"%v\". Got \"%v\".", "1.0.1", versToUpgrade.ConfigVersion)
	} else if versToUpgrade.CertVersion != "" {
		t.Errorf("Expected cert upgrade version \"%v\". Got \"%v\".", "", versToUpgrade.CertVersion)
	}

	err = persistence.SaveOrUpdateNMPStatus(db, "userdev/nmp1", exchangecommon.NodeManagementPolicyStatus{AgentUpgradeInternal: &exchangecommon.AgentUpgradeInternalStatus{ScheduledUnixTime: time.Unix(1649503422, 0)}, AgentUpgrade: &exchangecommon.AgentUpgradePolicyStatus{UpgradedVersions: exchangecommon.AgentUpgradeVersions{ConfigVersion: "1.1.1", CertVersion: "1.2.3"}}})
	nmpStatus.AgentUpgradeInternal.ScheduledUnixTime = time.Unix(1649503322, 0)
	upgradeVers = exchangecommon.AgentUpgradeVersions{SoftwareVersion: "0.0.1", ConfigVersion: "0.0.1", CertVersion: "0.0.1"}
	versToUpgrade, err = w.ResolveUpgradeVersions(&upgradeVers, "testNMP", &nmpStatus)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if versToUpgrade.SoftwareVersion != "0.0.1" {
		t.Errorf("Expected software upgrade version \"%v\". Got \"%v\".", "0.0.1", versToUpgrade.SoftwareVersion)
	} else if versToUpgrade.ConfigVersion != "" {
		t.Errorf("Expected config upgrade version \"%v\". Got \"%v\".", "", versToUpgrade.ConfigVersion)
	} else if versToUpgrade.CertVersion != "" {
		t.Errorf("Expected cert upgrade version \"%v\". Got \"%v\".", "", versToUpgrade.CertVersion)
	}
}

func Test_findAgentUpgradePackageVersions(t *testing.T) {
	availVers := exchangecommon.AgentFileVersions{SoftwareVersions: []string{"1.0.1", "1.0.5", "2.3.4"}, ConfigVersions: []string{"2.0.1", "3.0.5", "4.4.4"}, CertVersions: []string{"0.0.5"}}
	vers, err := findAgentUpgradePackageVersions(LATESTVERSION, "3.0.5", LATESTVERSION, getAvailibleVersionsHandler(&availVers))
	if err != nil {
		t.Errorf("No error expected but got %v.", err)
	} else if vers.SoftwareVersion != "2.3.4" {
		t.Errorf("Expected software version \"2.3.4\". Got \"%v\".", vers.SoftwareVersion)
	} else if vers.ConfigVersion != "3.0.5" {
		t.Errorf("Expected config version \"3.0.5\". Got \"%v\".", vers.ConfigVersion)
	} else if vers.CertVersion != "0.0.5" {
		t.Errorf("Expected cert version \"0.0.5\". Got \"%v\".", vers.CertVersion)
	}

	vers, err = findAgentUpgradePackageVersions("3.0.5", "3.0.5", "", getAvailibleVersionsHandler(&availVers))
	if err == nil {
		t.Errorf("No matching software version availible. Expected error but got none. Versions returned were: %v", vers)
	}

	vers, err = findAgentUpgradePackageVersions("1.0.1", LATESTVERSION, "", getAvailibleVersionsHandler(&availVers))
	if err != nil {
		t.Errorf("No error expected but got %v.", err)
	} else if vers.SoftwareVersion != "1.0.1" {
		t.Errorf("Expected software version \"1.0.1\". Got \"%v\".", vers.SoftwareVersion)
	} else if vers.ConfigVersion != "4.4.4" {
		t.Errorf("Expected config version \"4.4.4\". Got \"%v\".", vers.ConfigVersion)
	} else if vers.CertVersion != "" {
		t.Errorf("Expected cert version \"\". Got \"%v\".", vers.CertVersion)
	}
}

func getAvailibleVersionsHandler(vers *exchangecommon.AgentFileVersions) exchange.NodeUpgradeVersionsHandler {
	return func() (*exchangecommon.AgentFileVersions, error) {
		return vers, nil
	}
}

func Test_findBestMatchingVersion(t *testing.T) {
	availVers := []string{"1.0.0", "1.0.2", "3.5.12", "4.23.1"}

	vers, err := findBestMatchingVersion(availVers, "1.0.1")
	if vers != "" {
		t.Errorf("No matching version expected but got \"%v\".", vers)
	} else if err == nil {
		t.Errorf("Expected error for no matching version but did not get an error.")
	}

	vers, err = findBestMatchingVersion(availVers, LATESTVERSION)
	if err != nil {
		t.Errorf("No error expected but got %v.", err)
	} else if vers != "4.23.1" {
		t.Errorf("Expected latest availible version \"4.23.1\" but got \"%v\"", vers)
	}

	availVers = append(availVers, "4.23.2")
	vers, err = findBestMatchingVersion(availVers, LATESTVERSION)
	if err != nil {
		t.Errorf("No error expected but got %v.", err)
	} else if vers != "4.23.2" {
		t.Errorf("Expected latest availible version \"4.23.2\" but got \"%v\"", vers)
	}
}

func Test_getPkgArch(t *testing.T) {
	pkgArch := getPkgArch(RHELPACKAGETYPE, "arm")
	if pkgArch != "armhf" {
		t.Errorf("Package type should have been \"armhf\". Got \"%v\" instead.", pkgArch)
	}
	pkgArch = getPkgArch(RHELPACKAGETYPE, "amd64")
	if pkgArch != "x86_64" {
		t.Errorf("Package type should have been \"x86_64\". Got \"%v\" instead.", pkgArch)
	}
	pkgArch = getPkgArch(MACPACKAGETYPE, "amd64")
	if pkgArch != "x86_64" {
		t.Errorf("Package type should have been \"x86_64\". Got \"%v\" instead.", pkgArch)
	}
	pkgArch = getPkgArch(DEBPACKAGETYPE, "amd64")
	if pkgArch != "amd64" {
		t.Errorf("Package type should have been \"amd64\". Got \"%v\" instead.", pkgArch)
	}
}

func Test_formAgentUpgradePackageNames(t *testing.T) {
	dir, db, err := setupDB()
	if err != nil {
		t.Errorf("Error setting up db for tests: %v", err)
	}
	defer cleanupDB(dir)

	nodeProps := externalpolicy.PropertyList{}
	nodeProps = append(nodeProps, *externalpolicy.Property_Factory(externalpolicy.PROP_NODE_ARCH, "amd64"))
	nodeProps = append(nodeProps, *externalpolicy.Property_Factory(externalpolicy.PROP_NODE_OS, externalpolicy.OS_CLUSTER))
	nodeProps = append(nodeProps, *externalpolicy.Property_Factory(externalpolicy.PROP_NODE_CONTAINERIZED, false))
	nodePol := exchangecommon.NodePolicy{ExternalPolicy: externalpolicy.ExternalPolicy{Properties: nodeProps}}
	err = persistence.SaveNodePolicy(db, &nodePol)
	if err != nil {
		t.Errorf("Error saving node policy to db: %v", err)
	}

	dev, err := persistence.SaveNewExchangeDevice(db, "testNode", "testNode123", "testNode", persistence.DEVICE_TYPE_CLUSTER, false, "userdev", "", persistence.CONFIGSTATE_CONFIGURED, persistence.SoftwareVersion{})
	if err != nil {
		t.Errorf("Error saving node to db: %v", err)
	}

	w := NewDownloadWorker("download", &config.HorizonConfig{}, db)

	if downloadFiles, err := w.formAgentUpgradePackageNames(); err != nil {
		t.Errorf("No error expected. Got %v.", err)
	} else if len(*downloadFiles) != 2 {
		t.Errorf("Expected 2 files for download. Got %v.", downloadFiles)
	} else if !cutil.SliceContains(*downloadFiles, HZN_CLUSTER_FILE) {
		t.Errorf("Did not find expected file %s for download. Got %v.", HZN_CLUSTER_FILE, downloadFiles)
	} else if !cutil.SliceContains(*downloadFiles, HZN_CLUSTER_IMAGE) {
		t.Errorf("Did not find expected file %s for download. Got %v.", HZN_CLUSTER_IMAGE, downloadFiles)
	}

	nodeProps.Add_Property(externalpolicy.Property_Factory(externalpolicy.PROP_NODE_OS, externalpolicy.OS_UBUNTU), true)
	nodePol.Properties = nodeProps
	err = persistence.SaveNodePolicy(db, &nodePol)
	if err != nil {
		t.Errorf("Error saving node policy to db: %v", err)
	}

	dev, err = dev.SetNodeType(db, "testNode", persistence.DEVICE_TYPE_DEVICE)
	if err != nil {
		t.Errorf("Error updating node in db: %v", err)
	}

	if downloadFiles, err := w.formAgentUpgradePackageNames(); err != nil {
		t.Errorf("No error expected. Got %v.", err)
	} else if len(*downloadFiles) != 1 {
		t.Errorf("Expected 1 file for download. Got %v.", downloadFiles)
	} else if !cutil.SliceContains(*downloadFiles, "horizon-agent-linux-deb-amd64.tar.gz") {
		t.Errorf("Did not find expected file %s for download. Got %v.", "horizon-agent-linux-deb-amd64.tar.gz", downloadFiles)
	}

	nodeProps.Add_Property(externalpolicy.Property_Factory(externalpolicy.PROP_NODE_OS, externalpolicy.OS_RHEL), true)
	nodePol.Properties = nodeProps
	err = persistence.SaveNodePolicy(db, &nodePol)
	if err != nil {
		t.Errorf("Error saving node policy to db: %v", err)
	}

	if downloadFiles, err := w.formAgentUpgradePackageNames(); err != nil {
		t.Errorf("No error expected. Got %v.", err)
	} else if len(*downloadFiles) != 1 {
		t.Errorf("Expected 1 file for download. Got %v.", downloadFiles)
	} else if !cutil.SliceContains(*downloadFiles, "horizon-agent-linux-rpm-x86_64.tar.gz") {
		t.Errorf("Did not find expected file %s for download. Got %v.", "horizon-agent-linux-rpm-x86_64.tar.gz", downloadFiles)
	}

	nodeProps.Add_Property(externalpolicy.Property_Factory(externalpolicy.PROP_NODE_CONTAINERIZED, true), true)
	nodePol.Properties = nodeProps
	err = persistence.SaveNodePolicy(db, &nodePol)
	if err != nil {
		t.Errorf("Error saving node policy to db: %v", err)
	}

	if downloadFiles, err := w.formAgentUpgradePackageNames(); err != nil {
		t.Errorf("No error expected. Got %v.", err)
	} else if len(*downloadFiles) != 2 {
		t.Errorf("Expected 2 file for download. Got %v.", downloadFiles)
	} else if !cutil.SliceContains(*downloadFiles, "horizon-agent-linux-rpm-x86_64.tar.gz") {
		t.Errorf("Did not find expected file %s for download. Got %v.", "horizon-agent-linux-rpm-x86_64.tar.gz", downloadFiles)
	} else if !cutil.SliceContains(*downloadFiles, "amd64_anax.tar.gz") {
		t.Errorf("Did not find expected file %s for download. Got %v.", "amd64_anax.tar.gz", downloadFiles)
	}

	nodeProps.Add_Property(externalpolicy.Property_Factory(externalpolicy.PROP_NODE_OS, externalpolicy.OS_MAC), true)
	nodePol.Properties = nodeProps
	err = persistence.SaveNodePolicy(db, &nodePol)
	if err != nil {
		t.Errorf("Error saving node policy to db: %v", err)
	}

	if downloadFiles, err := w.formAgentUpgradePackageNames(); err != nil {
		t.Errorf("No error expected. Got %v.", err)
	} else if len(*downloadFiles) != 2 {
		t.Errorf("Expected 2 file for download. Got %v.", downloadFiles)
	} else if !cutil.SliceContains(*downloadFiles, "horizon-agent-macos-pkg-x86_64.tar.gz") {
		t.Errorf("Did not find expected file %s for download. Got %v.", "horizon-agent-macos-pkg-x86_64.tar.gz", downloadFiles)
	} else if !cutil.SliceContains(*downloadFiles, "amd64_anax.tar.gz") {
		t.Errorf("Did not find expected file %s for download. Got %v.", "amd64_anax.tar.gz", downloadFiles)
	}

	nodeProps.Add_Property(externalpolicy.Property_Factory(externalpolicy.PROP_NODE_OS, externalpolicy.OS_DEBIAN), true)
	nodeProps.Add_Property(externalpolicy.Property_Factory(externalpolicy.PROP_NODE_CONTAINERIZED, false), true)
	nodeProps.Add_Property(externalpolicy.Property_Factory(externalpolicy.PROP_NODE_ARCH, "arm"), true)
	nodePol.Properties = nodeProps
	err = persistence.SaveNodePolicy(db, &nodePol)
	if err != nil {
		t.Errorf("Error saving node policy to db: %v", err)
	}

	if downloadFiles, err := w.formAgentUpgradePackageNames(); err != nil {
		t.Errorf("No error expected. Got %v.", err)
	} else if len(*downloadFiles) != 1 {
		t.Errorf("Expected 1 file for download. Got %v.", downloadFiles)
	} else if !cutil.SliceContains(*downloadFiles, "horizon-agent-linux-deb-armhf.tar.gz") {
		t.Errorf("Did not find expected file %s for download. Got %v.", "horizon-agent-linux-deb-armhf.tar.gz", downloadFiles)
	}

	nodeProps.Add_Property(externalpolicy.Property_Factory(externalpolicy.PROP_NODE_ARCH, "arm64"), true)
	nodePol.Properties = nodeProps
	err = persistence.SaveNodePolicy(db, &nodePol)
	if err != nil {
		t.Errorf("Error saving node policy to db: %v", err)
	}

	if downloadFiles, err := w.formAgentUpgradePackageNames(); err != nil {
		t.Errorf("No error expected. Got %v.", err)
	} else if len(*downloadFiles) != 1 {
		t.Errorf("Expected 1 file for download. Got %v.", downloadFiles)
	} else if !cutil.SliceContains(*downloadFiles, "horizon-agent-linux-deb-arm64.tar.gz") {
		t.Errorf("Did not find expected file %s for download. Got %v.", "horizon-agent-linux-deb-arm64.tar.gz", downloadFiles)
	}
}

func setupDB() (string, *bolt.DB, error) {
	dir, err := ioutil.TempDir("", "container-")
	if err != nil {
		return "", nil, err
	}

	db, err := bolt.Open(path.Join(dir, "anax-int.db"), 0600, &bolt.Options{Timeout: 10 * time.Second})
	if err != nil {
		return dir, nil, err
	}

	return dir, db, nil
}

func cleanupDB(dir string) error {
	return os.RemoveAll(dir)
}
