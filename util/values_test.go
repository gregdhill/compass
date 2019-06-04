package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppendVals(t *testing.T) {
	prev := Values{"test1": "test1"}
	next := Values{"test1": "test1"}
	prev.Append(next)
	assert.Equal(t, 1, len(prev))
	next = Values{"test2": "test2"}
	prev.Append(next)
	assert.Equal(t, 2, len(prev))
	assert.Equal(t, 1, len(next))

}

func TestSliceVars(t *testing.T) {
	vals := Values{"dep1": "dep2"}
	actual := vals.ToSlice()
	assert.Equal(t, len(vals), len(actual))
}

func TestCascade(t *testing.T) {
	var cascading = []struct {
		values   Values
		current  string
		name     string
		field    string
		expected string
	}{
		// explicit is best
		{Values{}, "something", "object", "index", "something"},

		// prefer fully-qualified name
		{Values{"index": "this", "object_index": "that"}, "", "object", "index", "that"},

		// use generalized otherwise
		{Values{"index": "this"}, "", "object", "index", "this"},
	}

	for _, tt := range cascading {
		actual := tt.values.Cascade(tt.current, tt.name, tt.field)
		if actual != tt.expected {
			t.Errorf("expected %s, actual %s", tt.expected, actual)
		}
	}

}
