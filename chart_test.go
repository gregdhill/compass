package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestShellJobs(t *testing.T) {
	vals := []string{"chart=test"}
	jobs := []string{"-c echo \"hello\""}
	shellJobs(vals, jobs)

	jobs = []string{"-c commandnotfound"}
	assert.Panics(t, func() { shellJobs(vals, jobs) })
}
