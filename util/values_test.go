package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppendVals(t *testing.T) {
	prev := Values{"test": "test"}
	next := Values{"test": "test"}
	prev.Append(next)
	assert.Equal(t, 1, len(prev))
}

func TestSliceVars(t *testing.T) {
	vals := Values{"dep1": "dep2"}
	actual := vals.ToSlice()
	assert.Equal(t, len(vals)*2, len(actual))
}
