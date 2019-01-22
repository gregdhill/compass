package main

import (
	"testing"

	"gotest.tools/assert"
)

func TestMergeVals(t *testing.T) {
	prev := map[string]string{"test": "test"}
	next := map[string]string{"test": "test"}
	mergeVals(prev, next)
	assert.Equal(t, 1, len(prev))
}
