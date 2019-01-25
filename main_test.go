package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeVals(t *testing.T) {
	prev := map[string]string{"test": "test"}
	next := map[string]string{"test": "test"}
	mergeVals(prev, next)
	assert.Equal(t, 1, len(prev))
}

func TestLinter(t *testing.T) {
	c := Chart{
		Name: "test",
	}
	cs := map[string]*Chart{"Test": &c}
	p := Pipeline{
		Charts: cs,
	}
	assert.Panics(t, func() { lint(&p, nil) })

	p.Charts["Test"].Namespace = "test"
	p.Charts["Test"].Release = "test"
	lint(&p, nil)
}
