package container

import (
	"errors"
	dockerclient "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"os"
	"path"
)

func listImages(client *dockerclient.Client) ([]dockerclient.APIImages, error) {

	if images, err := client.ListImages(dockerclient.ListImagesOptions{
		All: true,
	}); err != nil {
		return nil, err
	} else {
		return images, nil
	}
}

func isLoaded(client *dockerclient.Client, imageHash string) (bool, error) {
	if images, err := listImages(client); err != nil {
		return false, err
	} else {
		for _, image := range images {
			for k, v := range image.Labels {
				if k == "engineering.hovitos.colonus.imagehash" && v == imageHash {
					return true, nil
				}
			}
		}

		return false, nil
	}
}

func rem(dir string) {
	if err := os.RemoveAll(dir); err != nil {
		glog.Error(err)
	}
}

func loadImages(client *dockerclient.Client, imageFiles []string) error {
	if len(imageFiles) == 0 {
		return errors.New("Received zero-length imageFiles spec")
	}

	for _, imageFile := range imageFiles {
		if loaded, err := isLoaded(client, imageFile); err != nil {
			return err
		} else if !loaded {
			// do load

			glog.Infof("Doing docker load of image file: %v", imageFile)

			processedImage, err := ProcessTar(imageFile)
			if err != nil {
				return err
			}

			tmpDir, _ := path.Split(processedImage)

			// delete processed image after use
			defer rem(tmpDir)

			if fileStream, err := os.Open(processedImage); err != nil {
				return err
			} else {
				defer fileStream.Close()

				if err := client.LoadImage(dockerclient.LoadImageOptions{
					InputStream: fileStream,
				}); err != nil {
					return err
				}
			}
		} else {
			glog.Infof("Docker image file %v is already loaded, skipping it. (determined by label comparison)", imageFile)
			return nil
		}
	}

	return nil
}
