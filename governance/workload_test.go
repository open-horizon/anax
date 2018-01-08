// +build unit

package governance

import (
	"github.com/open-horizon/anax/persistence"
	"sort"
	"testing"
)

func Test_WorkloadConfig_sort1(t *testing.T) {

	wcList := make([]persistence.WorkloadConfig, 0, 10)

	wcList = append(wcList, persistence.WorkloadConfig{
		WorkloadURL:       "url1",
		VersionExpression: "[1.2.4,INFINITY)"})

	wcList = append(wcList, persistence.WorkloadConfig{
		WorkloadURL:       "url1",
		VersionExpression: "[1.1.3,2.0.0)"})

	wcList = append(wcList, persistence.WorkloadConfig{
		WorkloadURL:       "url1",
		VersionExpression: "(1.2.3,INFINITY)"})

	wcList = append(wcList, persistence.WorkloadConfig{
		WorkloadURL:       "url1",
		VersionExpression: "(1.2.3,2.0.0)"})

	sort.Sort(WorkloadConfigByVersion(wcList))

	if wcList[0].VersionExpression != "[1.1.3,2.0.0)" {
		t.Error("Oldest element should be 1.1.3,2.0.0")
		t.Logf("%v", wcList)
	} else if wcList[1].VersionExpression != "(1.2.3,INFINITY)" {
		t.Error("Oldest element should be 1.2.3,INFINITY")
		t.Logf("%v", wcList)
	}

}
