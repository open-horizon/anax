package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/golang/glog"
	"io/ioutil"
	"net/http"
	"time"
)

// This exists to consolidate construction of clients to collaborating
// systems or other shared resources. We may need to do some poor man's
// DI here with build flags and selective compilation against varying
// concrete interfaces here.

type Collaborators struct {
	HTTPClientFactory *HTTPClientFactory
}

func NewCollaborators(hConfig HorizonConfig) (*Collaborators, error) {
	httpClientFactory, err := newHTTPClientFactory(hConfig)
	if err != nil {
		return nil, err
	}

	return &Collaborators{
		HTTPClientFactory: httpClientFactory,
	}, nil
}

func (c *Collaborators) String() string {
	return fmt.Sprintf("HTTPClientFactory: %v", c.HTTPClientFactory)
}

type HTTPClientFactory struct {
	NewHTTPClient func(overrideTimeoutS *uint) *http.Client
}

// TODO: use a pool of clients instead of creating them forevar
func newHTTPClientFactory(hConfig HorizonConfig) (*HTTPClientFactory, error) {
	var caBytes []byte

	if hConfig.Edge.CACertsPath != "" {
		var err error
		caBytes, err = ioutil.ReadFile(hConfig.Edge.CACertsPath)
		if err != nil {
			return nil, fmt.Errorf("Failed to read CACertsFile: %v", hConfig.Edge.CACertsPath)
		}
		glog.V(4).Infof("Read CA certs from provided file %v", hConfig.Edge.CACertsPath)
	}

	var tls tls.Config
	tls.InsecureSkipVerify = false

	var certPool *x509.CertPool

	if hConfig.Edge.TrustSystemCACerts {
		var err error
		certPool, err = x509.SystemCertPool()
		if err != nil {
			return nil, err
		}
		glog.V(4).Info("Added distribution-provided CA Certs to trust")

	} else {
		certPool = x509.NewCertPool()
	}

	certPool.AppendCertsFromPEM(caBytes)
	tls.RootCAs = certPool

	glog.V(5).Infof("Full HTTP client CA Cert trust list subject names: %v", certPool.Subjects())

	tls.BuildNameToCertificate()

	clientFunc := func(overrideTimeoutS *uint) *http.Client {
		var timeoutS uint

		if overrideTimeoutS != nil {
			timeoutS = *overrideTimeoutS
		} else {
			timeoutS = hConfig.Edge.DefaultHTTPClientTimeoutS
		}

		return &http.Client{
			Timeout: time.Second * time.Duration(timeoutS),
			Transport: &http.Transport{
				MaxIdleConns:    MaxHTTPIdleConnections,
				IdleConnTimeout: HTTPIdleConnectionTimeoutS * time.Second,
				TLSClientConfig: &tls,
			},
		}
	}

	return &HTTPClientFactory{
		NewHTTPClient: clientFunc,
	}, nil
}
