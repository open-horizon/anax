// +build integration
// +build go1.9

package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const listenOn = "127.0.0.1"

// generated with: ALTNAME=DNS:localhost,DNS:localhost.localdomain,IP:127.0.0.1 openssl req -newkey rsa:1024 -nodes -keyout collaborators-test-key.pem -x509 -days 36500 -out collaborators-test-cert.pem -subj "/C=US/ST=UT/O=Salt Lake City/CN=localhost.localdomain" -config /etc/ssl/openssl.cnf
// requires: /etc/ssl/openssl.cnf w/ section '[ v3_ca ]' having keyUsage = digitalSignature, keyEncipherment ; subjectAltName = $ENV::ALTNAME

// verify with: openssl req -in domain.csr -text -noout

const collaboratorsTestCert = `-----BEGIN CERTIFICATE-----
MIICuzCCAiSgAwIBAgIJAN7JXdbhU4bsMA0GCSqGSIb3DQEBCwUAMFMxCzAJBgNV
BAYTAlVTMQswCQYDVQQIDAJVVDEXMBUGA1UECgwOU2FsdCBMYWtlIENpdHkxHjAc
BgNVBAMMFWxvY2FsaG9zdC5sb2NhbGRvbWFpbjAgFw0xNzA5MTAyMjAwNTlaGA8y
MTE3MDgxNzIyMDA1OVowUzELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAlVUMRcwFQYD
VQQKDA5TYWx0IExha2UgQ2l0eTEeMBwGA1UEAwwVbG9jYWxob3N0LmxvY2FsZG9t
YWluMIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQCju4NREasdEKNIRB2bghrh
wcc7A5nW6TN8WG/LuDKzZbodLNbivB3BWeUNi64rn9TB5XhpMom0nOQRo4uKPuJd
2s8cKeOLpcR7Al6CUZkkrFn+i7BxShnnvR22wBdbfR+nXwNRc8/kq073U6YmLq/l
G59+b2kWmkNNgTfJbARYPwIDAQABo4GUMIGRMB0GA1UdDgQWBBTrA+IuPqqVJuhH
muQR5Krpw5GWGDAfBgNVHSMEGDAWgBTrA+IuPqqVJuhHmuQR5Krpw5GWGDAPBgNV
HRMBAf8EBTADAQH/MAsGA1UdDwQEAwIFoDAxBgNVHREEKjAogglsb2NhbGhvc3SC
FWxvY2FsaG9zdC5sb2NhbGRvbWFpbocEfwAAATANBgkqhkiG9w0BAQsFAAOBgQAu
JJZX6caJ31h1hG4X3rWTaX0K/8nNXq3XtYGPP88K0uJMWGrtTs5XLlgTxhYCo6JH
2D2YUnmSLDfIEhFDUiQGBSfABR0YDAe/3nC8q8Gk5m4VGHmdszY/EbzCdl7Nf/MZ
vSDcr4UB3UEwhhdfJ+hzPzSSPknSmUczZ9UoVvk48Q==
-----END CERTIFICATE-----`
const collaboratorsTestKey = `-----BEGIN PRIVATE KEY-----
MIICdgIBADANBgkqhkiG9w0BAQEFAASCAmAwggJcAgEAAoGBAKO7g1ERqx0Qo0hE
HZuCGuHBxzsDmdbpM3xYb8u4MrNluh0s1uK8HcFZ5Q2Lriuf1MHleGkyibSc5BGj
i4o+4l3azxwp44ulxHsCXoJRmSSsWf6LsHFKGee9HbbAF1t9H6dfA1Fzz+SrTvdT
piYur+Ubn35vaRaaQ02BN8lsBFg/AgMBAAECgYBufulMGKRl5QiMiIuCmvcRS/js
Nq3nf1GjpPstfI2azBgiAFS0h0d9aPFPhuhvwFmQ0Q/FzrloDklMLhbJoU6Z++kz
+RLy3rKX4zv1Sv5R4M0gBNRzr10EzCmJS5rOOrwPABLAfEnlagV4T4sDARY+V0Iz
N2u50DNoben+cQsywQJBAM9lzhbC0J1I4C17iIIOLsUIQuruWxLJ0iiYXS+zxPeH
marEe70sh6A7w6H9xZ4vRxdtr1xQYbuFjfqGUSbzeEcCQQDKGiV5ien0SQBGwLTq
CCR4B52/5VhvG11S3oyKLvlr54D1LcSb7eHe/6XDAMhnbBWFr04+SFMYr3t+5l8t
gpRJAkAAyBpxvYQ5w4eMxFVsYA9PEMvnxMQ1Guue2YwoXN4WLL2ohhsNSHiuYutG
1gUDppv2+6PYjjkAEu3JDu6JXguLAkEApLrPFNOu2CiwivsD+0YLw7IhiIo9nMJn
POadEvza3HLkD/PwL1CkLImf6OQ4dOQKXt7XHbkB0jsmo/bOWV/30QJAHXH3aoKT
P3RnSaBJcRIbi8VEGBP4DKSV5wky/KpGSRoNVqONuPnUAHOSyNGAqi402aq/tBLk
3CfGb5vgRdDMJQ==
-----END PRIVATE KEY-----`

// generated with: ALTNAME=DNS:testo openssl req -newkey rsa:1024 -nodes -keyout collaborators-test-key.pem -x509 -days 36500 -out collaborators-test-cert.pem -subj "/C=US/ST=UT/O=Salt Lake City/CN=testo" -config /etc/ssl/openssl.cnf
// requires: /etc/ssl/openssl.cnf w/ section '[ v3_ca ]' having keyUsage = digitalSignature, keyEncipherment ; subjectAltName = $ENV::ALTNAME

// verify with: openssl req -in domain.csr -text -noout

const collaboratorsOtherTestCert = `-----BEGIN CERTIFICATE-----
MIICeDCCAeGgAwIBAgIJALUrEoK32k14MA0GCSqGSIb3DQEBCwUAMEMxCzAJBgNV
BAYTAlVTMQswCQYDVQQIDAJVVDEXMBUGA1UECgwOU2FsdCBMYWtlIENpdHkxDjAM
BgNVBAMMBXRlc3RvMCAXDTE3MDkxMDIxNDAzNVoYDzIxMTcwODE3MjE0MDM1WjBD
MQswCQYDVQQGEwJVUzELMAkGA1UECAwCVVQxFzAVBgNVBAoMDlNhbHQgTGFrZSBD
aXR5MQ4wDAYDVQQDDAV0ZXN0bzCBnzANBgkqhkiG9w0BAQEFAAOBjQAwgYkCgYEA
2yy2sJu5DSJl6Cbdc78IlrvL9GBJ55sjQPco0Yhqef/5y0I/OMXMPTYTfEOB1boz
ErbpZYu/O5PLLXC6J6Foqi8IKQm++yv7pzWUHRvgh7B4gv/vVrDF9XtggVmRCZ2G
q0NLGhI7GU2j5r1gxVlpSsxjJ9Tf9AvLWK4KlWGhhgMCAwEAAaNyMHAwHQYDVR0O
BBYEFI40iczJUPBB5FLj8SSFgWGI0JC5MB8GA1UdIwQYMBaAFI40iczJUPBB5FLj
8SSFgWGI0JC5MA8GA1UdEwEB/wQFMAMBAf8wCwYDVR0PBAQDAgWgMBAGA1UdEQQJ
MAeCBXRlc3RvMA0GCSqGSIb3DQEBCwUAA4GBAF5wX7P8SfGg+KlT4RYybBoXuIHz
1Z/a6SKdkdOe6UimwH5M2Jievbz7qpISRohIXfd+HRClx15XgqSlXduvBUieqk+a
BKx9kxNOWtep48m/1caJnsS6GTrtc18jB0CzGeGxeIL1cJftL9N0lUSjehbsYGmz
XseH1jRdJJGVGJw7
-----END CERTIFICATE-----`
const collaboratorsOtherTestKey = `-----BEGIN PRIVATE KEY-----
MIICeAIBADANBgkqhkiG9w0BAQEFAASCAmIwggJeAgEAAoGBANsstrCbuQ0iZegm
3XO/CJa7y/RgSeebI0D3KNGIann/+ctCPzjFzD02E3xDgdW6MxK26WWLvzuTyy1w
uiehaKovCCkJvvsr+6c1lB0b4IeweIL/71awxfV7YIFZkQmdhqtDSxoSOxlNo+a9
YMVZaUrMYyfU3/QLy1iuCpVhoYYDAgMBAAECgYEAxT/fhuAO0cBEYIMhyDqD20xW
CK/js1oOhzgo9zJDSVrTD1emmEyDPA9/x9Tlc1ko/824DZiQWWjwcQvDrUj5bIxo
662cdPyEJHAs4nDbZeN6EdMJIEwSmpQOXGwoaUQMKyXKhm43uDGJfGfhYwQ4iFQV
GDxY35H1cPDt7q/3/VkCQQD/WYW6o6uC5bamBW74dVVQ6h+UkVV4VpAg52c1ZLOr
kAEIWrxVpkFyyT5GO4pYxTG/MD25riN0lHm51OFmBWLXAkEA27ubSj6v4M+w2e2p
lh9n+SHP+wVgiMsa9xDJpHDbAHwMYSK9Wh1pVslSpVP2u/7ZNwRBBlfKBj/6O0wN
p9H8tQJAQabwvSXrqQIKzfDDsVnpj55CdF5RjVkkQXF9lbrIfynNOiqqFZNjbHHV
cxVH4r8ApVlv5VeiggzSpzbWpPZpjQJBALqjJYnwqQ85GixhVDRxRK017SR4MsC+
U48bsUp9mWdV9mXjThZm+PyAUDShluej1fiHInwywSSB3xfSx56OHCkCQQDsTAih
sh5/kd46V/jJbNBiporBuz/kVJTXrvFrZNnKgYv2BVC7jQfgOv6gkkQI4tHvuStE
DuvdvPhYKonxjjLv
-----END PRIVATE KEY-----`

// generated with: ALTNAME=DNS:localhost,DNS:localhost.localdomain,IP:127.1.1.1 openssl req -newkey rsa:1024 -nodes -keyout collaborators-test-key.pem -x509 -days 36500 -out collaborators-test-cert.pem -subj "/C=US/ST=UT/O=Salt Lake City/CN=localhost.localdomain" -config /etc/ssl/openssl.cnf
// openssl x509 -outform der -in collaborators-test-cert.pem -out collaborators-test-cert.der
// openssl rsa -inform pem -in collaborators-test-key.pem -outform der -out collaborators-test-key.der
// requires: /etc/ssl/openssl.cnf w/ section '[ v3_ca ]' having keyUsage = digitalSignature, keyEncipherment ; subjectAltName = $ENV::ALTNAME
var collaboratorsTestDERCert = []byte{48, 130, 2, 198, 48, 130, 2, 47, 160, 3, 2, 1, 2, 2, 20, 9, 207, 242, 195, 89,
	229, 0, 168, 213, 222, 35, 132, 132, 5, 122, 176, 150, 254, 57, 112, 48, 13, 6, 9, 42, 134, 72, 134, 247, 13, 1, 1,
	11, 5, 0, 48, 83, 49, 11, 48, 9, 6, 3, 85, 4, 6, 19, 2, 85, 83, 49, 11, 48, 9, 6, 3, 85, 4, 8, 12, 2, 85, 84, 49,
	23, 48, 21, 6, 3, 85, 4, 10, 12, 14, 83, 97, 108, 116, 32, 76, 97, 107, 101, 32, 67, 105, 116, 121, 49, 30, 48, 28,
	6, 3, 85, 4, 3, 12, 21, 108, 111, 99, 97, 108, 104, 111, 115, 116, 46, 108, 111, 99, 97, 108, 100, 111, 109, 97,
	105, 110, 48, 32, 23, 13, 50, 48, 48, 57, 49, 49, 49, 49, 52, 52, 48, 54, 90, 24, 15, 50, 49, 50, 48, 48, 56, 49,
	56, 49, 49, 52, 52, 48, 54, 90, 48, 83, 49, 11, 48, 9, 6, 3, 85, 4, 6, 19, 2, 85, 83, 49, 11, 48, 9, 6, 3, 85, 4, 8,
	12, 2, 85, 84, 49, 23, 48, 21, 6, 3, 85, 4, 10, 12, 14, 83, 97, 108, 116, 32, 76, 97, 107, 101, 32, 67, 105, 116,
	121, 49, 30, 48, 28, 6, 3, 85, 4, 3, 12, 21, 108, 111, 99, 97, 108, 104, 111, 115, 116, 46, 108, 111, 99, 97, 108,
	100, 111, 109, 97, 105, 110, 48, 129, 159, 48, 13, 6, 9, 42, 134, 72, 134, 247, 13, 1, 1, 1, 5, 0, 3, 129, 141, 0,
	48, 129, 137, 2, 129, 129, 0, 204, 96, 104, 30, 35, 153, 75, 1, 211, 181, 27, 191, 15, 186, 242, 53, 115, 203, 118,
	221, 79, 93, 151, 169, 78, 238, 247, 81, 197, 169, 110, 147, 193, 122, 24, 48, 114, 248, 181, 105, 238, 198, 48, 33,
	217, 118, 29, 165, 59, 7, 35, 168, 73, 211, 47, 164, 28, 240, 223, 246, 10, 110, 162, 133, 58, 239, 121, 158, 173,
	36, 176, 161, 253, 143, 21, 202, 147, 89, 79, 230, 41, 140, 232, 195, 119, 121, 55, 127, 72, 224, 180, 17, 133, 237,
	80, 174, 101, 20, 10, 146, 137, 238, 123, 77, 53, 204, 42, 129, 19, 171, 22, 226, 217, 69, 158, 236, 158, 158, 116,
	148, 139, 112, 71, 55, 159, 134, 79, 197, 2, 3, 1, 0, 1, 163, 129, 148, 48, 129, 145, 48, 29, 6, 3, 85, 29, 14, 4,
	22, 4, 20, 236, 99, 40, 168, 110, 227, 56, 147, 109, 148, 155, 233, 154, 182, 53, 166, 132, 71, 39, 199, 48, 31, 6,
	3, 85, 29, 35, 4, 24, 48, 22, 128, 20, 236, 99, 40, 168, 110, 227, 56, 147, 109, 148, 155, 233, 154, 182, 53, 166,
	132, 71, 39, 199, 48, 15, 6, 3, 85, 29, 19, 1, 1, 255, 4, 5, 48, 3, 1, 1, 255, 48, 11, 6, 3, 85, 29, 15, 4, 4, 3, 2,
	5, 160, 48, 49, 6, 3, 85, 29, 17, 4, 42, 48, 40, 130, 9, 108, 111, 99, 97, 108, 104, 111, 115, 116, 130, 21, 108,
	111, 99, 97, 108, 104, 111, 115, 116, 46, 108, 111, 99, 97, 108, 100, 111, 109, 97, 105, 110, 135, 4, 127, 1, 1, 1,
	48, 13, 6, 9, 42, 134, 72, 134, 247, 13, 1, 1, 11, 5, 0, 3, 129, 129, 0, 53, 165, 28, 62, 141, 213, 107, 24, 234, 9,
	187, 139, 107, 34, 187, 84, 113, 202, 205, 185, 213, 67, 35, 122, 144, 178, 25, 98, 199, 164, 179, 132, 239, 86,
	125, 207, 71, 156, 219, 84, 108, 101, 55, 24, 64, 194, 240, 62, 76, 31, 54, 205, 181, 151, 0, 12, 191, 171, 160, 47,
	142, 230, 38, 86, 233, 142, 157, 209, 134, 128, 8, 90, 7, 29, 152, 88, 52, 208, 174, 234, 141, 141, 61, 59, 43, 198,
	5, 226, 64, 250, 156, 252, 40, 110, 107, 166, 151, 177, 155, 208, 237, 185, 131, 241, 103, 162, 80, 244, 92, 94,
	152, 124, 0, 226, 152, 248, 217, 200, 118, 207, 37, 248, 34, 131, 205, 226, 27, 91}
var collaboratorsTestDERKey = []byte{48, 130, 2, 93, 2, 1, 0, 2, 129, 129, 0, 204, 96, 104, 30, 35, 153, 75, 1, 211,
	181, 27, 191, 15, 186, 242, 53, 115, 203, 118, 221, 79, 93, 151, 169, 78, 238, 247, 81, 197, 169, 110, 147, 193,
	122, 24, 48, 114, 248, 181, 105, 238, 198, 48, 33, 217, 118, 29, 165, 59, 7, 35, 168, 73, 211, 47, 164, 28, 240,
	223, 246, 10, 110, 162, 133, 58, 239, 121, 158, 173, 36, 176, 161, 253, 143, 21, 202, 147, 89, 79, 230, 41, 140,
	232, 195, 119, 121, 55, 127, 72, 224, 180, 17, 133, 237, 80, 174, 101, 20, 10, 146, 137, 238, 123, 77, 53, 204, 42,
	129, 19, 171, 22, 226, 217, 69, 158, 236, 158, 158, 116, 148, 139, 112, 71, 55, 159, 134, 79, 197, 2, 3, 1, 0, 1, 2,
	129, 129, 0, 171, 246, 134, 68, 173, 193, 98, 234, 83, 158, 244, 140, 171, 136, 170, 25, 173, 167, 202, 8, 230, 169,
	162, 93, 148, 11, 223, 207, 79, 24, 131, 241, 16, 255, 81, 113, 26, 156, 7, 208, 237, 86, 150, 211, 52, 109, 124,
	254, 53, 15, 137, 194, 36, 243, 236, 7, 19, 78, 221, 178, 225, 14, 59, 241, 138, 88, 22, 232, 48, 181, 48, 180, 142,
	97, 54, 186, 89, 188, 90, 242, 32, 120, 119, 97, 173, 140, 214, 225, 226, 204, 102, 199, 195, 214, 230, 111, 78,
	244, 251, 1, 162, 133, 28, 68, 205, 2, 69, 152, 39, 195, 20, 65, 48, 160, 205, 163, 112, 250, 198, 56, 25, 133, 21,
	8, 68, 26, 155, 53, 2, 65, 0, 244, 208, 93, 140, 53, 38, 79, 245, 13, 113, 112, 27, 50, 113, 161, 209, 96, 4, 104,
	243, 144, 27, 123, 155, 130, 150, 129, 235, 26, 124, 35, 7, 36, 189, 116, 20, 195, 21, 126, 80, 71, 151, 205, 211,
	85, 17, 19, 2, 75, 173, 197, 27, 232, 52, 37, 64, 240, 33, 236, 203, 170, 39, 131, 119, 2, 65, 0, 213, 183, 9, 194,
	203, 222, 19, 249, 71, 70, 82, 246, 124, 52, 65, 36, 174, 95, 57, 68, 224, 58, 111, 105, 47, 213, 205, 61, 143, 63,
	21, 178, 122, 202, 80, 206, 14, 127, 1, 14, 35, 139, 214, 94, 178, 9, 186, 25, 2, 194, 142, 68, 100, 16, 232, 167,
	121, 100, 86, 96, 98, 87, 253, 163, 2, 65, 0, 160, 84, 234, 63, 77, 251, 198, 119, 222, 19, 1, 241, 189, 234, 175,
	168, 185, 50, 138, 45, 161, 158, 110, 40, 157, 176, 198, 107, 92, 16, 26, 188, 173, 242, 41, 217, 3, 30, 203, 119,
	246, 59, 84, 64, 104, 192, 226, 235, 40, 247, 40, 85, 43, 145, 35, 40, 209, 91, 214, 130, 87, 240, 194, 231, 2, 64,
	100, 27, 224, 115, 162, 33, 190, 3, 119, 242, 166, 44, 21, 212, 56, 107, 161, 78, 179, 185, 226, 187, 28, 179, 14,
	24, 61, 146, 199, 134, 10, 120, 215, 113, 235, 214, 10, 14, 78, 5, 60, 123, 101, 136, 104, 39, 140, 71, 232, 246,
	15, 196, 83, 135, 100, 36, 7, 6, 12, 60, 11, 245, 33, 183, 2, 64, 44, 16, 157, 148, 38, 154, 244, 5, 132, 235, 38,
	203, 250, 52, 27, 13, 13, 166, 140, 71, 129, 49, 44, 34, 52, 241, 112, 248, 130, 151, 212, 204, 229, 205, 223, 140,
	247, 61, 123, 113, 170, 251, 228, 91, 97, 181, 250, 69, 155, 221, 116, 94, 121, 229, 241, 95, 182, 100, 59, 68, 205,
	37, 161, 163}

func setupTesting(listenerCert, listenerKey string, trustSystemCerts, isPEMEncoded bool, t *testing.T) (string, net.Listener) {
	err := os.Setenv("GODEBUG", "1")
	if err != nil {
		t.Error(err)
	}

	dir, err := ioutil.TempDir("", "config-collaborators-")
	if err != nil {
		t.Error(err)
	}

	keyPath := filepath.Join(dir, "collaborators-test-key.pem")
	if err := ioutil.WriteFile(keyPath, []byte(collaboratorsTestKey), 0660); err != nil {
		t.Error(err)
	}

	// mostly for building up a CA cert path
	certPath := filepath.Join(dir, "collaborators-test-cert.pem")
	if err := ioutil.WriteFile(certPath, []byte(collaboratorsTestCert), 0660); err != nil {
		t.Error(err)
	}

	derKeyPath := filepath.Join(dir, "collaborators-test-key.der")
	if err := ioutil.WriteFile(keyPath, collaboratorsTestDERKey, 0660); err != nil {
		t.Error(err)
	}

	// mostly for building up a CA cert path
	certDERPath := filepath.Join(dir, "collaborators-test-cert.der")
	if err := ioutil.WriteFile(certDERPath, collaboratorsTestDERCert, 0660); err != nil {
		t.Error(err)
	}

	// add the public key path, since we are only interested in the file names in the test for
	// the key file fetcher, no real data are in the public key files.
	pubKeyPath := filepath.Join(dir, "mtn-PublicKey.pem")
	if err := ioutil.WriteFile(pubKeyPath, []byte("hello"), 0660); err != nil {
		t.Error(err)
	}

	config := &HorizonConfig{
		Edge: Config{
			TrustSystemCACerts: trustSystemCerts,
			CACertsPath:        dir,
			PublicKeyPath:      pubKeyPath,
		},
	}

	configBytes, err := json.Marshal(config)
	if err != nil {
		t.Error(err)
	}

	configPath := filepath.Join(dir, "config.json")
	if err := ioutil.WriteFile(configPath, configBytes, 0660); err != nil {
		t.Error(err)
	}

	// write listener key and cert
	listenerKeyName := "listener-key.pem"
	listenerCertName := "listener-cert.pem"
	if !isPEMEncoded {
		listenerKeyName = "listener-key.der"
		listenerCertName = "listener-cert.der"
	}
	listenerKeyPath := filepath.Join(dir, listenerKeyName)
	if err := ioutil.WriteFile(listenerKeyPath, []byte(listenerKey), 0660); err != nil {
		t.Error(err)
	}

	listenerCertPath := filepath.Join(dir, listenerCertName)
	if err := ioutil.WriteFile(listenerCertPath, []byte(listenerCert), 0660); err != nil {
		t.Error(err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/boosh", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("yup"))
	})

	listener, err := net.Listen("tcp", fmt.Sprintf("%s:0", listenOn))
	if err != nil {
		t.Error(err)
	}

	// setup https server
	go func() {
		err = http.ServeTLS(listener, mux, listenerCertPath, listenerKeyPath)
		if err != nil {
			t.Error(err)
		}
	}()

	return dir, listener
}

func cleanup(dir string, t *testing.T) {

	if err := os.RemoveAll(dir); err != nil {
		t.Error(err)
	}
}

func Test_HTTPClientFactory_Suite(t *testing.T) {
	timeoutS := uint(2)

	setupForTest := func(listenerCert, listenerKey string, trustSystemCerts, isPEMEncoded bool) (*HorizonConfig, string, string) {
		dir, listener := setupTesting(listenerCert, listenerKey, trustSystemCerts, isPEMEncoded, t)

		t.Logf("listening on %s", listener.Addr().String())

		cfg, err := Read(filepath.Join(dir, "config.json"))
		if err != nil {
			t.Error(nil)
		}
		return cfg, strings.Split(listener.Addr().String(), ":")[1], dir
	}

	cfg, port, dir := setupForTest(collaboratorsTestCert, collaboratorsTestKey, false, true)
	t.Run("HTTP client rejects trusted cert for wrong domain", func(t *testing.T) {

		client := cfg.Collaborators.HTTPClientFactory.NewHTTPClient(&timeoutS)
		// this'll fail b/c we're making a request to 127.0.1.1 but that isn't the CN or subjectAltName IP in the cert
		_, err := client.Get(fmt.Sprintf("https://%s:%s/boosh", "127.0.1.1", port))
		if err == nil {
			t.Error("Expected TLS error for sending request to wrong domain")
		}
	})

	t.Run("HTTP client accepts trusted cert for right domain", func(t *testing.T) {

		client := cfg.Collaborators.HTTPClientFactory.NewHTTPClient(&timeoutS)
		// all of these should pass b/c they are the subjectAltNames of the cert (either names or IPs) note that Golang doesn't verify the CA of the cert if it's localhost or an IP
		for _, dom := range []string{listenOn, "localhost"} {

			resp, err := client.Get(fmt.Sprintf("https://%s:%s/boosh", dom, port))

			if err != nil {
				t.Error("Unxpected error sending request to trusted domain", err)
			}

			if resp != nil {
				if resp.StatusCode != 200 {
					t.Errorf("Unexpected error from HTTP request (wanted 200). HTTP response status code: %v", resp.StatusCode)
				}

				content, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					t.Error("Unexpected error reading response from HTTP server", err)
				}

				if string(content) != "yup" {
					t.Error("Unexpected returned content from test")
				}
			}
		}

		cleanup(dir, t)
	})
	t.Run("HTTP client rejects untrusted cert", func(t *testing.T) {
		// need a new config and setup
		cfg, port, dir := setupForTest(collaboratorsOtherTestCert, collaboratorsOtherTestKey, false, true)

		client := cfg.Collaborators.HTTPClientFactory.NewHTTPClient(&timeoutS)
		// this should fail b/c even though we're sending a request to a trusted domain, the CA trust doesn't contain the cert
		_, err := client.Get(fmt.Sprintf("https://%s:%s/boosh", listenOn, port))
		if err == nil {
			t.Error("Expected TLS error for sending request to untrusted domain")
		}

		cleanup(dir, t)
	})

	t.Run("HTTP client trusts system certs", func(t *testing.T) {
		// important that the cert and key match for setup to succeed even though that's not what we're testing
		setupForTest(collaboratorsTestCert, collaboratorsTestKey, true, true)
		cleanup(dir, t)

		// if we got this far we're ok (an error gets raised during setup if the system ca certs couldn't be loaded)
	})

	t.Run("HTTP client accepts trusted DER-encoded cert for right domain", func(t *testing.T) {

		cfg, port, dir := setupForTest(string(collaboratorsTestDERCert), string(collaboratorsTestDERKey), false, false)
		client := cfg.Collaborators.HTTPClientFactory.NewHTTPClient(&timeoutS)
		// this'll should pass b/c we're making a request to 127.1.1.1 but that's the CN or subjectAltName IP in the cert

		resp, err := client.Get(fmt.Sprintf("https://%s:%s/boosh", "127.1.1.1", port))

		if err != nil {
			t.Error("Unxpected error sending request to trusted domain", err)
		}

		if resp != nil {
			if resp.StatusCode != 200 {
				t.Errorf("Unexpected error from HTTP request (wanted 200). HTTP response status code: %v", resp.StatusCode)
			}

			content, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Error("Unexpected error reading response from HTTP server", err)
			}

			if string(content) != "yup" {
				t.Error("Unexpected returned content from test")
			}
		}

		cleanup(dir, t)
	})

	t.Run("HTTP client rejects trusted DER-encoded cert for wrong domain", func(t *testing.T) {

		cfg, port, dir := setupForTest(string(collaboratorsTestDERCert), string(collaboratorsTestDERKey), false, false)
		client := cfg.Collaborators.HTTPClientFactory.NewHTTPClient(&timeoutS)
		// this'll fail b/c we're making a request to 127.0.1.1 but that isn't the CN or subjectAltName IP in the cert
		_, err := client.Get(fmt.Sprintf("https://%s:%s/boosh", "127.0.1.1", port))
		if err == nil {
			t.Error("Expected TLS error for sending request to wrong domain")
		}
	})
}

func Test_KeyFileNamesFetcher_Suite(t *testing.T) {
	setupForTest := func(listenerCert, listenerKey string, trustSystemCerts bool) (string, *HorizonConfig) {
		dir, _ := setupTesting(listenerCert, listenerKey, trustSystemCerts, t)

		cfg, err := Read(filepath.Join(dir, "config.json"))
		if err != nil {
			t.Error(nil)
		}

		err = os.Setenv("HZN_VAR_BASE", dir)
		if err != nil {
			t.Error(err)
		}

		cfg.Edge.PublicKeyPath = filepath.Join(dir, "/trusted/keyfile1.pem")
		return dir, cfg
	}

	dir, cfg := setupForTest(collaboratorsTestCert, collaboratorsTestKey, false, true)

	t.Run("Test zero *.pem files under user key path", func(t *testing.T) {

		fnames, err := cfg.Collaborators.KeyFileNamesFetcher.GetKeyFileNames(cfg.Edge.PublicKeyPath, cfg.UserPublicKeyPath())
		if err != nil {
			t.Error("Got error but should not.")
		}
		if len(fnames) != 0 {
			t.Errorf("Number of files should be 0 but got %v.", len(fnames))
		}
	})

	t.Run("Test filter out non .pem files", func(t *testing.T) {
		userKeyPath := cfg.UserPublicKeyPath()
		if err := os.Mkdir(userKeyPath, 0777); err != nil {
			t.Error(err)
		}

		if err := os.Mkdir(dir+"/trusted", 0777); err != nil {
			t.Error(err)
		}

		nonpemfile1 := filepath.Join(dir, "/trusted/non_pem_file1")
		if err := ioutil.WriteFile(nonpemfile1, []byte("hello from non pem file 1"), 0660); err != nil {
			t.Error(err)
		}

		nonpemfile2 := filepath.Join(userKeyPath, "/non_pem_file2")
		if err := ioutil.WriteFile(nonpemfile2, []byte("hello from non pem file 2"), 0660); err != nil {
			t.Error(err)
		}

		fnames, err := cfg.Collaborators.KeyFileNamesFetcher.GetKeyFileNames(cfg.Edge.PublicKeyPath, userKeyPath)
		if err != nil {
			t.Error("Got error but should not.")
		}
		if len(fnames) != 0 {
			t.Errorf("Number of files should be 0 but got %v.", len(fnames))
		}
	})

	t.Run("Test getting pem files", func(t *testing.T) {
		userKeyPath := cfg.UserPublicKeyPath()

		pemfile1 := filepath.Join(dir, "/trusted/realfile1.pem")
		if err := ioutil.WriteFile(pemfile1, []byte("hello from pem file 1"), 0660); err != nil {
			t.Error(err)
		}
		pemfile2 := filepath.Join(dir, "/trusted/realfile2.pem")
		if err := ioutil.WriteFile(pemfile2, []byte("hello from pem file 2"), 0660); err != nil {
			t.Error(err)
		}
		pemfile3 := filepath.Join(userKeyPath, "realfile3.pem")
		if err := ioutil.WriteFile(pemfile3, []byte("hello from pem file 3"), 0660); err != nil {
			t.Error(err)
		}
		pemfile4 := filepath.Join(userKeyPath, "realfile4.pem")
		if err := ioutil.WriteFile(pemfile4, []byte("hello from pem file 4"), 0660); err != nil {
			t.Error(err)
		}

		fnames, err := cfg.Collaborators.KeyFileNamesFetcher.GetKeyFileNames(cfg.Edge.PublicKeyPath, userKeyPath)
		if err != nil {
			t.Error("Got error but should not.")
		}
		if len(fnames) != 4 {
			t.Errorf("Number of files should be 4 but got %v.", len(fnames))
		} else {
			for _, fn := range fnames {
				if !strings.Contains(fn, "realfile") {
					t.Errorf("File %v should not be returned as a pem file.", fn)
				}
			}
		}
	})

	t.Run("Cleaning up", func(t *testing.T) {
		cleanup(dir, t)
	})
}
