package policy

import (
    "bytes"
    "encoding/json"
    "golang.org/x/crypto/bcrypt"
    "math/rand"
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
