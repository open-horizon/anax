package container

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/image"
	"github.com/golang/glog"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

func reTar(image string, writer *tar.Writer) error {
	_, imageFname := path.Split(image)

	original, err := os.Open(image)
	if err != nil {
		return err
	}
	defer original.Close()

	gzReader, err := gzip.NewReader(original)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	for {
		head, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		// a legitimate entry!
		name := head.Name
		switch head.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg:

			_, fname := path.Split(name)
			if fname == "json" {
				// ok only because we're expecting to read small text files from tarball
				buffer := new(bytes.Buffer)

				_, err := io.Copy(buffer, tarReader)
				if err != nil {
					return err
				}

				revised, err := addLabel(buffer.Bytes(), "engineering.hovitos.colonus.imagehash", strings.Split(imageFname, ".")[0])
				if err != nil {
					return err
				}

				// copy revised file to new dst dir
				if err := writer.WriteHeader(&tar.Header{
					Name: head.Name,
					Mode: head.Mode,
					Size: int64(len(revised)),
				}); err != nil {
					return err
				}
				if _, err := writer.Write(revised); err != nil {
					return err
				}
			} else {
				// write existing content
				if err := writer.WriteHeader(head); err != nil {
					return err
				}
				if _, err := io.Copy(writer, tarReader); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func ProcessTar(file string) (string, error) {
	// rely on an external system to clean this up
	dstDir, err := ioutil.TempDir("", "colonus-")
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(dstDir, 0600); err != nil {
		return "", err
	}

	_, fname := path.Split(file)
	noext := strings.Split(fname, ".")[0]
	outFile, err := os.Create(path.Join(dstDir, fmt.Sprintf("%v%v", noext, ".tar")))
	if err != nil {
		return "", err
	}

	tarWriter := tar.NewWriter(outFile)

	err = reTar(file, tarWriter)
	// close before handling error
	if err := tarWriter.Close(); err != nil {
		glog.Errorf("Failed to close tarwriter: %v", err)
	}

	if err != nil {
		return "", err
	}

	return outFile.Name(), nil
}

func addLabel(data []byte, key, value string) ([]byte, error) {
	var image image.V1Image

	if err := json.Unmarshal(data, &image); err != nil {
		return nil, err
	}

	if image.ContainerConfig.Labels == nil {
		image.ContainerConfig.Labels = make(map[string]string, 0)
	}

	image.ContainerConfig.Labels[key] = value

	revised, err := json.Marshal(image)
	if err != nil {
		return nil, err
	}

	return revised, nil
}
