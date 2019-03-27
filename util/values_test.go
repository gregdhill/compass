package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppendVals(t *testing.T) {
	prev := Values(map[string]string{"test": "test"})
	next := Values(map[string]string{"test": "test"})
	prev.Append(next)
	assert.Equal(t, 1, len(prev))
}

func TestSliceVars(t *testing.T) {
	vals := Values(map[string]string{"dep1": "dep2"})
	actual := vals.ToSlice()
	assert.Equal(t, len(vals)*2, len(actual))
}
