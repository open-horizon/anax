package sync_service

import (
	"fmt"
	"github.com/open-horizon/anax/cli/dev"
	"github.com/open-horizon/anax/cutil"
	"os"
	"strings"
)

func getFSSImageName() string {
	return fmt.Sprintf("openhorizon/%v_edge-sync-service", cutil.ArchString())
}

func getFSSImageTagName() string {
	tag := os.Getenv(dev.DEVTOOL_HZN_FSS_IMAGE_TAG)
	if tag == "" {
		tag = "latest"
	}
	return tag
}

func getFSSFullImageName() string {
	return fmt.Sprintf("%v:%v", getFSSImageName(), getFSSImageTagName())
}

func getCSSPort() string {
	port := os.Getenv(dev.DEVTOOL_HZN_FSS_CSS_PORT)
	if port == "" {
		port = "8580"
	}
	return port
}

func getMongoFullImage() string {
	image := os.Getenv(dev.DEVTOOL_HZN_FSS_MONGO_IMAGE)
	if image == "" {
		image = "mongo:4.0.6"
	}
	return image
}

func getMongoImageTag() string {
	parts := strings.Split(getMongoFullImage(), ":")
	return parts[1]
}

func getMongoImage() string {
	parts := strings.Split(getMongoFullImage(), ":")
	return parts[0]
}
