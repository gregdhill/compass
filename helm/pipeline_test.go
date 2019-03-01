package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeVals(t *testing.T) {
	prev := map[string]string{"test": "test"}
	next := map[string]string{"test": "test"}
	MergeVals(prev, next)
	assert.Equal(t, 1, len(prev))
}
