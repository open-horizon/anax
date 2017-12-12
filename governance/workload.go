package governance

import (
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"strings"
)

// Sorting functions used with the sort package
type WorkloadConfigByVersion []persistence.WorkloadConfig

func (s WorkloadConfigByVersion) Len() int {
	return len(s)
}

func (s WorkloadConfigByVersion) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s WorkloadConfigByVersion) Less(i, j int) bool {

	// Just compare the starting version in the two ranges
	first := s[i].VersionExpression[1:strings.Index(s[i].VersionExpression, ",")]
	second := s[j].VersionExpression[1:strings.Index(s[j].VersionExpression, ",")]

	c, _ := policy.CompareVersions(first, second)
	return c == -1
}
