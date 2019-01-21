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
