// +build unit

package torrent

import (
	"bufio"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"syscall"
	"testing"
)

func genTestFile(file *os.File) ([]string, map[string]string, map[string]string) {
	hashes := make(map[string]string, 0)

	ioutil.WriteFile(file.Name(), []byte("testotestotesto"), 0644)

	cmd := exec.Command("sha1sum", file.Name())
	if data, err := cmd.StdoutPipe(); err != nil {
		panic(err)
	} else if err := cmd.Start(); err != nil {
		panic(err)
	} else {
		reader := bufio.NewReader(data)
		if line, _, err := reader.ReadLine(); err != nil {
			panic(err)
		} else {
			_, filename := path.Split(file.Name())
			hashes[filename] = strings.Split(string(line[:]), " ")[0]
		}
	}

	signatures := make(map[string]string, 0)

	return []string{file.Name()}, hashes, signatures
}

func Test_checkHash_success(t *testing.T) {
	if file, err := ioutil.TempFile("", "somefile"); err != nil {
		panic(err)
	} else {
		defer syscall.Unlink(file.Name())

		paths, hashes, _ := genTestFile(file)

		if check, err := CheckHashes("", paths, hashes); err != nil {
			panic(err)
		} else if !check {
			t.Errorf("Failed hash check of test file. hashes: %v", hashes)
		}
	}
}
