package config

import (
	"path"
	"testing"
)

func Test_default_FSS(t *testing.T) {

	testCfg := &HorizonConfig{
		Edge: Config{
			FileSyncService: FSSConfig{
			},
		},
	}

	if !testCfg.FSSIsUnixProtocol() {
		t.Errorf("config API should indicate unix FSS protocol in use")
	} else if testCfg.GetFileSyncServiceProtocol() != "unix" {
		t.Errorf("config API should indicate unix FSS protocol value, is %v", testCfg.GetFileSyncServiceProtocol())
	} else if testCfg.GetFileSyncServiceAPIPort() != 0 {
		t.Errorf("config API should indicate port 0, is %v", testCfg.GetFileSyncServiceAPIPort())
	} else if testCfg.GetFileSyncServiceAPIListen() != path.Join(HZN_FSS_DOMAIN_SOCKET_PATH, HZN_FSS_DOMAIN_SOCKET) {
		t.Errorf("config API should be the default unix socket, is %v", testCfg.GetFileSyncServiceAPIListen())
	} else if testCfg.GetFileSyncServiceAPIUnixDomainSocketPath() != HZN_FSS_DOMAIN_SOCKET_PATH {
		t.Errorf("config API should be the default unix socket path, is %v", testCfg.GetFileSyncServiceAPIUnixDomainSocketPath())
	} else if testCfg.GetFileSyncServiceStoragePath() != path.Join(getDefaultBase(), HZN_FSS_STORAGE_PATH) {
		t.Errorf("config API should be the default persistence path, is %v", testCfg.GetFileSyncServiceStoragePath())
	} else if testCfg.GetFileSyncServiceAuthPath() != path.Join(getDefaultBase(), HZN_FSS_AUTH_PATH) {
		t.Errorf("config API should be the default authentication path, is %v", testCfg.GetFileSyncServiceAuthPath())
	} else if testCfg.GetCSSURL() != "" {
		t.Errorf("config API should have an empty CSS URL, is %v", testCfg.GetCSSURL())
	} else if testCfg.GetCSSPort() != 0 {
		t.Errorf("config API should have a zero CSS Port, is %v", testCfg.GetCSSPort())
	}

}

func Test_unix_config_FSS(t *testing.T) {

	testCfg := &HorizonConfig{
		Edge: Config{
			FileSyncService: FSSConfig{
				APIListen: "/var/run/something/my.sock",
			},
		},
	}

	if !testCfg.FSSIsUnixProtocol() {
		t.Errorf("config API should indicate unix FSS protocol in use")
	} else if testCfg.GetFileSyncServiceProtocol() != "unix" {
		t.Errorf("config API should indicate unix FSS protocol value, is %v", testCfg.GetFileSyncServiceProtocol())
	} else if testCfg.GetFileSyncServiceAPIPort() != 0 {
		t.Errorf("config API should indicate port 0, is %v", testCfg.GetFileSyncServiceAPIPort())
	} else if testCfg.GetFileSyncServiceAPIListen() != "/var/run/something/my.sock" {
		t.Errorf("config API should be the default unix socket, is %v", testCfg.GetFileSyncServiceAPIListen())
	} else if testCfg.GetFileSyncServiceAPIUnixDomainSocketPath() != "/var/run/something" {
		t.Errorf("config API should be the default unix socket path, is %v", testCfg.GetFileSyncServiceAPIUnixDomainSocketPath())
	}

}

func Test_unix_config_error_FSS(t *testing.T) {

	testCfg := &HorizonConfig{
		Edge: Config{
			FileSyncService: FSSConfig{
				APIListen: "1.1.1.1",
				APIPort: 5555,
			},
		},
	}

	if !testCfg.FSSIsUnixProtocol() {
		t.Errorf("config API should indicate unix FSS protocol in use")
	} else if testCfg.GetFileSyncServiceProtocol() != "unix" {
		t.Errorf("config API should indicate unix FSS protocol value, is %v", testCfg.GetFileSyncServiceProtocol())
	} else if testCfg.GetFileSyncServiceAPIPort() != 0 {
		t.Errorf("config API should indicate port 0, is %v", testCfg.GetFileSyncServiceAPIPort())
	} else if testCfg.GetFileSyncServiceAPIListen() != path.Join(HZN_FSS_DOMAIN_SOCKET_PATH, HZN_FSS_DOMAIN_SOCKET) {
		t.Errorf("config API should be the default unix socket, is %v", testCfg.GetFileSyncServiceAPIListen())
	} else if testCfg.GetFileSyncServiceAPIUnixDomainSocketPath() != HZN_FSS_DOMAIN_SOCKET_PATH {
		t.Errorf("config API should be the default unix socket path, is %v", testCfg.GetFileSyncServiceAPIUnixDomainSocketPath())
	}

}

func Test_TCP_default_FSS(t *testing.T) {

	testCfg := &HorizonConfig{
		Edge: Config{
			FileSyncService: FSSConfig{
				APIProtocol: "https",

			},
		},
	}

	if testCfg.FSSIsUnixProtocol() {
		t.Errorf("config API should not indicate unix FSS protocol in use")
	} else if testCfg.GetFileSyncServiceProtocol() != "https" {
		t.Errorf("config API should indicate https FSS protocol value, is %v", testCfg.GetFileSyncServiceProtocol())
	} else if testCfg.GetFileSyncServiceAPIPort() != HZN_FSS_API_LISTEN_PORT_DEFAULT {
		t.Errorf("config API should indicate port %v, is %v", HZN_FSS_API_LISTEN_PORT_DEFAULT, testCfg.GetFileSyncServiceAPIPort())
	} else if testCfg.GetFileSyncServiceAPIListen() != HZN_FSS_API_LISTEN_DEFAULT {
		t.Errorf("config API should be the default listen address, is %v", testCfg.GetFileSyncServiceAPIListen())
	} else if testCfg.GetFileSyncServiceAPIUnixDomainSocketPath() != "" {
		t.Errorf("config API should not return a unix socket path, is %v", testCfg.GetFileSyncServiceAPIUnixDomainSocketPath())
	} else if testCfg.GetFileSyncServiceStoragePath() != path.Join(getDefaultBase(), HZN_FSS_STORAGE_PATH) {
		t.Errorf("config API should be the default persistence path, is %v", testCfg.GetFileSyncServiceStoragePath())
	} else if testCfg.GetFileSyncServiceAuthPath() != path.Join(getDefaultBase(), HZN_FSS_AUTH_PATH) {
		t.Errorf("config API should be the default authentication path, is %v", testCfg.GetFileSyncServiceAuthPath())
	} else if testCfg.GetCSSURL() != "" {
		t.Errorf("config API should have an empty CSS URL, is %v", testCfg.GetCSSURL())
	} else if testCfg.GetCSSPort() != 0 {
		t.Errorf("config API should have a zero CSS Port, is %v", testCfg.GetCSSPort())
	}

}

func Test_TCP_config_FSS(t *testing.T) {

	testCfg := &HorizonConfig{
		Edge: Config{
			FileSyncService: FSSConfig{
				APIProtocol: "https",
				APIPort: 8888,
				APIListen: "1.1.1.1",
				PersistencePath: "/tmp/",
				AuthenticationPath: "/tmp/auth/",
				CSSURL: "cloud.css.com",
				CSSPort: 7777,
			},
		},
	}

	if testCfg.FSSIsUnixProtocol() {
		t.Errorf("config API should not indicate unix FSS protocol in use")
	} else if testCfg.GetFileSyncServiceProtocol() != "https" {
		t.Errorf("config API should indicate https FSS protocol value, is %v", testCfg.GetFileSyncServiceProtocol())
	} else if testCfg.GetFileSyncServiceAPIPort() != 8888 {
		t.Errorf("config API should indicate port %v, is %v", 8888, testCfg.GetFileSyncServiceAPIPort())
	} else if testCfg.GetFileSyncServiceAPIListen() != "1.1.1.1" {
		t.Errorf("config API should be the default listen address, is %v", testCfg.GetFileSyncServiceAPIListen())
	} else if testCfg.GetFileSyncServiceAPIUnixDomainSocketPath() != "" {
		t.Errorf("config API should not return a unix socket path, is %v", testCfg.GetFileSyncServiceAPIUnixDomainSocketPath())
	} else if testCfg.GetFileSyncServiceStoragePath() != "/tmp/" {
		t.Errorf("config API should be the default persistence path, is %v", testCfg.GetFileSyncServiceStoragePath())
	} else if testCfg.GetFileSyncServiceAuthPath() != "/tmp/auth/" {
		t.Errorf("config API should be the default authentication path, is %v", testCfg.GetFileSyncServiceAuthPath())
	} else if testCfg.GetCSSURL() != "cloud.css.com" {
		t.Errorf("config API should have an empty CSS URL, is %v", testCfg.GetCSSURL())
	} else if testCfg.GetCSSPort() != 7777 {
		t.Errorf("config API should have a zero CSS Port, is %v", testCfg.GetCSSPort())
	}

}
