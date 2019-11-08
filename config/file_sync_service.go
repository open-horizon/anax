package config

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"os"
)

// Configuration for the File Sync Service, which is implemented by the embedded ESS.
type FSSConfig struct {
	APIListen          string // The address on which the ESS will listen. The default is in the code below. For a unix domain socket path, it must be the full path name including the file name.
	APIPort            uint16 // The port on which the ESS will listen. For a unix domain socket, this will always be "0".
	APIProtocol        string // Can be 'unix' or 'https'. Default is unix. The value of this field determines the Listen and Port values.
	PersistencePath    string // The absolute location in the host filesystem where anax stores files retrieved by the file sync service.
	AuthenticationPath string // The absolute location in the host filesystem where anax stores authentication credentials for services so that the service can authenticate to the FSS (ESS) API.
	CSSURL             string // The URL used to access the CSS.
	CSSSSLCert         string // The path to the client side SSL certificate for the CSS.
	PollingRate        uint16 // The number of seconds between polls to the CSS for notification updates.
}

func (f *FSSConfig) String() string {
	return fmt.Sprintf("APIListen: %v, APIPort: %v, APIProtocol: %v, PersistencePath: %v, AuthenticationPath: %v, CSSURL: %v, CSSSSLCert: %v, PollingRate: %v", f.APIListen, f.APIPort, f.APIProtocol, f.PersistencePath, f.AuthenticationPath, f.CSSURL, f.CSSSSLCert, f.PollingRate)
}

func (c *HorizonConfig) FSSIsUnixProtocol() bool {
	return c.Edge.FileSyncService.APIProtocol == "unix" || c.Edge.FileSyncService.APIProtocol == ""
}

func (c *HorizonConfig) GetFileSyncServiceProtocol() string {
	if c.FSSIsUnixProtocol() {
		return "secure-unix"
	}
	return c.Edge.FileSyncService.APIProtocol
}

// The APIProtocol field controls the Listen and Port fields. If the Protocol field is empty or unix, then the Listen field must be a
// unix domain socket path and the Port field will be ignored.
func (c *HorizonConfig) GetFileSyncServiceAPIPort() uint16 {
	if c.FSSIsUnixProtocol() {
		return 0
	} else {
		if c.Edge.FileSyncService.APIPort == 0 {
			return HZN_FSS_API_LISTEN_PORT_DEFAULT
		} else {
			return c.Edge.FileSyncService.APIPort
		}
	}
}

func (c *HorizonConfig) GetFileSyncServiceAPIListen() string {
	if c.FSSIsUnixProtocol() {
		if filepath.IsAbs(c.Edge.FileSyncService.APIListen) {
			return c.Edge.FileSyncService.APIListen
		} else {
			return path.Join(HZN_FSS_DOMAIN_SOCKET_PATH, HZN_FSS_DOMAIN_SOCKET)
		}
	} else if c.Edge.FileSyncService.APIListen == "" {
		return HZN_FSS_API_LISTEN_DEFAULT
	} else {
		return c.Edge.FileSyncService.APIListen
	}
}

// Return empty string if unix file socket is not in use.
func (c *HorizonConfig) GetFileSyncServiceAPIUnixDomainSocketPath() string {
	if c.FSSIsUnixProtocol() {
		if filepath.IsAbs(c.Edge.FileSyncService.APIListen) {
			return filepath.Dir(c.Edge.FileSyncService.APIListen)
		} else {
			return HZN_FSS_DOMAIN_SOCKET_PATH
		}
	}
	return ""
}

func (c *HorizonConfig) GetFileSyncServiceStoragePath() string {
	if c.Edge.FileSyncService.PersistencePath == "" {
		return path.Join(getDefaultBase(), HZN_FSS_STORAGE_PATH)
	} else {
		return c.Edge.FileSyncService.PersistencePath
	}
}

func (c *HorizonConfig) GetFileSyncServiceAuthPath() string {
	if c.Edge.FileSyncService.AuthenticationPath == "" {
		return path.Join(getDefaultBase(), HZN_FSS_AUTH_PATH)
	} else {
		return c.Edge.FileSyncService.AuthenticationPath
	}
}

func (c *HorizonConfig) GetCSSURL() string {
	return strings.TrimRight(c.Edge.FileSyncService.CSSURL, "/")
}

func (c *HorizonConfig) GetCSSSSLCert() string {
	if c.Edge.FileSyncService.CSSSSLCert == "" {
		if cp := os.Getenv(OldMgmtHubCertPath); cp != "" {
			return cp
		} else {
			return os.Getenv(ManagementHubCertPath)
		}
	} else {
		return c.Edge.FileSyncService.CSSSSLCert
	}
}

func (c *HorizonConfig) GetESSSSLClientCertPath() string {
	return path.Join(c.GetFileSyncServiceAuthPath(), "SSL", "cert")
}

func (c *HorizonConfig) GetESSSSLCertKeyPath() string {
	return path.Join(c.GetFileSyncServiceAuthPath(), "SSL")
}

func (c *HorizonConfig) GetESSPollingRate() uint16 {
	if c.Edge.FileSyncService.PollingRate == 0 {
		return HZN_FSS_POLLING_RATE
	} else {
		return c.Edge.FileSyncService.PollingRate
	}
}
