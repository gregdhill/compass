package main

import (
	"testing"

	"gotest.tools/assert"
)

func TestDeleteDep(t *testing.T) {
	deps := []string{"dep1", "dep2"}
	actual := deleteDep("dep2", deps)
	assert.Equal(t, 1, len(actual))
}

func TestShellVars(t *testing.T) {
	vals := map[string]string{"dep1": "dep2"}
	actual := shellVars(vals)
	assert.Equal(t, len(vals)*2, len(actual))
}
