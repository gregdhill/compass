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
	lint(&p, map[string]string{"Test_version": "1.1"})
	assert.Equal(t, "1.1", p.Charts["Test"].Version)

	p.Charts["Test"].Release = ""
	lint(&p, map[string]string{"release": "test-release"})
	assert.Equal(t, "test-release-test", p.Charts["Test"].Release)

	p.Charts["Test"].Release = ""
	lint(&p, map[string]string{"Test_release": "test-release"})
	assert.Equal(t, "test-release-test", p.Charts["Test"].Release)
}
