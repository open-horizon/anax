package ethblockchain

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

func readIdFromFs(filename string) (string, error) {
	// TODO: change to SNAP_USER_COMMON if this can be a multi-user thing
	filepath := path.Join(os.Getenv("SNAP_COMMON"), "eth", filename)

	file, err := os.Open(filepath)
	defer file.Close()

	if err != nil {
		return "", fmt.Errorf("Error reading file: %s\n", filepath)
	} else if data, err := ioutil.ReadFile(file.Name()); err != nil {
		return "", err
	} else {
		return strings.Trim(string(data), "\n\r "), nil
	}
}

func DirectoryAddress() (string, error) {
	return readIdFromFs("directory.address")
}

func AccountId() (string, error) {
	return readIdFromFs("accounts")
}
