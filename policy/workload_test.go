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

//     tempKeyFile := "/tmp/mtn-publicKey.pem"
//     str := `{"services":{"culex":{"image":"summit.hovitos.engineering/x86/culex:latest","environment":["MTN_MQTT_TOKEN=ZZbrT4ON5rYzoBi7H1VK3Ak9n0Fwjcod"]},"gps":{"image":"summit.hovitos.engineering/x86/gps:v1.2","privileged":true,"environment":["MTN_MQTT_TOKEN=ZZbrT4ON5rYzoBi7H1VK3Ak9n0Fwjcod","SLEEP_INTERVAL=20"],"devices":["/dev/bus/usb/001/001:/dev/bus/usb/001/001"]},"location":{"image":"summit.hovitos.engineering/x86/location:v1.3","environment":["DEPL_ENV=staging"]}}}`
//     //str := `{\"services\":{\"geth\":{\"image\":\"summit.hovitos.engineering/private-eth:v1.5.7\",\"command\":[\"start.sh\"]}}}`
//     sig := `gB2d3aN9nvE926H0muL02k4JGUzvYF4oiNFelxMhLxyTlb37yyLyqcHIVV/RNbJM4UOPiyXAyha61MXL0O7zluyN6uJQ9In0puHkcPRX9pC+iDfi2v6ygYl/nCR7OYAwX/8P+NoV3cuQJ1OTAPkTJJCaITvk0S1JsF4QOk+EoNFZrcvZ0JYKc7rzulBEXacW9bydzcFPJuQGhk32SwSzMJ0Rf/XCydeI5VsIGNwCCgTNxZn3cuslUS2JvY0Th3910p662OkEmTKY412ozOLyRM99hWPSxaO5OvGzWYZB7vfQ6N0eGbGVQv6GnDSnnJ+plzXR/rFolgJ92ir2fC1biL4AgnTjm0Ckn5EKj6f63kpgzdFtTw/yMt+5VczQNET2541iV3xyRc8nCDR/HqlfVaY9Q1R+W9R5JZTyVULkNG0yLeP4Hs+3O32PvqtCJovzlppKYX9/Orq3pUTOjRQRZ0AcenPpgmQUxuJEAs0UmhK4oKe5+a9CJm/YigwzyLlk8whxZLIZgvTPAWjW29n41zj8JVAwWIb7UF3bpVxh63p12sUUD1I7cFWCQA6stTpL6yF5+RQHom5frWvhTaNnPGfcFP5EwFVYtH/lFNoICWOyZy98pAbMzK46c8FnFzVB/TM1dCMMcTSjfJILhi7SCURh42Rp6hj3EPf/lO444hI=`

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

func Test_nexthighestpriority_workload1(t *testing.T) {

    wl1 := `{"priority":{"priority_value":3,"retries":2,"retry_durations":5},"deployment":"3","deployment_signature":"1","deployment_user_info":"d","torrent":{"url":"torrURL","images":[{"file":"filename","signature":"abcdefg"}]},"workload_password":"mysecret"}`
    wl2 := `{"priority":{"priority_value":2,"retries":2,"retry_durations":5},"deployment":"2","deployment_signature":"1","deployment_user_info":"d","torrent":{"url":"torrURL","images":[{"file":"filename","signature":"abcdefg"}]},"workload_password":"mysecret"}`
    wl3 := `{"priority":{"priority_value":1,"retries":2,"retry_durations":5},"deployment":"1","deployment_signature":"1","deployment_user_info":"d","torrent":{"url":"torrURL","images":[{"file":"filename","signature":"abcdefg"}]},"workload_password":"mysecret"}`

    if wla := create_Workload(wl1, t); wla == nil {
        t.Errorf("Error unmarshalling Workload json string: %v\n", wl1)
    } else if wlb := create_Workload(wl2, t); wlb == nil {
        t.Errorf("Error unmarshalling Workload json string: %v\n", wl2)
    } else if wlc := create_Workload(wl3, t); wlc == nil {
        t.Errorf("Error unmarshalling Workload json string: %v\n", wl3)
    } else {

        pf_created := Policy_Factory("test creation")
        pf_created.Workloads = append(pf_created.Workloads, *wlb)

        // Try with 1 workload
        if wl := pf_created.NextHighestPriorityWorkload(0, 0, 0); wl == nil {
            t.Errorf("Error finding highest priority workload.\n")
        } else if wl.Deployment != "2" {
            t.Errorf("Returned workload is not the next highest priority, returned %v", wl)
        }

        // Now add multiple workloads
        pf_created.Workloads = append(pf_created.Workloads, *wla)
        pf_created.Workloads = append(pf_created.Workloads, *wlc)

        // Find priority 1 workload
        if wl := pf_created.NextHighestPriorityWorkload(0, 0, 0); wl == nil {
            t.Errorf("Error finding highest priority workload.\n")
        } else if wl.Deployment != "1" {
            t.Errorf("Returned workload is not the next highest priority, returned %v", wl)
        }

        // Find priority 1 again
        if wl := pf_created.NextHighestPriorityWorkload(1, 1, 1000); wl == nil {
            t.Errorf("Error finding highest priority workload.\n")
        } else if wl.Deployment != "1" {
            t.Errorf("Returned workload is not the next highest priority, returned %v", wl)
        }

        // Move to priority 2 due to retries
        now := uint64(time.Now().Unix())
        if wl := pf_created.NextHighestPriorityWorkload(1, 2, now-1); wl == nil {
            t.Errorf("Error finding highest priority workload.\n")
        } else if wl.Deployment != "2" {
            t.Errorf("Returned workload is not the next highest priority, returned %v", wl)
        }

        // Stay with priority 2
        if wl := pf_created.NextHighestPriorityWorkload(2, 1, now-1); wl == nil {
            t.Errorf("Error finding highest priority workload.\n")
        } else if wl.Deployment != "2" {
            t.Errorf("Returned workload is not the next highest priority, returned %v", wl)
        }

        // Stay with priority 2 - not enough retries in the alloted retry duration
        if wl := pf_created.NextHighestPriorityWorkload(2, 2, now-6); wl == nil {
            t.Errorf("Error finding highest priority workload.\n")
        } else if wl.Deployment != "2" {
            t.Errorf("Returned workload is not the next highest priority, returned %v", wl)
        }

        // Move to priority 3
        if wl := pf_created.NextHighestPriorityWorkload(2, 2, now-1); wl == nil {
            t.Errorf("Error finding highest priority workload.\n")
        } else if wl.Deployment != "3" {
            t.Errorf("Returned workload is not the next highest priority, returned %v", wl)
        }

        // Stay with priority 3 - not enough retries yet
        if wl := pf_created.NextHighestPriorityWorkload(3, 0, now-1); wl == nil {
            t.Errorf("Error finding highest priority workload.\n")
        } else if wl.Deployment != "3" {
            t.Errorf("Returned workload is not the next highest priority, returned %v", wl)
        }

        // Stay with priority 3 - not enough retries yet
        if wl := pf_created.NextHighestPriorityWorkload(3, 1, now-1); wl == nil {
            t.Errorf("Error finding highest priority workload.\n")
        } else if wl.Deployment != "3" {
            t.Errorf("Returned workload is not the next highest priority, returned %v", wl)
        }

        // Stay with priority 3 - retries exceeded but not in the time limit
        if wl := pf_created.NextHighestPriorityWorkload(3, 2, now-10); wl == nil {
            t.Errorf("Error finding highest priority workload.\n")
        } else if wl.Deployment != "3" {
            t.Errorf("Returned workload is not the next highest priority, returned %v", wl)
        }

        // Stay with priority 3 - retries exceeded but lowest priority workload
        if wl := pf_created.NextHighestPriorityWorkload(3, 2, now-1); wl == nil {
            t.Errorf("Error finding highest priority workload.\n")
        } else if wl.Deployment != "3" {
            t.Errorf("Returned workload is not the next highest priority, returned %v", wl)
        }

        // Stay with priority 3 - retries exceeded but lowest priority workload
        if wl := pf_created.NextHighestPriorityWorkload(3, 3, now-1); wl == nil {
            t.Errorf("Error finding highest priority workload.\n")
        } else if wl.Deployment != "3" {
            t.Errorf("Returned workload is not the next highest priority, returned %v", wl)
        }

    }

}

func Test_nexthighestpriority_workload2(t *testing.T) {

    wl1 := `{"priority":{"priority_value":3,"retries":2,"retry_durations":5},"deployment":"3","deployment_signature":"1","deployment_user_info":"d","torrent":{"url":"torrURL","images":[{"file":"filename","signature":"abcdefg"}]},"workload_password":"mysecret"}`
    wl2 := `{"priority":{"priority_value":2,"retries":0,"retry_durations":5},"deployment":"2","deployment_signature":"1","deployment_user_info":"d","torrent":{"url":"torrURL","images":[{"file":"filename","signature":"abcdefg"}]},"workload_password":"mysecret"}`
    wl3 := `{"priority":{"priority_value":1,"retries":2,"retry_durations":5},"deployment":"1","deployment_signature":"1","deployment_user_info":"d","torrent":{"url":"torrURL","images":[{"file":"filename","signature":"abcdefg"}]},"workload_password":"mysecret"}`

    if wla := create_Workload(wl1, t); wla == nil {
        t.Errorf("Error unmarshalling Workload json string: %v\n", wl1)
    } else if wlb := create_Workload(wl2, t); wlb == nil {
        t.Errorf("Error unmarshalling Workload json string: %v\n", wl2)
    } else if wlc := create_Workload(wl3, t); wlc == nil {
        t.Errorf("Error unmarshalling Workload json string: %v\n", wl3)
    } else {

        pf_created := Policy_Factory("test creation")
        pf_created.Workloads = append(pf_created.Workloads, *wlb)
        pf_created.Workloads = append(pf_created.Workloads, *wla)
        pf_created.Workloads = append(pf_created.Workloads, *wlc)

        // Find priority 3 workload
        now := uint64(time.Now().Unix())
        if wl := pf_created.NextHighestPriorityWorkload(2, 1, now-1); wl == nil {
            t.Errorf("Error finding highest priority workload.\n")
        } else if wl.Deployment != "3" {
            t.Errorf("Returned workload is not the next highest priority, returned %v", wl)
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
