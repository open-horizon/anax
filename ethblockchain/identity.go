package ethblockchain

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

func readIdFromFs(colonusDir string, filename string) (string, error) {

	colonusSuffix := strings.Split(colonusDir, "/root/")[1]
	filepath := path.Join(os.Getenv("SNAP_COMMON"), colonusSuffix, filename)

	file, err := os.Open(filepath)
	defer file.Close()

	if err != nil {
		return "", fmt.Errorf("Error reading file: %s\n", filepath)
	} else if data, err := ioutil.ReadFile(file.Name()); err != nil {
		return "", err
	} else {
		res := strings.Trim(string(data), "\n\r ")
		if !strings.HasPrefix(res, "0x") {
			res = "0x" + res
		}
		return res, nil
	}
}

func DirectoryAddress(colonusDir string) (string, error) {
	return readIdFromFs(colonusDir, "directory.address")
}

func AccountId(colonusDir string) (string, error) {
	return readIdFromFs(colonusDir, "accounts")
}
