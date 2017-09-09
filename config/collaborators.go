package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
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
	httpClientFactory, err := NewHTTPClientFactory(hConfig)
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

func NewHTTPClientFactory(hConfig HorizonConfig) (*HTTPClientFactory, error) {
	var caBytes []byte

	if hConfig.Edge.CACertsPath != "" {
		var err error
		caBytes, err = ioutil.ReadFile(hConfig.Edge.CACertsPath)
		if err != nil {
			return nil, fmt.Errorf("Failed to read CACertsFile: %v", hConfig.Edge.CACertsPath)
		}
	}

	var tls tls.Config
	tls.InsecureSkipVerify = false

	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(caBytes)
	tls.RootCAs = certPool

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
				TLSClientConfig: &tls,
			},
		}
	}

	return &HTTPClientFactory{
		NewHTTPClient: clientFunc,
	}, nil
}
