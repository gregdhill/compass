package core

import (
	"sync"
	"testing"

	"github.com/monax/compass/helm"
	"github.com/stretchr/testify/assert"
)

func newTestChart() Stage {
	c := Stage{
		Abandon: false,
		Chart: helm.Chart{
			Name:      "burrow",
			Repo:      "stable",
			Version:   "",
			Release:   "test-release",
			Namespace: "test",
		},
	}
	return c
}

func TestShellJobs(t *testing.T) {
	vals := []string{"chart=test"}
	jobs := []string{"echo \"hello\""}
	shellJobs(vals, jobs, false)

	jobs = []string{"commandnotfound"}
	err := shellJobs(vals, jobs, false)
	assert.Errorf(t, err, "exec: \"commandnotfound\": executable file not found in $PATH")
}

func TestNewChart(t *testing.T) {
	b := helm.NewFakeBridge()
	c := newTestChart()

	wgs := make(Depends, 1)
	var w sync.WaitGroup
	w.Add(1)
	wgs["test"] = &w

	values := make(map[string]string, 1)
	err := c.Create(b, "test", values, false, &wgs)
	assert.NoError(t, err)
}

func TestNoNewChart(t *testing.T) {
	b := helm.NewFakeBridge()
	c := newTestChart()
	c.Abandon = true

	b.InstallChart(c.Chart, nil)
	out, _ := b.ReleaseStatus(c.Release)
	assert.Equal(t, "DEPLOYED", out)

	wgs := make(Depends, 1)
	var w sync.WaitGroup
	w.Add(1)
	wgs["test"] = &w

	values := make(map[string]string, 1)
	err := c.Create(b, "test", values, false, &wgs)
	assert.EqualError(t, err, "chart already installed")
}
