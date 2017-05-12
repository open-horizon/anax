package torrent

import (
    "crypto"
    "crypto/rand"
    "crypto/rsa"
    "crypto/sha256"
    "crypto/x509"
    "encoding/base64"
    "encoding/pem"
    "github.com/open-horizon/anax/config"
    "io"
    "io/ioutil"
    "os"
    "testing"
)

func Test_verify(t *testing.T) {

    filePath := "/tmp/torrentsigtestfile.txt"

    if _, err := os.Stat(filePath); !os.IsNotExist(err) {
        os.Remove(filePath)
    }

    if _, err := os.Stat("/tmp" + config.USERKEYDIR); !os.IsNotExist(err) {
        os.RemoveAll("/tmp" + config.USERKEYDIR)
    }

    bytes := []byte(`some text in this file to be signed by a torrent test case.`)
    if err := ioutil.WriteFile(filePath, bytes, 0644); err != nil {
        t.Errorf("Could not create sig test file %v, error %v\n", filePath, err)
    } 

    publicFilePath := "/tmp/somekey.pem"
    os.MkdirAll("/tmp" + config.USERKEYDIR, 0777)

    tempKeyFile := "/tmp" + config.USERKEYDIR + "/tempverifysigtestkey.pem"
    if _, err := os.Stat(tempKeyFile); !os.IsNotExist(err) {
        os.Remove(tempKeyFile)
    }

    // Generate RSA key pair for testing and save in a temporary file
    if privateKey, err := rsa.GenerateKey(rand.Reader, 2048); err != nil {
        t.Errorf("Could not generate private key, error %v\n", err)
    } else if pubFile, err := os.Create(tempKeyFile); err != nil {
        t.Errorf("Could not create public key file %v, error %v\n", tempKeyFile, err)
    } else if err := pubFile.Chmod(0600); err != nil {
        t.Errorf("Could not chmod public key file %v, error %v\n", tempKeyFile, err)
    } else {
        publicKey := &privateKey.PublicKey

        if pubKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey); err != nil {
            t.Errorf("Could not marshal public key, error %v\n", err)
        } else {
            pubEnc := &pem.Block{
                Type:    "PUBLIC KEY",
                Headers: nil,
                Bytes:   pubKeyBytes}
            if err := pem.Encode(pubFile, pubEnc); err != nil {
                t.Errorf("Could not encode public key to file, error %v\n", err)
            } else {
                pubFile.Close()
            }
        }

        if file, err := os.Open(filePath); err != nil {
            t.Errorf("Unable to open file %v, error: %v", filePath, err)
        } else {

            hasher := sha256.New()
            if _, err := io.Copy(hasher, file); err != nil {
                t.Errorf("Unable to copy file into hash function, error: %v", err)
            } else {
                file.Close()

                if signature, err := rsa.SignPSS(rand.Reader, privateKey, crypto.SHA256, hasher.Sum(nil), nil); err != nil {
                    t.Errorf("Error signing the message, error: %v", err)
                } else if len(signature) == 0 {
                    t.Errorf("Error signing the message, signature is empty byte array")
                } else {
                    encoded := base64.StdEncoding.EncodeToString(signature)

                    if file, err := os.Open(filePath); err != nil {
                        t.Errorf("Unable to open file %v, error: %v", filePath, err)
                    } else if verified, err := verify(publicFilePath, encoded, file); err != nil {
                        t.Errorf("No verification: %v", err)
                    } else if !verified {
                        t.Errorf("No verification")   
                    }
                }
            }
        }
    }

}
