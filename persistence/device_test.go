//go:build unit
// +build unit

package persistence

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_GetFormatedPatternString(t *testing.T) {

	var org, name, pattern string

	org, name, pattern = GetFormatedPatternString("", "org1")
	assert.Equal(t, "", org, "Empty org string should be returned for an empty pattern.")
	assert.Equal(t, "", name, "Empty name string should be returned for an empty pattern.")
	assert.Equal(t, "", pattern, "Empty pattern string should be returned for an empty pattern.")

	org, name, pattern = GetFormatedPatternString("pattern1", "org1")
	assert.Equal(t, "org1", org, "Use the device org as the pattern org.")
	assert.Equal(t, "pattern1", name, "Use the device org as the pattern org.")
	assert.Equal(t, "org1/pattern1", pattern, "Use the device org as the pattern org.")

	org, name, pattern = GetFormatedPatternString("org1/pattern1", "org1")
	assert.Equal(t, "org1", org, "The pattern org and the device org are the same.")
	assert.Equal(t, "pattern1", name, "The pattern org and the device org are the same.")
	assert.Equal(t, "org1/pattern1", pattern, "The pattern org and the device org are the same.")

	org, name, pattern = GetFormatedPatternString("org2/pattern1", "org1")
	assert.Equal(t, "org2", org, "The pattern org and the device org are different.")
	assert.Equal(t, "pattern1", name, "The pattern org and the device org are different.")
	assert.Equal(t, "org2/pattern1", pattern, "The pattern org and the device org are different.")

	org, name, pattern = GetFormatedPatternString("pattern1", "")
	assert.Equal(t, "", org, "No org string found")
	assert.Equal(t, "pattern1", name, "No org string found")
	assert.Equal(t, "pattern1", pattern, "No org string found")
}
