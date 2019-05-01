package sync_service

import (
	"fmt"
	"github.com/open-horizon/anax/cli/dev"
	"github.com/open-horizon/anax/cutil"
	"os"
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
