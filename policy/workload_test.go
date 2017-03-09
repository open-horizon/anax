package policy

import (
    "bytes"
    "crypto"
    crand "crypto/rand"
    "crypto/rsa"
    "crypto/sha256"
    "crypto/x509"
    "encoding/base64"
    "encoding/json"
    "encoding/pem"
    "golang.org/x/crypto/bcrypt"
    "math/rand"
    "os"
    "testing"
    "time"
)

func Test_workload_pw_hash(t *testing.T) {

    random := rand.New(rand.NewSource(int64(time.Now().Nanosecond())))

    rpw := func (random *rand.Rand) string {
        var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

        b := make([]rune, 20)
        for i := range b {
            b[i] = letters[random.Intn(len(letters))]
        }
        return string(b)
    }

    rHash := func(random *rand.Rand) []byte {
        b := make([]byte, 32, 32)
        for i := range b {
            b[i] = byte(random.Intn(256))
        }
        return b
    }

    for i := 0; i < 10; i++ {
        password := rpw(random)
        // Perform the hash twice
        if hash1, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost); err != nil {
            t.Error(err)
        } else if hash2, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost); err != nil {
            t.Error(err)
        } else if bytes.Compare(hash1, hash2) == 0 {
            t.Errorf("2 calls to generate hash for %v result in equivalent hashes %v, but should not", password, hash1)
        } else if err := bcrypt.CompareHashAndPassword(hash1, []byte(password)); err != nil {
            t.Error(err)
        } else if err := bcrypt.CompareHashAndPassword(hash2, []byte(password)); err != nil {
            t.Error(err)
        }
    }

    password := rpw(random)
    rh := rHash(random)
    if err := bcrypt.CompareHashAndPassword(rh, []byte(password)); err == nil {
        t.Errorf("Random hash %v should not verify %v", rh, password)
    }

}

// This function is a quick debug tool for suspected workload password issues. Uncomment it, fill in the
// agbot password, a valid agreement id and the resulting hash. This test will verify that the hash came from
// the password and agreement id. NEVER NEVER check in this code with a valid password filled in. Yes, this is
// quick and dirty.
//
// func Test_workload_pw_hash2(t *testing.T) {

//     password := ""
//     agid := "250171459f48f81adc2d0ad92df0d37fb57f4d864abe23f5a98bef725daef50a"
//     hash := "$2a$10$kThocDiwbcdWWTxyiRd8A.YvmpWEncI9ip6vePTPTCFr2qPdw6pQm"

//     // Perform the hash twice
//     if hash1, err := bcrypt.GenerateFromPassword([]byte(password+agid), bcrypt.DefaultCost); err != nil {
//         t.Error(err)
//     } else if err := bcrypt.CompareHashAndPassword(hash1, []byte(password+agid)); err != nil {
//         t.Error(err)
//     } else if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password+agid)); err != nil {
//         t.Error(err)
//     }

// }

func Test_workload_pw_hash_error(t *testing.T) {

    pw := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

    if hash1, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost); err != nil {
        t.Error(err)
    } else if len(hash1) == 0 {
        t.Errorf("bcrypt returned zero length hash\n")
    } else if err := bcrypt.CompareHashAndPassword(hash1, []byte(pw[:71])); err == nil {
        t.Errorf("Password was shortened and should not validate, but it did.")
    }

}


func Test_workload_obscure(t *testing.T) {

    wl1 := `{"deployment":"deploymentabcdefg","deployment_signature":"123456","deployment_user_info":"duiabcdefg","torrent":{"url":"torrURL","images":[{"file":"filename","signature":"abcdefg"}]},"workload_password":"mysecret"}`
    wl2 := `{"deployment":"deploymentabcdefg","deployment_signature":"123456","deployment_user_info":"duiabcdefg","torrent":{"url":"torrURL","images":[{"file":"filename","signature":"abcdefg"}]},"workload_password":"mysecret"}`
    if wla := create_Workload(wl1, t); wla != nil {
        wpb4 := wla.WorkloadPassword
        wla.Obscure("01020304","")
        if wla.WorkloadPassword == wpb4 {
            t.Errorf("Workload section %v was not obscured correctly\n", wla)
        } else if wlb := create_Workload(wl2, t); wlb != nil {
            if wla.IsSame(*wlb) {
                t.Errorf("Workload section %v is the same as %v\n", wla, wlb)
            }
        } else {
            wla.Obscure("","05060708")
            if wla.WorkloadPassword == wpb4 {
                t.Errorf("Workload section %v was not obscured correctly\n", wla)
            } else if wlb := create_Workload(wl2, t); wlb != nil {
                if wla.IsSame(*wlb) {
                    t.Errorf("Workload section %v is the same as %v\n", wla, wlb)
                }
            }
        }
    }

}

func Test_workload_signature(t *testing.T) {

    tempKeyFile := "/tmp/temppolicytestkey.pem"
    if _, err := os.Stat(tempKeyFile); !os.IsNotExist(err) {
        os.Remove(tempKeyFile)
    }

    // Generate RSA key pair for testing and save in a temporary file
    if privateKey, err := rsa.GenerateKey(crand.Reader, 2048); err != nil {
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

            // Sign a simple deployment string and then verify the signature with the workload method
            hasher := sha256.New()
            if _, err = hasher.Write([]byte("teststring")); err != nil {
                t.Errorf("Could not hash the deployment string, error %v\n", err)
            } else if sig, err := rsa.SignPSS(crand.Reader, privateKey, crypto.SHA256, hasher.Sum(nil), nil); err != nil {
                t.Errorf("Could not sign test string, error %v\n", err)
            } else {
                strSig := base64.StdEncoding.EncodeToString(sig)
                wl1 := `{"deployment":"teststring","deployment_signature":"","deployment_user_info":"","torrent":{"url":"torrURL","images":[{"file":"filename","signature":"abcdefg"}]},"workload_password":"mysecret"}`
                if wla := create_Workload(wl1, t); wla != nil {
                    wla.DeploymentSignature = strSig
                    if err := wla.HasValidSignature("/tmp/temppolicytestkey.pem"); err != nil {
                        t.Errorf("Could not verify signed deployment, error %v\n", err)
                    }
                }
            }

        }
    }
}

// func Test_debug_signature(t *testing.T) {

//     tempKeyFile := "/tmp/e2edev/.colonus/mtn-publicKey.pem"
//     str := `{"services":{"geth":{"image":"summit.hovitos.engineering/private-eth:v1.5.7","command":["start.sh"]}}}`
//     //str := `{\"services\":{\"geth\":{\"image\":\"summit.hovitos.engineering/private-eth:v1.5.7\",\"command\":[\"start.sh\"]}}}`
//     sig := `DMFgRyyH8OVcoN97FN//8aPpJSy/vNxIxvzrF2N6PmutQOHe2QQaAVfTNJ29jamK7Dl4QuTf/Qk59iK8dz/MgvPSYF3jUEULItbj3sA9DciV1hqE3y0Rn0HP2VCv6qYO+g9A7Pjv73Tpxu+MaYoG4mVr6GovOnIq9udRTCjPUv/4a0gjaHZe4ePoV/BR5n9jeMNGiH8VVD4jv2uw0nOiVvo0X5NSxOzn5NlqvntQWdDLkWduDa4alFXuhIsVUnnTPWMvmZNOU8hiUbBzAnuG9YGX4XO4NkVdSR7jWC0vi2PyvHmb1GSzph1WvaSvcNPePPQuvmrxjSgdSRzDYfOxwl5EPbE+18nJuUou7HWOm1LcyHup8+8e0BQblFh0OUlXLDKDdQwWiLK4DUUdQbzUzX9UFJPiOKS+BvBorRIV3v2s99yR1bH9/ScCMnzgXzmr8lW03QSCwCIyiPc5AYelFWaBzNBERhy7b6r2fGkzOA+NcmoB25dMpOJAellNS4dSarSA82+nSR4gFeZ3znQhk2IH8VYs5kbdDSUpyk2cvYee4K7tx5CUvNyOCvErlayz9fxD2oTznlG2clqtu5GUTJcUKSDEccMJGZHFwPyXHc0BnMtKFFkv362Ybord1JQVFQg6kV9tDaSGbzuEhwls5P26zy+eeW2pSHxYAogaWag=`

//     wl1 := `{"deployment":"","deployment_signature":"","deployment_user_info":"","torrent":{"url":"torrURL","images":[{"file":"filename","signature":"abcdefg"}]},"workload_password":"mysecret"}`
//     if wla := create_Workload(wl1, t); wla != nil {
//         wla.DeploymentSignature = sig
//         wla.Deployment = str
//         if err := wla.HasValidSignature(tempKeyFile); err != nil {
//             t.Errorf("Could not verify signed deployment, error %v\n", err)
//         }
//     }
// }

func Test_workload_signature_invalid(t *testing.T) {

    tempKeyFile := "/tmp/temppolicytestkey.pem"
    if _, err := os.Stat(tempKeyFile); !os.IsNotExist(err) {
        os.Remove(tempKeyFile)
    }

    // Generate RSA key pair for testing and save in a temporary file
    if privateKey, err := rsa.GenerateKey(crand.Reader, 2048); err != nil {
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

            // Sign a simple deployment string and then verify the signature with the workload method
            hasher := sha256.New()
            if _, err = hasher.Write([]byte("teststringX")); err != nil {
                t.Errorf("Could not hash the deployment string, error %v\n", err)
            } else if sig, err := rsa.SignPSS(crand.Reader, privateKey, crypto.SHA256, hasher.Sum(nil), nil); err != nil {
                t.Errorf("Could not sign test string, error %v\n", err)
            } else {
                strSig := base64.StdEncoding.EncodeToString(sig)
                wl1 := `{"deployment":"teststring","deployment_signature":"","deployment_user_info":"","torrent":{"url":"torrURL","images":[{"file":"filename","signature":"abcdefg"}]},"workload_password":"mysecret"}`
                if wla := create_Workload(wl1, t); wla != nil {
                    wla.DeploymentSignature = strSig
                    if err := wla.HasValidSignature("/tmp/temppolicytestkey.pem"); err == nil {
                        t.Errorf("Should not have been able to verify signed deployment, error %v\n", err)
                    }
                }
            }

        }
    }
}

// Create a Workload section from a JSON serialization. The JSON serialization
// does not have to be a valid Workload serialization, just has to be a valid
// JSON serialization.
func create_Workload(jsonString string, t *testing.T) *Workload {
    wl := new(Workload)

    if err := json.Unmarshal([]byte(jsonString), &wl); err != nil {
        t.Errorf("Error unmarshalling Workload json string: %v error:%v\n", jsonString, err)
        return nil
    } else {
        return wl
    }
}
