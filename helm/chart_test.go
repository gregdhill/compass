package helm

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newTestChart() Chart {
	c := Chart{
		Name:      "burrow",
		Repo:      "stable",
		Version:   "",
		Release:   "test-release",
		Namespace: "test",
	}
	return c
}

func TestShellVars(t *testing.T) {
	vals := map[string]string{"dep1": "dep2"}
	actual := shellVars(vals)
	assert.Equal(t, len(vals)*2, len(actual))
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
	hc := newTestHelm()
	c := newTestChart()

	wgs := make(Depends, 1)
	var w sync.WaitGroup
	w.Add(1)
	wgs["test"] = &w

	values := make(map[string]string, 1)
	err := c.Make(hc, "test", values, false, &wgs)
	assert.NoError(t, err)
}

func TestNoNewChart(t *testing.T) {
	hc := newTestHelm()
	c := newTestChart()
	c.Abandon = true

	installChart(hc.client, hc.envset, c, nil)
	out, _ := releaseStatus(hc.client, c.Release)
	assert.Equal(t, "DEPLOYED", out)

	wgs := make(Depends, 1)
	var w sync.WaitGroup
	w.Add(1)
	wgs["test"] = &w

	values := make(map[string]string, 1)
	err := c.Make(hc, "test", values, false, &wgs)
	assert.EqualError(t, err, "chart already installed")
}
