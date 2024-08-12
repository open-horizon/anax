package resource

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/edge-sync-service/common"
	"math/big"
	"net"
	"os"
	"path"
	"time"
)

const (
	rsaBits      = 2048
	daysValidFor = 500
)

// Create ESS Cert file and key file if not exist
func CreateCertificate(org string, keyPath string, certPath string) error {

	// get message printer, this function is called by CLI
	msgPrinter := i18n.GetMessagePrinter()

	common.Configuration.ServerCertificate = path.Join(certPath, config.HZN_FSS_CERT_FILE)
	common.Configuration.ServerKey = path.Join(keyPath, config.HZN_FSS_CERT_KEY_FILE)

	if fileExists(common.Configuration.ServerCertificate) && fileExists(common.Configuration.ServerKey) {
		glog.V(3).Infof(reslog(fmt.Sprintf("ESS self signed cert and key already exist in %v, %v, skip creating...", common.Configuration.ServerCertificate, common.Configuration.ServerKey)))
		return nil
	}

	glog.V(5).Infof(reslog(fmt.Sprintf("creating self signed cert in %v", common.Configuration.ServerCertificate)))

	if err := os.MkdirAll(certPath, 0755); err != nil {
		return errors.New(msgPrinter.Sprintf("unable to make directory for self signed MMS API certificate, error %v", err))
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(daysValidFor * 24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return errors.New(msgPrinter.Sprintf("unable to generate random number for MMS API certificate serial number, error %v", err))
	}

	priv, err := rsa.GenerateKey(rand.Reader, rsaBits)
	if err != nil {
		return errors.New(msgPrinter.Sprintf("unable to generate private key for MMS API certificate, error %v", err))
	}

	ipAddress := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}

	agentNS := cutil.GetClusterNamespace()
	dnsName1 := fmt.Sprintf("agent-service.%v.svc.cluster.local", agentNS)
	dnsName2 := fmt.Sprintf("agent-service.%v.svc", agentNS)

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization:       []string{org},
			OrganizationalUnit: []string{"Edge Node"},
			CommonName:         "localhost",
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost", "e2edevtest", dnsName1, "agent-service", dnsName2},
		IPAddresses:           ipAddress,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return errors.New(msgPrinter.Sprintf("unable to create MMS API certificate, error %v", err))
	}

	certOut, err := os.Create(common.Configuration.ServerCertificate)
	if err != nil {
		return errors.New(msgPrinter.Sprintf("unable to write MMS API certificate to file %v, error %v", common.Configuration.ServerCertificate, err))
	}

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return errors.New(msgPrinter.Sprintf("unable to encode MMS API certificate to file %v, error %v", common.Configuration.ServerCertificate, err))
	}

	if err := certOut.Close(); err != nil {
		return errors.New(msgPrinter.Sprintf("unable to close MMS API certificate file %v, error %v", common.Configuration.ServerCertificate, err))
	}

	keyOut, err := os.OpenFile(common.Configuration.ServerKey, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return errors.New(msgPrinter.Sprintf("unable to write MMS API certificate private key to file %v, error %v", common.Configuration.ServerKey, err))
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}); err != nil {
		return errors.New(msgPrinter.Sprintf("unable to encode MMS API certificate private key to file %v, error %v", common.Configuration.ServerKey, err))
	}

	if err := keyOut.Close(); err != nil {
		return errors.New(msgPrinter.Sprintf("unable to close MMS API certificate private key file %v, error %v", common.Configuration.ServerKey, err))
	}

	glog.V(3).Infof(reslog(fmt.Sprintf("created MMS API SSL certificate at %v", common.Configuration.ServerCertificate)))

	return nil
}

// This function checks if file exits or not
func fileExists(filename string) bool {
	fileinfo, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	if fileinfo.IsDir() {
		return false
	}

	return true
}
