package torrent

import (
	"errors"
	dockerclient "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"os"
	"strings"
)

// this package encapsulates all docker image handling in the fetch process

func listImages(client *dockerclient.Client) ([]dockerclient.APIImages, error) {

	if images, err := client.ListImages(dockerclient.ListImagesOptions{
		All: true,
	}); err != nil {
		return nil, err
	} else {
		return images, nil
	}
}

// TODO: user needs to use image IDs instead of repotags to avoid overwriting or otherwise mistaken handling because of name collisions
func skipCheckFn(client *dockerclient.Client) func(repotag string) (bool, error) {

	return func(repotag string) (bool, error) {
		repotagParts := strings.Split(repotag, ":")

		if images, err := listImages(client); err != nil {
			return false, err
		} else {
			for _, image := range images {
				for _, r := range image.RepoTags {
					// don't permit skips over "latest" tag in case a newer version exists
					if r == repotag && repotagParts[1] != "latest" {
						return true, nil
					}
				}
			}

			return false, nil
		}
	}
}

func rem(dir string) {
	if err := os.RemoveAll(dir); err != nil {
		glog.Error(err)
	}
}

// imageFiles is a mapping of Pkg file path to docker image repotag
func loadImagesFromPkgParts(client *dockerclient.Client, imageFiles map[string]string) error {
	if len(imageFiles) == 0 {
		return errors.New("Received zero-length imageFiles spec")
	}

	for repotag, abspath := range imageFiles {
		if abspath == "" {
			// this indicates it was skipped from fetching because it was loaded already
			continue
		} else {

			glog.Infof("Doing docker load of image file: %v as docker image: %v", abspath, repotag)

			// wanna clean up the fetched file no matter what
			defer rem(abspath)

			if fileStream, err := os.Open(abspath); err != nil {
				return err
			} else {
				defer fileStream.Close()

				if err := client.LoadImage(dockerclient.LoadImageOptions{
					InputStream: fileStream,
				}); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
