package persistence

import (
	"sort"
	"testing"
)

func Test_WorkloadConfig_sort(t *testing.T) {

	wcList := make([]WorkloadConfig, 0, 10)

	wcList = append(wcList, WorkloadConfig{
		WorkloadURL: "url1",
		Version:     "1.2.3"})

	wcList = append(wcList, WorkloadConfig{
		WorkloadURL: "url1",
		Version:     "1.1.3"})

	wcList = append(wcList, WorkloadConfig{
		WorkloadURL: "url1",
		Version:     "1.0.0"})

	sort.Sort(WorkloadConfigByVersion(wcList))

	if wcList[0].Version != "1.0.0" {
		t.Error("Oldest element should be 1.0.0")
	}

}
