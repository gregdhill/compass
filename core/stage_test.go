package core

import (
	"sync"
	"testing"

	"github.com/monax/compass/helm"
	"github.com/stretchr/testify/assert"
)

func newTestChart() Stage {
	c := Stage{
		Abandon:   false,
		Namespace: "test",
		Chart: helm.Chart{
			Name:       "burrow",
			Repository: "stable",
			Version:    "",
			Release:    "test-release",
		},
	}
	return c
}

func TestShellJobs(t *testing.T) {
	vals := []string{"chart=test"}
	jobs := []string{"echo \"hello\""}
	shellJobs(vals, jobs, false)

	jobs = []string{"commandnotfound"}
	assert.Panics(t, func() { shellJobs(vals, jobs, false) })
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

	b.InstallChart(c.Chart, c.Namespace, nil)
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
