package cutil

import (
	"crypto/rand"
	"encoding/base64"
	"runtime"
)

func FirstN(n int, ss []string) []string {
	out := make([]string, 0)

	for ix := 0; ix < n-1; ix++ {
		if len(ss) == ix {
			break
		}

		out = append(out, ss[ix])
	}

	return out
}

func SecureRandomString() (string, error) {
	bytes := make([]byte, 64)

	if _, err := rand.Read(bytes); err != nil {
		return "", err
	} else {
		return base64.URLEncoding.EncodeToString(bytes), nil
	}
}

func ArchString() string {
	var archString string
	if runtime.GOARCH == "arm" {
		archString = "armhf"
	} else {
		archString = runtime.GOARCH
	}

	return archString
}
