package clusterupgrade

import (
	"bufio"
	"fmt"
	"github.com/golang/glog"
	"io/ioutil"
	"os"
	"strings"
)

func ReadAgentConfigFile(filename string) (map[string]string, error) {
	configInMap := make(map[string]string)

	if len(filename) == 0 {
		return configInMap, nil
	}

	file, err := os.Open(filename)
	if err != nil {
		glog.Errorf(fmt.Sprintf("Failed to get read agent config %v: %v", filename, err))
		return configInMap, err
	}
	defer file.Close()

	sc := bufio.NewScanner(file)
	for sc.Scan() {
		line := sc.Text()
		if keyValue := strings.Split(line, "="); len(keyValue) != 2 {
			return configInMap, fmt.Errorf(fmt.Sprintf("failed to parse content in agent config file %v", filename))
		} else {
			glog.V(5).Infof(cuwlog(fmt.Sprintf("get %v=%v", keyValue[0], keyValue[1])))
			configInMap[keyValue[0]] = keyValue[1]
		}
	}

	if err = sc.Err(); err != nil {
		glog.Errorf(fmt.Sprintf("Failed to get scan agent config %v: %v", filename, err))
	}

	return configInMap, err
}

func ReadAgentCertFile(filename string) ([]byte, error) {
	certFile, err := ioutil.ReadFile(filename)
	if err != nil {
		return make([]byte, 0), err
	}
	glog.V(5).Infof(cuwlog(fmt.Sprintf("get cert content %v", string(certFile))))
	return certFile, nil
}
